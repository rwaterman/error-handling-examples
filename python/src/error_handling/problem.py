import logging
from collections.abc import Awaitable, Callable, Mapping
from http import HTTPStatus
from typing import Any
from uuid import uuid4

from fastapi import FastAPI, Request, Response
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException

from error_handling.errors import AppError

PROBLEM_JSON = "application/problem+json"
REQUEST_ID_HEADER = "x-request-id"
UNEXPECTED_ERROR_DETAIL = "An unexpected error occurred."

logger = logging.getLogger(__name__)


def request_id_of(request: Request) -> str:
    request_id: str = request.state.request_id
    return request_id


def problem(
    request: Request,
    status: int,
    code: str,
    detail: str,
    *,
    headers: Mapping[str, str] | None = None,
    **extensions: Any,
) -> JSONResponse:
    body = {
        "type": "about:blank",
        "title": HTTPStatus(status).phrase,
        "status": status,
        "detail": detail,
        "code": code,
        "instance": request.url.path,
        "requestId": request_id_of(request),
        **extensions,
    }
    return JSONResponse(body, status_code=status, headers=headers, media_type=PROBLEM_JSON)


async def assign_request_id(request: Request, call_next: Callable[[Request], Awaitable[Response]]) -> Response:
    request.state.request_id = request.headers.get(REQUEST_ID_HEADER) or str(uuid4())
    response = await call_next(request)
    response.headers[REQUEST_ID_HEADER] = request_id_of(request)
    return response


def log_failure(request: Request, message: str, exc: Exception, code: str) -> None:
    logger.error(
        message,
        exc_info=exc,
        extra={"request_id": request_id_of(request), "code": code, "path": request.url.path},
    )


def field_of(location: tuple[str | int, ...]) -> str:
    return ".".join(str(part) for part in location[1:]) or str(location[0])


async def handle_validation_error(request: Request, exc: Exception) -> JSONResponse:
    assert isinstance(exc, RequestValidationError)
    errors = [{"field": field_of(tuple(error["loc"])), "message": error["msg"]} for error in exc.errors()]
    return problem(request, 400, "VALIDATION_FAILED", "The request is invalid.", errors=errors)


async def handle_app_error(request: Request, exc: Exception) -> JSONResponse:
    assert isinstance(exc, AppError)
    if exc.status >= 500:
        log_failure(request, "request failed", exc, exc.code)
    return problem(request, exc.status, exc.code, exc.detail)


async def handle_http_exception(request: Request, exc: Exception) -> JSONResponse:
    assert isinstance(exc, StarletteHTTPException)
    code = "ROUTE_NOT_FOUND" if exc.status_code == 404 else HTTPStatus(exc.status_code).name
    return problem(request, exc.status_code, code, str(exc.detail), headers=exc.headers)


async def handle_unexpected_error(request: Request, exc: Exception) -> JSONResponse:
    log_failure(request, "unhandled error", exc, "INTERNAL_ERROR")
    return problem(request, 500, "INTERNAL_ERROR", UNEXPECTED_ERROR_DETAIL)


def register_error_handlers(app: FastAPI) -> None:
    app.middleware("http")(assign_request_id)
    app.add_exception_handler(RequestValidationError, handle_validation_error)
    app.add_exception_handler(AppError, handle_app_error)
    app.add_exception_handler(StarletteHTTPException, handle_http_exception)
    app.add_exception_handler(Exception, handle_unexpected_error)
