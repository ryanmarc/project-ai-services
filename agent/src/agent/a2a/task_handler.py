"""A2A TaskHandler bridging the protocol to the agent loop."""

import asyncio
import logging
from functools import partial

from a2a.server.agent_execution import AgentExecutor, RequestContext
from a2a.server.events import EventQueue
from a2a.types import TaskState
from a2a.utils import new_agent_text_message

from agent.llm.base import LLMProvider
from agent.loop import run_agent
from agent.tools.base import ToolRegistry

logger = logging.getLogger(__name__)


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

    async def execute(self, context: RequestContext, event_queue: EventQueue) -> None:
        """Handle a non-streaming A2A task (message/send)."""
        user_text = context.get_user_input()
        if not user_text:
            await event_queue.enqueue_event(
                new_agent_text_message("Error: No text content found in the message.")
            )
            await event_queue.enqueue_event(TaskState.completed)
            return

        logger.info("A2A task received: %s", user_text[:100])

        try:
            loop = asyncio.get_running_loop()
            result = await loop.run_in_executor(
                None,
                partial(
                    run_agent,
                    user_text, self._llm, self._registry, self._max_iterations,
                ),
            )
            await event_queue.enqueue_event(new_agent_text_message(result))
            await event_queue.enqueue_event(TaskState.completed)
        except Exception as e:
            logger.exception("Agent execution failed")
            await event_queue.enqueue_event(
                new_agent_text_message(f"Error: Agent execution failed: {e}")
            )
            await event_queue.enqueue_event(TaskState.failed)

    async def cancel(self, context: RequestContext, event_queue: EventQueue) -> None:
        """Handle task cancellation."""
        await event_queue.enqueue_event(
            new_agent_text_message("Task cancelled.")
        )
        await event_queue.enqueue_event(TaskState.canceled)
