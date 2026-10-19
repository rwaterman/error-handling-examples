from error_handling.app import create_app
from error_handling.orders.repository import OrderRepository
from error_handling.orders.service import OrderService
from error_handling.orders.store import InMemoryOrderStore
from error_handling.structured_logging import configure_logging

configure_logging()
app = create_app(OrderService(OrderRepository(InMemoryOrderStore())))
