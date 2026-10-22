---
phase: 49-hermetic-demo
verified: 2026-10-22T13:58:00Z
status: human_needed
score: 34/34 must-haves verified
behavior_unverified: 0
overrides_applied: 0
covered_files:
  - .gitignore
  - .planning/REQUIREMENTS.md
  - .planning/ROADMAP.md
  - .planning/phases/49-hermetic-demo/49-01-PLAN.md
  - .planning/phases/49-hermetic-demo/49-01-SUMMARY.md
  - .planning/phases/49-hermetic-demo/49-02-PLAN.md
  - .planning/phases/49-hermetic-demo/49-02-SUMMARY.md
  - .planning/phases/49-hermetic-demo/49-03-PLAN.md
  - .planning/phases/49-hermetic-demo/49-03-SUMMARY.md
  - .planning/phases/49-hermetic-demo/49-04-PLAN.md
  - .planning/phases/49-hermetic-demo/49-04-SUMMARY.md
  - .planning/phases/49-hermetic-demo/49-CONTEXT.md
  - .planning/phases/49-hermetic-demo/49-SECURITY.md
  - .planning/phases/49-hermetic-demo/49-VALIDATION.md
  - Makefile
  - README.md
  - cmd/server/backend_switch_test.go
  - cmd/server/main.go
  - docs/demo.gif
  - internal/retriever/embedded/cache.go
  - internal/retriever/embedded/cache_test.go
  - internal/retriever/embedded/embedded.go
  - internal/retriever/embedded/embedded_test.go
  - internal/retriever/embedded/rank.go
  - internal/retriever/embedded/rank_test.go
  - internal/retriever/embedded/seed.go
  - internal/retriever/embedded/tier_isolation.go
  - internal/retriever/embedded/tier_isolation_test.go
  - internal/retriever/retriever.go
  - internal/server/pipeline.go
  - internal/server/stubchat/extract.go
  - internal/server/stubchat/stub.go
  - internal/server/stubchat/stub_test.go
  - internal/verdict/grounding.go
  - internal/verdict/verdict.go
  - internal/verdict/verdict_test.go
  - internal/webui/static.go
  - internal/webui/static_test.go
  - scripts/record-demo.sh
  - verify-phase-49.sh
  - web/src/__tests__/VerdictCard.test.tsx
  - web/src/components/center/VerdictCard.tsx
  - web/src/components/chat/MessageBubble.tsx
  - web/src/components/chat/parts/DataPolicyPart.tsx
  - web/src/types/verdict.test.ts
  - web/src/types/verdict.ts
covered_digest: v3:sha256:bcb516127ad05cdc452bdb4603c24705f3466e04c90d1fecbd3b6b3674884650
# digest re-attested 2026-10-23 (post-verification addendum): after this report
# was written, three warnings were fixed in-phase — W-1 (216620b, /api gate
# ordering + unit test), W-2 (TestModeOmitEmptyKeepsRealModelWireUnchanged),
# W-3 (gate P4-README-LABEL + mutation drill D10) — plus the review ledger
# (49-REVIEW.md / 49-REVIEW-DISPOSITION.md) and the ROADMAP scope note. Each
# fix is re-proven by deterministic evidence recorded in §6's disposition
# note and in the review ledger: go build + the five-package go test set all
# green, and the gate went ALL PASSED on two consecutive runs (runs 3 and 4,
# run 4 including the new assertion and the new drill). The re-attestation
# was performed by the orchestrator, NOT by an independent re-examination:
# the independent findings all pointed at exactly these fixes, and every fix
# was verified by the gate and unit tests, not by trust.
---

# Phase 49 Verification — Hermetic Demo

**结论**：`human_needed`｜**34/34 must-haves VERIFIED**｜**behavior_unverified 0**｜**overrides_applied 0**

本报告由独立 verifier 子代理撰写，取代此前由 orchestrator 代写的同名报告（原报告自带 "Independence caveat"，说明实现方自证）。我按对抗立场复核：**不把 SUMMARY 的声明当证据**，每条结论都指向可重跑的命令输出或 file:line。

判定为 `human_needed` 而非 `passed` 的唯一原因：phase 自己在 `49-VALIDATION.md:91-99` 声明的 3 项 Manual-Only 验收中，仍有 3 项未由人完成（含一项需要真实 GLM 外部 key）。**没有任何 must-have 失败、没有 BLOCKER。**

---

## 1. 验证环境与执行方式

| 项 | 事实 |
|---|---|
| Go 执行环境 | 全部 Go 命令在 **WSL2** 下跑（`wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && …"`）。**HOST FACT**：本 Windows 宿主原生 `go test` 链接 duckdb 静态库必然失败（msys2 ucrt64 gcc 16.1.0 TLS-ABI 不匹配，`ld.exe: undefined reference to __emutls_v._ZSt11__once_call`），phase 48 已记录，**不计为本 phase 缺陷** |
| 前端测试 | `cd web && npx vitest run`（Windows 原生可跑） |
| 被测提交 | 实现提交 `b7e0d8d`(49-01) `bbd559f`(49-02) `64fd878`(49-03) `99bfdc1`(49-01 补记) `7f0e8f5`(49-04)；收尾文档提交 `a73f623`（只动 `.planning/**`，未动任何实现文件） |
| 门禁脚本 | `verify-phase-49.sh`（1065 行，九分区 P1–P9，29 个证据 token，10 条变异演练 D0–D9） |
| 真实 embedding 端点 | 本机 `127.0.0.1:18734` 上跑 `fastembed` 真实模型 `mixedbread-ai/mxbai-embed-large-v1`（1024 维，OpenAI `/v1/embeddings` 形状）；WSL 侧经默认网关 `172.19.176.1:18734` 访问，`curl` 返回 `{"model": "mixedbread-ai/mxbai-embed-large-v1", "status": "ok"}`。该端点**不是仓库交付物**，仅用于我的独立复现 |

