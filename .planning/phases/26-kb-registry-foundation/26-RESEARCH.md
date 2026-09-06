# Phase 26: 知识库注册表地基 (KB Registry Foundation) - Research

**Researched:** 2026-09-07
**Domain:** Go/React codebase refactor — TierEnum 4-value enum → registry-driven dynamic knowledge bases (KBM-01..06) + milestone hygiene gate (HYG-01)
**Confidence:** HIGH (codebase-grounded; every claim carries a file:line anchor verified in this session)

## User Constraints (from ROADMAP Phase 26 + PROJECT.md v1.2 standards — no CONTEXT.md, autonomous long-range mode)

### Locked Decisions
- **破坏性重构授权** (PROJECT.md standards, 2026-09-07): 不受向后兼容与既有设计约束 — TierEnum → 注册表可破坏性重构；信任语义保留，实现自由。
- **零硬编码**: 格式/扩展名/限额/tier 语义全部配置或注册表驱动（PROJECT.md v1.2 Standards；HYG-02 为 Phase 30 验收，Phase 26 自身不得新增硬编码）。
- **处理全部 TODO** (PROJECT.md standards + HYG-01): 代码 TODO 清零（当前仅 `cmd/spike-history/long.go:54` 一处真实 TODO）+ 纳入结构门。
- **信任语义不削弱** (ROADMAP Phase 26 Notes): KNOW-02 fail-closed / D-09 拒答构造 / citation 白名单 只许迁移实现、不许削弱。
- **KBM-01**: 注册表 = 受控文件装载 + admin 运行时管理；条目含 slug、显示名、语义角色（authority/historical/personal/draft 四选一）、描述、检索启用位、时间戳；`skill` 包消费注册表**接口**而非具体枚举。
- **KBM-02**: admin 端点（token 门）+ `/admin` 视图；slug 合法性校验；创建/停用即审计；停用 = 检索与 Skill 校验不可见，**不物理删除数据**。
- **KBM-03**: authority=唯一可引用证据源（citation 白名单仅 authority 语义角色）；historical=被取代版本留档（不进默认检索，可显式时间点查询）；personal=用户私有（默认排除检索，显式声明才可读）；draft=未生效（永不进检索，仅入库预览）；语义**写入平台地板文档与 README**。
- **KBM-04**: reads 声明知识库 slug 或语义角色通配，validate 对照注册表 fail-closed；旧 4-tier 字面量不再被接受（破坏性切换 + 一次性迁移，既有 Skill 迁移后可正常加载）。
- **KBM-05**: Milvus/Mongo 过滤表达式注册表驱动；未知知识库值在**索引与检索两侧**均 fail-closed 拒绝。
- **KBM-06**: `knowledge/<slug>/` 按注册表装载；未知目录 WARN + 跳过 + 审计。
- **HYG-01**: 处置 `cmd/spike-history/long.go:54` MEM-05 TODO（核对 summarization 中间件真实落地状态后更新或移除）；全仓非测试 Go 文件 TODO/FIXME 计数归零并纳入结构门。
- **审计纪律**: INSERT-only 审计永不删；additive migration only（D14-E1）；admin-token 门（`X-Admin-Token` 常量时间比较）。
- **依赖事实**: 内网 BGE-M3 / Milvus 间歇断（memory: env-intranet-service-flakiness）——涉及真库 re-index 的迁移步骤必须作为 operator gate，hermetic 测试不得依赖真后端。

### Claude's Discretion
- 注册表存储形态细节（受控 YAML + sqlite 的具体分工、表结构、seed 语义）——KBM-01 只锁定「受控文件装载 + admin 运行时管理」二要素。
- reads 新语法的具体分隔符形式（slug 模式 vs 语义角色通配的拼写）。
- admin 端点路径与请求/响应 wire 形状。
- 结构门的具体 grep 形态与挂载位置（`scripts/verify-phase-26.sh` + Makefile target）。
- FE 改造的收窄范围（Phase 26 只要求「不因 N 库而破」+ admin KB 管理视图；GraphCanvas 全面动态化归 Phase 28）。

### Deferred Ideas (OUT OF SCOPE — 来自 ROADMAP/REQUIREMENTS，勿碰)
- 文档上传入库（md/pdf/docx/xlsx/pptx 解析管线）→ Phase 27。
- 图谱 chunk 关系边 + 邻居/路径查询 + GraphCanvas 关系可视化 → Phase 28。
- 反馈落库 / 工单接口 / FeedbackRow 接真 → Phase 29。
- 知识库级 RBAC / 多租户 → v2（REQUIREMENTS v1.2 Out of Scope）。
- 知识图谱 LLM 实体关系抽取 → 明确 Out of Scope。

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| KBM-01 | TierEnum → 注册表（受控文件 + admin 管理）；`skill` 包消费接口 | §Architecture Patterns R1/R2（`internal/kb` 包 + consumer-side interface）；§Consumer Inventory rows 1-4 |
| KBM-02 | admin 创建/停用 + slug 校验 + 审计 + 不物理删除 | §Architecture Patterns R4（端点形态、表设计、audit action 枚举）；§Consumer Inventory row 11 |
| KBM-03 | 四语义角色重定义落档 + citation 白名单仅 authority 角色 | §Architecture Patterns R3（citability 门控点 = chunksByID 插入点）；§Consumer Inventory rows 12-14 |
| KBM-04 | reads 按注册表校验 + 语义角色通配 + 一次性迁移 | §Architecture Patterns R2（新 reads 语法 + 迁移清单：2 个 skills/ + 5 个 examples/ + datarules + eval golden set） |
| KBM-05 | Milvus/Mongo 过滤注册表化 + KNOW-02 双侧 fail-closed | §Architecture Patterns R5；§Consumer Inventory rows 5-10；§Common Pitfalls P1（re-index） |
| KBM-06 | `knowledge/<slug>/` 按注册表装载，未知目录 WARN+跳过+审计 | §Architecture Patterns R6；§Consumer Inventory rows 2, 9；Pitfall P4（indexer/mongo seed 双份走查逻辑必须收敛） |
| HYG-01 | TODO 清零 + 结构门 | §Architecture Patterns R7（门的三条 grep 形态 + false-positive 实测数据）；§Pitfall P6（`deep.TODO` 误报） |
</phase_requirements>

## Summary

Phase 26 把知识层的信任边界从「编译期 4 值枚举」迁移到「运行期注册表」。经全仓扫描，TierEnum 的**真实消费面比 ROADMAP Notes 列出的 4 个锚点多 3 类**：除 `internal/skill`（tier.go/validate.go）、`internal/knowledge`（indexer.go/graph.go）、`internal/retriever/{milvus,mongo}`（filter/seed/查询）、`internal/server`（annotate.go 硬编码 `[]string{"历史"}` 反证通道 + knowledge.go 路径穿越白名单）之外，还有 **(a)** `internal/agent/hitl/hitl_memory_facts.go:235` 把 `[权威,历史,个人]` 硬编码进 LLM 工具 JSON schema 的 Enum，**(b)** `internal/data/rules.go:146` 用 `skill.TierEnumByValue` 校验数据集导入规则的 `tier:` 字段（datarules/loans.yaml:3 实际携带 `tier: 历史`），**(c)** `eval/golden/fanout.jsonl` 的 10 行 golden 用例 reads/tier_filter 全部使用 4-tier ASCII 字面量且被 `eval/frozen/fanout.lock.json` 的 `eval_set_sha256` 字节级钉死——迁移 golden set 必须重签 lock，否则 `internal/eval/fanout_lock_test.go` 直接红。

**Primary recommendation:** 新建 `internal/kb` 叶子包（interface + yaml-seed + sqlite 运行态 + 原子快照），`skill.ResolvedTiers()` 语义从「CN 字符 Milvus 值」改为「注册表 slug 集合」；**seed slug 沿用现有 ASCII 目录名 authority/historical/personal/draft**（零迁移收益最大：现有 SKILL.md reads、knowledge/ 目录、golden set 语法全都不用改拼写，真正破坏性切换体现在 CN 字符值从数据层消失 + TierEnum 映射被删除 + 新增 role 通配语法）；语义角色四值 hard-code 在 kb 包内（角色词表是平台语义不是配置，KBM-03 明确「四选一」），KB 条目本身全注册表驱动。Milvus/Mongo 既有向量携带 CN 字符 tier 值，**必须 drop+recreate/re-seed**（operator gate，BGE-M3 依赖）。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| KB 注册表真源（条目 CRUD + 状态） | Database/Storage (sqlite `knowledge_bases` 表) | API/Backend (`internal/kb` 包) | 运行时管理要持久化；受控 YAML 只是 seed（KBM-01 双要素） |
| reads 契约校验（fail-closed） | API/Backend (`internal/skill` Validate → registry interface) | — | 加载期拒绝，模式与今日 TierEnum 一致，仅真源换成接口 |
| 检索过滤表达式 | Database/Storage (Milvus expr / Mongo `$in`) | API/Backend (`internal/kb` 提供 slug 集合) | PRE-similarity 过滤留在后端（D7 不变式），值来源注册表 |
| KNOW-02 索引侧 fail-closed | API/Backend (`internal/knowledge.validateTiers` → registry) | — | 后端无关层是主闸（03-RESEARCH Pitfall 6 教训），Milvus/Mongo schema 校验是保险带 |
| citation 白名单仅 authority | API/Backend (`ChunkAccumulator.Add` + `trust_stage.go` chunksByID 构建点) | — | grounding map 是本轮检索的物理闭包（MEM-03）；角色门控加在插入点 |
| 反证（counter-evidence）通道 | API/Backend (`internal/server/annotate.go`) | — | 今日硬编码 `[]string{"历史"}`，改为按注册表 role=historical 解析 |
| KB 管理视图 + 动态 badge | Browser/Client (React) | API/Backend (admin 端点) | `/admin` 新 tab + TierBadge 显示名查注册表 |
| TODO/字面量结构门 | Build/CI (`scripts/verify-phase-26.sh` + Makefile) | — | 与 verify-phase-15/16 结构锁同形态 |

