---
phase: 16-p10-2
plan: 03
status: done
tasks_completed: 2
commits: [b29090f, 1a9e08e]
requirements: [KPI-02]
---

# Plan 16-03 Summary — kpi_calls INSERT-only 审计表层（KPI-02 / D16-D4）

每次 KPI 调用全量冻结为 kpi_calls 表一行：`PutKPICall` 单 INSERT 写路径 + 无 UPDATE/DELETE source-grep 结构锁 + ADM-03 SQL 直查面（List/Latest），import_batches 先例整文件复刻换列集。

## Tasks Completed

### Task 1: KPICallRow + kpi_calls DDL（additive）+ PutKPICall INSERT 写路径 + TestKPICallsDDL

**Files changed:**
- `internal/store/sqlite/schema.go` — schemaSQL 尾部追加 kpi_calls 建表块（10 列：id/ts/session_id/skill_id/func/args_json/as_of_period/k_min_applied/result_snapshot/signature DEFAULT ''），注释块照 import_batches 叙事写 rationale（SEPARATE INSERT-only / 大 JSON 快照不混 chat 审计主表 T-16-03-03 / ADM-03 直查无 JOIN / KPI-02 数据清理后回放语义）；三个索引 idx_kpi_calls_func / idx_kpi_calls_session_id / idx_kpi_calls_ts 作独立 const + migrate() 尾部 ExecContext（D14-E1 形态纪律，全新表无 ALTER 依赖但保持同一形态）
- `internal/store/store.go` — `type KPICallRow struct`（11 字段每字段一句语义注释，ImportBatchRow 密度）+ PutKPICall 接口方法声明
- `internal/store/sqlite/kpi_calls.go` [新] — putKPICallSQL（9 列全 ? 占位，列序与 DDL 一一对应）+ `func (s *SQLiteStore) PutKPICall`（ExecContext + UTC RFC3339Nano + fmtError 包装，逐行复刻 PutImportBatch）；全文件零行变异/行移除动词
- `internal/store/sqlite/kpi_calls_test.go` [新] — TestKPICallsDDL 三段断言：fresh 文件库 PRAGMA table_info 10 列名/顺序逐列匹配 + 三索引 sqlite_master 存在 + migrate() 重跑幂等；旧库升级路径复用 seedLegacyDatasetsDB 构造不含 kpi_calls 的 pre-Phase-16 库 → New() 补建表+索引且 legacy datasets 行存活（additive 幂等）

**Verification:** `go test ./internal/store/sqlite/ -run 'TestKPICallsDDL' -count=1 -v` PASS；`grep -ciE "UPDATE |DELETE " internal/store/sqlite/kpi_calls.go` = 0
**Commit:** `b29090f`

### Task 2: 只读查询面 ListKPICalls/LatestKPICallByFunc + TestKPICallsInsertOnly 结构锁

**Files changed:**
- `internal/store/sqlite/kpi_calls.go` — listKPICallsBaseSQL 全列 SELECT + kpiCallsClauseFunc/kpiCallsClauseSessionID 硬编码子句 const（strings.Join 组合 WHERE，值全 ? 绑定——QueryAudit 同款 posture，T-16-03-02）+ ORDER BY id DESC LIMIT ? 尾部 + `ListKPICalls`（QueryContext 扫描回 KPICallRow，RFC3339Nano 解析）；latestKPICallByFuncSQL 一条 GROUP BY 聚合 + `LatestKPICallByFunc` 扫描 map[string]time.Time
- `internal/store/store.go` — KPICallsFilter 类型（Func/SessionID/Limit，零值=不筛，Limit 默认 100）+ ListKPICalls/LatestKPICallByFunc 接口方法
- `internal/store/sqlite/kpi_calls_test.go` — TestKPICallsInsertOnly 双段：①source-grep 段读 kpi_calls.go 源码断言 "UPDATE "/"DELETE " 缺席（结构性防篡改锁 T-16-03-01）；②round-trip 段写 3 行（跨 func/session、ResultSnapshot 含 suppressed 标记行真实形态 JSON、纳秒精度时间戳）→ 零 filter 断言 3 行 newest-first 逐字段回读相等（ArgsJSON/ResultSnapshot 字节相等 + TS 纳秒保真）→ Func 过滤 → SessionID 过滤 → 组合过滤 → LatestKPICallByFunc 每 func MAX 与写入时刻一致 → 空表返回空 map

