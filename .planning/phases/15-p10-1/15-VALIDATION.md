---
phase: 15
slug: p10-1
status: active
nyquist_compliant: true
wave_0_complete: false
created: 2026-08-23
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Test files are created **inside** the plans — Wave 0 gap list below tracks them.
> 任务编号已回填（2026-08-23，PLAN.md 定稿）；行为、命令与验收形状来自 15-RESEARCH.md Validation Architecture，执行期不得削弱。

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
- **Before `/gsd:verify-work`:** `bash scripts/verify-phase-15.sh` 全段 PASS（build+vet → race → 全量测试 → 分包锁名套件 → contract 门 → source-grep → 冻结字节段）
- **Max feedback latency:** 300 s

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 15-01-T3 | 15-01 | 1 | DATA-01 | —（解码 fail-closed） | CSV 四编码解码正确（UTF-8/BOM/GBK/GB18030 fixture 往返） | unit | `go test ./internal/data/ -run TestDecode -count=1` | ❌ W0 | ⬜ pending |
| 15-04-T1 | 15-04 | 2 | DATA-01 | dump 内嵌恶意 SQL | dump 接受白名单语句 + 拒 DROP/UPDATE/ATTACH | unit | `go test ./internal/data/ -run TestDump -count=1` | ❌ W0 | ⬜ pending |
| 15-06-T2 | 15-06 | 4 | DATA-01 | multipart 预解析写 OS 临时文件 | multipart 流式（>16MB part 不触 TempDir） | integration | `go test ./internal/server/ -run TestImportStreaming -count=1` | ❌ W0 | ⬜ pending |
| 15-01-T1 | 15-01 | 1 | DATA-02 | 哈希字典攻击（规范化面） | 规范化 join 恒等（张三/尾空格/+86/全角） | unit | `go test ./internal/data/ -run TestHMACNormalizationJoinability -count=1` | ❌ W0 | ⬜ pending |
| 15-01-T2 | 15-01 | 1 | DATA-02 | 规则漂移（fail-closed） | 未知列 fail-closed | unit | `go test ./internal/data/ -run TestUnknownColumnFailClosed -count=1` | ❌ W0 | ⬜ pending |
| 15-05-T3 | 15-05 | 3 | DATA-02/17 | raw 文件生命周期（Pitfall 13） | 全流程后磁盘无 raw（含 os.TempDir() 扫描） | integration | `go test ./internal/data/ -run TestRawNeverOnDisk -count=1` | ❌ W0 | ⬜ pending |
| 15-04-T2 | 15-04 | 2 | DATA-15 | 预览/提交竞态 | 哈希桥接（预览 A + 提交 B → 拒；单元面） | unit | `go test ./internal/data/ -run TestPreviewCommitHashBridge -count=1` | ❌ W0 | ⬜ pending |
| 15-05-T2 | 15-05 | 3 | DATA-15/16 | 预览/提交竞态 | 哈希桥接（importer 集成面）+ 过期会话拒绝 | integration | `go test ./internal/data/ -run 'TestPreviewCommitHashBridge|TestCommitSessionExpired' -count=1` | ❌ W0 | ⬜ pending |
| 15-05-T2 | 15-05 | 3 | DATA-16 | 重复提交副作用 | 同 (dataset,period) 同哈希 → 无副作用 | integration | `go test ./internal/data/ -run TestIdempotentReimport -count=1` | ❌ W0 | ⬜ pending |
| 15-03-T1 | 15-03 | 1 | DATA-16 | supersede 中断破坏旧数据 | supersede 注入失败旧数据完好 + 成功链可查 | integration | `go test ./internal/duckdb/ -run TestSupersedeAtomicSwap -count=1` | ❌ W0 | ⬜ pending |
| 15-02-T2 | 15-02 | 1 | DATA-17 | 审计缺行 | 批次审计行字段形状 | integration | `go test ./internal/store/sqlite/ -run TestBatchAuditRowShape -count=1` | ❌ W0 | ⬜ pending |
| 15-07-T1 | 15-07 | 5 | SC-1 | 逐行 INSERT 性能陷阱 | Appender 路径（无逐行 INSERT） | 既有门 + 新 grep 门 | `go test ./internal/duckdb/ -run TestDuckDBAppenderThroughput -count=1` + WalkDir 式禁令 | ✓ 基准门 / ❌ 禁令门 W0 | ⬜ pending |
| 15-01-T1 | 15-01 | 1 | D15-D1 | 桶边界漂移 | 桶边界跨期稳定 | unit | `go test ./internal/data/ -run TestBucketBoundaryStableAcrossPeriods -count=1` | ❌ W0 | ⬜ pending |
| 15-06-T3 | 15-06 | 4 | D15-C3 | 删除端点未授权 | 删除端点审计 + 注册表/文件一致 | integration | `go test ./internal/server/ -run TestDeleteDatasetAudited -count=1` | ❌ W0 | ⬜ pending |
| 15-01-T1 | 15-01 | 1 | D15-D1/Pitfall 4 | 盐作用域漂移 | 哈希跨 period 稳定（盐不随 period 变） | unit | `go test ./internal/data/ -run TestHashStableAcrossPeriods -count=1` | ❌ W0（RESEARCH 建议补充项） | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*测试名与 CONTEXT §Specific test lock requirements 一一对应；`TestRawNeverOnDisk` 必须扩展扫描 `os.TempDir()`（15-RESEARCH Pitfall 1：Hertz 默认 multipart >16MB 溢写临时文件）——库层扫描在 15-05-T3，HTTP 层 /tmp 面在 15-06-T2 TestImportStreaming，两层分工已在各自 plan 注明。*

---

## Wave 0 Requirements

- [ ] `internal/data/` 包骨架 + `normalize.go`（纯函数最先可测）→ **15-01-T1**（Wave 1）
- [ ] `.dump` ground-truth fixtures（`internal/data/testdata/*.sql`，sqlite3 3.53.0 实测输出）→ **15-04-T1**（Wave 2）
- [ ] 四编码 CSV fixtures（testdata，含 GBK/GB18030/BOM 各一）→ **15-01-T3**（Wave 1）
- [ ] grep/WalkDir 禁令门：禁逐行 INSERT、禁 MultipartForm/FormFile、禁行值日志 → **15-07-T1**（Wave 5；D8-04 形态）
- [ ] `scripts/verify-phase-15.sh` + Makefile `verify-phase-15` target → **15-07-T2**（Wave 5）

*框架/依赖零安装：Go testing + testify 既有；DuckDB Appender 基准门（Phase 14）既有。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| SKILLFORGE_DATA_SALT 运维配置（生成随机 hex 入 .env；变更 = 全量重导警示） | DATA-02 | 密钥生成是 operator 动作（15-06 user_setup 已列） | 启动服务 → 未设盐 curl 导入端点应 503 → 设盐后 push 应 200 |
| verify-phase-15.sh duckdb 段完整运行 | SC-1 | 主机 Windows emutls 限制（verify-phase-14.sh:17-22 先例）——段脚本须 WSL2/Linux 跑 | `bash scripts/verify-phase-15.sh`（WSL2/Linux）→ PASS 10/10；主机不可跑时按 14-08 blocked-with-notes 记录 |

*其余全部 phase 行为有自动化验证。*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references（五项全部分配到 15-01/15-04/15-07）
- [x] No watch-mode flags
- [x] Feedback latency < 300 s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** 2026-08-23 — planner 签发（7 plans 定稿、任务编号回填完毕；wave_0_complete 待 Wave 1 执行后翻转）
