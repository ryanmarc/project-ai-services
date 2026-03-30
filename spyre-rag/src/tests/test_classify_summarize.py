"""Tests for classify and summarize functions using litellm."""
import pytest
from unittest.mock import patch, MagicMock


class TestClassifySingleText:
    """Verify LLM-based text classification uses litellm.completion."""

    @patch("litellm.completion")
    def test_returns_true_for_yes(self, mock_completion):
        from common.llm_utils import classify_single_text

        mock_completion.return_value.choices = [
            MagicMock(message=MagicMock(content="Yes"))
        ]

        result = classify_single_text("classify this", "hosted_vllm/granite", "http://vllm:8000")
        assert result is True

    @patch("litellm.completion")
    def test_returns_false_for_no(self, mock_completion):
        from common.llm_utils import classify_single_text

        mock_completion.return_value.choices = [
            MagicMock(message=MagicMock(content="No"))
        ]

        result = classify_single_text("classify this", "hosted_vllm/granite", "http://vllm:8000")
        assert result is False

    @patch("litellm.completion")
    def test_passes_correct_kwargs(self, mock_completion):
        from common.llm_utils import classify_single_text

        mock_completion.return_value.choices = [
            MagicMock(message=MagicMock(content="no"))
        ]

        classify_single_text("prompt", "openai/gpt-4o", "")
        kwargs = mock_completion.call_args[1]

        assert kwargs["model"] == "openai/gpt-4o"
        assert kwargs["temperature"] == 0
        assert kwargs["max_tokens"] == 3
        assert kwargs["num_retries"] == 3
        assert "api_base" not in kwargs


class TestSummarizeSingleTable:
    """Verify table summarization uses litellm.completion."""

    @patch("litellm.completion")
    def test_returns_stripped_content(self, mock_completion):
        from common.llm_utils import summarize_single_table

        mock_completion.return_value.choices = [
            MagicMock(message=MagicMock(content="  Summary of table  "))
        ]

        result = summarize_single_table("summarize", "hosted_vllm/granite", "http://vllm:8000")
        assert result == "summary of table"

    @patch("litellm.completion")
    def test_passes_api_base_when_set(self, mock_completion):
        from common.llm_utils import summarize_single_table

        mock_completion.return_value.choices = [
            MagicMock(message=MagicMock(content="summary"))
        ]

        summarize_single_table("prompt", "model", "http://ep:8000")
        kwargs = mock_completion.call_args[1]
        assert kwargs["api_base"] == "http://ep:8000"
        assert kwargs["stream"] is False
        assert kwargs["max_tokens"] == 512


class TestQueryLlmSummarize:
    """Verify summarization endpoint function."""

    @patch("litellm.completion")
    def test_returns_content_and_tokens(self, mock_completion):
        from common.llm_utils import query_llm_summarize

        usage = MagicMock()
        usage.prompt_tokens = 100
        usage.completion_tokens = 50
        mock_completion.return_value.choices = [
            MagicMock(message=MagicMock(content="Summary result."))
        ]
        mock_completion.return_value.usage = usage

        content, in_tok, out_tok = query_llm_summarize(
            "http://vllm:8000", [{"role": "user", "content": "summarize"}],
            "hosted_vllm/granite", 256, 0.3
        )

        assert content == "Summary result."
        assert in_tok == 100
        assert out_tok == 50
