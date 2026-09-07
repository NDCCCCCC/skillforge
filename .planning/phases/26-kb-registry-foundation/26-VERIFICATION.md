---
phase: 26-kb-registry-foundation
verified: 2026-09-07T05:56:09Z
status: passed
score: 54/54 must-haves verified (49 plan truths + 5 roadmap SCs; 1 via override) + 3/3 human items resolved 2026-09-07
overrides_applied: 1
human_verification_resolved: 2026-09-07 (browser walkthrough + post-fix live smoke; see Human Verification Resolution section)
overrides:
  - must_have: "四语义角色语义写入平台地板文档与 README (SC4 documentation anchor)"
    reason: "internal/platformfloor/platform_floor.go is byte-frozen by the D14-B4 red line and must not be reworded; the four-role semantics are instead dual-anchored in README 'Knowledge Bases' (lines 223-234) + the internal/kb package godoc (kb.go:5-38, 76-91) + docs/threat-model/STRIDE-26.md. Documentation substance delivered; only the file location deviates, and touching the prompt floor would have violated a stronger invariant."
    accepted_by: "developer (accepted in phase record; REVIEW-FIX + orchestrator handoff)"
    accepted_at: "2026-09-07T13:22:00Z"
re_verification:
  previous_status: none
  previous_score: n/a
  gaps_closed: []
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "Smoke the post-fix tree live: chat query with citations + one memory:facts HITL skill run on the CR-01..04-fixed binary (rebuild at 1ebbd1a, fresh restart, one chat turn + one HITL approval)"
    expected: "Retrieval still returns hits with a cited verdict; memory:facts answers without cross-session mis-attribution; no 'kb registry required' errors"
    why_human: "The UAT 5/5 PASS was executed at 12:03-12:22 on the PRE-fix binary; the four Critical fixes (e4211c6/adce584/dc05ca7/629590a) landed at ~13:22 and have hermetic test coverage but no live run. CR-04 was explicitly flagged for human verification by the fixer's logic-bug policy."
  - test: "Open /admin in a browser, switch to the Knowledge Bases tab, walk create-form validation (bad slug, bad role, duplicate slug) and one deactivate"
    expected: "Form errors surface the server rule text; created base appears in the list; deactivated base shows the inactive state and stays listed"
    why_human: "UAT criterion 4 was API-driven (status codes 201/200); the FE tab's visual rendering, form validation UX and error copy are browser-only behavior that no automated check in the phase record covers."
  - test: "Visual pass of registry-driven labels: TierBadge on citations, graph node labels, and the SourceDocument/DataSubprogress/Inspector tier labels against a live run"
    expected: "Badges show CN display names (权威/历史/个人/草稿) resolved from the registry; unknown slug degrades to [slug]; no blank badges"
    why_human: "Vitest asserts the components with mocked caches; real display-name resolution over the wire is visual."
---

# Phase 26: 知识库注册表地基 (KB Registry Foundation) — Verification Report

**Phase Goal:** 管理员可以创建/停用知识库，Skill `reads` 契约与检索/索引两侧全部按注册表 fail-closed 校验，四语义角色语义重定义落档——知识层从「硬编码 4 tier 枚举」变为「注册表驱动的运营平台地基」，信任语义不削弱；同时建立里程碑卫生门（TODO 清零 + 结构门）。
**Verified:** 2026-09-07T05:56:09Z
**Status:** human_needed
**Re-verification:** No — initial verification
**Code range verified:** `6cda7d4..1ebbd1a` (planning `5fd489b..f04d33b` excluded)

## Goal Achievement

All 5 roadmap Success Criteria are observably true on the final tree. The trust-contract
invariants hold: `kb.Citable` is the single citability predicate with all four insertion
points delegating to it; the nil-registry legacy posture is explicit and unambiguous;
admin 403 precedes 503 on every admin surface; KB audit rows are INSERT-only; the
`knowledge_bases` DDL is additive; and the platform floor is byte-unchanged since `174ee18`.

### Observable Truths

Roadmap Success Criteria (the contract):

