# Contributing to SkillForge

Thanks for your interest in contributing! SkillForge is built around a simple thesis: **the highest-leverage contribution is a domain `SKILL.md`, not Go code.** Most contributions land in `skills/` or `knowledge/`. This guide covers the three most common contribution patterns.

## How to position SkillForge in PRs, docs, and talks

SkillForge sits alongside (not against) two mature open-source agents:

| Lane | Project | What they own |
|---|---|---|
| 查得准 — retrieval + multi-tenant RBAC | WeKnora (Tencent) | Hybrid retrieval, Go single-binary, 4-level RBAC |
| 为什么 — decision provenance + causal graph | Semantica (semantica-agi) | W3C PROV-O source of truth, time-travel replay |
| **什么不许答、什么必须批 — execution-time trust boundaries** | **SkillForge (this repo)** | **Approve-and-remember primitive, tier-filter at DB, refusal-by-construction, CI-Integrated Trust Gate** |

When writing PR descriptions, docs, blog posts, or talks about SkillForge:

- ✅ **Lead with**: approve-and-remember primitive · tier-filter at the database query · refusal-by-construction · frozen-pin CI gate · PROV-O export (we ship it; we don't pretend it's our lane leader).
- ⚠️ **Don't claim**: "Go 企业 RAG 平台" (WeKnora is the leader there), "best RAG retrieval" (not our focus), "best evaluation suite" (Ragas / DeepEval / TruLens are).
- ⚠️ **Don't say**: "Skill is a sandboxed skill plugin" (WeKnora usage) or "Skill is an MCP tool" (Anthropic Agent Skills usage). In SkillForge, a Skill is a **domain adaptation contract** — one `SKILL.md` retargets the kernel to a new business domain. README has the de-confusion note at the top.

If a feature PR doesn't fit one of the three lanes above, that's a signal it may not belong in SkillForge.

## Fork + PR Flow

1. **Fork** the repo and clone your fork.
2. **Branch** off `main`: `git checkout -b feat/my-skill`.
3. **Make your change** (follow one of the three guides below).
4. **Validate locally**:
   ```bash
   gofmt -w .
   go test ./...
   go test ./internal/skill -run TestExamplesValidate   # if you added/edited a SKILL.md
   ```
5. **Open a PR** describing what you added and which Skill/tier/tool it touches.

### Code Style

- **Go:** `gofmt` formatting is mandatory. Follow the existing package conventions (exported identifiers get doc comments; error messages name the failing field + rule).
- **License header:** every Go file begins with the Apache-2.0 SPDX header block (see any file in `cmd/` or `internal/` — the canonical form is below). New Go files must carry it.
  ```go
  // Copyright 2026 SkillForge Authors
  //
  // Licensed under the Apache License, Version 2.0 (the "License");
  // you may not use this file except in compliance with the License.
  // You may obtain a copy of the License at
  //
  //     http://www.apache.org/licenses/LICENSE-2.0
  //
  // Unless required by applicable law or agreed to in writing, software
  // distributed under the License is distributed on an "AS IS" BASIS,
  // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  // See the License for the specific language governing permissions and
  // limitations under the License.
  //
  // SPDX-License-Identifier: Apache-2.0
  ```
- **Skill files:** YAML frontmatter is validated by `internal/skill.Validate` — see the rules below.

---

## Write a Skill

A Skill is a single `SKILL.md` file in `skills/<name>/`. It declares everything the kernel needs to route, run, and guardrail a domain — no Go code required. The manifest has **7 required fields**, validated at load time by [`internal/skill.Validate`](internal/skill/validate.go).

### The 7-Field Schema

| Field | Type | Rule |
|-------|------|------|
| `description` | string | Non-empty intent-routing text (the router matches user queries against this). |
| `agent` | string | One of the 6 Eino ADK agent types (see below). |
| `reads` | `[]string` | **Non-empty** knowledge-base patterns (default-deny): `authority/**` or a role wildcard `role:authority`. No `*` wildcard. |
| `writes` | `[]string` | Present (use `[]` for read-only Skills). |
| `steps` | string (block scalar) | Non-empty declarative ordered list. |
| `guardrails.prohibitions` | `[]string` | At least 3 entries. |
| `temperature` | **`*float32`** (pointer) | Required — no implicit default. Must be in `[0.0, 1.0]`. |

