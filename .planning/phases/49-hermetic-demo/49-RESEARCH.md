# Phase 49: Hermetic Demo — Research

**Phase:** 49 — Hermetic Demo
**Generated:** 2026-10-21
**Context:** `.planning/phases/49-hermetic-demo/49-CONTEXT.md`
**Status:** Scaffold landed by the plan-phase orchestrator; the technical sections below are filled by the gsd-phase-researcher subagent. Every claim must carry an evidence marker (`[VERIFIED: …]` / `[CITED: url]` / `[ASSUMED]`).

<user_constraints>
## Locked Decisions (verbatim from 49-CONTEXT.md — do not re-litigate)

### 向量从哪来（Area 1）

- **D-01 强制外部向量化模型，不做任何兜底。** 复用现有 `internal/embedder/bge`（HTTP 调 `BGE_M3_URL`，1024 维）。**不**引入进程内 embedder，**不**提交预计算向量文件，**不**用 hash/随机向量冒充。端点不可达 ⇒ 明确失败。
  依据：用户原话「我的意思是不考虑任何兜底方案，强制要求使用外部向量化模型」；与 `internal/embedder/bge/bge.go` 既有纪律一致（包注释 Pitfall 8：hash 兜底会用语义无意义的向量掩盖真实故障）。
- **D-02 端点形态两种都支持，以本地 Ollama 为主。** 本机/常见场景：`ollama pull bge-m3` ⇒ `BGE_M3_URL=http://localhost:11434/v1/embeddings`（OpenAI 兼容、无需 key；README 现已是此默认）。任意 OpenAI 兼容端点可通过改 `BGE_M3_URL` 替换，README 注明。
- **D-03 "embedded" 一词专指「向量存储进程内」，不指「向量化进程内」。** 这条必须写进任何实现文档 —— 否则 planner 极可能理解反，做成进程内 embedder。
- **D-04 向量存储 = 进程内 Go slice + cosine 排序。** 照搬 `internal/retriever/mongo/rank.go` 的纯函数 `rank` 与 (score desc, id asc) tie-break，保证可复现。
- **D-05 语料 = 仓库现有 `knowledge/` 全量**（authority 16 + sql-guides 202 + draft 1 + historical 1），沿用 `internal/knowledge/indexer.go` 的 tier 校验与文档结构解析。
- **D-06 接入方式 = 新增 backend 名 `embedded`**，与 `milvus` / `mongo` / `none` 并列。现有 `none` 的 degraded default-deny 语义（`cmd/server/main.go` 的 `noopRetriever`）一字不动。

### stub 模式怎么如实标注（Area 2）

- **D-07 stub 产出 = 命中 chunk 的原文抽取。** 像 `internal/knowledge/ecl/stub.go` 的 `StubExtract` 那样做确定性抽取（正则/句段），从 top-k chunk 拼出 answer，citations 指向真实 chunk。输出仍是**合法** `verdict.Verdict` JSON，wire 形状与真模型完全一致，下游（前端、审计、评测）零改动。
- **D-08 三处标注 stub 模式。** ①`verdict.Verdict` 增加一个 `omitempty` 字段（值形如 `mode:"stub"`；先例：`Answer` / `ComplianceStatus` 就是这么加的，legacy 生产者可 round-trip）②启动时打醒目 WARN ③README 写明。核对前端 `web/src/components/chat/MessageBubble.tsx` 的 `isVerdict` 类型守卫需一并扩展。
- **D-09 触发条件 = 无 key 自动 stub + 三重标注。** 配置了 key 走真模型；没 key 走 stub，且三处都明说。已知风险：生产环境掉 key 会静默降级到 stub —— 靠三重标注抵消，不得静默伪装成真模型（SC3 原文要求）。
- **D-10 stub 的引用与真模型完全同等，不豁免任何校验。** answer 逐字来自 chunk 原文，citations 走同一条 grounding 校验（引用 span 必须真实出现在 chunk 里，否则降级），confidence 上限 0.5，强制 `review_required=true`。
- **D-11 `cmd/server/main.go` 的启动必填项必须随之调整：** `BGE_M3_URL` **保留必填**（embedded 模式仍需外部模型，D-01），`GLM_API_KEY` **改为可选**（stub 模式无 key 也要能起）。

### clone 后怎么跑起来（Area 3）

- **D-12 纯本地进程，零 Docker。** `go run ./cmd/server`（:8888）+ `cd web && npm run dev`（:5173，Vite 已代理 `/api` → :8888）。
  依据：现有 `docker-compose.yml` 的 backend 段用 `${BGE_M3_URL:?…}` / `${GLM_API_KEY:?…}` 强制插值 —— 没有 key 连 `docker compose up` 都起不来；另有 milvus/etcd/minio 三容器与本 phase 的"无内网"目标冲突。
- **D-13 前端由后端同源托管（:8888）。** 新增静态托管 `web/dist` + SPA fallback（生产现由 nginx 容器承担）；后端当前**不**提供前端静态文件，需新增。
- **D-14 全量索引 + 本地向量缓存。** 首次启动把 `knowledge/` 全量过 BGE-M3 算出向量，存**本地缓存文件（不入仓库）**；后续启动直接读缓存，避免每次重算。
  **缓存 key 必须包含 model + dimension + 语料 hash**，否则换模型或改语料会静默吃到旧向量。
- **D-15 只加 `RETRIEVER_BACKEND=embedded`，其余配置体系一字不动。** 现有 milvus 路径、现有 compose、`.env.example` 均不修改，向后兼容最好。
- **D-16 embedded 分支不得要求 `MONGO_URI` / `MILVUS_ADDRESS`**（现由 `mongo` / `milvus` 分支各自 `envRequired`）。

### demo.gif（Area 4）

- **D-17 录终端 CLI 演示，不是浏览器 UI。** 用 curl/脚本跑一轮带引用的问答，终端里直接可见 citations。最能体现"改一个 `SKILL.md` 就换业务域"的内核卖点，且录屏工具链最轻。
- **D-18 生成方式 = asciinema（录）+ agg（渲染 gif）**，用 Makefile 目标封装（如 `make demo-gif`），文档写明前置依赖。素材是真实录屏，不是脚本画出来的假动画。
- **D-19 素材直接产出到 `docs/demo.gif`。** `README.md` 已经在引用这个路径而文件不存在（正是 Phase 50 FACE-01 的破图根因）；本 phase 直接把真实素材放到该路径，Phase 50 只需确认不断链。
- **D-20 录制脚本进仓库，素材也进仓库。** 脚本（形如 `scripts/record-demo.sh`）+ Makefile 目标入库，文档写明依赖。脚本同时充当 SC4「quickstart 逐步实测通过」的证据。

### The agent's Discretion (verbatim from 49-CONTEXT.md)

- 本地向量缓存的**目录位置、文件格式、并发首次启动的互斥方式**、缓存失效的具体实现（key 组成必须满足 D-14 的三要素）
- stub 抽取器的**具体抽取规则**（句段选择、正则、长度上限）—— 只需满足 D-07/D-10 的确定性与引用真实性
- 错误消息的具体文案与启动 WARN 的呈现形式
- `verify-phase-49.sh` 的**内部断言组织与分区结构**（只需满足 SC5 exit 0，且必须真能抓到对应的回归）
- 演示脚本的**具体命令序列**（只需体现一轮带引用的问答 + 一次 Skill 切换）
- 后端静态托管的**实现方式**（`http.FileServer` + SPA fallback 的具体写法）与 `web/dist` 缺失时的行为

### Deferred Ideas (verbatim from 49-CONTEXT.md — out of scope, do not plan for these)

- **浏览器 UI 版 demo** —— 用 playwright 录屏展示真实 Web 聊天界面与引用卡片。本 phase 只做终端 CLI 版；UI 版归 Phase 50（FACE-01 正好要处理 README 的 `docs/demo.gif` 破图），或作为独立后续项。
- **建议回 ROADMAP 修订 Phase 49 的措辞**（本 phase 不擅自改，phase 边界由 ROADMAP 固定）：
  - SC1 的"无预装 BGE"应改为"无需内网 BGE 端点"（本 phase 仍需一个外部 embedding 端点，默认本地 Ollama）。
  - SC2 的"兜底模式"应改为"本地向量存储模式"（D-01 明确不做向量兜底）。
- **向量兜底方案整体不作为 Deferred 保留** —— 用户明确排除。

</user_constraints>

<phase_requirements>
## Phase Requirements (verbatim from .planning/REQUIREMENTS.md)

- **DEMO-01**: 无内网依赖的检索链——embedded 向量兜底模式（无需外部 Milvus/BGE 即可检索）
- **DEMO-02**: 无 API key 路径——stub LLM chat（既有 stub 抽取器延伸），全链可跑
- **DEMO-03**: quickstart 实测通过——hermetic 模式下 README quickstart 逐步验证（产出 demo.gif 素材，反哺 FACE-01）

Traceability (`.planning/REQUIREMENTS.md`): DEMO-01 / DEMO-02 / DEMO-03 → owner Phase 49, status Pending.

</phase_requirements>

## Summary

Phase 49 的三个需求（DEMO-01/02/03）在本仓库里都有**已存在的接缝**，不需要新造轮子，也不需要引入任何第三方依赖。核心结论有三条：

第一，**embedded 向量存储不是新架构，是 `retriever.Retriever` 接口的第 4 个实现**。`internal/retriever/retriever.go:167-217` 已经把检索面收敛成一个接口（Health / Search / Ensure / Insert / ListMetadata / GetChunk / GetMaxID / Disconnect），`cmd/server/main.go:1226-1306` 的 `buildRetriever` 已经是一个按 `RETRIEVER_BACKEND` 分派的后端策略 switch（`none` / `mongo` / default→milvus）。新增 `internal/retriever/embedded` 包并加一个 `case "embedded":` 即可。排序内核不需要写：`internal/retriever/mongo/rank.go:94-132` 的 `rank` 是纯函数（无 I/O），`internal/vecmath/cosine.go:50-69` 的 `Cosine` 是全仓唯一的向量相似度真源——`internal/vecmath/cosine.go:17-34` 的包注释明确说它就是为了避免"第二份副本悄悄漂移"而抽出来的。[VERIFIED: internal/retriever/mongo/rank.go:94-132] [VERIFIED: internal/vecmath/cosine.go:17-34, 50-69]

第二，**stub LLM 的正确插入点是 `Deps.PipelineChatModel`，而不是在 pipeline 内部开分支**。`internal/server/pipeline.go:414` 已经是「`s.PipelineChatModel != nil` → 走流式路径；否则回退 `s.Runner.Query` 的 Phase 2 路径」的双通道结构，`internal/server/server.go:287-302` 明确写了 `PipelineChatModel model.BaseChatModel` 是"streaming seam"、`nil` 时回退 Runner。stub 只要实现 `model.BaseChatModel`（`Stream` + `Generate`）并产出与真模型**逐字节同构**的 Verdict JSON，pipeline 的 parse → `verdict.Validate` → `GroundingCheck` → `GroundingChecker` 全部原样跑过——这正是 D-07 / D-10 "wire 形状完全一致、不豁免任何校验"的技术落地方式。而且 stub 拿得到证据：chunks 已经被 `buildPrompt` 渲染进 user message（`internal/server/pipeline.go:770-785` 的 `[ref-N] tier=… source=…` 块），stub 解析 user message 就能确定性地抽出 answer 与 citations，**不需要改 pipeline 的函数签名**。[VERIFIED: internal/server/pipeline.go:414, 770-785] [VERIFIED: internal/server/server.go:287-302]

第三，**同源静态托管可以用 Hertz 自带的 `adaptor` 包做到零新依赖**。`github.com/cloudwego/hertz v0.10.6` 已是 go.mod 的直接依赖，其 module cache 中 `pkg/common/adaptor/handler.go:36` 提供了 `func HertzHandler(h http.Handler) app.HandlerFunc`——把 Go 标准库的 `http.FileServer` 直接挂进 Hertz，SPA fallback 用标准库 `http.Handler` 写即可。参照物是现成的：`Dockerfile.frontend:39` 的 nginx 配置已经写死了这套语义（`location / { try_files $uri /index.html; }` 与独立的 `location /api`），Go 侧复刻这两条即可。[VERIFIED: github.com/cloudwego/hertz@v0.10.6/pkg/common/adaptor/handler.go:35-36] [VERIFIED: Dockerfile.frontend:39]

研究阶段还发现三处**必须在新代码落地前先处理的既有隐患**（不是本 phase 制造的 bug，但会直接挡住 D-11 / D-06 的实现）：`cmd/server/main.go:302` 的 `envOr("LOCAL_API_KEY", envRequired("GLM_API_KEY"))` 因为 Go 的 eager 实参求值，即使 `LOCAL_API_KEY` 已设也会因 `GLM_API_KEY` 缺失而 `Fatalf`；`cmd/server/main.go:499-506` 的 `indexerBackend` switch 用 `default:` 兜底并做 `r.(*milvus.MilvusClient)` 断言，embedded backend 落进 default 会 **panic**；`internal/knowledge/indexer.go:168` 的 `Index` 签名耦合了 `milvus.Config` 与 `client.Client`，所以 embedded 只能照 `internal/retriever/mongo/seed.go:38-48` 说明的既有先例，自己写一份与 milvus 解耦的 seed 函数。[VERIFIED: cmd/server/main.go:302, 499-506] [VERIFIED: internal/knowledge/indexer.go:168] [VERIFIED: internal/retriever/mongo/seed.go:38-48]

**Primary recommendation:** 按「**新增 `internal/retriever/embedded` 包（照搬 mongo 的 rank + seed 形态）+ 在 `cmd/server/main.go` 的两个 switch 上各加一个 case + 用 `model.BaseChatModel` 实现一个 stub ChatModel 插进 `Deps.PipelineChatModel`**」这条主线实现；三处 demo 素材（asciinema 录制、agg 渲染、`docs/demo.gif`）作为**独立于主链的收尾工作项**推进，不要让录屏工具链的安装阻塞主链的代码落地。

**本轮复核后的三条硬更正**（细节见 `## Verification Corrections` 与 `## Answers to the Nine Research Questions`）：① **可入库语料是 218 篇 `.md`**，不是 D-05 写的 220 个目录条目——`draft`/`historical`/`personal` 三个目录**各只有一个 `.gitkeep`**，缓存条数断言与 corpusHash 输入都必须按 218 算；② **`.gitignore` 当前没有任何向量缓存条目**，D-14「不入仓库」需要**显式新增**一条规则，否则缓存必被误提交；③ **`.env.example` 不列 `embedded` 这个 backend 取值**且 D-15 要求一字不动 ⇒ README 的 hermetic 小节必须逐字给出 `export RETRIEVER_BACKEND=embedded`，否则读者照抄 `.env.example` 会落回 milvus 又要 Milvus。

## Verification Corrections (this research pass)

本轮（v2）按受限阅读清单逐条复核了与**检索面 / 向量化 / verdict 契约 / 门禁形态**直接相关的文件，并对上一轮研究稿做了以下**第一手实测更正**：

| # | 更正 | 本轮证据 | 影响 |
|---|---|---|---|
| C-01 | **真正可索引的语料 = 218 篇 `.md`**（`authority` 16 + `sql-guides` 202）。`knowledge/draft`、`knowledge/historical`、`knowledge/personal` 三个目录里**各只有一个 `.gitkeep`**，没有任何 `.md` 文档 | `find knowledge -name '*.md' -type f \| wc -l` → `218`；`find knowledge -type f ! -name '*.md' \| wc -l` → `3`（`knowledge/draft/.gitkeep`、`knowledge/historical/.gitkeep`、`knowledge/personal/.gitkeep`）；`find knowledge -type f \| wc -l` → `221` | D-05 的「authority 16 + sql-guides 202 + draft 1 + historical 1」= 220 统计的是**目录条目**而非文档数，且**漏列 `personal`**。⇒ 缓存的 `corpusHash` 覆盖的是**实际入库的 218 篇**；缓存条数断言写 218，不是 220/221。详见 `## Assumptions Log` A-01 / A-13 |
| C-02 | `docker`、`asciinema`、`agg`、`ollama`、`jq` **均不在 PATH**；`go` / `node` / `npm` / `python` / `cargo` / `golangci-lint` / `ffmpeg` / `git` 在 PATH | 本轮 `command -v` 探测（逐条结果见 `## Environment Availability` 的「探测记录」） | A-02 由「未验证」升级为**确定事实**；旧 quickstart 的 `docker compose up` 在本机**不可执行**；D-19 的素材产出必须先装 asciinema+agg |
| C-03 | WSL2 内 Go 可用且与宿主同版本（`go1.25.5 linux/amd64`） | `wsl.exe -e bash -lc "go version"` → `go version go1.25.5 linux/amd64` | 门禁的 Go 分区在 WSL 内可跑，命令形态照 `verify-phase-47.sh` 的 `wsl()` 包装 |
| C-04 | `.gitignore`（全文 40 行）**没有任何向量缓存条目**——现有条目是 `.env`、`*.exe`、`/spiked`、`spike-*trace.json`、`.DS_Store`、`node_modules/`、`web/dist/`、`skillforge.db*`、`/body_gbk_*`、`/body_quantum.json`、`/reports/`、`/eval/reports/`、`/data/`、`/skillforge-server*` | [VERIFIED: .gitignore:1-40] | D-14「不入仓库」必须**显式新增**一条规则，否则 `git status` 必脏 ⇒ Wave 0 缺口清单里那条是**必需项**不是可选项 |
| C-05 | `.env.example:8-13` 已把 `RETRIEVER_BACKEND` 写成可选开关并逐条说明 `milvus` / `mongo` 两个取值，**没有 `embedded`**；`:43-45` 写明 BGE 端点「Required in both backends. OpenAI-compatible /v1/embeddings endpoint.」 | [VERIFIED: .env.example:8-13, 43-45] | D-15「`.env.example` 一字不动」成立，但代价是**新 backend 只能靠 README 说明** ⇒ README 那一节必须点名 `RETRIEVER_BACKEND=embedded`（见 `## Open Questions` Q5） |

