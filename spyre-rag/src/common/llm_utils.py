import logging
import requests
import time
import json
from concurrent.futures import ThreadPoolExecutor, as_completed
from tqdm import tqdm

import litellm

from common.lang_utils import prompt_map
from common.misc_utils import get_logger
from common.settings import get_settings

logger = get_logger("LLM")

is_debug = logger.isEnabledFor(logging.DEBUG)

def tqdm_wrapper(iterable, **kwargs):
    """Wrapper for tqdm that only shows progress bar in debug mode."""
    if is_debug:
        return tqdm(iterable, **kwargs)
    else:
        return iterable

settings = get_settings()

# Module-level session for vLLM tokenize/detokenize endpoints.
# These are called in tight loops (32 threads during chunking) and need
# connection pooling to prevent ephemeral port exhaustion.
_tokenize_session = None


def _get_tokenize_session():
    global _tokenize_session
    if _tokenize_session is None:
        from requests.adapters import HTTPAdapter
        adapter = HTTPAdapter(
            pool_connections=2,
            pool_maxsize=32,
            pool_block=True
        )
        session = requests.Session()
        session.mount("http://", adapter)  # nosemgrep: request-session-with-http — vLLM runs on internal network
        session.mount("https://", adapter)
        _tokenize_session = session
    return _tokenize_session


def create_llm_session(pool_maxsize=32, pool_connections: int = 2, pool_block: bool = True):
    """No-op kept for backward compatibility with existing callers."""
    pass


def classify_text_with_llm(text_blocks, gen_model, llm_endpoint, pdf_path, batch_size=32):
    all_prompts = [settings.prompts.llm_classify.format(text=item.strip()) for item in text_blocks]
    decisions = []

    with ThreadPoolExecutor(max_workers=batch_size) as executor:
        futures = {
            executor.submit(classify_single_text, prompt, gen_model, llm_endpoint): idx
            for idx, prompt in enumerate(all_prompts)
        }

        for future in tqdm_wrapper(as_completed(futures), total=len(all_prompts),
                                   desc=f"Classifying table summaries of '{pdf_path}'"):
            decisions.append(future.result())

    return decisions


def classify_single_text(prompt, gen_model, llm_endpoint):
    kwargs = {
        "model": gen_model,
        "messages": [{"role": "user", "content": prompt}],
        "temperature": 0,
        "max_tokens": 3,
        "num_retries": 3,
    }
    if llm_endpoint:
        kwargs["api_base"] = llm_endpoint

    response = litellm.completion(**kwargs)
    reply = response.choices[0].message.content.strip().lower()
    return "yes" in reply


def summarize_single_table(prompt, gen_model, llm_endpoint):
    kwargs = {
        "model": gen_model,
        "messages": [{"role": "user", "content": prompt}],
        "temperature": 0,
        "max_tokens": 512,
        "stream": False,
        "num_retries": 3,
    }
    if llm_endpoint:
        kwargs["api_base"] = llm_endpoint

    response = litellm.completion(**kwargs)
    reply = response.choices[0].message.content.strip().lower()
    return reply


def summarize_table(table_html, gen_model, llm_endpoint, pdf_path, max_workers=32):
    all_prompts = [settings.prompts.table_summary.format(content=html) for html in table_html]

    summaries = [None] * len(all_prompts)

    with ThreadPoolExecutor(max_workers=min(max_workers, len(all_prompts))) as executor:
        futures = {
            executor.submit(summarize_single_table, prompt, gen_model, llm_endpoint): idx
            for idx, prompt in enumerate(all_prompts)
        }
        for future in tqdm_wrapper(as_completed(futures), total=len(all_prompts), desc=f"Summarizing tables of '{pdf_path}'"):
            idx = futures[future]
            summaries[idx] = future.result()

    return summaries


def query_llm_models(llm_endpoint):
    """Query available models from a vLLM endpoint. Uses inline requests.get (no session)."""
    logger.debug('Querying LLM models')
    response = requests.get(f"{llm_endpoint}/v1/models")
    response.raise_for_status()
    return response.json()


def build_completion_kwargs(question, documents, llm_model, llm_endpoint, stop_words,
                            max_new_tokens, temperature, stream, lang):
    """Build litellm.completion() kwargs dict with dynamic context truncation."""
    context = "\n\n".join([doc.get("page_content") for doc in documents])

    logger.debug(f'Original Context: {context}')

    # dynamic chunk truncation: truncates the context if it doesn't fit in the sequence length
    question_token_count = len(tokenize_with_llm(question, llm_model, llm_endpoint))
    remaining_tokens = settings.max_input_length - (settings.prompt_template_token_count + question_token_count)
    context = detokenize_with_llm(
        tokenize_with_llm(context, llm_model, llm_endpoint)[:remaining_tokens],
        llm_model, llm_endpoint
    )
    logger.debug(f"Truncated Context: {context}")

    prompt_key = prompt_map.get(lang, "query_vllm_stream")
    prompt = getattr(settings.prompts, prompt_key).format(context=context, question=question)

    logger.debug("PROMPT:  ", prompt)

    kwargs = {
        "model": llm_model,
        "messages": [{"role": "user", "content": prompt}],
        "max_tokens": max_new_tokens,
        "temperature": temperature,
        "stop": stop_words,
        "stream": stream,
        "num_retries": 3,
    }
    if llm_endpoint:
        kwargs["api_base"] = llm_endpoint
    if llm_model.startswith("hosted_vllm/"):
        kwargs["repetition_penalty"] = 1.1
    if stream:
        kwargs["stream_options"] = {"include_usage": True}
    return kwargs


