---
phase: 49-hermetic-demo
plan: 01
subsystem: embedded 进程内向量检索后端（tier 隔离 + 本地向量缓存）
tags: [embedded-retriever, tier-isolation, vector-cache, hermetic-demo, wave-1]
requires:
  - internal/retriever/retriever.go（Retriever / IndexerBackend 接口契约）
  - internal/retriever/mongo/rank.go（rank 纯函数先例，逐字 tie-break）
  - internal/vecmath（Cosine 全仓唯一相似度实现）
  - internal/retriever/mongo/seed.go（解耦 seed 先例）
  - internal/kb（Registry / validateTiers）
provides:
  - RETRIEVER_BACKEND=embedded（进程内向量存储，零外部数据库）
  - internal/retriever/embedded 包（8 个 Retriever 方法 + IndexerBackend + seed + 缓存）
  - SFVC 向量缓存格式（magic/version/model/dim/docCount/corpusHash/endpointHash/payloadHash 八项全字段校验，任一不符整份作废）
  - knowledgeDirForSeed 包级变量（cmd/server 测试可注入 t.TempDir()）
affects:
  - cmd/server/main.go（buildRetriever 与 indexerBackend 的新分支）
  - 49-02/49-03/49-04（依赖本后端可用）
actuals:
  tokens: 112000
  tasks: 3
  commits: 0
tech-stack:
  patterns:
    - 两段式 tier 隔离：candidates() 先按 tiers+effective_to 构造候选集，rank() 只收候选集，结构上杜绝后置过滤（Pitfall 11）
    - 缓存全字段校验、任一不符整份作废；语料哈希基于内容绝不用 mtime（Pitfall 8）
    - O_CREATE|O_EXCL 原子锁 + 轮询 + 超时明确报错的并发首启（绝不降级假向量，D-01）
    - 源码 lock 测试（caseBlock 嵌套感知取块）锁装配分支存在性与既有分支零漂移
key-files:
  created:
    - internal/retriever/embedded/embedded.go
    - internal/retriever/embedded/tier_isolation.go
    - internal/retriever/embedded/rank.go
    - internal/retriever/embedded/cache.go
    - internal/retriever/embedded/seed.go
    - internal/retriever/embedded/embedded_test.go
    - internal/retriever/embedded/tier_isolation_test.go
    - internal/retriever/embedded/rank_test.go
    - internal/retriever/embedded/cache_test.go
    - cmd/server/backend_switch_test.go
  modified:
    - cmd/server/main.go
key-decisions:
  - "缓存头部从 6 项扩到 8 项（加 corpusHash + endpointHash）：缓存文件名固定为 vectors.bin，key 必须靠头部自校验兜住，否则『换语料但文件名不变』会静默复用上一代向量 —— 这是本 cache 最危险的失效模式"
  - "缓存不存正文（Content），命中后由 hydrateContent 按 Source 零网络重读回填：缓存只省 218 次 embedding 网络调用，正文与向量必须同代；回填失败即整份作废，绝不留空正文的命中"
  - "readCache/writeCache 的散参数收敛为 cacheSpec 结构体：扩到 8 项校验后漏传一个要素会让某类失效静默通过，结构体让编译器替我们盯住这组要素"
  - "acquireCacheLock 超时后明确报错、绝不降级为假向量继续启动（D-01 零兜底）：一个会用假向量回答合规问题的进程，比一个起不来的进程危险得多"
  - "indexerBackend switch 必须加显式 case『embedded』：落进 default 会对 *milvus.MilvusClient 做类型断言，是 runtime panic 而非编译错，单元测试完全看不见"
  - "seed 的 embed 失败走 log.Fatalf 而非 WARN：embedded 没有第二数据源，WARN 会让服务以空语料『健康』运行，违反 D-01 明确失败原则"
  - "backend_switch_test.go 的 glmKey lock 只锁现状并注明『归 49-02 改造』：让下一次改动有据可查、有门可过，而不是被动阻碍后续 plan"
---

## Accomplishments

**Task 1（tracer）— embedded 包骨架与 8 个 Retriever 方法**

