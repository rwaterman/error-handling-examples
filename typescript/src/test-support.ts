import { type Hono } from 'hono';
import { type DestinationStream } from 'pino';
import { createApp } from './app.ts';
import { createLogger } from './logging.ts';
import { type CreateOrderRequest, type Order } from './orders/models.ts';
import { OrderRepository } from './orders/repository.ts';
import { OrderService } from './orders/service.ts';
import { type OrderStore } from './orders/store.ts';
import { type AppEnv, type ProblemDetails } from './problem.ts';

export const SECRET = 'postgres://admin:hunter2@db-01:5432/orders';

export const VALID_ORDER: CreateOrderRequest = {
  customerEmail: 'ada@example.com',
  items: [{ sku: 'BOOK-1', quantity: 2 }],
};

export const WARN_LEVEL = 40;
export const ERROR_LEVEL = 50;

export type LogRecord = { readonly level: number; readonly msg: string; readonly [key: string]: unknown };

export class LogCapture implements DestinationStream {
  readonly #lines: string[] = [];

  write(line: string): void {
    this.#lines.push(line);
  }

  records(): LogRecord[] {
    return this.#lines.map((line) => JSON.parse(line) as LogRecord);
  }

  atOrAbove(level: number): LogRecord[] {
    return this.records().filter((record) => record.level >= level);
  }
}

export class FailingOrderStore implements OrderStore {
  readonly #error: Error;

  constructor(error: Error) {
    this.#error = error;
  }

  get(): Promise<never> {
    return Promise.reject(this.#error);
  }

  put(): Promise<never> {
    return Promise.reject(this.#error);
  }
}

export function driverError(code: string, message: string): Error {
  return Object.assign(new Error(message), { code });
}

export type TestApp = { readonly app: Hono<AppEnv>; readonly logs: LogCapture };

export function createTestApp(store: OrderStore): TestApp {
  const logs = new LogCapture();
  const orderService = new OrderService(new OrderRepository(store));
  return { app: createApp({ orderService, logger: createLogger(logs) }), logs };
}

export async function postJson(app: Hono<AppEnv>, path: string, payload: unknown): Promise<Response> {
  return app.request(path, {
    method: 'POST',
    body: JSON.stringify(payload),
    headers: { 'content-type': 'application/json' },
  });
}

export async function problemOf(response: Response): Promise<ProblemDetails> {
  return (await response.json()) as ProblemDetails;
}

export async function orderOf(response: Response): Promise<Order> {
  return (await response.json()) as Order;
}

export async function createOrder(app: Hono<AppEnv>): Promise<string> {
  const order = await orderOf(await postJson(app, '/orders', VALID_ORDER));
  return order.id;
}
