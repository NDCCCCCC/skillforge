# Spike Conventions

Patterns and stack choices established across spike sessions for SkillForge (ai-agent).

## Stack

- **Backend**: Go 1.24+ + CloudWeGo Eino v0.9.13+ + Eino ADK (eino-ext/adk) + OpenAI adapter (eino-ext/components/model/openai v0.1.13+)
- **LLM**: GLM-5.2 via Coding Plan endpoint `https://open.bigmodel.cn/api/coding/paas/v4`
- **Test framework**: stdlib only — `net/http` for mock OpenAI server; `crypto/sha256` for output hashing; `encoding/json` for schema validation
- **Spike language**: All 4 spikes in Go (consistent with denova reference + project stack)

## Structure

- **Spike directory**: `.planning/spikes/NNN-kebab-name/`
- **Per-spike files**:
  - `README.md` — frontmatter (`spike`, `name`, `type`, `validates`, `verdict`, `related`, `tags`) + sections (Research / Approach / How to Run / What to Expect / Observability / Investigation Trail / Results)
  - `main.go` — single-file spike program
  - `skill.md` — test SKILL.md for the spike
  - `*-output.log` — captured run output
  - `*-outputs.json` — structured output (when applicable)
- **Naming**: `NNN-` zero-padded number; comparison spikes use `NNNb-` suffix
- **Verification artifacts**: SHA-256 first 8 bytes of normalized output used as deterministic identity
- **Coding Plan endpoint** (CRITICAL): `https://open.bigmodel.cn/api/coding/paas/v4` (NOT standard `/api/paas/v4`)

## Patterns

- **Mock server for transmission tests** (spike 001): local `http.Server` on `:18080` with handler printing request body; verify with `grep '"temperature":0'` in captured output. Use `netstat -ano | grep :PORT` to check availability before binding.
- **Variance measurement** (spike 002/002b): `crypto/sha256` of `strings.TrimSpace(output)` as identity; count unique per query × 10 iterations; pass if ≤2 unique (allowing whitespace variance).
- **Schema validation** (spike 003): Go struct with `json:` tags + custom validation logic for enum constraints.
- **Extra fields via Eino**: `openai.WithExtraFields(map[string]any{...})` for OpenAI extras like `response_format`, `thinking`, `modalities`, vendor-specific flags.
- **Reasoning control**: `thinking:{type:disabled}` to disable glm-5.2's reasoning path (saves tokens, faster, cleaner output).
- **Content safety**: Use ASCII content in JSON messages OR `--data-binary @file.json` (avoid UTF-8 jackson errors with curl `-d '{...}'` on win bash).

## Tools & Libraries

| Package | Version | Purpose |
|---|---|---|
| github.com/cloudwego/eino | v0.9.13 | LLM framework core |
| github.com/cloudwego/eino-ext/components/model/openai | v0.1.13 | OpenAI adapter (with WithExtraFields) |
| gopkg.in/yaml.v3 | latest | SKILL.md YAML frontmatter parsing |
| (no test framework — minimal stdlib) | — | Single-file spike programs, no tests framework needed |

## Avoid

- **Hardcoded model names in log strings** (spike 002 hardcoded `(glm-4-flash)` while actual was `glm-5.2` — caused misleading log)
- **Static port :18080** without checking `netstat -ano | grep :PORT` first (PID 61884 collision in spike 001)
- **CJK content in curl inline JSON** on win bash (UTF-8 jackson errors) — use `--data-binary @file.json`
- **`max_tokens < 500` with reasoning models** (was 300 — caused empty content when reasoning consumed budget)
- **Standard `/api/paas/v4` endpoint for Coding Plan keys** (returns 1113 — use `/api/coding/paas/v4`)
- **Trusting byte-determinism** with reasoning models (100% variance regardless of temperature + reasoning control)
- **`response_format:{type:json_schema}`** on GLM-5.2 (use `{type:json_object}` + app-side validation instead)
- **Trusting `Content` field** when reasoning is ON (reasoning budget may leave content empty)