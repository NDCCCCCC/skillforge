---
phase: 52-llm-provider-panel
plan: 01
status: complete
subsystem: llm-config
tags: [llm, catalog, persistence, probe]
provides:
  - "internal/llmconfig vendor/plan catalog (zhipu/opencode-go/go-plus/zen/openai/deepseek/moonshot/siliconflow/ollama/custom)"
  - "0600 persist (~/.skillforge/llm.json, SKILLFORGE_LLM_CONFIG override) + Mask + ResolveKey (api_key_env wins)"
  - "TestKey minimal-chat-completion probe with error classification + ListModels four-shape parsing"
key-files:
  - internal/llmconfig/catalog.go
  - internal/llmconfig/store.go
  - internal/llmconfig/probe.go
key-decisions:
  - "D-03 key storage: user-local 0600 JSON; api_key_env indirection preferred over plaintext"
deviations:
  - "validate tightened to require vendor/plan to exist in the catalog (custom exempt from base_url check)"

verification: |
  - go build ./... (WSL) rc=0; go test -count=1 all ok
  - bash verify-phase-52.sh — ALL PASSED (exit 0)
notes: |
  Set summary lives in 52-04-SUMMARY.md (plan: 04 is the collection record).
---

