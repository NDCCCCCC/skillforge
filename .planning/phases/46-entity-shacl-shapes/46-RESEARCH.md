# Phase 46: Entity SHACL Shapes - Research

**Researched:** 2026-10-04
**Domain:** Go 纯自研 SHACL 引擎扩展（severity 支持 / 多 shapes 文件 / 违规分层）+ 实体类 ABox shapes 交付 + governance UI 分层呈现
**Confidence:** HIGH（全部落点代码逐行核对；W3C SHACL 规范原文核对；live 快照数据实证）

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D-46-01（C2 安静全绿 — severity 政策进 shapes 文件）：**
- 引擎：`validate.go` 的 `parsePropertyShape` 新增 `sh:severity` 解析分支；`:428-434` 的硬编码推导（`path==sourcePath→warning, else error`）替换为「解析值或默认 error」。W3C 语义映射：`sh:Violation`（未声明时的默认）→ error 隔离，`sh:Warning`/`sh:Info` → warning 放行——D-SHACL-02 语义保持。默认=error 保证既有 8 个 TBox shapes 行为字节不变。
- 实体 shapes 约束清单：只把提取器无条件保证的属性设为必填@error——`rdf:type` hasValue 自检 + `rdfs:label` minCount 1（abox.go:210-211 无条件发出）；`skf:entityCode` / `skf:parentOrganization` / `skf:belongsToTier` 一律 **if-present 质量检查**（datatype/class，无 minCount）。live 数据 127 实体（71 险种 + 56 机构）首跑**零隔离、health=ok**。
- sourcePath 特例退役：D-PROV-10 的 `path==sourcePath→warning` 引擎硬编码退役——在 shapes.ttl 的两个 sourcePath PropertyShape 上显式声明 `sh:severity sh:Warning`，行为不变、政策归位数据。

**D-46-02（b 路径列表 — 延伸既有机制 + 修复 rollback no-op）：**
- 实体 shapes 独立文件 `internal/ontology/shacl/entities.ttl` 随仓库交付（与 shapes.ttl 同目录同家族）。
- `SKILLFORGE_SHACL_SHAPES` 延伸为**路径列表**（`filepath.SplitList`，os 路径列表分隔符）；默认值 = 既有 shapes.ttl + 新 entities.ttl。运行时是 WSL Linux，无 Windows `;`/`:` 分隔符歧义。
- `induction.go:383` rollback 重校验改读同一 env（当前硬编码 `data/shacl/shapes.ttl`——目录不存在，warn-and-return-0 静默 no-op）：修复 D-PROV-09 rollback/boot shapes parity 缺口。
- 多文件 parse 的错误聚合不得静默跳过缺失路径（「不允许静默吞错」标准）；`Report.ShapesPath` 保持单 string（join 显示），FE 类型零变化。
- `shapes.ttl:4` 陈旧头注释（声称默认 `data/ontology/shacl.ttl`，与 main.go:930 实际不符）一并修正。

**D-46-03（A 后端盖戳）：**
- 校验循环在 stamping 点（`v.Entity` 赋值处）给 `Violation` 新增 `target_class` + `layer` 字段（`omitempty`）：`ns.TargetClass` 循环内现成可用，确定性派生、可测试。layer 枚举 = `"tbox"` | `"entity"`；`checkGlobalReferences` 的跨实体完整性违规用 `"global"` 哨兵，避免按实体误分类。
- UI = 标签问题而非结构问题：每行违规加 layer/class 徽章 + 红黄区头部 per-layer 计数；不加新分区、不加分页 tab（D-SHACL-03 三色区语义不动）。
- FE 咽喉点 null-guard（governanceApi.ts:57-60）是红线先例——报告虽为 in-memory 每次 boot 重生成，null-guard 先例仍是红线。
- 否决 FE-only 分组：`Violation.entity` 是实例 IRI 非类名，前缀启发式违反零硬编码。

**D-46-04（a+c 保持 + 计数）：**
- boot 隔离循环（main.go:937-941）**subject-only 保持不动**。SC2「不出现在正常 KG 查询结果中」正式解读为 **subject 位置不可见**；对象位悬挂边可见但带计数。
- 报告新增只读 `dangling_edge_count`（每 quarantined entity）：对入边谓词 `involvesInsuranceProduct` / `involvesOrganization` / `parentOrganization` 反向 Query 计数，governance UI 展示。
- 否决级联隔离：`store.Quarantine` 按 `t.S` 键控——级联会误把健康 note 记入隔离并污染 TripleCount；隔离后 debounced snapshot 只序列化 `s.all` 会把删除永久化；破坏 Run ECL 恢复路径。

### Claude's Discretion
- env 默认列表值的确切形态、多文件 parse 错误聚合的具体结构（约束：不静默吞缺失路径）。
- entities.ttl 内部组织（沿用 shapes.ttl 的分节注释风格）与 PropertyShape 命名。
- `dangling_edge_count` 字段落点（`QuarantinedEntity` struct 内 vs Report 顶层）与入边谓词枚举的实现位置（Go 常量 vs shapes 派生；零硬编码标准约束的是 IRI 命名空间与 shapes 路径，不强制本 helper）。
- layer 枚举确切字符串、徽章样式与区头文案细节（46-UI-SPEC / ui-phase 域）。
- SC2 注入测试的形态（单测 fixture KG vs boot 级测试）与 verify-phase-46.sh 的测试编组（照 45 模式：fail-fast + anti-vacuous PASS 计数；shacl 包纯 Go 可 CGO_ENABLED=0，internal/server 测试需 WSL CGO_ENABLED=1）。
- 首跑全绿的断言方式（建议：以 BuildTriples 构造性保证为据做 hermetic fixture 断言，live 快照核对留 operator 域）。

### Deferred Ideas (OUT OF SCOPE)
- 级联隔离（object-keyed 抑制机制）。
- C1（entityCode minCount 1 @ warning 黄灯）。
- skf:layer shapes 标注（Area 3 Option C）。
- 图谱空心节点渲染修复。
- 入边谓词枚举 shapes 化。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SHACL-01 | 实体类 ABox SHACL shapes 文件随仓库交付，路径可配置（零硬编码） | entities.ttl 落点 `internal/ontology/shacl/`（与 shapes.ttl 同目录）；`SKILLFORGE_SHACL_SHAPES` 列表化经 `filepath.SplitList`（本地 go doc 语义已核）；默认列表 = 两文件；induction.go:383 硬编码修复点已定位；shapes.ttl:4 陈旧注释修正点已定位 |
| SHACL-02 | 启动校验覆盖实体 shapes；违规进现有 quarantine 流；boot 不被阻断（D-SHACL-02 保持） | 引擎 severity 分支插入点（validate.go:428-434）、stamping 循环（:118-135）、boot 链（main.go:930-951）、Quarantine subject 键控语义（store.go:193-225）、SC2 subject-位置不可见机制（QueryScoped 只查主索引 s.all，store.go:550-583）、隔离/boot 快照稳定循环已核（boot snapshot 写在 validate 之前，main.go:921 vs :931） |
| SHACL-03 | `/admin/governance` 报告可呈现实体类违规且与 TBox 层可区分；涉 UI 先 UI-SPEC（已批准） | Violation DTO 加字段点（validate.go:55-63）、handler 透传（admin_governance.go:45-63 直接 c.JSON(*shacl.Report)）、FE 落点（types.ts / ShaclReportView.tsx / GovernanceTab.tsx:83）、46-UI-SPEC 已 approved（5 PASS + 1 FLAG） |
</phase_requirements>

