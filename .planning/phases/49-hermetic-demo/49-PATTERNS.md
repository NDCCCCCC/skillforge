# Phase 49 — Pattern Map

> 为 `/gsd-plan-phase 49` 准备：本 phase 要创建/修改的每个文件 → 最接近的既有 analog → 可直接照抄的代码形状。
> 证据均来自 `49-RESEARCH.md` 本轮实读（`[VERIFIED: path:lines]` 引用见该文件）。本文件只做**映射与摘录**，不重复研究的完整论证。

---

## 1. 新包 `internal/retriever/embedded/` — 进程内向量存储后端

| 维度 | 值 |
|---|---|
| Role | service（后端检索实现） |
| Data flow | request-response（`Search` 进、`[]Hit` 出），启动期 bulk `Insert` |
| 最接近 analog | `internal/retriever/mongo/`（过滤在存储侧、打分在 Go 侧） |
| 必须实现的接口 | `internal/retriever/retriever.go:167-217` 的 `Retriever`，8 个方法 |

**接口面（逐字，必须全部实现）**

```go
Health(ctx context.Context) error
Search(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]Hit, error)
Ensure(ctx context.Context) error
Insert(ctx context.Context, docs []Document) error
ListMetadata(ctx context.Context) ([]DocumentMeta, error)
GetChunk(ctx context.Context, id int64) (ChunkContent, error)
GetMaxID(ctx context.Context, tier string) (int64, error)
Disconnect() error
```

> `GetChunk` 不存在时返回 `ErrChunkNotFound`（逐字 `errors.New("retriever: chunk not found by id")`）；`ListMetadata` **不过滤 tier**、**不**对 `effective_to` 做 PRE 过滤（图谱要暴露被取代的 chunk）。

**要照抄的两段形状**

1. `internal/retriever/mongo/mongo.go` 的 `Search` —— **先构造候选集，再打分**：

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

2. `internal/retriever/mongo/rank.go:94-132` 的纯函数 `rank` —— **排序比较器逐字**：

```go
sort.SliceStable(out, func(i, j int) bool {
    if out[i].score != out[j].score {
        return out[i].score > out[j].score
    }
    return out[i].row.ID < out[j].row.ID
})
```

**tier 隔离契约（D-TIER-03，不可削弱）**：空 tiers ⇒ **0 命中**（不是"全返回"）。对齐两条既有实现：`internal/retriever/milvus/filter.go:64-84`（空 tier ⇒ 字面量 `"false"`）、`internal/retriever/mongo/mongo.go:416-425`（`{tier: {$in: []}}` ⇒ 无行）。
**禁止**「全量打分后再 filter」——那会把信任边界交给打分逻辑（研究 Pitfall 11）。

**相似度**：**只能用** `internal/vecmath/cosine.go:50-69` 的 `Cosine`。包注释明说第二份实现会 silently drift。注意：`Cosine` 对长度不等的向量会 panic ⇒ 启动即校验维度（研究 Pitfall 7）。

**seed/装载**：**不能**复用 `internal/knowledge/indexer.go:168` 的 `Index`（签名耦合 `milvus.Config` + `client.Client`）。照 `internal/retriever/mongo/seed.go:38-48` 的先例**自写一份解耦 seed**；该注释也说明「后端无关化」是至今未做的 refactor —— 本 phase 显式推迟，不顺手改。

**测试 idiom**：包内 `_test.go`、`testify` v1.11.1、表驱动。新增 4 个测试文件（`embedded_test.go` / `tier_isolation_test.go` / `rank_test.go` / `cache_test.go`），见 `49-VALIDATION.md` 的 Wave 0 清单。

---

## 2. 新包 `internal/server/stubchat/` — 无 key 的 stub ChatModel

| 维度 | 值 |
|---|---|
| Role | service（LLM 替身） |
| Data flow | streaming（`Stream` 必须吐 token，**不得空流**） |
| 最接近 analog（形态） | `internal/knowledge/ecl/stub.go` 的 `StubExtract`（确定性规则与 LLM 抽取彻底分离） |
| 最接近 analog（接口） | 任何 `model.BaseChatModel` 实现；装配点看 `internal/server/server.go` 的 `Deps` |

