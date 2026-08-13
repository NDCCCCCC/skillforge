---
phase: 07-open-source-release
plan: 03
subsystem: docs
tags: [readme, contributing, docs, open-source, badges, mermaid, dist-01, dist-07]
requires:
  - "07-01-SUMMARY.md (LICENSE + scrubbed .env.example defaults + fail-fast contract)"
  - "07-02-SUMMARY.md (docker-compose.yml topology + Dockerfile.* for quick-start/architecture reference)"
  - "skills/compliance/SKILL.md (canonical 7-field Skill)"
  - "internal/skill/{manifest,validate,tier}.go (schema + validAgentTypes + TierEnum for CONTRIBUTING accuracy)"
  - "eval/frozen/compliance.lock.json (eval-pin metadata for the placeholder section)"
  - "D7-02 (ollama one-liner), D7-05/06 (GIF replaces live demo), D7-10 (calculator placeholder tier)"
provides:
  - "README.md — fork-run-benchmark entry point with all 7 DIST-01 elements (badges, value prop, demo GIF path, quick start, Mermaid architecture diagram, contributing link, license) + Eval Results placeholder for Plan 05"
  - "CONTRIBUTING.md — Skill / Tool / Tier contribution guide with the three named sections (DIST-07) + fork-PR flow + code style"
affects:
  - "Plan 07-04 (examples/) — CONTRIBUTING references examples/skills/{simple-qa,calculator} templates + go test ./internal/skill -run TestExamplesValidate"
  - "Plan 07-05 (eval snapshot + demo GIF) — README Eval Results section is a clearly-marked PLACEHOLDER for 07-05 to fill; docs/demo.gif path referenced now"
  - "Plan 07-06 (publication governance) — no filter-repo interaction (README/CONTRIBUTING are NEW files with no internal info at HEAD)"
tech-stack:
  added: []
  patterns:
    - "Mermaid fenced diagram (GitHub-native rendering, no PNG — RESEARCH recommendation + CONTEXT discretion)"
    - "shields.io static badge endpoints (license + Go version + CI placeholder)"
    - "Eval Results as explicit PLACEHOLDER (date TBD + _TBD_ cells) — no invented eval metrics"
    - "CONTRIBUTING references go test ./internal/skill -run TestExamplesValidate (CONTEXT Q4 resolution — no cmd/skillforge validate subcommand)"
key-files:
  created:
    - README.md
    - CONTRIBUTING.md
  modified: []
decisions:
  - "Mermaid diagram chosen over PNG (GitHub renders natively, diffable, maintainable — RESEARCH §Architecture Patterns + CONTEXT discretion)"
  - "Eval Results section left as PLACEHOLDER (date TBD + _TBD_ metrics) — Plan 07-05 fills real numbers via cmd/eval --frozen; no metrics invented"
  - "CONTRIBUTING points contributors at `go test ./internal/skill -run TestExamplesValidate` instead of inventing a `cmd/skillforge validate` subcommand (CONTEXT Q4 resolved — lighter surface, Plan 07-04 ships the test)"
  - "CI badge is a yellow 'pending' placeholder (06-03 eval-gate workflow deferred); license + Go-version badges are static shields.io endpoints"
  - "Calculator tool-only Skill documented as placeholder-tier pattern (reads: [draft/**]) per D7-10 — runtime is RAG pipeline today"
  - "README/CONTRIBUTING content in English (OSS artifacts for international audience); SUMMARY narrative in Chinese per response_language"
metrics:
  duration: 3m
  completed: 2026-08-13
  tasks: 2
  commits: 2
  files_changed: 2
requirements_covered:
  - DIST-01
  - DIST-07
---

# Phase 7 Plan 03: Docs (README + CONTRIBUTING) Summary

**One-liner:** Wrote the two documents defining a stranger's first experience — README.md（含全部 7 个 DIST-01 元素：badges + value prop + demo GIF path + quick start + Mermaid 架构图 + contributing link + license，Eval Results 节为 Plan 05 留占位符）与 CONTRIBUTING.md（Write a Skill / Add a Tool / Add a Knowledge Tier 三节，各含可工作代码示例，引用真实标识符 internal/skill.Validate / validAgentTypes / TierEnum / ResolveTiers）—— 零内网信息，纯文档产出。

## What Was Built

### Task 1 — README.md (`d35e7a6`)

完整的 fork-run-benchmark 入口文档，覆盖 DIST-01 全部 7 个必含元素：

