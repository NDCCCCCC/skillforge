---
phase: 08-p8-0-eino-v0-7-7-v0-9-13
plan: 02
subsystem: persistence (sqlite CheckPointStore skeleton)
tags: [sqlite, checkpoint-store, tdd, eino-compose, ddl-additive, sqli-regression]
requires:
  - "eino v0.9.14 dependency base + compose.CheckPointStore interface shape confirmed (Phase 8 / Plan 08-01, commit 36845c0)"
  - "internal/store/sqlite SQLiteStore infrastructure: isClosed guard / fmtError / Config.withDefaults / migrate() (Phase 3 / Plan 03-02, D18)"
provides:
  - "SQLiteStore.Get/Set satisfying compose.CheckPointStore (== adk.CheckPointStore alias) — the store-layer substrate for Phase 11 HITL approval state surviving restarts"
  - "checkpoint table DDL with session_id/skill_id/expires_at reserved columns (Phase 11 lifecycle lands without ALTER)"
  - "SC-4 store-layer semantics locked by tests: byte-identical round-trip incl. non-UTF8, Close+reopen readback, upsert overwrite, unknown-id tri-state, DROP TABLE injection survival, closed guards"
affects:
  - "Phase 8 / Plan 08-04 (SC-5 real interrupt round-trip consumes this store via adk.Runner CheckPointStore field)"
  - "Phase 11 HITL (resume-data validation on top of opaque passthrough; lifecycle predicates on expires_at)"
tech-stack:
  added: []
  patterns:
    - "同库 additive 建表：DDL 追加进既有 schemaSQL const 尾部，幂等性由既有 migrate() 自动获得（零新迁移函数）"
    - "接口方法挂 SQLiteStore（PATTERNS 形态 A）+ 编译期断言 var _ compose.CheckPointStore（一个断言同时锁 compose 与 adk 两个 alias 消费面）"
key-files:
  created:
    - internal/store/sqlite/checkpoint.go
    - internal/store/sqlite/checkpoint_test.go
  modified:
    - internal/store/sqlite/schema.go
decisions:
  - "D8-03: CheckPointStore 实现形态 = SQLiteStore 上的 Get/Set 方法（非独立 CheckpointStore 类型），断言一个 compose.CheckPointStore 即同时锁 adk alias；DDL 本 phase 即带 session_id/skill_id/expires_at 预留列（Phase 11 免 ALTER）；不实现 Delete（CheckPointDeleter 留待 Phase 11 生命周期语义明确）；Get 不滤 expires_at（无写入方，全 NULL 等价于无过滤）"
metrics:
  duration: 6m
  completed: 2026-08-17
---

# Phase 8 Plan 02: sqlite CheckPointStore 骨架 Summary

**One-liner:** 既有 sqlite 库上 additive 建出 checkpoint 表 + SQLiteStore 的 Get/Set 方法（编译期断言 compose.CheckPointStore，== adk alias），TDD 两提交锁死六面语义：byte 级 round-trip（含非 UTF8）、Close 重开 DB 等值读回（「进程重启」store 层证明，SC-4）、upsert 覆盖、unknown-id 三态 `(nil,false,nil)`、DROP TABLE 注入存活、closed 守卫 `sql.ErrConnDone`。

## What Was Built

### Task 1: RED — checkpoint_test.go 六面断言先行（commit `9a557b1`）

`internal/store/sqlite/checkpoint_test.go`（185 行，`package sqlite_test` 外部黑盒，license header 1-15 行照 sqlite.go 模板）：

