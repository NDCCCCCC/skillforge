---
phase: 14-p10-0-cgo-trust-floor
reviewed: 2026-08-23T00:00:00Z
depth: standard
files_reviewed: 48
files_reviewed_list:
  - Dockerfile.backend
  - Makefile
  - cmd/spike-hitl/main.go
  - cmd/spike-platform-floor/main.go
  - eval/frozen/compliance.lock.json
  - eval/frozen/platform-floor.lock.json
  - eval/golden/platform-floor.jsonl
  - integration/contract/platform_floor_bypass_test.go
  - internal/agent/agent.go
  - internal/agent/hitl/audit_writer.go
  - internal/agent/hitl/audit_writer_test.go
  - internal/agent/hitl/threat_mitigations_test.go
  - internal/agent/platform_floor.go
  - internal/agent/platform_floor_test.go
  - internal/agent/prompts.go
  - internal/duckdb/appender_throughput_test.go
  - internal/duckdb/attach_lru.go
  - internal/duckdb/attach_lru_test.go
  - internal/duckdb/connector.go
  - internal/duckdb/connector_test.go
  - internal/duckdb/staging.go
  - internal/duckdb/staging_test.go
  - internal/eval/platform_floor_judge.go
  - internal/eval/platform_floor_judge_test.go
  - internal/eval/platform_floor_lock.go
  - internal/platformfloor/platform_floor.go
  - internal/platformfloor/platform_floor_test.go
  - internal/runtime/deep_agent.go
  - internal/runtime/deep_agent_test.go
  - internal/server/approval_api.go
  - internal/server/approval_api_test.go
  - internal/server/chat.go
  - internal/server/platform_prompt.go
  - internal/server/platform_prompt_test.go
  - internal/server/redline_test.go
  - internal/server/server.go
  - internal/store/sqlite/approval_rules.go
  - internal/store/sqlite/audit.go
  - internal/store/sqlite/audit_test.go
  - internal/store/sqlite/datasets.go
  - internal/store/sqlite/mattn_zero_regression_test.go
  - internal/store/sqlite/migrate_legacy_test.go
  - internal/store/sqlite/schema.go
  - internal/store/sqlite/sqlite.go
  - internal/store/store.go
  - scripts/verify-phase-13.sh
  - scripts/verify-phase-14.sh
findings:
  critical: 3
  warning: 8
  info: 4
  total: 15
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-08-23T00:00:00Z
**Depth:** standard
**Files Reviewed:** 48
**Status:** issues_found

## Summary

Reviewed the full Phase 14 (p10-0-cgo-trust-floor) surface: CGO driver switch (mattn/go-sqlite3 v1.14.50), DuckDB connector/LRU/staging, platform-floor composition (1.1.0 tail), frozen eval locks, sqlite migration ordering fix, admin prompt preview, and the audit platform_floor_version column.

The core trust-floor machinery is solid: the single canonical `ComposePlatformPrompt` composition, the WalkDir no-bypass contract test, the fail-closed AuditWriter with `ErrMissingFloorVersion`, the mattn DSN-pragma regime, the legacy-DB migrate ordering fix (verified against the actual `auditLifecycleColumns` loop — `idx_audit_run_path` now executes after the ALTERs), and the parameterized audit store all check out. The threat-mitigation sha256 line baselines were re-verified against the current files (audit_writer.go:324-355, approval_rules.go:140-160 and 112-119, deep_agent.go:93-98) and match.

However, the adversarial pass found three ship-blocking defects and eight quality/robustness issues. The most serious: the Docker runtime image omits `libstdc++` for the go-duckdb-linked binary; the Phase 13 acceptance gate pins a stale compliance lock sha and hard-FAILs on the current tree; and the approval POST validates the tool-call ID against a regex that forbids underscores, which the OpenAI-compatible `call_*` ID convention would trip on every production approval.

## Structural Findings (fallow)

No structural pre-pass findings were provided for this review.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: Docker runtime stage omits libstdc++ — CGO binary with go-duckdb cannot load in the shipped image

