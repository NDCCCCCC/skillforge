# Phase 23 — Open-Source Release (Phase 7 completion)

> **状态**: 2026-09-06 IN PROGRESS。本里程碑完成 v1.1 OSS 发布
> 闭环——README 差异化叙事重排 + CHANGELOG + RELEASE-NOTES + 贡献者
> 指南。v1.1 全部 Phase（25/24/22/21）已闭环，Phase 23 是 v1.1 收官。

## 上下文与决策契约

### 为什么需要这个

v1.0 README 把 SkillForge 定位为"Go-Native Trust Evaluator" + 真源
分离 + Skill-as-Platform 三段——是当时唯一可比对标的写法。但 2026-09-05
+ 2026-09-06 两轮竞品验证（WeKnora / Semantica）后：

- WeKnora 已是成熟 Go RAG 平台（21k+ star / 腾讯背书），**"Go 企业
  RAG 平台"不再是 SkillForge 的差异化卖点**。
- Semantica 的 PROV-O 决策溯源是行业标准——SkillForge 自研 audit 表
  反而显得薄弱，但**执行前审批门**（Semantica 是事后因果追溯）和
  **信任契约内核**（执行时 tier 隔离 + 拒答构造）**是对方不具备的**。
- 4 信任指标 / CI gate 是 Ragas + 业界最佳实践，不再是 SkillForge 独有。

v1.1 的差异化定位必须**重新校准**——把"审计链 / Go RAG 平台 / 评测"
降为支撑能力，把"审批门一等公民原语 / 信任契约内核 / 评测门禁"
升为头条；并新增 Skill 术语去混淆段（WeKnora 的 Skill = 沙箱技能封
装，SkillForge 的 Skill = 领域适配契约——同名不同义）。

### 范围

| 任务 | 状态 |
|------|------|
| 23-01 README 差异化叙事重排 + 三分位对照表 + Skill 去混淆 | 待落地 |
| 23-02 CHANGELOG.md v1.1.0 | 待落地 |
| 23-03 RELEASE-NOTES.md（用户面向）| 待落地 |
| 23-04 CONTRIBUTING.md 定位指南 + 闭环 | 待落地 |

### 纪律

- **每一条 README 表述都标出竞品对照**：WeKnora 走查得准 / Semantica
  走为什么 / SkillForge 走不许答什么必须批什么——避免回到"Go RAG
  平台"自吹式叙事。
- **承认 PROV-O 是行业标准**（Phase 24 已补完 PROV-O 导出）：
  README 把"审计链"明确归为支撑能力（"我们也支持 PROV-O 导出"），
  不再作为头条。
- **Skill 术语去混淆必须在第一屏**：README 顶部 tagline 立刻声明
  SkillForge 的 Skill ≠ WeKnora 的 Skill，避免用户读到一半才意识到
  术语冲突。

### 不做（v1.1 范围内主动放弃）

- ❌ Video / GIF 重做（已有 docs/demo.gif 可继续用）
- ❌ 一键发布 GitHub Release（v1.0.0 之前的 1.0.0 tag 决定）
- ❌ 完整 Docker 镜像发布流水线（已有 docker-compose.yml，CI 单独
  里程碑）

### 关键文件

- `README.md`（已有，Phase 22 已改 §3，Phase 23 改 §2 + 加 §4）
- `CHANGELOG.md`（新）
- `RELEASE-NOTES.md`（新）
- `CONTRIBUTING.md`（已有，加一段定位指南）
- `.planning/uat/phase23-oss-2026-09-06.md`（闭环报告）
