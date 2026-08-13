---
phase: 07-open-source-release
plan: 04
subsystem: examples
tags: [examples, skill-templates, eval-sample, dist-06, dist-08, dist-10, tdd]
requires:
  - "skills/compliance/SKILL.md (canonical 7-field Skill — verbatim source for examples/skills/compliance/)"
  - "internal/skill/{manifest,validate,tier,loader}.go (the 7-field validator + ResolveTiers the test drives)"
  - "knowledge/authority/*.md (7 source docs for examples/data/ — RESEARCH confirmed shippable, no internal info)"
  - "eval/golden/compliance.jsonl (50-case source for eval/sample.jsonl, D7-12)"
  - "07-03-SUMMARY.md (CONTRIBUTING points contributors at go test ./internal/skill -run TestExamplesValidate)"
  - "D7-10 (validate-only literal DIST-08 requirement) + 07-RESEARCH Q2 (calculator placeholder tier, validator unchanged)"
provides:
  - "examples/skills/compliance/SKILL.md — verbatim copy of skills/compliance/SKILL.md (the canonical runnable 7-field Skill)"
  - "examples/skills/simple-qa/SKILL.md — minimal RAG Skill template (ChatModelAgent, reads [authority/**], writes [], temp 0.3)"
  - "examples/skills/calculator/SKILL.md — tool-type Skill template (ReActAgent, reads [draft/**] placeholder per D7-10, writes [skill_state], temp 0.0)"
  - "examples/data/*.md — 7 verbatim copies of knowledge/authority/*.md (annual_leave, anti_corruption_clause, business_gift_prohibition, contract_approval, expense_reimbursement_scope, non_compete, travel_expense)"
  - "internal/skill/examples_validate_test.go — TestExamplesValidate: schema-validation gate driving skill.NewLoader('../../examples/skills').Scan() + 3-manifest count assertion"
  - "eval/sample.jsonl — published 50-case eval sample (byte-identical to eval/golden/compliance.jsonl, D7-12)"
affects:
  - "Plan 07-05 (eval snapshot + demo GIF) — examples/data/ + eval/sample.jsonl let a stranger reproduce `go run ./cmd/eval --frozen=compliance` without authoring any new content"
  - "Plan 07-06 (publication governance) — examples/ + eval/sample.jsonl are NEW files with no internal info at HEAD; the full-history filter-repo scrub gate (D7-09) will cover them automatically"
  - "CONTRIBUTING.md (Plan 07-03) — the referenced `go test ./internal/skill -run TestExamplesValidate` command now exists and is GREEN"
tech-stack:
  added: []
  patterns:
    - "TDD RED/GREEN cycle for a non-code artifact (test drives schema-validation of declarative SKILL.md files)"
    - "Placeholder-tier pattern (reads: [draft/**]) for tool-only Skills — keeps internal/skill.Validate unchanged while letting calculator pass the P-SKILL-3 default-deny gate (D7-10 + RESEARCH Q2)"
    - "Verbatim-copy pattern with diff-q acceptance gate (examples/data/* mirror knowledge/authority/*; eval/sample.jsonl mirrors eval/golden/compliance.jsonl) — zero new content surface"
    - "Examples-validation test path convention: relative `../../examples/skills` from internal/skill/ — works in any clone without env vars"
key-files:
  created:
    - internal/skill/examples_validate_test.go
    - examples/skills/compliance/SKILL.md
    - examples/skills/simple-qa/SKILL.md
    - examples/skills/calculator/SKILL.md
    - examples/data/annual_leave.md
    - examples/data/anti_corruption_clause.md
    - examples/data/business_gift_prohibition.md
    - examples/data/contract_approval.md
    - examples/data/expense_reimbursement_scope.md
    - examples/data/non_compete.md
    - examples/data/travel_expense.md
    - eval/sample.jsonl
  modified: []
decisions:
  - "Calculator uses reads: [draft/**] (placeholder tier) per D7-10 + 07-RESEARCH Q2 — passes internal/skill.Validate WITHOUT changing the validator (ResolveTiers(['draft/**']) -> ['草稿']). draft chosen because it is the scratch / working tier and most accurately signals 'no authoritative source' among the 4 TierEnum entries."
  - "TestExamplesValidate path uses `../../examples/skills` relative to internal/skill/ (where `go test ./internal/skill` runs from) — no env var dependency, works in any fresh clone"
  - "Body prose in simple-qa + calculator deliberately avoids restating the literal `temperature: 0.X` token so the acceptance grep stays at strict `= 1` (the literal lives only in YAML frontmatter)"
  - "Eval/sample.jsonl is a byte-identical copy of eval/golden/compliance.jsonl (no header comment, no redaction, no filtering — RESEARCH confirmed all 50 cases are synthetic test data with no PII / no real user data)"
  - "Plan acceptance grep patterns for category counts assumed `\"category\": \"X\"` (with space); actual JSONL is `\"category\":\"X\"` (no space) — file content is correct (byte-identical to golden is the primary gate); grep patterns adjusted to the actual format during verification"