**插入点（研究已定，不要另设计）**：`internal/server/pipeline.go:414` 已有 `if s.PipelineChatModel != nil` 分支；`internal/server/server.go:287-302` 说明 nil 时回退 Runner。stub 实现 `model.BaseChatModel` 并注入 `Deps.PipelineChatModel` ⇒ pipeline 的 parse → `verdict.Validate` → `GroundingCheck` → 降级链**全部原样跑过**，pipeline 代码零分支。

**输出契约（机器可检查）**：
1. 合法 `verdict.Verdict`：`Status ∈ {"ok","needs_input","refused","error"}`；`Confidence ∈ [0,1]`；`Citations` **非 nil**（nil ⇒ 错误逐字 `citations field missing (must be array, can be empty)`）。
2. `Span` **必须逐字来自 chunk**（`GroundingCheck` 用 `strings.Contains(Normalize(chunkText), Normalize(c.Span))`）。
3. 走同一条降级链（`GroundingChecker` = `GroundingCheck` + `ApplyDowngrade`）。
4. D-10 额外：`Confidence ≤ 0.5` 且 `ReviewRequired = true`。
5. `data-policy` 帧就是 `json.Marshal(v)` ⇒ 新增 `Mode` 字段自动上 wire。

**证据来源**：pipeline 已把 `[ref-N] tier=… source=…` 块写进 user prompt（`internal/server/pipeline.go:770-785`），stub 解析 user message 即可确定性抽取 answer 与 citations。

---

## 3. 新包 `internal/webui/` — 同源静态托管

| 维度 | 值 |
|---|---|
| Role | middleware（HTTP 静态伺服 + SPA fallback） |
| Data flow | request-response |
| 语义基线 | `Dockerfile.frontend:39` 的 nginx 配置 |

**要复刻的 nginx 语义（逐字）**

```nginx
location / { try_files $uri /index.html; }
location /api {
    proxy_pass http://backend:8888;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_buffering off;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}
```

> `proxy_buffering off` 是 SSE 的 load-bearing 设置（否则 chunk 被缓冲成一个大 blob）。Go 侧直出没有 buffering 问题，但**不要**给 `/api` 加任何写缓冲。

**实现要点**：`http.FileServer(http.Dir(distDir))` 伺服 `web/dist`；SPA fallback 单独一段（**无扩展名且文件不存在** ⇒ 回 `index.html`；**有扩展名但不存在** ⇒ 404）；**catch-all 必须最后注册**（否则遮蔽 `/api/*`）；`web/dist` 缺失 ⇒ **503 + 明确提示**（不静默 404、不列目录）。

**零新依赖接缝**：`github.com/cloudwego/hertz v0.10.6` 的 `pkg/common/adaptor/handler.go:36` 提供 `HertzHandler(h http.Handler) app.HandlerFunc`。**未编译验证**（宿主 `go build` 因 duckdb TLS-ABI 失败）⇒ 实现第一件事是在 WSL 里 `go build ./...`。

**包归属理由**：`cmd/server/main.go` 已 1551 行，handler 单文件 ≤ 200 行是 PROJECT.md 硬约束 ⇒ 新建 `internal/webui` 小包，`cmd/server` 只做路由挂载（约 3 行）。

---

## 4. 修改 `cmd/server/main.go` — 装配面（**三处地雷**）

| 维度 | 值 |
|---|---|
| Role | controller（启动装配） |
| 最接近 analog | 文件内既有的 `case "mongo":` / `case "milvus":` 分支 |
| 必须改的行 | `:302`（eager 求值）、`:499-506`（default type assertion）、`:1226-1306`（`buildRetriever` switch） |

**地雷 1 — `:302` eager 求值（D-11 的硬前置）**

```go
glmKey := envOr("LOCAL_API_KEY", envRequired("GLM_API_KEY")) // 今天：缺 GLM_API_KEY 必 Fatalf
```

