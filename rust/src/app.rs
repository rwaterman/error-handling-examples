use axum::{Router, middleware};
use tower_http::catch_panic::CatchPanicLayer;

use crate::orders::routes::order_routes;
use crate::orders::service::OrderService;
use crate::problem::{handle_problems, panic_response, route_not_found};

/// `handle_problems` is the outermost layer, so it also renders the 500 that
/// `CatchPanicLayer` produces for a panicking handler.
pub fn build_app(service: OrderService) -> Router {
    order_routes(service)
        .fallback(route_not_found)
        .layer(CatchPanicLayer::custom(panic_response))
        .layer(middleware::from_fn(handle_problems))
}