## Summary

本 phase 是 **Phase 40 自研 SHACL 引擎的自然延伸**，零新依赖、零新库：引擎已是 7-constraint switch 设计（D-SHACL-01 哲学），`sh:severity` = 一个 parse 分支 + 推导替换（~15-20 行单文件改动）；`parseShapes` 的 `parsedShapes.NodeShapes` 是纯 slice，多文件 merge = 逐文件 append；`Violation` DTO 加 `omitempty` 字段经 handler 透传直达 FE。全部落点行号已在工作树逐行核对。

研究核对了 **W3C SHACL 1.0 规范 §2.1.4 原文**（webReader 抓取）：`sh:severity` 三值（`sh:Info`/`sh:Warning`/`sh:Violation`），**`sh:Violation` 是未声明时的默认**——与 D-46-01 的「默认=error」完全一致；且规范明言 severity 「对校验本身无影响、供 UI 分类用」——SkillForge 的 Warning→放行（不隔离）是 D-SHACL-02 之上的文档化扩展，规范兼容。

两个 **研究新发现（46-CONTEXT 未覆盖，planner 必须处理）**：
1. **`TestShaclViolation`（shacl_test.go:696-753）是唯一依赖 sourcePath 硬编码的既有测试**——其 inline fixture 声明 `sh:path skf:sourcePath ; sh:minCount 1` 但**无** `sh:severity`，期望 WarningCount==1/health=warn。引擎退役 sourcePath 特例后该测试**必然翻红**。解法：fixture 补一行 `sh:severity sh:Warning`（镜像 retrofitted shapes.ttl），**断言本身零改动**——「既有断言一条不能动」红线按此口径解释才可满足（见 Pitfall 1）。
2. **live 数据的实体字面量全部不带引号**（快照实证：126 个实体 label、16 个 entityCode 值均为裸字符串）——引擎 `checkDatatype`/`checkNodeKind` 用「`"` 前缀」判字面量，裸值被归类为 IRI → **`sh:datatype xsd:string` 放在 entityCode 上会隔离全部 16 个带码实体**，直接击穿 D-46-01 的「首跑零隔离」。entities.ttl 的 if-present 检查只能安全使用 `sh:class`（rdf:type 反查，与引号表示无关）；entityCode 的 datatype 检查在本 phase 内**不可安全落地**（详见 Pitfall 2 + Open Question 1）。

数据实证（live snapshot `data/ontology/skillforge_kg.json`，989 triples）：71 InsuranceProduct + 56 Organization rdf:type triples = **126 个不同实体主体**（CONTEXT 写 127——存在 1 个双 type 实体；hermetic 断言不受影响，live 核对按 126/127 双口径知悉即可）；126/126 实体有 label（minCount 1 @ error 零隔离成立）；126/126 有 belongsToTier 且目标全部是 Tier 个体（`sh:class skf:Tier` if-present 安全，注意 KG 里存在 `cat_sql-guides` 这种非 Tier 的 belongsToTier 对象但**不在实体上**）；1 条 parentOrganization 边且目标确为 Organization（`sh:class skf:Organization` if-present 安全）。

**Primary recommendation:** 按 D-46-01..04 逐字落地；`Validate` 签名改 variadic（`...string`）保全部既有调用点零改动编译通过；entityCode 不加 datatype 检查（记录为对 CONTEXT 约束清单字面的有意偏离，理由=引擎字面量启发式 + 回归红线，需用户确认）；`TestShaclViolation` fixture 补显式 severity（断言不动）；layer 派生用 parse 期源文件追踪（entities.ttl→entity，其余→tbox，global 哨兵在 checkGlobalReferences）。

## Project Constraints (from CLAUDE.md)

- **零硬编码**：ontology IRI 命名空间、SHACL shapes 路径必须可配置（v1.4 Standards 原文）。
- **Go 最佳实践**：无 cgo；handler ≤ 200 行；不允许静默吞错。
- **三条护城河不可动摇**：Apache-2.0 单 binary + Skill 驱动 + Tier 信任隔离（PySHACL sidecar 在 REQUIREMENTS Out of Scope 明确否决）。
- **UI 一致性 hard constraint**：每 web phase 先产 UI-SPEC.md——本 phase 已满足（46-UI-SPEC approved 2026-10-04）。
- **全 SDLC**：每 phase 配 verify-phase-NN.sh。
- GSD Workflow Enforcement：本文件即 GSD research-phase 产物，符合入口要求。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| shapes 文件交付（entities.ttl）+ 路径配置 | API/Backend（repo 资产 + 引擎 parse） | — | shapes 是引擎消费的数据文件，非前端资产；env 读取在 boot/rollback 两个 Go 消费点 |
| 启动校验 + severity 分级 | API/Backend（shacl 引擎） | — | D-SHACL-01 纯 Go 引擎单点；W3C 语义映射在 parse 层生效 |
| quarantine 分流 + SC2 不可见性 | API/Backend（store + boot 装配） | — | Quarantine 移出主索引即天然不可见（Query/QueryScoped 均读 s.all）；前端零参与 |
| 违规分层（layer/target_class 盖戳） | API/Backend（validate.go DTO） | Frontend（徽章渲染） | D-46-03 锁定后端盖戳；FE 只渲染不推导（Honesty 红线 9） |
| dangling_edge_count | API/Backend（shacl 包反向 Query） | Frontend（只读列展示） | 计数是 store Query 的只读产物，FE 不计算（UI-SPEC Honesty 红线 8） |
| governance 报告分层呈现 | Frontend（ShaclReportView 等 3 文件） | API/Backend（DTO omitempty 透传） | handler 是 passthrough（admin_governance.go:62 直接 c.JSON），无字段重组 |
| rollback 重校验 parity | API/Backend（induction.go） | — | D-46-02 修复点；boot/rollback 读同一 env 列表 |

## Standard Stack

