from concurrent.futures import ThreadPoolExecutor, as_completed
from common.misc_utils import get_logger
from typing import List, Tuple

import litellm

logger = get_logger("reranker")


def rerank_helper(query: str, document: dict, model: str, endpoint: str = None) -> Tuple[dict, float]:
    """
    Rerank a single document with respect to the query using litellm.
    Returns a (document, score) tuple.
    """
    page_content = document.get("page_content", "")
    if not page_content:
        logger.warning("Document has no page_content, assigning score 0.0")
        return document, 0.0

    kwargs = {
        "model": model,
        "query": query,
        "documents": [page_content],
        "max_tokens_per_doc": 512,
    }
    if endpoint:
        kwargs["api_base"] = endpoint

    result = litellm.rerank(**kwargs)
    score = result.results[0].relevance_score
    return document, score


def rerank_documents(query: str, documents: List[dict], model: str, endpoint: str, max_workers: int = 8) -> List[Tuple[dict, float]]:
    """
    Rerank documents for a given query using litellm.

    Returns:
        List of (document, score) sorted by descending score.
    """
    reranked: List[Tuple[dict, float]] = []

    with ThreadPoolExecutor(max_workers=max(1, min(max_workers, len(documents)))) as executor:
        futures = {
            executor.submit(rerank_helper, query, doc, model, endpoint): doc
            for doc in documents
        }

        for future in as_completed(futures):
            doc = futures[future]
            try:
                reranked.append(future.result())
            except Exception as e:
                logger.error(f"Thread error: {e}")
                reranked.append((doc, 0.0))

    return sorted(reranked, key=lambda x: x[1], reverse=True)
