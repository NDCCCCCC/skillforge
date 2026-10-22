---
phase: 49-hermetic-demo
plan: 1
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
  - SFVC 向量缓存格式（model+dim+docCount+payloadHash+endpointHash 全字段校验）
  - knowledgeDirForSeed 包级变量（cmd/server 测试可注入 t.TempDir()）
affects:
  - cmd/server/main.go（buildRetriever 与 indexerBackend 的新分支）
  - 49-02/49-03/49-04（依赖本后端可用）
actuals:
  tokens: 75000
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
  - "缓存条目带 endpointHash（BGE_M3_URL 的 sha256）：换端点必须重建缓存，否则旧端点的向量会被静默复用（D-14 三要素之外补的第四道防线）"
  - "seed 失败走 log.Fatalf 而非 WARN：embedded 模式没有第二数据源，语料读不出必然全量拒答，属 D-01 的『明确失败』"
  - "indexerBackend 的 embedded case 直接断言 *embedded.Client（它已实现 Insert），不引入第三个接口（对应 mongo 分支的形态）"
  - "main.go 的 case 注释不出现 milvus 断言字面量：backend_switch_test 的 case-body 扫描包含注释，字面量会触发假阳性 FAIL"
---

## Accomplishments

- `internal/retriever/embedded/`：`Retriever` 全部 8 方法 + `Insert`（`IndexerBackend`）+ `SeedFromKnowledgeDir`（解耦 seed，先例 mongo/seed.go）+ SFVC 缓存（magic/version/model/dim(1024)/docCount(218)/payloadHash/endpointHash，任一不符整份作废；O_EXCL 原子锁 + 轮询 + 超时报错）。
- tier 隔离两段式 PRE-filter：`tier_isolation.go` 的 `candidates()` 先按 tiers+effective_to（sentinel-0）构造候选集；`rank.go` 逐字照搬 mongo 的纯函数 rank（score desc, id asc），入参不含 tiers —— 结构上无法后置过滤（Pitfall 11 封死）。空 tiers 循环前 `return nil`。
- `cmd/server/main.go`：`buildRetriever` 加 `case "embedded":`（只读 `SKILLFORGE_VECTOR_CACHE_DIR` 与 `BGE_M3_URL` 的值，不读 MONGO_URI/MILVUS_ADDRESS，无 envRequired）；`indexerBackend` switch 加显式 case（封死 default 分支的 runtime panic 地雷 1）；新增包级变量 `knowledgeDirForSeed`。
- `cmd/server/backend_switch_test.go`：4 个源码 lock（embedded case 存在且无 milvus 断言 / D-16 无外部 DB / 既有三分支零漂移 / glmKey eager 现状交接凭据）。

## Task Commits

由编排者统一提交（本 plan 不做 git 操作）。

## Files Created / Modified

见 frontmatter key-files（10 created + 1 modified，共 11 项，无越界）。

## Deviations

- **D-01（Rule 1 修源码不修演练）**：executor 首版把 `indexerBackend` embedded case 的注释写成含 `r.(*milvus.MilvusClient)` 字面量，导致 `TestBackendSwitchEmbeddedHasExplicitCase` 假阳性 FAIL（case-body 扫描包含注释）。修法：改注释措辞（"对 MilvusClient 做类型断言"），保留测试的严格扫描。
- **D-02**：缓存在 D-14 的 model+dim+corpusHash 之外增加 endpointHash（BGE_M3_URL sha256）——防止换端点后静默复用旧向量。
- **D-03**：seed 的 embed 失败选择 `log.Fatalf`（计划允许 WARN 继续；embedded 无第二数据源，WARN 会让服务以空语料"健康"运行，违反 D-01 明确失败原则）。

## Verification

- `go test ./cmd/server/ -run 'TestBackendSwitch|TestBuildRetriever|TestGlmKeyLine' -count=1 -v`（WSL）→ **4/4 PASS, ok**
- `go test ./internal/retriever/embedded/ -count=1`（WSL）→ **ok**（35 个测试/子测试全绿：cache key 三要素、mtime 无关、真实语料树哈希、8 种缓存损坏逐项重建、并发首启四写手+持锁超时、embed 失败无部分语料、空语料非错误、validateDocs fail-closed、stripFrontmatter 对齐 mongo 算法、Ensure 缓存命中跳过 seed、陈旧缓存重建、tier 隔离被禁 chunk 得分 1.0 仍不入选等）
- `go vet ./internal/retriever/embedded/ ./cmd/server/`（WSL）→ **exit 0**
- `go build ./...`（WSL）→ **exit 0**
- 依赖清单 diff（go.mod/go.sum/web/package.json/web/package-lock.json）→ **UNCHANGED**（零新依赖）

## Notes for the next agent

- `cmd/server/main.go` 里 `glmKey := envOr("LOCAL_API_KEY", envRequired("GLM_API_KEY"))` 的 eager 求值地雷**归 49-02**（`TestGlmKeyLineStillEagerFor49_02` 是交接凭据，49-02 改动后须同步更新该测试）。
- 缓存目录由 `SKILLFORGE_VECTOR_CACHE_DIR` 注入（默认 `.skillforge-cache/vectors`）；`.skillforge-cache/` 的 .gitignore 条目归 49-04/49-05。
- 首次启动顺序 load-bearing：`Ensure`（读缓存，零网络）必须早于 `SeedFromKnowledgeDir`（缓存未命中才装载）。
- embedded 的 `ListMetadata` 不过滤 tier/effective 窗口（与 mongo 语义一致），调用方（知识图谱）自己处理。
