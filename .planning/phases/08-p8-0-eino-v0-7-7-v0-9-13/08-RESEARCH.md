# Phase 8: 全量依赖升级（P8.0 — eino v0.7.7 → v0.9.14）- Research

**Researched:** 2026-08-17
**Domain:** Go 依赖升级（eino / eino-ext）+ sqlite CheckPointStore 骨架 + 回归门重新锁定
**Confidence:** HIGH（编译面已用真实源码 + 可编译 spike 实证；运行时行为面 MEDIUM，靠 phase 内回归套件覆盖）

## Summary

本 phase 的最大不确定性（RF-2：升级后 `CheckPointStore` 接口精确形状）**已在研究阶段直接解决**。我把 eino v0.9.14 与 eino-ext openai v0.1.13 下载进 Go module cache 逐文件核对，并在仓外临时模块里跑通了「SkillForge 现有全部 eino API 形状 + 新 checkpoint 接线形状」的编译验证（`go build` + `go vet` 零错误）。结论：(a) v0.9.14 的 typed-generics 重构（`TypedRunner`/`TypedAgentEvent`/`TypedChatModelAgentConfig`）全部通过 **type alias** 保留旧名，我们现有的 `react.AgentConfig{ToolCallingModel, ToolsConfig, MaxStep}`、`flowagent.WithComposeOptions`、`compose.WithCallbacks/WithChatModelOption`、`adk.NewChatModelAgent/NewRunner/Query`、`drainFinalContent` 的事件形状（`ev.Err` / `ev.Output.MessageOutput.GetMessage()`）、`openai.NewChatModel` 全部原样编译——**升级对现有代码的强制源码改动为零**（唯一例外是 `integration/contract` 里按设计断言版本号的 `TestContractVersionPin`，它必须随升级改写）。(b) `CheckPointStore` 接口就是两个方法（`Get(ctx, id) ([]byte, bool, error)` / `Set(ctx, id, []byte) error`），且 **eino v0.9.14 核心模块不附带任何 store 实现（连内存版都没有**——文档示例的 `store.NewInMemoryStore()` 来自 eino-examples 仓库），自建 sqlite store 是唯一路径，这与 STACK.md「无官方持久化实现、自建」的判断一致且更强。

用户决策将升级目标从研究期锁定的 v0.9.13 改为 **v0.9.14**（2026-08-13 发布）。我核对了官方 GitHub release 页的完整 changelog：v0.9.13 → v0.9.14 **只有两处变更**——`fix(adk): summarization middleware 错误时返回输入 ctx 而非 nil`（#1157）与 `feat(adk): 新增 PatchedToolResultGenerator`（#1168）——均不触碰 CheckPointStore 接口、Interrupt/Resume API、react/compose/ADK 核心。因此 STACK.md 中针对 v0.9.13 的全部迁移分析对 v0.9.14 **逐字有效**；且 #1157 恰好修复的是 Phase 10 要用的 summarization middleware，v0.9.14 对我们严格优于 v0.9.13。

升级的真正工作量在三处：**(1)** 版本契约测试与 7 处 `eino@v0.7.7/...` godoc file:line 引用的重核；**(2)** sqlite `CheckPointStore` + `schema.RegisterName` 类型注册 + 真实中断往返测试（注意：注册 API 是 **`schema.RegisterName[T](name)`**，不是裸 `gob.RegisterName`——见 Common Pitfalls 1）；**(3)** 运行时行为面的回归重锁（cancel <200ms、contract 漂移套件、冻结 eval 重跑）——最后一项需要 live stack（内网 Milvus + BGE + GLM），本机无 Docker，沿用 Phase 7 的 human-verify checkpoint 模式。

**Primary recommendation:** 升级主提交只动 `go.mod`/`go.sum` + `TestContractVersionPin` 断言 + godoc 引用版本标注，零功能代码；CheckPointStore 与 lint 禁令作为独立任务跟进；live-verify 与冻结 eval 重跑设 `checkpoint:human-verify`。

<user_constraints>

## User Constraints

> 本 phase 目录下无 08-CONTEXT.md（目录为空）。以下约束来自用户 2026-08-17 的直接决策（经 orchestrator 传入，优先级高于 milestone 研究文档）。

### Locked Decisions

1. **升级目标 = eino v0.9.14**（2026-08-13 发布），覆盖研究期锁定的 v0.9.13（2026-07-22）。用户已用 `go list -m -u` 重新核对全部依赖最新版并做出选择。
2. **不用 v0.10.0-alpha.x**——maintainer 明确警告 breaking（沿用 STACK.md 结论，用户未推翻）。
3. **eino-ext 逐组件联动升级**，非整仓 bump：本 phase 实际涉及的只有 `eino-ext/components/model/openai`（v0.1.5 → v0.1.13）及其传递依赖（`eino-ext/libs/acl/openai` → v0.1.17、`meguminnnnnnnnn/go-openai` → v0.1.2）。
4. **milvus-sdk-go/v2 维持 v2.4.2**——v2.6 是新 major line（milvusclient），**不在本 phase 范围**（spike-004 已验证过迁移路径，留待后续）。
5. hertz v0.10.6 与 modernc.org/sqlite v1.56.0 均已是最新，无动作。
6. 所有既有回归门重新锁定（升级不混功能，回归归因干净——ROADMAP Phase 8 原则）。

### Claude's Discretion

- lint 禁令的实现机制（推荐 test-as-gate Go 测试，见 Architecture Patterns；项目无 golangci-lint 配置，不建议为此引入新工具链）。
- sqlite checkpoint 表结构与 store 实现细节（推荐形状见 Code Examples）。
- 中断往返测试对「进程重启」的模拟层级（新 DB handle + 新 Runner vs 真子进程；推荐前者，理由见 Open Questions Q2）。
- godoc file:line 引用重核的粒度与提交划分。

### Deferred Ideas (OUT OF SCOPE)

- milvus-sdk-go v2 → milvus/client/v2（milvusclient, Milvus 3.0）迁移——spike-004 已验证可行但属新 major line，明确不在本 phase。
- summarization middleware 的启用与 token 阈值校准（Phase 10 / RF-6）。
- ADK Runner 在 agent 路径（`react.NewAgent` → `Generate`）上的迁移——按 ARCHITECTURE-v08 §5.4 计划在 Phase 11（审批 phase）做，本 phase 只需保证 VerdictPipeline 的 Runner 路径在新版下继续工作。
- Langfuse/CozeLoop 观测组件（v0.8 milestone 明确 deferr）。
</user_constraints>

<phase_requirements>

## Phase Requirements

