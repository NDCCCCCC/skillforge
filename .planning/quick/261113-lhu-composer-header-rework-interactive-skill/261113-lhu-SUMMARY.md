---
status: blocked
task: 261113-lhu
completed: 2026-11-13
block_reason: verify 命令（npx vitest / npm run）被会话 Bash 权限拒绝（don't-ask 模式），测试与 typecheck 未执行；文件编辑已全部完成
---

# 261113-lhu SUMMARY — composer 头部交互改造

## 改动摘要（5 文件）

### Task 1 — `web/src/components/chat/ChatInput.tsx`
- 非 modeLabel 分支头部行：左侧只读徽章（`composer-skill-badge` + `未选择 Skill · 输入 / 选择` 提示）**整体替换**为 `<SkillPicker />`，外包 `<div className="w-44 flex-none">`（与 TopBar 原容器一致）。
- `chat-input-current-skill` testid 挂到 w-44 外层容器上（计划 Task 4 优先项：保 testid 稳定）；wizard modeLabel 分支的同名 testid 不动。
- 删除无引用的 `const currentSkill = skills.find(...)`；`currentSkillId`/`skills` 保留（stale-skill effect + 斜杠菜单仍用）。
- 新增 `import SkillPicker from './parts/SkillPicker'`。
- 斜杠菜单逻辑零改动（独立浮层，与 SkillPicker 下拉不冲突）。
- 头部注释与 `modeLabel` props 注释同步更新（移除"READ-ONLY pill badge / selection exclusively through slash menu"过期表述）。

### Task 2 — `web/src/components/layout/TopBar.tsx` + `web/src/__tests__/TopBar.test.tsx`
- TopBar：删除 `w-44` SkillPicker 容器、`SkillPicker` import；头部注释改为"Skill switcher → SkillPicker in the composer (261113-lhu)"。SessionPicker 容器（`hidden w-44 md:block`）不动。
- TopBar.test：第一个用例改名 `renders the SessionPicker trigger and no longer hosts SkillPicker`，断言 `queryByTestId('skill-picker-trigger')).not.toBeInTheDocument()`（钉住"不回流"），注释注明入口已下沉 composer（261113-lhu）；beforeEach 注释同步（skills seeding 保留，无害）。

### Task 3 — `web/src/components/chat/ModelPicker.tsx`
- 非列表兜底分支 `<span>` → `<button type="button" data-testid="model-picker-set" aria-label="设置模型">`。
- onClick：`window.prompt('输入模型名（如 glm-5.3-flash）后回车切换：')` → 非空 trim 后 `void switchModel(next.trim())`。
- title 组合：有 model → `点击手动输入模型名切换`；无 model → 原 admin 指引 + `或点击手动输入模型名切换`。
- 文案兜底保留：`model || (mode === 'stub' ? 'stub' : '未设置模型')`。
- 样式与 badge 协调：`bg-transparent text-inherit outline-none cursor-pointer hover:opacity-80`（与 select 分支同类）+ `style={{ font: 'inherit' }}`（button 不默认继承字体，对齐 badge 11px）。

### Task 4 — `web/src/__tests__/ChatInput.test.tsx`
- 波及面勘察：`chat-input-current-skill` 全仓只有 ChatInput.tsx + ChatInput.test.tsx 引用（无 wire-contract/E2E 依赖）；`model-picker*` testid 无任何测试引用；RoutingTracePart.test 直接渲染 SkillPicker 歧义模式不受影响；App.test 仅注释提及 TopBar。
- 三处断言同步：
  1. 用例 1：改为断言 `skill-picker-trigger` 存在 + wrapper textContent 含 `选择 Skill`（原 `未选择 Skill`）。
  2. 用例 3（Enter 选中后）：断言 `chat-input-current-skill` 含 `Calculator` 原样保留（wrapper testid 下 trigger 显示 current.name，天然通过），注释改为 Picker trigger。
  3. WR-02 用例：`S1` 断言原样保留；skills 清空后断言 `选择 Skill`（原 `未选择 Skill`）。

## 验证结果（阻塞说明）

**未执行 — Bash 权限拒绝**（don't-ask 模式拒绝 `npx vitest run` 与 `npm run typecheck`，共两次尝试后按执行纪律停止）。已用静态复查替代：5 个文件的改动区域全部重读确认 JSX/TS 语法正确、import 无残留、testid 断言与新结构一致。

编排方验证清单（计划 verify 原文）：
```bash
cd web && npx vitest run src/__tests__/ChatInput.test.tsx src/__tests__/TopBar.test.tsx src/__tests__/SkillPicker.test.tsx   # exit 0
npm run typecheck   # exit 0
# 波及面判断：App.test/ChatLayout.test/wire-contract 无断言波及（已勘察），
# 三文件通过即可；若编排方谨慎可跑全量 npx vitest run
```

预期风险点（供验证时关注）：
- ChatInput.test 用例 1/WR-02 依赖 SkillPicker trigger 文案 `选择 Skill`（SkillPicker.tsx:94 `current?.name ?? '选择 Skill'`）——行为已确认。
- ChatInput 渲染 SkillPicker 后，`approvalPending` selector（optional chaining）在测试 store 下安全。
- ModelPicker button `style={{ font: 'inherit' }}` 仅视觉，无逻辑风险。

## 偏差记录

1. **计划行号漂移**：Task 3 说":227-229"，实际 span 分支在 ModelPicker.tsx:236-244；Task 2 说":63-65/:71"，实际容器在 TopBar.tsx:70-72。内容指向唯一，按内容执行。
2. **testid 策略**：`chat-input-current-skill` 保留在 SkillPicker 外层 w-44 容器（计划 Task 4"优先保 testid 稳定"），而非全部改断言——三处断言中两处零改动。
3. **额外注释同步**（超出计划字面但属同一改动语义）：ChatInput 头部/props 注释、TopBar 头部注释、两测试文件头部注释、ModelPicker 分支注释——均为描述已改行为的过期注释，按仓库"注释解释决策"惯例同步。
4. **未提交 commit**：按编排方指令（验证后统一提交）。建议 message：`feat(web): composer Skill 切换入口下沉 + ModelPicker 空模型可手动设置`。
