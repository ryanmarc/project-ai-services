"""Local vLLM provider using OpenAI-compatible API."""

import json
import logging
import uuid
from typing import Any, Iterator

from agent.http_client import get_session, retry_on_transient_error
from agent.llm.base import LLMProvider

logger = logging.getLogger(__name__)


def _render_granite_prompt(
    messages: list[dict[str, Any]],
    tools: list[dict[str, Any]],
) -> str:
    """Render messages and tools into a Granite 3.3 prompt string."""
    parts: list[str] = []

    # Inject tool definitions
    parts.append("<|start_of_role|>available_tools<|end_of_role|>")
    parts.append(json.dumps(tools))
    parts.append("<|end_of_text|>")

    for msg in messages:
        role = msg["role"]

        if role == "system":
            parts.append("<|start_of_role|>system<|end_of_role|>")
            parts.append(msg["content"])
            parts.append("<|end_of_text|>")

        elif role == "user":
            parts.append("<|start_of_role|>user<|end_of_role|>")
            parts.append(msg["content"])
            parts.append("<|end_of_text|>")

        elif role == "assistant":
            parts.append("<|start_of_role|>assistant<|end_of_role|>")
            tool_calls = msg.get("tool_calls") or []
            if tool_calls:
                tc_list = []
                for tc in tool_calls:
                    fn = tc["function"]
                    raw_args = fn.get("arguments", "{}")
                    args = json.loads(raw_args) if isinstance(raw_args, str) else raw_args
                    tc_list.append({"name": fn["name"], "arguments": args})
                parts.append("<|tool_call|>")
                parts.append(json.dumps(tc_list))
            elif msg.get("content"):
                parts.append(msg["content"])
            parts.append("<|end_of_text|>")

        elif role == "tool":
            parts.append("<|start_of_role|>tool_response<|end_of_role|>")
            parts.append(msg["content"])
            parts.append("<|end_of_text|>")

    # Generation prompt
    parts.append("<|start_of_role|>assistant<|end_of_role|>")
    return "\n".join(parts)


def _parse_tool_calls_from_text(text: str) -> list[dict[str, Any]]:
    """Parse Granite tool call JSON from raw completion text.

    The model outputs a JSON array like: [{"name": "fn", "arguments": {...}}]
    when it wants to call tools. The <|tool_call|> token is consumed as a stop
    token by vLLM and does not appear in the text.
    """
    text = text.strip()
    if not text.startswith("["):
        return []
    try:
        calls = json.loads(text)
    except json.JSONDecodeError:
        return []
    if not isinstance(calls, list):
        return []
    result = []
    for c in calls:
        if not isinstance(c, dict) or "name" not in c:
            continue
        result.append({
            "id": f"call_{uuid.uuid4().hex[:8]}",
            "type": "function",
            "function": {
                "name": c["name"],
                "arguments": json.dumps(c.get("arguments", {})),
            },
        })
    return result


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
    def _completions_request(
        self, prompt: str, stream: bool = False
    ) -> Any:
        """Send a request to /v1/completions."""
        session = get_session(self._pool_maxsize)
        payload: dict[str, Any] = {
            "model": self._model,
            "prompt": prompt,
            "temperature": self._temperature,
            "max_tokens": self._max_tokens,
            "stream": stream,
        }
        logger.debug("vLLM completions request (prompt length=%d chars)", len(prompt))
        logger.debug("vLLM completions prompt: %s", prompt[:4000])
        response = session.post(
            f"{self._endpoint}/v1/completions",
            json=payload,
            headers={"Content-Type": "application/json"},
            timeout=self._timeout,
            stream=stream,
        )
        response.raise_for_status()
        if stream:
            return response
        result = response.json()
        logger.debug("vLLM completions response: %s", json.dumps(result, default=str)[:2000])
        return result

    @retry_on_transient_error(max_retries=3, initial_delay=1.0, backoff_multiplier=2.0)
    def _chat_completions_request(
        self, messages: list[dict[str, Any]]
    ) -> dict[str, Any]:
        """Send a request to /v1/chat/completions (no tools)."""
        session = get_session(self._pool_maxsize)
        payload: dict[str, Any] = {
            "model": self._model,
            "messages": messages,
            "temperature": self._temperature,
            "max_tokens": self._max_tokens,
            "stream": False,
        }
        logger.debug("vLLM chat request payload: %s", json.dumps(payload, default=str)[:4000])
        response = session.post(
            f"{self._endpoint}/v1/chat/completions",
            json=payload,
            headers={"Content-Type": "application/json"},
            timeout=self._timeout,
        )
        response.raise_for_status()
        result = response.json()
        logger.debug("vLLM chat response: %s", json.dumps(result, default=str)[:2000])
        return result

    def chat_completion(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]] | None = None,
        tool_choice: str | dict | None = None,
    ) -> dict[str, Any]:
        if not tools:
            return self._chat_completions_request(messages)

        # Tools provided: render Granite prompt and use completions API
        prompt = _render_granite_prompt(messages, tools)
        result = self._completions_request(prompt)

        text = result["choices"][0]["text"]
        finish_reason = result["choices"][0].get("finish_reason", "stop")
        tool_calls = _parse_tool_calls_from_text(text)

        if tool_calls:
            logger.debug(
                "Parsed %d tool call(s) from completion text: %s",
                len(tool_calls),
                [tc["function"]["name"] for tc in tool_calls],
            )

        return {
            "choices": [{
                "message": {
                    "role": "assistant",
                    "content": text if not tool_calls else None,
                    "tool_calls": tool_calls if tool_calls else None,
                },
                "finish_reason": finish_reason,
            }]
        }

    def chat_completion_stream(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]] | None = None,
        tool_choice: str | dict | None = None,
    ) -> Iterator[dict[str, Any]]:
        if not tools:
            # No tools — stream via chat completions
            session = get_session(self._pool_maxsize)
            payload: dict[str, Any] = {
                "model": self._model,
                "messages": messages,
                "temperature": self._temperature,
                "max_tokens": self._max_tokens,
                "stream": True,
            }
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
            return

        # Tools provided — stream via completions API
        prompt = _render_granite_prompt(messages, tools)
        resp = self._completions_request(prompt, stream=True)
        with resp:
            for raw_line in resp.iter_lines(decode_unicode=True):
                if not raw_line or not raw_line.startswith("data: "):
                    continue
                data_str = raw_line[len("data: "):]
                if data_str == "[DONE]":
                    break
                try:
                    chunk = json.loads(data_str)
                    # Translate completions chunk to chat completions format
                    text = chunk.get("choices", [{}])[0].get("text", "")
                    yield {
                        "choices": [{
                            "delta": {"content": text},
                            "finish_reason": chunk.get("choices", [{}])[0].get("finish_reason"),
                        }]
                    }
                except json.JSONDecodeError:
                    continue
