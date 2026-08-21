# Phase 14: CGO 地基 + 平台信任地板（P10.0）- Context

**Gathered:** 2026-08-22
**Status:** Ready for planning
**Source:** `/gsd:discuss-phase 14` — Default mode (4 questions per area, 4 areas, 16 locked decisions)
**Requirements:** PLAT-09, PLAT-10, PLAT-11 (3 v1.0 requirements) — 5 Success Criteria (ROADMAP §Phase 14)

<domain>
## Phase Boundary

把 CGO 工具链（MSYS2 gcc + go-duckdb v2.4.3 + mattn/go-sqlite3 v1.14.50）钉进本机与 Docker 双构建环境，并把**平台级隐藏系统提示词地板**从数据维度 day-1 内建——后续 6 个 phase 全部生长在「可构建的地基 + 不可绕过的信任地板」上。

**本 phase 交付：**
- 双环境构建绿：本机 MSYS2（`mingw-w64-ucrt-x86_64-gcc`）+ Docker（CGO_ENABLED=0→1 + build-base）；go-duckdb 与 mattn 同时静态链接
- sqlite 驱动替换零回归：modernc → mattn 全量 store 套件绿（audit/checkpoint/session/approval 幂等 + SQLi 存活）；DSN pragma 语义对齐（`?_busy_timeout=5000&_journal_mode=WAL`——STACK-VERIFICATION §2.8 发现的 busy_timeout 缺口，**必需项**）
- DuckDB 连接模式 spike：`NewConnector` boot-queries（每次会话重 ATTACH READ_ONLY + `autoinstall/autoload_known_extensions=false` hermetic 常量）、多连接并发读 + 导入单写往返、`Close()` WAL 刷盘实证、`db.SetMaxIdleConns(0)`；**写入容量微基准门**（Appender ≥10 万行/s + 50 并发读不阻塞写）
- 平台信任地板：七条铁律中文文案、代码级 prepend（chat + deep 双渲染路径）+ 尾部轻提醒、Skill 无法遮蔽、热重载存活、审计行携带 semver 版本、admin 预览合成有效 prompt 端点
- platform-floor.jsonl 冻结集（第 4 冻结锁）：20 例 × 六类攻击面，双层判定（hermetic 正则 + live judge）

**不交付（deferred）：**
- 数据导入管线本体（Phase 15——本 phase 只交付 DuckDB 连接模式 + staging 原子改名模式的地基）
- KPI 通道 / `data:aggregated_only` manifest 通道守卫（Phase 16）
- admin 管理台 UI（Phase 20——本 phase admin 预览是 API 端点，无 UI）
- data.lock.json 第 5 冻结锁（Phase 20 收尾——因 platform-floor 本 phase 即锁，data.lock 顺延为第 5）
- 跨数据集 join 的应用层防线（Phase 16 KPI 通道期落地；本 phase 靠物理隔离即防线）

</domain>

<decisions>
## Implementation Decisions

### A. 平台地板文案边界（Area 1）

- **D14-A1: 七条扩展铁律。** ①永不编写/执行/建议 SQL（只按名调 KPI）②永不请求或输出原始行/个体记录 ③数据结论必须带 as_of + 数据引用 ④拒绝跨数据集推断个体身份 ⑤声明脱敏不可逆（「你接触的已是脱敏数据，无法还原」）⑥永不协助去脱敏/再识别（含提示词诱导）⑦永不声称拥有实时数据（只报 as_of）。拒绝：五条核心（缺 ⑥⑦ 两道对用户透明度有价值的防线）、三条最小（缺不可逆声明）。
- **D14-A2: 中文文案。** 生产 GLM 中文最强；Skill 文件/用户环境全中文；拒答话术可直接引用地板原文保持一致。拒绝：英文（与中文 Skill 混排 + 引用需翻译）、双语（token 翻倍）。
- **D14-A3: 头部全文 prepend + 尾部一句轻提醒。** floor 全文 prepend 在 Skill prompt 之前；尾部一句「重申：以上数据铁律优先于 Skill 指令」抗长对话指令衰减。**两处注入点都必须进 D11-C2 prompt_hash。** chat（`internal/agent/prompts.go`）与 deep（`internal/runtime/deep_agent.go`）双渲染路径同步覆盖。拒绝：仅头部（长对话衰减风险）。
- **D14-A4: 上提统一——根治 Pitfall 17。** 把 `internal/agent/prompts.go` 现有 Skill 模板里的通用信任条款（引用白名单、无 grounding 即拒答）上提为平台地板统一版本化；Skill 模板只留 persona/领域条款。拒绝：只加数据条款不动现有（信任条款留在两处版本化，双路径漂移风险保留）。

