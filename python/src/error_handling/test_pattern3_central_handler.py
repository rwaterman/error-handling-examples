import logging
from uuid import uuid4

import pytest

from error_handling.conftest import FailingOrderStore, make_client

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