| ID | Description（来自 REQUIREMENTS.md） | Research Support |
|----|--------------------------------------|------------------|
| PLAT-04 | eino v0.7.7 → v0.9.13（用户决策覆盖为 **v0.9.14**）升级 + eino-ext 逐组件联动升级；回归门 = `integration/contract` 漂移套件 + 冻结 eval 基线全绿 + Phase-1 live verify 重新锁定（cancel <200ms、Milvus tier-filter）；legacy interrupt API lint 禁用 | 编译面零改动已实证（RF-2 spike）；`TestContractVersionPin` 需改写为新版本号（contract_test.go:90-113）；legacy API 清单与禁令实现方案已定（Pitfall 5 / Patterns）；cancel 测试双层路径已定位（hermetic + live）；冻结 eval 重跑需 live stack → human-verify |
| PLAT-05 | sqlite `CheckPointStore` 持久化实现（复用 `internal/store/sqlite`；所有自定义状态类型 `gob.RegisterName`；真实中断运行往返测试） | 接口形状已从 v0.9.14 源码锁定（2 方法 + 可选 Deleter）；**注册 API 修正为 `schema.RegisterName[T](name)`**（它内部才调 `gob.RegisterName`，见 Pitfall 1）；跨进程 resume 前提已被官方文档确认（同 CheckPointID + 共享 store 即可跨实例恢复）；往返测试的完整事件链（`Action.Interrupted.InterruptContexts[0].ID` → `ResumeWithParams(Targets)`）已从源码 + 官方 HITL 文档双源确认 |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 依赖版本升级（go.mod/go.sum） | 构建工具链（Go module/MVS） | — | 纯 dependency-management 关注点，无运行时分层 |
| sqlite CheckPointStore | Database / Storage（`internal/store/sqlite`） | API / Backend（Runner 接线在 Phase 11） | store 实现属于既有存储层（D18 interface-first 模式）；本 phase 只做骨架 + 测试，不接 server 路由 |
| checkpoint 状态类型注册 | API / Backend（类型声明所在的 `init()`） | 序列化层（eino InternalSerializer） | 注册必须发生在类型声明处（官方注释建议 init()），属代码关注点 |
| legacy interrupt lint 禁令 | 测试门（`integration/contract` repo 级 grep gate） | — | 与 `TestContractVersionPin` 同层的 repo 级漂移门 |
| cancel <200ms 重锁 | API / Backend（hermetic 测试） + 外部依赖（live GLM） | — | `TestCancelPropagationMidTool` 是 hermetic 的（stub hangChatModel）；Phase-1 live verify 需真 LLM |
| 冻结 eval 基线重跑 | 外部依赖（live Milvus + BGE + GLM） | — | 需完整 live stack，本机无 Docker → human-verify checkpoint |
| Milvus tier-filter live 重验 | 外部依赖（内网 Milvus） | — | `spiked e2e`/`live-verify.sh` 走内网端点 |

**本 phase 无前端改动**（`web/` 零触碰，UI hint: no）。

## Standard Stack

### Core（本 phase 的版本增量）

| Package | 当前 | 目标 | 用途 | 依据 |
|---------|------|------|------|------|
| `github.com/cloudwego/eino` | v0.7.7 | **v0.9.14** | agent 框架核心 | [VERIFIED: go list -m -u + module cache 源码 + 编译 spike] |
| `github.com/cloudwego/eino-ext/components/model/openai` | v0.1.5 | **v0.1.13** | GLM 的 OpenAI 兼容适配 | [VERIFIED: go.mod floor = eino v0.7.13，MVS 兼容 v0.9.14；编译 spike] |

### 传递依赖（go mod tidy 自动联动，不需手工指定）

| Package | 当前（indirect） | 升级后 | 依据 |
|---------|------------------|--------|------|
| `github.com/cloudwego/eino-ext/libs/acl/openai` | v0.1.2 | **v0.1.17** | [VERIFIED: ext v0.1.13 go.mod 要求] |
| `github.com/meguminnnnnnnnn/go-openai` | v0.1.0 | **v0.1.2** | [VERIFIED: ext v0.1.13 go.mod 要求] |

### 不动的依赖（已确认最新或明确排除）

| Package | 版本 | 理由 |
|---------|------|------|
| `github.com/cloudwego/hertz` | v0.10.6 | `go list -m -u` 显示无更新 [VERIFIED] |
| `github.com/milvus-io/milvus-sdk-go/v2` | v2.4.2 | 2.4 线最新；v2.6 属新 major line，用户明确排除 |
| `modernc.org/sqlite` | v1.56.0 | 已是最新 [VERIFIED]，checkpoint store 复用 |
| 其余（mongo-driver、fsnotify、testify 等） | — | 与 eino 无耦合，不在本 phase 范围 |

**Installation:**
```bash
go get github.com/cloudwego/eino@v0.9.14 github.com/cloudwego/eino-ext/components/model/openai@v0.1.13
go mod tidy
```

**版本事实核验（2026-08-17，本会话执行）：**
- `go list -m -u`：eino v0.7.7 → [v0.9.14]；ext openai v0.1.5 → [v0.1.13]；hertz/milvus/sqlite 无更新 [VERIFIED: Go proxy]
- eino v0.9.14 go directive 为 `go 1.18`——本机 go1.25.0 windows/amd64 满足 [VERIFIED: module cache go.mod]
- v0.9.13 → v0.9.14 changelog 仅 2 项（#1157 summarization ctx fix、#1168 PatchedToolResultGenerator feat），均不触及 CheckPointStore / interrupt / react / compose / Runner 核心 [VERIFIED: 官方 GitHub release 页原文]

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| eino v0.9.14 | v0.9.13（研究期原目标） | v0.9.14 多修一个 summarization middleware bug（Phase 10 直接受益）；无任何额外 breaking。无理由留在 v0.9.13 |
| eino v0.9.14 | v0.10.0-alpha.13 | maintainer 明确警告 alpha 不稳定；durable runtime 非本 milestone 需要。拒绝（沿用 STACK.md） |
| 自建 sqlite CheckPointStore | eino-examples 的 InMemoryStore | 不在生产路径（Pitfall 4 明令 Never）；且它不在 eino 核心模块内，引入反而多一个依赖源 |
| test-as-gate lint 禁令 | golangci-lint forbidigo/depguard 规则 | 项目从未配置 golangci-lint；为一条禁令引入整条工具链违背极简 posture。test-as-gate 与 `TestContractVersionPin` 先例一致 |

## Package Legitimacy Audit