- **Hero（首屏）：** 项目名 + 一句话价值主张（"Same agent kernel, customized per enterprise via pluggable Skill files — without code changes."）+ badges 行（Go 1.25 + Apache-2.0 + CI pending placeholder，全部用 shields.io 静态端点）+ demo GIF embed（`![SkillForge demo](docs/demo.gif)` —— GIF 本体由 Plan 05 产出，README 现在引用路径）。
- **Quick Start（2 屏内可达）：** 前置依赖表（Docker / ollama for BGE-M3 / GLM API key / ~8GB RAM）+ 5 步命令（clone → `cp .env.example .env` 填 `GLM_API_KEY` + `BGE_M3_URL` → `ollama pull bge-m3`（D7-02 一行 embedding 最低门槛）→ `docker compose up` → 打开 `http://localhost:8080`）+ 示例问题（出差住宿费超标 200 元）。
- **Architecture（Mermaid 图，GitHub 原生渲染）：** 展示 `docker compose up` 的 5 服务拓扑（etcd / minio / milvus / backend / frontend）+ 2 个外部端点（ollama BGE-M3 + GLM API），三个护城河直接标注在图上（真源分离 tier-filter / Skill-as-platform / trust evaluator）。未用 PNG（可维护性 + GitHub 原生渲染，RESEARCH 推荐）。
- **Three Moats（三护城河叙事）：** 每个一段，各引验证它的 spike/phase SC —— (1) 真源分离 tier-filter（Phase 1 SC-2 + default-deny validator）；(2) Skill-as-platform（D2 平台论点 + Height.app 反例）；(3) Go-native trust evaluator（faithfulness 逐字移植 Ragas v0.1.21 prompt，Apache-2.0 归因，单语言栈 + 行业可比）。
- **Skills：** 链接 `skills/compliance/SKILL.md` 作为样板间；提及 `examples/` 模板（Plan 04）。
- **Eval Results（PLACEHOLDER）：** 占位表（`Skill | Faithfulness | Context Precision | Refusal Accuracy | Cases`），日期标注 `_TBD (Plan 07-05)_`，指标单元格全部 `_TBD_`，lock 元数据（`compliance-2026-08-13-v1` / `glm-5.2` / `prompt_hash: bd59cbe0…`）已填。**未编造任何数字。** 复现命令 `go run ./cmd/eval --frozen=compliance`。
- **Contributing + License：** 一段 + 链接 CONTRIBUTING.md；Apache-2.0 链接 LICENSE + eval/NOTICE（Ragas 归因）。

### Task 2 — CONTRIBUTING.md (`4daade7`)

三个命名章节（DIST-07），每节含最小可工作代码示例（Pitfall 4：无代码的章节一律拒绝）：

- **Fork + PR Flow（顶部）：** fork → branch → PR 标准流 + 本地校验命令（`gofmt -w .` / `go test ./...` / `go test ./internal/skill -run TestExamplesValidate`）+ 代码风格（gofmt 强制 + Apache-2.0 SPDX 包头先例，给出完整 16 行 header block）。
- **Write a Skill：** 7-field schema 表（description / agent / reads / writes / steps / guardrails.prohibitions / temperature）+ 6 个 validAgentTypes（ChatModelAgent / ReActAgent / SequentialAgent / ParallelAgent / LoopAgent / ConditionalAgent）+ ~12 行 SKILL.md 骨架（simple-qa 形态）+ 两个 gotcha（reads 非空 default-deny P-SKILL-3；temperature 是 `*float32` 必填指针 SYSK-08，解释 Go 零值陷阱）+ 校验命令 `go test ./internal/skill -run TestExamplesValidate`（CONTEXT Q4 resolution —— 不发明 `cmd/skillforge validate` 子命令，复用 Plan 04 的 test）。
- **Add a Tool：** Eino `tool.InvokableTool` + `utils.NewTool` 模式，指向 `internal/agent/retrieve_tool.go` 作为参考实现，给出最小 `BuildMyTool` 骨架（ToolInfo + 泛型 InvokeFunc 闭包），标注 KNOW-03 tier-closure 模式（tier 在构造时快照，LLM 无法覆盖）+ calculator 占位 tier 约定（D7-10）。
- **Add a Knowledge Tier：** TierEnum 表（authority→权威 / historical→历史 / personal→个人 / draft→草稿）+ `knowledge/<dir>/*.md` 目录约定 + ResolveTiers 映射 + 两种场景（向已有 tier 加文档 = 只加 .md 文件；加全新第 5 tier = 改 TierEnum map + 建目录）。

## Deviations from Plan

None — plan executed exactly as written. 未触发任何 Rule 1-3 自动修复。所有 acceptance gate grep 一次通过。

## Threat Model — Disposition Status