Go 实参 eager 求值 ⇒ `envRequired` 总是先跑。改法：`glmKey := envOr("LOCAL_API_KEY", os.Getenv("GLM_API_KEY"))`，再用 `glmKey == ""` 决定 stub vs 真模型。`BGE_M3_URL` **保持 `envRequired`**（`:288`）。

**地雷 2 — `:499-506` `indexerBackend` switch 的 `default:`**

```go
default:
    // 今天：r.(*milvus.MilvusClient) 类型断言
```

`embedded` 落进 `default` 是 **runtime panic**（不是编译错）。必须显式加 `case "embedded":`。

**地雷 3 — `buildRetriever` 的 `case "embedded":`（D-16）**

不得出现 `envRequired("MONGO_URI")`（今天在 `case "mongo":`，`:1239`）或 `envRequired("MILVUS_ADDRESS")`（今天在 default，`:1266`）。embedded 只读 `BGE_M3_URL` / `BGE_M3_MODEL` / 缓存目录。
`none` 分支（`:1236-1237`）**一字不动**。

---

## 5. 修改 `internal/verdict/verdict.go` + `web/src/types/verdict.ts` — wire 契约

| 维度 | 值 |
|---|---|
| Role | model（wire 结构） |
| 最接近 analog | `Answer` / `ComplianceStatus` 的 `omitempty` 加法（`internal/verdict/verdict.go:78-81`） |
| 前端 analog | `web/src/types/verdict.ts` 的既有可选字段 + `isVerdict` 守卫 |

**Go 侧**：加 `Mode string \`json:"mode,omitempty"\``，**只有 stub 写 `"stub"`，真模型不写**（保持向后兼容：老前端遇无 `mode` 的 payload 行为不变）。

**前端侧**：类型在 **`web/src/types/verdict.ts`**（`isVerdict` 守卫在 `:138-146`；`MessageBubble.tsx:56` 只是 import）。守卫只判 `status` ⇒ 新字段**不会**破坏它，只加渲染分支。

---

## 6. 新建 `verify-phase-49.sh` — 门禁（**仓库根**，与 45–48 同级）

| 维度 | 值 |
|---|---|
| Role | test harness |
| 最接近 analog | `verify-phase-48.sh`（**仓库根**，不是 `scripts/`；`scripts/` 下是 11–39 的历史脚本） |
| 必须照抄的机制 | 分区计数 + 证据绑定的成功 token + 变异演练 |

**骨架（逐字语义来自 `verify-phase-48.sh`）**

```bash
set -uo pipefail                       # 注意：无 -e ⇒ 每个测量步骤都要显式断言
export MSYS_NO_PATHCONV=1
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
pass()/fail()/info()                   # fail() 打印后 exit 1
EXPECTED_PARTITIONS=N; RAN=0; RAN_LIST=()
begin_partition / ok_partition / done_partition
```

**成功 token 必须证据绑定**：分区体第一条语句写 `$WORK/ran/<P>.begin`；探针把**每个断言组**的 token 写进该分区自己的日志；`ok_partition <P> <log> <token...>` 校验日志存在、非空、且含**每一个** token，才写 `$WORK/ran/<P>.ok`；`done_partition` 缺 `.begin` 或 `.ok` 一律拒。**deposit 绝不早于它认证的断言。**

**WSL 包装**：Windows 宿主跑不了 `go test`（duckdb TLS-ABI）⇒ 照 `verify-phase-47.sh` 的 `wsl()` helper 写一个包装：Windows 上走 WSL，Linux 上直通（同一脚本在 CI runner 也能跑）。

**变异演练（没有演练的门禁等于没有门禁）**：每个分区都要有"把该分区断言的东西改坏 ⇒ 门禁必须红"的 drill，且 drill 必须留下证据（Phase 48 的 canary 做法：先证明探针能产出决定性结果 rc 0/1，再信任它）。
**纪律**：drill 的锚点必须**内容唯一且在第 0 列**（缩进的 marker 会让 cut 变 no-op，drill 假绿）；`grep -c` 的精确计数 ≥2 会被 plan-criteria 拒（用 `-ge`）。

