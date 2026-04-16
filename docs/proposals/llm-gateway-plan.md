# LLM Gateway Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a centralized LiteLLM proxy (LLM Gateway) that sits between all RAG services and LLM/embedding/reranker providers, enabling model and provider switching via configuration.

**Architecture:** A new `llm-gateway/` directory at the repo root contains the LiteLLM config and Containerfile. Deployment templates for both Podman and OpenShift are updated to route RAG services through the gateway instead of directly to vLLM. The only Python code change is redirecting `tokenize_with_llm()`/`detokenize_with_llm()` to use a direct vLLM endpoint since those are vLLM-specific APIs not proxied by LiteLLM.

**Tech Stack:** LiteLLM (Python proxy), Docker/Podman containers, Helm (OpenShift), Go templates (Podman)

**Spec:** `docs/proposals/llm-gateway-design.md`

---

### Task 1: Create LLM Gateway Config and Container

**Files:**
- Create: `llm-gateway/litellm_config.yaml`
- Create: `llm-gateway/Containerfile`

- [ ] **Step 1: Create the LiteLLM config file**

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

- [ ] **Step 2: Create the Containerfile**

```dockerfile
# llm-gateway/Containerfile
FROM ghcr.io/berriai/litellm:main-latest

COPY litellm_config.yaml /app/config.yaml

EXPOSE 4000

CMD ["--config", "/app/config.yaml", "--port", "4000"]
```

- [ ] **Step 3: Build and verify the container locally**

Run:
```bash
cd llm-gateway
podman build -t llm-gateway:dev -f Containerfile .
```
Expected: Image builds successfully.

- [ ] **Step 4: Commit**

```bash
git add llm-gateway/litellm_config.yaml llm-gateway/Containerfile
git commit -s -S -m "feat: add LLM gateway LiteLLM config and Containerfile"
```

---

### Task 2: Add Podman Deployment Template for LLM Gateway

**Files:**
- Create: `ai-services/assets/applications/rag-dev/podman/templates/llm-gateway.yaml.tmpl`
- Modify: `ai-services/assets/applications/rag-dev/podman/values.yaml`

- [ ] **Step 1: Create the Podman template**

This follows the same pattern as other pod templates in the directory (e.g., `vllm-server.yaml.tmpl`, `chat-bot.yaml.tmpl`). The gateway receives the vLLM URLs as env vars so the static `litellm_config.yaml` can reference them.

```yaml
# ai-services/assets/applications/rag-dev/podman/templates/llm-gateway.yaml.tmpl
apiVersion: v1
kind: Pod
metadata:
  name: "{{ .AppName }}--llm-gateway"
  labels:
    ai-services.io/application: "{{ .AppName }}"
    ai-services.io/template: "{{ .AppTemplateName }}"
    ai-services.io/version: "{{ .Version }}"
spec:
  containers:
    - name: llm-gateway
      image: "{{ .Values.llmGateway.image }}"
      env:
        - name: VLLM_INSTRUCT_URL
          value: "http://{{ .AppName }}--vllm-server:8000/v1"
        - name: VLLM_EMBEDDING_URL
          value: "http://{{ .AppName }}--vllm-server:8001/v1"
        - name: VLLM_RERANKER_URL
          value: "http://{{ .AppName }}--vllm-server:8002/v1"
        - name: LITELLM_MASTER_KEY
          value: "{{ .Values.llmGateway.masterKey }}"
        - name: WATSONX_API_KEY
          value: "{{ .Values.llmGateway.watsonxApiKey }}"
        - name: WATSONX_PROJECT_ID
          value: "{{ .Values.llmGateway.watsonxProjectId }}"
      ports:
        - containerPort: 4000
          protocol: TCP
      livenessProbe:
        httpGet:
          path: /health
          port: 4000
        initialDelaySeconds: 15
        periodSeconds: 30
        timeoutSeconds: 5
        failureThreshold: 3
      resources:
        requests:
          memory: "512Mi"
        limits:
          memory: "512Mi"
```

- [ ] **Step 2: Add gateway values to Podman values.yaml**

Add this block to the end of `ai-services/assets/applications/rag-dev/podman/values.yaml`:

```yaml
llmGateway:
  # @hidden
  image: llm-gateway:dev
  # @hidden
  masterKey: "sk-dev-key"
  # @description IBM watsonx API key for cloud LLM access. Leave empty to use local vLLM only.
  watsonxApiKey: ""
  # @description IBM watsonx project ID. Required when watsonxApiKey is set.
  watsonxProjectId: ""
```

