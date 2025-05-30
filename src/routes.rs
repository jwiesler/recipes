use std::collections::HashMap;

use actix_web::HttpRequest;
use actix_web::web::{Data, Form, Html, Json, Path, Redirect, ServiceConfig};
use serde::Deserialize;
use serde_json::{Value, json};
use tracing::instrument;

use crate::auth::{Authenticated, NoPermission, WritePermission};
use crate::context::Context;
use crate::error::Error;
use crate::id::to_id_string;
use crate::recipe::{RawRecipe, bake_string};

#[actix_web::get("/")]
async fn page_home(ctx: Data<Context>, _: Authenticated<NoPermission>) -> Html {
    let recipes: HashMap<String, Value> = ctx
        .recipes.list().await.iter()
        .map(|(k, v)| (k.clone(), json!({
            "name": v.name.clone(),
            "categories": v.categories.iter().cloned().map(Value::String).collect::<Vec<_>>(),
        })))
        .collect();
    let context = json!({
        "base_url": "",
        "recipes": recipes
    });

    let rendered = ctx
        .templates
        .read()
        .await
        .render("home.html", context)
        .unwrap();
    Html::new(rendered)
}

#[actix_web::get("/login")]
async fn page_login(
    ctx: Data<Context>,
    Authenticated(NoPermission(user)): Authenticated<NoPermission>,
) -> Html {
    let context = json!({
        "base_url": "",
        "user": user.as_ref().map_or(Value::Null, |u| Value::String(bake_string(u))),
    });

    let rendered = ctx
        .templates
        .read()
        .await
        .render("login.html", context)
        .unwrap();
    Html::new(rendered)
}

#[actix_web::get("/recipe/{recipe}")]
#[instrument(skip(ctx))]
async fn page_recipe(
    ctx: Data<Context>,
    id: Path<String>,
    _: Authenticated<NoPermission>,
) -> Result<Html, Error> {
    let id = id.into_inner().to_lowercase();
    let recipe = ctx.recipes.get(&id).await?;
    let value = serde_json::to_value(recipe.bake()).unwrap();
    let context = json!({
        "base_url": "",
        "id": id,
        "recipe": value,
    });

    let rendered = ctx
        .templates
        .read()
        .await
        .render("recipe-page.html", context)
        .unwrap();
    Ok(Html::new(rendered))
}

#[actix_web::get("/create")]
async fn page_create(ctx: Data<Context>, _: Authenticated<NoPermission>) -> Html {
    let context = json!({
        "base_url": "",
    });
    let rendered = ctx
        .templates
        .read()
        .await
        .render("edit-recipe-page.html", context)
        .unwrap();
    Html::new(rendered)
}

#[actix_web::get("/edit/{recipe}")]
#[instrument(skip(ctx, u), fields(user=u.0.0))]
async fn page_edit(
    ctx: Data<Context>,
    id: Path<String>,
    u: Authenticated<NoPermission>,
) -> Result<Html, Error> {
    let id = id.into_inner().to_lowercase();
    let recipe = ctx.recipes.get(&id).await?;
    let value = serde_json::to_value(recipe).unwrap();
    let context = json!({
        "base_url": "",
        "id": id,
        "recipe": value,
    });
    let rendered = ctx
        .templates
        .read()
        .await
        .render("edit-recipe-page.html", context)
        .unwrap();
    Ok(Html::new(rendered))
}

#[actix_web::post("/create")]
#[instrument(skip(ctx, recipe, u), fields(name=%recipe.name, user=u.0.0))]
async fn create(
    ctx: Data<Context>,
    u: Authenticated<WritePermission>,
    Json(mut recipe): Json<RawRecipe>,
) -> Result<Redirect, Error> {
    recipe.clean();
    let id = to_id_string(&recipe.name);
    if id.is_empty() {
        return Err(Error::EmptyId);
    }
    let url = format!("/recipe/{id}");
    ctx.recipes.create(id, recipe).await?;
    Ok(Redirect::to(url).see_other())
}

#[actix_web::post("/edit/{recipe}")]
#[instrument(skip(ctx, recipe, u), fields(name=%recipe.name, user=u.0.0))]
async fn edit(
    ctx: Data<Context>,
    u: Authenticated<WritePermission>,
    id: Path<String>,
    Json(mut recipe): Json<RawRecipe>,
) -> Result<Redirect, Error> {
    let id = id.into_inner();
    recipe.clean();
    let new_id = to_id_string(&recipe.name);
    if new_id.is_empty() {
        return Err(Error::EmptyId);
    }
    let url = format!("/recipe/{new_id}");
    ctx.recipes.replace(&id, new_id, recipe).await?;
    Ok(Redirect::to(url).see_other())
}