> 上一轮研究稿中标注 `[VERIFIED: …]` 的其它断言来自同一 phase 的第一次研究 pass；本轮**没有**重复打开那些文件，因此不在此处复述其引用，以免把未经本轮复核的证据再写一遍。下文新增的小节只引用**本轮实读**的文件。

## Answers to the Nine Research Questions

### Q1 — 进程内向量存储要满足什么接口面？tier 过滤必须怎么表现？

**接口面**：实现 `internal/retriever/retriever.go:167-217` 的 `Retriever`，共 **8 个方法**（逐字签名）：

- `Health(ctx context.Context) error`
- `Search(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]Hit, error)`
- `Ensure(ctx context.Context) error` — 幂等，每次启动都安全调用
- `Insert(ctx context.Context, docs []Document) error` — 调用方预分配数字 ID，**不做自动去重**
- `ListMetadata(ctx context.Context) ([]DocumentMeta, error)` — **不过滤 tier**，且**不得**对 `effective_to` 做 PRE 过滤（图谱要故意暴露被取代的 chunk 并打「已失效」徽标）
- `GetChunk(ctx context.Context, id int64) (ChunkContent, error)` — 不存在时返回 `ErrChunkNotFound`（逐字：`errors.New("retriever: chunk not found by id")`）
- `GetMaxID(ctx context.Context, tier string) (int64, error)` — tier 为空时返回 0
- `Disconnect() error`

[VERIFIED: internal/retriever/retriever.go:36, 167-217]

**最接近的现有类比**：`internal/retriever/mongo/rank.go:94-132` 的 `rank` 是**纯函数**（无 I/O、无全局状态），排序比较器逐字为：

```go
sort.SliceStable(out, func(i, j int) bool {
    if out[i].score != out[j].score {
        return out[i].score > out[j].score
    }
    return out[i].row.ID < out[j].row.ID
})
```

其包注释逐字解释了「过滤在存储侧、打分在 Go 侧」这个形状的由来：「We store each document with its embedding array and apply the tier filter as a real MongoDB query ({tier: {$in: tiers}}) BEFORE computing cosine similarity in Go.」[VERIFIED: internal/retriever/mongo/rank.go:17-35, 94-132]

**「过滤在存储侧、打分在 Go 侧」的实现形状**（`internal/retriever/mongo/mongo.go` 的 `Search` 逐字）：

```go
filter := bson.D{
    {Key: "tier", Value: bson.D{{Key: "$in", Value: tiers}}},
    {Key: "$or", Value: bson.A{
        bson.D{{Key: "effective_to", Value: 0}},
        bson.D{{Key: "effective_to", Value: bson.D{{Key: "$gte", Value: now}}}},
    }},
}
rows, err := c.finder.FindRows(ctx, filter)
if len(rows) == 0 {
    return nil, nil
}
return rank(rows, queryVec, topK), nil
```

同处注释逐字：「Empty tiers → $in:[] → no matches → 0 hits (D-09). This is THE trust boundary: building the filter any other way (e.g. skipping the find on empty tiers) would still produce 0 rows but would not be the DATABASE refusing — the structural guarantee would be in Go, not in the storage layer. Always build the filter, always run the find.」[VERIFIED: internal/retriever/mongo/mongo.go:397-441]

** prescriptive 结论**：embedded 的等价形态是**两段式**——先构造候选集（`tier ∈ tiers` **且** `effective_to == 0 || effective_to >= now`），再对候选集算 cosine。**空 tier 集合 ⇒ 候选集为空 ⇒ 0 命中**（refusal-by-construction）。**禁止**「全量打分后再 filter」：那会把信任边界交给打分逻辑，一旦打分分支出错，隔离即失效。`internal/retriever/retriever.go:142-161` 的契约注释也是这个意思：「Tier-filter semantics: PRE-similarity at the SDK/Search layer (NEVER post-filter). Empty/zero tiers → backend returns 0 hits (refusal-by-construction, D-09).」

### Q2 — 本地向量缓存条目必须包含什么？放哪里？并发首次启动怎么办？陈旧条目怎么判？

**条目必须包含**（D-14 三要素 + 必要元数据）：

| 字段 | 来源 | 为什么必须有 |
|---|---|---|
| `model` | `BGE_M3_MODEL`，缺省 `bge-m3`（[VERIFIED: cmd/server/main.go:289]） | 换模型必须失效 |
| `dimension` | `1024`（`dimensions = 1024 // BGE-M3 dense output dimension`，[VERIFIED: internal/embedder/bge/bge.go:47-51]） | 维度不匹配会让 `vecmath.Cosine` **panic**（长度不等直接 panic，[VERIFIED: internal/vecmath/cosine.go:50-69]） |
| `corpusHash` | 语料内容哈希 | 改语料必须失效 |
| `endpointHash` | `BGE_M3_URL` 的 sha256（**原串不落盘、不进文件名**） | URL 可能带 token；且换端点应失效 |
| 每条：`id` / `tier` / `source` / `effectiveFrom` / `effectiveTo` / 向量本体 | 来自 `retriever.Document` | 检索与引用都要用 |
| 整体 payload 校验和 | sha256(全部向量字节) | 损坏检测 ⇒ 重建而非部分可用 |

**放在哪**：仓库内 gitignored 目录（建议 `.skillforge-cache/vectors/`），并**新增一条 `.gitignore` 规则**（C-04：当前没有）。理由：与既有惯例一致（`.gitignore:12-17` 已经有 `skillforge.db` 这条"本地运行时状态不入库"的先例），且 demo 场景下"看得见的缓存"比"隐形缓存"更利于排障。

**并发首次启动**：两个进程同时首启会同时向 BGE 端点打 218 次请求。**做法**：用**原子创建**的锁文件（`os.OpenFile(path, os.O_CREATE|os.O_EXCL, 0o600)`）做互斥；抢不到锁的进程**轮询等待**缓存文件出现（带超时），超时后**不**降级为假向量，而是**明确报错退出**（D-01：端点不可达 ⇒ 明确失败）。不要用「固定名目录锁 + `sleep` 猜时长」。

**陈旧条目的判定**：读取时逐项校验 magic / version / dimension / 记录数 / payload 校验和 / **缓存 key 里的 corpusHash 是否等于当前语料的 hash**。任一不符 ⇒ **整个文件作废**（当作未命中并重建），**绝不**"部分可用"。

**语料哈希必须基于内容**：`git clone` / `git checkout` 会重写 mtime ⇒ 禁止用 mtime。哈希输入 = (相对路径, 文件字节) 按路径排序后串联。**注意条数是 218，不是 220/221**（C-01）。

### Q3 — BGE-M3 端点契约（录制/回放必须逐字一致）

**请求体**（`internal/embedder/bge/bge.go:142-150` 逐字）：

```go
type embedReq struct {
    Model string `json:"model,omitempty"`
    Input string `json:"input"`
}
```

⇒ body 为 `{"model":"<Model>","input":"<text>"}`；`Model` 为空时**整个字段被省略**（注释逐字：「Empty OMITS the field (back-compat with endpoints that reject unknown fields).」）

**请求头**：`Content-Type: application/json`；token 非空时 `Authorization: Bearer <token>`，否则回落到环境变量 `BGE_M3_AUTH`（`authTokenFromEnv` 读 `os.Getenv("BGE_M3_AUTH")`）。

**响应（两种形状，按顺序解析）**：
1. OpenAI 兼容：`{"data":[{"embedding":[ …1024 个 float… ]}]}`
2. TEI 裸数组：`[{"embedding":[ … ]}]`
3. 都不匹配 ⇒ 错误逐字：`could not parse embedding from response (no 'embedding' array found)`

[VERIFIED: internal/embedder/bge/bge.go:142-208]

**维度**：`1024`（`func (b *BGE) Dimensions() int { return dimensions }`）。

**模型名怎么设**：`bgeModel := envOr("BGE_M3_MODEL", "bge-m3")`（[VERIFIED: cmd/server/main.go:289]）；配置注释逐字：「model name for OpenAI-compatible /v1/embeddings (Ollama needs "bge-m3"; vLLM/TEI accept it).」⇒ **Ollama 与通用 OpenAI 兼容端点在请求体上完全一致**；差别只有两点：① Ollama 无需 key ⇒ `BGE_M3_AUTH` 留空即可，`BGE_M3_URL=http://localhost:11434/v1/embeddings`（README.md:52 的默认值）；② 通用端点通常要 key ⇒ 走 `BGE_M3_AUTH`。

**录制/回放夹具必须逐字一致的三处**（这是 hermetic 门禁的替身契约）：
- **请求体**：键集合与 `omitempty` 行为一致（`Model` 非空时 `model` 键**必须出现**）；
- **响应体**：优先 OpenAI 形状 `{"data":[{"embedding":[…]}]}`（`bge` 先试它，再试 TEI 形状）；
- **向量长度**：`1024`（否则 cosine 路径会 panic）。

> **关键陷阱**：**固定录制的向量对任意查询文本的 cosine 近乎无关** ⇒ 必然低于 `internal/server/pipeline.go:64` 的 `retrievalScoreThreshold = 0.3` ⇒ 门禁只能验「拒答」、验不了「带引用的问答」。**解法**：夹具必须是**按键查表**的（语料文本 → 录制向量；demo 查询文本 → 录制向量），未命中的键返回明确错误。详见 `## Common Pitfalls` Pitfall 17 与 `## Code Examples` 示例 10。

### Q4 — stub 路径怎么走？

**现有先例的形状**（模式先例，**不是代码复用**）：`internal/knowledge/ecl/stub.go:305-314` 的 `StubExtract` 把「确定性规则」与「LLM 抽取」彻底分离——它用一组正则规则（`sqlRules` 逐字：`{sqlUpdateRegex→"update", sqlInsertRegex→"insert", sqlCreateLine→"create", sqlDeleteRegex→"update", sqlMergeRegex→"update", sqlSelectInto→"select"}`）扫合并后的正文，按**首次出现顺序**排序、**按名字去重**后产出结构化结果；合法值集合由独立函数 `ValidateExtraction` 把关（错误逐字：`ecl: invalid extraction kind %q for table %q (legal: update|select|insert|create)`）。[VERIFIED: internal/knowledge/ecl/stub.go:75, 81-97, 251-276, 305-314, 324-357]

**本 phase 的 stub 必须满足的、可被机器检查的契约**：

1. 产出必须是**合法** `verdict.Verdict`：`Status ∈ {"ok","needs_input","refused","error"}`（[VERIFIED: internal/verdict/validate.go:47-52]）；`Confidence ∈ [0.0, 1.0]`；`Citations` **非 nil**（nil ⇒ 错误逐字 `citations field missing (must be array, can be empty)`）（[VERIFIED: internal/verdict/validate.go:73-84]）。
2. 走**同一条** grounding 校验：`GroundingCheck` 的 span 判定是 `strings.Contains(Normalize(chunkText), Normalize(c.Span))`，`Normalize` 做全角→半角（`width.Narrow`）+ 折叠空白 + TrimSpace（[VERIFIED: internal/verdict/grounding.go:66, 170-196]）⇒ stub 的 `Span` **必须逐字来自 chunk**。
3. 走**同一条**降级链：`GroundingChecker` = `GroundingCheck` + `ApplyDowngrade`，降级把 `Confidence` 钳到 `ConfidenceFloor = 0.89`、`Status` 改成 `needs_input`、`ReviewRequired` 置 true（[VERIFIED: internal/verdict/grounding.go:204, 216, 293-323, 341-347]）。
4. **D-10 额外约束**：`Confidence ≤ 0.5` 且 `ReviewRequired = true`。
5. 走**同一条** wire 序列化：`data-policy` 帧就是 `json.Marshal(v)`（[VERIFIED: internal/server/chat.go:790-820]）⇒ 新增的 `Mode` 字段自动上 wire。
6. `Stream` **不能返回空流**（pipeline 把零 token 流判为 ERROR）。

### Q5 — 启动必填项要改什么？embedded 分支的规则是什么？

**今天读的（逐字）**：

```go
bgeURL := envRequired("BGE_M3_URL")                         // :288
bgeModel := envOr("BGE_M3_MODEL", "bge-m3")                  // :289
bgeTimeoutSec := envOr("BGE_M3_TIMEOUT", "60")               // :290
bgeTLSSkipVerify := strings.EqualFold(os.Getenv("BGE_M3_TLS_SKIP_VERIFY"), "true") // :296
glmKey := envOr("LOCAL_API_KEY", envRequired("GLM_API_KEY")) // :302
```

[VERIFIED: cmd/server/main.go:288-302]

**D-11 的改法**：`GLM_API_KEY` 变可选 ⇒ **必须**改写 `envOr("LOCAL_API_KEY", envRequired("GLM_API_KEY"))` 这一行。Go 的实参在调用前**eager 求值**，`envRequired` 即使写在默认值位置也会先执行 ⇒ 只缺 `GLM_API_KEY` 也会 `log.Fatalf` 退出。改成 `glmKey := envOr("LOCAL_API_KEY", os.Getenv("GLM_API_KEY"))`，再用 `glmKey == ""` 决定走真模型还是 stub。`BGE_M3_URL` **保持 `envRequired`**（D-01 + D-11：embedded 模式仍需外部向量化模型）。

**embedded 分支的规则（D-16）**：`buildRetriever` 的 `case "embedded":` 里**不得**出现 `envRequired("MONGO_URI")`（今天在 `case "mongo":` 内，[VERIFIED: cmd/server/main.go:1239]）或 `envRequired("MILVUS_ADDRESS")`（今天在 default 分支内，[VERIFIED: cmd/server/main.go:1266]）。embedded 分支只读 `BGE_M3_URL` / `BGE_M3_MODEL` / 缓存目录（可选）。

**`none` 分支一字不动（D-06）**：今天逐字是 `log.Printf("[WARN] cmd/server: RETRIEVER_BACKEND=none — tier-channel retrieval disabled (all tier queries refuse with no_authorized_source); data-channel Skills unaffected")` 后 `return noopRetriever{}, "none(degraded default-deny)"`（[VERIFIED: cmd/server/main.go:1236-1237]）。

### Q6 — 同源静态托管：今天有什么、要复刻什么？

**今天没有**：后端不伺服 `web/dist`。`web/vite.config.ts:26-33` 只在 dev 期把 `/api` 代理到 `http://localhost:8888`，其注释逐字：「In dev, the `/api` prefix is proxied to the Hertz backend on :8888 so the browser sees a single origin (no CORS). In production, SkillForge serves the built frontend and the API from the same origin (single-origin deployment).」——即**生产同源今天由 nginx 容器承担**（`Dockerfile.frontend:39`）。

**要复刻的语义基线**（`Dockerfile.frontend:39` 的 nginx 配置逐字）：`location / { try_files $uri /index.html; }` 与 `location /api { proxy_pass http://backend:8888; proxy_http_version 1.1; proxy_set_header Connection ""; proxy_buffering off; proxy_read_timeout 3600s; proxy_send_timeout 3600s; }`；同文件注释逐字：「proxy_buffering OFF is load-bearing for UIMessageChunk SSE streaming — without it nginx buffers chunks and the trust UI degrades to a single flushed blob at connection close.」

**Go 侧的等价实现要点**：
- `http.FileServer(http.Dir(distDir))` 伺服 `web/dist`；SPA fallback 单独一段（**无扩展名且文件不存在** ⇒ 回 `index.html`；有扩展名但不存在 ⇒ 404）；
- **catch-all 必须最后注册**，否则遮蔽 `/api/*`；
- `web/dist` 缺失 ⇒ 返回 **503 + 明确提示**（不要静默 404、不要目录列表）；
- Go 侧直连后端**没有** nginx 的 buffering 问题（SSE 直出），但**不要**给 `/api` 路径加任何写缓冲；
- 路由顺序错误（catch-all 先注册）会被门禁的端到端分区直接暴露。

### Q7 — quickstart 端到端：今天的步骤、真实端口、哪些步骤在无 key / 无 Milvus / 无 BGE 时会断？

**今天的 README Quick Start**（`README.md:41-67`）：① `git clone https://github.com/ninedrunk/skillforge.git` + `cd skillforge`；② `cp .env.example .env`，填 `GLM_API_KEY=<your Zhipu key>` 与 `BGE_M3_URL=http://localhost:11434/v1/embeddings   (default; ollama)`；③ `ollama pull bge-m3`；④ 可选：把 `examples/skills/compliance` 拷进 `skills/` 并删掉 `skills/simple-qa`；⑤ `docker compose up`；⑥ 打开 `http://localhost:8080`。

