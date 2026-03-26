"""Entry point: starts A2A server or interactive CLI mode."""

import argparse
import logging
import sys

from agent.config import Settings


def build_llm(settings: Settings):
    """Factory: create the appropriate LLM provider."""
    if settings.llm_provider == "openai":
        from agent.llm.openai_provider import OpenAIProvider

        if not settings.openai_api_key:
            raise ValueError("OPENAI_API_KEY is required when llm_provider=openai")
        return OpenAIProvider(
            api_key=settings.openai_api_key,
            model=settings.openai_model,
            base_url=settings.openai_base_url,
            temperature=settings.llm_temperature,
            max_tokens=settings.llm_max_tokens,
        )

    from agent.llm.vllm_provider import VLLMProvider

    return VLLMProvider(
        endpoint=settings.llm_endpoint,
        model=settings.llm_model,
        temperature=settings.llm_temperature,
        max_tokens=settings.llm_max_tokens,
        pool_maxsize=settings.http_pool_maxsize,
        timeout=settings.http_timeout,
    )


def build_registry(settings: Settings):
    """Create ToolRegistry and register available tools."""
    from agent.tools.base import ToolRegistry
    from agent.tools.summarize import SummarizeTool

    registry = ToolRegistry()
    registry.register(
        SummarizeTool(
            endpoint=settings.summarize_endpoint,
            pool_maxsize=settings.http_pool_maxsize,
            timeout=settings.http_timeout,
        )
    )
    return registry


def run_cli(settings: Settings) -> None:
    """Interactive REPL for local testing."""
    from agent.loop import run_agent

    llm = build_llm(settings)
    registry = build_registry(settings)

    print("AI Services Agent CLI (type 'quit' to exit)")
    print(f"LLM: {settings.llm_provider} | Tools: {registry.list_names()}")
    print("-" * 50)

    while True:
        try:
            user_input = input("\n> ").strip()
        except (EOFError, KeyboardInterrupt):
            print("\nBye!")
            break
        if not user_input or user_input.lower() in ("quit", "exit"):
            print("Bye!")
            break

        try:
            result = run_agent(
                user_input, llm, registry, settings.max_tool_iterations
            )
            print(f"\n{result}")
        except Exception as e:
            print(f"\nError: {e}")


def run_server(settings: Settings) -> None:
    """Start the A2A protocol server."""
    from a2a.server.apps import A2AStarletteApplication
    from a2a.server.request_handlers import DefaultRequestHandler

    from agent.a2a.agent_card import build_agent_card
    from agent.a2a.task_handler import AgentTaskHandler

    llm = build_llm(settings)
    registry = build_registry(settings)

    agent_card = build_agent_card(host=settings.agent_host, port=settings.agent_port)
    task_handler = AgentTaskHandler(
        llm=llm, registry=registry, max_iterations=settings.max_tool_iterations
    )
    request_handler = DefaultRequestHandler(
        agent_executor=task_handler,
        agent_card=agent_card,
    )

    app = A2AStarletteApplication(
        agent_card=agent_card, http_handler=request_handler
    )

    import uvicorn

    uvicorn.run(
        app.build(),
        host=settings.agent_host,
        port=settings.agent_port,
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="AI Services Agent")
    parser.add_argument("--cli", action="store_true", help="Run in interactive CLI mode")
    args = parser.parse_args()

    settings = Settings()

    logging.basicConfig(
        level=getattr(logging, settings.log_level.upper(), logging.INFO),
        format="%(asctime)s %(name)s %(levelname)s %(message)s",
    )

    if args.cli:
        run_cli(settings)
    else:
        run_server(settings)


if __name__ == "__main__":
    main()