- [ ] **Step 3: Commit**

```bash
git add ai-services/assets/applications/rag-dev/podman/templates/llm-gateway.yaml.tmpl ai-services/assets/applications/rag-dev/podman/values.yaml
git commit -s -S -m "feat: add Podman deployment template for LLM gateway"
```

---

### Task 3: Update Podman RAG Service Templates to Use Gateway

**Files:**
- Modify: `ai-services/assets/applications/rag-dev/podman/templates/chat-bot.yaml.tmpl:52-65`
- Modify: `ai-services/assets/applications/rag-dev/podman/templates/digitize.yaml.tmpl:68-77`
- Modify: `ai-services/assets/applications/rag-dev/podman/templates/summarize-api.yaml.tmpl:31-34`

All three templates need the same pattern of changes: point `LLM_ENDPOINT`, `EMB_ENDPOINT`, `RERANKER_ENDPOINT` at the gateway, update model names to aliases, and add `VLLM_TOKENIZER_ENDPOINT`.

- [ ] **Step 1: Update chat-bot.yaml.tmpl**

In `ai-services/assets/applications/rag-dev/podman/templates/chat-bot.yaml.tmpl`, replace the env vars in the `backend-server` container (lines 52-65):

```yaml
      env:
        - name: EMB_ENDPOINT
          value: "http://{{ .AppName }}--llm-gateway:4000"
        - name: EMB_MODEL
          value: "granite-embedding-278m"
        - name: EMB_MAX_TOKENS
          value: "512"
        - name: LLM_ENDPOINT
          value: "http://{{ .AppName }}--llm-gateway:4000"
        - name: LLM_MODEL
          value: "granite-3.3-8b-instruct"
        - name: RERANKER_ENDPOINT
          value: "http://{{ .AppName }}--llm-gateway:4000"
        - name: RERANKER_MODEL
          value: "bge-reranker-v2-m3"
        - name: VLLM_TOKENIZER_ENDPOINT
          value: "http://{{ .AppName }}--vllm-server:8001"
```

Keep the `OPENSEARCH_*` and `LOG_LEVEL` env vars unchanged after this block.

- [ ] **Step 2: Update digitize.yaml.tmpl**

In `ai-services/assets/applications/rag-dev/podman/templates/digitize.yaml.tmpl`, replace the env vars in the `backend-server` container (lines 68-77):

```yaml
      env:
        - name: EMB_ENDPOINT
          value: "http://{{ .AppName }}--llm-gateway:4000"
        - name: EMB_MODEL
          value: "granite-embedding-278m"
        - name: EMB_MAX_TOKENS
          value: "512"
        - name: LLM_ENDPOINT
          value: "http://{{ .AppName }}--llm-gateway:4000"
        - name: LLM_MODEL
          value: "granite-3.3-8b-instruct"
        - name: VLLM_TOKENIZER_ENDPOINT
          value: "http://{{ .AppName }}--vllm-server:8001"
```

Keep the `OPENSEARCH_*` and `LOG_LEVEL` env vars unchanged after this block.

- [ ] **Step 3: Update summarize-api.yaml.tmpl**

In `ai-services/assets/applications/rag-dev/podman/templates/summarize-api.yaml.tmpl`, replace the env vars (lines 31-34):

```yaml
      env:
        - name: PORT
          value: "6000"
        - name: LLM_ENDPOINT
          value: "http://{{ .AppName }}--llm-gateway:4000"
        - name: LLM_MODEL
          value: "granite-3.3-8b-instruct"
        - name: LOG_LEVEL
          value: "{{ .Values.summarize.log_level}}"
```

Note: summarize-api does not use embeddings or reranker, so only `LLM_ENDPOINT`/`LLM_MODEL` change. It also does not use tokenize/detokenize, so no `VLLM_TOKENIZER_ENDPOINT` needed.

- [ ] **Step 4: Commit**

```bash
git add ai-services/assets/applications/rag-dev/podman/templates/chat-bot.yaml.tmpl ai-services/assets/applications/rag-dev/podman/templates/digitize.yaml.tmpl ai-services/assets/applications/rag-dev/podman/templates/summarize-api.yaml.tmpl
git commit -s -S -m "feat: update Podman RAG templates to route through LLM gateway"
```

