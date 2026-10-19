import { Hono, type Context } from 'hono';
import { problem, type AppEnv } from '../problem.ts';
import { assertNever } from '../result.ts';
import { validJson, validParams } from '../validation.ts';
import { createOrderRequestSchema, orderIdParamsSchema, type ProcessOrderError } from './models.ts';
import { type OrderService } from './service.ts';

function orderProblem(c: Context<AppEnv>, error: ProcessOrderError): Response {
  switch (error.kind) {
    case 'OrderNotFound':
      return problem(c, 404, 'ORDER_NOT_FOUND', `Order ${error.orderId} does not exist.`);
    case 'OrderAlreadyProcessed':
      return problem(c, 409, 'ORDER_ALREADY_PROCESSED', `Order ${error.orderId} was already processed.`);
    default:
      return assertNever(error);
  }
}

export function buildOrderRoutes(service: OrderService): Hono<AppEnv> {
  return new Hono<AppEnv>()
    .post('/', validJson(createOrderRequestSchema), async (c) => {
      const order = await service.create(c.req.valid('json'));
      return c.json(order, 201);
    })
    .get('/:orderId', validParams(orderIdParamsSchema), async (c) => {
      const result = await service.get(c.req.valid('param').orderId);
      return result.ok ? c.json(result.value) : orderProblem(c, result.error);
    })
    .post('/:orderId/process', validParams(orderIdParamsSchema), async (c) => {
      const result = await service.process(c.req.valid('param').orderId);
      return result.ok ? c.json(result.value) : orderProblem(c, result.error);
    });
}
