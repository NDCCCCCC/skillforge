# Milestones

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
