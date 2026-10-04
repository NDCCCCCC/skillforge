---
phase: 46
slug: entity-shacl-shapes
status: draft
nyquist_compliant: false
wave_0_complete: false
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

> Task/Plan/Wave 列在 planning 产出的 PLAN.md 定稿后对号入座；此处按 requirement 粒度锁定验证面。

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | 0 | SHACL-01 | T-SHACL 路径注入 | env 路径 parse 失败即整体 error 不静默 | unit（shacl 包，纯 Go） | `go test ./internal/ontology/shacl/... -run TestShapesPaths -count=1`（未设置/空串/单/多路径四态 + 默认双文件 + errors.Join 不吞缺失路径） | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | SHACL-01 | — | N/A | unit | `go test ./internal/ontology/shacl/... -run TestEntitiesShapesParse -count=1`（2 NodeShapes、type hasValue / label minCount / if-present 无 minCount 逐条断言） | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | SHACL-01 | — | N/A | unit（induction 包） | `go test ./internal/ontology/induction/... -run TestRollbackShaclParity -count=1`（rollback 真读 env，induction.go:383 修复回归） | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | SHACL-02 | — | N/A | unit | `go test ./internal/ontology/shacl/... -run TestSeverity -count=1`（Warning/Info→放行，未声明/Violation→error 隔离） | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | SHACL-02 | D-TIER-03 隔离绕过 | 违规实体 subject 不可见（Query 为空） | unit（fixture KG，hermetic） | `go test ./internal/ontology/shacl/... -run TestEntityQuarantine -count=1`（缺 label Product → Quarantined + Query 空 + 再 Validate 报告稳定） | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | SHACL-02 | — | 携违规 boot 存活（D-SHACL-02） | unit 间接 | P4 全量套件 + 既有 main 链路回归（Discretion 授权 fixture 路线，无 boot 级集成测试） | ✅ 既有 | ⬜ pending |
| TBD | TBD | 0 | SHACL-03 | — | N/A | unit | `go test ./internal/ontology/shacl/... -run TestViolationLayer -count=1`（entity/tbox/global 三层盖戳） | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | SHACL-03 | — | N/A | unit | `go test ./internal/ontology/shacl/... -run TestDanglingEdges -count=1`（involves/parent 入边计数；0 边渲染 0） | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | SHACL-03 | XSS（报告注入） | detail/constraint 文本节点渲染，无 dangerouslySetInnerHTML | FE unit（vitest，可选：planner 权衡组件测试 vs 目检 checklist） | `cd web && npx vitest run src/components/admin/governance`（Windows 侧） | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | 0 | 回归红线 | — | N/A | unit 回归 | P2/P4（既有 13 shacl 测试全绿；TestShaclViolation fixture 补 `sh:severity sh:Warning` 后断言不变） | ✅ 既有（1 处 fixture 单行补丁） | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/ontology/shacl/` 新增 severity / layer / 列表化 / entities.ttl / dangling 测试组（或独立 `entities_test.go`，照包内单文件惯例）
- [ ] `internal/ontology/induction` rollback SHACL parity 回归测试（新建）
- [ ] TestShaclViolation fixture 单行补丁（`sh:severity sh:Warning`）——断言不动
- [ ] FE 组件测试（可选）——若加，照 governanceApi.test.ts 的 vitest 惯例
- [ ] 框架安装：无需（Go testing + vitest 既有）

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `/admin/governance` 报告 UI 视觉/交互符合 46-UI-SPEC | SHACL-03 | UI 视觉契约不适用组件断言（若 planner 未选 FE 组件测试路线） | 照 46-UI-SPEC.md 逐条目检：layer 徽章区分 entity/tbox/global、dangling 边数 chip、0 值如实渲染 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 1s（quick run）
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
