mod common;

use std::io::ErrorKind;
use std::sync::Arc;

use axum::http::StatusCode;
use common::{
    FailingOrderStore, LogCapture, PanickingOrderStore, SECRET, app_with, get, get_with_request_id,
    send,
};
use uuid::Uuid;

fn order_uri() -> String {
    format!("/orders/{}", Uuid::new_v4())
}

#[tokio::test]
async fn dependency_outage_returns_503_without_leaking_internals() {
    let (logs, _guard) = LogCapture::start();
    let app = app_with(Arc::new(FailingOrderStore::new(
        ErrorKind::ConnectionRefused,
        SECRET,
    )));
    let uri = order_uri();

    let response = send(&app, get_with_request_id(&uri, "req-503")).await;

    assert_eq!(response.status, StatusCode::SERVICE_UNAVAILABLE);
    assert_eq!(response.content_type(), "application/problem+json");
    let body = response.json();
    assert_eq!(body["code"], "DEPENDENCY_UNAVAILABLE");
    assert_eq!(
        body["detail"],
        "A required dependency is unavailable: order-store."
    );
    assert_eq!(body["requestId"], "req-503");
    assert_eq!(body["instance"], uri);
    assert_eq!(response.header("x-request-id"), "req-503");
    assert!(!response.text.contains(SECRET));
    let record = logs.single_error_record();
    assert_eq!(record["request_id"], "req-503");
    assert_eq!(record["code"], "DEPENDENCY_UNAVAILABLE");
    assert_eq!(record["path"], uri);
    assert!(
        record["error"]
            .as_str()
            .is_some_and(|error| error.contains(SECRET))
    );
}

#[tokio::test]
async fn unexpected_error_returns_generic_500_and_logs_the_real_one() {
    let (logs, _guard) = LogCapture::start();
    let app = app_with(Arc::new(FailingOrderStore::new(ErrorKind::Other, SECRET)));

    let response = send(&app, get_with_request_id(&order_uri(), "req-500")).await;

    assert_eq!(response.status, StatusCode::INTERNAL_SERVER_ERROR);
    assert_eq!(response.content_type(), "application/problem+json");
    let body = response.json();
    assert_eq!(body["code"], "INTERNAL_ERROR");
    assert_eq!(body["detail"], "An unexpected error occurred.");
    assert_eq!(body["requestId"], "req-500");
    assert!(!response.text.contains(SECRET));
    let record = logs.single_error_record();
    assert_eq!(record["request_id"], "req-500");
    assert_eq!(record["code"], "INTERNAL_ERROR");
    assert!(
        record["error"]
            .as_str()
            .is_some_and(|error| error.contains(SECRET))
    );
}

#[tokio::test]
async fn a_panic_in_a_handler_returns_the_generic_500_and_is_logged() {
    let (logs, _guard) = LogCapture::start();
    let app = app_with(Arc::new(PanickingOrderStore));

    let response = send(&app, get_with_request_id(&order_uri(), "req-panic")).await;

    assert_eq!(response.status, StatusCode::INTERNAL_SERVER_ERROR);
    assert_eq!(response.content_type(), "application/problem+json");
    let body = response.json();
    assert_eq!(body["code"], "INTERNAL_ERROR");
    assert_eq!(body["detail"], "An unexpected error occurred.");
    assert_eq!(body["requestId"], "req-panic");
    assert_eq!(response.header("x-request-id"), "req-panic");
    assert!(!response.text.contains(SECRET));
    let record = logs.single_error_record();
    assert_eq!(record["request_id"], "req-panic");
    assert_eq!(record["code"], "INTERNAL_ERROR");
    assert!(
        record["error"]
            .as_str()
            .is_some_and(|error| error.contains(SECRET))
    );
}

#[tokio::test]
async fn request_id_is_generated_when_the_caller_sends_none() {
    let app = app_with(Arc::new(FailingOrderStore::new(ErrorKind::Other, "boom")));

    let response = send(&app, get(&order_uri())).await;

    let body = response.json();
    let request_id = body["requestId"].as_str().expect("requestId is a string");
    assert!(!request_id.is_empty());
    assert_eq!(response.header("x-request-id"), request_id);
}

#[tokio::test]
async fn untrusted_request_id_is_replaced() {
    let app = app_with(Arc::new(FailingOrderStore::new(ErrorKind::Other, "boom")));

    let response = send(&app, get_with_request_id(&order_uri(), "has space")).await;

    let body = response.json();
    let request_id = body["requestId"].as_str().expect("requestId is a string");
    assert_ne!(request_id, "has space");
    assert!(Uuid::parse_str(request_id).is_ok());
    assert_eq!(response.header("x-request-id"), request_id);
}

#[tokio::test]
async fn unknown_route_uses_the_same_problem_format() {
    let app = app_with(Arc::new(FailingOrderStore::new(ErrorKind::Other, "unused")));

    let response = send(&app, get("/nope")).await;

    assert_eq!(response.status, StatusCode::NOT_FOUND);
    assert_eq!(response.content_type(), "application/problem+json");
    let body = response.json();
    assert_eq!(body["code"], "ROUTE_NOT_FOUND");
    assert_eq!(body["detail"], "Not Found");
    assert_eq!(body["title"], "Not Found");
    assert_eq!(body["type"], "about:blank");
}
