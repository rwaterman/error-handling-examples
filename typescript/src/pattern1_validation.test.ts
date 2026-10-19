import { describe, expect, it } from 'vitest';
import { InMemoryOrderStore } from './orders/store.ts';
import { PROBLEM_JSON, type ProblemDetails } from './problem.ts';
import { createTestApp, orderOf, postJson, problemOf, VALID_ORDER } from './test-support.ts';

function fieldsOf(body: ProblemDetails): string[] {
  return (body.errors ?? []).map((error) => error.field);
}

describe('pattern 1: validation', () => {
  it.each([
    { name: 'missing customerEmail', payload: { items: VALID_ORDER.items }, field: 'customerEmail' },
    {
      name: 'malformed customerEmail',
      payload: { ...VALID_ORDER, customerEmail: 'not-an-email' },
      field: 'customerEmail',
    },
    { name: 'empty items', payload: { ...VALID_ORDER, items: [] }, field: 'items' },
    {
      name: 'zero quantity',
      payload: { ...VALID_ORDER, items: [{ sku: 'BOOK-1', quantity: 0 }] },
      field: 'items.0.quantity',
    },
    { name: 'empty sku', payload: { ...VALID_ORDER, items: [{ sku: '', quantity: 1 }] }, field: 'items.0.sku' },
  ])('invalid order is rejected with 400 naming the field ($name)', async ({ payload, field }) => {
    const { app } = createTestApp(new InMemoryOrderStore());

    const response = await postJson(app, '/orders', payload);

    expect(response.status).toBe(400);
    expect(response.headers.get('content-type')).toBe(PROBLEM_JSON);
    const body = await problemOf(response);
    expect(body.code).toBe('VALIDATION_FAILED');
    expect(fieldsOf(body)).toContain(field);
  });

  it('invalid order never reaches the store', async () => {
    const store = new InMemoryOrderStore();
    const { app } = createTestApp(store);

    await postJson(app, '/orders', { ...VALID_ORDER, customerEmail: 'nope' });

    expect(store.records.size).toBe(0);
  });

  it('unknown field is rejected', async () => {
    const { app } = createTestApp(new InMemoryOrderStore());

    const response = await postJson(app, '/orders', { ...VALID_ORDER, discountCode: 'FREE' });

    expect(response.status).toBe(400);
    const body = await problemOf(response);
    expect(body.code).toBe('VALIDATION_FAILED');
    expect(fieldsOf(body)).toEqual(['discountCode']);
  });

  it('malformed JSON is a validation failure', async () => {
    const { app } = createTestApp(new InMemoryOrderStore());

    const response = await app.request('/orders', {
      method: 'POST',
      body: '{not json',
      headers: { 'content-type': 'application/json' },
    });

    expect(response.status).toBe(400);
    const body = await problemOf(response);
    expect(body.code).toBe('VALIDATION_FAILED');
    expect(fieldsOf(body)).toEqual(['body']);
  });

  it('malformed order id is rejected before lookup', async () => {
    const { app } = createTestApp(new InMemoryOrderStore());

    const response = await app.request('/orders/not-a-uuid');

    expect(response.status).toBe(400);
    expect(fieldsOf(await problemOf(response))).toEqual(['orderId']);
  });

  it('valid order is created', async () => {
    const { app } = createTestApp(new InMemoryOrderStore());

    const response = await postJson(app, '/orders', VALID_ORDER);

    expect(response.status).toBe(201);
    const order = await orderOf(response);
    expect(order.customerEmail).toBe('ada@example.com');
    expect(order.status).toBe('pending');
    expect(order.processedAt).toBeNull();
  });
});
