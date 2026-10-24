---
phase: quick-260814-vga
plan: 01
subsystem: web-ui + skills-default
tags: [frontend, copywriting, de-compliance, simple-qa, default-skill]
requires: [260814-tsr]
provides: [generic-doc-qa-out-of-box-ui, simple-qa-repo-default]
affects: [web/src/components, web/src/__fixtures__/skills.json, skills/, README.md]
key-files:
  created:
    - skills/simple-qa/SKILL.md
    - examples/skills/compliance/eval-gate.json
  modified:
    - web/index.html
    - web/src/components/center/CenterPanel.tsx
    - web/src/components/layout/RightChat.tsx
    - web/src/components/chat/ChatInput.tsx
    - web/src/components/layout/SkillManagerMenu.tsx
    - web/src/__fixtures__/skills.json
    - web/src/__tests__/CenterPanel.test.tsx
    - web/src/__tests__/SkillPicker.test.tsx
    - cmd/eval/main_test.go
    - internal/skill/loader_test.go
    - internal/eval/frozen_test.go
    - README.md
  deleted:
    - skills/compliance/SKILL.md
    - skills/compliance/eval-gate.json
decisions:
  - "eval-gate.json relocated to examples/skills/compliance/ so a cp -r restore brings back the COMPLETE compliance eval experiment (gate threshold included)"
metrics:
  duration: 12m
  completed: 2026-08-14
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: unknown
---

# quick-260814-vga: UI 文案泛化 + skills/ 默认 simple-qa Summary

代码层去合规化收尾：web/ 开箱呈现为通用文档问答平台（标题/空状态/建议问题/占位符/tab 名），仓库默认 skills/ 仅含 simple-qa（字节等同 examples 模板），README 与新默认对齐。

## What Was Done

### Task 1 — 前端 UI 文案泛化 + fixture/测试同步 (commit `e42220e`)

- `web/index.html:10` 标题 → `SkillForge — Skill 驱动的文档问答平台`
- `CenterPanel.tsx`: tab `审查结论`→`结论`；空状态 `等待合规审查结果`→`等待回答结果`、`右侧提出合规问题…`→`右侧提出问题…`（含 :15-16 注释同步）；硬编码 `skillId: 'compliance-review'`→`'simple-qa'`
- `RightChat.tsx`: 3 个建议问题换为 knowledge/authority 种子可答题（年假折算/结转 ← annual_leave；采购合同 50 万以下审批 ← contract_approval；出差住宿超标审批 ← travel_expense）；`开始一次合规审查`→`开始一次文档问答`；副文案 `每个结论`→`每个回答`；图标 `ShieldCheck`→`BookOpen`（plan 自选项，弱化合规护盾视觉）
- `ChatInput.tsx:241` 占位符 → `输入问题，或 @ 触发 file/skill，或拖放文件...`
- `SkillManagerMenu.tsx:50` 硬编码 skillId → `'simple-qa'`
- `__fixtures__/skills.json`: compliance-review 条目删除，新增 simple-qa 条目（7 字段对应 examples/skills/simple-qa/SKILL.md：name 通用问答、reads [权威]、writes []、temp 0.3、steps 三步中文转写、prohibitions 三条、required_refusal_phrases ["no_relevant_source"]）；general-qa 条目保留不动
- 测试同步：`CenterPanel.test.tsx`（空状态文案 + tab 名 `结论` ×3 + skillId×2 + fixture 断言 通用问答）、`SkillPicker.test.tsx`（active-skill 断言 compliance-review→simple-qa，plan 预见的间接依赖）
- 验证：vitest **233/233 全绿**（35 文件；plan 写的 209 基线已随 05-07 后测试增长过时，全绿为准）；`npx tsc --noEmit` 0 错误

### Task 2 — skills/ 默认 Skill 换 simple-qa (commit `4bda3ce`)

- `git rm -r skills/compliance`（SKILL.md + eval-gate.json）；`skills/simple-qa/SKILL.md` 与 examples 字节等同（diff 无输出）
- `ls skills/` → 仅 `simple-qa/`；examples/skills/compliance 模板保留
- 验证：`go test ./internal/skill/ -run TestExamplesValidate` PASS；`go build ./...` + `go vet ./...` 通过；**全仓 `go test ./...` 全绿**

### Task 3 — README 最小一致性修正 + grep 验收 (commit `80b6c2d`)

- Architecture 段 `skills/compliance/SKILL.md` 链接 → `skills/simple-qa/SKILL.md`（"30-line" 措辞对 34 行的 simple-qa 仍成立，未改）
- Loaded Skills 段 repo default → `skills/simple-qa/SKILL.md`
- Quick Start step 4 改写：默认开箱即 simple-qa；原 cp 步骤变为"换领域模板"示例（`cp -r examples/skills/compliance skills/ && rm -r skills/simple-qa`，重启/热重载生效）；步骤总数与缩进结构不变
- Try-it 段核对：主演示已是 simple-qa + 差旅住宿问题，无需改

## "合规" grep 归因清单（web/ 非测试非 fixture 层，18 处全部 allowlist）

