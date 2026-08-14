---
phase: quick-260814-vga
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - web/index.html
  - web/src/components/center/CenterPanel.tsx
  - web/src/components/layout/RightChat.tsx
  - web/src/components/chat/ChatInput.tsx
  - web/src/components/layout/SkillManagerMenu.tsx
  - web/src/__fixtures__/skills.json
  - web/src/__tests__/CenterPanel.test.tsx
  - skills/simple-qa/SKILL.md
  - README.md
autonomous: true
requirements: [QUICK-260814-VGA]
tags: [frontend, copywriting, de-compliance, simple-qa, default-skill]

must_haves:
  truths:
    - "打开应用（仓库默认状态）看到的标题、空状态、建议问题、占位符均为通用文档问答措辞，无合规主线叙事"
    - "右侧 3 个建议问题均能由 knowledge/authority/ 现有种子文档（年假/合同审批/差旅住宿）回答"
    - "仓库默认 skills/ 只含 simple-qa；cmd/server 启动扫描后 GET /api/skills 返回 simple-qa（UI 技能面板显示 simple-qa 已加载）"
    - "cd web && npx vitest run 全绿；npx tsc --noEmit 0 错误"
    - "web/ 面向用户文案 grep '合规' 仅剩 allowlist 命中（wire 状态枚举 / 代码注释 / wizard 分类学 / fixture 业务数据）"
  artifacts:
    - path: "web/index.html"
      provides: "通用文档问答页面标题"
    - path: "skills/simple-qa/SKILL.md"
      provides: "默认 Skill（字节等同 examples/skills/simple-qa/SKILL.md）"
    - path: "web/src/__fixtures__/skills.json"
      provides: "SkillMdPreview/SkillPicker 演示数据，无 compliance 条目"
  key_links:
    - from: "web/src/components/center/CenterPanel.tsx"
      to: "web/src/__fixtures__/skills.json"
      via: "hardcoded skillId 'simple-qa' → SkillMdPreview fixture entry"
      pattern: "skillId: 'simple-qa'"
    - from: "cmd/server/main.go"
      to: "skills/"
      via: "skill.NewLoader(\"skills/\") 启动扫描"
      pattern: "skills/simple-qa/SKILL.md"
---

<objective>
代码层去合规化：web/ 面向用户的 UI 文案从"企业合规审查"泛化为"通用文档问答"，
仓库默认 skills/ 目录内容从 compliance 换为 simple-qa，使应用开箱（chrome-devtools 打开
http://localhost:5173）呈现为通用文档问答平台。这是 260814-tsr（文档叙事去合规化）的代码侧收尾。

Purpose: 上一轮已把 README/CLAUDE.md 叙事改为平台优先，但运行中的应用仍是合规主线（标题、空状态、建议问题、默认 Skill 均为合规）。本轮让代码呈现与文档叙事一致。
Output: 泛化后的前端文案 + 测试同步、默认 simple-qa 的 skills/ 目录、README 最小一致性修正。
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/quick/260814-tsr-remove-compliance-wording/260814-tsr-SUMMARY.md

## 已锁定的范围决策（用户锁定，勿再议）

1. 前端 UI 文案泛化：web/ 面向用户的合规措辞改通用文档问答措辞；对应测试与必要 fixture 同步。verdict wire 协议中的状态枚举（'合规'/'不合规'/'需补充'）是 schema 不是文案，不改。
2. skills/ 默认内容从 compliance 换为 simple-qa（从 examples/skills/simple-qa 复制）；examples/skills/compliance 模板保留不动。
3. 明确排除：internal/ 后端提示词/文案、eval/golden + eval/frozen 的 compliance 数据集、knowledge/ 种子文档、.planning/ 历史文档均不动。README 仅做与默认 Skill 变化相关的最小修正。

## 代码事实（规划期已核实，执行时直接使用）

