# Phase 49: Hermetic Demo - Context

**Gathered:** 2026-10-21
**Status:** Ready for planning

<domain>
## Phase Boundary

让外部用户 clone 仓库后，**不接内网 Milvus/BGE、不配任何 LLM API key**，即可把 SkillForge 完整跑起来并完成一轮带引用的问答 —— 把"clone→跑通"从碰运气变成保证。

本 phase 交付：
- **DEMO-01** 检索链的本地向量存储模式（embedded backend）
- **DEMO-02** 无 API key 路径下的 stub LLM chat
- **DEMO-03** quickstart 逐步实测通过，产出真实 demo.gif 素材（反哺 Phase 50 的 FACE-01）
- 配套门禁 `bash verify-phase-49.sh`（SC5，exit 0）

**不在本 phase 范围**：Phase 50 的 README 门面（FACE-01 badge 链接、示例 skill 数量更正为 7）、Phase 51 发布工程、任何新的前端交互设计。

</domain>

<decisions>
## Implementation Decisions

### 向量从哪来（Area 1）

- **D-01 — 强制外部向量化模型，不做任何兜底。** 复用现有 `internal/embedder/bge`（HTTP 调 `BGE_M3_URL`，1024 维）。**不**引入进程内 embedder，**不**提交预计算向量文件，**不**用 hash/随机向量冒充。端点不可达 ⇒ 明确失败。
  依据：用户原话「我的意思是不考虑任何兜底方案，强制要求使用外部向量化模型」；与 `internal/embedder/bge/bge.go` 既有纪律一致（包注释 Pitfall 8：hash 兜底会用语义无意义的向量掩盖真实故障）。
- **D-02 — 端点形态两种都支持，以本地 Ollama 为主。** 本机/常见场景：`ollama pull bge-m3` ⇒ `BGE_M3_URL=http://localhost:11434/v1/embeddings`（OpenAI 兼容、无需 key；README 现已是此默认）。任意 OpenAI 兼容端点可通过改 `BGE_M3_URL` 替换，README 注明。
- **D-03 — "embedded" 一词专指「向量存储进程内」，不指「向量化进程内」。** 这条必须写进任何实现文档 —— 否则 planner 极可能理解反，做成进程内 embedder。
- **D-04 — 向量存储 = 进程内 Go slice + cosine 排序。** 照搬 `internal/retriever/mongo/rank.go` 的纯函数 `rank` 与 (score desc, id asc) tie-break，保证可复现。
- **D-05 — 语料 = 仓库现有 `knowledge/` 全量**（authority 16 + sql-guides 202 + draft 1 + historical 1），沿用 `internal/knowledge/indexer.go` 的 tier 校验与文档结构解析。
- **D-06 — 接入方式 = 新增 backend 名 `embedded`**，与 `milvus` / `mongo` / `none` 并列。现有 `none` 的 degraded default-deny 语义（`cmd/server/main.go` 的 `noopRetriever`）一字不动。

### stub 模式怎么如实标注（Area 2）

- **D-07 — stub 产出 = 命中 chunk 的原文抽取。** 像 `internal/knowledge/ecl/stub.go` 的 `StubExtract` 那样做确定性抽取（正则/句段），从 top-k chunk 拼出 answer，citations 指向真实 chunk。输出仍是**合法** `verdict.Verdict` JSON，wire 形状与真模型完全一致，下游（前端、审计、评测）零改动。
- **D-08 — 三处标注 stub 模式。** ①`verdict.Verdict` 增加一个 `omitempty` 字段（值形如 `mode:"stub"`；先例：`Answer` / `ComplianceStatus` 就是这么加的，legacy 生产者可 round-trip）②启动时打醒目 WARN ③README 写明。核对前端 `web/src/components/chat/MessageBubble.tsx` 的 `isVerdict` 类型守卫需一并扩展。
- **D-09 — 触发条件 = 无 key 自动 stub + 三重标注。** 配置了 key 走真模型；没 key 走 stub，且三处都明说。已知风险：生产环境掉 key 会静默降级到 stub —— 靠三重标注抵消，不得静默伪装成真模型（SC3 原文要求）。
- **D-10 — stub 的引用与真模型完全同等，不豁免任何校验。** answer 逐字来自 chunk 原文，citations 走同一条 grounding 校验（引用 span 必须真实出现在 chunk 里，否则降级），confidence 上限 0.5，强制 `review_required=true`。
- **D-11 — `cmd/server/main.go` 的启动必填项必须随之调整：** `BGE_M3_URL` **保留必填**（embedded 模式仍需外部模型，D-01），`GLM_API_KEY` **改为可选**（stub 模式无 key 也要能起）。

