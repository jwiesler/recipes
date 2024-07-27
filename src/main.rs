use std::path::Path;
use std::time::Duration;

use actix_files::Files;
use actix_identity::config::LogoutBehaviour;
use actix_identity::IdentityMiddleware;
use actix_session::config::CookieContentSecurity;
use actix_session::SessionMiddleware;
use actix_session::storage::CookieSessionStore;
use actix_web::{App, Error, HttpResponse, HttpServer, middleware, web};
use actix_web::body::MessageBody;
use actix_web::cookie::{Key, SameSite};
use actix_web::dev::{ServiceRequest, ServiceResponse};
use actix_web::web::Data;
use clap::Parser;
use notify::{RecursiveMode, Watcher};
use tokio::sync::RwLock;
use tracing::{info, Span};
use tracing_actix_web::{DefaultRootSpanBuilder, RootSpanBuilder, TracingLogger};
use tracing_subscriber::fmt::format::FmtSpan;

use crate::auth::Users;
use crate::context::Context;
use crate::recipes::Recipes;
use crate::templates::Templates;

mod auth;
mod context;
mod error;
mod id;
mod recipe;
mod recipes;
mod routes;
mod templates;
mod unit;

#[derive(Parser)]
struct Cli {
    address: String,
    #[clap(long)]
    cookies_key: String,
}

#[derive(Default)]
struct DomainRootSpanBuilder;

impl RootSpanBuilder for DomainRootSpanBuilder {
    fn on_request_start(request: &ServiceRequest) -> Span {
        tracing_actix_web::root_span!(request)
    }

    fn on_request_end<B: MessageBody>(span: Span, outcome: &Result<ServiceResponse<B>, Error>) {
        DefaultRootSpanBuilder::on_request_end(span, outcome);
    }
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    tracing_subscriber::fmt()
        .with_span_events(FmtSpan::NEW | FmtSpan::CLOSE)
        .init();

    let Cli {
        address,
        cookies_key,
    } = Cli::parse_from(std::env::args_os());

    let recipes = Recipes::load_dir(Path::new("recipes")).await;
    let templates = RwLock::new(Templates::load("templates/**/*").await);
    let users = Users::load(Path::new("users.json").into()).await;
    let context = Data::new(Context {
        templates,
        recipes,
        users,
    });

    let _watcher = {
        let (rx, mut tx) = tokio::sync::watch::channel(());

        let mut watcher = notify::recommended_watcher(move |res| match res {
            Ok(_) => {
                let _ = rx.send(());
            }
            Err(e) => info!("watch error: {:?}", e),
        })
        .unwrap();

        watcher
            .watch(Path::new("templates"), RecursiveMode::Recursive)
            .unwrap();

        let context = (*context).clone();
        tokio::spawn(async move {
            while let Ok(()) = tx.changed().await {
                tokio::time::sleep(Duration::from_millis(200)).await;
                tx.mark_unchanged();
                info!("Reloading templates");
                let templates = Templates::load("templates/**/*").await;
                *context.templates.write().await = templates;
            }
        });

        watcher
    };

    let server = HttpServer::new(move || {
        let cookies_middleware = IdentityMiddleware::builder()
            .visit_deadline(Some(Duration::from_secs(30 * 24 * 60 * 60)))
            .logout_behaviour(LogoutBehaviour::PurgeSession)
            .build();
        let session_middleware = SessionMiddleware::builder(
            CookieSessionStore::default(),
            Key::from(cookies_key.as_bytes()),
        )
        .cookie_name("recipes-session".into())
        .cookie_http_only(true)
        .cookie_content_security(CookieContentSecurity::Private)
        .cookie_secure(true)
        .cookie_same_site(SameSite::Strict)
        .build();
        App::new()
            .wrap(TracingLogger::<DomainRootSpanBuilder>::new())
            .app_data(context.clone())
            .wrap(middleware::Compress::default())
            .wrap(cookies_middleware)
            .wrap(session_middleware)
            .service(Files::new("/static", "./static"))
            .configure(routes::configure)
            .default_service(web::to(HttpResponse::NotFound))
    });

    info!("Connecting to {}", address);
    let server = server.bind(address)?;

    server.run().await
}
