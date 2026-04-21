from __future__ import annotations

import hmac
import json
import os
import typing
from contextlib import asynccontextmanager

import httpx
from fastapi import FastAPI, HTTPException, Request, status
from fastapi.responses import JSONResponse, StreamingResponse

from app.config import GatewayConfig, load_config, resolve_env_ref
from app.providers.base import Provider
from app.router import build_providers


def _error(status_code: int, message: str, type_: str) -> JSONResponse:
    return JSONResponse(
        status_code=status_code,
        content={"error": {"message": message, "type": type_, "code": None}},
    )


def _resolve_master_key(ref: str | None) -> str | None:
    if not ref:
        return None
    try:
        value = resolve_env_ref(ref)
    except KeyError:
        return None
    return value or None


def create_app(cfg: GatewayConfig, providers: dict[str, Provider] | None = None) -> FastAPI:
    @asynccontextmanager
    async def lifespan(app: FastAPI):
        yield
        for provider in app.state.providers.values():
            close = getattr(provider, "aclose", None)
            if close is not None:
                await close()

    app = FastAPI(title="llm-gateway", lifespan=lifespan)
    app.state.config = cfg
    app.state.providers = providers if providers is not None else build_providers(cfg)

    @app.middleware("http")
    async def auth_middleware(request: Request, call_next):
        if request.url.path == "/health":
            return await call_next(request)
        master_key = _resolve_master_key(cfg.master_key_ref)
        if master_key is None:
            return await call_next(request)
        header = request.headers.get("authorization", "")
        if not header.startswith("Bearer ") or not hmac.compare_digest(header[7:], master_key):
            return _error(
                status.HTTP_401_UNAUTHORIZED,
                "missing or invalid bearer token",
                "auth",
            )
        return await call_next(request)

    @app.get("/health")
    async def health() -> dict:
        return {"status": "ok"}

    @app.get("/v1/models")
    async def list_models() -> dict:
        return {
            "object": "list",
            "data": [{"id": name, "object": "model"} for name in cfg.models],
        }

    async def _read_body(request: Request) -> dict:
        raw = await request.body()
        try:
            data = json.loads(raw)
        except json.JSONDecodeError as e:
            raise HTTPException(status_code=400, detail=f"malformed JSON: {e}")
        if not isinstance(data, dict):
            raise HTTPException(status_code=400, detail="request body must be a JSON object")
        return data

    def _lookup(body: dict) -> Provider:
        model = body.get("model")
        if not isinstance(model, str):
            raise HTTPException(status_code=400, detail="'model' field is required")
        try:
            return app.state.providers[model]
        except KeyError:
            raise HTTPException(status_code=404, detail=f"model {model!r} not configured")

    @app.post("/v1/chat/completions")
    async def chat_completions(request: Request):
        body = await _read_body(request)
        provider = _lookup(body)

        if body.get("stream"):
            async def gen() -> typing.AsyncIterator[bytes]:
                async for chunk in provider.chat_stream(body):
                    if await request.is_disconnected():
                        break
                    yield chunk

            return StreamingResponse(gen(), media_type="text/event-stream")

        return await provider.chat(body)

    @app.post("/v1/embeddings")
    async def embeddings(request: Request):
        body = await _read_body(request)
        provider = _lookup(body)
        return await provider.embeddings(body)

    @app.post("/rerank")
    async def rerank(request: Request):
        body = await _read_body(request)
        provider = _lookup(body)
        return await provider.rerank(body)

    @app.exception_handler(HTTPException)
    async def handle_http_exception(_: Request, exc: HTTPException) -> JSONResponse:
        type_map = {
            400: "invalid_request",
            401: "auth",
            404: "invalid_request",
            501: "invalid_request",
            502: "upstream",
            504: "upstream",
        }
        return _error(exc.status_code, str(exc.detail), type_map.get(exc.status_code, "internal"))

    @app.exception_handler(httpx.HTTPStatusError)
    async def handle_upstream_status(_: Request, exc: httpx.HTTPStatusError) -> JSONResponse:
        body_text = exc.response.text[:512] if exc.response is not None else ""
        return _error(502, f"upstream error: {exc.response.status_code}: {body_text}", "upstream")

    @app.exception_handler(httpx.TimeoutException)
    async def handle_upstream_timeout(_: Request, exc: httpx.TimeoutException) -> JSONResponse:
        return _error(504, "upstream timeout", "upstream")

    @app.exception_handler(KeyError)
    async def handle_missing_env(_: Request, exc: KeyError) -> JSONResponse:
        key = exc.args[0] if exc.args else "<unknown>"
        return _error(500, f"configuration error: {key}", "internal")

    @app.exception_handler(NotImplementedError)
    async def handle_not_implemented(_: Request, exc: NotImplementedError) -> JSONResponse:
        return _error(501, str(exc) or "not implemented", "invalid_request")

    return app


# Module-level app used by `uvicorn app.main:app`.
# Guard behind env var so test-time imports don't try to load a config file.
_config_path = os.environ.get("GATEWAY_CONFIG_PATH")
app: FastAPI | None = create_app(load_config(_config_path)) if _config_path else None
