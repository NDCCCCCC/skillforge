---
phase: quick-260814-w4n
verified: 2026-08-14T00:00:00Z
status: human_needed
score: 5/5 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Live-stack E2E hot reload (plan verification checklist item): go run ./cmd/server, then remove an existing skills/<name>/ dir and create a new skills/<new>/SKILL.md while the server runs"
    expected: "GET /api/skills reflects both changes within ~1-2s (new skill listed with version>0, removed skill gone, no failed residue)"
    why_human: "Verifier contract prohibits starting servers; requires live process + real HTTP observation. All constituent layers individually verified (real fsnotify tests on Windows + handler wiring), this is belt-and-braces E2E confirmation"
---

# Quick Task 260814-w4n: Skill 热重载目录级 create/remove — Verification Report

**Task Goal:** 修复 Skill 热重载缺口：fsnotify 目录级 create/remove 监听，新 Skill 目录自动发现 + 已删 Skill 自动移除（Windows 本机 + Linux CI 兼容，不回退现有防护）
**Verified:** 2026-08-14
**Status:** human_needed (1 live-stack E2E item; all code-level must-haves verified)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Watch 运行期间新增 skills/&#8203;<name>/SKILL.md 自动注册（Snapshot + GET /api/skills，含 lazy embed） | ✓ VERIFIED | `watchLoop` CREATE-on-direct-subdir 分支 (registry.go L366-373): `watcher.Add(subdir)` + `scheduleReconcile(<dir>/SKILL.md)`；`reconcileOne` L494 存在 → `reloadOne`（含 lazy embed）。真实 fsnotify 测试 `TestRegistry_Watch_NewSkillDirDiscovered` 本机 Windows PASS (1.11s)。HTTP 层：skills.go L132 `s.Registry.Snapshot()` 每请求直读、无缓存 → 列表自动出现 |
| 2 | Watch 运行期间删除 skills/<name>/ 目录 → entry 自动移除（含 failed map 残留），GET /api/skills 不再列出 | ✓ VERIFIED | `watchLoop` Remove/Rename 分支 (L374-385) 两种事件形状归一到 reconcile；`reconcileOne` L497-500 文件不存在 → `removeFromSnapshot(name)`（不落 setFailed）；`removeFromSnapshot` L534 `clearFailed(name)` 清除残留。`TestRegistry_Watch_RemovedSkillDirDropped` PASS (1.11s)，断言 Failed() 为空 |
| 3 | 原子写（remove + rename）不误删 —— 静默期后文件存在则走 reload | ✓ VERIFIED | `reconcileOne` 以 Lstat 存在性分流：存在 → reloadOne；`TestRegistry_Watch_EditorAtomicSave_NotDropped` PASS (1.11s)，断言 skill 保留且 description 更新为 "atomic-save-new"（证明走 reload 分支） |
| 4 | 同窗交织突发（删 A + 增 B）两者都收敛 —— O(n) 存在性扫描兜底 | ✓ VERIFIED | `reconcileOne` L505-511 遍历 snapshot 对每个 entry lstat `<root>/<name>/SKILL.md`，不存在即 `removeFromSnapshot`；`scheduleReconcile` 注释 L434-439 明确记录单 pending path 覆盖丢移除事件的根因。`TestRegistry_Watch_InterleavedBurst_Converges` PASS (1.11s)：无 compliance、有 qa、有 burst-new、Failed() 空；确定性用例 `TestRegistry_Reconcile_MissingFile_RemovesDeterministic` PASS (0.02s) 且含孤儿 entry 被扫描移除的子断言 |
| 5 | 现有单文件 reload 语义零回退（1s debounce / keep-old / V12 symlink guard / JS-safe version / 现有测试全绿） | ✓ VERIFIED | `ReloadDebounce = 1s` (L42) 未变；reloadOne 错误路径 keep-old + loadError hook (L562-573) 未变（经 parseAndValidate 复用 symlink guard）；`nextVersion` seq 计数器 (L216-218) 未变；watchLoop recover/ctx.Done/errors 分支保留 (L331-398)。`go test ./internal/skill/... -count=1` 全绿 (ok, 6.751s)，含 TestRegistry_Reload_Debounce / ValidationFailure_KeepsOld / FailedMap（SymlinkEscape 在 Windows 按既有设计 skip 并注明原因） |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/skill/registry.go` | 目录级 create/remove 监听 + reconcileOne（含 O(n) 扫描）+ removeFromSnapshot；contains 锚点 `reconcileOne` | ✓ VERIFIED | 存在（commit 3a46a24, +171/-7）；`reconcileOne` (L493)、`removeFromSnapshot` (L518)、`isDirectSubdir` (L458)、watchLoop 目录事件分类 (L366-385)、New() 启动期子目录 Add (L159-181，严格遵守 Add(root) 之后) 全部实质实现并含双平台语义注释 |
| `internal/skill/registry_test.go` | 4 组回归测试 + 1 个确定性直调用例；contains 锚点 `TestRegistry_Watch_NewSkillDirDiscovered` | ✓ VERIFIED | 存在（commit c3ccf55, +241）；5 个测试全部实现且本机 Windows 真跑全 PASS；fsnotify 实时用例均有 testing.Short() skip + require.Eventually(8s/100ms)，确定性用例无 skip |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| cmd/server/main.go (~L261) | registry.go watchLoop | `reg\.Watch\(ctx\)` | ✓ WIRED | main.go L261 `reg.Watch(ctx)` 已存在（本任务未改 main.go，git 确认）；Watch → watchLoop 消费全部目录级/文件级事件 → 新行为无需接线改动即生效 |
| registry.go removeFromSnapshot (copy-on-write Store) | internal/server/skills.go listSkillsHandler (GET /api/skills) | `s\.Registry\.Snapshot\(\)` | ✓ WIRED | skills.go L132 每请求直读 Snapshot()、无缓存；snapshot 收缩 → 列表自动收缩；Failed() overlay + 防御性 listing (L154-159) 因 removeFromSnapshot 的 clearFailed 不会冒出已删 skill 的幽灵条目 |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| 全仓构建 + vet | `go build ./... && go vet ./...` | 零输出 | ✓ PASS |
| internal/skill 全量测试（现有 + 新增，零回退门禁） | `go test ./internal/skill/... -count=1` | `ok ... 6.751s` | ✓ PASS |
| 5 个新测试 Windows 真跑 | `go test ./internal/skill/... -count=1 -v -run 'TestRegistry_Watch_NewSkillDirDiscovered\|...\|TestRegistry_Reconcile_MissingFile_RemovesDeterministic'` | 5/5 --- PASS (1.11s ×4 + 0.02s) | ✓ PASS |

### Scope Confirmation (git evidence)

- Commit `3a46a24` touched only `internal/skill/registry.go` (+171/-7)
- Commit `c3ccf55` touched only `internal/skill/registry_test.go` (+241)
- `cmd/server/main.go`、`internal/server/skills.go`、`web/`、wizard、HTTP 端点集合：本任务零改动（各自最后提交均早于本任务）— 符合计划 "不改动" 约束

### Linux CI 兼容（truth 隐含要求）

`New()` 在 `watcher.Add(root)` 之后、startupScan 之前对每个直属子目录 `watcher.Add` (registry.go L159-181)，修复 Linux inotify 非递归缺口；Add 失败仅 slog.Warn。代码 + 注释就绪；实际 ubuntu CI 运行属 CI 兜底，本机无法验证（见 human item 的覆盖范围之外，风险低 — require.Eventually 模式双平台稳定）。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/skill/examples_validate_test.go | 46 | "PLACEHOLDER tier" | ℹ️ Info | 预先存在、非本任务修改文件（git scope 确认），指测试夹具的 tier 命名，非未完成实现 |

本任务两个修改文件无 TBD/FIXME/XXX/TODO/HACK 标记，无空实现、无 console-only 处理。

### Human Verification Required

### 1. Live-stack E2E 热重载（plan verification checklist 末项）

**Test:** `go run ./cmd/server` 启动后，删除一个现有 `skills/<name>/` 目录并新建 `skills/<new>/SKILL.md`，轮询 `GET /api/skills`
**Expected:** 1-2s 内（1s debounce + 余量）新 skill 出现（version>0）、被删 skill 消失、无 failed 残留
**Why human:** 验证器契约禁止启动服务器；需真实进程 + HTTP 观察。各层已分别验证（真实 fsnotify 测试 + handler 无缓存直读 Snapshot 接线），此项为端到端兜底确认

### Gaps Summary

无代码级缺口。5/5 must-have truths 全部经真实测试（本机 Windows，非 mock）与代码走读验证；三条门禁命令全绿；范围外文件零改动。唯一待办是 plan 自身清单中的 live-stack 复验项，已列入 human_verification。

---

_Verified: 2026-08-14_
_Verifier: Claude (gsd-verifier)_
