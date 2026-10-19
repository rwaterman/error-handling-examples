mod common;

use std::sync::Arc;

use axum::http::StatusCode;
use common::{app_with, get, post_json, post_raw, send, valid_order, with_field};
use error_handling::orders::store::InMemoryOrderStore;
use serde_json::{Value, json};

fn healthy_app() -> axum::Router {
    app_with(Arc::new(InMemoryOrderStore::default()))
}

fn without_customer_email() -> Value {
    json!({ "items": valid_order()["items"] })
}

#[tokio::test]
async fn invalid_order_is_rejected_with_400_naming_the_field() {
    let cases = [
        (without_customer_email(), "customerEmail"),
        (
            with_field("customerEmail", json!("not-an-email")),
            "customerEmail",
        ),
        (with_field("items", json!([])), "items"),
        (
            with_field("items", json!([{ "sku": "BOOK-1", "quantity": 0 }])),
            "items.0.quantity",
        ),
        (
            with_field("items", json!([{ "sku": "", "quantity": 1 }])),
            "items.0.sku",
        ),
    ];
    let app = healthy_app();

    for (payload, field) in cases {
        let response = send(&app, post_json("/orders", &payload)).await;

        assert_eq!(response.status, StatusCode::BAD_REQUEST, "{payload}");
        assert_eq!(response.content_type(), "application/problem+json");
        assert_eq!(response.json()["code"], "VALIDATION_FAILED");
        assert!(
            response.error_fields().contains(&field.to_owned()),
            "{field} missing from {}",
            response.text
        );
    }
}

#[tokio::test]
async fn invalid_order_never_reaches_the_store() {
    let store = Arc::new(InMemoryOrderStore::default());
    let app = app_with(store.clone());

    send(
        &app,
        post_json("/orders", &with_field("customerEmail", json!("nope"))),
    )
    .await;

    assert!(store.is_empty().await);
}

#[tokio::test]
async fn unknown_field_is_rejected() {
    let response = send(
        &healthy_app(),
        post_json("/orders", &with_field("discountCode", json!("FREE"))),
    )
    .await;

    assert_eq!(response.status, StatusCode::BAD_REQUEST);
    assert_eq!(response.json()["code"], "VALIDATION_FAILED");
}

#[tokio::test]
async fn malformed_json_is_a_validation_failure() {
    let response = send(&healthy_app(), post_raw("/orders", "{not json")).await;

    assert_eq!(response.status, StatusCode::BAD_REQUEST);
    assert_eq!(response.json()["code"], "VALIDATION_FAILED");
    assert_eq!(response.error_fields(), ["body"]);
}

#[tokio::test]
async fn malformed_order_id_is_rejected_before_lookup() {
    let response = send(&healthy_app(), get("/orders/not-a-uuid")).await;

    assert_eq!(response.status, StatusCode::BAD_REQUEST);
    assert_eq!(response.error_fields(), ["orderId"]);
}

#[tokio::test]
async fn valid_order_is_created() {
    let response = send(&healthy_app(), post_json("/orders", &valid_order())).await;

    assert_eq!(response.status, StatusCode::CREATED);
    assert_eq!(response.content_type(), "application/json");
    let body = response.json();
    assert_eq!(body["customerEmail"], "ada@example.com");
    assert_eq!(body["status"], "pending");
    assert_eq!(body["processedAt"], Value::Null);
}