### Core（零新依赖——全部现有资产）
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `path/filepath` | go1.25.5 | `SplitList`（env 列表解析）/ `Join` / `ListSeparator` | 官方语义：空串返回**空 slice**（非 [""]）——已 `go doc` 本地核对；os 分隔符由 GOOS 决定（WSL Linux → `:`） |
| Go stdlib `errors` | go1.25.5 | `errors.Join` 聚合多文件 parse 错误（不静默吞缺失路径） | Go 1.20+ 标准做法，满足 D-46-02 聚合约束 |
| github.com/tggo/goRDFlib | 既有（go.mod 锁定） | turtle.Parse shapes 文件 | D-SHACL-01 既有引擎底座；前缀在 parse 期展开为全 IRI（live 校验通过为证） |
| internal/ontology/shacl | 本仓库 | 引擎 + shapes 资产目录 | entities.ttl 与 shapes.ttl 同目录同家族（D-46-02） |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| vitest | web 既有 | FE governanceApi/ShaclReportView 回归（若 planner 决定加组件测试） | FE 门禁照 39 模式 Windows 侧直接 node 调用（WSL 无 node） |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 引擎自研 severity（D-46-01） | PySHACL sidecar | REQUIREMENTS Out of Scope 明确否决（破坏单 binary 护城河） |
| `Validate(kg, paths ...string)` variadic | 新函数 `ValidateWithShapesList` + 保留旧签名 | variadic 让 13 处既有测试调用点**零改动**编译通过；新函数方案多一个 API 面且旧签名语义歧义（单 string 是否含列表？） |
| 独立 entities.ttl（D-46-02 b） | 单文件追加 / 第二 env | CONTEXT 已否决（全有全无逃生门 / 组合漂移） |

**Installation:** 无安装——零新 npm/Go 依赖。

**Version verification:**
- go1.25.5 linux/amd64（WSL Ubuntu-22.04）✓ 本会话 `go version` 实测。
- `filepath.SplitList` 语义 ✓ 本会话 `go doc filepath.SplitList` 实测："Unlike strings.Split, SplitList returns an empty slice when passed an empty string."
- shacl 包 `CGO_ENABLED=0 go test ./internal/ontology/shacl/...` ✓ 本会话实测 `ok ... 0.007s`（13 测试全绿基线）。
- node v24.19.0（Windows 侧）✓ 本会话实测（FE 门禁可用）。

## Package Legitimacy Audit

> 本 phase **零新外部包**。引擎扩展用 Go stdlib，shapes 是仓库内 TTL 文件，FE 改动是既有组件内编辑。无 registry 安装动作，无需 slopcheck 门禁。

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| （无新包） | — | — | — | — | — | — |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                        ┌──────────────────────────────────────────────┐
                        │ SKILLFORGE_SHACL_SHAPES env（路径列表 : 分隔） │
                        └──────────────┬───────────────────────────────┘
                                       │ 同一 env（D-46-02 parity）
                 ┌─────────────────────┴──────────────────────┐
                 ▼                                            ▼
   ┌─────────────────────────┐                  ┌──────────────────────────┐
   │ boot（main.go:930-951）  │                  │ rollback（induction.go:383│
   │ Restore 快照            │                  │  → 改读 env，修 no-op）    │
   │ → WriteSnapshot(:921)   │                  │ 只计数不隔离（D-PROV-09）  │
   │ → Validate(kg, paths…)  │                  └──────────────────────────┘
   │ → Quarantine(subject)   │
   │ → SetShaclReport        │
   └───────────┬─────────────┘
               ▼
   ┌─────────────────────────────────────────────────────────────┐
   │ shacl.Validate（引擎，纯 Go）                                 │
   │                                                             │
   │ parseShapes(path 列表)  ── 逐文件 parse + errors.Join 聚合    │
   │   ├─ shapes.ttl  → NodeShapes[8] ──▶ layer="tbox"           │
   │   └─ entities.ttl→ NodeShapes[2] ──▶ layer="entity"         │
   │                                                             │
   │ parsePropertyShape: +sh:severity 分支                        │
   │   sh:Warning/sh:Info → SeverityWarning（放行）               │
   │   未声明/其他      → SeverityError（默认, W3C §2.1.4）        │
   │   [退役] path==sourcePath→warning 硬编码 :428-434            │
   │                                                             │
   │ 校验循环 :118-135（stamping 点）                              │
   │   v.Entity = entity; v.TargetClass = ns.TargetClass;        │
   │   v.Layer = ns.Layer（tbox|entity）                          │
   │                                                             │
   │ checkGlobalReferences :139-146/:694-718                      │
   │   └─ 跨实体违规 → layer="global" 哨兵                        │
   │                                                             │
   │ [新] dangling_edge_count：对每个 error 实体                   │
   │   Σ len(kg.Query("", pred, entity))                         │
   │   pred ∈ {involvesInsuranceProduct,                         │
   │           involvesOrganization, parentOrganization}         │
   └───────────┬─────────────────────────────────────────────────┘
               ▼
   ┌─────────────────────────┐     ┌──────────────────────────────┐
   │ store.Quarantine（t.S）  │     │ GET /api/admin/governance/   │
   │ 实体 triples 移出主索引   │     │ shacl（handler 透传 c.JSON）   │
   │ → subject 位不可见(SC2)  │     └──────────────┬───────────────┘
   │ → 入边(对象位)仍可见      │                    ▼
   │ → 快照不回收(见 Pitfall 6)│     ┌──────────────────────────────┐
   └─────────────────────────┘     │ FE：types.ts(+layer/target_   │
                                   │ class/dangling_edge_count)   │
                                   │ ShaclReportView（徽章+区头计数 │
                                   │ +悬挂边列）GovernanceTab(:83) │
                                   └──────────────────────────────┘
