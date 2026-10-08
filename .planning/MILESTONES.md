# Milestones

## v1.6 SPARQL Schema Detection + Entity SHACL Shapes (Shipped: 2026-10-08)

**Phases completed:** 2 phases, 7 plans, 12 tasks

**Key accomplishments:**

- Landed D-SPARQL-05 Option β in the store — `isSchemaTriple` gained a subject∩predicate allowlist arm (13 metaPredicates, label/comment fail-closed) with query-time `schemaSubjects` derivation, making skill individuals' `hasReadAccess` triples tier-visible (admin counts 17/10) while all D-SPARQL-03 red lines pass with zero assertion edits.
- Landed the phase verification layer on 45-01's β semantics — D-45-01 emitter conformance gate (13-family fixture, label/comment negative pin), SPARQL-03 handler-level parity test proving /api/sparql and /api/admin make identical schema/ABox judgments on one fixtureKG, and verify-phase-45.sh (SC4, 4 fail-fast partitions under WSL CGO_ENABLED=1, exit 0 with the D-45-DEF-01 named waiver).
- `sh:severity` 政策从引擎硬编码归位 shapes 数据（W3C §2.1.4 三值映射，sourcePath 特例退役）+ shapes 路径列表化基建（variadic Validate、ShapesPathsFromEnv env 单点、errors.Join 多文件聚合、nodeShape.Layer 源文件追踪），既有 13 测试断言零改动全绿。
- Violation 三层盖戳（layer/target_class，global 无 chip 哨兵）+ 每隔离实体如实下发 dangling_edge_count（隔离前反查 involves*/parentOrganization 入边，0 也恒发）+ `entities.ttl` 资产交付（必填=无条件发射、if-present=sh:class、entityCode 零约束偏离记录在头）+ SC2 隔离语义四测试组全绿（subject 不可见 / 对象位披露 / 跨 boot 稳定循环 / 构造性首跑零隔离）。
- boot（main.go）与 rollback（induction.go Step 11）两个 shapes 消费点统一到 `shacl.ShapesPathsFromEnv()` 单点——induction.go:383 从未生效的硬编码路径（静默 no-op）退役，D-PROV-09 rollback/boot shapes parity 缺口闭合，并由真 RED→GREEN 的 TestRollbackShaclParity 双 case 钉死（修复前 rollback SHACL 重校验零覆盖）；boot 链时序与隔离语义零改动。
- 46-02 盖的 layer/target_class 戳与恒下发的 dangling_edge_count 在 /admin/governance 渲染为三态徽章（tbox indigo / entity neutral / global dashed 无 chip 哨兵）、红黄区头 per-layer 计数 chips 与隔离区第 7 列「悬挂边」（0 渲染 0 / 缺失渲染 —）——46-UI-SPEC 冲突优先级 5 条取代项全部落地，FE 零推导（Honesty 红线 8/9），governanceApi.ts 咽喉点零改动，legacy 报告无徽章不炸，9 组件回归全绿。
- 照 verify-phase-45.sh 骨架逐段交付 `verify-phase-46.sh`——P1 CGO=1 build+vet → P2 ontology 四包套件 → P3 targeted shacl+induction 双门（7 个具名测试 + >= 7 --- PASS，实测 22）→ P4 ./internal/... 全量，D-45-DEF-01 waiver 执行时点复探后照 45 收录；本地 `bash verify-phase-46.sh` 两次实跑 exit 0 全绿，SC4 机器可执行。

---

## v1.5 Outstanding Closure (Shipped: 2026-10-02)

**Phases completed:** 2 phases, 5 plans, 13 tasks

**Key accomplishments:**

- One-liner:
- One-liner:

---

## v1.4 Ontology Layer (Shipped: 2026-09-23)

**Phases completed:** 4 phases, 31 plans, 59 tasks

**Key accomplishments:**

