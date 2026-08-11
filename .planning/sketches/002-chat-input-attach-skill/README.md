---
sketch: 002
name: chat-input-attach-skill
question: "Chat 输入框怎么支持 file attachment 和 skill trigger？"
winner: "B"
tags: [chat-input, file-attachment, skill-trigger, slash-command, at-mention]
---

# Sketch 002: Chat Input — File Attachment + Skill Trigger

## Design Question

Chat panel 的输入框怎么支持两种关键操作：
- **File attachment**：让用户把文件（SKILL.md / 资料库 / 文档）拖进 chat，让 agent 引用或修改
- **Skill trigger**：让用户快速激活某个 Skill（compliance-review / code-explainer / kpi-dashboard）

## How to View

open `.planning/sketches/002-chat-input-attach-skill/index.html`

## Variants

- **A: Toolbar + Slash commands** — 经典 IDE 风格：`📎` paperclip 按钮 + 输入 `/` 触发 skill 下拉
- **B: Drag-drop + @-mentions** — 现代 IDE 风格：drag-drop file / 输入 `@` 触发 file 或 skill 自动补全
- **C: Smart input** — 最简化：drag anywhere + agent auto-detect 意图

## What to Look For

- 哪个 variant 最贴合 SkillForge "确定性正解" 哲学？
- file attachment 的视觉表达（chip / 缩略图 / 行内）
- skill trigger 的 discoverability（用户怎么知道有哪些 skill 可用？）
- 输入区与 chat 流的关系（attach 是先选后发 vs 边发边加）
- typing flow 不被打断（minimal cognitive load）

## Spike Findings Applied

- SKILL.md schema 已固定 7 字段（v1.0 + temperature + reasoning）
- Skill 是 plugin 化机制，dropdown 应支持 dynamic list（运行时加载）
- "确定性正解" 哲学 → UI 应清晰展示当前 active skill + reasoning chain