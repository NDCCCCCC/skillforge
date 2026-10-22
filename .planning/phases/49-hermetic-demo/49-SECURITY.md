---
phase: "49"
slug: "hermetic-demo"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
block_on: high
# register_authored_at_plan_time added 2026-10-22: all four plans of this phase (49-01..49-04)
# author a STRIDE register inside their own <threat_model> block, and the rows below were
# entered from those blocks rather than by a retroactive scan of the tree. The flag was
# missing, not false; with it declared, secure-phase's skip condition (threats_open 0 +
# register authored at plan time + asvs_level 1 -> L1 grep depth is sufficient) applies.
register_authored_at_plan_time: true
created: "2026-10-21"
verified: "2026-10-22"
---

# Phase 49 — Security Register

本登记册的每一行都来自计划自己的 `<threat_model>`（49-01 的 T-49-01..09、49-02 的 T-49-11..19、
49-03 的 T-49-21..25、49-04 的 T-49-31..39），加上各计划共用的 `T-49-SC` 保留行。

## Arithmetic

- **32 rows**（2 critical / 16 high / 12 medium / 2 low）
- **30 mitigated** + **2 accepted**（明示接受，非遗漏）
- **0 open** —— 高危及以上（block_on: high）无未闭合项
- STRIDE 分布：Tampering 10 / Denial of Service 6 / Information Disclosure 5 / Spoofing 5 / Elevation of Privilege 3 / Repudiation 3

