---
status: investigating
trigger: "chat 太宽了，请锁定最大宽度。关闭左边和右边边栏时 chat 变得过宽，不利于对话。"
created: 2026-08-19
updated: 2026-08-19
mode: find_root_cause_only (diagnostic; gsd-debug will own the fix)
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: investigating
---

## Current Focus

hypothesis: "当前 .chat-col 使用 `flex: 1` 没有 max-width 约束；两侧 panel 都是 `flex: none` 因此关闭边栏后 chat-col 会撑满整个 viewport，没有居中也没有最大宽度上限。"
test: static CSS read + 尝试 vs 已知最佳实践 (768-900px 阅读栏宽上限 45-75 字符)
expecting: 找到 `.chat-col` 没有 max-width 的 CSS 行作为根因证据；并给出三层修复方案 (chat-col / message-assistant / composer)
next_action: 已收集代码证据完成；输出 ROOT CAUSE FOUND 报告，等待 orchestrator 协调 gsd-debug 应用修复

## Symptoms

expected:
- chat 在超宽屏（>1440px）上保持一个可读的固定最大宽度（如 768-900px），而不是 100% 撑满
- 关闭左侧 SessionDrawer 和右侧 DetailPanel 后，chat 仍保持合理的宽度 + 居中
- 用户消息气泡（已 max-width: 75%）的视觉密度与 chat 主体保持一致

actual:
- chat 容器 `.chat-col` 是 `flex: 1`，无 `max-width` 约束
- 在 1920px 显示屏上：两侧 panel 全关 → chat-col 横跨 1920px - topbar (无 padding) ≈ 完全撑满
- assistant 消息 (`.message-assistant`) 没有内层 max-width，文本直接顶到 viewport 边缘
- composer (`.composer-modern`) 也没有居中容器，textarea 撑满整个 chat-col

errors:
- 无报错，纯视觉/排版问题

reproduction:
1. 启动 dev server（vite + hertz backend）
2. 打开浏览器到 / （localhost:5173 → 代理到 :8888/api/chat）
3. 进入 chat 三栏布局（live app path）
4. 关闭 SessionDrawer（左 panel）
5. 关闭 DetailPanel（右 panel）
6. 观察 chat 区域：从 ~620px 一路涨到 1920px（取决于显示器分辨率），文字一行动辄 200+ 字符

started:
- 用户报告时间：2026-08-19
- 第一次在最新 commit (60e497b "session panel as inline left column") 后出现 ≈ 因为 chat-first shell 替代了原来的 5-track grid；原来 .app-main 的三栏 grid 自然限制了中间列宽度，新 .chat-body 的 flex 1fr 没有保留该约束

## Eliminated

