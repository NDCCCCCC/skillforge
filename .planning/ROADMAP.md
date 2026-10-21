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

**Scope:** 13 项需求（FACE-01..03 / CICD-01..03 / DEMO-01..03 / REL-01..04）→ 5 phases（47-51），coverage 13/13。v1.6 遗留债务中的 D-46-DEF-02、audit WARNING-1、BL-01..04 由 REL 需求吸收进 Phase 47/51；其余遗留项（D-45-DEF-01/02/03、D-46-DEF-01、owl:Class quirk WARNING-2）在 Phase 47 逐项落处置记录。

**Standards（继承 v1.4–v1.6）：** 零硬编码；Go handler ≤200 行；不静默吞错；涉 web 先产 UI-SPEC；每 phase 配 `verify-phase-NN.sh`；文档表述与构建现实一致（CGO_ENABLED=1 是 go-duckdb 生产构建现实路径）。

## Phases

- [x] **Phase 47: 清债与可复现基线** - v1.6 遗留债务闭合或显式处置：flaky 测试修复 + rollback 治理缓存 + 警告处置记录，为 CI 全量门与 hermetic 套件铺平不闪红的地基
- [x] **Phase 48: CI 矩阵** - Go / FE / Docker 三线质量门进 GitHub Actions，每次 PR 公开可见绿灯
- [ ] **Phase 49: Hermetic Demo** - embedded 向量兜底 + stub LLM 路径，clone 后零内网依赖、零 API key 跑通全链
- [ ] **Phase 50: 开源门面** - README 修真 + 架构图 + vs Dify/RAGFlow 对比表 + 社区设施，门面与仓库实况零失真
- [ ] **Phase 51: 发布工程** - goreleaser + tag 触发 GitHub Release，二进制 + release notes 零手工

## Phase Details

### Phase 47: 清债与可复现基线

**Goal**: v1.6 遗留债务全部闭合或显式处置——全量测试门稳定可复现、rollback 治理数据可信、每个遗留警告有去向记录。这是后续所有 phase 的地基：Phase 48 的 CI 全量门和 Phase 49 的 hermetic 套件（跑全量门）都依赖一个不闪红的测试基线。
**Depends on**: Nothing（v1.7 首个 phase）
**Requirements**: REL-01, REL-02, REL-04
**Success Criteria** (what must be TRUE):

  1. `TestDeriveEdgesDeterministic` **边方向**确定性化，`-count=100` 全绿 + 一次 `-shuffle=on` 全绿——D-46-DEF-02 闭合。实测根因**不是**求值顺序/精度问题，而是 `keyword_cooccur` **边方向非确定性**：`internal/knowledge/graph_edges.go::ComputeEdges` 的 `sources` 由 `sourceTokens` 的 **map 迭代序**构造，`i<j` 配对把 `From` 取自 `sources[i]`、`To` 取自 `sources[j]`，切片顺序即决定边方向，而 `EdgeGraph.Neighbors` 是**有向**遍历；测试末尾的排序键 `(From, To, EdgeType)` 无法把 `(1→4)` 与 `(4→1)` 归一。修复 = 配对前对 `sources` 确定性排序（沿用同文件 `computeCitationEdges` 的既有做法）+ 最终比较器补 `Weight` 作第四排序键（`sort.Slice` 不稳定，`citation_cooccur` 同一 `(From,To)` 可因不同 audit 行产生不同 Weight）。证据分布：修复前 `-count=200` 实测两次分别 34 败 / 50 败（**17–25%**，非早期记录的「连续 6 次」）且失败**全部**落在 `From`/`To`/`EdgeType` 错位断言，`Weight` 断言 **0 次失败**——测试的严格比较是对的，不允许用放宽 `InDelta` 容差或改无序比较来换取绿灯（原「连续 6 次」门槛在 ~20% flake 率下仅约 47–67% 置信度，**不足以**作为确定性证据）
  2. rollback 成功后 `/admin/governance` 的 SHACL 报告与重校验结果一致：`GovernanceTab.tsx` 的 `trigger==='rollback'` 死分支收到真实缓存数据并正确渲染，与 `TimelineView.tsx` toast 披露的事实相符——audit WARNING-1 闭合（FE 死分支恢复复用 Phase 46 governance UI 既有契约与数据 shape，非新 UI 建设）
  3. v1.6 全部遗留项（D-45-DEF-01/02/03、D-46-DEF-01、owl:Class quirk WARNING-2、BL-01..04）在处置记录中各有明确去向（v1.7 修复 / 延期+理由 / ack backlog），v1.7 Traceability 表 100% 填充——不改 `.planning/milestones/` 归档
  4. `bash verify-phase-47.sh` 在 WSL CGO_ENABLED=1 下 exit 0，其中全量门（P4 `./internal/...`）**连续两跑无 flake（仅作冒烟门槛，不充分）**，且门内含一段**确定性强度段**并全绿：① `./internal/knowledge/` 的 `go test -run '^TestDeriveEdgesDeterministic$' -count=100` 全绿；② 对本 phase 触及的包跑一次 `go test -shuffle=on`。该门**必须空 waiver**（不得用 `t.Skip` / `WAIVED` 维持表面绿灯——CI 裸跑 `go test ./...` 不读本地 waiver 机制，残余失败只能落成显式记录而非 shell 放行）

