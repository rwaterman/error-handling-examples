# Error handling examples

One small orders API, built four times in Python, TypeScript, Go, and Rust, to show error handling that is designed up front: each kind of failure has one deliberate place where it is handled. Every implementation passes the same scenarios, so you can read the same idea side by side in four idioms.

The structure follows HabibWahid's article [Junior Devs Use try-catch Everywhere. Senior Devs Use These 4 Exception Handling Patterns](https://blog.stackademic.com/junior-devs-use-try-catch-everywhere-senior-devs-use-these-4-exception-handling-patterns-dcd869ed6551), which makes the case in Java and Spring. This repository carries the same four patterns into languages that range from exception-based (Python, TypeScript) to value-based (Go, Rust).

- [The idea in one table](#the-idea-in-one-table)
- [The four patterns](#the-four-patterns)
- [Where each pattern lives](#where-each-pattern-lives)
- [What changes between languages](#what-changes-between-languages)
- [The antipattern, as a test](#the-antipattern-as-a-test)
- [Run it](#run-it)
- [Repository layout](#repository-layout)

## The idea in one table

Every failure belongs to one of four kinds, and each kind has exactly one place where it is handled.

| Kind                   | Example                                   | Where it is handled                         | Response | Logged |
| ---------------------- | ----------------------------------------- | ------------------------------------------- | -------- | ------ |
| Invalid input          | Malformed email, quantity of 0            | Validation at the HTTP boundary             | 400      | No     |
| Expected outcome       | Order not found, order already processed  | The route, from a returned value            | 404, 409 | No     |
| Dependency unavailable | The database refuses the connection       | The central handler, via a typed error      | 503      | Yes    |
| Unexpected             | Anything else, which means a bug          | The central handler, via a catch-all        | 500      | Yes    |

The [shared contract](docs/contract.md) pins down every status, error code, response body, and log record, plus the scenario list each implementation's tests follow.

## The four patterns

**1. Validate at the edge.** Parse the request into a typed, already-valid value before any business code runs. Code downstream never asks whether the input is good, so nothing catches errors caused by bad input.

**2. A small error hierarchy.** Exceptional failures get specific types that carry their own status and stable code. The data layer translates driver failures it recognizes, such as a refused connection, into one of these types and keeps the original as its cause. It leaves every other error alone, so a bug is never relabeled as an outage.

**3. One central handler.** A single function turns errors into responses. It renders every error in one format ([RFC 9457 problem details](https://www.rfc-editor.org/rfc/rfc9457)), attaches the request ID, logs each server error exactly once with its full cause chain, and returns a generic message so internal details never reach the client. Routes contain no error-mapping boilerplate.

**4. Result values for expected outcomes.** "Order not found" is a normal answer. The service returns it as a value, and the route maps it to a 404 explicitly. It is never thrown, and never logged as an error.

## Where each pattern lives

| Pattern                | Python | TypeScript | Go | Rust |
| ---------------------- | ------ | ---------- | -- | ---- |
| 1. Validate at the edge | [`models.py`](python/src/error_handling/orders/models.py) | [`validation.ts`](typescript/src/validation.ts), [`models.ts`](typescript/src/orders/models.ts) | [`validation.go`](go/orders/validation.go) | [`extract.rs`](rust/src/extract.rs), [`models.rs`](rust/src/orders/models.rs) |
| 2. Error hierarchy     | [`errors.py`](python/src/error_handling/errors.py), [`repository.py`](python/src/error_handling/orders/repository.py) | [`errors.ts`](typescript/src/errors.ts), [`repository.ts`](typescript/src/orders/repository.ts) | [`apperr.go`](go/apperr/apperr.go), [`repository.go`](go/orders/repository.go) | [`errors.rs`](rust/src/errors.rs), [`repository.rs`](rust/src/orders/repository.rs) |
| 3. Central handler     | [`problem.py`](python/src/error_handling/problem.py) | [`problem.ts`](typescript/src/problem.ts) | [`handler.go`](go/app/handler.go) | [`problem.rs`](rust/src/problem.rs) |
| 4. Result values       | [`result.py`](python/src/error_handling/result.py), [`service.py`](python/src/error_handling/orders/service.py) | [`result.ts`](typescript/src/result.ts), [`service.ts`](typescript/src/orders/service.ts) | [`order.go`](go/orders/order.go), [`app.go`](go/app/app.go) | [`service.rs`](rust/src/orders/service.rs), [`routes.rs`](rust/src/orders/routes.rs) |

Each language's README explains the idioms in more depth: [Python](python/README.md), [TypeScript](typescript/README.md), [Go](go/README.md), [Rust](rust/README.md).

## What changes between languages

The behavior is identical. The mechanics differ in ways that are worth noticing.

| Concern                   | Python               | TypeScript                 | Go                                  | Rust                                        |
| ------------------------- | -------------------- | -------------------------- | ----------------------------------- | ------------------------------------------- |
| Framework                 | FastAPI              | Hono                       | `net/http`                          | axum                                        |
| Validation                | Pydantic models      | Zod schemas                | `encoding/json/v2` and a validate step | serde and `validator` in a custom extractor |
| Keeping the cause         | `raise ... from err` | `new Error(msg, { cause })` | `%w` and `Unwrap()`                 | `#[source]` in a `thiserror` enum           |
| Matching an error type    | `except` by class    | `instanceof`               | `errors.Is`, `errors.AsType`        | `match` on the enum                         |
| Central handler           | exception handlers   | `app.onError`              | a handler adapter that receives returned errors | `IntoResponse` plus one rendering middleware |
| Expected outcome          | `Ok \| Err` union    | discriminated union        | a domain error the route checks     | the inner `Result` of `Result<Result<T, E>, AppError>` |
| Unhandled failure         | uncaught exception   | thrown error               | panic, caught by `recover`          | panic, caught by `CatchPanicLayer`          |
| Structured logs           | `logging` with a JSON formatter | pino             | `log/slog`                          | `tracing`                                   |

Go and Rust have no exceptions, so pattern 3 has to be built from returned values. In Go, every handler returns an `error`, and one adapter decides what each error means. In Rust, the outer `Result` carries exceptional failures to the central handler through `?`, and the inner `Result` carries expected outcomes back to the route. The type signature states which failures are routine.

Go also shows a split the other languages leave implicit. `Error()` is the operator-facing text, which may include the wrapped cause. `Detail()` is the client-safe sentence.

## The antipattern, as a test

Each implementation has an `antipattern_catch_everything` test. It wraps the repository lookup in a catch-everything block that returns "nothing found" on any error. Then it shows that a missing order and a database outage return the same answer, and that nothing is logged. A second test calls the repository directly and gets a typed dependency error instead.

## Run it

Each directory is a self-contained project. CI runs all four on every push.

| Language   | Requires        | Verify                                                                 | Serve                       |
| ---------- | --------------- | ---------------------------------------------------------------------- | --------------------------- |
| Python     | uv              | `cd python && uv sync && uv run poe verify`                            | `uv run poe serve` (port 8000) |
| TypeScript | Node.js 24+     | `cd typescript && npm ci && npm run verify`                            | `npm run serve` (port 8000) |
| Go         | Go 1.27+        | `cd go && go vet ./... && go test -race ./...`                         | `go run ./cmd/server` (port 8080) |
| Rust       | Rust, edition 2024 | `cd rust && cargo fmt --check && cargo clippy --all-targets -- -D warnings && cargo test` | `cargo run` (port 3000) |

Try it against any of them, using that implementation's port:

```sh
curl -si -X POST localhost:8000/orders \
  -H 'content-type: application/json' \
  -d '{"customerEmail":"ada@example.com","items":[{"sku":"BOOK-1","quantity":2}]}'

curl -si localhost:8000/orders/00000000-0000-4000-8000-000000000000 -H 'x-request-id: demo-1'
```

The second request returns a `404` problem body with `"code": "ORDER_NOT_FOUND"` and `"requestId": "demo-1"`.

## Repository layout

```text
docs/contract.md   the behavior every implementation must match
python/            FastAPI, the reference implementation
typescript/        Hono, Zod, pino
go/                standard library only
rust/              axum, thiserror, validator, tracing
```
