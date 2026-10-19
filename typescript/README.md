# Error handling in TypeScript

This implementation serves the orders API described in the [shared contract](../docs/contract.md) with [Hono](https://hono.dev), [Zod](https://zod.dev) for validation, and [pino](https://getpino.io) for structured JSON logs. The [repository README](../README.md) explains the four patterns and why they exist.

- [Running it](#running-it)
- [How the four patterns look in TypeScript](#how-the-four-patterns-look-in-typescript)

## Running it

You need Node.js 24 or newer. Node runs the TypeScript sources directly through its built-in type stripping, so there is no build step.

```sh
npm ci
npm run verify   # format check, lint, typecheck, tests
npm run serve    # listens on port 8000, or on PORT when it is set
```

Try it with curl:

```sh
curl -s -X POST localhost:8000/orders \
  -H 'content-type: application/json' \
  -d '{"customerEmail":"ada@example.com","items":[{"sku":"BOOK-1","quantity":2}]}'
curl -s localhost:8000/orders/0b9e5a4e-8f43-4f7e-9d0a-7f6d1c2b3a4f
```

## How the four patterns look in TypeScript

**Validate at the edge.** [`src/validation.ts`](src/validation.ts) holds two small Hono middlewares that run each route's Zod schema from [`src/orders/models.ts`](src/orders/models.ts) before the handler. A successful `safeParse` hands the handler a value whose static type is the schema's output, so the service receives a `CreateOrderRequest` and never has to recheck it. The body middleware parses the JSON itself, because Hono's built-in JSON validator turns malformed JSON into a generic 400 exception and treats a request without a JSON content type as an empty object.

**A custom error hierarchy.** [`src/errors.ts`](src/errors.ts) defines `AppError`, an abstract subclass of `Error` whose subclasses declare their own `status` and `code`. The repository in [`src/orders/repository.ts`](src/orders/repository.ts) wraps only the store driver's connection and timeout failures, recognized by their Node error `code`, and passes the original through the standard ES2022 `cause` option. The client gets a safe message, and the log still shows the driver error that triggered it. Every other error leaves the repository untouched, so a bug is never reported as an outage.

**One central error handler.** [`src/problem.ts`](src/problem.ts) registers Hono's built-in `requestId()` middleware, `app.onError`, and `app.notFound`. Every error that a handler throws ends in `onError`, which is the only code that logs a failure and the only code that turns an error into an RFC 9457 problem body. Responses are built from the request context, so the `x-request-id` header set by the middleware is present on every response, including a 500.

**Result values for expected outcomes.** [`src/result.ts`](src/result.ts) defines `Result<T, E>` as a discriminated union on `ok`. The service in [`src/orders/service.ts`](src/orders/service.ts) returns tagged values such as `{ kind: 'OrderNotFound', orderId }`, and [`src/orders/routes.ts`](src/orders/routes.ts) maps them to 404 and 409 with a `switch` that ends in an `assertNever` check. Because the outcomes are part of the return type, the compiler rejects a new outcome that a route forgets to handle. Expected outcomes stay out of the logger entirely.

The tests sit next to the code and mirror the scenarios in the contract one file per pattern, plus [`src/antipattern_catch_everything.test.ts`](src/antipattern_catch_everything.test.ts), which shows how a catch-everything block makes an outage look like a missing order.