- `embedded.go`：`Config{Embedder, Model, Dimensions, CacheDir, EndpointURL, Logf, KnowledgeDir, Registry, CacheWaitTimeout}`、`Client`（`sync.RWMutex` 护 `docs []row` + `byID map[int64]int`）、`ErrDisconnected`、`New()`（校验 `Dimensions > 0` 且必须等于 `Embedder.Dimensions()`，`MkdirAll` 0o700）。`var _ retriever.Retriever = (*Client)(nil)` 与 `var _ retriever.IndexerBackend = (*Client)(nil)` 在编译期钉住接口完备性。
- `tier_isolation.go`：`tierAllowed`（空即 false、大小写敏感精确匹配）、`effectiveAllowed`（`effectiveTo == 0 || effectiveTo >= now`）、`candidates`（**首条语句** `if len(tiers) == 0 { return nil }`）。
- `rank.go`：包内 `row` / `ranked` 类型 + 纯函数 `rank(rows []row, queryVec []float32, k int) []retriever.Hit`，逐字照搬 `internal/retriever/mongo/rank.go:94-132`：三前置守卫 → 跳过维度不符/空 embedding → `sort.SliceStable`（score desc, id asc）→ k 截断 → 映射 Hit。相似度只调 `vecmath.Cosine`。
- 语义对齐 mongo/milvus：`Search` 的 `topK <= 0` / 空 queryVec 报错；`GetChunk` 未命中返回 `retriever.ErrChunkNotFound`（`errors.Is` 可判）；`GetMaxID` 空/未知 tier 返 `0, nil`；`ListMetadata` 不过滤 tier 与 effective 窗口（按 ID 升序）；`Ensure` 幂等；`Disconnect` 后所有方法返回 `ErrDisconnected`。
- **tier 隔离有两段式的结构性证明**：`rank` 的签名里没有 `tiers` 参数 —— 它物理上无法做 tier 判定。`TestSearchNeverScoresExcludedTiers` 用"被禁 chunk 本该 cos=1.0 排第一"的 queryVec 断言 top-1 是合法行；`TestSearchCandidateSetEmptySkipsRank` 断言候选集为空时 `rank` 根本不被调用。Pitfall 11 被封死成结构约束，不是注释里的承诺。

**Task 2 — 内容寻址向量缓存 + 自写解耦 seed**

- `cache.go`：`cacheKey(model, dim, corpusHash, endpointHash)`、`corpusHash(entries, read)`（写 `relPath \x00 len \x00 bytes`，**完全不读 mtime**）、`readCache`/`writeCache`（`cacheSpec` 结构体）、`acquireCacheLock`（`O_CREATE|O_EXCL` + 200ms ticker + 超时明确报错）。
- 二进制格式（头部顺序 load-bearing）：`magic "SFVC"[4] | version u32 | modelLen u16 + model | dim u32 | docCount u32 | corpusHash[32] | endpointHash[32] | payloadHash[32] | 每条: id i64 | tierLen u16+tier | sourceLen u16+source | effectiveFrom i64 | effectiveTo i64 | vec(dim × float32 LE)`。
- 写入走 `tmp → Flush → Sync → Rename`，任何提前返回都由 `fail` 闭包清掉 `.tmp`（"存在即完整"成立）。读取逐项校验 8 个头部字段，任一不符返回 `errCacheInvalid`，**绝不部分可用**。
- `seed.go`：`SeedFromKnowledgeDir(ctx, knowledgeDir, emb, reg)`（照 `mongo/seed.go:68-158` 先例自写，**未改** `internal/knowledge/indexer.go` 的 milvus 耦合签名）。流程：`docs` 非空短路 → nil embedder 报错（`"no in-process or hash fallback"`）→ nil registry fail-closed → 抢锁 → `WalkRegisteredDirs` + `WalkDocs` → `ReadFile` + `stripFrontmatter` → `emb.Embed`（**错误即 return，不留半份语料**）→ ID 1-based → `Source = ToSlash(Rel)` → `validateDocs` → `install` → 写缓存。`SetSeedAuditHook` 与 mongo 同形。
- 缓存命中路径零网络：`Ensure` 算语料 hash → `readCache` 命中 → `hydrateContent` 按 Source 从语料重读正文 → `install`。`SeedFromKnowledgeDir` 首条语句随即短路，218 次 embedding 全免。

**Task 3 — 启动装配接线 + 源码级回归 lock**

