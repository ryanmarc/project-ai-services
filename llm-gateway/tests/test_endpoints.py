from __future__ import annotations

import os

import httpx
import pytest
from fastapi.testclient import TestClient

from app.config import GatewayConfig, ModelEntry
from app.main import create_app


class _FakeProvider:
    def __init__(self, chat_response=None, stream_frames=None):
        self.chat_response = chat_response or {"id": "x", "choices": [{"message": {"content": "hi"}}]}
        self.stream_frames = stream_frames or [b"data: {\"a\":1}\n\n", b"data: [DONE]\n\n"]
        self.last_body = None

    async def chat(self, body):
        self.last_body = body
        return self.chat_response

    async def chat_stream(self, body):
        self.last_body = body
        for frame in self.stream_frames:
            yield frame

    async def embeddings(self, body):
        self.last_body = body
        return {"object": "list", "data": []}

    async def rerank(self, body):
        self.last_body = body
        return {"results": []}


def _client(providers: dict, master_key_ref: str | None = None) -> TestClient:
    cfg = GatewayConfig(
        models={
            name: ModelEntry(model_name=name, params={"model": "openai/x"})
            for name in providers
        },
        master_key_ref=master_key_ref,
        request_timeout=5.0,
    )
    app = create_app(cfg, providers=providers)
    return TestClient(app)


def test_health_returns_200_without_auth():
    client = _client({"m": _FakeProvider()})
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json() == {"status": "ok"}


def test_v1_models_lists_configured_aliases():
    client = _client({"granite": _FakeProvider(), "granite-wx": _FakeProvider()})
    r = client.get("/v1/models")
    assert r.status_code == 200
    ids = {m["id"] for m in r.json()["data"]}
    assert ids == {"granite", "granite-wx"}


def test_chat_dispatches_to_provider():
    provider = _FakeProvider()
    client = _client({"granite": provider})
    r = client.post(
        "/v1/chat/completions",
        json={"model": "granite", "messages": [{"role": "user", "content": "hi"}]},
    )
    assert r.status_code == 200
    assert provider.last_body["model"] == "granite"


def test_unknown_model_returns_openai_shape_404():
    client = _client({"granite": _FakeProvider()})
    r = client.post("/v1/chat/completions", json={"model": "nope", "messages": []})
    assert r.status_code == 404
    assert r.json()["error"]["type"] == "invalid_request"
    assert "nope" in r.json()["error"]["message"]


def test_missing_model_field_returns_400():
    client = _client({"granite": _FakeProvider()})
    r = client.post("/v1/chat/completions", json={"messages": []})
    assert r.status_code == 400
    assert r.json()["error"]["type"] == "invalid_request"


def test_malformed_json_returns_400():
    client = _client({"granite": _FakeProvider()})
    r = client.post(
        "/v1/chat/completions",
        content=b"not-json",
        headers={"content-type": "application/json"},
    )
    assert r.status_code == 400


def test_auth_enforced_when_master_key_set(monkeypatch):
    monkeypatch.setenv("GATEWAY_MASTER_KEY", "sekret")
    client = _client({"granite": _FakeProvider()}, master_key_ref="os.environ/GATEWAY_MASTER_KEY")

    r = client.post("/v1/chat/completions", json={"model": "granite", "messages": []})
    assert r.status_code == 401

    r = client.post(
        "/v1/chat/completions",
        json={"model": "granite", "messages": []},
        headers={"Authorization": "Bearer sekret"},
    )
    assert r.status_code == 200


def test_auth_skipped_when_master_key_absent():
    client = _client({"granite": _FakeProvider()}, master_key_ref=None)
    r = client.post("/v1/chat/completions", json={"model": "granite", "messages": []})
    assert r.status_code == 200


def test_auth_skipped_for_health_even_when_key_set(monkeypatch):
    monkeypatch.setenv("GATEWAY_MASTER_KEY", "sekret")
    client = _client({"granite": _FakeProvider()}, master_key_ref="os.environ/GATEWAY_MASTER_KEY")
    assert client.get("/health").status_code == 200


def test_chat_stream_forwards_frames_and_sets_content_type():
    provider = _FakeProvider(
        stream_frames=[
            b'data: {"delta":{"content":"a"}}\n\n',
            b'data: {"delta":{"content":"b"}}\n\n',
            b"data: [DONE]\n\n",
        ]
    )
    client = _client({"granite": provider})
    with client.stream(
        "POST",
        "/v1/chat/completions",
        json={"model": "granite", "messages": [], "stream": True},
    ) as r:
        assert r.status_code == 200
        assert r.headers["content-type"].startswith("text/event-stream")
        joined = b"".join(r.iter_raw())
    assert b'"a"' in joined
    assert b"[DONE]" in joined


def test_upstream_http_error_becomes_502():
    class BadProvider:
        async def chat(self, body):
            raise httpx.HTTPStatusError(
                "upstream boom",
                request=httpx.Request("POST", "http://x"),
                response=httpx.Response(503, json={"oops": True}),
            )

        async def chat_stream(self, body):  # pragma: no cover
            yield b""

        async def embeddings(self, body):  # pragma: no cover
            return {}

        async def rerank(self, body):  # pragma: no cover
            return {}

    client = _client({"m": BadProvider()})
    r = client.post("/v1/chat/completions", json={"model": "m", "messages": []})
    assert r.status_code == 502
    assert r.json()["error"]["type"] == "upstream"


def test_lifespan_calls_aclose_on_providers():
    class ClosableProvider(_FakeProvider):
        closed = False

        async def aclose(self):
            type(self).closed = True

    provider = ClosableProvider()
    cfg = GatewayConfig(
        models={"m": ModelEntry(model_name="m", params={"model": "openai/x"})},
        master_key_ref=None,
        request_timeout=5.0,
    )
    app = create_app(cfg, providers={"m": provider})
    with TestClient(app) as client:
        assert client.get("/health").status_code == 200
    assert ClosableProvider.closed is True


def test_watsonx_rerank_notimplemented_becomes_501():
    class WxLike:
        async def chat(self, body):  # pragma: no cover
            return {}

        async def chat_stream(self, body):  # pragma: no cover
            yield b""

        async def embeddings(self, body):  # pragma: no cover
            return {}

        async def rerank(self, body):
            raise NotImplementedError("rerank not supported")

    client = _client({"m": WxLike()})
    r = client.post("/rerank", json={"model": "m"})
    assert r.status_code == 501
