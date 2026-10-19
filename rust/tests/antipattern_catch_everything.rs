mod common;

use std::io::ErrorKind;

use common::{FailingOrderStore, LogCapture, repository_with};
use error_handling::errors::AppError;
use error_handling::orders::models::Order;
use error_handling::orders::repository::OrderRepository;
use error_handling::orders::store::InMemoryOrderStore;
use uuid::Uuid;

async fn find_order_swallowing_errors(
    repository: &OrderRepository,
    order_id: Uuid,
) -> Option<Order> {
    repository.find(order_id).await.ok().flatten()
}

#[tokio::test]
async fn swallowing_makes_an_outage_indistinguishable_from_a_missing_order() {
    let (logs, _guard) = LogCapture::start();
    let healthy = repository_with(InMemoryOrderStore::default());
    let down = repository_with(FailingOrderStore::new(
        ErrorKind::ConnectionRefused,
        "db-01:5432 refused",
    ));

    let missing = find_order_swallowing_errors(&healthy, Uuid::new_v4()).await;
    let outage = find_order_swallowing_errors(&down, Uuid::new_v4()).await;

    assert_eq!(missing, None);
    assert_eq!(outage, None);
    assert!(logs.records().is_empty());
}

#[tokio::test]
async fn letting_it_propagate_keeps_the_outage_visible() {
    let down = repository_with(FailingOrderStore::new(
        ErrorKind::ConnectionRefused,
        "db-01:5432 refused",
    ));

    let outcome = down.find(Uuid::new_v4()).await;

    assert!(
        matches!(outcome, Err(AppError::DependencyUnavailable { .. })),
        "{outcome:?}"
    );
}
