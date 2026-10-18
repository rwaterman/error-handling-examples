from uuid import UUID

from error_handling.errors import DependencyUnavailableError
from error_handling.orders.models import Order
from error_handling.orders.store import OrderStore

DRIVER_ERRORS = (ConnectionError, TimeoutError)


class OrderRepository:
    def __init__(self, store: OrderStore) -> None:
        self._store = store

    async def find(self, order_id: UUID) -> Order | None:
        try:
            record = await self._store.get(str(order_id))
        except DRIVER_ERRORS as err:
            raise DependencyUnavailableError("order-store") from err
        return None if record is None else Order.model_validate(record)

    async def save(self, order: Order) -> None:
        try:
            await self._store.put(str(order.id), order.model_dump(mode="json"))
        except DRIVER_ERRORS as err:
            raise DependencyUnavailableError("order-store") from err
