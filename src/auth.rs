use std::collections::HashMap;
use std::collections::hash_map::Entry;
use std::future::Future;
use std::ops::Deref;
use std::path::PathBuf;
use std::pin::Pin;

use actix_identity::Identity;
use actix_session::Session;
use actix_web::dev::Payload;
use actix_web::{FromRequest, HttpMessage, HttpRequest, web};
use serde::{Deserialize, Serialize};
use tokio::fs::read_to_string;
use tokio::sync::{Mutex, RwLock};
use tokio::task::spawn_blocking;
use tracing::instrument;

use crate::context::Context;
use crate::error::Error;
use crate::recipes::handle_io_error;

#[derive(Debug)]
pub struct NoPermission(pub Option<String>);
#[derive(Debug)]
pub struct WritePermission(#[allow(unused)] pub String);

trait PermissionCheck: Sized {
    fn from_user(user: Option<String>) -> Option<Self>;
}

impl PermissionCheck for NoPermission {
    fn from_user(user: Option<String>) -> Option<Self> {
        Some(Self(user))
    }
}

impl PermissionCheck for WritePermission {
    fn from_user(user: Option<String>) -> Option<Self> {
        user.map(Self)
    }
}

#[derive(Debug)]
pub struct Authenticated<P = NoPermission>(pub P);

const TOKEN_VERSION_IDENTIFIER: &str = "token_version";

async fn user_from_request(
    identity: Option<Identity>,
    session: &Session,
    context: &Context,
) -> Option<String> {
    let identity = identity?;
    let user_id = identity.id().unwrap();
    let token_version = session.get(TOKEN_VERSION_IDENTIFIER).ok().flatten()?;

    context
        .users
        .check_authenticated(&user_id, token_version)
        .await
        .map(|()| user_id)
        .inspect_err(|_| {
            identity.logout();
        })
        .ok()
}

pub fn store_session_info(request: &HttpRequest, user_id: String, token_version: u32) {
    Identity::login(&request.extensions(), user_id).unwrap();
    Session::extract(request)
        .into_inner()
        .unwrap()
        .insert(TOKEN_VERSION_IDENTIFIER, token_version)
        .unwrap();
}

impl<T> FromRequest for Authenticated<T>
where
    T: PermissionCheck + 'static,
{
    type Error = actix_web::Error;
    type Future = Pin<Box<dyn Future<Output = Result<Authenticated<T>, actix_web::Error>>>>;

    fn from_request(req: &HttpRequest, payload: &mut Payload) -> Self::Future {
        let identity = Identity::from_request(req, payload).into_inner().ok();
        let session = Session::from_request(req, payload).into_inner().unwrap();
        let context = req
            .app_data::<web::Data<Context>>()
            .expect("context not set")
            .deref()
            .clone();
        Box::pin(async move {
            let user = user_from_request(identity, &session, &context).await;
            T::from_user(user)
                .map(Authenticated)
                .ok_or(Error::Unauthorized.into())
        })
    }
}

#[derive(Serialize, Deserialize)]
struct User {
    password: String,
    locked: bool,
    version: u32,
}

struct Write(String);

struct Io(PathBuf);

impl Io {
    fn prepare(users: &HashMap<String, User>) -> Write {
        let content = serde_json::to_string(users).unwrap();
        Write(content)
    }

    async fn write(&mut self, write: &Write) -> Result<(), Error> {
        tokio::fs::write(&self.0, &write.0)
            .await
            .map_err(|e| handle_io_error(&self.0, &e))
    }
}

#[instrument(level = "debug", skip(password))]
async fn bcrypt_hash(password: impl AsRef<[u8]> + Send + 'static, bcrypt_cost: u32) -> String {
    spawn_blocking(move || bcrypt::hash(password, bcrypt_cost))
        .await
        .unwrap()
        .expect("bcrypt failed")
}

#[instrument(level = "debug", skip(password))]
async fn bcrypt_verify(password: impl AsRef<[u8]> + Send + 'static, password_hash: String) -> bool {
    spawn_blocking(move || bcrypt::verify(password, &password_hash))
        .await
        .unwrap()
        .expect("bcrypt failed")
}

pub struct Users {
    index: RwLock<HashMap<String, User>>,
    io: Mutex<Io>,
}

impl Users {
    pub async fn load(path: PathBuf) -> Users {
        let text = read_to_string(&path)
            .await
            .unwrap_or_else(|e| panic!("Failed to read {path:?}: {e}"));
        let users = serde_json::from_str(&text).unwrap();
        Users {
            index: RwLock::new(users),
            io: Mutex::new(Io(path)),
        }
    }
}

impl Users {
    #[instrument(skip(self, password), err)]
    pub async fn register(&self, login: String, password: String) -> Result<(), Error> {
        if login.chars().count() < 4 {
            return Err(Error::UserNameTooShort);
        }
        if password.chars().count() < 8 {
            return Err(Error::PasswordTooShort);
        }
        let hash = bcrypt_hash(password, bcrypt::DEFAULT_COST).await;
        let mut io = self.io.lock().await;
        let mut users = self.index.write().await;
        match users.entry(login) {
            Entry::Occupied(_) => Err(Error::AlreadyExists),
            Entry::Vacant(e) => {
                e.insert(User {
                    password: hash,
                    locked: true,
                    version: 0,
                });
                let write = Io::prepare(&users);
                drop(users);
                io.write(&write).await
            }
        }
    }

    #[instrument(skip(self, password, req), err)]
    pub async fn login(
        &self,
        login: String,
        password: String,
        req: &HttpRequest,
    ) -> Result<(), Error> {
        let users = self.index.read().await;
        let (hash, version) = users
            .get(&login)
            .filter(|u| !u.locked)
            .map(|u| (u.password.clone(), u.version))
            .ok_or(Error::Unauthorized)?;
        let valid = bcrypt_verify(password, hash).await;
        valid.then_some(()).ok_or(Error::Unauthorized)?;
        store_session_info(req, login, version);
        Ok(())
    }

    #[instrument(skip(self, req), err)]
    pub async fn invalidate_sessions(&self, login: String, req: &HttpRequest) -> Result<(), Error> {
        let mut io = self.io.lock().await;
        let (write, version) = {
            let mut users = self.index.write().await;
            let user = users.get_mut(&login).ok_or(Error::NotFound)?;
            user.version += 1;
            let version = user.version;
            (Io::prepare(&users), version)
        };

        io.write(&write).await?;
        store_session_info(req, login, version);
        Ok(())
    }

    #[instrument(level = "debug", skip(self))]
    pub async fn check_authenticated(&self, login: &str, token_version: u32) -> Result<(), Error> {
        let users = self.index.read().await;
        users
            .get(login)
            .filter(|u| !u.locked && u.version == token_version)
            .map(|_| ())
            .ok_or(Error::Unauthorized)
    }
}

#[cfg(test)]
mod tests {
    use actix_web::cookie::Cookie;
    use actix_web::http::header::SET_COOKIE;
    use actix_web::http::{Method, StatusCode};
    use actix_web::test;

    use crate::routes::tests::app;
    use crate::setup_tracing;

    #[actix_web::test]
    async fn test_auth_flow() {
        setup_tracing();
        let app = test::init_service(app().await).await;

        let req = test::TestRequest::with_uri("/login")
            .method(Method::GET)
            .to_request();
        let resp = test::call_and_read_body(&app, req).await;
        let body = std::str::from_utf8(&resp).unwrap();
        assert!(!body.contains("admin"));

        let req = test::TestRequest::with_uri("/login")
            .method(Method::POST)
            .set_form(serde_json::json!({
                "user": "admin",
                "password": "adminadmin"
            }))
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert_eq!(resp.status(), StatusCode::SEE_OTHER);

        let cookie = resp
            .headers()
            .get(SET_COOKIE)
            .expect("expecting set cookie header");
        let cookie = Cookie::parse_encoded(cookie.to_str().unwrap()).unwrap();
        let req = test::TestRequest::with_uri("/login")
            .method(Method::GET)
            .cookie(cookie)
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert_eq!(resp.status(), StatusCode::OK);
        let body = test::read_body(resp).await;
        let body = std::str::from_utf8(&body).unwrap();
        assert!(body.contains("admin"));
    }
}
