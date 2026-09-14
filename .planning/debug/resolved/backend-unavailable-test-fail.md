---
slug: backend-unavailable-test-fail
status: resolved
created: 2026-09-14
updated: 2026-09-14
resolved: 2026-09-14
trigger: "TestIsBackendUnavailable_NetErrorTimeout 在 pristine HEAD 上就失败（上一 debug 会话归档时记录的 pre-existing 失败），用户要求处理"
---

# Debug Session: backend-unavailable-test-fail

## Symptoms

- **Expected**: `go test ./internal/server/ -run TestIsBackendUnavailable_NetErrorTimeout` 应通过（测试套件绿是 CI 前置）。
- **Actual**: `upload_test.go:778` 断言 `assert.True(t, isBackendUnavailable(err))` 失败（Should be true）。
- **Errors**: `Error: Should be true`（无 panic，纯断言失败）。
- **Timeline**: pre-existing —— 2026-09-12 归档会话 new-session-shows-old-messages 时确认在 pristine HEAD 同样失败，与当时改动无关。
- **Reproduction**: WSL `go test ./internal/server/ -run TestIsBackendUnavailable_NetErrorTimeout -v` 稳定复现（orchestrator 2026-09-14 实跑确认）。
- **主修复验收（2026-09-14 human-verify PASSED，confirmed fixed）**: orchestrator WSL 实跑 `go test ./internal/server/ -run TestIsBackendUnavailable -v` → 7/7 全 PASS（含 NetErrorTimeout）；session-manager 归档前独立复核同样 7/7 PASS。测试侧根因与 `os.ErrDeadlineExceeded` 修法均获认可。

## Evidence

- timestamp: 2026-09-14 (orchestrator, WSL go test -v)
  - 失败输出：`upload_test.go:778: Error: Should be true`；测试体：
    ```go
    func TestIsBackendUnavailable_NetErrorTimeout(t *testing.T) {
        // Simulate a generic net.Error with Timeout() == true
        err := &net.OpError{Op: "read", Err: fmt.Errorf("i/o timeout")}
        assert.True(t, isBackendUnavailable(err))
    }
    ```
- timestamp: 2026-09-14 (code reading)
  - 测试注释声称「Timeout() == true」，但 `net.OpError.Timeout()` 会委托给内层 Err 的 `Timeout()` 方法（若实现了 `interface{ Timeout() bool }`）；`fmt.Errorf("i/o timeout")` 是普通 error，**没有** `Timeout()` 方法 → `OpError.Timeout()` 返回 false。测试构造与注释意图不符。
  - 相邻测试（758-773 行）用 `net.DNSError`、`context.DeadlineExceeded`、`context.Canceled` 都通过——`isBackendUnavailable` 对「真实可判定为超时/不可用」的错误分类是工作的。
- timestamp: 2026-09-14 (git 考古)
  - `git log -S isBackendUnavailable -- internal/server/upload.go` → 两个 commit：1ad524a（原始 strings.Contains 实现，4 个子串，不含 "i/o timeout"）、baf1ef0（Phase 32 REALBUG-03 改 typed sentinel + 同 commit 引入全部 7 个测试）。
  - 结论：实现从未对 `&net.OpError{Op:"read", Err: fmt.Errorf("i/o timeout")}` 返回 true；该测试出生即失败 → (b) 被推翻。
- timestamp: 2026-09-14 (WSL go run 最小 repro)
  - A) `&net.OpError{Op:"read", Err: fmt.Errorf("i/o timeout")}` → `Timeout()=false`（且 errors.As net.Error=true：会进 branch 4 但 Timeout 为 false）
  - B) `&net.OpError{Op:"read", Err: os.ErrDeadlineExceeded}` → `Timeout()=true`
  - C) plain `fmt.Errorf` 不实现 net.Error
  - D) `os.ErrDeadlineExceeded` ≠ syscall.ETIMEDOUT 且实现 `Timeout() bool` → 修复后测试只命中 upload.go:471-475 的 net.Error.Timeout() 分支（该分支此前零覆盖）
- timestamp: 2026-09-14 (fix 后自验，gsd-debugger，WSL)
  - stash 后 pristine HEAD 复现原失败 → stash pop → targeted test PASS（stash 往返前后各验一次）
  - `TestIsBackendUnavailable*` 全组 7 个 PASS；`go build` + `go vet` ./internal/server/ 干净
  - 全包失败集合修复前后完全一致（delta 检查零回归）：`TestDataDecisionExtensionNoNewChunkTypes` + `TestFeedbackDispositionAdminGate`，均为 pristine HEAD 上的 pre-existing 失败（见 Notes 移交事项）
