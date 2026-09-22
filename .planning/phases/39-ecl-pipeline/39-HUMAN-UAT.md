---
status: partial
phase: 39-ecl-pipeline
source: [39-VERIFICATION.md]
started: 2026-09-22T00:00:00Z
updated: 2026-09-22T00:00:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Live LLM F1 gate (SC-2 live half)
expected: Run ECL via admin (POST /api/admin/pipeline/run with token); live GLM extraction output scored against eval/ecl-golden/sql-guides.jsonl yields tables macro-F1 ≥ 0.8 (harness: TestStubVsGoldenF1 pattern with LLM extractor swapped in)
result: [pending]

### 2. Live Milvus E2E roundtrip (SC-3 live half)
expected: SKILLFORGE_MILVUS_RECREATE=1 boot → Run ECL → restart server → retrieval over sql-guides returns merged sections (section metadata populated; no legacy+ecl duplicate generations)
result: [pending]

### 3. UI-SPEC visual checklist (SC-4)
expected: 13-item 39-UI-SPEC.md visual pass across idle/running/done/failed/403/503 states — verbatim copy, stage sentinels, progress bar, failures table, retry button enablement (dead-only)
result: [pending]

### 4. CR-01 fixed-behavior confirmation (fix shipped: ae84393 + WR-01 9a4e25)
expected: force milvus-load failure (e.g. block Milvus mid-run); after the fix, Run/RetryNote error paths reset stage to idle (terminal sentinel) — FE returns to idle state with Run button re-enabled; retry claims stay async (WR-01) so the HTTP call returns immediately. Regression tests pin every failure mode (manager suite); this live check confirms the FE badge behavior matches
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