**真实端口**（逐字来源）：前端 `http://localhost:8080`（`README.md:66`；compose 里 nginx `:8080 → :80`，`README.md:94`）；后端 `http://localhost:8888`（`README.md:79`；`docker-compose.yml` 的 `ports: - "8888:8888"`）；Milvus gRPC `localhost:19530`（`README.md:79`）；dev 模式 Vite `:5173`（D-12）。

**在 hermetic 条件下的断点**：

| 步骤 | 无 key | 无 Milvus | 无 BGE 端点 | 结论 |
|---|---|---|---|---|
| ② 填 `.env` | **断**（要求填 `GLM_API_KEY`） | — | — | hermetic 版必须改成"只填 `BGE_M3_URL`" |
| ③ `ollama pull bge-m3` | — | — | **断**（模型不存在） | D-01 强制 ⇒ 这一步不能省，只能换端点 |
| ⑤ `docker compose up` | **断**：compose 用 `${GLM_API_KEY:?GLM_API_KEY is required — see .env.example}` 强制插值 ⇒ 容器**启动前**就失败 | **断**：compose 还要拉起 milvus/etcd/minio | — | 整条 compose 路径在 hermetic 场景不可用 ⇒ D-12 改走 `go run ./cmd/server` |
| 后端启动 | **断**：`envOr("LOCAL_API_KEY", envRequired("GLM_API_KEY"))` 的 eager 求值 ⇒ 缺 key 直接 `Fatalf` | — | — | D-11 必改 |
| 默认 `RETRIEVER_BACKEND=milvus` | — | **断**：`envRequired("MILVUS_ADDRESS")` | — | D-16 ⇒ embedded 分支不得要求它 |

**结论（ prescriptive ）**：hermetic quickstart = ① clone ② `cp .env.example .env` 并**只**填 `BGE_M3_URL` ③ 起一个 BGE 端点（Ollama 或等价 OpenAI 兼容端点）④ `export RETRIEVER_BACKEND=embedded` ⑤ `go run ./cmd/server`（:8888）⑥ `cd web && npm run dev`（:5173），或让后端同源伺服 `web/dist`（:8888）。

### Q8 — demo.gif 怎么录、怎么渲染、产物落在哪、怎么兼作 SC4 证据？

- **录**：`asciinema rec -c 100 -s 1 <out>.cast`（CLI 参数需按安装版本校准 —— A-03）。
- **渲染**：`agg <out>.cast docs/demo.gif`（同上）。
- **产物落点**：`docs/demo.gif` —— **该路径当前是空位**：`docs/` 下只有 `threat-model/`，而 `README.md:18` 已经写 `![SkillForge demo](docs/demo.gif)` ⇒ 这就是破图根因（本 phase 把真实素材放到该路径；**不要**改 README 的引用路径，那是 Phase 50 的范围）。
- **Makefile 目标**：新增 `demo-gif`（调 `scripts/record-demo.sh`）+ `verify-phase-49`（调**仓库根**的 `verify-phase-49.sh`）。现有 Makefile 的 verify 目标形态是 `bash scripts/verify-phase-NN.sh`（11..30 在 `scripts/`），而 45..48 的门禁脚本在**仓库根** ⇒ Phase 49 跟 45..48 同级。
- **兼作 SC4 证据**：录制脚本**本身**就是「quickstart 逐步实测通过」的执行记录 ⇒ 脚本进仓库（`scripts/record-demo.sh`），素材进仓库（`docs/demo.gif`）。
- **安全约束**：录制环境**不导出**任何真实 key（空 key 反而会展示 stub 模式的启动 WARN，正是 D-08② 要展示的）；门禁对产物做二进制密钥扫描。

### Q9 — `verify-phase-49.sh` 应该是什么形状？

**照 Phase 48 的形状**（`verify-phase-48.sh`，仓库根）：`set -uo pipefail` + `export MSYS_NO_PATHCONV=1` + `ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"` + `WORK="$(mktemp -d)"` + `trap` 清理 + `pass()/fail()/info()` + **分区计数**（`EXPECTED_PARTITIONS=11` / `RAN=0` / `RAN_LIST=()`）+ **证据绑定的成功 token**（`begin_partition` / `ok_partition` / `done_partition`）。

**成功 token 的落法**（逐字语义见 `## Code Examples` 示例 9）：每个分区体**第一条语句**写 `$WORK/ran/<P>.begin`；分区的探针把**每个断言组**的 token 写进**该分区自己的**日志；`ok_partition <P> <log> <token...>` 校验日志存在、非空、且**含每一个 token**，才写 `$WORK/ran/<P>.ok`；`done_partition` 缺 `.begin` 或 `.ok` 一律拒。Phase 48 的原话逐字：「success tokens are EVIDENCE-BOUND … and since the M13 round they are bound PER ASSERTION GROUP, not per probe … the deposit never precedes the assertion it certifies.」

**怎么证明「分区还会咬」**：每个分区都要有**变异演练**——把该分区断言的东西改坏（删一个断言 / 改一个阈值 / 换掉被禁 token），门禁必须**红**；演练本身必须留下证据（Phase 48 的 canary 做法：探针先被证明能产出决定性结果（rc 0/1），再被信任）。**没有变异演练的门禁等于没有门禁。**

**分区清单**沿用现有稿的 P1..P9 编号（见 `## Code Examples` 示例 8）。

## State of the Art

- **进程内向量存储**：主流做法仍是专用向量库（Milvus / Qdrant / pgvector）。本 phase 用**暴力 cosine** 是在 218 篇 × 1024 维规模下的**刻意选择**，不是技术代差；仓库里已有同款判断（`internal/retriever/mongo/rank.go` 包注释：MongoDB 4.4 无原生向量检索 ⇒ 过滤在存储侧、cosine 在 Go 侧）。
- **hermetic 测试替身**：业界通行做法是 **HTTP 层录制/回放**（固定 fixture + 本地 HTTP 服务），而不是在生产代码里留「测试分支」。本 phase 的替身端点**只存在于门禁脚本内**，生产二进制零分支 —— 与该通行做法一致（D-01 的边界见 `## Open Questions` Q1）。
- **确定性 stub LLM**：仓库已有同构先例（`internal/knowledge/ecl/stub.go` 的 `StubExtract`），用确定性规则替代模型调用，同时保留严格校验。
- **终端 demo 素材**：asciinema（录制）+ agg（渲染）是当前社区事实标准；`agg` 的 Docker 镜像长期未更新 ⇒ 优先 `cargo install --git https://github.com/asciinema/agg` 或预编译二进制（本机 `cargo` 在 PATH）。[ASSUMED: agg 的 Docker 镜像陈旧一条来自社区反馈，未取得一手来源 —— 见 S-30]

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|---|---|---|---|
| 进程内向量存储（embedded backend） | `internal/retriever/embedded`（新包） | `internal/retriever`（接口）、`internal/vecmath`（Cosine）、`internal/knowledge`（walk/frontmatter/校验） | 检索面已由 `internal/retriever/retriever.go:167-217` 收敛；排序与相似度复用 `internal/vecmath`；语料枚举必须走 `internal/knowledge/walk.go:78` / `:140` 的单 walker 契约（KBM-06） |
| 外部向量化调用（保持不变） | `internal/embedder/bge` | `internal/embedder`（接口） | D-01 要求复用现有实现、禁止兜底；`internal/embedder/bge/bge.go:47-51` 已把 1024 维与重试策略钉死 |
| stub LLM chat | `internal/server`（stub ChatModel 实现）+ `internal/verdict`（`Mode` 字段） | `internal/server/pipeline.go`（接缝，不改逻辑） | 接缝是 `Deps.PipelineChatModel`（`internal/server/pipeline.go:414`）；`Mode` 字段落在 `internal/verdict/verdict.go:82-89` 的结构体，随 `json.Marshal` 自动上 wire |
| 启动装配 / 配置分支 | `cmd/server/main.go` | `internal/kb`（registry）、`internal/knowledge`（seed） | `buildRetriever`（`:1226-1306`）与 `indexerBackend` switch（`:499-506`）都在 main.go；`Deps` 构造在 `cmd/server/main.go:1060-1087` |
| 同源静态托管 | `cmd/server`（路由挂载）+ 新增 `internal/webui`（建议） | `github.com/cloudwego/hertz/pkg/common/adaptor` | 路由注册顺序必须让 `/api/*` 先于 catch-all；`adaptor.HertzHandler` 已随 hertz v0.10.6 引入 |
| 前端类型与徽标 | `web/src/types/verdict.ts` | `web/src/components/chat/`（`DataPolicyPart` / `VerdictCard`） | `isVerdict` 守卫在 `web/src/types/verdict.ts:138-146`，不在 CONTEXT.md 点名的 `MessageBubble.tsx`（后者只是 import 者，见 Pitfall 15） |
| demo 素材与门禁 | `scripts/record-demo.sh` + `verify-phase-49.sh` + `Makefile` | `docs/demo.gif` | 门禁脚本形态照 `verify-phase-48.sh:13-52`；Makefile 现有目标见 `Makefile:129-142` |
| 知识库注册表（tier 真源） | `internal/kb` | `internal/knowledge` | tier 取值 = registry slug（不是中文名），见 `internal/retriever/milvus/filter.go:31-33`；可引用性由 `internal/kb/kb.go:267-276` 的 `Citable` 单点裁决 |

## Package Legitimacy Audit

本 phase **不安装任何外部包**（Go / npm 依赖零新增），因此无新增包的合法性审计需求。仅记录**复用的既有依赖**与其在本 phase 的使用面：

| Package | Registry | Age | Source Repo | Verdict | Disposition |
|---|---|---|---|---|---|
| `github.com/cloudwego/hertz` v0.10.6 | proxy.golang.org | 2020 起 | github.com/cloudwego/hertz | HIGH（已是 go.mod 直接依赖，CloudWeGo 官方） | 复用；新增使用面仅 `pkg/common/adaptor.HertzHandler` |
| `github.com/stretchr/testify` v1.11.1 | proxy.golang.org | 2017 起 | github.com/stretchr/testify | HIGH（已是 go.mod 直接依赖） | 复用，不新增 |
| `vitest` ^2.0.0 | npm | 2023 起 | github.com/vitest-dev/vitest | HIGH（已是 web devDependency） | 复用，不新增 |
| asciinema | PyPI | PyPI 上 2023-10-23 发布 | 官方站 `asciinema.org`（PyPI 包的 `repoUrl` 字段即此） | **SUS**（本轮 `gsd-tools query package-legitimacy check --ecosystem pypi asciinema` → `verdict: "SUS"`，`reasons: ["unknown-downloads"]`，`exists: true`, `deprecated: false`, `postinstall: null`） | 可选工具；仅产素材；**门禁不得依赖**。⚠️ SUS 判定来自「下载量未知」这一**元数据缺失**，不是来自 slop 信号；实施前建议人工确认官方安装方式（发行版包 / `pip install asciinema`）并加 planner `checkpoint:human-verify` |
| agg | **不是 crates.io 的 `agg`** | — | github.com/asciinema/agg | **SUS（同名冲突）** | crates.io 上名为 `agg` 的包是 `github.com/savge13/agg`（本轮 `check --ecosystem crates agg` → `verdict: "SUS"`, `repoUrl: "https://github.com/savge13/agg"`, `weeklyDownloads: 179`）——**与 asciinema 的 agg 无关**。⇒ 安装必须走 `cargo install --git https://github.com/asciinema/agg`（**git 源，不走 crates.io**）或官方预编译二进制。⚠️ 门禁不得依赖 |

> **门禁硬约束**：asciinema / agg 只在**生成素材**时用；`verify-phase-49.sh` 对素材的断言只用 **magic bytes**（`GIF8`）与**二进制密钥扫描**，不调用这两个工具。这样即使工具链缺失，门禁仍可判定素材合法 ⇒ `SUSP` 判定不构成对主链的阻塞。

## Validation Architecture

> `workflow.nyquist_validation: true`（`.planning/config.json`）⇒ 本节为强制项。
> 每条命令必须在**无 Milvus / 无 BGE / 无 GLM / 无网络**的干净 checkout 上可跑；每条命令都有明确的失败信号（测试失败 = 非零退出；门禁分区失败 = `fail()` 立即 `exit 1`）。

### Test Framework

| Layer | Tool | Command | Runs On |
|---|---|---|---|
| Go 单元（新增包） | `testing` + `testify` v1.11.1 | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/retriever/embedded/ -count=1"` | WSL bash（宿主 `go test` 因 duckdb 预编译静态库 TLS-ABI 不匹配而必红，非回归） |
| Go 单元（stub / 静态托管） | `testing` + `testify` | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/server/... ./internal/webui/ -count=1"` | WSL bash |
| Go 编译面检查 | `go vet` | `go vet ./...` | Windows 宿主可用（无链接步骤；`go vet ./cmd/server` 实测 exit 0） |
| Go 全量回归 | `go test` | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./... -count=1"` | WSL bash |
| 前端单测 | `vitest` ^2.0.0（复用 `web/vite.config.ts`） | `cd web && npm test -- verdict` | Windows 宿主 / Node v24.19.0 |
| Phase 门禁 | bash 分区脚本 | `bash verify-phase-49.sh` | 任意有 bash 的宿主（**不得**依赖 asciinema/agg/ollama/docker） |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|---|---|---|---|---|
| DEMO-01 | embedded backend 对 `knowledge/` 建索引并返回可引用 chunk | Go unit | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/retriever/embedded/ -run TestSearch -count=1"` | ❌ Wave 0 |
| DEMO-01 | **tier 隔离在 embedded 下不削弱**：空 tier → 0 命中；非白名单 tier → 0 命中；effective_to 过期 → 0 命中 | Go unit | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/retriever/embedded/ -run 'TestTier\|TestEffective' -count=1"` | ❌ Wave 0 |
| DEMO-01 | 排序确定性：同 query 两次 Search 顺序一致（score desc, id asc tie-break） | Go unit | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/retriever/embedded/ -run TestRank -count=1"` | ❌ Wave 0 |
| DEMO-01 | 缓存：key 含 model+dim+语料 hash；换模型/换语料 ⇒ 失效重建；损坏文件 ⇒ 重建而非部分可用 | Go unit | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/retriever/embedded/ -run TestCache -count=1"` | ❌ Wave 0 |
| DEMO-01 | 缓存条数断言 = **218**（实际 `.md` 文档数；不是 D-05 的 220 目录条目数，见 C-01/Pitfall 18） | Go unit | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/retriever/embedded/ -run TestCacheCorpusSize -count=1"` | ❌ Wave 0 |
| DEMO-01 | `RETRIEVER_BACKEND=embedded` 时**不**要求 `MONGO_URI` / `MILVUS_ADDRESS`（D-16） | Go unit（`buildRetriever` 表驱动） | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./cmd/server/ -run TestBuildRetriever -count=1"` | ❌ Wave 0 |
| DEMO-02 | 无 key 走 stub；产出 Verdict JSON 与真模型**逐字节同构**（同字段集、同 wire 形状） | Go unit | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/server/stubchat/ -run TestWireShape -count=1"` | ❌ Wave 0 |
| DEMO-02 | stub 的 answer 逐字来自 chunk；citations 的 span 真实出现在 chunk 内（grounding 不降级） | Go unit | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/server/stubchat/ -run 'TestGrounding\|TestCitation' -count=1"` | ❌ Wave 0 |
| DEMO-02 | stub 的 `confidence ≤ 0.5` 且 `review_required == true`（D-10） | Go unit | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/server/stubchat/ -run TestConfidence -count=1"` | ❌ Wave 0 |
| DEMO-02 | stub 走**同一条** grounding 校验路径，未被豁免 | Go unit（pipeline 集成） | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/server/ -run 'Stub.*Grounding\|Pipeline.*Stub' -count=1"` | ❌ Wave 0 |
| DEMO-02 | 三重标注：wire 有 `mode:"stub"` / 启动日志有 WARN / README 写了 stub 模式 | 门禁分区 P7（grep 断言） | `bash verify-phase-49.sh` | ❌ Wave 0 |
| DEMO-03 | 前端类型能解析 `mode` 字段，且 `isVerdict` 守卫不因新字段失效 | 前端 unit | `cd web && npm test -- verdict` | ❌ Wave 0 |
| DEMO-03 | 同源静态托管四分支：有文件→文件 / 无扩展名不存在→index.html / 有扩展名不存在→404 / dist 缺失→503 提示 | Go unit | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./internal/webui/ -count=1"` | ❌ Wave 0 |
| DEMO-03 | 端到端 hermetic：起服务端（测试替身 embedding 端点 + 无 key）→ 打一轮带引用的问答 → 断言 data-policy 帧含 citations 与 `mode:"stub"` | 门禁分区 P5 | `bash verify-phase-49.sh` | ❌ Wave 0 |
| DEMO-03 | 素材存在且合法：`docs/demo.gif` magic 为 `GIF8`；`scripts/record-demo.sh` 存在且可执行 | 门禁分区 P8 | `bash verify-phase-49.sh` | ❌ Wave 0 |
| DEMO-03 | 素材不含密钥：对 `docs/demo.gif` 做二进制扫描 | 门禁分区 P9 | `bash verify-phase-49.sh` | ❌ Wave 0 |
| SC5 | `bash verify-phase-49.sh` exit 0（含 hermetic 端到端） | 门禁（根脚本） | `bash verify-phase-49.sh` | ❌ Wave 0 |