## Standard Stack

### Core（零新增依赖——全部已在 go.mod）
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gopkg.in/yaml.v3 | v3.0.1（go.mod:29） | KB seed 受控文件解析 | 与 kpis/、external_views/、SKILL.md 同一解析栈 |
| mattn/go-sqlite3 | v1.14.50 | `knowledge_bases` 表 + INSERT-only 审计 | D14-C1 锁定的 CGO 驱动；migrate() additive 形态现成 |
| fsnotify/fsnotify | v1.5.4 | （仅 skills/ 热重载已有；knowledge/ 本 phase 不加 watcher） | KBM-06 只要求启动期装载 |
| sync/atomic | stdlib | 注册表原子快照（copy-on-write） | `internal/skill/registry.go:92` 的 `atomic.Pointer[map]` 模式直接复刻 |
| cloudwego/hertz | v0.10.6 | admin 端点 | 既有路由表 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| crypto/subtle | stdlib | admin token 门 | `isAdminRequest`（internal/server/audit.go:114-129）现成复用 |
| stretchr/testify | v1.11.1 | 单测断言 | 全仓标准 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| YAML seed + sqlite 运行态（推荐） | 纯受控 YAML（kpis/ 形态） | 无法满足 KBM-02 运行时创建/停用；且 D16-08-E 明确「任何能改 KPI 的 HTTP 面都是注册后门」——但 KBM-02 恰恰要求这个口子，所以 KB 与 KPI 的信任模型**刻意不同**：KB 条目是运营配置，不是承重 SQL 定义 |
| YAML seed + sqlite 运行态（推荐） | 纯 sqlite 表（无 seed 文件） | 新 clone 空注册表 → 全部 Skill 校验失败，开箱即坏；seed 文件保证 4 基线 KB 存在 |
| 可变 `knowledge_bases` 表 + INSERT-only 审计 | 全事件溯源（每变更一行 event，状态=末行投影） | 事件溯源更贴「INSERT-only 即证据」哲学且白得 Phase 24 time-travel 式重建，但代码量大；repo 已有可变配置表先例（`datasets` 表有真 DELETE，store.go:718），配置≠证据的切分成立。**若 planner 愿意多花一个 plan，事件溯源是更优解** |
| slug = 现有 ASCII 目录名（推荐） | 新 slug 命名（强制全量改名） | 后者让「破坏性切换」更诚实，但 golden set/语料/目录/文档全要改，收益只有「迁移路径被真实踩过」；破坏性已由 CN 字符值清除 + TierEnum 删除充分体现 |

**Installation:**
```bash
# 无需安装——本 phase 零新增 Go/npm 依赖（package legitimacy gate 跳过的正当理由）
```

## Package Legitimacy Audit

**本 phase 不安装任何新外部包**（yaml.v3 / fsnotify / sqlite3 / hertz 均已在 go.mod 与 web/package.json），legitimacy gate 无对象，跳过。若 planner 在计划中引入任何新依赖（例如 yaml schema 校验器），必须先过 `slopcheck install <pkg> --json` + `go list -m -versions <pkg>` 双闸。

## Architecture Patterns

### System Architecture Diagram

```
                        ┌──────────────────────────────────────────┐
                        │            internal/kb (新叶子包)          │
                        │  Registry interface + impl                │
                        │  ┌────────────┐   ┌────────────────────┐ │
                        │  │ kb_registry│   │ knowledge_bases    │ │
                        │  │ .yaml      │   │ (sqlite, additive) │ │
                        │  │ (seed,     │   │ slug/role/enabled  │ │
                        │  │ fail-closed│   │ + INSERT-only      │ │
                        │  │ D16-08-D)  │   │ audit rows         │ │
                        │  └─────┬──────┘   └─────────┬──────────┘ │
                        │        └────seed/upsert──────┘            │
                        │              │                            │
                        │     atomic.Pointer[map[slug]Entry]        │
                        │        (copy-on-write 快照)                │
                        └──────┬───────────────┬─────────────┬──────┘
                               │               │             │
              Lookup/Resolve   │               │ List()      │ Create/Deactivate
                               ▼               ▼             ▼
   skills/*.md ──► internal/skill.Validate ─┐  GET /api/admin/kb   POST /api/admin/kb
   (reads: slug/** | role:x)                │  (admin 视图)         POST /api/admin/kb/:slug/deactivate
                                            │                      (X-Admin-Token 门 + audit 行)
   knowledge/<slug>/*.md ──► internal/knowledge.Index ──► validateTiers(registry) ──► Milvus/Mongo
                               (未知目录: WARN+skip+audit)   (fail-closed: 未注册即拒)    tier 列 = slug
                                                                                        ▲
   Skill run: manifest.ResolvedTiers() = [slug...] ──► milvus.BuildMilvusFilter ────────┘
                                                     mongo {tier:{$in: slugs}}
                                                     (空集 → "false"/$in:[] → 0 hit → D-09 拒答)
                                                     (未注册 slug → kb.Registry 拒绝，到不了后端)

   citation 白名单门（KBM-03）:
   检索命中 chunk ──► kb.Role(slug)==authority ? ──yes──► ChunkAccumulator.Add ──► chunksByID ──► GroundingChecker
                                │
                                └─no──► 仅注记通道（annotate.go 形态），绝不入 chunksByID
```

### Recommended Project Structure
```
internal/kb/                  # 新叶子包（不 import internal/skill，避免环）
├── kb.go                     # Role 常量（authority/historical/personal/draft）+ Entry + Registry interface
├── registry.go               # 原子快照 + Lookup/List/ActiveSlugs/SlugsByRole/ResolveReads
├── seed.go                   # LoadSeed(yaml) — fail-closed（D16-08-D 形态）+ upsert 语义
├── slug.go                   # slug 语法校验（^[a-z][a-z0-9-]{0,19}$ + "data" 保留字）
├── store.go                  # sqlite 读写（可变表 + 审计行由调用方经 store.Store 写）
└── *_test.go
internal/skill/
├── tier.go                   # 删除 TierEnum/TierEnumByValue/TierEnumValues/knownTierDirs；
│                             # ResolveTiers 改为薄委托 registry.ResolveReads（保函数名以缩小爆炸半径）
└── manifest.go               # ResolvedTiers() 返回 slug 集合（godoc 更新）
scripts/verify-phase-26.sh    # 结构门（TODO-zero + TierEnum-zero + 字面量 scoped ban）
web/src/components/admin/KnowledgeBaseList.tsx   # /admin 新 tab
```

### Pattern R1: 注册表接口（KBM-01 的「skill 包消费接口」落地）
**What:** 消费方声明接口、`internal/kb` 实现的 D18「interface-first」模式。接口放 `internal/kb`（叶子包，无反向依赖），`skill`/`knowledge`/`server`/`data` 全部依赖接口。
**When to use:** 所有今日调用 `TierEnum`/`TierEnumByValue`/`TierEnumValues` 的位置。
**Example:**
```go
// Source: internal/kb/kb.go（新文件；语义锚定 REQUIREMENTS KBM-01/03）
package kb

type Role string

const (
    RoleAuthority  Role = "authority"
    RoleHistorical Role = "historical"
    RolePersonal   Role = "personal"
    RoleDraft      Role = "draft"
)

// Roles 是封闭词表（KBM-03「四选一」）。角色是平台语义，不是配置项——
// 这 4 个标识符是唯一允许 hard-code 的 tier 词汇（结构门白名单）。
func Roles() []Role { return []Role{RoleAuthority, RoleHistorical, RolePersonal, RoleDraft} }

// CitableRole 是 KBM-03 的代码级唯一真源：citation 白名单只接受该角色。
const CitableRole = RoleAuthority

type Entry struct {
    Slug             string    // ^[a-z][a-z0-9-]{0,19}$（≤ milvus.DefaultTierMaxLen=20）
    DisplayName      string    // UI 徽标文本（seed: 权威/历史/个人/草稿）
    Role             Role
    Description      string
    RetrievalEnabled bool      // KBM-02 停用位；停用≠删除
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

type Registry interface {
    Lookup(slug string) (Entry, bool)   // 仅活跃条目；停用=不存在（fail-closed）
    List() []Entry                      // 含停用（admin 视图用）
    ActiveSlugs() []string              // 检索过滤表达式原料（确定性排序）
    SlugsByRole(r Role) []string        // annotate.go 反证通道 / role 通配解析
    Signature() string                  // 内容 hash → 审计行（D16-A4 形态）
    ResolveReads(reads []string) ([]string, error) // KBM-04：slug/** 或 role:<r> → slug 集合，fail-closed
}
```

