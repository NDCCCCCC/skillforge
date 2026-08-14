# Contributing to SkillForge

Thanks for your interest in contributing! SkillForge is built around a simple thesis: **the highest-leverage contribution is a domain `SKILL.md`, not Go code.** Most contributions land in `skills/` or `knowledge/`. This guide covers the three most common contribution patterns.

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
| `reads` | `[]string` | **Non-empty** tier patterns (default-deny). No `*` wildcard. |
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

1. **`reads` must be non-empty.** The validator rejects an empty `reads` slice (default-deny — `P-SKILL-3`). Every Skill must declare at least one tier it may read. If your Skill is purely tool-driven with no knowledge tier, declare a placeholder tier (e.g. `draft/**`) and note it in prose — do **not** relax the validator.

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
- **Tool-only Skills (calculator-style):** declare a placeholder tier in `reads` (e.g. `draft/**`) so the 7-field validator passes. The runtime today is a RAG pipeline; pure tool-execution support is a future version. See `examples/skills/calculator/SKILL.md` for the pattern.

---

## Add a Knowledge Tier

SkillForge organizes knowledge into four tiers. A tier is both a **Milvus metadata value** (CN-char VARCHAR) and a **directory under `knowledge/`**. Adding a new tier means wiring both sides.

### The TierEnum

Tiers are defined in [`internal/skill/tier.go`](internal/skill/tier.go):

| Directory (`knowledge/<dir>/`) | Milvus tier value | Meaning |
|--------------------------------|-------------------|---------|
| `authority/` | `权威` | Authoritative, currently-effective source of truth |
| `historical/` | `历史` | Historical / superseded versions |
| `personal/` | `个人` | Personal notes / individual context |
| `draft/` | `草稿` | Drafts / unverified |

`ResolveTiers(reads)` maps a Skill's `reads` patterns (e.g. `authority/**`) to the unique set of Milvus tier values. The indexer (`internal/knowledge.Index`) rejects any document whose `Tier` field is outside the 4 canonical values — `fail-closed` default-deny.

### Add a Document to an Existing Tier

The simple case: drop a Markdown file into the tier directory.

```bash
# Add a new authoritative policy doc
knowledge/authority/new_policy.md
```

The file format is Markdown with optional YAML frontmatter (the indexer parses `.md` natively):

```markdown
---
tier: 权威
source: internal-policy-2026-01
effective_date: 2026-01-15
---

# New Policy Title

The policy body. This text is chunked, embedded, and indexed into Milvus
under tier=权威. A Skill with reads: [authority/**] can retrieve it.
```

On the next `cmd/server` startup (or hot-reload), the indexer scans `knowledge/`, chunks the file, embeds it via BGE-M3, and writes it to Milvus with the `tier` metadata. No code change needed.

### Add a Brand-New Tier

Adding a 5th tier requires a code change — the tier set is the locked trust boundary for v1:

1. Add an entry to `TierEnum` in [`internal/skill/tier.go`](internal/skill/tier.go):
   ```go
   var TierEnum = map[string]string{
       "authority":  "权威",
       "historical": "历史",
       "personal":   "个人",
       "draft":      "草稿",
       "newtier":    "新层",   // add your tier
   }
   ```
2. Create the directory: `mkdir knowledge/newtier`.
3. Add at least one document (see above).
4. A Skill can now declare `reads: [newtier/**]` and `ResolveTiers` will resolve it.

`TierEnumByValue` and `TierEnumValues` automatically pick up the new entry (they iterate `TierEnum`), so the indexer's default-deny check and error messages update without further edits.

---

## Questions?

- **Architecture / design rationale:** `.planning/PROJECT.md` (project history + locked decisions).
- **Skill schema deep-dive:** the doc comments in [`internal/skill/manifest.go`](internal/skill/manifest.go) and [`internal/skill/validate.go`](internal/skill/validate.go).
- **License:** Apache-2.0 — see [LICENSE](LICENSE).

Happy hacking.
