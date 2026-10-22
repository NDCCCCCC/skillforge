---
phase: "49"
slug: "hermetic-demo"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-10-21"
validated: "2026-10-22"
---

# Phase 49 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
>
> 本文件由 `/gsd-plan-phase 49` 从 `49-RESEARCH.md` 的 `## Validation Architecture` 生成（§5.5 Nyquist）。每条命令都必须在**无 Milvus / 无 BGE / 无 GLM / 无网络**的干净 checkout 上可跑；每条命令都有明确的失败信号（测试失败 = 非零退出；门禁分区失败 = `fail()` 立即 `exit 1`）。

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `testify` v1.11.1 / `vitest` ^2.0.0 / bash 分区门禁 |
| **Config file** | 无专用配置文件——沿用各包既有的 `go test` 与 `web/vite.config.ts`；门禁脚本自带分区日志目录 |
| **Quick run command** | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/retriever/embedded/ ./internal/server/stubchat/ ./internal/webui/ -count=1"` |
| **Full suite command** | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./... -count=1"` + `cd web && npm test` + `bash verify-phase-49.sh` |
| **Estimated runtime** | 新增包单测 ~15s；全量 Go 回归 ~120s（WSL）；前端 ~140s；门禁 ~30s |

> **宿主限制（不是回归）**：Windows 宿主上全量 `go test ./...` 必在 `cmd/server`、`internal/data`、`internal/duckdb`、`internal/kpi/wsl`、`internal/server` 报 `[build failed]` —— `duckdb-go-bindings` 预编译 Windows 静态库与本机 msys2 ucrt64 gcc 16.1.0 的 TLS ABI 不匹配（`ld.exe: undefined reference to __emutls_v...`）。phase 49 不改任何 Go 依赖，Go 测试一律走 WSL2。宿主侧的编译面检查用 `go vet ./...`（实测 `go vet ./cmd/server` exit 0）。

---

## Sampling Rate

