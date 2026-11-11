//! `ApiJson`: like `axum::Json`, but rejections use our error envelope.

use axum::extract::{FromRequest, Request};
use serde::de::DeserializeOwned;

use crate::error::ApiError;

pub struct ApiJson<T>(pub T);

impl<S, T> FromRequest<S> for ApiJson<T>
where
    S: Send + Sync,
    T: DeserializeOwned,
{
    type Rejection = ApiError;

    /// TODO: extract with `axum::Json<T>` and map every rejection (bad syntax,
    /// wrong types, missing content type) to `ApiError::BadRequest`.
    async fn from_request(_req: Request, _state: &S) -> Result<Self, Self::Rejection> {
        todo!("ApiJson::from_request")
    }
}
