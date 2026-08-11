---
sketch: 001
name: three-panel-layout
question: "三列布局视觉该是什么样？宽度比例 / 中间预览模式 / 两边折叠 UX"
winner: null
tags: [layout, three-panel, denova-style, agent-collaboration]
---

# Sketch 001: Three-Panel Layout

## Design Question

SkillForge 的三列工作 IDE 怎么呈现：
- **左 sidebar** (session + file tree)：宽度 / 内容组织 / 折叠
- **中 panel** (文档预览)：Markdown 渲染 vs Source vs Diff tab / 显示哪个文件
- **右 panel** (chat)：streaming 消息布局 / tool-call 展示 / reasoning chain / 引用 chip / Skill 选择器位置
- **两边 sidebar 折叠 UX**：收起后中间占满 / toolbar 显示折叠按钮

## How to View

open `.planning/sketches/001-three-panel-layout/index.html`

## Variants

- **A: Default widths (260 / flex / 420)** — 标准 IDE 宽度，左窄 + 中弹性 + 右中等
- **B: Compressed chat (260 / flex / 340)** — 右 chat 较窄（适合小屏或专注阅读）
- **C: Wide middle (300 / flex / 380)** — 中间预览优先（适合长文档查看场景）

## What to Look For

- 哪个 variant 的宽度比例最舒适？
- 中间文档预览的"预览/编辑/对比"三 tab 切换是否清晰？
- chat 中 tool call + reasoning + 引用溯源的视觉层次
- 两边 sidebar 折叠后中间占满的过渡
- Skill 选择器位置（顶部 vs 侧边）是否顺手
- verdict card（agent 输出）展示是否醒目