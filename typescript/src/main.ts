import { serve } from '@hono/node-server';
import { pino } from 'pino';
import { createApp } from './app.ts';
import { createLogger } from './logging.ts';
import { OrderRepository } from './orders/repository.ts';
import { OrderService } from './orders/service.ts';
import { InMemoryOrderStore } from './orders/store.ts';

const logger = createLogger(pino.destination(1));
const orderService = new OrderService(new OrderRepository(new InMemoryOrderStore()));
const app = createApp({ orderService, logger });
const port = Number(process.env.PORT ?? '8000');

serve({ fetch: app.fetch, port }, (info) => {
  logger.info({ port: info.port }, 'listening');
});
