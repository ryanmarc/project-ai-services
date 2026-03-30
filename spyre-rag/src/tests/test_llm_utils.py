"""Tests for llm_utils litellm integration."""
import json
import pytest
from unittest.mock import patch, MagicMock


class TestBuildCompletionKwargs:
    """Verify build_completion_kwargs produces correct litellm kwargs."""

    @patch("common.llm_utils.detokenize_with_llm", return_value="truncated context")
    @patch("common.llm_utils.tokenize_with_llm", return_value=list(range(10)))
    def test_basic_kwargs_structure(self, mock_tok, mock_detok):
        from common.llm_utils import build_completion_kwargs

        docs = [{"page_content": "test doc"}]
        kwargs = build_completion_kwargs(
            "what is AI?", docs, "hosted_vllm/granite", "http://vllm:8000",
            ["</s>"], 512, 0.7, False, "en"
        )

        assert kwargs["model"] == "hosted_vllm/granite"
        assert kwargs["api_base"] == "http://vllm:8000"
        assert kwargs["max_tokens"] == 512
        assert kwargs["temperature"] == 0.7
        assert kwargs["stop"] == ["</s>"]
        assert kwargs["stream"] is False
        assert kwargs["num_retries"] == 3
        assert "stream_options" not in kwargs

    @patch("common.llm_utils.detokenize_with_llm", return_value="ctx")
    @patch("common.llm_utils.tokenize_with_llm", return_value=list(range(5)))
    def test_streaming_includes_stream_options(self, mock_tok, mock_detok):
        from common.llm_utils import build_completion_kwargs

        docs = [{"page_content": "doc"}]
        kwargs = build_completion_kwargs(
            "q", docs, "hosted_vllm/granite", "http://vllm:8000",
            None, 256, 0.5, True, "en"
        )

        assert kwargs["stream"] is True
        assert kwargs["stream_options"] == {"include_usage": True}

    @patch("common.llm_utils.detokenize_with_llm", return_value="ctx")
    @patch("common.llm_utils.tokenize_with_llm", return_value=list(range(5)))
    def test_hosted_vllm_gets_repetition_penalty(self, mock_tok, mock_detok):
        from common.llm_utils import build_completion_kwargs

        docs = [{"page_content": "doc"}]
        kwargs = build_completion_kwargs(
            "q", docs, "hosted_vllm/granite", "http://vllm:8000",
            None, 256, 0.5, False, "en"
        )
        assert kwargs["repetition_penalty"] == 1.1

    @patch("common.llm_utils.detokenize_with_llm", return_value="ctx")
    @patch("common.llm_utils.tokenize_with_llm", return_value=list(range(5)))
    def test_openai_model_no_repetition_penalty(self, mock_tok, mock_detok):
        from common.llm_utils import build_completion_kwargs

        docs = [{"page_content": "doc"}]
        kwargs = build_completion_kwargs(
            "q", docs, "openai/gpt-4o", "",
            None, 256, 0.5, False, "en"
        )
        assert "repetition_penalty" not in kwargs

    @patch("common.llm_utils.detokenize_with_llm", return_value="ctx")
    @patch("common.llm_utils.tokenize_with_llm", return_value=list(range(5)))
    def test_no_endpoint_omits_api_base(self, mock_tok, mock_detok):
        from common.llm_utils import build_completion_kwargs

        docs = [{"page_content": "doc"}]
        kwargs = build_completion_kwargs(
            "q", docs, "openai/gpt-4o", "",
            None, 256, 0.5, False, "en"
        )
        assert "api_base" not in kwargs


class TestStreamLitellmToSSE:
    """Verify SSE conversion from litellm streaming response."""

    def _make_chunk(self, content=None, usage=None):
        chunk = MagicMock()
        if content:
            choice = MagicMock()
            choice.delta.content = content
            chunk.choices = [choice]
        else:
            chunk.choices = []
        chunk.usage = usage
        chunk.model_dump.return_value = {"choices": [{"delta": {"content": content}}]} if content else {}
        return chunk

    def test_yields_sse_lines(self):
        from common.llm_utils import _stream_litellm_to_sse

        chunks = [self._make_chunk("hello"), self._make_chunk(" world")]
        lines = list(_stream_litellm_to_sse(chunks))

        assert any("hello" in line for line in lines)
        assert any("world" in line for line in lines)
        assert lines[-1] == "data: [DONE]\n\n"

    def test_tracks_perf_stats(self):
        from common.llm_utils import _stream_litellm_to_sse

        usage = MagicMock()
        usage.completion_tokens = 10
        usage.prompt_tokens = 20
        chunks = [self._make_chunk("hi"), self._make_chunk(usage=usage)]

        perf = {}
        list(_stream_litellm_to_sse(chunks, perf))

        assert perf["completion_tokens"] == 10
        assert perf["prompt_tokens"] == 20
        assert "token_latencies" in perf
        assert "inference_time" in perf

    def test_error_yields_error_sse(self):
        from common.llm_utils import _stream_litellm_to_sse

        def exploding():
            yield self._make_chunk("ok")
            raise RuntimeError("boom")

        lines = list(_stream_litellm_to_sse(exploding()))
        error_lines = [l for l in lines if "error" in l]
        assert len(error_lines) == 1
        assert "boom" in error_lines[0]
        assert lines[-1] == "data: [DONE]\n\n"