- `cmd/server/main.go:146,257` 只扫描 `skills/` 目录（`skill.NewLoader("skills/")` + `skill.New("skills/", nil)`）；Registry+Router 已接线（route-then-run，chat.go:216），单 Skill 场景路由无歧义 → skills/ 只含 simple-qa 时 /api/chat 与 GET /api/skills 正常工作。
- `cmd/eval/main.go:600` `firstManifest()` 取 skills/ 第一个 manifest，name 无关 → 换默认 Skill 后 cmd/eval 机械上仍可跑（judge 口径不变，但被评 system prompt 变为 simple-qa 的；这与未完成的 Plan 07-05 Task 2 相关，仅需在 SUMMARY 记录，不改代码）。
- 前端两处硬编码 `skillId: 'compliance-review'`：`web/src/components/center/CenterPanel.tsx:80`（skill-md tab 点击）与 `web/src/components/layout/SkillManagerMenu.tsx:50`。SkillMdPreview 与 SkillPicker 均读静态 `web/src/__fixtures__/skills.json`（非 live 数据），其中含 compliance-review 条目 → 需替换为 simple-qa 条目并改两处硬编码 id，否则换默认 Skill 后这两个用户可见表面仍显示"企业合规/制度审查"。
- 路由默认路径为 VerdictPipeline（compliance 口径 system prompt，`USE_AGENT_PATH` unset 时）。这是 internal/ 范围，明确不改 —— 本轮只改"呈现"，不改后端行为。
- `internal/skill/examples_validate_test.go` 已驱动 `skill.NewLoader("../../examples/skills").Scan()` 校验 3 个模板 → simple-qa 的 SKILL.md 必然通过 loader 校验。

## "合规" 命中分类规则（grep 验收的 allowlist）

**改（面向用户文案）**：
- `web/index.html:10` 页面标题
- `CenterPanel.tsx:115` tab "审查结论"、`:151` "等待合规审查结果"、`:154` "右侧提出合规问题…"
- `RightChat.tsx:31-35` 3 个建议问题、`:107` "开始一次合规审查"、`:109` 副文案（"每个结论"→"每个回答"）
- `ChatInput.tsx:241` 占位符 "输入合规问题…"
- `__fixtures__/skills.json` compliance-review 条目（经 SkillMdPreview/SkillPicker 两个用户可见表面渲染）

**不改（allowlist，执行时按此核对）**：
- `web/src/types/verdict.ts:22` `VERDICT_STATUSES`（wire 状态枚举，BE 发什么 FE 显示什么）及 StatusBadge/VerdictCard/MessageBubble/RedlineCard/InspectorRetrieval 中以这些枚举值为 key 的映射与代码注释
- `web/src/styles/theme.css` 颜色变量注释（`--color-status-compliant` 等）
- `web/src/types/wizard.ts:65,75` wizard 分类学（"合规审查类"是三分类中的一个选项，对应三个示例模板，非主线叙事）
- `web/src/__fixtures__/{multi-part,verdict-*,refusal-*}.json`（wire fixture 业务数据，仅 dev/test MockChatTransport 使用）
- `__tests__/` 中作为业务数据的 skill 名称/描述/status 断言（ReloadToast/RoutingTracePart/LeftSidebar/useWizard/WizardMode/transport-swap/MessageBubble/VerdictCard/AuditTable/SkillsTable 等）
- `eval/__fixtures__/*.json`（eval 数据，明确排除）

## 建议问题替换依据（已核实种子内容）

knowledge/authority/ 现有可答种子：annual_leave（年假结转上限 5 天 + 300% 折算）、contract_approval（50 万以下部门负责人 / 50-200 万分管副总 / 200 万以上总经理）、travel_expense（住宿超标 200 元内部门负责人 / 超出部分分管副总）、business_gift_prohibition、non_compete、expense_reimbursement_scope、anti_corruption_clause。建议问题必须落在这个可答集合内。
</context>

<tasks>

<task type="auto">
  <name>Task 1: 前端 UI 文案泛化 + fixture/测试同步</name>
  <files>web/index.html, web/src/components/center/CenterPanel.tsx, web/src/components/layout/RightChat.tsx, web/src/components/chat/ChatInput.tsx, web/src/components/layout/SkillManagerMenu.tsx, web/src/__fixtures__/skills.json, web/src/__tests__/CenterPanel.test.tsx</files>
  <action>
按下方逐处修改（保持中文语气与现有 UI 一致；同步更新因文案改动而失真的代码注释）：

1. `web/index.html:10` 标题 → `SkillForge — Skill 驱动的文档问答平台`。
2. `CenterPanel.tsx`：
   - `:115` tab label `审查结论` → `结论`（与 引用原文/Skill 定义/Agent 状态/知识图谱 的 2-4 字风格一致）。
   - `:151` `等待合规审查结果` → `等待回答结果`；`:154` `右侧提出合规问题，agent 推理与引用会显示在这里。` → `右侧提出问题，agent 推理与引用会显示在这里。`；同步更新 `:15-16` 引用旧文案的注释。
   - `:80` 硬编码 `{ kind: 'skill-md', skillId: 'compliance-review' }` → `skillId: 'simple-qa'`。