**Valid `agent` values** (from `validAgentTypes` in [`internal/skill/manifest.go`](internal/skill/manifest.go)):

```
ChatModelAgent, ReActAgent, SequentialAgent, ParallelAgent, LoopAgent, ConditionalAgent
```

### Minimal Skill Skeleton

Drop this into `skills/my-domain/SKILL.md` and edit the domain-specific parts:

```yaml
---
description: <one sentence: what this Skill answers, and what it does NOT handle>
agent: ChatModelAgent
reads:
  - authority/**
writes: []
steps: |
  1. Retrieve relevant chunks via the reads tier filter
  2. Compose a draft answer with citations (chunk_id + verbatim span)
  3. Refuse if no authorized chunks retrieved
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved from the authorized tier"
    - "Never paraphrase critical clauses (quote verbatim span)"
    - "Never reveal personal or contact information"
  required_refusal_phrases:
    - "no_authorized_source"
  tier_limits:
    authority: ["read-only"]
temperature: 0.3
---

# My Domain Skill

<prose describing the Skill's scope and refusal policy>
```

### Two Gotchas

1. **`reads` must be non-empty.** The validator rejects an empty `reads` slice (default-deny — `P-SKILL-3`). Every Skill must declare at least one knowledge base it may read. If your Skill is purely tool-driven with no knowledge source, declare a draft-role KB as a placeholder (e.g. `draft/**`) and note it in prose — a draft-role KB never enters retrieval (KBM-03: `kb.ResolveReads` drops draft-role bases from the resolved set), so the declaration satisfies the non-empty contract while granting zero retrievable source. Do **not** relax the validator.

2. **`temperature` is a required pointer.** The field is `*float32`, not `float32`. This is intentional: a plain `float32` silently decodes a missing YAML key as `0.0`, defaulting the Skill to deterministic mode without the author knowing (`SYSK-08`). You **must** declare an explicit value — `temperature: 0.0` for compliance/data Skills, `temperature: 0.7+` for creative Skills.

### Validate Before PR

The example Skills in `examples/` are validated by a regression test. Run it against your own Skill:

```bash
go test ./internal/skill -run TestExamplesValidate
```

This calls `internal/skill.Validate` on every `examples/skills/*/SKILL.md`. To validate your new Skill, either drop it under `examples/skills/` temporarily, or write a one-off test that calls `skill.Validate` on your parsed manifest. The error messages name the failing field + rule, so fixing is straightforward.

For complete reference Skills, see the three validated templates under [`examples/skills/`](examples/skills/) and the loaded copy at [`skills/compliance/SKILL.md`](skills/compliance/SKILL.md).

---

## Add a Tool

Skills that need autonomous data lookup (beyond the built-in retrieve tool) call **Eino tools**. A tool is a `tool.InvokableTool` built via `utils.NewTool` from the `github.com/cloudwego/eino/components/tool` package.

### The Reference Implementation

The built-in retrieve tool is the canonical pattern — see [`internal/agent/retrieve_tool.go`](internal/agent/retrieve_tool.go). It:

1. Declares a `schema.ToolInfo` (name, description, JSON-param schema).
2. Defines a generic `InvokeFunc[Args, Response]` closure that closes over its dependencies (embedder, retriever, tier slice).
3. Returns `utils.NewTool(desc, invoke)`.

### Minimal Tool Shape

```go
package agent

import (
    "context"

    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/components/tool/utils"
    "github.com/cloudwego/eino/schema"
)

const MyToolName = "my_calculator"

type myToolArgs struct {
    Expression string `json:"expression"`
}

type myToolResponse struct {
    Result string `json:"result"`
}

// BuildMyTool constructs the tool. It closes over whatever dependencies
// the tool needs (here: none — a pure calculator). Thread-safe tools
// must not share mutable state without synchronization.
func BuildMyTool() tool.InvokableTool {
    desc := &schema.ToolInfo{
        Name: MyToolName,
        Desc: "Evaluate a numeric expression and return the exact result.",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "expression": {
                Type:     schema.String,
                Desc:     "The arithmetic expression to evaluate.",
                Required: true,
            },
        }),
    }

    invoke := func(ctx context.Context, args myToolArgs) (myToolResponse, error) {
        // ... your logic ...
        return myToolResponse{Result: "42"}, nil
    }

    return utils.NewTool(desc, invoke)
}
```

