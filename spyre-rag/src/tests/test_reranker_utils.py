"""Tests for reranker_utils litellm integration."""
import pytest
from unittest.mock import patch, MagicMock
from chatbot.reranker_utils import rerank_helper, rerank_documents


class TestRerankHelper:
    """Verify single-document reranking via litellm."""

    @patch("litellm.rerank")
    def test_returns_doc_and_score(self, mock_rerank):
        result_obj = MagicMock()
        result_obj.results = [MagicMock(relevance_score=0.95)]
        mock_rerank.return_value = result_obj

        doc = {"page_content": "some text", "metadata": {}}
        returned_doc, score = rerank_helper("query", doc, "hosted_vllm/reranker", "http://reranker:8000")

        assert returned_doc is doc
        assert score == 0.95

    @patch("litellm.rerank")
    def test_passes_api_base(self, mock_rerank):
        result_obj = MagicMock()
        result_obj.results = [MagicMock(relevance_score=0.5)]
        mock_rerank.return_value = result_obj

        rerank_helper("q", {"page_content": "text"}, "model", "http://ep:8000")

        call_kwargs = mock_rerank.call_args[1]
        assert call_kwargs["api_base"] == "http://ep:8000"
        assert call_kwargs["query"] == "q"
        assert call_kwargs["documents"] == ["text"]

    @patch("litellm.rerank")
    def test_no_endpoint_omits_api_base(self, mock_rerank):
        result_obj = MagicMock()
        result_obj.results = [MagicMock(relevance_score=0.5)]
        mock_rerank.return_value = result_obj

        rerank_helper("q", {"page_content": "text"}, "cohere/rerank-v3", None)

        call_kwargs = mock_rerank.call_args[1]
        assert "api_base" not in call_kwargs

    def test_empty_page_content_returns_zero_score(self):
        doc = {"page_content": ""}
        returned_doc, score = rerank_helper("query", doc, "model")
        assert score == 0.0


class TestRerankDocuments:
    """Verify batch reranking with sorting."""

    @patch("chatbot.reranker_utils.rerank_helper")
    def test_returns_sorted_by_score_descending(self, mock_helper):
        docs = [
            {"page_content": "low"},
            {"page_content": "high"},
            {"page_content": "mid"},
        ]

        def side_effect(query, doc, model, endpoint):
            scores = {"low": 0.1, "high": 0.9, "mid": 0.5}
            return doc, scores[doc["page_content"]]

        mock_helper.side_effect = side_effect

        result = rerank_documents("query", docs, "model", "http://ep:8000", max_workers=2)

        assert len(result) == 3
        assert result[0][1] == 0.9
        assert result[1][1] == 0.5
        assert result[2][1] == 0.1

    @patch("chatbot.reranker_utils.rerank_helper")
    def test_thread_error_assigns_zero_score(self, mock_helper):
        docs = [{"page_content": "ok"}, {"page_content": "fail"}]

        def side_effect(query, doc, model, endpoint):
            if doc["page_content"] == "fail":
                raise RuntimeError("thread error")
            return doc, 0.8

        mock_helper.side_effect = side_effect

        result = rerank_documents("query", docs, "model", "http://ep:8000")
        scores = [s for _, s in result]
        assert 0.0 in scores
        assert 0.8 in scores