`cmd/server/main.go` 只动 plan 允许的几处：import 组加 `internal/retriever/embedded`（字母序，在 `retriever` 与 `retriever/milvus` 之间）；`buildRetriever` 加 `case "embedded":`（`embedded.New(...)` → `ec.Ensure(ctx)` → `ec.SeedFromKnowledgeDir(...)`，失败 `log.Fatalf`，成功打 `[OK] embedded store ready (docs=%d cache=%s)`，返回 `ec, "embedded(" + cacheDir + ")"`；分支内**不含** `MONGO_URI` / `MILVUS_ADDRESS` / 任何 `envRequired(`）；`indexerBackend` switch 加 `case "embedded": indexerBackend = r.(*embedded.Client)`；包级变量 `var knowledgeDirForSeed = "knowledge/"`。**未动** `glmKey` 行（归 49-02），**未动** `internal/knowledge/indexer.go`。

新建 `cmd/server/backend_switch_test.go`（`package main`，四个测试，零网络零 env 不启动进程，手法沿用 `wiring_lock_test.go` 的源码 lock）：

- `TestBackendSwitchEmbeddedHasExplicitCase`：switch 含 `case "embedded":`、该分支含 `embedded.Client` 且**不含** `milvus.`、`default:` 仍含 `milvus.NewIndexerBackend(r.(*milvus.MilvusClient))`。
- `TestBuildRetrieverEmbeddedCaseNeedsNoExternalDB`：embedded 分支含 `embedded.New(` / `SKILLFORGE_VECTOR_CACHE_DIR` / `SeedFromKnowledgeDir(` / `.Ensure(`，且不含 `MONGO_URI` / `MILVUS_ADDRESS` / `envRequired(`。
- `TestBuildRetrieverNoneBranchUnchanged`：`case "none":` 仍含 `none(degraded default-deny)` 与 WARN；`case "mongo":` 仍含 `MONGO_URI`；`default:` 仍含 `MILVUS_ADDRESS`。
- `TestGlmKeyLineStillEagerFor49_02`：`envRequired("GLM_API_KEY")` 恰好 1 次且同行含 `envOr("LOCAL_API_KEY"`；`BGE_M3_URL` 仍由 `envRequired` 取。注释明确写出"归 49-02 改造，改完同步更新本测试"。

## Task Commits

本 plan 自身**不做任何 git 操作**（`actuals.commits: 0`）。编排者已统一提交为 `b7e0d8d feat(49-01): embedded in-process vector retriever with two-stage tier isolation and SFVC cache (DEMO-01)`（13 个文件：10 个 embedded 源/测试 + `cmd/server/main.go` + `cmd/server/backend_switch_test.go` + 本 SUMMARY）。

该提交之后工作区仍有两处**本 plan 的**未提交改动（均为收尾期的注释/文档润色，不影响行为）：

- `internal/retriever/embedded/rank.go`：注释里 `vecmath.Cosine` 字面量改为"调用 vecmath 包的 Cosine"（见 Deviation 9）。
- `.planning/phases/49-hermetic-demo/49-01-SUMMARY.md`：本文件本体。

## Files Created / Modified

**新建（10）**

| 文件 | 内容 |
|---|---|
| `internal/retriever/embedded/embedded.go` | Config / Client / New / 8 个 Retriever 方法 / ErrDisconnected / prepareCachePaths / computeCorpusHash / hydrateContent |
| `internal/retriever/embedded/tier_isolation.go` | tierAllowed / effectiveAllowed / candidates（唯一 PRE-filter 点） |
| `internal/retriever/embedded/rank.go` | row / ranked 类型 + 纯函数 rank |
| `internal/retriever/embedded/cache.go` | cacheKey / corpusHash / cacheSpec / readCache / writeCache / acquireCacheLock / SFVC 格式 |
| `internal/retriever/embedded/seed.go` | SeedFromKnowledgeDir / install / validateDocs / stripFrontmatter / SetSeedAuditHook |
| `internal/retriever/embedded/embedded_test.go` | 10 个行为契约测试 + fakeEmbedder + vec 辅助 |
| `internal/retriever/embedded/tier_isolation_test.go` | 8 个测试：空 tier / 非白名单 tier / 过期 effective_to / 过滤先于打分 |
| `internal/retriever/embedded/rank_test.go` | 4 个测试：排序确定性、tie-break、维度守卫、topK 守卫 |
| `internal/retriever/embedded/cache_test.go` | 13 个测试：key 要素、mtime 免疫、8 项头部校验、并发首启、seed 失败无半份语料 |
| `cmd/server/backend_switch_test.go` | 四道源码级装配 lock（caseBlock / firstStop / switchBlock 辅助） |