### Notes

- **Tier access is closed over at construction.** The retrieve tool snapshots `manifest.ResolvedTiers()` once when built — the LLM cannot override which tiers a tool reads (`KNOW-03`). Follow the same pattern for any tool that touches knowledge: pass the resolved tier slice in at construction, never accept it from LLM-emitted args.
- **Tool-only Skills (calculator-style):** declare a draft-role KB in `reads` (e.g. `draft/**`) so the 7-field validator passes — a draft-role KB never enters retrieval (KBM-03), so this grants no source. The runtime today is a RAG pipeline; pure tool-execution support is a future version. See `examples/skills/calculator/SKILL.md` for the pattern.

---

## Add a Knowledge Base

SkillForge organizes knowledge into **knowledge bases (KBs)** registered in the knowledge-base registry (Phase 26). A KB is a registry entry — a **slug**, a display name, and one of four semantic **roles** — paired with a directory under `knowledge/` whose name is the slug. Adding a new KB means registering it and creating the directory; no Go code changes.

### The Four Semantic Roles

Roles are platform constants (see `Roles()` in [`internal/kb/kb.go`](internal/kb/kb.go)), not configuration. They decide trust, not just filtering:

| Directory (`knowledge/<slug>/`) | Role | Retrieval | Citable |
|--------------------------------|------|-----------|---------|
| `authority/` | `authority` | default-on | **yes — the only citable role** (citation whitelist) |
| `historical/` | `historical` | only via explicit `reads` | no (annotation channel) |
| `personal/` | `personal` | only via explicit `reads` | no (annotation channel) |
| `draft/` | `draft` | **never** — the role outranks the entry switch | no |

### Two Ways to Register a Knowledge Base

1. **Seed PR (shipped baselines):** add an entry to [`knowledge_bases.yaml`](knowledge_bases.yaml) — a controlled file changed via PR review — and create the directory. Slugs must match `^[a-z][a-z0-9-]{0,19}$`; `data` and the role names are reserved.
2. **Admin API (runtime operations):** `POST /api/admin/kb` with the `X-Admin-Token` header. Creation and deactivation are INSERT-only audited; deactivation hides the KB from retrieval and Skill validation without deleting any data.

### Add a Document to an Existing Knowledge Base

The simple case: drop a Markdown file into the KB directory.

```bash
# Add a new authoritative policy doc
knowledge/authority/new_policy.md
```

The file format is Markdown with optional YAML frontmatter (the indexer parses `.md` natively). **The tier is decided by the directory, not by the document** — the registry-driven walker assigns `tier = <slug>` to every indexed file, and the frontmatter carries only the optional effective window:

```markdown
---
source: internal-policy-2026-01
effective_from: 1768435200
effective_to: 0
---

# New Policy Title

The policy body. This text is chunked, embedded, and indexed into Milvus
under tier=authority. A Skill with reads: [authority/**] can retrieve it,
and only authority-role chunks can become citations.
```

On the next `cmd/server` startup, the indexer scans the registered `knowledge/<slug>/` directories, chunks the file, embeds it via BGE-M3, and writes it to Milvus with the `tier` metadata. No code change needed. A directory with no registry entry is skipped with a WARN and an audit row (KBM-06).

### KNOW-02: Unknown Tiers Are Rejected

`internal/knowledge.validateTiers` rejects any document whose tier is not an **active** registry slug — `fail-closed` default-deny. The same fail-closed posture applies at Skill load time: a `reads` pattern naming an unknown or deactivated slug fails validation with the active slug list in the error message.

---

## Questions?

- **Architecture / design rationale:** `.planning/PROJECT.md` (project history + locked decisions).
- **Skill schema deep-dive:** the doc comments in [`internal/skill/manifest.go`](internal/skill/manifest.go) and [`internal/skill/validate.go`](internal/skill/validate.go).
- **License:** Apache-2.0 — see [LICENSE](LICENSE).

Happy hacking.
