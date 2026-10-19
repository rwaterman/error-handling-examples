# Error handling in Go

A standard-library-only `net/http` service that implements the [shared contract](../docs/contract.md). It has no third-party dependencies.

- [Run it](#run-it)
- [How the four patterns look in Go](#how-the-four-patterns-look-in-go)
- [The antipattern](#the-antipattern)

## Run it

Requires Go 1.27 or later. The code uses `encoding/json/v2`, the `uuid` package, and `errors.AsType`, all of which are available in the standard library as of Go 1.27.

```sh
go test -race ./...
go vet ./...
go run ./cmd/server            # http://127.0.0.1:8080, override with -addr
```

```sh
curl -s -X POST localhost:8080/orders \
  -H 'content-type: application/json' \
  -d '{"customerEmail":"ada@example.com","items":[{"sku":"BOOK-1","quantity":2}]}'
```

The scenario tests live in [`app/`](app) and mirror the Python test files one to one, with a Go-specific panic scenario added to pattern 3.

## How the four patterns look in Go

Go has no exceptions. Errors are ordinary values returned as the last result, so each pattern becomes a convention about which values a function returns and who inspects them.

### 1. Validate at the edge

File: [`orders/validation.go`](orders/validation.go).

`DecodeCreateOrderRequest` decodes the body with `encoding/json/v2` and `RejectUnknownMembers`, then checks every rule and returns either a validated `CreateOrderRequest` or one `*apperr.ValidationError` listing every bad field. Decode failures carry a JSON Pointer, which becomes the dotted field path (`/items/0/quantity` becomes `items.0.quantity`). `ParseOrderID` does the same for the path segment. The service only accepts the validated type, so nothing downstream rechecks input.

The v1 `encoding/json` decoder is a poor fit here. Its unknown-field error has no path and no type to match on, it matches field names case-insensitively, and it accepts duplicate keys. The v2 package rejects all three.

### 2. A small error hierarchy

Files: [`apperr/apperr.go`](apperr/apperr.go), [`orders/repository.go`](orders/repository.go).

The hierarchy is an interface: any error with `Status()`, `Code()`, and `Detail()` methods is an `AppError`, and callers find it anywhere in a wrapped chain with `errors.AsType[apperr.AppError]`. The interface separates two audiences. `Error()` is for operators and includes the wrapped cause. `Detail()` is the only text a client sees.

The repository wraps every store error with `%w` for context. It then checks the chain for `syscall.ECONNREFUSED`, `syscall.ECONNRESET`, or a `net.Error` whose `Timeout()` is true (which covers `context.DeadlineExceeded` and `os.ErrDeadlineExceeded`). Only those become a `*apperr.DependencyUnavailableError` that keeps the original in `Err` and exposes it through `Unwrap()`. Every other error passes through as a bug. The contract's "same identity" rule means `errors.Is(err, original)` holds, which is how Go expresses identity through a wrapped chain.

### 3. One central handler

Files: [`app/handler.go`](app/handler.go), [`app/problem.go`](app/problem.go), [`app/requestid.go`](app/requestid.go).

Route handlers have the signature `func(http.ResponseWriter, *http.Request) error` and return any error they do not handle. The `handle` adapter is the single place that turns those errors into RFC 9457 problem responses: an `AppError` supplies its own status, code, and detail, and anything else becomes the generic 500. It logs each 5xx exactly once at ERROR with `request_id`, `code`, `path`, and the full `error` text, and it logs nothing for a 4xx.

A panic is Go's closest thing to an uncaught exception. The `recoverPanics` middleware turns one into the same generic 500 and ERROR record, with the stack attached. The request ID middleware sits outside it and sets `x-request-id` before any handler runs, so the header is present even on a panic.

### 4. Result values for expected outcomes

Files: [`orders/service.go`](orders/service.go), [`orders/order.go`](orders/order.go), [`app/app.go`](app/app.go).

Go needs no `Result` type, because `(Order, error)` already is one. The expected outcomes are small comparable error types, `NotFoundError` and `AlreadyProcessedError`, that carry the order ID. The service documents which ones each method returns. A test can check one with `errors.Is(err, orders.NotFoundError{OrderID: id})`.

The split between expected and exceptional happens in the route. `writeExpectedOutcome` matches the expected types with `errors.AsType`, writes the 404 or 409 without logging, and returns every other error unchanged to the central handler. Go cannot check that the match is exhaustive the way `assert_never` does in Python. An outcome the route forgets to handle is still caught: it does not implement `AppError`, so the central handler logs it and returns a 500.

## The antipattern

[`app/antipattern_catch_everything_test.go`](app/antipattern_catch_everything_test.go) shows Go's version of catch-everything: `if err != nil { return orders.Order{}, false }`. The test shows that a missing order and a refused connection then look the same to the caller, and nothing is logged.
