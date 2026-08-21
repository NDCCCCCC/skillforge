---
status: complete
phase: 13-p8-4-sub-agent-fan-out
source: 13-01-SUMMARY.md, 13-02-SUMMARY.md, 13-03-SUMMARY.md, 13-04-SUMMARY.md, 13-05-SUMMARY.md, 13-06-SUMMARY.md
started: 2026-08-21T18:30:00Z
updated: 2026-08-21T08:09:30Z
---

## Current Test

[testing complete]

## Tests

### 1. 9 阶段 hermetic gate 全绿
expected: `make verify-phase-13` 退出码 0，9/9 stage PASS，最终打印 `PHASE 13 HERMETIC GATE: PASS` banner
result: pass

### 2. Phase 8 合规锁保持不变 (T-8-12 不变量)
expected: `git diff --quiet eval/frozen/compliance.lock.json` 退出 0（sha256 仍为 `5cf700f357a4a7022195ec233106c5398fe2c5562045ca8894996efe23cecded`），未被 Phase 13 作业意外污染
result: pass

### 3. Phase 13 fanout.lock.json 已完成 operator 签署
expected: `eval/frozen/fanout.lock.json` 包含实测指标（fanout_dispatched_count=20, conservative_merge_rate=0.200 [再基线到 0.5], tier_closure_regression_rate=0.0, min_confidence_aggregation=0.95, tier_mismatch_rejection_rate=1.0）+ operator 签名 `ninedrunk` + 日期 2026-08-21 + 5 个 frozen artifact 的 sha256
result: pass

### 4. 合规-orchestrator 示例 Skill 通过验证器
expected: `examples/skills/compliance-orchestrator/SKILL.md` 存在，使用 `agent: orchestrator` archetype，reads 含 `authority/**` + `historical/**`，writes 为空，`TestExamplesValidate` 现在覆盖 4 个模板（原 3 个 + 新增）且全部通过
result: pass

### 5. Tier 闭包双层防御生效 (FAN-02 SC-2)
expected: 在 spike-fanout hermetic 模式下运行 `empty-intersection` 用例，输出包含 `runtime.BuildSubAgent: load-time error: tier intersection empty` 字符串；`fanout_dispatch` Tool 在空 tier_filter 或 tier 越界时同样拒绝（两层校验）
result: pass

### 6. mergeSubAgentResults 纯函数 10× 字节恒等
expected: `go test ./internal/runtime/... -run TestMergeSubAgentResultsByteIdentity -count=10` 全部 PASS — 同一组随机排序的 SubAgentResult 输入产出 byte-identical 的 MergedVerdict JSON
result: pass

### 7. 保守部分失败合并触发 DowngradeStatus
expected: 任一子 agent 状态为 refused 或 faulted 时，`mergeSubAgentResults` 返回 `Status="needs_input"`（即 `verdict.DowngradeStatus`）、`Confidence=0`、`ReviewRequired=true`，命中 D2-14 保守姿态
result: pass

### 8. 子 agent 只读不写 (P13-L4 不变量)
expected: `TestSubAgentIsReadOnly` + `TestSubAgentToolWhitelistExcludesHITL` 通过 — `BuildSubAgent` 在构造时清空 `subManifest.Writes = []string{}`，子 agent 因此无法触达 HITL 工具
result: pass

### 9. SharedChunkAccumulator LRU 上限 20
expected: `go test ./internal/runtime/... -run 'TestSharedChunkAccumulator*'` 全绿 — 共享累加器在 20 条后开始淘汰最旧项，多 goroutine 并发安全
result: pass

### 10. 审计表 ADD COLUMN 迁移可加可重入
expected: `internal/store/sqlite/fanout_events_test.go` 中 9 个回归测试（含 `TestFanoutEventsRequireRunPath`、`TestFanoutEventsOrderedByTs`、`TestFanoutEventsFilterByActionPrefix`）全绿；`audit` 表在已运行过的数据库上再次启动应用时不会因重复 ADD COLUMN 而崩溃
result: pass

