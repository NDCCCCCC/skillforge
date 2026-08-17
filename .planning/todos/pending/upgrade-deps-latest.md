---
title: 升级所有依赖到最新 stable（含 eino 大版本）
date: 2026-08-17
priority: high
status: pending
trigger: v0.8 harness milestone 启动前（多轮记忆 phase 的前置）
---

# 升级所有依赖到最新 stable

用户在 2026-08-17 explore 会话中明确：**升级所有依赖到最新版**。

## 范围

- **eino / eino-ext**：升级到最新 stable tag。CLAUDE.md 记录的 "v0.7.7 latest" 已过时（main 已到 v0.10.0-alpha，需核实最新 stable）。升级动机：
  - `adk/middlewares/summarization`（历史压缩，v0.8.x 时代）——多轮记忆需要
  - ADK HITL（`Runner.ResumeWithParams` / `ChatModelAgentResumeData.HistoryModifier`）——审批门需要
  - 注意 v0.7.0 的 interrupt/resume 重构（`WithStateModifier` 旧API已替换），升级时核对迁移指南
- **其余 Go 依赖**：hertz、milvus SDK 等一并 `go get -u` 后跑全量测试
- **前端依赖**：React / Vite / ai SDK / Tailwind / shadcn 同步核实最新 stable

## 验收

- `go build ./...` + `go test ./...` 全绿
- 前端 `npm run build` + vitest 全绿
- golden set eval 回归：faithfulness / refusal_accuracy 不低于冻结基线（eval/frozen/compliance.lock.json）

关联：[[harness-roadmap-decision]]
