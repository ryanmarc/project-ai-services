# LLM Gateway Design Spec

## Overview

Replace direct vLLM connections from RAG services with a centralized LiteLLM proxy (LLM Gateway). The gateway abstracts LLM, embedding, and reranker providers behind a single endpoint, enabling model/provider switching via configuration without changing service code.

## Motivation

Currently, all RAG services (chatbot, summarize, digitize) connect directly to vLLM endpoints with hardcoded model names. Adding a new LLM provider (e.g., IBM watsonx) or swapping models requires changing environment variables across multiple services and redeploying them. A centralized gateway solves this by providing one routing layer with model aliases.

## Architecture

```
                          ┌──────────────────────┐
                          │    LiteLLM Proxy      │
┌──────────────┐          │    (llm-gateway)      │          ┌─────────────────┐
│ chatbot      │──┐       │                       │────────► │ vLLM Server     │
│ backend      │  │       │  Routes:              │          │ :8000 instruct  │
├──────────────┤  │       │  /v1/chat/completions  │          │ :8001 embedding │
│ summarize    │  ├──────►│  /v1/embeddings        │          │ :8002 reranker  │
│ api          │  │       │  /rerank               │          └─────────────────┘
├──────────────┤  │       │                       │
│ digitize     │──┘       │  Model aliases:       │          ┌─────────────────┐
│ api          │          │  granite-3.3-8b-inst..│────────► │ IBM watsonx     │
└──────────────┘          │  granite-embedding-.. │          └─────────────────┘
                          │  bge-reranker-v2-m3   │
                          └──────────────────────┘

Tokenize/detokenize calls bypass the gateway (vLLM-specific, not OpenAI-compatible).
```

### What changes

- `LLM_ENDPOINT`, `EMB_ENDPOINT`, `RERANKER_ENDPOINT` in all RAG services change from separate vLLM URLs to a single gateway URL (`http://llm-gateway:4000`).
- `LLM_MODEL`, `EMB_MODEL`, `RERANKER_MODEL` change to gateway model alias names.
- A new `VLLM_TOKENIZER_ENDPOINT` env var is added for the two vLLM-specific tokenize/detokenize calls.

### What stays the same

- The vLLM server continues running as before; the gateway routes to it.
- All existing retry logic, streaming, and payload construction in `llm_utils.py` remains intact.
- The OpenAI-compatible API interface (`/v1/chat/completions`, `/v1/embeddings`) is identical.

## Gateway Configuration

```yaml
# llm-gateway/litellm_config.yaml

model_list:
  # LLM (vLLM local)
  - model_name: granite-3.3-8b-instruct
    litellm_params:
      model: openai/ibm-granite/granite-3.3-8b-instruct
      api_base: os.environ/VLLM_INSTRUCT_URL
      api_key: "fake-key"

  # Embedding (vLLM local)
  - model_name: granite-embedding-278m
    litellm_params:
      model: openai/ibm-granite/granite-embedding-278m-multilingual
      api_base: os.environ/VLLM_EMBEDDING_URL
      api_key: "fake-key"

  # Reranker (vLLM local)
  - model_name: bge-reranker-v2-m3
    litellm_params:
      model: openai/BAAI/bge-reranker-v2-m3
      api_base: os.environ/VLLM_RERANKER_URL
      api_key: "fake-key"

  # LLM (watsonx cloud)
  - model_name: granite-3-8b-instruct-wx
    litellm_params:
      model: watsonx/ibm/granite-3-8b-instruct
      api_key: os.environ/WATSONX_API_KEY
      watsonx_project_id: os.environ/WATSONX_PROJECT_ID

general_settings:
  master_key: os.environ/LITELLM_MASTER_KEY

litellm_settings:
  drop_params: true
  request_timeout: 120
```

### Key config decisions

- **Model aliases use real model names** (e.g., `granite-3.3-8b-instruct`) so it's always obvious what model is being used in logs, configs, and env vars.
- **`drop_params: true`** silently drops provider-unsupported parameters (e.g., `repetition_penalty` is vLLM-specific and would error on watsonx).
- **API keys via env var references** (`os.environ/WATSONX_API_KEY`) so the config file contains no secrets and is safe to commit.
- **vLLM uses `api_key: "fake-key"`** since vLLM doesn't require authentication but LiteLLM requires the field.

### Switching models

To switch which model the RAG services use, change `LLM_MODEL` in the deployment template:

```
LLM_MODEL=granite-3.3-8b-instruct      # routes to vLLM (local)
LLM_MODEL=granite-3-8b-instruct-wx     # routes to watsonx (cloud)
```

No code changes, no gateway redeployment. Only the RAG service env var changes.

## Repository Structure

```
llm-gateway/
├── litellm_config.yaml    # Model routing config
├── Containerfile           # Container image build
└── README.md               # Setup and usage docs
```

### Containerfile

```dockerfile
FROM ghcr.io/berriai/litellm:main-latest

COPY litellm_config.yaml /app/config.yaml

EXPOSE 4000

CMD ["--config", "/app/config.yaml", "--port", "4000"]
```

## Deployment Changes

### Environment variable mapping

