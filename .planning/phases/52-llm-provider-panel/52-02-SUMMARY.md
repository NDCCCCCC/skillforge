---
phase: 52-llm-provider-panel
plan: 02
status: complete
subsystem: llm-config
tags: [llm, hot-reload, admin-api, assembly]
provides:
  - "llm.DynamicChatModel delegating hot-reload model (lazy WithTools rebind via boundDynamic)"
  - "internal/server/chatmodel_avail.go: nil semantics preserved for test doubles"
  - "5 admin endpoints under /api/admin/llm/* (providers/config GET+POST/test/models, X-Admin-Token gated, GET masked)"
  - "main.go assembly: toolChatModel = dynChat always; pipeline = llm.NewPipelineDynamic(dynChat, stubChat)"
key-files:
  - internal/llm/dynamic.go
  - internal/server/chatmodel_avail.go
  - internal/server/llm_admin.go
  - internal/server/server.go
  - cmd/server/main.go
key-decisions:
  - "D-01 hot reload via delegating model (user decision 2026-10-23)"
  - "D-04 stub nil semantics preserved via ChatModelAvailable (non-Dynamic = truthy)"
  - "D-05 verdict.Mode auto-clears on upstream switch (stamped by stubchat extractor only)"
deviations:
  - "D1: startup Fatalf kept ONLY for a broken env preset; a broken panel save empties the slot (server must not die from a panel action)"
  - "hertz has no c.BindJSON — save handler uses c.BindAndValidate (same as admin_ontology.go)"

verification: |
  - go build ./... (WSL) rc=0; go test -count=1 all ok
  - bash verify-phase-52.sh — ALL PASSED (exit 0)
notes: |
  Set summary lives in 52-04-SUMMARY.md (plan: 04 is the collection record).
---

