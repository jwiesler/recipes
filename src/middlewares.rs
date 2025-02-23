use crate::DomainRootSpanBuilder;
use actix_identity::IdentityMiddleware;
use actix_identity::config::LogoutBehaviour;
use actix_session::SessionMiddleware;
use actix_session::config::{CookieContentSecurity, PersistentSession};
use actix_session::storage::CookieSessionStore;
use actix_web::cookie::{Key, SameSite};
use std::time::Duration;
use tracing_actix_web::TracingLogger;

const SESSION_DURATION: Duration = Duration::from_secs(30 * 24 * 60 * 60);

pub(crate) fn tracing() -> TracingLogger<DomainRootSpanBuilder> {
    TracingLogger::<DomainRootSpanBuilder>::new()
}

pub(crate) fn identity() -> IdentityMiddleware {
    IdentityMiddleware::builder()
        .visit_deadline(Some(SESSION_DURATION))
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
        .session_lifecycle(
            PersistentSession::default().session_ttl(SESSION_DURATION.try_into().unwrap()),
        )
        .build()
}
