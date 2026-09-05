# SkillForge — Roadmap

**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes.
**Mode:** mvp (vertical slices, MVP-first delivery)
**Granularity:** standard
**Date:** 2026-08-10

---

## Milestones

- ✅ **v0.7.7** — 可信问答内核 (phases 1-7) — shipped 2026-08-13
- ✅ **v0.8** — Harness 可信×行动融合 (phases 8-13) — shipped 2026-08-21
- ✅ **v1.0** — 业务数据支撑 Business Data Support (phases 14-20) — shipped 2026-08-30
- 🚧 **Next** — v1.1 (planned)

## Phase Details

<details>
<summary>✅ v0.7.7 — 可信问答内核 (phases 1-7) — SHIPPED 2026-08-13</summary>

- [x] Phase 1: Integration Spike & Trust Contract — 4/4 plans (completed 2026-08-11)
- [x] Phase 2: Compliance Vertical Slice — 5/5 plans (completed 2026-08-11)
- [x] Phase 3: Agentic Retrieval & Trust Evaluation — 6/6 plans (completed 2026-08-12)
- [x] Phase 4: Streaming Trust UI — 4/4 plans (completed 2026-08-11)
- [x] Phase 5: Skill Platform & Full Knowledge Boundaries — 8/8 plans (completed 2026-08-12)
- [x] Phase 6: Eval Dashboard & Production Hardening (6a) — 3/3 plans (completed 2026-08-13)
- [ ] Phase 7: Open-Source Release — 5/6 plans (in progress — 07-02 T3, 07-05 T2/T3, 07-06 blocking)

</details>

<details>
<summary>✅ v0.8 — Harness 可信×行动融合 (phases 8-13) — SHIPPED 2026-08-21</summary>

- [x] Phase 8: 全量依赖升级 (P8.0) — 5/5 plans (completed 2026-08-18)
- [x] Phase 9: 历史注入 trust-floor spike (P8.S) — 6/6 plans (completed 2026-08-18, NO-GO + escape plan)
- [x] Phase 10: 多轮对话记忆 (P8.1) — 4/4 plans (completed 2026-08-20, closed/blocked-with-notes)
- [x] Phase 11: 写工具 + 审批门 (P8.2) — 8/8 plans (completed 2026-08-20)
- [x] Phase 12: 规划层 (P8.3) — 8/8 plans (completed 2026-08-20)
- [x] Phase 13: 子 agent 扇出 (P8.4) — 6/6 plans (completed 2026-08-21)

</details>

<details>
<summary>✅ v1.0 — 业务数据支撑 Business Data Support (phases 14-20) — SHIPPED 2026-08-30</summary>

- [x] Phase 14: CGO 地基 + 平台信任地板 (P10.0) — 12/12 plans (completed 2026-08-23)
- [x] Phase 15: 数据导入管线 + 导入即脱敏 (P10.1) — 7/7 plans (completed 2026-08-24)
- [x] Phase 16: KPI 聚合通道 (P10.2) — 9/9 plans (completed 2026-08-27)
- [x] Phase 17: KPI 仪表盘 Skill (P10.3) — 8/8 plans (completed 2026-08-27)
- [x] Phase 18: 业务月报 Skill (P10.4) — 5/5 plans (completed 2026-08-29)
- [x] Phase 19: 静态报告门户 + 调度器 (P10.5) — 5/5 plans (completed 2026-08-29)
- [x] Phase 20: 管理台 (P10.6) — 7/7 plans (completed 2026-08-30)

**Archived:** `.planning/milestones/v1.0-ROADMAP.md` + `.planning/milestones/v1.0-REQUIREMENTS.md`

</details>

### 🚧 Next Milestone (v1.1 — Planned)

- [ ] Phase 21: External Authorized Channel (DATA-04/05/06/10/11/13/14)
- [ ] Phase 22: Eval CI Wiring + Live UAT (Phase 6b completion) — 含 2026-09-05 竞品格局分析补充项：
  - [ ] **Judge 人工校准集**：30-50 例人工标注对齐 + judge-vs-human agreement 报告（MLflow 2026 指南的标准 practice）。现有 `--judge-models` 跨评测只覆盖 judge 间分歧（Pitfall 8 自评偏差），未覆盖 judge-人工一致性——这是行业标准做法中缺口的一半。golden set 已有，成本低。
  - [ ] **评测器定位声明**：`cmd/eval` 的 4 指标（faithfulness 等）与 Ragas/DeepEval/TruLens 完全同质（Ragas 口径已兼容），定位明确为 **Skill 晋级闭环的内部基础设施**，不作为独立卖点——README/文档措辞规避"评测器=核心能力"误读。
