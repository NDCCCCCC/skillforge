# STRIDE-14 — Phase 14 (P10.0 — CGO Trust Floor) Threat Model

**Phase:** 14 (P10.0 — CGO 信任地板 — 平台地板 + PLAT-10 溯源 + CGO 地基)
**Version:** 1.0
**Last updated:** 2026-08-22
**Owner:** Platform Engineering + Internal Audit Specialist
**Status:** Accepted at Phase 14 hermetic acceptance gate (14-07). Live
operator gates (14-08: Docker CGO build + GLM live judge) close the phase.

---

## 1. Scope & Trust Boundaries

Phase 14 establishes the platform-level trust floor and the CGO data-support
foundation: a single canonical system-prompt composition (`internal/platformfloor`)
that no Skill can shadow or bypass, audit-row provenance for the floor version
(PLAT-10), the mattn/go-sqlite3 + go-duckdb CGO toolchain, the DuckDB hermetic
connector with lazy ATTACH LRU and atomic staging commits, and the
platform-floor frozen eval set. The trust boundary crossings are:

| Boundary | Description |
|----------|-------------|
| Skill 模板 → 系统提示词合成 | Skill-authored prompt text enters the composed prompt BELOW the platform floor; composition order is code, not config (D14-A4) |
| admin 预览端点 → 合成 prompt 全文 | `GET /api/admin/platform-prompt` behind X-Admin-Token (ConstantTimeCompare, default-deny) |
| 数据集 ID/路径 → ATTACH 语句 | Externally supplied identifiers reach SQL text at exactly one site (AttachDataset), behind the allowlist |
| DuckDB 引擎 → 文件系统/网络 | Extension auto-install/auto-load could reach the network; disabled per-connection via boot-queries |
| 导入批次 → 文件系统 | staging rename path derived from batch id; datasets/ invariant = complete AND registered |
| LRU 逐出 / TTL 清理 → OS 文件锁 | DETACH must precede any os.Remove (Windows handle boundary) |
| 冻结锁 → 回归基线 | Re-locking is a recorded action (T-8-12); byte-identity verified by the Phase 14 gate Stage 09 |
| gate 脚本 → CI/验收 | verify-phase-14.sh is the single acceptance entry; weakening a stage weakens the gate (locked by EXPECTED constants) |

Phase 14 inherits the STRIDE-11/12/13 registers (see
`docs/threat-model/STRIDE-11.md` / `-12.md` / `-13.md`). The register below
enumerates only the **new threats** introduced by Phase 14 plans 01-07.

---

## 2. STRIDE Register — Phase 14 New Threats