```
Before                                          After
──────────────────────────────────────────────  ──────────────────────────────────────────
LLM_ENDPOINT=http://vllm-server:8000           LLM_ENDPOINT=http://llm-gateway:4000
LLM_MODEL=ibm-granite/granite-3.3-8b-instruct LLM_MODEL=granite-3.3-8b-instruct
EMB_ENDPOINT=http://vllm-server:8001           EMB_ENDPOINT=http://llm-gateway:4000
EMB_MODEL=ibm-granite/granite-embedding-278m.. EMB_MODEL=granite-embedding-278m
RERANKER_ENDPOINT=http://vllm-server:8002      RERANKER_ENDPOINT=http://llm-gateway:4000
RERANKER_MODEL=BAAI/bge-reranker-v2-m3         RERANKER_MODEL=bge-reranker-v2-m3
(none)                                         VLLM_TOKENIZER_ENDPOINT=http://vllm-server:8001
```

### Podman

New template: `llm-gateway.yaml.tmpl` — a new pod running the LiteLLM proxy container.

Updated templates:
- `chat-bot.yaml.tmpl` — update `LLM_ENDPOINT`, `EMB_ENDPOINT`, `RERANKER_ENDPOINT` to gateway, add `VLLM_TOKENIZER_ENDPOINT`
- `digitize.yaml.tmpl` — same env var updates
- `summarize-api.yaml.tmpl` — same env var updates

The gateway's `litellm_config.yaml` is a static file, not a Go template. In the Podman template, the vLLM hostnames are dynamic (e.g., `{{ .AppName }}--vllm-server`). To handle this, the gateway container receives the vLLM base URLs as environment variables (e.g., `VLLM_INSTRUCT_URL`, `VLLM_EMBEDDING_URL`, `VLLM_RERANKER_URL`), and the `litellm_config.yaml` references them via `os.environ/VLLM_INSTRUCT_URL`. The Podman template sets these env vars using the `{{ .AppName }}` convention. The same pattern applies in OpenShift, where the env vars are set in the Deployment manifest.

### OpenShift

New templates:
- `llm-gateway-deployment.yaml` — Deployment for the LiteLLM proxy container
- `llm-gateway-service.yaml` — ClusterIP Service exposing port 4000
- `llm-gateway-configmap.yaml` — ConfigMap mounting `litellm_config.yaml`
- `llm-gateway-secret.yaml` — Secret for `WATSONX_API_KEY`, `WATSONX_PROJECT_ID`, `LITELLM_MASTER_KEY`

Updated templates:
- `backend-deployment.yaml` — update env vars to gateway
- `digitize-api-deployment.yaml` — update env vars to gateway
- `summarize-api-deployment.yaml` — update env vars to gateway

## Python Code Changes

### `spyre-rag/src/common/misc_utils.py`

Add `VLLM_TOKENIZER_ENDPOINT` to `get_model_endpoints()`:

```python
tokenizer_dict = {
    'vllm_tokenizer_endpoint': os.getenv("VLLM_TOKENIZER_ENDPOINT", ""),
}
```

### `spyre-rag/src/common/llm_utils.py`

**`tokenize_with_llm()` and `detokenize_with_llm()`** — callers must pass `VLLM_TOKENIZER_ENDPOINT` instead of `emb_endpoint`. The only internal caller is `query_vllm_payload()` (line 114-116).

**All other functions** — no changes. They post to `{endpoint}/v1/chat/completions` with a `model` field, which is exactly the interface the gateway exposes.

### Behavioral note

`repetition_penalty` in `query_vllm_payload()` (line 128) is a vLLM-specific parameter. With `drop_params: true` in the gateway config, it is silently dropped when routing to watsonx. This is a minor behavioral difference when using watsonx vs vLLM.

## Testing & Validation

### Gateway health

LiteLLM exposes `/health` which checks connectivity to all configured models. Use for liveness/readiness probes in both Podman and OpenShift.

### Smoke tests

- `POST /v1/chat/completions` with `model: granite-3.3-8b-instruct` — verify LLM routing
- `POST /v1/embeddings` with `model: granite-embedding-278m` — verify embedding routing
- `POST /rerank` with `model: bge-reranker-v2-m3` — verify reranker routing
- `GET /v1/models` — verify all aliases are listed

### Existing tests

The existing RAG e2e tests in `ai-services/tests/e2e/` should pass unchanged. From the RAG services' perspective, the API interface is identical.

Streaming responses (`query_vllm_stream()`, `query_vllm_summarize_stream()`) must be verified to work through the proxy.

### Failure modes

- Gateway down: RAG services get connection errors (same as vLLM being down today)
- vLLM down: gateway returns provider error to RAG services
- Invalid model alias: gateway returns model not found error

## Scope

### In scope

- LiteLLM proxy container and config
- Podman deployment templates
- OpenShift deployment templates (Deployment, Service, ConfigMap, Secret)
- Env var updates in all RAG service templates
- `tokenize_with_llm()` / `detokenize_with_llm()` endpoint refactor
- vLLM and IBM watsonx provider support

### Out of scope

- Additional cloud providers (can be added to config later)
- LiteLLM database/logging features (SQLite, Langfuse, etc.)
- Rate limiting or budget controls
- UI for gateway management
- Load balancing across multiple instances of the same model
