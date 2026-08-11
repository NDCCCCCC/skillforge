# Sketch Manifest

## Design Direction

SkillForge frontend — denova-style 三列工作 IDE：
- **左 sidebar**：session 列表 + file tree + 系统菜单
- **中 panel**：文档预览（agent 修改的 SKILL.md / 资料库 / 引用溯源 / verdict card）
- **右 panel**：chat with Skill（streaming + tool call + reasoning chain + 引用溯源）

两边 sidebar 都可收起。中间是**文档预览**（不是 chat）—— 这是 denova 工作流特征，agent 操作中间文档，用户在右 chat 提问。

Vibe: 专业 / denova dark / 开发者友好 / 长文本阅读 / streaming + tool-call 透明。

## Reference Points

- **denova** (用户 fork 的项目) — 三列工作 IDE，多面板布局
- Cursor / Claude Code — chat IDE (但 SkillForge 中间是"文档预览"而非 chat)
- VS Code — 文件树 + 编辑器 + 侧栏布局
-## Sketches

| # | Name | Design Question | Winner | Tags |
|---|------|----------------|--------|------|
| 001 | three-panel-layout | 三列布局视觉：宽度比例 + 折叠 UX + 中间预览模式 + 拖拽 resize | ★ A (260/flex/420) | layout, three-panel, denova-style |