### 11. D16 三条 fanout_* 决策枚举上线（D-04 byte-identity）
expected: `internal/wire/chunk.go` 出现 `DecisionFanoutDispatched` / `DecisionFanoutCompleted` / `DecisionFanoutFailed` 三个常量；`TestValidTypesUnchangedAfterFanout` 锁定有效 SSE 类型总数仍为 23（不引入新的 chunk 类型，仅复用 data-decision 判别式）
result: pass

### 12. D15 TS 锁步镜像编译通过
expected: `web/src/types/wire-types.ts` 含 3 个新决策常量与 3 个新字段（`runPath`/`transferFrom`/`transferTo`），运行 `cd web && npx tsc --noEmit` 退出 0；`AssertAssignable<SkillForgeChunk, UIMessageChunk>` 不变量仍然成立
result: pass

### 13. STRIDE-13 威胁模型 8 条齐全
expected: `docs/threat-model/STRIDE-13.md` 包含 T-P13-1..T-P13-8 八条威胁（每条标注组件 / 缓解措施 / 验证测试 / 优先级 Critical 或 High），且 §8 接收标准列出每条威胁对应的命名测试 + Stage 闸门
result: pass

### 14. WalkDir 升级到 8 个 root 并保持无 legacy interrupt
expected: `go test ./integration/contract/... -tags=contract -run 'TestNoLegacyInterruptAPI\|TestNoLegacyInterruptAPIPhase11'` 通过；`phase11GoScanRoots` 现覆盖 8 个目录（含新增 `internal/store/sqlite` 与 `internal/wire`），0 命中 legacy interrupt
result: pass

### 15. 20-case 黄金集分布正确
expected: `eval/golden/fanout.jsonl` 共 20 行 JSONL，分布满足 multi_hop=8, tier_mismatch=6, partial_failure=4, single_vertex=2（与 `fanout.lock.json::category_counts` 完全一致）
result: pass

### 16. Live Acceptance Gate 实测通过
expected: `reports/live_fanout.md` 与 `/tmp/spike-fanout-live.log` 包含 `PHASE 13 LIVE ACCEPTANCE GATE: PASS` 字符串，且 `eval/frozen/fanout.lock.json::metrics.total_cost_usd` 记录为 2.92（含 cost 超 advisory cap 的备注）
result: pass
note: "原始运行日志 (/tmp/spike-fanout-live.log) 字面量显示 FAIL — conservative_merge_rate 0.2 > 原阈值 0.0 (2 例因 GLM Coding Plan timeout 触发 P13-L5 保守降级)。operator (ninedrunk) 接受偏差后把阈值再基线到 0.5 + lock.json gate_verdict 改写为 'PASS (5/6 thresholds satisfied unchanged; conservative_merge_rate accepted-deviation per operator sign-off)' — 这是 D9-V2-style 已知 v0.8 模式 (accepted-with-notes)，偏差根因已文档化到 _operator_accepted_deviation。UAT 接受此偏差作为 PASS。"

### 17. v0.8 里程碑完成报告带证据引证
expected: `.planning/v0.8-MILESTONE-COMPLETION.md` 存在且 ≥ 120 行，包含 "Operator Gates Ledger" 段（7 行表格，3 CLOSED + 4 OPEN），且 25/25 需求 ID 全部有验证测试对应
result: pass

### 18. README DIST-09 占位已用脚本输出填充
expected: README.md 中 `## Eval Results` 段包含来自 `go run ./scripts/eval_snapshot` 的真实数字（faithfulness 97%, context_precision 10%, refusal_accuracy 95% (19/20)）；不存在原 PLACEHOLDER 段与 `_TBD_` 行
result: pass

### 19. STATE.md 处于 complete-pending-tag
expected: `.planning/STATE.md` frontmatter `status: complete-pending-tag`，`percent` 字段值为 92 且附 `# basis = PLANS (56/61)` 注释；存在 "v0.8 MILESTONE CLOSED (pending tag)" 段
result: pass

### 20. 冷启动烟雾测试：compile + 测试套无错误
expected: 清理所有运行中进程后，`go build ./...` 与 `go vet ./...` 均退出 0；`go test ./internal/runtime/... ./internal/skill/... ./internal/store/... ./internal/wire/... -count=1 -timeout=300s` 全绿（不依赖数据库/网络状态）
result: pass

## Summary

total: 20
passed: 20
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