---

### Task 4: Add OpenShift Deployment Templates for LLM Gateway

**Files:**
- Create: `ai-services/assets/applications/rag-dev/openshift/templates/llm-gateway-deployment.yaml`
- Create: `ai-services/assets/applications/rag-dev/openshift/templates/llm-gateway-service.yaml`
- Create: `ai-services/assets/applications/rag-dev/openshift/templates/llm-gateway-secret.yaml`
- Modify: `ai-services/assets/applications/rag-dev/openshift/values.yaml`

- [ ] **Step 1: Create llm-gateway-deployment.yaml**

Follow the pattern from `backend-deployment.yaml` for labels, selectors, and probe structure.

```yaml
# ai-services/assets/applications/rag-dev/openshift/templates/llm-gateway-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "llm-gateway"
  labels:
    ai-services.io/application: {{ .Release.Name }}
    ai-services.io/template: {{ .Chart.Name }}
    ai-services.io/version: {{ default .Chart.AppVersion | quote }}
spec:
  replicas: 1
  selector:
    matchLabels:
      ai-services.io/application: {{ .Release.Name }}
      ai-services.io/component: llm-gateway
  template:
    metadata:
      labels:
        ai-services.io/application: {{ .Release.Name }}
        ai-services.io/template: {{ .Chart.Name }}
        ai-services.io/component: llm-gateway
        ai-services.io/version: {{ default .Chart.AppVersion | quote }}
    spec:
      automountServiceAccountToken: false
      containers:
        - name: llm-gateway
          image: "{{ .Values.llmGateway.image }}"
          env:
            - name: VLLM_INSTRUCT_URL
              value: http://instruct-predictor:8000/v1
            - name: VLLM_EMBEDDING_URL
              value: http://embedding-predictor:8080/v1
            - name: VLLM_RERANKER_URL
              value: http://reranker-predictor:8080/v1
            - name: LITELLM_MASTER_KEY
              valueFrom:
                secretKeyRef:
                  name: "llm-gateway-credentials"
                  key: master-key
            - name: WATSONX_API_KEY
              valueFrom:
                secretKeyRef:
                  name: "llm-gateway-credentials"
                  key: watsonx-api-key
            - name: WATSONX_PROJECT_ID
              valueFrom:
                secretKeyRef:
                  name: "llm-gateway-credentials"
                  key: watsonx-project-id
          ports:
            - containerPort: 4000
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /health
              port: 4000
            initialDelaySeconds: 15
            periodSeconds: 30
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /health
              port: 4000
            initialDelaySeconds: 15
            periodSeconds: 30
            timeoutSeconds: 5
            failureThreshold: 3
          resources:
            requests:
              cpu: "500m"
              memory: "512Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"
```

- [ ] **Step 2: Create llm-gateway-service.yaml**

```yaml
# ai-services/assets/applications/rag-dev/openshift/templates/llm-gateway-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: "llm-gateway"
  labels:
    ai-services.io/application: {{ .Release.Name }}
    ai-services.io/template: {{ .Chart.Name }}
    ai-services.io/version: {{ default .Chart.AppVersion | quote }}
spec:
  selector:
    ai-services.io/application: {{ .Release.Name }}
    ai-services.io/component: llm-gateway
  ports:
    - port: 4000
      targetPort: 4000
      protocol: TCP
  type: ClusterIP
```

- [ ] **Step 3: Create llm-gateway-secret.yaml**

```yaml
# ai-services/assets/applications/rag-dev/openshift/templates/llm-gateway-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: "llm-gateway-credentials"
  labels:
    ai-services.io/application: {{ .Release.Name }}
    ai-services.io/template: {{ .Chart.Name }}
    ai-services.io/version: {{ default .Chart.AppVersion | quote }}
type: Opaque
stringData:
  master-key: "{{ .Values.llmGateway.masterKey }}"
  watsonx-api-key: "{{ .Values.llmGateway.watsonxApiKey }}"
  watsonx-project-id: "{{ .Values.llmGateway.watsonxProjectId }}"
```

- [ ] **Step 4: Add gateway values to OpenShift values.yaml**

Add this block to the end of `ai-services/assets/applications/rag-dev/openshift/values.yaml`:

