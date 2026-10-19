use std::io::{self, ErrorKind};
use std::sync::Arc;

use uuid::Uuid;

use crate::errors::AppError;
use crate::orders::models::Order;
use crate::orders::store::OrderStore;

const ORDER_STORE: &str = "order-store";

#[derive(Clone)]
pub struct OrderRepository {
    store: Arc<dyn OrderStore>,
}

impl OrderRepository {
    pub fn new(store: Arc<dyn OrderStore>) -> Self {
        Self { store }
    }

    pub async fn find(&self, order_id: Uuid) -> Result<Option<Order>, AppError> {
        let record = self
            .store
            .get(&order_id.to_string())
            .await
            .map_err(classify_store_error)?;
        Ok(record.map(serde_json::from_value).transpose()?)
    }

    pub async fn save(&self, order: &Order) -> Result<(), AppError> {
        let record = serde_json::to_value(order)?;
        self.store
            .put(&order.id.to_string(), record)
            .await
            .map_err(classify_store_error)
    }
}

fn classify_store_error(error: io::Error) -> AppError {
    match error.kind() {
        ErrorKind::ConnectionRefused
        | ErrorKind::ConnectionReset
        | ErrorKind::ConnectionAborted
        | ErrorKind::NotConnected
        | ErrorKind::TimedOut => AppError::DependencyUnavailable {
            dependency: ORDER_STORE,
            source: error,
        },
        _ => AppError::Internal(Box::new(error)),
    }
}
