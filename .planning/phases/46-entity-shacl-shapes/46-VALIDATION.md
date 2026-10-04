---
phase: 46
slug: entity-shacl-shapes
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-10-04
---

# Phase 46 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: 46-RESEARCH.md `## Validation Architecture`（2026-10-04，HIGH confidence）

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing（stdlib，无 testify）go1.25.5；FE vitest（web/） |
| **Config file** | 无新增（shacl 包既有；web/vitest.config.ts 既有） |
| **Quick run command** | `wsl.exe -d Ubuntu-22.04 -- bash -c "cd /mnt/d/CODE/ClaudeCode/ai-agent && CGO_ENABLED=0 /usr/local/go/bin/go test ./internal/ontology/shacl/... -count=1"`（<1s） |
| **Full suite command** | `bash verify-phase-46.sh`（照 45 模式：P1 CGO=1 build+vet → P2 ontology 套件 → P3 targeted anti-vacuous `-run` + 最低 `--- PASS` 计数 → P4 `./internal/...` 全量） |
| **Estimated runtime** | quick ~1s；full suite ~数分钟 |

---

## Sampling Rate

- **After every task commit:** Run shacl 包 quick run（CGO_ENABLED=0，<1s）
- **After every plan wave:** Run `CGO_ENABLED=1 go test ./internal/ontology/... ./internal/server/... -count=1`
- **Before `/gsd:verify-work`:** `bash verify-phase-46.sh` 全绿（P1-P4）
- **Max feedback latency:** ~1 second（quick run）

---

## Per-Task Verification Map

> 已按 plan-phase 定稿的 12 个 task 回填（2026-10-04，与 plan-checker Dimension 8 核验表一致）。

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 46-01-T1 severity/sourcePath retrofit | 01 | 1 | SHACL-01/02 + 回归红线 | T-SHACL-01 | sourcePath 特例退役，severity 单点政策 | unit | `CGO_ENABLED=0 go test ./internal/ontology/shacl/... -count=1` | ❌ Wave 0（fixture 单行补丁随 task） | ⬜ pending |
| 46-01-T2 variadic + ShapesPathsFromEnv | 01 | 1 | SHACL-01 | T-SHACL 路径注入 | parse 失败即整体 error 不静默 | unit + build | `CGO_ENABLED=1 go build ./... && CGO_ENABLED=0 go test ./internal/ontology/shacl/... ./internal/ontology/induction/... -count=1` | ❌ Wave 0 | ⬜ pending |
| 46-01-T3 TestShapesPaths/TestSeverity (tdd) | 01 | 1 | SHACL-01, SHACL-02 | — | N/A | unit | `go test ./internal/ontology/shacl/... -run 'TestShapesPaths\|TestSeverity' -v -count=1` | ❌ Wave 0 | ⬜ pending |
| 46-02-T1 盖戳 + dangling_edge_count | 02 | 2 | SHACL-02, SHACL-03 | D-TIER-03 隔离绕过 | layer/target_class/global 哨兵；入边计数披露 | unit | `CGO_ENABLED=1 go build ./... && CGO_ENABLED=0 go test ./internal/ontology/shacl/... -count=1` | ❌ Wave 0 | ⬜ pending |
| 46-02-T2 entities.ttl 交付 | 02 | 2 | SHACL-01 | T-46-01 防顺手加强 | entityCode 零约束（grep 验收 == 0） | unit | `go test ./internal/ontology/shacl/... -run TestEntitiesShapesParse -v -count=1` | ❌ Wave 0 | ⬜ pending |
| 46-02-T3 四测试组 (tdd) | 02 | 2 | SHACL-02, SHACL-03 | D-TIER-03 | 违规实体 subject 不可见（Query 空） | unit（fixture KG，hermetic） | targeted `-run`（TestEntityQuarantine 等四名）`-v -count=1` | ❌ Wave 0 | ⬜ pending |
| 46-03-T1 main.go 切 env 列表 | 03 | 2 | SHACL-01, SHACL-02 | — | 携违规 boot 存活（D-SHACL-02）；快照 :921 先于 :931 时序零改动 | build + vet | `CGO_ENABLED=1 go build ./... && CGO_ENABLED=1 go vet ./...` | ✅ 既有链路 | ⬜ pending |
| 46-03-T2 rollback parity (tdd) | 03 | 2 | SHACL-01 | — | rollback 真读 env（induction.go:383 修复回归） | unit | `go test ./internal/ontology/induction/... -run TestRollbackShaclParity -v -count=1` | ❌ Wave 0 | ⬜ pending |
| 46-04-T1 types.ts 镜像 | 04 | 3 | SHACL-03 | XSS | optional 字段契约镜像 | tsc | `cd web && npx tsc --noEmit` | ❌ Wave 0 | ⬜ pending |
| 46-04-T2 ShaclReportView 徽章/区头/悬挂边 | 04 | 3 | SHACL-03 | XSS（报告注入） | detail/constraint 文本节点渲染，无 dangerouslySetInnerHTML | tsc + vitest | `cd web && npx tsc --noEmit && npx vitest run src/components/admin/governance` | ❌ Wave 0 | ⬜ pending |
| 46-04-T3 GovernanceTab + 组件测试 (tdd) | 04 | 3 | SHACL-03 | — | N/A | FE unit（vitest） | `cd web && npx vitest run src/components/admin/governance` | ❌ Wave 0 | ⬜ pending |
| 46-05-T1 verify-phase-46.sh | 05 | 3 | SHACL-01/02/03 | — | phase gate（P3 anti-vacuous pass_lines≥7） | script | `bash verify-phase-46.sh` | ❌ Wave 0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/ontology/shacl/` 新增 severity / layer / 列表化 / entities.ttl / dangling 测试组（或独立 `entities_test.go`，照包内单文件惯例）→ 46-01-T3 + 46-02-T3 认领
- [x] `internal/ontology/induction` rollback SHACL parity 回归测试（新建）→ 46-03-T2 认领
- [x] TestShaclViolation fixture 单行补丁（`sh:severity sh:Warning`）——断言不动 → 46-01-T1 认领
- [x] FE 组件测试（可选）——若加，照 governanceApi.test.ts 的 vitest 惯例 → 46-04-T3 认领（组件测试路线）
- [x] 框架安装：无需（Go testing + vitest 既有）

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `/admin/governance` 报告 UI 视觉/交互符合 46-UI-SPEC | SHACL-03 | UI 视觉契约不适用组件断言（若 planner 未选 FE 组件测试路线） | 照 46-UI-SPEC.md 逐条目检：layer 徽章区分 entity/tbox/global、dangling 边数 chip、0 值如实渲染 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references（Wave 0 gaps 已全部被 46-01..46-05 的 TDD task 认领，无外置缺口）
- [x] No watch-mode flags
- [x] Feedback latency < 1s（quick run）
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-04（plan-checker 12/12 task 核验 + 0 blocker）