#[actix_web::post("/delete/{recipe}")]
#[instrument(skip(ctx, u), fields(user=u.0.0))]
async fn delete(
    ctx: Data<Context>,
    u: Authenticated<WritePermission>,
    id: Path<String>,
) -> Result<Redirect, Error> {
    let id = id.into_inner();
    ctx.recipes.delete(&id).await?;
    Ok(Redirect::to("/").see_other())
}

#[derive(Deserialize)]
struct LoginForm {
    user: String,
    password: String,
}

#[actix_web::post("/register")]
#[instrument(skip(ctx, password))]
async fn register(
    ctx: Data<Context>,
    Form(LoginForm { user, password }): Form<LoginForm>,
) -> Result<Redirect, Error> {
    ctx.users.register(user, password).await?;
    Ok(Redirect::to("/").see_other())
}

#[actix_web::post("/login")]
#[instrument(skip(ctx, password, req))]
async fn login(
    ctx: Data<Context>,
    Form(LoginForm { user, password }): Form<LoginForm>,
    req: HttpRequest,
) -> Result<Redirect, Error> {
    ctx.users.login(user, password, &req).await?;
    Ok(Redirect::to("/").see_other())
}

#[actix_web::post("/invalidate-sessions")]
#[instrument(skip(ctx, req))]
async fn invalidate_sessions(
    ctx: Data<Context>,
    u: Authenticated<WritePermission>,
    req: HttpRequest,
) -> Result<Redirect, Error> {
    ctx.users.invalidate_sessions(u.0.0, &req).await?;
    Ok(Redirect::to("/").see_other())
}

pub(crate) fn configure(c: &mut ServiceConfig) {
    c.service(page_home)
        .service(page_login)
        .service(page_recipe)
        .service(page_create)
        .service(page_edit)
        .service(register)
        .service(login)
        .service(invalidate_sessions)
        .service(create)
        .service(edit)
        .service(delete);
}

#[cfg(test)]
pub(crate) mod tests {
    use std::path::Path;

    use actix_web::body::MessageBody;
    use actix_web::dev::{ServiceFactory, ServiceRequest, ServiceResponse};
    use actix_web::web::Data;
    use actix_web::{App, Error, http::header::ContentType, test};
    use tokio::sync::RwLock;

    use crate::auth::Users;
    use crate::context::Context;
    use crate::recipes::Recipes;
    use crate::templates::Templates;

    use super::configure;

    pub(crate) async fn make_app_data() -> Data<Context> {
        let recipes = Recipes::load_dir(Path::new("tests/recipes")).await;
        let users = Users::load(Path::new("tests/users.json").into()).await;
        let templates = RwLock::new(Templates::load("templates/**/*").await);
        Data::new(Context {
            templates,
            recipes,
            users,
        })
    }

    pub(crate) async fn app() -> App<
        impl ServiceFactory<
            ServiceRequest,
            Config = (),
            Response = ServiceResponse<impl MessageBody>,
            Error = Error,
            InitError = (),
        >,
    > {
        App::new()
            .app_data(make_app_data().await)
            .wrap(crate::middlewares::tracing())
            .wrap(crate::middlewares::identity())
            .wrap(crate::middlewares::session(&[0; 64]))
            .configure(configure)
    }

    #[actix_web::test]
    async fn test_home_page() {
        let app = test::init_service(app().await).await;
        let req = test::TestRequest::with_uri("/")
            .insert_header(ContentType::plaintext())
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert!(resp.status().is_success());
    }

    #[actix_web::test]
    async fn test_home_login() {
        let app = test::init_service(app().await).await;
        let req = test::TestRequest::with_uri("/login")
            .insert_header(ContentType::plaintext())
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert!(resp.status().is_success());
    }

    #[actix_web::test]
    async fn test_create_page() {
        let app = test::init_service(app().await).await;
        let req = test::TestRequest::with_uri("/create")
            .insert_header(ContentType::plaintext())
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert!(resp.status().is_success());
    }

    #[actix_web::test]
    async fn test_recipe_page() {
        let app = test::init_service(app().await).await;
        let req = test::TestRequest::with_uri("/recipe/test-1")
            .insert_header(ContentType::plaintext())
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert!(resp.status().is_success());
    }

    #[actix_web::test]
    async fn test_edit_page() {
        let app = test::init_service(app().await).await;
        let req = test::TestRequest::with_uri("/edit/test-1")
            .insert_header(ContentType::plaintext())
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert!(resp.status().is_success());
    }
}
