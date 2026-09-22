---
gsd_state_version: 1.0
milestone: v1.4
milestone_name: — Ontology Layer
status: executing
last_updated: "2026-09-22T10:10:30.554Z"
last_activity: 2026-09-22 -- Phase 40 planning complete
progress:
  total_phases: 11
  completed_phases: 2
  total_plans: 25
  completed_plans: 15
  percent: 18
---

# State — SkillForge

## Current Position

Phase: 40
Plan: 9 planned
Status: Ready to execute
Last activity: 2026-09-22 -- Phase 40 planning complete

## v1.4 Phase Map

| Phase | Domain | Key Deliverables | UI-SPEC Target |
|-------|--------|------------------|----------------|
| 38 | Foundation (TBox + KG store + auto lift) | internal/ontology/ 目录；TBox emitter (server boot); memory-map KG store + JSON snapshot; auto schema lift (LLM 提议 → human approve) | `/admin/ontology` (TBox 类层次 + KG 三元组浏览器) |
| 39 | ECL Pipeline (section-aware chunker + entity extraction) | 替换 internal/knowledge/indexer.go byte-cap; LLM 抽 etl_* tables + 关系; ABox 自动写入 + 双向 KG↔Milvus 同步 | `/admin/pipeline` (ECL Pipeline 实时状态) |
| 40 | Data Governance (SHACL + Provenance) | SHACL shapes 启动校验; Provenance 链 (source_path); TBox 版本号 + 演进日志 | `/admin/governance` (校验报告 + provenance 图 + 版本日志) |
| 41 | Platform Layer (MCP + SPARQL + Hybrid Retrieval) | MCP server 暴露 KG; SPARQL-like REST /api/sparql; Hybrid Retrieval 1-2 跳 expansion | `/admin/platform` (SPARQL Playground + MCP Inspector + Hybrid 调试) |

## v1.4 Standards

- **UI consistency hard constraint:** 每个 web 涉及 phase 必须先产 UI-SPEC.md（约定 + 信息架构 + 关键交互 + 验收标准），再进入实现
- **UI 沿用** shadcn/Tailwind v4 + Vercel AI SDK v5 + `/admin/*` 路由约定
- **零硬编码**：ontology IRI 命名空间、SHACL shapes 路径必须可配置
- **Go 最佳实践**：无 cgo；handler ≤ 200 行；不允许静默吞错
- **三条护城河不可动摇**：Apache-2.0 单 binary + Skill 驱动 + Tier 信任隔离
- **全 SDLC**：每个 phase 配 verify-phase-NN.sh 验证脚本

## v1.4 Source of Truth

- 005 系列 5 个 spike 全 VALIDATED (`.planning/spikes/005a` 到 `005e`)
- Spike 005b TBox emitter 215 triples 已可机器生成
- Spike 005c section-aware chunker 解决 sql-guides 切分痛点
- Spike 005d-b memory-map KG 比 Kuzu 快 9× 轻 1500×
- Spike 005e hybrid retrieval: KG 门控 + Milvus 重排

## Deferred Items (carried from v1.3)

| Category | Item | Status |
|----------|------|--------|
| operator_gate | BGE-M3 endpoint restore + compliance replay re-sign | Deferred |
| operator_gate | Docker live build+run verification | Deferred |
| operator_gate | Phase 18 live E2E | Deferred |
| env_issue | gcc linking (MSYS2 emutls ABI) | Deferred |

## v1.3 Summary (closed 2026-09-09)

**Phases:** 31-37 (7 phases, 26 requirements)
**Audit-driven:** 18 critical + 27 warning + 14 info findings
**Trust semantics preserved:** KNOW-02 fail-closed, D-09 拒答构造, T-3-AUDIT INSERT-only, D11-C3 audit-first, D14 wire 字节不变, HITL-07 3-way frozen_args
**Key fixes:** REALBUG-01/02/03, PERF-01, SRP-01..04, HITL-01..03, REPEATHTTP-01..03, REPEATFE-01..03, SILENTAUDIT-01, STORE-01/02, ROBUST-01..05, CONCUR-01

## Project Reference

**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes. From "application" to "platform" via the Skill schema.

**Demo Skill:** 企业合规 / 制度审查

**Audience:** Open-source personal brand.

**Stack (locked):** Go + Eino v0.9.14 + Hertz SSE + Milvus + BGE-M3 + React 19 + Vite + TS + Vercel AI SDK v5 + shadcn/Tailwind v4

**License:** Apache-2.0

## Accumulated Context

### Roadmap Evolution

- 2026-09-09: **v1.3 milestone closed** — code quality remediation from project-audit-2026-09-09.md. Phase 31-37 derived from 26 REQ-IDs. 23/26 REQ closed (88%); 4 项 tech debt.
- 2026-09-21: **v1.4 milestone started** — Ontology Layer based on 005 series (5 spikes all VALIDATED). Phase 38-41 derived from 4 capabilities (Foundation / ECL / Governance / Platform). UI 一致性 hard constraint: 每 web phase 先产 UI-SPEC.md.
- 2026-09-21: **Phase 38 COMPLETE** — Foundation (TBox + memory-map KG + induction). 6 plans / 6 commits (29ad95b..1bd93cc); hermetic gate 13/13; ONT-01..09 verified; `internal/ontology/{emitter,store,induction}` + 8 admin routes + 本体 tab. Notables: triple baseline 176 (源演化, spike 快照 215); rollback UX → Phase 40 (D-TBox-03); approve 闭环含 v{N+1}.ttl 归档 + 5s debounce snapshot.

