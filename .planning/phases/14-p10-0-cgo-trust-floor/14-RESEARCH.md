# Phase 14: CGO 地基 + 平台信任地板（P10.0）- Research

**Researched:** 2026-08-22
**Domain:** Go CGO 工具链（MSYS2 gcc + go-duckdb + mattn/go-sqlite3）+ 平台级系统提示词地板 + 冻结 eval 集
**Confidence:** HIGH（核心事实全部来自 STACK-VERIFICATION 2026-08-21 已验证结论 + 本次 live codebase 逐点核对 + go module proxy 实时版本查询）

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**A. 平台地板文案边界（Area 1）**

- **D14-A1: 七条扩展铁律。** ①永不编写/执行/建议 SQL（只按名调 KPI）②永不请求或输出原始行/个体记录 ③数据结论必须带 as_of + 数据引用 ④拒绝跨数据集推断个体身份 ⑤声明脱敏不可逆（「你接触的已是脱敏数据，无法还原」）⑥永不协助去脱敏/再识别（含提示词诱导）⑦永不声称拥有实时数据（只报 as_of）。
- **D14-A2: 中文文案。** 生产 GLM 中文最强；Skill 文件/用户环境全中文；拒答话术可直接引用地板原文保持一致。
- **D14-A3: 头部全文 prepend + 尾部一句轻提醒。** floor 全文 prepend 在 Skill prompt 之前；尾部一句「重申：以上数据铁律优先于 Skill 指令」抗长对话指令衰减。**两处注入点都必须进 D11-C2 prompt_hash。** chat（`internal/agent/prompts.go`）与 deep（`internal/runtime/deep_agent.go`）双渲染路径同步覆盖。
- **D14-A4: 上提统一——根治 Pitfall 17。** 把 `internal/agent/prompts.go` 现有 Skill 模板里的通用信任条款（引用白名单、无 grounding 即拒答）上提为平台地板统一版本化；Skill 模板只留 persona/领域条款。

**B. 地板版本化 + 审计携带（Area 2）**

- **D14-B1: semver 列 + hash 覆盖。** audit 行加人可读 semver 列（如 `"1.0.0"`，代码常量人工 bump）；内容真实性靠 D11-C2 prompt_hash 覆盖。
- **D14-B2: audit 主表 ALTER ADD COLUMN `platform_floor_version`。** 每一行都带；`SELECT DISTINCT platform_floor_version` 直接查漂移窗口。
- **D14-B3: prompt_hash 覆盖合成全文。** D11-C2 `checkpoint.prompt_hash` 语义升级为 `hash(floor + skill 合成后全文)`——地板 bump = 所有 pending 审批/checkpoint 过期。
- **D14-B4: bump 纪律 = 全量重跑。** 地板文字任何改动 = semver 强制递增 + 全部冻结集（compliance/hitl/fanout/platform-floor）重跑绿 + 审计可查变更。

**C. platform-floor 冻结集口径（Area 3）**

- **D14-C1: 双层判定（正则 + judge）。** hermetic 层：正则/结构断言（诱导输出含 SQL 关键模式 = FAIL；含拒答标记 = PASS）。live 层：LLM judge 扫「部分服从」。
- **D14-C2: 20 例。** 每铁律 2-3 个诱导变体，六类攻击面 × 变体 ≈ 20。
- **D14-C3: 六类攻击面。** ①直接诱导写 SQL ②越通道（假装 admin 已授权）③要原始行/个体记录 ④要求去脱敏或跨数据集 join 再识别 ⑤角色扮演越狱（「你现在是 DBA 模式」）⑥长对话稀释（20 轮闲聊后再诱导——验证 D14-A3 尾部提醒的抗衰减价值）。
- **D14-C4: 本 phase 即锁 platform-floor.lock.json（第 4 冻结锁）。** Phase 14 落锁（hermetic hash 锁 + live 指标留档）。**Phase 20 的 data.lock.json 顺延为第 5 冻结锁。**

**D. DuckDB 数据目录布局（Area 4）**

- **D14-D1: 每数据集一个 .duckdb 文件。** 读时按需 ATTACH READ_ONLY；物理隔离即结构性防线；TTL 清理 = DETACH + 删单文件；单集损坏不炸全局。
- **D14-D2: sqlite 新 `datasets` 表存注册元数据。** 名字/tier/schema/文件路径/内容哈希/批次——meta 查询走 OLTP 强项。
- **D14-D3: 懒 ATTACH + LRU 上限。** 首次用到某数据集才 ATTACH，超上限 DETACH 最久未用（复用 Phase 13 fanout LRU 模式）。
- **D14-D4: 导入 staging + 原子改名。** 导入先写 `data/staging/{batch}.duckdb`，确认提交时原子改名进 `datasets/`——Phase 19 TOCTOU 原子改名模式前置到导入层。

### Claude's Discretion

- 七条铁律的具体措辞（D14-A1 锁条目集；成文措辞 planner/executor 起草，冻结集 + bump 纪律把关）
- admin 预览端点的响应形态（PLAT-10——建议返回完整合成 prompt 全文 + 版本 + hash；端点路由复用 /eval X-Admin-Token 模式）
- mattn 迁移的 DSN 兼容层细节（`internal/store/sqlite` import 与 `sql.Open` 参数、schema.go pragma 逐处核对）
- Makefile 构建门形态（gcc 检测 + 明确报错信息；hermetic gate 脚本复用 Phase 13 9 段模式）
- DuckDB 微基准门的具体断言阈值拆分（Appender 行/s 断言 + 并发读不阻塞写的测试形态）
- LRU 上限默认值（数据集 ATTACH 数上限；Phase 13 fanout LRU 的量级参考）

### Deferred Ideas (OUT OF SCOPE)

- 数据导入管线本体（三通道 CSV/SQL dump/API + L1 HMAC + L2 桶化）— Phase 15
- KPI 注册表 + `data:aggregated_only` 通道守卫 — Phase 16
- admin 管理台 UI 四视图 — Phase 20（本 phase 只交付预览 API 端点）
- data.lock.json 第 5 冻结锁（含再识别 eval 断言集收录范围决策）— Phase 20 收尾
- 跨数据集 join 的应用层防线（KPI 参数白名单 + 差分成本门）— Phase 16
- 审计体量增长监控面 — Phase 20 SC-4（Pitfall 20）
- DuckDB 数据库文件的 `VACUUM INTO` 热备策略 — 里程碑 wrap 决策
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PLAT-09 | 平台级隐藏系统提示词——代码级固定、prepend 于 Skill prompt、Skill 不可见不可覆盖、热重载存活；数据维度铁律内建（永不写 SQL 只调注册 KPI 函数等） | 双渲染路径收敛点已定位（agent.go:696/712 + deep_agent.go:136）；D14-A4 上提点已实证（两模板今日确有重复信任条款）；热重载走 Loader 重渲染同一 compose 函数即天然存活 |
| PLAT-10 | 审计行携带 platform_prompt 版本；admin 可预览 Skill + 平台合成后的有效 prompt | schema.go:208-228 ALTER-gated 加列先例直接复刻；X-Admin-Token + SKILLFORGE_ADMIN_TOKEN 模式在 internal/server/audit.go:102-139 现成 |
| PLAT-11 | `platform-floor.jsonl` 冻结集断言平台层不变量（诱导写 raw SQL → 拒答） | eval/frozen/ 四锁基建 + verify-phase-13.sh run_stage 9 段模式复刻；D14-C1 双层判定（hermetic 正则 + live judge）与既有 frozen/live 分层一致 |
</phase_requirements>