> 本 phase **不安装任何新外部包**——只对两个既有 direct dependency 做版本升级。slopcheck 面向「新增包」的幻觉/抢注风险；对已存在于 go.mod、经 module proxy 解析且源码已在本地 cache 逐文件核对的包，该风险不适用。

| Package | Registry | 在库时长 | 状态 | slopcheck | Disposition |
|---------|----------|----------|------|-----------|-------------|
| `github.com/cloudwego/eino`（v0.7.7→v0.9.14） | Go proxy (proxy.golang.org) | 已在 go.mod（v0.7.7 pin 自 Phase 1） | 既有 direct dep | 不适用（非新增） | Approved |
| `github.com/cloudwego/eino-ext/components/model/openai`（v0.1.5→v0.1.13） | Go proxy | 已在 go.mod（v0.1.5 pin 自 Phase 1） | 既有 direct dep | 不适用（非新增） | Approved |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none
**新增外部包:** none（零新依赖——CheckPointStore 用 modernc.org/sqlite 既有依赖实现，lint 禁令用标准库 + testify）

## Architecture Patterns

### System Architecture Diagram（本 phase 交付物在系统中的位置）

```
                    ┌────────────────────────────────────────────────┐
                    │  任务 A：依赖升级（go.mod / go.sum）             │
                    │  eino v0.7.7 → v0.9.14 + ext openai v0.1.13    │
                    └───────────────┬────────────────────────────────┘
                                    │ 编译面已实证零源码改动
                                    ▼
        ┌───────────────────────────┴────────────────────────────┐
        │              既有回归门（全部重新锁定）                  │
        │  go build ./... ──► go test ./... ──► integration/contract │
        │       │                    │                  │           │
        │       │          TestCancelPropagationMidTool   TestContract│
        │       │          （hermetic <200ms 重锁）    VersionPin★    │
        │       │                                       (需改写断言) │
        └───────┼────────────────────────────────────────┼───────────┘
                ▼                                        ▼
   [live] spiked e2e probeCancellation        任务 B：legacy interrupt
   [live] 冻结 eval 重跑 (human-verify)        lint 禁令（repo 级 grep gate）
                │                                        │
                └───────────────┬────────────────────────┘
                                ▼
        ┌────────────────────────────────────────────────────────┐
        │  任务 C：sqlite CheckPointStore 骨架（internal/store/sqlite）│
        │                                                          │
        │  adk.Runner ──interrupt──► checkpoint bytes              │
        │      │                        │  Set/Get ([]byte)        │
        │      │                        ▼                          │
        │      │               sqlite checkpoint 表（新表，         │
        │      │               与 audit/session_turn 同库）          │
        │      │                        │                          │
        │      └────ResumeWithParams────┘（新 Runner 实例 + 重开 DB  │
        │            = 模拟进程重启；官方文档确认跨实例 resume 可行）  │
        │                                                        │
        │  自定义状态类型 ──schema.RegisterName[T]──► 序列化注册     │
        └────────────────────────────────────────────────────────┘
```

（★ = 升级唯一强制的既有代码改动：`integration/contract/contract_test.go:90-113` 按设计 fail-fast 断言 go.mod 版本字符串，升级后必红，需同步改写为新 pin。）

### Recommended Project Structure（增量）

```
internal/store/sqlite/
├── checkpoint.go          # NEW — CheckPointStore 实现（~100 LOC：Get/Set/可选 Delete + 表 DDL）
├── checkpoint_test.go     # NEW — Get/Set round-trip + 重开 DB 等值读回 + 类型注册覆盖
├── schema.go              # MODIFIED — checkpoint 表 DDL 追加（additive，不动既有两表）
└── ...

internal/store/
└── store.go               # （可选）若走 D18 interface-first，加编译期断言；
                           # 但 compose.CheckPointStore 接口属 eino，建议断言放
                           # sqlite 包内：var _ compose.CheckPointStore = (*CheckpointStore)(nil)

integration/contract/
├── contract_test.go       # MODIFIED — TestContractVersionPin 断言 v0.9.14 / v0.1.13
└── no_legacy_interrupt_test.go  # NEW — SC-3 lint 禁令（repo 级 grep gate）

（升级提交本身：go.mod / go.sum + 上述两处 + godoc 版本标注更新）
```

### Pattern 1: test-as-gate（lint 禁令与版本漂移门共用先例）

**What:** 把「禁止性规则」写成普通 Go 测试（读文件 + 断言零匹配），与 `TestContractVersionPin`（contract_test.go:90）同层。
**When to use:** 任何 repo 级字符串/标识符禁令，且不想引入 golangci-lint 等新工具链时。
**Example:** 见 Code Examples §4。

### Pattern 2: 类型声明处 `init()` 注册（checkpoint 序列化）

**What:** 所有会进入 checkpoint 的自定义类型，在其声明文件的 `init()` 里调 `schema.RegisterName[T]("稳定名字")`。
**When to use:** PLAT-05 全部自定义状态类型；Phase 11 的 ApprovalInfo/工具状态继承同一模式。
**Example:** 见 Code Examples §2。名字一旦上线即不可变更（兼容旧 checkpoint 的锚点）；官方注释明确「call this in an init() function in the file where the type is declared」。[VERIFIED: eino@v0.9.14/schema/serialization.go:59-90]

### Pattern 3: 升级提交的最小原子性

**What:** 升级主提交只含 `go.mod`/`go.sum` + 被升级按设计弄红的测试改写 + 注释里的版本标注；CheckPointStore、lint 禁令、live 重锁各自独立提交。
**When to use:** 一切「升级不混功能」场景——回归归因干净是 ROADMAP Phase 8 的显式 goal。

### Anti-Patterns to Avoid

