---
slug: hitl-approval-snapshot-500
status: resolved
created: 2026-09-14
updated: 2026-09-14
resolved: 2026-09-14
trigger: "TestDataDecisionExtensionNoNewChunkTypes pre-existing 失败（backend-unavailable-test-fail 会话归档时移交），用户要求处理"
---

# Debug Session: hitl-approval-snapshot-500

## Symptoms

- **Expected**: `go test ./internal/server/ -run TestDataDecisionExtensionNoNewChunkTypes` 应通过（approve 端点 200）。
- **Actual**: `approval_api_test.go:468` 断言 approve 返回 200，实际 500，body `{"error":"audit write failed"}`。
- **Errors**: 服务端日志 `[ERROR] server: approval MarkApproved (sess=sess-datadec-01 id=tc-000001) failed: hitl: MarkApproved: skill snapshot required (D11-C2)` —— 500 的对外错误文案（audit write failed）与真实失败原因（skill snapshot required）**错位**。
- **Timeline**: pre-existing（2026-09-14 Bug A 归档时确认 pristine HEAD 同样失败，与近期提交无关）。
- **Reproduction**: WSL `go test ./internal/server/ -run TestDataDecisionExtensionNoNewChunkTypes -v` 稳定复现（orchestrator 2026-09-14 实跑确认）。测试上下文：approval expire-check 日志显示 checkpoint fresh state，随后 MarkApproved 报 skill snapshot required。

## Evidence

- timestamp: 2026-09-14 (orchestrator, WSL go test -v)
  - 失败序列：expire-check fresh → `MarkApproved ... failed: hitl: MarkApproved: skill snapshot required (D11-C2)` → HTTP 500 `{"error":"audit write failed"}`。
  - 500 文案映射错位：snapshot-missing 被归到了 audit-write 失败的错误分支（初步判断，待代码确认是 handler 错误分类问题还是 sentinel error 包装问题）。
- timestamp: 2026-09-14 (移交记录，来自 resolved/backend-unavailable-test-fail.md)
  - 初步定位：测试 fixture 缺 D11-C2 要求的 skill snapshot；且快照缺失错误可能被误标为 audit write failed。
- timestamp: 2026-09-14 (代码阅读 — internal/server/approval_api.go)
  - `executeApprovalResume` L472：`snap := activeSkillSnapshot(manifestForSkill(s.Loader, row.SkillID))` —— **无 fallback**。`manifestForSkill(nil, …)` 返回 nil → `activeSkillSnapshot(nil)` 返回零值 snapshot → `AuditWriter.MarkApproved` 的 `snap.IsZero()` 守卫（audit_writer.go:376-378）报 `skill snapshot required (D11-C2)` → handler 归入 500 "audit write failed"。
  - 同一请求的 expire-check（L438-442）**有** fallback（`rowManifest == nil → s.ActiveManifest`），所以先判 "fresh state, proceeding"，随后 Mark* 才炸 —— 同一 handler 内同一 snapshot 两种推导，自相矛盾。
  - 同函数 L502-505 的 `resumeManifest`（供 BuildRunner）也有同样的 fallback —— 唯独 L472 的 audit snap 漏了。
  - 测试 fixture（approval_api_test.go:451-458）`Deps` 不设 `Loader` —— 这是全 repo 测试的既定模式，plan_api.go:365-371 注释明说 "tests construct Deps without a Loader; production wires both"。
  - 兄弟 handler plan_api.go 的 plan-approve 路径：L373-377 用 fallback 链推导 `snap` 并直接用于 Mark*（L428-450）→ plan 侧同构测试绿，approval 侧红。
  - handler 入口闸门 L319 保证 `ActiveManifest != nil` 才能走到这里 → 补 fallback 后零值 snapshot 在该路径不可达。
- timestamp: 2026-09-14 (git 考古)
  - 回归引入 commit：**27db382**（2026-09-09，"refactor(33-04): split approvalPostHandler into decode/validate/execute/drain helpers"）。
  - 拆分前（27db382~1）：Mark* 复用 expire-check 块里的 `snap`，其推导带 fallback 链，且有注释明说「Tests construct Deps without a Loader (rowManifest nil) but WITH ActiveManifest set; production wires both」。拆分时 expire-check 的 `snap` 留在 validateAndResolveApproval，executeApprovalResume 里新写了一行 **丢了 fallback** 的推导。
  - 时间线吻合：27db382 是 2026-09-09，失败被记录为 pre-existing（9-12、9-14 的归档会话均见）。
- timestamp: 2026-09-14 (字面量影响面)
  - 无任何 `_test.go` 断言 "audit write failed" 字面量（仅 approval_api.go / plan_api.go 生产代码 + scripts/verify-phase-36.sh grep `[WARN].*audit write failed`，且 scope 是 internal/agent + internal/runtime，不受影响）→ 修复无需动对外文案。

