import { randomUUID } from 'node:crypto';
import { describe, expect, it } from 'vitest';
import { PROBLEM_JSON, UNEXPECTED_ERROR_DETAIL, type ProblemDetails } from './problem.ts';
import { createTestApp, driverError, ERROR_LEVEL, FailingOrderStore, SECRET } from './test-support.ts';

describe('pattern 3: central handler', () => {
  it('dependency outage returns 503 without leaking internals', async () => {
    const { app, logs } = createTestApp(new FailingOrderStore(driverError('ECONNREFUSED', SECRET)));
    const path = `/orders/${randomUUID()}`;

    const response = await app.request(path, { headers: { 'x-request-id': 'req-503' } });

    expect(response.status).toBe(503);
    expect(response.headers.get('content-type')).toBe(PROBLEM_JSON);
    expect(response.headers.get('x-request-id')).toBe('req-503');
    const text = await response.text();
    expect(text).not.toContain(SECRET);
    const body = JSON.parse(text) as ProblemDetails;
    expect(body.code).toBe('DEPENDENCY_UNAVAILABLE');
    expect(body.detail).toBe('A required dependency is unavailable: order-store.');
    expect(body.requestId).toBe('req-503');
    const errors = logs.atOrAbove(ERROR_LEVEL);
    expect(errors).toHaveLength(1);
    expect(errors[0]).toMatchObject({
      request_id: 'req-503',
      code: 'DEPENDENCY_UNAVAILABLE',
      path,
      err: { type: 'DependencyUnavailableError', cause: { code: 'ECONNREFUSED', message: SECRET } },
    });
  });

  it('unexpected error returns generic 500 and logs the real one', async () => {
    const { app, logs } = createTestApp(new FailingOrderStore(new Error(SECRET)));
    const path = `/orders/${randomUUID()}`;

    const response = await app.request(path, { headers: { 'x-request-id': 'req-500' } });

    expect(response.status).toBe(500);
    expect(response.headers.get('content-type')).toBe(PROBLEM_JSON);
    expect(response.headers.get('x-request-id')).toBe('req-500');
    const text = await response.text();
    expect(text).not.toContain(SECRET);
    const body = JSON.parse(text) as ProblemDetails;
    expect(body.code).toBe('INTERNAL_ERROR');
    expect(body.detail).toBe(UNEXPECTED_ERROR_DETAIL);
    expect(body.requestId).toBe('req-500');
    const errors = logs.atOrAbove(ERROR_LEVEL);
    expect(errors).toHaveLength(1);
    expect(errors[0]).toMatchObject({
      request_id: 'req-500',
      code: 'INTERNAL_ERROR',
      path,
      err: { type: 'Error', message: SECRET, stack: expect.stringContaining(SECRET) as unknown },
    });
  });

  it('request id is generated when the caller sends none', async () => {
    const { app } = createTestApp(new FailingOrderStore(new Error('boom')));

    const response = await app.request(`/orders/${randomUUID()}`);

    const body = (await response.json()) as ProblemDetails;
    expect(body.requestId).toBeTruthy();
    expect(response.headers.get('x-request-id')).toBe(body.requestId);
  });

  it('untrusted request id is replaced', async () => {
    const { app } = createTestApp(new FailingOrderStore(new Error('boom')));

    const response = await app.request(`/orders/${randomUUID()}`, { headers: { 'x-request-id': 'has space' } });

    const body = (await response.json()) as ProblemDetails;
    expect(body.requestId).not.toBe('has space');
    expect(body.requestId).toMatch(/^[0-9a-f-]{36}$/);
    expect(response.headers.get('x-request-id')).toBe(body.requestId);
  });

  it('unknown route uses the same problem format', async () => {
    const { app } = createTestApp(new FailingOrderStore(new Error('unused')));

    const response = await app.request('/nope');

    expect(response.status).toBe(404);
    expect(response.headers.get('content-type')).toBe(PROBLEM_JSON);
    const body = (await response.json()) as ProblemDetails;
    expect(body.code).toBe('ROUTE_NOT_FOUND');
    expect(response.headers.get('x-request-id')).toBe(body.requestId);
  });
});
