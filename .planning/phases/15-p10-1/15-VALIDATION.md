---
phase: 15
slug: p10-1
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-23
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Test files are created **inside** the plans — Wave 0 gap list below tracks them.
> 任务编号（Task ID / Plan / Wave 列）在 PLAN.md 定稿后回填；行为、命令与验收形状来自 15-RESEARCH.md Validation Architecture，执行期不得削弱。

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go `testing` + stretchr/testify v1.11.1（仓内既有，零新增依赖） |
| **Config file** | 无独立配置（Go 惯例；CGO 门 = Makefile `check-cgo`） |
| **Quick run command** | `go test ./internal/data/... ./internal/duckdb/... ./internal/store/sqlite/... -count=1` |
| **Full suite command** | `go test ./... -count=1`（Makefile `test-unit`）；phase gate = `bash scripts/verify-phase-15.sh`（run_stage 形态复刻 verify-phase-14.sh） |
| **Estimated runtime** | quick ~30 s；full ~180–300 s（含 race 段 + Appender 基准段 + grep/WalkDir 禁令段） |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/data/... ./internal/duckdb/... ./internal/store/sqlite/... -count=1`（新包测试保持 hermetic，<30 s 目标）
- **After every plan wave:** `go test ./... -count=1` + `cd web && npx tsc --noEmit`（无 FE 改动，守既有 0 错误基线）
- **Before `/gsd:verify-work`:** `bash scripts/verify-phase-15.sh` 全段 PASS（build+vet → race → 全量测试 → 冻结字节段 → Appender 基准段 → WalkDir/grep 禁令段 → Makefile target）
- **Max feedback latency:** 300 s

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 15-XX-YY（回填） | — | — | DATA-01 | —（dump 文法 / 解码 fail-closed） | CSV 四编码解码正确（UTF-8/BOM/GBK/GB18030 fixture 往返） | unit | `go test ./internal/data/ -run TestDecode -count=1` | ❌ W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | DATA-01 | dump 内嵌恶意 SQL | dump 接受白名单语句 + 拒 DROP/UPDATE/ATTACH | unit | `go test ./internal/data/ -run TestDump -count=1` | ❌ W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | DATA-01 | multipart 预解析写 OS 临时文件 | multipart 流式（>16MB part 不触 TempDir） | integration | `go test ./internal/server/ -run TestImportStreaming -count=1` | ❌ W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | DATA-02 | 哈希字典攻击（规范化面） | 规范化 join 恒等（张三/尾空格/+86/全角） | unit | `go test ./internal/data/ -run TestHMACNormalizationJoinability -count=1` | ❌ W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | DATA-02 | 规则漂移（fail-closed） | 未知列 fail-closed | unit | `go test ./internal/data/ -run TestUnknownColumnFailClosed -count=1` | ❌ W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | DATA-02/17 | raw 文件生命周期（Pitfall 13） | 全流程后磁盘无 raw（含 os.TempDir() 扫描） | integration | `go test ./internal/data/ -run TestRawNeverOnDisk -count=1` | ❌ W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | DATA-15 | 预览/提交竞态 | 哈希桥接（预览 A + 提交 B → 拒） | unit | `go test ./internal/data/ -run TestPreviewCommitHashBridge -count=1` | ❌ W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | DATA-16 | 重复提交副作用 | 同 (dataset,period) 同哈希 → 无副作用 | integration | `go test ./internal/data/ -run TestIdempotentReimport -count=1` | ❌ W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | DATA-16 | supersede 中断破坏旧数据 | supersede 注入失败旧数据完好 + 成功链可查 | integration | `go test ./internal/duckdb/ -run TestSupersedeAtomicSwap -count=1` | ❌ W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | DATA-17 | 审计缺行 | 批次审计行字段形状 | integration | `go test ./internal/store/sqlite/ -run TestBatchAuditRowShape -count=1` | ❌ W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | SC-1 | 逐行 INSERT 性能陷阱 | Appender 路径（无逐行 INSERT） | 既有门 + 新 grep 门 | `go test ./internal/duckdb/ -run TestDuckDBAppenderThroughput -count=1` + WalkDir 式禁令 | ✓ 基准门 / ❌ 禁令门 W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | D15-D1 | 桶边界漂移 | 桶边界跨期稳定 | unit | `go test ./internal/data/ -run TestBucketBoundaryStableAcrossPeriods -count=1` | ❌ W0 | ⬜ pending |
| 15-XX-YY（回填） | — | — | D15-C3 | 删除端点未授权 | 删除端点审计 + 注册表/文件一致 | integration | `go test ./internal/server/ -run TestDeleteDatasetAudited -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*测试名与 CONTEXT §Specific test lock requirements 一一对应；`TestRawNeverOnDisk` 必须扩展扫描 `os.TempDir()`（15-RESEARCH Pitfall 1：Hertz 默认 multipart >16MB 溢写临时文件）。*

---

## Wave 0 Requirements

- [ ] `internal/data/` 包骨架 + `normalize.go`（纯函数最先可测——normalize/HMAC/桶化为无 IO 纯函数）
- [ ] `.dump` ground-truth fixtures（`internal/data/testdata/*.sql`）——用 15-RESEARCH Code Examples 的 sqlite3 3.53.0 实测输出直接落盘；再生成命令：`sqlite3 t.db "CREATE...; INSERT..."; sqlite3 t.db .dump`
- [ ] 四编码 CSV fixtures（testdata，含 GBK/GB18030/BOM 各一）
- [ ] grep/WalkDir 禁令门：导入代码禁逐行 INSERT、禁 MultipartForm/FormFile、import_api 禁行值日志（复刻 D8-04 形态）
- [ ] `scripts/verify-phase-15.sh` + Makefile `verify-phase-15` target

*框架/依赖零安装：Go testing + testify 既有；DuckDB Appender 基准门（Phase 14）既有。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| （无——本 phase 全 hermetic，无内网服务依赖、无 Docker 翻转、无 live eval） | — | — | — |

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 300 s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending（PLAN.md 定稿并回填任务编号后签发）
