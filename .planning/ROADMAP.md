# SkillForge Roadmap

## Milestones

| Milestone | Phase Range | Status |
|-----------|-------------|--------|
| v0.7.7 (Vertical Slice) | 1-7 | ✅ SHIPPED 2026-08-11 |
| v0.8 (Memory + HITL) | 8-13 | ✅ SHIPPED 2026-09-06 |
| v1.0 (Business Data) | 14-20 | ✅ SHIPPED 2026-09-06 |
| v1.1 (Eval Trust-Loop) | 21-25 | ✅ SHIPPED 2026-09-06 |
| v1.2 (Knowledge Ops) | 26-30 | ✅ SHIPPED 2026-09-08 |
| v1.3 (Code Quality) | 31-37 | ✅ SHIPPED 2026-09-09 |
| v1.4 (Ontology Layer) | 38-41 | ✅ SHIPPED 2026-09-23 |
| v1.5 (Outstanding Closure) | 42-44 | ✅ SHIPPED 2026-10-02 (closure-only) |
| v1.6 (SPARQL Schema Detection + Entity SHACL Shapes) | 45-46 | ✅ SHIPPED 2026-10-08 |
| v1.7 (Open-Source Readiness) | 47-51 | 🚧 IN PROGRESS |
| Backlog | 999.x | Backlog |

---

## ✅ Shipped Milestones (v0.7.7 – v1.6)

- **v0.7.7** Phases 1-7 — spike / compliance slice / agentic retrieval / streaming UI / skill platform / eval dashboard / OSS release
- **v0.8** Phases 8-13 — deps upgrade / history injection / multi-turn memory / write tools + approval gate / planning layer / sub-agent fan-out
- **v1.0** Phases 14-20 — CGO trust floor / KPI aggregation / dashboard / reports / scheduler / admin
- **v1.1** Phases 21-25 — 外部授权 + 审计链 + Eval CI + OSS
- **v1.2** Phases 26-30 — KBM / DOC / GRF / FBK / TKT (knowledge ops + feedback loop)
- **v1.3** Phases 31-37 — code quality remediation (23/26 REQ closed; trust semantics preserved)
- **v1.4** Phases 38-41 — Ontology Layer (TBox + KG store + ECL pipeline + SHACL governance + MCP/SPARQL/Hybrid platform). Phase detail records preserved below.
- **v1.5** Phases 42-44 — Outstanding Closure (42-03 operator runbook executed 2026-09-30/10-01: 8 PASS + L8 PARTIAL + L4/L7 env-SKIP; live 抽取 F1 0.39→0.87; Phase 44 SPARQL test fix 10/10 green, D-TIER-03 restored). Full details: `.planning/milestones/v1.5-ROADMAP.md`
- **v1.6** Phases 45-46 — SPARQL Schema Detection (D-SPARQL-05 β 语义：schemaSubjects ∩ 13 谓词 allowlist，skill `hasReadAccess` admin 可见，D-TIER-03 fail-closed 保持) + Entity SHACL Shapes (entities.ttl + severity 数据驱动 + boot/rollback env 列表化 + quarantine 分流 + governance 三层呈现；7 plans 全绿，verify-phase-45/46.sh 双门禁 exit 0，milestone audit 6/6 需求 + 11/11 wiring)。Full details: `.planning/milestones/v1.6-ROADMAP.md`

<details>
<summary>v1.4 phase records (38-41) — archived reference</summary>

### Phase 38: Foundation — TBox + Memory-map KG + Auto Schema Lift ✅ (2026-09-21)

6/6 plans. `internal/ontology/{emitter,store,induction}` + 8 admin routes + 本体 tab. Hermetic gate 13/13.

### Phase 39: ECL Pipeline — Section-aware Chunker + LLM Entity Extraction ✅ (2026-09-22)

9/9 plans. Section-aware chunker + LLM 抽取 + ABox 双写 KG/Milvus + /admin/pipeline tab. Golden F1 tables 0.9846 / related 1.0.

### Phase 40: Data Governance — SHACL + Provenance + TBox Versioning ✅ (2026-09-23)

10/10 plans. **D-SHACL-02: violations never block boot**（boot SHACL → quarantine 分流 → report cache → `/admin/governance` 三视图）; QueryScoped tier 隔离 (D-TIER-01/03/06/07); provenance 链; TBox changelog/rollback. Hermetic gate 17/17. **This is the quarantine flow Phase 46 wired entity shapes into.**

### Phase 41: Platform Layer — MCP Server + SPARQL Endpoint + Hybrid Retrieval ✅ (2026-09-23)

6/6 plans. MCP stdio (5 tools) + `POST /api/sparql` (X-Skill-Name tier isolation) + Hybrid Retrieval (KG 1-2 hop → Milvus → BGE-rerank) + /admin/platform 三视图.