metrics:
  duration: 4m
  completed: 2026-08-13
  tasks: 2
  commits: 3
  files_changed: 12
requirements_covered:
  - DIST-06
  - DIST-08
  - DIST-10
---

# Phase 7 Plan 04: Examples + Eval Sample Summary

**One-liner:** 三个 Skill 模板 + 7 篇样例文档 + 50 例评估样本上架 —— 同一个 agent 内核通过声明式 SKILL.md 文件同时承载合规审查（compliance，可运行）、通用问答（simple-qa，可运行）、纯工具（calculator，校验通过即可，D7-10），并由新增的 `TestExamplesValidate` Go 测试把"三个模板都通过 7-field validator"这一DIST-08 声明变成 CI-runnable 的硬 gate；eval/sample.jsonl 是 golden 的逐字副本，让陌生人无需编写任何内容即可复现评估。

## What Was Built

### Task 1 — examples/ tree + schema-validation gate (RED `9f2b734` → GREEN `1eddc53`)

按 TDD 顺序执行：先写 `TestExamplesValidate`（RED：`examples/skills/` 不存在，Scan 在 `os.Stat` 报"路径不存在"），再创建 10 个文件让测试转 GREEN。

**`internal/skill/examples_validate_test.go`（RED + GREEN）：**

- 驱动 `skill.NewLoader("../../examples/skills").Scan()` —— 与生产启动期完全相同的 parse+Validate 路径
- 三重断言：(1) Scan nil error（DIST-08 字面要求）；(2) 恰好 3 个 manifest（捕捉意外增删）；(3) 三个目录名 compliance/simple-qa/calculator 全部出现
- 路径 `../../examples/skills` 相对于 `internal/skill/`（`go test ./internal/skill` 的 cwd）—— 无 env var 依赖，任意 fresh clone 都工作
- 文件级 doc-comment 解释了 CONTEXT Q4 决议：**故意不发明 `cmd/skillforge validate` 子命令**，CONTRIBUTING（Plan 07-03）已指引贡献者用这个测试作为 canonical Skill 校验门

**`examples/skills/compliance/SKILL.md`：** `skills/compliance/SKILL.md` 的 verbatim 副本（`diff -q` identical）。唯一可运行模板，是 7-field schema 的 canonical 示例。

**`examples/skills/simple-qa/SKILL.md`：** 最小 RAG 模板：
- `agent: ChatModelAgent`，`reads: [authority/**]`，`writes: []`（read-only）
- `temperature: 0.3`（比 compliance 的 0.0 略高，允许小幅 paraphrase）
- 3 条 prohibitions（"no citation / no fabrication / no personal info"），`required_refusal_phrases: [no_relevant_source]`
- description 明确与 compliance 区分（通用问答 vs. 需 chunk_id+span 的合规裁决）

**`examples/skills/calculator/SKILL.md`：** 工具型模板（D7-10 validate-only）：
- `agent: ReActAgent`，`writes: [skill_state]`，`temperature: 0.0`
- `reads: [draft/**]` —— **PLACEHOLDER tier**（D7-10 + RESEARCH Q2 决议）：`internal/skill.Validate` 拒绝空 reads（P-SKILL-3 default-deny），所以纯工具型 Skill 必须声明一个占位 tier；`draft` 是 4 个 TierEnum 中最贴近"无权威源"语义的；`ResolveTiers(["draft/**"]) → ["草稿"]`，validator 完全未改动
- 3 条 prohibitions（含"Never cite retrieved chunks — calculator reads no knowledge tier"明确标注 placeholder 不是真源）
- 正文用大段 note 解释 validate-only 状态（"runtime does not yet execute pure-tool Skills (D7-10)"），引用 D7-10 与 RESEARCH Q2
- 不修改 validator 是有意的：让平台支持纯工具执行属"新能力"，超 Phase 7 范围

**`examples/data/*.md`：** 7 篇 `knowledge/authority/*.md` 的 verbatim 副本（annual_leave / anti_corruption_clause / business_gift_prohibition / contract_approval / expense_reimbursement_scope / non_compete / travel_expense）。`diff -q` 全部 identical。让陌生人把 `examples/data/` 拷进 `knowledge/` 即可跑检索，无需自备语料（DIST-06）。

