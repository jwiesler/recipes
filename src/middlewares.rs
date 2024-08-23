use crate::DomainRootSpanBuilder;
use actix_identity::config::LogoutBehaviour;
use actix_identity::IdentityMiddleware;
use actix_session::config::CookieContentSecurity;
use actix_session::storage::CookieSessionStore;
use actix_session::SessionMiddleware;
use actix_web::cookie::{Key, SameSite};
use std::time::Duration;
use tracing_actix_web::TracingLogger;

pub(crate) fn tracing() -> TracingLogger<DomainRootSpanBuilder> {
    TracingLogger::<DomainRootSpanBuilder>::new()
}

pub(crate) fn identity() -> IdentityMiddleware {
    IdentityMiddleware::builder()
        .visit_deadline(Some(Duration::from_secs(30 * 24 * 60 * 60)))
        .logout_behaviour(LogoutBehaviour::PurgeSession)
        .build()
}

pub(crate) fn session(cookies_key: &[u8]) -> SessionMiddleware<CookieSessionStore> {
    SessionMiddleware::builder(CookieSessionStore::default(), Key::from(cookies_key))
        .cookie_name("recipes-session".into())
        .cookie_http_only(true)
        .cookie_content_security(CookieContentSecurity::Private)
        .cookie_secure(true)
        .cookie_same_site(SameSite::Strict)
        .build()
}