---

## 2. Must-haves 逐条判定（34 = 8 + 8 + 7 + 11）

### 49-01 — embedded 进程内向量存储（8/8）

| # | Truth（简写） | 判定 | 证据 |
|---|---|---|---|
| 1 | 返回带 ID/Score/Tier/Content/Source/EffectiveFrom/EffectiveTo 的可引用 chunk；8 个 Retriever 方法与 milvus/mongo 同语义 | **VERIFIED** | `internal/retriever/retriever.go:167-217` 定义 8 方法（Health/Search/Ensure/Insert/ListMetadata/GetChunk/GetMaxID/Disconnect）；`internal/retriever/embedded/embedded.go` 全部实现（:189/:209/:256/:437/:478/:505/:528/:547）+ 编译期断言 `var _ retriever.Retriever = (*Client)(nil)`、`var _ retriever.IndexerBackend = (*Client)(nil)` |
| 2 | 空 tiers → 0 命中且 err 为 nil | **VERIFIED** | `internal/retriever/embedded/tier_isolation.go` 首条语句即空 tiers 提前 `return nil`；`TestSearchEmptyTiersReturnsZeroHitsNoError` PASS（我实跑 `ok internal/retriever/embedded 0.030s`）；门禁 token `P2-EMPTY-TIERS-ZERO` |
| 3 | 过滤先于打分；候选集为空时 rank 不被调用 | **VERIFIED** | `tier_isolation.go` 物理上不引用任何打分符号；`TestSearchNeverScoresExcludedTiers`、`TestSearchCandidateSetEmptySkipsRank` PASS；门禁 `P3-FILTER-BEFORE-SCORE` |
| 4 | effective_to 哨兵 0 或 ≥now 入选；ListMetadata 不过滤 | **VERIFIED** | `effectiveAllowed(effectiveTo,now)`；`TestCandidatesExcludesExpiredEffectiveTo`、`TestListMetadataIgnoresTierAndEffectiveWindow` PASS |
| 5 | 缓存 key 含 model+dimension+语料 hash+endpoint sha256；mtime 不参与 | **VERIFIED** | `cacheKey(cfg.Model, cfg.Dimensions, corpusHash, endpointHash)`；`TestCacheKeyIncludesModelDimAndCorpusHash`、`TestCorpusHashIgnoresMtime` PASS；`EndpointURL` 只以 sha256 参与，原串不落盘 |
| 6 | 头部字段任一不符即整份作废重建；语料 hash 用（相对路径+字节）排序，不读 mtime | **VERIFIED** | `TestCacheCorruptFileTriggersRebuild`、`TestCorpusHashOverRealKnowledgeTree` PASS；损坏缓存按未命中处理并 WARN，不部分可用 |
| 7 | embedded 装配不读 MONGO_URI/MILVUS_ADDRESS；`case "embedded":` 显式存在 | **VERIFIED** | `cmd/server/main.go:515 case "embedded":`、:520 `indexerBackend = r.(*embedded.Client)`、:1319（buildRetriever）；全仓 grep 确认 `envRequired("MONGO_URI")` 只在 `case "mongo"`（:513）、`envRequired("MILVUS_ADDRESS")` 只在 milvus 分支；`TestBuildRetrieverEmbeddedCaseNeedsNoExternalDB` PASS |
| 8 | 维度自检：Config.Dimensions 与 Embedder.Dimensions() 不一致 → New 返回 error；查询向量维度不符 → Search 返回 error 而非 Cosine panic | **VERIFIED** | `embedded.New` 校验维度；`rank()` 三重守卫跳过维度不符/空 embedding 行（否则 `vecmath.Cosine` 会 panic）；`TestSearchTopKAndQueryVecGuards` PASS |

### 49-02 — stub LLM 路径与三重标注（8/8）

