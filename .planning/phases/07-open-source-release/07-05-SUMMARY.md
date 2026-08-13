---
phase: 07-open-source-release
plan: 05
subsystem: eval-snapshot
tags: [eval, snapshot, refusal-accuracy, dist-09, dist-05, tdd, checkpoint]
requires:
  - "internal/eval (Report + CaseResult + LockFile structs + LoadReport + LoadLock — read-only consumer, no changes)"
  - "eval/frozen/compliance.lock.json (snapshot metadata source: model_ver=glm-5.2, eval_set_version=compliance-2026-08-13-v1, prompt_hash=bd59cbe0…)"
  - "07-03-SUMMARY.md (README '## Eval Results' PLACEHOLDER section this plan fills)"
  - "07-RESEARCH §Code Examples Example 6 (markdown table format) + §Open Questions Q1 (refusal_accuracy derivation — load-bearing, NOT an evaluator field)"
  - "07-CONTEXT D7-11 (manual snapshot + date stamp) + D7-05/06 (GIF replaces live demo)"
provides:
  - "scripts/eval_snapshot/main.go — standalone Go program that loads eval/report.json + eval/frozen/compliance.lock.json, derives refusal_accuracy from Report.Cases, and renders the README markdown snapshot block"
  - "scripts/eval_snapshot/main_test.go — 6 hermetic unit tests pinning the derivation (all-refused 1.0 / half-refused 0.5 / zero-refusal-required graceful 0,0,0 / category-filter invariant / renderSnapshot lock metadata / zero-refusal-required N/A cell)"
  - "deriveRefusalAccuracy — pure function, the ONE place the DIST-09 refusal_accuracy derivation happens (count(Cases cat=refusal-required && Refused) / count(Cases cat=refusal-required))"
affects:
  - "README.md (Plan 07-03) — Eval Results section still PLACEHOLDER (_TBD_); Task 2 (checkpoint:human-verify, BLOCKED) fills it after a live `cmd/eval --frozen=compliance` run"
  - "docs/demo.gif (Plan 07-03 hero embed) — file still absent; Task 3 (checkpoint:human-verify, BLOCKED) records it against a live stack"
  - "Plan 07-06 (publication governance) — must NOT publish until Task 2 + Task 3 resolve (README numbers + GIF are the brand artifacts)"
tech-stack:
  added: []
  patterns:
    - "Pure-function extraction for a derivation that is NOT a stored metric — deriveRefusalAccuracy is the single source of truth for DIST-09 refusal_accuracy so the README number always traces to one place"
    - "Read-only consumer of internal/eval public API (LoadReport + LoadLock) — cmd/eval and internal/eval stay unchanged; the snapshot script is a leaf, not a branch"
    - "Hermetic test fixtures built in-test (no disk I/O, no live stack) — the derivation logic is testable independently of the autonomous:false live-eval step"
    - "Graceful N/A cell for zero-refusal-required fixtures — a missing category renders 'N/A', not '0%' (a degenerate fixture is not a 0% score)"
key-files:
  created:
    - scripts/eval_snapshot/main.go
    - scripts/eval_snapshot/main_test.go
  modified: []
decisions:
  - "Pure-function shape `deriveRefusalAccuracy(report eval.Report) (ratio float64, total, refused int)` — returns three values so the renderer can show '95% (19/20)' instead of a bare ratio; total=0 (degenerate fixture) returns (0,0,0) and the renderer surfaces 'N/A' rather than '0%'"
  - "renderSnapshot uses `report.Skill` first, falling back to `lock.SkillName`, then `eval.DefaultGateSkill` — matches cmd/eval's gateDecision fallback ordering so a Skill-less legacy report still renders"
  - "categoryBreakdown renders ONLY the 3 canonical categories (adversarial / refusal-required / multi-hop) in fixed order; unknown categories are skipped rather than appended (the golden set is fixed at 3 categories; inventing an extras section would render one-off spike fixtures as if they were a stable contract)"
  - "shortHashLen=8 for the prompt_hash prefix in the snapshot header — the full hash is in eval/frozen/compliance.lock.json; the snapshot only needs enough to eyeball 'did this move?'"
  - "stdlib-only imports (flag / os / fmt / time / strings) — no new external deps, satisfying the 'script imports only internal/eval + standard library' plan constraint"
