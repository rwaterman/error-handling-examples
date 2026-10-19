use std::env::{self, VarError};
use std::error::Error;
use std::net::{Ipv4Addr, SocketAddr};
use std::sync::Arc;

use error_handling::build_app;
use error_handling::logging::json_subscriber;
use error_handling::orders::repository::OrderRepository;
use error_handling::orders::service::OrderService;
use error_handling::orders::store::InMemoryOrderStore;
use tokio::net::TcpListener;
use tracing_subscriber::filter::LevelFilter;

const DEFAULT_PORT: u16 = 3000;

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    tracing::subscriber::set_global_default(json_subscriber(std::io::stdout, LevelFilter::INFO))?;

    let store = Arc::new(InMemoryOrderStore::default());
    let app = build_app(OrderService::new(OrderRepository::new(store)));

    let listener = TcpListener::bind(SocketAddr::from((Ipv4Addr::LOCALHOST, port()?))).await?;
    tracing::info!(address = %listener.local_addr()?, "listening");
    axum::serve(listener, app).await?;
    Ok(())
}

fn port() -> Result<u16, Box<dyn Error>> {
    match env::var("PORT") {
        Ok(value) => Ok(value.parse()?),
        Err(VarError::NotPresent) => Ok(DEFAULT_PORT),
        Err(error) => Err(error.into()),
    }
}
