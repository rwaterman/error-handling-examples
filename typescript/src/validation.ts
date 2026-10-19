import { type Context, type MiddlewareHandler, type Next } from 'hono';
import { type z } from 'zod';
import { problem, type AppEnv, type FieldError } from './problem.ts';
import { err, ok, type Result } from './result.ts';

type ValidationTarget = 'json' | 'param';

export function validationProblem(c: Context<AppEnv>, errors: readonly FieldError[]): Response {
  return problem(c, 400, 'VALIDATION_FAILED', 'The request is invalid.', { errors });
}

export function validJson<Schema extends z.ZodType<object>>(
  schema: Schema,
): MiddlewareHandler<AppEnv, string, { out: { json: z.output<Schema> } }> {
  return async (c, next) => {
    const body = await readJson(c);
    if (!body.ok) {
      return validationProblem(c, [body.error]);
    }
    return validateInto(c, next, 'json', schema, body.value);
  };
}

export function validParams<Schema extends z.ZodType<object>>(
  schema: Schema,
): MiddlewareHandler<AppEnv, string, { out: { param: z.output<Schema> } }> {
  return async (c, next) => validateInto(c, next, 'param', schema, c.req.param());
}

async function readJson(c: Context<AppEnv>): Promise<Result<unknown, FieldError>> {
  const text = await c.req.text();
  try {
    const parsed: unknown = JSON.parse(text);
    return ok(parsed);
  } catch (error) {
    if (!(error instanceof SyntaxError)) {
      throw error;
    }
    return err({ field: 'body', message: error.message });
  }
}

async function validateInto(
  c: Context<AppEnv>,
  next: Next,
  target: ValidationTarget,
  schema: z.ZodType<object>,
  value: unknown,
): Promise<Response | undefined> {
  const parsed = schema.safeParse(value);
  if (!parsed.success) {
    return validationProblem(c, fieldErrorsOf(parsed.error.issues));
  }
  c.req.addValidatedData(target, parsed.data);
  await next();
  return undefined;
}

function fieldErrorsOf(issues: readonly z.core.$ZodIssue[]): FieldError[] {
  return issues.flatMap((issue) =>
    issue.code === 'unrecognized_keys'
      ? issue.keys.map((key) => ({ field: fieldOf([...issue.path, key]), message: issue.message }))
      : [{ field: fieldOf(issue.path), message: issue.message }],
  );
}

function fieldOf(path: readonly PropertyKey[]): string {
  return path.length === 0 ? 'body' : path.map(String).join('.');
}
