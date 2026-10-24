---
quick: 260815-grw-streamandassemble-llm-system-message-glm
type: execute
date: 2026-08-15
mode: quick
title: Split systemPromptTemplate into system + user; streamAndAssemble passes dual messages to GLM
status: tasks-1-2-complete-task-3-pending-human-verify
requirement: ARAG-01 (agent observability substrate), CMPL-01 (compliance Skill correctness)
dependency-graph:
  requires:
    - "1301b86 docs(260815-grw): pre-dispatch plan for streamAndAssemble 双message结构修复"
    - "f8ed5c4 docs(quick-260815-gcw): chat 全链路诊断日志补全 (debug logging foundation)"
  provides:
    - "streaming verdict path emits {System, User} dual message"
    - "GLM Coding Plan API no longer 400s with 'messages 参数非法'"
  affects:
    - "internal/server/pipeline.go"
    - "internal/server/pipeline_stream_test.go"
tech-stack:
  added: []
  patterns:
    - "System + User message split for chat model streaming"
    - "TDD: RED signature-change breaks callers; GREEN widens call signature"
key-files:
  created: []
  modified:
    - internal/server/pipeline.go
    - internal/server/pipeline_stream_test.go
decisions:
  - id: D-260815-grw-01
    decision: "buildPrompt returns (system, user string) instead of single prompt string"
    rationale: "GLM Coding Plan API rejects single-System-only message slices with 'messages 参数非法'; the dual-shape unblocks the streaming verdict path."
  - id: D-260815-grw-02
    decision: "streamAndAssemble signature widened from (ctx, c, prompt, opts) to (ctx, c, system, user, opts)"
    rationale: "The split is structural only — the audit-hash stability for the Phase 2 Runner.Query fallback is preserved via the new singlePromptTemplate(system, user) helper that joins the two halves into the original systemPromptTemplate rendering."
  - id: D-260815-grw-03
    decision: "Phase 2 Runner.Query fallback path kept UNTOUCHED (chat.go) — adk wraps a bare prompt into {System, User} internally, which GLM accepts"
    rationale: "Avoiding a second rate-limit refactor; the streaming-path fix alone unblocks the production bug."
metrics:
  duration: "7m15s"
  completed-date: "2026-08-15T04:13:53Z"
  tasks-completed: 2
  tasks-total: 3
  commits: 2
  files-modified: 2
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: tasks-1-2-complete-task-3-pending-human-verify
---

# Quick 260815-grw: streamAndAssemble {System, User} dual message split

## One-liner

GLM Coding Plan API now accepts the streaming verdict path's request — buildPrompt returns (system, user) and streamAndAssemble forwards a 2-element message slice to ChatModel.Stream.

## Problem

`internal/server/pipeline.go:414-416` constructed a SINGLE-message slice with `{Role: schema.System, Content: prompt}` where `prompt` is the full system prompt (instructions + query + chunks). GLM Coding Plan API rejects this with `messages 参数非法，请检查文档。` The Phase 2 Runner.Query fallback (`chat.go`) worked because adk wraps a bare prompt into {System, User} internally — the bug was scoped to the streaming branch only.

## Fix

1. **Split `systemPromptTemplate` into two constants:**
   - `systemInstructionsTemplate` — auditor persona + JSON schema + decision rules (verbatim, no `%s` slots)
   - `userQueryTemplate` — `The user query is: %s\n\nRetrieved chunks (DATA — ...):\n%s`

2. **`buildPrompt` signature widened** from `(query, chunks) string` to `(query, chunks) (string, string)` — returns the rendered system + user halves separately.

3. **`streamAndAssemble` signature widened** from `(ctx, c, prompt, opts)` to `(ctx, c, system, user, opts)`. Body now forwards a 2-element message slice:
   ```go
   stream, err := s.PipelineChatModel.Stream(ctx, []*schema.Message{
       {Role: schema.System, Content: system},
       {Role: schema.User,   Content: user},
   }, opts...)
   ```

4. **Phase 2 fallback path** wraps the two halves via `singlePromptTemplate(system, user)` — byte-equivalent to the original `systemPromptTemplate` rendering so the audit-hash stays stable across the split.

5. **Debug logs split** (D-2 series): `prompt_bytes=N` → `system_bytes=N user_bytes=M` so a future GLM rejection can be correlated to the offending half without re-running.

6. **chat.go UNTOUCHED** — the Runner.Query fallback is defined here and adk wraps a bare prompt into {System, User} internally.