metrics:
  duration: 2m
  completed: 2026-08-13
  tasks: 1
  commits: 2
  files_changed: 2
requirements_covered: []
---

# Phase 7 Plan 05: Eval Snapshot + Demo GIF Summary

**One-liner:** `scripts/eval_snapshot` Go 程序上架 —— `internal/eval` 的只读消费者，把 `cmd/eval --frozen` 产出的 `eval/report.json` + `eval/frozen/compliance.lock.json` 渲染成 README「Eval Results」节可直接粘贴的 markdown 快照（含日期 + lock 元数据 + faithfulness/context_precision + **派生**的 refusal_accuracy + case 构成）。refusal_accuracy 是 DIST-09 要求但 evaluator **不直接产出**的字段，本脚本里的 `deriveRefusalAccuracy` 纯函数是全仓库唯一派生点（RESEARCH Q1 load-bearing）。6 个 hermetic 单元测试覆盖 all-refused/half-refused/zero-refusal-required/category-filter invariant/renderSnapshot 元数据/N/A cell，无需 live stack。Task 1 完成并提交（TDD RED→GREEN）；**Task 2（live eval run + README 快照填充）+ Task 3（demo GIF 录制）是 `checkpoint:human-verify gate=blocking` 任务，autonomous:false —— 需要本地 Milvus + BGE-M3 + GLM_API_KEY live stack + 屏幕录制工具，本环境无此条件，已 STOP 并交还控制权给人工**。

## What Was Built

### Task 1 — `scripts/eval_snapshot` 拒答准确率派生 + markdown 渲染器（TDD RED `abf2f4d` → GREEN `4873213`）

按 TDD 顺序执行：先写 `main_test.go`（RED：`deriveRefusalAccuracy` / `renderSnapshot` undefined，编译失败），再写 `main.go` 让 6/6 测试转 GREEN。

**`scripts/eval_snapshot/main_test.go`（RED，commit `abf2f4d`）：**

6 个 hermetic 单元测试，**全部在测试内构造 `eval.Report` fixture，无磁盘 I/O、无 live stack 依赖**：

1. `TestDeriveRefusalAccuracy_AllRefused` —— 2 例 refusal-required 全拒答 → `ratio=1.0 / total=2 / refused=2`
2. `TestDeriveRefusalAccuracy_HalfRefused` —— 4 例中 2 例拒答 → `ratio=0.5 / total=4 / refused=2`
3. `TestDeriveRefusalAccuracy_ZeroRefusalRequired` —— 0 例 refusal-required → 优雅降级 `total=0 / refused=0 / ratio=0`（renderer 转 "N/A"）
4. `TestDeriveRefusalAccuracy_OnlyCountsRefusalRequiredCategory` —— adversarial / multi-hop 上的 `Refused==true` **不得**进入分子，只统计 refusal-required 类目（category-filter 不变量）
5. `TestRenderSnapshot_IncludesLockMetadataAndColumns` —— 渲染输出包含 14 项断言：日期 / eval_set_version / model_ver / prompt_hash 短前缀 / skill 名 / 4 列表头 / 派生百分比 / 92% / 85% / reproduce 命令
6. `TestRenderSnapshot_ZeroRefusalRequired_ShowsNA` —— 零 refusal-required 时渲染 "N/A"，**不**渲染 "0% (...)"（避免把"degenerate fixture"误读为"0% 拒答"）

**`scripts/eval_snapshot/main.go`（GREEN，commit `4873213`）：**

- 独立 `package main` 程序，只 import `internal/eval` + 标准库（`flag` / `os` / `fmt` / `time` / `strings`）
- 两个 flag：`-report`（默认 `eval/report.json`）+ `-lock`（默认 `eval/frozen/compliance.lock.json`）+ `-date`（默认 `time.Now().UTC()` 当日）
- `deriveRefusalAccuracy(report eval.Report) (ratio float64, total, refused int)` —— **RESEARCH Q1 的唯一派生点**：`count(Cases where Category=="refusal-required" && Refused==true) / count(Cases where Category=="refusal-required")`。`refusal-required` 作为 `const refusalCategory` 单点定义，让字面量只出现一次。`total==0` 时返回 `(0, 0, 0)` 并交由 renderer 显示 "N/A"
- `renderSnapshot(report, lock, runDate)` —— 输出符合 RESEARCH Example 6 格式的 markdown 块：
  - 头行：`Frozen run on **<date>** · eval_set_version: <v> · model_ver: <m> · prompt_hash: <8-char short>…`
  - 表格：`| Skill | Faithfulness | Context Precision | Refusal Accuracy | Cases |` + 数据行（含 `92% / 85% / 50% (1/2) / 4 (1 adversarial / 2 refusal-required / 1 multi-hop)`）
  - reproduce 命令：`go run ./cmd/eval --frozen=compliance`