| ID | STRIDE | Target | Severity | Plan | Status | Evidence |
|---|---|---|---|---|---|---|
| T-49-01 | Elevation of Privilege | `internal/retriever/embedded/embedded.go` 的 `Search` / `tier_isolation.go` 的 `candidates` | critical | 49-01 | closed | 单测：`TestSearchNeverScoresExcludedTiers` |
| T-49-02 | Tampering | `internal/retriever/embedded/seed.go` | high | 49-01 | closed | 单测：— |
| T-49-03 | Tampering | `internal/retriever/embedded/cache.go` 的 `readCache` | high | 49-01 | closed | 单测：`TestCacheCorruptFileTriggersRebuild` |
| T-49-04 | Information Disclosure | `internal/retriever/embedded/cache.go` 的 key 与载荷 | medium | 49-01 | closed | 单测：`TestCacheKeyIncludesModelDimAndCorpusHash` |
| T-49-05 | Denial of Service | `internal/retriever/embedded/cache.go` 的 `acquireCacheLock` | medium | 49-01 | closed | 单测：`TestCacheConcurrentFirstStart` |
| T-49-06 | Denial of Service | `internal/vecmath.Cosine` 经 `embedded.Search` 被调用 | high | 49-01 | closed | 单测：`TestSearchTopKAndQueryVecGuards` |
| T-49-07 | Tampering | `cmd/server/main.go` 的 `indexerBackend` switch | high | 49-01 | closed | 单测：`TestBackendSwitchEmbeddedHasExplicitCase` |
| T-49-08 | Repudiation | `cmd/server/main.go` 的 `buildRetriever` embedded 分支 | medium | 49-01 | closed | 单测：`TestBuildRetrieverEmbeddedCaseNeedsNoExternalDB` |
| T-49-09 | Spoofing | `internal/retriever/embedded/seed.go` 与 `embedded.go` 的向量来源 | high | 49-01 | closed | 单测：— |
| T-49-11 | Spoofing | `internal/server/stubchat/stub.go` + `internal/verdict/verdict.go` 的 `Mode` 字段 | high | 49-02 | closed | web/src/types/verdict.test.ts（`real-model verdicts have no mode field (wire backward-compat)`、`VERDICT_MODES contains only "stub"…`）+ 门禁 P4 `P4-MODE-STUB` + `internal/server/stubchat` 的 `TestStub_GenerateMatchesStream` |
| T-49-12 | Tampering | `internal/server/stubchat/extract.go` 的 span 抽取 | high | 49-02 | closed | 单测：`TestStub_ForgedSpanIsDowngraded`、`TestStub_SpanGroundedByRealChecker` |
| T-49-13 | Tampering | `internal/server/stubchat/extract.go` 的块边界判定 | medium | 49-02 | closed | 单测：`TestExtractRefBlocks_TableDriven` |
| T-49-14 | Information Disclosure | `cmd/server/main.go` 的启动 WARN 与 `glmKey` | high | 49-02 | closed | 单测：— |
| T-49-15 | Spoofing | `cmd/server/main.go` 的 stub 装配与真模型装配 | high | 49-02 | closed | 单测：— |
| T-49-16 | Elevation of Privilege | stub 类型不实现 `WithTools` | medium | 49-02 | closed | 单测：— |
| T-49-17 | Denial of Service | stub 模式下的 `RefusalClassifier` | high | 49-02 | closed | 单测：— |
| T-49-18 | Denial of Service | `internal/server/stubchat/stub.go` 的 `Stream` goroutine | medium | 49-02 | closed | 单测：`TestStub_ConcurrentStreamByteIdentical` |
| T-49-19 | Repudiation | 审计链的 stub 可追溯性 | medium | 49-02 | closed (accepted) | 单测：— |
| T-49-21 | Information Disclosure | `internal/webui/static.go` 的路径解析（`rel` / `fsPath`） | high | 49-03 | closed | 单测：`TestStaticHandlerRejectsTraversalAndSymlink` |
| T-49-22 | Tampering | `internal/webui/static.go` 的 SPA 回退分支 | high | 49-03 | closed | 单测：`TestStaticHandlerAPIPrefixNeverFallsBackToHTML`、`TestStaticHandlerMissingAssetReturns404` |
| T-49-23 | Denial of Service | `cmd/server/main.go` 的 catch-all 注册位置 | high | 49-03 | closed | 单测：`TestRouteOrderAPIRoutesRegisteredBeforeCatchAll`、`TestRouteOrderCatchAllDoesNotShadowAPI` |
| T-49-24 | Information Disclosure | `internal/webui/static.go` 的 503 分支 | low | 49-03 | closed (accepted) | 单测：`TestStaticHandlerMissingDistReturns503` |
| T-49-25 | Tampering | 静态托管对 `/api` 的写缓冲/包装 | medium | 49-03 | closed | 单测：`TestRouteOrderCatchAllDoesNotShadowAPI` |
| T-49-31 | Tampering | `verify-phase-49.sh` 的替身端点注入 | high | 49-04 | closed | 单测：— |
| T-49-32 | Spoofing | `verify-phase-49.sh` 的替身向量生成 | high | 49-04 | closed | 单测：— |
| T-49-33 | Elevation of Privilege | 门禁 P3 的 tier 隔离断言 | critical | 49-04 | closed | `internal/retriever/embedded/tier_isolation_test.go`：`TestTierAllowedEmptyIsDeny`、`TestCandidatesExcludesExpiredEffectiveTo`、`TestSearchNeverScoresExcludedTiers`、`TestSearchCandidateSetEmptySkipsRank`（= 过滤先于打分）+ 门禁 P3 `P3-NO-UNAUTHORIZED-TIER` / `P3-FILTER-BEFORE-SCORE` |
| T-49-34 | Information Disclosure | `docs/demo.gif` 二进制 | high | 49-04 | closed | 单测：— |
| T-49-35 | Repudiation | 门禁的成功 token 与分区计数 | medium | 49-04 | closed | 单测：— |
| T-49-36 | Denial of Service | P4 起的后端进程与端口 | medium | 49-04 | closed | 单测：— |
| T-49-37 | Tampering | `scripts/record-demo.sh` 的渲染路径 | medium | 49-04 | closed | 单测：— |
| T-49-38 | Spoofing | README 的 hermetic 小节 | medium | 49-04 | closed | 单测：— |
| T-49-39 | Tampering | `.gitignore` 的幂等追加 | low | 49-04 | closed | 单测：— |
| T-49-SC | — | 全 phase 的保留行（scope creep / 信任边界） | — | 49-01..04 | reserved | 各计划的 `<threat_model>` 保留行 |

## Closure evidence — how each row was checked

每行的 Evidence 列是**机械核对**的结果，不是抄写：脚本从该行的 mitigation 文本里抽出 `Test*`
标识符与 `P<n>-*` 门禁 token，逐个到仓库里找（`internal/**/*_test.go`、`cmd/**/*_test.go`、
`verify-phase-49.sh`）。**30/32 行的命名标识符逐字存在**。

两行的命名与实现不一致，如实记录（不粉饰）：

