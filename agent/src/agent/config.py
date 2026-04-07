"""Environment-driven settings for the AI agent."""

import json
import logging
from pathlib import Path
from typing import Literal, Optional

from pydantic import Field
from pydantic_settings import BaseSettings

logger = logging.getLogger(__name__)


class RuntimeConfig:
    """Runtime configuration loaded from mounted volume."""

    def __init__(self, config_path: str = "/runtime/config.json"):
        self.host_ip: Optional[str] = None
        self.ports: dict[str, str] = {}
        self.pod_name: Optional[str] = None
        self.pod_id: Optional[str] = None
        self.app_name: Optional[str] = None
        self.created: Optional[str] = None

        self._load_config(config_path)

    def _load_config(self, config_path: str) -> None:
        """Load runtime config from JSON file if it exists."""
        path = Path(config_path)
        if not path.exists():
            logger.info(f"Runtime config not found at {config_path}, using defaults")
            return

        try:
            with open(path) as f:
                data = json.load(f)
                self.host_ip = data.get("host_ip")
                self.ports = data.get("ports", {})
                self.pod_name = data.get("pod_name")
                self.pod_id = data.get("pod_id")
                self.app_name = data.get("app_name")
                self.created = data.get("created")
                logger.info(f"Loaded runtime config: host_ip={self.host_ip}, ports={self.ports}")
        except Exception as e:
            logger.warning(f"Failed to load runtime config from {config_path}: {e}")

    def get_host_port(self, container_port: str) -> Optional[str]:
        """Get the host port for a given container port."""
        return self.ports.get(container_port)

    def get_public_url(self, container_port: str, scheme: str = "http") -> Optional[str]:
        """Get the public URL for a given container port."""
        host_port = self.get_host_port(container_port)
        if host_port and self.host_ip:
            return f"{scheme}://{self.host_ip}:{host_port}"
        return None


class Settings(BaseSettings):
    model_config = {"env_prefix": "", "case_sensitive": False}

    # LLM provider selection
    llm_provider: Literal["vllm", "openai"] = "vllm"

    # Local vLLM settings
    llm_endpoint: str = "http://localhost:8000"
    llm_model: str = "ibm-granite/granite-3.3-8b-instruct"

    # External OpenAI-compatible API settings
    openai_api_key: Optional[str] = None
    openai_base_url: Optional[str] = None
    openai_model: str = "gpt-4o"

    # Generation parameters
    llm_temperature: float = 0.0
    llm_max_tokens: int = 1024

    # Service endpoints
    summarize_endpoint: str = "http://localhost:6000"

    # Agent loop
    max_tool_iterations: int = 5

    # HTTP client
    http_pool_maxsize: int = 10
    http_timeout: int = 600

    # Server
    agent_port: int = 9000
    agent_host: str = "0.0.0.0"

    # Logging
    log_level: str = "INFO"

    # Runtime config path
    runtime_config_path: str = "/runtime/config.json"
