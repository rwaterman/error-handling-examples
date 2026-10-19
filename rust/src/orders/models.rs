use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;
use validator::Validate;

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Order {
    pub id: Uuid,
    pub customer_email: String,
    pub items: Vec<OrderItem>,
    pub status: OrderStatus,
    pub created_at: DateTime<Utc>,
    pub processed_at: Option<DateTime<Utc>>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct OrderItem {
    pub sku: String,
    pub quantity: u32,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum OrderStatus {
    Pending,
    Processed,
}

// `default` turns a missing field into an empty value, so `validate()` reports
// it under its own field path instead of serde failing on the whole body.
#[derive(Debug, Default, Serialize, Deserialize, Validate)]
#[serde(default, deny_unknown_fields, rename_all = "camelCase")]
pub struct CreateOrderRequest {
    #[validate(email(message = "must be a valid email address"))]
    pub customer_email: String,

    #[validate(length(min = 1, max = 50, message = "must have 1 to 50 items"), nested)]
    pub items: Vec<OrderItemRequest>,
}

#[derive(Debug, Default, Serialize, Deserialize, Validate)]
#[serde(default, deny_unknown_fields)]
pub struct OrderItemRequest {
    #[validate(length(min = 1, max = 64, message = "must be 1 to 64 characters"))]
    pub sku: String,

    #[validate(range(min = 1, max = 100, message = "must be between 1 and 100"))]
    pub quantity: u32,
}

impl From<OrderItemRequest> for OrderItem {
    fn from(request: OrderItemRequest) -> Self {
        Self {
            sku: request.sku,
            quantity: request.quantity,
        }
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum GetOrderError {
    NotFound(Uuid),
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ProcessOrderError {
    NotFound(Uuid),
    AlreadyProcessed(Uuid),
}
