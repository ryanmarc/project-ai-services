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
        self._cached_client: httpx.AsyncClient | None = None
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
        if self._cached_client is None:
            kwargs: dict = {"timeout": self.timeout}
            if self._transport is not None:
                kwargs["transport"] = self._transport
            self._cached_client = httpx.AsyncClient(**kwargs)
        return self._cached_client

    async def aclose(self) -> None:
        if self._cached_client is not None:
            await self._cached_client.aclose()
            self._cached_client = None

    # --- Auth -------------------------------------------------------------

    async def _get_bearer(self) -> str:
        if self._key_type() == "zen":
            return self._api_key()
        async with self._token_lock:
            now = time.time()
            if self._token and self._token_expires_at - _TOKEN_REFRESH_BUFFER > now:
                return self._token
            c = self._client()
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

        # Transform messages to watsonx format
        # Watsonx expects content as array of objects with type and text fields
        messages = []
        for msg in body.get("messages", []):
            transformed_msg = {"role": msg.get("role", "user")}
            content = msg.get("content", "")

            # If content is already in watsonx format (list), use as-is
            if isinstance(content, list):
                transformed_msg["content"] = content
            # Otherwise, transform string content to watsonx format
            else:
                transformed_msg["content"] = [
                    {
                        "type": "text",
                        "text": content
                    }
                ]
            messages.append(transformed_msg)

        return {
            "model_id": self._model_id(),
            "project_id": self._project_id(),
            "messages": messages,
            "parameters": params,
        }

    def _from_watsonx_chat_response(self, payload: dict, model_name: str) -> dict:
        # Watsonx text/chat response is already in OpenAI format
        # Just need to ensure it has the model name we expect
        response = payload.copy()
        response["model"] = model_name
        return response

    def _passthrough_sse_frame(self, frame: bytes, model_name: str):
        """Pass through watsonx SSE frames, updating the model name."""
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
            # Watsonx response is already in OpenAI format, just update model name
            payload["model"] = model_name
            yield b"data: " + json.dumps(payload).encode() + b"\n\n"

    async def chat(self, body: dict) -> dict:
        token = await self._get_bearer()
        wx_body = self._to_watsonx_chat_body(body)
        c = self._client()
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
        async with self._client().stream(
            "POST",
            f"{self._watsonx_url()}/ml/v1/text/chat_stream?version=2024-05-31",
            json=wx_body,
            headers={"Authorization": f"Bearer {token}"},
        ) as r:
            r.raise_for_status()
            # Watsonx streaming responses are already in OpenAI format
            # Use aiter_bytes() instead of aiter_raw() to handle gzip decompression
            buf = b""
            async for chunk in r.aiter_bytes():
                buf += chunk
                while b"\n\n" in buf:
                    frame, buf = buf.split(b"\n\n", 1)
                    for sse_chunk in self._passthrough_sse_frame(frame, body["model"]):
                        yield sse_chunk
            if buf.strip():
                for sse_chunk in self._passthrough_sse_frame(buf, body["model"]):
                    yield sse_chunk
        yield b"data: [DONE]\n\n"


    async def embeddings(self, body: dict) -> dict:
        token = await self._get_bearer()
        raw_input = body.get("input")
        inputs = [raw_input] if isinstance(raw_input, str) else list(raw_input or [])
        wx_body = {
            "model_id": self._model_id(),
            "project_id": self._project_id(),
            "inputs": inputs,
        }
        c = self._client()
        r = await c.post(
            f"{self._watsonx_url()}/ml/v1/text/embeddings?version=2024-05-31",
            json=wx_body,
            headers={"Authorization": f"Bearer {token}"},
        )
        r.raise_for_status()
        payload = r.json()
        return {
            "object": "list",
            "model": body["model"],
            "data": [
                {"object": "embedding", "index": i, "embedding": result["embedding"]}
                for i, result in enumerate(payload.get("results", []))
            ],
        }

    async def rerank(self, body: dict) -> dict:
        token = await self._get_bearer()
        query = body.get("query", "")
        documents = body.get("documents", [])

        # Transform documents to watsonx format
        # Each document should be an object with a "text" field
        inputs = []
        for doc in documents:
            if isinstance(doc, dict):
                # If already in correct format, use as-is
                inputs.append(doc)
            else:
                # If string, wrap in {"text": ...} format
                inputs.append({"text": doc})

        # Build watsonx rerank request body
        wx_body = {
            "model_id": self._model_id(),
            "project_id": self._project_id(),
            "query": query,
            "inputs": inputs,
        }

        # Add optional parameters under parameters.return_options
        return_options = {}
        if "top_n" in body:
            return_options["top_n"] = body["top_n"]
        if "return_documents" in body:
            return_options["return_documents"] = body["return_documents"]
        if "return_query" in body:
            return_options["return_query"] = body["return_query"]

        if return_options:
            wx_body["parameters"] = {"return_options": return_options}

        c = self._client()
        r = await c.post(
            f"{self._watsonx_url()}/ml/v1/text/rerank?version=2024-05-31",
            json=wx_body,
            headers={"Authorization": f"Bearer {token}"},
        )
        r.raise_for_status()
        payload = r.json()

        # Transform watsonx response to expected format
        results = []
        for i, result in enumerate(payload.get("results", [])):
            results.append({
                "index": result.get("index", i),
                "relevance_score": result.get("score", 0.0),
            })
            # Include document if present in response
            if "document" in result:
                results[-1]["document"] = result["document"]

        return {
            "id": f"rerank-{uuid.uuid4().hex}",
            "results": results,
            "model": body["model"],
        }
