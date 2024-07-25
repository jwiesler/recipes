use std::path::Path;

use actix_files::Files;
use actix_web::{App, Error, HttpResponse, HttpServer, middleware, web};
use actix_web::body::MessageBody;
use actix_web::dev::{ServiceRequest, ServiceResponse};
use actix_web::web::Data;
use clap::Parser;
use tracing::{info, Span};
use tracing_actix_web::{DefaultRootSpanBuilder, RootSpanBuilder, TracingLogger};
use tracing_subscriber::fmt::format::FmtSpan;

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

    let Cli { address } = Cli::parse_from(std::env::args_os());

    let recipes = Recipes::load_dir(Path::new("recipes")).await;
    let templates = Templates::load("templates/**/*").await;
    let context = Data::new(Context { templates, recipes });
    let server = HttpServer::new(move || {
        App::new()
            .app_data(context.clone())
            .wrap(middleware::Compress::default())
            .wrap(TracingLogger::<DomainRootSpanBuilder>::new())
            .service(Files::new("/static", "./static"))
            .configure(routes::configure)
            .default_service(web::to(HttpResponse::NotFound))
    });

    info!("Connecting to {}", address);
    let server = server.bind(address)?;

    server.run().await
}