```

### Recommended Project Structure
```
internal/ontology/shacl/
├── validate.go          # 引擎：+severity 分支、+layer/target_class 盖戳、+dangling 计数、parseShapes 列表化
├── shapes.ttl           # 既有 8 TBox NodeShapes（:192-197/:215-220 补 sh:severity；:4 注释修正）
├── entities.ttl         # [新建] InsuranceProductShape + OrganizationShape（D-46-02）
└── shacl_test.go        # 既有 13 测试 + 新增 severity/layer/列表化/dangling 测试
```

### Pattern 1: sh:severity 解析分支（D-46-01 核心改动）
**What:** parsePropertyShape 末尾的硬编码推导替换为解析值或默认 error。
**When to use:** 唯一改动点；severity 已从 propertyConstraint 流入全部 check* 函数，解析后自动生效。
**Example:**
```go
// Source: internal/ontology/shacl/validate.go:428-434（替换体）+ W3C SHACL §2.1.4
// 解析 sh:severity（PropertyShape 级；NodeShape 级引擎不读——见 Pitfall 3）
for _, tr := range subs {
    if tr.P == sh("severity") {
        sev := expandPrefixed(tr.O)
        if sev == sh("Warning") || sev == sh("Info") {
            c.Severity = SeverityWarning
        } else {
            c.Severity = SeverityError // sh:Violation 与未知 IRI 一律 error
        }
        break
    }
}
// W3C: "sh:Violation is the default if sh:severity is unspecified"
if c.Severity == "" {
    c.Severity = SeverityError
}
```
（注意初始化 `c := propertyConstraint{}` 时 Severity 为零值 ""，先解析后判空即可；D-PROV-10 的 `if c.Path == pf("sourcePath")` 分支整段删除，shapes.ttl 两个 sourcePath PropertyShape 显式补 `sh:severity sh:Warning`。）

### Pattern 2: Validate 签名 variadic 化（列表化零迁移成本）
**What:** `func Validate(kg *store.Store, shapesPaths ...string)`；内部逐文件 parseShapes、errors.Join 聚合、`Report.ShapesPath = strings.Join(shapesPaths, string(filepath.ListSeparator))`。
**When to use:** D-46-02 的最小侵入路径——13 处既有测试调用 `Validate(kg, tmpPath)` 不改一行照常编译。
**Example:**
```go
// Source: go doc filepath.SplitList（本地核对）+ D-46-02 不静默吞错约束
func ShapesPathsFromEnv() []string {
    if v := os.Getenv("SKILLFORGE_SHACL_SHAPES"); v != "" {
        return filepath.SplitList(v)
    }
    return []string{ // 默认列表 = D-46-02
        filepath.Join("internal", "ontology", "shacl", "shapes.ttl"),
        filepath.Join("internal", "ontology", "shacl", "entities.ttl"),
    }
}
// boot:   shacl.Validate(ontologyKG, shacl.ShapesPathsFromEnv()...)
// rollback（induction.go:383 替换硬编码）:
//   shacl.ValidateWithTrigger(m.kg, shacl.ShapesPathsFromEnv()..., "rollback")
```
默认列表 helper 放 shacl 包单点（boot/rollback 两消费点共用，杜绝组合漂移）；确切形态属 CONTEXT Discretion。

### Pattern 3: layer/target_class 盖戳（D-46-03）
**What:** 校验循环 stamping 点（`v.Entity = entity` 处，validate.go:126）追加两个字段；nodeShape 结构体在 parse 期携带来源。
**When to use:** layer 派生推荐「源文件追踪」——parseShapes 列表化后逐文件 append 时给 nodeShape 记 `Layer`（basename `entities.ttl` → "entity"，其余 → "tbox"）。零类名硬编码；新增实体类只改 TTL 不改 Go。checkGlobalReferences（:694-718）的 violation 构造点直接写 `Layer: "global"` 哨兵。
**Example:**
```go
// Source: internal/ontology/shacl/validate.go:122-132（扩展现有循环）
for _, v := range violations {
    v.Entity = entity
    v.TargetClass = nsRec.TargetClass // 循环内现成可用（omitempty）
    v.Layer = nsRec.Layer             // "tbox" | "entity"（parse 期定）
    ...
}
// global 哨兵：checkGlobalReferences 内每个 Violation 构造加 Layer: LayerGlobal
```
备选锚点（TargetClass 集合判定）被否决理由同 D-46-03 FE-only：维护一份类名集合 = OntologyGraphView.tsx:21-25 同款硬编码债。layer 枚举字符串 `tbox`/`entity`/`global` 已被 46-UI-SPEC 锁定（`ViolationLayer` type）。

### Pattern 4: dangling_edge_count（D-46-04）
**What:** Validate 构建报告时（隔离发生**前**，kg 全量可查），对每个 error 实体反向计数入边。
**When to use:** 落点推荐 `shacl.QuarantinedEntity.DanglingEdgeCount int` + `json:"dangling_edge_count"`（UI-SPEC §Component 1 的 JSON 契约按 per-entity 字段锁定）。谓词用 Go 常量（pf(...) 三值）；shapes 化派生属 deferred。
**Example:**
```go
// Source: internal/knowledge/ecl/abox.go:68-82（谓词 IRI 家族）+ store.Query byO 索引
var danglingEdgePredicates = []string{
    pf("involvesInsuranceProduct"), pf("involvesOrganization"), pf("parentOrganization"),
}
func countDanglingEdges(kg *store.Store, entity string) int {
    n := 0
    for _, pred := range danglingEdgePredicates {
        n += len(kg.Query("", pred, entity)) // byO 索引反查
    }
    return n
}
```

### Anti-Patterns to Avoid
- **FE 前缀启发式分层**：按 entity IRI 前缀/类名前端猜 layer——D-46-03 明确否决（OntologyGraphView.tsx:21-25 反例）；FE 只渲染 BE 戳（UI-SPEC Honesty 红线 9）。
- **级联隔离**：`Quarantine` subject 键控 + snapshot 只序列化 s.all，级联会误伤健康 note 并永久化删除——D-46-04 否决记录在案。
- **severity 写在 NodeShape 上**：引擎只在 PropertyShape 粒度读 severity（见 Pitfall 3）——entities.ttl 每个约束行都要显式声明。
- **在 quarantine 之后触发任何快照写**：会把删除永久化（Pitfall 6）——本 phase 不新增任何 post-quarantine onChange/快照触发。
- **静默跳过缺失 shapes 路径**：D-46-02 明令禁止；错误聚合后整体返回，由 main.go:934 既有 WARN 路径呈现。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| env 路径列表解析 | 手写 `strings.Split(v, ";")` / 自定义分隔约定 | `filepath.SplitList` | os 分隔符由 GOOS 决定；空串语义（空 slice）已被 stdlib 定义并文档化；运行时 WSL Linux `:` 无歧义 |
| 多文件错误聚合 | 自定义 error 字符串拼接 | `errors.Join`（Go 1.20+） | 标准 unwrap/Is 兼容；满足「不静默吞错」且有单一规范形态 |
| SHACL severity/约束语义 | 自造分级词汇或新约束类型 | W3C SHACL §2.1.4 三值 + 既有 7-constraint switch | 规范默认（未声明=Violation）与 D-46-01 逐字吻合；新约束类型=新 case 分支（引擎 docstring 既有承诺），本 phase 不需要 |
| SHACL 校验本身 | PySHACL / 第三方 Go SHACL 库 | 既有自研引擎 | REQUIREMENTS Out of Scope（单 binary 护城河）；D-SHACL-01 锁定 |

**Key insight:** 本 phase 的每一个机制扩展都设计为「一个 parse 分支 / 一个 append / 一个字段」量级——引擎在 Phase 40 就把扩展点留好了（severity 流经全部 check*、parsedShapes 全 slice、handler passthrough）。任何超出此量级的方案都应先质疑是否偏离了 D-46-01..04。

## Common Pitfalls

### Pitfall 1: TestShaclViolation 与 sourcePath 退役的回归冲突（研究新发现，必处理）
**What goes wrong:** shacl_test.go:696-753 的 fixture 声明 `sh:path skf:sourcePath ; sh:minCount 1`（无 sh:severity），期望 `WarningCount==1 / health=warn / 0 quarantined`。引擎退役 `path==sourcePath→warning` 特例后 severity 默认 error → note_001 被隔离 → 测试翻红。
**Why it happens:** CONTEXT 红线写「Phase 40 shacl_test.go 全部既有断言一条不能动」，但该测试的**前提**（sourcePath 自动 warning）正是 D-46-01 要退役的机制。逐测试核对结论：13 个既有测试中**只有这一个**依赖硬编码（TestErrorWarningSeparation 的 sourcePath 值合法不触发违规、TestReportStructure 期望 0 warnings，均不受影响）。
**How to avoid:** fixture 补一行 `sh:severity sh:Warning ;`（与 retrofitted shapes.ttl 镜像），**断言零改动**——测试继续验证「warning 放行不隔离」同一语义，只是政策声明从引擎硬编码迁到 shapes 数据（这正是 D-46-01 的目的）。planner 须在 plan 中显式记录此红线口径调整。
**Warning signs:** 改完引擎跑 `go test ./internal/ontology/shacl/...` 出现 `TestShaclViolation` FAIL、`expected health=warn, got error`。

### Pitfall 2: 裸字面量 × sh:datatype/sh:nodeKind = 大规模误隔离（研究新发现，数据实证）
**What goes wrong:** live 快照实证：实体 label（126/126）与 entityCode（16/16）的值**不带 `"` 引号**（BuildTriples abox.go:211/:214 直接 `O: e.Name / O: e.Code`，不经 turtle parse）。引擎 `checkDatatype`（validate.go:545-554）与 `checkNodeKind`（:600-602）以 `"` 前缀判字面量——裸值被归类 IRI → `sh:datatype xsd:string` 在 entityCode 上会隔离全部 16 个带码实体，击穿「首跑零隔离」。
**Why it happens:** store 的字面量表示不统一（LoadFromTurtle 渲染 `"value"`，BuildTriples 裸写）；该表示差异是 Phase 39/40 既有现实。修复引擎启发式会打翻 TestDatatypeViolation 的 pinned 断言（:377 `not-a-literal-iri` 无引号无冒号 → 期望违规），违反红线。
**How to avoid:** entities.ttl 的 if-present 检查**只用 `sh:class`**（rdf:type 反查，与字面量表示无关——belongsToTier→skf:Tier、parentOrganization→skf:Organization 均安全，live 实证 0 违规）。entityCode 不加 datatype/nodeKind 检查（`sh:pattern` 对裸值是静默 no-op，:638-642——装了也不响，不要用）。label 只加 minCount 1（CONTEXT 本就如此指定）。详见 Open Question 1。
**Warning signs:** 首跑报告 error_count=16、隔离区全是带 entityCode 的实体。

### Pitfall 3: sh:severity 写在 NodeShape 上被静默忽略
**What goes wrong:** W3C 允许 shape 级（Node/Property）severity；但引擎的 `parsePropertyShape` 只扫 PropertyShape 的 triples，`nodeShape` 结构体无 Severity 字段——NodeShape 级 `sh:severity` 不会生效也无报错。
**Why it happens:** 引擎校验粒度 = PropertyShape（W3C 语义下 property 约束结果的 source shape 就是 PropertyShape，合规但收窄）。
**How to avoid:** entities.ttl 与 shapes.ttl retrofit 全部把 `sh:severity` 写在每个 `sh:PropertyShape` 内。顺带：不要用 `sh:deactivated`（§2.1.6）——引擎同样不实现，静默无效。
**Warning signs:** 显式声明 warning 的约束仍把实体送进隔离区。

### Pitfall 4: SKILLFORGE_SHACL_SHAPES 空串 → SplitList 返回空 slice
**What goes wrong:** `filepath.SplitList("")` 返回**空 slice**（非 `[""]`，go doc 实测）——若空串被当列表处理，校验静默零 shapes 通过（比解析失败更糟）。
**Why it happens:** stdlib 刻意区分「空列表」与「单空元素」。
**How to avoid:** env 读取 helper 先判空回退默认列表（Pattern 2 的 `if v != ""` 分支）；补一个单测钉住空串/未设置/单路径/多路径四态。
**Warning signs:** 设置空 env 后 boot 日志 SHACL 行消失（校验被跳过走 main.go:934 WARN 路径）。

### Pitfall 5: 缺失路径的错误语义 —— 隔离全跳过而非部分跳过
**What goes wrong:** 列表中任一路径 parse 失败时若「跳过坏文件继续校验」，会产生半套 shapes 的部分校验结果并进 quarantine——比整体跳过更危险。
**Why it happens:** 单文件时代 parse 失败 → Validate 返回 error → main.go:933-934 WARN + **整个校验（含 quarantine 分流）跳过**——这是 D-SHACL-02「violations never block boot」的既有语义。
**How to avoid:** 多文件沿用同语义：任一文件失败 → errors.Join 整体返回 error → 整轮校验跳过（WARN 里点名缺失路径）。rollback 侧沿用 :385-389 warn-and-return-0 既有姿态（该路径修复后应加回归测试钉住「真的读到 env」——当前 no-op 无覆盖，CONTEXT Specifics 已要求）。
**Warning signs:** 错删一个路径后报告照常生成但实体 shapes 全部失效。

### Pitfall 6: quarantine 后的快照时序 —— 稳定循环 vs 永久化一线之隔
**What goes wrong:** 两个事实叠加：(a) boot 快照写在 validate **之前**（main.go:921 vs :931）→ 隔离实体的 triples 仍留在快照 → 下次 boot 重校验重新隔离 → 报告每 boot 稳定呈现（这是对的）；(b) 但 debounced snapshot（store.go:370-374 只序列化 `s.all`）若在 quarantine 之后被任何 onChange 触发，删除即被持久化 → 重启后实体彻底消失、报告「自愈」回 ok。
**Why it happens:** Quarantine 移出主索引但不进快照；debounce 只由 induction/ECL 变更触发。
**How to avoid:** 本 phase 零新增 post-quarantine 触发器；SC2 测试按「稳定循环」断言（隔离后立即再 Validate 仍报同违规 + Query 查不到该 subject）。隔离区 hint 文案（UI-SPEC 已锁）如实描述「待下次启动重新校验」。
**Warning signs:** 重启一次后隔离区清空但实体在图谱/查询里也没了。

### Pitfall 7: 127 vs 126 —— 双 type 实体
**What goes wrong:** live 快照有 71+56=127 条实体 rdf:type triples 但只有 **126 个不同实体主体**（1 个实体同时是 InsuranceProduct 和 Organization——EntityIRI 按名哈希，跨 kind 同名即碰撞）。逐类 NodeShape 评估时该实体出现在两个 target 集合里，violation 可能双计。
**Why it happens:** D-NZK-01 哈希 IRI 不含 kind 前缀。
**How to avoid:** hermetic 断言按构造性 fixture（BuildTriples 产物）写，不写 live 计数断言（CONTEXT Discretion 已建议此路线）；live 核对留 operator 域、按双口径知悉。
**Warning signs:** 实体级统计对不上 127 口径时的困惑。

### Pitfall 8: cat_sql-guides —— belongsToTier 目标不全是 Tier 个体
**What goes wrong:** live KG 的 belongsToTier 对象里存在 `skf:cat_sql-guides`（类型是 KBCategory 不是 Tier）。实证**当前零实体**指向它（都在 note/category 上），entities.ttl 的 `sh:class skf:Tier` if-present 安全；但 fixture 设计若粗放复刻 live 形态可能误报。
**Why it happens:** 260928-wzl 修的是 resolver 归一化（查询侧），KG 存量边未清洗。
**How to avoid:** fixture 用 BuildTriples 构造（天然只发 Tier 目标）；不要从快照导 fixture。另注意：shapes.ttl:26-29 的全局 belongsToTierShape 目前**解析但从不评估**（parsed.GlobalBelongsTo 无消费点，validate.go 只有 GlobalRefs 进 checkGlobalReferences）——不要误以为它已在线生效。
**Warning signs:** 实体 shapes 上线后出现来源不明的 belongsToTier 违规。

### Pitfall 9: FE 咽喉点与 DTO 纪律
**What goes wrong:** 新 FE 字段若引入新数组/嵌套对象，绕过 governanceApi.ts:57-60 null-guard 会复刻 260928-jxb 治理白屏。
**Why it happens:** Go nil slice 序列化为 null。
**How to avoid:** 本 phase 新字段全部是标量 + omitempty（46-UI-SPEC 已锁定零新数组、咽喉点不动）；`dangling_edge_count` 是 int（BE 恒下发），`layer`/`target_class` optional。QuarantinedEntity.Violations 既有字段不动。
**Warning signs:** PR 里出现 governanceApi.ts 改动即偏离 UI-SPEC（唯一例外：planner 引入新数组字段才扩 normalize，属例外路径）。

### Pitfall 10: shapes.ttl:4 陈旧注释与 IRIPropertyShape 冷知识
**What goes wrong:** (a) shapes.ttl:4 写「default data/ontology/shacl.ttl」，实际默认是 `internal/ontology/shacl/shapes.ttl`（main.go:930）——D-46-02 已列修正；(b) shapes.ttl 第 9 个 NodeShape `skf:IRIPropertyShape`（:227-232）**无 sh:targetClass**——引擎 `collectIndividuals(kg, "")` 返回全部带 rdf:type 主体，空 TargetClass = 「所有 typed 实体」的隐性全域 shape（当前约束无害但列表化/计数测试须知悉它存在，勿把「8 NodeShapes」当文件全集断言）。
**How to avoid:** 新测试若断言 shapes.ttl 的 NodeShape 数量，按 9 计（或断言 TargetClass 集合）；头注释修正时顺带更新 D-SHACL-04 描述。
**Warning signs:** 「expected 8 NodeShapes, got 9」类断言自摆乌龙。

## Code Examples

### W3C severity 语义（权威锚点）
```turtle
# Source: W3C SHACL Recommendation §2.1.4 (https://www.w3.org/TR/shacl/#severity)
# "For every shape, sh:Violation is the default if sh:severity is unspecified."
# 三值: sh:Info | sh:Warning | sh:Violation（任意 IRI 也可，引擎按未知→error 处理）
# 语法规则 severity-maxCount（至多一个值）/ severity-nodeKind（必须 IRI）
skf:EntityCodeShape a sh:PropertyShape ;
    sh:path skf:entityCode ;
    sh:nodeKind sh:IRI .   # ← 反例！裸字面量会被判 IRI 违规（Pitfall 2），不要这么写