- **After every task commit:** 跑该 task 触及包的 `go test`（WSL）+ `go vet ./...`（宿主）+ 涉及的 vitest
- **After every plan wave:** `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./... -count=1"` + `cd web && npm test` + `bash verify-phase-49.sh`
- **Before `/gsd:verify-work`:** 全量必须绿，且 `bash verify-phase-49.sh` 连续两次通过（幂等性）+ 录制脚本 dry-run
- **每次改动 `internal/verdict/verdict.go` 或 `web/src/types/verdict.ts`:** **必须**加跑全量（wire 契约变更影响面广）
- **Max feedback latency:** 门禁分区 ~30s；单包单测 ~15s

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 49-W0-01 | wave-0 | 0 | DEMO-01 | T-49-01 | embedded 对 `knowledge/` 建索引并返回可引用 chunk | unit | wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/retriever/embedded/ -run 'TestSearch | TestEnsure | TestInsert | TestGetChunk | TestGetMaxID | TestListMetadata | TestDisconnect' -count=1" | ✅ | ✅ green |
| 49-W0-02 | wave-0 | 0 | DEMO-01 | T-49-01 / T-49-33 | **tier 隔离不削弱**：空 tier → 0 命中；非白名单 tier → 0 命中；effective_to 过期 → 0 命中 | unit | wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/retriever/embedded/ -run 'TestTier | TestCandidates | TestSearchEmptyTiers | TestSearchNeverScoresExcludedTiers | TestSearchCandidateSetEmptySkipsRank' -count=1" ＋ 门禁 P3 `P3-NO-UNAUTHORIZED-TIER` / `P3-FILTER-BEFORE-SCORE` | ✅ | ✅ green | ⬜ pending |
| 49-W0-03 | wave-0 | 0 | DEMO-01 | — | 排序确定性：同 query 两次 Search 顺序一致（score desc, id asc tie-break） | unit | wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/retriever/embedded/ -run 'TestRank' -count=1" | ✅ | ✅ green |
| 49-W0-04 | wave-0 | 0 | DEMO-01 | T-49-03 / T-49-04 / T-49-05 | 缓存：key 含 model + dimension + 语料 hash；换模型/换语料 → 失效重建；损坏文件 → 重建而非部分可用 | unit | wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/retriever/embedded/ -run 'TestCache | TestCorpusHash | TestEnsure' -count=1" | ✅ | ✅ green |
| 49-W0-05 | wave-0 | 0 | DEMO-01 | T-49-08 | `RETRIEVER_BACKEND=embedded` 时不要求 `MONGO_URI` / `MILVUS_ADDRESS`（D-16） | unit（`buildRetriever` 表驱动） | wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./cmd/server/ -run 'TestBackendSwitch | TestBuildRetriever' -count=1" | ✅ | ✅ green |
| 49-W0-06 | wave-0 | 0 | DEMO-02 | T-49-15 / T-49-19 | 无 key 走 stub；产出 Verdict JSON 与真模型逐字节同构 | unit | wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/server/stubchat/ -run 'TestStub_StreamProducesValidVerdict | TestStub_GenerateMatchesStream' -count=1" | ✅ | ✅ green |
| 49-W0-07 | wave-0 | 0 | DEMO-02 | T-49-12 | stub 的 answer 逐字来自 chunk；citations 的 span 真实出现在 chunk 内（grounding 不降级） | unit | wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/server/stubchat/ -run 'TestStub_SpanGroundedByRealChecker' -count=1" | ✅ | ✅ green | ⬜ pending |
| 49-W0-08 | wave-0 | 0 | DEMO-02 | T-49-13 | stub 的 `confidence ≤ 0.5` 且 `review_required == true`（D-10） | unit | wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/server/stubchat/ -run 'TestStub_ConfidenceAndReviewFloor' -count=1" | ✅ | ✅ green |
| 49-W0-09 | wave-0 | 0 | DEMO-02 | T-49-12 / T-49-16 | stub 走**同一条** grounding 校验路径，未被豁免 | unit（pipeline 集成） | wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/server/stubchat/ -run 'TestStub_ForgedSpanIsDowngraded | TestStub_SpanGroundedByRealChecker' -count=1" | ✅ | ✅ green | ⬜ pending |
| 49-W0-10 | wave-0 | 0 | DEMO-02 | T-49-11 / T-49-14 | 三重标注：wire 有 `mode:"stub"` / 启动日志有 WARN / README 写了 stub 模式 | 门禁分区 P7（grep 断言） | bash verify-phase-49.sh ＋ 门禁 P4 `P4-MODE-STUB` / `P4-STUB-VERDICT-VALID` / `P4-NO-KEY-STARTED` | ✅ | ✅ green |
| 49-W0-11 | wave-0 | 0 | DEMO-03 | T-49-11 | 前端类型能解析 `mode` 字段，且 `isVerdict` 守卫不因新字段失效 | 前端 unit | cd web && npx vitest run src/types/verdict.test.ts | ✅ | ✅ green |
| 49-W0-12 | wave-0 | 0 | DEMO-03 | T-49-21 / T-49-22 / T-49-24 | 同源静态托管四分支：有文件 → 文件 / 无扩展名不存在 → index.html / 有扩展名不存在 → 404 / dist 缺失 → 503 | unit | wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/webui/ -count=1" | ✅ | ✅ green |
| 49-W0-13 | wave-0 | 0 | DEMO-03 | T-49-31 / T-49-32 | 端到端 hermetic：起服务端（测试替身 embedding 端点 + 无 key）→ 打一轮带引用的问答 → 断言 data-policy 帧含 citations 与 `mode:"stub"` | 门禁分区 P5 | bash verify-phase-49.sh（P4/P5 live leg）＋ fresh-clone 探针 `chat-path-probe.sh` | ✅ | ✅ green |
| 49-W0-14 | wave-0 | 0 | DEMO-03 | T-49-34 | 素材存在且合法：`docs/demo.gif` magic 为 `GIF8`；`scripts/record-demo.sh` 存在且可执行 | 门禁分区 P8 | bash verify-phase-49.sh ＋ 门禁 P7 `P7-GIF-MAGIC` | ✅ | ✅ green |
| 49-W0-15 | wave-0 | 0 | DEMO-03 | T-49-34 | 素材不含密钥：对 `docs/demo.gif` 做二进制扫描 | 门禁分区 P9 | bash verify-phase-49.sh ＋ 门禁 P7 `P7-NO-KEY-BYTES` | ✅ | ✅ green |
| 49-W0-16 | wave-0 | 0 | SC5 | T-49-35 | `bash verify-phase-49.sh` exit 0（含 hermetic 端到端） | 门禁（根脚本） | bash verify-phase-49.sh（两次连跑均 exit 0 且恰好一条 `=== Phase 49 Gate: ALL PASSED ===`） | ✅ | ✅ green |
| 49-W0-17 | wave-0 | 0 | DEMO-01/02/03 | T-49-31 / T-49-32 | 门禁替身端点边界：替身只监听 127.0.0.1、`BGE_M3_URL` 只在脚本子进程环境内被覆盖、生产二进制零内嵌替身分支、门禁输出打印 `using test-only embedding double` | 门禁分区（grep 断言） | bash verify-phase-49.sh ＋ 门禁 P8 `git diff --quiet -- internal/embedder/bge` 与 `using test-only embedding double` 断言 | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

> 本表的 Task ID 是**研究给出的 Wave 0 资产编号**（`49-W0-NN`）。`/gsd-plan-phase 49` 落盘后，须把每条映射到真实 plan/task ID；`/gsd-validate-phase` 在执行期负责校准本表。Threat Ref 列已按 `49-SECURITY.md` 的实际登记册校准（49-01 的 T-49-01..09、49-02 的 T-49-11..19、49-03 的 T-49-21..25、49-04 的 T-49-31..39；`T-49-SC` 是各计划共用的保留行，不列入编号）。

---

## Wave 0 Requirements