</details>

---

## 🚧 Current Milestone: v1.7 Open-Source Readiness

**Milestone Goal:** 把"核心引擎产品级、门面个人项目级"的落差补齐——外部用户 clone→跑通→看懂→信任，确立信任边界赛道的开源门面。

**Source:** 2026-10-13 缺口审查（vs Dify/RAGFlow/FastGPT 等明星项目 + 商业标准）。

**Scope:** 14 项需求（FACE-01..03 / CICD-01..03 / DEMO-01..03 / REL-01..04）→ 5 phases（47-51），coverage 14/14。v1.6 遗留债务中的 D-46-DEF-02、audit WARNING-1、BL-01..04 由 REL 需求吸收进 Phase 47/51；其余遗留项（D-45-DEF-01/02/03、D-46-DEF-01、owl:Class quirk WARNING-2）在 Phase 47 逐项落处置记录。

**Standards（继承 v1.4–v1.6）：** 零硬编码；Go handler ≤200 行；不静默吞错；涉 web 先产 UI-SPEC；每 phase 配 `verify-phase-NN.sh`；文档表述与构建现实一致（CGO_ENABLED=1 是 go-duckdb 生产构建现实路径）。

## Phases

- [ ] **Phase 47: 清债与可复现基线** - v1.6 遗留债务闭合或显式处置：flaky 测试修复 + rollback 治理缓存 + 警告处置记录，为 CI 全量门与 hermetic 套件铺平不闪红的地基
- [ ] **Phase 48: CI 矩阵** - Go / FE / Docker 三线质量门进 GitHub Actions，每次 PR 公开可见绿灯
- [ ] **Phase 49: Hermetic Demo** - embedded 向量兜底 + stub LLM 路径，clone 后零内网依赖、零 API key 跑通全链
- [ ] **Phase 50: 开源门面** - README 修真 + 架构图 + vs Dify/RAGFlow 对比表 + 社区设施，门面与仓库实况零失真
- [ ] **Phase 51: 发布工程** - goreleaser + tag 触发 GitHub Release，二进制 + release notes 零手工

## Phase Details

### Phase 47: 清债与可复现基线
**Goal**: v1.6 遗留债务全部闭合或显式处置——全量测试门稳定可复现、rollback 治理数据可信、每个遗留警告有去向记录。这是后续所有 phase 的地基：Phase 48 的 CI 全量门和 Phase 49 的 hermetic 套件（跑全量门）都依赖一个不闪红的测试基线。
**Depends on**: Nothing（v1.7 首个 phase）
**Requirements**: REL-01, REL-02, REL-04
**Success Criteria** (what must be TRUE):
  1. `TestDeriveEdgesDeterministic` 连续 6 次运行全绿（浮点 Weight 确定性化，不再受求值顺序/精度抖动影响）——D-46-DEF-02 闭合
  2. rollback 成功后 `/admin/governance` 的 SHACL 报告与重校验结果一致：`GovernanceTab.tsx` 的 `trigger==='rollback'` 死分支收到真实缓存数据并正确渲染，与 `TimelineView.tsx` toast 披露的事实相符——audit WARNING-1 闭合（FE 死分支恢复复用 Phase 46 governance UI 既有契约与数据 shape，非新 UI 建设）
  3. v1.6 全部遗留项（D-45-DEF-01/02/03、D-46-DEF-01、owl:Class quirk WARNING-2、BL-01..04）在处置记录中各有明确去向（v1.7 修复 / 延期+理由 / ack backlog），v1.7 Traceability 表 100% 填充——不改 `.planning/milestones/` 归档
  4. `bash verify-phase-47.sh` 在 WSL CGO_ENABLED=1 下 exit 0，其中全量门（P4 `./internal/...`）连续两跑无 flake
**Plans**: TBD

### Phase 48: CI 矩阵
**Goal**: 外部贡献者的每次 PR 与维护者的每次 push 都被自动化质量门守护——Go / FE / Docker 三线在 GitHub Actions 上公开绿灯，把"666 Go 测试 + 84 FE 测试"的可信度从本机 WSL 搬到公共可见的 CI。
**Depends on**: Phase 47（flaky 修复先落，否则 CI 全量门闪红）
**Requirements**: CICD-01, CICD-02, CICD-03
**Success Criteria** (what must be TRUE):
  1. PR/push 触发 Go CI 并全绿：golangci-lint + `go test ./...` + CGO_ENABLED=1 build+vet（ubuntu runner 原生 gcc 即 WSL CGO=1 惯例的 CI 等价物）
  2. PR/push 触发 FE CI 并全绿：`tsc --noEmit` + `vitest run` 全量
  3. CI 中 backend/frontend 双 Dockerfile 构建成功
  4. 依赖内网服务或 API key 的 live 门禁不进 CI（`.github/workflows/eval-gate.yml` 的 frozen-replay + faithfulness gate 是既有 hermetic 先例，保持不回归），CI 不因内网断连闪红
  5. `bash verify-phase-48.sh` exit 0