### clone 后怎么跑起来（Area 3）

- **D-12 — 纯本地进程，零 Docker。** `go run ./cmd/server`（:8888）+ `cd web && npm run dev`（:5173，Vite 已代理 `/api` → :8888）。
  依据：现有 `docker-compose.yml` 的 backend 段用 `${BGE_M3_URL:?…}` / `${GLM_API_KEY:?…}` 强制插值 —— 没有 key 连 `docker compose up` 都起不来；另有 milvus/etcd/minio 三容器与本 phase 的"无内网"目标冲突。
- **D-13 — 前端由后端同源托管（:8888）。** 新增静态托管 `web/dist` + SPA fallback（生产现由 nginx 容器承担）；后端当前**不**提供前端静态文件，需新增。
- **D-14 — 全量索引 + 本地向量缓存。** 首次启动把 `knowledge/` 全量过 BGE-M3 算出向量，存**本地缓存文件（不入仓库）**；后续启动直接读缓存，避免每次重算。
  **缓存 key 必须包含 model + dimension + 语料 hash**，否则换模型或改语料会静默吃到旧向量。
- **D-15 — 只加 `RETRIEVER_BACKEND=embedded`，其余配置体系一字不动。** 现有 milvus 路径、现有 compose、`.env.example` 均不修改，向后兼容最好。
- **D-16 — embedded 分支不得要求 `MONGO_URI` / `MILVUS_ADDRESS`**（现由 `mongo` / `milvus` 分支各自 `envRequired`）。

### demo.gif（Area 4）

- **D-17 — 录终端 CLI 演示，不是浏览器 UI。** 用 curl/脚本跑一轮带引用的问答，终端里直接可见 citations。最能体现"改一个 `SKILL.md` 就换业务域"的内核卖点，且录屏工具链最轻。
- **D-18 — 生成方式 = asciinema（录）+ agg（渲染 gif）**，用 Makefile 目标封装（如 `make demo-gif`），文档写明前置依赖。素材是真实录屏，不是脚本画出来的假动画。
- **D-19 — 素材直接产出到 `docs/demo.gif`。** `README.md` 已经在引用这个路径而文件不存在（正是 Phase 50 FACE-01 的破图根因）；本 phase 直接把真实素材放到该路径，Phase 50 只需确认不断链。
- **D-20 — 录制脚本进仓库，素材也进仓库。** 脚本（形如 `scripts/record-demo.sh`）+ Makefile 目标入库，文档写明依赖。脚本同时充当 SC4「quickstart 逐步实测通过」的证据。

### Claude's Discretion

以下留给 planner / executor 在上述决策约束内自行决定：

- 本地向量缓存的**目录位置、文件格式、并发首次启动的互斥方式**、缓存失效的具体实现（key 组成必须满足 D-14 的三要素）
- stub 抽取器的**具体抽取规则**（句段选择、正则、长度上限）—— 只需满足 D-07/D-10 的确定性与引用真实性
- 错误消息的具体文案与启动 WARN 的呈现形式
- `verify-phase-49.sh` 的**内部断言组织与分区结构**（只需满足 SC5 exit 0，且必须真能抓到对应的回归）
- 演示脚本的**具体命令序列**（只需体现一轮带引用的问答 + 一次 Skill 切换）
- 后端静态托管的**实现方式**（`http.FileServer` + SPA fallback 的具体写法）与 `web/dist` 缺失时的行为

</decisions>

<specifics>
## Specific Ideas

- 用户原话（向量方案）：「我的意思是不考虑任何兜底方案，强制要求使用外部向量化模型」—— 任何"没有 BGE 也能跑"的方案都在本 phase 的边界之外。
- "embedded" 指的是**向量存储进程内**（D-03），不是"向量化进程内"。
- 演示要能体现 SkillForge 的核心卖点：同一个 agent 内核，改一份 `SKILL.md` 就换业务域（如先问 calculator 再问 compliance），因此演示脚本应包含一次 Skill 切换。
- 素材路径要落在 README 已经引用的 `docs/demo.gif`，而不是另起一个名字 —— 让 Phase 50 的 FACE-01 变成"确认不断链"而不是"再接线"。

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — Phase 49 的 goal / depends on / 5 条 success criteria；Phase 50 的 FACE-01（demo.gif 消费方）
- `.planning/REQUIREMENTS.md` — DEMO-01 / DEMO-02 / DEMO-03 的原文与 traceability
- `.planning/PROJECT.md` — 硬约束：零硬编码、Go handler ≤200 行、不静默吞错、每 phase 配 `verify-phase-NN.sh`、文档表述与构建现实一致
- `.planning/phases/49-hermetic-demo/49-CONTEXT.md` — 本文件

