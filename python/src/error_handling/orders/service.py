from datetime import UTC, datetime
from uuid import UUID, uuid4

from error_handling.orders.models import (
    CreateOrderRequest,
    Order,
    OrderAlreadyProcessed,
    OrderNotFound,
    ProcessOrderError,
)
from error_handling.orders.repository import OrderRepository
from error_handling.result import Err, Ok, Result


class OrderService:
    def __init__(self, repository: OrderRepository) -> None:
        self._repository = repository

    async def create(self, request: CreateOrderRequest) -> Order:
        order = Order(
            id=uuid4(),
            customer_email=request.customer_email,
            items=request.items,
            status="pending",
            created_at=datetime.now(UTC),
            processed_at=None,
        )
        await self._repository.save(order)
        return order

    async def get(self, order_id: UUID) -> Result[Order, OrderNotFound]:
        order = await self._repository.find(order_id)
        if order is None:
            return Err(OrderNotFound(order_id))
        return Ok(order)

    async def process(self, order_id: UUID) -> Result[Order, ProcessOrderError]:
        order = await self._repository.find(order_id)
        if order is None:
            return Err(OrderNotFound(order_id))
        if order.status == "processed":
            return Err(OrderAlreadyProcessed(order_id))
        processed = order.model_copy(update={"status": "processed", "processed_at": datetime.now(UTC)})
        await self._repository.save(processed)
        return Ok(processed)