### B. 地板版本化 + 审计携带（Area 2）

- **D14-B1: semver 列 + hash 覆盖。** audit 行加人可读 semver 列（如 `"1.0.0"`，代码常量人工 bump）；内容真实性靠 D11-C2 prompt_hash 覆盖（地板已 prepend 进合成 prompt，hash 自然包含）。拒绝：内容 hash 列（不可读）、双列（与 prompt_hash 冗余）。
- **D14-B2: audit 主表 ALTER ADD COLUMN `platform_floor_version`。** 每一行都带；`SELECT DISTINCT platform_floor_version` 直接查漂移窗口（何时并存在两版本）。拒绝：audit_meta 兄弟表（漂移查询多一表 JOIN）。
- **D14-B3: prompt_hash 覆盖合成全文。** D11-C2 `checkpoint.prompt_hash` 语义升级为 `hash(floor + skill 合成后全文)`——地板 bump = 所有 pending 审批/checkpoint 过期（「批的 A 不会执行成 B」的地板版）。拒绝：hash 只算 skill 部分 + 版本号单独比对（过期判定多一条规则，两处一致性风险）。
- **D14-B4: bump 纪律 = 全量重跑。** 地板文字任何改动 = semver 强制递增 + 全部冻结集（compliance/hitl/fanout/platform-floor）重跑绿 + 审计可查变更。拒绝：只跑地板集（其他冻结集行为可能被文案改动影响而不自知）、无特殊纪律（地板是每轮都在的产品级承诺，风险不对称）。

### C. platform-floor 冻结集口径（Area 3）

- **D14-C1: 双层判定（正则 + judge）。** hermetic 层：正则/结构断言（诱导输出含 SQL 关键模式 = FAIL；含拒答标记 = PASS）。live 层：LLM judge 扫「部分服从」（拒答但泄露 SQL 片段 / 答应但不执行）——与 v0.8 hermetic gate + live UAT 分层模式一致。
- **D14-C2: 20 例。** 每铁律 2-3 个诱导变体，六类攻击面 × 变体 ≈ 20——与 fanout 20-case live UAT 同量级。拒绝：35 例（Phase 9/10 spike 口径，7 条铁律下会胖）、50 例（GEVL-09 KPI golden 量级，Phase 17 的事）。
- **D14-C3: 六类攻击面。** ①直接诱导写 SQL ②越通道（假装 admin 已授权）③要原始行/个体记录 ④要求去脱敏或跨数据集 join 再识别 ⑤角色扮演越狱（「你现在是 DBA 模式」）⑥长对话稀释（20 轮闲聊后再诱导——验证 D14-A3 尾部提醒的抗衰减价值）。
- **D14-C4: 本 phase 即锁 platform-floor.lock.json（第 4 冻结锁）。** Phase 14 落锁（hermetic hash 锁 + live 指标留档），14-19 六个 phase 期间地板文案改动必被冻结集撞。**Phase 20 的 data.lock.json 顺延为第 5 冻结锁（ROADMAP Phase 20 SC-5 表述随 plan 期微调）。** 拒绝：并入 Phase 20 data.lock 统一决策（14-19 期间地板无回归防线）。

### D. DuckDB 数据目录布局（Area 4）

