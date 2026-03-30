"""Tests for _auto_prefix_model in misc_utils."""
import pytest
from common.misc_utils import _auto_prefix_model, LITELLM_PROVIDERS


class TestAutoPrefix:
    """Verify model strings get the right litellm provider prefix."""

    def test_bare_model_with_endpoint_gets_hosted_vllm(self):
        result = _auto_prefix_model("ibm-granite/granite-3.3-8b-instruct", "http://vllm:8000")
        assert result == "hosted_vllm/ibm-granite/granite-3.3-8b-instruct"

    def test_already_prefixed_hosted_vllm_unchanged(self):
        result = _auto_prefix_model("hosted_vllm/ibm-granite/granite-3.3-8b-instruct", "http://vllm:8000")
        assert result == "hosted_vllm/ibm-granite/granite-3.3-8b-instruct"

    def test_openai_model_unchanged(self):
        result = _auto_prefix_model("openai/gpt-4o", "")
        assert result == "openai/gpt-4o"

    def test_anthropic_model_unchanged(self):
        result = _auto_prefix_model("anthropic/claude-sonnet-4-20250514", None)
        assert result == "anthropic/claude-sonnet-4-20250514"

    def test_no_endpoint_no_prefix(self):
        result = _auto_prefix_model("ibm-granite/granite-3.3-8b-instruct", "")
        assert result == "ibm-granite/granite-3.3-8b-instruct"

    def test_no_endpoint_none_no_prefix(self):
        result = _auto_prefix_model("ibm-granite/granite-3.3-8b-instruct", None)
        assert result == "ibm-granite/granite-3.3-8b-instruct"

    def test_empty_model_returns_empty(self):
        assert _auto_prefix_model("", "http://vllm:8000") == ""

    def test_none_model_returns_none(self):
        assert _auto_prefix_model(None, "http://vllm:8000") is None

    @pytest.mark.parametrize("provider", sorted(LITELLM_PROVIDERS))
    def test_all_known_providers_detected(self, provider):
        model = f"{provider}/some-model"
        result = _auto_prefix_model(model, "http://endpoint:8000")
        assert result == model, f"Provider '{provider}' should be detected and left unchanged"

    def test_unknown_prefix_with_endpoint_gets_hosted_vllm(self):
        result = _auto_prefix_model("my-custom-org/my-model", "http://vllm:8000")
        assert result == "hosted_vllm/my-custom-org/my-model"

    def test_simple_model_name_with_endpoint(self):
        result = _auto_prefix_model("llama3", "http://vllm:8000")
        assert result == "hosted_vllm/llama3"

    def test_simple_model_name_without_endpoint(self):
        result = _auto_prefix_model("llama3", "")
        assert result == "llama3"
