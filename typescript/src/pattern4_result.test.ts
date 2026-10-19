import { randomUUID } from 'node:crypto';
import { describe, expect, it } from 'vitest';
import { orderAlreadyProcessed, orderNotFound } from './orders/models.ts';
import { OrderRepository } from './orders/repository.ts';
import { OrderService } from './orders/service.ts';
import { InMemoryOrderStore } from './orders/store.ts';
import { err } from './result.ts';
import { createOrder, createTestApp, orderOf, problemOf, VALID_ORDER, WARN_LEVEL } from './test-support.ts';

function createService(): OrderService {
  return new OrderService(new OrderRepository(new InMemoryOrderStore()));
}

describe('pattern 4: result values', () => {
  it('get unknown order returns not-found value', async () => {
    const orderId = randomUUID();

    expect(await createService().get(orderId)).toEqual(err(orderNotFound(orderId)));
  });

  it('processing twice returns already-processed value', async () => {
    const service = createService();
    const order = await service.create(VALID_ORDER);
    const first = await service.process(order.id);

    const second = await service.process(order.id);

    expect(first.ok).toBe(true);
    expect(second).toEqual(err(orderAlreadyProcessed(order.id)));
  });

  it('unknown order maps to 404', async () => {
    const { app } = createTestApp(new InMemoryOrderStore());
    const orderId = randomUUID();

    const response = await app.request(`/orders/${orderId}`);

    expect(response.status).toBe(404);
    const body = await problemOf(response);
    expect(body.code).toBe('ORDER_NOT_FOUND');
    expect(body.detail).toBe(`Order ${orderId} does not exist.`);
  });

  it('already processed order maps to 409', async () => {
    const { app } = createTestApp(new InMemoryOrderStore());
    const orderId = await createOrder(app);
    await app.request(`/orders/${orderId}/process`, { method: 'POST' });

    const response = await app.request(`/orders/${orderId}/process`, { method: 'POST' });

    expect(response.status).toBe(409);
    const body = await problemOf(response);
    expect(body.code).toBe('ORDER_ALREADY_PROCESSED');
    expect(body.detail).toBe(`Order ${orderId} was already processed.`);
  });

  it('processing a pending order succeeds', async () => {
    const { app } = createTestApp(new InMemoryOrderStore());
    const orderId = await createOrder(app);

    const response = await app.request(`/orders/${orderId}/process`, { method: 'POST' });

    expect(response.status).toBe(200);
    const order = await orderOf(response);
    expect(order.status).toBe('processed');
    expect(order.processedAt).toMatch(/Z$/);
  });

  it('expected outcomes are not logged as errors', async () => {
    const { app, logs } = createTestApp(new InMemoryOrderStore());
    const orderId = await createOrder(app);
    await app.request(`/orders/${orderId}/process`, { method: 'POST' });

    const conflict = await app.request(`/orders/${orderId}/process`, { method: 'POST' });
    const missing = await app.request(`/orders/${randomUUID()}`);

    expect([conflict.status, missing.status]).toEqual([409, 404]);
    expect(logs.atOrAbove(WARN_LEVEL)).toEqual([]);
  });
});