```

### entities.ttl 骨架（按 D-46-01 约束清单 + Pitfall 2 修正）
```turtle
# Source: internal/ontology/shacl/shapes.ttl 风格 + abox.go:207-236 发射契约
@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix rdf:  <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

skf:InsuranceProductShape a sh:NodeShape ;
    sh:targetClass skf:InsuranceProduct ;
    sh:property skf:InsuranceProduct_type , skf:InsuranceProduct_label ,
                skf:InsuranceProduct_code , skf:InsuranceProduct_tier .

skf:InsuranceProduct_type a sh:PropertyShape ;
    sh:path rdf:type ;
    sh:hasValue skf:InsuranceProduct ;
    sh:message "skf:InsuranceProduct must have rdf:type skf:InsuranceProduct"@en .

# 提取器无条件发出（abox.go:210-211）→ 必填 @error；live 126/126 满足
skf:InsuranceProduct_label a sh:PropertyShape ;
    sh:path rdfs:label ;
    sh:minCount 1 ;
    sh:message "skf:InsuranceProduct must have at least one rdfs:label"@en .

# if-present（abox.go:213-215 条件发出，live 缺失 88%+）→ 无 minCount、无 datatype（Pitfall 2）
skf:InsuranceProduct_code a sh:PropertyShape ;
    sh:path skf:entityCode ;
    sh:message "skf:entityCode if present is carried verbatim from the corpus"@en .

