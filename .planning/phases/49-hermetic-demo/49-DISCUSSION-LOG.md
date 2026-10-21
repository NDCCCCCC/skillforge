# Phase 49: Hermetic Demo - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-21
**Phase:** 49-hermetic-demo
**Areas discussed:** 向量从哪来（Area 1）, stub 模式怎么如实标注（Area 2）, clone 后怎么跑起来（Area 3）, demo.gif 录什么/怎么录（Area 4）

---

## Area 1 — 向量从哪来

| Option | Description | Selected |
|--------|-------------|----------|
| 1a | 复用现有 `BGE_M3_URL` 外部端点，只新增"embedded"存储模式 | ✓（与 1c 合并采纳） |
| 1b | 向量化进程内（本地模型嵌入二进制） | |
| 1c | 两者都支持，quickstart 默认给本地 Ollama | ✓ |
| 1d | 仓库内提交预计算向量文件作为兜底 | |

**User's choice:** "1c，其他的按你的建议来"

**Notes:**
- 自由文本澄清（Area 1 第 1 题后）：**"我的意思是不考虑任何兜底方案，强制要求使用外部向量化模型"** —— 排除 1b 与 1d，也排除"端点不可达时降级为假向量"。
- 1c 具体化为：本地 Ollama 为主（`ollama pull bge-m3` → `BGE_M3_URL=http://localhost:11434/v1/embeddings`，README 现已是此默认、无需 key），同时支持用户自带任意 OpenAI 兼容端点，README 注明可替换。
- 措辞纠正：SC1 的"无预装 BGE"实际应表述为"无需内网 BGE 端点"（已记 Deferred 建议回 ROADMAP 修订）。
- "embedded" 被钉死为**向量存储进程内**，不是"向量化进程内"。

### 向量存哪

| Option | Description | Selected |
|--------|-------------|----------|
| 进程内 slice + cosine 排序 | 照搬 `internal/retriever/mongo/rank.go` 的纯函数 `rank` + (score desc, id asc) tie-break | ✓ |
| 落盘向量库 | 引入新的向量存储依赖 | |
| 内存 map 暴力扫描 | 语料大时不可接受 | |

**User's choice:** 推荐项（进程内 slice + cosine 排序）
**Notes:** 沿用 mongo backend 已有形态，保证可复现。

### 装什么语料

| Option | Description | Selected |
|--------|-------------|----------|
| 现有 `knowledge/` 全量 | authority 16 + sql-guides 202 + draft 1 + historical 1 | ✓ |
| 只权威层 | 语料更小但削弱检索证据面 | |
| 只 authority + sql-guides | 折中 | |

**User's choice:** 推荐项（现有 `knowledge/` 全量）
**Notes:** tier 过滤沿用 `internal/knowledge/indexer.go` 的既有校验。

### 怎么接入

| Option | Description | Selected |
|--------|-------------|----------|
| 新增 `embedded` backend | 与 `milvus`/`mongo`/`none` 并列，实现同一 `Retriever` 接口 | ✓ |
| 扩展 mongo backend | 复用其过滤+打分形态但换存储 | |
| 扩展 milvus | 与"无内网"目标冲突 | |

**User's choice:** 推荐项（新增 `embedded` backend）
**Notes:** `none` 的 degraded default-deny 语义（`noopRetriever`）不动。

---

## Area 2 — stub 模式怎么如实标注

### stub 产出什么

| Option | Description | Selected |
|--------|-------------|----------|
| 命中 chunk 原文抽取 | 像 `ecl/stub.go` 的 `StubExtract` 那样确定性抽取，citations 指向真实 chunk | ✓ |
| 固定模板答案 | 明显假，且无法体现引用 | |
| 回显检索结果摘要 | 不构成回答 | |

**User's choice:** 推荐项
**Notes:** 输出仍是合法 `verdict.Verdict` JSON，wire 形状与真模型完全一致，下游零改动。

### 怎么标注

| Option | Description | Selected |
|--------|-------------|----------|
| 字段 + 日志 + README 三重标注 | `Verdict` 加 `omitempty` 字段（先例 `Answer`/`ComplianceStatus`）+ 启动 WARN + README | ✓ |
| 只响应体标注 | 前端不展示就等于没标 | |
| 只日志标注 | 用户看不到 | |

**User's choice:** 推荐项
**Notes:** 前端 `web/src/components/chat/MessageBubble.tsx` 的 `isVerdict` 类型守卫要一并核对。

### 什么时候走 stub

| Option | Description | Selected |
|--------|-------------|----------|
| 无 key 自动 stub | 有 key 走真模型；掉 key 降级但三处都明说 | ✓ |
| 显式开关 | 需要用户多记一个环境变量 | |
| 永远走 stub | 真模型路径失效 | |

**User's choice:** 推荐项
**Notes:** 已知风险：生产掉 key 会静默降级，靠三重标注抵消。`cmd/server/main.go` 的 `GLM_API_KEY` 必填必须改成可选；`BGE_M3_URL` 保持必填。

### stub 的引用算不算真

| Option | Description | Selected |
|--------|-------------|----------|
| 完全同等，标注模式差异 | 同一套 grounding 校验，confidence 上限 0.5、强制 review_required | ✓ |
| 放宽校验 | 破坏引用可信度 | |
| stub 不走引用校验 | 与 SC2 冲突 | |

**User's choice:** 推荐项
**Notes:** stub 的 answer 逐字来自 chunk 原文，citations 走同一条 grounding 校验。

---

## Area 3 — clone 后怎么跑起来

### 启动形态