> 30 秒预算说明：各单测包均只依赖进程内 slice + 假 embedder，无 I/O 等待；唯一需要外部端点的是门禁 P5，而它用的是**本地测试替身端点**（见 `## Open Questions` Q1），不触网。WSL 下 `go test` 的实际耗时需在 Wave 0 首跑后校准（见 `## Assumptions Log` A-11）。

### Sampling Rate

| 触发点 | 采样 |
|---|---|
| 每个 task 提交 | 跑该 task 触及的包的 `go test`（WSL）+ `go vet ./...`（宿主）+ 涉及的 vitest |
| 每个 wave 合并 | 全量 `go test ./... -count=1`（WSL）+ `cd web && npm test` + `bash verify-phase-49.sh` |
| Phase 收口（门禁） | 上述全部 + `bash verify-phase-49.sh` 连续两次通过（幂等性）+ 录制脚本 dry-run |
| 每次改动 `internal/verdict/verdict.go` 或 `web/src/types/verdict.ts` | **必须**加跑全量（wire 契约变更影响面广） |

### Wave 0 Gaps

需要新建的测试 / 门禁资产（**全部 ❌ = Wave 0 必须先建**）：

- [ ] `internal/retriever/embedded/embedded_test.go` —— Search / Ensure / Insert / GetChunk / GetMaxID / ListMetadata / Disconnect 的行为契约
- [ ] `internal/retriever/embedded/tier_isolation_test.go` —— 空 tier → 0 命中、非白名单 tier → 0 命中、effective_to 过期 → 0 命中（**D-TIER-03 不削弱的核心断言**）
- [ ] `internal/retriever/embedded/rank_test.go` —— 排序确定性（score desc, id asc tie-break）
- [ ] `internal/retriever/embedded/cache_test.go` —— 缓存 key 三要素、失效、损坏重建、并发首次启动、**语料条数 = 218**（C-01 / Pitfall 18）
- [ ] `internal/server/stubchat/stub_test.go` —— wire 逐字节同构、confidence ≤ 0.5、review_required、grounding 不豁免
- [ ] `internal/webui/static_test.go` —— SPA fallback 四分支 + dist 缺失
- [ ] `cmd/server/backend_switch_test.go` —— `buildRetriever` 表驱动：embedded 分支不要求 MONGO_URI/MILVUS_ADDRESS（D-16）
- [ ] `web/src/types/verdict.test.ts` —— `mode` 字段解析 + `isVerdict` 不回归
- [ ] `verify-phase-49.sh` —— 根目录门禁（P1..P9 分区、每区落日志、空日志判失败、fail-fast、收尾横幅按已运行分区计数；**成功 token 按断言组逐个绑定**，机制照 `verify-phase-48.sh:213-279`）
- [ ] `verify-phase-49.sh` 内的**替身 embedding 端点夹具** —— 必须**按键查表**（语料文本 → 向量；demo 查询 → 向量），未命中返回明确错误（Pitfall 17）；不得返回单一固定向量
- [ ] `scripts/record-demo.sh` —— 录屏脚本（SC4 证据）
- [ ] `.gitignore` 增补向量缓存目录条目 —— **必需项**，当前 40 行里没有任何缓存条目（C-04 / A-14 / Pitfall 19）
- [ ] README 新增「无 Docker / 无 API key 也能跑」小节 —— 必须逐字给出 `export RETRIEVER_BACKEND=embedded`（`.env.example` 一字不动，D-15 / Q8）

> 门禁脚本**不得**依赖 asciinema / agg / Ollama / Docker（否则就不是 hermetic 门禁）；素材存在性用 magic bytes 断言即可。

## Security Domain

> `workflow.security_enforcement` 在 `.planning/config.json` 中无独立键 ⇒ 默认生效。本 phase 的安全面集中在**信任表达**与**信任边界不回退**。

### Applicable ASVS Categories

| Category | Applicable | 本 phase 的具体面 |
|---|---|---|
| V2 Authentication | ⚠️ 部分 | stub 模式**没有** API key ⇒ 任何能连到 :8888 的人都能拿到"抽取自 chunk 的答案"。这正是 D-09 承认的风险，靠三重标注抵消（SC3：不得静默伪装成真模型）。缓解：启动 WARN 明示 + wire `mode:"stub"` + README 说明；**不**因缺 key 就把答案降级为"不可用"（那会破坏 DEMO-02 的目标） |
| V3 Session Management | ✅ | stub 不引入新会话状态；沿用既有 session/audit 链路（`cmd/server/main.go:1060-1087` 的 Deps 装配不变） |
| V4 Access Control | ✅✅ **最高优先** | **tier 隔离在 embedded 模式下不得削弱**（SC2 / D-TIER-03 / PROJECT.md 第三条护城河）。三处防线：①`Search` 的候选集构造先于打分（结构性）②`internal/retriever/milvus/filter.go:64-84` 的空 tier ⇒ `"false"` 契约要在 embedded 等价复刻 ③门禁分区 P6 端到端断言 |
| V5 Input Validation | ✅ | stub 要解析 prompt 文本（`[ref-N]` 块）⇒ 输入长度必须有上限、正则必须不 panic；`vecmath.Cosine` 长度不等会 panic（`internal/vecmath/cosine.go:50-69`）⇒ 启动即校验维度；缓存文件读取必须做长度/魔数/哈希校验（损坏即重建） |
| V6 Cryptography | ✅（轻） | 本地向量缓存**不含**任何密钥；缓存 key 中的 endpoint 用 sha256 代替原文；`docs/demo.gif` 是二进制入库物，必须确认不含 key |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---|---|---|
| **stub 模式伪装成真模型**（用户以为拿到的是模型生成的答案） | Spoofing / Repudiation | D-08 三重标注：①`verdict.Verdict.Mode`（`omitempty`，值 `"stub"`）随 `data-policy` 帧上 wire（`internal/server/chat.go:802` 的 `json.Marshal(v)` 自动携带）②启动时醒目 WARN（`cmd/server/main.go` 的 chat model 装配处）③README 写明。**额外**：`review_required=true` + `confidence ≤ 0.5` 让结论卡片本身就标"需人工复核" |
| **embedded 模式下 tier 过滤被削弱**（Skill 读到未授权 tier 的内容） | Elevation of Privilege / Information Disclosure | 候选集先于打分（示例 1）；tier 取值来自 `kb.Registry` 的 slug 而非前端传入的自由字符串；`internal/kb/kb.go:267-276` 的 `Citable` 仍是 grounding 闭包的单点裁决；门禁 P6 端到端断言"空 tier → 0 命中" |
| **向量缓存写入 secrets**（磁盘 / 备份 / 误提交泄露） | Information Disclosure | 缓存只存 model 名、维度、文档路径与向量；**不存** key、URL 原串、请求/响应原文；缓存目录加 `.gitignore`；缓存 key 用 endpoint 的 sha256 |
| **录制脚本把 API key 录进 `docs/demo.gif`**（进仓库的二进制泄露） | Information Disclosure | 录屏时 `unset GLM_API_KEY LOCAL_API_KEY`（示例 7）；门禁 P9 对 GIF 做二进制密钥模式扫描；`scripts/record-demo.sh` 里不出现任何 key 字面量 |
| **替身 embedding 端点被误当作生产组件**（有人在生产里起一个假端点） | Spoofing | 替身端点只存在于 `verify-phase-49.sh` 内部，监听 `127.0.0.1`，`BGE_M3_URL` 只在该脚本子进程环境内被覆盖；生产二进制**不含**任何内置假 embedder 分支（D-01） |
| **静态托管泄露仓库源码**（把 `.env` / `internal/` 一起伺服出去） | Information Disclosure | 只托管 `web/dist` 目录（构建产物），**不**托管仓库根；`web/dist` 缺失时返回 503 提示页而不是目录列表；`http.FileServer` 自带路径清洗，不要自己拼路径 |
| **SPA fallback 造成目录遍历 / 任意文件读** | Tampering / Information Disclosure | 路径拼接用 `filepath.Clean` + `filepath.Join(distDir, …)`，并交给 `http.FileServer`（其内部已做路径清洗）；对带扩展名的缺失路径返回 404 而不是回 index.html |
| **catch-all 路由遮蔽 `/api`**（功能绕过 + 安全过滤失效） | Tampering | 路由注册顺序固定：全部 API 路由先注册，catch-all 最后（示例 5）；门禁 P5 的端到端打 `/api/chat` 即可暴露顺序错误 |
| **stub 的引用被豁免校验**（引用了 chunk 里不存在的 span 却被当成有据） | Tampering | stub 走**同一条** `verdict.GroundingCheck` / `GroundingChecker` 路径（`internal/verdict/grounding.go:170-196` / `:293-323`），pipeline 代码零分支；门禁/单测断言 stub 的 citation 会因 span 不实而被降级 |

## Standard Stack

### Core

| 依赖 | 版本（仓库实际） | 用途 | 证据 |
|---|---|---|---|
| Go | `go 1.25.5` | 后端 | [VERIFIED: go.mod] |
| `github.com/cloudwego/hertz` | `v0.10.6` | HTTP + SSE + `pkg/common/adaptor`（静态托管接缝） | [VERIFIED: go.mod]；[VERIFIED: github.com/cloudwego/hertz@v0.10.6/pkg/common/adaptor/handler.go:35-36] |
| `github.com/cloudwego/eino` | `v0.9.14` | `model.BaseChatModel` / `schema.Message` / `adk`（stub 与 Runner 共用的类型面） | [VERIFIED: go.mod] |
| `github.com/cloudwego/eino-ext/components/model/openai` | `v0.1.13` | 真模型路径（`openai.NewChatModel`） | [VERIFIED: go.mod] |
| `github.com/stretchr/testify` | `v1.11.1` | 单元测试断言（已是直接依赖，**无需新增**） | [VERIFIED: go.mod] |
| Go 标准库 `net/http` | Go 1.25.5 | `http.FileServer` + SPA fallback（**不要手写静态文件服务**） | [VERIFIED: internal/retriever/backend.go:24-36]（`IndexerBackend` 的窄接口先例） |
| `github.com/cloudwego/eino-ext` 其它组件 | 见 go.mod | 文档解析 / 切分 | [VERIFIED: go.mod] |

### Supporting