### 前序 phase 的决策（继承，不得推翻）
- `.planning/phases/47-debt-cleanup-reproducible-baseline/47-CONTEXT.md` — D-04 hermetic 基线：clean `git archive HEAD` checkout 必须可复现通过，waiver 为空
- `.planning/phases/48-ci/48-CONTEXT.md` — D-13 / D-16：`cmd/eval --frozen` 不是 hermetic 门（bootstrap 时就要求 BGE/Milvus/GLM，缺 secret 致命退出）；Go job 绝不声明 Milvus/BGE/GLM 入口
- `.planning/phases/46-ci-matrix/46-CONTEXT.md` — SHACL 语义与检索层的历史决策

### 检索 / 向量 / 嵌入层
- `internal/embedder/embedder.go` — `Embedder` 接口契约；「同库所有实现必须同维度」
- `internal/embedder/bge/bge.go` — 现有外部 embedding 客户端（`BGE_M3_URL`）；Pitfall 8「hash 兜底会掩盖真实故障」；D-01 的直接依据
- `internal/retriever/retriever.go` — `Retriever` 接口（`Health` / `Search` / `Ensure` / `Insert` / `ListMetadata` / `GetChunk` / `GetMaxID` / `Disconnect`）；tier 过滤为 pre-similarity，空 tier 必须 0 hits
- `internal/retriever/mongo/rank.go` — 纯函数 `rank` + (score desc, id asc) tie-break；D-04 的照搬对象
- `internal/retriever/mongo/mongo.go` — "存储层过滤 + Go 内打分"形态的既有实现
- `internal/knowledge/indexer.go` — 文档结构解析、tier 校验、索引流程（D-05 的语料与流程来源）
- `internal/kb/` — tier registry（`Registry` / `Citable` / `ActiveSlugs` / `SlugsByRole`）

### Verdict / stub / 审计链
- `internal/verdict/verdict.go` — `Verdict` 结构（D-08 的 `omitempty` 字段加法先例）
- `internal/verdict/validate.go` — `Validate`（status / confidence / citations 约束）
- `internal/verdict/grounding.go` — `GroundingCheck` / `ApplyDowngrade`（D-10 的引用真实性校验）
- `internal/knowledge/ecl/stub.go` — 既有 stub 抽取器先例（D-07 的照搬对象）
- `internal/server/pipeline.go` — 生产默认问答管线（tier 解析 → embed → search → 阈值拒答 → LLM → validate → grounding）
- `internal/server/chat.go` — `data-policy` 帧下发与审计行写入位置（D-08 的 wire 落点）

### 启动装配与前端
- `cmd/server/main.go` — 启动必填项装配（`BGE_M3_URL` / `GLM_API_KEY` / `MONGO_URI` / `MILVUS_ADDRESS`）、backend 分支选择、duckdb 仅在 data salt 非空时初始化
- `internal/store/sqlite` — `modernc.org/sqlite` 纯 Go 存储（无需 CGO）
- `web/vite.config.ts` — dev server 的 `/api` → :8888 代理
- `web/package.json` — 前端 scripts（dev / build / test）
- `Dockerfile.frontend` — 生产前端镜像形态（nginx + SPA fallback + `/api` 反代 + `proxy_buffering off`），D-13 静态托管的参照
- `web/src/transport/index.ts` — 唯一区分 Mock / Real transport 的位置
- `web/src/components/chat/MessageBubble.tsx` — `isVerdict` 类型守卫（D-08 前端落点）