**Plans**: TBD

### Phase 49: Hermetic Demo
**Goal**: 外部用户 clone 后不接内网 Milvus/BGE、不配任何 API key，即可把 SkillForge 完整跑起来并完成一轮带引用的问答——"clone→跑通"从碰运气变成保证。本里程碑工作量最大的真实代码改动（检索链现状：Milvus 必须 + BGE-M3 必须 + GLM API key）。
**Depends on**: Phase 47（hermetic 套件跑全量门，基线须先稳定）
**Requirements**: DEMO-01, DEMO-02, DEMO-03
**Success Criteria** (what must be TRUE):
  1. 全新 clone + README quickstart 命令序列（无内网、无 API key、无预装 Milvus/BGE）服务可启动，chat 完成一轮问答并返回结构化结果
  2. 检索链在 embedded 向量兜底模式下返回可引用 chunk，tier 隔离语义（D-TIER-03 fail-closed）在兜底模式下不削弱
  3. 无 API key 时 LLM 环节走 stub 路径（`internal/knowledge/ecl/stub.go` 既有 stub 抽取器先例的延伸），日志/响应如实标注 stub 模式，不静默伪装真模型
  4. quickstart 在 hermetic 模式下逐步实测通过，录制产出真实 demo.gif 素材（反哺 FACE-01）
  5. `bash verify-phase-49.sh` exit 0（含 hermetic 模式端到端）
**Plans**: TBD

### Phase 50: 开源门面
**Goal**: 新访客 5 分钟内理解 SkillForge 是什么、凭什么不同（vs Dify/RAGFlow）、怎么跑起来——门面素材与仓库实况零失真。纯文档/社区设施，无生产代码改动。
**Depends on**: Phase 49（demo.gif 素材与 quickstart 实测结论反哺 README）
**Requirements**: FACE-01, FACE-02, FACE-03
**Success Criteria** (what must be TRUE):
  1. README 与实际一致：`docs/demo.gif` 正常播放（Phase 49 真实运行素材）、示例 skill 数量更正为 7、badge 链接全部有效、构建表述如实（CGO_ENABLED=1 为 go-duckdb 生产构建现实路径，不宣称无 cgo 单 binary）
  2. `docs/architecture.md` 组件图覆盖 agent 内核 / 知识层 / 治理层 / 平台层，新读者可据图定位代码入口；README 含 vs Dify/RAGFlow 差异化对比表，差异点可追溯到仓库实际能力
  3. SECURITY.md / CODE_OF_CONDUCT.md / GitHub issue/PR 模板齐备且从 README 可达
  4. `bash verify-phase-50.sh` exit 0（badge 链接有效性检查 + 文档与仓库实况一致性断言，含示例 skill 计数与实际目录比对）
**Plans**: TBD

### Phase 51: 发布工程
**Goal**: 打 tag 即出正式 GitHub Release——外部用户可下载预构建二进制直接跑，或按 quickstart 自建；发布过程零手工步骤。这是 v1.7 的收官动作。
**Depends on**: Phase 48（CI 构建矩阵为 release workflow 提供基础）、Phase 49（hermetic demo 是二进制里的能力，quickstart 已实测）、Phase 50（README 与发布物一致）
**Requirements**: REL-03
**Success Criteria** (what must be TRUE):
  1. push 版本 tag（如 v1.7.0）自动触发 GitHub Release 创建，附结构化 release notes
  2. Release 资产包含可执行二进制（goreleaser 构建，CGO_ENABLED=1 现实路径下产出），至少一个平台目标在干净环境可启动
  3. 下载的二进制按 README quickstart 可完成一轮 hermetic 问答（与 Phase 49 验证路径一致）
  4. `bash verify-phase-51.sh` exit 0（goreleaser check + release 流程本地演练）
**Plans**: TBD

## Progress

**Execution Order:** 47 → (48 ∥ 49) → 50 → 51

Phase 48 与 Phase 49 均只依赖 Phase 47，可并行推进；Phase 50 依赖 Phase 49 的素材产出；Phase 51 收尾合流。

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 47. 清债与可复现基线 | v1.7 | 0/TBD | Not started | - |
| 48. CI 矩阵 | v1.7 | 0/TBD | Not started | - |
| 49. Hermetic Demo | v1.7 | 0/TBD | Not started | - |
| 50. 开源门面 | v1.7 | 0/TBD | Not started | - |
| 51. 发布工程 | v1.7 | 0/TBD | Not started | - |

---

*Last updated: 2026-10-13 — v1.7 roadmap created（14 REQ → Phases 47-51，coverage 14/14）*