- `countByCategory` + `categoryBreakdown` —— 在 3 个 canonical 类目上按固定顺序渲染 case 构成
- `run(reportPath, lockPath, date, w *os.File)` —— 可测试 seam，让 `main()` 只负责 flag 解析 + `os.Exit`

**Smoke 验证（非 commit 一部分，在 GREEN 提交前跑过）：**

```bash
$ go run ./scripts/eval_snapshot -report /tmp/smoke_report.json -lock eval/frozen/compliance.lock.json -date 2026-08-13
Frozen run on **2026-08-13** · `eval_set_version: compliance-2026-08-13-v1` · `model_ver: glm-5.2` · `prompt_hash: bd59cbe0…`

| Skill | Faithfulness | Context Precision | Refusal Accuracy | Cases |
|-------|-------------|-------------------|------------------|-------|
| compliance | 92% | 85% | 50% (1/2) | 4 (1 adversarial / 2 refusal-required / 1 multi-hop) |

**Reproduce:** `go run ./cmd/eval --frozen=compliance`
```

## Checkpointed Tasks (BLOCKING — autonomous:false)

### Task 2 — live-stack eval run + README 快照填充（`checkpoint:human-verify gate=blocking`）

**Status:** BLOCKED —— 本环境无 Docker / Milvus / BGE-M3 / GLM_API_KEY live stack。STOP 并交还控制权。

**前置条件：**
1. 安装并启动 Docker Desktop（Plan 07-02 Task 3 同一前置条件；STATE.md 显示该项也仍 BLOCKING）
2. `ollama pull bge-m3` —— 提供 embedding 端点（默认 `http://localhost:11434/v1/embeddings`）
3. `cp .env.example .env` 并填入：
   - `GLM_API_KEY=<your Zhipu Coding Plan key>`
   - `BGE_M3_URL=http://localhost:11434/v1/embeddings`
4. `docker compose up` —— 起 5 服务（etcd + minio + milvus + backend + frontend）

**人工执行步骤：**
```bash
# 1. 跑 frozen eval —— 50 case 全跑，产出 eval/report.json + eval/calibration.json
go run ./cmd/eval --frozen=compliance
# 期望日志：50 [OK] case 行 + "[OK] cmd/eval: 50 cases scored. Report: eval/report.json ..."

# 2. 校验报告
test -f eval/report.json && python -c "import json; r=json.load(open('eval/report.json')); assert 'overall' in r and 'cases' in r and len(r['cases'])==50; print('report OK')"

# 3. 生成快照 markdown
go run ./scripts/eval_snapshot
# 输出形如：
# Frozen run on **2026-08-13** · `eval_set_version: compliance-2026-08-13-v1` · `model_ver: glm-5.2` · `prompt_hash: bd59cbe0…`
# | compliance | 9X% | 8X% | 9X% (XX/20) | 50 (...) |

# 4. 把步骤 3 的整段输出粘贴进 README.md 的 "## Eval Results" 节，替换当前 _TBD_ 表格
#    （确认日期为实际运行日期；保留 ## Eval Results 节标题不动）

# 5. 追溯性自检：README 表中每个数字都能在 eval/report.json 里找到来源
#    - faithfulness/context_precision ← report.Overall
#    - refusal_accuracy (XX/20) ← 派生自 report.Cases
#    - case 总数 50 ← len(report.Cases)
```

**Resume signal：** 把 `go run ./scripts/eval_snapshot` 的完整 stdout 粘回聊天（我会写进 README），或描述失败原因（例如 "BGE-M3 endpoint refused — 用了 X 替代"）。若本会话确实无 live stack，回复 "deferred — snapshot pending live stack"，README 占位符保留。