**Verification:** `go test ./internal/store/sqlite/ -run 'TestKPICallsDDL|TestKPICallsInsertOnly|TestKPICallsList' -count=1 -v` PASS；`go test ./internal/store/... -count=1` 两包全绿零回归；grep = 0
**Commit:** `1a9e08e`

## Acceptance Criteria Status

| Criterion | Status |
|-----------|--------|
| schema.go 含 `CREATE TABLE IF NOT EXISTS kpi_calls` 与三个 idx_kpi_calls_* 独立 const | PASS |
| kpi_calls.go 含 PutKPICall 与 `INSERT INTO kpi_calls`；`grep -ciE "UPDATE \|DELETE "` 为 0 | PASS |
| store.go 含 `type KPICallRow struct` 与接口方法三签名 | PASS |
| TestKPICallsDDL 断言 10 列列序逐列匹配 + 三索引存在 + 旧库 additive 补建 | PASS |
| kpi_calls.go 含 ListKPICalls/LatestKPICallByFunc 与 GROUP BY func | PASS |
| TestKPICallsInsertOnly source-grep 段 + round-trip 段全断言 | PASS |
| `go test ./internal/store/... -count=1` 全量 exit 0（既有套件零回归） | PASS |
| Windows hermetic（mattn/go-sqlite3 经 MSYS2 gcc v16.1.0 可链接） | PASS |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] List/Latest 接口签名从 Task 1 顺延至 Task 2**
- **Found during:** Task 1
- **Issue:** 计划原文要求 Task 1 在 store.go 追加全部三个接口签名，但其实现分属 Task 2——中间态会破坏 `var _ store.Store = (*SQLiteStore)(nil)` 编译期一致性检查，Task 1 自身 verify（go test ./internal/store/sqlite/）无法通过
- **Fix:** Task 1 只加 PutKPICall 签名；KPICallsFilter + List/Latest 签名随其实现落入 Task 2 提交。最终树与计划目标完全一致，每个提交独立可编译
- **Files modified:** internal/store/store.go
- **Commit:** 1a9e08e

**2. [Rule 1 - Bug] LatestKPICallByFunc 用 MAX(id) 替代计划原文 MAX(ts)**
- **Found during:** Task 2
- **Issue:** RFC3339Nano 会裁剪尾随小数零："…T10:00:00Z"（零纳秒）与同秒更晚的 "…T10:00:00.25Z" 文本比较时 'Z'(0x5A) > '.'(0x2E)，MAX(ts) 会错报后写入者为较早行——计划要求「注释写明此前提」的前提实际不成立
- **Fix:** `WHERE id IN (SELECT MAX(id) FROM kpi_calls GROUP BY func)`——AUTOINCREMENT 保证 id 与插入序严格单调（即表中回放序），无文本比较边界；仍是单条 GROUP BY 聚合，满足 must-have「最后调用时间一条 GROUP BY 可得」；注释记录成因
- **Files modified:** internal/store/sqlite/kpi_calls.go
- **Commit:** 1a9e08e

### Shared-state note (not a code deviation)

STATE.md / ROADMAP.md 在本会话启动时已带未提交 WIP（orchestrator 的 Phase 16 execution-started 状态转换）。dbf8609 元数据提交在添加本计划的 decisions/progress 行时一并捕获了这些先前未提交但合法且自洽的状态推进（Plan: 1 of 9 EXECUTING 等），使共享状态树一致化；未触碰任何 internal/kpi WIP 或其他代码文件。

## Out-of-Scope Discoveries (recorded in deferred-items.md)

1. `internal/kpi/kmin.go:124/:127` 引用未定义的 jsonNumber / errNonNumericCount —— 并行 16-05/16-06 会话未提交 WIP 所有权，本计划按指示未触碰；当前仓内零包 import internal/kpi，不影响任何已提交消费者。
2. `go build ./...` 主包链接失败 —— cmd/server+cmd/eval 在 MSYS2 gcc 下链 duckdb 静态库失败，D15-01-1 已记录的环境限制（WSL2 跑）。16-03 采用 scoped 门：`go build`（./internal/... 除 kpi）PASS + `go test ./internal/store/...` PASS。

## Verification Summary

- 新增测试：TestKPICallsDDL、TestKPICallsInsertOnly（hermetic，Windows 本机 PASS）
- 回归：`go test ./internal/store/... -count=1` 全绿；`go vet ./internal/store/...` 干净
- 编译面：store 接口扩展对全部消费者编译通过（scoped build ./internal/... 除 kpi WIP）
