"""LLM provider abstract base class."""

from abc import ABC, abstractmethod
from typing import Any, Iterator


class LLMProvider(ABC):
    @abstractmethod
    def chat_completion(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]] | None = None,
        tool_choice: str | dict | None = None,
    ) -> dict[str, Any]:
        """Non-streaming chat completion. Returns OpenAI-compatible response."""
        ...

    @abstractmethod
    def chat_completion_stream(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]] | None = None,
        tool_choice: str | dict | None = None,
    ) -> Iterator[dict[str, Any]]:
        """Streaming chat completion. Yields OpenAI-compatible SSE chunks."""
        ...
