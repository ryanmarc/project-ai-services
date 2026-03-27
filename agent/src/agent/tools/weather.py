"""Mock weather tool for testing tool calling behavior."""

from typing import Any

from agent.tools.base import Tool


class WeatherTool(Tool):
    @property
    def name(self) -> str:
        return "get_weather"

    @property
    def description(self) -> str:
        return "Get the current weather for a given city. Returns temperature, conditions, and humidity."

    @property
    def parameters_schema(self) -> dict[str, Any]:
        return {
            "type": "object",
            "properties": {
                "city": {
                    "type": "string",
                    "description": "The city name to get weather for.",
                },
            },
            "required": ["city"],
        }

    def execute(self, arguments: dict[str, Any]) -> str:
        city = arguments.get("city", "Unknown")
        return f"Weather in {city}: 72°F, partly cloudy, humidity 45%."
