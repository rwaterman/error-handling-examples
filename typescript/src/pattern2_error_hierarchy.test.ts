import { randomUUID } from 'node:crypto';
import { describe, expect, it } from 'vitest';
import { AppError, DependencyUnavailableError } from './errors.ts';
import { OrderRepository } from './orders/repository.ts';
import { driverError, FailingOrderStore } from './test-support.ts';

async function rejectionOf(operation: Promise<unknown>): Promise<unknown> {
  try {
    await operation;
  } catch (error) {
    return error;
  }
  throw new Error('Expected the operation to reject.');
}

describe('pattern 2: error hierarchy', () => {
  it('dependency unavailable carries its status and code', () => {
    const error = new DependencyUnavailableError('order-store');

    expect(error).toBeInstanceOf(AppError);
    expect(error).toBeInstanceOf(Error);
    expect(error.name).toBe('DependencyUnavailableError');
    expect(error.status).toBe(503);
    expect(error.code).toBe('DEPENDENCY_UNAVAILABLE');
    expect(error.dependency).toBe('order-store');
  });

  it.each([
    { name: 'connection refused', driverFailure: driverError('ECONNREFUSED', 'connect ECONNREFUSED db-01:5432') },
    { name: 'timeout', driverFailure: driverError('ETIMEDOUT', 'read ETIMEDOUT') },
  ])('repository translates driver errors and keeps the cause ($name)', async ({ driverFailure }) => {
    const repository = new OrderRepository(new FailingOrderStore(driverFailure));

    const raised = await rejectionOf(repository.find(randomUUID()));

    expect(raised).toBeInstanceOf(DependencyUnavailableError);
    expect((raised as DependencyUnavailableError).cause).toBe(driverFailure);
  });

  it('repository lets unexpected errors through untouched', async () => {
    const bug = new TypeError("Cannot read properties of undefined (reading 'customerEmail')");
    const repository = new OrderRepository(new FailingOrderStore(bug));

    await expect(repository.find(randomUUID())).rejects.toBe(bug);
  });
});