def _stream_litellm_to_sse(response, perf_stat_dict=None):
    """Convert a litellm streaming response to SSE format.

    Yields 'data: {json}\n\n' lines. Handles usage extraction,
    token latency tracking (when perf_stat_dict is provided),
    error wrapping, and the [DONE] sentinel.
    """
    try:
        token_latencies = []
        start_time = time.time()
        last_token_time = start_time

        for chunk in response:
            chunk_dict = chunk.model_dump()

            # Extract usage from final chunk
            if chunk.usage is not None:
                if perf_stat_dict is not None:
                    perf_stat_dict["completion_tokens"] = chunk.usage.completion_tokens or 0
                    perf_stat_dict["prompt_tokens"] = chunk.usage.prompt_tokens or 0

            # Track latency and yield for actual content chunks
            if chunk.choices:
                now = time.time()
                token_latencies.append(now - last_token_time)
                last_token_time = now
                yield f"data: {json.dumps(chunk_dict)}\n\n"

        request_time = time.time() - start_time
        if perf_stat_dict is not None:
            perf_stat_dict["token_latencies"] = token_latencies
            perf_stat_dict["inference_time"] = request_time

    except Exception as e:
        logger.error(f"Error during LLM streaming: {e}")
        yield f"data: {json.dumps({'error': str(e)})}\n\n"

    yield "data: [DONE]\n\n"


def query_llm_non_stream(question, documents, llm_endpoint, llm_model, stop_words,
                         max_new_tokens, temperature, perf_stat_dict, lang):
    kwargs = build_completion_kwargs(
        question, documents, llm_model, llm_endpoint, stop_words,
        max_new_tokens, temperature, False, lang
    )

    start_time = time.time()
    response = litellm.completion(**kwargs)
    request_time = time.time() - start_time
    perf_stat_dict["inference_time"] = request_time

    if response.usage:
        perf_stat_dict["completion_tokens"] = response.usage.completion_tokens or 0
        perf_stat_dict["prompt_tokens"] = response.usage.prompt_tokens or 0

    return response.model_dump()


def query_llm_stream(question, documents, llm_endpoint, llm_model, stop_words,
                     max_new_tokens, temperature, perf_stat_dict, lang):
    kwargs = build_completion_kwargs(
        question, documents, llm_model, llm_endpoint, stop_words,
        max_new_tokens, temperature, True, lang
    )

    response = litellm.completion(**kwargs)
    yield from _stream_litellm_to_sse(response, perf_stat_dict)


def query_llm_summarize(
    llm_endpoint: str,
    messages: list,
    model: str,
    max_tokens: int,
    temperature: float,
):
    stop_words = [w for w in settings.summarization_stop_words.split(",") if w]

    kwargs = {
        "model": model,
        "messages": messages,
        "max_tokens": max_tokens,
        "temperature": temperature,
        "num_retries": 3,
    }
    if llm_endpoint:
        kwargs["api_base"] = llm_endpoint
    if stop_words:
        kwargs["stop"] = stop_words

    response = litellm.completion(**kwargs)

    content = ""
    input_tokens = 0
    output_tokens = 0
    if response.choices:
        content = response.choices[0].message.content or ""
        if response.usage:
            input_tokens = response.usage.prompt_tokens or 0
            output_tokens = response.usage.completion_tokens or 0
    return content.strip(), input_tokens, output_tokens


def query_llm_summarize_stream(
    llm_endpoint: str,
    messages: list,
    model: str,
    max_tokens: int,
    temperature: float,
):
    """Stream a summarization request, yielding SSE lines."""
    stop_words = [w for w in settings.summarization_stop_words.split(",") if w]

    kwargs = {
        "model": model,
        "messages": messages,
        "max_tokens": max_tokens,
        "temperature": temperature,
        "stream": True,
        "stream_options": {"include_usage": True},
        "num_retries": 3,
    }
    if llm_endpoint:
        kwargs["api_base"] = llm_endpoint
    if stop_words:
        kwargs["stop"] = stop_words

    response = litellm.completion(**kwargs)
    yield from _stream_litellm_to_sse(response)


def tokenize_with_llm(prompt, model, endpoint=None):
    """Tokenize text using vLLM endpoint (when available) or litellm.encode().

    Args:
        prompt: Text to tokenize
        model: litellm model string (e.g., 'hosted_vllm/ibm-granite/granite-3.3-8b-instruct')
        endpoint: Optional vLLM endpoint URL. When set, uses vLLM's /tokenize
                  directly (no HuggingFace download needed).

    Returns:
        List of token IDs
    """
    if endpoint:
        session = _get_tokenize_session()
        response = session.post(f"{endpoint}/tokenize", json={"prompt": prompt})
        response.raise_for_status()
        return response.json().get("tokens", [])
    else:
        result = litellm.encode(model=model, text=prompt)
        # litellm.encode returns a list of token IDs
        return result


def detokenize_with_llm(tokens, model, endpoint=None):
    """Detokenize tokens using vLLM endpoint (when available) or litellm.decode().

    Args:
        tokens: List of token IDs to detokenize
        model: litellm model string
        endpoint: Optional vLLM endpoint URL

    Returns:
        Detokenized text string
    """
    if endpoint:
        session = _get_tokenize_session()
        response = session.post(f"{endpoint}/detokenize", json={"tokens": tokens})
        response.raise_for_status()
        return response.json().get("prompt", "")
    else:
        return litellm.decode(model=model, tokens=tokens)
