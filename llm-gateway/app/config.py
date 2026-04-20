from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path

import yaml

_ENV_PREFIX = "os.environ/"


@dataclass(frozen=True)
class ModelEntry:
    model_name: str
    params: dict


@dataclass(frozen=True)
class GatewayConfig:
    models: dict[str, ModelEntry]
    master_key_ref: str | None
    request_timeout: float = 120.0
    extras: dict = field(default_factory=dict)


def resolve_env_ref(value):
    """Resolve a possible os.environ/NAME reference.

    Passes literal strings through unchanged. For references, looks up the env
    var and raises KeyError if it is unset or empty.
    """
    if not isinstance(value, str) or not value.startswith(_ENV_PREFIX):
        return value
    name = value[len(_ENV_PREFIX):]
    resolved = os.environ.get(name)
    if not resolved:
        raise KeyError(f"env var {name!r} is referenced by config but not set")
    return resolved


def load_config(path: str | Path) -> GatewayConfig:
    raw = yaml.safe_load(Path(path).read_text())

    models: dict[str, ModelEntry] = {}
    for entry in raw.get("model_list", []):
        name = entry["model_name"]
        params = entry["params"]
        models[name] = ModelEntry(model_name=name, params=params)

    general = raw.get("general_settings") or {}
    master_key_ref = general.get("master_key")

    settings = raw.get("settings") or {}
    request_timeout = float(settings.get("request_timeout", 120))

    return GatewayConfig(
        models=models,
        master_key_ref=master_key_ref,
        request_timeout=request_timeout,
    )