### Task 3 — demo GIF 录制 + 嵌入校验（`checkpoint:human-verify gate=blocking`）

**Status:** BLOCKED —— 需要运行中的 stack（依赖 Task 2）+ 屏幕录制工具。

**前置条件：** Task 2 完成后 stack 仍在运行（`http://localhost:8080` 可达）。

**人工执行步骤：**
1. 浏览器打开 `http://localhost:8080`，选 **compliance** Skill
2. 启动屏幕录制（ScreenToGif / OBS / macOS Screen Recording）
3. 提一个能产出 **cited verdict** 的问题（例："出差住宿费超标 200 元，能批吗？请引用具体制度条款。"）—— 让裁决卡 + 引用渲染完整
4. 提一个能触发 **structured refusal** 的问题（范围外问题，触发 `no_authorized_source` / out-of-scope 拒答卡）
5. 停止录制，导出为 GIF（目标 ≤60s，用 `gifsicle --optimize=3 --colors 128` 或 `ffmpeg` 压到 <5MB）
6. 保存为 `docs/demo.gif`（若 `docs/` 不存在则创建）
7. 校验 README 嵌入：`grep -c 'docs/demo.gif' README.md` 应为 1（Plan 07-03 已写好 embed）

**覆盖要求（D7-05/06 品牌叙事）：** GIF 必须**同时**包含 cited verdict **和** structured refusal 两个流程 —— 这是 "trust" + "refuse-by-construction" 两条护城河的 UI 显形。

**Resume signal：** 回复 "approved"（GIF 已录、<5MB、两个流程都覆盖），或描述缺漏（例如 "只录了 verdict，refusal 没触发"）。无录制工具则回复 "deferred — GIF pending capture tooling"，README embed 显示 broken-image 占位直到解决。

## Deviations from Plan

None. Task 1 严格按 PLAN 执行：TDD RED→GREEN 顺序、`deriveRefusalAccuracy` 纯函数 shape、stdlib-only imports、read-only 消费 `internal/eval`、不修改 `cmd/eval` 或 `internal/eval`。所有 6 条 acceptance criteria 全部通过（详见 Self-Check）。Task 2/3 是 PLAN 明确标记的 `checkpoint:human-verify gate=blocking` 任务，非偏差。

## TDD Gate Compliance

| Gate | Required Commit | Status |
|------|-----------------|--------|
| RED | `test(07-05): add failing tests for refusal_accuracy derivation` | `abf2f4d` ✓ |
| GREEN | `feat(07-05): implement eval_snapshot refusal_accuracy derivation` | `4873213` ✓ |
| REFACTOR | optional | n/a（GREEN 已是最小可读形式；`deriveRefusalAccuracy` 是纯函数、`renderSnapshot` 是单一渲染 seam、无冗余可清） |

Git log 顺序：`abf2f4d` → `4873213` —— RED gate 严格在 GREEN 之前，符合 TDD 强制序列。

## Threat Model — Disposition Status

| Threat ID | Component | Plan Disposition | HEAD Status after 07-05 Task 1 |
|-----------|-----------|------------------|--------------------------------|
| T-7-OSS-INFO | docs/demo.gif content | mitigate | **Not yet applicable** —— GIF 未录制（Task 3 BLOCKED）；待人工录制时遵守"只录 app viewport、不录 devtools/URL"约束 |
| T-7-OSS-REP | README eval snapshot numbers | mitigate | **Tscripted but not yet published** —— `scripts/eval_snapshot` 已实现 reproducibility 机制（date + eval_set_version + model_ver + prompt_hash 从 lock 文件读取 + refusal_accuracy 派生自 Cases 的纯函数），但 README 表仍是 _TBD_（Task 2 BLOCKED）；Task 2 完成后任何填入的数字都可由陌生人 `go run ./cmd/eval --frozen=compliance` 复现验证 |
| T-7-SC | screen-capture / gifsicle / ffmpeg tooling | mitigate | **Not yet applicable** —— 这些是 release-time OS 工具，Task 3 human-verify 时由人工安装/使用；本任务未引入任何 npm/pip/cargo 运行时依赖 |

## Threat Flags

