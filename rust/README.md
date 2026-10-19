# Error handling in Rust

The orders API from the [shared contract](../docs/contract.md), built with axum. It passes the same scenarios as the Python reference, plus the Rust-specific panic scenario.

- [Running it](#running-it)
- [How the four patterns look in Rust](#how-the-four-patterns-look-in-rust)
- [Notes on the contract](#notes-on-the-contract)

## Running it

Requires a Rust toolchain with edition 2024 support. It was developed on Rust 1.99.

```sh
cargo test
cargo clippy --all-targets -- -D warnings
cargo fmt --check
cargo run                # listens on 127.0.0.1:3000; set PORT to change it
```

```sh
curl -i -X POST localhost:3000/orders -H 'content-type: application/json' \
  -d '{"customerEmail":"ada@example.com","items":[{"sku":"BOOK-1","quantity":2}]}'
curl -i localhost:3000/orders/0b9e5a4e-8f43-4f7e-9d0a-7f6d1c2b3a4f
```

Logs are JSON lines on stdout from `tracing-subscriber`.

## How the four patterns look in Rust

Rust has no exceptions. A failure is either a value in a `Result` or a panic. The type system is what separates expected outcomes from exceptional failures here.

**1. Validate at the edge** ([`src/extract.rs`](src/extract.rs), [`src/orders/models.rs`](src/orders/models.rs)). Request types derive `Deserialize` with `deny_unknown_fields` and `Validate` from the `validator` crate. The `ValidJson<T>` extractor runs three stages and maps each failure to a `VALIDATION_FAILED` field error: axum's JSON rejection becomes `body`, a serde shape error takes its field path from `serde_path_to_error`, and `validate()` errors are flattened into dot paths such as `items.0.quantity`. `OrderIdParam` does the same for the path segment, so a handler that receives these types never sees invalid input.

**2. A typed error hierarchy** ([`src/errors.rs`](src/errors.rs), [`src/orders/repository.rs`](src/orders/repository.rs)). `AppError` is a `thiserror` enum whose variants carry their status and code through `status()` and `code()`. The repository classifies store errors: connection and timeout `io::Error` kinds become `DependencyUnavailable` with the driver error as `#[source]`, and every other error becomes `Internal` with the original boxed as its source. `?` with `From<serde_json::Error>` propagates the rest.

**3. One central handler** ([`src/problem.rs`](src/problem.rs), [`src/app.rs`](src/app.rs)). `impl IntoResponse for AppError` turns an error into a `Problem` and stores it in the response extensions, because `IntoResponse` has no access to the request. The `handle_problems` middleware is the outermost layer and knows the request ID and path. It renders every `Problem` as `application/problem+json`, logs each 5xx exactly once at ERROR with `request_id`, `code`, `path`, and the error's source chain, and sets `x-request-id` on every response. `CatchPanicLayer` sits inside that middleware and turns a panic into the same `Internal` error, so a panic gets the generic 500 body and the same single log record. The router fallback returns a `ROUTE_NOT_FOUND` problem through the same path.

**4. Result values for expected outcomes** ([`src/orders/service.rs`](src/orders/service.rs), [`src/orders/routes.rs`](src/orders/routes.rs)). Service methods return `Result<Result<Order, ProcessOrderError>, AppError>`. The outer `Result` holds exceptional failures and the route's `?` sends them to the central handler. The inner `Result` holds expected outcomes such as `NotFound` and `AlreadyProcessed`, which the route maps to 404 and 409 problems with `From` impls. Those are never logged. A single flat enum holding both kinds would let a caller handle an outage and a missing order in the same `match` arm, which is the confusion the article warns about.

The antipattern test ([`tests/antipattern_catch_everything.rs`](tests/antipattern_catch_everything.rs)) shows the Rust form of a catch-everything block: `repository.find(id).await.ok().flatten()` turns an outage into `None`.

## Notes on the contract

- Store errors are modeled as `std::io::Error`, the closest analog to the connection and timeout errors a database driver returns. The Python reference re-raises an unexpected error with the same object identity. Rust has to return one error type from the repository, so the equivalent guarantee is that the original `io::Error` is reachable through `source()` and downcasting, and that an error outside the recognized kinds is classified as `Internal`. It is never classified as `DependencyUnavailable`. The pattern 2 tests assert exactly that.
- Request types use `#[serde(default)]` so that a missing field deserializes to an empty value and `validate()` reports it under its own field name. Without it, serde rejects the whole body and only knows the path of the enclosing object. The domain `Order` type has no defaults, so a corrupt stored record still fails loudly.
- `validator` reports Rust field names and ignores `#[serde(rename_all)]`, so the extractor converts them to camelCase.
- The default panic hook still prints the panic message to stderr. That line is outside the JSON log stream, and the single ERROR record comes from the central handler.
- `405 Method Not Allowed` is out of scope in the contract. axum returns it with an empty body and the `x-request-id` header.
