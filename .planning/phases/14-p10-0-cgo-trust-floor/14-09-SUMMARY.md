---
phase: 14-p10-0-cgo-trust-floor
plan: 09
subsystem: database
tags: [sqlite, mattn-go-sqlite3, migration, legacy-upgrade, audit, uat-gap-closure]

requires:
  - phase: 14-p10-0-cgo-trust-floor plan 03
    provides: mattn/go-sqlite3 v1.14.50 driver + DSN pragma regime (the driver the legacy boot runs on)
  - phase: 14-p10-0-cgo-trust-floor plan 02
    provides: platform_floor_version audit column + admin preview endpoint (UAT Test 6 / Test 2 surfaces)
provides:
  - Legacy-DB upgrade path fix: idx_audit_run_path created AFTER the ALTER loop (mirrors createAuditToolCallIDPendingUniqueIdxSQL precedent) — pre-Phase-12 skillforge.db migrates and boots
  - Hermetic regression lock for the legacy upgrade path (file-backed 21-column audit table → New() → index + 5 columns + legacy row survival + idempotent reopen)
  - Live re-verification evidence: UAT Test 2 (admin three faces) issue→pass; UAT Test 6 (audit floor version) blocked→pass
affects: [14-10 floor-1.1.0-reanchor, verify-work 14, any deployment holding a pre-Phase-12 skillforge.db]

tech-stack:
  added: []
  patterns:
    - "Index-after-ALTER ordering contract: any CREATE INDEX referencing a migratable column must ExecContext AFTER the ALTER loops in migrate(), never inline in schemaSQL (second instance of the D11-C4 precedent, now generalized)"

key-files:
  created:
    - internal/store/sqlite/migrate_legacy_test.go
  modified:
    - internal/store/sqlite/schema.go
    - .planning/phases/14-p10-0-cgo-trust-floor/14-UAT.md
    - .planning/phases/14-p10-0-cgo-trust-floor/deferred-items.md

key-decisions:
  - "D14-E1: idx_audit_run_path moved to a dedicated const (createAuditRunPathIdxSQL) executed in migrate() after the ALTER loops — schemaSQL stays fresh-DB-only for index statements referencing migratable columns"
  - "D14-E2: live re-verify uses the deployed skill literal \"compliance\" (findSkill Phase-2 gate accepts only that literal; 107/115 legacy audit rows carry it) — plan's simple-qa literal 404s by pre-existing contract, deferred to deferred-items.md"
  - "D14-E3: Test 6 live assert scoped to rows id > pre-chat max (blanket last-3 would include legacy rows whose platform_floor_version is legitimately '' after the ALTER NOT NULL DEFAULT '')"

patterns-established:
  - "Legacy-upgrade regression tests use file-backed t.TempDir() DSNs with a hand-seeded legacy DDL — hermetic :memory: fresh-DB tests can never exercise the pre-existing-table ALTER path"

requirements-completed: [PLAT-10]

duration: 17m
completed: 2026-08-23
---

# Phase 14 Plan 09: Legacy-DB Migrate Gap Closure Summary

**idx_audit_run_path moved after the ALTER loop — a pre-Phase-12 skillforge.db (21-column audit table, 115 real rows) now migrates via sqlite.New() and boots; UAT Test 2 (admin 403/403/200/404) and Test 6 (audit platform_floor_version=1.0.0) re-verified live.**

## Performance

- **Duration:** ~17 min
- **Started:** 2026-08-22T19:35:19Z
- **Completed:** 2026-08-22T19:52:00Z (evidence captured 2026-08-23 03:48 WSL-local = 2026-08-22 19:48 UTC)
- **Tasks:** 2
- **Files modified:** 2 code (1 created + 1 modified) + 3 planning docs

## Accomplishments
- Root cause of the UAT Test 2 boot blocker fixed: `CREATE INDEX IF NOT EXISTS idx_audit_run_path` was embedded in the single schemaSQL ExecContext, firing BEFORE the ALTER loop that adds `run_path` on a legacy DB — `sqlite.migrate: no such column: run_path`. The index now lives in a dedicated const executed in `migrate()` after the ALTER loops, mirroring the `createAuditToolCallIDPendingUniqueIdxSQL` precedent (schema.go).
- The hermetic `:memory:` blind spot is permanently closed: `TestMigrateLegacyAuditUpgradePath` builds a file-backed 21-column legacy audit table (exact schema observed on the real repo-root skillforge.db), migrates via `New()`, and asserts index presence + all 5 newer columns + legacy-row survival; two guard tests lock the fresh-file and idempotent-reopen directions.
- Live re-verification on the REAL legacy skillforge.db: server boots to `store open (dsn=skillforge.db, WAL mode)` + `knowledge corpus indexed` + listening; admin endpoint faces 403/403/200/404; one real chat POST landed audit row id=116 with `platform_floor_version='1.0.0'`; all 115 legacy rows intact.
- Phase-14 hermetic gate re-run: all 11 stages PASS, 4 byte-frozen locks + platform-floor lock untouched.