| 文件:行 | 内容 | 归因 |
|---|---|---|
| `types/verdict.ts:22` | `VERDICT_STATUSES = ['合规','不合规','需补充','unanswered']` | wire 状态枚举（schema，BE 发什么 FE 显示什么） |
| `common/StatusBadge.tsx:4-5,14-15,21-22` | 枚举值为 key 的 label/color 映射 + 注释 | wire 枚举映射与注释 |
| `center/VerdictCard.tsx:19-20,26-27` | 枚举值为 key 的 border/bg 映射 | wire 枚举映射 |
| `chat/MessageBubble.tsx:89-90` | 不合规/需补充 才渲染 RedlineCard 的行为注释 | 代码注释（wire 行为说明） |
| `verdict/RedlineCard.tsx:8-9` | 合规 verdict 不发 data-redline chunk 的注释 | 代码注释 |
| `types/wizard.ts:65,75` | wizard 三分类提示（合规审查类/通用问答类/创意类）及温度默认值 | wizard 分类学（选项之一，非主线叙事） |

`grep "合规\|审查"` 对 index.html / RightChat / ChatInput / CenterPanel 四文件用户可见字符串层 **0 命中**。README 中 `skills/compliance` 仅剩 Quick Start 的 `examples/skills/compliance` 模板引用（plan 明确允许的形式）。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] worktree 缺 web/node_modules**
- **Found during:** Task 1 verify（vitest 无法启动，ERR_MODULE_NOT_FOUND）
- **Issue:** git worktree 不继承未跟踪的 node_modules
- **Fix:** `web/` 下 `npm ci`（仅安装 package-lock.json 已锁定的依赖，无新包）
- **Commit:** 不适用（node_modules 已 gitignore）

**2. [Rule 1 - Bug] 三个 "shipped artifact" Go 测试因 skills/ 换默认而失败**
- **Found during:** Task 2（plan 的 verify 只要求 TestExamplesValidate + go build，未覆盖全仓 go test；全量跑出 3 处失败）
- **Issue:** `cmd/eval` TestShippedGateFileIsValid、`internal/eval` TestShippedFrozenLockIsInSync、`internal/skill` TestLoader_ParseComplianceCanonicalSkill 均断言仓库 skills/ 里存在 compliance——换默认后直接红
- **Fix:** 三处测试改指 `examples/skills/`（compliance 模板的唯一居所），断言本体不变，意图（守护 shipped 合规工件）保留；同时把 `eval-gate.json` 随模板迁至 `examples/skills/compliance/eval-gate.json`（内容不变 `{"faithfulness_threshold": 0.9}`），使 README 的 `cp -r examples/skills/compliance skills/` 恢复机制带回**完整**合规 eval 实验（含 gate 阈值文件）
- **Files modified:** `cmd/eval/main_test.go`、`internal/eval/frozen_test.go`、`internal/skill/loader_test.go`、`examples/skills/compliance/eval-gate.json`(新增)
- **Commit:** `4bda3ce`
- **边界说明:** internal/ 改动仅限 3 个测试文件的路径指向 + 注释，后端提示词/逻辑零改动；examples/skills/compliance 的 SKILL.md 模板本体未动（仅新增同目录 eval-gate.json）

## Known Impact（记录项，非代码改动）

- `cmd/eval` `firstManifest()` 此后取 skills/ 第一个 manifest = simple-qa。未来的 `--frozen=compliance` 复现实验需先 `cp -r examples/skills/compliance skills/ && rm -r skills/simple-qa`（README Quick Start 已写明该机制；迁入的 eval-gate.json 使 gate-check 同步可用）。这与未完成的 Plan 07-05 Task 2 相关。
- 路由默认路径仍为 VerdictPipeline（compliance 口径 system prompt，`USE_AGENT_PATH` unset 时）——internal/ 范围，本轮明确不改，只改呈现。
- `internal/server` 测试（chat_test.go / pipeline_default_test.go 注释提及 skills/compliance）在换默认后依然全绿（这些测试自建 fixture 目录，不依赖仓库默认）。

## Verification

- [x] `cd web && npx vitest run` — 233/233 全绿
- [x] `cd web && npx tsc --noEmit` — 0 错误
- [x] `ls skills/` → 仅 `simple-qa`；`diff examples/skills/simple-qa/SKILL.md skills/simple-qa/SKILL.md` 无输出
- [x] `go test ./...` 全绿（含 TestExamplesValidate + 三个 repoint 后的 shipped-artifact 测试）
- [x] `go build ./...` + `go vet ./...` 通过
- [x] grep 归因清单完整（上表 18 处全部 allowlist）
- [x] internal/ 提示词、eval/ 数据集、knowledge/ 种子、.planning/ 历史文档零改动（git diff --stat 169e8b4..HEAD 核实）
- [ ] chrome-devtools 实测（http://localhost:5173）由 orchestrator 复验（标题/空状态/建议问题/占位符/技能面板 simple-qa 已加载）——executor 不启动 live 栈

## Self-Check: PASSED

- skills/simple-qa/SKILL.md FOUND；examples/skills/compliance/eval-gate.json FOUND；260814-vga-SUMMARY.md FOUND
- Commits e42220e / 4bda3ce / 80b6c2d 均在 git log 中（worktree-agent-a3463c2b408e418c6 分支）
