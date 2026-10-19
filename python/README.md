# Error handling in Python

A FastAPI service that implements the [shared contract](../docs/contract.md). It is the reference implementation for the other languages.

- [Run it](#run-it)
- [How the four patterns look in Python](#how-the-four-patterns-look-in-python)
- [The antipattern](#the-antipattern)

## Run it

Requires [uv](https://docs.astral.sh/uv/). uv installs the pinned Python version itself.

```sh
uv sync
uv run poe verify   # format check, lint, strict mypy, tests
uv run poe serve    # http://127.0.0.1:8000
```

```sh
curl -s -X POST localhost:8000/orders \
  -H 'content-type: application/json' \
  -d '{"customerEmail":"ada@example.com","items":[{"sku":"BOOK-1","quantity":2}]}'
```

## How the four patterns look in Python

### 1. Validate at the edge

Files: [`orders/models.py`](src/error_handling/orders/models.py), [`orders/routes.py`](src/error_handling/orders/routes.py).

Pydantic models declare the rules (`EmailStr`, `Field(ge=1, le=100)`, `extra="forbid"`). FastAPI parses each request body and path parameter into those models before the route function runs. The route receives a typed, already-valid `CreateOrderRequest`, so nothing downstream checks for bad input or catches errors caused by it.

### 2. A small error hierarchy

File: [`errors.py`](src/error_handling/errors.py).

`AppError` carries an HTTP status and a stable code as class attributes. `DependencyUnavailableError` subclasses it. The repository raises it with `raise ... from err`, so the original driver error stays on `__cause__` for the logs, and the client sees only the safe `detail`. The repository catches only `ConnectionError` and `TimeoutError`. Anything else is a bug and passes through untouched.

### 3. One central handler

File: [`problem.py`](src/error_handling/problem.py).

`register_error_handlers` installs one handler per error family: validation errors, `AppError`, framework HTTP errors, and a final `Exception` catch-all. Every one of them renders the same RFC 9457 problem body with the request ID. Only 5xx responses are logged, with the full traceback. Routes contain no `try` blocks at all.

### 4. Result values for expected outcomes

Files: [`result.py`](src/error_handling/result.py), [`orders/service.py`](src/error_handling/orders/service.py).

`Ok[T] | Err[E]` is a two-class union. `OrderService.get` and `OrderService.process` return `Err(OrderNotFound(...))` or `Err(OrderAlreadyProcessed(...))` instead of raising. The route handles both with `match` and `assert_never`, so mypy fails the build if a new outcome is added and left unhandled. These outcomes are never logged, because nothing went wrong.

## The antipattern

[`test_antipattern_catch_everything.py`](src/error_handling/test_antipattern_catch_everything.py) wraps `repository.find` in `except Exception: return None`. The test shows that a missing order and a database outage then return the same `None`, and nothing is logged. That is the failure mode the article opens with.