- KG store gains DeleteSubject (index-rebuilding note-level replace) + InsertUnique (TOCTOU-free dedupe merge); TBox emitter extended to the locked 8-class/8-objProp D-TBox2-01 schema with NoteChunk sentinel tests.
- 11-field Milvus schema with 4 native ECL section fields (D-Milvus-01), zero-value legacy seeding (D-Milvus-02), DeleteByExpr marker-scoped purge, and GetMaxIDCollection collection-wide watermark — the two seam functions Plans 05/06 inject
- Section-aware walk+merge chunker productionized from validated spike 005c (D-ECL-02) with the IsSecFragment routing predicate, the ChunkMeta contract, and a deterministic regex stub extractor — corpus-backed tests prove SC-1: 185/185 boundary completeness, 185 fragments → 26 notes → 69 sections, note 001 contains 3/3 corpus SQL targets.
- Extractor seam (LLM temp-0/json_object + regex stub) with 40K-rune per-section union guardrail, pure macro-F1 math over the D-GT-01 golden jsonl, and idempotent note-level ABox replace with orphan sweep and snapshot trigger
- SQLite ecl_failures dead-letter ledger behind a narrow FailureStore, the section→Document chunker with recursive sub-split of oversized sections (3500-rune cap), and the Manager state machine running merge → extract → kg-load → milvus-load with per-note retry ×3, batch isolation, idempotent re-enqueue, and the locked status DTO.
- ECL pipeline wired into its two highest-risk seams: the indexer routing branch + marker-scoped sql-guides purge (Pitfall 1) and the boot sequence — one-time TBox 6→8 upgrade (Pitfall 4), snapshot-first KG restore with dedupe merge (Pitfall 3 / D-KG-02), and the armed-but-never-auto-run ECL manager behind SKILLFORGE_ECL_ENABLED (D-Run-01).
- 9th admin tab 「ECL 管线」 per the approved 39-UI-SPEC: confirm-gated Run ECL, server-truth-only progress/queue/failure chips, 6-column failures table with dead-only retry, and a 2s/10s self-re-arming polling hook — all copy verbatim, zero new deps or tokens.
- Human-reviewed 26-row ECL golden set (policy A writes-only) committed with a stub-vs-golden macro-F1 harness gate (tables 0.9846 / related 1.0) and verify-phase-39.sh — the hermetic SC-1..6 all-green phase gate.
- Provenance struct
- `QueryScoped(subj, pred, obj string, allowedTiers []string) []Triple`
- New types
- `internal/ontology/store/store.go`
- `AdminDashboard.tsx`
- Three governance views complete the /admin/governance tab: quarantine red zone + warnings yellow zone (ShaclReportView), Cytoscape provenance graph with breadthfirst layout (ProvenanceGraphView), and version timeline with rollback lifecycle (TimelineView). Cytoscape ^3.34.0 pinned in package.json.
- `resolveSkillTiers(skillName string) []string`
- `TestShaclViolation`
- MCP stdio server spawned at Go boot, exposing 5 KG tools (skf_query/skf_skills/skf_chunk_for/skf_schema/skf_neighbors) with mandatory tier filtering via QueryScoped
- POST /api/sparql accepts triple-pattern queries, returns JSON triples with tier isolation via X-Skill-Name header
- Hybrid Retrieval pipeline: query → KG 1-2 hop expansion → Milvus hybrid top-50 → BGE-reranker-base → top-5. Tier isolation via X-Skill-Name header.
- 11th admin tab 平台 registered in AdminDashboard with Network icon, store union widened with 'platform', and PlatformTab placeholder shell created. AdminFeedbackTabs test extended to 11 entries.
- Replaced the PlatformTab placeholder with three full views (SPARQL Playground / MCP Inspector / Hybrid 调试) and all supporting TypeScript types and API wrappers. CodeMirror 6 dependencies added to web/package.json.
- 41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval

---

## v1.3 — 代码质量修复 (Code Quality Remediation) — ✅ Complete 2026-09-09

**Goal:** 闭合 `.planning/audits/project-audit-2026-09-09.md` 报告中的全部 18 critical + 27 warning + 14 info findings。

**Achieved:** 23/26 REQ closed (88%). 4 项已文档化 tech debt (see `v1.3-MILESTONE-AUDIT.md`).

**Phase summary:**

- Phase 31 — FE Real Bugs (REALBUG-01, REALBUG-02)
- Phase 32 — BE Real Bug + PERF (REALBUG-03, PERF-01)
- Phase 33 — SRP Long Function Split (4 handler ≤ target lines)
- Phase 34 — HITL Handler Template Extraction
- Phase 35 — HTTP Layer Deduplication
- Phase 36 — FE Deduplication + Silent Audit Errors
- Phase 37 — STORE + Robustness + CONCUR (5/8 REQ closed)

**Trust semantics preserved:** KNOW-02 fail-closed, D-09 拒答构造, T-3-AUDIT INSERT-only, D11-C3 audit-first, D14 wire 字节不变, HITL-07 3-way frozen_args.

**Verification:** 7 hermetic `scripts/verify-phase-NN.sh` gates committed.

**Audit report:** `.planning/v1.3-MILESTONE-AUDIT.md` (status: tech_debt).

---

## v1.2 — 知识库运营与反馈闭环 (Knowledge Base Operations & Feedback Loop) — ✅ Complete 2026-09-08

**Goal:** Knowledge layer from hardcoded 4-tier directories to operational KB platform.

Phases 26-30 shipped (KBM-01..06 / DOC-01..06 / GRF-01..05 / FBK-01..05 / TKT-01..04).

---

## v1.1.1 — Phase 21 清理 — ✅ Complete 2026-09-07

Phases 21 接线 shipped.

---

## v1.1 — 外部授权 + 审计链 + Eval CI + OSS — ✅ Complete 2026-09-06

---

## v1.0 — 业务数据支撑 (Business Data Support) — ✅ Complete 2026-08-30

Phases 14-20 shipped (CGO trust floor, KPI aggregation, dashboard, reports, scheduler, admin).

---

## v0.8 — Harness Milestone (可信×行动融合) — ✅ Complete 2026-08-20

Phases 8-13 shipped (deps upgrade, history injection spike, multi-turn memory, write tools + approval gate, planning layer, sub-agent fan-out).

---

## v1 — Initial Release — ✅ Complete 2026-08-14

Phases 1-7 shipped (spike, compliance slice, agentic retrieval, streaming UI, skill platform, eval dashboard, open-source release).