(None yet — only one dominant hypothesis and it's well supported by the CSS; no competing hypotheses needed)

## Evidence

- timestamp: 2026-08-19
  checked: D:\code\ClaudeCode\ai-agent\web\src\components\layout\ChatLayout.tsx (260 lines)
  found: 整个 layout 是一个 `.app-shell` grid (56px topbar / 1fr chat-body / 26px statusbar)；`.chat-body` 是横向 flex (session-panel-wrap + chat-col + detail-panel-wrap)。session-panel-wrap 默认 280px (220-360 clamp)，detail-panel-wrap 默认 380px (320-520 clamp)。两个 wrap 都是 `flex: none` + 显式 width。
  implication: 两个侧栏不打开时它们不占位；chat-col 拿全部剩余空间

- timestamp: 2026-08-19
  checked: D:\code\ClaudeCode\ai-agent\web\src\styles\theme.css:1463-1468 (`.chat-col` rule)
  found:
    ```
    .chat-col {
      flex: 1;
      min-width: 0;
      height: 100%;
      overflow: hidden;
    }
    ```
  implication: 没有 `max-width`、`margin: 0 auto`、也没有内部约束容器。`min-width: 0` 只防止 flex 子项被内容撑大，不限制外伸

- timestamp: 2026-08-19
  checked: D:\code\ClaudeCode\ai-agent\web\src\styles\theme.css:1457-1461 (`.chat-body`)
  found: `display: flex; flex: 1; min-height: 0;` — 没有 `justify-content: center`、`gap` 或内层 wrapper
  implication: flex 容器本身不居中；chat-col 也没有自我居中手段

- timestamp: 2026-08-19
  checked: D:\code\ClaudeCode\ai-agent\web\src\components\layout\ChatColumn.tsx:64-65
  found: `<div data-testid="chat-column" className="flex h-full flex-col">` — ChatColumn 是 chat-col 的子，自身没有内层 max-width 包装
  implication: 这是第二个可注入 max-width 的层级

- timestamp: 2026-08-19
  checked: D:\code\ClaudeCode\ai-agent\web\src\styles\theme.css:376-407 (`.message-assistant` + `.message-assistant-body`)
  found:
    - `.message-assistant { display: flex; flex-direction: row; gap: var(--space-3); margin: var(--space-5) 0; padding: 0; }`
    - `.message-assistant-body { flex: 1; min-width: 0; }`
  implication: 整条 assistant 消息没有 max-width；30px 方标 + body 横跨整个 chat-col。`.message-user-card` 有 max-width: 75%（个人气泡 OK），但消息外层无。

- timestamp: 2026-08-19
  checked: D:\code\ClaudeCode\ai-agent\web\src\styles\theme.css:907-910 (`.composer-modern`)
  found: `padding: var(--space-3) var(--space-4) var(--space-4); background: transparent;` — 跟 chat-col 同宽，无居中
  implication: 输入框也会跟随 chat-col 撑满

- timestamp: 2026-08-19
  checked: D:\code\ClaudeCode\ai-agent\web\src\styles\theme.css:1191-1247 (`.chat-empty-modern`、`.chat-empty-prompts`)
  found:
    - `.chat-empty-modern { padding: 48px 24px; ... }` — 空状态的内层 max-width: 440px 局限于 suggested prompts；icon + title + subtitle 仍可以拉到 chat-col 的全宽
    - `.chat-empty-prompts { width: 100%; max-width: 440px; }`
  implication: 即便是设计良好的空状态也只对 prompts 设了上限；hero region（icon + title + subtitle）没有

- timestamp: 2026-08-19
  checked: D:\code\ClaudeCode\ai-agent\web\src\styles\theme.css:1541-1549 (`@media (max-width: 767px)` 移动端 fallback)
  found: 移动端把 `.message-user-card` 和 `.verdict-inline` 改成 `max-width: 100%`。但没有对应的"宽屏"分支
  implication: 已有的 CSS 模式是"窄屏拆上限"；缺少对应"宽屏封上限"的镜像分支

## Resolution

root_cause: |
  `.chat-col` 在 CSS (theme.css:1463) 使用 `flex: 1` 但没有任何 `max-width` 或
  居中容器。其父 `.chat-body` 是横向 flex，没有 `justify-content: center`。
  结果：当左右 panel (`.session-panel-wrap`、`.detail-panel-wrap`) 都关闭时，
  `chat-col` 撑满整个 viewport (典型 1920px+)。Assistant 消息 (`.message-assistant`)
  与 composer (`.composer-modern`) 都没有内层 max-width 容器，所以文本行宽可超过
  200 字符（远超阅读舒适度的 45-75 字符上限）。

  注：旧的 `.app-main` 5-track grid 隐式限制了中间列宽度；chat-first redesign (B2)
  之后切到 flex 但没有把"固定中间列"的语义带过来。

fix: (留给 gsd-debug 应用；这是诊断报告，不在本次会话执行)
  推荐方案 B（详见下方的 Root Cause Report）

verification: (待 fix 后由人工 + 自动化共同验证)
  1. 1920×1080 viewport：chat 居中显示，最大宽度 ~840px（视觉密度与 ChatGPT/Claude.ai 对齐）
  2. 关闭两侧 panel：chat 区域继续保持居中 + max-width
  3. 打开右侧 panel (380px)：chat 自动压缩到 ~840px 但仍居中
  4. <1280px：行为不变（panel 退化为 fixed overlay，chat 已 100% width）
  5. 既有 vitest 快照（如有 message-assistant 截图）不破

files_changed: []