| # | Truth (SC) | Status | Evidence |
|---|-----------|--------|----------|
| 1 | 管理员可经 token 门端点 + /admin 视图创建/停用知识库；停用从检索与 Skill 校验消失、数据不删；变更均写 INSERT-only 审计行 | ✓ VERIFIED | `internal/server/kb_admin.go` (349 lines): GET/POST `/api/admin/kb` + POST `/api/admin/kb/:slug/deactivate`; 403→503 CHECK ORDER in all three admin handlers; 409 on duplicate (List-scan incl. deactivated), 404 unknown, 400 carries rule text; `putKBAdminAuditRow` → `store.PutAudit` with `action_kb_created`/`action_kb_deactivated` + entry snapshot in FrozenArgs; `internal/server/kb_admin_test.go` asserts 400/403/404/409/200/201/503 + both audit actions via a recording store; `TestKBAdminCheckOrder` proves 403 with nil registry. FE: `web/src/components/admin/KnowledgeBaseList.tsx` (363 lines, ≥80 required) with X-Admin-Token create/deactivate. Restart survival locked by `TestRestartDeactivateStaysDeactivated` + `TestOpenAdminCreatedRowSurvives`; live-confirmed by UAT ④ (201 → deactivate → restart → `active:false`, created_at unchanged, public projection excludes it) |
| 2 | `reads` 未注册/停用 → 加载期 fail-closed；旧 4-tier 字面量不再被接受；既有 Skill 迁移后可加载 | ✓ VERIFIED | `skill.TierEnum`/`TierEnumByValue`/`TierEnumValues`/`knownTierDirs` deleted — live-tree grep (excluding `.claude/worktrees/`) returns zero hits; `internal/skill/tier.go` `ResolveTiers(reads, reg)` is a thin delegate to `reg.ResolveReads` with a nil-registry fail-closed error; `registry.go ResolveReads:259` errors with `(active: %v)` listing the active slug set; `internal/kb/registry_test.go:262 TestResolveReadsDeactivatedSlug`; the 7 SKILL.md files load without renaming (breaking = CN value removal + enum deletion, verified by `go build`/`go test` green); `datarules/loans.yaml` now `tier: historical`; eval golden migrated to slugs (below) |
| 3 | 索引侧与检索侧对未注册知识库值均 fail-closed；Milvus/Mongo 过滤注册表驱动；KNOW-02 不削弱 | ✓ VERIFIED | Index side: `internal/knowledge/indexer.go:297` `reg.Lookup(d.Tier)` in validateTiers; graph side re-check `internal/knowledge/graph.go:219` `b.reg.Lookup(m.Tier)` → `ErrInvalidTier{...ActiveSlugs}`; retriever side: `internal/retriever/milvus/config.go:51 DefaultTierMaxLen = kb.MaxSlugLen` (single home), `filter.go` comment "callers now pass kb.Registry-derived slugs", mongo `seed.go:85` walks via the shared walker; empty slug set → `false` / `$in:[]` constructive refusal preserved (D-09); assertion surface (`TestFanoutLock*`, KNOW-02 tests) green |
| 4 | 四语义角色语义落档；citation 白名单代码强制仅 authority | ✓ VERIFIED (override on documentation anchor) | Code: `internal/kb/kb.go:114 const CitableRole = RoleAuthority` + `kb.Citable(reg, tier)` (:267) as the single predicate; all four insertion points delegate — `internal/agent/trust_floor.go:119`, `internal/server/pipeline.go:1051` (`citableChunksByID`), `internal/runtime/trust_stage.go:251`, `internal/runtime/fanout_runtime.go:250` (pre-dedup drop); pinned by `TestCitableRoleIsAuthority`. Docs: README §Knowledge Bases (:223-234) names `internal/kb.CitableRole` as the code anchor; kb.go package godoc (:11-30, :76-91) is the code-level canonical statement; STRIDE-26.md registers the five registry-era threats. Platform floor byte-unchanged — see override |
| 5 | 启动期 `knowledge/<slug>/` 按注册表装载（未知目录 WARN+跳过+审计）；TODO/FIXME 归零且有结构门 | ✓ VERIFIED | `internal/knowledge/walk.go` `WalkRegisteredDirs` — three-state table (active → index / deactivated → WARN no audit / unregistered → WARN + `action_kb_dir_unregistered` INSERT-only audit), nil-registry fails closed, service never fails on unknown dirs; consumed by both backends (`indexer.go:146`, `mongo/seed.go:85`); `cmd/server/main.go` binds the hook to real sqlite (`TestAuditHookLandsInSqliteAuditTable`). Hygiene: `scripts/verify-phase-26.sh` 3 stages — **run by this verifier: 3/3 PASS, exit 0**; `cmd/spike-history/long.go:55-62` MEM-05 TODO factually rewritten (summarization middleware never happened; real escape is D10-B3 `TruncateToCharBudget`, reference target exists) |

Plan-level truths (49 across 10 plans): all VERIFIED. Per-plan spot-check evidence:

| Plan | Truths | Spot-check result |
|------|--------|-------------------|
| 26-01 | 5 | `kb.go` (276 ≥60), `slug.go` (77), `seed.go` (158), `registry.go` (363 ≥120), `knowledge_bases.yaml` (4 baseline KBs, CN display names, draft with `retrieval_enabled: true` + role-outranks-switch comment); `atomic.Pointer[snapshotState]` copy-on-write; draft never in `RetrievalEligible` (`registry.go:154` unconditional exclusion); `ValidateSlug` whitelist + reserved words; malformed seed = error / absent = only legal degraded state (`seed.go:75-103`, strict `KnownFields(true)`) |
| 26-02 | 4 | `schema.go:364-372` additive `CREATE TABLE IF NOT EXISTS knowledge_bases` + `CHECK (role IN ('authority','historical','personal','draft'))` + migrate-tail role index (`TestRoleCheckConstraint`, `TestMigrateIdempotent`, `TestExistingDBUpgrades`); `kb_registry.go` `var _ kb.Store = (*SQLiteStore)(nil)`; `open.go` structure-only upsert passes the persisted switch back through (`RetrievalEnabled: cur.RetrievalEnabled`) — seed cannot resurrect an admin deactivation; no delete path in the file; `TestUpsertBaseIdempotentUpdate` |
| 26-03 | 5 | Single walker `WalkRegisteredDirs` used by both `indexer.go:146` and `mongo/seed.go:85`; `graph.New(r, reg)` (:169) with dynamic node set from `reg.List()` + Lookup filter (:258-261); `cmd/server/main.go` initially `LoadSeedRegistry` → replaced by `kb.Open` in 26-09 (no PROVISIONAL residue: grep clean); `indexAuditHook` package var + `SetIndexAuditHook` callback injection |
| 26-04 | 5 | Enum family deleted (live-tree grep zero); `internal/skill/tier.go` thin delegate keeping the name; `loader.go:74 / registry.go:145 / wizard.go:83 / manifest_builder.go:155` all carry `reg kb.Registry`; `internal/data/rules.go:157-162` `reg.Lookup(m.Tier)` + `Role == kb.RoleDraft` rejection; `datarules/loans.yaml` migrated; `Deps.KBRegistry` in `internal/server/server.go` |
| 26-05 | 6 | `kb_admin.go` create/deactivate/public endpoints + INSERT-only audit; deactivate stays in `List()` but vanishes from `Lookup`; CHECK ORDER; `annotate.go:97` `s.KBRegistry.SlugsByRole(kb.RoleHistorical)` replaces `[]string{"历史"}`; `knowledge.go` — `validKnowledgeTiers`/`tierDirEn` gone, docs/doc gated by `kb.ValidateSlug` + `Lookup` double traversal gate (:369-376); `TestCitableChunksByID` + `TestPipelineNilRegistryGraceful` keep the chunksByID snapshot contract |
| 26-06 | 5 | `trust_floor.go` `Add`-point gate via `citable()` → `kb.Citable`; nil registry = legacy posture, `SetKBRegistry` ignores nil so bound-then-cleared is impossible; `BuildToolsForSkill(..., reg kb.Registry)` with both production call sites passing the registry (`runner_setup.go:382` `cfg.KBRegistry`, `cmd/spike-deep/main.go:457` `kbReg`); `hitl_memory_facts.go:208-209` Enum from `ActiveSlugs()` snapshot + authority default via `SlugsByRole(kb.RoleAuthority)`; refusal keyword = registry DisplayName + `suggestionRoleSuffix` (`pipeline.go:1018,1059-1069`) — byte-identical copy under the baseline seed, frozen eval surface untouched (eval green) |
| 26-07 | 5 | `eval/golden/fanout.jsonl` — 24 lines (unchanged count), all 20 `tier_ground_truth` values slug vocabulary (19 `authority` + 1 empty for the mismatch case), zero CN values in truth/reads fields (CN survives only in `query`/`notes` prose, which are byte-preserved by design); `fanout.lock.json` re-signed with `_kb_registry_migration` note + both hashes; `internal/eval` tests green (run in WSL); calculator SKILL.md PLACEHOLDER comment now describes draft-role semantics; no CN tier data values left in non-eval data files (gate 3 proves it) |
| 26-08 | 5 | `KnowledgeBaseList.tsx` (363 ≥80) with create form + deactivate + token gate; `TierBadge.tsx` `[slug]` fallback; `chunk-lookup.ts` hardcoded `'权威'` fallback deleted → `tier: null` + badge hidden (documented in-file); the three named components (`SourceDocumentPart.tsx:35,52`, `DataSubprogressPart.tsx:21,62`, `InspectorRetrieval.tsx`) all consume `useTierLabel`/`displayNameFor` from `kb-bases.ts`; `Tier = string` (`types/verdict.ts:55`); `adminSubroute` union extended with `'bases'` (`app-store.ts:457`) |
| 26-09 | 5 | Boot order store → `kb.Open` → skill → Deps (`main.go:330-342` log.Fatalf on malformed seed; `TestBootAssemblyOrder`); sqlite persistence across restart (UAT ④ + `TestBuildKBRegistryFromStore`); `indexAuditHook` → `store.PutAudit` sqlite rows (`TestAuditHookWritesInsertOnly`, `TestAuditHookLandsInSqliteAuditTable`); `SKILLFORGE_MILVUS_RECREATE` env-gated `DropCollection` → `EnsureCollection` (`client.go:91-129`, `TestMilvusRecreateGateWiring`); WSL full gate green (orchestrator record at 1ebbd1a; trust-critical packages re-run by this verifier) |
| 26-10 | 4 | README §Knowledge Bases + kb godoc + STRIDE-26 (see SC4); `verify-phase-26.sh` 3 gates RED→GREEN documented (D8-04: Stage 01 proven red on long.go:54, stages on injected probes) and **independently executed green by this verifier**; repo-wide non-test Go TODO(/TODO:/FIXME(/FIXME: count = 0 (gate output); UAT runbook with operator-executable steps + all five criteria PASS on an authorized live run |

**Score:** 54/54 must-haves verified (1 via override)

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/kb/kb.go` | Role/CitableRole/Entry/Registry/Store + `Citable` | ✓ VERIFIED | 276 lines (≥60); substantive, leaf package |
| `internal/kb/registry.go` | Atomic snapshot Registry, 10 methods | ✓ VERIFIED | 363 lines (≥120); WR-02 fix: entries+sig in one immutable `snapshotState` |
| `internal/kb/slug.go` / `seed.go` / `open.go` | Whitelist / fail-closed loader / production merge | ✓ VERIFIED | 77 / 158 / 137 lines; all substantive + tested |
| `internal/store/sqlite/kb_registry.go` | kb.Store sqlite adapter | ✓ VERIFIED | Compile-time assert; fully `?`-parameterized; no delete path |
| `internal/knowledge/walk.go` / `indexer.go` / `graph.go` | Shared walker / registry validate / dynamic graph | ✓ VERIFIED | Three-state table; Lookup on both sides; N-dynamic node set |
| `internal/skill/tier.go` (+ validate/loader/data) | Enum deleted, registry delegate | ✓ VERIFIED | Thin delegate + nil fail-closed; all consumers migrated |
| `internal/server/kb_admin.go` / `annotate.go` / `knowledge.go` / `pipeline.go` | Admin surface + registry projection | ✓ VERIFIED | 349 lines (≥100); CHECK ORDER; citable closure single-sourced |
| `internal/agent/trust_floor.go` / `hitl_tools.go` / `hitl/hitl_memory_facts.go` | Insertion-point citability gate + hitl snapshot | ✓ VERIFIED | All substantive; both production call sites threaded |
| `eval/golden/fanout.jsonl` / `eval/frozen/fanout.lock.json` | Slug-vocabulary eval set + re-signed lock | ✓ VERIFIED | 24 lines; `_kb_registry_migration` note; eval tests green |
| `web/src/components/admin/KnowledgeBaseList.tsx` / `lib/kb-bases.ts` / `TierBadge.tsx` | FE admin view + registry projection | ✓ VERIFIED | 363 / 135 / 44 lines; vitest + tsc green |
| `cmd/server/main.go` / `internal/retriever/milvus/client.go` | Production assembly + RECREATE gate | ✓ VERIFIED | No PROVISIONAL residue; env-gated re-seed |
| `scripts/verify-phase-26.sh` / `Makefile` / `STRIDE-26.md` / `README.md` / `long.go` | HYG-01 gate + docs + TODO rewrite | ✓ VERIFIED | Gate executed green by verifier; Makefile target present |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `trust_floor.go` / `pipeline.go` / `trust_stage.go` / `fanout_runtime.go` | `kb.Citable` / `CitableRole` | single predicate call | ✓ WIRED | 4/4 insertion points delegate; zero independent role comparisons |
| `cmd/server/main.go` | `kb.Open` | production assembly | ✓ WIRED | `buildKBRegistry` at :206-211, fatal on malformed seed; 3× `KBRegistry: kbReg` wirings |
| `cmd/server/main.go` | `indexAuditHook` | sqlite INSERT-only audit | ✓ WIRED | Hook bound at boot; locked by `TestAuditHookLandsInSqliteAuditTable` |
| `internal/store/sqlite/kb_registry.go` | `kb.Store` | compile-time assert | ✓ WIRED | `var _ kb.Store = (*SQLiteStore)(nil)` |
| `internal/kb/open.go` | `SetRetrievalEnabled`/`UpsertBase` | structure-only seed merge | ✓ WIRED | Persisted switch passed back through; `TestOpenAdminCreatedRowSurvives` |
| `internal/skill/tier.go` | `kb.Registry.ResolveReads` | thin delegate | ✓ WIRED | Nil-registry fail-closed before delegation |
| `internal/server/kb_admin.go` | `kb.Registry.Create/Deactivate` + `PutAudit` | mutation + audit | ✓ WIRED | `action_kb_created`/`action_kb_deactivated` asserted in tests |
| `internal/server/annotate.go` | `SlugsByRole(RoleHistorical)` | counter-evidence set | ✓ WIRED | :97 — hardcoded CN list gone |
| `internal/server/knowledge.go` | `kb.ValidateSlug` + Lookup | traversal double gate | ✓ WIRED | :369-376; regex first, then active-registry check |
| `hitl/hitl_memory_facts.go` | `ActiveSlugs`/`SlugsByRole(RoleAuthority)` | Enum snapshot | ✓ WIRED | :208-209, per-request clone post-CR-04 |
| `eval/frozen/fanout.lock.json` | `eval/golden/fanout.jsonl` | LF-normalized sha256 | ✓ WIRED | `internal/eval` lock tests green in WSL |
| `web/src/lib/kb-bases.ts` | `/api/knowledge/bases` | public read-only fetch | ✓ WIRED | :66; 3-field minimized payload |
| `web/src/components/admin/KnowledgeBaseList.tsx` | `/api/admin/kb` | X-Admin-Token mutations | ✓ WIRED | :89, :136, :178 (create/deactivate/list) |
| `web/src/components/*` | `kb-bases.ts` | display-name projection | ✓ WIRED | `useTierLabel` consumers in all three named parts |
| `scripts/verify-phase-26.sh` | `internal/kb` Roles() | gate-3 vocabulary home | ✓ WIRED | Whitelist reasons reference the kb package; gate green |
| `cmd/spike-fanout/main.go` | golden tier vocabulary | slug sync | ✓ WIRED | `authority/**` / `[]string{"authority"}` throughout |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `KnowledgeBaseList.tsx` | bases list | `GET /api/admin/kb` → `Registry.List()` → sqlite `knowledge_bases` | Yes — sqlite-persisted rows incl. deactivated | ✓ FLOWING |
| `TierBadge` / graph labels | display names | `GET /api/knowledge/bases` → `Registry.List()` → seed/sqlite | Yes — registry `DisplayName` (seed `权威` etc.) | ✓ FLOWING |
| `annotate.go` counter-evidence | retrieval set | `Registry.SlugsByRole(RoleHistorical)` | Yes — live registry; empty set = constructive 0-hit | ✓ FLOWING |
| `hitl_memory_facts.go` Enum | slug enum | `Registry.ActiveSlugs()` snapshot at construction | Yes — registry snapshot; authority default via `SlugsByRole` | ✓ FLOWING |
| Boot registry | entry set | `kb.Open` → `LoadBases` + seed merge → re-read store | Yes — snapshot built from persisted reality (step 5 re-read) | ✓ FLOWING |
| `KnowledgeModal.tsx` file tree | docs by tier | `GET /api/knowledge/docs` tiers render verbatim (registry slug order); labels via `useTierLabel` | Yes — live-verified post-R-1-fix (05c096c): `[权威] 16` tree, server order, default-open first tier | ✓ FLOWING (R-1 resolved) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| HYG-01 structural gates | `bash scripts/verify-phase-26.sh` | 3/3 stages PASS, `PHASE 26 STRUCTURAL GATES: PASS`, exit 0 | ✓ PASS |
| kb + skill packages | `go test ./internal/kb/... ./internal/skill/... -count=1` (Windows) | both `ok` | ✓ PASS |
| agent / server / knowledge / store / runtime / data / eval | `go test ./internal/{agent,server,knowledge,store,runtime,data,eval}/... -count=1` (WSL go1.25.0) | 12 packages `ok`, 0 FAIL | ✓ PASS |
| FE suite | `npx vitest run` | 67 files, 497/497 passed | ✓ PASS |
| FE types | `npx tsc --noEmit` | exit 0 | ✓ PASS |
| Platform floor byte-freeze | `git diff --stat 174ee18 HEAD -- internal/platformfloor/` | empty diff; last touch commit is 174ee18 itself | ✓ PASS |
| Enum deletion | live-tree grep `TierEnum\|knownTierDirs` (`*.go`, excl. `.claude/`) | 0 hits | ✓ PASS |
| CN tier as Go data value | quoted-literal grep, non-test Go | only hit is a SQL doc comment (`schema.go:197`) | ✓ PASS |
| verify-phase-15 sha pin | `sha256sum eval/frozen/fanout.lock.json` vs `EXPECTED_FANOUT_SHA` | `f00f5b31…` matches the pin exactly — 26-07 updated the pin | ✓ PASS (residue R-4 resolved) |
| verify-phase-15 on this host | `bash scripts/verify-phase-15.sh` | FAIL at Stage 02: Windows gcc cannot link duckdb (`collect2.exe error: ld returned 5`) | ✗ FAIL — environment, not regression: backend builds are WSL-only per the env runbook; hermetic WSL suite green |

### Probe Execution

| Probe | Command | Result | Status |
| ----- | ------- | ------ | ------ |
| `scripts/verify-phase-26.sh` | `bash scripts/verify-phase-26.sh` | exit 0, 3/3 stages | PASS |
| Live migration acceptance (26-UAT) | operator-authorized run 2026-09-07 12:03-12:22 | 5/5 criteria PASS with logged evidence (documented in 26-UAT.md §6; run on the pre-CR-fix binary — see human verification item 1) | PASS (as documented; pre-fix binary) |

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
| ----------- | ------------ | ----------- | ------ | -------- |
| KBM-01 | 26-01, 26-02, 26-04, 26-09 | 注册表替代 TierEnum；条目含 slug/显示名/角色/描述/开关/时间戳；skill 消费接口 | ✓ SATISFIED | `internal/kb` contract layer; enum deleted from live tree; all consumers on the interface |
| KBM-02 | 26-02, 26-05, 26-08, 26-09 | admin 端点 + /admin 视图；slug 校验；审计；停用不可见不删除 | ✓ SATISFIED | kb_admin.go + KnowledgeBaseList.tsx + INSERT-only audit + no delete verb anywhere; UAT ④ |
| KBM-03 | 26-01, 26-05, 26-06, 26-10 | 四角色语义落档；citation 白名单仅 authority | ✓ SATISFIED (override on doc anchor) | `CitableRole` single source + 4 gated insertion points; README/godoc/STRIDE-26 |
| KBM-04 | 26-04, 26-07 | reads 按注册表 fail-closed；旧字面量不再接受；一次性迁移 | ✓ SATISFIED | Enum deletion + Validate fail-closed; golden/loans.yaml/SKILL.md migrations; existing skills load |
| KBM-05 | 26-03, 26-09 | 检索过滤注册表化；KNOW-02 双侧 fail-closed 保留 | ✓ SATISFIED | validateTiers/graph Lookup + slug-derived filter sets; empty set constructive refusal |
| KBM-06 | 26-03, 26-09 | 启动期目录按注册表装载；未知目录 WARN+跳过+审计 | ✓ SATISFIED | `WalkRegisteredDirs` three-state + sqlite-bound audit hook |
| HYG-01 | 26-10 | TODO 清零 + 结构门 | ✓ SATISFIED | Gate 3/3 green (verifier-executed); long.go:54 factually rewritten; zero debt markers in phase Go files |

**Orphaned requirements:** none — REQUIREMENTS.md maps exactly KBM-01..06 + HYG-01 to Phase 26 (lines 474-480, 500), and every one is claimed by at least one plan's `requirements` frontmatter.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| `internal/knowledge/indexer.go` | 172 | prose "placeholder" in a comment | ℹ️ Info | Documentation wording, not a stub |
| `internal/server/pipeline.go` | 49-53 | `retrievalScoreThreshold` placeholder constant | ℹ️ Info | Pre-existing Phase 2 artifact (D2-15), outside phase scope |
| `.claude/worktrees/*` | — | stale worktrees still reference `TierEnum` | ℹ️ Info | GSD scratch area, not the build tree; excluded from gates by design |

Debt-marker gate: zero `TBD`/`FIXME(`/`FIXME:`/`XXX` markers in any phase-modified non-test Go file. No vacuous assertions found in sampled tests (admin tests assert specific status codes and audit actions; trust-floor tests assert refusal semantics; the WR-02 test reproduced the torn read pre-fix). The verify chain captures exit codes explicitly (`|| rc=$?` + `exit 1`) — the only `|| true` instances are grep no-match handling, which is the pass condition, and each stage cross-checks its own pattern (Stage 01 anti-false-positive probe on `deep.TODO`). TDD chains verified: RED commits are test-only (e.g. `6672071` touches 5 `_test.go` files only).

### Trust-Contract Invariants

| Invariant | Status | Evidence |
| --------- | ------ | -------- |
| `kb.Citable` single source | ✓ HELD | 4/4 insertion points delegate (`trust_floor.go:119`, `pipeline.go:1051`, `trust_stage.go:251`, `fanout_runtime.go:250`); `TestCitableRoleIsAuthority` pins the constant |
| Nil-registry legacy posture | ✓ HELD | `kb.Citable` nil → true; `SetKBRegistry` ignores nil (bound-then-cleared impossible); `TestPipelineNilRegistryGraceful`; `ResolveTiers` nil → error (fail-closed) |
| Admin 403-before-503 | ✓ HELD | All three admin handlers check `isAdminRequest` before the nil-registry 503; `TestKBAdminCheckOrder` locks it with nil registry + unset token |
| INSERT-only audit | ✓ HELD | kb package never writes audit rows; server caller uses `PutAudit`; no UPDATE/DELETE on the KB audit path (the `audit.go:360` UPDATE is the pre-existing T-3-AUDIT `UpdateAuditMetrics` backfill, unrelated) |
| Additive DDL | ✓ HELD | `CREATE TABLE IF NOT EXISTS knowledge_bases` + role CHECK; `TestMigrateIdempotent` + `TestExistingDBUpgrades` green |
| Platform floor byte-freeze | ✓ HELD | `git diff 174ee18 HEAD -- internal/platformfloor/` empty |

### Human Verification Required

> **RESOLVED 2026-09-07** — see the Human Verification Resolution section below for per-item evidence.

1. **Post-fix live smoke (highest priority)**
   **Test:** Rebuild at `1ebbd1a`, restart the server, run one chat query expecting cited hits, and one memory:facts HITL skill run.
   **Expected:** Retrieval hits + cited verdict as in UAT ①; HITL tool answers with correct per-session attribution; no "kb registry required" errors.
   **Why human:** The 5/5 UAT run executed at 12:03-12:22 on the pre-CR-fix binary; CR-01..04 landed ~13:22. CR-04 changed request-scoping logic and was flagged for human verification by the fixer's policy. Hermetic `-race` coverage exists but no live run does.
2. **/admin Knowledge Bases tab visual walkthrough**
   **Test:** Browser: create-form validation (bad slug / bad role / duplicate), one deactivate, deactivated-state badge.
   **Expected:** Server rule text surfaces in form errors; list reflects create/deactivate.
   **Why human:** UAT ④ was API-driven; FE rendering is browser-only.
3. **Registry label rendering pass**
   **Test:** Inspect citation TierBadges, graph node labels, and part-component labels on a live run.
   **Expected:** CN display names resolved from the registry; unknown slug → `[slug]`; no blank badges.
   **Why human:** Unit tests use mocked caches; wire-level resolution is visual.

### Human Verification Resolution (2026-09-07)

Executed via chrome-devtools against the live stack: WSL server `/tmp/skillforge-uat26` on :8888
(process start 13:38 — AFTER the CR-01..04 fixes landed ~13:22, so every observation below is on
the post-fix binary) + Vite dev server :5173 with the R-1 fix (05c096c) live. Console: zero
errors/warnings across the whole walkthrough.

| # | Item | Execution | Verdict |
|---|------|-----------|---------|
| 1 | Post-fix live smoke — chat half | The chat verdict under inspection (「出差住宿费超标怎么办？」→ cited answer, `依据 权威` tier label, 2 citations rendered `[1][2]`) was served by the post-fix binary; KnowledgeModal / DocViewer / graph / admin surfaces all hit live registry paths with no "kb registry required" errors. **HITL memory:facts half: NOT EXECUTABLE as a live run — no shipped skill declares `memory:facts` writes** (skills/ = simple-qa + kpi-dashboard, both `writes: []`); the capability has no production routing surface today. Compensating evidence: full `-race` suite green + the residual `hitl.InjectDeps` seam is recorded as R-2. Reclassified as residue rather than a gap. | PASS (chat) / residue (HITL, folded into R-2) |
| 2 | /admin Knowledge Bases walkthrough | Visual pass: 5-row table renders slug/显示名/角色/描述/状态/操作 (4 seed bases active + `uat-check` deactivated R-5 residue). Create-form round-trip via UI: valid form created `walkthrough-tmp` (UTF-8 CN display name 走查临时库 stored correctly — the browser sends proper UTF-8, unlike the GBK curl in UAT §6); it appeared in the list active, then deactivation surfaced the audit confirm dialog (「停用知识库 walkthrough-tmp？该操作会写入审计日志。」) and flipped the row to 停用. Invalid slug `Bad_Slug!` surfaced the server rule text verbatim in a form error and created nothing. Cleanup: walkthrough-tmp left deactivated (no delete verb by design). | PASS |
| 3 | Registry label rendering pass | Live wire-level: KnowledgeModal tree renders `[权威] 16` / `[草稿] 0` / `[历史] 0` / `[个人] 0` (registry display names, bracketed badge form, server slug order, first tier default-open); DocViewer badge `[权威]`; graph center node `[权威]` + 16 doc nodes; ChunkSourceView citation badge `[权威]` + version/effective-date metadata via the 结论 tab → citation-cluster `[1]` click (the 引用原文 tab's bare-click no-op is the documented chunk-source mapping design, not a regression). No blank badges, no raw slugs in any label surface. | PASS |

**Frontmatter status flipped to `passed`** — the sole reason for `human_needed` (no browser-level
evidence) is closed, with item 1's HITL half honestly reclassified as residue R-2 rather than
pretended executed.

### Residues (recorded; judged NOT goal-blocking)

- **R-1 `web/src/components/layout/KnowledgeModal.tsx:40` — RESOLVED 2026-09-07 (commit 05c096c).** The CN `TIER_ORDER`/`TIER_DIR` hardcode is gone: the modal renders the server's tier array verbatim (registry slug order — KBM-01), labels resolve through `useTierLabel` (KNOW-05 bracketed badge form), the empty-tier dir hint is computed as `knowledge/<slug>/`, the offline posture derives the seed list from `kb-bases` (the whitelisted FE home), and an OK-but-empty tier list renders an empty tree instead of resurrecting the seed. DocViewer's tier badge joined the same pass (registry label; raw slug remains only in the loading line where it names the fetched path). Tests migrated to the slug world + registry projection stubs; new cases pin server-order fidelity, unknown-slug degradation, empty-registry answer, and dual-outage `[slug]` labels. FE suite 67 files / 500 tests + `tsc --noEmit` green; live walkthrough verified.
- **R-2 `hitl.InjectDeps` per-request-hook seam** — same CR-04 class on the `plan_approve`/`report:publish` singletons; documented in 26-REVIEW.md's Fix Round as out of scope for this round.
- **R-3 Pre-existing `-race` failure in `fanout_tools_test.go` (`stubKPICaller.Call`)** — test-fixture only, reproduced on base `9054cc4`, unrelated to Phase 26.
- **R-4 verify-phase-{14,15,16} fanout sha pin — RESOLVED, recording the correction.** The task brief listed this as an open ripple; verification shows `EXPECTED_FANOUT_SHA = f00f5b31…` matches the re-signed `fanout.lock.json` byte-for-byte (26-07 updated the pin, per its own inline comment). No ripple remains. (Those older gates still fail on this Windows host at the gcc/duckdb link stage — a pre-existing environment limitation, WSL builds are the sanctioned path.)
- **R-5 `uat-check` throwaway KB** left deactivated in the live sqlite registry (mojibake display name from a GBK curl). Deactivated = invisible to retrieval/validation; there is deliberately no delete verb, so it persists as an inert admin-list row. Operator may rename or ignore it.
- **R-6 Honest-accounting note** — "platform_floor zero-byte" in earlier plan/UAT shorthand means **byte-unchanged** files (both exist and are non-empty: `platformfloor/platform_floor.go` 9331 B, `agent/platform_floor.go` 2768 B). The freeze invariant verified here is byte-identity vs `174ee18`, which holds.

### Gaps Summary

None. All 49 plan must-have truths and all 5 roadmap Success Criteria are verified against the actual codebase, including the four trust-critical areas (KBM-03 citability single-sourcing, KBM-04 breaking switch, KBM-05 dual-side fail-closed, HYG-01 gates executed green by this verifier). The SC4 documentation anchor deviates from the literal "平台地板文档" wording and is recorded as an accepted override (D14-B4 byte-freeze outranks the wording; README + kb godoc + STRIDE-26 deliver the semantics). The `human_needed` status was closed 2026-09-07 via the browser walkthrough + post-fix live smoke (see Human Verification Resolution); the only non-executed sub-item (memory:facts live HITL run) is residue R-2, not a goal gap.

---

_Verified: 2026-09-07T05:56:09Z_
_Verifier: Claude (gsd-verifier)_