| 依赖 | 版本 | 用途 | 证据 |
|---|---|---|---|
| `vitest` | `^2.0.0` | 前端单测（`web/src/types/verdict.ts` 的 `mode` 字段解析测试） | [VERIFIED: web/package.json:37-53] |
| `vite` | `^5.4.0` | 前端构建（`web/dist` 产物来源）+ dev proxy `/api`→:8888 | [VERIFIED: web/package.json:13-36]；[VERIFIED: web/vite.config.ts:26-33] |
| asciinema | 未安装（需 `pip install asciinema` 或发行版包） | 终端录屏 → `.cast` | [ASSUMED] |
| agg | 未安装（`cargo install --git https://github.com/asciinema/agg` 或预编译二进制） | `.cast` → `docs/demo.gif` | [ASSUMED]；[CITED: https://docs.asciinema.org/manual/agg/] |

> **STACK.md 与仓库实际的偏差（以仓库为准）**：`.planning/` 里的 STACK.md 写 "Vercel AI SDK v5 / Vite 8"，但 `web/package.json:13-36` 实际是 `"ai": "^7.0.59"` + `vite ^5.4.0`；STACK.md 写 "Hertz v0.10.2+"，go.mod 是 `v0.10.6`。**实施时以 go.mod / package.json 为准**，不要按 STACK.md 装包。

### Alternatives Considered

| 方案 | 为什么不用 |
|---|---|
| 进程内 embedder（本地 ONNX / 自实现 hashing） | D-01 明文排除（用户原话「不考虑任何兜底方案，强制要求使用外部向量化模型」）；`internal/embedder/bge/bge.go:17-26` 的包注释把 hash 兜底定性为"掩盖真实故障" |
| 提交预计算向量文件 | D-01 明文排除；且缓存 key 含语料 hash（D-14），语料一改就失效 |
| pgvector / SQLite-vec / hnswlib | 新增依赖；本 phase 明确"不引向量库"；218 篇 × 1024 维的暴力 cosine 完全够用（`internal/retriever/mongo/rank.go:27-28`：「For Phase 2's ~30-chunk corpus the in-Go cosine is instant and mathematically exact」） |
| 继续用 `docker compose up` 做 demo | D-12 排除；`docker-compose.yml` 的 `${GLM_API_KEY:?…}` 强制插值让"无 key 跑起来"根本不可能 |
| 浏览器 UI 录屏（playwright） | D-17 排除，归 Phase 50（Deferred） |
| 自己写 HTTP 静态文件服务 / 自己写 GIF 编码 | 分别由 `net/http` 与 asciinema+agg 承担（见 `## Don't Hand-Roll`） |

### Installation

本 phase **不新增任何 Go / npm 依赖**。只有两个外部命令行工具，且**只用于生成素材、不参与门禁**：

```bash
# 录制工具链（非 Go 侧依赖；门禁不得依赖它们）
pip install asciinema                                     # 或发行版包管理器安装
cargo install --git https://github.com/asciinema/agg       # 必须走 git 源；crates.io 上的 "agg" 是另一个包
```

> ⚠️ 两个包的合法性判定见 `## Package Legitimacy Audit`：**asciinema = SUS**（元数据下载量未知）、**crates.io 的 `agg` = SUS 且同名不同物**。二者都只用于产素材，**门禁不得依赖**，因此不阻塞主链；但实施前建议人工确认安装来源（planner `checkpoint:human-verify`）。

### Version verification

```bash
# Go 侧：确认零新增依赖
git diff go.mod go.sum          # 期望：空 diff
# 前端：确认零新增依赖
git diff web/package.json       # 期望：空 diff
# 工具链（可选，仅生成素材时需要）
asciinema --version && agg --version
```

## Architecture Patterns

### System Architecture Diagram

```
                        ┌──────────────────────────────────────────────┐
   用户 clone 仓库       │  hermetic 模式（无 Docker / 无 key / 无内网） │
                        └───────────────┬──────────────────────────────┘
                                        │  RETRIEVER_BACKEND=embedded
                                        ▼
  ┌───────────────────────── cmd/server（:8888，单进程） ─────────────────────────┐
  │                                                                              │
  │  main()                                                                      │
  │   ├─ envRequired("BGE_M3_URL")  ← 仍然必填（D-11 / D-01）                     │
  │   ├─ envOr("LOCAL_API_KEY", os.Getenv("GLM_API_KEY"))  ← 改为可选（D-11）      │
  │   ├─ buildRetriever("embedded")   ★新增 case                                 │
  │   │     └─ internal/retriever/embedded                                      │
  │   │           ├─ Ensure: 读本地向量缓存（命中则跳过 embedding）               │
  │   │           ├─ 未命中 → knowledge.WalkRegisteredDirs + WalkDocs            │
  │   │           │        → emb.Embed（bge，1024 维，外部端点）                  │
  │   │           │        → validateTiers / validateEffective                   │
  │   │           │        → 写缓存（key = model+dim+corpusHash）                 │
  │   │           └─ Search: tier PRE-filter → effective_to PRE-filter           │
  │   │                      → vecmath.Cosine → mongo/rank.go 同款 (score,id) 排序 │
  │   │                                                                             │
  │   ├─ 无 key → stub ChatModel（model.BaseChatModel）                           │
  │   │        解析 user message 里的 [ref-N] 块 → 逐字抽 answer + citations      │
  │   │        产出与真模型逐字节同构的 Verdict JSON                                │
  │   │                                                                             │
  │   └─ 路由注册顺序：/api/* … → catch-all /*  ★新增（adaptor.HertzHandler）     │
  │          catch-all = http.FileServer(web/dist) + SPA fallback(index.html)       │
  └──────────────┬───────────────────────────────────────────────┬───────────────┘
                 │ HTTP POST /v1/embeddings                     │ 静态资源
                 ▼                                              ▼
  ┌────────────────────────────┐              ┌──────────────────────────────┐
  │ 外部向量化端点（D-01 强制） │              │ web/dist（前端构建产物）      │
  │ 首选 Ollama bge-m3（无 key）│              │ Vite dev 时则由 :5173 提供    │
  │ 任意 OpenAI 兼容端点       │              │ web/vite.config.ts 代理 /api  │
  └────────────────────────────┘              └──────────────────────────────┘
```

### Recommended Project Structure

```
internal/retriever/embedded/          # 新增：进程内向量存储
  embedded.go            # Client 实现 retriever.Retriever（8 个方法）
  rank.go                # 纯函数 rank（照搬 mongo/rank.go 的 (score desc, id asc)）
  seed.go                # SeedFromKnowledgeDir（照搬 mongo/seed.go 的解耦形态）
  cache.go               # 本地向量缓存：key 组成 / 读写 / 损坏检测
  cache_test.go          # 缓存 key 三要素、损坏重建、并发首次启动
  rank_test.go           # tier PRE-filter / empty tiers → 0 hit / tie-break 稳定
internal/verdict/…                    # verdict.go 加 Mode 字段（omitempty）
internal/server/stubchat/             # 新增（或 internal/server/stub_llm.go）
  stub.go                # model.BaseChatModel 实现：从 prompt 抽 chunk → Verdict JSON
  stub_test.go           # wire 逐字节同构、confidence≤0.5、review_required=true
internal/webui/                       # 新增：静态托管 + SPA fallback（建议独立包）
  static.go              # http.FileServer + SPA fallback + dist 缺失行为
  static_test.go
cmd/server/main.go                    # 3 处改动：env 必填项 / buildRetriever case / indexerBackend case
web/src/types/verdict.ts              # Verdict 加 mode?: 字段
scripts/record-demo.sh                # 新增：录屏脚本（SC4 证据）
verify-phase-49.sh                    # 新增：根目录门禁（与 45-48 同级）
docs/demo.gif                         # 新增：真实录屏素材
Makefile                              # 新增 demo-gif / verify-phase-49 目标
```

### Pattern 1 — 后端策略 switch（`buildRetriever`）

`cmd/server/main.go:490-494` 已经是「读 `RETRIEVER_BACKEND` → 派发」的策略入口：

```go
backend := strings.ToLower(os.Getenv("RETRIEVER_BACKEND"))
if backend == "" { backend = "milvus" }
r, backendLabel := buildRetriever(ctx, backend, emb, kbReg)
```

**照此模式加一个 `case "embedded":`**（在 `buildRetriever` 的 `case "none":` 与 `case "mongo":` 之后），不要动 `none` 分支（D-06：`noopRetriever` 的 degraded default-deny 语义一字不动）。[VERIFIED: cmd/server/main.go:490-494, 1226-1306]

### Pattern 2 — 窄接口 + 组合（`IndexerBackend`）

`internal/retriever/backend.go:24-36`：

```go
type IndexerBackend interface {
	Insert(ctx context.Context, docs []Document) error
}
const IndexerBackendNotWiredError = "indexer backend not wired (server started without a retriever backend)"
```

embedded 同时满足 `retriever.Retriever`（8 方法）与 `retriever.IndexerBackend`（1 方法），因为两者都只要求 `Insert`。**不要**为 embedded 造第三套接口。[VERIFIED: internal/retriever/backend.go:24-36]

### Pattern 3 — 纯函数排序（可测性优先）

`internal/retriever/mongo/rank.go:94-132` 的 `rank(rows, queryVec, k) []retriever.Hit` 是纯函数：无 I/O、无全局状态。**照搬**到 embedded 包（连注释一起），并保留它的 tie-break 语义：

```go
sort.SliceStable(out, func(i, j int) bool {
    if out[i].score != out[j].score {
        return out[i].score > out[j].score // higher cosine first
    }
    return out[i].row.ID < out[j].row.ID // tie-break: lower ID first
})
```

这条不是洁癖：`internal/retriever/mongo/rank.go:92-93` 的注释写明 tie-break 的目的是 "so re-runs produce identical ordering — important for eval reproducibility (Ragas)"。评测要可复现，排序就必须确定性。[VERIFIED: internal/retriever/mongo/rank.go:92-132]

### Pattern 4 — 单 walker 契约（KBM-06）

`internal/knowledge/walk.go:78` 的 `WalkDocs(absDir)` 与 `internal/knowledge/walk.go:140` 的 `WalkRegisteredDirs(knowledgeDir, reg, audit)` 是**唯一**的语料枚举入口。`internal/knowledge/walk.go:22-49` 的包注释记录了这段历史：「before this file existed each kept its own hand-copied list of the four tier directories, and the two lists could drift independently (26-RESEARCH Pitfall 4)」。

**embedded 的 seed 必须用这两个函数**，不得第三次手写目录遍历。[VERIFIED: internal/knowledge/walk.go:22-49, 78, 140]

### Pattern 5 — fail-closed 的空 tier 拒绝（跨后端契约）

三个后端必须给出**同一个**空 tier 语义：

| 后端 | 实现 | 证据 |
|---|---|---|
| Milvus | `BuildMilvusFilter` 空 tier → 字面量 `"false"` | [VERIFIED: internal/retriever/milvus/filter.go:64-84] |
| MongoDB | `{tier: {$in: []}}` → 匹配 0 行 | [VERIFIED: internal/retriever/mongo/mongo.go:416-425] |
| embedded（新增） | 候选集构造阶段直接返回空（**不是**先打分后过滤） | 需实现（见 `## Code Examples` 示例 1） |

`internal/retriever/mongo/mongo.go:405-410` 的注释把这条讲成结构性保证：

> Always build the filter, always run the find.

embedded 的"存储层"就是 Go 代码，因此**"先构造候选集再算 cosine"** 就是它的等价形态——但必须写成两段式（先过滤候选、再算 cosine），不能写成"全量打分后再 filter"，否则 tier 隔离就退化成事后过滤（一旦打分逻辑出错，隔离就失效）。[VERIFIED: internal/retriever/mongo/mongo.go:405-425]

### Pattern 6 — LLM 接缝（nil-gate 语义）

`internal/server/pipeline.go:414`：

```go
if s.PipelineChatModel != nil {
    … return s.streamAndAssemble(ctx, c, systemPrompt, userPrompt, historyPtrs, chatModelOpts)
}
```

`internal/server/server.go:299-302` 写明 `nil` 的语义是"Step 8 falls back to Runner.Query + drainFinalContent (the Phase 2 byte-identical path)"。

**stub 走 `PipelineChatModel` 通道**（不是新增 if 分支），理由：

1. D-07 要求"输出仍是合法 `verdict.Verdict` JSON，wire 形状与真模型完全一致"——走同一条 `streamAndAssemble` → `json.Unmarshal` → `verdict.Validate` 路径是唯一能保证这一点的做法；
2. D-10 要求"不豁免任何校验"——校验发生在 pipeline 里，stub 绕开 pipeline 就绕开了校验；
3. 改动面最小，且不触碰 D-TIER-03 的信任边界。

[VERIFIED: internal/server/pipeline.go:397-443] [VERIFIED: internal/server/server.go:287-302]

### Pattern 7 — 哨兵 0 的 effective_to（避免 NULL 陷阱）

`internal/retriever/mongo/mongo.go:364-396` 的注释记录了 sentinel-0 设计（`effective_to == 0` 表示永久有效）是为绕开 "MongoDB NULL-evaluates-FALSE" 坑。embedded 沿用同一表示：

```go
{Key: "$or", Value: bson.A{
    bson.D{{Key: "effective_to", Value: 0}},
    bson.D{{Key: "effective_to", Value: bson.D{{Key: "$gte", Value: now}}}},
}},
```

[VERIFIED: internal/retriever/mongo/mongo.go:364-425]

### Pattern 8 — 缓存条目 + 并发首次启动（原子锁，不用 sleep 猜时长）
D-14 要求缓存 key 含「模型 + 维度 + 语料哈希」三要素。落地成三件事：

1. **条目内容** = magic(`SFVC`) + version + model + dim(1024) + docCount(**218**) + payloadHash(sha256 全部向量字节) + 每条 (id, tier, source, effectiveFrom, effectiveTo, vec)。**不存** key、**不存** `BGE_M3_URL` 原串（只存它的 sha256）。
2. **陈旧判定** = magic / version / dim / docCount / payloadHash 任一不符 ⇒ **整份作废**并重建（绝不"部分可用"）。语料哈希**基于内容**（相对路径 + 字节），**绝不用 mtime**——`git clone` / `git checkout` 会改写 mtime。
3. **并发首次启动** = 用 `os.OpenFile(path, os.O_CREATE|os.O_EXCL, 0o600)` 原子创建锁文件做互斥；抢不到锁的进程**轮询等待**缓存文件出现（带超时），超时后**明确报错退出**，**不**降级为假向量（D-01）。

**落地位置**：仓库内 gitignored 目录 `.skillforge-cache/vectors/`（Q2 建议）——**必须同时新增一条 `.gitignore` 规则**，因为当前 `.gitignore` 40 行里没有任何缓存条目（C-04 / A-14 / Pitfall 19）。

### Anti-Patterns to Avoid

| 反模式 | 为什么禁止 |
|---|---|
| 在 `internal/embedder/bge` 里加任何"端点不可达就用本地向量"的分支 | D-01 明文排除；`internal/embedder/bge/bge.go:17-26` 的 Pitfall 8 把它定性为"用语义无意义的向量掩盖真实故障" |
| 第三次手写 knowledge 目录遍历 | `internal/knowledge/walk.go:22-49` 记录了两次漂移事故（26-RESEARCH Pitfall 4） |
| 改 `knowledge.Index` 让它变成后端无关 | `internal/retriever/mongo/seed.go:38-48` 明确说这是"应该做但至今没做"的 refactor；在本 phase 动它会牵连 milvus 路径，风险与收益不成比例 |
| 给 stub 单独开一条"跳过 grounding"的分支 | D-10 明文排除；而且会直接违反 D-TIER-03 的信任边界叙事 |
| 在 `default:` 分支里塞 embedded | `cmd/server/main.go:499-506` 的 default 做 `r.(*milvus.MilvusClient)` 断言 → **panic** |
| 把 `web/dist` 之外的东西挂到静态托管 | 源码与 `.env` 一旦可下载，本机可达即等于泄密 |
| 在录制脚本里 export 真实 `GLM_API_KEY` | 录屏会把 key 录进 `docs/demo.gif` 并进仓库 |
| 把 `BGE_M3_URL` 原文写进缓存文件名 | URL 里可能带 token；文件名会进 git status / 日志 |

## Don't Hand-Roll

| 能力 | 用什么 | 不要自己写 |
|---|---|---|
| 向量存储 | `[]retriever.Document` + `vecmath.Cosine` + 暴力排序 | HNSW / IVF / PQ 索引、量化、倒排桶——218 篇文档不需要；`internal/retriever/mongo/rank.go:27-28` 已经给出"in-Go cosine is instant and mathematically exact"的结论 |
| 向量相似度 | `internal/vecmath/cosine.go:50-69` 的 `Cosine` | 第二份 cosine 实现（包注释 L17-34 明说会"silently drift (different zero-norm handling, etc.)"） |
| 语料枚举 | `internal/knowledge/walk.go:78` / `:140` | 第三个 walker |
| 静态文件服务 | `net/http` 的 `http.FileServer` + `hertz/pkg/common/adaptor.HertzHandler` | 自己写 MIME 表、自己写路径清洗、自己写 Range 处理 |
| SPA fallback | 一个 `http.Handler`（复刻 `Dockerfile.frontend:39` 的 `try_files $uri /index.html` 语义） | 引入前端路由库或 Node 中间层 |
| 录屏 | asciinema | 自己写 pty 捕获 / ANSI 解析 / 帧序列化 |
| GIF 渲染 | agg | 自己写 GIF 编码器；ffmpeg 虽在本机可用，但转换链路比 agg 长且体积不可控 |
| 向量缓存序列化 | 自定义紧凑二进制头 + 原始 `float32` LE（见 `## Code Examples` 示例 2） | JSON 存 218×1024 float（体积约 10×）；或用 gob（跨 Go 版本不保证兼容） |
| 嵌入向量化 | 现有 `internal/embedder/bge` | 任何"端点不可达就本地算"的分支（D-01） |
| 语料内容哈希 | `crypto/sha256` + 自己按路径排序遍历（见 Pattern 8） | 用 mtime / size 做"版本"——`git clone` / `git checkout` 会改写 mtime；用 `hashicorp/go-version` 之类新依赖 |
| 进程互斥（缓存首启） | `os.OpenFile(..., os.O_CREATE\|os.O_EXCL, 0o600)` | `sleep` 猜时长、"建目录当锁"、引入 `gofrs/flock`（新增依赖） |

**Key insight:** 本 phase 的全部新增逻辑都是**编排**（orchestration），不是**发明**。三个需求各自都能在仓库里找到一条已经存在的、风格一致的实现路径；真正的风险不在"写不出来"，而在"接错位置"——`cmd/server/main.go:499-506` 的 type assertion panic、`cmd/server/main.go:302` 的 eager 求值、以及把 `knowledge.Index` 改成后端无关这三处，都是"接错位置"的典型。

## Common Pitfalls

### Pitfall 1 — `indexerBackend` switch 的 default 分支会 panic
`cmd/server/main.go:499-506` 的形状是：`case "mongo": … ; default: indexerBackend = milvus.NewIndexerBackend(r.(*milvus.MilvusClient))`。embedded backend 落进 `default` 时，`r.(*milvus.MilvusClient)` 是**非法类型断言 ⇒ 运行时 panic**（不是编译错误）。**必须显式加 `case "embedded":`**，且该分支的 `IndexerBackend` 就是 embedded 客户端自身（它已实现 `Insert`）。[VERIFIED: cmd/server/main.go:499-506]

### Pitfall 2 — `envRequired` 作为 `envOr` 默认值被 eager 求值
`cmd/server/main.go:302`：
```go
glmKey := envOr("LOCAL_API_KEY", envRequired("GLM_API_KEY"))
```
Go 的实参在调用前求值 ⇒ `envRequired("GLM_API_KEY")` **总是**先执行。`cmd/server/main.go:165-171` 的 `envRequired` 在变量为空时 `log.Fatalf` ⇒ **即使 `LOCAL_API_KEY` 已设，只缺 `GLM_API_KEY` 也会直接退出**。D-11「GLM_API_KEY 改可选」必须把这一行改写成不触发 `envRequired` 的形式（例如 `glmKey := envOr("LOCAL_API_KEY", os.Getenv("GLM_API_KEY"))`，再在后面按 `glmKey == ""` 分支）。[VERIFIED: cmd/server/main.go:165-178, 302]

### Pitfall 3 — 第三份目录遍历
`internal/knowledge/walk.go:22-49` 的包注释是活教材：两份手抄的目录清单"could drift independently (26-RESEARCH Pitfall 4)"。embedded 的 seed 只能用 `WalkRegisteredDirs` + `WalkDocs`。[VERIFIED: internal/knowledge/walk.go:22-49]

### Pitfall 4 — `knowledge.Index` 无法复用
`internal/knowledge/indexer.go:168` 的签名是 `Index(ctx, knowledgeDir, emb embedder.Embedder, mc client.Client, cfg milvus.Config, reg kb.Registry)` —— 耦合 milvus SDK。`internal/retriever/mongo/seed.go:38-48` 解释了为什么 mongo 另写了一份 seed。embedded **照抄 mongo 的解耦形态**：`SeedFromKnowledgeDir(ctx, knowledgeDir, emb, reg)`，不碰 milvus。[VERIFIED: internal/knowledge/indexer.go:168] [VERIFIED: internal/retriever/mongo/seed.go:38-48]

### Pitfall 5 — `stripFrontmatter` 需要第 N 份副本
`internal/retriever/mongo/seed.go:186` 的 `stripFrontmatter` 是包内私有函数，embedded 包用不到。两条路：(a) 若 `<canonical_refs>` 允许改 `internal/knowledge`，导出 `knowledge.StripFrontmatter` 让 mongo/seed.go 与 embedded 共用（**更优，消除漂移**）；(b) 否则在 embedded 内复制一份并注释指向真源（与 mongo 既有先例一致）。注意 `internal/knowledge/indexer.go:586` 已有另一份 `splitFrontmatter`（返回 body + frontmatter 两段），语义不完全相同，别混用。[VERIFIED: internal/retriever/mongo/seed.go:186-200] [VERIFIED: internal/knowledge/indexer.go:586]

### Pitfall 6 — 空流 = 错误，stub 必须吐 token
`internal/server/pipeline.go:651-656` 的注释：「An empty stream (zero tokens at EOF) is an ERROR, not an empty result」。stub 的 `Stream` 必须返回**至少一个 chunk**（哪怕整段 JSON 一次吐完），否则 pipeline 判失败。[VERIFIED: internal/server/pipeline.go:608-656]

### Pitfall 7 — `vecmath.Cosine` 长度不等会 panic
`internal/vecmath/cosine.go:50-69`：`length mismatch` 直接 panic。若 BGE 端点返回的维度不是 1024（例如换成了别的模型），在**打分路径**上会 panic 掉整个 server。⇒ 启动时校验 `emb.Dimensions() == 1024`（bge 侧已固定 `dimensions = 1024`），并把实际维度写进缓存 key。[VERIFIED: internal/vecmath/cosine.go:50-69] [VERIFIED: internal/embedder/bge/bge.go:47-51]

### Pitfall 8 — 缓存 key 用 mtime 会在 clone 后失效或错命中
`git clone` 会重写 mtime；`git checkout` 改文件也会重写 mtime。缓存 key 必须基于**内容**（路径 + 字节的 sha256），不是 mtime，也不是文件数。

### Pitfall 9 — 缓存文件名里带 URL 会泄 token
`BGE_M3_URL` 可能带 query token。缓存 key 应对 URL 取 sha256，且**不**把原始 URL 写进文件名或日志。

### Pitfall 10 — 随机向量会让 `retrievalScoreThreshold` 永远拦截
`internal/server/pipeline.go:64` 的 `retrievalScoreThreshold = 0.3`，命中分数低于它会走 `low_retrieval_score` 拒答。若门禁用的替身端点返回**互不相关**的随机向量，cosine ≈ 0 ⇒ 永远拒答 ⇒ 门禁只能验"拒答"而验不了"带引用的问答"。替身端点必须产出**与文本相关**的确定性向量（见 `## Code Examples` 示例 6）。[VERIFIED: internal/server/pipeline.go:64]

### Pitfall 11 — tier 过滤退化为事后过滤
把"全量算 cosine 再按 tier 过滤"当成 tier 隔离，等于把信任边界交给打分逻辑。必须先构造候选集再打分（`## Architecture Patterns` Pattern 5）。[VERIFIED: internal/retriever/mongo/mongo.go:405-425]

### Pitfall 12 — 录制脚本把 key 录进素材
`docs/demo.gif` 是**进仓库的二进制**。录制时必须用**空 key**（顺带展示 stub 模式的启动 WARN，正好是 D-08②），并在门禁里对产物做一次 key 模式扫描。

### Pitfall 13 — Hertz 路由顺序：catch-all 先注册会遮蔽 `/api`
catch-all `/*` 一旦先注册，`/api/*` 就永远打不到。**先注册全部 API 路由，最后注册 catch-all**。

### Pitfall 14 — `verdict.Validate` 要求 citations 非 nil
`internal/verdict/validate.go:73-84`：`v.Citations == nil` → `"citations field missing (must be array, can be empty)"`。stub 必须输出 `"citations": []` 或非空数组，不能省略该键。[VERIFIED: internal/verdict/validate.go:73-84]

### Pitfall 15 — 前端真正的类型定义不在 CONTEXT.md 点名的文件里
D-08 写的是「`web/src/components/chat/MessageBubble.tsx` 的 `isVerdict` 类型守卫」，但 `isVerdict` 实际定义在 `web/src/types/verdict.ts:138-146`，`MessageBubble.tsx:56` 只是 import。⇒ 改 `web/src/types/verdict.ts` 的 `Verdict` 接口，`isVerdict` 本身只判 `status`，**不会**因新增 `mode` 字段而破。[VERIFIED: web/src/types/verdict.ts:64-79, 138-146]

### Pitfall 16 — `docs/demo.gif` 当前是死链
`README.md:18` 引用 `docs/demo.gif` 而文件不存在（`docs/` 下只有 `threat-model/`）。本 phase 产出真实素材即可，**不要**同时改 README 的引用路径（那是 Phase 50 的范围）。[VERIFIED: README.md:18]

### Pitfall 17 — 固定录制的向量会让门禁的端到端分区永远拿不到引用
替身端点如果对**所有**输入返回同一条固定向量，那么查询向量与任意 chunk 向量的 cosine 近乎 0，必然低于 `internal/server/pipeline.go:64` 的 `retrievalScoreThreshold = 0.3` ⇒ 端到端分区只能验到「拒答」，验不到「带引用的问答」。⇒ 夹具必须**按键查表**（语料文本 → 录制向量；demo 查询文本 → 录制向量），未命中的键返回**明确错误**而不是兜底向量。这样门禁既能断言 `citations` 非空，又不会引入随机性。[VERIFIED: internal/server/pipeline.go:64; internal/embedder/bge/bge.go:142-208]

### Pitfall 18 — 把 D-05 的 220 当成"文档数"会让缓存条数断言永远失败
D-05 的「authority 16 + sql-guides 202 + draft 1 + historical 1」统计的是**目录条目**；磁盘上 `draft` / `historical` / `personal` 三个目录**各只有一个 `.gitkeep`、0 篇 `.md`**，真正可入库的是 **218 篇 `.md`**。⇒ 缓存里记录/断言的**条数写 218**；`corpusHash` 的输入也只含这 218 篇（否则空目录的存在/缺失会无谓地让缓存失效）。[VERIFIED: `find knowledge -name '*.md' -type f | wc -l` → 218]

### Pitfall 19 — 忘了给向量缓存加 `.gitignore` 规则
`.gitignore` 全文 40 行里**没有任何向量缓存条目**（只有 `.env` / `*.exe` / `/spiked` / `spike-*trace.json` / `.DS_Store` / `node_modules/` / `web/dist/` / `skillforge.db*` / `/body_gbk_*` / `/body_quantum.json` / `/reports/` / `/eval/reports/` / `/data/` / `/skillforge-server*`）。D-14 要求缓存「不入仓库」，而这条规则**必须显式新增**——否则 `git status` 必脏、缓存可能被误提交。⇒ Wave 0 必须包含这一步。[VERIFIED: .gitignore:1-40]

## Code Examples

### 示例 1 — embedded 的 tier + effective_to 双 PRE-filter（必须两段式）

```go
// internal/retriever/embedded/embedded.go
// Search 必须先把 tier 与 effective_to 过滤成候选集，再打分。
// 禁止"全量打分后再过滤"——那会把信任边界交给打分逻辑。
func (c *Client) Search(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]retriever.Hit, error) {
    if topK <= 0 {
        return nil, errors.New("embedded.Search: topK must be > 0")
    }
    if len(queryVec) == 0 {
        return nil, errors.New("embedded.Search: queryVec is empty")
    }
    // 阶段一：构造候选集（结构性拒绝；空 tiers → 0 命中，refusal-by-construction）
    candidates := make([]row, 0, len(c.docs))
    for _, d := range c.docs {
        if !tierAllowed(tiers, d.Tier) {
            continue
        }
        if d.EffectiveTo != 0 && d.EffectiveTo < now {
            continue // effective_to PRE-filter（与 tier 同层，不是 post-filter）
        }
        candidates = append(candidates, row{...})
    }
    if len(candidates) == 0 {
        return nil, nil // 0 命中 ⇒ 调用方按 no_authorized_source 拒答
    }
    // 阶段二：打分 + 确定性排序（照搬 internal/retriever/mongo/rank.go:94-132）
    return rank(candidates, queryVec, topK), nil
}
```

对应契约证据：
- 空 tier ⇒ 0 命中（Milvus 版：`internal/retriever/milvus/filter.go:64-84` 返回字面量 `"false"`；Mongo 版：`internal/retriever/mongo/mongo.go:416-425` 的 `{tier: {$in: []}}`）
- `effective_to` 哨兵 0 = 永久有效（`internal/retriever/mongo/mongo.go:364-396`）
- 排序确定性（`internal/retriever/mongo/rank.go:109-114`）
- `topK <= 0` / 空 queryVec 的错误文案与 mongo 版对齐（`internal/retriever/mongo/mongo.go:398-403`）

### 示例 2 — 缓存 key 与文件格式（D-14 三要素）

```go
// internal/retriever/embedded/cache.go
// cacheKey 必须包含 model + dimension + 语料 hash（D-14）。
// URL 也参与哈希（BGE_M3_URL 可能带 token），但原串绝不落盘、不进文件名。
func cacheKey(model string, dim int, corpusHash string, endpointHash string) string {
    h := sha256.New()
    fmt.Fprintf(h, "v1\x00%s\x00%d\x00%s\x00%s", model, dim, corpusHash, endpointHash)
    return hex.EncodeToString(h.Sum(nil))
}

// corpusHash 基于内容（路径 + 字节），绝不用 mtime：clone/checkout 会改写 mtime。
func corpusHash(entries []knowledge.DocEntry, read func(rel string) ([]byte, error)) (string, error) {
    h := sha256.New()
    for _, e := range entries { // entries 已由 knowledge.WalkDocs 按路径排序 ⇒ 顺序确定
        b, err := read(e.RelPath)
        if err != nil {
            return "", err
        }
        fmt.Fprintf(h, "%s\x00%d\x00", e.RelPath, len(b))
        h.Write(b)
    }
    return hex.EncodeToString(h.Sum(nil)), nil
}

// 文件格式：紧凑二进制头 + 原始 float32 小端。
// JSON 存 218×1024 float 体积约 10×，没有必要。
//
//	magic      [4]byte  "SFVC"
//	version    uint32   1
//	modelLen   uint16 + model bytes
//	dim        uint32
//	docCount   uint32
//	payloadHash [32]byte  ← sha256(全部向量字节)，用于损坏检测
//	then per doc: id int64 | tierLen uint16 + tier | sourceLen uint16 + source
//	              | ef int64 | et int64 | vec(dim × float32 LE)
//
// 读取时校验 magic / version / dim / 长度 / payloadHash；
// 任一不符 ⇒ 视作未命中，重建（绝不"部分可用"）。
```

### 示例 3 — stub ChatModel：从 prompt 还原证据，产出真模型同构的 Verdict

```go
// internal/server/stubchat/stub.go
// stub 实现 model.BaseChatModel，产出与真模型逐字节同构的 Verdict JSON。
// 证据来源：buildPrompt 已把 chunk 渲染进 user message（internal/server/pipeline.go:770-785）：
//
//	[ref-12] tier=authority source=authority/travel_expense.md
//	<chunk 正文>
//	---
//
// stub 解析这些块 → 逐字抽 answer + 指向真实 chunk 的 citations。
// pipeline 的 parse → Validate → GroundingCheck → GroundingChecker 全部原样跑过（D-10）。

var refBlock = regexp.MustCompile(`(?ms)^\[ref-(\d+)\] tier=(\S+) source=(\S+)\n(.*?)\n---$`)

func (s *Stub) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (schema.StreamReader[*schema.Message], error) {
    raw, err := s.buildVerdictJSON(msgs)
    if err != nil {
        return nil, err
    }
    // 必须至少一个 chunk：空流在 internal/server/pipeline.go:651-656 被判为 ERROR。
    sr, sw := schema.Pipe[*schema.Message](1)
    go func() {
        defer sw.Close()
        sw.Send(schema.AssistantMessage(raw, nil))
    }()
    return sr, nil
}

func (s *Stub) buildVerdictJSON(msgs []*schema.Message) (string, error) {
    blocks := extractRefBlocks(msgs) // 解析 [ref-N] 块
    if len(blocks) == 0 {
        return "", errors.New("stubchat: no retrieved chunks in prompt")
    }
    top := blocks[0]
    span := firstSentence(top.content) // 抽取规则由 executor 定；必须逐字来自 chunk
    v := verdict.Verdict{
        Status:         "ok",
        Confidence:     0.5,                      // D-10：上限 0.5
        ReviewRequired: true,                     // D-10：强制
        Answer:         span,
        Mode:           verdict.ModeStub,          // D-08①：omitempty 字段
        Citations: []verdict.Citation{{
            ChunkID: top.id, Span: span, Tier: top.tier,
        }},
    }
    b, err := json.Marshal(v)
    if err != nil {
        return "", err
    }
    return string(b), nil
}
```

关键点：
- `span` 必须逐字出现在 chunk 正文里 ⇒ `internal/verdict/grounding.go:170-196` 的 `GroundingCheck`（`strings.Contains(Normalize(chunkText), Normalize(c.Span))`）才能通过，D-10「不豁免任何校验」成立；
- `citations` 必须是**非 nil** 的数组（`internal/verdict/validate.go:73-84` 对 nil 报错）；
- `confidence = 0.5` 与 `review_required = true` 自洽（`internal/verdict/verdict.go:82-89` 的注释：`review_required` 为 true 当 `Confidence < 0.9`）；
- `Status` 取值必须在 `internal/verdict/validate.go:47-52` 的 `ValidStatuses` 内（`ok` / `needs_input` / `refused` / `error`）。

### 示例 4 — 建议加一道 server 侧兜底钳制（防御纵深）

```go
// stub 不是 Mode 字段的唯一写者，因此钳制应放在 server 侧，而不是只放在 stub 里。
// 位置：internal/server/pipeline.go 的 Validate 之后、grounding 之前。
if v.Mode == verdict.ModeStub {
    if v.Confidence > 0.5 {
        v.Confidence = 0.5
    }
    v.ReviewRequired = true
}
```

### 示例 5 — 同源静态托管（零新依赖）

```go
// internal/webui/static.go
// 用 net/http 的 FileServer + 自写 SPA fallback，再挂进 Hertz。
// hertz v0.10.6 的 pkg/common/adaptor 已随 hertz 直接依赖引入 ⇒ 无新依赖。
// 语义基线：Dockerfile.frontend:39 的 nginx 配置
//
//	location / { try_files $uri /index.html; }
//	location /api { proxy_pass http://backend:8888; ... }
func Handler(distDir string) (http.Handler, error) {
    st, err := os.Stat(distDir)
    if err != nil || !st.IsDir() {
        // web/dist 缺失时的行为（Claude's Discretion）：明确提示，不静默 404
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "text/plain; charset=utf-8")
            w.WriteHeader(http.StatusServiceUnavailable)
            io.WriteString(w,
                "SkillForge: 前端未构建。请先执行：\n  cd web && npm install && npm run build\n"+
                    "（开发模式请另开：cd web && npm run dev → http://localhost:5173）\n")
        }), nil
    }
    files := http.FileServer(http.Dir(distDir))
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // SPA fallback：静态文件不存在且路径无扩展名 ⇒ 回 index.html
        if _, err := os.Stat(filepath.Join(distDir, filepath.Clean(r.URL.Path))); err != nil {
            if filepath.Ext(r.URL.Path) == "" {
                http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
                return
            }
        }
        files.ServeHTTP(w, r)
    }), nil
}

// 路由挂载（cmd/server/main.go）：必须最后注册 catch-all，
// 否则会遮蔽 /api/*（Pitfall 13）。
h.GET("/*", adaptor.HertzHandler(webui.Handler("web/dist")))
```

### 示例 6 — 门禁里的替身 embedding 端点（只用于测试，绝不进生产路径）

```go
// 思路：门禁需要一个"外部向量化端点"，但不能依赖内网/真实 Ollama。
// 生产路径不动：internal/embedder/bge 仍然只会在端点不可达时明确失败（D-01）。
// 这里起一个**仅测试用**的本地端点，让它表现为一个外部模型服务。
//
// 关键：不能返回互不相关的随机向量。
// internal/server/pipeline.go:64 的 retrievalScoreThreshold = 0.3 会把随机向量全部拦成
// low_retrieval_score 拒答 ⇒ 门禁只能验"拒答"，验不了"带引用的问答"。
// hashing vectorizer（按 token hash 落桶 + L2 归一化）让 cosine 反映词面重叠，
// 同主题文本能拿到 > 0.3 的分数，且完全确定性、可复现。
func embedHash(text string, dim int) []float32 {
    v := make([]float32, dim)
    for _, tok := range tokenizeCJK(text) {
        h := fnv32(tok)
        v[h%uint32(dim)] += 1
        if h&1 == 1 {
            v[(h/7+1)%uint32(dim)] -= 1 // 第二个哈希位，降低单桶碰撞影响
        }
    }
    return l2normalize(v)
}
```

> 该替身端点**只在 `verify-phase-49.sh` 内部启动**，只监听 `127.0.0.1`，且 `BGE_M3_URL` 仅在该脚本的子进程环境里被覆盖。生产二进制里不存在任何"内置假 embedder"的分支（这是 D-01 的边界）。详见 `## Open Questions` Q1（需 planner 确认这不违反 D-01 的字面约束）。

### 示例 7 — 录屏脚本骨架（D-17/D-20/SC4）

```bash
#!/usr/bin/env bash
# scripts/record-demo.sh —— 终端 CLI 演示录制（D-17：录终端，不是浏览器 UI）
# SC4 证据：脚本本身就是"quickstart 逐步实测通过"的执行记录。
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# 关键安全约束：绝不在录制环境里导出真实 key。
# 空 key 反而会展示 stub 模式的启动 WARN（D-08②）——那正是本 demo 要讲的事。
unset GLM_API_KEY LOCAL_API_KEY || true
export BGE_M3_URL="${BGE_M3_URL:-http://localhost:11434/v1/embeddings}"
export RETRIEVER_BACKEND=embedded

OUT_DIR="$(mktemp -d)"; trap 'rm -rf "$OUT_DIR"' EXIT
CAST="$OUT_DIR/demo.cast"

# 1) 启动后端（后台，输出进日志文件，不进终端画面）
# 2) 一轮带引用的问答（curl /api/chat，终端可见 citations）
# 3) 一次 Skill 切换（cp -r examples/skills/compliance skills/ && rm -r skills/simple-qa）
# 4) 停后端
asciinema rec -c 100 -s 1 "$CAST"   # 参数需按安装版本校准（见 Assumptions Log A-03）

agg "$CAST" docs/demo.gif            # 参数需按安装版本校准
echo "wrote docs/demo.gif"
```

### 示例 8 — 门禁分区（照 `verify-phase-48.sh:13-52` 的形态）

```
verify-phase-49.sh
  P1  编译面          go vet ./... （Windows 宿主可跑；go test 需 WSL）
  P2  embedded 单测   go test ./internal/retriever/embedded/... -count=1
  P3  stub 单测       go test ./internal/server/... -run 'Stub' -count=1
  P4  前端类型        cd web && npm test
  P5  端到端 hermetic 起服务端（替身 embedding 端点 + 无 key）→ 打一轮带引用问答
  P6  tier 隔离不回退  空 tier → 0 命中；不在白名单的 tier → 0 命中
  P7  stub 三重标注   wire 里有 "mode":"stub" / 启动日志有 WARN / README 写了 stub
  P8  素材存在性      docs/demo.gif 是合法 GIF（magic GIF8）；scripts/record-demo.sh 存在
  P9  素材无密钥      对 docs/demo.gif 做二进制扫描，确认不含 key 模式
```
每个分区**独立落日志**、空日志判失败、`fail()` 立即 `exit 1`、收尾横幅以"已运行分区计数"对齐期望分区数（`verify-phase-48.sh:56-58` 的原则：全部可静态判定，缺一个分区就是非零退出，不是更软的口号）。

**照 Phase 48 的三件套落成功 token**（`verify-phase-48.sh:213-279`，本 phase **直接复用**这套机制，不发明新的）：

```bash
# 1) 计数与目录（Phase 48 逐字：EXPECTED_PARTITIONS / RAN / RAN_LIST）
EXPECTED_PARTITIONS=9
RAN=0
RAN_LIST=()
mkdir -p "$WORK/ran"

# 2) 分区体第一条语句就写 begin 标记
begin_partition() { : > "$WORK/ran/$1.begin"; }

# 3) 证据绑定的成功 token：日志必须存在、非空、且含【每一个】断言组 token
ok_partition() {
  local pn="$1" log="$2"; shift 2
  [ -f "$log" ] || fail "${pn}: the probe that certifies ${pn} did not run (R2-WR-03)"
  [ -s "$log" ] || fail "${pn}: ${log} is empty — infrastructure failure, no evidence the probe ran"
  local tok
  for tok in "$@"; do
    grep -qF -- "$tok" "$log" || fail "${pn}: assertion group '${tok}' did not deposit its token"
  done
  : > "$WORK/ran/$pn.ok"
}

# 4) 收口：缺 begin 或缺 ok 一律拒；计数只认真正跑完的分区
done_partition() {
  local pn="$1"
  [ -f "$WORK/ran/$pn.begin" ] || fail "${pn}: its body did not run (IN-06)"
  [ -f "$WORK/ran/$pn.ok" ]    || fail "${pn}: its probe did not run to success (R2-WR-03)"
  RAN=$((RAN + 1)); RAN_LIST+=("$pn"); pass "$pn"
}
```

调用形态（每个断言组一个 token，token 写在**该分区自己的**日志里）：

```bash
begin_partition "P5"
# …P5 的探针把每个断言组的 token 追加到自己的日志…
printf 'P5-HERMETIC-OK\n'   >> "$WORK/p5.log"
printf 'P5-CITATIONS-NONEMPTY\n' >> "$WORK/p5.log"
ok_partition "P5" "$WORK/p5.log" "P5-HERMETIC-OK" "P5-CITATIONS-NONEMPTY"
done_partition "P5 end-to-end hermetic"
```

**门禁怎么证明"还会咬"**：每个分区都要有**变异演练**——把该分区断言的东西改坏（删一个断言 / 改一个阈值 / 换掉被禁 token），门禁必须**红**。演练本身要留证据（Phase 48 的 canary 做法：探针先被证明能产出决定性结果（rc 0/1），再被信任）。**没有变异演练的门禁等于没有门禁。**

**P5 的替身端点必须按键查表**（见 `## Common Pitfalls` Pitfall 17）：对所有输入返回同一条固定向量 ⇒ cosine 恒低于 `retrievalScoreThreshold = 0.3` ⇒ 端到端分区只能验到「拒答」。

**门禁不得依赖** asciinema / agg / Ollama / Docker —— 否则就不是 hermetic 门禁；素材存在性用 magic bytes 断言即可。

## Migration Plan

本 phase **不涉及数据迁移**——它是新增功能，不是对既有状态的改造。但有两处"准迁移"需要显式记录，避免被误当成"零影响"：

| 对象 | 是否需要迁移 | 说明 |
|---|---|---|
| Milvus / MongoDB 后端里的既有 chunk | ❌ 不需要 | `embedded` 是**新增** backend 名（D-06），`milvus` / `mongo` / `none` 三条路径一字不动。用户切回 `RETRIEVER_BACKEND=milvus` 时数据原样在 Milvus 里 |
| 本地向量缓存 | ❌ 不是迁移，是**新建** | 首次运行 embedded 时生成，落在 gitignored 目录（D-14）。删掉该目录 = 触发一次全量重新向量化，无残留状态 |
| `verdict.Verdict` 的 wire 形状 | ⚠️ **向后兼容的加字段** | 新增 `Mode` 字段带 `omitempty`（先例 `Answer` / `ComplianceStatus`，`internal/verdict/verdict.go:78-81`）。真模型路径不写该字段 ⇒ 老前端解析老 payload 行为不变；新前端遇到无 `mode` 的 payload 走默认分支 ⇒ 无破坏性变更。**不存在需要回填的历史数据** |
| `web/src/types/verdict.ts` | ⚠️ 仅加可选字段 | `mode?: ...` 可选 ⇒ 现有 `isVerdict` 守卫（`web/src/types/verdict.ts:138-146`）不需改逻辑，只加渲染分支 |
| `knowledge.Index` 的后端无关化 | ⏭️ **显式推迟** | `internal/retriever/mongo/seed.go:38-48` 记录了"应该重构但至今没做"。本 phase **不动**它（会牵连 milvus 路径）。embedded 照 mongo 先例自写一份解耦 seed，把这次重构留给独立后续项（见 `## Common Pitfalls` Pitfall 4） |
| SQLite 审计库 | ❌ 不需要 | stub 走既有 chat 路径，审计行的写入方式不变（`cmd/server/main.go:1060-1087` 的 Deps 装配不动） |

> 一句话：本 phase 的"迁移面"为零，唯一需要向后兼容处理的是 `verdict.Verdict` 的可选新字段——而它已经有仓内先例。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|---|---|---|
| A-01 | CONTEXT.md D-05 写「authority 16 + sql-guides 202 + draft 1 + historical 1」，但磁盘实测 `knowledge/draft`、`knowledge/historical`、`knowledge/personal` 三个目录**各只有一个 `.gitkeep`、0 个 `.md`**；实际可入库语料 = **218 篇 `.md`**（authority 16 + sql-guides 202），磁盘文件总数 221 | Standard Stack / Architecture Patterns / `## Verification Corrections` C-01 | 低：按磁盘为准即可；**但** README/文档里若照抄 "draft 1 / historical 1" 会与实际不符，且 D-05 **漏列了 `personal` 目录**。建议实施时以 218 为准，并在文档注明「D-05 原文按目录条目计数，含 3 个空目录占位」 |
| A-02 | 本机 **未安装** asciinema 与 agg（本轮 `command -v` 逐条探测：两者均无输出）；ffmpeg 已安装（2021-03-24） | Environment Availability | 中：D-19 的 `docs/demo.gif` 本 phase **产出不了**，需先装工具链。门禁必须不依赖这两个工具 |
| A-03 | asciinema / agg 的具体 CLI 参数（`agg --cols/--rows/--font-size/--theme/--speed`、`asciinema rec -c/-s`）未逐字核实 | Code Examples 示例 7 | 低：脚本按实际安装版本校准即可；但 `docs/demo.gif` 的体积/清晰度预期（估 1–3 MB）依赖这些参数 |
| A-04 | Ollama `/v1/embeddings` 的请求/响应形状未逐字核实；本 phase 以 `internal/embedder/bge/bge.go:143-147`（请求体）与 `:183-201`（两种响应解析）为准 | Architecture Patterns / Code Examples | 低：bge 已实现并被验证过；若 Ollama 实际返回第三种形状，由 bge 侧补解析即可（不在本 phase 范围） |
| A-05 | `hertz/pkg/common/adaptor.HertzHandler` 在 v0.10.6 的 module cache 中**确实存在**（已读文件），但**未编译验证**（Windows 宿主 `go build` 因 duckdb 静态库 TLS-ABI 失败） | Code Examples 示例 5 | 中：若签名不符，静态托管实现需改用 Hertz 原生 handler 写法；实现时第一件事就是在 WSL 里 `go build ./...` 验证 |
| A-06 | `Deps` 构造点已定位在 `cmd/server/main.go:1060-1087`（`Runner` 在 L1065，`PipelineChatModel` 在 L1087），但 stub 注入时**周边变量作用域**未逐行通读 | Architecture Patterns | 低：实施时按行核对即可 |
| A-07 | stub 的抽取规则（句段选择、正则、长度上限）由 executor 定；本报告只约束"确定性 + 逐字来自 chunk" | Code Examples 示例 3 | 低：这是 Claude's Discretion 明文授权项 |
| A-08 | 门禁使用**测试专用**本地替身 embedding 端点，不违反 D-01（D-01 禁止的是生产路径里的兜底） | Code Examples 示例 6 | **中**：这是本报告唯一需要 planner/用户显式确认的边界判断，见 `## Open Questions` Q1 |
| A-09 | `knowledge/draft`、`knowledge/historical`、`knowledge/personal` 三个目录**只有 `.gitkeep`、0 篇 `.md`**，但仍在 `knowledge_bases.yaml` 中注册且 `retrieval_enabled: true`（`knowledge_bases.yaml:30-40` 的 draft 条目注释逐字：「draft 角色的检索闸由 Role 语义强制关闭（角色优先于条目开关）」） | Architecture Patterns | 低：空 tier 在检索时自然 0 命中，不影响；仅影响 D-05 的数字表述与缓存条数断言 |
| A-10 | 本 phase 的门禁目标文件放**仓库根目录** `verify-phase-49.sh`（与 `verify-phase-45..48.sh` 同级；`scripts/` 下另有 11..39 的历史脚本） | Standard Stack / Architecture Patterns | 低：位置约定问题，planner 一眼可定 |
| A-11 | WSL 下各 `go test` 包的实测耗时未跑，"30 秒内可跑"是基于纯内存单测的推断 | Validation Architecture | 低：若某包（尤其 `internal/server` 全量）超过 30 秒，按 Test Map 拆成更细的 `-run` 选择器 |
| A-12 | `docs/demo.gif` 的实际体积与观感未验证（无工具链） | Code Examples 示例 7 / Open Questions Q6 | 低：先产出再按需调参；压不下去就缩短录制内容（不引入 LFS） |
| A-13 | **D-05 的目录清单漏了 `personal`**（原文只列 authority/sql-guides/draft/historical 四个 tier，磁盘上还有 `knowledge/personal/`），且 `draft`/`historical`/`personal` 三个目录实际**只有 `.gitkeep`、0 篇 `.md`** | `## Verification Corrections` C-01 / Q2 / A-01 | 低：缓存条数断言按 218；tier 过滤的负例仍可用 `personal`（空目录 ⇒ 天然 0 命中） |
| A-14 | **`.gitignore` 当前没有任何向量缓存条目**（全文 40 行，条目为 `.env` / `*.exe` / `/spiked` / `spike-*trace.json` / `.DS_Store` / `node_modules/` / `web/dist/` / `skillforge.db*` / `/body_gbk_*` / `/body_quantum.json` / `/reports/` / `/eval/reports/` / `/data/` / `/skillforge-server*`） | `## Verification Corrections` C-04 / `## Wave 0 Gaps` | **中**：D-14「不入仓库」需要**显式新增**一条 gitignore 规则；漏了会让 `git status` 变脏、缓存被误提交 |
| A-15 | **`.env.example` 不列 `embedded` 这个 backend 取值**（`:8-13` 只逐条说明 `milvus` / `mongo`；`:43-45` 写 BGE 端点「Required in both backends. OpenAI-compatible /v1/embeddings endpoint.」） | `## Verification Corrections` C-05 / Open Questions Q5 | 低：D-15 明确「`.env.example` 一字不动」⇒ 新 backend 只能靠 README 说明；这是已知的可接受代价，但**README 必须点名 `RETRIEVER_BACKEND=embedded`** |
| A-16 | WSL2 内的 Go 版本与宿主一致（`go version go1.25.5 linux/amd64`），故门禁的 Go 分区在 WSL 内**无需额外工具链** | `## Verification Corrections` C-03 / Environment Availability | 低：与 `go.mod` 的 `go 1.25.5` 一致 |

## Open Questions

### Q1 — 门禁里的"替身 embedding 端点"是否被 D-01 允许？
**我们知道的**：D-01 的原文是"强制要求使用外部向量化模型，**不**引入进程内 embedder，**不**提交预计算向量文件，**不**用 hash/随机向量冒充。端点不可达 ⇒ 明确失败"；其依据是用户原话"不考虑任何兜底方案"。生产路径（`internal/embedder/bge`）在 D-01 下**必须**保持"端点不可达即失败"。
**不清楚的**：门禁脚本在 CI/无网环境下需要一个可用的向量化端点。起一个**只监听 127.0.0.1、只在 verify 脚本子进程里生效**的 HTTP 端点，从服务端视角它**就是一个外部模型服务**（`BGE_M3_URL` 指向它），生产二进制里不存在任何内置假 embedder 分支——这与 D-01 的字面约束并不冲突；但"用 hash 生成的向量"字面上撞上了"不用 hash/随机向量冒充"。
**建议**：允许，但把边界写死——(a) 替身端点只存在于 `verify-phase-49.sh` 内部，`BGE_M3_URL` 只在该脚本的子进程环境里被覆盖；(b) 生产代码路径**零改动**（`internal/embedder/bge` 不加任何分支）；(c) 替身向量用 **hashing vectorizer**（与文本相关、可复现），不用纯随机，且必须在门禁输出里打印 "using test-only embedding double" 以免误读为生产行为；(d) 在 49-RESEARCH / PLAN 里显式记录这一边界，供 reviewer 复核。

### Q2 — 向量缓存目录放仓库内还是系统缓存目录？
**我们知道的**：D-14 只要求"本地缓存文件（不入仓库）"+"key 含三要素"；目录位置是 Claude's Discretion。
**不清楚的**：仓库内（如 `.skillforge-cache/`）可预测、易清理、但会出现在 `git status`（需 .gitignore 规则）；系统目录（如 `os.UserCacheDir()`）更干净、但多份 clone 互不共享、路径不透明。
**建议**：仓库内 `.skillforge-cache/vectors/`，加 `.gitignore` 条目 + 一条 README 说明（"删掉这个目录会触发一次全量重新向量化"）。理由：demo 场景下"看得见的缓存"比"隐形缓存"更利于排障，且与 `skillforge.db`（仓库内 SQLite）的既有惯例一致。

### Q3 — 静态托管放独立包还是 `cmd/server` 内联？
**我们知道的**：`Dockerfile.frontend:39` 给出了语义基线；`adaptor.HertzHandler` 提供了零依赖接缝。
**不清楚的**：`internal/server` 已有 40+ 文件，handler 单文件 ≤ 200 行是 PROJECT.md 硬约束。
**建议**：新建 `internal/webui` 小包（Handler + 测试），`cmd/server` 只做路由挂载 3 行。理由：可独立单测（SPA fallback 语义需要覆盖：有文件→文件、无扩展名→index.html、有扩展名不存在→404、dist 缺失→503 提示页），也避免 `cmd/server/main.go` 继续膨胀（已 1551 行）。

### Q4 — `mode` 字段的取值集合与判定方式？
**我们知道的**：D-08 写"值形如 `mode:"stub"`"，先例是 `Answer` / `ComplianceStatus` 的 `omitempty` 加法。
**不清楚的**：真模型路径是否也要显式写 `mode:"llm"`？
**建议**：`Mode string` + `omitempty`，**只有 stub 写 `"stub"`，真模型不写**（保持 wire 向后兼容：老前端遇到无 `mode` 的 payload 行为不变）。前端按 `verdict.mode === 'stub'` 渲染徽标。`omitempty` 语义已在 `internal/verdict/verdict.go:78-81` 有先例说明。

### Q5 — README 的改写范围？
**我们知道的**：D-08③ 要求"README 写明 stub 模式"；D-12 的 quickstart 是 `go run ./cmd/server` + `cd web && npm run dev`；但当前 README.md:26-79 的 Quick Start 是 Docker Compose 流程。
**不清楚的**：README 是"增加一节 hermetic 模式"还是"改写主流程为 hermetic"？
**建议**：**增加**一节 "Run without Docker / without an API key"，保留原 Docker 流程（向后兼容 + 保留内网 Milvus 的生产演示路径），并把 D-08③ 的 stub 说明写在这一节里。README 的整体门面调整属 Phase 50（FACE-01），不要抢。

### Q6 — `docs/demo.gif` 是否需要 Git LFS？
**我们知道的**：仓库当前没有 `.gitattributes` 的 LFS 配置；`web/dist/` 已被 .gitignore。
**不清楚的**：60–90 秒终端录屏的 GIF 体积估计 1–3 MB（依赖 A-03 的参数）。
**建议**：先不引入 LFS（新增基础设施超出本 phase 范围）。若最终体积 > 5 MB，改用 asciinema 的 `--idle-time-limit` 压缩停顿 + agg 的 `--speed`/`--theme` 调参压到可接受区间。**若压不下去，正确做法是缩短录制内容而不是引入 LFS。**

### Q7 — D-05 漏列的 `personal` tier 与三个空目录怎么处置？
**我们知道的**：D-05 逐字写「authority 16 + sql-guides 202 + draft 1 + historical 1」，**没有 `personal`**；磁盘上 `knowledge/personal/` 存在（1 个 `.gitkeep`），且 `knowledge_bases.yaml:23-27` 里 `personal` 是**已注册**条目（role `personal`、`retrieval_enabled: true`）。`internal/kb/kb.go:11-30` 的包注释逐字说明 personal 的语义：「personal is user-private material: excluded from retrieval unless a skill explicitly declares it, and never citable」。
**不清楚的**：D-05 的清单是「**要索引哪些 tier**」还是「**目录里有什么**」的描述？前者若照抄，会让 `personal` 永远不入库；后者则是笔误。
**建议**：按**目录全量**走（registry 驱动的 walker 天然会覆盖 5 个 slug，active 且 retrieval-eligible 的才进候选集），**不**在代码里硬编码 D-05 的四元组；缓存条数按**实际入库的 218 篇 `.md`** 断言（C-01）。⇒ **planner 需在 PLAN 里显式记一句**：D-05 的数字是目录条目计数，不是文档数，且 `personal` 是已注册 tier。

### Q8 — `.env.example` 不改（只靠 README）会不会让 hermetic 路径难找？
**我们知道的**：D-15 逐字要求「`.env.example` **不改**」；`.env.example:8-13` 逐字列出 `RETRIEVER_BACKEND` 的可选值说明，但**只有 `milvus` / `mongo`**，没有 `embedded`（C-05）。
**不清楚的**：`RETRIEVER_BACKEND` 是可选还是必填（`:8-13` 的措辞是"可选"，但 compose 里写死 `RETRIEVER_BACKEND: milvus`）。
**建议**：README 的 hermetic 小节**必须**逐字给出 `export RETRIEVER_BACKEND=embedded` 这一行（否则读者照抄 `.env.example` 会落回 milvus ⇒ 又要 Milvus）。同时在 `.env.example` **旁边**（不是里面）放一句 README 指引。**不**改 `.env.example` 本体 —— 那是 D-15 的硬约束。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|---|---|---|---|---|
| Go toolchain | 后端、门禁 P1–P3 | ✅ | `go1.25.5 windows/amd64` | — |
| WSL bash | 跑 `go test ./...` | ✅（`wsl.exe -e bash -lc` 可用） | WSL 内 `go version go1.25.5 linux/amd64`（与宿主同版本） | 宿主 `go test` **不可用**（duckdb 静态库 TLS-ABI 链接失败） |
| `go vet` | 宿主侧编译面检查 | ✅（`go vet ./cmd/server` exit 0） | — | WSL `go vet` |
| Node.js | 前端构建 / vitest | ✅ | `v24.19.0` | — |
| npm / vitest | 前端单测 | ✅（`web/package.json` 已声明） | `vitest ^2.0.0` | — |
| `jq` | JSON 解析 | ❌ | — | `python -c` 或 Go 小脚本 |
| `gsd_run` | GSD 工具 | ❌（不在 PATH） | — | `node "$HOME/.agents/gsd-core/bin/gsd-tools.cjs"` |
| **BGE-M3 端点**（Ollama 或 OpenAI 兼容） | embedded 索引 + 查询 | ❌（`ollama` 未安装） | — | 门禁用**测试专用**替身端点（Q1）；demo 录制需先装 Ollama + `ollama pull bge-m3` |
| **asciinema** | 录屏 | ❌ | — | 需 `pip install asciinema`；门禁不依赖 |
| **agg** | 渲染 GIF | ❌ | — | 需 `cargo install --git https://github.com/asciinema/agg` 或预编译二进制；门禁不依赖 |
| ffmpeg | 备用 GIF 转换 | ✅ | 2021-03-24-git-a77beea6c8 | 优先 agg |
| Docker / docker compose | 旧 quickstart | ❌（本轮 `command -v docker` 无输出） | — | 本 phase 的 demo 路径不需要 Docker（D-12）——这同时意味着**旧的 `docker compose up` quickstart 在本机不可执行**，hermetic 路径是唯一可演示路径 |
| Milvus / etcd / minio | 生产后端 | ❌（无 Docker ⇒ 无法起 compose 服务） | — | embedded backend 取代其 demo 角色 |
| `cargo` | 装 agg | ✅（在 PATH） | — | `cargo install --git https://github.com/asciinema/agg` |
| `git` | 语料哈希/脚本 | ✅ | — | — |
| `python` | 门禁里的 JSON 处理 | ✅ | — | 替代 `jq`（`jq` 不在 PATH） |

> **宿主 `go build ./cmd/server` 失败的性质**：`cmd/server/main.go` 直接 import `internal/data` / `internal/duckdb`，链接 `marcboeker/go-duckdb/v2` 的预编译静态库时出现 `undefined reference to __emutls_v...`（gcc 16.1.0 与预编译 duckdb 库的 TLS ABI 不匹配）。**与 Phase 49 无关，非回归**。宿主侧用 `go vet` 做编译面检查，测试一律走 `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./..."`。

## Sources

### Primary (HIGH confidence)

| # | Source | 用到的内容 |
|---|---|---|
| S-01 | `internal/retriever/retriever.go:45-53, 133-140, 167-217` | `Hit` / `ChunkContent` 结构、`Retriever` 接口 8 方法与 `Search` 的 fail-closed 契约 |
| S-02 | `internal/retriever/mongo/rank.go:19-25, 55-63, 92-132` | 排序纯函数、tie-break 语义、tier filter 契约注释 |
| S-03 | `internal/retriever/mongo/mongo.go:364-441` | `Search` 的 filter 构造、sentinel-0、PRE-filter 位置、`ListMetadata` 不过滤 |
| S-04 | `internal/retriever/milvus/filter.go:24-84` | `BuildMilvusFilter` 的跨后端契约（空 tier ⇒ `"false"`、tier 在前 effective 在后） |
| S-05 | `internal/retriever/mongo/seed.go:38-48, 68, 92-157, 186-200` | milvus 解耦 seed 的既有先例、ID 分配、frontmatter 剥离 |
| S-06 | `internal/knowledge/walk.go:22-49, 78, 140` | KBM-06 单 walker 契约、三态决策表 |
| S-07 | `internal/knowledge/indexer.go:71, 168, 285-339, 586` | `maxDocBytes`、milvus 耦合签名、`validateTiers` 的后端无关硬路径、`splitFrontmatter` |
| S-08 | `internal/embedder/bge/bge.go:17-26, 47-51, 111-136, 143-147, 183-201` | Pitfall 8、1024 维与重试、请求体、两种响应解析 |
| S-09 | `internal/embedder/embedder.go:36-47` | Embedder 接口契约（并发安全、不得静默降级） |
| S-10 | `internal/verdict/verdict.go:78-89`、`internal/verdict/validate.go:47-84`、`internal/verdict/grounding.go:170-196, 204, 293-323` | `Verdict` 的 `omitempty` 加法先例、Validate 规则、grounding 判定与降级 |
| S-11 | `internal/knowledge/ecl/stub.go:60-64, 305-314, 324-357` | "确定性抽取、不调模型" 的模式先例（**模式**先例，非代码复用） |
| S-12 | `internal/server/pipeline.go:64, 200-344, 397-443, 563-656, 770-785, 952-1003` | pipeline 11 步、LLM 双通道接缝、空流=错误、`[ref-N]` prompt 渲染、grounding 闭包 |
| S-13 | `internal/server/server.go:163-176, 287-302` | `Runner` 接口与 `PipelineChatModel` 的 nil-gate 语义 |
| S-14 | `internal/server/chat.go:550-569, 802` | Verdict → `data-policy` 帧的序列化路径（`Mode` 字段自动上 wire） |
| S-15 | `cmd/server/main.go:165-178, 288-305, 490-506, 512-539, 1060-1087, 1226-1306, 1525-1551` | env 必填项与 eager 求值、两个 switch、chat model 装配、Deps 构造、`buildRetriever`、`noopRetriever` |
| S-16 | `internal/vecmath/cosine.go:17-34, 50-69` | cosine 单一真源、长度不等 panic |
| S-17 | `internal/kb/kb.go:259-276`、`internal/kb/registry.go:97-171`、`internal/kb/slug.go:18-43` | `Citable` 单点裁决、registry fail-closed、slug 白名单与保留字 |
| S-18 | `internal/retriever/backend.go:24-36` | `IndexerBackend` 窄接口 |
| S-19 | `web/src/types/verdict.ts:32-97, 138-156` | 前端 Verdict/Citation 类型、`isVerdict` 守卫、wire snake_case 约定 |
| S-20 | `web/vite.config.ts:4-8, 20-33`、`web/package.json:7-53`、`Dockerfile.frontend:4-10, 39` | dev proxy、实际依赖版本、nginx SPA fallback 语义基线 |
| S-21 | `README.md:18, 26-79, 87-184`、`Makefile:129-142`、`verify-phase-48.sh:4-62` | 死链 demo.gif、当前 quickstart、Makefile 目标现状、门禁脚本形态 |
| S-22 | `go.mod`、`.gitignore`、`.env.example`、`knowledge_bases.yaml` | 依赖实况、gitignore 现状、配置样例、5 个 KB 的 role 与 retrieval_enabled。**本轮逐行复读**：`.gitignore` 全文 40 行（无任何向量缓存条目）、`.env.example:8-13`（`RETRIEVER_BACKEND` 只列 milvus/mongo）与 `:43-45`（BGE 端点「Required in both backends. OpenAI-compatible /v1/embeddings endpoint.」）、`knowledge_bases.yaml:15-40`（5 个 slug 及其 role） |
| S-39 | `gsd-tools query package-legitimacy check --ecosystem pypi asciinema` → `verdict: "SUS"`, `reasons: ["unknown-downloads"]`, `exists: true`, `deprecated: false`, `postinstall: null`, `repoUrl: "https://asciinema.org"` | 包合法性审计的一手判定 |
| S-40 | `gsd-tools query package-legitimacy check --ecosystem crates agg` → `verdict: "SUS"`, `repoUrl: "https://github.com/savge13/agg"`, `weeklyDownloads: 179` | crates.io 上的 `agg` **不是** asciinema 的 agg（同名不同物）⇒ 安装必须走 git 源 |
| S-23 | `internal/knowledge/ecl/stub.go`、`internal/retriever/mongo/seed.go` | 两个 stub 先例的**职责边界**（ECL 是表抽取器；D-07 是 verdict 层 stub，**模式**先例非代码复用） |
| S-32 | `internal/llm/extract.go:83-120, 135-154, 162-167, 185-192, 213-265, 283-285` | `ExtractJSON` 的「只切出 JSON 对象、不做校验」语义、`phase2SystemInstructions` 的 4 条决策规则（含「NEVER refuse when a relevant chunk exists」）、`withJSONObjectResponseFormat` 用 `model.WrapImplSpecificOptFn` 的原因（「eino v0.9.14 has no model.WithGenerateConfig」） |
| S-33 | `docker-compose.yml:80-95` | backend 服务的 `environment` 逐字：`RETRIEVER_BACKEND: milvus` / `MILVUS_ADDRESS: milvus:19530` / `BGE_M3_URL: ${BGE_M3_URL:?BGE_M3_URL is required — see .env.example}` / `GLM_API_KEY: ${GLM_API_KEY:?GLM_API_KEY is required — see .env.example}` / `SQLITE_DSN: /data/skillforge.db` / `ports: - "8888:8888"` ⇒ **无 key ⇒ compose 在容器启动前就失败** |
| S-34 | `verify-phase-47.sh:7-40, 65-115, 142-157, 203-237, 274-297` | 门禁形态的**根目录**归属、「HERMETIC: no live LLM / Milvus / BGE required.」、`wsl()` 包装、`pass_lines()` 反空跑、`run_go_test()` 的空日志即失败、P6 唯一可软失败分区的先例 |
| S-35 | `verify-phase-48.sh:7-62, 68-96, 213-279, 295-313` | `EXPECTED_PARTITIONS` / `RAN` / `RAN_LIST` 计数、「NO WAIVERS」原则、空日志规则、`begin_partition`/`ok_partition`/`done_partition` 三件套、**逐断言组绑定**的成功 token、LINT_VERSION 钉版、禁 token 运行时拼装 |
| S-36 | `find knowledge -name '*.md' -type f \| wc -l` → 218；`find knowledge -type f ! -name '*.md' \| wc -l` → 3（`draft`/`historical`/`personal` 各一个 `.gitkeep`）；`find knowledge -type f \| wc -l` → 221 | 语料实况（复核 D-05） |
| S-37 | `command -v` 探测：`docker` / `asciinema` / `agg` / `ollama` / `jq` **均无输出**；`go` / `node` / `npm` / `python` / `cargo` / `golangci-lint` / `ffmpeg` / `git` 在 PATH | 环境实况（复核 Environment Availability） |
| S-38 | `wsl.exe -e bash -lc "go version"` → `go version go1.25.5 linux/amd64` | WSL 内 Go 可用且与宿主同版本 |

### Secondary (MEDIUM confidence)

| # | Source | 用到的内容 |
|---|---|---|
| S-24 | [agg — asciinema documentation](https://docs.asciinema.org/manual/agg/) | agg 的职责（.cast → GIF）与 CLI 形态；[CITED] |
| S-25 | [asciinema/agg (GitHub)](https://github.com/asciinema/agg) | agg 源码仓库与安装形态；[CITED] |
| S-26 | [OpenAI compatibility — Ollama documentation](https://docs.ollama.com/api/openai-compatibility) | Ollama 提供 OpenAI 兼容端点面；`/v1/embeddings` 具体形状未逐字确认（A-04）；[CITED] |
| S-27 | `github.com/cloudwego/hertz@v0.10.6/pkg/common/adaptor/handler.go:35-36`（module cache 实读） | `HertzHandler(h http.Handler) app.HandlerFunc` 的存在（**未编译验证**，A-05） |
| S-28 | `.planning/phases/49-hermetic-demo/49-CONTEXT.md` | 20 条锁定决策 / 6 条 Discretion / 3 条 Deferred（本报告逐字引用的真源） |
| S-29 | `.planning/ROADMAP.md:186-199, 231`、`.planning/REQUIREMENTS.md:21-25, 72-74`、`.planning/PROJECT.md:144-159`、`.planning/config.json` | Phase 49 的目标 / 5 条 SC / 依赖关系；DEMO-01..03 原文；硬约束与三条护城河；nyquist_validation=true |

### Tertiary (LOW confidence)

| # | Source | 用到的内容 |
|---|---|---|
| S-30 | 社区讨论（Reddit） | agg 的 Docker 镜像长期未更新 ⇒ 优先 cargo / 预编译二进制；[ASSUMED]。**本轮降级**：crates.io 上的 `agg` 已被一手验证为**同名不同物**（S-40）⇒ 这条社区结论只作背景，不作依据 |
| S-31 | 训练知识 | `docs/demo.gif` 的体积估计（1–3 MB）与 asciinema/agg 的常用参数组合；[ASSUMED]（A-03） |

## Metadata

**Confidence breakdown:**

| Area | Level | Reason |
|---|---|---|
| Standard stack | **HIGH** | 全部来自本会话实读的 go.mod / package.json / module cache；零新增依赖的结论是"不写新 import"的直接推论。唯一 MEDIUM 点是 hertz adaptor 未编译验证（A-05）。**外部素材工具链**另计（asciinema=SUS / crates.io 的 agg 是同名不同物） |
| Architecture | **HIGH** | 三个需求各自都有实读过的接缝：检索面（`retriever.Retriever` 8 方法 + `buildRetriever` switch）、LLM 面（`Deps.PipelineChatModel` 双通道）、静态托管面（hertz adaptor + nginx 语义基线）。三处隐藏地雷（panic 断言 / eager 求值 / milvus 耦合签名）也已定位到行号。**本轮复核**：接口面、tier PRE-filter 语义、seed 流程、stub 契约全部逐字复读确认 |
| Nine questions | **HIGH** | 九问全部基于本轮**实读**的文件给出 prescriptive 答案：接口面（retriever.go:167-217）、过滤语义（mongo.go:397-441）、端点契约（bge.go:142-208）、stub 契约（verdict.go/validate.go/grounding.go）、启动必填（main.go:288-305, 1236-1266）、静态托管（Dockerfile.frontend:39 + vite.config.ts:26-33）、quickstart 断点（README.md:41-79 + docker-compose.yml:80-95）、素材链（Makefile + Dockerfile.frontend）、门禁形态（verify-phase-47/48.sh） |
| Pitfalls | **MEDIUM-HIGH** | 19 条 pitfall 中 16 条有逐字代码证据；3 条（缓存并发首次启动、替身端点的 D-01 边界、demo.gif 体积）依赖尚未验证的外部条件，已在 Assumptions Log / Open Questions 单列 |
| Validation architecture | **MEDIUM-HIGH** | 测试框架、命令与 Wave 0 缺口均可从仓库现状推导（testify 已是直接依赖、vitest 已在 devDeps、门禁形态有 `verify-phase-47.sh` / `verify-phase-48.sh` 两个先例，且本轮已逐字复读其 token 落盘机制）。MEDIUM 点：WSL 下的实际 `go test` 耗时未实测，30 秒预算需在 Wave 0 首跑后校准（A-11） |
| 外部工具链（asciinema/agg/Ollama embeddings） | **LOW-MEDIUM** | 本机三者皆未安装（本轮 `command -v` 逐条确认），CLI 参数与端点响应形状未逐字核实（A-03/A-04）；合法性判定为 SUS（见 `## Package Legitimacy Audit`）；已全部隔离在 `## Assumptions Log` 与 `## Open Questions`，且**不阻塞**主链实现 |

**Research date:** 2026-10-21（本轮 v2 复核：语料普查、工具链探测、`.gitignore`/`.env.example` 实读、包合法性检查）
**Valid until:** 2026-12-21（依赖：asciinema/agg 的安装与 CLI 确认、Ollama `/v1/embeddings` 形状确认、hertz adaptor 编译验证）

**Researcher:** gsd-phase-researcher
**Phase:** 49 — Hermetic Demo
**Requirements:** DEMO-01, DEMO-02, DEMO-03
