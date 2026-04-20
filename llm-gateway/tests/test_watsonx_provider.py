import time

import httpx
import pytest

from app.config import ModelEntry
from app.providers.watsonx import WatsonxProvider


def _entry(**overrides) -> ModelEntry:
    params = {
        "model": "watsonx/ibm/granite-3-8b-instruct",
        "api_key": "the-apikey",
        "watsonx_project_id": "the-project",
        "api_base": "https://wx.example.com",
    }
    params.update(overrides)
    return ModelEntry(model_name="granite-wx", params=params)


def _make(transport: httpx.MockTransport, entry: ModelEntry | None = None) -> WatsonxProvider:
    p = WatsonxProvider(entry or _entry(), timeout=5.0)
    p._transport = transport
    return p


class _TokenServer:
    """Counts IAM token fetches and returns configurable expiry."""

    def __init__(self, expires_in: int = 3600):
        self.expires_in = expires_in
        self.fetches = 0

    def handle(self, request: httpx.Request) -> httpx.Response:
        assert request.url.host == "iam.cloud.ibm.com"
        assert request.url.path == "/identity/token"
        self.fetches += 1
        return httpx.Response(
            200,
            json={
                "access_token": f"token-{self.fetches}",
                "expires_in": self.expires_in,
            },
        )


async def test_iam_token_fetched_on_first_use():
    server = _TokenServer()
    p = _make(httpx.MockTransport(server.handle))
    token = await p._get_bearer()
    assert token == "token-1"
    assert server.fetches == 1


async def test_iam_token_cached_within_expiry():
    server = _TokenServer(expires_in=3600)
    p = _make(httpx.MockTransport(server.handle))
    await p._get_bearer()
    await p._get_bearer()
    await p._get_bearer()
    assert server.fetches == 1


async def test_iam_token_refreshed_when_near_expiry(monkeypatch):
    server = _TokenServer(expires_in=60)  # within the 60s buffer
    p = _make(httpx.MockTransport(server.handle))
    await p._get_bearer()
    # Advance time beyond buffer; capture real time.time before patching to
    # avoid infinite recursion in the lambda.
    _real_time = time.time
    monkeypatch.setattr(time, "time", lambda: _real_time() + 61)
    await p._get_bearer()
    assert server.fetches == 2


async def test_zen_api_key_skips_iam_fetch():
    server = _TokenServer()
    entry = _entry(api_key_type="zen", api_key="zenkey123")
    p = _make(httpx.MockTransport(server.handle), entry=entry)
    token = await p._get_bearer()
    assert token == "zenkey123"
    assert server.fetches == 0


async def test_iam_token_surfaces_error_response():
    def handler(request):
        return httpx.Response(401, json={"errorMessage": "Invalid apikey"})

    p = _make(httpx.MockTransport(handler))
    with pytest.raises(httpx.HTTPStatusError):
        await p._get_bearer()
