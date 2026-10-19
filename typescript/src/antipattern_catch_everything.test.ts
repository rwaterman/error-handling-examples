import { randomUUID } from 'node:crypto';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { DependencyUnavailableError } from './errors.ts';
import { type Order } from './orders/models.ts';
import { OrderRepository } from './orders/repository.ts';
import { InMemoryOrderStore } from './orders/store.ts';
import { driverError, FailingOrderStore } from './test-support.ts';

async function findOrderSwallowingErrors(repository: OrderRepository, orderId: string): Promise<Order | undefined> {
  try {
    return await repository.find(orderId);
  } catch {
    return undefined;
  }
}

function refusingRepository(): OrderRepository {
  return new OrderRepository(new FailingOrderStore(driverError('ECONNREFUSED', 'connect ECONNREFUSED db-01:5432')));
}

describe('antipattern: catch everything', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('swallowing makes an outage indistinguishable from a missing order', async () => {
    const consoleCalls = ['error', 'warn', 'info', 'log'] as const;
    const spies = consoleCalls.map((method) => vi.spyOn(console, method));
    const healthy = new OrderRepository(new InMemoryOrderStore());

    const missing = await findOrderSwallowingErrors(healthy, randomUUID());
    const outage = await findOrderSwallowingErrors(refusingRepository(), randomUUID());

    expect(missing).toBeUndefined();
    expect(outage).toBeUndefined();
    for (const spy of spies) {
      expect(spy).not.toHaveBeenCalled();
    }
  });

  it('letting it propagate keeps the outage visible', async () => {
    await expect(refusingRepository().find(randomUUID())).rejects.toBeInstanceOf(DependencyUnavailableError);
  });
});