# if-present class 检查；全局 belongsToTierShape 从不被评估（Pitfall 8），此处是实体首次真实生效点
skf:InsuranceProduct_tier a sh:PropertyShape ;
    sh:path skf:belongsToTier ;
    sh:class skf:Tier ;
    sh:message "skf:belongsToTier value must be a skf:Tier instance"@en .

# OrganizationShape 同构（+ parentOrganization sh:class skf:Organization if-present）
```

### rollback parity 修复（D-46-02）
```go
// Source: internal/ontology/induction/induction.go:382-390（现状）—— 硬编码路径
// filepath.Join(m.dataDir, "..", "shacl", "shapes.ttl")
//   = data/shacl/shapes.ttl（目录不存在）→ os.ReadFile error → :386-388 warn-and-return-0 no-op
// 修复：改读与 boot 同一 env 列表（helper 见 Pattern 2），回归测试断言
// 「rollback 触发后 report.ErrorCount 反映 env 列表 shapes」——当前 no-op 无覆盖，修完补测
```

### FE 徽章渲染（UI-SPEC §Component 2 落点）
```tsx
// Source: web/src/components/admin/governance/ShaclReportView.tsx:123-128（约束 cell 扩展点）
// 渲染顺序：layer 徽章 → target_class chip → constraint mono → +{extra}
// layer 缺省（legacy）→ 不渲染徽章；global 哨兵 → 「跨实体」dashed 徽章 + 无 chip
// 色板（UI-SPEC §Color）：tbox=bg-primary-dim text-primary · entity=bg-surface-3 text-text-muted
//                        global=border border-dashed border-border text-text-muted
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| sourcePath→warning 引擎硬编码（D-PROV-10 特例） | shapes 文件显式 `sh:severity sh:Warning` + 引擎默认 error | 本 phase（D-46-01） | 政策归位数据；一次性特例退役；唯一受影响测试 TestShaclViolation fixture 补一行（Pitfall 1） |
| 单 shapes 路径（SKILLFORGE_SHACL_SHAPES 单值） | 路径列表（SplitList）+ 默认双文件 | 本 phase（D-46-02） | TBox/ABox 生命周期分离；operator 可独立禁用实体校验 |
| Violation 无层级信息 | layer/target_class 盖戳 + global 哨兵 | 本 phase（D-46-03） | 报告可区分实体/TBox/跨实体三层；FE 类型向后兼容（omitempty + optional） |