**修改（1）**：`cmd/server/main.go`（import + `buildRetriever` 的 `case "embedded":` + `indexerBackend` 的 `case "embedded":` + 包级 `knowledgeDirForSeed`）

共 11 项，全部落在 plan 的 `files_modified` 清单内，无越界文件。

**明确未触碰**（已核验）：`internal/knowledge/indexer.go`、`glmKey` 行、`.planning/STATE.md`、`.planning/ROADMAP.md`、`49-CONTEXT.md`、`49-RESEARCH.md`、`go.mod`、`go.sum`、`web/package.json`、`web/package-lock.json`。

## Deviations

- **Deviation 1（修源码，不迁就测试）**：executor 首版把 `indexerBackend` 的 embedded case 注释写成含 `r.(*milvus.MilvusClient)` 字面量，导致 `TestBackendSwitchEmbeddedHasExplicitCase` 假阳性 FAIL（case-body 扫描把注释也算进去）。修法：改注释措辞（"对 MilvusClient 做类型断言"），保留测试的严格扫描 —— 不缩小扫描范围来迁就注释。
- **Deviation 2（Rule 2）**：缓存在 model+dim+corpusHash 之外增加 endpointHash（`BGE_M3_URL` 的 sha256）。缓存文件名固定为 `vectors.bin`，key 必须靠头部自校验兜住；换端点后静默复用旧向量是最危险的一类失效。原始 URL 绝不落盘（只存 sha256）。
- **Deviation 3（Rule 2）**：缓存头部从 6 项扩到 8 项，把 corpusHash 与 endpointHash 也写进头部（而非只参与 cacheKey 的文件名）。理由同上：文件名固定，无法靠文件名区分"换了语料"。
- **Deviation 4（Rule 1）**：缓存**不存正文**（Content），命中后由 `hydrateContent` 按 Source 零网络回填；回填失败即整份缓存作废。缓存只负责省掉 218 次 embedding 网络调用，正文与向量必须来自同一代语料 —— 留空正文的命中在 UI 上表现为"命中但无内容"，比报错更难排查。副作用：缓存文件从 ~1.05MB 降到 ~0.9MB（1024 × 4 × 218 ≈ 893KB）。
- **Deviation 5（Rule 3）**：`readCache` / `writeCache` 的散参数收敛为 `cacheSpec` 结构体。扩到 8 项校验后散参数需要 5 个实参，任一漏传都会让某类失效静默通过；结构体让编译器盯住这组要素。
- **Deviation 6（Rule 3）**：`backend_switch_test.go` 新增 `caseBlock` / `firstStop` / `switchBlock` 三个包内辅助。`wiring_lock_test.go` 的 `structLiteral` 取的是 `{` 匹配块，而这里要取"到下一个同级 case 为止"的分支，语义不同，无法复用。
- **Deviation 7**：seed 的 embed 失败选择 `log.Fatalf`（plan 允许 WARN 继续）。embedded 没有第二数据源，WARN 会让服务以空语料"健康"运行，违反 D-01 的明确失败原则。
- **Deviation 8（测试实现细节）**：`fakeEmbedder` 用指针接收器才满足 `embedder.Embedder`（值接收器导致编译错）；`cache_test.go` 定位仓库根用 `filepath.Abs("../../..")`（包在 `internal/retriever/embedded` 下，不是 `../..`）。
- **Deviation 9（注释措辞）**：`rank.go` 注释里原本写了 `vecmath.Cosine` 字面量，使"全仓只有一处 cosine 调用点"的 grep 核验变成 2 次。改成"必须调用 vecmath 包的 Cosine"后核验为 1。**这只是注释，代码路径未变**；真正防复制的是 `grep -c 'func Cosine' internal/retriever/embedded/*.go` = 0。该改动提交时仍在工作区（未纳入 b7e0d8d）。

## Verification

Task 3 的三条 `<automated>` 实跑结果（全部走 WSL）：

