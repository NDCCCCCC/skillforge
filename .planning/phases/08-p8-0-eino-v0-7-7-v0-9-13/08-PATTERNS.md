# Phase 8: 全量依赖升级（P8.0 — eino v0.7.7 → v0.9.14）- Pattern Map

**Mapped:** 2026-08-17
**Files analyzed:** 9（4 个新建 + 5 组修改；godoc 8 处注释合并计为 1 组）
**Analogs found:** 8 / 9（go.mod 升级与纯注释改动无需 analog，按"自身"计）

> 本 phase 目录无 08-CONTEXT.md（用户决策已内嵌于 08-RESEARCH.md `<user_constraints>`）。
> 文件清单来源：08-RESEARCH.md 的 Recommended Project Structure（第 172-191 行）+ Wave 0 Gaps（第 495-501 行）。

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `go.mod` + `go.sum` | config（依赖清单） | N/A（MVS 解析） | 自身（`go get` + `go mod tidy`，无代码 analog 需要） | self |
| `integration/contract/contract_test.go` | test（版本漂移门） | file-I/O（读 go.mod + 字串断言） | 自身 `TestContractVersionPin`（contract_test.go:90-113）——原位改写断言 | exact |
| `internal/store/sqlite/schema.go` | config（DDL 常量） | N/A | 自身 `schemaSQL` const + `migrate()`（schema.go:32-74）——原位追加 | exact |
| `internal/store/sqlite/checkpoint.go`（NEW） | store（数据访问） | CRUD（`[]byte` 的 Get/Set upsert） | `internal/store/sqlite/session.go`（单行 SELECT + ON CONFLICT upsert）+ `sqlite.go`（编译期断言 / fmtError / isClosed） | exact（role + flow 双匹配） |
| `internal/store/sqlite/checkpoint_test.go`（NEW） | test | CRUD round-trip + 重开 DB 等值读回 | `internal/store/store_test.go`（newTestStore helper + round-trip 断言风格） | exact |
| `internal/store/sqlite/checkpoint_resume_test.go`（NEW，建议名） | test（中断→持久化→"重启"→Resume） | event-driven（Runner 迭代器 drain）+ 持久化 | 复合三源：`internal/server/chat.go:914-939`（drainFinalContent 事件循环）+ `internal/agent/agent_test.go:657-719`（multiTurnStubChatModel 脚本化模型）+ `cmd/server/main.go:196-206`（adk Runner 接线） | composite（事件链 `Action.Interrupted` 无仓内先例，骨架来自 RESEARCH §3） |
| `integration/contract/no_legacy_interrupt_test.go`（NEW） | test（repo 级 lint 禁令门） | file-I/O（遍历 .go 源文件 + 禁令字串匹配） | `integration/contract/contract_test.go` 的 `TestContractVersionPin`（os.ReadFile + strings.Contains 断言形态）+ `findModuleRoot`（:484-502 向上走定位 repo 根） | partial（目录遍历扫描部分无仓内先例） |
| godoc 版本标注 ×8（`internal/agent/agent.go:52,167,226,236`、`internal/agent/callbacks.go:103`、`internal/agent/retrieve_tool.go:182`、`scripts/spiked/e2e.go:244`、`scripts/spiked/wire.go:22`） | comment-only | N/A | 无需 analog（逐处改 `v0.7.7`→`v0.9.14` 并重核 file:line） | self |
| （可选文档提交）`REQUIREMENTS.md`/`ROADMAP.md`/`CLAUDE.md` 版本字串 | docs | N/A | — | —（Open Q1，建议收尾纯文档提交） |

---

## Pattern Assignments

### `internal/store/sqlite/checkpoint.go`（NEW — store, CRUD）

**Analog:** `internal/store/sqlite/session.go`（单行读 + upsert 写）+ `internal/store/sqlite/sqlite.go`（包级惯例）

**推荐实现形态（A —— 方法挂在 SQLiteStore 上，最贴仓内惯例）：**
新类型不引独立 `*sql.DB` —— `SQLiteStore` 已持有 `db`、`isClosed()`、`fmtError()`、`migrate()` 全部基建，checkpoint 的 Get/Set 直接作为 `SQLiteStore` 的新方法落在 checkpoint.go，编译期断言放同文件。RESEARCH §2 的裸 `CheckpointStore{db}` 骨架为备选 B（需从 `SQLiteStore.DB()` 取 handle，该 accessor 注释明示 production 不走 —— 故不推荐）。

