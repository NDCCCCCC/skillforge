# Phase 22 — Plan Outline (5 waves)

## Wave 1 — 数据基础（已完成）
- frozen compliance 集 source 注解 + corpus_file attribution (commit)
- spike-anchor 三题 (adv-001/002/003) GT 修复（GBK mojibake → UTF-8
  verbatim 抄自 knowledge/authority/）

## Wave 2 — CI wiring（已完成）
- `.github/workflows/eval-gate.yml`：
  - trigger: pull_request + push to main + workflow_dispatch
  - steps: checkout → go build → frozen replay → gate check → artifact upload
  - required secrets: GLM_API_KEY, BGE_M3_URL, MILVUS_ADDRESS
  - 不接 auto-merge（人工 reviewer 必看 CI 状态）
  - timeout-minutes: 90（50 例 frozen 大约 80 分钟）

## Wave 3 — 评测定位声明（待办）
- `README.md` 新增段落"评测定位声明"：
  - 明确 4 指标 (faithfulness / answer_relevancy / context_precision /
    context_recall) = Ragas v0.1.21 verbatim port
  - 与 Ragas / DeepEval / TruLens 差异表（事实：指标同质 / 价值是
    绑定 Skill 晋级闭环 / 不构成独立平台对标）
  - 诚实承认"如果想要独立评测平台，请用 Ragas"
- `cmd/eval/README.md` 同段落摘录

## Wave 4 — judge 人工校准集（待办）
- 30-50 题人工标注 + judge-vs-human agreement 报告
- 来源：从现有 frozen compliance 集 30 题 + 新增 20 题（multi-hop
  域）
- 计算：cross-judge agreement + judge-vs-human agreement + judge
  bias（按 category / refusal 切片）
- 输出：`eval/calibration/judge-human-agreement.md`（每次 eval 自动
  重新计算；作为 v1.1 候选）

## Wave 5 — Live UAT（待办）
- CI dry-run：本仓库 push 一个 trivial PR 触发 eval-gate.yml
- 本地复跑：`./cmd/eval --frozen compliance --report eval/reports/ci/compliance.json`
- 闭环：把 CI output 写入 `.planning/uat/phase22-eval-ci-2026-09-06.md`
- 提交 ROADMAP/STATE 更新
