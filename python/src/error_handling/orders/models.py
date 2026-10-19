from dataclasses import dataclass
from datetime import datetime
from typing import Annotated, Literal
from uuid import UUID

from pydantic import BaseModel, ConfigDict, EmailStr, Field
from pydantic.alias_generators import to_camel


class CamelModel(BaseModel):
    model_config = ConfigDict(alias_generator=to_camel, validate_by_name=True, validate_by_alias=True, frozen=True)


class OrderItem(CamelModel):
    model_config = ConfigDict(extra="forbid")

    sku: Annotated[str, Field(min_length=1, max_length=64)]
    quantity: Annotated[int, Field(strict=True, ge=1, le=100)]


class CreateOrderRequest(CamelModel):
    model_config = ConfigDict(extra="forbid", validate_by_name=False)

    customer_email: EmailStr
    items: Annotated[list[OrderItem], Field(min_length=1, max_length=50)]


class Order(CamelModel):
    id: UUID
    customer_email: str
    items: list[OrderItem]
    status: Literal["pending", "processed"]
    created_at: datetime
    processed_at: datetime | None


@dataclass(frozen=True, slots=True)
class OrderNotFound:
    order_id: UUID


@dataclass(frozen=True, slots=True)
class OrderAlreadyProcessed:
    order_id: UUID


type ProcessOrderError = OrderNotFound | OrderAlreadyProcessed
