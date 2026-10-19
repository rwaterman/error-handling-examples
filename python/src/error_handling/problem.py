import logging
import re
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
REQUEST_ID_PATTERN = re.compile(r"[A-Za-z0-9_=-]{1,255}")
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
    response_headers = {**(headers or {}), REQUEST_ID_HEADER: request_id_of(request)}
    return JSONResponse(body, status_code=status, headers=response_headers, media_type=PROBLEM_JSON)


def trusted_request_id(candidate: str | None) -> str:
    if candidate is not None and REQUEST_ID_PATTERN.fullmatch(candidate):
        return candidate
    return str(uuid4())


async def assign_request_id(request: Request, call_next: Callable[[Request], Awaitable[Response]]) -> Response:
    request.state.request_id = trusted_request_id(request.headers.get(REQUEST_ID_HEADER))
    response = await call_next(request)
    response.headers[REQUEST_ID_HEADER] = request_id_of(request)
    return response


def log_failure(request: Request, message: str, exc: Exception, code: str) -> None:
    logger.error(
        message,
        exc_info=exc,
        extra={"request_id": request_id_of(request), "code": code, "path": request.url.path},
    )


def field_of(error: Mapping[str, Any]) -> str:
    location: tuple[str | int, ...] = tuple(error["loc"])
    if error["type"] == "json_invalid":
        return "body"
    return ".".join(str(part) for part in location[1:]) or str(location[0])


async def handle_validation_error(request: Request, exc: Exception) -> JSONResponse:
    assert isinstance(exc, RequestValidationError)
    errors = [{"field": field_of(error), "message": error["msg"]} for error in exc.errors()]
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
