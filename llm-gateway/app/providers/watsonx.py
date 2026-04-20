from __future__ import annotations

from app.config import ModelEntry


class WatsonxProvider:
    def __init__(self, entry: ModelEntry, timeout: float) -> None:
        self.entry = entry
        self.timeout = timeout