**License header**（每个新 .go 文件必带，复制 `internal/store/sqlite/sqlite.go:1-15` 的 Apache-2.0 头，SPDX 行结尾）。

**编译期接口断言**（D18 模式；sqlite.go:79-82 先例）：

```go
// sqlite.go:79-82 现有形状 —— checkpoint 断言照此写：
// Compile-time check that *SQLiteStore satisfies store.Store...
var _ store.Store = (*SQLiteStore)(nil)

// checkpoint.go 新增（compose.CheckPointStore 与 adk.CheckPointStore 是同一接口的 alias，
// 断言一个即同时锁两个 —— RESEARCH Code Examples §1）：
var _ compose.CheckPointStore = (*SQLiteStore)(nil)
```

**Get：单行 QueryRow + Scan + sql.ErrNoRows 三态**（单行读 analog：sqlite.go:120-132 Health 的 `QueryRowContext(...).Scan(...)`；`existed=false` 不是错误的语义按 RESEARCH §2）：

```go
// sqlite.go:125-127（Health 的单行读形状）：
if err := s.db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
    return fmtError("Health SELECT 1", err)
}

// checkpoint Get 应写成（ErrNoRows → (nil, false, nil)，其余走 fmtError）：
row := s.db.QueryRowContext(ctx, getCheckpointSQL, id)
var b []byte
if err := row.Scan(&b); err != nil {
    if errors.Is(err, sql.ErrNoRows) {
        return nil, false, nil // existed=false 不是错误（Pitfall 4 warning sign 的反面）
    }
    return nil, false, fmtError("CheckpointGet", err)
}
return b, true, nil
```

**Set：const SQL + 全 `?` 参数化 + ON CONFLICT upsert**（upsert analog：session.go:73-77 `appendTurnSQL` 的 `ON CONFLICT ... DO NOTHING`；checkpoint 用 `DO UPDATE SET payload = excluded.payload`）：

```go
// session.go:73-77（现有 upsert 先例）：
const appendTurnSQL = `
INSERT INTO session_turn (session_id, role, content, ts)
VALUES (?, ?, ?, ?)
ON CONFLICT(session_id, role, ts) DO NOTHING
`
```

**closed 守卫**（session.go:43-45 先例，每个方法入口）：

```go
if s.isClosed() {
    return nil, sql.ErrConnDone
}
```

**错误包装**（sqlite.go:162-170 `fmtError` —— 已是包级函数，直接复用，不要新写包装器）。

**可选 Delete**：实现 `CheckPointDeleter`（`Delete(ctx, id) error`），同样模式；不实现则框架不自动清理（RESEARCH §1）。

---

### `internal/store/sqlite/schema.go`（MODIFIED — DDL 追加）

**Analog:** 自身 `schemaSQL` const（schema.go:32-64）

**追加位置：`schemaSQL` 常量字符串内部**，紧跟 `session_turn` 的索引之后 —— 不是新 const、不是新迁移文件（RESEARCH Anti-Pattern 明令：复用同一 DB 文件 + `CREATE TABLE IF NOT EXISTS` 追加）。幂等性由既有的 `migrate()`（schema.go:69-74，每次 `New()` 都跑）自动获得，无需新迁移函数。

```go
// schema.go:32（现有 const 起点）：
const schemaSQL = `
CREATE TABLE IF NOT EXISTS audit ( ... );
...
CREATE INDEX IF NOT EXISTS idx_session_turn_sid ON session_turn(session_id);
` // ← checkpoint 表 DDL 追加到这个字符串尾部：

CREATE TABLE IF NOT EXISTS checkpoint (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL DEFAULT '',
    skill_id   TEXT NOT NULL DEFAULT '',
    payload    BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    expires_at TEXT
);
```

（列结构对齐 ARCHITECTURE-v08 §4.2；`session_id`/`skill_id`/`expires_at` 为 Phase 11 预留，本 phase 建表即带。）

---

### `internal/store/sqlite/checkpoint_test.go`（NEW — test, CRUD round-trip）

**Analog:** `internal/store/store_test.go`（黑盒包 + helper + 断言风格）