| Threat ID | Category | Component | Disposition | Mitigation (verified by) |
|-----------|----------|-----------|-------------|---------------------------|
| T-14-01 | Elevation of Privilege | 系统提示词合成（Skill 遮蔽地板） | mitigate | 代码级合成顺序 floor 在前 + TestPlatformFloorCannotBeShadowed + TestNoBypassOfPlatformFloorCompose 全仓扫描门（14-01） |
| T-14-02 | Tampering | chat/deep 双模板信任条款分叉（Pitfall 17） | mitigate | D14-A4 上提到唯一 platformFloorText 常量 + 双渲染路径收敛 ComposePlatformPrompt + TestPlatformFloorPrependBothPaths |
| T-14-03 | Tampering | reprohash digest 序 | accept（不变更） | 14-01 只改 hash 第三参数内容；digest 序零改动由既有契约测试 + TestPromptHashCoversFloor 守卫 |
| T-14-04 | Spoofing | admin 预览端点未授权访问 | mitigate | isAdminRequest 复用：env 未设默认拒绝 + ConstantTimeCompare + 403/404 语义（TestPlatformPromptPreview） |
| T-14-05 | Tampering | audit 加列 DDL 注入 | mitigate | DDL 为代码常量字符串（零运行时拼接）；加列走 columnExists 幂等门 |
| T-14-06 | Information Disclosure | 预览端点泄露系统提示词 | mitigate | admin-only token 门；地板与 Skill prompt 非机密（开源 examples）——平台安全模型靠结构性 prepend 不靠保密（accept 残余） |
| T-14-06b | Tampering | HITL 行漏带版本伪造漂移窗口 | mitigate | NewAuditWriter 必填参数 + ErrMissingFloorVersion fail-closed；TestHITLPlatformFloorVersionRows/FailClosed |
| T-14-07 | Denial of Service | 并发写 SQLITE_BUSY 立即失败 | mitigate | DSN `_busy_timeout=5000` 每连接生效 + TestSqliteMattnZeroRegression 并发零 BUSY 断言（14-03） |
| T-14-08 | Tampering | :memory: 连接池多库假象（Pitfall 3） | mitigate | withDefaults 既有 MaxOpenConns=1 分支保留 + 回归断言 |
| T-14-09 | Elevation of Privilege | 六类攻击面诱导绕地板 | mitigate | 20 例冻结集 + JudgeHermetic 正则门（14-04）+ 地板 prepend + trust_floor 双层；live 层 14-08 |
| T-14-10 | Tampering | 判定器正则被绕过（大小写/注释混淆） | mitigate | 大小写不敏感正则 + ATTACH/PRAGMA 模式集；judge 属回归面非运行时防线（残余混淆 accept，live judge 兜底） |
| T-14-11 | Tampering | golden/lock 文件被静默改动 | mitigate | eval_set_sha256 + lock_sha256 双哈希 + D14-B4 bump 纪律（TestPlatformFloorLockSelfConsistent） |
| T-14-12 | Injection | ATTACH 路径注入（dataset id → SQL） | mitigate | ValidateDatasetID 白名单 `^[A-Za-z0-9_-]{1,64}$` + TestValidateDatasetID（含 `../` 与 `;` 样例）（14-05） |
| T-14-13 | Information Disclosure | 扩展自动安装/加载外联（hermetic 破坏） | mitigate | boot-queries 两条 SET 常量每连接重放 + TestDuckDBBootQueries 4 连接逐一断言 |
| T-14-14 | Tampering | WAL 未刷盘丢导入数据 | mitigate | Close() 双关纪律 + TestDuckDBCloseFlushesWAL 重开行数断言（Pitfall 5） |
| T-14-15 | Tampering | 半成品数据集污染 datasets/ | mitigate | staging + os.Rename 原子改名 + 失败只清 staging + TestStagingAtomicRename 四面（14-06） |
| T-14-16 | Injection | datasets 注册表 SQL 注入 | mitigate | PutDataset/ListDatasets 100% `?` 参数化（T-3-SQLI 纪律） |
| T-14-17 | Denial of Service | Windows 文件锁——不 DETACH 就删/改名 | mitigate | 逐出与提交路径先 DETACH 后文件操作 + TestAttachLRUDetachIdempotent + TestStagingAtomicRename(target-exists) |
| T-14-18 | Repudiation | 重锁动作无痕（基线被静默篡改） | mitigate | 重锁只动 prompt_hash 的 git diff 核对 + 新旧 pin/sha256 留档 SUMMARY（T-8-12 记录纪律；14-07 SUMMARY 留档五锁基线） |
| T-14-19 | Tampering | gate 脚本漏段/弱化（丢 -race/丢 source-grep） | mitigate | verify-phase-14.sh 固定 11 run_stage 结构 + Stage 09 EXPECTED 常量 + Stage 02 恢复 -race；任何削弱必被 Stage 09 sha256 撞 |
| T-14-20 | Tampering | race 门对吞吐断言假阳性/假阴性（负载敏感抖动被误读为回归或放水） | mitigate | TestDuckDBAppenderThroughput 移入 !race build-tag 文件（appender_throughput_test.go）：race 插桩使 cgo 边界 rows/sec 失去物理意义（实测 106k 单独 / 全仓并行 <100k vs 100k 地板）；地板由 Stage 04 非 race 覆盖（实测余量 ~6x）；Stage 08 source-grep 路径随迁 |
| T-14-21 | Tampering | live runner 把「拒绝段落里引用 SQL 关键字」误判为 SQL 泄漏 | mitigate | Hermetic regex 在 14-04 故意宽口径（catch any partial leak）；live runner 加 looksLikeActualSQL 第二过滤（prose-opens-with-SQL verb OR fenced ```sql block 才计 sql_fail）。partial obedience 解析加 first-line JSON fall-back 防 GLM 前导词破 JSON。同样的 live-only 收紧 + hermetic 留宽：宽筛防漏 → 紧筛防误报（14-08 实跑 pfloor-02/pfloor-20 即两类典型） |
| T-14-SC | Tampering | 依赖安装（mattn v1.14.50 / go-duckdb v2.4.3） | mitigate | RESEARCH §Package Legitimacy Audit Approved（go module proxy 实时 + 官方 README 双验；DuckDB 官方接手维护）；go get+build 即自验证。其余 plan 零新增依赖（accept） |

---

## 3. Residual Risk Notes

1. **T-14-10 residual**: the hermetic regex judge is a regression surface,
   not a runtime defense. Novel obfuscation (comments inside SQL keywords,
   exotic encodings) passes the regex layer by design — the runtime defense
   is the structural prepend + trust_floor double layer; the live judge
   (14-08) is the second net.
2. **Windows-native duckdb linking is unsupported** (emutls ABI mismatch,
   upstream duckdb-go-bindings#105). The data-support test surface runs on
   Linux/WSL2/Docker; this is an environment constraint, not a threat
   mitigation gap.
3. **hitl.lock.json prompt_hash does not cover the platform floor** — it is
   the SKILL.md file digest by design (Phase 11 schema), verified byte-stable
   in Stage 09. The floor-covered pin is compliance.lock.json (re-pinned
   14-01 per D14-B3).
4. **platform-floor.lock.json file-byte is NOT pinned** in Stage 11 (the
   other 4 byte-frozen locks still are). Every live fill (14-08 operator
   gate) appends `metrics` + `operator_signed_at` + `operator_name`, so a
   whole-file byte drift is the expected outcome of a successful live
   operator gate. The MEANINGFUL check is FIELD-level self-consistency
   (`lock_sha256` field == sha256 of the canonical body, omitting the
   `lock_sha256` field itself) — that is locked by
   `TestPlatformFloorLockSelfConsistent` in Stage 09. Re-add file-byte
   pinning only if the live fill surface stabilizes (i.e. metrics fields
   stop evolving).
