---
phase: 45
slug: sparql-schema-visibility-d-sparql-05
status: draft
nyquist_compliant: false
wave_0_complete: false
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

> Task ID 由 planner 在 PLAN.md 落定后对应；Req→Test 映射已由 RESEARCH 钉死。

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (planner) | 01 | 1 | SPARQL-01 | — | skill 个体 hasReadAccess tier-可见 | unit (store) | `CGO_ENABLED=1 go test ./internal/ontology/store/ -run TestStoreSchemaSubject -v -count=1` | ❌ new (`store_schema_subject_test.go`) | ⬜ pending |
| (planner) | 01 | 1 | SPARQL-01 | — | admin 层计数 17/10 + 正向断言 | unit (server) | `CGO_ENABLED=1 go test ./internal/server/ -run "TestSparqlQueryScoped_TierFilter\|TestSparqlQueryScoped_FailClosedNoBelongsToTier" -v -count=1` | ✅ (update admin_platform_test.go) | ⬜ pending |
| (planner) | 01 | 1 | SPARQL-01 (anti-drift) | — | emitter 谓词 ⊆ allowlist | unit (store, conformance) | `CGO_ENABLED=1 go test ./internal/ontology/store/ -run TestEmitterPredicatesWithinAllowlist -v -count=1` | ❌ new (`store_emitter_conformance_test.go`) | ⬜ pending |
| (planner) | 01 | 1 | SPARQL-02 | — | fail-closed 不回退（负向断言 + Path A 冻结 + 红线三测试） | unit (regression) | `CGO_ENABLED=1 go test ./internal/ontology/store/... -count=1` + `./internal/server/ -run "TestSparqlQueryScoped\|TestHybridSearchScoped" -count=1` | ✅ (existing, must stay green unchanged) | ⬜ pending |
| (planner) | 01 | 1 | SPARQL-03 | — | 双端点 schema 判定一致 | integration (handler, hermetic) | `CGO_ENABLED=1 go test ./internal/server/ -run TestSchemaJudgmentParity -v -count=1` | ❌ new (placement per RESEARCH Open Q3) | ⬜ pending |
| (planner) | 01 | 1 | SC4 | — | verify-phase-45.sh 全绿 | gate script | `bash verify-phase-45.sh`（WSL CGO chain, 4 fail-fast partitions） | ❌ new | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements（test harness / WSL toolchain / 22-triple fixture 均已存在且经 RESEARCH 实机探测）。三个新测试文件 + verify-phase-45.sh 是 phase 交付物（普通任务），不是 Wave-0 blocker。

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