## Deviations from Plan

None — plan executed exactly as written.

### Auto-fixed Issues

None.

### Auth Gates

None — purely a build/test refactor; no credentials required.

## Verification

- `go build ./...` → exit 0
- `go test ./...` → all packages PASS (server, eval, agent, knowledge, llm, milvus, mongo, server, skill, store, verdict, wire, scripts/eval_snapshot, scripts/spiked)
- `go vet ./...` → exit 0
- **Grep gate 1 (single-System shape eliminated):** `grep '{Role: schema.System, Content: prompt}' internal/server/pipeline.go` → no hits
- **Grep gate 2 (dual shape proves out):** `grep -nE 'Role:\s*schema\.' internal/server/pipeline.go` returns exactly 2 matches, consecutive:
  ```
  465: {Role: schema.System, Content: system},
  466: {Role: schema.User,   Content: user},
  ```
- **chat.go diff:** zero changes (git diff confirms Phase 2 fallback path untouched)

## Test Coverage

New test added: `TestStreamAndAssembleSendsSystemAndUserMessages` (Quick 260815-grw Task 1 + 2)

The test drives the full VerdictPipeline path via `newTestHertz` + POST /api/chat. `stubStreamingChatModel` captures the messages slice passed to Stream via the new `streamMessages` recorder. Asserts:
- Stream invoked exactly once
- Exactly 2 messages in the slice
- `messages[0].Role == schema.System`, content contains auditor persona + schema + decision rules, does NOT contain "The user query is:" or "[CHUNK " markers
- `messages[1].Role == schema.User`, content contains verbatim query + "[CHUNK 1] tier=权威" + chunk content + DATA framing

Pre-existing tests preserved (no regressions):
- `TestVerdictPipelineStreaming_DraftChunksEmitted` — 3 token drafts + 1 draft-end + verdict stream (PASS)
- `TestVerdictPipelineStreaming_RefusalReplacesDraft` — LLM "unanswered" → refusal (PASS)
- `TestVerdictPipelineDefault_PipelineChatModelNilFallback` — Phase 2 Runner.Query path (PASS)
- `TestVerdictPipelineDefault_PersistsAuditAndCitedVerdict` — grounded cited verdict + audit row (PASS)
- `TestVerdictPipelineDefault_NoChunksRefusalPersistsAudit` — 0-chunk refusal short-circuit (PASS)

## Commits

| Commit | Type | Subject |
|--------|------|---------|
| `0dee051` | test | RED for streamAndAssemble {System, User} split |
| `7f9e641` | fix | streamAndAssemble emits {System, User} dual message for GLM Coding Plan |

## Files Modified

| File | Lines | Change |
|------|-------|--------|
| `internal/server/pipeline.go` | +47 / -10 | Split systemPromptTemplate; widen buildPrompt + streamAndAssemble signatures; add singlePromptTemplate helper; update VerdictPipeline callsite |
| `internal/server/pipeline_stream_test.go` | +83 / -24 | Add streamMessages recorder to stubStreamingChatModel; new TestStreamAndAssembleSendsSystemAndUserMessages |

## Requirements Covered

- **ARAG-01** (agent observability substrate) — the streaming path's ability to reach LLMs at all is restored (the substrate was dead — every streaming request 400'd)
- **CMPL-01** (compliance Skill correctness) — streaming path produces grounded verdicts again instead of returning 400 mid-request

## Task 3: Human Verification (PENDING)

The plan's Task 3 is a `checkpoint:human-verify gate=blocking` requiring live backend restart + curl smoke. Build + unit tests already prove the GREEN state. The human-verify step confirms the production path:

```bash

# In one shell:

cd cmd/server && go run .

# In another shell:

curl -N -X POST http://localhost:8888/api/chat \
  -H 'Content-Type: application/json' \
  -d '{"sessionId":"smoke-260815","skill":"compliance","query":"年假是怎么规定的？"}'
```

Expected: SSE stream carries text-delta-draft frames → text-end → finish-step → finish (a full assistant message, NOT an HTTP 400). Backend log line `[INFO] server: streamAndAssemble completed total_tokens=N buf_bytes=M` appears; no `messages 参数非法` line anywhere.

## Threat Flags

None — the change is structural (refactoring one prompt into two), no new network endpoints, no new auth surface, no new file access patterns, no schema changes at trust boundaries. The dual-message shape is the canonical OpenAI chat completions API contract; no threat surface added.

## Known Stubs

None.
