"""Core agent loop: prompt -> LLM -> tool calls -> result."""

import json
import logging
from typing import Any, Iterator

from agent.llm.base import LLMProvider
from agent.tools.base import ToolRegistry

logger = logging.getLogger(__name__)

SYSTEM_PROMPT = (
    "You are a helpful AI assistant with access to the following tools. "
    "When a tool is required to answer the user's query, respond only with "
    "<|tool_call|> followed by a JSON list of tools used. "
    "If a tool does not exist in the provided list of tools, notify the user "
    "that you do not have the ability to fulfill the request."
)


def _extract_tool_calls(message: dict[str, Any]) -> list[dict[str, Any]]:
    """Extract tool calls from an assistant message."""
    return message.get("tool_calls") or []


def _execute_tool_calls(
    tool_calls: list[dict[str, Any]], registry: ToolRegistry
) -> list[dict[str, str]]:
    """Execute tool calls and return tool response messages."""
    results = []
    for call in tool_calls:
        tool_name = call["function"]["name"]
        call_id = call.get("id", "")
        raw_args = call["function"].get("arguments", "{}")

        try:
            arguments = json.loads(raw_args) if isinstance(raw_args, str) else raw_args
        except json.JSONDecodeError as e:
            logger.warning("Tool %s: failed to parse arguments: %s", tool_name, e)
            results.append({
                "role": "tool",
                "tool_call_id": call_id,
                "content": f"Error parsing arguments: {e}",
            })
            continue

        tool = registry.get(tool_name)
        if tool is None:
            logger.warning("Tool %s: not found in registry", tool_name)
            results.append({
                "role": "tool",
                "tool_call_id": call_id,
                "content": f"Error: unknown tool '{tool_name}'",
            })
            continue

        logger.debug("Tool %s: executing with args %s", tool_name, json.dumps(arguments)[:200])
        try:
            result = tool.execute(arguments)
            logger.debug("Tool %s: completed, result length=%d chars", tool_name, len(result))
            logger.debug("Tool %s: result preview=%s", tool_name, result[:500])
        except Exception as e:
            logger.exception("Tool %s execution failed", tool_name)
            result = f"Error executing {tool_name}: {e}"

        results.append({
            "role": "tool",
            "tool_call_id": call_id,
            "content": result,
        })
    return results


def run_agent(
    user_input: str,
    llm: LLMProvider,
    registry: ToolRegistry,
    max_iterations: int = 5,
) -> str:
    """Run the agent loop (non-streaming). Returns the final text response."""
    messages: list[dict[str, Any]] = [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "user", "content": user_input},
    ]
    tools = registry.all_schemas() or None

    logger.debug("Agent loop started: input=%s, available tools=%s", user_input[:100], [t["function"]["name"] for t in (tools or [])])

    for i in range(max_iterations):
        logger.debug("Agent loop iteration %d/%d: calling LLM", i + 1, max_iterations)
        response = llm.chat_completion(messages, tools=tools)
        choice = response["choices"][0]
        assistant_msg = choice["message"]
        finish_reason = choice.get("finish_reason", "unknown")

        tool_calls = _extract_tool_calls(assistant_msg)
        if not tool_calls:
            content = assistant_msg.get("content", "") or ""
            logger.debug("Agent loop finished: no tool calls, finish_reason=%s, response length=%d chars", finish_reason, len(content))
            return content

        tool_names = [tc["function"]["name"] for tc in tool_calls]
        logger.debug("Agent loop iteration %d/%d: LLM requested %d tool call(s): %s", i + 1, max_iterations, len(tool_calls), tool_names)

        messages.append(assistant_msg)
        tool_results = _execute_tool_calls(tool_calls, registry)
        messages.extend(tool_results)

    logger.debug("Agent loop exhausted %d iterations without final answer", max_iterations)
    return "I was unable to complete your request within the allowed number of steps."


def run_agent_stream(
    user_input: str,
    llm: LLMProvider,
    registry: ToolRegistry,
    max_iterations: int = 5,
) -> Iterator[str]:
    """Run the agent loop, streaming the final LLM response."""
    messages: list[dict[str, Any]] = [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "user", "content": user_input},
    ]
    tools = registry.all_schemas() or None

    for i in range(max_iterations):
        logger.debug("Agent stream loop iteration %d/%d", i + 1, max_iterations)

        # Use non-streaming for tool-calling iterations
        response = llm.chat_completion(messages, tools=tools)
        choice = response["choices"][0]
        assistant_msg = choice["message"]

        tool_calls = _extract_tool_calls(assistant_msg)
        if not tool_calls:
            # Final iteration — re-do as streaming for the answer
            # Remove the non-streaming response and stream instead
            for chunk in llm.chat_completion_stream(messages, tools=tools):
                choices = chunk.get("choices", [])
                if choices:
                    delta = choices[0].get("delta", {})
                    content = delta.get("content", "")
                    if content:
                        yield content
            return

        messages.append(assistant_msg)
        tool_results = _execute_tool_calls(tool_calls, registry)
        messages.extend(tool_results)

    yield "I was unable to complete your request within the allowed number of steps."
