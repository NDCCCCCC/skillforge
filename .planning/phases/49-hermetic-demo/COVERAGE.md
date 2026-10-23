# API Coverage — BGE-M3 embeddings endpoint (OpenAI-compatible)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.
>
> Phase 49 integrates ONE new external API surface: the BGE-M3 embeddings HTTP
> endpoint, reached through the existing `internal/embedder/bge` client
> (`Config.URL` / `Config.Model` / `Config.AuthToken` / `Config.Timeout` /
> `Config.InsecureSkipVerify`). D-01 forbids any vector fallback, so this
> surface is load-bearing: if it is not covered, the whole phase has no
> retrieval path.
>
> The baseline is the full capability surface of an OpenAI-compatible
> embeddings service, not the subset the current client happens to call. The
> `embedder.Embedder` interface (`Embed(ctx, text) ([]float32, error)` +
> `Dimensions() int`) is the narrow part; the rows below are the wide part.

| capability | decision | reason |
|---|---|---|
| `POST /v1/embeddings` — single-string `input` | INTEGRATE | the only call shape `embedder.Embedder.Embed` makes; verified against the real model in the fresh-clone probe. |
| `POST /v1/embeddings` — batch `input` array | OPT-OUT | `Embed` is one text per call; batching would change the interface for no measured need — revisit only if indexing time becomes a measured problem. |
| `POST /v1/embeddings` — response shape `{"data":[{"embedding":[…]}]}` (OpenAI) | INTEGRATE | the primary parse branch in `bge.go`; covered by the real-model probe and the shape unit tests. |
| `POST /v1/embeddings` — response shape `[{"embedding":[…]}]` (raw TEI) | INTEGRATE | the fallback parse branch in `bge.go`; kept because TEI serves this shape verbatim. |
| `POST /v1/embeddings` — `dimensions` request parameter | OPT-OUT | BGE-M3 dense output is fixed at 1024 (`dimensions = 1024` in `internal/embedder/bge/bge.go`); asking the server for a size the model cannot produce would be a lie to the endpoint. |
| `POST /v1/embeddings` — `encoding_format: "base64"` | OPT-OUT | the client parses a JSON float array; base64 is a transport optimisation with no correctness effect. |
| `POST /v1/embeddings` — sparse / multi-vector (ColBERT-style) output | OPT-OUT | the retriever's contract is one dense vector per chunk; sparse output would change `embedder.Embedder` and every consumer of it. |
| `GET /v1/models` — model discovery | OPT-OUT | the model name is configuration (`Config.Model`, set for Ollama to `bge-m3`); discovering it at runtime would make the cache key non-deterministic (D-14 requires the model in the cache key). |
| `GET /v1/models/{id}` | OPT-OUT | same reason as the collection listing. |
| `POST /api/embed` — Ollama native (non-OpenAI-compatible) surface | OPT-OUT | D-02 pins the OpenAI-compatible `/v1/embeddings` shape; Ollama serves that shape natively, so a second Ollama-specific client would be a second HTTP client for one endpoint. |
| `GET /health` (or `GET /`) — endpoint liveness probe | OPT-OUT | the embed call itself observes liveness; a down endpoint is a non-200/transport error and, per D-01, an explicit refusal — a second probe adds surface, not information. |
| `POST /v1/rerank` — cross-encoder reranking | OPT-OUT | reranking is not in the phase boundary; the embedded store reuses the existing cosine + deterministic tie-break ordering (D-04). |
| Bearer authentication (`Config.AuthToken` / `BGE_M3_AUTH`) | INTEGRATE | `bge.go` sets the header when configured; needed for hosted endpoints that require a key. |
| TLS verification opt-out (`BGE_M3_TLS_SKIP_VERIFY`) | INTEGRATE | per-instance client keeps the opt-out scoped; supports self-hosted endpoints with private CAs. |

# API Coverage — GLM chat endpoint (pre-existing surface, re-decided for this phase)

> Phase 49 does **not** add a second LLM integration, but it does change the
> credential requirement of the existing one: `GLM_API_KEY` / `LOCAL_API_KEY`
> becomes optional and the no-key path routes to the deterministic stub
> extractor (D-07, D-09, D-11). The rows below are therefore re-decided
> against the same full-coverage baseline rather than inherited.
>
> Endpoint: `GLM_BASE_URL` / `LOCAL_BASE_URL` (default
> `https://open.bigmodel.cn/api/coding/paas/v4`), model `GLM_MODEL` /
> `LOCAL_MODEL` (default `glm-5.3-flash`), key `GLM_API_KEY` / `LOCAL_API_KEY`.

| capability | decision | reason |
|---|---|---|
| `POST {base}/chat/completions` — chat completion | INTEGRATE | the pre-existing verdict path (`toolChatModel`); re-decided because the key became optional (D-11). |
| `POST {base}/chat/completions` — streaming (`stream: true`) | OPT-OUT | the verdict path consumes a single complete JSON answer; the SSE surface belongs to the transport layer, not to this phase's stub decision. |
| `POST {base}/chat/completions` — structured output / response_format | OPT-OUT | the existing client already extracts JSON via `llm.ExtractJSON`; adding a response_format contract is a separate change with its own compatibility surface. |
| `POST {base}/responses` — Responses API | OPT-OUT | the client targets the Chat Completions surface; switching API generations is not needed for the no-key path. |
| `GET {base}/models` — model discovery | OPT-OUT | the model is configuration (`GLM_MODEL` / `LOCAL_MODEL`), consistent with the BGE decision above. |
| `POST {base}/embeddings` | OPT-OUT | D-01 pins BGE-M3 as the single external embedder; a second provider embedder reopens the "which embedder" question this phase closed. |
| `POST {base}/rerank` | OPT-OUT | reranking is outside the phase boundary; retrieval keeps the existing cosine + deterministic tie-break (D-04). |
