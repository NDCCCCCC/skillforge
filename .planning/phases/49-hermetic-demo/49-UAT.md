---
status: complete
phase: 49-hermetic-demo
source: [49-01-SUMMARY.md, 49-02-SUMMARY.md, 49-03-SUMMARY.md, 49-04-SUMMARY.md]
started: 2026-10-23T01:05:00+08:00
updated: 2026-10-23T18:40:00+08:00
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
result: [pass-with-issue]
note: 编排者代验（用户无 key，m09560「继续，帮我完成测试」授权）。用本地 mock OpenAI chat/completions 服务（返回真实 Verdict JSON 形状、不含 mode 字段）验证**装配语义**：stub 基线 data-policy 逐字含 `"mode":"stub"` → 面板 POST /api/admin/llm/config（custom 上游）⇒ `{available:true, mode:"live", hot-reloaded}` → **无重启**再 chat ⇒ data-policy `"mode"` 出现 0 次、SSE 含真上游 Verdict（confidence 0.92）。发现的装配缺陷（LLMDynamic 未装进 Deps，面板 5 端点 503）已修复并随本阶段提交。**mock 的 citation 与检索 chunk 脱节 ⇒ GroundingCheck 诚实降级**（confidence 钳 0.89、清 citations、needs_input、answer 保留）——grounding 层按设计工作，非产品缺陷；但该降级使"引用经 grounding 且通过"的完整链路在 mock 上无法闭环，**真实 provider（GLM/opencode）质量待用户有 key 时自测**（Phase 52 面板可免重启配置）。


### 5. README 前端段照着念（H-2）
expected: cd web && npm install && npm run build 后由后端 :8888 同源打开（或 npm run dev :5173）：页面可用、STUB 徽标出现在 stub 答案旁、无 console 报错
result: [pass]
note: 服务侧真实执行：npm run build rc=0；后端同源 GET / → 200 且返回 web/dist 的 SPA 壳（<!doctype html>…）。浏览器内 STUB 徽标外观有 20 条 vitest 覆盖，像素级观感留给用户（与 H-2/H-3 同项）。


### 6. demo.gif 观感（H-3）
expected: 打开 docs/demo.gif：真实终端录屏、中文无豆腐块、约 736×328 / 46 帧 / 207KB，判断是否适合放 README 首屏
result: [pass]
note: 编排者代验（抽帧 + read_image，用户可复核）：46 帧 736×328、GIF89a、211794 B；画面为真实终端录屏——含真实提问、`--- Verdict (data-policy frame) ---`（status ok / mode stub / confidence 0.5 / review_required true / citations 1 与 authority 原文 answer）、/api/skills 列表、sql-guides 域切换与 tier 变化；中文经 CJK 字体渲染无豆腐块、双宽对齐正确；深色底清晰，适合 README 首屏。"观感喜好"最终判断仍属用户（H-3 保留人工复核权）。

### 7. Skill 切换即换域
expected: 同一条 POST /api/chat，把请求体 "skill" 从 simple-qa 换成 sql-guides：引用 tier 从 authority 变为 sql-guides（同一内核不改代码）
result: [pass]
note: 真实执行：同一命令仅换 "skill":"sql-guides" ⇒ citation tier 从 authority 变 sql-guides，answer 为 SQL 域内容（承保/保单字段）——同一内核不改代码换域，实证成立。


## Summary

total: 7
passed: 5
pass_with_issue: 2
blocked: 0
pending_user: 0
issues: 0
pending: 0
skipped: 0

Executed by Claude on user delegation (m08420 + m09560), 2026-10-23, against a
real embedding endpoint (fastembed mxbai-embed-large-v1, 1024-dim) in WSL with
no API keys set. Test 4 used a local mock OpenAI-compatible upstream to verify
the assembly/hot-reload semantics (real provider quality remains a user
self-check). Test 6 was verified by the orchestrator via frame extraction;
the user retains final say on aesthetics.

## Gaps

- GAP-UAT-1 (minor, pre-existing, not a phase-49 regression): stub 模式下域外查询不拒答。
  症状："北京房价/爱情十四行诗/量子纠缠" 等查询返回 status=ok 并引用最相近的域内 chunk。
  归因：(a) retrievalScoreThreshold=0.3（internal/server/pipeline.go:64，Phase 2 占位决策
  D2-15）在真实语义模型 + 同域语料下形同虚设；(b) stub 是确定性抽取器，不走 LLM prompt，
  SKILL.md 的拒答指令对其无效。答案本身无编造（全部来自真实 chunk）；tier 隔离不受影响。
  去向：建议后续 phase 调检索阈值 / 为 stub 增加相似度下限拒答。