**Deprecated/outdated:**
- `induction.go:383` 的 `data/shacl/shapes.ttl` 推导路径：从未生效（目录不存在），本 phase 以 env 列表取代。
- shapes.ttl:4 头注释的默认路径描述：与 main.go:930 不符，随本 phase 修正。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | layer 派生采用 parse 期源文件追踪（entities.ttl→entity，其余→tbox）而非 TargetClass 集合判定 | Architecture Pattern 3 | operator 以 env 自定义文件名时会盖 tbox 戳——降级可接受但需知悉；CONTEXT 将「layer 枚举确切字符串」留 Discretion，派生机制本身未被锁定 |
| A2 | entityCode 不加 datatype/nodeKind if-present 检查（对 D-46-01 约束清单「datatype/class」字面的有意收窄） | Pitfall 2 / Open Question 1 | 若用户坚持要 datatype 检查，须先改 BuildTriples 引号表示（波及 FE/快照/SPARQL 展示）或改引擎启发式（打翻 pinned 测试）——二者均超本 phase 量级 |
| A3 | `Validate` 改 variadic 签名是列表化的首选形态（CONTEXT Discretion 允许形态自选） | Standard Stack / Pattern 2 | 若 planner 选新函数方案，测试调用点迁移成本略增，语义等价 |
| A4 | dangling 计数在 Validate 期（隔离前）计算与 UI-SPEC「指向该隔离实体的入边数量」语义一致 | Pattern 4 | 若误在隔离后计算，subject 边已被移走但入边仍在——数值恰好相同（入边主体是健康 note），风险极低 |
| A5 | 46-CONTEXT 的「127 实体」与快照实测 126 主体（1 双 type）的差异不影响任何断言（hermetic 路线） | Summary / Pitfall 7 | 若 planner 写了 live 计数断言会困惑；hermetic 路线下无影响 |

## Open Questions

1. **entityCode 的 if-present 检查形态（需用户确认）**
   - What we know: CONTEXT 写「datatype/class if-present」，但 live 数据字面量裸表示 + 引擎 `"` 前缀启发式 + TestDatatypeViolation pinned 断言三者夹击，datatype 检查必然误隔离 16 个带码实体（Pitfall 2）。
   - What's unclear: 用户是否接受本 phase 的 entityCode「零约束」（仅 message 说明）或 prefers 改发射表示。
   - Recommendation: 本 phase entityCode 零约束 + decisions 记录偏离理由；发射表示修复留 backlog。
2. **layer 派生锚点（planner 可决策）**
   - What we know: 三候选（源文件追踪 / TargetClass 集合 / shapes 自标注）——末者被 CONTEXT deferred 明确排除。
   - What's unclear: 无——仅形式确认。
   - Recommendation: 源文件追踪（Pattern 3），一个常量 + 零类名维护。
3. **ShapesPath join 分隔符（cosmetic）**
   - What we know: UI-SPEC 要求「含 os 分隔符」；WSL Linux `:`。
   - Recommendation: `string(filepath.ListSeparator)`（与 SplitList 对偶）。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| WSL Ubuntu-22.04 + Go | 全部 Go 构建/测试门禁（CGO_ENABLED=1 生产路径） | ✓ | go1.25.5 linux/amd64（实测） | — |
| CGO_ENABLED=0 go test | shacl 包快速迭代（纯 Go） | ✓ | 实测 ok 0.007s | — |
| node（Windows 侧） | FE vitest 门禁（WSL 无 node，照 39 模式） | ✓ | v24.19.0（实测） | FE 改动小，可由 checker 目检 + 现有 83 测试文件回归 |
| goRDFlib | shapes turtle 解析 | ✓（go.mod 既有） | — | — |
| live LLM/Milvus/BGE | 不需要（全套件 hermetic，env-intranet-service-flakiness） | 不适用 | — | — |

**Missing dependencies with no fallback:** none
**Missing dependencies with fallback:** none

## Runtime State Inventory

> 本 phase 非 rename/refactor/migration phase——不适用完整清单。相关运行态事实已并入 Pitfall 6（quarantine×snapshot 时序）与 Pitfall 8（KG 存量 cat_sql-guides 边）：均无需迁移动作，仅需测试设计知悉。

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing（stdlib，无 testify）go1.25.5；FE vitest（web/） |
| Config file | 无新增（shacl 包既有；web/vitest.config.ts 既有） |
| Quick run command | `wsl.exe -d Ubuntu-22.04 -- bash -c "cd /mnt/d/CODE/ClaudeCode/ai-agent && CGO_ENABLED=0 /usr/local/go/bin/go test ./internal/ontology/shacl/... -count=1"`（本会话实测可用） |
| Full suite command | `bash verify-phase-46.sh`（照 45 模式：P1 CGO=1 build+vet → P2 ontology 套件 → P3 targeted anti-vacuous `-run` + 最低 `--- PASS` 计数 → P4 `./internal/...` 全量） |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SHACL-01 | env 列表解析（未设置/空串/单/多路径四态）+ 默认双文件 + errors.Join 不吞缺失路径 | unit（shacl 包，纯 Go） | `go test ./internal/ontology/shacl/... -run TestShapesPaths -count=1` | ❌ Wave 0 |
| SHACL-01 | entities.ttl parse：2 NodeShapes、约束逐条断言（type hasValue / label minCount / if-present 无 minCount） | unit | `go test ./internal/ontology/shacl/... -run TestEntitiesShapesParse -count=1` | ❌ Wave 0 |
| SHACL-01 | rollback 重校验真读 env（induction.go:383 修复回归；当前 no-op 零覆盖） | unit（induction 包） | `go test ./internal/ontology/induction/... -run TestRollbackShaclParity -count=1` | ❌ Wave 0 |
| SHACL-02 | severity 三态：sh:Warning→warning 放行、sh:Info→warning、未声明/sh:Violation→error 隔离 | unit | `go test ./internal/ontology/shacl/... -run TestSeverity -count=1` | ❌ Wave 0 |
| SHACL-02 | 注入缺 label 的 Product fixture → 进 Quarantined + `Query(entity,…)` 为空（SC2 subject 不可见）+ 再 Validate 报告稳定（boot 稳定循环） | unit（fixture KG，hermetic） | `go test ./internal/ontology/shacl/... -run TestEntityQuarantine -count=1` | ❌ Wave 0 |
| SHACL-02 | 携违规 boot 存活（D-SHACL-02）——由 P4 全量套件 + 既有 main 链路回归覆盖；无需 boot 级集成测试（Discretion 授权 fixture 路线） | unit 间接 | P4 | ✅ 既有 |
| SHACL-03 | layer/target_class 盖戳：实体违规 layer=entity、TBox 违规 layer=tbox、global refs=global 哨兵 | unit | `go test ./internal/ontology/shacl/... -run TestViolationLayer -count=1` | ❌ Wave 0 |
| SHACL-03 | dangling_edge_count：注入 involves/parent 入边 fixture 计数正确；0 边渲染 0 | unit | `go test ./internal/ontology/shacl/... -run TestDanglingEdges -count=1` | ❌ Wave 0 |
| SHACL-03 | FE：types optional 字段渲染（legacy 无徽章 / global 无 chip / 0 如实渲染）——governanceApi 咽喉点回归不动 | FE unit（vitest） | `cd web && npx vitest run src/components/admin/governance`（Windows 侧） | ❌ Wave 0（可选：planner 权衡组件测试 vs 目检 checklist） |
| 回归红线 | 既有 13 shacl 测试全绿（TestShaclViolation fixture 补 severity 后断言不变，Pitfall 1） | unit 回归 | P2/P4 | ✅ 既有（1 处 fixture 单行补丁） |