### 语料 / Skill / 门禁 / 门面
- `knowledge/` — 仓库现有语料树（authority / sql-guides / draft / historical）
- `skills/` 与 `examples/skills/` — 8 个被跟踪的 `SKILL.md` 与 7 个示例 skill（演示脚本换 skill 的来源）
- `internal/skill/loader.go` — 只认精确文件名 `SKILL.md`（大小写敏感）
- `verify-phase-47.sh` / `verify-phase-48.sh` — 既有门禁脚本形态（仓库根）
- `Makefile` — 现有目标（`test-unit` / `test-frontend` / `build` / `clean` / `check-cgo`）；D-18 的 Makefile 目标参照
- `README.md` — Quick Start 现状与 `docs/demo.gif` 引用（D-19 的落点）
- `docker-compose.yml` — 5 服务现状与两个强制插值（D-12 的绕行理由）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/embedder/bge` — 现成的外部 embedding 客户端，embedded backend 直接复用，不写第二套 HTTP 客户端
- `internal/retriever/mongo/rank.go` 的 `rank` — 现成的纯 Go cosine 排序 + 确定性 tie-break
- `internal/knowledge/indexer.go` — 现成的文档解析 / tier 校验 / chunk 构造流程
- `internal/knowledge/ecl/stub.go` — 现成的确定性 stub 抽取器
- `internal/verdict` 包 — 完整的 verdict 校验与 grounding 链
- `internal/retriever/retriever.go` 的 `Retriever` 接口 — embedded backend 实现同一接口即可接入 `pipeline.go`，不改编排层

### Established Patterns
- **退让式降级要有明确语义**：`none` backend 是 degraded default-deny（tier 查询一律拒答 `no_authorized_source`）。embedded 模式**不是**降级，它是等价可用模式 —— 引用与真模型完全同等。
- **错误不静默**：项目级纪律"不静默吞错"；`bge.go` 明确拒绝在 BGE-M3 故障时返回假向量。D-01 沿用同一姿态。
- **测试替身 vs 真实后端分离**：`cmd/eval --frozen` 在 bootstrap 阶段就要求真实 BGE/Milvus/GLM，因此 **hermetic 门禁不能复用它的 bootstrap 形态**；SC5 的 hermetic 门禁要用 HTTP 层录制回放。
- **Go job 零外部依赖入口**：CI 的 Go job 不声明任何 Milvus/BGE/GLM 环境变量（Phase 48 D-16）。
- **可复现性靠确定性 tie-break**：现有检索排序与各 fixture 都以确定性顺序为前提，embedded 排序必须保持同样性质。

### Integration Points
- `cmd/server/main.go` 的 backend 分支装配 — 插入 `embedded` 分支；`BGE_M3_URL` 保持必填、`GLM_API_KEY` 改可选；embedded 分支不得引入 `MONGO_URI` / `MILVUS_ADDRESS`
- `internal/retriever` 的 backend 注册点 — `embedded` 实现 `Retriever` 接口后由 `pipeline.go` 消费
- `internal/verdict/verdict.go` — 加 `omitempty` 的 stub 模式字段
- `internal/server/chat.go` 的 `data-policy` 帧与审计行 — 承载 stub 标记与 `VerdictJSON`
- `internal/server/pipeline.go` 的 LLM 环节 — 无 key 时切到 stub 抽取器，其余（tier 解析 / 阈值拒答 / grounding）完全不变
- `cmd/server/main.go` 的静态文件服务 — 新增 `web/dist` 同源托管与 SPA fallback
- `cmd/server/main.go` 的启动装配 — 加向量缓存的读/写/失效逻辑
- `web/src/components/chat/MessageBubble.tsx` — 消费 `Verdict` 的 stub 模式字段
- `Makefile` + `scripts/` — 新增演示录制目标与脚本
- `docs/demo.gif` — 演示素材落点
- `verify-phase-49.sh`（仓库根） — 本 phase 的门禁脚本

</code_context>

<deferred>
## Deferred Ideas

- **浏览器 UI 版 demo** —— 用 playwright 录屏展示真实 Web 聊天界面与引用卡片。本 phase 只做终端 CLI 版；UI 版归 Phase 50（FACE-01 正好要处理 README 的 `docs/demo.gif` 破图），或作为独立后续项。
- **建议回 ROADMAP 修订 Phase 49 的措辞**（本 phase 不擅自改，phase 边界由 ROADMAP 固定）：
  - SC1 的"无预装 BGE"应改为"无需内网 BGE 端点"（本 phase 仍需一个外部 embedding 端点，默认本地 Ollama）。
  - SC2 的"兜底模式"应改为"本地向量存储模式"（D-01 明确不做向量兜底）。
- **向量兜底方案整体不作为 Deferred 保留** —— 用户明确排除。

</deferred>

---

*Phase: 49-hermetic-demo*
*Context gathered: 2026-10-21*
