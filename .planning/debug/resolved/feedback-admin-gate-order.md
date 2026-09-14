---
slug: feedback-admin-gate-order
status: resolved
created: 2026-09-14
updated: 2026-09-14
trigger: "TestFeedbackDispositionAdminGate pre-existing 失败（backend-unavailable-test-fail 会话归档时移交），用户要求处理"
resolved: 2026-09-14
---

# Debug Session: feedback-admin-gate-order

## Symptoms

- **Expected**: `go test ./internal/server/ -run TestFeedbackDispositionAdminGate` 全部子测试通过。
- **Actual**: 唯一失败子测试 `admin_gate_is_the_FIRST_statement_of_every_admin_handler` —— 源码顺序正则断言 feedbackMetricsHandler 的第一条语句必须是 `if !isAdminRequest(c) {`（允许中间夹注释行）。其余 6 个行为子测试全部 PASS。
- **Errors**: regex 不匹配。失败输出里回显的源码片段显示匹配窗口落在了 `putFeedbackAuditRow` 函数体上（helper 的第一条语句是 store-nil guard，不是 admin gate）。
- **Timeline**: pre-existing（2026-09-14 Bug A 归档时确认 pristine HEAD 同样失败）。
- **Reproduction**: WSL `go test ./internal/server/ -run TestFeedbackDispositionAdminGate -v` 稳定复现（orchestrator 2026-09-14 实跑确认）。

## Evidence

- timestamp: 2026-09-14 (orchestrator, WSL go test -v)
  - 失败断言正则：`func \(s \*Deps\) feedbackMetricsHandler\([^)]*\) \{\s*\n(?:\s*//[^\n]*\n)*\s*if !isAdminRequest\(c\) \{`，message: "feedbackMetricsHandler must run the admin-token gate as its FIRST statement"。
  - 关键事实：**行为全对**（no token → 403 + 零写入、nil store → 503、valid token 正常处置等 6/6 PASS），失败的只是「gate 必须是第一条语句」的源码结构断言 → 要么 handler 前言新加了语句（重构引入），要么正则扫描窗口 drift。
- timestamp: 2026-09-14 (失败输出回显)
  - 回显片段是 putFeedbackAuditRow（feedback audit helper，含 `if s.Store == nil` guard）——提示正则的函数匹配可能跨函数吞进了后续 helper（比如 `func ... { ... }\n\n// comment\nfunc (s *Deps) putFeedbackAuditRow` 之间的空白/注释结构变化影响 `\{` 后的匹配窗口），而非 handler 前言真的插了语句。
- timestamp: 2026-09-14 (Read feedback.go:455/553/680, audit.go:100-148)
  - **定案**：三个 admin handler 的第一条可执行语句都是 `if !requireAdmin(c) {`（feedback.go:458/556/683），其前只有正则允许的注释行 —— gate 仍是 FIRST statement，hypothesis (a)（前言新增语句）ELIMINATED。
  - audit.go:142 `requireAdmin` = `isAdminRequest` 检查 + 集中写标准 403 envelope（Phase 35 REPEATHTTP-02，commit bb3128c，替换 14 处 copy-paste gate 模板）；`if !isAdminRequest(c) {` 字面量在全仓库已不存在（仅剩 audit.go:136 注释里的历史示例）。
  - 测试正则（feedback_test.go:821，commit 5898fc7 / Phase 29-05 引入）仍锚定旧字面量 → 对 3 个 handler 全部失配；bb3128c 未更新该正则。
  - putFeedbackAuditRow 回显是 red herring：testify `assert.Regexp` 失败时打印整个源文件字符串，orchestrator 看到的只是长输出的一个窗口，并非跨函数匹配。
- timestamp: 2026-09-14 (WSL go test 实跑)
  - 复现基线：`-run TestFeedbackDispositionAdminGate -v` 仅 admin_gate 子测试 FAIL（3 条 assert.Regexp 全报，feedbackMetricsHandler 最后一条）；`./internal/server/` 全包 pristine 失败集 = 恰好这 1 个子测试，与其余全部 PASS —— 与 orchestrator 移交信息一致。

## Current Focus

