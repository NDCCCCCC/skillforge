---
phase: 52-llm-provider-panel
plan: 04
status: complete
subsystem: llm-config
tags: [llm, admin-panel, hot-reload, openai-compatible]
requires: ["49-02 stub assembly (PipelineChatModel seam)"]
provides:
  - "internal/llmconfig vendor/plan catalog + 0600 persist + probe"
  - "llm.DynamicChatModel delegating hot-reload model (lazy WithTools rebind)"
  - "5 admin endpoints under /api/admin/llm/* (X-Admin-Token gated)"
  - "web admin LLM config panel (vendor→plan→URL, key/env, test, model list)"
  - "verify-phase-52.sh nine-partition gate"
affects:
  - "cmd/server/main.go chat assembly (dynamic toolChatModel + pipeline dynamic fallback to stub)"
  - "internal/server nil semantics preserved via ChatModelAvailable helper"
actuals:
  tokens: 200000
  tasks: 4
  commits: 1
tech-stack:
  patterns:
    - "delegating model wrapper: resolve per call, swap upstream atomically under RWMutex"
    - "WithTools lazy rebind: boundDynamic resolves CURRENT upstream then WithTools(tools)"
    - "config precedence: panel file (user-local llm.json, 0600) > env; GET returns masked key"
    - "resolve failure empties the slot (Available=false → stub semantics), never kills the server"
key-files:
  - internal/llmconfig/catalog.go
  - internal/llmconfig/store.go
  - internal/llmconfig/probe.go
  - internal/llm/dynamic.go
  - internal/server/llm_admin.go
  - internal/server/chatmodel_avail.go
  - cmd/server/main.go
  - web/src/components/admin/llm/LLMProviderPanel.tsx
  - web/src/components/admin/llm/llmApi.ts
  - verify-phase-52.sh
key-decisions:
  - "D-01 hot reload via delegating model (user decision 2026-10-23)"
  - "D-02 OpenAI-compatible only"
  - "D-03 panel key stored in a user-local JSON file chmod 0600 (SKILLFORGE_LLM_CONFIG overrides the default path); opencode's own /connect credentials stay in opencode's user-level auth store — SkillForge never touches them"
  - "D-04 stub nil semantics preserved: ChatModelAvailable treats non-Dynamic implementations as truthy (test doubles unchanged)"
  - "D-05 verdict.Mode auto-clears on upstream switch (stamped by stubchat extractor only)"
deviations:
  - "D1: startup Fatalf contract kept ONLY for a broken env preset; a broken PANEL save empties the slot at runtime (server must not die from a panel action)"
  - "D2: main_test boot-assembly assertions untouched — dynamic assembly kept the same variable names and log shapes"
verification: |
  - go build ./... (WSL) rc=0
  - go test ./internal/llmconfig/ ./internal/llm/ ./internal/server/ ./cmd/server/ -count=1 → all ok (server 21.9s)
  - web: npx vitest run src/components/admin/llm → 3/3 passed; npx tsc --noEmit rc=0
  - bash verify-phase-52.sh → === Phase 52 Gate: ALL PASSED === (exit 0)
notes: |
  opencode Go/Zen is usable with zero code changes: LOCAL_API_KEY +
  LOCAL_BASE_URL=https://opencode.ai/inference/openai/v1/chat/completions +
  LOCAL_MODEL=glm-5.3-flash (README documents the triple and where opencode
  itself keeps the key its /connect command writes).
---