**包形态**（store_test.go:17-24）：外部测试包 `package store_test`，只走导出面 —— checkpoint 测试同构：`package sqlite_test`（或并入既有 `store_test`，若断言挂在导出的 SQLiteStore 方法上则可直接并入；推荐独立 `checkpoint_test.go` 文件但同包 `store_test`，复用 helper）。

**newTestStore helper**（store_test.go:47-59 原样复用）：

```go
// 每个测试独立 shared-cache in-memory DB：
dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
s, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
require.NoError(t, err, "sqlite.New should open an in-memory store")
t.Cleanup(func() { _ = s.Close() })
```

**「重开 DB 等值读回」用 file-backed DSN**（store_test.go 的 in-memory 形态在此不适用 —— 重开 `:memory:` 是空库）。用 `t.TempDir()`：

```go
dbPath := filepath.Join(t.TempDir(), "checkpoint-reopen.db")
s1, err := sqlite.New(ctx, sqlite.Config{DSN: dbPath})   // 第一次开：Set
require.NoError(t, err)
// ... s1.CheckpointSet(ctx, "cp-1", payload) ...
require.NoError(t, s1.Close())                            // "进程退出"
s2, err := sqlite.New(ctx, sqlite.Config{DSN: dbPath})   // 第二次开：Get 等值
got, existed, err := s2.CheckpointGet(ctx, "cp-1")
require.True(t, existed)
require.Equal(t, payload, got)                            // bytes 级等值（SC-4）
```

**断言风格**（store_test.go 全文先例）：`require.NoError` / `require.True` / `assert.Equal`，消息里写明 SC 编号与失败含义（如 store_test.go:100 `"u1 has two rows"`）。

**SQL 注入回归**：参照 store_test.go:151-185 `TestAuditRowSQLInjection` 的 DROP TABLE payload 手法，对 checkpoint id 加一例（id 走 `?` 参数化 —— T-3-SQLI posture 延续，RESEARCH Security Domain V5 已按此写）。

---

### `internal/store/sqlite/checkpoint_resume_test.go`（NEW — test, event-driven + 持久化）

**Analog（复合三源）：**

**(1) adk Runner 接线** —— `cmd/server/main.go:196-206`：

```go
// main.go:197-206（现有形状）：
adkAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Name:        "skillforge-compliance",
    Description: "SkillForge compliance-review ChatModelAgent",
    Instruction: "You are a strict compliance auditor. ...",
    Model:       cm,
})
if err != nil { ... }
runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: adkAgent})
```

新差异点（RESEARCH §3）：`RunnerConfig` 增设 `CheckPointStore` 字段 + Query 带 `adk.WithCheckPointID("test-cp-1")`。**必须建在 adk.ChatModelAgent + Runner 路径上，不可用 react.NewAgent**（react → Generate 不经过 Runner，无 checkpoint 语义 —— RESEARCH Anti-Patterns 明令）。

**(2) 迭代器 drain 事件循环** —— `internal/server/chat.go:914-939` `drainFinalContent`：

```go
// chat.go:917-936（现有事件循环形状）：
for {
    ev, ok := iter.Next()
    if !ok { break }
    if ev == nil { continue }
    if ev.Err != nil && firstErr == nil { firstErr = ev.Err }
    if ev.Output != nil && ev.Output.MessageOutput != nil {
        msg, mErr := ev.Output.MessageOutput.GetMessage()
        ...
    }
}
```

新差异点：循环里**多捕一个分支** `ev.Action != nil && ev.Action.Interrupted != nil` → 记下 `ev.Action.Interrupted.InterruptContexts[0].ID`（此事件链仓内零先例 —— 形状以 RESEARCH Code Examples §3 为准，已对 v0.9.14 源码 + 官方 HITL 文档双源核验）。

**(3) 脚本化 ChatModel（第一轮回 tool call、resume 后回终答）** —— `internal/agent/agent_test.go:657-719` `multiTurnStubChatModel`：

