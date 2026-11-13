---
status: ready_for_execution
task: 261113-luu
created: 2026-11-13
---

# 261113-luu PLAN — composer 发送按钮视觉重设计

## 背景（勘察已锁定）

`web/src/styles/theme.css:1271-1302` 的 `.composer-send`：
- 正圆（`border-radius: var(--radius-full)`）40×40 实心块
- 禁用态 `background: var(--color-surface-3)` + `color: var(--color-text-dim)` → **一坨实心灰圆**（截图里那颗突兀的"灰月亮"），视觉重量压过输入框
- 无 focus-visible 环；图标 16px 偏小

设计 token 已核实存在：`--radius-md/lg/composer/full`、`--color-primary/-hover/-fg`、`--color-glow/-strong`、`--color-surface-3`、`--color-text-dim`、`--color-border-muted`、`--transition-fast`。

## 任务（单文件 CSS 改动）

**files:** `web/src/styles/theme.css`

**action:** 重写 `.composer-send` 及其 hover/active/disabled 规则：

1. **形状**：正圆 → 圆角方形 `border-radius: var(--radius-md)`（与 composer-card 的 16px 圆角语言协调，不再像一颗孤立的圆球）
2. **尺寸/位置**：40×40 → 38×38；`right/bottom: 12px` → `10px`（贴紧但留呼吸）
3. **启用态**：`linear-gradient(135deg, var(--color-primary), var(--color-primary-hover))` + 内高光 `inset 0 1px 0 rgba(255,255,255,.14)` + `0 2px 10px var(--color-glow)`
4. **图标**：`.composer-send > svg { width:18px; height:18px; stroke-width:2.25 }`（不改 TSX）
5. **hover**：`translateY(-1px) scale(1.04)` + 强光晕；**active**：`scale(.98)` 回弹
6. **focus-visible**：`outline: 2px solid var(--color-primary-hover); outline-offset: 2px`
7. **禁用态**：改为**幽灵态** —— `background: transparent; color: var(--color-text-dim); border: 1px solid var(--color-border-muted); box-shadow: none; opacity:.55`（不再实心灰块）

**verify:**
- `cd web && npx vitest run src/__tests__/ChatInput.test.tsx` exit 0
- `npm run typecheck` exit 0
- `npm run lint` exit 0（CSS 不参与 eslint，但确认无连带）

## 收口

- 单 commit（CSS + 规划文档），message 简体中文
- 不动 TSX、不动后端
- backlog：图标若想换更饱满的形态（当前 ArrowUp / Square 保持），留后续