**分区清单**（P1..P9，详见 `49-RESEARCH.md` Code Examples 示例 8）：P1 Go 编译面 / P2 新增包单测 / P3 stub 单测 / P4 静态托管单测 / P5 端到端 hermetic / P6 tier 隔离 / P7 三重标注 / P8 素材存在性 / P9 素材无密钥。

---

## 7. 新建 `scripts/record-demo.sh` + `docs/demo.gif` + `Makefile` 目标

| 维度 | 值 |
|---|---|
| Role | utility（演示素材生成） |
| 最接近 analog | Makefile 既有目标（`test-unit` / `test-frontend` / `build` / `check-cgo`） |
| 素材落点 | `docs/demo.gif`（`README.md:18` 已引用而文件不存在 —— 破图根因） |

**录制路径**：D-18 指定 asciinema + agg。**本机 `asciinema` / `agg` 均未安装，且 `github.com` 不可达（`agg` 官方源拿不到）** ⇒ 脚本必须支持**降级渲染路径**：`asciinema rec` → `.cast` → `termtosvg`(pip) → `.svg` → `cairosvg`(pip) → `.png` → 已装的 `ffmpeg` → `.gif`。有 `agg` 时优先用 `agg`。
**门禁只断言产物**（`GIF8`/`GIF89a` magic、非空、二进制内无 API key 字节）与脚本存在，**绝不断言**某个渲染器已安装。

**安全约束**：录制环境**不导出**任何真实 key（空 key 反而会展示 stub 的启动 WARN，正是 D-08② 要展示的）。

**Makefile**：新增 `demo-gif`（调 `scripts/record-demo.sh`）+ `verify-phase-49`（调**仓库根** `verify-phase-49.sh`）。注意根 Makefile 至今**没有 `test:` 目标**（Phase 48 的回归门因此解析到 `go test ./...`）。

---

## 8. 修改 `README.md` + `.gitignore`

**README**：**增加**一节「Run without Docker / without an API key」，保留原 Docker 流程（门面调整属 Phase 50 / FACE-01，不要抢）。这一节必须逐字给出 `export RETRIEVER_BACKEND=embedded` —— 因为 `.env.example:8-13` 只列了 `milvus`/`mongo` 且 D-15 要求它一字不动。hermetic quickstart = ① clone ② `cp .env.example .env` 并**只**填 `BGE_M3_URL` ③ 起一个 BGE 端点（Ollama 或任意 OpenAI 兼容端点）④ `export RETRIEVER_BACKEND=embedded` ⑤ `go run ./cmd/server`（:8888）⑥ `cd web && npm run dev`（:5173）或让后端同源伺服 `web/dist`。

**.gitignore**：当前 40 行里**没有任何向量缓存条目** ⇒ 必须**显式新增**一条（如 `.skillforge-cache/`），否则首次运行后 `git status` 必脏（D-14「缓存不入仓库」）。这是**必需交付项**，不是可选项。

---

## 9. 语料事实（所有计数断言必须按此）

- `knowledge/` 下**实际可入库的 `.md` = 218**（`authority` 16 + `sql-guides` 202）。
- `knowledge/draft`、`knowledge/historical`、`knowledge/personal` **各只有一个 `.gitkeep`**，无 `.md`；三者仍在 `knowledge_bases.yaml` 注册且 `retrieval_enabled: true`（空 tier 自然 0 命中）。
- 文件总数 221（218 `.md` + 3 `.gitkeep`）。
- **D-05 写的 220 是目录条目数且漏列 `personal`** ⇒ 缓存 `corpusHash` 与条数断言一律按 **218**，并在计划里说明偏差。

---

## 10. 测试 idiom（全 phase 共用）

- 包内 `_test.go`、`testing` + `testify/assert`、表驱动；假 embedder / 假端点用 `httptest.NewServer`。
- 跑测一律 WSL：`wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./<pkg>/ -count=1"`；宿主只跑 `go vet ./...`。
- 前端 `cd web && npx vitest run`（85 文件 / 689 测试为基线）。