### Task 2 — eval/sample.jsonl（DIST-10，commit `51b042f`）

`cp eval/golden/compliance.jsonl eval/sample.jsonl` —— 字节级 identical（D7-12："发布完整 50 例金标集作为 eval/sample.jsonl"）。

- 50 行 non-empty JSONL（20 adversarial + 20 refusal-required + 10 multi-hop）
- 每行 valid Case JSON（含 id/category/query/ground_truth/expected_chunks/source?）
- 复现路径：`go run ./cmd/eval --frozen=compliance`（lock 元数据 `eval/frozen/compliance.lock.json` 已在仓库中：`compliance-2026-08-13-v1` / `glm-5.2` / `prompt_hash: bd59cbe0…`）
- 无 header comment（JSONL 惯例，与 golden 一致）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan acceptance grep patterns for category counts assumed space-after-colon JSON**

- **Found during:** Task 2 verification
- **Issue:** The PLAN's acceptance criteria used `grep -c '"category": "adversarial"'`（带空格），但 golden 源文件（及 byte-identical 的 sample.jsonl）实际格式是 `"category":"adversarial"`（无空格，标准 JSONL 紧凑形式）。原始 grep 模式返回 0 匹配。
- **Fix:** 验证时改用实际格式 `"category":"adversarial"`（无空格）。文件本身正确（`diff -q` 与 golden identical 是首要 gate），category 计数 20+20+10=50 经程序化校验通过。**未修改 sample.jsonl 内容**——它是 golden 的逐字副本，内容必须保持一致。
- **Files modified:** none（验证命令调整，无代码/数据变更）
- **Commit:** 不单独 commit（验证步骤的注解，记录在 SUMMARY 的 decisions）

**2. [Rule 3 - Blocking] Body prose restated `temperature: 0.X` token, breaking strict-equality grep**

- **Found during:** Task 1 GREEN 验证
- **Issue:** 初版 simple-qa + calculator 的正文段落为解释设计选择，重复了字面 token `temperature: 0.3` / `temperature: 0.0`，使 `grep -c` 返回 2 而非 PLAN 要求的严格 `= 1`。
- **Fix:** 改写正文段落，用描述性措辞替代字面重复（"a moderate (non-zero) temperature" / "a frozen-deterministic temperature"）。YAML frontmatter 仍是字面 token 的唯一出现位置。
- **Files modified:** `examples/skills/simple-qa/SKILL.md`（正文段落），`examples/skills/calculator/SKILL.md`（正文段落）
- **Commit:** `1eddc53`（合并进 GREEN 提交，未单独 commit）

无 Rule 2/Rule 4 偏差；validator 完全未改动；无 architectural 决策超出 PLAN 范围。

## TDD Gate Compliance

| Gate | Required Commit | Status |
|------|-----------------|--------|
| RED | `test(07-04): add failing TestExamplesValidate gate for examples/` | `9f2b734` ✓ |
| GREEN | `feat(07-04): ship examples/ tree (3 Skill templates + 7 sample docs)` | `1eddc53` ✓ |
| REFACTOR | optional | n/a（GREEN 已是最小可读形式，无冗余可清） |

Git log 顺序：`9f2b734` → `1eddc53` → `51b042f` —— RED gate 在 GREEN 之前，符合 TDD 强制序列。

## Threat Model — Disposition Status

