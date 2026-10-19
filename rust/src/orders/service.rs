use chrono::Utc;
use uuid::Uuid;

use crate::errors::AppError;
use crate::orders::models::{
    CreateOrderRequest, GetOrderError, Order, OrderStatus, ProcessOrderError,
};
use crate::orders::repository::OrderRepository;

/// The outer `Result` carries exceptional failures to the central handler.
/// The inner `Result` carries expected outcomes back to the route.
#[derive(Clone)]
pub struct OrderService {
    repository: OrderRepository,
}

impl OrderService {
    pub fn new(repository: OrderRepository) -> Self {
        Self { repository }
    }

    pub async fn create(&self, request: CreateOrderRequest) -> Result<Order, AppError> {
        let order = Order {
            id: Uuid::new_v4(),
            customer_email: request.customer_email,
            items: request.items.into_iter().map(Into::into).collect(),
            status: OrderStatus::Pending,
            created_at: Utc::now(),
            processed_at: None,
        };
        self.repository.save(&order).await?;
        Ok(order)
    }

    pub async fn get(&self, order_id: Uuid) -> Result<Result<Order, GetOrderError>, AppError> {
        let order = self.repository.find(order_id).await?;
        Ok(order.ok_or(GetOrderError::NotFound(order_id)))
    }

    pub async fn process(
        &self,
        order_id: Uuid,
    ) -> Result<Result<Order, ProcessOrderError>, AppError> {
        let Some(order) = self.repository.find(order_id).await? else {
            return Ok(Err(ProcessOrderError::NotFound(order_id)));
        };
        if order.status == OrderStatus::Processed {
            return Ok(Err(ProcessOrderError::AlreadyProcessed(order_id)));
        }
        let processed = Order {
            status: OrderStatus::Processed,
            processed_at: Some(Utc::now()),
            ..order
        };
        self.repository.save(&processed).await?;
        Ok(Ok(processed))
    }
}
