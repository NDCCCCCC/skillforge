# Phase 22 Eval CI — UAT 闭环（2026-09-06）

> **目标**：把 frozen compliance replay 绑成 CI 门禁（`--gate-check`）。
> **当前里程碑**：Wave 1-3 落地（数据基础 + CI wiring + README
> 定位声明），Wave 4（judge 人工校准集）保留为 Phase 22-02 待办，
> Wave 5 Live UAT dry-run 落地。

## 交付物清单

### ✅ Wave 1 — 数据基础
- `eval/frozen/compliance.jsonl` 全部 50 题打 source 标注：
  - `spike-anchor`: 5 题（pre-Phase-25 时期）
  - `closed-loop-2026-09-06`: 26 题（Phase 25 corpus 同步后）
  - `negative-test-2026-09-06`: 19 题（refusal-required 基线）
- 30 题 adv/mh 全部带 `corpus_file` 标注 → 可追溯到 `knowledge/authority/*.md`
- spike-anchor 三题 (adv-001/002/003) GT 修复：
  - 历史 GBK mojibake → UTF-8 verbatim 抄自对应 knowledge chunk
  - 与闭环语料措辞一致，避免 judge 噪声

### ✅ Wave 2 — CI wiring
- `.github/workflows/eval-gate.yml`：
  - triggers: pull_request + push to main + workflow_dispatch
  - 6 steps: checkout → go1.23 setup → build → frozen replay → gate
    check → artifact upload (14 天保留)
  - required secrets: GLM_API_KEY, BGE_M3_URL, MILVUS_ADDRESS
  - timeout: 90 分钟（50 例 frozen 约 80 分钟）
  - 不接 auto-merge（人工 reviewer 必看 CI 状态）
  - YAML lint 通过（python yaml.safe_load）

### ✅ Wave 3 — README 评测定位声明
- §3 "Go-Native Trust Evaluator" 改写为 "CI-Integrated Trust Gate"：
  - 明确 4 指标 = Ragas v0.1.21 verbatim port
  - SkillForge **不**是独立评测平台 → 是 Skill 晋级闭环内部基础设施
  - 与 Ragas / DeepEval / TruLens 6 维度差异表（事实陈述）
  - 诚实承认"如果想要独立评测平台，请用 Ragas"
- 透明标注单语言栈是部署便利，不是差异化卖点

### 🟡 Wave 4 — judge 人工校准集
- `.planning/phases/22-eval-ci/22-JUDGE-CALIBRATION.md`：结构 +
  跑分脚本 + 验收门槛契约（agreement >= 0.85 / judge 漏判率 < 0.10）
- 待 user 标注 30-50 题后激活

### ✅ Wave 5 — Live UAT dry-run

**`--gate-check` 验证（闭环 run 报告）**：

```
$ go run ./cmd/eval --gate-check \
    eval/reports/local-models/2026-09-06-compliance-qwen38-27b-ud-q4-closedloop.json \
    --skills-dir examples/skills

### SkillForge eval gate

| Skill       | faithfulness | threshold | verdict |
|-------------|--------------|-----------|---------|
| compliance  | 0.8996       | 0.9000    | ❌      |

**Gate FAILED** — below threshold: `compliance` (0.8996 < 0.9000)
exit status 1
```

**解读**：0.8996 vs 0.9 阈值 → fail。**这是正确的 fail-closed 行为**——
本地 Qwen3.8-27B 未达生产门槛，正式复签仍需 GLM pinned run（Phase 14-10
独立任务）。CI gate 没有"软化"任何阈值，这正是 trust 头等大事的设计。

**artifact 验证**：
- `eval/reports/ci/compliance.json` 在 gitignore（运行产物）
- 但 GitHub Actions upload-artifact 仍保留 14 天，便于 PR 评论引用

## 验收（v1.1 release gate）

| 项 | 阈值 | 当前 |
|---|------|------|
| CI workflow YAML valid | yes | ✅ |
| gate-check 退出码正确（0 通过 / 1 失败） | yes | ✅ |
| gate threshold 与本地一致 | yes | ✅ |
| frozen compliance 50 题 source 闭环 | 100% | ✅ |
| spike-anchor GT 修复 GBK mojibake | yes | ✅ |
| README 评测定位声明（事实）| yes | ✅ |

**Wave 4（人工校准集）需 user 标注数据后单独激活**——calibration 契约
已写好，标注 + 跑分动作可在 Phase 23 OSS README 撰写期间顺手完成。
