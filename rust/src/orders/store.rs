use std::collections::HashMap;
use std::io;

use async_trait::async_trait;
use serde_json::Value;
use tokio::sync::RwLock;

/// A key-value store driver. Its errors are `io::Error`, the closest analog to
/// the connection and timeout errors a real database client raises.
#[async_trait]
pub trait OrderStore: Send + Sync {
    async fn get(&self, key: &str) -> io::Result<Option<Value>>;
    async fn put(&self, key: &str, value: Value) -> io::Result<()>;
}

#[derive(Debug, Default)]
pub struct InMemoryOrderStore {
    records: RwLock<HashMap<String, Value>>,
}

impl InMemoryOrderStore {
    pub async fn is_empty(&self) -> bool {
        self.records.read().await.is_empty()
    }
}

#[async_trait]
impl OrderStore for InMemoryOrderStore {
    async fn get(&self, key: &str) -> io::Result<Option<Value>> {
        Ok(self.records.read().await.get(key).cloned())
    }

    async fn put(&self, key: &str, value: Value) -> io::Result<()> {
        self.records.write().await.insert(key.to_owned(), value);
        Ok(())
    }
}