| Option | Description | Selected |
|--------|-------------|----------|
| 纯本地进程，零 Docker | `go run ./cmd/server` + `npm run dev` | ✓ |
| compose 新增 hermetic profile | 保留容器化，但要改 compose 强制插值 | |
| 两条都做 | 工作量最大 | |

**User's choice:** 推荐项（纯本地进程，零 Docker）
**Notes:** 现有 `docker-compose.yml` 的 `${BGE_M3_URL:?…}` / `${GLM_API_KEY:?…}` 强制插值意味着没 key 连 compose 都起不来；另有 milvus/etcd/minio 三容器与"无内网"目标冲突。

### 前端怎么给

| Option | Description | Selected |
|--------|-------------|----------|
| 新增后端静态托管（:8888 同源） | 后端当前不 serve 前端（无 go:embed / FileServer），生产靠 nginx | ✓ |
| 复用 vite dev server | 零新增后端代码，但演示要开两个终端、录出来像开发环境 | |
| 本 phase 不做前端 | 与"可交互 Web 应用"定位冲突 | |

**User's choice:** 推荐项
**Notes:** 同源后一个口开浏览器，演示只需一个终端。

### 首次启动的语料向量化

| Option | Description | Selected |
|--------|-------------|----------|
| 全量索引 + 本地向量缓存 | 缓存文件不入仓库；key 含 model + dimension + 语料 hash | ✓ |
| 全量索引，不缓存 | 每次启动重算，clone→跑通体验差 | |
| 只索引权威层 | 快但削弱 SC2 证据面 | |

**User's choice:** 推荐项
**Notes:** 缓存 key 三要素是硬要求 —— 否则换模型会静默吃旧向量。

### hermetic 模式与现有配置体系的关系

| Option | Description | Selected |
|--------|-------------|----------|
| 只加 `RETRIEVER_BACKEND=embedded` | 现有 milvus 路径 / compose / .env.example 一字不动 | ✓ |
| 同时改造 compose 为 profile 分层 | 改到现有部署文件 | |
| 新增独立 compose 文件 | 仓库多一份 compose 维护成本 | |

**User's choice:** 推荐项
**Notes:** embedded 分支不得要求 `MONGO_URI` / `MILVUS_ADDRESS`。

---

## Area 4 — demo.gif

### 录什么形态

| Option | Description | Selected |
|--------|-------------|----------|
| 终端 CLI 演示 | curl/脚本跑一轮带引用的问答，终端可见 citations | ✓ |
| 浏览器 UI 演示 | 真实 Web 界面 + 引用卡片，但需 playwright + 浏览器下载 | |
| 先终端，浏览器版记 Deferred | — | |

**User's choice:** 推荐项（终端 CLI 演示）
**Notes:** 最能体现"改一个 `SKILL.md` 就换业务域"的内核卖点；本机现状：asciinema / agg / magick / vhs 均未安装，ffmpeg 只有 2021 旧版，仓库无 playwright、无 e2e 目录。

### gif 怎么生成

| Option | Description | Selected |
|--------|-------------|----------|
| 装 asciinema + agg | 两个轻量 CLI，Makefile 目标封装，真实录屏 | ✓ |
| playwright 截图 + ffmpeg 合成 | 工具链重、字体渲染差异 | |
| 只交 .cast 文本，gif 不入库 | README 引用的 gif 仍会破 | |

**User's choice:** 推荐项
**Notes:** 素材是真实录屏，不是脚本画出来的假动画。

### 录好的素材放哪

| Option | Description | Selected |
|--------|-------------|----------|
| 直接产出 `docs/demo.gif` | README 已经引用该路径而文件不存在（Phase 50 FACE-01 破图根因） | ✓ |
| 产出到另一个文件名，phase 50 接线 | 两个 phase 同时改 README 同一行 | |
| 产出到子目录 | 同上 | |

**User's choice:** 推荐项
**Notes:** 让 Phase 50 的 FACE-01 变成"确认不断链"而不是"再接线"。

### 录制过程要不要可复现

| Option | Description | Selected |
|--------|-------------|----------|
| 脚本进仓库，素材也进 | `scripts/record-demo.sh` + Makefile 目标 + 文档写依赖 | ✓ |
| 只交素材 | 无法向 reviewer 证明素材真实 | |
| 脚本进仓库 + CI 重录校验 | CI 需装录屏工具，改动面大 | |

**User's choice:** 推荐项
**Notes:** 脚本同时充当 SC4「quickstart 逐步实测通过」的证据。

---

## Claude's Discretion

以下在上述决策约束内留给 planner / executor 决定（用户未逐条指定）：

- 本地向量缓存的目录位置、文件格式、并发首次启动互斥、缓存失效实现（key 组成须满足三要素）
- stub 抽取器的具体抽取规则（句段选择、正则、长度上限）
- 错误消息文案与启动 WARN 的呈现形式
- `verify-phase-49.sh` 的内部断言组织与分区结构
- 演示脚本的具体命令序列（须含一轮带引用问答 + 一次 Skill 切换）
- 后端静态托管的实现方式与 `web/dist` 缺失时的行为

---

## Deferred Ideas

- 浏览器 UI 版 demo（playwright 录屏）—— 留给 Phase 50 / FACE-01
- 建议回 ROADMAP 修订 Phase 49 的 SC1 / SC2 措辞（"无预装 BGE"→"无需内网 BGE 端点"；"兜底模式"→"本地向量存储模式"）
- 向量兜底方案整体被明确排除，不保留

---

*Phase: 49-hermetic-demo*
*Discussion log generated: 2026-10-21*
