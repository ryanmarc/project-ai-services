"""A2A TaskHandler bridging the protocol to the agent loop."""

import logging
from collections.abc import AsyncIterator

from a2a.server.agent_execution import AgentExecutor
from a2a.server.events import EventQueue
from a2a.types import (
    FilePart,
    Message,
    Part,
    Role,
    Task,
    TaskState,
    TextPart,
)
from a2a.utils import new_agent_text_message

from agent.llm.base import LLMProvider
from agent.loop import run_agent, run_agent_stream
from agent.tools.base import ToolRegistry

logger = logging.getLogger(__name__)


def _extract_user_text(message: Message) -> str:
    """Extract plain text from an A2A message's parts."""
    parts: list[Part] = message.parts or []
    texts = []
    for part in parts:
        if isinstance(part.root, TextPart):
            texts.append(part.root.text)
        elif isinstance(part.root, FilePart) and part.root.file and part.root.file.name:
            texts.append(f"[File: {part.root.file.name}]")
    return "\n".join(texts) if texts else ""


class AgentTaskHandler(AgentExecutor):
    """Bridges A2A protocol tasks to the agent loop."""

    def __init__(
        self,
        llm: LLMProvider,
        registry: ToolRegistry,
        max_iterations: int = 5,
    ) -> None:
        self._llm = llm
        self._registry = registry
        self._max_iterations = max_iterations

    async def execute(self, message: Message, event_queue: EventQueue) -> None:
        """Handle a non-streaming A2A task (tasks/send)."""
        user_text = _extract_user_text(message)
        if not user_text:
            event_queue.enqueue_event(
                new_agent_text_message("Error: No text content found in the message.")
            )
            event_queue.enqueue_event(TaskState.completed)
            return

        logger.info("A2A task received: %s", user_text[:100])

        try:
            result = run_agent(
                user_text, self._llm, self._registry, self._max_iterations
            )
            event_queue.enqueue_event(new_agent_text_message(result))
            event_queue.enqueue_event(TaskState.completed)
        except Exception as e:
            logger.exception("Agent execution failed")
            event_queue.enqueue_event(
                new_agent_text_message(f"Error: Agent execution failed: {e}")
            )
            event_queue.enqueue_event(TaskState.failed)

    async def cancel(self, message: Message, event_queue: EventQueue) -> None:
        """Handle task cancellation."""
        event_queue.enqueue_event(
            new_agent_text_message("Task cancelled.")
        )
        event_queue.enqueue_event(TaskState.canceled)
