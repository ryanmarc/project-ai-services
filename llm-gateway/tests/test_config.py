import os
from pathlib import Path

import pytest

from app.config import (
    GatewayConfig,
    ModelEntry,
    load_config,
    resolve_env_ref,
)


CONFIG_YAML = """
model_list:
  - model_name: granite-3.3-8b-instruct
    params:
      model: openai/ibm-granite/granite-3.3-8b-instruct
      api_base: os.environ/VLLM_INSTRUCT_URL
      api_key: "fake-key"
  - model_name: granite-3-8b-instruct-wx
    params:
      model: watsonx/ibm/granite-3-8b-instruct
      api_key: os.environ/WATSONX_API_KEY
      watsonx_project_id: os.environ/WATSONX_PROJECT_ID

general_settings:
  master_key: os.environ/GATEWAY_MASTER_KEY

settings:
  request_timeout: 90
"""


def _write(tmp_path: Path, content: str) -> Path:
    p = tmp_path / "gateway_config.yaml"
    p.write_text(content)
    return p


def test_load_config_parses_models(tmp_path):
    cfg = load_config(_write(tmp_path, CONFIG_YAML))

    assert isinstance(cfg, GatewayConfig)
    assert set(cfg.models) == {
        "granite-3.3-8b-instruct",
        "granite-3-8b-instruct-wx",
    }
    entry = cfg.models["granite-3.3-8b-instruct"]
    assert isinstance(entry, ModelEntry)
    assert entry.params["model"] == "openai/ibm-granite/granite-3.3-8b-instruct"


def test_load_config_request_timeout_default(tmp_path):
    content = CONFIG_YAML.replace("request_timeout: 90", "").replace("settings:\n  \n", "")
    cfg = load_config(_write(tmp_path, content))
    assert cfg.request_timeout == 120.0


def test_load_config_request_timeout_override(tmp_path):
    cfg = load_config(_write(tmp_path, CONFIG_YAML))
    assert cfg.request_timeout == 90.0


def test_load_config_master_key_is_raw_string(tmp_path):
    cfg = load_config(_write(tmp_path, CONFIG_YAML))
    assert cfg.master_key_ref == "os.environ/GATEWAY_MASTER_KEY"


def test_load_config_master_key_optional(tmp_path):
    content = CONFIG_YAML.replace(
        "general_settings:\n  master_key: os.environ/GATEWAY_MASTER_KEY",
        "general_settings: {}",
    )
    cfg = load_config(_write(tmp_path, content))
    assert cfg.master_key_ref is None


def test_resolve_env_ref_passes_through_literal():
    assert resolve_env_ref("fake-key") == "fake-key"


def test_resolve_env_ref_substitutes_when_set(monkeypatch):
    monkeypatch.setenv("TEST_VAR", "hello")
    assert resolve_env_ref("os.environ/TEST_VAR") == "hello"


def test_resolve_env_ref_raises_when_unset(monkeypatch):
    monkeypatch.delenv("TEST_VAR", raising=False)
    with pytest.raises(KeyError, match="TEST_VAR"):
        resolve_env_ref("os.environ/TEST_VAR")


def test_resolve_env_ref_empty_string_counts_as_unset(monkeypatch):
    monkeypatch.setenv("TEST_VAR", "")
    with pytest.raises(KeyError, match="TEST_VAR"):
        resolve_env_ref("os.environ/TEST_VAR")
