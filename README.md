# SkillForge

> **Execution-time trust boundaries for AI agents — 什么不许答、什么必须批.**
> We sit alongside WeKnora (查得准) and Semantica (为什么) — three complementary
> lanes in the open-source AI agent stack.

**What is a Skill here?** A `SKILL.md` file in SkillForge is a *domain
adaptation contract* (knowledge tier boundaries + write targets +
refusal guardrails + system prompt). It is **not** a sandboxed skill
plugin (the WeKnora usage) and not an MCP tool (the Anthropic Agent
Skills usage). One `SKILL.md` retargets the same agent kernel to
legal review, HR policy, IT helpdesk, or any other business domain
— no recompilation, no code changes. Three ready-to-run templates
ship in [`examples/skills/`](examples/skills/) — `simple-qa`,
`calculator`, and `compliance` — and
[`CONTRIBUTING.md`](CONTRIBUTING.md) walks you through writing your own.

![SkillForge demo](docs/demo.gif)

[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![CI](https://img.shields.io/badge/CI-eval--gate--22--09--06-green)](.github/workflows/)

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

These three already-validated mechanisms are the defensible core of v1.1. Each is proven by a spike or phase success criterion; each is **not yet offered by the two mature competitors** in our lane (see comparison below).

### 1. Approve-and-Remember as a first-class primitive

HITL write actions (Phase 11) bind operator approval to **byte-identical tool-call args** (`frozen_args` SHA-256) scoped to `(session, skill)`. A rejected argument cannot silently drift into an approved one (D11-C4 byte-identity invariant). Approved actions become **remembered rules** (`approval_rules` table) — the same `(target, args_hash, skill_id, scope_session_id)` PK, written by `INSERT … SELECT` in one tx with the audit row (`action_rule_created`). Revocation is `DELETE + action_rule_revoked` in another tx — T-3-AUDIT INSERT-only ledger, never an UPDATE.

| Pattern | SkillForge | Semantica | WeKnora |
|---------|-----------|-----------|---------|
| Pre-execution approval | ✅ `tool_call_id == approval_id` (D11-C4) | ❌ decision intelligence is post-hoc causal trace | ❌ no approval gate |
| Byte-identity binding | ✅ `frozen_args` SHA-256 | ❌ n/a (no gate) | ❌ n/a |
| Approve-and-remember rule | ✅ `(target, args_hash, skill_id, scope_session_id)` PK | ❌ n/a | ❌ n/a |
| Revocation is audit | ✅ `DELETE + action_rule_revoked` one tx | ❌ n/a | ❌ n/a |

Validated by `internal/approval_api_test.go` (D11-D3 / D11-D4 / D11-C4).

### 2. Trust Contract Kernel — refuse-by-construction

The agent kernel enforces four trust primitives **before any LLM call**, not by prompt pleading:

1. **Tier-filter at the database query** — `tier IN [...]` is a Milvus metadata filter; an authority-only Skill physically cannot see draft / personal notes.
2. **Refuse-by-construction** — when no chunk matches the declared `reads` filter, the answer is `no_authorized_source` *before* the verdict prompt. The agent never sees an ungrounded question.
3. **Citation whitelist = this retrieval round** — every answer carries the chunks that justified it. Adding chunks after the fact is impossible (the verifier checks the exact set).
4. **Default-deny on missing fields** — `internal/skill.Validate` rejects Skills with empty `reads`/`writes`/`steps` at load time. The trust boundary is the schema, not the operator's discipline.

This is why the trust floor matters in production: WeKnora's hybrid search gets more relevant chunks per query, but a WeKnora Skill without the SkillForge-style `reads` field can still hallucinate against the user's private notes. SkillForge physically cannot.

### 3. CI-Integrated Trust Gate (Skill 晋级闭环)

Every Skill promotion runs through `.github/workflows/eval-gate.yml`: frozen compliance replay (50 cases) → `cmd/eval --gate-check` against per-Skill thresholds (`examples/skills/<skill>/eval-gate.json`) → fail-closed exit code. Headline: faithfulness ≥ 0.9 / refusal-accuracy ≥ 0.95 — the gate that catches a Skill regression **before** it ships. Cross-judge + planned human-judge calibration (Phase 22-02) close the self-eval bias loop.

**评测器定位声明（Phase 22）**

The 4 metrics are a verbatim port of Ragas v0.1.21 (Apache-2.0, attributed in `eval/NOTICE`). SkillForge does **not** ship an independent evaluation platform — `internal/eval` is the **internal infrastructure** that backs the Skill promotion gate, and nothing more. If you want a standalone RAG evaluation suite, use [Ragas](https://docs.ragas.io/) directly (or DeepEval / TruLens); we deliberately do not compete in that lane.

| Aspect | Ragas / DeepEval / TruLens | SkillForge `internal/eval` |
|--------|---------------------------|----------------------------|
| Goal | Standalone RAG evaluation platform | Skill promotion gate inside the platform |
| Metrics | 4+ proprietary | 4 verbatim-port (Ragas v0.1.21) |
| Calibration set | User-supplied | 50-case frozen compliance golden (`.planning/phases/22-eval-ci/`) |
| Pin / lock | Optional | **Required** (`AssertPin` + frozen replay) |
| CI integration | User wires their own | First-class (`.github/workflows/eval-gate.yml`) |
| Cross-judge | Via LLM swap | `--judge-models glm-5.2,deepseek` + planned human-judge calibration (Phase 22-02) |
| License | Ragas Apache-2.0 / DeepEval Apache-2.0 / TruLens MIT | In-tree, no sidecar |

**Single-language posture**: keeping the evaluator in Go (no Python sidecar, no Ragas pip install) is a deployment convenience, not a competitive moat. We document this transparently because the value is the **CI integration + Skill promotion loop**, not the metric implementation.

---

## SkillForge vs WeKnora vs Semantica — three lanes, not three head-to-head rivals

| | SkillForge | WeKnora (Tencent) | Semantica (semantica-agi) |
|---|---|---|---|
| **Lane** | 什么不许答 / 什么必须批 — execution-time trust boundaries | 文档怎么查得准 — retrieval + multi-tenant RBAC | 决策为什么 — knowledge fact provenance + causal graph |
| **Differentiator** | approve-and-remember primitive + tier-filter at DB + refusal-by-construction + frozen-pin CI gate | hybrid retrieval quality (RAG + Wiki Mode), Go single-binary deployment, 4-level RBAC | W3C PROV-O standard for fact / decision tracing + time-travel replay |
| **Agent audit** | INSERT-only execution ledger (Phase 11 + 24); PROV-O export (Phase 24-01) | workspace audit log + AES-256-GCM at rest | PROV-O source of truth (their first-class citizen) |
| **Write actions** | HITL approval gate with frozen-args byte-identity + remember rule | ❌ no approval gate | ❌ decision intelligence is post-hoc, not pre-execution |
| **Eval posture** | In-tree Go gate (Ragas v0.1.21 verbatim port) | external benchmark suites | external |
| **License** | Apache-2.0 | MIT | MIT |
| **"Skill" means** | domain adaptation contract (one file = new domain) | sandboxed skill plugin (Docker / E2B / Cube) | n/a |

We are **complementary, not competing**:

- If you want **better document recall** → use WeKnora.
- If you want **why-the-agent-decided-X explanations** → use Semantica.
- If you want **an agent that physically cannot leak private data, cannot run unapproved writes, and cannot ship a regressed Skill** → use SkillForge.

**Support, not head-to-head.** SkillForge ships **PROV-O export** (`GET /api/session/:id/audit/prov`, Phase 24-01) so Semantica users can import SkillForge's execution audit as PROV-O entities. We view audit / decision-trace as a **supporting capability**, not a headline; Semantica is the leader there and we acknowledge it.

---

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
