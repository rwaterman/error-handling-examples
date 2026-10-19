use axum::extract::State;
use axum::http::StatusCode;
use axum::routing::{get, post};
use axum::{Json, Router};
use uuid::Uuid;

use crate::errors::AppError;
use crate::extract::{OrderIdParam, ValidJson};
use crate::orders::models::{CreateOrderRequest, GetOrderError, Order, ProcessOrderError};
use crate::orders::service::OrderService;
use crate::problem::Problem;

pub fn order_routes(service: OrderService) -> Router {
    Router::new()
        .route("/orders", post(create_order))
        .route("/orders/{orderId}", get(get_order))
        .route("/orders/{orderId}/process", post(process_order))
        .with_state(service)
}

async fn create_order(
    State(service): State<OrderService>,
    ValidJson(request): ValidJson<CreateOrderRequest>,
) -> Result<(StatusCode, Json<Order>), AppError> {
    let order = service.create(request).await?;
    Ok((StatusCode::CREATED, Json(order)))
}

async fn get_order(
    State(service): State<OrderService>,
    OrderIdParam(order_id): OrderIdParam,
) -> Result<Result<Json<Order>, Problem>, AppError> {
    let outcome = service.get(order_id).await?;
    Ok(outcome.map(Json).map_err(Problem::from))
}

async fn process_order(
    State(service): State<OrderService>,
    OrderIdParam(order_id): OrderIdParam,
) -> Result<Result<Json<Order>, Problem>, AppError> {
    let outcome = service.process(order_id).await?;
    Ok(outcome.map(Json).map_err(Problem::from))
}

impl From<GetOrderError> for Problem {
    fn from(error: GetOrderError) -> Self {
        match error {
            GetOrderError::NotFound(order_id) => order_not_found(order_id),
        }
    }
}

impl From<ProcessOrderError> for Problem {
    fn from(error: ProcessOrderError) -> Self {
        match error {
            ProcessOrderError::NotFound(order_id) => order_not_found(order_id),
            ProcessOrderError::AlreadyProcessed(order_id) => Problem::new(
                StatusCode::CONFLICT,
                "ORDER_ALREADY_PROCESSED",
                format!("Order {order_id} was already processed."),
            ),
        }
    }
}

fn order_not_found(order_id: Uuid) -> Problem {
    Problem::new(
        StatusCode::NOT_FOUND,
        "ORDER_NOT_FOUND",
        format!("Order {order_id} does not exist."),
    )
}
