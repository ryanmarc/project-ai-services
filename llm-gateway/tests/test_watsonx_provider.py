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


async def test_chat_translates_request_and_response():
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.host == "iam.cloud.ibm.com":
            return httpx.Response(200, json={"access_token": "t", "expires_in": 3600})
        captured["url"] = str(request.url)
        captured["body"] = request.content
        captured["auth"] = request.headers.get("Authorization")
        return httpx.Response(
            200,
            json={
                "results": [
                    {
                        "generated_text": "hello world",
                        "input_token_count": 7,
                        "generated_token_count": 2,
                        "stop_reason": "eos_token",
                    }
                ]
            },
        )

    p = _make(httpx.MockTransport(handler))
    resp = await p.chat(
        {
            "model": "granite-wx",
            "messages": [{"role": "user", "content": "hi"}],
            "max_tokens": 64,
            "temperature": 0.2,
            "repetition_penalty": 1.1,  # should be dropped
        }
    )

    import json as _json

    assert "ml/v1/text/chat" in captured["url"]
    assert "version=2024-05-31" in captured["url"]
    assert captured["auth"] == "Bearer t"
    body = _json.loads(captured["body"])
    assert body["model_id"] == "ibm/granite-3-8b-instruct"
    assert body["project_id"] == "the-project"
    assert body["messages"] == [{"role": "user", "content": "hi"}]
    assert body["parameters"] == {"max_tokens": 64, "temperature": 0.2}

    assert resp["object"] == "chat.completion"
    assert resp["model"] == "granite-wx"
    assert resp["choices"][0]["message"] == {"role": "assistant", "content": "hello world"}
    assert resp["choices"][0]["finish_reason"] == "eos_token"
    assert resp["usage"] == {
        "prompt_tokens": 7,
        "completion_tokens": 2,
        "total_tokens": 9,
    }


async def test_chat_stream_translates_watsonx_sse_to_openai_chunks():
    wx_sse = (
        b'data: {"results":[{"generated_text":"hel"}]}\n\n'
        b'data: {"results":[{"generated_text":"lo"}]}\n\n'
        b'data: {"results":[{"generated_text":"","stop_reason":"eos_token"}]}\n\n'
    )

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.host == "iam.cloud.ibm.com":
            return httpx.Response(200, json={"access_token": "t", "expires_in": 3600})
        assert "ml/v1/text/chat_stream" in str(request.url)
        return httpx.Response(
            200,
            headers={"content-type": "text/event-stream"},
            stream=httpx.ByteStream(wx_sse),
        )

    p = _make(httpx.MockTransport(handler))
    chunks: list[bytes] = []
    async for c in p.chat_stream({"model": "granite-wx", "messages": [{"role": "user", "content": "hi"}]}):
        chunks.append(c)

    joined = b"".join(chunks)
    # Expect OpenAI-shaped chunk frames with content, finish_reason, then DONE.
    assert b'"delta": {"content": "hel"}' in joined
    assert b'"delta": {"content": "lo"}' in joined
    assert b'"finish_reason": "eos_token"' in joined
    assert joined.endswith(b"data: [DONE]\n\n")


class _ChunkedByteStream(httpx.AsyncByteStream):
    """Async byte stream that yields pre-determined chunks one at a time."""

    def __init__(self, chunks: list[bytes]):
        self._chunks = chunks

    async def __aiter__(self):
        for c in self._chunks:
            yield c

    async def aclose(self):
        return


async def test_chat_stream_handles_frame_split_across_reads():
    # Watsonx sends a single logical SSE frame, but TCP coalesces it into two
    # byte chunks that split the JSON payload mid-token.
    chunks = [
        b'data: {"results":[{"gen',
        b'erated_text":"hello","stop_reason":"eos_token"}]}\n\n',
    ]

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.host == "iam.cloud.ibm.com":
            return httpx.Response(200, json={"access_token": "t", "expires_in": 3600})
        return httpx.Response(
            200,
            headers={"content-type": "text/event-stream"},
            stream=_ChunkedByteStream(chunks),
        )

    p = _make(httpx.MockTransport(handler))
    out: list[bytes] = []
    async for c in p.chat_stream({"model": "granite-wx", "messages": [{"role": "user", "content": "hi"}]}):
        out.append(c)
    joined = b"".join(out)
    assert b'"content": "hello"' in joined
    assert b'"finish_reason": "eos_token"' in joined
    assert joined.endswith(b"data: [DONE]\n\n")


async def test_chat_stream_flushes_trailing_frame_without_final_blank_line():
    # Stream ends without a trailing \n\n — the buffer-flush path must still
    # emit the last frame.
    body_bytes = b'data: {"results":[{"generated_text":"bye","stop_reason":"eos_token"}]}\n'

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.host == "iam.cloud.ibm.com":
            return httpx.Response(200, json={"access_token": "t", "expires_in": 3600})
        return httpx.Response(
            200,
            headers={"content-type": "text/event-stream"},
            stream=httpx.ByteStream(body_bytes),
        )

    p = _make(httpx.MockTransport(handler))
    out: list[bytes] = []
    async for c in p.chat_stream({"model": "granite-wx", "messages": [{"role": "user", "content": "hi"}]}):
        out.append(c)
    joined = b"".join(out)
    assert b'"content": "bye"' in joined
    assert b'"finish_reason": "eos_token"' in joined
    assert joined.endswith(b"data: [DONE]\n\n")