| Threat ID | Component | Plan Disposition | HEAD Status after 07-04 |
|-----------|-----------|------------------|-------------------------|
| T-7-OSS-INFO | examples/data sample docs | mitigate | **Mitigated** — `diff -q` 校验 examples/data/*.md 与 knowledge/authority/*.md 全部 identical，即"零新内容面"——这些文档早已是公开可发布的语料（RESEARCH 确认无内部信息）。HEAD grep `10\.62\.10\.34\|internal\.company\.com\|bge-m3\.internal` 在 examples/ 与 eval/sample.jsonl 全部 0 匹配。 |
| T-7-OSS-INFO | eval/sample.jsonl | accept | **Accepted as intended** — D7-12 字面要求发布 50 例；RESEARCH §Runtime State Inventory 已确认 50 例全部为合成测试数据（5 spike-anchors from cmd/spiked/e2e.go + 45 derived），无 PII / 无真实用户数据。 |
| T-7-OSS-V5 | examples/skills/calculator `reads` field | mitigate | **Mitigated** — calculator 声明 `reads: [draft/**]` placeholder tier，通过 Validate 而不破坏 P-SKILL-3 default-deny 不变量；validator 字节级未改动（grep `func Validate` 在 internal/skill/validate.go 与 HEAD~3 identical）。 |

## Threat Flags

None. examples/ + eval/sample.jsonl 全部是声明式文件（SKILL.md / .md / .jsonl），不引入任何新网络端点、auth 路径、文件访问模式或信任边界 schema 变更。SKILL.md 通过已有的 7-field validator 校验（不修改 validator）；examples/data/*.md 与 eval/sample.jsonl 是已有公开内容的逐字副本。Plan 的 `<threat_model>` 已覆盖全部三个相关威胁，无新威胁面。

## Known Stubs

None. 所有交付物都是有完整内容的成品：
- 3 个 SKILL.md 模板：完整的 7-field YAML frontmatter + 解释性正文
- 7 个 examples/data/*.md：verbatim 副本，内容完整（与 knowledge/authority/ 同）
- 1 个 eval/sample.jsonl：50 例完整金标集
- 1 个 examples_validate_test.go：可执行 Go 测试，GREEN

calculator 模板虽是"validate-only"（runtime 不执行纯工具 Skill），但这**不是 stub**——它是 D7-10 明确要求的交付物形态（"DIST-08 字面只要求 each passing the schema validator"），且其正文大段 note 明确标注此限制，verifier 不会误读为可运行。

## Open Items Handed to Downstream Plans

- **Plan 07-05（eval snapshot + demo GIF）：** `examples/data/` + `eval/sample.jsonl` 现已就位，07-05 跑 `go run ./cmd/eval --frozen=compliance` 即可基于这些样本产出真实 faithfulness / context_precision / refusal_accuracy 指标填进 README 的 PLACEHOLDER 表。
- **Plan 07-06（publication governance）：** `examples/` + `eval/sample.jsonl` 是 NEW 文件，HEAD 无内部信息；D7-09 的全历史 filter-repo scrub gate 会自动覆盖。无需 07-04 单独的历史清理动作。

## Self-Check: PASSED

**Files exist:**
- `examples/skills/compliance/SKILL.md` — FOUND
- `examples/skills/simple-qa/SKILL.md` — FOUND
- `examples/skills/calculator/SKILL.md` — FOUND
- `examples/data/annual_leave.md` — FOUND
- `examples/data/anti_corruption_clause.md` — FOUND
- `examples/data/business_gift_prohibition.md` — FOUND
- `examples/data/contract_approval.md` — FOUND
- `examples/data/expense_reimbursement_scope.md` — FOUND
- `examples/data/non_compete.md` — FOUND
- `examples/data/travel_expense.md` — FOUND
- `internal/skill/examples_validate_test.go` — FOUND
- `eval/sample.jsonl` — FOUND
- `.planning/phases/07-open-source-release/07-04-SUMMARY.md` — FOUND (this file)

**Commits exist (verified in `git log`):**
- `9f2b734` — `test(07-04): add failing TestExamplesValidate gate for examples/` — FOUND
- `1eddc53` — `feat(07-04): ship examples/ tree (3 Skill templates + 7 sample docs)` — FOUND
- `51b042f` — `docs(07-04): publish eval/sample.jsonl (50-case golden set, DIST-10)` — FOUND

**Plan verification gates (all pass):**
- DIST-08 schema gate `go test ./internal/skill -run TestExamplesValidate -v` → PASS ✓
- DIST-06 sample docs `ls examples/data/*.md | wc -l` → 7 ✓
- DIST-10 `wc -l < eval/sample.jsonl` → 50 ✓
- DIST-10 `diff -q eval/sample.jsonl eval/golden/compliance.jsonl` → identical ✓
- Full `internal/skill` regression suite → ok (2.297s, no regressions) ✓

**Task 1 acceptance gates (all 11 pass):** 3 SKILL.md templates ✓; 7 data docs ✓; ChatModelAgent in simple-qa = 1 ✓; ReActAgent in calculator = 1 ✓; `draft/**` placeholder present (3 matches) ✓; `temperature: 0.0` in calculator = 1 ✓; `temperature: 0.3` in simple-qa = 1 ✓; `diff -q examples/skills/compliance/SKILL.md skills/compliance/SKILL.md` identical ✓; `diff -q examples/data/annual_leave.md knowledge/authority/annual_leave.md` identical ✓; calculator description `calculate|算术|计算` = 1 ✓; calculator note `validate-only|D7-10|tool-type` = 7 ✓.

**Task 2 acceptance gates (all 6 pass):** byte-identical to golden ✓; 50 lines ✓; 20 adversarial ✓; 20 refusal-required ✓; 10 multi-hop ✓; head -1 valid JSON ✓.

**Deletion check:** zero file deletions across all 3 task commits.