```yaml
llmGateway:
  # @hidden
  image: icr.io/ai-services-cicd/llm-gateway:v0.1.0
  # @hidden
  masterKey: "sk-default-key"
  # @description IBM watsonx API key for cloud LLM access. Leave empty to use local vLLM only.
  watsonxApiKey: ""
  # @description IBM watsonx project ID. Required when watsonxApiKey is set.
  watsonxProjectId: ""
```

- [ ] **Step 5: Commit**

```bash
git add ai-services/assets/applications/rag-dev/openshift/templates/llm-gateway-deployment.yaml ai-services/assets/applications/rag-dev/openshift/templates/llm-gateway-service.yaml ai-services/assets/applications/rag-dev/openshift/templates/llm-gateway-secret.yaml ai-services/assets/applications/rag-dev/openshift/values.yaml
git commit -s -S -m "feat: add OpenShift deployment templates for LLM gateway"
```

---

### Task 5: Update OpenShift RAG Service Templates to Use Gateway

**Files:**
- Modify: `ai-services/assets/applications/rag-dev/openshift/templates/backend-deployment.yaml:41-54`
- Modify: `ai-services/assets/applications/rag-dev/openshift/templates/digitize-api-deployment.yaml:41-50`
- Modify: `ai-services/assets/applications/rag-dev/openshift/templates/summarize-api-deployment.yaml:42-46`

- [ ] **Step 1: Update backend-deployment.yaml**

In `ai-services/assets/applications/rag-dev/openshift/templates/backend-deployment.yaml`, replace the model endpoint env vars (lines 41-54):

```yaml
          env:
            - name: EMB_ENDPOINT
              value: http://llm-gateway:4000
            - name: EMB_MODEL
              value: granite-embedding-278m
            - name: EMB_MAX_TOKENS
              value: "512"
            - name: LLM_ENDPOINT
              value: http://llm-gateway:4000
            - name: LLM_MODEL
              value: granite-3.3-8b-instruct
            - name: RERANKER_ENDPOINT
              value: http://llm-gateway:4000
            - name: RERANKER_MODEL
              value: bge-reranker-v2-m3
            - name: VLLM_TOKENIZER_ENDPOINT
              value: http://embedding-predictor:8080
```

Keep all `OPENSEARCH_*` and `LOG_LEVEL` env vars unchanged after this block.

- [ ] **Step 2: Update digitize-api-deployment.yaml**

In `ai-services/assets/applications/rag-dev/openshift/templates/digitize-api-deployment.yaml`, replace the model endpoint env vars (lines 41-50):

```yaml
          env:
            - name: EMB_ENDPOINT
              value: http://llm-gateway:4000
            - name: EMB_MODEL
              value: granite-embedding-278m
            - name: EMB_MAX_TOKENS
              value: "512"
            - name: LLM_ENDPOINT
              value: http://llm-gateway:4000
            - name: LLM_MODEL
              value: granite-3.3-8b-instruct
            - name: VLLM_TOKENIZER_ENDPOINT
              value: http://embedding-predictor:8080
```

Keep all `OPENSEARCH_*`, `LOG_LEVEL`, `OMP_NUM_THREADS`, `MKL_NUM_THREADS`, `DOCLING_NUM_THREADS`, `OPENBLAS_NUM_THREADS` env vars unchanged after this block.

- [ ] **Step 3: Update summarize-api-deployment.yaml**

In `ai-services/assets/applications/rag-dev/openshift/templates/summarize-api-deployment.yaml`, replace the model endpoint env vars (lines 42-46):

```yaml
          env:
            - name: PORT
              value: "6000"
            - name: LLM_ENDPOINT
              value: http://llm-gateway:4000
            - name: LLM_MODEL
              value: granite-3.3-8b-instruct
            - name: LOG_LEVEL
              value: "{{ .Values.summarize.log_level }}"
```

- [ ] **Step 4: Commit**

```bash
git add ai-services/assets/applications/rag-dev/openshift/templates/backend-deployment.yaml ai-services/assets/applications/rag-dev/openshift/templates/digitize-api-deployment.yaml ai-services/assets/applications/rag-dev/openshift/templates/summarize-api-deployment.yaml
git commit -s -S -m "feat: update OpenShift RAG templates to route through LLM gateway"
```

---

### Task 6: Update Python Code for Tokenizer Endpoint

