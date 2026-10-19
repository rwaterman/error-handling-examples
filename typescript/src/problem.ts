import { STATUS_CODES } from 'node:http';
import { type Context, type Hono } from 'hono';
import { requestId, type RequestIdVariables } from 'hono/request-id';
import { type ContentfulStatusCode } from 'hono/utils/http-status';
import { type Logger } from 'pino';
import { AppError } from './errors.ts';

export type AppEnv = { Variables: RequestIdVariables };

export type FieldError = { readonly field: string; readonly message: string };

export type ProblemDetails = {
  readonly type: 'about:blank';
  readonly title: string;
  readonly status: number;
  readonly detail: string;
  readonly code: string;
  readonly instance: string;
  readonly requestId: string;
  readonly errors?: readonly FieldError[];
};

export const PROBLEM_JSON = 'application/problem+json';
export const UNEXPECTED_ERROR_DETAIL = 'An unexpected error occurred.';

export function problem(
  c: Context<AppEnv>,
  status: ContentfulStatusCode,
  code: string,
  detail: string,
  extensions: Pick<ProblemDetails, 'errors'> = {},
): Response {
  const body: ProblemDetails = {
    type: 'about:blank',
    title: STATUS_CODES[status] ?? String(status),
    status,
    detail,
    code,
    instance: c.req.path,
    requestId: c.get('requestId'),
    ...extensions,
  };
  return c.body(JSON.stringify(body), status, { 'Content-Type': PROBLEM_JSON });
}

function logFailure(logger: Logger, c: Context<AppEnv>, message: string, error: Error, code: string): void {
  logger.error({ err: error, request_id: c.get('requestId'), code, path: c.req.path }, message);
}

export function registerErrorHandlers(app: Hono<AppEnv>, logger: Logger): void {
  app.use(requestId());

  app.onError((error, c) => {
    if (error instanceof AppError) {
      if (error.status >= 500) {
        logFailure(logger, c, 'request failed', error, error.code);
      }
      return problem(c, error.status, error.code, error.detail);
    }
    logFailure(logger, c, 'unhandled error', error, 'INTERNAL_ERROR');
    return problem(c, 500, 'INTERNAL_ERROR', UNEXPECTED_ERROR_DETAIL);
  });

  app.notFound((c) => problem(c, 404, 'ROUTE_NOT_FOUND', 'Not Found'));
}
