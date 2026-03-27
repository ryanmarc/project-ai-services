"""Local vLLM provider using OpenAI-compatible API."""

import json
import logging
from typing import Any, Iterator

from agent.http_client import get_session, retry_on_transient_error
from agent.llm.base import LLMProvider

logger = logging.getLogger(__name__)


class VLLMProvider(LLMProvider):
    def __init__(
        self,
        endpoint: str,
        model: str,
        temperature: float = 0.0,
        max_tokens: int = 1024,
        pool_maxsize: int = 10,
        timeout: int = 120,
    ) -> None:
        self._endpoint = endpoint.rstrip("/")
        self._model = model
        self._temperature = temperature
        self._max_tokens = max_tokens
        self._pool_maxsize = pool_maxsize
        self._timeout = timeout

    @retry_on_transient_error(max_retries=3, initial_delay=1.0, backoff_multiplier=2.0)
    def chat_completion(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]] | None = None,
        tool_choice: str | dict | None = None,
    ) -> dict[str, Any]:
        session = get_session(self._pool_maxsize)
        payload: dict[str, Any] = {
            "model": self._model,
            "messages": messages,
            "temperature": self._temperature,
            "max_tokens": self._max_tokens,
            "stream": False,
        }
        if tools:
            payload["tools"] = tools
        if tool_choice is not None:
            payload["tool_choice"] = tool_choice

        logger.debug("vLLM request payload: %s", json.dumps(payload, default=str)[:4000])
        response = session.post(
            f"{self._endpoint}/v1/chat/completions",
            json=payload,
            headers={"Content-Type": "application/json"},
            timeout=self._timeout,
        )
        response.raise_for_status()
        result = response.json()
        logger.debug("vLLM response: %s", json.dumps(result, default=str)[:2000])
        return result

    def chat_completion_stream(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]] | None = None,
        tool_choice: str | dict | None = None,
    ) -> Iterator[dict[str, Any]]:
        session = get_session(self._pool_maxsize)
        payload: dict[str, Any] = {
            "model": self._model,
            "messages": messages,
            "temperature": self._temperature,
            "max_tokens": self._max_tokens,
            "stream": True,
        }
        if tools:
            payload["tools"] = tools
        if tool_choice is not None:
            payload["tool_choice"] = tool_choice

        with session.post(
            f"{self._endpoint}/v1/chat/completions",
            json=payload,
            headers={"Content-Type": "application/json"},
            timeout=self._timeout,
            stream=True,
        ) as response:
            response.raise_for_status()
            for raw_line in response.iter_lines(decode_unicode=True):
                if not raw_line or not raw_line.startswith("data: "):
                    continue
                data_str = raw_line[len("data: "):]
                if data_str == "[DONE]":
                    break
                try:
                    yield json.loads(data_str)
                except json.JSONDecodeError:
                    continue