## Task Commits

1. **Task 1 RED: legacy-DB migrate regression test** — `dd81dd2` (test) — TestMigrateLegacyAuditUpgradePath reproduced `sqlite.migrate: no such column: run_path` exactly
2. **Task 1 GREEN: idx_audit_run_path after ALTER loop** — `e1c1a12` (fix)
3. **Task 2: live re-verify (UAT Tests 2 + 6)** — zero code commits by design (evidence below; skillforge.db untracked, never staged)

**Plan metadata:** this docs commit.

## Files Created/Modified
- `internal/store/sqlite/migrate_legacy_test.go` (created) — legacy-upgrade regression lock: 21-column legacy DDL seed helper + 3 tests (upgrade path / fresh-file guard / idempotent reopen guard)
- `internal/store/sqlite/schema.go` (modified) — removed the inline index (+3-line P13-L6 comment) from schemaSQL; added `createAuditRunPathIdxSQL` const with the ordering-contract comment; ExecContext in `migrate()` after the checkpoint ALTER loop in the same block as the tcid index. auditLifecycleColumns / columnExists / all other schemaSQL statements untouched.
- `.planning/phases/14-p10-0-cgo-trust-floor/14-UAT.md` — Test 2 issue→pass, Test 6 blocked→pass (evidence pointer here)
- `.planning/phases/14-p10-0-cgo-trust-floor/deferred-items.md` (created) — findSkill Phase-2 literal gate discovery

## Decisions Made
- **D14-E1 — index-after-ALTER as a generalized contract:** second occurrence of the D11-C4 pattern is now documented on the new const itself ("same ordering contract as createAuditToolCallIDPendingUniqueIdxSQL"); any future index referencing a migratable column follows it.
- **D14-E2 — live verification skill literal:** the deployed single-Skill system resolves ONLY the literal `"compliance"` through the Phase-2 `findSkill` gate (chat.go:660) even though the loaded manifest is simple-qa; face C and the Test 6 chat therefore used `"compliance"` (matches 107/115 legacy audit rows). Not fixed here — pre-existing, out of plan scope, logged to deferred-items.md.
- **D14-E3 — Test 6 assert scope:** asserted rows `id > 115` (pre-chat high-water mark) all carry `'1.0.0'` + legacy count preserved, instead of the plan's blanket last-3 (legacy rows legitimately carry `''` post-ALTER).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] WSL VM restart cleared /tmp mid-task**
- **Found during:** Task 2 (live re-verify)
- **Issue:** The plan reuses `/tmp/skillforge-server` built by `scripts/tmp-uat-admin.sh`; the WSL VM shut down between tool invocations, wiping `/tmp` (binary, logs, and the step-A backup `/tmp/skillforge.db.pre-14-09`).
- **Fix:** Re-ran as ONE self-contained script (backup → build → boot → faces → chat → assert) so all artifacts live in a single VM session; a post-migration safety backup was taken instead (the first successful boot had already applied the additive migration — verified: 26 columns, idx present, 115 rows intact before the chat test).
- **Files modified:** none (scratch scripts under scripts/, deleted after use)
- **Verification:** `SCRIPT_EXIT=0`, full evidence log below.
- **Committed in:** n/a (no code change)

**2. [Plan-text inaccuracy] Guard test 3 predicted PASS pre-fix but fails pre-fix**
- **Found during:** Task 1 RED
- **Issue:** Plan note said both guard tests "当前即 PASS"; `TestMigrateLegacyIdempotentReopen` seeds a legacy DB so its first `New()` hits the same blocker (cannot pass pre-fix by construction).
- **Fix:** None needed — the behavior spec ("旧库迁移后 Close → 再次 New() NoError") is implemented exactly; only the planner's pass-prediction was wrong.
- **Verification:** RED run shows Test 3 failing with the same `no such column: run_path`; GREEN run all 3 PASS.
- **Committed in:** `dd81dd2` / `e1c1a12`

**3. [Scope boundary] Face C / chat used skill literal "compliance" instead of plan's "simple-qa"**
- **Found during:** Task 2 (first admin run: face C returned 404, not 200)
- **Issue:** `findSkill` (chat.go:660, Phase-2 simplification) accepts only the literal `"compliance"`; `?skill=simple-qa` → 404 by pre-existing contract. Plan's expected 200 was written assuming simple-qa resolves (the earlier UAT never got past boot to discover this).
- **Fix:** Verified live contract via legacy audit rows (107/115 carry `skill_id='compliance'`), ran face C + Test 6 chat with `"compliance"`; discovery logged to `deferred-items.md` (NOT auto-fixed — pre-existing, changes chat routing semantics, out of plan `files_modified`).
- **Files modified:** `.planning/phases/14-p10-0-cgo-trust-floor/deferred-items.md`
- **Verification:** face C 200 + 4 fields with floor-first prompt; chat audit row skill_id='compliance'.
- **Committed in:** this docs commit

---

