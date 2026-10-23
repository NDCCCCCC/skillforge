---
phase: 50-open-source-facade
captured: 2026-10-23
mode: direct-planning (user decision, ask_user_question 2026-10-23; discuss/research skipped)
---

# Phase 50 Context — 开源门面

<domain>

**Phase Boundary**（ROADMAP 原文）：新访客 5 分钟内理解 SkillForge 是什么、凭什么不同、怎么跑起来——门面素材与仓库实况零失真。**纯文档/社区设施，无生产代码改动。**

本 phase 的宪法：**门面与仓库实况零失真**——每句 README/文档声明必须能被仓库事实或本 phase 新建文件证实；verify-phase-50.sh 是机械判据。

</domain>

<decisions>

- **D-50-01 — 对比表对象跟随 README 现定位（WeKnora / Semantica），不用 ROADMAP 旧措辞 Dify/RAGFlow。** README 已被改写为 "Execution-time trust boundaries for AI agents — 什么不许答、什么必须批"（三车道：WeKnora 查得准 / Semantica 讲为什么 / SkillForge 管执行时边界，出现 9 次）。ROADMAP :60/:71/:214/:220 的 "vs Dify/RAGFlow" 是旧规划期措辞。**已由用户选择直接规划 ⇒ 本条作为 flagged assumption 记入 plan（planner 不得擅自改回 Dify/RAGFlow）；ROADMAP Phase 50 段的旧措辞由 50-01 顺手更新为现定位（这属于"门面与实况零失真"的一部分）。**
- **D-50-02 — 示例 skill 数量以 `examples/skills/` 实际目录为准 = 7**（calculator、compliance、compliance-orchestrator、external-lookup、kpi-dashboard、monthly-report、simple-qa）。README:13-15 的 "Three ready-to-run templates … simple-qa, calculator, compliance" 必须更正；verify-phase-50.sh 断言 README 的计数与 `ls examples/skills | wc -l` 一致。
- **D-50-03 — SECURITY.md 从 `docs/threat-model/` 既有 STRIDE 材料蒸馏**（STRIDE-11/12/13/14/26…），不新写安全声明；报告渠道指向 GitHub Security Advisory（仓库无私有漏洞渠道）。CODE_OF_CONDUCT.md 用 Contributor Covenant v2.1 标准文本（Apache-2.0 开源项目惯例）。
- **D-50-04 — GitHub 模板最小集**：`.github/ISSUE_TEMPLATE/bug_report.md` + `feature_request.md` + `config.yml`（禁空 issue 直开，引导到模板）+ `.github/PULL_REQUEST_TEMPLATE.md`（change 类别 / 动机 / 验证方式 checklist——与 CONTRIBUTING.md 的 verify-phase-NN 门槛一致）。不加 discussion 模板（仓库未开 Discussions）。
- **D-50-05 — badge 如实化**：现有 CI badge（`CI-eval-gate-22-09-06-green` 指向目录的静态绿标）改为指向真实 workflow 文件的 badge（shields.io endpoint 或 GitHub badge，链接到 `.github/workflows/ci.yml` 的实际检查）；Go 版本 badge 与 go.mod 的 go 指令一致（1.25.x）。禁止保留任何"装饰性绿标"。
- **D-50-06 — docs/architecture.md 用 Mermaid 组件图**（GitHub 原生渲染，无图片资产维护成本），覆盖四层并给出代码入口路径：agent 内核（`internal/server`、`internal/skill`、`cmd/server`）、知识层（`internal/knowledge`、`internal/retriever`、`internal/embedder`、`knowledge/`）、治理层（`internal/verdict`、`internal/governance`、`internal/audit`、`docs/threat-model/`）、平台层（`internal/kb`、`internal/store`、`web/`、面板/admin）。每层标注 1-2 个"从这张图进入代码"的入口文件。
- **D-50-07 — CGO_ENABLED=1 如实表述**：go-duckdb 生产构建现实路径；README 不宣称 "no cgo / single static binary"（那是 Phase 51 发布工程的课题）。
- **D-50-08 — Claude's Discretion**：对比表的行维度与措辞、architecture.md 的图布局、SECURITY.md 结构、issue 模板字段、badge 具体形态、README 各小节顺序微调（不改动 Phase 49/52 新增的 hermetic/面板小节语义）。

</decisions>

<specifics>

- demo.gif 已存在且为真实运行素材（Phase 49 交付，211794 B，46 帧）——本 phase 只需确认 README 引用有效 + 门禁断言 magic bytes，**不重录**。
- `CONTRIBUTING.md` 已存在（284 行）——SECURITY/CODE_OF_CONDUCT 与它的链接互通即可，不重写。
- 所有新文档语言：README/en 系保持英文，SECURITY/CODE_OF_CONDUCT 英文（开源惯例）；`docs/architecture.md` 英文。
- verify-phase-50.sh 的断言必须全部机械可判（文件存在 / grep 计数 / magic bytes / 目录比对），不做"看起来对"的检查。

</specifics>

<canonical_refs>

- README（现状改写基准）：`README.md`
- 示例目录（计数真源）：`examples/skills/`
- 威胁材料（SECURITY.md 输入）：`docs/threat-model/`
- 贡献指南（互通链接）：`CONTRIBUTING.md`
- CI workflow（badge 指向）：`.github/workflows/ci.yml`
- demo 素材：`docs/demo.gif`
- 架构事实源：`.planning/codebase/`（不存在 ⇒ 以 `internal/` 目录结构与 `cmd/server/main.go` 装配为准）
- 门禁先例：`verify-phase-49.sh`、`verify-phase-52.sh`

</canonical_refs>

<deferred>

- ROADMAP 里旧措辞 "vs Dify/RAGFlow" 的全面清理（:60/:71/:214/:220）——50-01 只改 Phase 50 自己的段落，其余留给 milestone 收尾。
- Discussions / 社交媒体 badge / star-history 等扩展设施。

</deferred>