None. `scripts/eval_snapshot` 是纯只读 CLI（`eval.LoadReport` + `eval.LoadLock` 各一次 `os.Open` + `json.Decode`，向 stdout `fmt.Fprint`），不引入任何新网络端点、auth 路径、文件访问模式或信任边界 schema 变更。Plan 的 `<threat_model>` 已覆盖 Task 2/3 相关威胁（T-7-OSS-INFO / T-7-OSS-REP / T-7-SC），Task 1 无新威胁面。

## Known Stubs

`README.md` 「Eval Results」节仍为 PLACEHOLDER（_TBD_ 表格）—— **这是有意的、由 PLAN 设计的 stub**：

- **位置：** `README.md` 第 122-134 行（`## Eval Results` 节，Plan 07-03 留下的占位符）
- **原因：** D7-11 明确要求"手工快照 + 日期标注"，而非编造的静态数字。在人工跑 `cmd/eval --frozen=compliance` 产出真实 `eval/report.json` 之前（Task 2），任何填入的数字都是 fabrication，违反 T-7-OSS-REP mitigation（"refusal_accuracy traces to Report.Cases via the open-source derivation — no black-box numbers"）。
- **解决路径：** Task 2 `checkpoint:human-verify` 完成后，把 `go run ./scripts/eval_snapshot` 的 stdout 粘进 README，stub 即被真实数字替换。`docs/demo.gif` 同理 —— Plan 07-03 的 hero embed（`![SkillForge demo](docs/demo.gif)`）是显式指向 Task 3 产物的占位符。

Verifier 不应把这两处 PLACEHOLDER 当作"plan 未完成"信号 —— 它们是 `autonomous:false` plan 的预期中间态，由 checkpoint 任务收敛。

## Open Items Handed to Downstream (Human + Plan 07-06)

- **人工（Task 2）：** 跑 `go run ./cmd/eval --frozen=compliance` + 把 `scripts/eval_snapshot` stdout 粘进 README Eval Results 节。详细步骤见本文档 "Task 2 — live-stack eval run"。
- **人工（Task 3）：** 录制 `docs/demo.gif`（cited verdict + structured refusal 两条流程，<5MB）。详细步骤见本文档 "Task 3 — demo GIF 录制"。
- **Plan 07-06（publication governance）：** **不得在 Task 2 + Task 3 解决前 push 到 public remote** —— README eval 数字 + demo GIF 是品牌叙事的两个 measurable artifact；缺任意一个都让"陌生人 fork-run-benchmark"叙事失效。Plan 07-06 应把"Task 2 + Task 3 resume signal approved"作为发布前置 gate。

## Self-Check: PASSED

**Files exist:**
- `scripts/eval_snapshot/main.go` — FOUND
- `scripts/eval_snapshot/main_test.go` — FOUND
- `.planning/phases/07-open-source-release/07-05-SUMMARY.md` — FOUND (this file)

**Commits exist (verified in `git log`):**
- `abf2f4d` — `test(07-05): add failing tests for refusal_accuracy derivation` — FOUND
- `4873213` — `feat(07-05): implement eval_snapshot refusal_accuracy derivation` — FOUND

**Task 1 acceptance gates (all 8 pass):**
- `go build ./scripts/eval_snapshot/` → exit 0 ✓
- `go test ./scripts/eval_snapshot/ -v` → 6/6 PASS ✓
- `grep -c 'refusal-required' scripts/eval_snapshot/main.go` → 9 (≥1) ✓
- `grep -cE 'eval\.LoadReport|eval\.LoadLock' scripts/eval_snapshot/main.go` → 2 (≥1) ✓
- `grep -c 'Refused' scripts/eval_snapshot/main.go` → 3 (≥1) ✓
- `grep -c 'deriveRefusalAccuracy' scripts/eval_snapshot/main_test.go` → 4 (≥1) ✓
- `grep -cE '"flag"|"os"|"fmt"|"time"' scripts/eval_snapshot/main.go` → 4 (≥3) ✓
- `git status --short cmd/eval internal/eval` → empty (unchanged) ✓

**Additional gates:**
- `go vet ./scripts/eval_snapshot/` → clean ✓
- End-to-end smoke render against synthesized report → output format matches RESEARCH Example 6 ✓
- Test fixture coverage: all-refused (1.0) ✓ half-refused (0.5) ✓ zero-refusal-required (graceful 0/N/A) ✓ category-filter invariant ✓

**Deletion check:** zero file deletions across both task commits.