### Pattern R2: reads 新语法（KBM-04 破坏性切换 + 一次性迁移）
**What:** 保持既有 `<slug>/**` / 裸 `<slug>` 形态（值从枚举键改为注册表 slug），新增 `role:<r>` 语义角色通配，加载期展开为具体 slug 集合。data 通道（`data:aggregated_only` / `data:external:authorized`）维持 D16-C1 XOR，不动。
**When to use:** 全部 SKILL.md + golden set + `internal/runtime/sub_agent_builder.go` 反向翻译。
**Example:**
```go
// Source: internal/kb/registry.go（新）——替换 internal/skill/tier.go:120 ResolveTiers
// 语义角色通配：role:authority → 所有 role=authority 的活跃 KB slug 集合。
// 展开结果为空 = 加载期错误（fail-closed，镜像今日 unknown-tier-dir 行为）。
// slug 模式：Lookup 失败（未注册或已停用）→ 错误，错误信息列出活跃 slug（镜像
// tier.go:152 的 known 列表错误形态）。
func (r *snapshot) ResolveReads(reads []string) ([]string, error) {
    seen := map[string]bool{}
    out := []string{}
    for _, read := range reads {
        top := read
        if i := strings.Index(read, "/"); i >= 0 {
            top = read[:i]
        }
        if strings.HasPrefix(top, "role:") {
            role := Role(strings.TrimPrefix(top, "role:"))
            slugs := r.SlugsByRole(role)
            if len(slugs) == 0 {
                return nil, fmt.Errorf("reads %q: role %q matches zero active KBs (fail-closed)", read, role)
            }
            for _, s := range slugs { /* dedupe + append */ }
            continue
        }
        e, ok := r.Lookup(top)
        if !ok {
            return nil, fmt.Errorf("reads %q: unknown or deactivated KB %q (active: %v)", read, top, r.ActiveSlugs())
        }
        _ = e
        /* dedupe + append top */
    }
    return out, nil
}
```
**迁移清单（一次性，破坏性切换的证据）：**
| 文件 | 现值 | 动作 |
|------|------|------|
| `skills/simple-qa/SKILL.md:5` | `authority/**` | 不改拼写（slug=authority 仍有效）；建议顺手加 `role:` 示例可留 examples |
| `skills/kpi-dashboard/SKILL.md:5` | `data:aggregated_only` | 不动 |
| `examples/skills/{compliance,simple-qa}/SKILL.md` | `authority/**` | 不改 |
| `examples/skills/compliance-orchestrator/SKILL.md:5-6` | `authority/**` + `historical/**` | 不改（historical-role KB 显式声明才可读 = KBM-03 personal/historical 语义的既有形态） |
| `examples/skills/calculator/SKILL.md:18` | `draft/**`（PLACEHOLDER） | 不改拼写；注释里「draft tier」措辞更新为「draft-role KB」（CONTRIBUTING.md:117 同步） |
| `examples/skills/external-lookup/SKILL.md:5` | `data:external:authorized` | 不动 |
| `datarules/loans.yaml:3` | `tier: 历史` | **必须改**——改为 role 词汇 `tier: historical`（见 Consumer row 20） |
| `eval/golden/fanout.jsonl`（10 行全量） | `reads:[authority/**...]`、`tier_ground_truth:"权威"` | **必须改**：`tier_ground_truth` 从 CN 字符改 slug；然后重签 `eval/frozen/fanout.lock.json` 的 `eval_set_sha256`（见 Pitfall P3） |
| `cmd/spike-fanout/main.go` GoldenSubSkill/GoldenCase | 消费 reads/tier_filter | 语法不变则零改；`tier_ground_truth` 断言值随 golden set 改 |

### Pattern R3: citation 白名单仅 authority（KBM-03 代码强制点）
**What:** 角色门控加在 chunk 进入 grounding 闭包的**插入点**，不在 retriever（检索仍允许显式声明的非 authority 角色，但产物不成为证据）。两个插入点：(a) `internal/agent/trust_floor.go` `ChunkAccumulator.Add`（agent 路径），(b) `internal/server/pipeline.go`（VerdictPipeline 路径 Step 4 之后构建 chunksByID 处，锚点 chat.go:709 提到的 `Citations[].Tier` 载荷 + `internal/runtime/trust_stage.go:190-198`）。
**Why:** MEM-03 的既有不变式是「chunksByID 只含本轮检索」；KBM-03 在同一闸门上叠加「且仅 authority 角色」。非 authority 命中走 annotate.go 的 D5-13 注记通道（已存在、已测试：`TestCounterEvidence_...` 家族）。
**Warning:** 不要把非 authority chunk 原样放进 prompt 再指望 grounding 剥除——LLM 引用了就会触发降级/拒答（行为退化）。正确做法：非 citable chunk 要么不进 prompt，要么以注记框架（`[ANNOTATION]`）进 prompt 且结构上无法成为 `[ref-N]`。
**Example:**
```go
// Source: internal/agent/trust_floor.go（Add 处叠加；锚点 :113 TierMatched/:147 三方一致注释）
func (a *ChunkAccumulator) Add(h retriever.Hit) {
    if a.kb == nil { // 测试/降级路径：nil registry 维持旧行为（fail-open 到今日语义）
        a.addHit(h)
        return
    }
    e, ok := a.kb.Lookup(h.Tier) // h.Tier 已是 slug
    if !ok || e.Role != kb.CitableRole {
        return // 结构性不成为证据；反证/注记由 annotate.go 独立通道处理
    }
    a.addHit(h)
}
```

### Pattern R4: admin 创建/停用端点（KBM-02）
**What:** 与 `internal/server/kpi_admin.go` 同形态的独立文件 `internal/server/kb_admin.go`——admin token 门、nil registry 503、审计行经 `store.PutAudit`。与 KPI 端点的**刻意差异**：KPI 端点只读是结构性纪律（D16-08-E），KB 端点**有意提供** POST 变更面（KBM-02 要求），但每个变更写 INSERT-only 审计行。
**Example:**
```go
// Source: 形态镜像 internal/server/kpi_admin.go:93-99 + internal/server/audit.go:114-129
// POST /api/admin/kb        {slug, display_name, role, description} → 201
// POST /api/admin/kb/:slug/deactivate → 200（retrieval_enabled=0；不删数据）
// GET  /api/admin/kb        → 200 {signature, bases:[...]}（含停用条目，标 active:false）
func (s *Deps) kbCreateHandler(ctx context.Context, c *app.RequestContext) {
    if !isAdminRequest(c) { /* 403，镜像 kpi_admin.go:94-99 */ }
    if s.KBRegistry == nil { /* 503 fail-closed，镜像 kpiPipelineNotWiredError */ }
    var req kbCreateRequest
    if err := c.BindJSON(&req); err != nil { /* 400 */ }
    if err := kb.ValidateSlug(req.Slug); err != nil { /* 400 + slug 规则明文 */ }
    entry, err := s.KBRegistry.Create(req) // 内部: slug 唯一性 + 写表 + 返回
    if err != nil { /* 409 已存在 / 400 非法 */ }
    _ = s.Store.PutAudit(ctx, store.AuditRow{
        Action: "action_kb_created", UserID: adminActor, FrozenArgs: mustJSON(entry),
        // Timestamp/SessionID 等按 audit.go 现有填法
    })
    c.JSON(http.StatusCreated, entry)
}
```
- slug 校验规则（建议，零硬编码体现为 kb 包内单一 const + 校验函数）：`^[a-z][a-z0-9-]{0,19}$`；上限 19 来自 `milvus.DefaultTierMaxLen = 20`（internal/retriever/milvus/config.go:44）留 1 余量——**或将该 const 改为从 kb 包读取，二选一，禁止两处各写一份**。
- 保留字：`data`（与 `data:` 通道前缀冲突，D16-C2）。建议同表拒绝 role 名作为 slug（`authority` 等可以允许作 slug 但会与 role 通配语义混淆——推荐同样保留，理由写进 godoc）。
- 审计 action 枚举新增：`action_kb_created` / `action_kb_deactivated`（镜像 store.go:74-81 的 Phase 11 枚举注释处，须同步更新 AuditRow.Action godoc 枚举清单）。

