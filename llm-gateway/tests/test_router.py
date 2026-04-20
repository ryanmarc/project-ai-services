import pytest

from app.config import GatewayConfig, ModelEntry
from app.providers.vllm import VLLMProvider
from app.providers.watsonx import WatsonxProvider
from app.router import build_providers


def _cfg(model_name: str, upstream: str, **extra_params) -> GatewayConfig:
    return GatewayConfig(
        models={
            model_name: ModelEntry(
                model_name=model_name,
                params={"model": upstream, **extra_params},
            )
        },
        master_key_ref=None,
        request_timeout=30.0,
    )


def test_router_builds_vllm_provider_for_openai_prefix():
    cfg = _cfg(
        "granite",
        "openai/ibm-granite/granite-3.3-8b-instruct",
        api_base="http://example:8000/v1",
        api_key="fake-key",
    )
    providers = build_providers(cfg)
    assert set(providers) == {"granite"}
    assert isinstance(providers["granite"], VLLMProvider)


def test_router_builds_watsonx_provider_for_watsonx_prefix():
    cfg = _cfg(
        "granite-wx",
        "watsonx/ibm/granite-3-8b-instruct",
        api_key="fake-key",
        watsonx_project_id="pid",
    )
    providers = build_providers(cfg)
    assert isinstance(providers["granite-wx"], WatsonxProvider)


def test_router_rejects_unknown_prefix():
    cfg = _cfg("bogus", "openrouter/foo")
    with pytest.raises(ValueError, match="bogus"):
        build_providers(cfg)
