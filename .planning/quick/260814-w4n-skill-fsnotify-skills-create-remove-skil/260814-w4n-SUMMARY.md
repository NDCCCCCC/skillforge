---
phase: quick-260814-w4n
plan: 01
subsystem: backend/skill-registry
tags: [backend, skill, fsnotify, hot-reload, windows-compat]
requires:
  - "internal/skill/registry.go Watch/debounce 基建 (D5-06/D5-08)"
  - "internal/skill/loader.go parseAndValidate (V12 symlink guard)"
provides:
  - "Watch 运行期 skills/<name>/ 目录增删自动发现/自动移除（reconcileOne + O(n) 存在性扫描）"
  - "Linux 子目录 watch（New() 启动时 Add 直属子目录）"
affects:
  - "internal/server/skills.go listSkillsHandler（经 Snapshot() 读端自动感知，无代码改动）"
  - "FE useSkillReload 轮询（经 GET /api/skills 感知增删，无 FE 改动）"
tech-stack:
  added: []
  patterns:
    - "debounce 回调统一为 reconcileOne：存在→reload / 缺失→remove（绝不 setFailed）+ snapshot 全量存在性扫描兜底单 pending path 覆盖丢失"
    - "目录级事件双平台归一：CREATE 直属子目录（Add+schedule）/ REMOVE·RENAME（SKILL.md 或目录名 → join SKILL.md）"
key-files:
  created: []
  modified:
    - internal/skill/registry.go
    - internal/skill/registry_test.go
decisions:
  - "删除分支绝不 setFailed/loadError —— 删除是移除不是加载失败，防 GET /api/skills 幽灵 failed 条目"
  - "单 timer 单 pending path 结构保留，移除语义由 reconcile 末尾 O(n) 存在性扫描兜底（替代多 pending path 重构，改动面最小）"
  - "scheduleReload 更名为 scheduleReconcile（回调从 reloadOne 换成 reconcileOne，语义扩展）"
  - "isDirectSubdir 对 Remove 事件只做路径形状判定（目录已不存在无法 Stat）"
metrics:
  duration: "~4 min (2026-08-14T15:19:11Z → 15:22:49Z)"
  completed: 2026-08-14
---

# Quick Task 260814-w4n: Skill fsnotify 目录级增删 Summary

修复 Skill 热重载的目录级缺口：watchLoop 现在消费 skills/ 直属子目录的 CREATE（挂子目录 watch + debounce 后 reconcile 注册，兜住 cp -r 落盘竞态）与 REMOVE/RENAME（存在性二次确认后移除 entry），reconcileOne 以 O(n) snapshot 存在性扫描兜底单 pending path debounce 在"删 A + 增 B"同窗突发下丢失的移除事件，并补齐 Linux 从未建立的子目录 watch。

## What Was Done

### Task 1 — internal/skill/registry.go（commit 3a46a24）

- `reconcileOne(path)`（未导出，测试同包直调）：SKILL.md 存在 → 现有 `reloadOne`（注册/重载，含 lazy embed）；缺失 → `removeFromSnapshot(skillNameFromPath(path))`，**绝不走 setFailed/loadError**（防幽灵 failed 条目冒上 GET /api/skills 的 defensive listing）。末尾 O(n) 存在性扫描：遍历 snapshot 每个 entry，`lstat(rootAbs/<name>/SKILL.md)` 不存在即移除。
- `removeFromSnapshot(name)`：copy-on-write 克隆 + delete + CAS Store（仅 name 原先存在才 Store），成功后 `clearFailed(name)` 清 failed map 残留，`slog.Info` 记录移除。
- watchLoop 事件分类（resolveSkillPath 之前）：
  1. `Create` + rootAbs 直属子目录（`filepath.Dir(event.Name) == rootAbs` 严格相等 + Stat 为 dir）→ `watcher.Add(event.Name)` + `scheduleReconcile(join(event.Name, "SKILL.md"))`
  2. `Remove|Rename`：event.Name 以 SKILL.md 结尾 → schedule；直属子目录名（仅路径形状判定，目录已不存在无法 Stat）→ schedule join 后的 SKILL.md
  3. 其余走原 `resolveSkillPath` → 统一 `scheduleReconcile`