- **裸 `gob.RegisterName` 直调**：v0.9.14 默认序列化器是 `serialization.InternalSerializer`（sonic/JSON 反射树），裸 gob 注册只覆盖一半注册面——必须走 `schema.RegisterName[T]`（它同时做 `gob.RegisterName` + `serialization.GenericRegister`）[VERIFIED: schema/serialization.go:83-90]。
- **用 `compose.RegisterSerializableType`**：已在 v0.9.14 标记 Deprecated（compose/checkpoint.go:47）。
- **在 react 路径上做中断测试**：`react.NewAgent → Generate` 不经过 Runner，没有 checkpoint 语义；SC-5 往返测试必须建在 `adk.NewChatModelAgent + NewRunner(CheckPointStore)` 上。
- **工具里用 `compose.Interrupt`**：v0.9.14 提供了对称的工具层 API `components/tool/interrupt.go`（`tool.Interrupt/StatefulInterrupt/GetInterruptState[T]/GetResumeContext[T]`）[VERIFIED: module cache]——Phase 11 写工具用这个，不要借 compose 层。
- **把 checkpoint DDL 塞进新迁移文件/新库**：复用 `internal/store/sqlite` 同一 DB 文件 + `CREATE TABLE IF NOT EXISTS` 追加（ARCHITECTURE-v08 §4.2 已定，单文件 ops 叙事）。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| checkpoint 序列化/类型注册 | 自写反射序列化或裸 gob 流程 | `schema.RegisterName[T]` + eino 默认 `InternalSerializer` | 序列化要处理流转换/子图/channel 类型（`compose/checkpoint.go` init 里注册了 5 个内部类型），自写必炸 |
| 中断信号构造 | 自定义 error 类型 | `tool.Interrupt` / `compose.Interrupt` / `adk.Interrupt`（按所在层选） | 地址系统（Address/InterruptSignal）是 resume 定位的根基，自造 error 无地址 → resume 串台（Pitfall 5 原文场景） |
| 版本漂移检测 | 自写 go.mod 解析器 | 既有 `TestContractVersionPin` 模式（`os.ReadFile` + `strings.Contains`） | 已验证 7 个 phase 的先例；改断言即可 |
| cancel 传播 | 自写超时轮询 | 既有 `TestCancelPropagationMidTool` + ctx cancel | 测试已在（chat_session_test.go:201），重跑即可 |
| checkpoint schema 迁移工具（若未来需要） | 自写字节级 patch | `compose.MigrateCheckpointState`（compose/checkpoint.go:261） | 官方已提供——decode→migrate→re-encode；Phase 11 的 Pitfall 6（skill 版本漂移拒绝 resume）可直接复用 |

**Key insight:** v0.9.14 的 HITL 面比 milestone 研究时认知的更完整：工具层中断 API、`MigrateCheckpointState`、`WithWriteToCheckPointID`/`WithForceNewRun`/`WithStateModifier` run options 全部就位——Phase 8 只需把地基（store + 注册 + 往返证明）打好，Phase 11 全部站在官方 API 上。

## Runtime State Inventory

> 本 phase 属迁移类（依赖升级 + 新持久化骨架），逐类显式回答：

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `skillforge.db`（本地 sqlite，gitignored）：audit + session_turn 两表现有数据；checkpoint 表尚不存在 | 无数据迁移——新表 additive `CREATE TABLE IF NOT EXISTS`，既有行零触碰 |
| Live service config | 无外部服务以 eino 版本为配置项。内网 Milvus/BGE/GLM 端点与 eino 版本正交 | 无 |
| OS-registered state | 无（无计划任务/服务注册；`.claude/worktrees/` 两个 worktree 自带 go.mod，被 Go `./...` 天然排除） | 无 |
| Secrets/env vars | `.env` 不存在（本机当前未配置）；`scripts/test`（**untracked**）内含 live GLM API key 与内网 IP——属 T-7-OSS 同类卫生风险 | **不提交、不入 .gitignore 白名单外的任何路径**；建议本 phase 顺手确认其保持 untracked 或删除（见 Pitfall 6） |
| Build artifacts | go.sum（本 phase 主对象）；Go module cache 已含 eino@v0.9.14 / @v0.9.13 / @v0.7.7 / @v0.7.13 与 ext openai@v0.1.13（本会话下载） | go.sum 随升级提交；module cache 无需清理 |

## Common Pitfalls

### Pitfall 1（本 phase 最高优先）：注册 API 用错——`gob.RegisterName` 是半截方案

**What goes wrong:** REQUIREMENTS PLAT-05 与 PITFALLS.md 都写「`gob.RegisterName` 注册」。v0.9.14 的事实是：默认序列化器为 `serialization.InternalSerializer`（sonic/JSON 反射树，`compose/checkpoint.go:177`），裸 `gob.RegisterName` 只满足 gob 半边，`InternalSerializer` 走的 `serialization.GenericRegister` 没注册 → resume 解码时才炸（延迟引爆，与 PITFALLS Pitfall 4 描述的延迟引爆同型）。
**Why it happens:** milestone 研究引用的 v0.8-era 文档（agent_extension 页）描述的是 gob 序列化；v0.9 换了默认序列化器，注册入口统一收口到 `schema.RegisterName[T]`。
**How to avoid:** 全部注册走 `schema.RegisterName[T]("稳定名字")`（它一次做两件事，见源码 serialization.go:83-90）。SC-4 的验收措辞按此理解：「经注册后可序列化写库、重启读回等值」——机制以 `schema.RegisterName` 为准。
**Warning signs:** resume 时 `Unmarshal` 报 unknown type / 空 state；单测 round-trip 过但跨进程 resume 挂。

### Pitfall 2：升级主提交与功能混在一起 → 回归归因污染

**What goes wrong:** 把 CheckPointStore、lint 禁令与 go.mod bump 塞一个提交，若冻结 eval 重跑出漂移，无法区分「升级引入」还是「新代码引入」。
**How to avoid:** 提交序列 = ① 升级+测试改写+注释标注（零功能）→ ② contract/全量单测绿 → ③ CheckPointStore+注册+round-trip → ④ 中断往返测试 → ⑤ lint 禁令 → ⑥（human）live 重锁。ROADMAP Phase 8 goal 原文「升级不混功能，回归归因干净」。

### Pitfall 3：`TestContractVersionPin` 按设计变红被误当升级事故

**What goes wrong:** 该测试（contract_test.go:79-113）的存在意义就是 fail-fast 防版本漂移——断言 `eino v0.7.7` / `openai v0.1.5` 字面量。升级后必红，若执行者不明其设计会绕行（skip/删除断言）。
**How to avoid:** 把「改写断言到 v0.9.14 / v0.1.13 + 更新注释里 v0.7.7 时代的 pin 理由」列为升级提交的显式任务；hertz v0.10.6 与 milvus v2.4.2 断言原样保留（本次不动）。

### Pitfall 4（承接 PITFALLS Pitfall 4）：内存 checkpoint 丢中断态

**What goes wrong / avoid:** 已由里程碑研究覆盖。本 phase 的落实 = sqlite store 骨架 + SC-5 重启往返测试。新增强事实：eino v0.9.14 核心连内存 store 都不附带，文档明示生产用分布式存储——官方 HITL 文档原话确认「As long as the CheckPointID is the same... and the CheckPointStore... is distributed storage, Eino can achieve cross-instance interrupt recovery」（sqlite 单文件在单机部署下即满足「跨进程共享」语义）。
**Warning signs:** 重启后 resume 报 checkpoint 不存在；`Get` 返回 `existed=false`。

