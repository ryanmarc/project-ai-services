"""Tests for emb_utils litellm integration."""
import numpy as np
import pytest
from unittest.mock import patch, MagicMock
from common.emb_utils import Embedding


class TestEmbedding:
    """Verify Embedding class uses litellm.embedding correctly."""

    @patch("litellm.embedding")
    def test_embed_query_returns_numpy_array(self, mock_emb):
        mock_emb.return_value.data = [{"embedding": [0.1, 0.2, 0.3]}]

        emb = Embedding("hosted_vllm/bge-base", "http://emb:8000", 512)
        result = emb.embed_query("hello")

        assert isinstance(result, np.ndarray)
        assert result.dtype == np.float32
        np.testing.assert_array_almost_equal(result, [0.1, 0.2, 0.3])

    @patch("litellm.embedding")
    def test_embed_documents_returns_list_of_arrays(self, mock_emb):
        mock_emb.return_value.data = [
            {"embedding": [0.1, 0.2]},
            {"embedding": [0.3, 0.4]},
        ]

        emb = Embedding("hosted_vllm/bge-base", "http://emb:8000", 512)
        result = emb.embed_documents(["hello", "world"])

        assert len(result) == 2
        assert all(isinstance(r, np.ndarray) for r in result)

    @patch("litellm.embedding")
    def test_passes_api_base_when_endpoint_set(self, mock_emb):
        mock_emb.return_value.data = [{"embedding": [0.1]}]

        emb = Embedding("hosted_vllm/bge-base", "http://emb:8000", 512)
        emb.embed_query("test")

        call_kwargs = mock_emb.call_args[1]
        assert call_kwargs["api_base"] == "http://emb:8000"
        assert call_kwargs["model"] == "hosted_vllm/bge-base"

    @patch("litellm.embedding")
    def test_omits_api_base_when_no_endpoint(self, mock_emb):
        mock_emb.return_value.data = [{"embedding": [0.1]}]

        emb = Embedding("openai/text-embedding-3-small", None, 8191)
        emb.embed_query("test")

        call_kwargs = mock_emb.call_args[1]
        assert "api_base" not in call_kwargs

    @patch("litellm.embedding")
    def test_truncate_prompt_tokens(self, mock_emb):
        mock_emb.return_value.data = [{"embedding": [0.1]}]

        emb = Embedding("hosted_vllm/bge-base", "http://emb:8000", 512)
        emb.embed_query("test")

        call_kwargs = mock_emb.call_args[1]
        assert call_kwargs["truncate_prompt_tokens"] == 511  # max_tokens - 1

    @patch("litellm.embedding")
    def test_num_retries_set(self, mock_emb):
        mock_emb.return_value.data = [{"embedding": [0.1]}]

        emb = Embedding("hosted_vllm/bge-base", "http://emb:8000", 512)
        emb.embed_query("test")

        call_kwargs = mock_emb.call_args[1]
        assert call_kwargs["num_retries"] == 3