- timestamp: 2026-09-14 (human-verify，orchestrator + session-manager 双重复核)
  - orchestrator WSL 实跑 `-run TestIsBackendUnavailable -v` → 7/7 PASS；session-manager 归档前独立实跑同样 7/7 PASS（含 NetErrorTimeout 0.00s）
  - `git diff internal/server/upload_test.go` 复核：仅 1 处构造改动 + 注释扩写，实现零改动

## Resolution

root_cause: 测试侧构造违约（假设 (a)）。`net.OpError.Timeout()` 委托给内层 Err 的 `Timeout()` 方法；`fmt.Errorf("i/o timeout")` 是普通 error（无 Timeout 方法），所以 `&net.OpError{Op:"read", Err: fmt.Errorf("i/o timeout")}` 的 `Timeout()` 实测返回 false（尽管 errors.As 能匹配 net.Error），`isBackendUnavailable` 的 `netErr.Timeout()` 分支（upload.go:471-475）判 false。测试注释声称「Timeout() == true」但构造从未交付该语义 —— baf1ef0（Phase 32）同时引入 typed 实现与 7 个测试，该测试出生即失败（原实现 1ad524a 的 strings.Contains 也不匹配 "i/o timeout"，实现侧从未支持过这个构造）。
fix: internal/server/upload_test.go TestIsBackendUnavailable_NetErrorTimeout：内层 Err 从 `fmt.Errorf("i/o timeout")` 改为 `os.ErrDeadlineExceeded`（真实 net I/O deadline 超时内核构造，实现 `Timeout() bool`，repro 实测 Timeout()=true；且独立于 DNS/syscall/context 分支，单独覆盖此前零覆盖的 net.Error.Timeout() 分支）。实现不动 —— 加 "i/o timeout" 子串匹配会重新引入 Phase 32 移除的脆弱匹配。
verification: WSL 实证链 —— ① stash 后 pristine HEAD 复现原失败（upload_test.go:778 Should be true）；② 修复后 targeted test PASS（stash pop 前后各一次，稳定）；③ TestIsBackendUnavailable 全组 7 个 PASS（debugger 与 orchestrator、session-manager 三方各验一遍）；④ `go build` + `go vet` 通过；⑤ 全包失败集合修复前后完全一致（TestDataDecisionExtensionNoNewChunkTypes + TestFeedbackDispositionAdminGate，均为 pristine HEAD 上的 pre-existing 失败，属其他子系统/进行中工作，与本修复无关，delta 检查证明零回归）。
files_changed:
  - internal/server/upload_test.go

## Current Focus

hypothesis: (a) 已确认 —— 测试侧构造违约：`net.OpError.Timeout()` 委托给内层 Err 的 `Timeout()` 方法，`fmt.Errorf("i/o timeout")` 无该方法 → 构造的 Timeout()=false → isBackendUnavailable 走 branch 4 但返回 false。测试意图（覆盖 upload.go:471-475 的 net.Error.Timeout() 分支）从未被交付。
test: 最小 repro（WSL go run）已实证 A/B/C/D 四个语义事实；修完后 WSL 跑 targeted test → TestIsBackendUnavailable* 全组 → 包 build → 全包 test。
expecting: 改构造为 `&net.OpError{Op:"read", Err: os.ErrDeadlineExceeded}` 后 targeted test 绿，且 TestIsBackendUnavailable* 全组绿（证明 branch 4 被独立覆盖、无误伤其他分支）。
next_action: 已归档（2026-09-14 human-verify confirmed fixed → resolved/ + knowledge base + commit）
reasoning_checkpoint:
  hypothesis: "测试侧构造错误 —— net.OpError.Timeout() 委托给内层 Err 的 Timeout() 方法，fmt.Errorf 产生的普通 error 没有 Timeout() 方法，因此 &net.OpError{Op:\"read\", Err: fmt.Errorf(\"i/o timeout\")} 的 Timeout() 返回 false，isBackendUnavailable 判 false，断言失败"
  confirming_evidence:
    - "repro A：该构造 Timeout()=false（且 errors.As net.Error=true，说明会进 branch 4 但 Timeout 为 false）"
    - "repro B：&net.OpError{Op:\"read\", Err: os.ErrDeadlineExceeded} Timeout()=true"
    - "git 考古：原实现（1ad524a）只匹配 4 个子串（connection refused/context deadline exceeded/no such host/context canceled），从不匹配 i/o timeout；测试与 typed 实现同 commit（baf1ef0）出生即失败"
    - "同 commit 其余 6 个测试（nil/generic/ECONNREFUSED/DNS/Deadline/Canceled）全绿 —— 实现分类逻辑正常"
  falsification_test: "若 repro A 输出 Timeout()=true，则实现侧应已使测试通过（嫌疑转向实现）—— 实测 false，实现侧嫌疑被推翻"
  fix_rationale: "测试注释声称 'generic net.Error with Timeout() == true'，意图是覆盖 upload.go:471-475 的 net.Error.Timeout() 分支（当前零覆盖）。改用 os.ErrDeadlineExceeded（真实 net I/O deadline 超时的标准内核构造，实测 Timeout()=true，且不命中 DNS/syscall/context 分支）使构造真正交付注释意图；不动实现 —— 给 isBackendUnavailable 加 'i/o timeout' 字符串匹配会重新引入 Phase 32 刚移除的脆弱子串匹配（commit baf1ef0 明确要 robust against upstream message changes）"
  blind_spots: "假设 WSL Go 版本下 OpError.Timeout() 委托语义稳定（Go 长期稳定行为，repro 已实证当前环境）；未单独验证 os.ErrDeadlineExceeded 不会经 errors.Is 匹配 context.DeadlineExceeded（repro D 显示其为独立类型 —— 修完后跑 TestIsBackendUnavailable* 全组实证无误伤）"