### Pitfall 5（承接 PITFALLS Pitfall 5）：Interrupt/Resume 双语义并存

**What goes wrong:** legacy `compose.NewInterruptAndRerunErr` / `compose.InterruptAndRerun` 在 v0.9.14 仍存在（deprecated，无地址），照旧能编译——不删干净就会在未来 HITL 代码里混入。
**How to avoid:** SC-3 lint 禁令在本 phase 落地（test-as-gate，见 Code Examples §4）；禁令标识符清单：`NewInterruptAndRerunErr`、`InterruptAndRerun`（sentinel var）、`deprecatedInterruptAndRerun`（内部名一并列入防止绕行）。`WrapInterruptAndRerunIfNeeded` **不在禁令内**（它是官方迁移辅助）。当前代码库 grep 零命中 [VERIFIED: 本会话全仓检索]——禁令是预防性的。
**Warning signs:** 未来代码 review 出现 `compose.NewInterruptAndRerunErr`；resume 后工具拿到 nil 参数。

### Pitfall 6：`scripts/test` 泄密文件与脏工作树

**What goes wrong:** 工作树当前有未提交改动（internal/server、web/ 多文件 M 状态）+ untracked `scripts/test`（含 live GLM API key 与内网 IP）+ `screenshots/`。升级提交若 `git add -A` 会把密钥文件提交进历史（Phase 7 的 T-7-OSS-INFO / 密钥 gate 会全部破防）。
**How to avoid:** 升级提交只 add 明确路径（go.mod go.sum contract_test.go 注释文件等）；`scripts/test` 保持 untracked（或建议用户删除/移出仓库目录）；把「提交清单显式列举」写进 plan 的任务动作。

### Pitfall 7：godoc file:line 引用过期误导后续 phase

**What goes wrong:** agent.go 等处 7 个 `eino@v0.7.7/flow/agent/react/react.go:50` 式引用，升级后行号漂移（如 ToolCallingModel 文档在 v0.9.14 已移到 react.go:139 附近），后续 phase 按旧行号对源码会看错位置。
**How to avoid:** 升级提交内逐处重核并改标 `eino@v0.9.14/...:NNN`；无法逐行核的（行为注释）至少改版本号。位置清单见 Validation Architecture 的 Wave 0 清单。

## Code Examples

> 以下均从 v0.9.14 真实源码/本会话编译验证过的形状提炼。

### 1. CheckPointStore 接口（RF-2 答案，file:line 精确引用）

```go
// Source: eino@v0.9.14/internal/core/interrupt.go:27-41（compose/checkpoint.go:52 与 adk/runner.go:64 均为该接口的 alias）
type CheckPointStore interface {
    Get(ctx context.Context, checkPointID string) ([]byte, bool, error)
    Set(ctx context.Context, checkPointID string, checkPoint []byte) error
}

// 可选接口——实现了才有显式删除；不实现则框架不做自动清理（store 自管生命周期/TTL）
type CheckPointDeleter interface {
    Delete(ctx context.Context, checkPointID string) error
}
```

### 2. sqlite 实现骨架（推荐形状，~100 LOC）

```go
// internal/store/sqlite/checkpoint.go
package sqlite

import (
    "context"
    "database/sql"

    "github.com/cloudwego/eino/compose"
)

// 编译期接口断言（D18 模式；compose.CheckPointStore == adk.CheckPointStore 同一 alias）
var _ compose.CheckPointStore = (*CheckpointStore)(nil)

type CheckpointStore struct{ db *sql.DB }

func (s *CheckpointStore) Get(ctx context.Context, id string) ([]byte, bool, error) {
    const q = `SELECT payload FROM checkpoint WHERE id = ? AND (expires_at IS NULL OR expires_at > strftime('%Y-%m-%dT%H:%M:%fZ','now'))`
    var b []byte
    err := s.db.QueryRowContext(ctx, q, id).Scan(&b)
    if err == sql.ErrNoRows {
        return nil, false, nil // existed=false 不是错误
    }
    return b, err == nil, err
}

func (s *CheckpointStore) Set(ctx context.Context, id string, cp []byte) error {
    const q = `INSERT INTO checkpoint (id, payload) VALUES (?, ?)
               ON CONFLICT(id) DO UPDATE SET payload = excluded.payload`
    _, err := s.db.ExecContext(ctx, q, id, cp)
    return err
}
```

```sql
-- schema.go 追加（additive；列结构对齐 ARCHITECTURE-v08 §4.2，session_id/skill_id/expires_at
-- 为 Phase 11 预留，本 phase 建表即带，避免后续 ALTER）
CREATE TABLE IF NOT EXISTS checkpoint (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL DEFAULT '',
    skill_id   TEXT NOT NULL DEFAULT '',
    payload    BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    expires_at TEXT
);
```

### 3. 类型注册 + 中断往返测试的事件链

```go
// 注册：类型声明处 init()（Source: eino@v0.9.14/schema/serialization.go:59-90 官方注释）
func init() {
    schema.RegisterName[*ApprovalState]("skillforge_approval_state")
}

// SC-5 往返测试的骨架（stub ChatModel 剧本：第一轮返回 tool call，resume 后返回终答）：
iter := runner.Query(ctx, "触发写工具的 query", adk.WithCheckPointID("test-cp-1"))
var interruptID string
for { // drain：中断事件在 Action.Interrupted 上（官方 HITL Quick Start 原文形状）
    ev, ok := iter.Next(); if !ok { break }
    if ev.Err != nil { t.Fatal(ev.Err) }
    if ev.Action != nil && ev.Action.Interrupted != nil {
        interruptID = ev.Action.Interrupted.InterruptContexts[0].ID // 定位符
    }
}

// 「进程重启」模拟 = 重开同一 DB 文件 + 新建 Runner（官方文档确认跨实例语义成立）
db2, _ := sql.Open("sqlite", samePath)
runner2 := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent2, CheckPointStore: &CheckpointStore{db: db2}})
iter2, err := runner2.ResumeWithParams(ctx, "test-cp-1", &adk.ResumeParams{
    Targets: map[string]any{interruptID: &ApprovalResult{Approved: true}},
})
```

### 4. legacy interrupt lint 禁令（test-as-gate）