## Current Focus

hypothesis: 已确认（实现侧回归，非 fixture 违约）—— 27db382（refactor 33-04）把 approvalPostHandler 拆成 4 个 helper 时，executeApprovalResume 为 audit 写入重新推导 snapshot：`snap := activeSkillSnapshot(manifestForSkill(s.Loader, row.SkillID))`，丢掉了拆分前 expire-check 块里 `rowManifest == nil → s.ActiveManifest` 的 fallback。测试 fixture 不设 Loader（repo 既定模式）→ manifestForSkill 返回 nil → 零值 snapshot → MarkApproved 的 D11-C2 守卫拒绝 → 500 且文案误标 "audit write failed"（错误映射错位是这行遗漏的下游症状：补 fallback 后该路径零值 snapshot 不可达，"audit write failed" 重新只表示真实 audit store 失败）。
test: 最小修复 —— executeApprovalResume 顶部把 resumeManifest 的 fallback 推导上移到 audit 写入之前，snap 从 fallback 后的 manifest 推导，删除 L502-505 的重复推导；然后 WSL 跑 targeted test → 包 build/vet → 全包对比失败集合。
expecting: targeted test GREEN（approve 200 + "memory_facts_write OK" + data-decision tool-resumed + action_approved 行 + already_resolved 幂等）；全包失败集合从 {TestDataDecisionExtensionNoNewChunkTypes, TestFeedbackDispositionAdminGate} 缩到 {TestFeedbackDispositionAdminGate}（另一独立 bug，非本会话范围）。
next_action: DONE — 已归档 2026-09-14（fix commit 93570d7；会话移 resolved/ + knowledge-base.md 条目）
reasoning_checkpoint:
  hypothesis: "27db382 重构在 executeApprovalResume 中为 audit 写入重新推导 snapshot 时丢失了 manifestForSkill==nil → ActiveManifest 的 fallback；Loader 为 nil 的测试 Deps（以及生产中 skill 未注册/被热卸载的场景）得到零值 snapshot，MarkApproved 的 D11-C2 snap.IsZero() 守卫拒绝写入，handler 把它归入 500 'audit write failed'"
  confirming_evidence:
    - "approval_api.go L472 无 fallback；同函数 L502-505（resumeManifest）与 validateAndResolveApproval L438-442（expire-check）均有 fallback —— 同一请求三种推导，唯独 audit snap 漏了"
    - "git 考古：27db382 拆分前 Mark* 复用带 fallback 的 expire-check snap；diff 显示拆分新增了无 fallback 的 `+ snap := activeSkillSnapshot(manifestForSkill(s.Loader, row.SkillID))`（commit 2026-09-09，与 pre-existing 失败时间线吻合）"
    - "测试 fixture 不设 Loader 是 repo 既定模式（plan_api.go:365-371 注释 + server_test.go/pipeline_default_test.go 等）；plan 侧同构路径（带 fallback）测试绿"
    - "失败序列自洽：expire-check 用 fallback 判 fresh → executeApprovalResume 用零值 snapshot 写 audit → 恰好是日志里 'fresh state, proceeding' 后紧跟 'skill snapshot required' 的顺序"
  falsification_test: "若给 executeApprovalResume 补上 fallback 后 targeted test 仍 500/snapshot required，则假设错误（嫌疑转向 fixture 或 AuditWriter 层）—— 实测预期 GREEN"
  fix_rationale: "修复的是根因（重构丢失的 fallback），不是症状：恢复拆分前已存在且被 plan_api.go 镜像的推导链，使 expire-check 的 'fresh' 判定与 audit 行的 snapshot 标注出自同一 manifest（D11-C2 语义一致性）。副作用消除错误映射错位 —— 入口闸门保证 ActiveManifest 非 nil，补 fallback 后零值 snapshot 在该路径不可达，'audit write failed' 恢复为仅表示真实 store 失败，无需改任何对外文案（无测试断言该字面量）"
  blind_spots: "① 未验证生产 Loader 存在但 row.SkillID 未注册时的行为差异 —— 补 fallback 后会沿用 ActiveManifest 标注 audit 行，与 expire-check 判定一致（可接受，且这正是 plan_api.go 的既有行为）；② 假设 approve 后续 resume 路径（BuildRunner/ResumeWithParams/drain）在该 fixture 下原本就工作 —— 若 fix 后 targeted test 在后续断言失败，需二次调查（独立于本根因）"

## Eliminated

