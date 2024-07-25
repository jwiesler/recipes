use std::fmt::{Display, Formatter};

use actix_web::http::StatusCode;
use actix_web::ResponseError;

#[derive(Debug)]
pub enum Error {
    NotFound,
    AlreadyExists,
    Internal,
    EmptyId,
}

impl Display for Error {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        let code = match self {
            Error::AlreadyExists => "already-exists",
            Error::NotFound => "not-found",
            Error::Internal => "internal-error",
            Error::EmptyId => "empty-id",
        };
        write!(f, "{code}")
    }
}

impl ResponseError for Error {
    fn status_code(&self) -> StatusCode {
        match self {
            Error::AlreadyExists => StatusCode::BAD_REQUEST,
            Error::NotFound => StatusCode::NOT_FOUND,
            Error::Internal => StatusCode::INTERNAL_SERVER_ERROR,
            Error::EmptyId => StatusCode::BAD_REQUEST,
        }
    }
}