- **TestCheckpointRoundTrip**：file-backed `t.TempDir()` DSN → Set → Get 返回逐字节等值 bytes（payload 刻意含 `\x00\x01\xfe\xff` 非 UTF8 字节，证明 BLOB 透传不解析——T-8-05 accept 姿态的测试面）
- **TestCheckpointSurvivesReopen**：s1.Set → s1.Close()（「进程退出」）→ 同一 DSN `sqlite.New` → s2.Get 等值读回（existed=true）——SC-4「进程重启」store 层证明，Pitfall 4（内存 checkpoint 丢中断态）的反面锚点
- **TestCheckpointUpsertOverwrites**：同 id 两次 Set → Get 返回第二个 payload（再中断的会话不得复活旧 checkpoint）
- **TestCheckpointUnknownID**：Get 未 Set 过的 id → `(nil, false, nil)`——existed=false 是三态答案不是错误
- **TestCheckpointSQLInjection**：id 传 `x'; DROP TABLE checkpoint; --` → Get/Set 均无 SQL 语法错，事后表仍可读写（种子行 + 注入字串作为字面 row key 双向读回）——T-8-04 / T-3-SQLI posture 延续
- **TestCheckpointClosedStore**：Close 后 Get 与 Set 均 `require.ErrorIs(err, sql.ErrConnDone)`

辅助面：`newCheckpointStore`（in-memory shared-cache DSN，镜像 store_test.go:47-59）+ `openCheckpointStore(t, dsn)`（file-backed 变体；Close 幂等使 t.Cleanup 在显式 Close 后是安全 no-op）。RED 态 = 包编译失败（`s.Get`/`s.Set` undefined），`go vet ./internal/store/sqlite` 同样报 undefined——Go TDD 合法红态，测试语义即 SC-4 store 层规格。

### Task 2: GREEN — schema.go DDL 追加 + checkpoint.go 实现（commit `e6a580a`）

**(1) schema.go**（+23/−3）：
- `schemaSQL` 常量字符串**尾部**（`idx_session_turn_sid` 之后、闭合反引号之前）追加 checkpoint 表 DDL——同库 additive，幂等性由既有 `migrate()` 自动获得，零新迁移函数；audit / session_turn 两表 DDL 逐字未动
- 列结构：`id TEXT PRIMARY KEY` / `session_id` / `skill_id`（DEFAULT ''，Phase 11 绑会话预留）/ `payload BLOB NOT NULL` / `created_at`（strftime 默认）/ `expires_at`（NULL，Phase 11 TTL 预留）——本 phase 即带齐避免后续 ALTER
- 顶部注释块补 checkpoint 表说明段（INSERT+SELECT 透传 bytes、唯一写入是 id 键 upsert、expires_at 本 phase 不写不滤）

**(2) checkpoint.go**（87 行，新建）：
- `var _ compose.CheckPointStore = (*SQLiteStore)(nil)` 编译期断言（接口属 eino，断言放 sqlite 包内；compose 与 adk 的 CheckPointStore 是同一 core 接口 alias，断言一个锁两个消费面）
- `Get(ctx, checkPointID) ([]byte, bool, error)`：isClosed 守卫 → `(nil, false, sql.ErrConnDone)`；`const getCheckpointSQL = "SELECT payload FROM checkpoint WHERE id = ?"`（**不滤 expires_at**——无写入方，全 NULL 等价无过滤）；`errors.Is(err, sql.ErrNoRows)` → `(nil, false, nil)`；其余 `fmtError("CheckpointGet", err)`
- `Set(ctx, checkPointID, checkPoint) error`：isClosed 守卫 → `sql.ErrConnDone`；`const setCheckpointSQL` = `INSERT ... ON CONFLICT(id) DO UPDATE SET payload = excluded.payload`；ExecContext 全 `?` 参数化
- **不实现 Delete**（不在任何 SC 内，Phase 11 落 CheckPointDeleter）；零 `fmt.Sprintf`；零裸 `gob.RegisterName`（注册属 08-04 测试类型）

## Must-Haves Truths 验证

| Truth | 状态 | 证据 |
|-------|------|------|
| checkpoint 表随既有 migrate() 幂等建出（同库 additive，audit/session_turn 零触碰） | ✅ | schema.go DDL 追加进 schemaSQL 尾部；`grep CREATE TABLE IF NOT EXISTS audit` = 1 不变；`go test ./internal/store/... -count=1` 全绿（audit/session 零回归） |
| Get/Set round-trip：bytes 写入后读回逐字节等值 | ✅ | TestCheckpointRoundTrip PASS（含非 UTF8 字节） |
| 「进程重启」store 层证明：Close 后重开同一 DB 文件等值读回 | ✅ | TestCheckpointSurvivesReopen PASS（file-backed t.TempDir DSN） |
| 未知 id Get 返回 (nil, false, nil) | ✅ | TestCheckpointUnknownID PASS |
| 注入 payload 存活——全 `?` 参数化 | ✅ | TestCheckpointSQLInjection PASS；checkpoint.go 零 fmt.Sprintf，两条 const SQL 仅 `?` 占位 |

