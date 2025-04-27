#![deny(rust_2018_idioms, nonstandard_style)]
#![warn(future_incompatible)]
#![warn(clippy::pedantic)]
#![allow(clippy::module_name_repetitions)]

use std::path::{Path, PathBuf};
use std::time::Duration;

use actix_files::Files;
use actix_web::body::MessageBody;
use actix_web::dev::{ServiceRequest, ServiceResponse};
use actix_web::web::Data;
use actix_web::{App, Error, HttpResponse, HttpServer, middleware, web};
use config::Environment;
use notify::{RecursiveMode, Watcher};
use serde::Deserialize;
use tokio::sync::RwLock;
use tracing::metadata::LevelFilter;
use tracing::{Span, info};
use tracing_actix_web::{DefaultRootSpanBuilder, RootSpanBuilder};
use tracing_subscriber::EnvFilter;
use tracing_subscriber::fmt::format::FmtSpan;

use crate::auth::Users;
use crate::context::Context;
use crate::recipes::Recipes;
use crate::templates::Templates;

mod auth;
mod context;
mod error;
mod id;
mod middlewares;
mod recipe;
mod recipes;
mod routes;
mod templates;
mod unit;

#[derive(Default)]
pub(crate) struct DomainRootSpanBuilder;

impl RootSpanBuilder for DomainRootSpanBuilder {
    fn on_request_start(request: &ServiceRequest) -> Span {
        tracing_actix_web::root_span!(request)
    }

    fn on_request_end<B: MessageBody>(span: Span, outcome: &Result<ServiceResponse<B>, Error>) {
        DefaultRootSpanBuilder::on_request_end(span, outcome);
    }
}

pub(crate) fn setup_tracing() {
    tracing_subscriber::fmt()
        .with_span_events(FmtSpan::NEW | FmtSpan::CLOSE)
        .with_env_filter(
            EnvFilter::builder()
                .with_default_directive(LevelFilter::INFO.into())
                .from_env_lossy(),
        )
        .init();
}

#[derive(Deserialize)]
struct Config {
    address: String,
    cookies_key: String,
}

impl Config {
    fn load(path: PathBuf) -> Result<Config, config::ConfigError> {
        let config = config::Config::builder()
            .add_source(config::File::from(path))
            .add_source(Environment::with_prefix("RECIPES"))
            .build()?;

        config.try_deserialize()
    }
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    setup_tracing();
    let Config {
        address,
        cookies_key,
    } = Config::load("Recipes.toml".into()).unwrap();

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
        let cookies_middleware = middlewares::identity();
        let session_middleware = middlewares::session(cookies_key.as_bytes());
        App::new()
            .app_data(context.clone())
            .wrap(middlewares::tracing())
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
