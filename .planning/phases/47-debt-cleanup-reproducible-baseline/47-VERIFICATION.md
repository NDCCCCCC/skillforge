---
phase: 47-debt-cleanup-reproducible-baseline
verified: 2026-10-19T14:05:00Z
status: passed
score: 26/28 must-haves verified (24 VERIFIED, 0 FAILED, 2 PARTIAL, 2 non-scored open items)
overrides_applied: 0
overrides: []
re_verification: false

# D-03b (plan 47-06 explicit hand-off): the authoritative per-ID disposition record for
# every v1.6 leftover is NOT reproduced here — this file carries a pointer line to it so
# the audit chain and the human read path are both complete.
debt_disposition_record: .planning/phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md

deferred:
  - truth: "D-45-DEF-01 — emitter TestEmitSubCategoryInstanceCount 6 vs 7 (clean-clone red)"
    addressed_in: "47-DEBT-DISPOSITION.md § D-45-DEF-01"
    evidence: "去向 `v1.7 修复`; plan 47-03 commits 22f8480 + e88d6ca; row carries mutation proof (dropping the sql-guides guard yields 3 ≠ 2)"
  - truth: "D-45-DEF-02 — untracked runtime-stale internal/server/platform/verify_main.go"
    addressed_in: "47-DEBT-DISPOSITION.md § D-45-DEF-02"
    evidence: "去向 `v1.7 修复` (work-tree cleanup); row states plainly that no delete-commit SHA exists because the file was never tracked — verified: `git ls-files` empty, `git log --all -- <path>` empty"
  - truth: "D-45-DEF-03 — 45-REVIEW WR-01 / WR-02 / IN-02"
    addressed_in: "47-DEBT-DISPOSITION.md § D-45-DEF-03"
    evidence: "去向 `延期+理由`, three independent re-entry preconditions, grouped with WARNING-2 as one schema-visibility decision record"
  - truth: "D-46-DEF-01 — DetailPanel.test.tsx SC-5 empty-state hero deterministic failure"
    addressed_in: "47-DEBT-DISPOSITION.md § D-46-DEF-01"
    evidence: "去向 `v1.7 修复`; plan 47-05 commit 5f9459a; KnowledgeGraphBrowser.tsx has zero diff"
  - truth: "D-46-DEF-02 — TestDeriveEdgesDeterministic flaky (REL-01)"
    addressed_in: "47-DEBT-DISPOSITION.md § D-46-DEF-02"
    evidence: "去向 `v1.7 修复`; commits 0ecd9c3 + a05929a; pre-fix -count=200 measured 34 and 50 failures (17–25%), post-fix -count=200/-count=100/-shuffle=on all exit 0"
  - truth: "WARNING-1 — rollback 后治理报告缓存不刷新 (REL-02)"
    addressed_in: "47-DEBT-DISPOSITION.md § WARNING-1"
    evidence: "去向 `v1.7 修复`; BE e7ca19a + d3f86c2 + c90f91f, FE 197421e + bf1c1e0 + abbb594, review fix 95ed0a3"
  - truth: "WARNING-2 — owl:Class-vs-skf:Class declaration quirk (skill view tier-invisible)"
    addressed_in: "47-DEBT-DISPOSITION.md § WARNING-2"
    evidence: "去向 `延期+理由`; admin baseline unaffected; re-entry precondition = merge with D-45-DEF-03 as one schema-visibility decision record"
  - truth: "BL-01 — 双 resolver 去重 (resolveSkillTiers vs resolveSkillTiersFromKG)"
    addressed_in: "47-DEBT-DISPOSITION.md § BL-01"
    evidence: "去向 `延期+理由`; REQUIREMENTS.md:79 Backlog（无 owner phase）/ Deferred; re-entry = standalone item defining the two resolvers' responsibility boundary"
  - truth: "BL-02 — server 包 lint 清扫"
    addressed_in: "47-DEBT-DISPOSITION.md § BL-02"
    evidence: "去向 `延期+理由`; REQUIREMENTS.md:80 Backlog（Phase 48 lint 落地后立项）/ Deferred; scope is undefined until golangci-lint actually produces a report"
  - truth: "BL-03 — live 抽取 prompt 对齐 writes-only 的残余微调"
    addressed_in: "47-DEBT-DISPOSITION.md § BL-03"
    evidence: "去向 `延期+理由`; REQUIREMENTS.md:81 Backlog（needs-live）/ Deferred; re-entry = re-run live extraction under the Phase 49 hermetic suite"
  - truth: "BL-04 — DetailPanel / emitter 两个 deferred 测试"
    addressed_in: "47-DEBT-DISPOSITION.md § BL-04"
    evidence: "去向 `v1.7 修复`; REQUIREMENTS.md:82 Phase 47 / Complete; both halves closed by plans 47-03 and 47-05"
  - truth: "D-05 — boot executes Quarantine, rollback does not (banner/confirm text overstated on the rollback path)"
    addressed_in: "47-DEBT-DISPOSITION.md § D-05 and § 3 SC-2 truthfulness note"
    evidence: "去向 `延期+理由` — accepted known limitation, explicitly NOT a fix; re-verified independently by this audit (cmd/server/main.go:941-945 calls Quarantine; RollbackTBox contains no Quarantine call)"
  - truth: "IN-01..IN-07 — 46-REVIEW Info 级观察"
    addressed_in: "47-DEBT-DISPOSITION.md § IN-01..IN-07"
    evidence: "去向 `ack backlog`; each row cites its own 46-REVIEW line range and file:line"
  - truth: "MEDIUM-2 — keyword_cooccur edge *attachment* still depends on `docs` first-match order (graph_edges.go:232-241)"
    addressed_in: "47-REVIEW.md (status: partially_resolved) + 47-REVIEW-FIXES-SUMMARY.md § Left open, deliberately"
    evidence: "open by explicit decision; re-confirmed present by this audit at graph_edges.go:232-241 — a different symptom of the GRF-02 class from the direction flake that REL-01 closed; the -count=100 / -shuffle=on gate constrains direction and ordering and is met"
  - truth: "LOW-1 — the GovernanceTab → onRollbackComplete wiring line has no test"
    addressed_in: "47-REVIEW.md § LOW-1"
    evidence: "open by explicit decision; re-confirmed by this audit: `grep -rl GovernanceTab web/src --include='*.test.tsx'` returns nothing, while the wiring exists at GovernanceTab.tsx:167"
  - truth: "LOW-2 — verify-phase-47.sh hardcodes REPO=/mnt/d/CODE/ClaudeCode/ai-agent"
    addressed_in: "47-REVIEW.md § LOW-2"
    evidence: "open by explicit decision; re-confirmed present at verify-phase-47.sh:58 (self-derived ROOT is at :64 and is used by P6/P7 only)"
  - truth: "LOW-3 — induction.go cache-write comment claim (b) is false on the D-02c failure branch"
    addressed_in: "47-REVIEW.md § LOW-3"
    evidence: "open by explicit decision; re-confirmed present: the (b) claim holds for the success write at induction.go:497 but not for the sibling error branch at :469-484"
  - truth: "INFO-1 / INFO-2 / INFO-3"
    addressed_in: "47-REVIEW.md § INFO"
    evidence: "open, informational; untouched by the fix commits"

