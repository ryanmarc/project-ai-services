"""Agent Card definition for A2A protocol discovery."""

from a2a.types import AgentCard, AgentCapabilities, AgentSkill


def build_agent_card(host: str = "0.0.0.0", port: int = 9000) -> AgentCard:
    """Build the Agent Card served at /.well-known/agent.json."""
    return AgentCard(
        name="ai-services-agent",
        description=(
            "AI agent that summarizes text content using the ai-services summarize service. "
            "Accepts natural language requests and uses tool calling to produce summaries."
        ),
        url=f"http://{host}:{port}",
        version="0.1.0",
        capabilities=AgentCapabilities(streaming=True),
        defaultInputModes=["text"],
        defaultOutputModes=["text"],
        skills=[
            AgentSkill(
                id="summarize",
                name="Summarize",
                description="Summarize text content to a shorter form.",
                tags=["summarization", "text"],
                examples=[
                    "Summarize this article: ...",
                    "Give me a 50-word summary of the following text: ...",
                ],
            ),
        ],
    )
