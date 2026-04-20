from __future__ import annotations

from typing import AsyncIterator

import httpx

from app.config import ModelEntry, resolve_env_ref

_PREFIX = "openai/"


class VLLMProvider:
    def __init__(self, entry: ModelEntry, timeout: float) -> None:
        self.entry = entry
        self.timeout = timeout
        self._transport: httpx.BaseTransport | None = None

    # --- Upstream address helpers -----------------------------------------

    def _upstream_model(self) -> str:
        return self.entry.params["model"][len(_PREFIX):]

    def _api_base(self) -> str:
        return resolve_env_ref(self.entry.params["api_base"]).rstrip("/")

    def _client(self) -> httpx.AsyncClient:
        kwargs: dict = {"timeout": self.timeout}
        if self._transport is not None:
            kwargs["transport"] = self._transport
        return httpx.AsyncClient(**kwargs)

    def _rewrite_model(self, body: dict) -> dict:
        return {**body, "model": self._upstream_model()}

    # --- Public provider API ----------------------------------------------

    async def chat(self, body: dict) -> dict:
        async with self._client() as c:
            r = await c.post(
                f"{self._api_base()}/chat/completions",
                json=self._rewrite_model(body),
            )
            r.raise_for_status()
            return r.json()

    async def chat_stream(self, body: dict) -> AsyncIterator[bytes]:
        body = {**self._rewrite_model(body), "stream": True}
        client = self._client()
        try:
            async with client.stream(
                "POST",
                f"{self._api_base()}/chat/completions",
                json=body,
            ) as r:
                r.raise_for_status()
                async for chunk in r.aiter_raw():
                    yield chunk
        finally:
            await client.aclose()

    async def embeddings(self, body: dict) -> dict:
        async with self._client() as c:
            r = await c.post(
                f"{self._api_base()}/embeddings",
                json=self._rewrite_model(body),
            )
            r.raise_for_status()
            return r.json()

    async def rerank(self, body: dict) -> dict:
        async with self._client() as c:
            r = await c.post(
                f"{self._api_base()}/rerank",
                json=self._rewrite_model(body),
            )
            r.raise_for_status()
            return r.json()