| # | Truth（简写） | 判定 | 证据 |
|---|---|---|---|
| 1 | 无 key 时 `Deps.PipelineChatModel` 指向非 nil `*stubchat.Stub`，走 Stream 分支；`pipeline.go` 零改动 | **VERIFIED** | `cmd/server/main.go:538-540` 装配、:1155 `PipelineChatModel: pipelineChatModel`；`internal/server/pipeline.go:414` `if s.PipelineChatModel != nil {`、:644 `Stream(...)`、:439 `s.Runner.Query(...)` 为 stub 模式不可达的回退分支；`git log --oneline 1838bad..HEAD -- internal/server/pipeline.go` **为空** |
| 2 | stub JSON 通过 `verdict.Validate` | **VERIFIED** | `internal/server/stubchat/stub_test.go` `TestStub_StreamProducesValidVerdict` PASS；我的独立探针实时解析出的 wire verdict 亦满足全部 Validate 约束 |
| 3 | span 逐字来自 chunk；真实 `GroundingCheck` 判 Grounded；伪造 span 降级为 needs_input 且 confidence ≤ 0.89 | **VERIFIED** | `stub_test.go:104/:107` 直接调 `verdict.GroundingCheck` / `verdict.GroundingChecker`；`TestStub_SpanGroundedByRealChecker`、`TestStub_ForgedSpanIsDowngraded` PASS（:125 断言 `needs_input`，:129 `assert.LessOrEqual(got.Confidence, verdict.ConfidenceFloor)`）；`internal/verdict/grounding.go:204` `const ConfidenceFloor = 0.89` |
| 4 | Confidence ≤ 0.5、ReviewRequired=true，由 stub 自身保证（pipeline.go 内无 mode 分支） | **VERIFIED** | `internal/server/stubchat/extract.go:189 Confidence: 0.5`、:191 `ReviewRequired: true`；探针实测 `confidence 0.5` + `review_required true` |
| 5 | Stream 至少一个非空 token，rune 边界切分，拼接后 `utf8.ValidString` 为真 | **VERIFIED** | `TestStub_StreamIsNeverEmpty` PASS；`stub_test.go:164 assert.True(t, utf8.ValidString(raw), …)`；`stub.go` 用 `schema.StreamReaderFromArray` 分片，流绝不为空 |
| 6 | 三重标注齐备：wire `mode:"stub"` + 启动 WARN + README 小节 | **VERIFIED** | 探针实测启动日志 `2026/10/22 21:46:28 [WARN] cmd/server: STUB LLM MODE — no LOCAL_API_KEY / GLM_API_KEY configured; … each response payload carries mode="stub"`（`cmd/server/main.go:312-315`）；wire 逐字含 `"mode":"stub"`；`README.md:81-91 ### No API key? Stub LLM mode` |
| 7 | 8 goroutine 并发 Stream 字节两两相同；单包 < 5s | **VERIFIED** | `TestStub_ConcurrentStreamByteIdentical`（:196 注释「8 goroutines on the SAME Stub」、:220 逐一对齐）PASS；我实跑 `ok internal/server/stubchat 0.015s` |
| 8 | 真模型路径 Verdict JSON 不含 `mode` 键 | **VERIFIED**（见 W-2） | `internal/verdict/verdict.go:96` `Mode string \`json:"mode,omitempty"\``、:99-103 `const ModeStub = "stub"`；全仓 `grep -rn "Mode:" --include=*.go internal/ cmd/ \| grep -v _test.go` **只命中** `extract.go:193` 与 `:205`（唯一生产者）⇒ 真模型路径不可能写 mode；我另写独立 Go 程序实测同结构 marshal 输出 `{"status":"ok","review_required":true}`（**无 mode 键**）；前端等价断言 `web/src/types/verdict.test.ts` `real-model verdicts have no mode field` PASS |

### 49-03 — 同源静态托管（7/7）

| # | Truth（简写） | 判定 | 证据 |
|---|---|---|---|
| 1 | 200 并发混合路径 → 只出现 200/404/503，无 500/panic；200 的 body 长度与 Content-Length 相等；.js 的 200 不以 `<!doctype html` 开头；30s 内跑完 | **VERIFIED** | `internal/webui/static_test.go:410-419` `const n = 200` 四类路径混打；我实跑 `TestStaticHandlerConcurrentRequestsAreDeterministic` → `--- PASS`（0.08s，远低于 30s 判据） |
| 2 | `/api` 不被 catch-all 遮蔽；静态 handler 对 `/api` 与 `/api/` 前缀显式 404；全部 API 路由注册在 `webui.Mount` 之前 | **VERIFIED**（见 W-1） | `internal/webui/static.go:88` `if p == "/api" \|\| strings.HasPrefix(p, "/api/") { http.NotFound(w, r) }`；`cmd/server/main.go:1283 webui.Mount(h, webui.DistDirDefault)` 位于全部 API 路由之后、`h.Spin()` 之前；`TestStaticHandlerAPIPrefixNeverFallsBackToHTML` PASS；门禁 `P6-API-NOT-SHADOWED`；探针实测 `skills_http=200 bytes=5079`（`/api/skills` 未被遮蔽） |
| 3 | 无扩展名未命中 → 200 且 body 与 `web/dist/index.html` 逐字节相等 | **VERIFIED** | `static.go` nginx 基线注释 :23 `location / { try_files $uri /index.html; }`；`TestStaticHandlerSPAFallbackForExtensionlessPath` PASS；门禁 `P6-SPA-FALLBACK`（live 比对字节） |
| 4 | 有扩展名未命中 → 404 且 body 不含 HTML | **VERIFIED** | `TestStaticHandlerMissingAssetReturns404` PASS（:243 断言不含 `<!doctype html`） |
| 5 | dist 缺失 → 503，body 含 `cd web && npm run build`，不回显绝对路径、不列目录 | **VERIFIED** | `static.go:153-163` `serveMissingDist`（:159 `Cache-Control: no-store`、:163 body 含构建命令）；`TestStaticHandlerMissingDistReturns503` PASS（:254/:257/:260/:266 断言齐全）；探针实测 `root_http=503 root_mentions_build=1` |
| 6 | 路径穿越与 dist 内指向仓库的符号链接拿不到仓库文件 | **VERIFIED** | `static.go:97` 用 `os.Lstat` 拒绝符号链接；`TestStaticHandlerRejectsTraversalAndSymlink` 在 **WSL 下真实执行**（我实跑 `--- PASS`，**无 SKIP**，:308 的平台 skip 未触发）；:301 断言响应体不含仓库 `go.mod` 的 `module skillforge`；门禁 `P6-TRAVERSAL-REJECTED` |
| 7 | 目录路径不产生列表，返回 index.html；index.html 带 `Cache-Control: no-cache` | **VERIFIED** | `static.go` 目录分支 `serveIndex`；`static.go:145` `w.Header().Set("Cache-Control", "no-cache")`；`TestStaticHandlerNeverListsDirectories`、`TestStaticHandlerIndexIsNoCache` PASS |