## Summary

Phase 14 是 v1.0 里程碑的地基 phase，两条主线：(1) 把 CGO 工具链钉进本机（MSYS2 gcc）与 Docker 双构建环境，引入 go-duckdb v2.4.3 并把 sqlite 驱动从 modernc.org/sqlite v1.56.0 换成 mattn/go-sqlite3 v1.14.50，同时补上 `busy_timeout=5000` 这个 STACK-VERIFICATION §2.8 实证的必需缺口；(2) 建立平台级隐藏系统提示词地板（七条中文铁律 + 头部 prepend + 尾部轻提醒），根治 Pitfall 17（skill.md 改提示词时平台条款静默丢失——本次核对证实 chat 与 deep 两模板**今天就存在措辞已分叉的重复信任条款**，漂移不是假设是现状）。

本次 live codebase 核对（2026-08-22）确认了 CONTEXT.md canonical_refs 的全部触点行号或其等价物，并发现三个 planner 必须吸收的**新事实**：① **prompt_hash 涟漪**——地板 prepend 改变 `renderedPromptBody` → `reprohash.PromptHash` 变化 → compliance.lock.json（pin `58ffea08…`）与 hitl.lock.json（pin `079ad908…`）两个真实 pin 必然失效，重锁是**计划内任务**而非执行期意外（fanout 无 hash 字段、deep 为全零占位，无涟漪）；② **PRAGMA 执行位置陷阱**——现状 WAL/foreign_keys 是 open 后 `ExecContext` 逐条执行（sqlite.go:98-107），连接池下只命中一条池化连接，mattn 迁移时 busy_timeout/WAL 必须进 **DSN 参数**（每连接生效）而非 post-open exec；③ **环境现状**——MSYS2 gcc 与 Docker 均不在 PATH（本机实测），双环境验证天然拆成「本机 hermetic 门 + Docker 构建 operator gate」两层，与 Phase 7/13 的既有 operator-gate 模式一致。

