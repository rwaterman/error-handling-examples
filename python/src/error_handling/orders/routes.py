from typing import Annotated, assert_never
from uuid import UUID

from fastapi import APIRouter, Path, Request
from fastapi.responses import JSONResponse

from error_handling.orders.models import CreateOrderRequest, Order, OrderAlreadyProcessed, OrderNotFound
from error_handling.orders.service import OrderService
from error_handling.problem import problem
from error_handling.result import Err, Ok

OrderId = Annotated[UUID, Path(alias="orderId")]


def order_problem(request: Request, error: OrderNotFound | OrderAlreadyProcessed) -> JSONResponse:
    match error:
        case OrderNotFound(order_id=order_id):
            return problem(request, 404, "ORDER_NOT_FOUND", f"Order {order_id} does not exist.")
        case OrderAlreadyProcessed(order_id=order_id):
            return problem(request, 409, "ORDER_ALREADY_PROCESSED", f"Order {order_id} was already processed.")
        case _:
            assert_never(error)


def build_order_router(service: OrderService) -> APIRouter:
    router = APIRouter(prefix="/orders")

    @router.post("", status_code=201, response_model=Order)
    async def create_order(body: CreateOrderRequest) -> Order:
        return await service.create(body)

    @router.get("/{orderId}", response_model=Order)
    async def get_order(request: Request, order_id: OrderId) -> Order | JSONResponse:
        match await service.get(order_id):
            case Ok(order):
                return order
            case Err(error):
                return order_problem(request, error)

    @router.post("/{orderId}/process", response_model=Order)
    async def process_order(request: Request, order_id: OrderId) -> Order | JSONResponse:
        match await service.process(order_id):
            case Ok(order):
                return order
            case Err(error):
                return order_problem(request, error)

    return router
