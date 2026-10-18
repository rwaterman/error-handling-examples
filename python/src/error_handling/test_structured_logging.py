import json
import logging
from types import TracebackType

from error_handling.structured_logging import JsonFormatter

type ExcInfo = tuple[type[BaseException], BaseException, TracebackType | None]


def make_record(exc_info: ExcInfo | None = None) -> logging.LogRecord:
    record = logging.LogRecord("orders", logging.ERROR, __file__, 1, "request failed", None, exc_info)
    record.request_id = "req-1"
    return record


def test_extra_fields_become_top_level_json_keys() -> None:
    payload = json.loads(JsonFormatter().format(make_record()))

    assert payload["message"] == "request failed"
    assert payload["level"] == "ERROR"
    assert payload["request_id"] == "req-1"
    assert "exception" not in payload


def test_exception_traceback_is_included() -> None:
    try:
        raise ValueError("boom")
    except ValueError as err:
        record = make_record((type(err), err, err.__traceback__))

    payload = json.loads(JsonFormatter().format(record))

    assert "ValueError: boom" in payload["exception"]
