mod common;

use std::error::Error;
use std::io::{self, ErrorKind};

use axum::http::StatusCode;
use common::{FailingOrderStore, repository_with};
use error_handling::errors::AppError;
use uuid::Uuid;

fn source_io_error(error: &AppError) -> &io::Error {
    error
        .source()
        .and_then(|source| source.downcast_ref::<io::Error>())
        .expect("the original io::Error is reachable as the source")
}

#[test]
fn dependency_unavailable_carries_its_status_and_code() {
    let error = AppError::DependencyUnavailable {
        dependency: "order-store",
        source: io::Error::from(ErrorKind::ConnectionRefused),
    };

    assert_eq!(error.status(), StatusCode::SERVICE_UNAVAILABLE);
    assert_eq!(error.code(), "DEPENDENCY_UNAVAILABLE");
    assert!(matches!(
        error,
        AppError::DependencyUnavailable {
            dependency: "order-store",
            ..
        }
    ));
}

#[tokio::test]
async fn repository_translates_driver_errors_and_keeps_the_cause() {
    let driver_errors = [
        (ErrorKind::ConnectionRefused, "db-01:5432 refused"),
        (ErrorKind::TimedOut, "read timed out"),
    ];

    for (kind, message) in driver_errors {
        let repository = repository_with(FailingOrderStore::new(kind, message));

        let error = repository
            .find(Uuid::new_v4())
            .await
            .expect_err("store is down");

        assert!(
            matches!(error, AppError::DependencyUnavailable { .. }),
            "{error:?}"
        );
        let cause = source_io_error(&error);
        assert_eq!(cause.kind(), kind);
        assert_eq!(cause.to_string(), message);
    }
}

#[tokio::test]
async fn repository_lets_unexpected_errors_through_untouched() {
    let repository = repository_with(FailingOrderStore::new(
        ErrorKind::InvalidData,
        "customerEmail",
    ));

    let error = repository
        .find(Uuid::new_v4())
        .await
        .expect_err("store has a bug");

    assert!(matches!(error, AppError::Internal(_)), "{error:?}");
    let cause = source_io_error(&error);
    assert_eq!(cause.kind(), ErrorKind::InvalidData);
    assert_eq!(cause.to_string(), "customerEmail");
}