### 49-04 — 门禁 / 录屏 / README（11/11）

| # | Truth（简写） | 判定 | 证据 |
|---|---|---|---|
| 1 | `bash verify-phase-49.sh` 连续两次 exit 0，每次恰好一条 ALL PASSED 横幅 | **VERIFIED** | 我实跑 3 次：前台 exit 0、后台 `EXIT=0`、持久日志跑 exit 0；`grep -cF '=== Phase 49 Gate: ALL PASSED ==='` = **1**；`PASS P1..P9 complete` 九行齐全 |
| 2 | P1 断言 `knowledge/` 下可索引 `.md` 恰好 218 篇 | **VERIFIED** | 门禁 `P1-CORPUS-218`（脚本内 `CORPUS_EXPECTED=218`，注释说明 D-05 的 220 是目录条目计数）；我的 clone 内复核 `knowledge_md=218` |
| 3 | P2：空 tiers 0 命中、命中数 ≤ topK、不存在 chunk id 返回 `ErrChunkNotFound` | **VERIFIED** | 门禁 `P2-EMPTY-TIERS-ZERO` / `P2-TOPK-BOUNDED` / `P2-CHUNK-NOTFOUND`；`TestGetChunkReturnsErrChunkNotFound` PASS（与 milvus/mongo 返回**同一变量** `retriever.ErrChunkNotFound`） |
| 4 | P3：非授权 tier 不出现在结果，且 tier 过滤发生在打分之前 | **VERIFIED** | 门禁 `P3-NO-UNAUTHORIZED-TIER` / `P3-FILTER-BEFORE-SCORE`；见 49-01 #3 |
| 5 | P4：无 key 能启动、一轮问答返回合法 Verdict，`data-policy` 载荷逐字含 `"mode":"stub"` | **VERIFIED** | 门禁 `P4-NO-KEY-STARTED` / `P4-STUB-VERDICT-VALID` / `P4-MODE-STUB`（live 腿起真实后端 + 抓 SSE）；**我的独立 fresh-clone 探针同样复现**（§4） |
| 6 | P5：citation span 真实出现在 chunk 文本里；伪造 span 被降级为 needs_input | **VERIFIED** | 门禁 `P5-GROUNDED-SPAN` / `P5-FORGED-SPAN-DOWNGRADED` / `P5-CONFIDENCE-CEILING`；`TestStub_ForgedSpanIsDowngraded` PASS |
| 7 | P6：`/api` 前缀不被静态托管遮蔽、无扩展名未命中走 SPA fallback、目录穿越被拒 | **VERIFIED** | 门禁 `P6-API-NOT-SHADOWED` / `P6-SPA-FALLBACK` / `P6-TRAVERSAL-REJECTED`（live 腿） |
| 8 | P7：`docs/demo.gif` 存在、magic 为 GIF87a/GIF89a、非空、二进制内无 API key 形态字节；`scripts/record-demo.sh` 存在 | **VERIFIED** | 门禁 `P7-GIF-MAGIC` / `P7-NO-KEY-BYTES` / `P7-RECORD-SCRIPT`（含 `grep` rc=2 必须判失败的严谨分支）；我独立复核：PIL 读出 **736×328、46 帧、211794 bytes**，逐帧目视第 20 帧为**真实终端录屏**（`status: ok` / `mode: stub` / `confidence: 0.5` / `review_required: true` / `citations: 1` / `[13] tier=authority` / 真实中文答复 + 8 个 Skill 的真实 reads 列），CJK 正常无豆腐块 |
| 9 | P9 自证：九分区各有 begin/ok 调用点、空分区副本必红、十条变异演练按预期变红、D0 基线必绿、门禁跑完不留产物 | **VERIFIED**（见 W-4） | 门禁 `P9-SELF-PROOF` / `P9-EMPTY-PARTITION-RED` / `P9-DRILLS-RED` / `P9-DOUBLE-ANNOUNCED`；我 3 次跑后 `git status --porcelain` 无 `p4.db` / `.cast` / `.skillforge-cache` 残留 |
| 10 | README hermetic 小节逐字给出 `export RETRIEVER_BACKEND=embedded`、写明仍需外部 embedding 端点、embedded 指向量存储进程内、D-02 两种端点形态、stub 三重标注 | **VERIFIED**（见 W-3） | `README.md:93-156 ## Run without Docker / without an API key`；:97 引用块 "What `embedded` means"（向量存储进程内 ≠ 向量化进程内、**no fallback**、端点不可达就大声失败）；:109-114 两种端点形态表（本地 Ollama `http://localhost:11434/v1/embeddings` 为主 / 任意 OpenAI 兼容）；:129-131 三行 export；:81-91 stub 三重标注小节；:124 `ollama pull bge-m3`；:155 `make verify-phase-49` |
| 11 | P8：milvus/mongo/none 三条既有分支未被破坏，`.env.example` 与 `docker-compose.yml` 未被修改 | **VERIFIED** | 门禁 `P8-BACKENDS-INTACT` / `P8-CONFIG-UNTOUCHED` / `P8-PROD-EMBEDDER-UNTOUCHED` / `P8-DEPS-UNCHANGED`；**我独立于门禁复核**：`git log --oneline 1838bad..HEAD -- .env.example docker-compose.yml Dockerfile.backend Dockerfile.frontend .github/workflows go.mod go.sum web/package.json web/package-lock.json internal/embedder/bge internal/server/pipeline.go` → **空输出**（零新依赖、生产 embedder 与既有配置一字未动） |

