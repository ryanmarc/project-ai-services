from __future__ import annotations

import asyncio
import json
import time
import uuid

import httpx

from app.config import ModelEntry, resolve_env_ref

_PREFIX = "watsonx/"
_IAM_URL = "https://iam.cloud.ibm.com/identity/token"
_IAM_GRANT = "urn:ibm:params:oauth:grant-type:apikey"
_TOKEN_REFRESH_BUFFER = 60  # seconds


class WatsonxProvider:
    def __init__(self, entry: ModelEntry, timeout: float) -> None:
        self.entry = entry
        self.timeout = timeout
        self._transport: httpx.BaseTransport | None = None
        self._token: str | None = None
        self._token_expires_at: float = 0.0
        self._token_lock = asyncio.Lock()

    # --- Helpers ----------------------------------------------------------

    def _model_id(self) -> str:
        return self.entry.params["model"][len(_PREFIX):]

    def _project_id(self) -> str:
        return resolve_env_ref(self.entry.params["watsonx_project_id"])

    def _api_key(self) -> str:
        return resolve_env_ref(self.entry.params["api_key"])

    def _key_type(self) -> str:
        return self.entry.params.get("api_key_type", "iam")

    def _watsonx_url(self) -> str:
        return resolve_env_ref(self.entry.params["api_base"]).rstrip("/")

    def _client(self) -> httpx.AsyncClient:
        kwargs: dict = {"timeout": self.timeout}
        if self._transport is not None:
            kwargs["transport"] = self._transport
        return httpx.AsyncClient(**kwargs)

    # --- Auth -------------------------------------------------------------

    async def _get_bearer(self) -> str:
        if self._key_type() == "zen":
            return self._api_key()
        async with self._token_lock:
            now = time.time()
            if self._token and self._token_expires_at - _TOKEN_REFRESH_BUFFER > now:
                return self._token
            async with self._client() as c:
                r = await c.post(
                    _IAM_URL,
                    data={"grant_type": _IAM_GRANT, "apikey": self._api_key()},
                    headers={"Content-Type": "application/x-www-form-urlencoded"},
                )
                r.raise_for_status()
                payload = r.json()
            self._token = payload["access_token"]
            self._token_expires_at = now + int(payload["expires_in"])
            return self._token

    # --- Public API -------------------------------------------------------

    _OPENAI_TO_WX_PARAMS = {
        "max_tokens": "max_tokens",
        "temperature": "temperature",
        "top_p": "top_p",
        "stop": "stop_sequences",
    }

    def _to_watsonx_chat_body(self, body: dict) -> dict:
        params: dict = {}
        for src, dst in self._OPENAI_TO_WX_PARAMS.items():
            if src in body:
                params[dst] = body[src]
        return {
            "model_id": self._model_id(),
            "project_id": self._project_id(),
            "messages": body.get("messages", []),
            "parameters": params,
        }

    def _from_watsonx_chat_response(self, payload: dict, model_name: str) -> dict:
        result = payload["results"][0]
        prompt_tokens = result.get("input_token_count", 0)
        completion_tokens = result.get("generated_token_count", 0)
        return {
            "id": f"chatcmpl-{uuid.uuid4().hex}",
            "object": "chat.completion",
            "created": int(time.time()),
            "model": model_name,
            "choices": [
                {
                    "index": 0,
                    "message": {
                        "role": "assistant",
                        "content": result.get("generated_text", ""),
                    },
                    "finish_reason": result.get("stop_reason", "stop"),
                }
            ],
            "usage": {
                "prompt_tokens": prompt_tokens,
                "completion_tokens": completion_tokens,
                "total_tokens": prompt_tokens + completion_tokens,
            },
        }

    async def chat(self, body: dict) -> dict:
        token = await self._get_bearer()
        wx_body = self._to_watsonx_chat_body(body)
        async with self._client() as c:
            r = await c.post(
                f"{self._watsonx_url()}/ml/v1/text/chat?version=2024-05-31",
                json=wx_body,
                headers={"Authorization": f"Bearer {token}"},
            )
            r.raise_for_status()
            return self._from_watsonx_chat_response(r.json(), body["model"])

    async def chat_stream(self, body: dict):
        token = await self._get_bearer()
        wx_body = self._to_watsonx_chat_body(body)
        chat_id = f"chatcmpl-{uuid.uuid4().hex}"
        created = int(time.time())
        client = self._client()
        try:
            async with client.stream(
                "POST",
                f"{self._watsonx_url()}/ml/v1/text/chat_stream?version=2024-05-31",
                json=wx_body,
                headers={"Authorization": f"Bearer {token}"},
            ) as r:
                r.raise_for_status()
                buf = b""
                async for raw in r.aiter_raw():
                    buf += raw
                    while b"\n\n" in buf:
                        frame, buf = buf.split(b"\n\n", 1)
                        for chunk in self._translate_sse_frame(frame, body["model"], chat_id, created):
                            yield chunk
            yield b"data: [DONE]\n\n"
        finally:
            await client.aclose()

    def _translate_sse_frame(
        self, frame: bytes, model_name: str, chat_id: str, created: int
    ):
        for line in frame.splitlines():
            if not line.startswith(b"data:"):
                continue
            payload_text = line[5:].strip()
            if not payload_text or payload_text == b"[DONE]":
                continue
            try:
                payload = json.loads(payload_text)
            except json.JSONDecodeError:
                continue
            result = (payload.get("results") or [{}])[0]
            delta: dict = {}
            text = result.get("generated_text")
            if text:
                delta["content"] = text
            finish_reason = result.get("stop_reason")
            openai_chunk = {
                "id": chat_id,
                "object": "chat.completion.chunk",
                "created": created,
                "model": model_name,
                "choices": [
                    {
                        "index": 0,
                        "delta": delta,
                        "finish_reason": finish_reason,
                    }
                ],
            }
            yield b"data: " + json.dumps(openai_chunk).encode() + b"\n\n"

    async def embeddings(self, body: dict) -> dict:  # pragma: no cover - Task 7
        raise NotImplementedError

    async def rerank(self, body: dict) -> dict:  # pragma: no cover - Task 7
        raise NotImplementedError
