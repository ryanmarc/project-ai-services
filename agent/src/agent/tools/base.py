"""Tool abstraction and registry for the agent."""

from abc import ABC, abstractmethod
from typing import Any, Iterator


class Tool(ABC):
    @property
    @abstractmethod
    def name(self) -> str: ...

    @property
    @abstractmethod
    def description(self) -> str: ...

    @property
    @abstractmethod
    def parameters_schema(self) -> dict[str, Any]: ...

    @abstractmethod
    def execute(self, arguments: dict[str, Any]) -> str:
        """Execute the tool and return the full result."""
        ...

    def execute_stream(self, arguments: dict[str, Any]) -> Iterator[str]:
        """Execute the tool and yield result chunks. Defaults to non-streaming."""
        yield self.execute(arguments)

    def to_openai_function(self) -> dict[str, Any]:
        """Convert to OpenAI function-calling format."""
        return {
            "type": "function",
            "function": {
                "name": self.name,
                "description": self.description,
                "parameters": self.parameters_schema,
            },
        }


class ToolRegistry:
    def __init__(self) -> None:
        self._tools: dict[str, Tool] = {}

    def register(self, tool: Tool) -> None:
        self._tools[tool.name] = tool

    def get(self, name: str) -> Tool | None:
        return self._tools.get(name)

    def all_schemas(self) -> list[dict[str, Any]]:
        return [t.to_openai_function() for t in self._tools.values()]

    def list_names(self) -> list[str]:
        return list(self._tools.keys())
