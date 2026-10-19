import asyncio
import json
import logging
from unittest.mock import Mock
from uuid import uuid4

import pytest
from uvicorn import Config
from uvicorn.protocols.http.h11_impl import H11Protocol
from uvicorn.server import ServerState

from error_handling.app import create_app
from error_handling.conftest import FailingOrderStore, make_client
from error_handling.orders.repository import OrderRepository
from error_handling.orders.service import OrderService

SECRET = "postgres://admin:hunter2@db-01:5432/orders"


def test_dependency_outage_returns_503_without_leaking_internals(caplog: pytest.LogCaptureFixture) -> None:
    client = make_client(FailingOrderStore(ConnectionRefusedError(SECRET)))

    response = client.get(f"/orders/{uuid4()}", headers={"x-request-id": "req-503"})

    assert response.status_code == 503
    assert response.headers["content-type"] == "application/problem+json"
    body = response.json()
    assert body["code"] == "DEPENDENCY_UNAVAILABLE"
    assert body["requestId"] == "req-503"
    assert SECRET not in response.text
    [record] = [record for record in caplog.records if record.levelno >= logging.ERROR]
    assert record.__dict__["request_id"] == "req-503"
    assert record.exc_info is not None


def test_unexpected_error_returns_generic_500_and_logs_the_real_one(caplog: pytest.LogCaptureFixture) -> None:
    client = make_client(FailingOrderStore(RuntimeError(SECRET)))

    response = client.get(f"/orders/{uuid4()}", headers={"x-request-id": "req-500"})

    assert response.status_code == 500
    body = response.json()
    assert body["code"] == "INTERNAL_ERROR"
    assert body["detail"] == "An unexpected error occurred."
    assert body["requestId"] == "req-500"
    assert SECRET not in response.text
    [record] = [record for record in caplog.records if record.levelno >= logging.ERROR]
    assert record.__dict__["request_id"] == "req-500"
    assert record.exc_info is not None
    assert SECRET in str(record.exc_info[1])


@pytest.mark.parametrize(
    ("error", "status", "code"),
    [
        (RuntimeError(SECRET), 500, "INTERNAL_ERROR"),
        (ConnectionRefusedError(SECRET), 503, "DEPENDENCY_UNAVAILABLE"),
    ],
)
async def test_server_failure_is_logged_once_through_uvicorn(
    caplog: pytest.LogCaptureFixture, error: Exception, status: int, code: str
) -> None:
    app = create_app(OrderService(OrderRepository(FailingOrderStore(error))))
    config = Config(app, log_config=None, access_log=False, lifespan="off")
    server_state = ServerState()
    protocol = H11Protocol(config, server_state, {})
    transport = Mock(spec=asyncio.Transport)
    transport.get_extra_info.side_effect = {"sockname": ("127.0.0.1", 8000), "peername": ("127.0.0.1", 1234)}.get
    transport.is_closing.return_value = False
    path = f"/orders/{uuid4()}"
    request = f"GET {path} HTTP/1.1\r\nHost: localhost\r\nx-request-id: req-server\r\n\r\n"

    protocol.connection_made(transport)
    try:
        protocol.data_received(request.encode())
        await asyncio.wait_for(asyncio.gather(*server_state.tasks), timeout=5)
    finally:
        protocol.connection_lost(None)

    response = b"".join(call.args[0] for call in transport.write.call_args_list)
    headers, body = response.split(b"\r\n\r\n", 1)
    assert headers.startswith(f"HTTP/1.1 {status} ".encode())
    assert b"content-type: application/problem+json" in headers.lower()
    assert b"x-request-id: req-server" in headers.lower()
    assert SECRET.encode() not in body
    assert json.loads(body)["requestId"] == "req-server"
    assert json.loads(body)["code"] == code
    [record] = [record for record in caplog.records if record.levelno >= logging.ERROR]
    assert record.__dict__["request_id"] == "req-server"
    assert record.__dict__["code"] == code
    assert record.__dict__["path"] == path
    assert record.exc_info is not None
    assert SECRET in logging.Formatter().formatException(record.exc_info)


def test_request_id_is_generated_when_the_caller_sends_none() -> None:
    client = make_client(FailingOrderStore(RuntimeError("boom")))

    response = client.get(f"/orders/{uuid4()}")

    request_id = response.json()["requestId"]
    assert request_id
    assert response.headers["x-request-id"] == request_id


def test_untrusted_request_id_is_replaced() -> None:
    client = make_client(FailingOrderStore(RuntimeError("boom")))

    response = client.get(f"/orders/{uuid4()}", headers={"x-request-id": "has space"})

    request_id = response.json()["requestId"]
    assert request_id != "has space"
    assert response.headers["x-request-id"] == request_id


def test_unknown_route_uses_the_same_problem_format() -> None:
    client = make_client(FailingOrderStore(RuntimeError("unused")))

    response = client.get("/nope")

    assert response.status_code == 404
    assert response.headers["content-type"] == "application/problem+json"
    assert response.json()["code"] == "ROUTE_NOT_FOUND"
