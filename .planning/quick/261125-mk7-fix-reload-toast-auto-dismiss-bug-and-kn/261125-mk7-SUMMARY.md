---
status: complete
quick_id: 261125-mk7
date: 2026-11-25
---

# Quick 261125-mk7 Summary — ReloadToast 自动消失失效 + graph-browser 片段 403

## 结果

两个用户上报的 UI 回归均已修复并测试锁定：

**Bug 1 — ReloadToast 成功 toast 永挂**（commit `2ff4d6a`）
根因：父级 effect 的 timers 数组在 `skillReloadStatus` 变化时被 cleanup 全清，重跑时 ts 水位又跳过旧条目，timer 永不再建。修复：auto-dismiss 下沉到 `ToastItem` 的 per-item useEffect（ok 状态 4s 后 onDismiss；failed 常驻 + X 手动关闭不变）。回归测试：两条 ok toast 叠放 → advance 4s 后两条必须都消失。

**Bug 2 — 知识图谱浏览器取片段 403 /「加载片段失败」**（commit `377706f`）
根因：2026-11-09 的 `knowledgeReadAllowed` 闸门只认 X-Admin-Token / X-Skill-Name，而 FE `fetchChunkContent` 是裸 fetch，两个身份注入点都不经过——**已认证的浏览器这一条请求也不带任何身份头**（用户实测复现）。修复三件套：
1. BE：`knowledgeReadAllowed` 增加非空 `X-Owner-Token` 放行分支（与 X-Skill-Name 同级的自述身份，拦匿名批量枚举不变；graph 元数据本就公开读）。
2. FE：`fetchChunkContent` 同时发送 `X-Admin-Token`（localStorage `sf_admin_token`，有则带）与 `ownerTokenHeaders()`。
3. FE：ChunkSourceView 对 403 渲染专属文案「片段访问被拒绝（403）：请求缺少浏览器身份…」。

测试对齐修正（commit `c998bf7`）：`SkillReloadEntry.version` 是 `'new'|'old-kept'` 字面量类型，修正测试夹具；`chunk-lookup.test.ts` 的 fetch-headers 断言更新为含身份头。

## 验证

- `go test ./internal/server/... -run TestChunkHandler`：ok（含新增 OwnerTokenAllowed / NoHeadersStill403 / AdminTokenAllowed 三例）
- `go test ./...`：除 `cmd/server TestBootAssemblyOrder` 外全部 ok —— **该失败为既有失败**（在改动前提交 `fe73c0e` 上同样失败：main.go 未按断言注入 kbReg，与本任务无关）
- web vitest：全量 876 用例仅 chunk-lookup 1 例因新 headers 断言失败，修正后三个相关测试文件 45 例全过
- `tsc --noEmit`：干净
- `gofmt -l internal/server`：7 个文件未格式化为**既有状态**（identity.go / knowledge_test.go 均不在列表；executor 曾 stash 一份无关文件的 gofmt churn，见 stash@{0}）

## 提交

- `2ff4d6a` fix(web): ReloadToast per-item auto-dismiss survives successive reloads
- `377706f` fix(knowledge): allow declared browser owner token on chunk read; FE chunk fetch sends admin + owner identity headers
- `c998bf7` test(web): align 261125-mk7 tests with SkillReloadEntry literal types and new chunk-fetch identity headers