3. `RightChat.tsx`：
   - `:31-35` SUGGESTED_PROMPTS 换为（已核实可由 knowledge/authority 种子回答）：`年假未休完的部分如何折算或结转？` / `采购合同 50 万元以下由谁审批？` / `出差住宿费超过标准额度时如何审批？`。执行时如调整措辞，必须保持每题都能由上述种子文档直接回答。
   - `:107` `开始一次合规审查` → `开始一次文档问答`；`:109` 副文案 → `选择 Skill，在此处提问。每个回答都附可点击的引用与置信度。`（"结论"→"回答"，其余保留）。
   - 可选（自由裁量）：`:24` `ShieldCheck` 图标换 `BookOpen`（lucide-react，同 import 路径），弱化"合规护盾"视觉暗示；`data-testid` 不变。
4. `ChatInput.tsx:241` 占位符 → `输入问题，或 @ 触发 file/skill，或拖放文件...`（保留 @ 与拖放提示结构）。
5. `SkillManagerMenu.tsx:50` 硬编码 `skillId: 'compliance-review'` → `skillId: 'simple-qa'`。
6. `web/src/__fixtures__/skills.json`：删除 `compliance-review` 条目，新增 `simple-qa` 条目（字段与真实 `examples/skills/simple-qa/SKILL.md` 对应：id `simple-qa`，name `通用问答`，description 取该 SKILL.md frontmatter description 的中文首句（通用问答 / general Q&A…），agent `ChatModelAgent`，reads `["权威"]`，writes `[]`，temperature `0.3`，steps 按 SKILL.md steps 三步中文转写，guardrails.prohibitions 三条直译，required_refusal_phrases `["no_relevant_source"]`）。保留 `general-qa` 条目不动。
7. 测试同步：`CenterPanel.test.tsx` `:44` 断言改新空状态文案；`:80` 断言 `企业合规/制度审查` 改 `通用问答`（新 fixture 条目）。其余测试文件跑 vitest 后按失败最小修复（预期仅此两处 + 可能的 SkillPicker 间接依赖；状态枚举/业务数据断言一律不动）。

禁止：不改 `types/verdict.ts` 的 VERDICT_STATUSES；不改 wizard.ts 分类学；不改 `__fixtures__/{multi-part,verdict-*,refusal-*}.json`；不改任何 data-testid（避免连锁断言失效）。
  </action>
  <verify>
    <automated>cd /d/CODE/ClaudeCode/ai-agent/web && npx vitest run 2>&1 | tail -5 && npx tsc --noEmit && echo TSC_OK</automated>
  </verify>
  <done>vitest 全绿（基线 209/209，含更新后的 CenterPanel 断言）；tsc 0 错误；grep 复核：`grep -rn "合规\|审查" web/index.html web/src/components/layout/RightChat.tsx web/src/components/chat/ChatInput.tsx web/src/components/center/CenterPanel.tsx | grep -v "^\s*//" ` 中用户可见字符串层仅剩 allowlist 之外的零命中（CenterPanel 注释里指向 wire 状态的可保留）。</done>
</task>

<task type="auto">
  <name>Task 2: skills/ 默认 Skill 换 simple-qa</name>
  <files>skills/simple-qa/SKILL.md</files>
  <action>
1. `git rm -r skills/compliance`（删除仓库内 compliance 默认 Skill；examples/skills/compliance 模板保留不动）。
2. 复制 `examples/skills/simple-qa/SKILL.md` → `skills/simple-qa/SKILL.md`（字节等同，含 Apache-2.0 头若有）。
3. 不触碰 `skillforge.db*` 等运行态文件（已 gitignore，且 wizard 热写对象是 skills/ 运行时目录而非本轮目标）。
4. 校验装载合法性：跑 `go test ./internal/skill/ -run TestExamplesValidate`（证明该 SKILL.md 通过 loader+Validate，因字节等同）+ `go build ./...` + `go vet ./cmd/...`。不要尝试启动 cmd/server（需 live Milvus/BGE/GLM 环境，非本轮验收手段）。
5. SUMMARY 中记录已知影响：cmd/eval `firstManifest()` 此后将取 simple-qa —— 未来的 `--frozen=compliance` 复现实验若需合规口径，需临时 `cp -r examples/skills/compliance skills/`（与 README Quick Start 的模板替换步骤同一机制）。这是记录项，不是代码改动。
  </action>
  <verify>
    <automated>cd /d/CODE/ClaudeCode/ai-agent && ls skills/ && diff examples/skills/simple-qa/SKILL.md skills/simple-qa/SKILL.md && test ! -d skills/compliance && go test ./internal/skill/ -run TestExamplesValidate && go build ./... && echo SWAP_OK</automated>
  </verify>
  <done>skills/ 目录仅含 simple-qa/（SKILL.md 与 examples 字节等同）；examples/skills/compliance 完整保留；Go 全仓 build 通过；TestExamplesValidate 通过。</done>
