---
status: ready_for_execution
task: 261113-jau
created: 2026-11-13
---

# 261113-jau PLAN — TopBar 会话入口重排 + 即显 tooltip（合并用户两轮反馈）

## 背景（勘察已锁定，勿重复勘察）

- `web/src/components/layout/TopBar.tsx`：右上角按钮组（:70-121）含「会话列表」按钮（MessagesSquare，:75-85，`data-testid="topbar-sessions"`，toggle `sessionPanelOpen`），但面板实际是 ChatLayout 的**左侧** `session-panel-wrap` —— 按钮在右上、面板出在左，用户觉得别扭。
- 用户决策（locked）：
  1. **会话列表按钮挪到 TopBar 左区**（SessionPicker 下拉之后，同左侧面板对齐）；
  2. **右上角原位置换成「新建会话」按钮**（直接 createSession，比藏在面板里顺手）；
  3. **右上角所有图标按钮加即显 hover 提示**（原生 title 延迟约 1s 且弱，用户感知为"没有提示"）。
- tooltip 先例：`web/src/components/chat/parts/DataReferenceChip.tsx:13` CSS-only hover tooltip（项目 UI-SPEC 约束"零新增 shadcn block"）。
- store 动作：`useAppStore.getState().createSession()`（SessionDrawer:158 同款用法）。

## Task 1 — TopBar 重排（唯一改动文件）

**files:** `web/src/components/layout/TopBar.tsx`

**action:**
1. 左区（:66-68 SessionPicker 的 `hidden w-44 md:block` div 之后）新增「会话列表」按钮：图标 `MessagesSquare`，`data-testid="topbar-sessions"` 保留（测试锚点不换），`onClick={() => setSessionPanelOpen(!sessionPanelOpen)}`、`aria-pressed={sessionPanelOpen}`、`title`/`aria-label`="会话列表"，样式与现有图标按钮一致（`inline-flex h-8 w-8 ... hover:bg-surface-2`）。给按钮加 `data-tip="会话列表"`。
2. 右区（:75-85 原位置）替换为「新建会话」按钮：图标 `Plus`，`data-testid="topbar-new-session"`（新锚点），`onClick={() => createSession()}`（store hook：`const createSession = useAppStore((s) => s.createSession)`），`data-tip="新建会话"`，title/aria-label="新建会话"。
3. 右区其余四个图标按钮（知识库/评估/详情面板/主题）各加 `data-tip="<现有 title 文案>"`（知识库、评估、展开详情面板/收起详情面板、主题切换——ThemeToggle 若是独立组件则本轮只给它外层不动的部分加，若组件内部有按钮，可传入或跳过并在 SUMMARY 注明）。原 `title` 属性保留（无障碍兜底）。
4. SkillManagerMenu 的触发按钮若支持透传也加 `data-tip="Skill 管理"`；不支持则跳过并记录。

## Task 2 — CSS tooltip 样式

**files:** `web/src/styles/theme.css`

**action:** 追加（位置：TopBar 相关样式附近或文件通用段）：

```css
/* 261113-jau — topbar 即显 hover 提示（原生 title 延迟 ~1s 且不可样式化）。
   CSS-only：[data-tip] 悬停时在按钮下方弹出小黑片。z-index 高于 topbar。 */
[data-tip] { position: relative; }
[data-tip]:hover::after {
  content: attr(data-tip);
  position: absolute;
  top: calc(100% + 6px);
  left: 50%;
  transform: translateX(-50%);
  padding: 4px 8px;
  border-radius: 6px;
  background: var(--color-surface-3, #333);
  color: var(--color-text, #fff);
  font-size: 11px;
  line-height: 1.4;
  white-space: nowrap;
  pointer-events: none;
  z-index: 60;
}
```
（变量名先 grep theme.css 确认存在性，不存在用现有 token 替换。）

**verify:**
- `cd web && npx vitest run src/__tests__/TopBar.test.tsx` exit 0（若测试断言 `topbar-sessions` 在右区或断言新建行为需同步最小修改，新锚点 `topbar-new-session` 补一条"点击调 createSession"断言——参考测试文件现有 mock 手法）
- `npm run typecheck` exit 0

## 收口

- 单 commit，message 简体中文 conventional 风格
- 不碰 ChatLayout/SessionDrawer（面板定位不动）
- backlog 注记：SkillPicker/SessionPicker 下拉触发器（非图标按钮）本轮不加 data-tip
