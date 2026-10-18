import logging
from uuid import UUID, uuid4

import pytest
from fastapi.testclient import TestClient

from error_handling.conftest import VALID_ORDER
from error_handling.orders.models import CreateOrderRequest, OrderAlreadyProcessed, OrderNotFound
from error_handling.orders.repository import OrderRepository
from error_handling.orders.service import OrderService
from error_handling.orders.store import InMemoryOrderStore
from error_handling.result import Err, Ok


@pytest.fixture
def service() -> OrderService:
    return OrderService(OrderRepository(InMemoryOrderStore()))


async def test_get_unknown_order_returns_not_found_value(service: OrderService) -> None:
    order_id = uuid4()

    assert await service.get(order_id) == Err(OrderNotFound(order_id))


async def test_processing_twice_returns_already_processed_value(service: OrderService) -> None:
    order = await service.create(CreateOrderRequest.model_validate(VALID_ORDER))
    first = await service.process(order.id)

    second = await service.process(order.id)

    assert isinstance(first, Ok)
    assert second == Err(OrderAlreadyProcessed(order.id))


def create_order(client: TestClient) -> UUID:
    return UUID(client.post("/orders", json=VALID_ORDER).json()["id"])


def test_unknown_order_maps_to_404(client: TestClient) -> None:
    response = client.get(f"/orders/{uuid4()}")

    assert response.status_code == 404
    assert response.json()["code"] == "ORDER_NOT_FOUND"


def test_already_processed_order_maps_to_409(client: TestClient) -> None:
    order_id = create_order(client)
    client.post(f"/orders/{order_id}/process")

    response = client.post(f"/orders/{order_id}/process")

    assert response.status_code == 409
    assert response.json()["code"] == "ORDER_ALREADY_PROCESSED"


def test_processing_a_pending_order_succeeds(client: TestClient) -> None:
    order_id = create_order(client)

    response = client.post(f"/orders/{order_id}/process")

    assert response.status_code == 200
    assert response.json()["status"] == "processed"


def test_expected_outcomes_are_not_logged_as_errors(client: TestClient, caplog: pytest.LogCaptureFixture) -> None:
    order_id = create_order(client)
    client.post(f"/orders/{order_id}/process")

    client.post(f"/orders/{order_id}/process")
    client.get(f"/orders/{uuid4()}")

    assert [record for record in caplog.records if record.levelno >= logging.WARNING] == []
