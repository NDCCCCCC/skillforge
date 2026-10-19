# Phase 47 — v1.6 遗留项逐 ID 处置主表（47-DEBT-DISPOSITION）

**Created:** 2026-10-19
**Owner:** plan 47-06（REL-04 的一部分 —— 遗留项处置记录）

## 本文件的角色

这是 Phase 47 **唯一权威**的遗留项处置记录。`.planning/STATE.md` § `Deferred Items` 是**索引**，每行链回本文件，**不构成第二份记录**（D-03c：STATE.md 在 v1.7 roadmap 会话已被紧凑重写过一次，索引必须能存活）；`.planning/REQUIREMENTS.md` § `Backlog Items Carried From v1.6` 同样只声明「去向与本表一致」，不复制证据。

**去向（`去向`）取值闭集 —— 只有这三个值，新增第四个值即视为记录失真：**

| 去向 | 含义 |
|---|---|
| `v1.7 修复` | 本 phase 内已由某个 plan 落地修复，且有可引用的 commit SHA |
| `延期+理由` | 本 phase 不修，理由写在证据格内，且指明重新立项的前提 |
| `ack backlog` | 已知、非缺陷、登记入 backlog 不修（Info 级观察） |

**机器校验约定（供 plan 47-08 的 `verify-phase-47.sh` 门段使用）：** 每个 ID 恰好一行；行首形如 `| \`<ID>\` |`；`去向` 列取上述三个值之一且非空；`证据` 列必须含至少一个文件路径或 commit SHA，不得为空、不得为「以后再说」式空承诺。

---

## 1. Bucket summary

| 去向 | Count | Closure path |
|---|---|---|
| `v1.7 修复` | 6 | D-45-DEF-01、D-45-DEF-02、D-46-DEF-01、D-46-DEF-02、WARNING-1、BL-04 —— 各由 plan 47-01..47-05 / 47-08 落地，证据列为 commit SHA |
| `延期+理由` | 6 | D-45-DEF-03、WARNING-2、BL-01、BL-02、BL-03、D-05 —— 各自附「重新立项的前提」，无一是含糊的「以后处理」 |
| `ack backlog` | 7 | IN-01..IN-07 —— 46-REVIEW Info 级观察，非缺陷，登记 backlog |
| **Total** | **19** | |

分布：6 已修 / 6 显式延期（各带理由与前提）/ 7 登记 backlog。**零项被静默丢弃，零项留作空白去向。**

---

## 2. Per-ID master table

