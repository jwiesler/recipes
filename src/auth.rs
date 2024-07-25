use std::future::{ready, Ready};
use std::marker::PhantomData;

use actix_web::{Error, FromRequest, HttpRequest};
use actix_web::dev::Payload;
use actix_web::error::ErrorUnauthorized;

pub struct NoPermission;
pub struct WritePermission;

pub trait PermissionCheck {
    fn check() -> bool;
}

impl PermissionCheck for NoPermission {
    fn check() -> bool {
        true
    }
}

impl PermissionCheck for WritePermission {
    fn check() -> bool {
        true
    }
}

pub struct Authenticated<P = NoPermission> {
    _marker: PhantomData<P>,
}

impl<T: PermissionCheck> Authenticated<T> {
    fn new() -> Result<Self, Error> {
        if T::check() {
            Ok(Authenticated {
                _marker: PhantomData,
            })
        } else {
            Err(ErrorUnauthorized("unauthorized"))
        }
    }
}

impl<T> FromRequest for Authenticated<T>
where
    T: PermissionCheck,
{
    type Error = Error;
    type Future = Ready<Result<Authenticated<T>, Error>>;

    fn from_request(_: &HttpRequest, _: &mut Payload) -> Self::Future {
        ready(Self::new())
    }
}
