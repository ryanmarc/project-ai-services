"""SummarizeTool — calls the summarize service at POST /v1/summarize."""

import json
import logging
import time
import uuid
from typing import Any, Iterator

from agent.http_client import get_session, retry_on_transient_error
from agent.tools.base import Tool

logger = logging.getLogger(__name__)


class SummarizeTool(Tool):
    def __init__(self, endpoint: str, pool_maxsize: int = 10, timeout: int = 120) -> None:
        self._endpoint = endpoint.rstrip("/")
        self._pool_maxsize = pool_maxsize
        self._timeout = timeout

    @property
    def name(self) -> str:
        return "summarize"

    @property
    def description(self) -> str:
        return (
            "Summarize text content to a shorter form. "
            "Accepts plain text and an optional target length in words."
        )

    @property
    def parameters_schema(self) -> dict[str, Any]:
        return {
            "type": "object",
            "properties": {
                "text": {
                    "type": "string",
                    "description": "The text content to summarize.",
                },
                "length": {
                    "type": "integer",
                    "description": "Optional desired summary length in words.",
                },
            },
            "required": ["text"],
        }

    @retry_on_transient_error(max_retries=3, initial_delay=1.0, backoff_multiplier=2.0)
    def execute(self, arguments: dict[str, Any]) -> str:
        session = get_session(self._pool_maxsize)
        payload: dict[str, Any] = {"text": arguments["text"], "stream": False}
        if "length" in arguments and arguments["length"] is not None:
            payload["length"] = arguments["length"]

        request_id = str(uuid.uuid4())
        url = f"{self._endpoint}/v1/summarize"
        logger.debug("Summarize API request: url=%s, request_id=%s, text_length=%d chars", url, request_id, len(payload["text"]))
        start = time.time()
        response = session.post(
            url,
            json=payload,
            headers={"X-Request-ID": request_id, "Content-Type": "application/json"},
            timeout=self._timeout,
        )
        elapsed_ms = int((time.time() - start) * 1000)
        logger.debug("Summarize API response: status=%d, request_id=%s, elapsed=%dms", response.status_code, request_id, elapsed_ms)

        if response.status_code == 429:
            return "Error: The summarize service is busy. Please try again later."
        if response.status_code in (400, 413, 415):
            try:
                err = response.json().get("error", {})
                return f"Error: {err.get('message', response.text)}"
            except Exception:
                return f"Error: {response.text}"
        response.raise_for_status()

        elapsed_ms = int((time.time() - start) * 1000)
        data = response.json()
        summary = data.get("summary", "")
        original_length = data.get("original_length", 0)
        summary_length = data.get("summary_length", 0)
        model = data.get("model", "unknown")
        usage = data.get("usage", {})

        return (
            f"Summary: {summary}\n\n"
            f"Original: {original_length} words -> Summary: {summary_length} words\n"
            f"Model: {model} | Time: {elapsed_ms}ms\n"
            f"Tokens: {usage.get('input_tokens', 0)} in / {usage.get('output_tokens', 0)} out"
        )

    def execute_stream(self, arguments: dict[str, Any]) -> Iterator[str]:
        session = get_session(self._pool_maxsize)
        payload: dict[str, Any] = {"text": arguments["text"], "stream": True}
        if "length" in arguments and arguments["length"] is not None:
            payload["length"] = arguments["length"]

        request_id = str(uuid.uuid4())
        response = session.post(
            f"{self._endpoint}/v1/summarize",
            json=payload,
            headers={"X-Request-ID": request_id, "Content-Type": "application/json"},
            timeout=self._timeout,
            stream=True,
        )

        if response.status_code != 200:
            try:
                err = response.json().get("error", {})
                yield f"Error: {err.get('message', response.text)}"
            except Exception:
                yield f"Error: {response.text}"
            return

        for raw_line in response.iter_lines(decode_unicode=True):
            if not raw_line or not raw_line.startswith("data: "):
                continue
            data_str = raw_line[len("data: "):]
            if data_str == "[DONE]":
                break
            try:
                chunk = json.loads(data_str)
                choices = chunk.get("choices", [])
                if choices:
                    delta = choices[0].get("delta", {})
                    content = delta.get("content", "")
                    if content:
                        yield content
            except json.JSONDecodeError:
                continue
