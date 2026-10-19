from typing import Any

import pytest
from fastapi.testclient import TestClient

from error_handling.conftest import VALID_ORDER
from error_handling.orders.store import InMemoryOrderStore


def fields_of(body: dict[str, Any]) -> list[str]:
    return [error["field"] for error in body["errors"]]


@pytest.mark.parametrize(
    ("payload", "field"),
    [
        ({"items": VALID_ORDER["items"]}, "customerEmail"),
        ({**VALID_ORDER, "customerEmail": "not-an-email"}, "customerEmail"),
        ({**VALID_ORDER, "items": []}, "items"),
        ({**VALID_ORDER, "items": [{"sku": "BOOK-1", "quantity": 0}]}, "items.0.quantity"),
        ({**VALID_ORDER, "items": [{"sku": "", "quantity": 1}]}, "items.0.sku"),
    ],
)
def test_invalid_order_is_rejected_with_400_naming_the_field(
    client: TestClient, payload: dict[str, Any], field: str
) -> None:
    response = client.post("/orders", json=payload)

    assert response.status_code == 400
    assert response.headers["content-type"] == "application/problem+json"
    body = response.json()
    assert body["code"] == "VALIDATION_FAILED"
    assert field in fields_of(body)


def test_invalid_order_never_reaches_the_store(client: TestClient, store: InMemoryOrderStore) -> None:
    client.post("/orders", json={**VALID_ORDER, "customerEmail": "nope"})

    assert store.records == {}


def test_unknown_field_is_rejected(client: TestClient) -> None:
    response = client.post("/orders", json={**VALID_ORDER, "discountCode": "FREE"})

    assert response.status_code == 400
    assert response.json()["code"] == "VALIDATION_FAILED"


def test_malformed_json_is_a_validation_failure(client: TestClient) -> None:
    response = client.post("/orders", content=b"{not json", headers={"content-type": "application/json"})

    assert response.status_code == 400
    body = response.json()
    assert body["code"] == "VALIDATION_FAILED"
    assert fields_of(body) == ["body"]


def test_malformed_order_id_is_rejected_before_lookup(client: TestClient) -> None:
    response = client.get("/orders/not-a-uuid")

    assert response.status_code == 400
    assert fields_of(response.json()) == ["orderId"]


def test_valid_order_is_created(client: TestClient) -> None:
    response = client.post("/orders", json=VALID_ORDER)

    assert response.status_code == 201
    body = response.json()
    assert body["customerEmail"] == "ada@example.com"
    assert body["status"] == "pending"
    assert body["processedAt"] is None
