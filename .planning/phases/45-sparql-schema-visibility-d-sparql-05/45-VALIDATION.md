---
phase: 45
slug: sparql-schema-visibility-d-sparql-05
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-10-03
---

# Phase 45 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: 45-RESEARCH.md `## Validation Architecture`（已对 HEAD `24710bf` 代码实证）。

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib testing + testify（现有；go1.25.5） |
| **Config file** | none（go.mod module `github.com/skillforge/skillforge`；无需测试框架配置） |
| **Quick run command** | `wsl.exe -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/CODE/ClaudeCode/ai-agent && CGO_ENABLED=1 go test ./internal/ontology/store/ -count=1'` |
| **Full suite command** | `wsl.exe -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/CODE/ClaudeCode/ai-agent && CGO_ENABLED=1 go test ./internal/... -count=1'` |
| **Estimated runtime** | store quick ≤30s；全量 ~数分钟（hermetic，无 live LLM/Milvus 依赖） |

---

## Sampling Rate

- **After every task commit:** store suite quick command（≤30s, CGO_OK）+ touched-package vet
- **After every plan wave:** targeted SPARQL+Hybrid gate（最低 PASS 计数：10 SPARQL + 9 Hybrid）
- **Before `/gsd:verify-work`:** `bash verify-phase-45.sh` 全分区绿
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> Task ID 已对齐 PLAN.md（plan-checker 2026-10-03 验证 6/6 任务含 automated verify）；Req→Test 映射已由 RESEARCH 钉死。

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 45-01 T1 | 01 | 1 | SPARQL-01/02 | T-45-01 | admin 层计数 17/10 + 正向断言（RED） | unit (server) | `CGO_ENABLED=1 go test ./internal/server/ -run "TestSparqlQueryScoped_TierFilter\|TestSparqlQueryScoped_FailClosedNoBelongsToTier" -v -count=1` | ✅ (update admin_platform_test.go) | ⬜ pending |
| 45-01 T2 | 01 | 1 | SPARQL-01/02 | T-45-01/T-45-04 | β 臂 + schemaSubjects 查询时派生 + 8 处 nil ripple（GREEN） | unit (store+server) | `CGO_ENABLED=1 go test ./internal/ontology/store/... ./internal/server/ -run "TestSparqlQueryScoped" -count=1` | ✅ (modify store.go + 8 ripple lines) | ⬜ pending |
| 45-01 T3 | 01 | 1 | SPARQL-01/02 | T-45-01 | skill 个体 hasReadAccess tier-可见 且同 subject label 仍不可见（一测双断言） | unit (store) | `CGO_ENABLED=1 go test ./internal/ontology/store/ -run TestStoreSchemaSubject -v -count=1` | ❌ new (`store_schema_subject_test.go`) | ⬜ pending |
| 45-02 T1 | 02 | 2 | SPARQL-01 (anti-drift) | T-45-02 | emitter 谓词 ⊆ {rdf:type} ∪ metaPredicates | unit (store, conformance) | `CGO_ENABLED=1 go test ./internal/ontology/store/ -run TestEmitterPredicatesWithinAllowlist -v -count=1` | ❌ new (`store_emitter_conformance_test.go`) | ⬜ pending |
| 45-02 T2 | 02 | 2 | SPARQL-03 | T-45-06 | 双端点 schema 判定一致（/api/sparql vs /api/admin） | integration (handler, hermetic) | `CGO_ENABLED=1 go test ./internal/server/ -run TestSchemaJudgmentParity -v -count=1` | ❌ new (扩展 admin_ontology_test.go) | ⬜ pending |
| 45-02 T3 | 02 | 2 | SC4 | T-45-07 | verify-phase-45.sh 全绿 | gate script | `bash verify-phase-45.sh`（WSL CGO chain, 4 fail-fast partitions, P3 ≥19 PASS） | ❌ new | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements（test harness / WSL toolchain / 22-triple fixture 均已存在且经 RESEARCH 实机探测）。三个新测试文件 + verify-phase-45.sh 是 phase 交付物（普通任务），不是 Wave-0 blocker。

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies（6/6，plan-checker 验证）
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references（无 Wave-0 缺口——既有基建全覆盖）
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-03（plan-checker Dimension 8 通过后签署）