| Threat ID | Component | Plan Disposition | HEAD Status after 07-03 |
|-----------|-----------|------------------|-------------------------|
| T-7-OSS-INFO | README.md / CONTRIBUTING.md content | mitigate | **Mitigated at HEAD** — 两文件零内网 IP/域名匹配（`grep -cE '10\.62\.10\.34\|internal\.company\.com\|bge-m3\.internal'` = 0 across both files）。Quick start 仅引用公共端点（localhost:8080 / milvus:19530 / ollama / open.bigmodel.cn）。全历史 filter-repo scrub 仍是 Plan 07-06 的 gate。 |
| T-7-OSS-INFO | README quick-start commands | mitigate | **Mitigated** — 所有 quick-start 命令仅引用公共产物（docker compose up / ollama pull bge-m3 / localhost:8080），无内网主机名。 |

## Threat Flags

None. README + CONTRIBUTING 是纯文档产出，不引入新的网络端点、auth 路径、文件访问模式或信任边界 schema 变更。所有引用的标识符（internal/skill.Validate / TierEnum / retrieve tool / docker-compose）都是已有代码契约的文档化。

## Known Stubs

**Eval Results section（README.md）是有意为之的占位符** —— 这是 Plan 的明确要求（Plan 07-05 填真实数字）。占位状态：
- 日期 = `_TBD (Plan 07-05)_`
- 四个指标单元格 = `_TBD_`
- lock 元数据（eval_set_version / model_ver / prompt_hash）已填真实值（来自 `eval/frozen/compliance.lock.json`）

这不是阻止 Plan 目标实现的 stub —— 它是跨 Plan 的契约：07-03 建结构，07-05 填数据。README 明确标注 "Placeholder ... Do not treat the table as measured yet."，verifier 不会误读。

## Open Items Handed to Downstream Plans

- **Plan 07-04（examples/）：** CONTRIBUTING 引用了 `examples/skills/{simple-qa,calculator}` 模板 + `go test ./internal/skill -run TestExamplesValidate`。Plan 04 必须交付这两个测试对象（calculator 用 `reads: [draft/**]` 占位 tier 通过 validator，D7-10）。
- **Plan 07-05（eval snapshot + demo GIF）：**
  - README 的 `## Eval Results` 节是 PLACEHOLDER —— 07-05 跑 `cmd/eval --frozen=compliance` 后把 `_TBD_` 替换为真实 faithfulness / context_precision / refusal_accuracy 数字 + 填日期。
  - README 的 `![SkillForge demo](docs/demo.gif)` 引用 `docs/demo.gif` 路径 —— 07-05 必须把录制好的 GIF 放到该路径。
- **Plan 07-06（publication governance）：** 无 filter-repo 交互（README/CONTRIBUTING 是新文件，HEAD 无内网信息）。07-06 的全历史 grep 审计 gate 会自动覆盖这两个文件。

## Self-Check: PASSED

**Files exist:**
- `README.md` — FOUND
- `CONTRIBUTING.md` — FOUND
- `.planning/phases/07-open-source-release/07-03-SUMMARY.md` — FOUND (this file)

**Commits exist (verified in `git log`):**
- `d35e7a6` — `feat(07-03): add README.md ...` — FOUND
- `4daade7` — `feat(07-03): add CONTRIBUTING.md ...` — FOUND

**Plan verification gates (all pass):**
- DIST-01 README elements `grep -cE 'badge|Quick Start|docker compose|```mermaid|Contributing|Apache-2.0' README.md` = 12 (≥6) ✓
- DIST-07 CONTRIBUTING sections `grep -cE 'Write a Skill|Add a Tool|Add a Knowledge Tier' CONTRIBUTING.md` = 3 (=3) ✓
- Cross-file internal-info `grep -cE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' README.md CONTRIBUTING.md` = 0 (README:0, CONTRIBUTING:0) ✓

**Task 1 acceptance gates (all 11 pass):** ## Quick Start = 1; docker compose up = 3; ```mermaid = 1; docs/demo.gif = 1; ollama pull bge-m3 = 2; Apache-2.0 = 3; internal-info = 0; moats narrative = 7; ## Eval Results = 1; go run ./cmd/eval --frozen = 1; localhost:8080 = 3.

**Task 2 acceptance gates (all 10 pass):** Write a Skill = 1; Add a Tool = 1; Add a Knowledge Tier = 1; fenced code blocks = 18 (≥6); temperature = 3; TierEnum CN chars = 10; internal-info = 0; ChatModelAgent/ReActAgent = 2; gofmt/Apache-2.0/SPDX = 5.

**Deletion check:** zero file deletions across both task commits.
