import numpy as np
import litellm
from common.misc_utils import get_logger

logger = get_logger("Embedding")

_embedder_instance = None

class Embedding:
    def __init__(self, emb_model, emb_endpoint, max_tokens):
        self.emb_model = emb_model
        self.emb_endpoint = emb_endpoint
        self.max_tokens = int(max_tokens)

    def embed_documents(self, texts):
        return self._post_embedding(texts)

    def embed_query(self, text):
        return self._post_embedding([text])[0]

    def _post_embedding(self, texts):
        kwargs = {
            "model": self.emb_model,
            "input": texts,
            "num_retries": 3,
        }
        if self.emb_endpoint:
            kwargs["api_base"] = self.emb_endpoint
        # truncate_prompt_tokens is vLLM-specific; pass it through and let
        # litellm forward it to providers that support it.
        kwargs["truncate_prompt_tokens"] = self.max_tokens - 1

        response = litellm.embedding(**kwargs)
        embeddings = [item["embedding"] for item in response.data]
        return [np.array(embed, dtype=np.float32) for embed in embeddings]

def get_embedder(emb_model, emb_endpoint, max_tokens) -> Embedding:
    """
    Returns an instance of the Embedding class.
    """
    global _embedder_instance
    if _embedder_instance is None:
        _embedder_instance = Embedding(emb_model, emb_endpoint, max_tokens)
    return _embedder_instance
