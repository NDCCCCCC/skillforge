# Phase 22 — Eval CI Wiring + Live UAT (Phase 6b completion)

> **状态**: 2026-09-06 IN PROGRESS。本计划接续 Phase 25（golden-corpus
> 同步 + agent_error 指标分离）的产物——golden set 已经可机读、闭环
> 跑通、agent_error 不再污染拒答指标。Phase 22 把这一切绑成 CI 门禁。

## 上下文与决策契约

### 必须解决的痛点

1. **frozen compliance 集一直靠手工跑**：当前没有任何 CI 自动跑
   `cmd/eval --frozen=compliance`，每次改 prompt / 重写语料 / 调
   knowledge.Index 都没法验证 `faithfulness >= 0.9` /
   `refusal_accuracy >= 0.95` 是否仍满足。这是 v1.0 UAT 阶段最
   容易回退的入口。
2. **judge 人工校准集缺失**：当前 cross-eval 只覆盖 judge 间分歧
   （`--judge-models glm-5.2,deepseek`），不覆盖 **judge vs 人工**。
   这是 Ragas / TruLens / DeepEval 行业标准实践的另一半。
3. **评测器定位声明缺位**：README 没有明确说明"4 指标 = Ragas 口径
   同质 = Skill 晋级闭环的内部基础设施"，容易被误读为"SkillForge
   有自研评测平台"这种第三方对标口径（事实上和 Ragas 完全同质，
   不构成差异化卖点）。

### 范围

| 任务 | 状态 |
|------|------|
| 22-01 frozen replay CI wiring | ✅ commit (本里程碑) |
| 22-02 judge 人工校准集（30-50 题） | planned |
| 22-03 评测器定位声明 (README + docs) | planned |
| 22-04 Live UAT：CI dry-run + 本地 gate 复跑 | planned |

### 纪律

- **不做新指标**：4 指标 = Ragas v0.1.21 口径 verbatim port，新增
  任何指标都需要新里程碑（避免"SkillForge 有自研评测平台"误读）。
- **CI 不接 push trigger 必然跑**：每次 PR + main push 必跑，但**不**
  触发自动 merge；Human-in-the-loop 必须由 reviewer 看 CI 状态。
- **CI 阈值与本地 gate 一致**：`./cmd/eval --gate-check` 是 ground
  truth，CI 是它的包装；阈值漂移 = 信任契约破坏。

### 不做

- ❌ 不接 semantic-release / auto-merge（仓库没设发版节奏）
- ❌ 不接 nightly 自动 replay（GLM 配额敏感；需要时手动跑）
- ❌ 不把 production 模型 pin 到 CI（生产门禁 = frozen replay +
  AssertPin，CI 只测合规域）

### 关键文件

- `.github/workflows/eval-gate.yml` — GitHub Actions 工作流
- `examples/skills/compliance/eval-gate.json` — per-Skill 阈值（已有）
- `cmd/eval/main.go` — `--gate-check` 已实现，复用
- `eval/frozen/compliance.jsonl` — 50 题已带 source/corpus_file 注解
  （Phase 25 + 本次重写）
- `README.md` — 评测定位声明新增段落