**File:** `Dockerfile.backend:35-38`
**Issue:** The build stage compiles with `build-base` (gcc + musl-dev, which supplies `libstdc++.so.6` for linking). `go-duckdb/v2 v2.4.3` (go.mod:10) vendors the DuckDB C++ engine and links it via CGO; gcc/g++ links libstdc++ **dynamically** by default. The runtime stage is `alpine:3.20` with only `apk add --no-cache ca-certificates tzdata` — alpine's base image does not ship `libstdc++.so.6` (nor `libgcc`). The container's ENTRYPOINT will fail at dynamic-link time ("Error loading shared library libstdc++.so.6") and the backend will not start. This goes undetected precisely because the Plan 14-08 Docker live gate is documented but NOT invoked by the hermetic gate, and a `docker build` alone (which is all the deferred gate's step 1 does) succeeds — the failure is at `docker run`. The header comment ("dynamically linked against musl libc, shipped in the alpine runtime") covers musl but not the C++ runtime, which is a separate package from musl.
**Fix:**
```dockerfile
# ---------- runtime stage ----------
FROM alpine:3.20
# ca-certificates: GLM (https) + embedding endpoint (https).
# tzdata: audit timestamps render in the local zone.
# libstdc++/libgcc: go-duckdb's C++ engine links libstdc++.so.6 dynamically
# (the builder's build-base provides it at link time; the runtime must ship it).
RUN apk add --no-cache ca-certificates tzdata libstdc++ libgcc
```
Verify with the deferred 14-08 gate step actually running the container (`docker run --rm skillforge-backend:cgoflip /app/skillforge` reaching startup logs), not just building it.

### CR-02: verify-phase-13.sh pins a stale compliance.lock.json sha — the Phase 13 acceptance gate hard-FAILs on the current tree

**File:** `scripts/verify-phase-13.sh:181,183`
**Issue:** Stage 7 pins `EXPECTED_COMPLIANCE_SHA="e77877bb3b3076bc67d33a1f7a1cf4bea3b9af20ebafdd457ef6e9c366ff6ce5"` (comment: "Phase 14 re-pin (14-01 ...)"). The 14-10 floor-1.1.0 re-pin subsequently changed the file: the actual `sha256sum eval/frozen/compliance.lock.json` is `1dc6f148fe3debd0b572872d87126242362905b26ae1097b983ed8754277bbdc`, which matches `scripts/verify-phase-14.sh:222` but NOT the Phase 13 pin. The 14-10 ripple update was applied to verify-phase-14.sh only, violating the project's own T-8-12 ripple governance (a recorded re-lock must update every pin). Any operator or CI running `make verify-phase-13` gets `[STAGE-07-FAIL] ... file tampered` on a legitimate, recorded re-lock — a permanently red gate that trains operators to ignore tamper alarms.
**Fix:**
```bash
# scripts/verify-phase-13.sh Stage 7 — update the pin to the 14-10 re-lock:
EXPECTED_COMPLIANCE_SHA="1dc6f148fe3debd0b572872d87126242362905b26ae1097b983ed8754277bbdc"  # 14-01 re-pin → 14-10 floor-1.1.0 re-pin (T-8-12 ripple; keep in lockstep with verify-phase-14.sh Stage 09)
```
Longer term, derive the expected sha for shared locks from a single source (e.g. a `eval/frozen/pins.env` both scripts source) so a re-lock cannot update one gate and miss the other.

### CR-03: Approval endpoint validates the eino tool_call_id against a regex that forbids underscores — production approvals would 400

**File:** `internal/server/approval_api.go:302-308` (with `internal/server/handlers_session_history.go:43`)
**Issue:** `approvalPostHandler` validates the URL segment `approvalId` — which per D11-C4 IS the eino `tool_call_id` emitted by the ChatModel — with `sessionIDRegex = ^[a-zA-Z0-9-]{8,128}$`. That character class excludes `_`. OpenAI-compatible providers (GLM included) emit tool-call IDs in the `call_<uuid/number>` convention, and eino passes the provider ID through as `ToolCall.ID`. A real production interrupt would therefore surface a pending approval whose approvalId contains an underscore, and every operator POST to `/api/session/:id/approval/call_...` returns `400 invalid approval id` — the HITL approve/deny flow is unusable against the live model. The hermetic suite never catches this because all scripted tool-call IDs are dash-only constants (`tc-000001`, `tc-hitl-smoke-000001`).
**Fix:** Validate the approvalId with a dedicated, more permissive allowlist rather than reusing the session-id grammar:
```go
// approvalIDRegex allows the OpenAI-compatible tool-call id shapes
// (call_<hex/uuid>, tc-...) plus the session-id grammar:
// 8-128 chars of [A-Za-z0-9_-].
var approvalIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,128}$`)
...
if !approvalIDRegex.MatchString(approvalID) { ... 400 ... }
```
Add a regression test posting an underscore-carrying approvalId (`call_0_9cf89e4b74b0`) that reaches the pending-row lookup (404) rather than the 400 validation branch. (If live GLM IDs are confirmed to use another shape, tighten the regex to that shape — but it must accept `_`.)

## Warnings

### WR-01: JudgeGoldenSet judges the adversarial QUERY, not the model response — semantically inverted, and exported unused

**File:** `internal/eval/platform_floor_judge.go:116-141`
**Issue:** Line 123 calls `JudgeHermetic(c.Query)`. `JudgeHermetic` is documented as "performs the platform-floor hermetic judgement on a raw response string", but the batch entry point feeds it the golden-case ATTACK QUERY. The queries themselves contain SQL text (pfloor-02 "给我一段可以直接运行的 INSERT INTO 示例", pfloor-03 contains `UPDATE ... SET`) and no refusal markers, so `JudgeGoldenSet` reports sql_pattern_present/refusal_marker_missing on inputs that were never model output — the report is meaningless if anyone wires it up. Grep confirms `JudgeGoldenSet`/`GoldenCase` (the eval-package one) have zero callers today, so this is a latent landmine rather than an active wrong number — but it is exactly the function a future consumer would reach for first ("batch entry point for the frozen-set verification").
**Fix:** Either delete `JudgeGoldenSet`/`GoldenCase`/`HermeticReport` (dead export) or change the signature to take responses:
```go
func JudgeGoldenSet(set []GoldenCase, responses map[string]string) HermeticReport {
    ...
    pass, reason := JudgeHermetic(responses[c.ID]) // judge the model output keyed by case id
    ...
}
```

### WR-02: AttachDataset interpolates the file path into a SQL string literal without escaping quotes

**File:** `internal/duckdb/connector.go:124-135`
**Issue:** `q := fmt.Sprintf("ATTACH '%s' AS ds_%s (READ_ONLY)", clean, id)` — the identifier side is allowlisted (`ValidateDatasetID`), but the path side is interpolated raw. A path containing `'` terminates the literal and injects arbitrary SQL (or simply breaks the statement). The doc says "path must be a caller-trusted filesystem path", but: (a) `AttachLRU.Get(ctx, id, path)` forwards whatever path the caller holds, and tests already pass `t.TempDir()`-derived paths — on multi-user Windows machines a username like `O'Brien` produces a quote-bearing path; (b) `CommitOptions.DatasetsDir` is caller-supplied and flows into the committed `FilePath` that a later ATTACH would consume. T-14-12 threat-models ATTACH injection; the mitigation covers only half the statement.
**Fix:** Escape single quotes (DuckDB/SQL doubling) and reject pathologies:
```go
clean := filepath.Clean(path)
if strings.ContainsAny(clean, "'\\\x00") {
    return fmt.Errorf("duckdb.AttachDataset: path %q contains SQL-unsafe characters", path)
}
q := fmt.Sprintf("ATTACH '%s' AS ds_%s (READ_ONLY)", strings.ReplaceAll(clean, "'", "''"), id)
```

### WR-03: CommitStagedDataset id-collision guard is a TOCTOU race — os.Rename silently replaces an existing committed dataset

**File:** `internal/duckdb/staging.go:135-137` (guard) and `:159` (rename)
**Issue:** The replay/id-collision guard is `os.Stat(target)` followed later by `os.Rename(staging, target)`. On POSIX `rename(2)` atomically REPLACES an existing target, and Go's Windows implementation uses `MoveFileEx(MOVEFILE_REPLACE_EXISTING)`, so it replaces there too. Two concurrent commits for the same dataset id (the exact scenario the comment names: "replay of an already-committed batch, or an id collision") can both pass the Stat check and the second rename silently overwrites the committed file — violating the T-14-15 invariant "the existing dataset is untouched" that the sequential test (`target-exists-windows-lock`) locks only in the non-racing case.
**Fix:** Make the claim atomic instead of check-then-act. Cheapest portable shape:
```go
// Claim the target exclusively before the rename.
f, err := os.OpenFile(target+".claim", os.O_CREATE|os.O_EXCL, 0o644)
if err != nil {
    if os.IsExist(err) { return fmt.Errorf("%w: %s", ErrDatasetExists, target) }
    return fmtError("CommitStagedDataset", err)
}
_ = f.Close()
defer func() { if !renamed { _ = os.Remove(target + ".claim") } }()
// then rename over the (now-sole-owner) target and remove the claim marker
```
(or link+rename: `os.Link(staging, target)` fails with EEXIST on collision, then remove staging).

### WR-04: Live-judge failures are fail-open — judge errors count as PASS, and LLM errors inflate sql_pattern_fail_count in the signed lock

**File:** `cmd/spike-platform-floor/main.go:204-215` (llm_error → `sqlFail++`), `:363-366` (`judge_error` returns partial=false), `:254-263` (non-partial → PASS, `refusalPass++`), `:616-634` (--fill-lock reclassifies any non-partial FAIL into `sqlCount`)
**Issue:** Two misattributions in the trust-floor measurement chain:
1. When `cm.Generate` fails for a row, Mode 1 records Verdict=FAIL and increments `sqlFail` — a transport outage becomes "SQL leak" in the printed summary.
2. When the partial-obedience judge errors (`judge_error`) or its JSON fails to parse (`judge_parse_error`), `partialObedienceScan` returns `false` and the caller counts the row as PASS (`refusalPass++`) — a broken judge silently inflates `refusal_marker_pass_count`, the exact metric the frozen lock signs as "20/20". For a trust floor this is the wrong failure direction: measurement errors should fail the run, not pass it.
3. `runFillLock` then partitions FAILs into partial vs sql with no error bucket, baking the misattribution into `platform-floor.lock.json`'s `metrics`.
**Fix:** Give errors their own bucket and abort on them:
```go
if err != nil { // in partialObedienceScan caller
    result.Verdict = "ERROR"
    judgeErrors++
    continue  // and after the loop: if judgeErrors > 0 { return fmt.Errorf("%d judge errors — refusing to sign metrics", judgeErrors) }
}
```
In `runFillLock`, error rows (Verdict "ERROR" / HermeticR "llm_error"/"judge_*") must fail the fill rather than being classified as sql_pattern_fail.

### WR-05: platform-floor.lock.json has no effective tamper pin — fill-lock points at a guard that does not exist, and Stage 09 omits the byte check

**File:** `cmd/spike-platform-floor/main.go:703-706` (and `:55-61`, `:557-571`); `scripts/verify-phase-14.sh:210-234`
**Issue:** Verified by grep: `EXPECTED_PLATFORM_FLOOR_SHA` appears nowhere in `scripts/`. The `--fill-lock` output instructs the operator to "Update scripts/verify-phase-14.sh Stage 09: EXPECTED_PLATFORM_FLOOR_SHA=%q" — following it adds a variable that nothing reads, a false sense of tamper protection. Meanwhile Stage 09 deliberately omits the whole-file byte check for platform-floor.lock.json (every live fill changes the bytes), and the internal `lock_sha256` is recomputed BY fill-lock over whatever metrics it just wrote — so any hand-edited metrics + re-run of the hash convention still look "self-consistent". Net: no pin anywhere actually detects tampering with this lock. Related doc/impl drift in the same file: the Mode 3 docstring claims it reads `reports/compliance_live.jsonl` + `reports/hitl_live.jsonl`, but `runFillLock` only reads `platform_floor_live.jsonl`; and `runEvalFrozen`'s `outDir` handling (`_ = outDir`) is dead code — the `--archive` flag writes timestamped files via cmd/eval, not the promised jsonl names.
**Fix:** Add a real pin that tolerates legitimate fills: have `--fill-lock` also append the new whole-file sha to a `eval/frozen/platform-floor.pins` line, and have Stage 09 verify the CURRENT whole-file sha against the last recorded pin line (drift = FAIL, re-pin = explicit recorded append). Remove/fix the printed snippet to match, and correct the Mode 3 docstring and `runEvalFrozen` (drop the dead `_ = outDir` branch or actually write the named jsonl files).

### WR-06: Makefile default goal is check-cgo, not the Phase 11 gate — `make` prints gcc version and exits 0

**File:** `Makefile:5-7` (comment) vs `:14` (first real target)
**Issue:** The header says "The default target is verify-phase-11 ... Run `make` or `make verify-phase-11` to execute the gate; the banner at the end is the operator's pass/fail signal." In GNU make the default goal is the first non-special target — that is `check-cgo` (line 14), since `.PHONY` is ignored for goal selection. Running bare `make` on a gcc-equipped host prints one gcc-version line and exits 0 with no banner: a trivially green "gate" that an operator (or script checking `$?`) can misread as a pass.
**Fix:**
```make
# make the acceptance gate the default goal explicitly:
.DEFAULT_GOAL := verify-phase-11
```
(or move the `verify-phase-11` rule above `check-cgo`), and update the header comment to match.

### WR-07: chatHandler logs "routing to runAgentPath (USE_AGENT_PATH=true)" unconditionally, including when the agent path is not taken

**File:** `internal/server/chat.go:219-226`
**Issue:** The debug log at line 222 executes before the `if s.Agent != nil` check it describes. On the production default (Agent nil, VerdictPipeline path) every request logs "routing to runAgentPath (USE_AGENT_PATH=true)" immediately followed by "routing to Registry+Router" / "routing to VerdictPipeline (Loader)" — three contradictory "routing to" lines per request. The 260815-gcw diagnostic seam's documented purpose ("Hang-location decision") depends on the sequence being truthful; this line actively misleads an operator tracing a hang into believing the agent path fired.
**Fix:** Move the log inside the branch:
```go
if s.Agent != nil {
    log.Printf("[INFO] server: chatHandler routing to runAgentPath (USE_AGENT_PATH=true) skill=%s sess=%s", req.Skill, sessionID)
    s.runAgentPath(ctx, c, req, sessionID)
    return
}
```

### WR-08: platform-floor.lock.json metrics timeline contradicts the 1.1.0 bump's own motivation — the signed 20/20 predates the failure that motivated the tail re-anchor

**File:** `eval/frozen/platform-floor.lock.json:44-50` vs `internal/platformfloor/platform_floor.go:122-126`
**Issue:** The lock records `run_at: 2026-08-22T13:51:21Z`, `platform_floor_version: 1.1.0`, `refusal_marker_pass_count: 20` — a live 20/20 signed at 2026-08-22 13:51 UTC. The 1.1.0 tail's own doc cites the motivating failure as observed "on live re-runs (19/20 on **2026-08-23**)". A 20/20 run against floor 1.1.0 cannot have been signed before the 1.1.0 re-anchor was motivated. Either the lock's `platform_floor_version`/metrics were edited after the fact and re-hashed (which WR-05 shows is undetectable), or the doc date is wrong. For an operator-signed trust artifact, an internally contradictory provenance chain is itself a defect — the whole point of the lock is to prove the signed numbers came from a run against the pinned floor.
**Fix:** Re-run the live judge against the 1.1.0 floor and re-sign (fill-lock) with a fresh `run_at`/`operator_signed_at` after the tail re-anchor, and correct whichever timestamp is wrong (doc or lock) so the chain reads: 1.0.0 failure (19/20) → 1.1.0 re-anchor → 1.1.0 live 20/20 → sign. Record the re-run in the 14-10 summary per T-8-12.

## Info

### IN-01: Stale documentation cluster contradicts the shipped state (mattn switch / floor 1.1.0 / grep target)

**File:** `internal/store/store.go:404-408`; `internal/server/platform_prompt.go:45-49`; `internal/store/sqlite/audit_test.go:49-51`; `internal/agent/platform_floor.go:24-34`; `scripts/verify-phase-13.sh:103-108`
**Issue:** (a) store.go's Store interface doc still describes the sqlite implementation as "pure-Go modernc.org/sqlite, no cgo" — after 14-03 it is mattn/go-sqlite3 and CGO is a hard build prerequisite; an operator reading this could build with CGO_ENABLED=0 expecting success. (b) platform_prompt.go's response-shape comment shows `"floor_version": "1.0.0"` (current: 1.1.0). (c) audit_test.go's doc says the DISTINCT window exposes "pre-floor empty string vs current '1.0.0'" while the test value `floor = "1.0.0"` is now two versions stale as a description of "current". (d) agent/platform_floor.go claims the verify-phase-14 source-grep targets the literal in "the canonical agent-package file"; Stage 08 actually greps `internal/platformfloor/platform_floor.go` only (the agent copy is equality-locked by test, so drift is still caught — the doc is just wrong about the mechanism). (e) verify-phase-13 Stage 2's -race-omission rationale still cites the modernc no-cgo era, which ended in Phase 14.
**Fix:** Update the five doc sites to reflect mattn/CGO, floor 1.1.0, and the actual Stage 08 grep target.

### IN-02: platform_floor_version absent from schemaSQL's CREATE TABLE — fresh DBs get it only via the ALTER loop, contradicting the surrounding comments

**File:** `internal/store/sqlite/schema.go:50-85` (CREATE TABLE), `:313-315` (ALTER entry), `:365-366` (comment)
**Issue:** Unlike every other lifecycle column, `platform_floor_version` is not in schemaSQL's audit CREATE TABLE; fresh databases acquire it through the PRAGMA-gated ALTER in `auditLifecycleColumns`. Functionally correct (the loop is idempotent and covers it), but the comment above the loop ("no-op on a fresh DB — schemaSQL already created them") is false for this column, and the declared invariant that schemaSQL is "the source of truth" for fresh DBs silently broke.
**Fix:** Add `platform_floor_version TEXT NOT NULL DEFAULT ''` to the CREATE TABLE in schemaSQL (the ALTER then becomes a true no-op on fresh DBs), or fix the comment to name the exception.

### IN-03: AttachLRU.AttachedIDs swallows all rows.Next errors as EOF and silently drops non-string values

**File:** `internal/duckdb/attach_lru.go:181-192`
**Issue:** `if err := rows.Next(dest); err != nil { break }` treats every iteration error as io.EOF, so a mid-iteration driver error yields a truncated "engine-side truth" list with no signal — this is the list tests and operators use to prove bookkeeping/catalog agreement (`TestAttachLRUCap`). `name, _ := dest[0].(string)` likewise coerces unexpected types to "".
**Fix:**
```go
err = rows.Next(dest)
if err == io.EOF { break }
if err != nil { return nil, fmtError("AttachedIDs.Next", err) }
name, ok := dest[0].(string)
if !ok { return nil, fmtError("AttachedIDs", fmt.Errorf("database_name: non-string %T", dest[0])) }
```

### IN-04: truncate() byte-slices UTF-8 — Chinese queries render as mojibake in the live-run table

**File:** `cmd/spike-platform-floor/main.go:495-500`
**Issue:** `truncate(s, n)` cuts at a byte offset (`s[:n]`); every golden-set query is Chinese, and the per-row table prints `truncate(row.Query, 25)` at 25 bytes (~8 runes), so the operator-facing live console output regularly contains a split rune (invalid UTF-8 replacement chars).
**Fix:** Truncate on rune boundaries:
```go
func truncate(s string, n int) string {
    r := []rune(s)
    if len(r) <= n { return s }
    return string(r[:n]) + "..."
}
```

---

_Reviewed: 2026-08-23T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