- hypothesis: (a) fixture 层违约 —— 测试应播种 D11-C2 skill snapshot / 应设置 Deps.Loader
  evidence: 测试确实播种了 snapshot（seedPendingApproval + stampCheckpointSnapshot，approval_api_test.go:447-449），且 expire-check 正是据此判定 fresh 通过的；「Deps 不设 Loader」是 repo 全部 server 测试的既定构造模式（plan_api.go:365-371 注释明文背书 "tests construct Deps without a Loader; production wires both"），plan 侧同构测试因此是绿的 —— 要求 fixture 设 Loader 会让 approval 测试偏离全 repo 模式，且掩盖生产同路径的真实缺陷（skill 未注册 → 500 误标 audit write failed）。
  timestamp: 2026-09-14
- hypothesis: (b) 独立的 handler 错误分类 if/else 顺序错或 sentinel error 包装问题（需单独改错误映射分支）
  evidence: 错误映射分支本身工作正常（任何 Mark* error → 500 + 日志含真实 error，运维可从日志定位）；文案错位是上游 snapshot 推导遗漏的下游症状。补 fallback 后该路径零值 snapshot 不可达（入口闸门 L319 保证 ActiveManifest 非 nil），无需新增错误分支 —— 单独改映射反而是给不可达路径加死代码。
  timestamp: 2026-09-14

## Resolution

root_cause: 重构回归（27db382，refactor 33-04，2026-09-09）。approvalPostHandler 拆分为 decode/validate/execute/drain 四个 helper 时，executeApprovalResume 为 audit 决议行写入重新推导 D11-C2 snapshot：`snap := activeSkillSnapshot(manifestForSkill(s.Loader, row.SkillID))`，丢失了拆分前 expire-check 块中 `rowManifest == nil → s.ActiveManifest` 的 fallback（该 fallback 当时有注释明确覆盖「tests construct Deps without a Loader」场景）。测试 Deps 无 Loader → manifestForSkill 返回 nil → 零值 snapshot → AuditWriter.MarkApproved 的 D11-C2 守卫（audit_writer.go:376-378）报 "skill snapshot required" → 500。文案错位（"audit write failed"）是该遗漏的下游症状：同一请求 expire-check 用 fallback 判 fresh、audit 写入用零值 snapshot 被拒，两个推导不一致。
fix: internal/server/approval_api.go executeApprovalResume —— 把 resumeManifest 的 fallback 推导（per-row manifest → ActiveManifest）上移到 audit 写入之前，snap 从 fallback 后的 manifest 推导（与 expire-check / plan_api.go / 拆分前行为一致），删除原 L502-505 的重复推导。不改对外错误文案（无测试断言字面量；补 fallback 后零值 snapshot 路径不可达，"audit write failed" 重新只表示真实 audit store 失败）。
verification: WSL 实证链（2026-09-14，gsd-debugger 自验）—— ① targeted test：`go test ./internal/server/ -run TestDataDecisionExtensionNoNewChunkTypes -v` PASS（修复前同命令稳定 500 失败，orchestrator 实跑；falsification test 通过）；② `go build` + `go vet` ./internal/server/ 干净；③ 全包失败集合 {TestDataDecisionExtensionNoNewChunkTypes, TestFeedbackDispositionAdminGate} → {TestFeedbackDispositionAdminGate}，与预测 delta 完全一致（后者为另一独立 bug，非本会话范围）；④ 相邻回归扫描：Approval|DataDecision|ExpiredCheckpoint|CancelPropagation|Decision 全 PASS（8 个），./internal/agent/hitl/ 包全绿；⑤ 稳定性：targeted test 连续 5 次 -count=1 全 PASS（0.060-0.063s，含 self-heal retry 路径）；⑥ gsd-debug-session-manager 独立复核 targeted test 亦 PASS。修复 commit **93570d7**（fix(server): restore ActiveManifest fallback for D11-C2 audit snapshot in approval resume，1 file +14/-5）。orchestrator 独立复核通过：`go test -run "TestDataDecisionExtensionNoNewChunkTypes|TestFeedbackDispositionAdminGate" -v` 两者全 PASS（TestFeedbackDispositionAdminGate 已被并行会话修复）+ diff 复核与 Resolution 一致。
files_changed:
  - internal/server/approval_api.go

## Notes

- WSL 跑测试（runbook）；:8888 在跑（pid 5120），单测不需要它，勿杀。
- 相关背景：HITL plan_approve 流曾有 runner_sse.go:101 的 data-decision chunk 丢失修复（记忆 fix-runner-sse-interrupt-chunk），同域但不同层。
- 全包已知另一个 pre-existing 失败：TestFeedbackDispositionAdminGate（source-order 断言，feedback.go）—— 非本会话范围，勿顺手改（归档时确认已被并行会话 feedback-admin-gate-order 修复）。
- internal/server/pipeline.go + upload_test.go 已被其他会话修复并提交，勿动。
- **ARCHIVED 2026-09-14**：fix commit 93570d7（只 stage internal/server/approval_api.go，+14/-5；feedback_test.go 属并行会话未动）；会话文件移 resolved/；knowledge-base.md 已加条目。