gaps: []
human_verification: []
---

# Phase 47 Verification — 清债与可复现基线

**Phase Goal:** v1.6 leftover debt fully closed or explicitly dispositioned — a
non-flashing, reproducible full-suite baseline that Phase 48's public CI gate and
Phase 49's hermetic suite can both stand on.

**Verified:** 2026-10-19T14:05:00Z
**Status:** **PASSED** — 26/28 must-haves VERIFIED, 0 FAILED, 2 PARTIAL (both PARTIALs
are the already-adjudicated open items MEDIUM-2 and LOW-1, each re-confirmed by this
audit to exist exactly as described and to be accurately labelled as open).

**The phase's hard constraint:** `verify-phase-47.sh` exits 0 under WSL `CGO_ENABLED=1`,
and the full gate (P4 `./internal/...`) is at zero waivers. **Held — independently
reproduced, not taken on report.**

---

## 0. How I verified (and what I refused to take on report)

Everything below was run by this verification pass. Commands are stated so they can be
re-run; where a SUMMARY made a claim, I re-derived the result rather than citing it.

| # | Check | Command | Observed |
|---|-------|---------|----------|
| V1 | The gate, end to end | `bash verify-phase-47.sh` (Git Bash → WSL Ubuntu-22.04, CGO_ENABLED=1) | **exit 0**; all 7 partitions ran; banner `ALL PASSED`; P6 real — `Test Files 85 passed (85) / Tests 689 passed (689)` |
| V2 | **Can the gate go red?** (the false-green hunt) | injected `internal/knowledge/zz_gate_probe_test.go` containing one `t.Fatalf`, re-ran the gate, then deleted the file | **exit 1**, with `--- P4 unique failure set ---` / `--- FAIL: TestZZGateProbeDeliberateFailure` printed by name. The gate does **not** mask failures. |
| V3 | Clean-checkout reproducibility (D-04a) | `git archive HEAD \| tar -x -C $(mktemp -d)` then build + test inside that tree, WSL | `skills/` = **3** tracked dirs, `knowledge/sql-guides/` = **6** sub-dirs; `RC_build=0 RC_test_internal=0 RC_det100=0 RC_emitter=0 RC_test_all=0` |
| V4 | CI-scope parity | `CGO_ENABLED=1 go test ./... -count=1` (WSL) | **exit 0** — the 8 packages P4 does not cover (`cmd/eval`, `cmd/server`, `cmd/spike-*`, `scripts/eval_snapshot`, `scripts/spiked`) are all green |
| V5 | Frontend suite, Windows side | `cd web && npm test` | **exit 0** — 85 files / 689 tests / 0 failed / 0 skipped |
| V6 | Flake claim (serial) | `go test ./internal/... -count=1`, 3 consecutive serial runs | `rc=0 / rc=0 / rc=0` |
| V7 | Flake source predates this phase | `git log -S "context.WithTimeout" -- internal/agent/hitl/audit_writer_test.go` | `9613d4d 2026-08-20 feat(11-03): 6 hermetic HITL audit + checkpoint tests` — Phase 11, not Phase 47 |

