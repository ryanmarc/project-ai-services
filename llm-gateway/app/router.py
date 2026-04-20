from __future__ import annotations

from app.config import GatewayConfig
from app.providers.base import Provider
from app.providers.vllm import VLLMProvider
from app.providers.watsonx import WatsonxProvider


def build_providers(cfg: GatewayConfig) -> dict[str, Provider]:
    providers: dict[str, Provider] = {}
    for name, entry in cfg.models.items():
        upstream = entry.params["model"]
        if upstream.startswith("openai/"):
            providers[name] = VLLMProvider(entry, timeout=cfg.request_timeout)
        elif upstream.startswith("watsonx/"):
            providers[name] = WatsonxProvider(entry, timeout=cfg.request_timeout)
        else:
            raise ValueError(
                f"model {name!r} uses unsupported provider prefix in {upstream!r}"
            )
    return providers