class TestTokenizeHybridPath:
    """Verify tokenize/detokenize uses vLLM endpoint when available, litellm otherwise."""

    @patch("common.llm_utils._get_tokenize_session")
    def test_tokenize_with_endpoint_uses_vllm(self, mock_session):
        from common.llm_utils import tokenize_with_llm

        resp = MagicMock()
        resp.json.return_value = {"tokens": [1, 2, 3]}
        resp.raise_for_status = MagicMock()
        mock_session.return_value.post.return_value = resp

        result = tokenize_with_llm("hello", "hosted_vllm/model", "http://vllm:8000")
        assert result == [1, 2, 3]
        mock_session.return_value.post.assert_called_once_with(
            "http://vllm:8000/tokenize", json={"prompt": "hello"}
        )

    @patch("litellm.encode", return_value=[10, 20, 30])
    def test_tokenize_without_endpoint_uses_litellm(self, mock_encode):
        from common.llm_utils import tokenize_with_llm

        result = tokenize_with_llm("hello", "openai/gpt-4o", None)
        assert result == [10, 20, 30]
        mock_encode.assert_called_once_with(model="openai/gpt-4o", text="hello")

    @patch("common.llm_utils._get_tokenize_session")
    def test_detokenize_with_endpoint_uses_vllm(self, mock_session):
        from common.llm_utils import detokenize_with_llm

        resp = MagicMock()
        resp.json.return_value = {"prompt": "hello world"}
        resp.raise_for_status = MagicMock()
        mock_session.return_value.post.return_value = resp

        result = detokenize_with_llm([1, 2], "hosted_vllm/model", "http://vllm:8000")
        assert result == "hello world"

    @patch("litellm.decode", return_value="hello world")
    def test_detokenize_without_endpoint_uses_litellm(self, mock_decode):
        from common.llm_utils import detokenize_with_llm

        result = detokenize_with_llm([1, 2], "openai/gpt-4o", None)
        assert result == "hello world"
        mock_decode.assert_called_once_with(model="openai/gpt-4o", tokens=[1, 2])


class TestQueryLlmNonStream:
    """Verify non-streaming completion path."""

    @patch("common.llm_utils.build_completion_kwargs")
    @patch("litellm.completion")
    def test_returns_model_dump(self, mock_completion, mock_build):
        from common.llm_utils import query_llm_non_stream

        mock_build.return_value = {"model": "m", "messages": [], "stream": False}
        usage = MagicMock()
        usage.completion_tokens = 5
        usage.prompt_tokens = 10
        mock_completion.return_value.usage = usage
        mock_completion.return_value.model_dump.return_value = {
            "choices": [{"message": {"content": "answer"}}]
        }

        perf = {}
        result = query_llm_non_stream("q", [], "ep", "m", None, 100, 0.5, perf, "en")

        assert result["choices"][0]["message"]["content"] == "answer"
        assert perf["completion_tokens"] == 5
        assert perf["prompt_tokens"] == 10
        assert "inference_time" in perf


class TestQueryLlmStream:
    """Verify streaming completion path."""

    @patch("common.llm_utils.build_completion_kwargs")
    @patch("litellm.completion")
    def test_yields_sse_lines(self, mock_completion, mock_build):
        from common.llm_utils import query_llm_stream

        mock_build.return_value = {"model": "m", "messages": [], "stream": True}

        chunk = MagicMock()
        chunk.choices = [MagicMock()]
        chunk.choices[0].delta.content = "hi"
        chunk.usage = None
        chunk.model_dump.return_value = {"choices": [{"delta": {"content": "hi"}}]}
        mock_completion.return_value = iter([chunk])

        perf = {}
        lines = list(query_llm_stream("q", [], "ep", "m", None, 100, 0.5, perf, "en"))

        assert any("hi" in l for l in lines)
        assert lines[-1] == "data: [DONE]\n\n"
