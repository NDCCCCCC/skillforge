# Retrospective — SkillForge

## Milestone: v1.6 — SPARQL Schema Detection + Entity SHACL Shapes

**Shipped:** 2026-10-08
**Phases:** 2 (45, 46) | **Plans:** 7 | **Tasks:** 12+ | **Commits:** 61（2026-10-03 → 10-08）

### What Was Built

- D-SPARQL-05 β 语义：isSchemaTriple subject∩predicate arm（13 metaPredicates，label/comment fail-closed），skill `hasReadAccess` admin 双面可见（17/10），查询时派生零持久态
- SPARQL-03 单点化：isSchemaTriangle 全仓单调用点 + handler 级 parity 测试 + emitter anti-drift gate
- 实体 SHACL：severity 数据驱动归位（sourcePath 特例退役）+ entities.ttl（Product/Org）+ 三层盖戳 + dangling_edge_count 恒下发
- Shapes 路径 env 列表化：ShapesPathsFromEnv 单点，boot/rollback 双消费点收口（rollback 曾是恒 no-op，TDD 真 RED→GREEN 钉死）
- governance FE 三层呈现（46-UI-SPEC 逐字落地，FE 零推导）+ verify-phase-45/46.sh 双门禁
- 46-REVIEW 9 项 Critical/Warning 修复（含 CR-01 Phase 40 遗留 rollback ABox wipe）

### What Worked

- **研究先行钉死红线**：researcher 用 live 快照实证裸字面量问题，把「entityCode datatype 检查会隔离 16 实体」在 planning 前变成用户决策——执行期零返工
- **PATTERNS.md 行锚传递**：12/12 analog 映射让 5 个 executor 零盲飞；wave 间 SUMMARY 契约传递（46-01 引擎签名 → 46-02/03 消费）无断链
- **post-context 用户决策闭环**：研究 Open Questions 当场问、当场写进 plan 的 acceptance_criteria（含防顺手加强的 grep 验收）
- **审计驱动收口**：milestone audit 的 3 源交叉 + integration checker 11 条接线逐一核对，2 条 WARNING 全部定位到行级

### What Was Inefficient

- MSYS→WSL 双层 shell 的 `$?` 转义被吃——退出码读数两次假象（GATE_EXIT/RC=0），要文件中转才可信
- gsd-sdk 不在 PATH：CLI 版本用 gsd-tools.cjs，workflow 里的 `gsd-sdk query` 调用全部要手工映射（每个 agent 都重复发现这一点）
- flaky 测试（TestDeriveEdgesDeterministic）在门禁收尾才暴露——P4 waiver 机制没预演「非 waiver 失败」路径
- TestShaclViolation 回归冲突是研究期才发现——引擎改 severity 的 plan 若先落会翻红 13 个测试（好在 research→plan 顺序保住了）

### Patterns Established

- verify-phase-NN.sh 的 named-waiver 口径（waiver_ok 精确匹配单个 pre-existing 失败）跨 phase 复用（45→46）
- shapes 资产「必填=无条件发射 / if-present=sh:class only」约束清单模式（裸字面量安全的唯一形态）
- plan 间依赖经 SUMMARY 传递签名契约（46-01 mechanical adaptation → 46-03 replacement 两跳改造）

### Key Lessons

1. 引擎语义改动（severity/sourcePath）必须先 grep 全部测试依赖再定 fixture 策略——pinned 断言是红线
2. 字面量表示（裸 vs 引号）是 KG 校验的隐形契约——engineer 启发式前先 dump live 数据
3. 门禁退出码要经文件中转读，跨 shell 层的 `$?` 不可信
4. rollback 等低频路径的「静默 no-op」要靠回归测试真 RED 才能暴露（induction.go:383 存在两个 milestone 无人知）

### Cost Observations

- Model mix: glm-5.3-flash 全程（orchestrator + 5 executor + researcher/planner/checker/verifier/fixer/reviewer）
- Sessions: 1（单会话连续：plan-phase → execute-phase → code-review --fix → audit → complete-milestone）
- Notable: executor 平均 ~11 min/plan（46-01 10m / 46-02 11m / 46-03 11m / 46-04 16m / 46-05 8m）；fixer 最重（38 min, 9 commits）

## Milestone: v1.5 — Outstanding Closure

**Shipped:** 2026-10-02
**Phases:** 2 (42, 44) | **Plans:** 5 | **Quick tasks:** 10+ (wzl/d5w/d5w2/d5w3/g7x/h8k/cz7/jxb)

### What Was Built

- 8 个 SPARQL 测试修复(CGO 构建暴露)+ D-TIER-03 fail-closed 恢复
- 42-03 runbook 全门执行(Claude 代跑): 8 PASS + L8 PARTIAL + L4/L7 env-SKIP
- live 测量工具从零到一(live F1 scorer / hybrid recall / frozen replay 修复)
- 抽取 prompt writes-only 对齐 + schema 前缀剥离(live F1 0.39→0.87)
- admin 11-tab 全可用(治理白屏/SPARQL 401/MCP wiring/图谱空白画布 4 连修)
- 3D 知识图谱 + 本体视图 + 实体消解(65 险种/38 机构)+ 全屏/聚焦/标签
- TBox v1→v7 本体演进闭环全程走通(提案→人工批准→版本归档)

### What Worked

- **走查驱动**: 浏览器实测代替口头确认,每个 bug 都有截图/日志证据
- **测量先行**: 修 prompt 前先建 live F1 scorer——修完立即可证伪
- **确定性规则优先**: 实体消解用后缀规则而非模糊匹配,零误合并
- **per-gate 真实测量**: bootstrap 门从"捏造测试名"到真跑实测

### What Was Inefficient

- bootstrap 脚本三轮才跑通(CGO=0 遗留 + MCP 必填项 + LOCAL_* 路由)——脚本缺少 smoke 测试
- L1 门首跑 FAIL 后才发现评分器不存在——脚本评审时未核对测试名
- localStorage admin token 无 UI 写入路径——新浏览器必踩

### Patterns Established

- quick task 目录惯例: 诊断→plan/执行→SUMMARY→STATE 行
- live 测试 env 门惯例(SKILLFORGE_LIVE_*=1,缺省 skip 不破 hermetic)
- ONT-04 提案闭环可代跑: LLM 提案 → 人工批准(custom_local_name)→ v{N+1}

### Key Lessons

1. CGO=0 掩盖 go-duckdb 编译需求——构建矩阵必须含 CGO=1
2. 测量工具不存在时门会"假绿/假红"——先核对其存在性再跑门
3. nil-slice 序列化 null 对 FE 是地雷——API 层归一化是单点解
4. Vite HMR 多轮改动后失效,重缓存清理要进排障清单第一位

### Cost Observations

- Model mix: glm-5.3-flash(会话模型)执行;GLM quota 5h 窗口两次触顶均遇重置/充值
- Sessions: 1(跨 3 天)
- Notable: bootstrap 脚本三轮修复的教训已沉淀进 STATE.md
