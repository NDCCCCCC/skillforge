# SkillForge

> **Same agent kernel, customized per enterprise via pluggable Skill files — without code changes.**

SkillForge is an open-source, Skill-driven enterprise Agent platform. The same Go + Eino ADK agent core adapts to any business domain by editing or adding a `SKILL.md` file — no recompilation, no code changes.

The same kernel serves legal review, HR policy, IT helpdesk, document-grounded QA, or any business domain — swap in your own `SKILL.md` and the new domain is live. Three ready-to-run templates ship in [`examples/skills/`](examples/skills/) — `simple-qa`, `calculator`, and `compliance` — and [`CONTRIBUTING.md`](CONTRIBUTING.md) walks you through writing your own.

![SkillForge demo](docs/demo.gif)

[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![CI](https://img.shields.io/badge/CI-pending%20%2807--05%29-yellow)](.github/workflows/)

---

## Quick Start

Get a document-grounded, cited answer running in your browser in under 10 minutes.

### Prerequisites

| Dependency | Why | Minimum |
|------------|-----|---------|
| **Docker** (with Compose v2) | Runs the full stack (Milvus + backend + frontend) | Docker Desktop or Docker Engine |
| **Ollama** (or any OpenAI-compatible embedding endpoint) | Serves the BGE-M3 embedding model | `ollama pull bge-m3` |
| **GLM API key** (Zhipu Coding Plan) | Powers the LLM + the trust evaluator judge | [open.bigmodel.cn](https://open.bigmodel.cn/) |
| **~8 GB RAM** | Milvus standalone + Go backend + nginx | Milvus alone uses ~4 GB |

### Steps

```bash
# 1. Clone
git clone https://github.com/ninedrunk/skillforge.git
cd skillforge

# 2. Configure environment
cp .env.example .env
# Edit .env and fill in:
#   GLM_API_KEY=<your Zhipu key>
#   BGE_M3_URL=http://localhost:11434/v1/embeddings   (default; ollama)

# 3. Start the embedding endpoint (local minimum)
ollama pull bge-m3

# 4. The repo already ships the minimal QA Skill as the live default
#    (skills/simple-qa) — nothing to do. To swap the demo domain, drop
#    any examples/skills/ template into skills/ and remove the old one,
#    e.g. the cited-verdict compliance demo:
#      cp -r examples/skills/compliance skills/ && rm -r skills/simple-qa
#    Restart (or let hot-reload pick it up) for the change to take effect.

# 5. Bring up the full stack (Milvus + backend + frontend)
docker compose up

# 6. Open the app
#    Browser: http://localhost:8080
```

### Try it

Open **http://localhost:8080**, pick the **simple-qa** Skill, and ask a question about the loaded documents, e.g.:

> 差旅住宿费的报销标准是什么？

You should see either a **cited answer** (with clickable chunk references) or a **structured refusal** (e.g. `no_relevant_source`) — never a fabricated answer.

That is the platform in action: one kernel, one Markdown Skill file. Swap in a different template — the `calculator` or `compliance` Skill from `examples/skills/` — and the same kernel runs a different behavior, with a different knowledge boundary and a different set of guardrails.

The backend is also directly reachable at `http://localhost:8888` for `curl` probes, and Milvus gRPC is at `localhost:19530`.

---

## Architecture

`docker compose up` brings up five services plus two external endpoints. The three moats — **真源分离 tier-filter**, **Skill-as-platform**, and the **Go-native trust evaluator** — are labeled below.

```mermaid
flowchart TD
    subgraph compose["docker compose up (5 services)"]
        etcd["etcd<br/>(metadata)"]
        minio["minio<br/>(object storage)"]
        milvus["Milvus v2.4.x<br/>(vector DB, RocksMQ)"]
        backend["Go backend<br/>(Hertz :8888)<br/>─── Skill-as-platform ───<br/>pluggable SKILL.md<br/>no code changes"]
        frontend["nginx frontend<br/>(:8080 → :80)"]
    end

    etcd --> milvus
    minio --> milvus
    milvus -->|"真源分离 tier-filter<br/>PRE-similarity metadata filter<br/>(Moat 1)"| backend
    backend -->|"/api proxy_pass<br/>SSE UIMessageChunk stream"| frontend
    frontend --> browser["🌍 Browser<br/>localhost:8080<br/>cited verdict / refusal card"]

    ollama["External: BGE-M3<br/>(ollama, embedding)"] -.->|"BGE_M3_URL"| backend
    glm["External: GLM API<br/>(Zhipu, LLM + judge)"] -.->|"GLM_API_KEY"| backend
    backend -.->|"trust evaluator<br/>Go-native, Ragas 口径<br/>(Moat 3)"| glm

    classDef moat fill:#fff3cd,stroke:#856404,stroke-width:2px;
    class backend glm milvus moat;
```

**The demo Skill is replaceable, not the product.** The Milvus tier-filter, the Skill registry, and the trust evaluator are domain-agnostic. All of the demo behavior — the retrieval boundary, the answer format, the refusal guardrails — lives in a single 30-line Markdown file ([`skills/simple-qa/SKILL.md`](skills/simple-qa/SKILL.md)). Delete it and SkillForge has no domain-specific logic left to remove.

---

## Why SkillForge? — The Three Moats

These three already-validated mechanisms are the defensible core. Each is proven by a spike or phase success criterion.

### 1. 真源分离 Tier-Filter (True-Source Separation)

Knowledge is split into four tiers — **权威** (authority), **历史** (historical), **个人** (personal), **草稿** (draft). Each Skill declares which tiers it may read via the `reads` field. The retriever applies a **metadata filter before similarity search** (`tier IN [...]`), so an authority-only Skill physically cannot see draft or personal notes — the trust boundary is enforced in the database query, not in prompt pleading. Validated by Phase 1 SC-2 (Milvus tier-filter) and the default-deny validator (`internal/skill.Validate` rejects empty `reads`).

### 2. Skill-as-Platform

The agent kernel is fixed Go + Eino ADK code. Adapting SkillForge to a new enterprise domain (legal review, HR policy, IT helpdesk) means **authoring or editing a `SKILL.md` file** — never touching Go source. Skills are hot-reloaded: drop a file in `skills/`, the registry picks it up, intent routing vectors the description, and the new domain is live. This turns SkillForge from a single "compliance app" into a **platform** — and is the direct counter to the "AI wrapper is dead" lesson (Height.app closed 2025.9 because it was a feature, not a platform).

### 3. Go-Native Trust Evaluator

Hallucination is the #1 failure mode for enterprise RAG. SkillForge ships a single-language Go evaluator (`internal/eval` + `cmd/eval`) that scores every Skill on four RAG-trust metrics. The **faithfulness** metric ports the Ragas v0.1.21 claim-extraction + entailment prompts **verbatim** (Apache-2.0, attributed in `eval/NOTICE`) so the headline number is industry-comparable ("Ragas 口径"); the other three metrics use Go-authored judge prompts. This keeps the stack single-language (no Python sidecar) while remaining benchmark-comparable. Validated by Phase 3 SC-6 (reproducibility via frozen lock + StubJudge).

---

## Skills

Skills are declarative Markdown + YAML frontmatter files. Each defines a 7-field manifest (`description` / `agent` / `reads` / `writes` / `steps` / `guardrails` / `temperature`) that the kernel loads, validates, and routes against.

- **Templates:** [`examples/skills/`](examples/skills/) ships three ready-to-run Skills — `simple-qa` (minimal authority-tier QA), `calculator` (tool-calling), and `compliance` (cited verdicts + structured refusals). Each passes the schema validator out of the box.
- **Loaded Skills:** the kernel loads whatever is under [`skills/`](skills/) — the repo default is [`skills/simple-qa/SKILL.md`](skills/simple-qa/SKILL.md); drop any template there (and restart or let hot-reload pick it up) to make it the live domain.

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for how to write your own Skill — the format is 7 fields of YAML frontmatter plus Markdown steps.

---

## Eval Results

This snapshot is the **frozen v0.8 compliance baseline** (Phase 8 pin — `eval/frozen/compliance.lock.json`, sha256 `5cf700f3…`). Every figure below is rendered verbatim by `go run ./scripts/eval_snapshot` from `eval/report.json` + the lock; none are hand-typed. Regenerating the lock is tampering (T-8-12) — it pins the model, prompt hash, and seed so the run is byte-identically reproducible.

Frozen run on **2026-08-21** · `eval_set_version: compliance-2026-08-13-v1` · `model_ver: glm-5.2` · `prompt_hash: 58ffea08…`

| Skill | Faithfulness | Context Precision | Refusal Accuracy | Cases |
|-------|-------------|-------------------|------------------|-------|
| compliance | 97% | 10% | 95% (19/20) | 50 (20 adversarial / 20 refusal-required / 10 multi-hop) |

**Reproduce:** `go run ./cmd/eval --frozen=compliance`

The frozen lock (`eval/frozen/compliance.lock.json`) pins the model, prompt hash, and seed for byte-identical reproducibility (SC-6).

---

## Contributing

Contributions are welcome — especially **new Skills** that prove the platform thesis. SkillForge is designed so that the highest-leverage contribution is a domain `SKILL.md`, not Go code.

See **[CONTRIBUTING.md](CONTRIBUTING.md)** for guides on:
- Writing a Skill (the 7-field schema + validation)
- Adding a tool (the Eino `tool.InvokableTool` interface)
- Adding a knowledge tier (the `TierEnum` + `knowledge/` convention)

The standard flow is fork → branch → PR. Run `gofmt` and `go test ./...` before opening a PR.

---

## License

**Apache-2.0** — see [LICENSE](LICENSE).

The faithfulness evaluator ports Ragas v0.1.21 prompts (Apache-2.0); attribution is in [`eval/NOTICE`](eval/NOTICE).
