mod common;

use std::sync::Arc;

use axum::Router;
use axum::http::StatusCode;
use common::{LogCapture, app_with, get, post, post_json, send, valid_order};
use error_handling::orders::models::{CreateOrderRequest, GetOrderError, ProcessOrderError};
use error_handling::orders::repository::OrderRepository;
use error_handling::orders::service::OrderService;
use error_handling::orders::store::InMemoryOrderStore;
use uuid::Uuid;

fn service() -> OrderService {
    OrderService::new(OrderRepository::new(
        Arc::new(InMemoryOrderStore::default()),
    ))
}

fn healthy_app() -> Router {
    app_with(Arc::new(InMemoryOrderStore::default()))
}

fn create_request() -> CreateOrderRequest {
    serde_json::from_value(valid_order()).expect("valid order deserializes")
}

async fn create_order(app: &Router) -> String {
    let response = send(app, post_json("/orders", &valid_order())).await;
    response.json()["id"]
        .as_str()
        .expect("order has an id")
        .to_owned()
}

#[tokio::test]
async fn get_unknown_order_returns_not_found_value() {
    let order_id = Uuid::new_v4();

    let outcome = service().get(order_id).await.expect("store is healthy");

    assert_eq!(outcome, Err(GetOrderError::NotFound(order_id)));
}

#[tokio::test]
async fn processing_twice_returns_already_processed_value() {
    let service = service();
    let order = service
        .create(create_request())
        .await
        .expect("store is healthy");
    let first = service.process(order.id).await.expect("store is healthy");

    let second = service.process(order.id).await.expect("store is healthy");

    assert!(first.is_ok());
    assert_eq!(second, Err(ProcessOrderError::AlreadyProcessed(order.id)));
}

#[tokio::test]
async fn unknown_order_maps_to_404() {
    let order_id = Uuid::new_v4();

    let response = send(&healthy_app(), get(&format!("/orders/{order_id}"))).await;

    assert_eq!(response.status, StatusCode::NOT_FOUND);
    assert_eq!(response.content_type(), "application/problem+json");
    let body = response.json();
    assert_eq!(body["code"], "ORDER_NOT_FOUND");
    assert_eq!(body["detail"], format!("Order {order_id} does not exist."));
}

#[tokio::test]
async fn already_processed_order_maps_to_409() {
    let app = healthy_app();
    let order_id = create_order(&app).await;
    send(&app, post(&format!("/orders/{order_id}/process"))).await;

    let response = send(&app, post(&format!("/orders/{order_id}/process"))).await;

    assert_eq!(response.status, StatusCode::CONFLICT);
    let body = response.json();
    assert_eq!(body["code"], "ORDER_ALREADY_PROCESSED");
    assert_eq!(
        body["detail"],
        format!("Order {order_id} was already processed.")
    );
}

#[tokio::test]
async fn processing_a_pending_order_succeeds() {
    let app = healthy_app();
    let order_id = create_order(&app).await;

    let response = send(&app, post(&format!("/orders/{order_id}/process"))).await;

    assert_eq!(response.status, StatusCode::OK);
    let body = response.json();
    assert_eq!(body["status"], "processed");
    assert!(
        body["processedAt"]
            .as_str()
            .is_some_and(|at| at.ends_with('Z'))
    );
}

#[tokio::test]
async fn expected_outcomes_are_not_logged_as_errors() {
    let (logs, _guard) = LogCapture::start();
    let app = healthy_app();
    let order_id = create_order(&app).await;
    send(&app, post(&format!("/orders/{order_id}/process"))).await;

    let conflict = send(&app, post(&format!("/orders/{order_id}/process"))).await;
    let missing = send(&app, get(&format!("/orders/{}", Uuid::new_v4()))).await;

    assert_eq!(conflict.status, StatusCode::CONFLICT);
    assert_eq!(missing.status, StatusCode::NOT_FOUND);
    assert_eq!(
        logs.records_at_or_above_warn(),
        Vec::<serde_json::Value>::new()
    );
}
