from uuid import UUID, uuid4

import pytest

from error_handling.conftest import FailingOrderStore
from error_handling.errors import DependencyUnavailableError
from error_handling.orders.models import Order
from error_handling.orders.repository import OrderRepository
from error_handling.orders.store import InMemoryOrderStore


async def find_order_swallowing_errors(repository: OrderRepository, order_id: UUID) -> Order | None:
    try:
        return await repository.find(order_id)
    except Exception:
        return None


async def test_swallowing_makes_an_outage_indistinguishable_from_a_missing_order(
    caplog: pytest.LogCaptureFixture,
) -> None:
    healthy = OrderRepository(InMemoryOrderStore())
    down = OrderRepository(FailingOrderStore(ConnectionRefusedError("db-01:5432 refused")))

    missing = await find_order_swallowing_errors(healthy, uuid4())
    outage = await find_order_swallowing_errors(down, uuid4())

    assert missing is None
    assert outage is None
    assert caplog.records == []


async def test_letting_it_propagate_keeps_the_outage_visible() -> None:
    down = OrderRepository(FailingOrderStore(ConnectionRefusedError("db-01:5432 refused")))

    with pytest.raises(DependencyUnavailableError):
        await down.find(uuid4())