**Total deviations:** 3 (1 Rule-3 blocking auto-handled, 1 plan-text inaccuracy documented, 1 scope-boundary deferral)
**Impact on plan:** All UAT Test 2/Test 6 done-criteria met; no scope creep (findSkill fix explicitly deferred).

## Live Evidence (Task 2 — real legacy skillforge.db, WSL, 2026-08-23 03:48 local)

Pre-check (before first boot): 21 columns, NO run_path/budget_layer/transfer_from/transfer_to/platform_floor_version, 115 audit rows.

```
=== server boot (with 14-09 fix) ===
NO_MIGRATE_ERROR
2026/08/23 03:48:11 [OK] loaded 1 Skill(s)
2026/08/23 03:48:14 [OK] knowledge corpus indexed into skillforge_knowledge
2026/08/23 03:48:14 [OK] cmd/server: store open (dsn=skillforge.db, WAL mode)
=== server listening ===
--- face A: no token (expect 403) ---        403
--- face B: wrong token (expect 403) ---     403
--- face C: correct token + resolvable skill (expect 200) ---
200
fields: ['composed_prompt', 'floor_version', 'prompt_hash', 'skill_id']
floor_version: 1.0.0
skill_id: compliance
composed_prompt head (floor must be FIRST): '【平台数据铁律——优先于任何 Skill 指令】\n\n1. 永不编写、执行或建议 '
prompt_hash: ca333fbadbfa30eb…
--- face C2: literal simple-qa (informational) --- 404   (Phase-2 findSkill gate; see deferred-items.md)
--- face D: unknown skill (expect 404) ---    404
=== UAT Test 6: real chat POST (SSE) ===
CHAT_EXIT=0   sse bytes: 904
data: {"type":"finish"}
data: {"type":"data-decision","data":{"step":0,"decision":"history_classifier_refused","query":"out_of_scope"}}
=== read-only audit assert ===
new audit rows (id > 115): [(116, '1.0.0', 'compliance')]
row116 full: skill_id='compliance', query='公司年假有几天？', refusal_reason='out_of_scope', platform_floor_version='1.0.0'
legacy rows intact (id <= 115): 115
TEST6-LIVE-PASS
=== plan verify-command equivalence ===
grep -c 'SERVER_DID_NOT_LISTEN|no such column' → 0 → UAT-TEST2-LIVE-PASS
```

Note: the chat's verdict was an out_of_scope refusal by the history classifier (live system behavior, unrelated to this plan); the audit row was still written by the server-mediated path — Test 6's requirement is exactly that this write carries the floor version, which it does (`1.0.0`). Hermetic equivalent (gate Stage 08) already locks the answered-path variants.

### Hermetic verification (Task 1 + plan-level)

```
go test ./internal/store/sqlite/ -run 'TestMigrateLegacy|TestMigrateFreshFileDB' -count=1 -v
  PASS: TestMigrateLegacyAuditUpgradePath (0.04s)
  PASS: TestMigrateFreshFileDBRunPathIdx (0.02s)
  PASS: TestMigrateLegacyIdempotentReopen (0.03s)
go test ./internal/store/... -count=1   → ok (store + store/sqlite)
go build ./... → 0     go vet ./... → 0
bash scripts/verify-phase-14.sh → PHASE 14 HERMETIC GATE: PASS (all 11 stages; 4 byte-frozen locks + platform-floor self-consistency intact; GATE_EXIT=0)
```

## UAT Status Flips

| Test | Before | After | Evidence |
|------|--------|-------|----------|
| #2 Admin Prompt Preview (was issue/blocker: `sqlite.migrate: no such column: run_path`) | issue | **pass** | boot log above + faces 403/403/200/404 |
| #6 Audit Rows Carry Floor Version (was blocked_by: server) | blocked | **pass** | row 116 `platform_floor_version='1.0.0'`, TEST6-LIVE-PASS |

(Test 4 live re-run 19/20 remains open — Plan 14-10 floor 1.1.0 re-anchor.)

## Issues Encountered
None blocking. WSL `/tmp` clearing mid-task (handled, deviation 1); findSkill literal discovery (deferred, deviation 3).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 14-10 (floor 1.1.0 re-anchor for Test 4) is next — no dependencies on this plan's artifacts beyond the now-bootable server.
- Deployment note: any site holding a pre-Phase-12 skillforge.db upgrades cleanly now (additive migration, legacy rows preserved, index created post-ALTER).
- Follow-up (non-blocking): findSkill real manifest-name lookup (SYSK-05) — see deferred-items.md.

## Self-Check: PASSED

- internal/store/sqlite/migrate_legacy_test.go — FOUND (166 lines ≥ 80 min)
- internal/store/sqlite/schema.go contains `createAuditRunPathIdxSQL` — FOUND
- Commits dd81dd2 + e1c1a12 — FOUND on master
- grep `idx_audit_run_path` in schemaSQL string body → only the fresh-DB table DDL comment-free zone remains; index created via const — VERIFIED by tests
