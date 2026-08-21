---
phase: 14
slug: p10-0-cgo-trust-floor
status: planned
nyquist_compliant: true
wave_0_complete: false
created_at: 2026-08-22
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Test files are created **inside** the plans (TDD tasks) — Wave 0 gap list below tracks them.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test（Go stdlib `testing` + 内部断言式 benchmark；无新增框架） |
| **Config file** | none — Makefile + scripts/verify-phase-14.sh 为 gate 入口 |
| **Quick run command** | `go build ./... && go vet ./... && go test ./... -count=1`（CGO 波之前缺 gcc 会失败——地板波先行；14-03 起 `make check-cgo` 为前置） |
| **Full suite command** | `bash scripts/verify-phase-14.sh`（9 段 hermetic 验收门，14-07 落地） |
| **Estimated runtime** | ~180–300 s（含 Appender 50 万行 + 50 并发读基准段） |

---

## Sampling Rate

- **After every task commit:** `go build ./... && go vet ./... && go test <受影响包>/... -count=1`
- **After every plan wave:** `go test ./... -count=1`；Wave 6 起加 `bash scripts/verify-phase-14.sh`
- **Before `/gsd:verify-work`:** verify-phase-14.sh 9/9 PASS + 冻结锁字节一致 + 重锁 sha256 留档
- **Max feedback latency:** 300 s

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | PLAT-09 | T-14-01/02/03 | 地板合成唯一出口；hash 覆盖地板版本 | unit | `go test ./internal/agent/ -run 'TestPromptHashCoversFloor\|TestComposePlatformPrompt' -count=1` | ❌ W0 | ⬜ pending |
| 14-01-02 | 01 | 1 | PLAT-09 | T-14-01 | 双路径 prepend + 不可遮蔽 + 热重载存活 | unit | `go test ./internal/agent/ ./internal/runtime/ -run 'TestPlatformFloor\|TestFloorHotReload' -count=1` | ❌ W0 | ⬜ pending |
| 14-01-03 | 01 | 1 | PLAT-09 | T-14-02 | 全仓无绕过合成出口路径 | unit (source-scan) | `go test ./integration/contract/ -run TestNoBypassOfPlatformFloorCompose -count=1 -tags=contract` | ❌ W0 | ⬜ pending |
| 14-02-01 | 02 | 2 | PLAT-10 | T-14-05 | audit 行携带版本列；DDL 常量无注入面 | unit | `go test ./internal/store/... -run 'TestAuditPlatformFloorVersionColumn' -count=1` | ❌ W0 | ⬜ pending |
| 14-02-02 | 02 | 2 | PLAT-10 | T-14-04/06 | admin 预览 token 门（403/404/200） | unit | `go test ./internal/server/ -run 'TestPlatformPromptPreview\|TestAdminToken' -count=1` | ❌ W0 | ⬜ pending |
| 14-03-01 | 03 | 3 | PLAT-10 | T-14-SC | mattn 链接 + DSN pragma 迁移 | build gate | `make check-cgo && go build ./... && go vet ./... && go test ./internal/store/... -count=1` | ❌ W0 | ⬜ pending |
| 14-03-02 | 03 | 3 | PLAT-10 | T-14-07/08 | busy_timeout 并发等待非 BUSY；:memory: 防线 | unit | `go test ./internal/store/ -run TestSqliteMattnZeroRegression -count=1 -timeout 120s` | ❌ W0 | ⬜ pending |
| 14-04-01 | 04 | 2 | PLAT-11 | T-14-09 | 20 例 schema 门（六面 × 铁律映射） | unit | `go test ./internal/eval/ -run TestPlatformFloorGoldenSchema -count=1` | ❌ W0 | ⬜ pending |
| 14-04-02 | 04 | 2 | PLAT-11 | T-14-09/10/11 | hermetic 正则判定双向 + 锁自洽 | unit | `go test ./internal/eval/ -run 'TestPlatformFloorFrozenSet\|TestPlatformFloorLockSelfConsistent' -count=1` | ❌ W0 | ⬜ pending |
| 14-05-01 | 05 | 4 | PLAT-09 | T-14-12/13/14 | boot-queries 每连接重放 + WAL 刷盘 + ATTACH 标识符校验 | unit (CGO) | `go test ./internal/duckdb/ -run 'TestDuckDBBootQueries\|TestDuckDBCloseFlushesWAL\|TestValidateDatasetID' -count=1 -timeout 120s` | ❌ W0 | ⬜ pending |
| 14-05-02 | 05 | 4 | PLAT-09 | T-14-14 | Appender ≥10 万行/s + 50 并发读不阻塞写 | unit-bench | `go test ./internal/duckdb/ -run 'TestDuckDBAppenderThroughput\|TestDuckDBConcurrentReadNoBlockWrite' -count=1 -timeout 300s` | ❌ W0 | ⬜ pending |
| 14-06-01 | 06 | 5 | PLAT-09 | T-14-17 | 懒 ATTACH LRU（cap/touch/非法 id/幂等 DETACH） | unit (CGO) | `go test ./internal/duckdb/ -run 'TestAttachLRU' -count=1 -timeout 120s` | ❌ W0 | ⬜ pending |
| 14-06-02 | 06 | 5 | PLAT-09 | T-14-15/16 | staging 原子改名四面 + 注册表参数化 | unit (CGO) | `go test ./internal/duckdb/ -run TestStagingAtomicRename -count=1 -timeout 120s && go test ./internal/store/... -count=1` | ❌ W0 | ⬜ pending |
| 14-07-01 | 07 | 6 | PLAT-11 | T-14-18 | compliance/hitl 重锁只动 pin 字段 | gate + 手工记录 | `go test ./internal/eval/ -run 'TestAssertPin\|TestFrozen' -count=1 && git diff --name-only eval/frozen/` | ✅ 既有框架 | ⬜ pending |
| 14-07-02 | 07 | 6 | PLAT-11 | T-14-19 | 9 段验收门全绿（含 -race + 冻结字节段） | gate | `bash scripts/verify-phase-14.sh` | ❌ W0 | ⬜ pending |
| 14-08-01 | 08 | 7 | PLAT-11 | T-14-20 | Dockerfile CGO 翻转 | grep gate | `grep -c "CGO_ENABLED=1" Dockerfile.backend && grep -c build-base Dockerfile.backend` | ✅ Dockerfile 既有 | ⬜ pending |
| 14-08-02 | 08 | 7 | PLAT-11 | T-14-20 | Docker CGO 构建可运行 | manual-only（operator gate） | `docker build -f Dockerfile.backend .` + 冒烟 | ❌ checkpoint | ⬜ pending |
| 14-08-03 | 08 | 7 | PLAT-11 | T-14-21 | live judge 20 例 + 指标回填签名 | manual-only（operator gate） | _live_runbook 留档于 platform-floor.lock.json | ❌ checkpoint | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/agent/platform_floor_test.go` — TestComposePlatformPrompt / TestPromptHashCoversFloor（14-01 T1 RED）
- [ ] `internal/agent/platform_floor_test.go` 扩展 — PrependBothPaths / CannotBeShadowed / FloorHotReloadSurvives（14-01 T2）
- [ ] `integration/contract/platform_floor_bypass_test.go` — TestNoBypassOfPlatformFloorCompose（14-01 T3）
- [ ] `internal/store/sqlite/audit_test.go` 扩展 — TestAuditPlatformFloorVersionColumn（14-02 T1）
- [ ] `internal/server/platform_prompt_test.go` — TestPlatformPromptPreview（14-02 T2）
- [ ] `internal/store/sqlite/mattn_zero_regression_test.go` — TestSqliteMattnZeroRegression（14-03 T2）
- [ ] `internal/eval/platform_floor_judge_test.go` — TestPlatformFloorFrozenSet + GoldenSchema + LockSelfConsistent（14-04）
- [ ] `internal/duckdb/connector_test.go` — BootQueries / CloseFlushesWAL / ValidateDatasetID / AppenderThroughput / ConcurrentReadNoBlockWrite（14-05）
- [ ] `internal/duckdb/attach_lru_test.go` + `staging_test.go` — TestAttachLRU* / TestStagingAtomicRename（14-06）
- [ ] `scripts/verify-phase-14.sh` + Makefile `verify-phase-14`（14-07 T2）

*框架/依赖零安装：Go testing + 既有 store 六面套件 + eval frozen 框架 + run_stage 脚本模式全部现成。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Docker CGO 构建成功 + 镜像可运行 | PLAT-11（SC-1 Docker 半） | Docker 自 Phase 7/13 不在 PATH | `docker build -f Dockerfile.backend .` + `docker run --rm <img> --help`（无 exec format error）；不可用则 blocked-with-notes（14-08 Task 2） |
| live judge 层 20 例「部分服从」扫描 | PLAT-11（SC-5 live 半） | 需 GLM_API_KEY 真实栈 | 按 platform-floor.lock.json `_live_runbook` 执行；指标 + 签名回填（14-08 Task 3） |
| compliance/hitl live 指标重跑 | PLAT-11（D14-B4 live 半） | 同上 | 既有冻结 eval 流程重跑两集，超阈值则记录并评估 bump（14-08 Task 3 步骤 4） |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references（全部 ❌ W0 项均由对应 plan 的 TDD 任务创建）
- [x] No watch-mode flags
- [x] Feedback latency < 300 s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending（plans 2026-08-22 创建；随 execute-phase 逐 wave 转绿）
