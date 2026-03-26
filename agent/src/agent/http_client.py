"""Shared HTTP session factory and retry decorator."""

import functools
import logging
import time
from typing import Any, Callable, TypeVar

import requests
from requests.adapters import HTTPAdapter

logger = logging.getLogger(__name__)

T = TypeVar("T")

_session: requests.Session | None = None


def get_session(pool_maxsize: int = 10) -> requests.Session:
    """Return a shared requests.Session with connection pooling."""
    global _session
    if _session is None:
        adapter = HTTPAdapter(
            pool_connections=2,
            pool_maxsize=pool_maxsize,
            pool_block=True,
        )
        session = requests.Session()
        # nosemgrep: request-session-with-http -- internal pod network uses HTTP
        session.mount("http://", adapter)
        session.mount("https://", adapter)
        _session = session
    return _session


def retry_on_transient_error(
    max_retries: int = 3,
    initial_delay: float = 1.0,
    backoff_multiplier: float = 2.0,
    max_delay: float = 10.0,
) -> Callable[[Callable[..., T]], Callable[..., T]]:
    """Retry on 5xx and connection errors with exponential backoff."""

    def decorator(func: Callable[..., T]) -> Callable[..., T]:
        @functools.wraps(func)
        def wrapper(*args: Any, **kwargs: Any) -> T:
            last_exc: Exception | None = None
            for attempt in range(max_retries):
                try:
                    return func(*args, **kwargs)
                except requests.exceptions.RequestException as e:
                    last_exc = e
                    if not _is_retryable(e):
                        raise
                    if attempt == max_retries - 1:
                        raise
                    delay = min(initial_delay * (backoff_multiplier ** attempt), max_delay)
                    logger.warning(
                        "%s attempt %d/%d failed, retrying in %.1fs: %s",
                        func.__name__, attempt + 1, max_retries, delay, e,
                    )
                    time.sleep(delay)
            if last_exc:
                raise last_exc
            raise RuntimeError(f"{func.__name__} failed after all retries")

        return wrapper

    return decorator


def _is_retryable(exc: requests.exceptions.RequestException) -> bool:
    if isinstance(exc, requests.exceptions.HTTPError) and exc.response is not None:
        return exc.response.status_code >= 500
    if isinstance(exc, (requests.exceptions.ConnectionError, requests.exceptions.Timeout)):
        return True
    error_str = str(exc)
    return any(
        p in error_str
        for p in ("Connection aborted", "RemoteDisconnected", "Connection reset")
    )
