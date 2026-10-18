from uuid import uuid4

import pytest

from error_handling.conftest import FailingOrderStore
from error_handling.errors import AppError, DependencyUnavailableError
from error_handling.orders.repository import OrderRepository


def test_dependency_unavailable_carries_its_status_and_code() -> None:
    error = DependencyUnavailableError("order-store")

    assert isinstance(error, AppError)
    assert error.status == 503
    assert error.code == "DEPENDENCY_UNAVAILABLE"
    assert error.dependency == "order-store"


@pytest.mark.parametrize("driver_error", [ConnectionRefusedError("db-01:5432 refused"), TimeoutError("read timed out")])
async def test_repository_translates_driver_errors_and_keeps_the_cause(driver_error: Exception) -> None:
    repository = OrderRepository(FailingOrderStore(driver_error))

    with pytest.raises(DependencyUnavailableError) as raised:
        await repository.find(uuid4())

    assert raised.value.__cause__ is driver_error


async def test_repository_lets_unexpected_errors_through_untouched() -> None:
    bug = KeyError("customerEmail")
    repository = OrderRepository(FailingOrderStore(bug))

    with pytest.raises(KeyError) as raised:
        await repository.find(uuid4())

    assert raised.value is bug