### Pattern R5: 检索层过滤注册表化（KBM-05）
**What:** `BuildMilvusFilter(tiers, now)` 与 Mongo `{tier:{$in:tiers}}` 的**函数签名与 PRE-similarity 位置不变**，只是传入值从 CN 字符变为 slug。KNOW-02 双侧保留：
- 索引侧：`internal/knowledge/indexer.go:291 validateTiers` 改为 `kb.Registry.Lookup(d.Tier)` 失败即拒（后端无关主闸，Pitfall 6 教训不回退）。
- 检索侧：Skill 加载期 `ResolveReads` 已挡未注册值（到不了 retriever）；retriever 自身保持「空集 → `false` / `$in:[]` → 0 hit」的构造性拒答（filter.go:29-33、mongo.go:316-320）。
- Milvus expr 形态不变：`tier in ["authority","historical"] and (effective_to == 0 or effective_to >= N)`（filter.go:36-37；`%q` 对 ASCII slug 同样注入安全，filter.go:49-52 注释更新）。
- `milvus.DefaultTierMaxLen=20`（config.go:44）注释里的「CN tier names」措辞更新；若 slug 上限与 20 对齐则 schema 无需改。
- Mongo `{tier:1}` 索引（mongo.go:210）与 `{tier:{$in}}`（mongo.go:358）零结构变化。

### Pattern R6: 启动期目录装载（KBM-06）
**What:** `knowledge.Index` 与 `mongo.SeedFromKnowledgeDir` 的固定 `knownTierDirs` 走查（indexer.go:73 / mongo/seed.go:66）改为 `os.ReadDir(knowledgeDir)` + 注册表匹配：
| 目录情况 | 行为 |
|----------|------|
| dir ∈ 活跃 slug | 索引（tier=slug） |
| dir ∈ 注册表但已停用 | 跳过 + `[WARN]`（内容不入检索，符合停用语义） |
| dir ∉ 注册表 | `[WARN]` + 跳过 + **INSERT-only 审计行**（KBM-06 明确要求审计；建议 action=`action_kb_dir_unregistered`） |
| 非 .md 文件 / 空 dir | 维持今日语义（跳过/合法空态） |
**收敛纪律（Pitfall P4）：** indexer.go:73 与 mongo/seed.go:66 是两份手抄目录清单，今日靠「与 TierEnum 同步」注释维系。重构必须收敛为单一 walker（建议放 `internal/knowledge`，`mongo/seed.go` 改为调用它——mongo/seed.go 已 import internal/skill，反向依赖不新增）。
**热重载事实：** `knowledge/` **没有** fsnotify watcher（全仓 fsnotify 仅出现在 internal/skill/{registry,wizard}.go + internal/server/{server,skills}.go + cmd/server/main.go，均服务 skills/）。KBM-06 只要求启动期装载正确；新 KB 的内容入库归 Phase 27 上传管线。运行时创建的空 KB → 检索 0 命中 → D-09 拒答，语义自洽。

### Pattern R7: HYG-01 结构门（TODO 清零 + 字面量门）
**What:** `scripts/verify-phase-26.sh` 三条结构锁（镜像 verify-phase-16.sh Stage 09「structural-constant source-grep locks」形态，脚本 ：319），挂 Makefile target（verify-phase-15/16 已有先例）。
**实测基线（2026-09-07 全仓扫描）：**
- 真 TODO 仅 1 处：`cmd/spike-history/long.go:54` `TODO(phase-10): 把 B3 替换为 summarization 中间件（MEM-05）。`
- **4 处 `deep.TODO` 误报**（eino 的类型名，非 TODO 注释）：`internal/agent/deep_audit.go:158`、`internal/store/sqlite/schema.go:171`、`internal/wire/chunk.go:270,273`。
**MEM-05 真实落地状态（核对结论）：** summarization 中间件**未落地**。全仓 `summariz*` 命中全部是 `cmd/spike-history` 的报表生成（Plan 09-05 的 `--summarize` 标志 + summary.go/summary_test.go），与 MEM-05 摘要压缩无关。Phase 10 实际交付的逃逸方案是 **D10-B3 字符预算窗口**（`internal/history/projection.go`：`TruncateToCharBudget` 是 PRIMARY 且唯一窗口，「legacy 6-message pre-cap is gone」）。因此 TODO 的正确处置是**改写为事实陈述并去 TODO 化**（不能保留任何 TODO 词汇），例如：「B3 演进：Phase 10 以 D10-B3 字符预算窗口作为长会话逃逸方案（MEM-05 摘要中间件未实现，见 STATE.md Phase 10 记录）；spike 观察结论见 .planning/phases/09-*。」

**三条门的具体形态（防误报是本门的唯一技术难点）：**
```bash
# 门 1: TODO-zero —— 匹配 TODO 后随 "(" 或 ":" 或空格+非大写；显式排除 deep.TODO 的 ".TODO" 形态
#   实测形态: 真 = "TODO(phase-10):"；误报 = "deep.TODO.Status" / "deep.TODO shape"
if grep -rnIE '(^|[^.[:alnum:]_])TODO([(]:]|:)' --include='*.go' internal cmd \
   | grep -v '_test.go' | grep -q .; then echo "FAIL: TODO remains"; exit 1; fi
#   替代（更简单更稳）: 约定全仓 TODO 必须带 "(" 或 ":"，门只认这两个字符：
#   grep -rnIE 'TODO\(|TODO:' --include='*.go' internal cmd | grep -v '_test.go'

# 门 2: TierEnum 标识符归零（枚举被删除的编译级证据的 grep 补强）
grep -rn 'TierEnum' --include='*.go' internal cmd | grep -v '_test.go' && exit 1

# 门 3: 语义角色词表封闭 + CN tier 字面量 scoped ban
#   允许: internal/kb/kb.go 的 Role 常量（词表唯一家）、seed yaml、FE 显示名映射
#   禁止: 其余非测试 .go 中出现 "权威"|"历史"|"个人"|"草稿" 作为数据值
#   已知合法残留（门 3 必须白名单，否则红）:
#     internal/platformfloor/platform_floor.go:111,142 —— 「个人/PII 信息」「历史回复」是自然语言非 tier
#     internal/history/verdict_renderer.go:31 —— "[历史结论不可解析]" 显示哨兵
#   推荐做法: 门 3 对 CN 字面量整体 ban + 上述 2 文件白名单 + 每 6 个月复核，而非逐词豁免
```

## Consumer Inventory (TierEnum / tier 字面量 全量消费清单)

> 「What breaks if tiers become dynamic registry slugs」逐行回答。CN 字符列为今日字面量值。

