"""Shared fixtures for litellm migration tests."""
import os
import sys
import types

# Ensure src/ is on the path so imports work like they do in production
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

# Stub lingua before anything imports it
_lingua = types.ModuleType("lingua")
_lingua.Language = type("Language", (), {"ENGLISH": "en", "GERMAN": "de"})

class _FakeBuilder:
    def from_languages(self, *args): return self
    def with_preloaded_language_models(self): return self
    def build(self): return None

_lingua.LanguageDetectorBuilder = _FakeBuilder()
sys.modules["lingua"] = _lingua

# Stub digitize.config before any module imports it
_digitize = types.ModuleType("digitize")
_cfg = types.ModuleType("digitize.config")
_cfg.DIGITIZED_DOCS_DIR = "/tmp/test-digitized"
_cfg.WORKER_SIZE = 1
_cfg.HEAVY_PDF_CONVERT_WORKER_SIZE = 1
_cfg.HEAVY_PDF_PAGE_THRESHOLD = 50
_cfg.LLM_POOL_SIZE = 1
sys.modules["digitize"] = _digitize
sys.modules["digitize.config"] = _cfg