| 命令 | 结果 |
|---|---|
| `go test ./cmd/server/ -run 'TestBackendSwitch\|TestBuildRetriever\|TestGlmKeyLine' -count=1 -v` | **4/4 PASS**，`ok github.com/skillforge/skillforge/cmd/server 0.040s` |
| `go test ./internal/retriever/embedded/ -count=1 && go vet ./internal/retriever/embedded/ ./cmd/server/` | **ok 9.4s**；vet exit 0（无输出） |
| 依赖清单 diff（go.mod / go.sum / web/package.json / web/package-lock.json） | **UNCHANGED**（零新增依赖） |

补充核验：

- `go test ./cmd/server/ -count=1` → `ok ... 0.423s`（含既有 `wiring_lock_test.go` 的两道 lock，零回归）
- `go test ./internal/retriever/embedded/ -count=1` → `ok ... 28.716s`（全量重跑，含真实语料树哈希与缓存竞态用例）
- `go test ./internal/retriever/... ./internal/knowledge/ -count=1` → embedded / milvus / mongo / knowledge **4 包全 ok**（既有后端零漂移）
- `go build $(go list ./... 排除 stubchat 与 kpi/wsl)` → **BUILD_ALL_ELSE_OK**（55 包）；`go vet` 同集合 → **无输出**（exit 0）
- `grep -c 'vecmath.Cosine' internal/retriever/embedded/rank.go` → **1**；`grep -c 'func Cosine' internal/retriever/embedded/*.go` → **0**（全仓唯一 cosine 实现未被复制）
- `grep 'math/rand\|rand\.\|hashVector\|deterministicVector' internal/retriever/embedded/` → **空**（D-01 零兜底）
- `find knowledge -name '*.md' | wc -l` → **218**（与 plan 一致，不是 220）

> **关于 `go build ./...` 的一条如实说明**：本 plan 收尾时 `go build ./...` 仍报错，但错误**不在本 plan 的文件里**，而在 `internal/server/stubchat/stub.go`（该目录当前是 untracked，正由同 wave 的另一个 plan 并发编写中）：
> ```
> internal/server/stubchat/stub.go:46:2: "github.com/skillforge/skillforge/internal/verdict" imported and not used
> internal/server/stubchat/stub.go:103:16: not enough arguments in call to sw.Send
> 	have (*schema.Message)
> 	want (*schema.Message, error)
> ```
> 本 plan 的 12 个包（`internal/retriever/...`、`internal/knowledge/`、`internal/vecmath/`、`internal/embedder/...`、`internal/kb/`、`cmd/server/`）**全部 BUILD_OK / VET_OK**；除 stubchat 与无 Go 源文件的 `internal/kpi/wsl` 之外，其余 55 个包也全部通过 build 与 vet。未触碰、也不应触碰 stubchat（属其它 plan 的 files_modified）；STUB-01 落地后 `go build ./...` 自会恢复。

35 个顶层测试函数（+15 子测试）全绿，其中行为层的关键守卫：

- **tier 隔离**：`TestSearchNeverScoresExcludedTiers`（被禁 chunk 与 queryVec 精确同向 cos=1.0，合法 chunk cos<1，断言 top-1 是合法 chunk —— 实现若改成"全量打分后过滤"此测试必红）、`TestSearchCandidateSetEmptySkipsRank`、`TestSearchEmptyTiersReturnsZeroHitsNoError`、`TestCandidatesExcludesExpiredEffectiveTo`（now=1000：0/1000/1001 通过，999 排除）。
- **排序契约**：`TestRankOrdersByScoreDescThenIDAsc`、`TestRankSkipsDimMismatchAndEmptyEmbedding`（`assert.NotPanics`）、`TestRankDeterministicAcrossRuns`（20 次 `reflect.DeepEqual`）、`TestRankTopKGuards`。
- **缓存**：`TestCacheKeyIncludesModelDimAndCorpusHash`、`TestCorpusHashIgnoresMtime`（改 `os.Chtimes` 后 hash 不变）、`TestCorpusHashOverRealKnowledgeTree`、`TestCacheCorruptFileTriggersRebuild`（8 个子用例分别翻转 magic/version/dim/docCount/corpusHash/endpointHash/payloadHash 与截尾，每个都断言 `errors.Is(err, errCacheInvalid)` 且返回 nil rows）、`TestCacheConcurrentFirstStart`（四写手 + 持锁超时）、`TestEnsureLoadsValidCacheAndSkipsSeed`、`TestEnsureRebuildsOnStaleCache`。
- **seed 零兜底**：`TestSeedEmbedFailureIsFatalAndLeavesNoPartialCorpus`（中途 Embed 失败 ⇒ 返回错误、docs 保持空、缓存文件不存在）、`TestSeedRequiresEmbedderAndRegistry`、`TestValidateDocsFailClosed`、`TestStripFrontmatterMatchesMongoAlgorithm`。
- **接口契约**：`TestGetChunkReturnsErrChunkNotFound`（`errors.Is`）、`TestGetMaxIDEmptyTierAndUnknownTierReturnZero`、`TestListMetadataIgnoresTierAndEffectiveWindow`、`TestInsertRejectsDimMismatchAndDuplicateID`、`TestDisconnectMakesClientUnusable`、`TestNewRejectsBadDimensions`。

