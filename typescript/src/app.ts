import { Hono } from 'hono';
import { type Logger } from 'pino';
import { buildOrderRoutes } from './orders/routes.ts';
import { type OrderService } from './orders/service.ts';
import { registerErrorHandlers, type AppEnv } from './problem.ts';

export type AppDependencies = {
  readonly orderService: OrderService;
  readonly logger: Logger;
};

export function createApp({ orderService, logger }: AppDependencies): Hono<AppEnv> {
  const app = new Hono<AppEnv>();
  registerErrorHandlers(app, logger);
  app.route('/orders', buildOrderRoutes(orderService));
  return app;
}
