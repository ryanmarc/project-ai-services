"""Environment-driven settings for the AI agent."""

from typing import Literal, Optional

from pydantic import Field
from pydantic_settings import BaseSettings


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
