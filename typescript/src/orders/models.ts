import { z } from 'zod';

export const orderItemSchema = z.strictObject({
  sku: z.string().min(1).max(64),
  quantity: z.int().min(1).max(100),
});

export const createOrderRequestSchema = z.strictObject({
  customerEmail: z.email(),
  items: z.array(orderItemSchema).min(1).max(50),
});

export const orderIdParamsSchema = z.object({
  orderId: z.uuid().toLowerCase(),
});

export const orderSchema = z.object({
  id: z.uuid(),
  customerEmail: z.string(),
  items: z.array(orderItemSchema),
  status: z.enum(['pending', 'processed']),
  createdAt: z.iso.datetime(),
  processedAt: z.iso.datetime().nullable(),
});

export type OrderItem = z.infer<typeof orderItemSchema>;
export type CreateOrderRequest = z.infer<typeof createOrderRequestSchema>;
export type Order = z.infer<typeof orderSchema>;

export type OrderNotFound = { readonly kind: 'OrderNotFound'; readonly orderId: string };
export type OrderAlreadyProcessed = { readonly kind: 'OrderAlreadyProcessed'; readonly orderId: string };
export type ProcessOrderError = OrderNotFound | OrderAlreadyProcessed;

export function orderNotFound(orderId: string): OrderNotFound {
  return { kind: 'OrderNotFound', orderId };
}

export function orderAlreadyProcessed(orderId: string): OrderAlreadyProcessed {
  return { kind: 'OrderAlreadyProcessed', orderId };
}