</task>

<task type="auto">
  <name>Task 3: README 最小一致性修正 + 全仓文案 grep 验收</name>
  <files>README.md</files>
  <action>
仅做与"默认 Skill 已是 simple-qa"相关的最小修正（上一轮 260814-tsr 的其余叙事不动）：

1. `README.md:100`（Architecture）：文件路径例证 `skills/compliance/SKILL.md`（两处链接）→ `skills/simple-qa/SKILL.md`；同段 "30-line Markdown file" 措辞若因文件不同失真可顺带核对（simple-qa 同为紧凑模板，预计无需改）。
2. `README.md:127`（Loaded Skills）：`the repo default is skills/compliance/SKILL.md` → `the repo default is skills/simple-qa/SKILL.md`。
3. Quick Start（L46-49 附近）的 `cp -r examples/skills/simple-qa skills/` 步骤：默认目录已是 simple-qa，该步骤变为冗余 —— 将其改写为"换领域模板"示例（如：默认开箱即 simple-qa；想换成 compliance 演示可 `cp -r examples/skills/compliance skills/ && rm -r skills/simple-qa`，重启或热重载生效），保持步骤总数与缩进结构，最小改动。
4. Try-it 段（L58-66）核对：主演示已是 simple-qa + 差旅住宿问题，与新默认一致，预计无需改；仅当出现与默认 Skill 矛盾的表述时做最小修正。
5. 全仓验收 grep（分类规则见 context）：
   - `grep -rn "合规" web/ --include="*.tsx" --include="*.ts" --include="*.html" | grep -v __tests__ | grep -v __fixtures__` → 逐条归入 allowlist（verdict.ts 枚举 / 组件内以枚举值为 key 的映射与注释 / wizard.ts 分类学），不得有面向用户的活动文案。
   - `grep -n "skills/compliance" README.md` → 0 命中（examples/skills/compliance 作为模板引用除外，若 Quick Start 改写引入则允许该形式）。
   - `cd web && npx vitest run` 终态复核全绿。
  </action>
  <verify>
    <automated>cd /d/CODE/ClaudeCode/ai-agent && grep -rn "合规" web/ --include="*.tsx" --include="*.html" | grep -v __tests__ | grep -v __fixtures__ | grep -v "^\s*//" ; grep -c "skills/simple-qa" README.md</automated>
  </verify>
  <done>README 无 "repo default is compliance" 类陈述；Quick Start 与新默认一致；grep 输出中每一处 "合规" 均可归入 allowlist 类别（在 SUMMARY 中逐条列出归因）；vitest 全绿。</done>
</task>

</tasks>

<verification>
- `cd web && npx vitest run` 全绿；`npx tsc --noEmit` 0 错误。
- `ls skills/` → 仅 `simple-qa`；`diff examples/skills/simple-qa/SKILL.md skills/simple-qa/SKILL.md` 无输出。
- `go build ./...` 通过。
- web/ 用户可见层无合规叙事（分类规则 + allowlist 见 context；SUMMARY 逐条归因）。
- chrome-devtools 实测（http://localhost:5173）由 orchestrator 复验：标题、空状态、建议问题、占位符、技能面板（simple-qa 已加载）——executor 不负责启动 live 栈。
</verification>

<success_criteria>
1. 开箱 UI（标题/空状态/建议问题/占位符/tab 名）全部为通用文档问答措辞。
2. 3 个建议问题均能由 knowledge/authority 现有种子回答（规划期已核实三题对应 annual_leave / contract_approval / travel_expense）。
3. skills/ 默认 = simple-qa（字节等同 examples 模板）；examples/skills/compliance 保留。
4. vitest 全绿 + tsc 0 错误 + go build 通过 + grep 归因清单完整。
5. internal/、eval/、knowledge/、.planning/ 零改动（git diff 验证）。
</success_criteria>

<output>
Create `.planning/quick/260814-vga-ui-skill-simple-qa/260814-vga-SUMMARY.md` when done
</output>