---

*Last updated: 2026-09-22 — Phase 39 all 9 plans executed (39-09 complete: golden set + F1 harness + hermetic gate)*

## Decisions

- [Phase ?]: 39-01 store: DeleteSubject rebuilds all three indices; InsertUnique dedupes under one write-lock — Planner-pinned approach from 39-PLAN Task 1
- [Phase ?]: 39-01 TBox v2: emitter 8 classes / 8 objProps (+NoteChunk +TableSchema +belongsToNote +referencesTable per D-TBox2-01); belongsToNote declared-only, zero ABox instances — D-TBox2-01 locked; NoteChunk sentinel string is the Plan 06 boot upgrade trigger
- [Phase ?]: 39-02 Milvus schema 11 fields: +source_idx VarChar(64) / section VarChar(256) / section_idx Int64 / section_count Int64 appended after effective_to (D-Milvus-01 native fields, NOT JSON-in-source); widths single-homed in config consts; deploy requires SKILLFORGE_MILVUS_RECREATE=1 once (no new migration code)
- [Phase ?]: 39-02 DeleteByExpr: caller-built expr passes through VERBATIM (helper never interpolates; %q escaping is caller duty per filter.go precedent); empty/nil expr rejected before any DML; mechanics = DeleteTierRows verbatim (Has no-op → Load → Delete → Flush)
- [Phase ?]: 39-02 GetMaxIDCollection: always-true `id >= 0` expr watermark — ECL chunk-ID allocation must start above the collection-wide max (per-tier GetMaxID under-estimates; Pitfall 5 duplicate-generation class)
- [Phase ?]: 39-03 ECL chunker = 005c spike port verbatim (D-ECL-02): WalkAndMerge/WalkFragments/MergeFragments + IsSecFragment (secRegex AND non-empty section frontmatter, both required) + ChunkMeta 7-tag contract; CRLF-normalized pre-split means body_bytes self-consistent but may differ from spike chunk_metadata.json on CRLF files (counts identical)
- [Phase ?]: 39-03 stub extractor related-notes anchor pinned strict (参考/见-word + 3-digit source_idx): corpus has ZERO explicit numeric cross-references today (only generic-noun collisions + one title-based ref, cited in stub.go) → drafter emits [], human review owns relations per D-GT-01
- [Phase ?]: 39-03 stub kind mapping: DELETE FROM / MERGE INTO → "update" (delete not in D-Ext-01 enum), SELECT…INTO → "select"; positional first-seen dedupe by table name; ValidateExtraction exported for Plan 04 LLM-output gating
- [Phase ?]: 39-04 JSONObjectResponseFormat exported end-of-file in llm/extract.go — below ExtractVerdictJSONWithStructuredOutput so the T-P12-7 sha256 line-range baseline stays byte-stable (insertion above tripped the tamper guard, relocated in 4c0e52f)
- [Phase ?]: 39-04 NewLLMExtractor(chat, sectionCap ...int) frozen cross-plan contract (Plan 06 calls ecl.NewLLMExtractor(cm)); default cap 40000 runes; over-cap → per-section calls with union (first kind wins, related deduped source_idx+relation)
- [Phase ?]: 39-04 F1 conventions: zero denominator → 0.0 for that metric; kind secondary check bites only when both sides specify; macro-average counts absent notes as zero rows; LoadGolden errors carry filename:line, never silent
- [Phase ?]: 39-04 ApplyNote ordering pinned: validate-before-mutate → capture refs → DeleteSubject(note) → orphan sweep → Insert/InsertUnique → onChange exactly once; ONT-13 left unchecked (KG half done, Milvus half = Plans 05/06)
- [Phase ?]: 39-05 ecl_failures: single upsert write path (INSERT ON CONFLICT(note_id) DO UPDATE — status transitions ARE the point, D-DL-01 sanctioned non-audit); updated_at RFC3339 UTC set by the manager, never SQL default; narrow FailureStore seam means sqlite imports ecl for the conformance check → failures_test.go lives in external ecl_test pkg
- [Phase ?]: 39-05 chunk: splitter separator ladder terminates with "" (recursive splitter's no-separator-left branch would pass oversized text through unsplit — unreachable by construction); BuildDocuments post-condition errors over 3500 runes (T-39-13); Document.SectionIdx emitted 1-BASED per Plan 39-02 retriever contract (0 = legacy zero-fill), not merge's 0-based index
- [Phase ?]: 39-05 manager: done/total/queue_len are per-stage counters (done resets at stage entry, total = len(notes)); RetryNote claims the running flag for its synchronous run (concurrent Run → 409 path); corpus-missing retry target parks dead + ErrUnknownNote; milvus-load batch failures park all batch notes dead via individual per-note rows; SeedFunc seam keeps milvus imports out of ecl (grep-verified); extract worker pool ≤ 3 LLM calls (race-detector green)
- [Phase ?]: 39-06 indexer ECL mode: routing branch sits BETWEEN splitFrontmatter and maxDocBytes (oversize __secNN must reach the ECL skip — behavioral + static source-order guard); SetECLMode applied BEFORE buildRetriever so the boot-time Index itself routes; sql-guides legacy purge = `tier in ["sql-guides"] and section_count == 0` (%q via purgeExprFor), ECL-off path byte-identical (single DeleteTierRows)
- [Phase ?]: 39-06 TBox upgrade: UpgradeIfNeeded 3-state (missing pointer → v1; NoteChunk sentinel → strict no-op never re-emits; Phase-38 pointer → one-time v{N+1} + pointer swap + version bump + induction.ApprovedTriples replay); store.Restore = snapshot-first + InsertUnique dedupe merge, corrupt snapshot ERRORS (silent TTL fallback would be the ABox wipe itself); boot WriteSnapshot persists the MERGED store
- [Phase ?]: 39-06 boot wiring: deps.ECL field pre-added in server.Deps (Rule 3 — Plan 07 registers routes against it, do not re-declare); ecl manager armed with cm + KG + sqlite FailureStore + SeedFunc closure (marker purge section_count > 0, GetMaxIDCollection watermark allocator, shared embedder) + SetOnChange shares induction kgDebounce; mongo/none → nil + WARN (A7); boot never calls Run (D-Run-01, grep-proof)
- [Phase ?]: 39-07 pipeline routes: run = CASRunning claim → detached goroutine on context.Background() (run outlives request; failed run WARN-logs via log.Printf) → immediate 200 {"status":"started"}; status = StatusDTO marshaled VERBATIM (Plan 05 owns the json tags — no field re-mapping, one source of truth); retry = ^[0-9]{3}$ format gate 400 (T-39-10 layer 2) BEFORE manager, then errors.Is mapping ErrPipelineBusy→409 / ErrUnknownNote→404 with pinned machine-stable strings; all 3 body-less (no BindAndValidate — D-Route-01); server-side test fakes rebuilt in external server_test pkg (unexported backoff never touched — all-succeeding stubs mean zero sleeps)
- [Phase ?]: 39-08 FE pipeline tab: PipelineApiError {status, message} — BE error envelope unwrapped VERBATIM (network → {status:0, 网络错误}, never a fabricated status); PipelineTab branches 403/409/503 on .status; unknown stage keys render the raw string AND count as running (isRunning = stage ∉ {idle, done} drives cadence + Run button + pulse badge); no-data poll states render 「—」, never fabricated zeros
- [Phase ?]: 39-08 polling hook co-located in PipelineTab.tsx (UI-SPEC locks 3 new files): cadence (isRunning ? 2000 : 10000) sits in the interval-effect deps → flip re-arms immediately with a same-tick refetch (useEvalReports shape); Run/retry POSTs bypass the poller + trigger immediate fetchOnce; poll errors never stop the loop; inline error line clears on next success
- [Phase ?]: 39-08 test suite: vi.useFakeTimers tick-by-tick cadence proof (2s while extract → 10s after done flip); stage-shaped makeStatus fixture + statusOverrides for never-run/idle/dead-letter states; object-holder instead of closure-assigned callback (TS CFA narrows `let` to never at the release site)
- [Phase ?]: 39-08 stale FE pins surfaced by first FE whole-suite gate since Phase 38: OntologyTab count 8→9; AdminFeedbackTabs 7-tab order array extended with 本体 + ECL 管线 (that test had been failing since Phase 38 — 39-07 was Go-only so it never ran); pnpm pre-run hook broken by protected untracked workspace files → FE gates run via direct node tsc/vitest invocation
- [Phase ?]: 39-09 golden set policy A (writes-only) locked by human review 2026-09-22: read-only FROM/JOIN etl_* refs deliberately NOT golden as select; touch-semantics prompt vs writes-only golden = documented live-gate risk (revisit only if live F1 nears 0.8)
- [Phase ?]: 39-09 row 039 human-corrected: -4 CREATE OR REPLACE FUNCTION false positives (3 fn names + 1 param), kept 3 merge-into update targets — harness scores 039 F1 0.6 BY DESIGN; macro tables 0.9846 / related 1.0 vs gate >= 0.8
- [Phase ?]: 39-09 harness scoring convention: empty-vs-empty notes score (1,1,1) agreement at the harness level; ScoreTables/ScoreRelated's pinned 0.0 zero-denominator convention stays LLM-gate-only
- [Phase ?]: 39-09 verify-phase-39.sh: regex -run stages pair exit code with a minimum --- PASS count (anti-vacuous); FE stage cascade node→pnpm→PHASE39_FE_GATE=skip-external explicit-first override (this box's WSL: no node, interop disabled, broken Windows pnpm shim on PATH; FE suite ran out-of-band Windows-side 628/628 green)
