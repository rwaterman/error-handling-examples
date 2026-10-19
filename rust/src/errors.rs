use std::error::Error;
use std::io;

use axum::http::StatusCode;
use serde::Serialize;

pub type BoxError = Box<dyn Error + Send + Sync>;

#[derive(Clone, Debug, PartialEq, Eq, Serialize)]
pub struct FieldError {
    pub field: String,
    pub message: String,
}

impl FieldError {
    pub fn new(field: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            field: field.into(),
            message: message.into(),
        }
    }
}

#[derive(Debug, thiserror::Error)]
pub enum AppError {
    #[error("request validation failed")]
    Validation(Vec<FieldError>),

    #[error("dependency {dependency} is unavailable")]
    DependencyUnavailable {
        dependency: &'static str,
        #[source]
        source: io::Error,
    },

    #[error("unexpected error")]
    Internal(#[source] BoxError),
}

impl AppError {
    pub fn status(&self) -> StatusCode {
        match self {
            Self::Validation(_) => StatusCode::BAD_REQUEST,
            Self::DependencyUnavailable { .. } => StatusCode::SERVICE_UNAVAILABLE,
            Self::Internal(_) => StatusCode::INTERNAL_SERVER_ERROR,
        }
    }

    pub fn code(&self) -> &'static str {
        match self {
            Self::Validation(_) => "VALIDATION_FAILED",
            Self::DependencyUnavailable { .. } => "DEPENDENCY_UNAVAILABLE",
            Self::Internal(_) => "INTERNAL_ERROR",
        }
    }

    pub fn detail(&self) -> String {
        match self {
            Self::Validation(_) => "The request is invalid.".to_owned(),
            Self::DependencyUnavailable { dependency, .. } => {
                format!("A required dependency is unavailable: {dependency}.")
            }
            Self::Internal(_) => "An unexpected error occurred.".to_owned(),
        }
    }
}

impl From<serde_json::Error> for AppError {
    fn from(error: serde_json::Error) -> Self {
        Self::Internal(Box::new(error))
    }
}

pub fn error_chain(error: &(dyn Error + 'static)) -> String {
    std::iter::successors(Some(error), |&current| current.source())
        .map(ToString::to_string)
        .collect::<Vec<_>>()
        .join(": ")
}