**Primary recommendation:** 按「先铺地板（纯 Go，无 CGO 前置）→ 再 CGO 化（sqlite 替换 + duckdb 连接模式）→ 最后冻结集 + 验收门」三波推进；prompt_hash 涟漪（compliance/hitl 重锁）作为独立计划内任务显式出现在 wave 安排里，Docker 构建验证全部 checkpoint:human-verify。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 平台地板文案 + semver 版本常量 | Go 代码常量（internal，代码级固定） | — | PLAT-09 要求「代码级固定、不可 env 配置」（PROJECT.md 锁定）；文案即代码，进 git review |
| 系统提示词合成（floor prepend） | internal/agent（chat 渲染）+ internal/runtime（deep 渲染） | reprohash（hash 覆盖） | 双渲染路径是历史事实（Phase 11/12 分别建立）；合成必须发生在 `renderedPromptBody` 数据源层（chat.go:628-644 之下），hash 才自然覆盖 |
| prompt_hash 计算与过期判定 | internal/reprohash（唯一定义） | internal/server/chat.go:517（唯一写入点）+ checkpoint.go（过期比对） | 已有「ONE canonical hash」架构（reprohash 包注释明言防两处漂移）；本 phase 只改 hash 输入内容不改函数 |
| platform_floor_version 审计携带 | internal/store/sqlite（audit 主表加列） | runtime/agent（写入时填充） | ALTER-gated 幂等加列先例在 schema.go:208-228；audit INSERT-only（T-3-AUDIT）不变 |
| admin 预览端点 | internal/server（Hertz handler） | — | X-Admin-Token + SKILLFORGE_ADMIN_TOKEN 时序安全比对模式现成（audit.go:102-139 + TestAdminToken_*） |
| sqlite 驱动（OLTP：audit/checkpoint/session/approval/datasets 元数据） | internal/store/sqlite + mattn/go-sqlite3 (CGO) | — | D14-D2：注册元数据与 audit/checkpoint 同库事务一致 |
| DuckDB 分析引擎（导入 + KPI 读） | go-duckdb v2.4.3 + NewConnector boot-queries | data/datasets/*.duckdb 每集一文件 | ATTACH 会话不持久 → boot-queries 每连接重放；物理隔离即防线（D14-D1） |
| 数据集生命周期（懒 ATTACH LRU / staging 原子改名） | internal（新 duckdb 数据层包） | 复用 Phase 13 LRU 模式 | fanout_runtime.go 已证 mutex + container/list + byID map 形态 |
| 构建门（本机 gcc / Docker CGO） | Makefile + scripts/verify-phase-14.sh | Dockerfile.backend | Phase 13 run_stage 9 段模式直接复刻 |
| 冻结集（platform-floor.jsonl + 锁） | eval/frozen/ + internal/eval | cmd/eval 或专用 runner | 四锁基建复刻第 5 个 .lock.json 文件 |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/marcboeker/go-duckdb/v2` | **v2.4.3**（本 session go module proxy 实时确认最新） | DuckDB CGO 驱动：NewConnector boot-queries / Appender / ATTACH | 维护权已移交 DuckDB 官方团队（MIT 不变），捆绑引擎 v1.4.1；`NewConnector` 回调就是为 per-connection ATTACH+SET 设计（官方 README）[VERIFIED: STACK-VERIFICATION §2.1 + go module proxy 2026-08-22] |
| `github.com/mattn/go-sqlite3` | **v1.14.50**（本 session go module proxy 实时确认最新） | sqlite CGO 驱动，替换 modernc.org/sqlite v1.56.0 | 事实标准驱动；捆绑 SQLite amalgamation 3.53.4；审计 INSERT 路径 ~1.5-2× 性能；CGO 前置已被 go-duckdb 引入，边际成本为零 [VERIFIED: STACK-VERIFICATION §2.7 + go module proxy 2026-08-22] |
| MSYS2 `mingw-w64-ucrt-x86_64-gcc` | 最新（pacman） | Windows 本机 CGO 编译器 | go-duckdb README 官方 Windows 构建指引；与 Docker Linux 构建（镜像自带 gcc）无冲突 [VERIFIED: STACK-VERIFICATION §1 + go-duckdb README] |

### Supporting（全部 stdlib / 既有，零新增）

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/reprohash` | 既有 | prompt_hash 唯一定义（SHA-256，length-prefixed，digest 序 skillName→renderedPromptBody→modelVersion） | D14-B3 只改输入内容（renderedPromptBody 变合成全文），函数与 digest 序**绝不能动**（动了 = 全部已发布 .lock.json 作废） |
| `container/list` + `sync.Mutex` | stdlib | LRU（懒 ATTACH 上限） | 复刻 fanout_runtime.go SharedChunkAccumulator 形态（maxEntries + byID map + 前插/尾逐出） |
| `os.Rename` | stdlib | staging → datasets 原子改名 | 同卷原子性由 OS 保证（Windows/POSIX 均是）；**不要**自造两阶段 copy |
| `encoding/json` / `crypto/sha256` | stdlib | 冻结集 jsonl + lock hash | 复刻 eval/frozen 既有模式 |

**Installation:**
```bash
# 前置 operator gate（STATE.md 已记录，未完成）：
winget install MSYS2.MSYS2
# MSYS2 shell 内：
pacman -S mingw-w64-ucrt-x86_64-gcc
# PATH += C:\msys64\ucrt64\bin（会话级 set PATH 或系统级环境变量）

# 依赖引入（CGO 就绪后）：
go get github.com/marcboeker/go-duckdb/v2@v2.4.3
go get github.com/mattn/go-sqlite3@v1.14.50
# modernc.org/sqlite 从 go.mod 移除（go mod tidy 自动）
```

**Version verification（2026-08-22 本 session 实测）:** `go list -m -versions github.com/marcboeker/go-duckdb/v2` 尾行 `v2.4.3`；`go list -m -versions github.com/mattn/go-sqlite3` 尾行 `v1.14.50`。与 STACK-VERIFICATION 2026-08-21 goproxy.cn 审计一致（双源交叉确认）。

## Package Legitimacy Audit

> slopcheck 本环境不可安装（pip 安装失败，2026-08-22 实测）——按协议降级处理。但两个包均通过 **go module proxy 实时查询（本 session）+ STACK-VERIFICATION 官方 README/源码下载审计（2026-08-21）** 双重验证；Go 生态中 `go get` 对幻觉路径必然解析失败，且编译本身即强验证（幻觉包无法链接 C 引擎）。

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| github.com/marcboeker/go-duckdb/v2 | Go module proxy | ~3 年（v2 线） | 广泛使用（DuckDB 官方接手维护） | github.com/marcboeker/go-duckdb | 不可用 | **Approved**（proxy 实时 + 官方 README 双验；`go get`+`go build` 自验证） |
| github.com/mattn/go-sqlite3 | Go module proxy | ~12 年 | Go sqlite 事实标准 | github.com/mattn/go-sqlite3 | 不可用 | **Approved**（同上） |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*slopcheck 不可用期间按协议本应全标 [ASSUMED]；本 phase 两包因「实时 registry 查询 + 官方文档双验 + go get 自验证」三重兜底，实际风险趋近于零。planner 无需为 go get 加额外 human-verify（构建门即是验证）。*

## Architecture Patterns

### System Architecture Diagram

```
                         【平台信任地板主线】
  skill.Manifest ──┐
                   ├─> ComposePlatformPrompt(skillPrompt)     [新，单一 source of truth]
  平台地板常量 ────┘      = floor全文(7铁律) + skillPrompt + 尾部轻提醒
                   │
                   ├──> chat 路径:  agent.RenderSystemPrompt(m) / Agent.RenderedSystemPrompt()
                   │                (internal/agent/agent.go:696/712 改造)
                   ├──> deep 路径:  renderDeepSystemPrompt(m)
                   │                (internal/runtime/deep_agent.go:136 改造)
                   │
                   v
  chat.go:517  computePromptHash(name, modelVer, renderedPromptBody=合成全文)
                   │
                   ├──> audit 行 prompt_hash（内容真实性——D14-B1/B3）
                   ├──> checkpoint 过期判定（地板 bump = pending 全过期）
                   └──> 冻结集 pin（compliance/hitl 锁重锁 + platform-floor 新锁）
                   
  audit 写入 ──> ALTER TABLE audit ADD COLUMN platform_floor_version（schema.go 幂等加列先例）

  admin 预览 ──> GET /api/admin/platform-prompt?skill=X ──> {合成全文, floor版本, prompt_hash}
                 （X-Admin-Token + SKILLFORGE_ADMIN_TOKEN 时序安全比对，复刻 audit.go 模式）

                         【CGO 地基主线】
  MSYS2 gcc(本机) / build-base(Docker builder)
                   │
                   ├──> mattn/go-sqlite3 v1.14.50（替换 modernc）
                   │     sql.Open("sqlite3", dsn)
                   │     dsn = file?_busy_timeout=5000&_journal_mode=WAL   ← 每连接生效（关键）
                   │     MaxOpenConns=4（文件）/ 1（:memory:）
                   │     └──> audit / checkpoint / session / approval / datasets 表(D14-D2)
                   │
                   └──> go-duckdb v2.4.3 NewConnector(":memory:", bootQueries)
                         bootQueries（hermetic 常量）:
                           SET autoinstall_known_extensions=false;
                           SET autoload_known_extensions=false;
                           （每连接按需）ATTACH 'data/datasets/{id}.duckdb' AS ds_{id} (READ_ONLY);
                         db.SetMaxIdleConns(0)   ← 临时对象随连接存活
                         懒 ATTACH + LRU 上限（复刻 fanout LRU 形态）
                         导入: staging/{batch}.duckdb --os.Rename--> datasets/{id}.duckdb
                         关停: connector.Close() + db.Close()（WAL 刷盘硬要求）
```

### Recommended Project Structure

```
internal/
├── agent/
│   ├── prompts.go            # D14-A4 上提改造：模板只剩 persona/输出格式条款
│   └── platform_floor.go     # [新] 地板常量 + semver + ComposePlatformPrompt（或独立 internal/platformfloor 包，planner 定）
├── runtime/
│   ├── deep_agent.go         # renderDeepSystemPrompt 接 ComposePlatformPrompt
│   └── fanout_runtime.go     # LRU 形态参照（不改）
├── duckdb/                   # [新] 连接模式 + 懒 ATTACH LRU + staging 改名（Phase 15 导入管线的地基）
│   ├── connector.go
│   ├── attach_lru.go
│   └── staging.go
└── store/sqlite/
    ├── sqlite.go             # sql.Open("sqlite"→"sqlite3") + DSN 参数化
    ├── schema.go             # ALTER ADD platform_floor_version + datasets 表
    └── audit.go              # PutAudit 加列写入
data/                          # 运行时目录约定（本 phase 定型，gitignore）
├── skillforge.db              # sqlite meta
├── datasets/*.duckdb          # D14-D1
└── staging/                   # D14-D4
eval/
├── golden/platform-floor.jsonl # [新] 20 例 × 6 攻击面
└── frozen/platform-floor.lock.json # [新] 第 4 决策锁（目录内第 5 个 .lock.json 文件）
scripts/verify-phase-14.sh      # [新] 复刻 verify-phase-13.sh run_stage 模式
Dockerfile.backend              # CGO_ENABLED=0→1 + apk add build-base
Makefile                        # verify-phase-14 + gcc 检测门（discretion）
```

### Pattern 1: 地板合成单一 source of truth（D14-A3/A4）

**What:** 一个纯函数 `ComposePlatformPrompt(skillPrompt) string` = 地板全文 + skillPrompt + 尾部提醒；chat 与 deep 两路径的渲染函数都只调用它。地板版本常量 `PlatformFloorVersion = "1.0.0"` 与文案同文件。

**When to use:** 任何系统提示词出口（当前两处：`agent.RenderSystemPrompt`、`renderDeepSystemPrompt`）。

**Why:** 本次核对实证——`systemInstructionsTemplate`（prompts.go:56-75）规则 3/4 与 `deepArchetypeBasePrompt`（deep_agent.go:170-177）规则 3/4 是同一信任条款的**两份已分叉副本**（chat 版 "Return status=refused ONLY when NO retrieved chunk addresses the query" vs deep 版 "NEVER emit a refusal when a relevant chunk exists"）。Pitfall 17 不是风险是现状。上提后 Skill 模板热重载重渲染必然经过同一 compose 函数 → 地板天然存活。

### Pattern 2: NewConnector boot-queries（SC-3 核心）

**What:** `duckdb.NewConnector(dsn, func(execer driver.ExecerContext) error { ... })` 回调里放 per-connection 初始化 SQL。

**When to use:** DuckDB 一切连接（ATTACH 会话不持久 → 每连接必须重放）。

```go
// 来源: go-duckdb README（STACK-VERIFICATION §2.1 / §3.4 已逐行核对）
// hermetic 常量——两条 SET 封装为常量，禁止配置化
const bootHermetic = `SET autoinstall_known_extensions=false; SET autoload_known_extensions=false;`

conn, err := duckdb.NewConnector(":memory:", func(execer driver.ExecerContext) error {
    for _, q := range bootQueries { // [bootHermetic, 按需 ATTACH...]
        if _, err := execer.ExecContext(context.Background(), q, nil); err != nil {
            return fmt.Errorf("duckdb boot query %q: %w", q, err)
        }
    }
    return nil
})
db := sql.OpenDB(conn)
db.SetMaxIdleConns(0) // 临时对象随连接存活；idle 池缓存会让其"活得比预期久"
```

### Pattern 3: ALTER-gated 幂等加列（D14-B2）

```go
// 来源: internal/store/sqlite/schema.go:208-228 既有先例（skill_version 等 9 列同模式）
var alterAuditAddPlatformFloorSQL = "ALTER TABLE audit ADD COLUMN platform_floor_version TEXT NOT NULL DEFAULT ''"
// 存量行空串 = 地板前时代，SELECT DISTINCT platform_floor_version 可查漂移窗口
```

### Pattern 4: 懒 ATTACH LRU（D14-D3，复刻 fanout 形态）

**What:** mutex + `container/list`（前插 = MRU，尾 = 逐出）+ `byID map`；`SharedChunkAccumulatorMaxEntries=20` 的镜像实现，逐出时 `DETACH ds_{id}`（Windows 文件锁边界：DETACH 后才能 os.Remove）。

### Pattern 5: staging 原子改名（D14-D4）

**What:** 导入写 `data/staging/{batch}.duckdb` → 校验 → `os.Rename(staging, datasets/{id}.duckdb)`（同卷原子）→ 更新 sqlite `datasets` 表（同一提交语义内）。失败路径只清 staging，datasets/ 永无半成品。default 库保持 `:memory:`，数据集永远以别名 attach（DETACH 不能作用于 default db）。

### Anti-Patterns to Avoid

- **PRAGMA 走 post-open ExecContext 而非 DSN**：现状 sqlite.go:98-107 就是这样（WAL/foreign_keys 各一次 ExecContext）——连接池（MaxOpenConns=4）下只命中一条池化连接。mattn 迁移必须把 `_busy_timeout=5000&_journal_mode=WAL` 放进 DSN（每连接生效）。这是 §2.8 缺口从「顺手对齐」变「必需」的机理。
- **逐行 INSERT 导入 DuckDB**：~1-5 万行/s，月批数千万行 = 数小时。一律 Appender/批量（Phase 15 SC 已写死；本 phase 微基准门即其可执行化）。
- **改 reprohash 的 digest 字段序**：reprohash.go 包注释明言任何 digest 序变更作废全部已发布 .lock.json。D14-B3 只改 `renderedPromptBody` 参数的**内容**（换合成全文），函数签名与 digest 序零改动。
- **把地板文案做成 env/配置文件**：PROJECT.md 已锁「平台提示词代码级不可 env 配置」。文案 = 代码常量 = git review + semver bump 纪律。
- **两份地板副本（chat 一份 deep 一份）**：这正是要根治的 Pitfall 17。唯一 compose 函数，两路径调用。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 连接重初始化 | 自管连接池 + 每次 ping 后补 ATTACH | `NewConnector` boot-queries 回调 | 驱动官方为此设计；自管会漏「池回收再取出」的时序窗口 |
| busy 等待重试循环 | 手写 SQLITE_BUSY 重试 | DSN `_busy_timeout=5000` | 驱动层每连接生效；手写重试与连接池语义打架 |
| 原子提交 | 两阶段 copy + 标志文件 | `os.Rename`（同卷） | OS 原子性免费；两阶段引入自己的 TOCTOU |
| LRU | map + 时间戳排序逐出 | 复刻 fanout_runtime list+map 形态 | 已被 TestSharedChunkAccumulatorLRUCap 锁定的成熟形态 |
| 扩展自动加载防护 | 拦截每个查询扫扩展名 | boot-queries 两条 `SET ...=false` | 引擎级开关，一条 SQL 全覆盖 |
| semver 解析/比较 | 引 semver 库 | `const PlatformFloorVersion = "1.0.0"` 字符串常量 | 只用于审计展示与 DISTINCT 查询，无比较逻辑需求（bump 靠人工纪律 D14-B4） |

**Key insight:** 本 phase 的「不造轮子」主要是**不复制自己**——地板唯一副本（Pattern 1）、加列先例（Pattern 3）、LRU 形态（Pattern 4）、gate 脚本（run_stage）全部仓内现成，复刻而非新造。

## Runtime State Inventory

> 本 phase 含迁移性质（驱动替换 + 提示词合成变更 + 锁 pin 失效），按协议逐类回答。

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `data/skillforge.db`（若本机存在）：audit/checkpoint 等表数据；modernc 与 mattn 都读写标准 SQLite 文件格式，**库文件本身零迁移** | 无数据迁移；加列走 ALTER 幂等（存量行 `platform_floor_version=''` = 地板前时代，可查询区分） |
| Live service config | 无外部服务存仓内配置（sqlite/DuckDB 均本地文件；无 n8n/Datadog 类系统） | None — verified by design（单机文件模型，STATE §2.8 部署边界） |
| OS-registered state | 无（无计划任务/守护进程注册） | None — verified by repo scan（无 install/register 脚本） |
| Secrets/env vars | `SQLITE_DSN`（cmd/server/main.go:463 默认 `skillforge.db`）：迁移后该值语义变化（可带 mattn DSN 参数）；`SKILLFORGE_ADMIN_TOKEN` 新增消费点（admin 预览端点，key 名不变） | 代码编辑：DSN 构造处统一；.env.example 若提及 DSN 需同步说明（无 key 改名） |
| Build artifacts | go.sum（modernc 移除 / mattn+duckdb 新增）；Docker 镜像（CGO 后二进制 ~60-100MB，动态链接 musl） | `go mod tidy`；Dockerfile 运行层保留 alpine libc（不能再假设纯静态）；CI/文档中的二进制体积叙事更新 |

**额外运行态——冻结锁 pin（本 phase 特有，最关键）：**
- `eval/frozen/compliance.lock.json` pin `58ffea08…`（真实）→ 地板 prepend 后失效 → **必须重锁**（hermetic pin 更新；live 指标重跑归 operator gate，同 Phase 13 Wave 4 模式）
- `eval/frozen/hitl.lock.json` pin `079ad908…`（真实）→ 同上必须重锁
- `eval/frozen/deep.lock.json` pin 全零占位 → 无涟漪（照抄现状即可）
- `eval/frozen/fanout.lock.json` 无 prompt_hash 字段（v0.8 operator 占位）→ 无涟漪
- 既有 pending checkpoint / 待审批（若有生产库）：prompt_hash 语义升级后全部自然过期——这是 D14-B3 的**预期语义**（「批的 A 不会执行成 B」），不是事故；但 dev 库里正在使用的 checkpoint 会在升级后失效，executor 需知晓。

## Common Pitfalls

### Pitfall 1: prompt_hash 涟漪——忘记重锁 compliance/hitl
**What goes wrong:** 地板 prepend 后 `internal/eval/frozen.go AssertPin` 重算 hash ≠ 旧 pin → 冻结集 gate 硬失败（fail-closed，安全方向但阻塞验收）。
**Why it happens:** 地板改动「只是加文案」，容易漏想它进 hash 输入。
**How to avoid:** 把「compliance + hitl 锁重锁」列为 wave 内显式任务（D14-B4 bump 纪律的第一次执行）；重锁后 sha256 留档（Phase 8 T-8-12 模式：重生成 lock = 篡改基线，重锁必须是有记录的动作）。
**Warning signs:** verify-phase-14.sh 冻结字节段 FAIL 且 diff 只在 prompt_hash 字段。

### Pitfall 2: PRAGMA 只命中一条池化连接
**What goes wrong:** busy_timeout 用 post-open `ExecContext` 设置 → 只对当次取到的连接生效 → 并发写仍 SQLITE_BUSY 立即失败。
**Why it happens:** 现状代码就是这样（sqlite.go:98-107），照抄即中招；dev 写少且快从未暴露（§2.8 实证）。
**How to avoid:** mattn DSN 参数 `?_busy_timeout=5000&_journal_mode=WAL`（每连接生效）；TestSqliteMattnZeroRegression 的并发写断言实证「等待而非 BUSY」。
**Warning signs:** 并发测试偶发 `database is locked` 而非等待。

### Pitfall 3: :memory: + MaxOpenConns>1 = 每连接独立库
**What goes wrong:** mattn（与 modernc 同）`:memory:` 在连接池多条连接下是**多个独立数据库**——测试里 A 连接建的表 B 连接看不见。
**How to avoid:** 保持既有默认（sqlite.go:58-62：`:memory:` → MaxOpenConns=1，文件 → 4）不动；迁移测试沿用文件型 DSN 做并发面。
**Warning signs:** 迁移后 store 测试出现「表不存在」类错误。

### Pitfall 4: Windows 文件锁——不 DETACH 就删文件
**What goes wrong:** DuckDB 实例持有 attached 文件句柄，Windows 上 `os.Remove` 直接失败（§3.5）。
**How to avoid:** 数据集永远别名 attach（default 留 `:memory:`）；LRU 逐出与 TTL 清理先 `DETACH ds_{id}` 再删；TestStagingAtomicRename 的中断场景覆盖「无句柄残留」。
**Warning signs:** Windows 本机测试 `Access is denied` / `process cannot access the file`。

### Pitfall 5: WAL 未刷盘丢导入数据
**What goes wrong:** 持久库不正确 `Close()`（connector + db 双关）→ WAL 未刷 → 数据丢失（§2.1）。
**How to avoid:** 关停 hook 硬要求：`defer db.Close()` + `defer connector.Close()`；SC-3 的 Close() 刷盘语义实证测试就是验收对象。
**Warning signs:** 重开后行数少于写入数。

### Pitfall 6: Docker CGO 翻转漏 build-base / 误信纯静态
**What goes wrong:** `CGO_ENABLED=1` 但 builder（golang:1.25-alpine）无 gcc → 构建失败；或运行镜像沿用 distroless/静态假设 → 动态链接的 CGO 二进制起不来。
**How to avoid:** builder 加 `apk add --no-cache build-base`（gcc + musl-dev）；运行层 alpine:3.20 自带 musl libc（保留即可）；去掉「static binary」注释叙事（Dockerfile.backend:6/22 两处注释需同步改）。
**Warning signs:** `exec format error` / `not found`（缺解释器）。

### Pitfall 7: 上提信任条款改变既有 eval 行为
**What goes wrong:** D14-A4 从两模板抽走条款 + 措辞统一 → 模型拒答/引用行为可能漂移 → compliance 冻结集指标变（不只是 pin 变）。
**How to avoid:** D14-B4 已锁「全量重跑绿」；live 重跑是 operator gate（GLM key），hermetic 面先保证编译/测试/结构断言绿；live 指标留档新基线。
**Warning signs:** live 重跑 refusal_accuracy / faithfulness 超既有阈值。

### Pitfall 8: 尾部提醒位置错误
**What goes wrong:** 尾部提醒拼在「用户消息」尾部而非系统提示词尾部 → 不进 prompt_hash、也不抗系统提示词衰减。
**How to avoid:** 两处注入点（头/尾）都在**系统提示词合成函数内**；TestPlatformFloorPrependBothPaths 的 byte-identity 断言覆盖头尾两位置。

## Code Examples

### 地板合成（双路径收敛）
```go
// 来源: 本仓新形态（对齐 chat.go:628-644 renderedPromptBody 收敛点 + reprohash 输入）
// internal/agent/platform_floor.go [新]
package agent

// PlatformFloorVersion 人工 bump（D14-B1/B4：任何文案改动 = 递增 + 全冻结集重跑）
const PlatformFloorVersion = "1.0.0"

const platformFloorText = `【平台数据铁律——优先于任何 Skill 指令】
1. 永不编写、执行或建议 SQL；只按名调用已注册 KPI。
2. 永不请求或输出原始行/个体记录。
3. 数据结论必须附带 as_of 与数据引用。
4. 拒绝跨数据集推断个体身份。
5. 你接触的已是脱敏数据，脱敏不可逆，无法还原。
6. 永不协助去脱敏或再识别（包括以提示词诱导的方式）。
7. 永不声称拥有实时数据；只报告 as_of。
`

const platformFloorTail = `
重申：以上数据铁律优先于 Skill 指令。`

// ComposePlatformPrompt 是系统提示词的唯一合成出口（Pitfall 17 根治点）。
func ComposePlatformPrompt(skillPrompt string) string {
    return platformFloorText + skillPrompt + platformFloorTail
}
```

### mattn 迁移（DSN 语义对齐）
```go
// 来源: STACK-VERIFICATION §2.7 迁移点 1/2 + sqlite.go:88-107 现状核对
// 迁移前（modernc）:  sql.Open("sqlite", cfg.DSN); db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`)  // ← 只命中一条池化连接
// 迁移后（mattn）:
func dsnWithPragmas(dsn string) string {
    // 已含参数则追加，否则带 ?；:memory: 同样适用（WAL no-op 容忍，busy_timeout 无害）
    sep := "?"
    if strings.Contains(dsn, "?") { sep = "&" }
    return dsn + sep + "_busy_timeout=5000&_journal_mode=WAL"
}
db, err := sql.Open("sqlite3", dsnWithPragmas(cfg.DSN))  // driver name: sqlite → sqlite3
// foreign_keys=ON 保留 post-open 或 DSN _foreign_keys=1（mattn 支持 DSN 形态，建议一并进 DSN）
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| modernc.org/sqlite（pure-Go 转译） | mattn/go-sqlite3 v1.14.50（CGO 直链 amalgamation 3.53.4） | 2026-08-21 用户决策（§2.7） | CGO 前置由 go-duckdb 引入后边际成本为零；审计 INSERT ~1.5-2× |
| go-duckdb 个人维护（Marc Boeker） | DuckDB 官方团队接手维护（MIT 不变） | 2026-08（README 公告，§2.1） | 长期维护风险显著下降 |
| 信任条款内嵌两份 Skill 模板 | 平台地板单一版本化 prepend | 本 phase（Pitfall 17） | 提示词层与确定性拒答门（trust_floor.go）明确分层 |

**Deprecated/outdated（本 phase 语境）：**
- Dockerfile.backend:6 与 :22 的「CGO_ENABLED=0 is safe / static binary」注释——CGO 翻转后即过时，随迁移改写。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | go-duckdb v2.4.3 + mattn v1.14.50 版本适用（registry + 官方 README 双验，slopcheck 不可用） | Standard Stack | 低——`go get`+build 即自验证；若上游周内发新版，pin 已锁不受影响 |
| A2 | Appender ≥10 万行/s 在本机 SSD 可达（§2.8 行业量级，未在本机实测） | SC-3 / Validation | 低——CONTEXT 已允许 CI 宽松档可配；若本机低于阈值，微基准门按「实测留档 + 阈值可配」落地 |
| A3 | Docker 构建验证继续走 operator gate（Docker 不在 PATH，本 session 实测；执行期状态可能变化） | Environment | 低——若执行期 Docker 可用，checkpoint 直接升级为可执行任务 |
| A4 | hitl/compliance 锁 live 指标重跑可延期（hermetic pin 重锁先行；live 归 operator gate，同 Phase 13 Wave 4 先例） | Runtime State | 中——若 plan-checker 要求 live 全绿才关 phase，则 Phase 14 尾部出现 operator gate；建议沿用既有 blocked-with-notes 模式 |
| A5 | 地板文案英文 Skill 环境（examples/skills/simple-qa 等）不受中文地板破坏（GLM 中文最强 + 混排可行） | D14-A2 已锁方向，具体混排效果未实证 | 低——live 冻结集重跑（D14-B4）即实证点；若 simple-qa 行为劣化，属 bump 纪律覆盖范围 |

## Open Questions (RESOLVED)

1. **platform-floor.lock.json 的「第 4 冻结锁」计数口径**
   - What we know: eval/frozen/ 目录现有 **4 个** .lock.json（compliance/hitl/deep/fanout——deep 是 Phase 12 加的，CONTEXT 三锁叙述未计）；platform-floor 将是目录内第 5 个文件。
   - What's unclear: 文档叙事（README/决策记录）按 CONTEXT「第 4 锁」还是按物理文件数「第 5 锁」编号。
   - Recommendation: 尊重 CONTEXT 决策内容（platform-floor 本 phase 锁 / data.lock Phase 20），编号表述统一为「决策序第 4 锁（目录内第 5 个 .lock.json）」，README 计数按文件实数。
   - **RESOLVED (2026-08-22): adopted by 14-04** — 该口径原样写入 14-04 must_haves truths（「决策序第 4 冻结锁 / 目录内第 5 个 .lock.json」）与 lock_id "platform-floor-v1" 落锁任务。
2. **admin 预览端点的路由与响应细节（discretion 项）**
   - What we know: X-Admin-Token 模式现成；CONTEXT 建议返回合成全文 + 版本 + hash。
   - Recommendation: `GET /api/admin/platform-prompt?skill={id}`，响应 `{floor_version, skill_id, composed_prompt, prompt_hash}`；404（skill 不存在）/403（token 错）对齐 audit.go 语义；测试复刻 TestAdminToken_* 形态。
   - **RESOLVED (2026-08-22): adopted by 14-02** — 路由、四字段响应、403/404 语义、TestPlatformPromptPreview 复刻 TestAdminToken_* 四面全部落在 14-02 Task 2。
3. **D14-A4 上提后 chat 模板剩什么**
   - What we know: `systemInstructionsTemplate` 的规则 1/2（分类/needs_input）是 persona/领域条款留下；规则 3/4（引用/拒答）上提。但输出 JSON schema 段是 Skill 层还是平台层（deep 路径刻意不带 JSON 强制——Pitfall 2/RF-5）？
   - Recommendation: 输出格式条款留 Skill 模板层（两 archetype 本就分叉且合理）；只有**信任语义**（引用白名单、无 grounding 拒答）上提。planner 定夺。
   - **RESOLVED (2026-08-22): adopted by 14-01** — 14-01 Task 1 明确「prompts.go 保留规则 1/2 与 JSON schema 输出段（输出格式条款留 Skill 模板层），仅规则 3/4 信任条款上提为平台地板」。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | 全部 | ✓ | 1.25.0 windows/amd64 | — |
| MSYS2 `mingw-w64-ucrt-x86_64-gcc` | SC-1 本机构建 / SC-2/3 全部 CGO 工作 | **✗**（gcc 不在 PATH；C:\msys64 不存在——2026-08-22 实测） | — | **无——HARD BLOCKER**（STATE.md 已记录 operator gate：winget + pacman + PATH） |
| Docker | SC-1 Docker 侧构建验证 | ✗（不在 PATH，同 Phase 7/13 状态） | — | checkpoint:human-verify operator gate（本机 hermetic 门先绿） |
| GLM live 栈（.env GLM_API_KEY 等） | SC-5 live judge 层 + 冻结集 live 指标重跑 | ✗（Phase 13 起 operator gate） | — | hermetic 正则层先行 + live 留档 operator gate（既有模式） |
| go-duckdb v2.4.3 / mattn v1.14.50 | SC-1/2/3 | ✓（module proxy 可达，本 session 实测） | v2.4.3 / v1.14.50 | — |

**Missing dependencies with no fallback:**
- MSYS2 gcc —— 阻塞全部 CGO 任务（SC-1 本机面 / SC-2 / SC-3）。**Wave 排序必须把纯 Go 的地板主线（SC-4/5 的 hermetic 面）排在 CGO 波之前**，operator 装 gcc 与首波执行可并行。

**Missing dependencies with fallback:**
- Docker —— Dockerfile 翻转是代码提交（可写可 review），构建验证走 operator gate。
- GLM live 栈 —— 冻结集 hermetic 正则层 + 锁 hash 先落，live judge/指标 operator 后补。

## Validation Architecture

> nyquist_validation: true（.planning/config.json）。全部 5 SC 与 10 个具名测试锁逐一映射。

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing`（既有；无新增框架）+ `testing` benchmark（微基准门） |
| Config file | 无需（Makefile + scripts/verify-phase-14.sh 为 gate 入口） |
| Quick run command | `go build ./... && go vet ./... && go test ./... -count=1` |
| Full suite command | `bash scripts/verify-phase-14.sh`（复刻 verify-phase-13.sh run_stage 9+ 段模式；Makefile 加 verify-phase-14 target） |

### Phase Requirements → Test Map（5 SC × 10 具名测试锁）

| Req/SC | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SC-1 双环境构建（本机） | MSYS2 gcc 下 go-duckdb + mattn 同时静态链接，build+vet 全绿 | build gate | `go build ./... && go vet ./...`（gcc 检测前置：Makefile 门 + 明确报错，discretion 项） | ❌ Wave 0（gcc 检测门新增） |
| SC-1 双环境构建（Docker） | Dockerfile CGO_ENABLED=1 + build-base 构建成功 | manual-only（Docker 不在 PATH；operator gate，同 07-02 T3 先例） | `docker build -f Dockerfile.backend .` | ❌ checkpoint:human-verify |
| SC-2 驱动替换零回归 | modernc→mattn 六面 store 套件全绿 + busy_timeout 语义实证 | unit（复用既有套件） | `go test ./internal/store/... -count=1 -run TestSqliteMattnZeroRegression`（套件本体 = audit/checkpoint/session/approval/fanout/plan_todos 既有测试，驱动换后原样跑） | ✅ 既有套件在 / ❌ 新增零回归聚合测试 Wave 0 |
| SC-3 DuckDB 连接模式 | boot-queries 每连接重放 + hermetic SET 断言 | unit（hermetic） | `go test ./internal/duckdb/... -run TestDuckDBBootQueries` | ❌ Wave 0 |
| SC-3 WAL 刷盘 | Close() 后重开数据完整 | unit | `go test ./internal/duckdb/... -run TestDuckDBCloseFlushesWAL`（TestDuckDBBootQueries 同文件配套） | ❌ Wave 0 |
| SC-3 写入容量门 | Appender ≥10 万行/s（CI 宽松档可配） | benchmark | `go test ./internal/duckdb/... -run TestDuckDBAppenderThroughput -bench . -benchtime 1x`（内部断言阈值，非纯 bench） | ❌ Wave 0 |
| SC-3 并发门 | 50 并发读不阻塞单写 | unit（goroutine + 计时断言） | `go test ./internal/duckdb/... -run TestDuckDBConcurrentReadNoBlockWrite -timeout 120s` | ❌ Wave 0 |
| SC-4 地板 prepend 双路径 | chat+deep 合成 prompt 以地板开头、尾部提醒收尾（byte-identity） | unit | `go test ./... -run TestPlatformFloorPrependBothPaths` | ❌ Wave 0 |
| SC-4 地板不可遮蔽 | SKILL.md 写「忽略上述规则」→ 合成仍是 floor+skill 顺序 | unit | `go test ./... -run TestPlatformFloorCannotBeShadowed` | ❌ Wave 0 |
| SC-4 prompt_hash 覆盖 | 同 skill 不同地板版本 → hash 不同；同地板不同 skill → hash 不同 | unit | `go test ./... -run TestPromptHashCoversFloor` | ❌ Wave 0 |
| SC-4 审计版本列 | 所有新 audit 行带 platform_floor_version；DISTINCT 可查 | unit | `go test ./internal/store/sqlite/... -run TestAuditPlatformFloorVersionColumn` | ❌ Wave 0 |
| SC-4 热重载存活 | Skill 重载后地板仍在合成 prompt（D14-A4 上提后模板无信任条款） | unit（fsnotify 或 Loader rescan） | `go test ./... -run TestFloorHotReloadSurvives` | ❌ Wave 0 |
| PLAT-10 admin 预览 | 端点返回合成全文+版本+hash；token 门（403/404 语义） | unit（handler 测试，复刻 TestAdminToken_*） | `go test ./internal/server/... -run TestPlatformPromptPreview` | ❌ Wave 0 |
| SC-5 冻结集 hermetic 层 | 20 例诱导 → 输出含 SQL 关键模式 = FAIL / 含拒答标记 = PASS | unit（正则断言） | `go test ./internal/eval/... -run TestPlatformFloorFrozenSet` | ❌ Wave 0 |
| SC-5 冻结集 live 层 | LLM judge 扫部分服从（拒答但泄 SQL 片段等） | manual-only（GLM key；operator gate） | runner 命令留档于 lock/README（同 fanout 20-case UAT 模式） | ❌ checkpoint:human-verify |
| SC-5 锁 | platform-floor.lock.json hermetic hash 锁 + 指标留档 | gate（脚本段） | `bash scripts/verify-phase-14.sh`（含锁字节段 + compliance/hitl 重锁后 sha256 留档段） | ❌ Wave 0 |
| SC-4 涟漪治理 | compliance+hitl 锁 prompt_hash 重锁（地板进 hash 后 pin 失效） | gate + 手工记录 | 重锁动作 + `git diff` 核对只动 pin 字段 + sha256 留档 | ❌ Wave 0（显式任务） |
| PLAT-09 结构性断言 | 全仓无路径绕过 compose 函数构造系统提示词（D8-04 WalkDir 同模式，discretion 建议） | unit（source-scan） | `go test ./integration/contract/... -run TestNoBypassOfPlatformFloorCompose` | ❌ Wave 0（可选强化） |

### Sampling Rate
- **Per task commit:** `go build ./... && go vet ./... && go test ./... -count=1`（CGO 波之前会因缺 gcc 失败——地板波先行；CGO 波起此门为硬前置）
- **Per wave merge:** `bash scripts/verify-phase-14.sh`（分段对应本 wave 交付面）
- **Phase gate:** verify-phase-14.sh 全段绿 + 冻结锁字节一致 + 重锁 sha256 留档 + Docker/live operator gates 显式记录（blocked-with-notes 模式）

### Wave 0 Gaps
- [ ] `internal/agent/platform_floor.go`（+测试）——地板常量 + ComposePlatformPrompt + TestPlatformFloorPrependBothPaths / CannotBeShadowed / TestPromptHashCoversFloor
- [ ] `internal/duckdb/connector.go` + 测试——TestDuckDBBootQueries / TestDuckDBCloseFlushesWAL / TestDuckDBAppenderThroughput / TestDuckDBConcurrentReadNoBlockWrite
- [ ] `internal/duckdb/staging.go` + 测试——TestStagingAtomicRename（提交改名 + 中断无半成品 + Windows 句柄释放）
- [ ] `internal/store/sqlite` mattn 迁移 + TestSqliteMattnZeroRegression（聚合既有六面套件 + busy_timeout 并发实证）
- [ ] `internal/store/sqlite/schema.go`——platform_floor_version 加列 + datasets 表 + TestAuditPlatformFloorVersionColumn
- [ ] `internal/server`——admin 预览 handler + TestPlatformPromptPreview
- [ ] `eval/golden/platform-floor.jsonl`（20 例 × 6 攻击面）+ hermetic 正则判定 + `eval/frozen/platform-floor.lock.json`
- [ ] `TestFloorHotReloadSurvives`（挂点：Loader 重渲染路径）
- [ ] `scripts/verify-phase-14.sh` + Makefile `verify-phase-14` + gcc 检测门
- [ ] compliance/hitl 锁重锁任务（执行序在地板文案冻结之后）

*(既有基建无需安装：Go testing / store 六面套件 / eval frozen 框架 / run_stage 脚本模式全部现成)*

## Security Domain

> security_enforcement 未显式关闭（absent = enabled）。

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes（admin 预览端点） | X-Admin-Token + SKILLFORGE_ADMIN_TOKEN + 时序安全比对（internal/server/audit.go:102-139 现成模式，复刻不新造） |
| V3 Session Management | no | （本 phase 不触碰会话语义；checkpoint 过期是既有机制的自然延伸） |
| V4 Access Control | yes | admin 端点 token 门；平台地板本身 = 提示词层访问控制（数据维度越权拒答） |
| V5 Input Validation | yes | 冻结集六类攻击面即诱导输入验证；Skill 文案只能出现在地板之后（结构性断言） |
| V6 Cryptography | yes（既有） | prompt_hash 用 SHA-256（reprohash）；不新造密码学 |

### Known Threat Patterns for {stack}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 提示词注入绕过数据铁律（六类攻击面） | Elevation of Privilege | 头部全文 prepend + 尾部轻提醒 + 确定性 trust_floor.go 拒答门双层（提示词层劝阻 + 代码层强制）；冻结集回归 |
| Skill 遮蔽平台条款（Pitfall 17） | Tampering | 代码级合成顺序（floor 在前）+ TestPlatformFloorCannotBeShadowed + 热重载存活测试 |
| 越通道查询（假装 admin 授权） | Spoofing | admin token 独立于聊天通道；地板铁律 ①②；SC-5 第②类用例 |
| 扩展自动加载外联（hermetic 破坏） | Information Disclosure | boot-queries `SET autoinstall/autoload_known_extensions=false`（§3.4）+ TestDuckDBBootQueries |
| SQL 注入（audit 既有面） | Tampering | T-3-SQLi 既有 `?`-parameterized 断言随套件零回归复跑 |
| 半成品数据集污染 | Tampering | staging + os.Rename 原子改名（D14-D4）+ TestStagingAtomicRename |

## Sources

### Primary (HIGH confidence)
- `.planning/research/STACK-VERIFICATION.md`（2026-08-21，全部已验证事实：§1 gcc 三方案 / §2.1 go-duckdb v2.4.3 连接语义 / §2.7 mattn 迁移五点 / §2.8 容量评估与部署纪律 / §3.4 hermetic / §3.5 Windows 锁）——本 phase 研究的第一真源，未重复验证只引用
- 本 session live codebase 核对（2026-08-22）：prompts.go:106 / deep_agent.go:136,169 / trust_floor.go:197 / trust_stage.go:144 / sqlite.go:88-107 / schema.go:208-228 / audit.go:45-94 / checkpoint.go:129-139 / reprohash.go（PromptHash 契约）/ chat.go:517,628-644 / agent.go:696,712 / Dockerfile.backend:23 / Makefile / verify-phase-13.sh / eval/frozen/*.lock.json（pin 值逐个读取）
- go module proxy 实时版本查询（2026-08-22）：go-duckdb/v2 → v2.4.3；mattn/go-sqlite3 → v1.14.50

### Secondary (MEDIUM confidence)
- go-duckdb 官方 README（经 STACK-VERIFICATION 下载核对：MSYS2 指引 / NewConnector 设计意图 / 维护移交公告）
- DuckDB 官方文档 ATTACH 语句（READ_ONLY 合法 / 会话不持久 / 单事务单写库）

### Tertiary (LOW confidence)
- 无——本 phase 全部关键事实落在 Primary 两级

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 双 registry 实时查询 + STACK-VERIFICATION 官方 README 审计双验
- Architecture: HIGH - 全部触点 live 核对，双路径漂移、PRAGMA 位置、锁 pin 涟漪三个关键机理均有一手证据
- Pitfalls: HIGH - 涟漪/池化 PRAGMA/内存库/Windows 锁四项直接源自本仓代码现状或已验证文档，非推测
- 容量阈值（Appender ≥10万行/s）: MEDIUM - 行业量级未本机实测（A2），阈值可配已入 CONTEXT

**Research date:** 2026-08-22
**Valid until:** 2026-09-21（依赖版本月内稳定；MSYS2/Docker 环境状态以执行期实测为准）