```go
// agent_test.go:657-667 + 681-705（现有脚本化模型形状）：
type multiTurnStubChatModel struct {
    toolCallQuery string
    maxToolCalls  int
    finalMsg      *schema.Message
    calls       int
    mu          sync.Mutex
}
var _ model.ToolCallingChatModel = (*multiTurnStubChatModel)(nil)

func (m *multiTurnStubChatModel) Generate(ctx context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
    m.mu.Lock(); m.calls++; turnIdx := m.calls; m.mu.Unlock()
    if turnIdx <= m.maxToolCalls {
        return &schema.Message{
            Role: schema.Assistant,
            ToolCalls: []schema.ToolCall{{
                ID:   fmt.Sprintf("call_%d", turnIdx),
                Type: "function",
                Function: schema.FunctionCall{ Name: ..., Arguments: ... },
            }},
        }, nil
    }
    return m.finalMsg, nil
}
// Stream → return nil, fmt.Errorf("... not implemented")
// WithTools → return m, nil
```

（接口满足形状的另一先例：chat_session_test.go:66-81 `hangChatModel` —— 四方法齐全 + `var _ model.ToolCallingChatModel` 断言。）

**「进程重启」模拟 = 重开 DB + 新 Runner + 新 Agent 实例**（用户已授权此层级，Open Q2 推荐项）：file-backed t.TempDir DSN（同上 checkpoint_test.go 的重开手法）+ 第二个 `adk.NewRunner(RunnerConfig{Agent: agent2, CheckPointStore: ...})` + `runner2.ResumeWithParams(ctx, "test-cp-1", &adk.ResumeParams{Targets: map[string]any{interruptID: ...}})`（RESEARCH §3）。

**类型注册**：本 phase 的自定义状态类型（如测试用 `*ApprovalState`）在**其声明文件的 `init()`** 里调 `schema.RegisterName[T]("skillforge_...")` —— 不是裸 `gob.RegisterName`（Pitfall 1，最高优先）。测试文件内的声明就在测试文件 init() 注册。

---

### `integration/contract/no_legacy_interrupt_test.go`（NEW — test, repo 级禁令门）

**Analog:** `integration/contract/contract_test.go` 的版本门 + repo 根定位 helper

**构建标签（必须）**：同包所有测试带 `//go:build contract`（contract_test.go:17 先例）。**注意：RESEARCH Validation Architecture 表中的运行命令 `go test ./integration/contract -run TestNoLegacyInterruptAPI -v` 未带 `-tags=contract` —— planner 写任务动作时必须补 `-tags=contract`，否则测试根本不编译执行。** 包注释里的标准命令形态见 contract_test.go:47-48：`go test ./integration/contract -tags=contract -count=1 -timeout 10s -v`。

**断言形态**（contract_test.go:90-113 `TestContractVersionPin` 先例：os.ReadFile + strings.Contains + assert.True + 失败消息写明约束）：

```go
// contract_test.go:91-99（现有读文件断言形状）：
content, err := os.ReadFile(findGoMod(t))
if err != nil {
    t.Fatalf("read go.mod: %v", err)
}
body := string(content)
assert.True(t, strings.Contains(body, "github.com/cloudwego/eino v0.7.7"),
    "Version pin violated: ... must be v0.7.7 (got body without that substring)")
```

禁令门改成「命中即失败」：遍历 repo `.go` 源文件（**排除** module cache、`.claude/worktrees/*`、`web/`），对禁令清单 `[]string{"NewInterruptAndRerunErr", "compose.InterruptAndRerun", "deprecatedInterruptAndRerun"}`（`WrapInterruptAndRerunIfNeeded` 不禁 —— 官方迁移辅助；清单来自 RESEARCH Pitfall 5）逐文件 `strings.Contains`，命中即 `t.Fatalf("%s:%d 使用了被禁的 legacy interrupt API: %s", ...)`。

**repo 根定位**（contract_test.go:484-502 `findModuleRoot` 已存在且同包可直接复用 —— 不要重写；从根出发 `filepath.WalkDir` 下探扫描）。

**License header + 包名**：`package contract`（同包复用 helper），license 头照 contract_test.go:1-15。

**无先例部分**：目录递归遍历 + 排除清单是仓内测试首例 —— 实现按标准库 `filepath.WalkDir`，无既有代码可抄（见 No Analog Found 表）。

---

### `integration/contract/contract_test.go`（MODIFIED — TestContractVersionPin 改写）

**Analog:** 自身（原位改写，唯一被升级按设计弄红的测试 —— Pitfall 3：它是 fail-fast 门，改断言是显式任务，不是"绕过红测"）

**改动点 1 —— 断言字串**（contract_test.go:97-108）：