---

## 3. 成功标准（ROADMAP SC1–SC5）

| SC | 判定 | 证据 |
|---|---|---|
| **SC1** 全新 clone + README quickstart（无内网、无 API key）服务可启动，chat 完成一轮问答并返回结构化结果 | **VERIFIED** | 我的独立探针：`git clone --depth 1` → clone 内 `go build ./cmd/server` → 无任何 `*_API_KEY`、真实 embedding 端点启动 → `READY after 186s`（冷索引 218 篇）→ `GET /api/skills` = 200 → `POST /api/chat` = 200，SSE 19 帧，含 `data-subprogress{step:retrieve,tier:authority}`、`{step:found_candidates,count:5}`、`{step:generate}` 与 `text-delta-draft` 携带的**结构化 Verdict**（见 §4 逐字输出）。**注意边界**：按 D-01/D-02 仍需一个外部 embedding 端点（本机为本地 Ollama 或任意 OpenAI 兼容 `/v1/embeddings`），"无内网"成立、"无预装 BGE"**不成立**（见 W-6） |
| **SC2** embedded 模式返回可引用 chunk，tier 隔离 fail-closed 不削弱 | **VERIFIED** | 49-01 #2/#3/#4；探针实测引用 `{"chunk_id":"13","tier":"authority","span":"员工出差住宿费超过标准额度时，200 元以内的超标部分可由部门负责人审批；…"}` —— tier 白名单 + `effective_to` 过滤发生在打分**之前**（`tier_isolation.go` 不含任何打分符号），`none` 后端的 default-deny 语义未被改动 |
| **SC3** 无 API key 时走 stub 路径，日志/响应如实标注，不静默伪装真模型 | **VERIFIED** | 三处标注全部实测命中：启动 `[WARN] cmd/server: STUB LLM MODE …`；wire `"mode":"stub"`；README:81-91。stub 的引用与真模型走**同一条** validate→grounding 链（`pipeline.go` 零改动，stub 插在 `Deps.PipelineChatModel`），confidence ≤0.5 + `review_required=true` 由 stub 自身保证；伪造 span 会被降级（`TestStub_ForgedSpanIsDowngraded`） |
| **SC4** quickstart 在 hermetic 模式下逐步实测通过，录制产出真实 demo.gif 素材 | **VERIFIED**（素材本体）／**部分 HUMAN**（观感与体积，见 §6） | `docs/demo.gif` = 736×328 / 46 帧 / 211794 bytes，内容为真实运行录屏（我逐帧目视）；`scripts/record-demo.sh` 硬约束：unset 全部 `*_API_KEY`、只读 `.env` 的 `BGE_M3_URL` 一行、**禁止替身端点**、无可用渲染器时非零退出 `no renderer available`、cast 无 output 事件时 `refuse to render an empty animation`、中间产物落临时目录并 trap 清理。quickstart 的**后端段**已由我的 fresh-clone 探针实测通过 |
| **SC5** `bash verify-phase-49.sh` exit 0（含 hermetic 模式端到端） | **VERIFIED** | 3 次运行全部 exit 0；`=== Phase 49 Gate: ALL PASSED ===` 每次恰好 1 条；P1–P9 九分区齐活、29 个证据 token；10 条变异演练 D0..D9 逐条验证"删掉某条断言 → 副本必须变红并点名分区" |

---

## 4. 独立复现：全新 clone → 真实 embedding 端点 → 一轮带引用问答

我的探针写在**仓库外**（`C:\Users\CPIC\AppData\Local\Temp\p49-verify\probe-fresh-clone.sh`），只往临时目录写产物，不改仓库任何文件。

