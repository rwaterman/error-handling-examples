import { DependencyUnavailableError } from '../errors.ts';
import { orderSchema, type Order } from './models.ts';
import { type OrderStore } from './store.ts';

const DRIVER_ERROR_CODES: ReadonlySet<string> = new Set(['ECONNREFUSED', 'ECONNRESET', 'ETIMEDOUT']);

function isDriverError(error: unknown): boolean {
  return (
    error instanceof Error && 'code' in error && typeof error.code === 'string' && DRIVER_ERROR_CODES.has(error.code)
  );
}

async function translateDriverErrors<T>(operation: () => Promise<T>): Promise<T> {
  try {
    return await operation();
  } catch (error) {
    if (isDriverError(error)) {
      throw new DependencyUnavailableError('order-store', { cause: error });
    }
    throw error;
  }
}

export class OrderRepository {
  readonly #store: OrderStore;

  constructor(store: OrderStore) {
    this.#store = store;
  }

  async find(orderId: string): Promise<Order | undefined> {
    const record = await translateDriverErrors(() => this.#store.get(orderId));
    return record === undefined ? undefined : orderSchema.parse(record);
  }

  async save(order: Order): Promise<void> {
    await translateDriverErrors(() => this.#store.put(order.id, order));
  }
}