## Verification（plan `<verification>` 四项）

| # | 命令 | 结果 |
|---|------|------|
| 1 | `go test ./internal/store/sqlite -run TestCheckpoint -v -count=1` | 六面全 `--- PASS`（0.259s） |
| 2 | `go test ./internal/store/sqlite -count=1`（+ `./internal/store/...` 加跑） | 全绿（0.212s / 0.194s——schema 共享面 audit/session 零回归） |
| 3 | `go build ./... && go vet ./...` | exit 0（gofmt -l 空） |
| 4 | `git log --oneline -2` | `9a557b1` RED → `e6a580a` GREEN 两提交分离 |

验收 grep 全过：断言 =1、`func (s *SQLiteStore) Get(`/`Set(` 各 =1、checkpoint DDL =1、audit DDL =1、checkpoint.go fmt.Sprintf =0、RED 提交只含 checkpoint_test.go（185 行）、GREEN 提交只含 schema.go + checkpoint.go（+107/−3）。

## Deviations from Plan

**1. [Rule 1 - 注释准确性] schema.go audit 注释一句的范围收窄**
- **Found during:** Task 2 步骤 (1)
- **Issue:** 既有注释称「there is NO row-modification path **anywhere in this package**」，而 checkpoint 的 `ON CONFLICT(id) DO UPDATE` 恰是包内新的行修改路径——保留原句会留下事实错误的文档（与 plan「补一段 checkpoint 表说明」的意图冲突）。
- **Fix:** 最小改动：「anywhere in this package」→「for audit rows」（T-3-AUDIT 不变量的真实保护对象不变），同步「two tables」→「three tables」。audit/session_turn 两表 **DDL** 仍逐字未动。
- **Files modified:** internal/store/sqlite/schema.go
- **Commit:** e6a580a（GREEN 同一原子提交，属本任务改写面）

其余：plan executed exactly as written（方法名 Get/Set 按 planner 裁决；不滤 expires_at 按 plan；Delete 未实现按 plan）。

## 提交卫生（T-8-04 / Pitfall 6 延续）

- RED 提交 `git show --stat` = 仅 checkpoint_test.go；GREEN 提交 = 仅 schema.go + checkpoint.go
- 零文件删除（diff-filter=D 空）；untracked `.claude/`、`scripts/test`（含 live key）、`screenshots/`、`du.exe.stackdump`、`07-REVIEW.md` 未入任何提交
- 零新依赖（复用 modernc.org/sqlite v1.56.0 + eino v0.9.14 既有 go.mod——T-8-SC 不适用面维持）

## Auth Gates

None — 纯 hermetic store 层，无 live stack 依赖（live 重锁属 08-05）。

## Known Stubs

None — 无 UI / 无占位数据。session_id / skill_id / expires_at 三列是**有意的 Phase 11 预留**（非 stub：本 phase 无任何写入方或过滤方，行为完全由六面测试锁定）。

## Threat Flags

None — 无新增安全面。威胁登记表 T-8-04（mitigate：全 `?` 参数化 + DROP TABLE 存活断言）已按 plan 落实；T-8-05 / T-8-06（accept）维持 store 层透传不解析 + 无 HTTP 暴露姿态。

## Self-Check: PASSED

- 文件存在性：internal/store/sqlite/checkpoint.go / checkpoint_test.go / schema.go — 全部 FOUND（git tracked @ e6a580a / 9a557b1）
- 提交存在性：`9a557b1`（RED）+ `e6a580a`（GREEN）FOUND（git log 验证）
- 四项 verification 输出留存于本文件「Verification」表