- [ ] Phase 24: 审计链强化 (Audit Chain Hardening) — 2026-09-06 新增，对标 Semantica 补齐三弱势；**建议先于 Phase 23 执行**（产出直接把 Phase 23 README 里"承认 PROV-O 是行业标准"的弱势措辞翻转为"支持 PROV-O 导出"的正面特性）：
  - [ ] **PROV-O 标准导出**（补弱势①无标准导出）：新 `internal/auditprov` 包，把既有 INSERT-only 审计数据映射为 W3C PROV-JSON——prov:Entity = query/chunk/verdict/规则行，prov:Activity = run/retrieve/approve/revoke，prov:Agent = user/skill/model_version/platform_floor_version；关系 wasGeneratedBy / used / wasDerivedFrom。零 LLM 纯确定性（Semantica 同款哲学），经既有 retention 脱敏规则，导出端点 `GET /api/session/:id/audit/prov` + 行内 sha256 完整性清单。
  - [ ] **决策因果链查询**（对标 trace_decision_chain，补弱势②扁平事件无因果链）：`GET /api/session/:id/decision-chain?root=<tool_call_id>`，沿既有 `tool_call_id==approval_id`（D11-C4）+ session_id + approval_rules 审计痕迹行走链：query → retrieve(chunk_ids) → verdict → approve/reject → rule 创建 → auto-pass(last_used) → revoke；FE AuditTimelineTab 增链式视图。
  - [ ] **时间点状态重建**（对标 time-travel，补弱势③无历史回放）：`GET /api/session/:id/state?at=<RFC3339>` 从 INSERT-only 数据确定性重建 T 时刻的 turns + 审计事件 + 规则表状态——已撤销规则从 `action_rule_revoked` 的 frozen_args PK 复原（T-3-AUDIT"审计行即证据"设计的终极验证，纯只读实现）。
  - [ ] （可选，需 planner 决策）审计行 prev-hash 哈希链 additive 列——防篡改可验证，超出 Semantica 对标范围，属增强项；如做须守 D14-E1 索引后置纪律。
  - 纪律：additive migration only（D14-E1）；INSERT-only 不变式 source-grep 锁不放松（T-3-AUDIT / D16-03-A 同款）；审计路径零 LLM；导出必须过既有脱敏。
- [ ] Phase 23: Phase 7 completion (Open-Source Release) — 含 2026-09-05 竞品格局分析 + 2026-09-06 第二轮深度竞品验证（WeKnora/Semantica）补充项：
  - [ ] **README 差异化叙事重排**（依据两轮联网验证的成熟度判定）：
    - 头条差异点（两轮验证仍空白/存活）：① **approve-and-remember 审批门一等公民原语**（`approval_rules` frozen-args-hash + session/skill 二元 scope + 撤销即审计——主流框架只做 pause-resume；Semantica 的 decision intelligence 是事后因果追溯，非执行前审批；WeKnora 无审批门）；② **信任契约内核**（tier 知识隔离 + refusal-by-construction + citation 白名单=本轮检索——WeKnora 走"查得准"混合检索无拒答构造；Semantica 管知识事实溯源不管执行边界）；③ **评测门禁**（frozen pin + CI gate + 跨评源——两个项目与业界均无）。
    - 降权/收窄的叙事：**审计链从头条降为支撑能力**（Semantica 以 W3C PROV-O 标准做溯源+决策因果图+冲突检测+时间旅行，2026-08 GitHub Trending #1；SkillForge 自研 audit 表无标准导出——差异收窄为"agent 写操作执行审计 vs 知识事实溯源"，措辞必须承认 PROV-O 是行业标准）；**"Go 企业 RAG 平台"不作为卖点**（WeKnora 21k+ star 腾讯背书 Go 单体后端+RBAC+审计已是成熟对标）；4 信任指标与 CI 门禁模式维持降权（Ragas 同款/行业最佳实践）。
    - 新增 Skill 术语去混淆段：WeKnora 的"Skill"=可插拔技能封装安装进 Docker/E2B/Cube 沙箱（Anthropic Agent Skills 路线）；SkillForge 的 Skill=**领域适配契约**（知识层边界 reads + 写目标 writes + 拒答护栏 + prompt 主体，同一内核不改代码换领域）——README 第一屏必须消除同名误读。
    - 竞品对照段扩为三分位：WeKnora 管"文档怎么查得准"（RAG+Wiki+多租户 RBAC）/ Semantica 管"决策为什么"（PROV-O 溯源+因果图+知识图谱）/ SkillForge 管"**什么不许答、什么必须批**"（执行时信任边界：tier 隔离、拒答构造、审批门、评测门禁）——三者互补而非同赛道对垒。

