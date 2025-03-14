use std::fmt::{Display, Formatter};

use actix_web::ResponseError;
use actix_web::http::StatusCode;

#[derive(Debug)]
pub enum Error {
    NotFound,
    AlreadyExists,
    Internal,
    EmptyId,
    Unauthorized,
    UserNameTooShort,
    PasswordTooShort,
}

impl Display for Error {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        let code = match self {
            Error::AlreadyExists => "already-exists",
            Error::NotFound => "not-found",
            Error::Internal => "internal-error",
            Error::EmptyId => "empty-id",
            Error::Unauthorized => "unauthorized",
            Error::UserNameTooShort => "user-name-too-short",
            Error::PasswordTooShort => "password-too-short",
        };
        write!(f, "{code}")
    }
}

impl ResponseError for Error {
    fn status_code(&self) -> StatusCode {
        match self {
            Error::NotFound => StatusCode::NOT_FOUND,
            Error::Internal => StatusCode::INTERNAL_SERVER_ERROR,
            Error::Unauthorized => StatusCode::UNAUTHORIZED,
            Error::EmptyId
            | Error::AlreadyExists
            | Error::UserNameTooShort
            | Error::PasswordTooShort => StatusCode::BAD_REQUEST,
        }
    }
}
