from __future__ import annotations

import asyncio
import time

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

    # --- Public API (implemented in later tasks) --------------------------

    async def chat(self, body: dict) -> dict:  # pragma: no cover - filled in Task 6
        raise NotImplementedError

    async def chat_stream(self, body: dict):  # pragma: no cover - filled in Task 6
        raise NotImplementedError
        yield b""  # keep as async generator

    async def embeddings(self, body: dict) -> dict:  # pragma: no cover - filled in Task 7
        raise NotImplementedError

    async def rerank(self, body: dict) -> dict:  # pragma: no cover - filled in Task 7
        raise NotImplementedError