```
== 0. embedding endpoint reachability ==
embed_http=200
embed_model=mixedbread-ai/mxbai-embed-large-v1 dim=1024
== 1. fresh clone of HEAD ==
a73f623 docs(49): close the phase with a passed verification, a 32-row threat register, and a validated Nyquist contract
skills_dir=present knowledge_md=218 web_dist=absent
== 2. build in the clone ==
build ok
== 3. start with NO api key, embedded backend, real embedding endpoint ==
READY after 186s (http 200)
== 4. startup stub banner ==
2026/10/22 21:46:28 [WARN] cmd/server: STUB LLM MODE — no LOCAL_API_KEY / GLM_API_KEY configured; every answer is
assembled by the deterministic extractor (internal/server/stubchat) from retrieved chunk text, generated by NO LLM;
confidence is capped at 0.5 and review_required is always true; each response payload carries mode="stub"
== 5. GET /api/skills ==
skills_http=200 bytes=5079
== 6. POST /api/chat (one grounded round, no api key) ==
chat_http=200 bytes=2943
== 7. SSE frames ==
data_frames=19
data: {"type":"data-subprogress","data":{"step":"retrieve","tier":"authority"}}
data: {"type":"data-subprogress","data":{"step":"found_candidates","tier":"authority","count":5}}
data: {"type":"data-subprogress","data":{"step":"generate"}}
data: {"type":"text-delta-draft","id":"draft-1","delta":"{\"status\":\"ok\",\"confidence\":0.5,\"citations\":[{\"chunk_id\":\"13\",\"s"}
== 8. verdict assertions ==
verdict= {"status": "ok", "confidence": 0.5, "citations": [{"chunk_id": "13", "span": "员工出差住宿费超过标准额度时，200 元以内的超标部分
可由部门负责人审批；超过 200 元的部分须附书面情况说明，并经分管副总审批。", "tier": "authority"}], "review_required": true,
"answer": "员工出差住宿费超过标准额度时，200 元以内的超标部分可由部门负责人审批；超过 200 元的部分须附书面情况说明，并经分管副总审批。",
"mode": "stub"}
  PASS  status in valid set        PASS  confidence <= 0.5      PASS  mode == stub
  PASS  review_required true       PASS  citations is list       PASS  citations non-empty
  PASS  citation carries tier      PASS  span non-empty
== 9. SPA fallback in the fresh clone (no web/dist yet) ==
root_http=503 root_mentions_build=1
api_missing_http=503
== 10. leftovers in the clone (must be none) ==
（空 —— clone 内 `git status --porcelain` 无输出）
```

**探针自身唯一的 FAIL 是我的断言写严了**（我额外要求 citation 带 `source` 字段，而 `verdict.Citation` 的字段就是 `{ChunkID, Span, Tier}`——`source` 属于 retriever 层的 chunk 元数据，不属于 wire citation）。这是**探针 bug，不是产品缺陷**。

---

## 5. Gaps（BLOCKER）

**无。** 34 条 must-have 全部 VERIFIED；无 MISSING/STUB 产物；无未接线的 key link；无阻断级反模式。

---

## 6. Warnings（非阻断，需记录）

> **Post-verification disposition（2026-10-23 收尾补记）**：本报告定稿后，以下 warnings 已在本 phase 内处置——
> **W-1 已修复**（提交 `216620b`：`/api` 闸前移到 dist 检查之前 + `TestStaticHandlerMissingDistStill404sUnregisteredAPIPaths`；门禁修复后连跑两次 ALL PASSED）；
> **W-2 已修复**（`TestModeOmitEmptyKeepsRealModelWireUnchanged`：零值 Verdict marshal 无 `"mode"` 键、stub marshal 出字面 `"mode":"stub"`）；
> **W-3 已修复**（门禁 P4 新增 `P4-README-LABEL` 四条 README grep + 变异演练 **D10**；第 4 次连跑 ALL PASSED，`ok_partition "P4"` 扩为 4 token）；
> **W-6 部分处理**（ROADMAP Phase 49 段新增 **Scope Note** 记录实施口径——SC1 实施为「无需内网 BGE 端点」、SC2 实施为「本地向量存储模式」、SC3 三重标注；**不改验收标准原文**，措辞修订按本报告建议留给 Phase 50）；
> W-4/W-5/W-7/W-8 按下表建议接受并记录（处置账本见 `49-REVIEW-DISPOSITION.md`，评审工件见 `49-REVIEW.md`）。

