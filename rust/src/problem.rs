use std::any::Any;
use std::error::Error;
use std::sync::Arc;

use axum::Json;
use axum::extract::Request;
use axum::http::{HeaderValue, StatusCode, header};
use axum::middleware::Next;
use axum::response::{IntoResponse, Response};
use serde::Serialize;

use crate::errors::{AppError, FieldError, error_chain};
use crate::request_id::{RequestId, X_REQUEST_ID};

pub const PROBLEM_JSON: &str = "application/problem+json";

type Cause = Arc<dyn Error + Send + Sync>;

/// An error response that is not rendered yet. Handlers return it inside the
/// response extensions; [`handle_problems`] renders it once it knows the
/// request path and request id.
#[derive(Clone, Debug)]
pub struct Problem {
    status: StatusCode,
    code: &'static str,
    detail: String,
    errors: Vec<FieldError>,
    cause: Option<Cause>,
}

impl Problem {
    pub fn new(status: StatusCode, code: &'static str, detail: impl Into<String>) -> Self {
        Self {
            status,
            code,
            detail: detail.into(),
            errors: Vec::new(),
            cause: None,
        }
    }

    fn with_errors(self, errors: Vec<FieldError>) -> Self {
        Self { errors, ..self }
    }

    fn with_cause(self, cause: AppError) -> Self {
        Self {
            cause: Some(Arc::new(cause)),
            ..self
        }
    }

    fn log_if_server_error(&self, request_id: &RequestId, path: &str) {
        if !self.status.is_server_error() {
            return;
        }
        let error = self.cause.as_deref().map(|cause| error_chain(cause));
        tracing::error!(%request_id, code = self.code, path, error, "request failed");
    }

    fn render(&self, request_id: &RequestId, path: &str) -> Response {
        let body = ProblemBody {
            kind: "about:blank",
            title: self.status.canonical_reason().unwrap_or_default(),
            status: self.status.as_u16(),
            detail: &self.detail,
            code: self.code,
            instance: path,
            request_id: request_id.as_str(),
            errors: &self.errors,
        };
        let content_type = [(header::CONTENT_TYPE, HeaderValue::from_static(PROBLEM_JSON))];
        (self.status, content_type, Json(body)).into_response()
    }
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct ProblemBody<'a> {
    #[serde(rename = "type")]
    kind: &'static str,
    title: &'static str,
    status: u16,
    detail: &'a str,
    code: &'static str,
    instance: &'a str,
    request_id: &'a str,
    #[serde(skip_serializing_if = "<[FieldError]>::is_empty")]
    errors: &'a [FieldError],
}

impl IntoResponse for Problem {
    fn into_response(self) -> Response {
        let mut response = self.status.into_response();
        response.extensions_mut().insert(self);
        response
    }
}

impl From<AppError> for Problem {
    fn from(error: AppError) -> Self {
        let problem = Self::new(error.status(), error.code(), error.detail());
        match error {
            AppError::Validation(errors) => problem.with_errors(errors),
            exceptional => problem.with_cause(exceptional),
        }
    }
}

impl IntoResponse for AppError {
    fn into_response(self) -> Response {
        Problem::from(self).into_response()
    }
}

/// The single place that turns problems into responses and logs server errors.
pub async fn handle_problems(request: Request, next: Next) -> Response {
    let request_id = RequestId::from_headers(request.headers());
    let path = request.uri().path().to_owned();

    let mut response = next.run(request).await;

    if let Some(problem) = response.extensions_mut().remove::<Problem>() {
        problem.log_if_server_error(&request_id, &path);
        response = problem.render(&request_id, &path);
    }
    response
        .headers_mut()
        .insert(X_REQUEST_ID, request_id.header_value());
    response
}

pub async fn route_not_found() -> Problem {
    Problem::new(StatusCode::NOT_FOUND, "ROUTE_NOT_FOUND", "Not Found")
}

#[derive(Debug, thiserror::Error)]
#[error("handler panicked: {message}")]
pub struct Panic {
    message: String,
}

impl Panic {
    fn from_payload(payload: &(dyn Any + Send)) -> Self {
        let message = payload
            .downcast_ref::<String>()
            .cloned()
            .or_else(|| {
                payload
                    .downcast_ref::<&str>()
                    .map(|text| (*text).to_owned())
            })
            .unwrap_or_else(|| "panic payload is not a string".to_owned());
        Self { message }
    }
}

pub fn panic_response(payload: Box<dyn Any + Send + 'static>) -> Response {
    AppError::Internal(Box::new(Panic::from_payload(payload.as_ref()))).into_response()
}