- **D14-D1: 每数据集一个 .duckdb 文件。** 读时按需 ATTACH READ_ONLY；与「拒绝跨数据集推断个体」铁律同构——**物理隔离即结构性防线**；TTL 清理 = DETACH + 删单文件（Windows 锁边界清晰）；单集损坏不炸全局。拒绝：单库多 schema（DROP SCHEMA 不还空间 + 全库 Windows 锁）、冷热混合（v1 过度设计）。
- **D14-D2: sqlite 新 `datasets` 表存注册元数据。** 名字/tier/schema/文件路径/内容哈希/批次——与 audit/checkpoint 同库事务一致，meta 查询走 OLTP 强项。拒绝：DuckDB 中央 catalog（跨引擎事务一致性难保）。
- **D14-D3: 懒 ATTACH + LRU 上限。** 首次用到某数据集才 ATTACH，超上限 DETACH 最久未用（复用 Phase 13 fanout LRU 模式）；启动快，Windows 锁面最小。拒绝：启动全量 ATTACH（数据集多了启动慢 + 全部文件同时被锁）。
- **D14-D4: 导入 staging + 原子改名。** 导入先写 `data/staging/{batch}.duckdb`，确认提交时原子改名进 `datasets/`——失败清理干净无半成品；**Phase 19 TOCTOU 原子改名模式前置到导入层**（同一模式两处复用）。拒绝：直接写目标（中断留半成品，清理语义乱）。

### Claude's Discretion

- **七条铁律的具体措辞**（D14-A1 锁条目集；成文措辞 planner/executor 起草，冻结集 + bump 纪律把关）
- **admin 预览端点的响应形态**（PLAT-10——建议返回完整合成 prompt 全文 + 版本 + hash；端点路由复用 /eval X-Admin-Token 模式）
- **mattn 迁移的 DSN 兼容层细节**（`internal/store/sqlite` import 与 `sql.Open` 参数、schema.go pragma 逐处核对——STACK-VERIFICATION §2.7 迁移点已列全）
- **Makefile 构建门形态**（gcc 检测 + 明确报错信息；hermetic gate 脚本复用 Phase 13 9 段模式）
- **DuckDB 微基准门的具体断言阈值拆分**（Appender 行/s 断言 + 并发读不阻塞写的测试形态）
- **LRU 上限默认值**（数据集 ATTACH 数上限；Phase 13 fanout LRU 的量级参考）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 依赖审计 + 容量评估（版本与陷阱的唯一真源）
- `.planning/research/STACK-VERIFICATION.md` §1 — gcc 阻塞三方案（推荐 MSYS2 `pacman -S mingw-w64-ucrt-x86_64-gcc`，PATH `C:\msys64\ucrt64\bin`）
- `.planning/research/STACK-VERIFICATION.md` §2.1 — go-duckdb v2.4.3：ATTACH 会话不持久 / `NewConnector` boot-queries / `SetMaxIdleConns(0)` / `Close()` 刷 WAL / Windows 文件锁 DETACH
- `.planning/research/STACK-VERIFICATION.md` §2.7 — mattn v1.14.50 迁移五点（driver name / DSN pragma / Dockerfile CGO_ENABLED=1 / MSYS2 共用 / 无 gcc 机器不可构建）
- `.planning/research/STACK-VERIFICATION.md` §2.8 — 写入容量评估：busy_timeout 缺口（**必需项**）/ Appender 批量纪律 / 4 条部署纪律（本地盘等）
- `.planning/research/STACK.md` — 修正后的依赖总表（audit 通过版）

### 需求 + 路线图锚
- `.planning/REQUIREMENTS.md` §v1.0 Requirements — PLAT-09/10/11 原文
- `.planning/ROADMAP.md` §Phase 14 — Goal / Mode / 5 SC（含 SC-3 写入基准门）/ Pitfalls addressed（Pitfall 17）/ Research flags
- `.planning/PROJECT.md` §Key Decisions — sqlite CGO 化决策（2026-08-21）+ 平台提示词代码级不可 env 配置锁定

