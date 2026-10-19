from typing import Any

import pytest
from fastapi.testclient import TestClient

from error_handling.app import create_app
from error_handling.orders.repository import OrderRepository
from error_handling.orders.service import OrderService
from error_handling.orders.store import InMemoryOrderStore, OrderStore

VALID_ORDER: dict[str, Any] = {
    "customerEmail": "ada@example.com",
    "items": [{"sku": "BOOK-1", "quantity": 2}],
}


class FailingOrderStore:
    def __init__(self, error: Exception) -> None:
        self.error = error

    async def get(self, key: str) -> dict[str, Any] | None:
        raise self.error

    async def put(self, key: str, value: dict[str, Any]) -> None:
        raise self.error


def make_client(store: OrderStore) -> TestClient:
    service = OrderService(OrderRepository(store))
    return TestClient(create_app(service), raise_server_exceptions=False)


@pytest.fixture
def store() -> InMemoryOrderStore:
    return InMemoryOrderStore()


@pytest.fixture
def client(store: InMemoryOrderStore) -> TestClient:
    return make_client(store)