- `New()`：`watcher.Add(root)` 之后、`startupScan` 之前（Windows L126-131 顺序约束），`os.ReadDir(root)` 对每个直属子目录 `watcher.Add` —— 修复 Linux inotify 非递归导致 SKILL.md 编辑收不到事件（此前仅靠 Windows 递归特性掩盖）；Add 失败仅 slog.Warn。
- `scheduleReload` 更名 `scheduleReconcile`（同一单 timer 单 pending path 结构，回调改 reconcileOne）。
- 零回退清单逐项核对：ReloadDebounce=1s、单 timer 结构、keep-old（reloadOne 错误路径原样）、V12 symlink guard（经 parseAndValidate 复用）、nextVersion JS-safe、watchLoop recover/ctx.Done/errors 分支 —— 均未触碰。

### Task 2 — internal/skill/registry_test.go（commit c3ccf55）

5 个新测试（复用 writeRegistrySkill / stubEmbedder；实时用例 testing.Short() skip 对齐既有模式，断言一律 require.Eventually 100ms/8s）：

| 测试 | 覆盖 must_have truth | 结果 |
|------|---------------------|------|
| TestRegistry_Watch_NewSkillDirDiscovered | truth 1 新目录自动发现（含 cp -r 竞态兜底） | PASS 1.11s |
| TestRegistry_Watch_RemovedSkillDirDropped | truth 2 删除自动移除 + 无 failed 残留 | PASS 1.11s |
| TestRegistry_Watch_EditorAtomicSave_NotDropped | truth 3 原子写不误删（reload 而非 remove） | PASS 1.11s |
| TestRegistry_Watch_InterleavedBurst_Converges | truth 4 同窗交织突发收敛（O(n) 扫描兜底） | PASS 1.12s |
| TestRegistry_Reconcile_MissingFile_RemovesDeterministic | truth 2/4 确定性直调（remove/reload 分支 + 孤儿扫描，不 skip -short） | PASS 0.03s |

四个实时用例在本机 **Windows 实际运行通过（-v 输出 PASS，非 skip）**。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] 孤儿扫描断言的时序修正**
- **Found during:** Task 2
- **Issue:** 首版测试在步骤 (a) reconcile compliance 之前就删除了 orphan 目录 —— 而 reconcileOne 的 O(n) 扫描在**每次** reconcile 末尾都运行，orphan 在步骤 (a) 就已被扫掉，导致 "sanity: orphan still registered" 断言失败。
- **Fix:** 将 orphan 目录删除移到步骤 (a) 之后、步骤 (c) reconcile(qaPath) 之前，使孤儿确实由"对另一路径的 reconcile 触发扫描"移除，忠实于计划表述。
- **Files modified:** internal/skill/registry_test.go
- **Commit:** c3ccf55

### 说明：TDD 标注

Task 1 在 frontmatter 标注 `tdd="true"`，但计划将全部测试放在 Task 2（Task 1 的 verify 只跑现有测试）。按计划的任务划分执行（feat 先行、test 随后），git log 中 test 提交在 feat 之后而非 RED-first。功能正确性由 Task 2 的 5 个新用例 + 全量回归共同背书。

## Verification

- [x] `go build ./...` 零错误
- [x] `go vet ./...` 零告警
- [x] `go test ./internal/skill/... -count=1` 全绿（现有 + 5 新，含 TestRegistry_Reload_Debounce / ValidationFailure_KeepsOld / SymlinkEscape_Rejected / FailedMap 零回退）
- [x] 5 个新测试 `-v` 单独跑全 PASS（Windows 实跑，无 skip）
- [x] git diff vs base 仅含 internal/skill/registry.go + registry_test.go —— 未触碰 cmd/server/main.go、internal/server/、web/、wizard、HTTP 端点集合
- [ ] live 栈复验（go run cmd/server + git rm/add + GET /api/skills）—— 计划明确由 orchestrator 复验，executor 未启动 live 栈

## Self-Check: PASSED

- internal/skill/registry.go — FOUND (modified, committed 3a46a24)
- internal/skill/registry_test.go — FOUND (modified, committed c3ccf55)
- commit 3a46a24 — FOUND
- commit c3ccf55 — FOUND