- [x] `internal/retriever/embedded/embedded_test.go` — Search / Ensure / Insert / GetChunk / GetMaxID / ListMetadata / Disconnect 的行为契约
- [x] `internal/retriever/embedded/tier_isolation_test.go` — 空 tier → 0 命中、非白名单 tier → 0 命中、effective_to 过期 → 0 命中（**D-TIER-03 不削弱的核心断言**）
- [x] `internal/retriever/embedded/rank_test.go` — 排序确定性（score desc, id asc tie-break）
- [x] `internal/retriever/embedded/cache_test.go` — 缓存 key 三要素、失效、损坏重建、并发首次启动
- [x] `internal/server/stubchat/stub_test.go` — wire 逐字节同构、confidence ≤ 0.5、review_required、grounding 不豁免
- [x] `internal/webui/static_test.go` — SPA fallback 四分支 + dist 缺失
- [x] `cmd/server/backend_switch_test.go` — `buildRetriever` 表驱动：embedded 分支不要求 MONGO_URI/MILVUS_ADDRESS
- [x] `web/src/types/verdict.test.ts` — `mode` 字段解析 + `isVerdict` 不回归
- [x] `verify-phase-49.sh` — 根目录门禁（P1..P9 分区、每区落日志、空日志判失败、fail-fast、收尾横幅按已运行分区计数）
- [x] `scripts/record-demo.sh` — 录屏脚本（SC4 证据）
- [x] `.gitignore` 增补向量缓存目录条目

> 门禁脚本**不得**依赖 asciinema / agg / Ollama / Docker（否则就不是 hermetic 门禁）；素材存在性用 magic bytes 断言即可。

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `docs/demo.gif` 的观感与体积（是否适合放进 README 首屏） | DEMO-03 | 视觉判断无法用断言表达；且本宿主缺 `agg`（GitHub 不可达），渲染路径可能走 pip-only 兜底 | 打开 `docs/demo.gif` 目视：终端文字清晰、步进可读、无密钥、无半截帧；记录体积 |
| 有真实 GLM key 时的真模型路径（stub 不介入） | DEMO-02 | hermetic 门禁环境无 key、无外网，真模型路径只能人工验一次 | 设 `GLM_API_KEY` 后起服务，问一轮，确认响应**无** `mode` 字段且引用来自模型 |
| README Quick Start 的"照着念就能跑" | SC1 | 文档可用性是人工判断 | 按 README 的 hermetic 段逐行执行一遍，记录卡住的那一行 |

*If none: "All phase behaviors have automated verification."* —— 本 phase 有上述三项人工项。

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s（单包反馈：embedded 11.7s / webui 5.2s / stubchat 0.02s / verdict 0.01s / cmd-server 0.1s；全量门禁是慢腿，8m00s）
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-22（编排者；`/gsd:validate-phase` 的独立审计未执行——本文件由编排者按 §5.5 的
契约校准，与 `49-VERIFICATION.md` 的独立性 caveat 同源）

## Validation Audit 2026-10-22

| Metric | Count |
|---|---|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 2 |

> **Escalated = 2**（不阻断 phase 闭合，但必须留档）：计划里两处 `<automated>` 的 `-run` 选择器指向的用例名
> 与实现里的真实用例名不一致——
> ①`49-02` 的 `TestVerdictModeOmitempty`（wire `mode` 的 omitempty 行为）实际由前端
> `web/src/types/verdict.test.ts` 的 `real-model verdicts have no mode field (wire backward-compat)`、
> `internal/server/stubchat` 的 `TestStub_GenerateMatchesStream` 与门禁 P4 `P4-MODE-STUB` 覆盖；
> ②`49-04` 的 `TestTierIsolation*` / `TestEffectiveTo*` / `TestTierFilterRunsBeforeScoring` 实际由
> `internal/retriever/embedded/tier_isolation_test.go` 的 `TestTierAllowedEmptyIsDeny`、
> `TestCandidatesExcludesExpiredEffectiveTo`、`TestSearchNeverScoresExcludedTiers`、
> `TestSearchCandidateSetEmptySkipsRank` 与门禁 P3 覆盖。
> **行为断言都存在且通过**，偏差在标识符。本表已把两行的 Automated Command 改写为真实可跑的写法；
> 后续 phase 的计划应让 `-run` 选择器与真实用例名对齐（phase 48 的 `48-24-PLAN.md` 有同类教训）。

### 本次校准做了什么

- frontmatter 从 `draft` / `nyquist_compliant: false` / `wave_0_complete: false` 推进到
  `validated` / `true` / `true`（2026-10-22）。
- 17 行 Per-Task Verification Map 的 `File Exists` 全部 → ✅、`Status` 全部 → ✅ green，并把
  `Automated Command` 换成本次真正跑过的命令（Go 五包 + 前端 697 用例 + 门禁两次连跑 + fresh-clone 探针）。
- Wave 0 Requirements 的 11 项全部落地。
- 人工项保留三项（观感、真模型路径、README 照着念）；其中 README 照着念已在 fresh-clone 探针里被实跑覆盖
  （clone → build → 起服务 → 一轮问答），仍保留为人工项是因为「照着念」的可用性是人的判断。