| # | 警告 | 证据 | 建议 |
|---|---|---|---|
| **W-1** | **`web/dist` 缺失时，未注册的 `/api/*` 路径返回 503（"web/dist not found"）而不是 404。** `static.go:75` 的 dist 存在性检查位于 `:88` 的 `/api` 闸**之前**，故缺 dist 时所有未匹配请求（含 `/api/unknown`）都走 `serveMissingDist` | 我的 fresh-clone 探针：`api_missing_http=503`（clone 内无 `web/dist`）；而 `web/dist` 存在时 `/api/unknown` → 404（`TestStaticHandlerAPIPrefixNeverFallsBackToHTML` PASS，门禁 `P6-API-NOT-SHADOWED`） | 修法很小：把 `/api` 闸移到 dist 检查之前，或在 `serveMissingDist` 里放行 `/api` 前缀。**不影响 SC1–SC5**（已注册 API 路由在缺 dist 时照常工作：实测 `skills_http=200`、`chat_http=200`），仅影响"前端未构建时未注册 API 的 404 语义" |
| **W-2** | 49-02 truth ⑧ 声明的 Go 侧机械判据（marshal 零值 Verdict 后断言无 `mode` 键）**没有以 Go 测试落地**：`internal/verdict/verdict_test.go`（141 行）无任何 mode 用例 | `grep` 确认 verdict_test.go 无 mode 相关断言；行为本身已被我独立实测 + 前端 `verdict.test.ts` 等价用例覆盖 | 补一条 Go 断言，防止将来有人给真模型路径写 `Mode` 时无声回归 |
| **W-3** | 门禁**不做 README 内容断言**：`grep README/quickstart` 在 `verify-phase-49.sh` 中只命中注释与 `docs/demo.gif` 引用行 ⇒ 49-04 truth ⑩ 完全靠人工复核 | 我已逐行人工复核通过（README.md:93-156 等），但回归无护栏 | 在门禁加一条 README 关键行断言（`export RETRIEVER_BACKEND=embedded`、两种端点形态、stub 三重标注） |
| **W-4** | 49-04 truth ⑨ 字面要求"门禁跑完后 `git status --porcelain` 为空"，实现用的是**前后差集**（`comm -13` + 过滤 `skillforge.db|p4.db|.cast|.skillforge-cache`），脚本注释写明"只看差集，开发机上既存的无关脏文件不会把健康树打成假红" | `verify-phase-49.sh` `partition_P9` 第 (4) 项 | 语义更强，非缺陷；但真值措辞与实现不一致，属文档对齐项 |
| **W-5** | **D-08 字面落点未改**：决策要求"前端 `web/src/components/chat/MessageBubble.tsx` 的 `isVerdict` 守卫需一并扩展"，实际实现改为 `web/src/types/verdict.ts` 新增独立 `isStubVerdict` + `VerdictCard.tsx` 两处 `data-testid="stub-mode-badge"` 徽标；`MessageBubble.tsx` 与 `DataPolicyPart.tsx` 在本 phase 的 5 个提交里**未改动** | `git log --oneline 1838bad..HEAD -- web/src/components/chat/MessageBubble.tsx web/src/components/chat/parts/DataPolicyPart.tsx` 为空；`MessageBubble.tsx:56` 仍只 import `isVerdict`；徽标链路 `MessageBubble.tsx:38/:88` → `DataPolicyPart.tsx:49/:66` → `VerdictCard.tsx:31/:93/:131/:181` 真实存在 | 49-02-PLAN 已把它登记为 flagged_assumption（"MessageBubble 的消费留给后续 slice"）⇒ 是**已登记的偏离**而非缺陷；D-08 的目标（用户可见 stub 标注）已达成且有组件测试兜底 |
| **W-6** | **ROADMAP / REQUIREMENTS 的措辞与锁定决策 D-01/D-02 不一致，且 `49-CONTEXT.md` 登记的"回 ROADMAP 修订措辞"至今未落地。** ROADMAP SC1 仍写"无预装 Milvus/BGE"，SC2 与 DEMO-01 仍写"embedded 向量**兜底**模式 / 无需外部 Milvus/BGE 即可检索"；而实现按 D-01（强制外部向量化模型、**明确不做任何兜底**）与 D-02（默认本地 Ollama）落地，`BGE_M3_URL` 在 stub 模式下**仍为必填**（`cmd/server/main.go` `envRequired("BGE_M3_URL")`），README.md:81-91 亦明写"stubbed is the LLM, not the retriever" | `.planning/ROADMAP.md:191-193` 与 `.planning/REQUIREMENTS.md:23` 原文；`49-CONTEXT.md` `<deferred>` 块已写明应改为"无需内网 BGE 端点"/"本地向量存储模式"；`a73f623` 只勾选了 checkbox，**没有改措辞** | **建议 Phase 50（开源门面，README 修真）顺手改掉 SC1/SC2/DEMO-01 的措辞**，否则读者会以为"连 BGE 都不需要"。**这不是本 phase 的实现缺陷**（实现严格遵循了锁定决策），但它是当前最容易被误读的一处文档债 |
| **W-7** | 门禁的 live 腿使用**测试专用 embedding 替身**（0.9 共享基底 + 0.1 token 哈希残差），与 D-01"不使用任何替身"的字面表述冲突；替身只 bind 127.0.0.1、只注入子进程环境、生产二进制零改动、门禁打印 `using test-only embedding double` | `verify-phase-49.sh` live 驱动段；`internal/embedder/bge` 未被修改（git log 为空） | 49-04-PLAN 已把它登记为 flagged_assumption 并要求人工复核。**该边界由我的独立 fresh-clone 探针用真实模型端点补上了证据**（§4）⇒ 该 warning 的风险面已收敛 |
| **W-8** | 门禁不构建前端：`web/dist/index.html` 缺失时 P6 明确变红（`FAIL P6: web/dist/index.html is absent — the live SPA assertions cannot run (build the frontend first)`） | `verify-phase-49.sh:547/:564` | 全新 clone 未 `npm run build` 时门禁会红（**不是静默假绿**）。若把"门禁即 CI 门"用起来，需在 CI 里先 build 前端或把该断言降级为跳过 |

---

### Decision Coverage

`node "$HOME/.agents/gsd-core/bin/gsd-tools.cjs" query check.decision-coverage-verify ".planning/phases/49-hermetic-demo" ".planning/phases/49-hermetic-demo/49-CONTEXT.md"`

```json
{"skipped": false, "blocking": false, "total": 20, "honored": 20, "not_honored": [],
 "message": "All trackable CONTEXT.md decisions are honored by shipped artifacts."}
```