```go
// 现有（v0.7.7 时代，:98-99）：
assert.True(t, strings.Contains(body, "github.com/cloudwego/eino v0.7.7"), ...)
// 改为 v0.9.14；同理 :107-108 的 openai v0.1.5 → v0.1.13

// 原样保留（:101-103 hertz v0.10.6、:110-112 milvus-sdk-go/v2 v2.4.2 —— 本次不动）
```

**改动点 2 —— 注释块的 pin 理由**（contract_test.go:79-89 函数 doc + :105-107 行内注释）：`v0.7.7 时代的兼容性叙述（"newer v0.1.13+ requires eino v0.7.13+"）已过时 —— 按 v0.9.14 / v0.1.13 重写理由，注明用户 2026-08-17 决策覆盖 v0.9.13`。

**连带检查**：`TestContractRunsUnder10s`（contract_test.go:412-424）聚合了 4 个校验子测试；新禁令门是否纳入聚合由 planner 决定（禁令门是纯文件扫描、毫秒级，纳入不破 10s 预算；不纳入也独立可跑）。

---

### `go.mod` / `go.sum`（MODIFIED — 依赖升级）

**Analog:** 自身；无代码 pattern。要点：

```bash
# RESEARCH Standard Stack 的安装命令（go.sum 随之更新）：
go get github.com/cloudwego/eino@v0.9.14 github.com/cloudwego/eino-ext/components/model/openai@v0.1.13
go mod tidy
```

- direct 段两行手 bump（go.mod:6-7）；indirect 两个联动（`eino-ext/libs/acl/openai` v0.1.2→v0.1.17、`meguminnnnnnnnn/go-openai` v0.1.0→v0.1.2，go.mod:25/48）**由 `go mod tidy` 自动改，绝不手编 indirect**。
- hertz v0.10.6 / milvus v2.4.2 / modernc.org/sqlite v1.56.0 三行不动（go.mod:8,10,15）。
- **提交卫生（Pitfall 6）**：`git add` 只列明确路径（go.mod go.sum contract_test.go 及注释文件），工作树现有 M 状态文件与 untracked `scripts/test`（含 live key）绝不入提交。

---

### godoc 版本标注 ×8（MODIFIED — comment-only）

无 analog 需要。逐处把 `v0.7.7` 改 `v0.9.14` 并重核 file:line（升级后行号漂移，如 ToolCallingModel 文档 v0.9.14 在 react.go:139 附近 —— RESEARCH Pitfall 7）。已核实当前 8 处坐标（2026-08-17 本会话 grep）：

| 文件:行 | 当前形态 |
|---|---|
| `internal/agent/agent.go:52` | `eino@v0.7.7`（adk.Agent 接口不满足的实证注） |
| `internal/agent/agent.go:167` | `eino@v0.7.7/flow/agent/react/option_test.go:71` |
| `internal/agent/agent.go:226` | `eino@v0.7.7/flow/agent/react/react.go:50`（→ v0.9.14 约 :139） |
| `internal/agent/agent.go:236` | `eino@v0.7.7/compose/tool_node.go:121` |
| `internal/agent/callbacks.go:103` | `eino@v0.7.7/utils/callbacks/template.go:430-433` |
| `internal/agent/retrieve_tool.go:182` | `eino@v0.7.7/components/tool/utils/invokable_func.go:101-105` |
| `scripts/spiked/e2e.go:244` | `eino@v0.7.7/adk/call_option.go:54` |
| `scripts/spiked/wire.go:22` | 裸字串 `Eino v0.7.7 + eino-ext openai v0.1.5`（grep `eino@` 搜不到，须单独改） |

---

## Shared Patterns

### Apache-2.0 License Header
**Source:** `internal/store/sqlite/sqlite.go:1-15`（仓内全部 .go 文件统一形态）
**Apply to:** 所有 3 个新 .go 文件（checkpoint.go、checkpoint_test.go、checkpoint_resume_test.go、no_legacy_interrupt_test.go —— 共 4 个）。逐字复制，SPDX-License-Identifier 行结尾。

### 编译期接口断言（D18 interface-first）
**Source:** `internal/store/sqlite/sqlite.go:82` `var _ store.Store = (*SQLiteStore)(nil)`；同类先例 `internal/agent/agent.go:350`、`internal/agent/agent_test.go:667`
**Apply to:** checkpoint.go（`var _ compose.CheckPointStore = (*SQLiteStore)(nil)`）、resume 测试（`var _ model.ToolCallingChatModel = (*...Stub)(nil)`）。断言放 sqlite 包内，不放 `internal/store/store.go`（接口属 eino，RESEARCH Structure 注记）。