Probe file removed; `git status --porcelain` shows **no tracked modifications** created by
this audit. The only untracked residue under `internal/` is the **9** pre-existing
`skillforge-server-test-*` directories that `47-DEBT-DISPOSITION.md` § D-45-DEF-02
explicitly records as *recorded, deliberately not deleted* — I counted them (`9`) and the
record's number is accurate.

---

## 1. Must-have scoring

### Plan 47-01 — REL-01 determinism (3 truths)

| # | Truth | Score | Evidence |
|---|-------|-------|----------|
| 1 | `ComputeEdges` byte-identical across repeated calls, incl. `From`/`To` direction | **VERIFIED** | `internal/knowledge/graph_edges.go:249` `sort.Slice(sources, func(i,j) bool { return sources[i].source < sources[j].source })` immediately before the `i<j` pairing loop; `:154-165` groups iterated over a `sort.Strings`'d key slice instead of `range` over the map; `:282-292` comparator gains `Weight` as 4th tie-break. Gate P5a (`-count=100`) **PASS** in my run. |
| 2 | `TestDeriveEdgesDeterministic` green at `-count=100` and `-shuffle=on` | **VERIFIED** | Gate P5a **PASS** and P5b **PASS** in V1. Test body `graph_edges_test.go:103` `for run := 0; run < 100; run++` with whole-struct `assert.Equal` on every run — not a field-by-field or delta comparison. |
| 3 | The GRF-02 doc contract is *literally* true for edge direction | **PARTIAL** | True for **direction and ordering** — that is exactly what the `-count=100` + `-shuffle=on` gate proves, and it is genuinely fixed. But MEDIUM-2 remains: `graph_edges.go:232-241` still picks each source's representative chunk ID by a **first-match scan over the caller-supplied `docs` slice**, so `From`/`To` *attachment* still depends on row order when two rows share a `Source`. Direction ≠ attachment; the SC-1 wording is honest about which one it closes. **I re-confirmed this is present** rather than trusting the review. |

`artifacts`: `graph_edges.go` `contains: "sort.Slice(sources"` — present at :249 ✅. `graph_edges_test.go` `contains: "for run := 0; run < 100; run++"` — present at :103 ✅.

**Note on D-01c (anti-laundering):** the test carries an explicit in-file prohibition at
`:99-102` — *"FORBIDDEN to 'fix' a red run here: widening an InDelta tolerance, or
switching to an order-insensitive / count-based comparison."* No tolerance was widened, no
comparison was loosened, and all `InDelta` calls on this path were **removed** in favour of
whole-struct equality. **This must-have passed by strengthening, not by weakening.**

### Plan 47-02 — REL-02 backend cache write (4 truths)

| # | Truth | Score | Evidence |
|---|-------|-------|----------|
| 1 | Cached report after rollback is the revalidation report (`trigger == "rollback"`) | **VERIFIED** | `induction.go` `m.kg.SetShaclReport(report)` immediately after `ValidateWithTrigger(..., "rollback")`, inside the same `m.mu` critical section as Steps 3-5. Pinned by `induction_test.go:464-466` `if cr.Trigger != "rollback" { t.Errorf(...) }`. |
| 2 | Served report agrees with the rollback response's violation count | **VERIFIED** | `induction_test.go:469-471` asserts `cr.ErrorCount != violations` → error, with the comment *"they must come from the same validation"*. The count and the cached report are provably one object, not two computations. |
| 3 | Failed validation leaves the previous report intact; no nil/typed-nil pushed | **VERIFIED** | Error branch (`induction.go:469-484`) has no `SetShaclReport` call at all. Pinned at `induction_test.go:534-545`: `if gr != seed` (pointer identity) and `gr.Trigger != "boot"` — i.e. the seeded boot report must survive *intact*, which fails if anything clears or overwrites it. |
| 4 | Read-path comments describe the two real writers | **VERIFIED** | `c90f91f` corrected `admin_governance.go` / `server.go`; the new write site names boot as the first writer and itself as the second (`induction.go:495-497`). |

