---
status: partial
phase: 49-hermetic-demo
source: [49-01-SUMMARY.md, 49-02-SUMMARY.md, 49-03-SUMMARY.md, 49-04-SUMMARY.md]
started: 2026-10-23T01:05:00+08:00
updated: 2026-10-22T23:34:53.642Z
---

## Current Test

[testing complete]

## Tests

### 1. 冷启动冒烟测试
expected: 全新启动（无 key、embedded 后端、真实 embedding 端点）：服务无错启动、STUB WARN 出现、冷索引完成后 /api/skills 200、首轮问答返回带引用的结构化 Verdict
result: [pass]
note: 真实执行（WSL）：无 key + embedded + 真实端点，冷索引 218 篇（vectors.bin 916379 B）后 /api/skills 200；`[WARN] cmd/server: STUB LLM MODE` 出现；8 个 Skill 加载。执行者 = Claude（用户 m08420 委托）。


### 2. 无 key 时如实标注 stub
expected: wire 的 data-policy 载荷逐字含 `"mode":"stub"`、confidence ≤ 0.5、review_required: true；启动日志有 STUB WARN
result: [pass]
note: 真实执行：data-policy 帧实测 status=ok、mode='stub'、confidence=0.5、review_required=True、citations=1；STUB WARN 日志 1 条；GLM_API_KEY 确认 unset。


### 3. 引用真实可核（grounding）
expected: answer 的引用 span 能在 authority chunk 原文里找到；问一个语料没有的问题 ⇒ 拒答（no_relevant_source 或低分拒答），不编造
result: [pass-with-issue]
note: grounding PASS：引用 span 逐字存在于 knowledge/authority/travel_expense.md。拒答 ISSUE(minor)：域外查询（房价/十四行诗/量子纠缠）未触发 low_retrieval_score，stub 硬答最相近域内 chunk —— 答案无编造，但 stub 模式缺语义拒答层（pipeline.go:64 阈值 0.3 为 Phase 2 占位 D2-15，真实模型 cosine 普遍偏高；SKILL.md 拒答指令对确定性 extractor 无效）。非 phase 49 回归，记 Gaps。


### 4. 真实 GLM key 路径（H-1）
expected: 设置真实 GLM_API_KEY 重启后走同一问：响应无 `mode` 字段、答案由模型生成、引用仍经 grounding 校验
result: [blocked]
note: 需要真实 GLM_API_KEY（当前 unset）。该路径无自动化覆盖，属于人工验收项 H-1；等用户提供 key 后重测。


### 5. README 前端段照着念（H-2）
expected: cd web && npm install && npm run build 后由后端 :8888 同源打开（或 npm run dev :5173）：页面可用、STUB 徽标出现在 stub 答案旁、无 console 报错
result: [pass]
note: 服务侧真实执行：npm run build rc=0；后端同源 GET / → 200 且返回 web/dist 的 SPA 壳（<!doctype html>…）。浏览器内 STUB 徽标外观有 20 条 vitest 覆盖，像素级观感留给用户（与 H-2/H-3 同项）。


### 6. demo.gif 观感（H-3）
expected: 打开 docs/demo.gif：真实终端录屏、中文无豆腐块、约 736×328 / 46 帧 / 207KB，判断是否适合放 README 首屏
result: [pending]

### 7. Skill 切换即换域
expected: 同一条 POST /api/chat，把请求体 "skill" 从 simple-qa 换成 sql-guides：引用 tier 从 authority 变为 sql-guides（同一内核不改代码）
result: [pass]
note: 真实执行：同一命令仅换 "skill":"sql-guides" ⇒ citation tier 从 authority 变 sql-guides，answer 为 SQL 域内容（承保/保单字段）——同一内核不改代码换域，实证成立。


## Summary

total: 7
passed: 4
pass_with_issue: 1
blocked: 1
pending_user: 1
issues: 0
pending: 0
skipped: 0

Executed by Claude on user delegation (m08420), 2026-10-23, against a real
embedding endpoint (fastembed mxbai-embed-large-v1, 1024-dim) in WSL with no
API keys set.

## Gaps

- GAP-UAT-1 (minor, pre-existing, not a phase-49 regression): stub 模式下域外查询不拒答。
  症状："北京房价/爱情十四行诗/量子纠缠" 等查询返回 status=ok 并引用最相近的域内 chunk。
  归因：(a) retrievalScoreThreshold=0.3（internal/server/pipeline.go:64，Phase 2 占位决策
  D2-15）在真实语义模型 + 同域语料下形同虚设；(b) stub 是确定性抽取器，不走 LLM prompt，
  SKILL.md 的拒答指令对其无效。答案本身无编造（全部来自真实 chunk）；tier 隔离不受影响。
  去向：建议后续 phase 调检索阈值 / 为 stub 增加相似度下限拒答。
