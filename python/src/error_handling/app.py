from fastapi import FastAPI

from error_handling.orders.routes import build_order_router
from error_handling.orders.service import OrderService
from error_handling.problem import register_error_handlers


def create_app(order_service: OrderService) -> FastAPI:
    app = FastAPI(title="Error handling patterns")
    register_error_handlers(app)
    app.include_router(build_order_router(order_service))
    return app