**Adversarial note:** the fix reuses the report the rollback *already computed* rather
than adding a second validation — the honest zero-cost shape. It does not make
"report consistent" true by making the report vacuous.

### Plan 47-03 — emitter hermeticization (3 truths)

| # | Truth | Score | Evidence |
|---|-------|-------|----------|
| 1 | Instance-count tests pass on a git-tracked-files-only checkout | **VERIFIED** | V3: `RC_emitter=0` inside a `git archive HEAD` tree holding only **3** skills dirs and **6** sql-guides sub-dirs. Before this change these were the two deterministic clean-clone reds named in D-04. |
| 2 | The test asserts the emitter's contract against its own fixture, not a corpus-pinned number | **VERIFIED** | `TestEmitSubCategoryInstanceCount` now builds the tree under `t.TempDir()` and asserts `subs != len(subDirs)` — derived from the fixture, not a literal 7. |
| 3 | Production contract unchanged — the fix is on the assertion side | **VERIFIED** | `git diff --stat 22f8480~1 e88d6ca` touches `emitter_test.go` only; `emitter.go` unchanged. D-04b's "改实现凑 7" was explicitly refused. |

**Anti-vacuity check (I checked this myself rather than accepting the SUMMARY's word):**
the fixture deliberately gives the `authority` control category a *nested* sub-directory,
so a regression that dropped the `sql-guides` scoping guard would yield **3** SubCategories
and turn the `!= 2` assertion red. A childless control would have yielded 2 either way and
proved nothing. The test is falsifiable, not decorative.

### Plan 47-04 — REL-02 frontend (3 truths)

| # | Truth | Score | Evidence |
|---|-------|-------|----------|
| 1 | After rollback the banner re-renders with the post-rollback report and shows 「回滚后」 | **PARTIAL** | The mechanism is real and correct: `GovernanceTab.tsx:112` `refreshShaclReport`, wired at `:163` (`onRetry`) and `:167` (`onRollbackComplete={refreshShaclReport}`); the dead branch is `GovernanceTab.tsx:83` `report.trigger === 'boot' ? '启动时' : '回滚后'`; `TimelineView.tsx` fires `onRollbackComplete?.()`. **But the line that does it — `GovernanceTab.tsx:167` — has no test.** `grep -rl GovernanceTab web/src --include='*.test.tsx'` returns **nothing**; `TimelineView.test.tsx` renders `TimelineView` directly with its own spy. Deleting the prop leaves all 4 tests green. This is **LOW-1**, correctly labelled open. |
| 2 | A failed refetch never converts success into a failure toast | **VERIFIED** | Commit `95ed0a3`. Post-fix `TimelineView.tsx`: the POST result alone decides the outcome; `onRollbackComplete?.()` fires first inside its own try/catch; the changelog refetch is `void fetchChangelog().then(setChangelog).catch(() => {})` — **not awaited**, so its rejection has no path to the outer `catch`. Pinned by `TimelineView.test.tsx:148` with a stub whose POST-rollback refetch answers **HTTP 500**, and asserted non-vacuously via the changelog call count (2). |
| 3 | Success toast text unchanged; its count agrees with the fetched report | **VERIFIED** | `TimelineView.tsx:149` still emits the literal `已回滚至 v${targetVersion}（当前 v${result.version}）· 新增 ${result.new_violations} 条 violation` — unchanged per D-02e. The count and the banner report come from the same `ValidateWithTrigger` (truth 47-02/2). |

**On MEDIUM-1 specifically:** I confirmed the fix is *structural*, not incidental. The
pre-fix code had `await fetchChangelog()` inside the same outer `try` as the POST, so a
refetch rejection reached the catch. Moving the refetch out of the awaited path removes
the possibility entirely rather than adding another guard. The fixes-summary's mutation
proof (revert → both toasts in one DOM → restore) is the right kind of evidence and I did
not need to re-run it to accept the shape of the fix.

### Plan 47-05 — D-46-DEF-01 frontend determinism (3 truths)

| # | Truth | Score | Evidence |
|---|-------|-------|----------|
| 1 | Full FE suite green, no waiver, no skip | **VERIFIED** | V1 (P6, 689/689) and V5 (`npm test`, exit 0, 85 files / 689 tests / 0 failed / 0 skipped) — reproduced twice, independently of each other. |
| 2 | SC-5 assertion retargeted at the mode that renders the hero — not deleted, not weakened | **VERIFIED** | `DetailPanel.test.tsx:205` clicks `graph-mode-document` before asserting, and `:211` additionally pins the text `知识库为空` — **strictly stronger** than the original assertion. Mutation-checked per the SUMMARY: guarding the hero with `&& false` turns it red. |
| 3 | No production component modified to make a test pass | **VERIFIED** | `git diff --stat 5f9459a~1 5f9459a -- web/src/components/` is **empty**. The fix is entirely on the assertion side. |

### Plan 47-06 — disposition record (4 truths)

| # | Truth | Score | Evidence |
|---|-------|-------|----------|
| 1 | Every leftover has exactly one row with a non-empty 去向 and citable 证据 | **VERIFIED** | 19 rows: 6 `v1.7 修复` / 6 `延期+理由` / 7 `ack backlog`. Enforced mechanically by gate P7, which **PASSED** in V1. |
| 2 | 去向 is a closed vocabulary, machine-checkable | **VERIFIED** | P7 `case "$dest" in 'v1.7 修复'|'延期+理由'|'ack backlog') ;; *) fail ;; esac` — a fourth value fails the gate. |
| 3 | STATE.md keeps a per-ID index linking to the master table | **VERIFIED** | 11 per-ID rows at `STATE.md:90-101`, each carrying a link to `phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md`. |
| 4 | D-05 appears as an explicit known limitation, not an implied fix | **VERIFIED** | Row `D-05` = `延期+理由` with the text **「已知限制，本 phase 明确不修」**; mirrored in `STATE.md:101` as `known_limit`; and § 3 of the record is an explicit *"SC-2 的成立不等于 rollback 路径上的全部文案都属实"* truthfulness note. **I independently re-derived the underlying fact**: `cmd/server/main.go:941-945` iterates `shaclReport.Quarantined` and calls `ontologyKG.Quarantine(t)`; `RollbackTBox` contains **no** `Quarantine` call (only two comment mentions). The banner's 「N 个实体已隔离」 really is untrue on the rollback path, and the record says so rather than implying REL-02 fixed it. |

### Plan 47-07 — REL-04 traceability + wording correction (3 truths)

| # | Truth | Score | Evidence |
|---|-------|-------|----------|
| 1 | REL-01/SC-1 wording states the measured root cause (edge direction, map iteration) | **VERIFIED** | `ROADMAP.md` SC-1 reads 边方向非确定性 … `sources` 由 `sourceTokens` 的 **map 迭代序**构造 … 并明确记录「`Weight` 断言 **0 次失败**」. `REQUIREMENTS.md:29` mirrors it. The superseded floating-point explanation is gone from both. |
| 2 | REL-01 acceptance strength recorded as `-count=100` + `-shuffle=on`, two-runs marked insufficient | **VERIFIED** | `ROADMAP.md` SC-4 states both and explicitly parentheticalises 连续两跑 as 仅作冒烟门槛，不充分. `REQUIREMENTS.md:29` repeats it. |
| 3 | BL-01..04 appear as Traceability rows with owner/status; table 100% filled | **VERIFIED** | `REQUIREMENTS.md:79-82` — BL-01/02/03 = Deferred with owner-phase rationale, BL-04 = Phase 47 / Complete. Coverage line reads `17/17 mapped items`. Definitions cited verbatim from the read-only v1.6 archive at `:55-58`. |

### Plan 47-08 — zero-waiver gate (5 truths)

| # | Truth | Score | Evidence |
|---|-------|-------|----------|
| 1 | Gate exits 0 under WSL CGO_ENABLED=1 with no waiver/allowlist/skip; single non-fatal branch is the npx tool check | **VERIFIED** | V1 exit 0. `grep -n "waiv\|WAIVED\|allowlist"` over all three gates returns **only prose comments** explaining the retirement — no mechanism. `grep "t.Skip\|WAIVED"` in `verify-phase-47.sh` returns nothing executable. The one non-fatal branch keys on `command -v npx` — a tool check, never a test name or outcome. |
| 2 | The suite is green inside a clean `git archive HEAD` checkout | **VERIFIED** | V3, run by me from scratch: all five RC values 0. |
| 3 | The gate exercises the claimed determinism strength | **VERIFIED** | P5a `-count=100` and P5b `-shuffle=on` over knowledge + ontology, both **PASS** in V1. This is the segment the script's own header correctly identifies as *"the evidence"* rather than the SC-4 two-run smoke threshold. |
| 4 | The retired D-45-DEF-01 waiver no longer exists in the 45/46 gates | **VERIFIED** | `verify-phase-46.sh:128-139` P4 is now strictly `rc != 0 → fail` plus an empty-log guard (`if [ ! -s ... ]; then fail "...no evidence of a test run"`). Same shape in `verify-phase-45.sh`. The old grep-based judgment that could have passed an infrastructure failure as "zero failures found" is gone from both. |
| 5 | The disposition record is checked mechanically by the gate | **VERIFIED** | P7 asserts exactly-one-row per ID across all 19 IDs, closed 去向 vocabulary, and non-empty 证据. **PASSED** in V1. |

---

## 2. Requirements cross-reference — every ID accounted for

`.planning/REQUIREMENTS.md` v1.7 Traceability, all 17 rows inspected:

| ID | Phase | Status in file | Where implemented | Accounted for |
|----|-------|-----------------|-------------------|---------------|
| REL-01 | 47 | `Pending` (bookkeeping — see note) | plan 47-01 → `0ecd9c3`, `a05929a` | ✅ verified above (2 VERIFIED, 1 PARTIAL) |
| REL-02 | 47 | `Pending` (same note) | plan 47-02 → `e7ca19a`/`d3f86c2`/`c90f91f`; plan 47-04 → `197421e`/`bf1c1e0`/`abbb594`; review fix → `95ed0a3` | ✅ verified above (5 VERIFIED, 1 PARTIAL) |
| REL-03 | 51 | `Pending` | not in this phase's scope | ✅ correctly out of scope |
| REL-04 | 47 | `Pending` (same note) | plan 47-06, 47-07, 47-08 | ✅ verified above |
| BL-01 | Backlog（无 owner phase） | `Deferred` | disposition row BL-01, `延期+理由` | ✅ |
| BL-02 | Backlog（Phase 48 lint 落地后立项） | `Deferred` | disposition row BL-02, `延期+理由` | ✅ |
| BL-03 | Backlog（needs-live） | `Deferred` | disposition row BL-03, `延期+理由` | ✅ |
| BL-04 | 47 | `Complete` | plans 47-03 + 47-05 | ✅ |
| FACE-01..03, CICD-01..03, DEMO-01..03 | 48/49/50/51 | `Pending` | not in this phase's scope | ✅ correctly out of scope |

**All 17 IDs accounted for. BL-01..04 — the specific backfill REL-04 names — are present
as rows with definitions cited verbatim from the read-only v1.6 archive, per-row
destinations, and an explicit 去向 summary at `REQUIREMENTS.md:60`.**

**One bookkeeping gap for the orchestrator (not a defect, and not mine to write):**
`REQUIREMENTS.md:75,76,78` still read `Pending` for REL-01/02/04 while the
implementations are complete and now verified. Per the phase convention these should flip
to `Complete` as part of the phase close-out write, alongside the STATE.md/ROADMAP updates
the orchestrator owns.

---

## 3. Open items and their explicit dispositions

Everything below was re-checked by me against the working tree. None is represented as
fixed, and none is inflated beyond what the evidence supports.

### Left open by explicit decision — re-confirmed present and accurately labelled

| Item | Disposition | My independent confirmation |
|------|-------------|------------------------------|
| **MEDIUM-2** — `keyword_cooccur` edge *attachment* depends on `docs` first-match order (`graph_edges.go:232-241`) | **Open, untouched.** Same GRF-02 class, different symptom from the direction flake REL-01 closed. | Confirmed at `:232-241`: the representative chunk ID comes from `for _, d := range docs { if d.Source == src { representativeID = d.ID; break } }`. The `sort.Slice(sources, ...)` at `:249` orders by `source`, which is unique — so it cannot remove this ambiguity. **Precise scoping:** this is *input-permutation* sensitivity (Milvus `Query` returns rows in segment/insertion order with no `ORDER BY`, so the same corpus can yield a different representative after a compaction), not run-to-run nondeterminism on a fixed `docs` slice. That distinction is why the `-count=100` / `-shuffle=on` gate is correctly scoped to direction and ordering, and why it is honest for it to be green. |
| **LOW-1** — the `GovernanceTab` → `onRollbackComplete` wiring line has no test | **Open, untouched.** | Confirmed: zero test files reference `GovernanceTab`; the wiring is real at `GovernanceTab.tsx:167`. The only direct evidence it works is a jsdom banner-flip trace whose harness was deliberately deleted. |
| **LOW-2** — gate hardcodes `REPO=/mnt/d/CODE/ClaudeCode/ai-agent` | **Open, untouched.** | Confirmed at `verify-phase-47.sh:58`, against the self-derived `ROOT` at `:64` used by P6/P7. Cosmetic on this box (DrvFs resolves `/mnt/d/CODE` and `/mnt/d/code`), but every Go partition depends on this machine's drive letter and path depth. Failure mode is loud, not silent. |
| **LOW-3** — `induction.go` comment claim (b) is false on the D-02c failure branch | **Open, untouched.** | Confirmed: the (b) justification ("no window where the served report disagrees with the on-disk TBox") is true of the write at `:497` and false of the sibling branch at `:469-484`, where Steps 3-5 already advanced the TBox but validation then failed — the cache then serves a report describing `v{N+1}` while the pointer says `v{N+2}`. It reads as a blanket property of the site rather than of the success path. |
| **INFO-1 / INFO-2 / INFO-3** | **Open, informational.** | Untouched by `659879b` / `95ed0a3`, as stated. |

### Accepted known limitation — never presented as a fix

**D-05 — boot executes `Quarantine`, rollback does not.** Recorded as `延期+理由` in the
master table, as `known_limit` in `STATE.md:101`, and as an explicit SC-2 truthfulness
note in § 3 of the record. Independently re-derived (§ 1 plan 47-06 truth 4). The rollback
confirm dialog's 「失效引用实体将进入隔离区」 and the banner's 「N 个实体已隔离」 genuinely
overstate rollback behaviour. D-05b's reasoning — that fixing it would change rollback's
side-effect surface and is a separate decision, not a cache-consistency question — is
sound. **This appears in the record as a known limitation, never as an implied fix.**

### Out-of-scope-and-why

| Item | Why out of scope |
|------|-----------------|
| **`TestAuditWriterPreconditions` flake** (`internal/agent/hitl/audit_writer_test.go:59`, 5s `context.WithTimeout` on a file-backed SQLite store) | **Pre-existing, not introduced by this phase.** `git log -S` dates the line to `9613d4d` (2026-08-20, Phase 11). **I verified the serial claim: 3/3 of my own serial `go test ./internal/... -count=1` runs returned rc=0**, on top of the green run inside the gate (V1) and the green run inside the clean `git archive` tree (V3) — 5 green full-suite observations from this audit. The characterisation ("expires under CPU oversubscription; Phase 48 runs one job per line") is consistent with everything I measured. Reported honestly: this is a latent timing sensitivity in a Phase-11 test that would bite under a parallel CI matrix, and it is not this phase's to fix. Worth a backlog row when Phase 48 lays out its job matrix. |
| **REPO path portability (LOW-2)** | Cosmetically true on this host; already recorded as open. |
| **D-45-DEF-02 harness rewrite** | D-04d scoped this phase to *record + clean the work tree*. Verified: the file was never tracked (`git ls-files` empty, `git log --all -- <path>` empty), has no consumer, and is gone from the work tree. The record states plainly that no delete-commit SHA can exist — an honest limitation of the evidence, not a gap in it. |

---

## 4. Adversarial findings from this audit (not raised by the executor)

### A1 — The gate cannot lie about test outcomes. **Verified by fault injection, not by inspection.**

This is the single most important claim in the phase and I tested it destructively rather
than by reading the script. I added one test file to `internal/knowledge/` whose body is a
single `t.Fatalf`, re-ran the whole gate, and then deleted the file:

```
--- P4 unique failure set ---
--- FAIL: TestZZGateProbeDeliberateFailure
FAIL P4 failed (rc=1)
INJECTED_EXIT=1
```

The gate exits **1**, names the exact offending test, and stops before P5/P6/P7. There is
no `|| true` on the test path, no `grep -v FAIL`, no `set +e` around `go test`, and the
empty-log guard means a crashed toolchain or a failed WSL spawn cannot be misread as
"zero failures found". **A false green is not reachable through a test failure.**

I searched the three gate scripts for the classic laundering constructs and found none.
The waiver/`WAIVED_FAIL` mechanism that let Phase 45/46 pass P2/P4 by *naming* a known
failure is genuinely gone — what remains in those files is prose explaining the retirement,
not code.

### A2 — The FE-skip branch is honest in output but still returns exit 0. **Real residual risk for Phase 48.**

HIGH-1's fix is real and correct: `P6_ran` gates the banner, `PARTIAL PASS — FRONTEND
PARTITION NOT VERIFIED` replaces `ALL PASSED`, and P6 moves out of `Partitions verified`
into a `NOT VERIFIED` block. A reader of the output cannot be misled.

But the **exit code stays 0**, deliberately, so a WSL-only host can still run the gate.
That means any automation keying purely on `$?` — which is exactly what a CI step does —
would read a **green** from a run that executed zero frontend tests. Today this box has
node on the Windows PATH, so the real gate prints `ALL PASSED` with 689 tests actually
executed (V1), and the risk does not materialize. It becomes live the moment Phase 48
wires this gate into Actions on a runner where node is not on PATH.

**Recommendation for Phase 48 (not a Phase 47 gap):** CI must not rely on this script's exit
code alone. Either assert the banner does not contain `PARTIAL PASS`, or pass
`SKIP_FE=1` semantics forward so a skip is a CI failure. I am not scoring this as a
must-have failure — the phase's own hard constraint is exit 0, which is met — but it is
the one place where a future automation could inherit a false green.

### A3 — P4's package scope (`./internal/...`) is narrower than the CI scope (`./...`) the phase's own rationale invokes. **No failure hidden today; latent divergence confirmed.**

D-04a's stated justification for zeroing was that *"`verify-phase-46.sh`'s `waiver_ok` is a
WSL shell mechanism that GitHub Actions never reads — CI runs a bare `go test ./...`"*.
The gate's P4 runs `./internal/...` only, and I counted **18 tracked `_test.go` files
outside `internal/`**: `cmd/eval` (2), `cmd/server` (2), `cmd/spike-deep`, `cmd/spike-fanout`,
`cmd/spike-history` (2), `cmd/spike-hitl`, `scripts/eval_snapshot`, `scripts/spiked` (4),
and `integration/contract` (4). All except `integration/contract` — which is behind a
`//go:build contract` tag and so is excluded from a bare `./...` run — are untagged and
**would** run in CI.

I checked whether this hides anything: **V4, `go test ./... -count=1`, exits 0**, with all
8 out-of-scope packages green. So **the "local gate == CI gate" claim holds empirically
today**, and no failure is being masked.

It is still worth recording, because the guarantee is *incidental*, not *structural*: a test
added tomorrow under `cmd/` or `scripts/` would be executed by Phase 48's CI and would
**not** be gated by this phase's P4, so the two would silently diverge again — the exact
failure mode D-04a was created to eliminate. Note this is **not** a violation of SC-4,
which defines the full gate as P4 `./internal/...` verbatim; the gate satisfies the letter
of the SC. The suggestion is to widen P4 to `./...` in Phase 48 when the CI command is
actually fixed.

---

## 5. Direct answers to the two questions asked

### Is the baseline reproducible? **Yes — verified, not asserted.**

A `git archive HEAD` checkout containing only tracked files reproduces green end to end
(V3): `RC_build=0`, `RC_test_internal=0`, `RC_det100=0`, `RC_emitter=0`, `RC_test_all=0`,
in a tree holding **3** `skills/` directories and **6** `knowledge/sql-guides/`
sub-directories — precisely the counts that made the pre-Phase-47 suite red. No untracked
directory is required for any green. The frontend suite is green at 689/689 with no skip
and no waiver (V5). The determinism evidence is at the strength the phase demanded
(`-count=100` + `-shuffle=on`), not the weaker two-run smoke threshold.

### Can the gate be trusted not to lie? **Yes for test outcomes; with two named caveats.**

The failure-injection test (A1) is the reason for that answer. A deliberately failing test
produced exit 1 with the test named. There is no waiver mechanism, no allowlist, no
`-run` narrowing on the full-suite partitions, and no grep-based pass/fail judgment that
could mistake a crash for a clean run. The two caveats are stated plainly and neither
materializes on this host today:

1. **A2** — a skipped frontend partition still exits 0 (by design, so a WSL-only host can
   run the gate at all). The *output* is unambiguous; the *exit code* is not. Phase 48 must
   not consume this exit code without also checking the banner.
2. **A3** — P4 covers `./internal/...` while CI will cover `./...`. The delta is green today
   (V4), so nothing is masked, but the parity is incidental rather than structural.

One further honesty note, because a green that was bought rather than earned is exactly
what this phase exists to prevent: **no must_have in this phase was made to pass by
weakening a test.** The two tests most at risk of that treatment were checked directly —
the determinism test kept whole-struct equality and explicitly forbids widening `InDelta`
in its own comments, and the emitter fixture was checked for falsifiability rather than
merely for a green exit code.

---

## 6. Score summary

| Plan | Truths | VERIFIED | PARTIAL | FAILED |
|------|--------|----------|---------|--------|
| 47-01 | 3 | 2 | 1 | 0 |
| 47-02 | 4 | 4 | 0 | 0 |
| 47-03 | 3 | 3 | 0 | 0 |
| 47-04 | 3 | 2 | 1 | 0 |
| 47-05 | 3 | 3 | 0 | 0 |
| 47-06 | 4 | 4 | 0 | 0 |
| 47-07 | 3 | 3 | 0 | 0 |
| 47-08 | 5 | 5 | 0 | 0 |
| **Total** | **28** | **26** | **2** | **0** |

**Status: `passed`.** The phase's hard constraint — `verify-phase-47.sh` exits 0 under WSL
`CGO_ENABLED=1` with a zero-waiver full gate — is met and was reproduced independently, and
the gate was proven capable of failing before its green was accepted.

**Carry-forward to the orchestrator / Phase 48:**
- REL-01/02/04 traceability rows at `REQUIREMENTS.md:75,76,78` still read `Pending`.
- A2 — CI must not consume this gate's exit code without checking for `PARTIAL PASS`.
- A3 — align P4's scope with the CI command when the matrix is fixed.
- The `TestAuditWriterPreconditions` 5s-timeout flake deserves a backlog row before
  Phase 48 introduces parallelism.

---

*Verified: 2026-10-19T14:05:00Z | Verifier: Claude (goal-backward audit) | Gate re-run, failure-injection test, clean-checkout reproduction, FE suite, and 3 serial full-suite runs all executed by this pass*
