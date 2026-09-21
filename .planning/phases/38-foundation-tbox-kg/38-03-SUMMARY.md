---
plan: 38-03
title: internal/ontology/induction — LLM schema induction + PascalCase IRI + proposals 持久化
status: complete
commit: 29ad95b..568b819 后续 (git log 查最新)
requirements: [ONT-04]
---

# Plan 38-03 Summary — LLM Schema Induction

## What Was Built

- `internal/ontology/induction/proposal.go` — Proposal struct + Validate
- `internal/ontology/induction/induction.go` — Manager:
  - `Generate(ctx, text)` — 固定 JSON prompt (kind + description_zh/en) + temperature 0 + ExtractJSON 防御解析; 后端确定性 ToPascalCase (acronym 保留 SQL/ETL/KPI/PII; 冲突追加 N)
  - `Approve(id, customName)` — TBox v{N+1}.ttl 归档 + pointer 文件 copy-fallback 刷新 + tbox_version.txt 自增 + KG Insert; customName 经 PreviewIRI 冲突检查
  - `Reject(id)` — 保留记录; 非 pending 状态 fail-closed
  - `PreviewIRI(name)` — 当前 TBox 文本 + 队列内 local name 双重冲突检查 → ErrIRIConflict
  - `proposals.json` 持久化 (queue + nextID), 重启恢复
  - Generator 接口窄化为 Eino `Generate(ctx, msgs, opts...)`, stub-friendly

## Tests

```
8/8 PASS — TestToPascalCase (6 用例表) / TestGeneratePascalCaseAndBilingualLabel /
TestGenerateSecondSameDescriptionGetsSuffix / TestApproveIncrementsTBoxVersion (v1→v2,
pointer==v2 copy 契约, KG 可查) / TestApproveCustomLocalName / TestRejectKeepsRecord /
TestPreviewIRIConflictDetection / TestQueuePersistsAcrossRestart
```

## Deviations from Plan

- **proposals 持久化选 JSON 文件而非 SQLite** (CONTEXT.md D-Route-01 标注 "planner 选"): 单进程 + 低频 + 原子写 helper 已有, 不碰现有 store.Store 核心管道, 解耦更干净
- **软链 fallback 直接实现为 copy** (D-TBox-01 的 Windows 无 symlink 权限场景在实现时即内置, 非 Phase 40 才补)
- ToPascalCase 增强: acronym 保留 (SQLNote 而非 SqlNote)

## Self-Check: PASSED

- [x] proposal.go + induction.go + induction_test.go 落盘
- [x] `go test ./internal/ontology/induction/` 全绿 + `go vet` 通过
- [x] ONT-04 完整覆盖 (LLM 提议 + 人工审批 + 版本自增 + 持久化)