| # | 位置 | 今日形态 | 注册表化后 | 破坏点 |
|---|------|----------|-----------|--------|
| 1 | `internal/skill/tier.go:37,55,76,88,120` | TierEnum map / ByValue / Values / knownTierDirs / ResolveTiers | 删除 map 与 ByValue/Values；ResolveTiers 薄委托 `kb.Registry.ResolveReads` | 接口注入点：Loader/Registry/测试需携带 registry；`SetResolvedTiersForTest` 保语义（值变 slug） |
| 2 | `internal/skill/validate.go:102-146` | data XOR 预扫 + `ResolveTiers(m.Reads)` | 预扫不动；tier 支路换 registry | 空 registry 时全部 tier Skill 校验失败 = 正确 fail-closed，但 boot 顺序必须 seed 先于 Loader.Scan |
| 3 | `internal/skill/manifest.go:87,116` | `resolvedTiers []string`（CN 值） | 同字段存 slug | godoc/下游全部按 slug 重读 |
| 4 | `internal/skill/registry.go:92` + loader.go | 原子快照 + Scan | Registry/Loader 构造需注入 kb.Registry | 构造签名变化波及 cmd/server/main.go:520 |
| 5 | `internal/knowledge/indexer.go:73` | `knownTierDirs` 固定 4 目录 | `os.ReadDir` + 注册表匹配（R6） | 未知目录从「不存在于清单=静默不扫」变「WARN+skip+audit」——审计行落 store，Index 签名需增 store/audit 参数或回调 |
| 6 | `internal/knowledge/indexer.go:141,145` | `skill.TierEnum[tierDir]` 缺失即 fail-fast | `Lookup(slug)` | 同上 |
| 7 | `internal/knowledge/indexer.go:291-303` validateTiers | `TierEnumByValue` + `TierEnumValues()` 错误消息 | `Registry.Lookup(d.Tier)`；错误消息列活跃 slug | KNOW-02 主闸语义原样保留（KBM-05） |
| 8 | `internal/knowledge/graph.go:69,176,201` | ErrInvalidTier + ByValue 复检 + **恒发 4 tier**（TierEnumValues 顺序） | 复检修复为 Lookup；**节点集 = 注册表活跃条目**（N 动态、确定性排序） | graph_test.go:91-107 硬断言 4 tier 顺序必改；FE KnowledgeGraphBrowser「BE 恒返 4 tier」注释（:46-48）失真；GraphCanvas 径向布局本就按 `tiers.length` 循环（GraphCanvas.tsx:58-62），N 动态天然兼容 |
| 9 | `internal/retriever/mongo/seed.go:66-76` | 手抄 tierDirs + TierEnum | 收敛到共享 walker（R6） | 双份走查逻辑是今日潜伏漂移点，P4 |
| 10 | `internal/retriever/milvus/filter.go:57-77` + `mongo/mongo.go:358` | `tier in [%q...]` / `{tier:{$in}}` | 值=slug，签名不变 | 无（PRE-similarity 位置不动，D7 保留） |
| 11 | `internal/retriever/milvus/config.go:44` | `DefaultTierMaxLen=20` 注释 CN 名 | 值不动；slug 上限对齐 20；或改从 kb 包读 | 两处写一份上限 = 新硬编码，禁 |
| 12 | `internal/server/knowledge.go:262-273` | `validKnowledgeTiers` + `tierDirEn` 两个硬编码 map（路径穿越闸） | `Registry.List()` 提供 slug→目录名（slug 即目录名，map 消失）；穿越闸= `ValidateSlug` | docsHandler:298 还有一份内联 4 值循环，三处合一 |
| 13 | `internal/server/annotate.go:76` | `Search(ctx, qvec, []string{"历史"}, 5, now)` 硬编码反证层 | `SlugsByRole(kb.RoleHistorical)`（可为空 → 空集搜索 = 构造性 0 命中，annotate_test.go:147 空态分支已存在） | `TestCounterEvidence_PassesHistoryTierOnly`（annotate_test.go:236-248）断言字面量 `[]string{"历史"}` 必改 |
| 14 | `internal/server/pipeline.go:215,295-296,1006-1010` | ResolvedTiers 读取 + SubprogressStep.Tier + `tier = "权威制度"` 默认建议词 | slug 直通；默认建议词从注册表 DisplayName 派生 | wire.SubprogressStep.Tier 是 string（wire/chunk.go:264），wire 零改；FE 显示走 DisplayName |
| 15 | `internal/agent/trust_floor.go:197,227-235` | 信任地板 + ruleBasedSuggestion 拼 tier 关键词 | 拼 DisplayName（或 slug） | 拒答建议文本变化 → 冻结 eval 的 suggested_alternative_query 断言面需核对 |
| 16 | `internal/agent/hitl/hitl_memory_facts.go:235,236,379-380` | 工具 schema `Enum:[权威,历史,个人]` + 默认 `"权威"` | Enum=活跃 KB DisplayName/slug（构造时快照）；默认=首个 authority-role KB | 构造链需注入 registry（BuildRunner 处）；hitl 测试 :63,218-230,281 字面量必改 |
| 17 | `internal/agent/tools/retrieve.go:91-131` | 闭包快照 ResolvedTiers | 语义不变（slug 闭包） | 无结构变化；tool Desc 文案 "authority / historical / personal" 措辞更新 |
| 18 | `internal/agent/fanout_tools.go:65-70,155-192` | TierFilter ⊆ parent ResolvedTiers | 不变式原样，值=slug | fanout_tools_test.go:33-143 全部字面量必改 |
| 19 | `internal/runtime/sub_agent_builder.go:99-135,217-227` | 交集 + `TierEnumByValue` 反向翻译回 `dir/**` 模式 | slug 即模式顶段，`resolvedTiersToReadPatterns` 变平凡（`slug+"/**"`）；反向查表删除 | FAN-02 SC-2「tier intersection empty」错误字符串是测试契约（sub_agent_builder_test / spike-fanout golden notes），保留字符串 |
| 20 | `internal/data/rules.go:119,146-149` | 数据集规则 `tier:` 用 `TierEnumByValue` 校验（排除 draft） | 改 `kb.Registry`：Lookup 成功且 Role != draft | **受控文件 `datarules/loans.yaml:3` 的 `tier: 历史` 必须迁移**为 role/slug 词汇；Phase 15 导入路径回归 |
| 21 | `cmd/server/main.go:520,598-642,677-736` | 构造 skill.New / Deps / seed 路径 | 装配顺序：kb seed → store migrate → registry → loader；Deps 增 `KBRegistry` | seed 失败 = log.Fatalf（D16-08-D 姿态） |
| 22 | `cmd/spike-fanout/main.go:395-429` + `eval/golden/fanout.jsonl` + `eval/frozen/fanout.lock.json` | golden reads/tier_filter/tier_ground_truth + sha256 钉死 | golden 改 slug 词汇 → **重签 lock** | `internal/eval/fanout_lock_test.go:37-104` 比对 golden LF 归一 sha；不改 lock 直接红（P3） |
| 23 | `web/src/types/verdict.ts:48-49` | `TIERS` 4 值 const + `Tier` 联合类型 | `Tier = string`（保留 TIERS 作为 seed fixture 列表） | 类型放宽即全绿；显示名改查注册表 |
| 24 | `web/src/components/common/TierBadge.tsx:13-18` | `TIER_LABELS: Record<Tier,string>` 4 键 | `Record<string,string>`，值来自 `GET /api/knowledge/bases`（或 admin 端点公开只读投影），miss 时 fallback `` `[${tier}]` `` | 未知 slug 的 chunk（历史审计回放）不再白屏 |
| 25 | `web/src/lib/chunk-lookup.ts:131,148` | fallback `tier: '权威'` 硬编码 | fallback 改 null + 徽标隐藏态 | 硬编码清理项（PROJECT.md 零硬编码标准） |
| 26 | `web/src/store/app-store.ts:251,267` + `types/knowledge.ts:24` | `centerView {tier: Tier}` / `TierNode.tier: Tier` | 类型放宽为 string；doc-view 增加 displayName | GraphTree/GraphCanvas 的 `tier as Tier` 断言（GraphCanvas.tsx:114）可删 |
| 27 | `web/src/components/graph/*` + 测试 | 4-tier 空态注释/断言 | 节点循环已按 `tiers.length`，主要改测试 fixture | GraphCanvas 布局升级归 Phase 28，本 phase 只保不破 |
| 28 | `CONTRIBUTING.md:117` + `README.md:246` | 「declare a placeholder tier (draft/**)」+「Adding a knowledge tier (TierEnum + knowledge/ convention)」 | 改为注册表流程（admin 建库 / seed 文件） | KBM-03「语义写入 README」的落点：README 增四角色语义节 |

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 原子并发注册表快照 | 自旋锁/mutex 包全表 | `atomic.Pointer[map[string]Entry]` copy-on-write（skill/registry.go:92-107 现成形态） | 读路径无锁、写路径换指针；项目已验证该模式（D5-06） |
| admin token 校验 | 新的认证检查 | `isAdminRequest`（audit.go:114-129，`crypto/subtle.ConstantTimeCompare`） | ASVS V6.2.1 既有实现；ADM-01「不新增认证模型」是 v1.0 锁定决策 |
| 内容 hash 签名 | 自创 hash 组合 | 沿用 kpi registry Signature 形态（D16-A4 内容 hash） | 审计行可回放「哪个注册表版本产生了该答案」 |
| Milvus filter 转义 | 手写引号逻辑 | 既有 `%q` 形态（filter.go:67-72） | 已覆盖注入面；只换值不换机制 |
| 受控文件加载/报错 | 新的 loader 框架 | 复刻 kpi.LoadKPIs / externalauth.LoadViewsDir 的 fail-closed + 「空是唯一合法降级态」姿态（cmd/server/main.go:401-409 / 495-506） | 三处已同构，第四处照抄 |
| INSERT-only 审计行 | 新表/新机制 | `store.PutAudit` + `AuditRow.Action` 枚举扩展 | T-3-AUDIT 纪律；kpi_calls 专用表先例（D16-03-A）仅在审计行装不下时才走 |

**Key insight:** 本 phase 的所有机制在仓内都有已验证同构物（受控文件加载 ×2、admin 只读端点 ×6、原子快照 ×1、additive migration ×8、INSERT-only 审计 ×3）。planner 的工作是把既有形态映射到新域，而不是发明新形态——每发明一个新形态就多一个被 30 号 phase「零硬编码抽查」打回的面。

## Runtime State Inventory

> 重构 phase 必答。**逐类核实，未发现也显式声明。**

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| **Stored data** | ① Milvus collection `skillforge_knowledge`（cmd/server/main.go:116）既有行 `tier` VARCHAR 列 = CN 字符（权威/历史/个人/草稿），switch 后 filter 匹配 0 行 → 全量拒答（fail-closed 但功能性宕机）。Insert **不去重**（milvus/seed.go:117 + indexer.go:93-94），原地重跑会叠行。② Mongo `skillforge.knowledge` 同样存 CN 字符；有 `Reset()`（mongo.go:226 DeleteMany）可清。③ sqlite 审计行 `verdict_json` 内嵌 citation tier CN 字符 + `approval_rules.frozen_args` 可能含 `"tier":"权威"`（hitl registry_test.go:107 形态）——**证据数据，禁止改写**（INSERT-only + T-3-AUDIT）。④ sqlite 无 tier 键控表（audit 表无 tier 列）。 | ① drop+recreate collection 或 bump `defaultCollection` 后重索引（operator gate，需 BGE-M3）；② `mongo.Reset()` + re-seed（operator gate）；③ 不迁移，FE 徽标对未知 slug/CN 旧值降级显示（fallback `[slug]`）；④ 无 |
| **Live service config** | 无外部服务配置承载 tier 名（无 n8n/Datadog/Tailscale 类服务）。Milvus/Mongo 集合 schema 由代码 Ensure 创建，不属外部配置。 | None — verified by code inspection（buildRetriever main.go:677-736 无外部 tier 配置面） |
| **OS-registered state** | 无（无 Task Scheduler/pm2/launchd/systemd 注册项；repo 根的 *.exe / server / server.test 是未跟踪的本地构建产物，git status 确认未入库）。 | None — verified |
| **Secrets/env vars** | 无新增密钥。既有 `SKILLFORGE_ADMIN_TOKEN` 复用为 KB admin 门；`RETRIEVER_BACKEND`/`MILVUS_ADDRESS`/`MONGO_URI` 不变。`.env`（本地，未入库）无需改键名。 | None |
| **Build artifacts** | ① `eval/frozen/fanout.lock.json` 的 `eval_set_sha256` 钉死 `eval/golden/fanout.jsonl` LF 归一内容（internal/eval/fanout_lock_test.go:104 断言）——golden 迁移后必须重算并更新 lock（一次显式 re-sign，记录在 SUMMARY）。② `cmd/spike-fanout` 的 `main_test.go:447` 以相对路径直读 golden set，结构断言随行。③ 根目录 `server`/`server.test`/`skillforge-server-dev*` 旧二进制与源码不同步——不阻塞（未被任何门引用），但 verify 脚本若复用会测旧码，建议本 phase 顺手不碰（不在 scope）。 | ① 重签 lock（code edit + 记录）；② 测试随 golden 同步；③ 无 |

**Milvus collection re-index 结论（KBM-05 附加事实）：** tier 是**行内 metadata 列**（D7：单 collection + filter，非 per-tier collection），不存在「向量需重算」——embedding 与 tier 值无关。迁移成本 = 重写 tier 列值，但 Milvus SDK 路径无 upsert-by-PK 的现成封装（seed.go:117 明言「unless upsert is used」且未用），故**最便宜的正确路径是 drop+recreate+重嵌**（成本：一次 BGE-M3 全量 re-embed，16 篇语料 × <4KB ≈ 分钟级）。Mongo 侧 `Reset()` 已备。

## Common Pitfalls

### Pitfall 1: CN 字符 tier 残留在向量库 → 切换后「全部静默拒答」
**What goes wrong:** 代码切换为 slug 但 Milvus/Mongo 行仍存 CN 字符；`tier in ["authority"]` 匹配 0 行，每个 tier 查询都拒答。系统「健康」（D-09 正确拒答），功能全灭，且无人报错。
**Why it happens:** boot 索引路径是「已存在则 Insert 叠行」，不清理旧值；Milvus 侧连 drop 都没有现成函数。
**How to avoid:** (a) 本 phase 交付一个显式 re-seed 路径（milvus DropCollection/新 collection 名 + mongo.Reset()）；(b) 验收含一条 live 检查：检索返回非零命中（operator gate）；(c) hermetic 层用 indexSeedHook 断言新 docs 的 Tier 值 ∈ 活跃 slug 集。
**Warning signs:** 重启后 `/eval` refusal 率 100%、graph 各节点 chunkCount=0。

### Pitfall 2: KBM-03 语义收窄把反证/历史 Skill 打断
**What goes wrong:** 「citation 白名单仅 authority」若错误实现成「检索层只查 authority」，`examples/skills/compliance-orchestrator`（reads 权威+历史）与 annotate.go 反证通道直接失效；若实现成「prompt 里照放历史 chunk」，LLM 引用后 grounding 剥除 → 无辜降级。
**Why it happens:** 把「不可引用」误译为「不可检索」。
**How to avoid:** 三层分明——检索按 reads（explicit-only 角色仍可达）；prompt 呈现按 citable（authority 进 `[ref-N]`，非 authority 走 `[ANNOTATION]` 框架或直接不进 prompt）；grounding 闭包只收 citable（R3 插入点）。`annotate_test.go:163-212` 的 chunksByID 快照测试是现成的回归闸，必须保持绿。
**Warning signs:** compliance-orchestrator golden 用例（fanout-01/02/06/07）merged_status 漂移。

### Pitfall 3: 改 golden set 忘记重签 lock（或反过来以为改不得）
**What goes wrong:** `eval/golden/fanout.jsonl` 改 slug 后 `internal/eval/fanout_lock_test.go` 红（sha 不匹配）；或 planner 见红而回退 golden 改动，留下「语法已切换但评测集还在旧词汇」的假绿。
**Why it happens:** lock 钉的是**内容 hash**（fanout_lock_test.go:104），这是 D14-B4「重签=显式行为」纪律的作用对象。
**How to avoid:** 计划里把「改 golden → 重算 sha256 → 更新 fanout.lock.json → SUMMARY 记录 re-sign 理由」写成单一 task 的连续步骤；同时核对 `cmd/spike-fanout/main_test.go:447` 的结构断言与 `verify-phase-13.sh` Stage 7 引用。
**Warning signs:** `TestFanoutLock*` 红；`go run ./cmd/spike-fanout` 报 golden 解析错。

### Pitfall 4: indexer 与 mongo/seed 的双份目录走查只改一份
**What goes wrong:** `RETRIEVER_BACKEND=mongo` 部署下 KBM-06 的「WARN+skip+audit」缺位或语义漂移（今日两份清单靠人肉同步，indexer.go:73 注释自述「Mirrors internal/skill.TierEnum keys」）。
**How to avoid:** 收敛为单一 walker（internal/knowledge），mongo/seed.go 调用之；两侧各留一条共享行为测试（同一临时目录树 → 相同 skip/audit 决策）。
**Warning signs:** mongo 后端下未知目录无 WARN/审计行。

### Pitfall 5: 结构门自身误报（deep.TODO / platformfloor 自然语言）
**What goes wrong:** TODO 门用裸 `\bTODO\b` → 4 处 `deep.TODO` 类型名误报；CN 字面量门不分语境 → `platform_floor.go:111`「个人/PII 信息」、`:142`「历史回复」、`verdict_renderer.go:31`「历史结论」误报，门第一天就红，团队学会 `# noqa` 式绕过，门废。
**How to avoid:** 门 1 只认 `TODO(` / `TODO:`（并写入 CONTRIBUTING 作为全仓约定）；门 3 白名单上述 2 文件 + internal/kb 词表家 + seed yaml + FE 显示映射，白名单带注释与复核日期。
**Warning signs:** verify-phase-26 首跑即红于 platformfloor。

### Pitfall 6: seed 文件与 sqlite 运行态的语义含糊（drift 或覆盖管理员决策）
**What goes wrong:** 两种失败方向——(a) boot 总是 upsert YAML → 管理员在运行时停用的 KB 每次重启被 seed 复活；(b) YAML 只在空表时 seed → YAML 后续改动（PR 新增 KB）对已有部署永不生效。
**How to avoid:** 明确选定并写进 godoc：推荐「YAML 条目按 slug **always upsert 声明字段**（slug/display_name/role/description），但 `retrieval_enabled` 以 sqlite 现值优先（admin 运行时状态不被 seed 覆盖）」——即「结构归配置，开关归运营」。若 planner 选纯「空表才 seed」，必须在 README 写明「改 seed 后已有部署需手工同步」，并接受 KBM-01「受控文件装载」打折。
**Warning signs:** 重启后停用库复活（a）；PR 加的 KB 老部署看不到（b）。

### Pitfall 7: audit 行污染 — Index 签名被迫携带 Store
**What goes wrong:** KBM-06 要求「未知目录审计」，但 `knowledge.Index(ctx, dir, emb, mc, cfg)` 无 store 依赖；粗暴加 `store.Store` 参数会把纯索引函数与 sqlite 耦合，`cmd/eval` / 测试路径全要跟。
**How to avoid:** 用回调注入（镜像既有 `indexSeedHook` / `yamlUnmarshal` 的测试间接层，indexer.go:354/438）：`Index` 增 `AuditFn func(action string, detail string)` 可选参数，production 由 main.go 绑 store.PutAudit，hermetic 测试绑内存收集器。
**Warning signs:** indexer_test 为凑签名造 sqlite 内存库。

## Code Examples

### 受控 seed 文件形态（knowledge_bases.yaml）
```yaml
# Source: 形态复刻 kpis/loans.yaml 头注（D16-A1 受控文件纪律）
# CONTROLLED FILE: versioned in git, changed via PR review.
# 语义角色词表（authority/historical/personal/draft）是平台常量，见 internal/kb/kb.go Roles()。
# retrieval_enabled 在此仅是 seed 初值；运行时以 sqlite 现值优先（管理员停用不被本文件复活）。
version: "1"
knowledge_bases:
  - slug: authority
    display_name: 权威
    role: authority
    description: 唯一可引用证据源（citation 白名单仅此角色）
    retrieval_enabled: true
  - slug: historical
    display_name: 历史
    role: historical
    description: 被取代版本留档；不进默认检索，可显式时间点查询；不可引用
    retrieval_enabled: true
  - slug: personal
    display_name: 个人
    role: personal
    description: 用户私有；默认排除检索，显式声明才可读；不可引用
    retrieval_enabled: true
  - slug: draft
    display_name: 草稿
    role: draft
    description: 未生效；永不进检索，仅入库预览；不可引用
    retrieval_enabled: true   # draft 角色的检索闸由 Role 语义强制关闭（RoleDraft 永不进 Search 集合）
```
> 注意：draft 条目即使 `retrieval_enabled: true` 也永不参与检索——角色语义（KBM-03「永不进检索」）优先于条目开关，这个优先级必须在 `ActiveSlugs()`/`RetrievalEligible()` 里体现并单测锁定，否则一个 seed 手滑就击穿「草稿永不检索」。

### additive migration（knowledge_bases 表）
```sql
-- Source: internal/store/sqlite/schema.go migrate()-tail 形态（D14-E1 纪律，kpi_calls 先例 :442-465）
-- 独立 const + migrate() 尾部执行；新表无 ALTER 依赖，但索引仍走 tail 惯例保持单一形态。
CREATE TABLE IF NOT EXISTS knowledge_bases (
    slug              TEXT PRIMARY KEY,
    display_name      TEXT NOT NULL,
    role              TEXT NOT NULL CHECK (role IN ('authority','historical','personal','draft')),
    description       TEXT NOT NULL DEFAULT '',
    retrieval_enabled INTEGER NOT NULL DEFAULT 1,   -- 0 = 停用（不物理删数据，KBM-02）
    created_at        DATETIME NOT NULL,
    updated_at        DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_role ON knowledge_bases(role);
```

### role 优先级闸（draft 永不检索）
```go
// Source: internal/kb/registry.go（新）——KBM-03 机械化的唯一裁决点
// RetrievalEligible 返回可进入检索过滤集合的 slug。
// 规则: draft 角色 → 永不（无论开关）；其余角色 → 条目开关。
// 锁测试: TestDraftRoleNeverRetrievable（seed 手滑 retrieval_enabled=true 仍被拒）。
func (s *snapshot) RetrievalEligible() []string {
    out := []string{}
    for _, e := range s.entries {
        if e.Role == RoleDraft || !e.RetrievalEnabled {
            continue
        }
        out = append(out, e.Slug)
    }
    sort.Strings(out)
    return out
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `TierEnum` 4 值 map（tier.go:37） | `kb.Registry` 接口 + seed + sqlite | 本 phase | `skill` 包对知识库集合的去知识化（KBM-01 原文） |
| reads 顶段 ∈ 枚举键 | reads 顶段 ∈ 注册表 slug ∪ `role:<r>` 通配 | 本 phase | 未来多 KB 同角色可一通配全含 |
| graph 恒发 4 节点（graph.go:26,201） | 节点集 = 活跃注册表条目 | 本 phase（Phase 28 加关系边） | graph_test 与 FE 空态注释需同步 |
| 未知目录 = 不在清单所以静默 | 未知目录 = WARN + skip + audit | 本 phase | 运营可见性 |
| 「Adding a knowledge tier = 改代码 + 建目录」（README:246） | 「= admin 建库（或 seed PR）+ 建目录」 | 本 phase | CONTRIBUTING/README 同步 |

**Deprecated/outdated:**
- `skill.TierEnum` / `TierEnumByValue` / `TierEnumValues` / `knownTierDirs`（tier.go + indexer.go:73 + mongo/seed.go:66）：删除。D17 决策（STATE.md）记录的「TierEnum 反查 helpers」历史使命终结，STATE/ROADMAP 不必改写（身份锚点纪律，D8-05），但新代码不得引用。
- CN 字符作为**数据层** tier 值：退役，仅存于历史审计证据与 display_name。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | seed slug 沿用 authority/historical/personal/draft 是用户可接受的「破坏性切换」解读（切换 = CN 值清除 + 枚举删除 + 语料词汇升级，而非强制改名） | Summary / R2 | 若用户预期「连 ASCII 名也换」，迁移面扩大到全部 SKILL.md + 语料目录 + golden；需 plan 前确认或 plan 内置 checkpoint:human-verify |
| A2 | KBM-03「historical 不进默认检索」的机械实现 = 显式声明（reads）才可达 + role 开关可关；effective_to 窗口继续承担「被取代版本」过滤 | R6 / Assumptions | 若用户意图是「historical 一律不可经 reads 声明」，compliance-orchestrator 等 Skill 迁移方案不同 |
| A3 | draft 角色永不检索优先于条目 retrieval_enabled 开关 | Code Examples | 若开关可覆盖角色语义，「草稿永不检索」承诺可被 seed 手滑击穿 |
| A4 | seed「结构归配置、开关归运营」的 upsert 语义 | Pitfall 6 | 选错方向 → 管理员停用被复活（安全语义回退）或 seed 变更对存量部署无效 |
| A5 | Milvus drop+recreate + 全量 re-embed（16 篇 <4KB）成本可接受，operator gate 可等 BGE-M3 恢复 | Runtime State / Pitfall 1 | 若语料规模预期大增，需改写成 tier 列 in-place 迁移（Milvus 无 upsert 封装，成本更高） |
| A6 | `role:` 前缀作通配分隔符（非 `@` / `%`） | R2 | 纯语法选择；与 `data:` 前缀风格一致故推荐，无功能风险 |
| A7 | 审计 action 新枚举 `action_kb_created` / `action_kb_deactivated` / `action_kb_dir_unregistered` 命名 | R4 / R6 | 命名不符既有 `action_*` 风格会被 AuditFilter 消费方（admin_audit 视图）漏显；无功能风险 |

## Open Questions

1. **可变表 vs 事件溯源注册表**
   **RESOLVED (26-02):** 采用可变表 knowledge_bases + INSERT-only 审计行（internal/store/sqlite/kb_registry.go 的 UPDATE retrieval_enabled = 停用语义；事件溯源的时间点重建留作 Phase 24 time-travel 的未来能力）——见 26-02-PLAN.md Task 1/2。
   - What we know: KBM-02 要停用不删 + INSERT-only 审计；repo 已有可变配置表先例（datasets 有真 DELETE）也有 INSERT-only 生命周期先例（approval_rules）。
   - What's unclear: `knowledge_bases` 用可变表（UPDATE retrieval_enabled）还是全事件行（末行投影）。
   - Recommendation: **可变表 + 审计行**（本 phase 体积可控）；Phase 24 的 time-travel 模式未来可在审计行上重建任意 T 时刻注册表，不必现在付事件溯源的代码成本。若 planner 判体量允许，事件溯源是上限更高的选择（A4 联动）。
2. **`GET /api/knowledge/bases` 公开只读投影是否存在**
   **RESOLVED (26-05 GET /api/knowledge/bases):** 新增公开只读端点（仅活跃条目的 slug/display_name/role，无 token、无 description/enabled），admin 端点保留完整视图——见 26-05-PLAN.md Task 1；FE 消费在 26-08 的 web/src/lib/kb-bases.ts。
   - What we know: FE TierBadge/管理视图需要 slug→display_name 映射；chat 用户侧徽标不该要求 admin token。
   - What's unclear: 复用 admin 端点（FE 聊天页无 token）还是新增公开只读端点。
   - Recommendation: 新增公开 `GET /api/knowledge/bases`（返回活跃条目的 slug/display_name/role；无描述无开关——信息面最小化），admin 端点保留完整视图。
3. **`cmd/eval` / `internal/eval` 是否消费 tier 值**
   **RESOLVED — verified (planner 复核):** internal/eval 与 web/src/components/eval/ 均 0 tier 引用（grep 复核 2026-09-07）——eval 链唯一触面是 fanout golden/lock 的词汇迁移（26-07），无 FE 报表视图清单项，LOW residual risk 关闭。
   - What we know: 全仓扫描 `internal/eval` 仅 fanout_lock_test 触及 golden 文件；`cmd/eval` 未见 tier 分支。
   - What's unclear: eval 报告/reports INDEX 是否在 UI 按 tier 分组（若有则 FE 报表视图也在清单）。
   - Recommendation: planner 执行时 grep `internal/eval` + `web/src/components/eval` 复核一次（本 research 已扫，未见 tier 依赖；标 LOW residual risk）。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25 | 全部 BE | ✓ | go.mod `go 1.25.0` | — |
| WSL (CGO build/test) | cmd/server / internal/data / internal/store/sqlite 测试（D15-01-1） | ✓（既有工作流，memory: env-wsl-backend-runbook） | — | Windows 上 hermetic 非 CGO 包可跑；CGO 包 blocked-with-notes 先例 |
| Milvus | KBM-05 live 重索引验收 | ✗ 间歇（memory: env-intranet-service-flakiness） | — | hermetic 用 indexSeedHook；live 归 operator gate |
| BGE-M3 (10.62.0.123:11434) | re-embed | ✗ 间歇（STATE.md Blockers：no route to host 记录在案） | — | 同上；迁移验收明确挂 operator gate |
| MongoDB | mongo 后端分支测试/live | 间歇 | driver v2.8.0 | hermetic mongo_test 已有 stub 形态 |
| Node/npm + vitest | FE 测试 | ✓ | web/package.json `vitest run` | — |

**Missing dependencies with no fallback:** 无（BGE/Milvus 属 operator gate 而非缺失）。
**Missing dependencies with fallback:** 见上表 Fallback 列。

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + testify v1.11.1（BE）；vitest（FE） |
| Config file | 无 Go 配置文件（约定优于配置）；FE `web/vitest.config.*` + `web/package.json` |
| Quick run command | `go test ./internal/kb/ ./internal/skill/ ./internal/knowledge/ -count=1` |
| Full suite command | `go test ./... -count=1`（Windows hermetic 子集）/ WSL `go test ./... -count=1`（CGO 全量）+ `cd web && npm test` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| KBM-01 | seed 加载 fail-closed + Registry 接口语义 + 原子快照 | unit | `go test ./internal/kb/ -count=1` | ❌ Wave 0 |
| KBM-01 | `skill.Validate` 经接口校验（未注册/停用=失败；role 通配展开） | unit | `go test ./internal/skill/ -run TestValidate -count=1` | ✅（改断言） |
| KBM-02 | create/deactivate 403/400/409/201 + 审计行 + 停用后 Lookup 不可见 | integration（httptest） | `go test ./internal/server/ -run TestKBAdmin -count=1` | ❌ Wave 0 |
| KBM-03 | draft 永不检索（开关不可覆盖）+ citability 门（非 authority 不入 chunksByID） | unit | `go test ./internal/kb/ ./internal/agent/ -run 'TestDraftRole|TestCitable' -count=1` | ❌ Wave 0 |
| KBM-03 | D5-13 反证通道回归（chunksByID 快照不变）保持绿 | unit | `go test ./internal/server/ -run TestCounterEvidence -count=1` | ✅（改字面量断言） |
| KBM-04 | 既有 7 个 SKILL.md 迁移后可加载 + calculator PLACEHOLDER 注释更新 | integration | `go test ./internal/skill/ -run TestExamplesValidate -count=1` | ✅ |
| KBM-05 | milvus filter slug 表达式 + 空 slug 集 `"false"`；mongo `$in` + KNOW-02 双侧拒 | unit | `go test ./internal/retriever/... ./internal/knowledge/ -run 'TestBuildMilvusFilter|TestIndexer|TestSearch' -count=1` | ✅（改断言） |
| KBM-06 | 未知目录 WARN+skip+audit；停用目录 skip；共享 walker 两侧一致 | unit | `go test ./internal/knowledge/ -run TestIndex -count=1` | ✅（扩用例） |
| HYG-01 | TODO-zero / TierEnum-zero / CN 字面量 scoped ban 三门 | structural gate | `bash scripts/verify-phase-26.sh` | ❌ Wave 0 |
| 全局 | 冻结 lock 字节一致性（compliance/hitl/platform-floor 不动）+ fanout 重签 | gate | `go test ./internal/eval/ -count=1` | ✅ |

### Sampling Rate
- **Per task commit:** `go test ./internal/kb/ ./internal/skill/ ./internal/knowledge/ ./internal/server/ -count=1`（< 30s，纯 hermetic）
- **Per wave merge:** `go test ./... -count=1` + `bash scripts/verify-phase-26.sh` + `cd web && npx vitest run`
- **Phase gate:** WSL `make check-cgo` + `go build ./...` + `go vet ./internal/server/ ./cmd/server/` 全绿 + FE `npx tsc --noEmit` exit 0（HYG-03 预演）+ live re-index/检索命中作为 operator checkpoint

### Wave 0 Gaps
- [ ] `internal/kb/`（包 + registry/seed/slug/store + 全套单测）——覆盖 KBM-01/02/03 核心
- [ ] `internal/server/kb_admin_test.go`——KBM-02 端点 + 审计断言（含 TestKBAdmin retrieval verbs 结构锁的反向：create 端点**必须**有 POST，勿照抄 KPI 只读锁）
- [ ] `scripts/verify-phase-26.sh` + Makefile target——HYG-01 三门（先写门、后清 TODO，RED→GREEN 顺序与 D8-04 负向证明纪律一致：门先证明能红，再清场变绿）
- [ ] `internal/knowledge/indexer_registry_test.go`——未知目录/停用目录/审计回调三态（P4 共享 walker 两侧各一条）

## Security Domain

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | 无用户体系（v1.2 Out of Scope：RBAC/多租户） |
| V3 Session Management | no | 无会话认证面 |
| V4 Access Control | yes | `X-Admin-Token` 常量时间门（audit.go:114-129，ASVS V4 先例 ADM-01）；KB 变更面仅 admin 可达 |
| V5 Input Validation | yes | `kb.ValidateSlug` 白名单正则 + `filepath.Base(file)==file` 既有穿越闸（knowledge.go:326-338）保持——slug 成为路径段后穿越面不变，但白名单从 4 值变开放式，**必须**继续走正则闸而非查表直通 |
| V6 Cryptography | yes | 既有 `crypto/subtle`；注册表 Signature 用 sha256 内容 hash（不引入新原语） |
| V7 Error/Logging | yes | W-3 姿态：500 envelope 不带 err.Error()（knowledge.go:38-42 既有） |
| V14 Config | yes | 受控文件 fail-closed（D16-08-D）；seed 不覆盖运行时停用（Pitfall 6，防安全语义回退） |

### Known Threat Patterns for 本 phase 触面
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 注册表后门（HTTP 面 KPI 化教训反用） | Elevation of Privilege | 变更面仅 `POST /api/admin/kb*` + token 门 + 每变更 INSERT-only 审计行；无 delete 端点（KBM-02 无删除） |
| slug 路径穿越（`tier=../cmd` 教训，knowledge.go:258-261 自述） | Tampering | `^[a-z][a-z0-9-]{0,19}$` 白名单 + Join 前校验；目录名=slug 单层，禁 `os.ReadDir` 用户输入直拼 |
| 停用复活（seed 覆盖运营态） | Tampering / Repudiation | 「结构归配置、开关归运营」seed 语义 + 重启后停用态保持测试 |
| 灯下黑拒答（CN 残留 → 全拒答看似安全） | Denial of Service | Pitfall 1 live 验收（检索非零命中）+ graph chunkCount 检查 |
| 非 authority chunk 混入证据（T-5-25 同族） | Elevation of Privilege | R3 插入点门控 + annotate chunksByID 快照测试保持绿 |

## Sources

### Primary (HIGH confidence)
- 本仓代码直接核查（2026-09-07，全部 file:line 为当次会话 Read/Grep 产物）：`internal/skill/{tier,validate,manifest,loader,registry,router}.go`、`internal/knowledge/{indexer,graph}.go`、`internal/retriever/{retriever.go,milvus/*,mongo/*}`、`internal/server/{server,knowledge,annotate,pipeline,kpi_admin,audit}.go`、`internal/agent/{trust_floor,runner_setup,fanout_tools}.go`、`internal/agent/{tools/retrieve.go,hitl/hitl_memory_facts.go}`、`internal/runtime/sub_agent_builder.go`、`internal/data/rules.go`、`internal/history/projection.go`、`internal/store/{store.go,sqlite/schema.go}`、`internal/platformfloor/platform_floor.go`、`cmd/server/main.go`、`cmd/spike-fanout/main.go`、`cmd/spike-history/long.go`
- 受控文件与评测数据：`kpis/loans.yaml`、`external_views/v_user_consumption.yaml`、`datarules/loans.yaml`、`eval/golden/fanout.jsonl`、`eval/frozen/fanout.lock.json`、`skills/*/SKILL.md`、`examples/skills/*/SKILL.md`
- FE：`web/src/types/{verdict,knowledge}.ts`、`web/src/components/common/TierBadge.tsx`、`web/src/components/graph/*`、`web/src/lib/chunk-lookup.ts`、`web/src/store/app-store.ts`、`web/src/components/admin/AdminDashboard.tsx`

### Secondary (MEDIUM confidence)
- `.planning/{ROADMAP,REQUIREMENTS,PROJECT,STATE}.md` v1.2 节（决策契约，本文件 User Constraints 节逐字对齐）
- `internal/eval/fanout_lock_test.go`（sha256 钉死机制推理自测试断言注释）

### Tertiary (LOW confidence)
- Open Question 3（internal/eval / FE eval 组件的 tier 残留）——本 research 已扫未见，建议 planner 执行时单点复核

## Metadata

**Confidence breakdown:**
- Consumer inventory: HIGH — 每行带 file:line，本会话逐文件 Read/Grep 核实；两处内联清单（knowledge.go:298、mongo/seed.go:66）均在清单内
- Registry design: HIGH（机制侧——全部复刻仓内已验证形态）/ MEDIUM（seed-vs-sqlite 语义与 slug 命名策略属 A1/A4 假设）
- Pitfalls: HIGH — 每条均有仓内代码证据（如 Insert 不去重 seed.go:117、golden sha 钉死 fanout_lock_test.go:104、deep.TODO 误报实测）
- Validation: HIGH — 沿用 verify-phase-15/16 既有门形态

**Research date:** 2026-09-07
**Valid until:** 2026-10-07（稳定——代码面已冻结于 v1.1.1；若 26 执行前有其他 phase 落地触及上述清单须重扫）