## Notes for the next agent

- `cmd/server/main.go` 里 `glmKey := envOr("LOCAL_API_KEY", envRequired("GLM_API_KEY"))` 的 eager 求值地雷**归 49-02**。`TestGlmKeyLineStillEagerFor49_02` 是交接凭据：它断言 `envRequired("GLM_API_KEY")` 恰好出现 1 次且同行含 `envOr("LOCAL_API_KEY"`。**49-02 改动后必须同步更新这个测试**，不要误以为 lock 在阻止改造（测试注释里已写明这一点）。
- `BGE_M3_URL` 仍是必填（`main.go:288`，D-11）。embedded 后端不做进程内 embedding，端点故障必须暴露、不能悄悄降级；放开它会同时影响 embedded 分支。
- 缓存目录由 `SKILLFORGE_VECTOR_CACHE_DIR` 注入（默认 `.skillforge-cache/vectors`）。`.skillforge-cache/` 的 .gitignore 条目归 49-04/49-05 —— 在那之前跑过 embedded 的开发者会在工作区留下 `vectors.bin`（~900KB）与 `vectors.lock`。
- 首次启动顺序 load-bearing：`Ensure`（读缓存，零网络）必须早于 `SeedFromKnowledgeDir`（缓存未命中才装载并持锁写缓存）。缓存命中时 seed 的首条语句短路返回，218 次 embedding 全免。
- embedded 的 `ListMetadata` 不过滤 tier / effective 窗口（与 mongo 语义一致），调用方（知识图谱）自己处理。
- 缓存的并发原语是"单文件 `O_CREATE|O_EXCL` + 200ms 轮询"，不是 flock：Windows 开发机与 WSL 双跑时需要两边都可靠的原语。超时默认 5 分钟（`defaultCacheWaitTimeout`，可经 `Config.CacheWaitTimeout` 调整），超时报错文案含 `no fake-vector fallback: D-01`，便于日志检索。
- `rank` 对行级维度不符是"跳过"而非报错：`Search` 入口已校验 queryVec 与 `Config.Dimensions` 一致，行级不符只可能来自 Insert 混入或缓存损坏，跳过比 panic 安全（`TestRankSkipsDimMismatchAndEmptyEmbedding` 用 `assert.NotPanics` 守住）。
- `corpusHash` 只吃 `.md` 内容：加/删/改任一 `.md` 都换 key，但改 `.gitkeep` 或加图片不会 —— 刻意的，那类文件不参与索引。
- 后续若加第四、第五个后端，扩展 `TestBuildRetrieverNoneBranchUnchanged`：它的意义是"新增并行路径 ≠ 改写既有路径"，新后端应为它补上"既有分支零漂移"的断言。
- **wave 1 并发提示**：本 plan 收尾时 `internal/server/stubchat/` 是 untracked 且编译不过（另一 plan 正在写），`internal/verdict/verdict.go` 也被并发修改中。因此 `go build ./...` 与 `go test ./...` 的全仓视图在 wave 1 期间不可作为单个 plan 的判据；验证时请按包集合收窄（本 plan 的做法：`go build $(go list ./... | grep -v stubchat | grep -v kpi/wsl)`）。STUB-01 落地后全仓视图自然会恢复。
- `internal/kpi/wsl` 只有 `_test.go` 没有非测试源文件，`go vet` 会对它报 `no non-test Go files` —— 这是既有状态，与本 plan 无关，收窄包集合时一并排除。