### Sampling Rate
- **Per task commit:** shacl 包 quick run（<1s，CGO_ENABLED=0）
- **Per wave merge:** `CGO_ENABLED=1 go test ./internal/ontology/... ./internal/server/... -count=1`
- **Phase gate:** `bash verify-phase-46.sh` 全绿（P1-P4）→ `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/ontology/shacl/shacl_test.go` 内新增 severity/layer/列表化/entities.ttl/dangling 测试组（或独立 `entities_test.go`，照包内单文件惯例）
- [ ] `internal/ontology/induction` rollback SHACL parity 回归测试（新建）
- [ ] TestShaclViolation fixture 单行补丁（`sh:severity sh:Warning`）——断言不动
- [ ] FE 组件测试（可选）——若加，照 governanceApi.test.ts 的 vitest 惯例
- [ ] 框架安装：无需（Go testing + vitest 既有）

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | 本 phase 无新端点/身份面（governance 路由既有 X-Admin-Token 门禁不动） |
| V3 Session Management | no | — |
| V4 Access Control | no（继承） | 既有 requireOntology/requireAdmin 前置不变；SC2 的 subject 不可见性本身是数据访问控制的一部分（D-TIER-03 不回退） |
| V5 Input Validation | **yes（本 phase 主题）** | SHACL shapes 即声明式校验层；引擎零第三方（D-SHACL-01）；shapes 文件随 repo 版本化，env 只控路径不控内容语义 |
| V6 Cryptography | no | — |

### Known Threat Patterns for {Go self-hosted KG + admin console}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| shapes 路径注入（env 指向任意文件） | Tampering | os.ReadFile 失败即整体 error（不静默）；路径仅 operator env 可控，非请求输入 |
| 报告注入（violation detail 含实体 IRI/文本进 FE） | XSS | React 默认转义；FE 渲染 detail/constraint 为文本节点（既有 idiom），无 dangerouslySetInnerHTML |
| 隔离绕过（通过对象位悬挂边投毒查询） | Elevation | D-46-04 明示对象位可见是有意披露（带计数）；subject 写路径全走 QueryScoped（D-TRUST-01 MCP 同源） |

## Sources

### Primary (HIGH confidence)
- **W3C SHACL 1.0 Recommendation** §2.1.4（severity 三值、`sh:Violation` 默认、severity 不影响校验本身）、§4.2（minCount/maxCount 语义）、§2.1.6（sh:deactivated）——本会话 webReader 全文抓取 https://www.w3.org/TR/shacl/
- **工作树代码逐行核对**（HEAD dede743）：
  - `internal/ontology/shacl/validate.go`（引擎全文：:55-83 DTO、:118-135 stamping、:218-331 parseShapes、:333-437 parsePropertyShape、:428-434 sourcePath 硬编码、:440-451 collectIndividuals、:543-596 checkDatatype、:599-631 checkNodeKind、:634-656 checkPattern、:694-718 checkGlobalReferences、:727-750 ns helpers）
  - `internal/ontology/shacl/shapes.ttl`（:4 陈旧注释、:26-29 全局 belongsToTierShape、:192-197/:215-220 sourcePath shapes、:227-232 IRIPropertyShape）
  - `internal/ontology/shacl/shacl_test.go`（13 测试逐个核对 severity 依赖：仅 TestShaclViolation 依赖 sourcePath 特例）
  - `cmd/server/main.go:896-960`（boot 链：:921 WriteSnapshot 先于 :931 Validate、:930 env、:937-941 Quarantine、:943 SetShaclReport）
  - `internal/ontology/induction/induction.go:382-390`（rollback 硬编码 + warn-and-return-0）
  - `internal/ontology/store/store.go`（:193-225 Quarantine、:230-249 QuarantinedEntities、:251-262 report cache、:273-306 Query、:366-374 Snapshot 只序列化 s.all、:487-584 QueryScoped、:550-583 subject 可见性）
  - `internal/knowledge/ecl/abox.go`（:165-239 BuildTriples、:210-236 实体发射条件、:300-377 ApplyNote 恢复路径 + 二次父解析）
  - `internal/knowledge/ecl/stub.go:99-110`（legalEntityKinds 枚举）
  - `internal/server/admin_governance.go:40-63`（handler passthrough）、`admin_platform.go:189-252`（SparqlQueryScoped）
  - `web/src/components/admin/governance/{types.ts, governanceApi.ts, ShaclReportView.tsx, GovernanceTab.tsx:83}`（FE 落点）
- **live 数据实证**：`data/ontology/skillforge_kg.json`（989 triples；71+56 实体 type triples / 126 主体；16 个裸 entityCode；126/126 裸 label；belongsToTier 目标含非 Tier 的 cat_sql-guides 但实体零命中；1 条 parent 边目标合法）
- **本地工具链**：`go doc filepath.SplitList`、`go version`（WSL go1.25.5）、`CGO_ENABLED=0 go test ./internal/ontology/shacl/...`（13 测试 ok 0.007s）——全部本会话实测

### Secondary (MEDIUM confidence)
- 无（本轮所有关键断言均有一级来源）

### Tertiary (LOW confidence)
- 无

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 零新依赖，全部既有资产本会话实测（编译/测试/工具链）
- Architecture: HIGH — 全部落点行号逐行核对；W3C 规范原文核对 severity 语义
- Pitfalls: HIGH — Pitfall 1/2 为本会话数据与测试逐条实证的新发现，非训练知识推断

**Research date:** 2026-10-04
**Valid until:** 2026-11-03（30 天；引擎/装配面稳定，唯一变量是 Phase 46 自身实现推进）
