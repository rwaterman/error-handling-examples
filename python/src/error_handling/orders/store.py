from typing import Any, Protocol


class OrderStore(Protocol):
    async def get(self, key: str) -> dict[str, Any] | None: ...

    async def put(self, key: str, value: dict[str, Any]) -> None: ...


class InMemoryOrderStore:
    def __init__(self) -> None:
        self.records: dict[str, dict[str, Any]] = {}

    async def get(self, key: str) -> dict[str, Any] | None:
        return self.records.get(key)

    async def put(self, key: str, value: dict[str, Any]) -> None:
        self.records[key] = value
