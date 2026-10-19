import { randomUUID } from 'node:crypto';
import { err, ok, type Result } from '../result.ts';
import {
  orderAlreadyProcessed,
  orderNotFound,
  type CreateOrderRequest,
  type Order,
  type OrderNotFound,
  type ProcessOrderError,
} from './models.ts';
import { type OrderRepository } from './repository.ts';

export class OrderService {
  readonly #repository: OrderRepository;

  constructor(repository: OrderRepository) {
    this.#repository = repository;
  }

  async create(request: CreateOrderRequest): Promise<Order> {
    const order: Order = {
      id: randomUUID(),
      customerEmail: request.customerEmail,
      items: request.items,
      status: 'pending',
      createdAt: new Date().toISOString(),
      processedAt: null,
    };
    await this.#repository.save(order);
    return order;
  }

  async get(orderId: string): Promise<Result<Order, OrderNotFound>> {
    const order = await this.#repository.find(orderId);
    if (order === undefined) {
      return err(orderNotFound(orderId));
    }
    return ok(order);
  }

  async process(orderId: string): Promise<Result<Order, ProcessOrderError>> {
    const order = await this.#repository.find(orderId);
    if (order === undefined) {
      return err(orderNotFound(orderId));
    }
    if (order.status === 'processed') {
      return err(orderAlreadyProcessed(orderId));
    }
    const processed: Order = { ...order, status: 'processed', processedAt: new Date().toISOString() };
    await this.#repository.save(processed);
    return ok(processed);
  }
}