**Plans:** 8 plans

Plans:
**Wave 1**

- [ ] 47-01-PLAN.md — REL-01 边方向确定性（map 迭代序消除 + `Weight` tie-break + 100 轮全结构断言）
- [ ] 47-02-PLAN.md — REL-02 域层：`RollbackTBox` 把已算出的重校验报告写入 SHACL 缓存 + 读路径注释纠偏
- [ ] 47-03-PLAN.md — emitter 两测试 hermetic 化（temp fixture，改断言不改 emitter 契约）
- [ ] 47-05-PLAN.md — D-46-DEF-01：DetailPanel SC-5 测试对齐「文档」模式（改测试不改组件）

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 47-04-PLAN.md — REL-02 FE：`TimelineView.onRollbackComplete` + `GovernanceTab` 共享 refetch + 新测试
- [ ] 47-07-PLAN.md — REL-04：BL-01..04 补录 Traceability + SC-1/REL-01 根因措辞更正

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 47-06-PLAN.md — `47-DEBT-DISPOSITION.md` 19 行主表 + STATE.md 逐 ID 索引

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 47-08-PLAN.md — 仓库根 `verify-phase-47.sh`（零 waiver + 确定性强度段 + FE 段）+ 45/46 waiver 退役 + clean-clone 复现 + 陈旧 harness 清理

### Phase 48: CI 矩阵

**Goal**: 外部贡献者的每次 PR 与维护者的每次 push 都被自动化质量门守护——Go / FE / Docker 三线在 GitHub Actions 上公开绿灯，把"666 Go 测试 + 84 FE 测试"的可信度从本机 WSL 搬到公共可见的 CI。
**Depends on**: Phase 47（flaky 修复先落，否则 CI 全量门闪红）
**Requirements**: CICD-01, CICD-02, CICD-03
**Success Criteria** (what must be TRUE):

  1. PR/push 触发 Go CI 并全绿：golangci-lint + `go test ./...` + CGO_ENABLED=1 build+vet（ubuntu runner 原生 gcc 即 WSL CGO=1 惯例的 CI 等价物）
  2. PR/push 触发 FE CI 并全绿：`tsc --noEmit` + `vitest run` 全量
  3. CI 中 backend/frontend 双 Dockerfile 构建成功
  4. 依赖内网服务或 API key 的 live 门禁不进 CI（`.github/workflows/eval-gate.yml` 的 frozen-replay + faithfulness gate 是既有 **live** workflow，保持维护者 / 同仓行为不回归；`--frozen` 在打分前即 bootstrap BGE-M3 / Milvus / GLM，缺 secret 时 `log.Fatalf`，只有 `--gate-check` 阶段 hermetic，fork / Dependabot PR 得到带说明的 skip notice 而非红色失败或假绿），CI 不因内网断连闪红
  5. `bash verify-phase-48.sh` exit 0

**Plans**: 26/26 executed (48-01..48-15 original + gap closures; 48-16..48-19 round-4; 48-20..48-22 round-5; 48-23..48-26 round-6 — round-6 closed 2026-10-21)