## Eliminated

- hypothesis: (b) `isBackendUnavailable` 曾对「OpError 包裹的 i/o timeout 字符串」做过启发式分类，重构收紧后测试没跟上（实现侧意图违约）
  evidence: git 考古 —— 原始实现（1ad524a，Phase 27-07-T1）用 `strings.Contains` 只匹配 "connection refused"/"context deadline exceeded"/"no such host"/"context canceled" 四个子串，从不匹配 "i/o timeout"；baf1ef0（Phase 32 REALBUG-03）同时引入 typed 实现与全部 7 个测试，即该测试是「出生即失败」，实现从未对这个构造返回 true。
  timestamp: 2026-09-14

## Notes

- 修复时注意：该测试在 WSL 跑（runbook），Windows 侧 go 不可用于本包（duckdb bindings 链接问题）。
- 若判定为 (a) 改测试：保持「generic net.Error with Timeout()==true」这个测试意图，用能真正产生该语义的构造（如 `&net.OpError{Op:"read", Err: os.ErrDeadlineExceeded}`——os.ErrDeadlineExceeded 实现了 Timeout() bool）。
- 若判定为 (b) 改实现：不要用过宽的字符串匹配把普通网络错误都判成 backend unavailable（会误伤上游把可重试/不可重试的区分），分类收紧语义需与 upload 上报路径的实际用途一致。
- 归档时 worktree 注意：`internal/server/pipeline.go`（+1 行 emitSubprogress "retrieve"）属于并行会话 subprogress-missing-retrieve，未随本会话提交。

### 移交事项（本轮不开新会话，由 orchestrator 统一安排）

2026-09-14 实测：全包 `./internal/server/` 在本修复之外另有 2 个 pre-existing 失败（pristine HEAD 同样失败，与本会话零关联；全包复现命令 `wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && /usr/local/go/bin/go test ./internal/server/ -v'`）。

1. **TestDataDecisionExtensionNoNewChunkTypes** —— HITL approval 500
   - 现象：POST `/api/session/:id/approval/:approvalId` MarkApproved 返回 `{"error":"audit write failed"}`；服务端日志 `[ERROR] server: approval MarkApproved (sess=sess-datadec-01 id=tc-000001) failed: hitl: MarkApproved: skill snapshot required (D11-C2)`；断言失败于 `internal/server/approval_api_test.go:468`（"approve must succeed"）。
   - 复现：`wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && /usr/local/go/bin/go test ./internal/server/ -run TestDataDecisionExtensionNoNewChunkTypes -v'`
   - 初步定位：approval 测试 fixture 未 seed D11-C2 要求的 skill snapshot → MarkApproved 前置校验报错；二选一 —— 测试 fixture 补 skill snapshot，或确认 MarkApproved 对 snapshot 缺失的报错映射成 "audit write failed" 是否语义正确（错误归因疑似失真：snapshot 缺失被包装成 audit 写失败）。
2. **TestFeedbackDispositionAdminGate / "admin gate is the FIRST statement of every admin handler"** —— source-order 断言
   - 现象：源码顺序正则断言失败，期望 `func (s *Deps) feedbackMetricsHandler(...) {` 后第一条语句即 `if !isAdminRequest(c) {`；报错消息 "feedbackMetricsHandler must run the admin-token gate as its FIRST statement"。同测试其余 6 个 subtest 全 PASS。
   - 复现：`wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && /usr/local/go/bin/go test ./internal/server/ -run TestFeedbackDispositionAdminGate -v'`
   - 初步定位：feedback.go 的 `feedbackMetricsHandler` 当前首语句不是 admin gate（可能有前置语句插在 gate 之前），或 handler 改写后正则漂移；二选一 —— 把 gate 挪回首语句位，或（若 gate 语义已变化）更新断言正则并确认安全边界不弱化。