```go
// integration/contract/no_legacy_interrupt_test.go
// SC-3 / Pitfall 5：无地址旧语义 interrupt API 出现在代码库即 fail。
func TestNoLegacyInterruptAPI(t *testing.T) {
    banned := []string{
        "NewInterruptAndRerunErr",      // deprecated func（compose/interrupt.go:52）
        "compose.InterruptAndRerun",    // deprecated sentinel var（compose/interrupt.go:46）
        "deprecatedInterruptAndRerun",  // 内部名，防绕行
        // WrapInterruptAndRerunIfNeeded 不禁——官方迁移辅助
    }
    // 走 .go 源文件（排除 module cache / worktrees），命中即 t.Fatalf 报文件:行
}
```

### 5. TestContractVersionPin 改写样例

```go
// Source: 现有 integration/contract/contract_test.go:97-108（v0.7.7 时代断言）
// 升级后：
assert.True(t, strings.Contains(body, "github.com/cloudwego/eino v0.9.14"), ...)      // v0.7.7 → v0.9.14
assert.True(t, strings.Contains(body, ".../eino-ext/components/model/openai v0.1.13"), ...) // v0.1.5 → v0.1.13
// hertz v0.10.6 / milvus-sdk-go/v2 v2.4.2 两断言原样保留
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| v0.7.x interrupt/resume 重构线（当前 pin v0.7.7） | v0.9.x agentic-runtime（typed generics + alias 保旧名） | v0.8（ADK middlewares，2026 上半年）→ v0.9（agentic-runtime）→ v0.9.14（2026-08-13） | 对我们：编译面零改动（实证）；能力面解锁 summarization middleware（Phase 10）、HITL 成熟 API（Phase 11）、deep prebuilt（Phase 12） |
| `compose.RegisterSerializableType` 类型注册 | `schema.RegisterName[T](name)` / `schema.Register[T]()` | v0.9.x | Deprecated；新代码禁用旧入口 |
| checkpoint 序列化 = gob | 默认 `serialization.InternalSerializer`（sonic/JSON 反射树；gob 规则仍约束字段可见性） | v0.9.x | 注册必须走 schema.RegisterName（双注册） |
| （文档示例）InMemoryStore | eino 核心不附带任何 store；文档明示生产自备（Redis 等） | v0.9.x | 自建 sqlite store 是唯一路径（强化 STACK.md 结论） |
| v0.10.0-alpha.x（durable runtime/automemory） | 预发布，maintainer 警告 breaking，stable 目标 2026 Q3 | — | 本 milestone 不碰；v0.10 GA 后再评估 |

**Deprecated/outdated（对我们代码零影响，但 Phase 10+ 落地时须知）：**
- `adk.ChatModelAgentConfig.Middlewares []AgentMiddleware`（struct 型）：deprecated，将移除；新代码用 `Handlers`（interface 型 `ChatModelAgentMiddleware`）[VERIFIED: eino@v0.9.14/adk/chatmodel.go:310-321]
- `adk.ResumeInfo` 内嵌 `*InterruptInfo`：deprecated，改用 `InterruptContexts` + `GetInterruptState` [VERIFIED: adk/interrupt.go:33-46]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | 运行时行为（cancel 传播、SSE 事件时序、回调触发点）在 v0.7.7→v0.9.14 间与编译面同构——仅经 STACK.md 引用的 v0.9 release notes（cancel scopes #1143/#1151）与本会话编译验证推断，未跑完整套件 | Summary / Pitfalls | 中：可能暴露行为漂移——但这正是 SC-1/SC-2 回归门存在的目的，套件会抓住 |
| A2 | 官方文档示例的 `store.NewInMemoryStore()` 来自 eino-examples 仓库而非 eino 核心（依据：v0.9.14 模块全量 grep 零命中 + 示例 import 了 eino-examples common 包；未直接打开 eino-examples 确认包路径） | Summary / Don't Hand-Roll | 低：不影响任何决策（我们不用它） |
| A3 | `InternalSerializer` 产出的 checkpoint 字节为 sonic/JSON 封装格式（源码 internalMarshal→sonic.Marshal 推断；未逐字段验证兼容性约束） | Pitfall 1 | 低：我们只透传 []byte，不解析其内容；格式变化风险由 eino 自担 |
| A4 | 冻结 eval 重跑的「重新锁定」语义 = 同一 lock（model_ver/prompt_hash/seed 未变，lock 文件不含 eino 版本）重跑比对容差，而非重新生成 lock | Open Questions Q3 | 低-中：若用户期望重新生成 lock，任务动作不同（已列为待确认） |

**其余所有 API 断言均为 [VERIFIED]**（module cache 源码 file:line）或 [CITED]（官方 release 页 / 官方 HITL 文档原文）。

## Open Questions (RESOLVED)

> 2026-08-17 planner 修订：以下 4 题全部在 Phase 8 plan 集内采纳落地（Q1→08-03 T2 文档对齐；Q2→08-04 T1 重启层级=新 DB handle + 新 Runner；Q3→08-05 T3 口径裁决；Q4→08-05 checkpoint 模式），无遗留待确认项。

1. **ROADMAP/REQUIREMENTS/CLAUDE.md 中 v0.9.13 字样是否随本 phase 更新为 v0.9.14？（RESOLVED → 08-03 Task 2 文档对齐：三文件定点改 v0.9.14 + 决策注记；目录名不动）**
   - What we know: ROADMAP Phase 8 标题与 goal、REQUIREMENTS PLAT-04、STATE.md、CLAUDE.md 栈表均写 v0.9.13；用户决策已改 v0.9.14。phase 目录名也含 `v0-9-13`。
   - What's unclear: 文档对齐属于本 phase 还是独立 quick task。
   - Recommendation: 本 phase 收尾加一个纯文档提交（REQUIREMENTS/ROADMAP/CLAUDE.md 三处版本字串 + 必要处注一句「用户 2026-08-17 决策 v0.9.14」）；目录名不动（GSD 目录名是身份锚点）。

2. **SC-5 的「进程重启」验证深度：新 DB handle + 新 Runner 是否足够，还是要真子进程？（RESOLVED → 08-04 Task 1：重启层级 = 新 DB handle + 新 Agent + 新 Runner 实例；真子进程列为可选加固非硬门）**
   - What we know: 官方文档确认跨实例/跨进程 resume 的成立条件就是「同 CheckPointID + 共享 CheckPointStore」；序列化边界（bytes 进出 store）已被 round-trip 覆盖。
   - What's unclear: 用户对「进程重启」字面的验收期望。
   - Recommendation: 主测试用「重开 DB + 新 Runner + 新 Agent 实例」（覆盖序列化与状态恢复的全部风险面）；真子进程版（`go test` 环境变量两阶段或 `exec.Command` 自身再跑）列为可选加固，不作 SC 硬门。

3. **冻结 eval「重新运行全绿」的判定口径（RESOLVED → 08-05 Task 3 口径裁决：frozen run 完成 + gate 通过 + 指标留档为 v0.8 基线；绝不重生成 lock 掩盖漂移）**
   - What we know: `eval/frozen/compliance.lock.json` 只含 `model_ver/prompt_hash/seed/eval_set_version/skill_name`，不含 eino 版本 [VERIFIED: 本会话读文件]；eino 升级不改变 pin 输入。
   - What's unclear: 全绿 = 四指标与既有 report 在容差内一致（SC-6 可复现性口径），还是允许刷新基线。
   - Recommendation: 按前者执行（与 Phase 6 SC-6 同口径）；若漂移超容差，那是升级引入的真实回归，按 blocking 处理并归因——绝不刷新 lock 掩盖。

4. **live 重锁三项（cancel live / Milvus tier-filter / 冻结 eval）的 checkpoint 归属（RESOLVED → 08-05：三项全设 checkpoint:human-verify gate=blocking，Phase 7 先例模式；hermetic 部分自动跑）**
   - What we know: 本机无 Docker、无 .env；`scripts/test`（untracked）显示内网 Milvus 10.62.10.34:19530 + BGE 10.62.0.123:11434 + live GLM key 可达。
   - Recommendation: 全部标 `checkpoint:human-verify gate=blocking`（Phase 7 先例模式）；hermetic 部分（TestCancelPropagationMidTool、contract 套件）自动跑。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | 升级 + 全部测试 | ✓ | go1.25.0 windows/amd64（eino v0.9.14 仅要求 go 1.18） | — |
| Go module proxy 网络 | `go get`/`go mod tidy` | ✓（本会话多次下载成功，cache 已预热 eino@v0.9.14 + ext@v0.1.13） | — | — |
| Docker | 无（本 phase 不依赖容器） | ✗（不在 PATH） | — | 不需要 |
| live Milvus（内网） | SC-2 tier-filter live 重验、冻结 eval | ✓ 端点可达（10.62.10.34:19530，见 scripts/test）但需用户起栈 | v2.4.x 线 | human-verify checkpoint |
| live BGE-M3（内网） | 冻结 eval、live-verify | ✓ 端点可达（10.62.0.123:11434，需 `BGE_M3_TLS_SKIP_VERIFY=true`） | — | human-verify checkpoint |
| GLM API（公网） | live cancel 重验（spiked e2e 仅需 GLM）、冻结 eval | ✓ key 存在于 scripts/test（untracked） | glm-5.2 | human-verify checkpoint |
| git-filter-repo / gh | 无 | — | — | 不需要（无远端操作） |

**Missing dependencies with no fallback:** 无——核心交付（升级 + store + 往返测试 + hermetic 回归）全部可在本机自动完成。
**Missing dependencies with fallback:** live 三项 → human-verify checkpoint（Phase 7 同模式）。

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go 标准 `testing` + `testify`（后端）；vitest（前端，本 phase 不触碰） |
| Config file | 无独立配置（Go 惯例；`go vet` 作为静态门） |
| Quick run command | `go build ./... && go vet ./... && go test ./internal/... ./cmd/... ./scripts/...` |
| Full suite command | `go test ./...`（含 `./integration/contract`；`.claude/worktrees/*` 自带 go.mod 被天然排除） |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PLAT-04 | 升级后编译 + 全量单测绿 | 回归 | `go build ./... && go test ./...` | ✅（既有套件，升级后重跑） |
| PLAT-04 | SSE wire 契约不漂移（hermetic：mock OpenAI server + 纯函数 filter） | 回归/contract | `go test ./integration/contract -v` | ✅（`TestContractVersionPin` 需改写断言后绿） |
| PLAT-04 | cancel <200ms（hermetic 层） | 回归 | `go test ./internal/server -run TestCancelPropagationMidTool -v` | ✅ chat_session_test.go:201 |
| PLAT-04 | cancel <200ms（live 层） | manual/live | `go run ./scripts/spiked e2e`（probeCancellation，仅需 GLM） | ✅ 脚本在；执行 = human-verify |
| PLAT-04 | Milvus tier-filter 声明外 tier 零行（live） | manual/live | `bash scripts/live-verify.sh`（内网 Milvus） | ✅ 脚本在；执行 = human-verify |
| PLAT-04 | 冻结 eval 基线重跑 | manual/live | `go run ./cmd/eval --frozen=compliance` | ✅ CLI 在；执行 = human-verify |
| PLAT-04 | legacy interrupt API 出现即 fail | unit（repo grep gate） | `go test ./integration/contract -run TestNoLegacyInterruptAPI -v` | ❌ Wave 0 新建 |
| PLAT-05 | CheckPointStore Get/Set round-trip + 重开 DB 等值读回 + 注册覆盖 | unit | `go test ./internal/store/sqlite -run TestCheckpoint -v` | ❌ Wave 0 新建 |
| PLAT-05 | 真实中断 → 持久化 → 「重启」→ Resume 续行 | unit（stub model 剧本） | `go test ./internal/store/sqlite -run TestInterruptRoundTrip -v`（或独立包） | ❌ Wave 0 新建 |

### Sampling Rate

- **Per task commit:** `go build ./... && go vet ./... && go test ./internal/... ./cmd/...`
- **Per wave merge:** `go test ./...`（全量，含 contract）
- **Phase gate:** 全量绿 + live 三项 human-verify 关闭后才进 `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/store/sqlite/checkpoint.go` + `checkpoint_test.go` — 覆盖 PLAT-05 SC-4（含 `schema.RegisterName` 注册的自定义类型 round-trip + 重开 DB）
- [ ] 中断往返测试（建议同包或 `internal/store/sqlite/checkpoint_resume_test.go`）— 覆盖 PLAT-05 SC-5（stub ChatModel 剧本 + adk Runner + `tool.Interrupt`）
- [ ] `integration/contract/no_legacy_interrupt_test.go` — 覆盖 PLAT-04 SC-3
- [ ] `integration/contract/contract_test.go` `TestContractVersionPin` 断言改写（v0.9.14 / v0.1.13；hertz/milvus 保留）
- [ ] godoc 版本标注更新 7 处：`internal/agent/agent.go:52,167,226,236`、`internal/agent/callbacks.go:103`、`internal/agent/retrieve_tool.go:182`、`scripts/spiked/e2e.go:244`（+`scripts/spiked/wire.go:22` 注释）

## Security Domain

> `security_enforcement` 未在 config.json 显式设置 → 按 enabled 处理。

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|------------------|
| V2 Authentication | no | 本 phase 无新端点/身份面 |
| V3 Session Management | no | session 机制不动 |
| V4 Access Control | no | 无新路由；checkpoint 表本 phase 仅供测试访问 |
| V5 Input Validation | 间接 | checkpoint `Get/Set` 的 id/[]byte 经 100% `?` 参数化 SQL（延续 T-3-SQLI posture——示例代码已按此写） |
| V6 Cryptography | no | 无新加密需求 |
| V10 供应链（dependency integrity） | **yes** | 本 phase 主对象：仅升级既有 direct deps；go.sum 校验和由 Go 工具链强制；零新包引入（见 Package Legitimacy Audit）；版本号经 go proxy + 官方 release 双源核对 |

### Known Threat Patterns for 依赖升级 + checkpoint 存储

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 恶意/抢注依赖混入 | Tampering/Supply-chain | 只 bump 既有 module path（`github.com/cloudwego/eino` 等），零新 module path；go.sum 由工具链校验 |
| checkpoint payload 注入（未来 resume 数据面） | Tampering | 本 phase store 只透传 bytes 不解析；Phase 11 的 resume-data schema 校验在 HITL-04 落地（预警入档） |
| checkpoint ID 可猜测 → 越权 Get（未来 HTTP 面） | Information Disclosure | 本 phase 无 HTTP 暴露；Phase 11 落地时绑定 session（SC-9 纪律）——研究已预留 `session_id` 列 |
| 密钥文件误提交（scripts/test 含 live key） | Information Disclosure | 提交清单显式列举、绝不 `git add -A`（Pitfall 6）；建议用户删除该文件或确保持续 untracked |

## Sources

### Primary (HIGH confidence)

- **eino v0.9.14 module cache 源码**（本会话 `go mod download` 后逐文件核对，路径 `$(go env GOMODCACHE)/github.com/cloudwego/eino@v0.9.14`）：
  - `internal/core/interrupt.go:27-41` — CheckPointStore/CheckPointDeleter 接口
  - `compose/checkpoint.go:29-95,261` — 内部类型注册、`WithCheckPointStore/WithCheckPointID/WithWriteToCheckPointID/WithForceNewRun/WithStateModifier`、`MigrateCheckpointState`、Deprecated `RegisterSerializableType`
  - `compose/interrupt.go:44-117,299` — legacy API 仍存且 deprecated、`WrapInterruptAndRerunIfNeeded`、`Interrupt/StatefulInterrupt/CompositeInterrupt`、`ExtractInterruptInfo`
  - `schema/serialization.go:59-148` — `RegisterName[T]`/`Register[T]`（官方注释：init() 注册、gob 规则）
  - `adk/runner.go:58-151` — `RunnerConfig{Agent,EnableStreaming,CheckPointStore}`、`Run/Query/Resume/ResumeWithParams`
  - `adk/chatmodel.go:260-321,417,503` — ChatModelAgentConfig（alias 保留、Middlewares deprecated）
  - `adk/interface.go:357-438` — AgentAction.Interrupted / AgentEvent alias
  - `adk/interrupt.go:33-196` — ResumeInfo/InterruptInfo/InterruptCtx、`adk.WithCheckPointID`
  - `flow/agent/react/react.go:136-160` + `flow/agent/agent_option.go:41` + `compose/graph_call_options.go:198,224` + `utils/callbacks/template.go` + `components/tool/interrupt.go:44-183` — 既有编译面 + 工具层中断 API
  - `go.mod` — go 1.18 directive
- **官方 GitHub release v0.9.14 页**（2026-08-13，commit 32759e6，GPG verified）— v0.9.13→v0.9.14 完整 changelog（仅 #1157 + #1168）
- **官方 HITL 文档** cloudwego.io `/docs/eino/core_modules/eino_adk/agent_hitl/` — Quick Start 完整事件链（`Action.Interrupted.InterruptContexts[0].ID/.Info` → `ResumeWithParams(Targets)`）、跨实例 resume 条件原文、生产 store 自备指引
- **本会话编译验证**（RF-2 spike）：仓外临时模块 require eino@v0.9.14 + ext openai@v0.1.13，按 SkillForge 实际 API 形状写 main.go，`go build` + `go vet` exit 0（验证后已删除）
- **`go list -m -u`**（2026-08-17 本会话）— eino/ext/hertz/milvus/sqlite 最新版核对
- **本仓代码检索** — 26 个 eino import 文件清单、`TestContractVersionPin`、`TestCancelPropagationMidTool`、7 处 `eino@v0.7.7` godoc 引用、interrupt 零使用、`scripts/live-verify.sh`、`eval/frozen/compliance.lock.json` 结构

### Secondary (MEDIUM confidence)

- `.planning/research/STACK.md` / `ARCHITECTURE-v08.md` / `PITFALLS.md`（2026-08-17 里程碑研究，v0.9.13 取向）— 迁移叙事与 v0.8/v0.9 breaking changes 清单（本次逐项对 v0.9.14 源码复核，未发现与 v0.9.14 矛盾处；其「eino-ext README 声明 eino ≥v0.9 compat」的逐组件核对建议已由 go.mod floor 事实替代）
- STACK.md 引用的 v0.9 cancel-scope 变更（#1143/#1151）——未直接核对 PR，按 [CITED] 承接，其结论（重锁 cancel SC）不受影响

### Tertiary (LOW confidence)

- A2（InMemoryStore 位于 eino-examples 的具体包路径）、A3（InternalSerializer 字节格式细节）——见 Assumptions Log

## Metadata

**Confidence breakdown:**
- 升级目标与版本事实: HIGH - go proxy + 官方 release + go.mod floor 三源一致
- 编译面影响（现有代码零改动）: HIGH - 真实源码 diff + 可编译 spike 实证（覆盖 SkillForge 全部 eino API 形状）
- CheckPointStore/中断 API 形状（RF-2）: HIGH - 源码 file:line + 官方文档双源；往返测试设计为 MEDIUM（stub 剧本待 phase 内落地验证）
- 运行时行为不漂移: MEDIUM - 编译同构 ≠ 行为同构；这正是 SC-1/SC-2 回归门 + live 重锁存在的理由
- 文档对齐范围（Open Q1）与冻结 eval 口径（Open Q3）: MEDIUM - 待用户确认

**Research date:** 2026-08-17
**Valid until:** 2026-09-16（eino 发布节奏 ~1-2 周/patch；若执行时 v0.9.15+ 已出，按用户决策仍锁 v0.9.14，`TestContractVersionPin` 语义不受影响）
