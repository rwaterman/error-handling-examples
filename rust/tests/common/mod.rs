#![allow(dead_code)]

use std::io::{self, ErrorKind, Write};
use std::sync::{Arc, Mutex, MutexGuard};

use async_trait::async_trait;
use axum::Router;
use axum::body::Body;
use axum::http::{HeaderMap, Request, StatusCode, header};
use error_handling::build_app;
use error_handling::logging::json_subscriber;
use error_handling::orders::repository::OrderRepository;
use error_handling::orders::service::OrderService;
use error_handling::orders::store::OrderStore;
use http_body_util::BodyExt;
use serde_json::{Value, json};
use tower::ServiceExt;
use tracing::subscriber::DefaultGuard;
use tracing_subscriber::filter::LevelFilter;
use tracing_subscriber::fmt::MakeWriter;

pub const SECRET: &str = "postgres://admin:hunter2@db-01:5432/orders";

pub fn valid_order() -> Value {
    json!({
        "customerEmail": "ada@example.com",
        "items": [{ "sku": "BOOK-1", "quantity": 2 }],
    })
}

pub fn with_field(field: &str, value: Value) -> Value {
    let mut order = valid_order();
    order[field] = value;
    order
}

/// Fails every call with a fresh `io::Error`, because `io::Error` is not `Clone`.
pub struct FailingOrderStore {
    kind: ErrorKind,
    message: String,
}

impl FailingOrderStore {
    pub fn new(kind: ErrorKind, message: &str) -> Self {
        Self {
            kind,
            message: message.to_owned(),
        }
    }

    fn error(&self) -> io::Error {
        io::Error::new(self.kind, self.message.clone())
    }
}

#[async_trait]
impl OrderStore for FailingOrderStore {
    async fn get(&self, _key: &str) -> io::Result<Option<Value>> {
        Err(self.error())
    }

    async fn put(&self, _key: &str, _value: Value) -> io::Result<()> {
        Err(self.error())
    }
}

pub struct PanickingOrderStore;

#[async_trait]
impl OrderStore for PanickingOrderStore {
    async fn get(&self, _key: &str) -> io::Result<Option<Value>> {
        panic!("store invariant broken: {SECRET}");
    }

    async fn put(&self, _key: &str, _value: Value) -> io::Result<()> {
        panic!("store invariant broken: {SECRET}");
    }
}

pub fn repository_with(store: impl OrderStore + 'static) -> OrderRepository {
    OrderRepository::new(Arc::new(store))
}

pub fn app_with(store: Arc<dyn OrderStore>) -> Router {
    build_app(OrderService::new(OrderRepository::new(store)))
}

pub struct TestResponse {
    pub status: StatusCode,
    pub headers: HeaderMap,
    pub text: String,
}

impl TestResponse {
    pub fn json(&self) -> Value {
        serde_json::from_str(&self.text).expect("response body is JSON")
    }

    pub fn header(&self, name: &str) -> &str {
        self.headers
            .get(name)
            .unwrap_or_else(|| panic!("response has a {name} header"))
            .to_str()
            .expect("header is ASCII")
    }

    pub fn content_type(&self) -> &str {
        self.header(header::CONTENT_TYPE.as_str())
    }

    pub fn error_fields(&self) -> Vec<String> {
        self.json()["errors"]
            .as_array()
            .expect("problem has errors")
            .iter()
            .map(|error| {
                error["field"]
                    .as_str()
                    .expect("field is a string")
                    .to_owned()
            })
            .collect()
    }
}

pub async fn send(app: &Router, request: Request<Body>) -> TestResponse {
    let response = app
        .clone()
        .oneshot(request)
        .await
        .expect("router is infallible");
    let status = response.status();
    let headers = response.headers().clone();
    let bytes = response
        .into_body()
        .collect()
        .await
        .expect("body is readable")
        .to_bytes();
    let text = String::from_utf8(bytes.to_vec()).expect("body is UTF-8");
    TestResponse {
        status,
        headers,
        text,
    }
}

pub fn get(uri: &str) -> Request<Body> {
    Request::get(uri)
        .body(Body::empty())
        .expect("valid request")
}

pub fn get_with_request_id(uri: &str, request_id: &str) -> Request<Body> {
    Request::get(uri)
        .header("x-request-id", request_id)
        .body(Body::empty())
        .expect("valid request")
}

pub fn post(uri: &str) -> Request<Body> {
    Request::post(uri)
        .body(Body::empty())
        .expect("valid request")
}

pub fn post_json(uri: &str, body: &Value) -> Request<Body> {
    post_raw(uri, body.to_string())
}

pub fn post_raw(uri: &str, body: impl Into<Body>) -> Request<Body> {
    Request::post(uri)
        .header(header::CONTENT_TYPE, "application/json")
        .body(body.into())
        .expect("valid request")
}

#[derive(Clone, Default)]
pub struct LogCapture {
    buffer: Arc<Mutex<Vec<u8>>>,
}

impl LogCapture {
    /// Captures every event, at every level, on the current thread until the guard drops.
    pub fn start() -> (Self, DefaultGuard) {
        let capture = Self::default();
        let subscriber = json_subscriber(capture.clone(), LevelFilter::TRACE);
        (capture, tracing::subscriber::set_default(subscriber))
    }

    pub fn records(&self) -> Vec<Value> {
        let buffer = self.lock();
        String::from_utf8_lossy(&buffer)
            .lines()
            .map(|line| serde_json::from_str(line).expect("each log line is JSON"))
            .collect()
    }

    pub fn records_at_or_above_warn(&self) -> Vec<Value> {
        self.records()
            .into_iter()
            .filter(|record| matches!(record["level"].as_str(), Some("WARN" | "ERROR")))
            .collect()
    }

    pub fn single_error_record(&self) -> Value {
        let errors: Vec<Value> = self
            .records()
            .into_iter()
            .filter(|record| record["level"] == "ERROR")
            .collect();
        assert_eq!(
            errors.len(),
            1,
            "expected exactly one ERROR record, got {errors:?}"
        );
        errors.into_iter().next().expect("one record")
    }

    fn lock(&self) -> MutexGuard<'_, Vec<u8>> {
        self.buffer.lock().expect("log buffer lock is not poisoned")
    }
}

pub struct LogWriter(LogCapture);

impl Write for LogWriter {
    fn write(&mut self, bytes: &[u8]) -> io::Result<usize> {
        self.0.lock().extend_from_slice(bytes);
        Ok(bytes.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

impl<'writer> MakeWriter<'writer> for LogCapture {
    type Writer = LogWriter;

    fn make_writer(&'writer self) -> Self::Writer {
        LogWriter(self.clone())
    }
}