| ID | 来源 | 去向 | 证据 |
|---|---|---|---|
| `D-45-DEF-01` | `45/deferred-items.md:7`（emitter `TestEmitSubCategoryInstanceCount` 6 vs 7） | `v1.7 修复` | plan 47-03。修的是**断言**不是实现：`emitter.go` 契约「每个 `sql-guides` 子目录发一个 SubCategory」正确，是 commit `1aaf853` 把 `其他/` 下 6 个文件移入 `操作手册/` 让语料目录数变 6，导致断言值过时。`22f8480` 把 `TestEmitSkillInstanceCount` 改为 3-skill `t.TempDir()` fixture（断言 3），`e88d6ca` 把 `TestEmitSubCategoryInstanceCount` 改为合成 KB 树（`sql-guides/取数口径` + `sql-guides/操作手册`，控制组 `authority/nested-control`，断言 `SubCategory == 2` 且 `KBCategory == 2`）。非空洞证明：把 `emitter.go` 的 `loadKBCategories` 里 `sql-guides` 守卫改成恒真后测试立刻红（3 ≠ 2）。`git archive HEAD` 干净 checkout 下 `./internal/ontology/...` 全绿。附带：`verify-phase-45.sh` / `verify-phase-46.sh` 里的 `waiver_ok` / `WAIVED_FAIL` 放行机制随本项作废，plan 47-08 归零 waiver 时一并退役。 |
| `D-45-DEF-02` | `45/deferred-items.md:8`（`internal/server/platform/verify_main.go` 自 Phase 44 起 runtime-stale） | `v1.7 修复` | plan 47-08 完成。落点是**清理工作树**而非修复（47-CONTEXT D-04d）。**本行不存在可引用的「删除 commit SHA」，这是事实而非遗漏**：该文件从未被跟踪 —— `git ls-files internal/server/platform/verify_main.go` 输出为空，`git log --all -- internal/server/platform/verify_main.go` 在整个仓库历史中无任何 commit —— 因此删除它不产生任何 diff —— 记录该删除的 commit 是 `7382d69`（它携带的就是本次证据格回填；被删文件本身不在其 diff 内）。删除前的取证：755 行，SHA-256 `8c694fc8f7fb79388acab35c26e6fd8c84b1d33dbe625751a8dcf1d9b7f3e83e`，mtime 2026-09-23 15:00:49 +0800（Phase 44 之后无人再动）。**runtime-stale 的理由**（45-REVIEW 已记录）：pre-44 fixture、owl:Class IRIs、raw counts、label-as-schema 断言四类内容全部与 Phase 45 之后的 emitter / `isSchemaTriple` 实现脱节；它能被本地 `go build ./...` 编译只因它在磁盘上，而 `git archive HEAD` 的干净 checkout 根本不含它 —— 这正是本地/CI 差异的来源。**干净 checkout 可复现性（D-04a）**：一次 `wsl.exe` 调用内 `D=$(mktemp -d)` → `git archive HEAD | tar -x -C "$D"` → 在 `$D` 内逐条执行并打印原始 rc，得到 `RC_build=0` / `RC_test_internal=0` / `RC_det100=0`（`-run '^TestDeriveEdgesDeterministic$' -count=100`）/ `RC_emitter=0`；该干净树只有 **3** 个被跟踪的 `skills/` 目录与 **6** 个 `knowledge/sql-guides/` 子目录，任何仍钉住外部语料的断言都会在这里红。命令原文与 rc 见 `47-08-SUMMARY.md`。**其余 untracked 遗留物（记录但不删除）**：`internal/server/skillforge-server-test-*` 共 **9** 个目录仍然存在（1315038830 / 2346041323 / 2516342759 / 282873371 / 2853703676 / 2861472387 / 2967410029 / 4158293520 / 480999863），是历史测试遗留的临时目录，**不在 D-04d 范围内故未删除**；`verify-phase-47.sh` Task 3 的验收命令把它们作为唯一豁免前缀，收窄该豁免后 `git status --porcelain internal/` 无其他 stray。harness **重写不在立项范围**：没有任何消费者引用它（`grep -rn verify_main internal/ cmd/ web/` 无调用点），重写即凭空造物（`45/deferred-items.md` 已将其登记为 BL-05 backlog 候选）。 |
| `D-45-DEF-03` | `45/deferred-items.md:15-23`（45-REVIEW 的 WR-01 / WR-02 / IN-02） | `延期+理由` | 三条子项各有独立理由，均非「以后再说」：**WR-01**（`store.go:556-561` schema passthrough 早于 `:568-577` 的 S/P/O pattern filter）—— 把 pattern filter 同样施加到 schema 行会改变 count 语义，而 count 敏感的 fixture 被钉在 17/10，改语义需要它自己的决策记录（`45/deferred-items.md:19`）；**WR-02**（`admin_ontology.go:214-216` 的 `kg/query?skill=<unresolvable>` fail-open，nil tiers → Path A 全量无过滤）—— 需要与姊妹 `/api/sparql` 的 401-on-nil 姿态对齐，属 fail-closed 姿态的取舍决策，先例见 `45/deferred-items.md:20`；**IN-02**（`isSchemaTriple` 的 OWL 分支查 `skf:domain`/`skf:range`，而 emitter 发的是 `rdfs:domain`/`rdfs:range`，另有 `skf:Property`/`skf:NamedIndividual` 死检查）—— 方向是 fail-closed（过度过滤），当前无生产影响，改它属于整族 IRI 对齐、须与 owl:Class 半边（WARNING-2）一起立项（`45/deferred-items.md:21`）。重新立项前提：与 WARNING-2 合并为一次 schema 可见性语义决策记录。 |
| `D-46-DEF-01` | `46/deferred-items.md`（`DetailPanel.test.tsx` SC-5 空态 hero 测试确定性失败 12/13） | `v1.7 修复` | plan 47-05。根因是断言漂移而非实现错误：`web/src/components/graph/KnowledgeGraphBrowser.tsx:75` 自 quick-260929-g7x 起默认 `useState<GraphViewMode>('ontology')`，而空态 hero 的渲染条件是 `mode === 'document'`，测试从未选 mode，于是观察到的是「本体暂无数据」。`5f9459a` 让测试先点击用户可见的 `graph-mode-document` 再断言，并额外钉 `知识库为空` 文案（严格强于原断言）。`KnowledgeGraphBrowser.tsx` 零改动（`git diff --stat web/src/components/` 空）。变异验证：把 hero 的渲染守卫改成 `mode === 'document' && false` 后测试立即红，证明断言非烟囱。FE 全量基线 **688 tests / 85 files / 0 failed / 0 skipped**（47-04 补 `TimelineView.test.tsx` 后由 685/84 上升）。无 `skip` / `todo` / waiver。 |
| `D-46-DEF-02` | `46/deferred-items.md`（`TestDeriveEdgesDeterministic` flaky，约 1/6 概率红） | `v1.7 修复` | plan 47-01（**REL-01 由此行闭合**）。根因不是浮点 Weight，而是 `internal/knowledge/graph_edges.go` 用 `for src, tokens := range sourceTokens` 构造 `sources`，`i<j` 的 Jaccard 配对让 `From`/`To` 随 map 迭代序翻转。`0ecd9c3` 在配对前排序 `sources`，把 `groups` 改成 map→key slice→`sort.Strings`→消费（沿用同文件 `computeCitationEdges` 既有先例），并给最终比较器加 `Weight` 第四键。`a05929a` 把测试加固为 100 次调用 + 整体结构相等（删掉全部 `InDelta`）。实测：修复前 `-count=200` 两跑分别 34 败 / 50 败（**17–25% flake 率**），失败全落在 `From`/`To`/`EdgeType`，**`Weight` 零失败**；修复后 `-count=200`、`-count=100`、`-shuffle=on`（knowledge 包及 knowledge+ontology）全 exit 0。 |
| `WARNING-1` | `v1.6-MILESTONE-AUDIT.md:30`（rollback 后治理报告缓存不刷新；`SetShaclReport` 唯一调用点是 boot；FE `trigger==='rollback'` 标签为死分支） | `v1.7 修复` | **REL-02 由此行闭合**，后端与前端两半各一个 SHA。**后端 `e7ca19a`**（plan 47-02）：`RollbackTBox`（`internal/ontology/induction/induction.go`）本已跑完 `ValidateWithTrigger(..., "rollback")` 却只消费 `ErrorCount` 丢弃报告，现在把**同一份** report 经 `m.kg.SetShaclReport(report)` 写进缓存，与版本/指针/快照写入同处一个 `m.mu` 临界区（无 TOCTOU，零额外校验开销）；`d3f86c2` 把两个分支都钉住（成功分支断言 `Trigger == "rollback"` 且 `ErrorCount` 等于返回值；失败分支先种入 boot 报告再断言其幸存 —— 变异验证非破坏性）；`c90f91f` 修正了 `admin_governance.go` / `server.go` 里描述了「并不存在的写入方」的读路径注释。**前端 `197421e` + `bf1c1e0`**（plan 47-04）：`TimelineView` 新增可选 `onRollbackComplete`（成功 toast 之后触发），`GovernanceTab` 抽出 `refreshShaclReport` 并同时挂到 `onRetry` 与 `onRollbackComplete`，使 banner 的「回滚后」分支从死分支变为可达；jsdom trace：`（启动时）… error 1` → `（回滚后）… error 3`，toast `已回滚至 v2（当前 v4）· 新增 2 条 violation`，恰好 2 次 SHACL GET。`abbb594` 补测试并证明「refetch 抛错不能把已完成的回滚翻成失败 toast」（变异验证）。 |
| `WARNING-2` | `v1.6-MILESTONE-AUDIT.md:25`（`owl:Class`-vs-`skf:Class` 声明 quirk：`skf:Skill a owl:Class` 因 `owl:Class` 不在 `schemaClasses` 而在 skill 视图 tier-invisible） | `延期+理由` | 证据：`v1.6-MILESTONE-AUDIT.md:25` 的原文 + 45-02 的 conformance 注释（该 quirk 在 Phase 45-02 已被记录，Phase 47 未改动任何相关文件）。理由：**「`owl:Class` 是否并入 `schemaClasses`」是一条 tier 可见性语义决策，影响面覆盖 schema 判定与 skill 视图过滤两条路径**，不是一行开关；admin baseline 不受影响，因此当前无用户可见缺陷，不值得在没有独立决策记录的情况下顺手改。与 D-45-DEF-03 的 WR-01 / IN-02 同族（同一 `isSchemaTriple` / schema 可见性面），重新立项前提：三者合并为一次「schema 可见性语义」决策记录后一起处理。 |
| `BL-01` | `v1.6-REQUIREMENTS.md:40`（双 resolver 去重：`resolveSkillTiers` vs `resolveSkillTiersFromKG`） | `延期+理由` | 定义逐字引自归档 `.planning/milestones/v1.6-REQUIREMENTS.md:40`（只读引用，未修改）；去向在 `.planning/REQUIREMENTS.md:79` 记录为 `Backlog（无 owner phase）` / `Deferred`。理由：这是**带行为语义的重构**（两个 resolver 的边界如何划分本身是设计问题），当前没有可复现的缺陷，且 Phase 47 的全量门已归零，没有证据表明它会阻碍本 milestone。重新立项前提：单独立项并给出两个 resolver 的职责边界。 |
| `BL-02` | `v1.6-REQUIREMENTS.md:41`（server 包 lint 清扫） | `延期+理由` | 定义引自 `.planning/milestones/v1.6-REQUIREMENTS.md:41`；去向在 `.planning/REQUIREMENTS.md:80` 记录为 `Backlog（Phase 48 lint 落地后立项）` / `Deferred`。理由：这一项**当前没有可定义的范围** —— 它是「lint 报告」而不是「缺陷清单」，范围只有在 `golangci-lint` 真正跑起来并产出一份具体报告之后才成立；Phase 47 全量门里 `go vet ./internal/...` 已是绿的（无新告警），凭猜测划定 lint 范围只会制造假债。重新立项前提：Phase 48 把 `golangci-lint` 接入 CI 并产出首份 server 包报告，届时按报告逐条立项。 |
| `BL-03` | `v1.6-REQUIREMENTS.md:42`（live 抽取 prompt 对齐 writes-only 的残余微调） | `延期+理由` | 定义引自 `.planning/milestones/v1.6-REQUIREMENTS.md:42`；去向在 `.planning/REQUIREMENTS.md:81` 记录为 `Backlog（needs-live）` / `Deferred`。理由：needs-live（内网 Milvus / BGE / GLM），而「内网服务间歇断」正是 `STATE.md` § `Blockers/Concerns` 已记录的环境约束，也是 v1.7 要用 hermetic 化削弱的依赖；在基线未固化前跑 live 抽取只会引入环境噪声结论。重新立项前提：Phase 49 的 hermetic 套件落地后，在可复现环境下重跑 live 抽取再评估 prompt 微调。 |
| `BL-04` | `v1.6-REQUIREMENTS.md:43`（DetailPanel / emitter 两个 deferred 测试） | `v1.7 修复` | 两半各由一个 plan 闭合。**emitter 半**（plan 47-03）：`22f8480` + `e88d6ca`，两个实例计数测试改为 `t.TempDir()` fixture 并断言 fixture 自身的计数（Skill = 3、SubCategory = 2、KBCategory = 2），生产文件 `emitter.go` 未改一字，见 D-45-DEF-01 行。**DetailPanel 半**（plan 47-05）：`5f9459a`，SC-5 空态断言先点击 `graph-mode-document` 并钉 `知识库为空` 文案，见 D-46-DEF-01 行。`.planning/REQUIREMENTS.md:82` 已把 BL-04 记为 `Phase 47` / `Complete`。 |
| `D-05` | `47-CONTEXT.md` D-05（boot 校验会真正执行 `Quarantine`，rollback 不会） | `延期+理由` | **已知限制，本 phase 明确不修。** 事实：`cmd/server/main.go:941-945` 在 boot 路径按 `report.Quarantined` 逐条调用 `ontologyKG.Quarantine(t)`，而 `RollbackTBox` 的 Step 11 只写缓存报告、**不执行隔离**（`internal/ontology/induction/induction.go`）。后果：rollback confirm 对话框的「失效引用实体将进入隔离区」与治理页 banner 的「N 个实体已隔离」**在 rollback 路径上并不属实**。已记录于：`47-02-SUMMARY.md` 的 Notes、`47-04-SUMMARY.md` 的 Notes（两处均确认 `window.confirm` 与 toast 文案按 D-02e 原样保留、未被夹带修改），以及 `induction.go` 新增写入点的代码注释。理由：修它会**改变 rollback 的副作用面**（回滚后实体被移出主图），属独立决策而非缓存一致性问题，夹带进 REL-02 会让「报告与重校验一致」的验收范围失真（D-05b）。47-04 的 trace 中该失真可被肉眼复现。重新立项前提：单独立项评估 rollback 是否应执行隔离；若判定**不**执行，则改为修正文案而非补执行。 |
| `IN-01` | `46-REVIEW.md:177-183` § Info | `ack backlog` | 被隔离实体的 warning 级违规从报告中整体消失：`internal/ontology/shacl/validate.go:259-264` 的「Collect pure warnings」只收无 error 实体的 warnings，同时有 1 error + 1 warning 的实体其 warning 既不进 `Warnings` 也不计入 `WarningCount`。非缺陷（口径取舍），登记 backlog：要么把被隔离实体的 warnings 附进 `QuarantinedEntity.Violations`，要么在 `WarningCount` 口径处注释声明该取舍（`46-REVIEW.md:183`）。 |
| `IN-02` | `46-REVIEW.md:185-191` § Info | `ack backlog` | Quarantined / Warnings 顺序非确定：`internal/ontology/shacl/validate.go:241,259` 两个报告分区都遍历 map，Go map 序随机，同一份图每次 boot 输出顺序不同，FE 表格行序在两次启动间漂移。不影响正确性（FE 测试按 `data-testid` 索引）。登记 backlog，修法为输出前按 `entity` 字典序 `sort.Slice`（`46-REVIEW.md:191`）。注意：本条与 D-45-DEF-03 的 IN-02 是**同名的不同文件**（那条在 `isSchemaTriple`，这条在 `validate.go`），勿混。 |
| `IN-03` | `46-REVIEW.md:193-199` § Info | `ack backlog` | `danglingEdgePredicates` 硬编码三条三元组族（`validate.go:862-880`），计数口径窄于 FE 表头 tooltip 承诺（`ShaclReportView.tsx:183`）。今天计数正确（`abox.go:66-82` 只产这三类 entity 边），属 D-46-04 认可的设计 + 漂移风险备忘。登记 backlog：在定义处加注释指向 `abox.go` 谓词族作为同步点，或改为通用入边计数（`46-REVIEW.md:199`）。 |
| `IN-04` | `46-REVIEW.md:201-207` § Info | `ack backlog` | `Violation.SourcePath` / `LineStart` / `LineEnd` 引擎从不赋值（`validate.go:97-99`，结构体有字段、全包无赋值点），FE 来源/行号列恒渲染 `—`。当前渲染 `—` 至少诚实，故非缺陷。登记 backlog：要么 parse 阶段记录来源文件并在 stamping 循环赋值，要么从 FE 删掉两列留待 D-PROV-05 落地（`46-REVIEW.md:207`）。 |
| `IN-05` | `46-REVIEW.md:209-215` § Info | `ack backlog` | `hasBackfill` 的判据 `'provenance-missing'` 从未被任何 BE 版本发射（`git log -S 'provenance-missing'` 无 Go 侧产出），Run ECL 回填按钮自 40-07a 起未出现、46 之后结构性不可达（`GovernanceTab.tsx:50-52,64-80`）。PipelineTab 另有独立入口，功能本身可达。登记 backlog：需产品侧确认意图后清理或改挂新信号（`46-REVIEW.md:215`）。 |
| `IN-06` | `46-REVIEW.md:217-223` § Info | `ack backlog` | `GovernanceTab ↔ ShaclReportView` 循环 import（`ShaclReportView.tsx:18` re-export `GovernanceTab.tsx:16` 的 `HealthBanner`）。ESM/Vite 目前靠渲染期延迟求值容忍，但属脆弱模式。登记 backlog：把 `HealthBanner` 抽成独立文件、两个视图单向导入（`46-REVIEW.md:223`）。 |
| `IN-07` | `46-REVIEW.md:225-231` § Info | `ack backlog` | 默认 shapes 路径是 CWD 相对路径（`validate.go:293-296`，消费点 `cmd/server/main.go:934-938`），不在仓库根启动时两个文件读不到 → boot 校验 WARN 跳过 + 治理页 503 + rollback 重验恒 0，三个面同时静默降级但只有一行启动 WARN。登记 backlog：用 `runtime.Caller(0)` 或 `go:embed` 锚定默认路径，至少把解析后的绝对路径打进 WARN 日志（`46-REVIEW.md:231`）。本 phase 的 v1 本地启动 + Docker Compose 路径均在仓库根，故当前不影响验收。 |