**Blocked on**: Resolved — the four-green run was obtained. Run 39380373930 on head `630f875` (2026-10-20, public `main`) concluded `go=fe=docker=verify=success`; canonical fragment: `ci.yml total runs: 6; first all-green: 39380373930`. The three source fixes cleared the three classified reds: 48-16 materialised the `-list` output to `$RUNNER_TEMP/go-test-list.txt` (P5a SIGPIPE), resolved the Sparkline test subject through `import.meta.url` (Windows-path ENOENT on Linux), and moved the backend image to a glibc base and runtime (duckdb's `backtrace` / `malloc_trim` / `__res_init`). The earlier runs (runs 1-5) were all red — the first four on those classified build causes, and run 5 on a load-sensitive DuckDB reader-block ratio flake that was fixed at the source (best-of-5 minimum sampling) so run 6 could land. CICD-01/02/03 are Complete against run 39380373930. WR-06 remains closed-by-evidence on run-3 (lint green with `only-new-issues: true` alone on a fast-forward push). Round-6 update (2026-10-21, plan 48-26): the round-6 tree (head `c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67`, carrying 48-23..48-25's gate and record work) was pushed to public `main` as a fast-forward (`630f875..c060c84`, no force); run 7 of 7 — id `39525757252`, event `push` — concluded `go=success fe=success docker=success verify=success` and is the first public run of the round-6 gate, closing the round-5 local-green-only record boundary. go/fe/docker job spans are byte-identical between `630f875` and `c060c84` (the ci.yml delta is the push-trigger comment block, the verify-job header comment block, and the verify install step's declared env map), so CICD-01/02/03 stay Complete against run 39380373930 without re-flipping. Canonical fragment: `ci.yml total runs: 7; first all-green: 39380373930; run 7 (head c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67, id 39525757252, event push) is the first public run of the round-6 gate (four-green: go=success fe=success docker=success verify=success)`.

**Wave 1** — tracer slice

- [x] 48-01-PLAN.md — tracer: checkout-first composite action (D-22/D-30) + v2 `.golangci.yml` (D-01..D-08) + `ci.yml` `go` job (CICD-01) + runnable zero-waiver `verify-phase-48.sh` 分片 P1–P4

**Wave 2** — remaining gates (parallel; disjoint file ownership)

- [x] 48-02-PLAN.md — `fe` job (CICD-02) + `docker` job 双 Dockerfile build / `push: false` / 分离 GHA cache scope (CICD-03) + 分片 P5–P8
- [x] 48-03-PLAN.md — `eval-gate.yml` fork-PR/Dependabot 门禁 + `::notice::` skip-notice (D-09/D-10/D-11) + `go-version-file: go.mod` 迁移 (D-17) + ROADMAP SC-4 措辞更正 (D-12)

**Wave 3** — full coverage + CI-run proof

- [x] 48-04-PLAN.md — 分片 P9–P11（eval-gate 门禁 / SC-4 措辞 / verifier 自证 P23）**已落地**；branch protection one-way door checkpoint **已决策 = Option A（不加保护）**；**真实 GitHub Actions run 三 job 全绿未取得**（本机 git 传输到 github.com 不通，PR-to-main 路径因 `origin/main` 与本分支无共同历史而封闭）→ CICD-01/02/03 保持 Pending

**Gap closure**（48-VERIFICATION.md `gaps_found`：3/9 must-haves；19 条 review findings 已全部终态处置 = 18 fixed / 1 deferred〔WR-06，48-11 option D〕，见 `48-REVIEW-DISPOSITION.md`）

- [x] 48-05-PLAN.md — 周界分片按文件扫描（CR-01/WR-05/IN-02/IN-03）：P8 的 `paths:`/`secrets.` 规则下沉到逐文件循环、枚举 `*.yml` + `*.yaml`、registry token 大小写不敏感、显式 secret 白名单 + 变异探针
- [x] 48-06-PLAN.md — eval-gate live 门完整性（WR-03/WR-08）：replay step 加 `shell: bash` + `set -o pipefail`；三条 live secret 由 job 级 env 收到两个消费 step
- [x] 48-07-PLAN.md — step 级 `if:` 拒绝（WR-01）覆盖 P2/P3/P6/P7；P2 env 并集（IN-05）；计数锚定正则（IN-04）；go job checkout `persist-credentials: false`（IN-10）
- [x] 48-08-PLAN.md — P9 断言归一化表达式（WR-07）并把 skip-notice 改成 gate 的真补集（WR-02），两者关系由 P9 机械断言
- [x] 48-09-PLAN.md — P4 sentinel D 正向 revive 探针（WR-04）；sentinel A 证据改为归因匹配（IN-01）；分区账目改由 body 自写 begin 标记（IN-06）；preset 注释补全（IN-09）
- [x] 48-10-PLAN.md — 以变异证据重闭 9 行 open threat（`threats_open: 0` / `status: verified`）；19 条 findings 终态处置 + deferred 清单；ROADMAP/REQUIREMENTS 与 48-11 记录的 M1 状态对齐
- [x] 48-11-PLAN.md — WR-06/IN-08/IN-07 三项决策已记录（option D / F / A，IN-08+IN-07 已实现）+ Class A 首次真实 CI run 的 run recipe（M1: BLOCKED，不伪造运行）

**Gap closure round 3**（48-VERIFICATION.md round-2 再验证 `gaps_found` 5/9：R2-CR-01 supply-chain、R2-WR-01/03 verifier 两处 vacuous PASS、R2-WR-05 Gate-check secrets、四条 advisory；48-12..48-15）

- [x] 48-12-PLAN.md — ci.yml `curl|sh` bootstrap → module-proxy `go install` + `[ -x ]` postcondition（R2-CR-01/WR-02）；PyYAML pin、verify checkout `persist-credentials: false`、四 job header、P5a `-list` 防改名 guard（R2-IN-01/02/03、R2-WR-04）；eval-gate `Gate check` 撤除三个不可消费 secret（R2-WR-05）
- [x] 48-13-PLAN.md — verifier 成功 token 账目：`done_partition` 要求 begin+ok、去掉 `:-0` 默认、P11 四方计数（R2-WR-03，M8 变异转红）；P2/P3 pin verify job 契约与 48-12 各修复（R2-WR-01，M9/M10 变异转红）；五项沙箱变异演练
- [x] 48-14-PLAN.md — 48-SECURITY.md 补录 supply-chain 行 T-48-47（引用 48-11 threat_flag）+ round-2 closure 段 + audit trail；传输事实更正（ROADMAP Blocked-on / SECURITY "does NOT establish"）；PyYAML 行补进 Package Legitimacy Audit
- [x] 48-15-PLAN.md — Class A：执行 48-11 持久化的 M1 run recipe（历史决策 checkpoint〔one-way〕→ 本机 SSH push → 三次 runners 触发后闭合证据 + 三处真实持久红故障分类）；CICD-01/02/03 按配方 blocked 分支保持 Pending；详见 `48-15-SUMMARY.md`。

**Gap closure round 4**（48-VERIFICATION.md round-3 再验证 `gaps_found` 9/15：SC-1/2/3 三线仍红、truth 6 verifier 诚实性、truth 11/15 记录对账；48-16..48-19 planned）

- [x] 48-16-PLAN.md — 三处源侧/工作流侧修复：determinism `-list` 输出落盘（去掉管道/SIGPIPE）、Sparkline 测试改为 `import.meta.url` 相对解析、`Dockerfile.backend` 切换 glibc 基础镜像与运行时
- [x] 48-17-PLAN.md — verifier 诚实性：每个分区的 success token 绑定探针自产证据（日志非空 + 自产 token），`fe`/`docker` 增加 job 级 `if:` 拒绝，P2 pin 住 48-16 的落盘形状
- [x] 48-18-PLAN.md — 记录对账：M1c 根因从 "runner environment" 改为 "workflow pipeline shape"（VALIDATION/SECURITY/ROADMAP）、运行次数三改四、STATE line 117 失实原因标注、round-2 disposition ledger 关闭并提交
- [x] 48-19-PLAN.md — 推送 main 并取一次 `go=fe=docker=verify=success` 运行（run 39380373930），按该 run id 翻转 CICD-01/02/03，并把 VALIDATION/SECURITY/STATE 对账到同一次运行

**Gap closure round 5**（48-VERIFICATION.md round-4 再验证 `gaps_found` 13/15：truth 12 断言级 token 共享〔M13〕、truth 15 R1 ledger 尾段未标注；4 条 advisory；48-20..48-22 planned）

- [x] 48-20-PLAN.md — verifier 断言级 token 第一部分：`ok_partition` 接受 token 列表；P2/P3 拆为 4+4 个 per-assertion token，P1 拆为 3 个，P4 拆为 4 个；四个删除演练必须 red 并点名分区
- [x] 48-21-PLAN.md — verifier 断言级 token 第二部分：P6/P7/P9 各拆为 per-assertion token；P11 else-detector 改为先消费 range 再 grep（WR-01）；删除演练 + banner-in-else 演练
- [x] 48-22-PLAN.md — trust-surface 规则改为解析后文档驱动（quoted `"paths":` / `paths :` / `toJSON(secrets)` / quoted-hash secret，P11 fixtures 12 个）；ci.yml linter install 声明 GOTOOLCHAIN/GOSUMDB/GOPROXY；版本双写说明；R1 ledger 历史标注 + T-48-70..T-48-79 注册

**Gap closure round 6**（48-VERIFICATION.md round-5 再验证 `gaps_found` 13/15：truth 12 P11 fixture 循环把删除折叠成 PASS〔WR-R5-01，R11/R12 复现〕、truth 9 install env 无 pin 且 T-48-78 条款失实〔WR-R5-02，R13/R14 复现〕；6 条 advisory；48-23..48-26 planned）

- [x] 48-23-PLAN.md — 两个 must-have gap：P11 fixture 循环按 fixture 断言拒绝理由 + file-presence guard + 五个 per-class token（WR-R5-01，R11/R12 转红）；publication token 引号变体 + dyn-secrets fixture（IN-R5-01 / IN-R5-02a，fixtures 12→14）；P3-VERIFYJOB 精确相等断言 install-step env map + P3-INSTENV-OK token（WR-R5-02，R13/R14/断言删除均转红）
- [x] 48-24-PLAN.md — advisory 硬化：P4 config-verify 组自有证据 P4-CONFIG-OK（IN-R5-02b）；P1 `pull_request_target` 扫描改注释剥离文本、双向演练（IN-R5-03）；死代码清理（IN-R5-05）+ 全量 R1-R14/G1-G4 演练回归记录
- [x] 48-25-PLAN.md — 记录半场：ci.yml push 触发注释更正为 run-3 实证（IN-R5-04，comment-only、job spans 不变）；48-SECURITY.md T-48-78 失实条款更正为 P3-INSTENV-OK pin + T-48-80..T-48-82 注册（34 行 0 open）；48-REVIEW-DISPOSITION.md 对账（15 条 round-4 rows 终态 + 7 条 round-5 rows）；COVERAGE.md round 标注
- [x] 48-26-PLAN.md — Class-A：快进推送 main 重立 round-6 verify job 公共绿灯（闭合 round-5 的 local-green-only 记录边界），go/fe/docker job spans 断言不变、CICD 状态不重翻；VALIDATION/SECURITY/ROADMAP/STATE 四处 run 记录对账（total 由 gh run list 现读，first all-green 保持 39380373930）——已完成 2026-10-21：快进推送 `630f875..c060c84`，run 7/7（id `39525757252`）四绿，四处记录对账至 total 7

**Round 7 — gap closure**（`48-ANALYSIS.md` + `48-VERIFICATION.md` 复核后的 CR-01/WR-01/WR-02）

- [ ] 48-27-PLAN.md — CR-01：`on:` 触发判定改解析优先（quoted key / flow 形态）+ 两处注释剥离共用一个 quote-aware helper（`:281` 与 `:1207`）+ 新 token `P1-NO-PRT-PARSED` + flow-style 引号内 `#` 正向 canary；WR-01：三条权限规则各配隔离 fixture 与专属 class token（14→17 fixtures、5→8 token，`yaml-ext` 改缴 `P11-YAMLEXT-OK`）；WR-02：P4 config-verify 证据行改为 append 前实测字节数；演练扩到 R15-R21（含 R19b）

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
| 47. 清债与可复现基线 | v1.7 | 8/8 | Complete | 2026-10-19 |
| 48. CI 矩阵 | v1.7 | 24/26 | In Progress |  |
| 49. Hermetic Demo | v1.7 | 0/TBD | Not started | - |
| 50. 开源门面 | v1.7 | 0/TBD | Not started | - |
| 51. 发布工程 | v1.7 | 0/TBD | Not started | - |

---

*Last updated: 2026-10-13 — v1.7 roadmap created（13 REQ → Phases 47-51，coverage 13/13）*