**Files:**
- Modify: `spyre-rag/src/common/misc_utils.py:181-198`
- Modify: `spyre-rag/src/common/llm_utils.py:107-116`
- Modify: `spyre-rag/src/chatbot/app.py:64-66,314-325`
- Modify: `spyre-rag/src/chatbot/backend_utils.py:11-16`
- Modify: `spyre-rag/src/digitize/ingest.py:41`
- Modify: `spyre-rag/src/digitize/doc_utils.py:530-531`

The tokenize/detokenize functions call vLLM-specific `/tokenize` and `/detokenize` endpoints. Once `EMB_ENDPOINT` points at the LiteLLM gateway (which doesn't proxy these), callers must use the new `VLLM_TOKENIZER_ENDPOINT` env var instead.

- [ ] **Step 1: Add `VLLM_TOKENIZER_ENDPOINT` to `get_model_endpoints()`**

In `spyre-rag/src/common/misc_utils.py`, update `get_model_endpoints()` (line 181) to return a fourth dict:

Change:
```python
def get_model_endpoints():
    emb_model_dict = {
        'emb_endpoint': os.getenv("EMB_ENDPOINT"),
        'emb_model':    os.getenv("EMB_MODEL"),
        'max_tokens':   int(os.getenv("EMB_MAX_TOKENS", "512")),
    }

    llm_model_dict = {
        'llm_endpoint': os.getenv("LLM_ENDPOINT", ""),
        'llm_model':    os.getenv("LLM_MODEL", ""),
    }

    reranker_model_dict = {
        'reranker_endpoint': os.getenv("RERANKER_ENDPOINT"),
        'reranker_model':    os.getenv("RERANKER_MODEL"),
    }

    return emb_model_dict, llm_model_dict, reranker_model_dict
```

To:
```python
def get_model_endpoints():
    emb_model_dict = {
        'emb_endpoint': os.getenv("EMB_ENDPOINT"),
        'emb_model':    os.getenv("EMB_MODEL"),
        'max_tokens':   int(os.getenv("EMB_MAX_TOKENS", "512")),
    }

    llm_model_dict = {
        'llm_endpoint': os.getenv("LLM_ENDPOINT", ""),
        'llm_model':    os.getenv("LLM_MODEL", ""),
    }

    reranker_model_dict = {
        'reranker_endpoint': os.getenv("RERANKER_ENDPOINT"),
        'reranker_model':    os.getenv("RERANKER_MODEL"),
    }

    vllm_tokenizer_endpoint = os.getenv("VLLM_TOKENIZER_ENDPOINT", os.getenv("EMB_ENDPOINT", ""))

    return emb_model_dict, llm_model_dict, reranker_model_dict, vllm_tokenizer_endpoint
```

Note: `VLLM_TOKENIZER_ENDPOINT` falls back to `EMB_ENDPOINT` for backward compatibility when the gateway isn't deployed.

- [ ] **Step 2: Update chatbot `app.py` to unpack the new return value**

In `spyre-rag/src/chatbot/app.py`, update the global declaration (around line 55) and `initialize_models()` (line 64):

Change:
```python
emb_model_dict = {}
llm_model_dict = {}
reranker_model_dict = {}
```

To:
```python
emb_model_dict = {}
llm_model_dict = {}
reranker_model_dict = {}
vllm_tokenizer_endpoint = ""
```

Change:
```python
def initialize_models():
    global emb_model_dict, llm_model_dict, reranker_model_dict
    emb_model_dict, llm_model_dict, reranker_model_dict = get_model_endpoints()
```

To:
```python
def initialize_models():
    global emb_model_dict, llm_model_dict, reranker_model_dict, vllm_tokenizer_endpoint
    emb_model_dict, llm_model_dict, reranker_model_dict, vllm_tokenizer_endpoint = get_model_endpoints()
```

- [ ] **Step 3: Update `validate_query_length` call in chatbot `app.py`**

In `spyre-rag/src/chatbot/app.py`, the call to `validate_query_length` (around line 324) currently passes `emb_endpoint`:

Change:
```python
        is_valid, error_msg = await asyncio.to_thread(
            validate_query_length, query, emb_endpoint
        )
```

To:
```python
        is_valid, error_msg = await asyncio.to_thread(
            validate_query_length, query, vllm_tokenizer_endpoint
        )
```

- [ ] **Step 4: Update `backend_utils.py` `validate_query_length`**

The function signature in `spyre-rag/src/chatbot/backend_utils.py` (line 11) takes `emb_endpoint` but it's really being used as a tokenizer endpoint. Rename for clarity:

Change:
```python
def validate_query_length(query, emb_endpoint):
    
    # Validate that the query length does not exceed the maximum allowed tokens.

    try:
        tokens = tokenize_with_llm(query, emb_endpoint)
```

To:
```python
def validate_query_length(query, tokenizer_endpoint):
    
    # Validate that the query length does not exceed the maximum allowed tokens.

    try:
        tokens = tokenize_with_llm(query, tokenizer_endpoint)
```

- [ ] **Step 5: Update `query_vllm_payload` tokenize calls in `llm_utils.py`**

In `spyre-rag/src/common/llm_utils.py`, `query_vllm_payload()` (line 107) calls `tokenize_with_llm` and `detokenize_with_llm` with `llm_endpoint`. This worked before because vLLM exposed `/tokenize` on the same server. Now `llm_endpoint` is the gateway, which doesn't have `/tokenize`. Add a `tokenizer_endpoint` parameter:

Change:
```python
def query_vllm_payload(question, documents, llm_endpoint, llm_model, stop_words, max_new_tokens, temperature,
                stream, lang):
    context = "\n\n".join([doc.get("page_content") for doc in documents])

    logger.debug(f'Original Context: {context}')

    # dynamic chunk truncation: truncates the context, if doesn't fit in the sequence length
    question_token_count = len(tokenize_with_llm(question, llm_endpoint))
    reamining_tokens = settings.max_input_length - (settings.prompt_template_token_count + question_token_count)
    context = detokenize_with_llm(tokenize_with_llm(context, llm_endpoint)[:reamining_tokens], llm_endpoint)
```

To:
```python
def query_vllm_payload(question, documents, llm_endpoint, llm_model, stop_words, max_new_tokens, temperature,
                stream, lang, tokenizer_endpoint=None):
    context = "\n\n".join([doc.get("page_content") for doc in documents])

    logger.debug(f'Original Context: {context}')

    # dynamic chunk truncation: truncates the context, if doesn't fit in the sequence length
    tok_endpoint = tokenizer_endpoint or llm_endpoint
    question_token_count = len(tokenize_with_llm(question, tok_endpoint))
    reamining_tokens = settings.max_input_length - (settings.prompt_template_token_count + question_token_count)
    context = detokenize_with_llm(tokenize_with_llm(context, tok_endpoint)[:reamining_tokens], tok_endpoint)
```

The `tokenizer_endpoint=None` default with fallback to `llm_endpoint` ensures backward compatibility.

- [ ] **Step 6: Update `query_vllm_non_stream` and `query_vllm_stream` to pass `tokenizer_endpoint`**

In `spyre-rag/src/common/llm_utils.py`, update the two functions that call `query_vllm_payload`:

Change `query_vllm_non_stream` (line 143):
```python
def query_vllm_non_stream(question, documents, llm_endpoint, llm_model, stop_words, max_new_tokens, temperature, perf_stat_dict, lang):
```
To:
```python
def query_vllm_non_stream(question, documents, llm_endpoint, llm_model, stop_words, max_new_tokens, temperature, perf_stat_dict, lang, tokenizer_endpoint=None):
```

And update its call to `query_vllm_payload` (line 147):
```python
    headers, payload = query_vllm_payload(question, documents, llm_endpoint, llm_model, stop_words, max_new_tokens, temperature, False, lang, tokenizer_endpoint=tokenizer_endpoint)
```

Change `query_vllm_stream` (line 162):
```python
def query_vllm_stream(question, documents, llm_endpoint, llm_model, stop_words, max_new_tokens, temperature, perf_stat_dict, lang):
```
To:
```python
def query_vllm_stream(question, documents, llm_endpoint, llm_model, stop_words, max_new_tokens, temperature, perf_stat_dict, lang, tokenizer_endpoint=None):
```

And update its call to `query_vllm_payload` (line 166):
```python
    headers, payload = query_vllm_payload(question, documents, llm_endpoint, llm_model, stop_words, max_new_tokens,
                                          temperature, True, lang, tokenizer_endpoint=tokenizer_endpoint)
```

- [ ] **Step 7: Update chatbot `app.py` to pass `tokenizer_endpoint` to LLM query functions**

In `spyre-rag/src/chatbot/app.py`, update the calls to `query_vllm_stream` and `query_vllm_non_stream` (around lines 377-384):

Change:
```python
                vllm_stream = await asyncio.to_thread(
                    query_vllm_stream, query, docs, llm_endpoint, llm_model, req.stop, max_tokens, req.temperature, perf_stat_dict, lang
                )
```
To:
```python
                vllm_stream = await asyncio.to_thread(
                    query_vllm_stream, query, docs, llm_endpoint, llm_model, req.stop, max_tokens, req.temperature, perf_stat_dict, lang, tokenizer_endpoint=vllm_tokenizer_endpoint
                )
```

Change:
```python
            vllm_non_stream = await asyncio.to_thread(
                query_vllm_non_stream, query, docs, llm_endpoint, llm_model, req.stop, max_tokens, req.temperature, perf_stat_dict, lang
            )
```
To:
```python
            vllm_non_stream = await asyncio.to_thread(
                query_vllm_non_stream, query, docs, llm_endpoint, llm_model, req.stop, max_tokens, req.temperature, perf_stat_dict, lang, tokenizer_endpoint=vllm_tokenizer_endpoint
            )
```

- [ ] **Step 8: Update digitize `ingest.py` to unpack the new return value**

In `spyre-rag/src/digitize/ingest.py` (line 41):

Change:
```python
        emb_model_dict, llm_model_dict, _ = get_model_endpoints()
```
To:
```python
        emb_model_dict, llm_model_dict, _, vllm_tokenizer_endpoint = get_model_endpoints()
```

Then find where `emb_model_dict["emb_endpoint"]` is passed to `process_documents` for tokenization purposes (line 49) and pass `vllm_tokenizer_endpoint` instead:

Change:
```python
        doc_chunks_dict, converted_pdf_stats = process_documents(
            input_file_paths, out_path, llm_model_dict['llm_model'], llm_model_dict['llm_endpoint'],  emb_model_dict["emb_endpoint"],
            max_tokens=emb_model_dict['max_tokens'] - 100, job_id=job_id, doc_id_dict=doc_id_dict)
```
To:
```python
        doc_chunks_dict, converted_pdf_stats = process_documents(
            input_file_paths, out_path, llm_model_dict['llm_model'], llm_model_dict['llm_endpoint'],  vllm_tokenizer_endpoint,
            max_tokens=emb_model_dict['max_tokens'] - 100, job_id=job_id, doc_id_dict=doc_id_dict)
```

- [ ] **Step 9: Update summarize `app.py` to unpack the new return value**

In `spyre-rag/src/summarize/app.py` (line 114):

Change:
```python
    _, llm_model_dict,_  = get_model_endpoints()
```
To:
```python
    _, llm_model_dict, _, _  = get_model_endpoints()
```

Summarize does not use tokenize/detokenize, so no further changes needed.

- [ ] **Step 10: Commit**

```bash
git add spyre-rag/src/common/misc_utils.py spyre-rag/src/common/llm_utils.py spyre-rag/src/chatbot/app.py spyre-rag/src/chatbot/backend_utils.py spyre-rag/src/digitize/ingest.py spyre-rag/src/summarize/app.py
git commit -s -S -m "feat: add VLLM_TOKENIZER_ENDPOINT for direct vLLM tokenize/detokenize calls"
```

---

### Task 7: Verify Existing Tests Pass

**Files:**
- Read: `spyre-rag/` test files
- Read: `ai-services/tests/e2e/`

- [ ] **Step 1: Find and run existing Python tests**

Run:
```bash
cd spyre-rag && find . -name "test_*.py" -o -name "*_test.py" | head -20
```

Then run whatever test suite exists:
```bash
python -m pytest src/ -v --tb=short 2>&1 | tail -30
```

Expected: All existing tests pass. If tests fail due to the `get_model_endpoints()` return value change (now 4 values instead of 3), update the test mocks to unpack 4 values.

- [ ] **Step 2: Verify Helm template rendering (OpenShift)**

Run:
```bash
cd ai-services && helm template test-release assets/applications/rag-dev/openshift/ 2>&1 | head -50
```

Expected: Templates render without errors. The `llm-gateway` deployment, service, and secret appear in the output.

- [ ] **Step 3: Commit any test fixes**

If any tests needed updating:
```bash
git add -u
git commit -s -S -m "fix: update tests for get_model_endpoints 4-value return"
```