20/20 决策被遵守（该门 non-blocking）。三处**形态层面**的落地差异已分别记录：W-5（D-08 的前端落点）、W-6（D-01/D-02 与 ROADMAP 措辞）、W-7（门禁替身端点与 D-01 字面表述）。`verification-overrides.md` 中**没有** phase 49 的任何 override ⇒ `overrides_applied: 0`。

---

## 7. Human Verification（3 项，未完成）

`49-VALIDATION.md:91-99` 自带的 Manual-Only 表列了三项；按 verifier 规则（phase 目标明确面向外部用户 ⇒ 用户可感知项必须人工过一遍），三项都成立：

| # | 人工项 | 为什么必须是人 | 我的机器已覆盖到哪 |
|---|---|---|---|
| **H-1** | 有真实 GLM key 时的真模型路径（stub 不介入）：确认响应**无** `mode` 字段且引用来自模型 | 需要真实的外部付费 key，本机没有；stub 分支之外的 `toolChatModel` 分支无自动化覆盖（门禁只跑 stub 路径） | 代码路径已读：`cmd/server/main.go:308`（key 惰性读取，语义等价于原 `envRequired`）、:538-540、:605-608（stub 时 `toolChatModel=nil`）、:848（三分支 switch）、:1116-1117、:1136、:1155。**运行时未验** |
| **H-2** | README Quick Start "照着念就能跑"（含前端 `npm install` / `npm run dev` / `npm run build` 与浏览器交互） | 文档可用性 + 浏览器端交互是人的判断 | **后端段已独立实测**（fresh clone → 无 key → 真端点 → 一轮带引用问答，§4）。**前端段未验**：我未跑 `npm run dev`（:5173 代理 `/api`→:8888），也未在浏览器里确认 STUB 徽标呈现（徽标逻辑本身有 vitest 覆盖：`VerdictCard.test.tsx` 20 tests passed，其中 2 条 49-02 新用例断言两个渲染面都有/没有 `stub-mode-badge`） |
| **H-3** | `docs/demo.gif` 的观感与体积（是否适合放进 README 首屏） | 视觉判断无法用断言表达 | 我已逐帧抽查（736×328 / 46 帧 / 211794 bytes，内容为真实终端录屏，CJK 正常无豆腐块），但"好不好看 / 211KB 是否够小"是人的判断 |

**因此本 phase 的最终状态是 `human_needed` 而不是 `passed`**：实现层（34/34 must-have）已闭合，剩余的是 3 项人工验收。`a73f623` 已把 DEMO-01/02/03 勾为 Complete、把 Phase 49 勾为完成，这与本报告不冲突（实现完成 ≠ 人工验收完成）。

---

## 8. Regression

| 回归面 | 结果 |
|---|---|
| Go（WSL2，linux/amd64） | `go test ./internal/retriever/embedded/... ./internal/server/stubchat/... ./internal/webui/... ./cmd/server/... -count=1` → 四包全 `ok`（embedded 10.062s / stubchat 0.017s / webui 5.144s / cmd/server 0.084s）；另单独实跑 webui 三个关键用例（穿越+符号链接 / 缺 dist 503 / 200 并发）→ 全 `--- PASS`，**无 SKIP** |
| 前端（Windows 原生 vitest） | `src/types/verdict.test.ts` → **6 tests passed**；`src/__tests__/VerdictCard.test.tsx` → **20 tests passed**（含 2 条 49-02 徽标用例） |
| 依赖面 | **零新增**：go.mod / go.sum / web/package.json / web/package-lock.json 在 `1838bad..HEAD` 内无任何提交 |
| 既有后端 | milvus / mongo / none 三分支未被破坏（门禁 P8 + `TestBuildRetrieverNoneBranchUnchanged`） |
| 既有生产组件 | `internal/embedder/bge`、`internal/server/pipeline.go` 均**未被本 phase 修改**（git log 为空） |
| 门禁幂等 | 3 次连续运行 exit 0、每次恰好 1 条 ALL PASSED；跑完不在仓库留产物（无 `p4.db` / `.cast` / `.skillforge-cache`） |

---

## 9. 结论

- **must-haves 34/34 VERIFIED**，`behavior_unverified = 0`，`overrides_applied = 0`，**无 BLOCKER gap**。
- **SC1–SC5 全部成立**，其中 SC1 由我在**全新 clone + 真实 embedding 模型端点 + 零 API key**条件下独立复现（fresh clone → build → 186s 冷索引 → `/api/skills` 200 → `/api/chat` 200 → 结构化 Verdict 带 `tier=authority` 引用且 `mode:"stub"` / `confidence 0.5` / `review_required true`）。
- 唯一让状态停在 `human_needed` 的是 **3 项人工验收**（真实 GLM key 路径 / README 前端段 / GIF 观感），而非实现缺陷。
- 需要上层动作的三件事：
  1. **执行或显式接受 H-1/H-2/H-3**（其中 H-1 需要真实 key，H-2 需要浏览器）。
  2. **改 ROADMAP SC1/SC2 与 DEMO-01 的措辞**（W-6）：实现按 D-01/D-02 要求一个外部 embedding 端点，"无预装 BGE""向量兜底模式"的字面说法与实现不符；`49-CONTEXT.md` 已登记为 deferred，最适合在 Phase 50 一并修订。
  3. 修 W-1（缺 dist 时 `/api` 应 404 而非 503）——小改动，不阻断本 phase。