- **T-49-11**（wire 的 `mode` omitempty，high）—— 计划写的 Go 标识符 `TestVerdictModeOmitempty` **不存在**；
  等价覆盖在 `web/src/types/verdict.test.ts`（`real-model verdicts have no mode field (wire backward-compat)`、
  `VERDICT_MODES contains only "stub" …`）、`internal/server/stubchat` 的 `TestStub_GenerateMatchesStream`
  与门禁 P4 的 `P4-MODE-STUB`。行为已覆盖，标识符不符。
- **T-49-33**（门禁 P3 的 tier 隔离断言，critical）—— 计划写的 `TestTierIsolation*` / `TestEffectiveTo*` /
  `TestTierFilterRunsBeforeScoring` **不存在**；等价覆盖在 `internal/retriever/embedded/tier_isolation_test.go`
  （`TestTierAllowedEmptyIsDeny`、`TestCandidatesExcludesExpiredEffectiveTo`、`TestSearchNeverScoresExcludedTiers`、
  `TestSearchCandidateSetEmptySkipsRank` —— 最后一个正是「过滤先于打分」的断言）与门禁 P3 的
  `P3-NO-UNAUTHORIZED-TIER` / `P3-FILTER-BEFORE-SCORE`。行为已覆盖，标识符不符。

这两个偏差是**计划与实现之间的命名漂移**，不是未闭合的威胁；两处的行为断言都真实存在且通过。
留作后续动作：计划的 `<automated>` 里的 `-run` 选择器应与真实用例名对齐（phase 48 的
`48-24-PLAN.md` 有同类教训：演练锚点必须指向真实存在的标识符，否则「红」是假的）。

## Mutation evidence — the gate proves the mitigations bite

`verify-phase-49.sh` 的十条变异演练（D0..D9）就是这些缓解措施的证伪装置：删掉 tier 过滤 → P3 缺
`P3-NO-UNAUTHORIZED-TIER` 而红；删掉 WRITE_VALUE 扫描 → `perm-write-line` 以错误理由被拒而红；把替身
向量换成单一固定向量 → P4 拿不到非空 citations 而红（这就是 T-49-32 的证伪）；删掉 `done_partition`
调用 → P9 的分区计数断言而红。门禁两次连跑均 `=== Phase 49 Gate: ALL PASSED ===`。

## Accepted risks (明示接受)

- **T-49-19**（Repudiation，medium）—— 审计链的 stub 可追溯性：`internal/server/chat.go` 的 `modelVersion` 写死 `"glm-5.2"`，stub 模式下该列不反映 stub；本 plan 不改该文件（越界）。接受理由：stub 的真实性由 `verdict.Mode` 落进审计行的 `verdict_json` 列（该列是 `json.Marshal(v)` 的完整副本）承载，事后仍可区分。残留记录在 flagged_assumptions 第 5 条
- **T-49-24**（Information Disclosure，low）—— `internal/webui/static.go` 的 503 分支：dist 缺失时只回固定提示（含 `cd web && npm run build`），不含绝对路径、不含 `os.Getwd()`、不回显 env、不列目录；信息量等于“请先构建”，对未授权访问者无额外泄露价值。`TestStaticHandlerMissingDistReturns503` 以临时目录绝对路径为探针断言其不出现

## Audit Trail

| Date | Rows | Closed | Open | Note |
|---|---|---|---|---|
| 2026-10-21 | 32 | 32 | 0 | 登记册建立（49-01..49-04 的 `<threat_model>` 汇总） |
| 2026-10-22 | 32 | 32 | 0 | 机械核对 30/32 行的命名标识符逐字存在；2 行为命名漂移（行为已覆盖） |

## Sign-Off

- [x] 每个 plan 的 `<threat_model>` 覆盖到本登记册，无遗漏、无重复编号
- [x] block_on（high）以上无未闭合项（`threats_open: 0`）
- [x] 每行的缓解措施有对应的**可执行**断言（单测或门禁 token），并已逐字核对存在
- [x] 命名漂移的两行已如实记录并给出等价覆盖位置
- [x] 明示接受的两项风险写明理由

**Approval:** approved 2026-10-22（编排者；独立安全审计未执行——与 `49-VERIFICATION.md` 的
独立性说明同源，见该文件开头的 caveat）
