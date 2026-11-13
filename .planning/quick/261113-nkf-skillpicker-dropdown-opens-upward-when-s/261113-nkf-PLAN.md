---
status: ready_for_execution
task: 261113-nkf
created: 2026-11-13
---

# 261113-nkf PLAN — SkillPicker 下拉方向自适应（空间不足时向上展开）

## 背景（勘察已锁定）

用户截图：composer 头部（左上角）的 Skill 下拉，展开后**下面的选项被裁掉看不见**。

- `web/src/components/chat/parts/SkillPicker.tsx:123-126`：菜单是 `absolute z-10 mt-1 w-full min-w-56 ...`，**永远向下**展开（`mt-1` = margin-top）。
- 261113-lhu 把 SkillPicker 从 TopBar 移到 composer 头部后，触发器位于输入框**上方**——向下展开要穿过输入框区域，且 composer 卡片有 overflow 约束，超出部分被裁掉。
- 参照实现：同项目的 composer 斜杠菜单（`theme.css:1440` `.composer-skill-menu`）用的就是 `bottom: calc(100% + 8px)` 向上展开。

## Task 1 — SkillPicker 菜单方向自适应

**files:** `web/src/components/chat/parts/SkillPicker.tsx`

**action:**
1. 加 `const [dropUp, setDropUp] = useState(false)` 与一个 menu ref。
2. 打开菜单时（`setOpen(true)` 之后）用 `useLayoutEffect` 测量：触发器按钮的 `getBoundingClientRect().bottom` 与 `window.innerHeight` 的差 = 下方可用空间；菜单自身高度用 `menuRef.current.scrollHeight`（或估算 `Math.min(skills.length * 34 + 40, 320)`）。若**下方可用空间 < 菜单高度**且**上方空间（rect.top）更大**，则 `setDropUp(true)`。
3. 菜单 className 按 `dropUp` 切换：`dropUp ? 'bottom-[calc(100%+4px)]' : 'mt-1'`（用 Tailwind 任意值；底部定位时去掉 `mt-1` 改用 `mb-1`）。
4. 菜单加 `max-h-[min(320px,60vh)] overflow-y-auto`，长列表内部滚动而不是溢出被裁。
5. 每次 open 状态变化重置 `dropUp=false`（菜单关闭时）。

**注意**：布局测量必须在菜单已渲染后进行 —— 用 `useLayoutEffect` 依赖 `[open]`。

## Task 2 — 回归测试

**files:** `web/src/__tests__/SkillPicker.test.tsx`

**action:** 新增一条测试：mock `window.innerHeight` 与 trigger 的 `getBoundingClientRect`（jsdom 默认返回全 0，需要 stub），构造「下方空间不足、上方空间充足」的场景，断言菜单带向上定位的 class / `data-placement="top"`。

推荐实现：给菜单加 `data-placement={dropUp ? 'top' : 'bottom'}` 属性（便于测试断言，也便于将来调试），测试 stub `Element.prototype.getBoundingClientRect` 与 `window.innerHeight`。

**verify:**
- `cd web && npx vitest run src/__tests__/SkillPicker.test.tsx src/__tests__/ChatInput.test.tsx src/__tests__/RoutingTracePart.test.tsx` exit 0
- `npm run typecheck` exit 0；`npm run lint` exit 0

## 收口

- 单 commit，message 简体中文 conventional 风格
- 只改 SkillPicker.tsx + 其测试 + 规划文档

## 备注（不在本任务范围）

- TopBar 里的 SkillManagerMenu 也有类似方向问题（TopBar 顶部按钮向下展开通常够用），本轮不动。
- composer 斜杠菜单已是向上展开，符合预期。