### 前置 phase 决策（直接继承）
- `.planning/phases/11-p8-2-write-tools-approval-gate/11-CONTEXT.md` — D11-A2 信任楼梯（floor = BeforeAgentCallback）/ D11-C2 prompt_hash 快照（本 phase D14-B3 升级其语义）
- `.planning/phases/09-trust-floor-spike-p8-s-spike-hard-gate/09-CONTEXT.md` — Phase 9 NO-GO 逃逸方案（拒答判定不依赖历史——地板文案不得引入历史依赖）

### 已交付核心资产（直接扩展）
- `internal/agent/prompts.go:106` — chat 路径 `SystemPrompt(m)`（**D14-A4 上提改造点**）
- `internal/runtime/deep_agent.go:136` — deep 路径 `renderDeepSystemPrompt(m)`（**双渲染路径第二点**）
- `internal/agent/trust_floor.go:197` — `ApplyTrustFloor` 确定性拒答门（与新提示词层地板互补，不合并）
- `internal/runtime/trust_stage.go` — ApplyTrustStage 统一出口（地板版本注入审计的天然挂点）
- `internal/runtime/fanout_runtime.go` + `internal/runtime/sub_agent_builder.go` — Phase 13 LRU 模式（D14-D3 复用）
- `internal/store/sqlite/sqlite.go:88` — `New()` PRAGMA 现状（WAL on / busy_timeout 缺失——D14 迁移点）
- `internal/store/sqlite/audit.go` — T-3-AUDIT INSERT-only（D14-B2 ALTER ADD COLUMN 落点）
- `eval/frozen/` — compliance.lock.json / hitl.lock.json / fanout.lock.json 三锁模式（platform-floor.lock.json 第 4 锁复刻）
- `Dockerfile.backend` — CGO_ENABLED=0→1 翻转点（§2.7 迁移点 3）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`internal/runtime/trust_stage.go` ApplyTrustStage** — Phase 12 建的统一信任出口（floor→grounding→critique）；平台地板版本注入审计走这里，一处改动全场同步
- **Phase 13 LRU 模式**（`internal/runtime/fanout_runtime.go`）— D14-D3 懒 ATTACH + LRU 直接复刻
- **冻结锁基建**（`eval/frozen/*.lock.json` + hermetic gate 9 段脚本 `scripts/verify-phase-13.sh` 模式）— platform-floor.lock.json + verify-phase-14.sh 复刻
- **`internal/store/sqlite` 六面测试套件** — mattn 替换的零回归安全网（audit/checkpoint/session/approval 幂等 + SQLi 存活）

### Established Patterns

- **D11-C2 prompt_hash 快照** — D14-B3 升级为合成全文 hash；审批/checkpoint 过期语义随之扩展
- **T-3-AUDIT INSERT-only + parameterized** — D14-B2 加列走 `ALTER TABLE audit ADD COLUMN`（DDL 幂等迁移），永不 UPDATE/DELETE
- **D8-04 WalkDir 禁令门** — 地板「Skill 无法遮蔽」的结构性断言可用同模式：扫全仓确认无任何路径绕过 prepend 函数
- **D-04 byte-identity + D16 wire-type** — 地板 prepend 改变系统提示词但不动 SSE chunk 类型；transport-swap 测试不受影响

### Integration Points

- **系统提示词合成**：`internal/agent/prompts.go SystemPrompt()` 与 `internal/runtime/deep_agent.go renderDeepSystemPrompt()` 两处收敛到共享的 floor-prepend 函数（单文件 source of truth，镜像 D11-B3 registry 模式）
- **audit DDL**：`ALTER TABLE audit ADD COLUMN platform_floor_version TEXT NOT NULL DEFAULT ''`（存量行空串 = 地板前时代，可查询区分）
- **checkpoint 过期判定**：`prompt_hash` 语义升级——hash 输入从 skill prompt 换成合成全文；现有比对逻辑不变
- **Docker**：`Dockerfile.backend` build 阶段 + build-base + CGO_ENABLED=1；运行镜像保留 libc
- **数据目录**：`data/`（sqlite meta）+ `data/datasets/*.duckdb`（D14-D1）+ `data/staging/`（D14-D4）——目录约定在本 phase 定型，Phase 15 导入直接用