**Full phase details in archived milestones:**
- `.planning/milestones/v0.7.7-ROADMAP.md`
- `.planning/milestones/v0.8-ROADMAP.md`
- `.planning/milestones/v1.0-ROADMAP.md`

---

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Integration Spike & Trust Contract | 4/4 | Complete | 2026-08-11 |
| 2. Compliance Vertical Slice | 5/5 | Complete | 2026-08-11 |
| 3. Agentic Retrieval & Trust Evaluation | 6/6 | Complete | 2026-08-12 |
| 4. Streaming Trust UI | 4/4 | Complete | 2026-08-11 |
| 5. Skill Platform & Full Knowledge Boundaries | 8/8 | Complete | 2026-08-12 |
| 6. Eval Dashboard (6a) | 3/3 | Complete | 2026-08-13 |
| 7. Open-Source Release | 5/6 | In Progress | — |
| 8. 全量依赖升级 (P8.0) | 5/5 | Complete | 2026-08-18 |
| 9. 历史注入 spike (P8.S) | 6/6 | Complete | 2026-08-18 |
| 10. 多轮对话记忆 (P8.1) | 4/4 | Closed/blocked | 2026-08-20 |
| 11. 写工具 + 审批门 (P8.2) | 8/8 | Complete | 2026-08-20 |
| 12. 规划层 (P8.3) | 8/8 | Complete | 2026-08-20 |
| 13. 子 agent 扇出 (P8.4) | 6/6 | Complete | 2026-08-21 |
| 14. CGO + 平台信任地板 (P10.0) | 12/12 | Complete | 2026-08-23 |
| 15. 数据导入管线 (P10.1) | 7/7 | Complete | 2026-08-24 |
| 16. KPI 聚合通道 (P10.2) | 9/9 | Complete | 2026-08-27 |
| 17. KPI 仪表盘 Skill (P10.3) | 8/8 | Complete | 2026-08-27 |
| 18. 业务月报 Skill (P10.4) | 5/5 | Complete | 2026-08-29 |
| 19. 静态报告门户 (P10.5) | 5/5 | Complete | 2026-08-29 |
| 20. 管理台 (P10.6) | 7/7 | Complete | 2026-08-30 |

---

## Backlog

### Phase 999.1: 分层分块器决策 — hierarchical markdown splitter (BACKLOG)

**Goal:** Decide the fate of the markdown hierarchical splitter that was silently dropped from scope. Two options: (a) formally record one-doc-one-chunk + maxDocBytes(4000) guardrail as the permanent design (fine if knowledge corpus stays short-clause .md), or (b) implement eino-ext markdown header splitter to support long policy docs / PDFs. Origin: stale comments in internal/knowledge/indexer.go claimed "deferred to Phase 3 (CMPL-06)" — corrected in quick task 260814-ofn; no phase ever owned this work.
**Requirements:** TBD
**Plans:** 8/8 plans complete

### Phase 999.2: Eval CI Wiring + Live UAT (BACKLOG) — Phase 6b

**Goal:** Complete the deferred CI-gate half of Phase 6 (split decision 2026-08-14: code-side scope completed as Phase 6a, this backlog holds the runtime-dependent remainder). Items: (1) merge `.github/workflows/eval-gate.yml` from unmerged worktree branch `worktree-agent-a1bfd67bedf21ad40` (commit fc26d12, 138 lines) to master; (2) register `GLM_API_KEY` (required) + optional `BGE_M3_URL` / `MILVUS_ADDRESS` as GitHub repository secrets; (3) sacrificial PR proving the gate blocks a faithfulness < 0.9 regression with `gh pr comment` naming skill + threshold + ❌; (4) the 4 pending `06-HUMAN-UAT.md` items — dashboard visual check (`--archive` → `/eval` + `/eval/audit`), `SKILLFORGE_ADMIN_TOKEN` redaction toggle, `go test -tags liveeval` jitter layer with `GLM_API_KEY`, archive→dashboard walkthrough.
**Requirements:** GEVL-07 (end-to-end)
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd:review-backlog when ready)
