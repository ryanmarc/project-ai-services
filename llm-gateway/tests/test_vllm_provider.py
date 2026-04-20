import json

import httpx
import pytest

from app.config import ModelEntry
from app.providers.vllm import VLLMProvider


def _entry(api_base: str = "http://vllm.example:8000/v1") -> ModelEntry:
    return ModelEntry(
        model_name="granite",
        params={
            "model": "openai/ibm-granite/granite-3.3-8b-instruct",
            "api_base": api_base,
            "api_key": "fake-key",
        },
    )


def _make_provider(transport: httpx.MockTransport, entry: ModelEntry | None = None) -> VLLMProvider:
    p = VLLMProvider(entry or _entry(), timeout=5.0)
    p._transport = transport  # test hook
    return p


async def test_chat_rewrites_model_and_forwards():
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["url"] = str(request.url)
        captured["body"] = json.loads(request.content)
        return httpx.Response(200, json={"id": "x", "choices": []})

    provider = _make_provider(httpx.MockTransport(handler))
    result = await provider.chat(
        {"model": "granite", "messages": [{"role": "user", "content": "hi"}]}
    )

    assert captured["url"] == "http://vllm.example:8000/v1/chat/completions"
    assert captured["body"]["model"] == "ibm-granite/granite-3.3-8b-instruct"
    assert captured["body"]["messages"] == [{"role": "user", "content": "hi"}]
    assert result == {"id": "x", "choices": []}


async def test_chat_forwards_upstream_error_status():
    def handler(request):
        return httpx.Response(502, json={"error": "upstream bad"})

    provider = _make_provider(httpx.MockTransport(handler))
    with pytest.raises(httpx.HTTPStatusError) as excinfo:
        await provider.chat({"model": "granite", "messages": []})
    assert excinfo.value.response.status_code == 502


async def test_chat_stream_yields_chunks_unchanged():
    frames = [
        b'data: {"choices":[{"delta":{"content":"he"}}]}\n\n',
        b'data: {"choices":[{"delta":{"content":"llo"}}]}\n\n',
        b"data: [DONE]\n\n",
    ]

    def handler(request):
        return httpx.Response(
            200,
            headers={"content-type": "text/event-stream"},
            stream=httpx.ByteStream(b"".join(frames)),
        )

    provider = _make_provider(httpx.MockTransport(handler))
    out = []
    async for chunk in provider.chat_stream({"model": "granite", "messages": []}):
        out.append(chunk)
    joined = b"".join(out)
    for frame in frames:
        assert frame in joined


async def test_embeddings_rewrites_model_and_forwards():
    captured = {}

    def handler(request):
        captured["url"] = str(request.url)
        captured["body"] = json.loads(request.content)
        return httpx.Response(200, json={"data": [{"embedding": [0.1]}]})

    entry = ModelEntry(
        model_name="emb",
        params={
            "model": "openai/ibm-granite/granite-embedding-278m-multilingual",
            "api_base": "http://vllm.example:8080/v1",
            "api_key": "fake-key",
        },
    )
    provider = _make_provider(httpx.MockTransport(handler), entry=entry)

    result = await provider.embeddings({"model": "emb", "input": "hi"})

    assert captured["url"] == "http://vllm.example:8080/v1/embeddings"
    assert captured["body"]["model"] == "ibm-granite/granite-embedding-278m-multilingual"
    assert result["data"][0]["embedding"] == [0.1]


async def test_rerank_posts_to_rerank_path():
    captured = {}

    def handler(request):
        captured["url"] = str(request.url)
        captured["body"] = json.loads(request.content)
        return httpx.Response(200, json={"results": []})

    provider = _make_provider(httpx.MockTransport(handler))
    await provider.rerank({"model": "granite", "query": "q", "documents": ["d1"]})

    assert captured["url"].endswith("/rerank")
    assert captured["body"]["query"] == "q"