---

## 3. SC-2 truthfulness note

SC-2 的验收陈述是「rollback 后 `/admin/governance` 的 SHACL 报告与重校验结果一致」，这条陈述在 47-02（`e7ca19a`）之后**成立且可证**：被服务的报告与 rollback 响应里的 violation 计数来自**同一次** `ValidateWithTrigger(..., "rollback")` 结果，`d3f86c2` 的成功分支断言 `Trigger == "rollback"` 且 `ErrorCount` 等于返回值即其直接 pin；47-04（`197421e` / `bf1c1e0`）把这份报告送到界面上，使 banner 的「回滚后」标签从死分支变为可达，jsdom trace 记录了 `（启动时）… error 1` → `（回滚后）… error 3` 的翻转。

**但 SC-2 的成立不等于 rollback 路径上的全部文案都属实。** 上表 D-05 行记录的是**另一件事**：rollback 根本不执行 `Quarantine`，所以 banner 的「N 个实体已隔离」与 confirm 框的「失效引用实体将进入隔离区」在 rollback 路径上过时。这条限制被显式登记为 `延期+理由` 并给出重新立项的前提，而不是被 REL-02 的成功悄悄掩盖 —— 这正是 SC-2 的验收陈述可以保持原样、而不需要扩张范围的原因：前者说的是**报告一致性**，后者说的是**隔离执行语义**，两者不互相蕴含。

---

*Phase: 47-debt-cleanup-reproducible-baseline | Plan: 47-06 | Created: 2026-10-19*