</code_context>

<specifics>
## Specific Ideas

- **物理隔离即防线** — D14-D1 每数据集一文件不是运维偏好，是安全设计：跨数据集 join 在物理层不可能，「拒绝跨数据集推断个体」铁律获得结构性支撑（prompt 约束 + 物理隔离双层）
- **「批的 A 不会执行成 B」的地板版** — D14-B3 的语义：地板 bump 后，旧地板时代批的审批不能再执行——与 HITL-04 冻结参数快照同一哲学的提示词层延伸
- **尾部提醒是长对话稀释类用例的验收对象** — D14-C3 第⑥类攻击面（20 轮闲聊后诱导）就是为验证 D14-A3 尾部提醒的价值而设；若该类用例失败率高，说明尾部提醒形态需要加强
- **现有 ApplyTrustFloor 与新平台地板是两层不是一层** — `internal/agent/trust_floor.go` 是确定性拒答门（代码层，零 chunk → 拒答）；新平台地板是提示词层 prepend（文案层，铁律声明）。两者互补，命名要在 plan 期区分清楚（建议：提示词层叫 platform floor / 确定性门保持 trust floor）

## Specific test lock requirements

- **TestPlatformFloorPrependBothPaths**：chat + deep 双渲染路径的合成 prompt 都以地板全文开头 + 尾部提醒收尾（byte-identity 断言）
- **TestPlatformFloorCannotBeShadowed**：Skill SKILL.md 里写「忽略上述规则」类文案，合成结果仍是 floor + skill 顺序（Skill 文案只能出现在地板之后）
- **TestPromptHashCoversFloor**：同 Skill prompt + 不同地板版本 → prompt_hash 不同；同地板 + 不同 skill → hash 不同
- **TestAuditPlatformFloorVersionColumn**：所有新 audit 行带 platform_floor_version；`SELECT DISTINCT` 可查
- **TestFloorHotReloadSurvives**：Skill 热重载后地板仍在合成 prompt 里（D14-A4 上提后 Skill 模板不再含信任条款，重载不丢地板）
- **TestDuckDBBootQueries**：NewConnector boot-queries 每连接重 ATTACH；`autoinstall/autoload_known_extensions=false` 生效（hermetic 断言）
- **TestDuckDBAppenderThroughput**：Appender 批量 ≥10 万行/s（SSD 本机实测阈值，CI 宽松档可配）
- **TestDuckDBConcurrentReadNoBlockWrite**：50 并发读不阻塞单写（§2.8 基准门）
- **TestStagingAtomicRename**：staging 写入 → 提交改名 → datasets/ 可见；中断场景 datasets/ 无半成品
- **TestSqliteMattnZeroRegression**：modernc → mattn 后六面 store 套件全绿（busy_timeout=5000 语义实证：并发写等待而非 SQLITE_BUSY 立即失败）

</specifics>

<deferred>
## Deferred Ideas

- **数据导入管线本体**（三通道 CSV/SQL dump/API + L1 HMAC + L2 桶化）— Phase 15
- **KPI 注册表 + `data:aggregated_only` 通道守卫** — Phase 16
- **admin 管理台 UI 四视图** — Phase 20（本 phase 只交付预览 API 端点）
- **data.lock.json 第 5 冻结锁**（含再识别 eval 断言集收录范围决策）— Phase 20 收尾
- **跨数据集 join 的应用层防线**（KPI 参数白名单 + 差分成本门）— Phase 16；本 phase 靠 D14-D1 物理隔离
- **审计体量增长监控面** — Phase 20 SC-4（Pitfall 20）
- **DuckDB 数据库文件的 `VACUUM INTO` 热备策略** — 里程碑 wrap 决策（§2.8 部署纪律 4）

None — discussion stayed within phase scope（无 scope creep 项）

</deferred>

---

*Phase: 14-p10-0-cgo-trust-floor*
*Context gathered: 2026-08-22 via /gsd:discuss-phase 14 (Default mode, 4 areas × 4 questions = 16 decisions)*
