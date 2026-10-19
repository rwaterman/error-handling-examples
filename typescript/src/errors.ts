import { type ContentfulStatusCode } from 'hono/utils/http-status';

export abstract class AppError extends Error {
  abstract readonly status: ContentfulStatusCode;
  abstract readonly code: string;
  readonly detail: string;

  constructor(detail: string, options?: ErrorOptions) {
    super(detail, options);
    this.name = new.target.name;
    this.detail = detail;
  }
}

export class DependencyUnavailableError extends AppError {
  readonly status = 503;
  readonly code = 'DEPENDENCY_UNAVAILABLE';
  readonly dependency: string;

  constructor(dependency: string, options?: ErrorOptions) {
    super(`A required dependency is unavailable: ${dependency}.`, options);
    this.dependency = dependency;
  }
}