hypothesis: CONFIRMED + FIXED + SELF-VERIFIED —— root cause 见 Resolution.root_cause。修复只动 feedback_test.go 的 admin_gate 子测试：正则锚点从旧字面量 `if !isAdminRequest\(c\) \{` 换成现行 `if !requireAdmin\(c\) \{`，并新增第二条源序断言锁定 audit.go 里 `requireAdmin` 自身的第一条语句是 `if isAdminRequest(c) {` —— 使「token 检查先于任何副作用/数据访问」的原始 defense-in-depth 契约在 helper 抽取后由两半共同锁定，断言未被删除或放宽为无意义。
test: WSL 实跑（复现基线 → 修复 → 定向 + 全包 + build + vet）。
expecting: 已达成 —— 定向 7/7 PASS；全包 `ok`（pristine 失败集恰为 1 个子测试，修复后失败集为空，无新增失败）；go build ./... 与 go vet ./internal/server/ 均 0 退出。
next_action: ARCHIVED —— human-verify checkpoint 由 orchestrator 独立复核确认（2026-09-14：WSL 实跑定向 7/7 PASS + diff 复核确认两半契约未删除未放宽）；按 archive_session 协议提交 fix + docs + knowledge base。
reasoning_checkpoint:（见上方 Evidence 2026-09-14 定案条目；五字段已在修复前填写：hypothesis=断言锚定过期字面量；evidence=三 handler 源码 + audit.go 双 helper + 全仓库字面量清零；falsification=任一 handler gate 前有可执行语句或 requireAdmin 不再先查 token；fix_rationale=把断言升级为表达 helper 抽取后的真实两半契约；blind_spots=其他 admin 源序断言复用风险——已用 grep 排除：全仓库仅此一处结构性 lock，其余isAdminRequest 提及均为注释）

## Eliminated

- hypothesis (a) feedbackMetricsHandler 前言重构新增语句 —— 读源码排除：三个 handler gate 前只有注释行，gate 仍是第一条可执行语句。
- hypothesis 正则跨函数匹配（吞进 putFeedbackAuditRow）—— 排除：回显是 testify assert.Regexp 失败时打印整个源文件，非正则行为。

## Resolution

root_cause: commit bb3128c（Phase 35 REPEATHTTP-02）把 14 处 inline admin gate 模板（`if !isAdminRequest(c) { c.JSON(403); return }`）抽取为 helper `requireAdmin(c)`（audit.go:142，内部第一步仍是 isAdminRequest 检查，失败时集中写标准 403 envelope），但未同步更新 feedback_test.go:821 的源序正则 —— 该正则（Phase 29-05 / 5898fc7 引入）仍锚定已不存在的旧字面量 `if !isAdminRequest\(c\) \{`，对三个 feedback admin handler 全部失配。gate 语义自始未坏（三个 handler 第一条可执行语句都是 `if !requireAdmin(c) {`，6/6 行为子测试一直 PASS）；失败的只是源码结构断言。附：失败输出里的 putFeedbackAuditRow 回显是 red herring（testify assert.Regexp 失败时打印整个源文件，看到的只是长输出的一个窗口）。
fix: 仅改 internal/server/feedback_test.go 的 admin_gate 子测试 —— (1) 正则锚点 `if !isAdminRequest\(c\) \{` → `if !requireAdmin\(c\) \{`（保留「brace 与 gate 之间只允许注释行」的严格结构）；(2) 新增第二条源序断言锁定 audit.go 的 `func requireAdmin\([^)]*\) bool \{` 第一条语句是 `if isAdminRequest(c) {`。两半合计把「token 检查先于任何副作用/数据访问」的 defense-in-depth 契约在 helper 抽取后完整表达，断言意图未删除、未放宽。
verification: WSL 实跑 —— 修复前基线：`-run TestFeedbackDispositionAdminGate -v` 仅 admin_gate 子测试 FAIL，`./internal/server/` 全包失败集 = 恰该 1 个子测试；修复后：定向测试 7/7 PASS，全包 `ok`（失败集为空，无新增失败），`go build ./...` 与 `go vet ./internal/server/` 均 0 退出。测试为确定性源序 grep + 行为断言，无时序/环境依赖。Orchestrator 独立复核（2026-09-14）：实跑 `-run "TestDataDecisionExtensionNoNewChunkTypes|TestFeedbackDispositionAdminGate" -v` 双测全 PASS + diff 复核通过 → 用户 confirmed fixed。
files_changed:
  - internal/server/feedback_test.go

## Notes

- WSL 跑测试；:8888 在跑，勿杀。
- 该断言的意图（源码级 defense-in-depth：admin gate 必须先于任何处理）与 putFeedbackAuditRow 的 best-effort 语义都来自 ADM-06/feedback 处置流，修复时保持两个意图都不受损。
- 若判定 handler 前言新增 guard 是合理演进，检查同文件其他 admin handler 是否同样漂移（正则可能只抽查了 feedbackMetricsHandler 一个）→ 已排查：全仓库仅此一处结构性 lock 锚定旧字面量。
- 经验：helper 抽取类重构（如 Phase 35 抽 requireAdmin）必须同步 grep 所有锚定函数体字面量的结构断言（源序 lock、regex 注入类测试）。