### 错误包装 fmtError
**Source:** `internal/store/sqlite/sqlite.go:162-170`
**Apply to:** checkpoint Get/Set 的所有非 ErrNoRows 错误路径。复用包级函数，方法名作第一参（"CheckpointGet" / "CheckpointSet"）。

### closed 守卫 → sql.ErrConnDone
**Source:** `internal/store/sqlite/session.go:43-45`（每个方法首行）
**Apply to:** checkpoint Get/Set（及可选 Delete）。

### SQL 全 `?` 参数化（T-3-SQLI）
**Source:** `internal/store/sqlite/audit.go:31-37`（putAuditSQL）+ store_test.go:151-185（注入回归测试）
**Apply to:** checkpoint 的全部 SQL；id 是 TEXT 主键也走参数化；禁令在于 `fmt.Sprintf` 拼 SQL。

### const SQL 字符串 + 每表一个 const
**Source:** `internal/store/sqlite/session.go:33-38,73-77`（getSessionSQL / appendTurnSQL）
**Apply to:** checkpoint.go 的 `getCheckpointSQL` / `setCheckpointSQL`。

### 测试 helper：newTestStore + t.Cleanup
**Source:** `internal/store/store_test.go:47-59`（in-memory 每测隔离 DSN）；file-backed 变体见上文 checkpoint_test.go 段
**Apply to:** 两个新测试文件。

### contract 包构建标签
**Source:** `integration/contract/contract_test.go:17` `//go:build contract` + :47-48 运行命令
**Apply to:** no_legacy_interrupt_test.go 必须带同 tag；**所有涉及该包的验证命令必须带 `-tags=contract`**（RESEARCH Validation 表的命令缺此 flag —— planner 修正）。

### 提交原子性（升级不混功能）
**Source:** RESEARCH Pattern 3 / Pitfall 2（提交序列 ①升级+断言改写+注释 → ③store → ④往返测试 → ⑤禁令 → ⑥human live 重锁）
**Apply to:** 所有 plan 的任务切分 —— 升级提交零功能代码，git add 显式列举路径。

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `checkpoint_resume_test.go` 的中断事件链处理 | test | event-driven | `ev.Action.Interrupted.InterruptContexts[0].ID` / `ResumeWithParams` 仓内零使用（全仓 interrupt API grep 零命中，RESEARCH 已证）。用 RESEARCH.md Code Examples §3 的骨架（已对 v0.9.14 源码 + 官方 HITL 文档双源核验）+ 本文件给出的三个仓内部分 analog（drain 循环 / 脚本模型 / Runner 接线）拼装 |
| `no_legacy_interrupt_test.go` 的目录递归扫描 | test | file-I/O | 仓内无「遍历 repo 源文件做字串禁令」的先例（VersionPin 只读单文件）。用标准库 `filepath.WalkDir` 从 `findModuleRoot`（contract_test.go:484，同包复用）出发，排除 `.claude/worktrees/`、`web/`、module cache |
| `schema.RegisterName[T]` 类型注册 | 初始化 | N/A | 仓内首次使用。形状：类型声明文件 `init()` 内 `schema.RegisterName[*T]("skillforge_稳定名")`（RESEARCH Code Examples §2；名字上线即不可变更）。**禁裸 `gob.RegisterName` 与 `compose.RegisterSerializableType`（已 Deprecated）** |

## Metadata

**Analog search scope:** `internal/store/`、`internal/store/sqlite/`、`internal/server/`、`internal/agent/`、`integration/contract/`、`cmd/server/`、`scripts/spiked/`、`go.mod`
**Files scanned:** 14（sqlite.go / schema.go / audit.go / session.go / store.go / store_test.go / contract_test.go / chat.go(部分) / chat_session_test.go / pipeline.go(部分) / agent.go / agent_test.go(部分) / main.go(部分) / wire.go(部分)+e2e.go(部分)）
**Grep 工具说明:** 本会话 Glob/Grep 专用工具环境故障，检索经 Bash grep 完成（结果等价，file:line 已逐处人工核验）
**Pattern extraction date:** 2026-08-17
