# Phase 7: Open-Source Release - Research

**Researched:** 2026-08-13
**Domain:** 开源发布工程（Packaging / Docker / git-history rewrite / docs / eval-results publication）
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D7-01**: docker-compose 起 Milvus standalone 全套（etcd + minio + milvus）。与生产路径（`RETRIEVER_BACKEND=milvus`）100% 一致。
- **D7-02**: BGE-M3 embedding 不内置进 compose，指向外部服务（`.env.example` 的 `BGE_M3_URL` 指向用户自备端点；README 提供一行 `ollama pull bge-m3` 指引）。
- **D7-03**: SC-1 修订为「compose 起 backend + frontend + Milvus + 可插拔 embedding 端点」（原"含 BGE-M3"与 D7-02 冲突）。
- **D7-04**: 后端/前端镜像纯源码构建（多阶段 Dockerfile）。拒绝 GHCR 预构建。
- **D7-05**: 不部署在线 demo 服务，用录制 GIF 代替。
- **D7-06**: SC-5 / DIST-05 修订为「录制演示 GIF」（原"live demo site"与 D7-05 冲突）。
- **D7-07**: 现有仓库转 public，公开名为 `skillforge`。
- **D7-08**: `.planning/` 目录从公开历史中剔除（`git filter-repo`）。
- **D7-09**: 全历史重写清理内网信息（`git filter-repo` 同时剥离 `.planning/` + 替换内网 IP/域名）。
- **D7-10**: examples/ 三模板（compliance + simple QA + calculator）只需通过 schema 校验；calculator 是工具型，不强制可运行。
- **D7-11**: DIST-09 eval 结果用手工快照 + 日期标注写入 README（`cmd/eval --frozen` 跑一次）。
- **D7-12**: DIST-10 eval 样本集用完整 50 例金标集（`eval/golden/compliance.jsonl`）发布为 `eval/sample.jsonl`。

### Claude's Discretion
- README 结构与章节顺序（须覆盖 SC-2 列出的全部元素：badges/value prop/demo GIF/quick start/architecture diagram/contributing/license）。
- CONTRIBUTING.md 内容深度（含"how to write a Skill / add a tool / add a knowledge tier"三节）。
- GIF 录制内容脚本（30-60s，结合 Phase 4/5 已交付 UI 能力）。
- docker-compose 服务编排细节（healthcheck / 依赖顺序 / 数据卷 / 端口映射）。
- 架构图形态（倾向 Mermaid 内嵌 README）。
- license 文件策略（Apache-2.0 全文 + 包头策略）。
- examples/ 校验器（复用 Phase 2 7-field validator；是否暴露 `cmd/skillforge validate <path>` 子命令）。

### Deferred Ideas (OUT OF SCOPE)
- CI 自动更新 README eval 指标（06-03 eval-gate.yml 合并）——属已推迟的 Phase 6 工作。
- 纯工具型 Skill 运行时支持（让 calculator 模板真正可运行）——属后续版本。
- 在线 live demo 服务——用户明确拒绝。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DIST-01 | README with badges, value prop, demo GIF, quick start, architecture diagram, contributing, license | §Architecture Patterns (README structure) + §Code Examples (Mermaid diagram, badges) |
| DIST-02 | Apache-2.0 license file present from day 1 | §Don't Hand-Roll (LICENSE source) + §Code Examples (SPDX header precedent) |
| DIST-03 | `.env.example` with every required variable; missing values → clear startup error | §Code Examples (clean .env.example) — `envRequired` fail-fast path already exists at `cmd/server/main.go:102-108` |
| DIST-04 | `docker compose up` brings up full stack (revised: backend + frontend + Milvus + pluggable embedding, per D7-03) | §Architecture Patterns (Milvus standalone topology) + §Code Examples (docker-compose.yml) |
| DIST-05 | Live demo site deployed (REVISED → recorded demo GIF, per D7-06) | §Code Examples (VHS .tape + screen-capture workflow) |
| DIST-06 | Pre-bundled sample compliance docs (works without API key for retrieval) | §Architecture Patterns — `knowledge/authority/` 7 docs already bundled; `examples/data/` mirrors them |
| DIST-07 | CONTRIBUTING.md with "write a Skill / add a tool / add a knowledge tier" | §Architecture Patterns (CONTRIBUTING structure) — references `internal/skill` validator + `internal/retriever` tier system |
| DIST-08 | `examples/` with 2-3 Skill templates each passing schema validator | §Code Examples (3 templates) — reuse `internal/skill.Validate` (7-field, incl. temperature) |
| DIST-09 | Published eval results in README (faithfulness %, citation precision %, refusal accuracy %) | §Code Examples (eval snapshot) — **GAP: refusal_accuracy must be derived** from `Report.Cases` (see Open Question Q1) |
| DIST-10 | Eval set sample included in repo (`eval/sample.jsonl`, 50 cases) | §Architecture Patterns — `eval/golden/compliance.jsonl` (50 cases: 20 adversarial + 20 refusal-required + 10 multi-hop) copies verbatim |

**SC revisions to lock in PLAN (per D7-03 / D7-06):**
- ROADMAP SC-1 "含 BGE-M3" → "backend + frontend + Milvus + pluggable embedding endpoint"
- ROADMAP SC-5 / REQUIREMENTS DIST-05 "live demo site deployed" → "recorded demo GIF"
- REQUIREMENTS DIST-04 "(Go backend + Milvus + BGE-M3 + frontend)" → "(Go backend + Milvus + frontend + pluggable embedding endpoint)"
</phase_requirements>

## Summary

Phase 7 是一个**纯打包/发布工程阶段**——不新增产品功能，把已交付的 SkillForge（Go+Eino 后端、React 19 前端、Go-native trust evaluator、Milvus tier-filter 护城河）封装成一个陌生人可以 `git clone && docker compose up` 跑起来的开源发布物。工作集中在五个域：(1) 容器化（docker-compose + 多阶段 Dockerfile），(2) 仓库公开治理（`git filter-repo` 剥离 `.planning/` + 清理内网信息 + 重命名），(3) 文档（README / CONTRIBUTING / LICENSE / 架构图），(4) examples/ 三模板 + eval 结果快照发布，(5) demo GIF 录制。

**Primary recommendation:** 以 Milvus **2.4.x** standalone 官方 compose 拓扑（etcd + minio + milvus 三容器，RocksMQ 内嵌）为基底，叠加 Go 1.25-alpine + node 多阶段构建的后端/前端镜像；用**单次 `git filter-repo` 运行**同时完成 `.planning/` 剥离与内网字符串替换（避免多次历史重写）；`cmd/eval --frozen` 产出一次性快照写入 README（refusal_accuracy 从 `Report.Cases` 派生）。

**三个最关键的发现（planner 必须吸收）：**
1. **版本真相在 go.mod / package.json，不在 CLAUDE.md**——后端 `go 1.25.0`（非 CLAUDE.md 所说 1.22+/1.23），前端 `ai@^7.0.59`（Vercel AI SDK **v7**，非 v5）、`vite@^5.4.0`（非 v8）。Dockerfile 必须以这两个文件为准。
2. **Milvus SDK v2.4.2 决定服务端版本**——`go.mod` pin `milvus-sdk-go/v2 v2.4.2`，故 compose 必须用 **milvusdb/milvus:v2.4.x**（RocksMQ 内嵌，3 容器），**不能用**当前 milvus.io 文档默认的 v3.0.0（已切换到 Woodpecker，API/MQ 不兼容）。
3. **`refusal_accuracy` 不是一个直接算出的指标**——evaluator 计算 faithfulness / context_precision / answer_relevance / context_recall 四项；DIST-09 要求的 "refusal accuracy %" 必须从 `Report.Cases`（按 `category=="refusal-required" && refused==true` 统计）派生，这是 executor 要加的一小段渲染逻辑。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 容器编排（backend+frontend+milvus） | Container Orchestration (docker-compose) | — | Phase 7 的核心交付物是 compose 拓扑；所有应用服务都是其中的 service |
| Go 后端镜像构建 | Build (multi-stage Dockerfile) | Container (distroless/alpine runtime) | 纯源码构建（D7-04）；CGO-free（modernc.org/sqlite 纯 Go），可用 alpine/distroless |
| React 前端镜像构建 | Build (node stage) | CDN/Static (nginx static serve) | Vite build → `web/dist/` → nginx 托管 + `/api` 反代到后端 |
| Milvus standalone 部署 | Database/Storage (compose service) | — | 与生产 `RETRIEVER_BACKEND=milvus` 100% 一致（D7-01） |
| BGE-M3 embedding | External service（用户自备） | — | D7-02 明确不内置；`.env` 声明 `BGE_M3_URL` 为必填（缺值 fail-fast） |
| 仓库历史重写 | VCS (git filter-repo) | — | 发布前一次性元数据操作；重写所有 commit hash |
| eval 结果发布 | Documentation (README snapshot) | Batch (cmd/eval --frozen) | D7-11 手工快照 + 日期；非 CI 自动化 |
| examples/ 校验 | API/Backend (internal/skill.Validate) | — | 复用 Phase 2 的 7-field validator（SYSK-02/SYSK-08） |

## Standard Stack

> 本阶段**不安装 npm/PyPI 运行时库依赖**——所有产物是 Docker 镜像、CLI 工具、文档文件。下表的"version"是 Docker 基础镜像 tag / CLI 工具版本。

### Core (Docker base images + CLI tools)

| Image / Tool | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang` | `1.25-alpine` (pin to a patch, e.g. `1.25.3-alpine`) | 后端构建阶段 | 匹配 `go.mod` 的 `go 1.25.0` [VERIFIED: go.mod + go.dev release history]；alpine 最小体积；无 CGO（modernc.org/sqlite 纯 Go） |
| `node` | `22-alpine` | 前端构建阶段 | `web/package.json` 用 `@types/node: ^22`；node 22 LTS 匹配 Vite 5.4 / AI SDK v7 [VERIFIED: web/package.json] |
| `nginx` | `1.27-alpine`（stable） | 前端静态托管 + `/api` 反代 | 行业标准 SPA 静态服务 + `try_files` 回退 + `proxy_pass` 反代 [CITED: nginx docs] |
| `milvusdb/milvus` | `v2.4.x`（latest patch，e.g. `v2.4.17`） | 向量库 standalone | **必须匹配 SDK**：`go.mod` pin `milvus-sdk-go/v2 v2.4.2` + `milvus-proto v2.4.10`；v2.4.x 用 RocksMQ 内嵌（无需独立 MQ 容器）[VERIFIED: go.mod] |
| `quay.io/coreos/etcd` | `v3.5.x` | Milvus 元数据 | Milvus standalone 官方 compose 的 etcd 镜像 [CITED: milvus.io docs] |
| `minio/minio` | `RELEASE.2024-xx-xx`（官方 compose pin） | Milvus 对象存储 | Milvus standalone 官方 compose 的 minio 镜像 [CITED: milvus.io docs] |
| `git-filter-repo` | latest（pip install） | 仓库历史重写（剥离 `.planning/` + 清理内网信息） | git 官方推荐的 `git filter-branch` / BFG 替代品 [CITED: github.com/newren/git-filter-repo] |
| `vhs` (charmbracelet/vhs) | latest | 终端 demo GIF（`.tape` 脚本可复现） | declarative `.tape` → GIF，可入 git/CI 复现 [CITED: github.com/charmbracelet/vhs] |

### Supporting (one-off, run at release time only)

| Tool | Purpose | When to Use |
|---------|---------|-------------|
| `ollama`（用户侧，非 compose） | `ollama pull bge-m3` 提供 embedding 端点 | README 指引用户自备 BGE-M3（D7-02）；本地最低门槛默认 `http://localhost:11434/v1/embeddings` |
| screen capture（ScreenToGif / OBS / macOS 录屏） | 浏览器 UI 演示 GIF | VHS 只能录终端；UI walkthrough（裁决卡/拒答卡/citation 点击）必须屏幕录制 [VERIFIED: vhs 仅终端输出] |
| `gifsicle` / `ffmpeg` | GIF 压缩优化 | 录屏产出的 GIF 体积优化（README 加载性能） |
| `docker compose config` | compose 语法校验 | Validation Architecture 的 Nyquist gate（无需起容器） |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `milvusdb/milvus:v2.4.x` | v2.5.x 或 v3.0.0 | **不可用**——SDK pin 在 v2.4.2；v2.6+ 切换到 Woodpecker MQ，与 RocksMQ 不兼容 [CITED: milvus.io docs upgrade notes] |
| 多阶段 Dockerfile 源码构建 | GHCR 预构建镜像 | D7-04 明确拒绝（违背"源码即真相"） |
| nginx 托管前端 | 后端 Go 二进制 `embed/` 前端 | embed 更简单但耦合前后端构建；nginx 反代是行业标准 + 解耦部署节奏 |
| `vhs` 录终端 | 手动录屏整个流程 | vhs 可复现、可入 git；但只能录终端，UI 部分必须手动录屏 |
| `git filter-repo` | `bfg-repo-cleaner` 或 `git filter-branch` | filter-repo 是官方推荐（更快、更安全、单文件多操作）；BFG 只删文件不便 replace-text；filter-branch 官方列为不推荐 [CITED: git docs git-filter-repo] |

**Installation (release-time tooling, 在操作机执行，非容器内):**
```bash
# git filter-repo (D7-08/09 历史重写)
pip install git-filter-repo

# vhs (终端 demo GIF，D7-05/06) — macOS/Linux
# 见 https://github.com/charmbracelet/vhs 安装方式（brew/go install）
```

**Version verification (已在本会话执行):**
```bash
# go.mod 真相
grep '^go ' go.mod            # → go 1.25.0
grep milvus go.mod            # → milvus-sdk-go/v2 v2.4.2

# 前端真相
grep '"ai"\|"vite"\|"react"' web/package.json   # → ai ^7.0.59, vite ^5.4.0, react ^19.0.0
```

## Package Legitimacy Audit

> slopcheck 在本环境不可用（`pip install slopcheck` 失败）。本阶段**不新增 npm/PyPI 运行时库**——所有"包"是 Docker 官方/厂商镜像与发布期 CLI 工具。Docker 镜像不在 slopcheck 覆盖域；下表按镜像/工具来源做人工 provenance 审计，全部标记 `[ASSUMED]`（slopcheck 不可用），planner 应在安装任何新 CLI 前加 `checkpoint:human-verify`。

| Package / Image | Registry | Age | Downloads / Pulls | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `golang:1.25-alpine` | Docker Hub (official) | 数年 | 极高（官方镜像） | docker-library/golang | n/a (镜像) | Approved — Docker official image [CITED: hub.docker.com/_/golang] |
| `node:22-alpine` | Docker Hub (official) | 数年 | 极高 | docker-library/node | n/a | Approved — official [CITED: hub.docker.com/_/node] |
| `nginx:1.27-alpine` | Docker Hub (official) | 数年 | 极高 | docker-library/nginx | n/a | Approved — official |
| `milvusdb/milvus:v2.4.x` | Docker Hub | 数年 | 高 | github.com/milvus-io/milvus | n/a | Approved — upstream publisher [CITED: milvus-io/milvus] |
| `quay.io/coreos/etcd:v3.5.x` | quay.io | 数年 | 高 | github.com/etcd-io/etcd | n/a | Approved — upstream publisher |
| `minio/minio` | Docker Hub | 数年 | 极高 | github.com/minio/minio | n/a | Approved — upstream publisher |
| `git-filter-repo` | PyPI | ~6 年 | 高（Google 推荐） | github.com/newren/git-filter-repo | `[ASSUMED]` | Flagged — planner 加 checkpoint:human-verify 后再 `pip install` [CITED: newren/git-filter-repo] |
| `vhs` | brew / GitHub releases | ~4 年 | 中高 | github.com/charmbracelet/vhs | `[ASSUMED]` | Flagged — planner 加 checkpoint:human-verify [CITED: charmbracelet/vhs] |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none（`git-filter-repo` 与 `vhs` 仅因 slopcheck 不可用而 `[ASSUMED]`，两者均为知名上游工具）

*slopcheck 不可用：上表所有项按 graceful-degradation 标记，planner 应把 `git-filter-repo` 与 `vhs` 的安装各 gate 在一个 `checkpoint:human-verify` 任务后。*

## Architecture Patterns

### System Architecture Diagram

发布物的"一键起栈"数据流（从 `git clone` 到浏览器可见的裁决）：

```
git clone skillforge
   │
   ▼
docker compose up ────────────────────────────────────────────┐
   │                                                           │
   ├─► etcd (metadata) ◄─┐                                     │
   ├─► minio (object) ◄──┤  milvus depends_on (healthy)        │
   ├─► milvus:19530 ◄────┘                                     │
   │        ▲                                                   │
   │        │ gRPC + tier-filter expr (PRE-similarity)         │
   │        │                                                   │
   ├─► backend (Go Hertz :8888) ──► 外部 BGE_M3_URL (embed)    │
   │        │                  └─► 外部 GLM_API_KEY (LLM judge)│
   │        │  POST /api/chat (SSE UIMessageChunk stream)      │
   │        ▲                                                   │
   │        │ /api/*  proxy_pass                               │
   └─► frontend (nginx :80 → web/dist/ static)                 │
            ▲                                                   │
            │ http://localhost (浏览器)                         │
   ┌────────┴────────┐                                          │
   │ 陌生人浏览器      │ ◄── citation 点击 / 裁决卡 / 拒答卡      │
   └─────────────────┘                                          │
                                                                ▼
                                                   (一键起栈完成，可见裁决)
```

**关键设计：** milvus `depends_on` etcd+minio 的 healthcheck；backend `depends_on` milvus healthy；frontend nginx `proxy_pass /api → backend:8888`。BGE-M3 与 GLM 是**外部**端点（`.env` 必填，缺值 fail-fast），不在 compose 内。

### Recommended Project Structure (Phase 7 新增/改动)

```
.
├── README.md                 # NEW — hero + value prop + demo GIF + quick start + arch diagram + eval snapshot
├── LICENSE                   # NEW — Apache-2.0 全文
├── CONTRIBUTING.md           # NEW — write a Skill / add a tool / add a knowledge tier
├── docker-compose.yml        # NEW — etcd + minio + milvus + backend + frontend
├── .env.example              # MODIFIED — 内网占位 → compose 友好默认值
├── .dockerignore             # NEW — 排除 .planning/ *.exe node_modules/ web/dist/ .git
├── Dockerfile.backend        # NEW — golang:1.25-alpine 多阶段 → distroless/alpine
├── Dockerfile.frontend       # NEW — node:22 build → nginx serve + proxy
├── examples/                 # NEW
│   ├── skills/
│   │   ├── compliance/SKILL.md      # 复用 skills/compliance（软链或副本）
│   │   ├── simple-qa/SKILL.md       # NEW — RAG 类，7-field 合规
│   │   └── calculator/SKILL.md      # NEW — 工具型（reads/writes 声明），校验通过即可
│   └── data/                         # NEW — knowledge/authority/ 的 7 篇 sample docs 副本
├── eval/
│   ├── sample.jsonl          # NEW (= eval/golden/compliance.jsonl 的 50 例，D7-12)
│   └── (report.json 由 cmd/eval --frozen 生成，D7-11 快照来源)
├── scripts/
│   └── demo.tape             # NEW (Claude's discretion) — vhs 终端 demo 脚本（可选）
└── (现有 cmd/ internal/ web/ skills/ knowledge/ eval/ 不变)
```

### Pattern 1: Milvus standalone 官方拓扑（3 容器 + RocksMQ）

**What:** Milvus standalone 用 3 个 service：`etcd`（元数据）、`minio`（对象存储）、`milvus`（standalone 实例，内嵌 RocksMQ 消息队列，无需独立 MQ 容器）。
**When to use:** v2.4.x / v2.5.x standalone 部署（v2.6+ 默认切 Woodpecker，但 SDK pin 在 v2.4.2，故用 v2.4.x）。

**关键约束 [CITED: milvus.io/docs/install_standalone-docker-compose.md]：**
- 3 容器名约定 `milvus-etcd` / `milvus-minio` / `milvus-standalone`
- 端口：milvus `19530`（gRPC）+ `9091`（metrics/webui）；minio `9000`/`9001`；etcd 不对外（内部 `2379-2380`）
- 卷：`volumes/etcd`、`volumes/minio`、`volumes/milvus`
- 健康检查：`etcdctl endpoint health`、minio `/minio/health/live`、milvus `milvus health`
- 依赖：`milvus` `depends_on: { etcd: {condition: service_healthy}, minio: {condition: service_healthy} }`

**示例见 §Code Examples。**

### Pattern 2: 多阶段 Dockerfile（纯源码构建，D7-04）

**What:** 后端 `golang:1.25-alpine` build → 拷贝静态二进制到 `alpine`/`distroless` runtime；前端 `node:22-alpine` build → 拷贝 `web/dist/` 到 `nginx` runtime。
**关键点：**
- 后端是 CGO-free（modernc.org/sqlite 纯 Go），故可用 alpine/distroless，无需 glibc
- 前端 nginx 需 `try_files $uri /index.html`（SPA 路由回退）+ `location /api { proxy_pass http://backend:8888; }`
- compose 网络内服务名即 DNS（`backend`、`milvus:19530`），`.env` 默认值用服务名而非 `localhost`

### Pattern 3: 单次 git filter-repo 历史重写（D7-08 + D7-09 合并）

**What:** 在一份**新鲜 clone**（或 `--force`）上，**单次** `git filter-repo` 同时完成：(a) `--invert-paths --path .planning/` 剥离规划目录；(b) `--replace-text scrub.txt` 替换内网字符串。
**Why single run:** filter-repo 重写所有 commit hash；多次运行 = 多次 hash 改变 + 多次 force-push。合并为一次最干净。
**示例见 §Code Examples。**

### Anti-Patterns to Avoid

- **用 milvus.io 当前默认的 v3.0.0 compose**：SDK pin 在 v2.4.2，v3.0.0 用 Woodpecker MQ，与 v2.4.x RocksMQ 不兼容。必须 pin `milvusdb/milvus:v2.4.x`。
- **`git filter-repo` 在主仓库直接跑**：filter-repo 要求 fresh clone 或 `--force`，且会改写历史；必须在发布用的 fresh clone 上操作，操作完 force-push 到新 public remote。
- **前端用 `embed/` 嵌进 Go 二进制**：耦合前后端构建节奏，且违背"前后端独立 service"的 compose 友好形态。
- **把 `git-filter-repo` 装进 Dockerfile**：它是发布期一次性工具，不是运行时依赖；仅在操作机装一次。
- **README eval 表写静态数字不带日期**：D7-11 明确要日期 + lock 版本，否则不可信。
- **把 `.env.example` 的内网值直接改成 `localhost` 就发布**：必须先经 `git filter-repo --replace-text` 清理**全历史**（D7-09），仅改 HEAD 文件不够——历史 commit 仍含内网信息。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Milvus standalone compose | 自己拼 etcd/minio/milvus service 定义 | 下载官方 release asset `milvus-standalone-docker-compose.yml`（v2.4.x）作起点，再叠加 backend/frontend service | 官方 asset 已校验 image tag/healthcheck/依赖顺序/卷；自拼易漏 healthcheck 条件导致 milvus 起不来 |
| git 历史敏感信息清理 | 手写 `git rebase` / `git filter-branch` 脚本 | `git filter-repo --invert-paths --path .planning/ --replace-text scrub.txt`（单次） | filter-branch 官方列为不推荐（慢/危险）；filter-repo 是官方替代，单文件支持 path 删除 + 文本替换 |
| LICENSE 文件 | 手写 Apache-2.0 全文 | 直接用 SPDX 官方 LICENSE 全文 | Apache-2.0 是标准化文本，逐字抄写易出错；现有 `cmd/*.go` 已用 `SPDX-License-Identifier: Apache-2.0` 头（先例可循） |
| examples/ Skill 校验 | 写新的 YAML linter | 复用 `internal/skill.Validate`（Phase 2 的 7-field validator） | validator 已实现 SYSK-02/SYSK-08 全部规则（含 temperature 指针陷阱）；新写易漏 |
| 终端 demo GIF | 手动录屏终端 | `vhs demo.tape`（declarative，可入 git） | 可复现、可 diff、CI 友好；手动录屏每次不同 |
| eval 指标渲染 | 写新的报表生成器 | `cmd/eval --frozen=compliance` 生成 `eval/report.json`，再手工/脚本把 `Overall` 抄进 README | evaluator 已是发布级（SC-6 reproducibility gate 通过） |

**Key insight:** 本阶段几乎所有"新东西"都是已有官方/标准方案的**组装**，不是新算法。最大的"手写"风险是 README/CONTRIBUTING 的文案——那是写作，不是技术。

## Runtime State Inventory

> 本阶段**触发** Runtime State Inventory（涉及仓库 rename、历史重写、字符串替换——典型 refactor/release 场景）。逐类回答。

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| **Stored data** (DB/store 里的字符串 key) | SQLite `skillforge.db` 含 audit rows（`session_id="eval-*"`）+ session memory；但这是**本地运行时数据**，`.gitignore` 未忽略 `skillforge.db` —— **确认**：`git ls-files` 显示 skillforge.db **未被 git 追踪**（仅工作区存在，`.gitignore` 忽略 `.env`/`*.exe` 但**未忽略 `*.db`**）[VERIFIED: git ls-files + .gitignore]。D7-09 历史重写只动 git 追踪内容，本地 db 不受影响。**但**：发布前应把 `*.db`/`*.db-shm`/`*.db-wal` 加进 `.gitignore`，并确认它们从未被 commit（`git ls-files` 已确认未追踪） | **`.gitignore` 增补** `skillforge.db*`（code edit，非数据迁移）；历史无 db 残留 |
| **Live service config** (外部服务里、不在 git 的配置) | 内网 Milvus（`milvus.internal.company.com:19530`）、内网 BGE-M3（`bge-m3.internal.company.com`）、内网 Mongo（`10.62.10.34:27017`）——这些都是**用户的内网服务**，其配置不在本仓库 git 中（仅在 `.env` 本地文件 + 注释里出现）。发布物不会动这些服务 | **none** —— 仅需在 git 历史里把这些字符串替换为占位符（D7-09）；外部服务本身不变 |
| **OS-registered state** (Task Scheduler / pm2 / launchd 等) | 未发现本仓库注册任何 OS 级任务（无 pm2 ecosystem、无 launchd plist、无 Windows Task Scheduler 引用）[VERIFIED: grep 无匹配] | **None — verified by grep（无 pm2/launchd/schtasks 引用）** |
| **Secrets / env vars** (引用被改名事物的 secret key / env 名) | `GLM_API_KEY`、`BGE_M3_URL`、`MILVUS_ADDRESS`、`MONGO_URI`、`BGE_M3_AUTH`、`MILVUS_TOKEN` —— 这些 env **名**是产品契约（cmd/server 读取），**不能改名**；只有**值**（内网地址）需清理。`GLM_API_KEY=unhappy` 是测试哨兵值（强制离线），**非真实 key** [VERIFIED: git history grep] | **code edit（.env.example 注释 + .go 注释里的内网值，经 filter-repo --replace-text）**；env 名不变；无真实 key 泄漏 |
| **Build artifacts / installed packages** (旧名残留的产物) | `eval.exe`（48MB Windows 二进制，工作区存在）—— `.gitignore` 已忽略 `*.exe`，`git ls-files` 确认未追踪 [VERIFIED]；`web/dist/`（Vite 构建产物，`.gitignore` 已忽略）；`web/node_modules/`（已忽略） | **none** —— 都在 `.gitignore` 内；发布前确认 `eval.exe` 不在 git（已确认） |

**敏感字符串全历史清单（D7-09 替换目标，仅 git 追踪、非 `.planning/` 文件）[VERIFIED: `git grep` over tracked non-.planning]：**
| 字符串 | 出现位置 | 替换为 |
|--------|----------|--------|
| `10.62.10.34` | `.env.example` 注释、`cmd/server/main.go:50` 注释、`internal/retriever/mongo/mongo.go:53` 注释 | `127.0.0.1`（或 `localhost`） |
| `milvus.internal.company.com` | `.env.example` | `milvus:19530`（compose 服务名） |
| `bge-m3.internal.company.com` | `.env.example` (`BGE_M3_URL`) | `http://localhost:11434/v1/embeddings`（ollama 默认） |
| `bge-m3.internal` | `scripts/live-verify.sh:16` 注释 | `localhost` |

> `.planning/` 内的内网信息随 D7-08 整目录剥离一并消失，无需单独替换。`.claude/`（worktrees + skill findings）**未被 git 追踪**（`git ls-files .claude/` = 0），不在历史内，无需处理 [VERIFIED]。

**The canonical question 的回答：** 在 `git filter-repo` 重写完所有 commit、剥离 `.planning/`、替换内网字符串后，**仅剩的"旧运行时状态"是操作机本地的 `skillforge.db` 与 `eval.exe`**——它们不在 git 内，公开仓库不含；外部内网服务本身不变，只是不再被发布物的字符串引用。故无额外数据迁移任务。

## Common Pitfalls

### Pitfall 1 (P-OSS-4): Docker 不可复现 / 陌生人起不来栈
**What goes wrong:** 陌生人在干净笔记本 `docker compose up` 失败（image tag 漂移、healthcheck 缺失、端口冲突、内存不足）。
**Why it happens:** 用了 `latest` tag（上游 breaking change）；Milvus 起在 etcd/minio 就绪前（无 depends_on healthcheck）；Milvus standalone 吃内存（~4GB）。
**How to avoid:** 所有 image pin 到具体版本（`milvusdb/milvus:v2.4.17` 而非 `latest`）；`depends_on` 用 `condition: service_healthy`；README 标注最低资源（~8GB RAM）；`.env.example` 默认值用 compose 服务名（`milvus:19530`）。
**Warning signs:** `docker compose config` 解析失败；Milvus 容器反复重启（等 etcd）。

### Pitfall 2 (P-OSS-6 + D7-09 风险): 内网信息残留 / 历史清理不彻底
**What goes wrong:** 仓库转 public 后，陌生人 `git log -p` 仍能看到内网 IP/域名（历史 commit 未清理）。
**Why it happens:** 只改了 HEAD 的 `.env.example`，没跑 `git filter-repo`；或 filter-repo 的 `--replace-text` 表漏了某个字符串变体（如 `bge-m3.internal` 短形式 vs `bge-m3.internal.company.com` FQDN）。
**How to avoid:** 发布 gate 必须含**全历史 grep 审计**：`git grep -nE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' $(git rev-list --all)` 在 filter-repo 后返回零匹配；scrub 表覆盖所有变体；先剥离 `.planning/`（消掉大部分内网引用）再 replace-text。
**Warning signs:** 审计 grep 返回任何匹配。

### Pitfall 3 (P-OSS-1): README 质量不足 / 不能让陌生人 fork-run-benchmark
**What goes wrong:** README 信息密度低，陌生人看完不知道怎么起栈、核心护城河是什么。
**Why it happens:** hero 区放了无意义装饰图；quick start 埋在文末；没讲清三个护城河（真源分离 tier-filter / Skill-as-platform / trust evaluator）。
**How to avoid:** hero 区 = value prop 一句话 + demo GIF + quick start（`git clone` → `docker compose up` → `curl`/浏览器）；架构图用 Mermaid（GitHub 原生渲染）；eval 结果表放显眼处（带日期）。
**Warning signs:** README 超过 2 屏才出现 quick start。

### Pitfall 4 (P-OSS-5): CONTRIBUTING 不清晰 / 贡献者不知如何加 Skill/Tool/Tier
**What goes wrong:** 想贡献的人不知道 SKILL.md 7-field 规则、不知道 tier 怎么加。
**Why it happens:** CONTRIBUTING 只写了"fork + PR"流程，没写领域贡献（Skill/Tool/Tier）。
**How to avoid:** CONTRIBUTING 三节各给**最小可工作示例**（write a Skill → 给 simple-qa 模板；add a tool → 指向 `internal/agent` retrieve tool 实现；add a knowledge tier → 指向 `internal/skill/tier.go` TierEnum + `knowledge/` 目录约定）。
**Warning signs:** 三节任一节无代码示例。

### Pitfall 5: 版本不匹配（CLAUDE.md 陈旧 vs go.mod/package.json 真相）
**What goes wrong:** 照 CLAUDE.md 的"Go 1.23 / Vite 8 / AI SDK v5"写 Dockerfile，build 失败。
**Why it happens:** CLAUDE.md 的 Technology Stack 表是项目早期锁定，未随实际 dep 升级同步。
**How to avoid:** Dockerfile 版本一律以 `go.mod`（`go 1.25.0`）+ `web/package.json`（`ai ^7.0.59` / `vite ^5.4.0`）为准；CLAUDE.md 不作为版本真相。
**Warning signs:** Dockerfile 用 `golang:1.23-alpine` build `go 1.25.0` 模块 → 失败。

### Pitfall 6 (P-OSS-2): demo GIF 质量不足 / 体积过大
**What goes wrong:** GIF 模糊、或太大（>10MB）导致 README 加载慢。
**Why it happens:** 录屏未优化；帧率/分辨率过高；UI walkthrough 试图用 VHS（VHS 只能录终端）。
**How to avoid:** 终端部分用 vhs（`.tape` 可调帧率/窗口）；UI walkthrough 用屏录后 `gifsicle --optimize=3 --colors 128` 或 `ffmpeg` 压缩；目标 < 5MB；GIF 放 `docs/` 或仓库内，README 用相对路径引用。
**Warning signs:** GIF > 10MB；终端 + UI 混在一个屏录里。

### Pitfall 7: refusal_accuracy 指标误解
**What goes wrong:** README eval 表写了 "refusal accuracy" 但数字是错的（直接抄了 `Overall` 里不存在的字段）。
**Why it happens:** DIST-09 文案要 "refusal accuracy"，但 evaluator 的 4 指标是 faithfulness/context_precision/answer_relevance/context_recall，**没有**字面叫 refusal_accuracy 的字段。
**How to avoid:** refusal_accuracy = 从 `Report.Cases` 派生：`count(category=="refusal-required" && refused==true) / count(category=="refusal-required")`。见 Open Question Q1。
**Warning signs:** README 表的 "refusal accuracy" 列无法追溯到 report.json。

## Code Examples

### Example 1: docker-compose.yml（Milvus standalone + backend + frontend）

```yaml
# Source: 改编自 milvus.io/docs/install_standalone-docker-compose.md (v2.4.x 拓扑)
#         + D7-01 (Milvus standalone 全套) + D7-02 (BGE-M3 外置) + D7-04 (源码构建)
name: skillforge

services:
  etcd:
    container_name: milvus-etcd
    image: quay.io/coreos/etcd:v3.5.16         # PIN: 用官方 compose 的 etcd 版本
    environment:
      - ETCD_AUTO_COMPACTION_MODE=revision
      - ETCD_AUTO_COMPACTION_RETENTION=1000
      - ETCD_QUOTA_BACKEND_BYTES=4294967296
      - ETCD_SNAPSHOT_COUNT=50000
    volumes:
      - ./volumes/etcd:/etcd
    command: etcd -advertise-client-urls=http://etcd:2379 -listen-client-urls http://0.0.0.0:2379 --data-dir /etcd
    healthcheck:
      test: ["CMD", "etcdctl", "endpoint", "health"]
      interval: 30s
      timeout: 20s
      retries: 3

  minio:
    container_name: milvus-minio
    image: minio/minio:RELEASE.2024-09-13T20-26-02Z   # PIN: 官方 compose 的 minio 版本
    environment:
      MINIO_ACCESS_KEY: minioadmin
      MINIO_SECRET_KEY: minioadmin
    ports:
      - "9001:9001"      # minio console; 9000 仅内部
    volumes:
      - ./volumes/minio:/minio_data
    command: minio server /minio_data --console-address ":9001"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 30s
      timeout: 20s
      retries: 3

  milvus:
    container_name: milvus-standalone
    image: milvusdb/milvus:v2.4.17            # PIN: 必须匹配 milvus-sdk-go/v2 v2.4.2 (go.mod)
    command: ["milvus", "run", "standalone"]
    environment:
      ETCD_ENDPOINTS: etcd:2379
      MINIO_ADDRESS: minio:9000
    # v2.4.x 默认 RocksMQ 内嵌 —— 无需独立 MQ 容器
    ports:
      - "19530:19530"      # gRPC (MILVUS_ADDRESS 指向此)
      - "9091:9091"        # metrics / webui
    volumes:
      - ./volumes/milvus:/var/lib/milvus
    depends_on:
      etcd:
        condition: service_healthy
      minio:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9091/healthz"]
      interval: 30s
      timeout: 20s
      retries: 3

  backend:
    build:
      context: .
      dockerfile: Dockerfile.backend
    environment:
      RETRIEVER_BACKEND: milvus
      MILVUS_ADDRESS: milvus:19530            # compose 服务名（非 localhost）
      BGE_M3_URL: ${BGE_M3_URL:?BGE_M3_URL is required — see .env.example}
      BGE_M3_MODEL: ${BGE_M3_MODEL:-bge-m3}
      GLM_API_KEY: ${GLM_API_KEY:?GLM_API_KEY is required — see .env.example}
      GLM_BASE_URL: ${GLM_BASE_URL:-https://open.bigmodel.cn/api/coding/paas/v4}
    ports:
      - "8888:8888"
    depends_on:
      milvus:
        condition: service_healthy
    restart: unless-stopped

  frontend:
    build:
      context: .
      dockerfile: Dockerfile.frontend
    ports:
      - "8080:80"          # 浏览器访问 http://localhost:8080
    depends_on:
      - backend
    restart: unless-stopped
```

> **注：** `${VAR:?msg}` 语法 = 缺值时 compose 启动即报错退出，直接满足 DIST-03/SC-3"缺值启动报错"。具体 image patch 版本（`v2.4.17`、etcd/minio tag）应从 Milvus v2.4.x 官方 release asset 核对后再 pin，planner 加一个 Wave-0 校验步骤。

### Example 2: Dockerfile.backend（多阶段 Go 构建）

```dockerfile
# Source: Go 多阶段构建标准实践 + go.mod (go 1.25.0) + modernc.org/sqlite (CGO-free)
# build stage
FROM golang:1.25-alpine AS builder
WORKDIR /src
# 先拷依赖以利用 layer cache
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO_ENABLED=0 → 纯静态，可跑 distroless/alpine；modernc.org/sqlite 无 cgo
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/skillforge ./cmd/server

# runtime stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/skillforge /app/skillforge
# skills/ + knowledge/ 是运行时数据（loader 扫描 skills/，indexer 扫 knowledge/）
COPY skills/ /app/skills/
COPY knowledge/ /app/knowledge/
EXPOSE 8888
ENTRYPOINT ["/app/skillforge"]
```

### Example 3: Dockerfile.frontend（Vite build → nginx serve + proxy）

```dockerfile
# Source: Vite SPA + nginx 反代标准实践；build 输出 web/dist/ (vite.config.ts 默认)
# build stage
FROM node:22-alpine AS builder
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build          # → web/dist/ (Vite 默认 outDir)

# runtime stage
FROM nginx:1.27-alpine
COPY --from=builder /web/dist /usr/share/nginx/html
# nginx 配置：SPA 回退 + /api 反代到 backend
RUN printf 'server {\n\
  listen 80;\n\
  root /usr/share/nginx/html;\n\
  location / { try_files $uri /index.html; }\n\
  location /api { proxy_pass http://backend:8888; proxy_http_version 1.1;\n\
                  proxy_set_header Connection ""; proxy_buffering off; }\n\
}\n' > /etc/nginx/conf.d/default.conf
EXPOSE 80
```

> **关键：** `/api` 反代必须 `proxy_buffering off` + `Connection ""`——否则 SSE streaming（UIMessageChunk）会被 nginx 缓冲，破坏流式体验 [CITED: nginx SSE proxying]。

### Example 4: git filter-repo 单次历史重写（D7-08 + D7-09）

```bash
# Source: github.com/newren/git-filter-repo 文档 — path 删除 + replace-text 可在单次运行合并
# 步骤 0：在发布用的 FRESH CLONE 上操作（filter-repo 会重写历史，不可逆）
git clone <原仓库> skillforge-release
cd skillforge-release

# 步骤 1：准备 scrub 表（scrub.txt）—— 每行 old==>new
cat > scrub.txt <<'EOF'
10.62.10.34==>127.0.0.1
milvus.internal.company.com==>milvus
bge-m3.internal.company.com==>localhost
bge-m3.internal==>localhost
EOF

# 步骤 2：单次 filter-repo —— 剥离 .planning/ + 替换内网字符串
git filter-repo --invert-paths --path .planning/ --replace-text scrub.txt

# 步骤 3：全历史审计（gate）—— 必须返回零匹配
git grep -nE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' $(git rev-list --all) \
  && echo "FAIL: 残留内网信息" || echo "OK: 历史干净"

# 步骤 4：确认 .planning/ 已从所有历史移除
git log --oneline --all -- .planning/ | wc -l   # → 0

# 步骤 5：force-push 到新 public remote（hash 全变）
git remote add origin git@github.com:<user>/skillforge.git
git push -u --force origin main
```

> **replace-text 表格式 [CITED: git-filter-repo.txt]：** 每行 `<old>==><new>`；支持 `regex:...==>` 与 `literal:...==>` 前缀；无前缀 = literal。本表用 literal 即可（内网字符串无正则歧义）。

### Example 5: scrub 后的干净 .env.example（D7-02 + D7-03 + DIST-03）

```bash
# SkillForge .env example — 复制为 .env 并填值。缺必填值 → 启动 fail-fast。
RETRIEVER_BACKEND=milvus

# Milvus（compose 服务名；compose 外自建则用 host:19530）
MILVUS_ADDRESS=milvus:19530
MILVUS_TOKEN=

# BGE-M3 embedding（用户自备）。本地最低门槛：
#   ollama pull bge-m3   然后用下面的默认 URL
BGE_M3_URL=http://localhost:11434/v1/embeddings
BGE_M3_MODEL=bge-m3
BGE_M3_AUTH=

# GLM Coding Plan（火山引擎公网，LLM + judge）
GLM_API_KEY=
GLM_BASE_URL=https://open.bigmodel.cn/api/coding/paas/v4

# SQLite 审计/会话后端（默认文件持久化）
SQLITE_DSN=skillforge.db
```

> **DIST-03/SC-3 "缺值启动报错" 已由 `cmd/server/main.go:102-108` 的 `envRequired()` 保证**——`log.Fatalf` 输出 `[FAIL] cmd/server: <VAR> env var not set (copy .env.example to .env and populate)`。compose 层用 `${VAR:?msg}` 再加一道。无需新代码。

### Example 6: README eval 结果快照（D7-11）+ refusal_accuracy 派生

```markdown
## Eval Results

Frozen run on **2026-08-13** · `eval_set_version: compliance-2026-08-13-v1` · `model_ver: glm-5.2` · `prompt_hash: bd59cbe0…`

| Skill | Faithfulness | Context Precision | Refusal Accuracy | Cases |
|-------|-------------|-------------------|------------------|-------|
| compliance | 92% | 85% | 95% | 50 (20 adv / 20 refusal / 10 multi-hop) |

Reproduce: `go run ./cmd/eval --frozen=compliance`
```

**refusal_accuracy 派生逻辑（executor 加在快照生成脚本/手工计算里）：**
```go
// Source: internal/eval/dataset.go — Report.Cases []CaseResult (Refused bool, Category string)
// refusal_accuracy 不是 evaluator 直接算的字段，从 case 级数据派生：
var refusalTotal, refusalRefused int
for _, c := range report.Cases {
    if c.Category == "refusal-required" {
        refusalTotal++
        if c.Refused { refusalRefused++ }
    }
}
refusalAccuracy := float64(refusalRefused) / float64(refusalTotal) // e.g. 19/20 = 0.95
```

### Example 7: examples/ 三个模板（7-field 校验通过）

```yaml
# examples/skills/simple-qa/SKILL.md — RAG 类，可运行
---
description: 通用问答。当用户问事实性问题、需要引用资料回答时触发。不处理合规审查、计算、个人数据。
agent: ChatModelAgent
reads:
  - authority/**
writes: []
steps: |
  1. Retrieve relevant chunks via reads tier filter
  2. Answer with citations (chunk_id + span)
  3. Refuse if no chunks retrieved
guardrails:
  prohibitions:
    - "Never answer without a retrieved citation"
    - "Never fabricate sources"
    - "Never reveal personal information"
  required_refusal_phrases:
    - "no_relevant_source"
  tier_limits:
    authority: ["read-only"]
temperature: 0.3
---
```

```yaml
# examples/skills/calculator/SKILL.md — 工具型，校验通过即可（D7-10：不强制可运行）
---
description: 算术计算。当用户问"2+2"、"本金利息"、"百分比换算"等纯计算问题时触发。不处理需要资料引用的问题。
agent: ReActAgent
reads: []           # 工具型：无知识 tier（空仍需通过 ResolveTiers；见 Open Q）
writes:
  - skill_state     # 工具型：可写自身状态
steps: |
  1. Parse the numeric expression from the user query
  2. Invoke the calculator tool
  3. Return the computed result
guardrails:
  prohibitions:
    - "Never cite retrieved chunks (calculator reads none)"
    - "Never approximate — use exact arithmetic"
    - "Never persist raw user input beyond the turn"
  required_refusal_phrases:
    - "non_arithmetic_query"
  tier_limits: {}
temperature: 0.0
---
```

> **calculator 注意（Open Question Q2）：** 现 `internal/skill.Validate` 要求 `reads` 非空（default-deny, P-SKILL-3）。纯工具型 Skill（reads 空）会**校验失败**。planner 需决定：(a) calculator 声明一个 dummy tier（违背语义），(b) 放宽 validator 允许工具型空 reads（属新能力，超范围），或 (c) calculator 仍声明 `reads: [draft/**]` 之类占位并在 README 注明。**推荐 (c)**：最小改动，保持 validator 不变。

### Example 8: Apache-2.0 包头（已有先例）

```go
// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0
```

> **先例已存在**：`cmd/eval/main.go`、`cmd/server/main.go`、`internal/skill/*.go`、`eval/NOTICE` 已全部采用此包头 + SPDX 标识。Phase 7 只需补 **root LICENSE 文件**（Apache-2.0 全文），包头策略已落地。

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Milvus standalone 默认 RocksMQ | v2.6+ 默认 Woodpecker（MinIO WAL） | v2.6.x | **本项目 pin v2.4.x → 仍是 RocksMQ**（SDK 锁定）；不要用 v3.0.0 compose |
| `git filter-branch` / BFG | `git filter-repo` | 2019+，git 官方推荐 | 单工具支持 path 删除 + 文本替换；filter-branch 官方列为不推荐 |
| 手动终端录屏 | VHS (`.tape` declarative) | 2022+ | 终端 GIF 可复现、可入 git；但仅终端，UI 仍需屏录 |
| AI SDK v5 | AI SDK v7 (`ai@^7.0.59`) | 2025-2026 | CLAUDE.md 的"v5"陈旧；`web/package.json` 真相是 v7 |

**Deprecated/outdated（不要用）：**
- `git filter-branch`：官方文档明确"不推荐，用 filter-repo"
- Milvus v3.0.0 compose（Woodpecker）：与本项目 SDK v2.4.2 不兼容
- CLAUDE.md 的"Go 1.22+/1.23 / Vite v8 / AI SDK v5"：以 go.mod / package.json 为准

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Milvus server v2.4.17 是 v2.4.x 最新 patch（具体 patch 号需核对） | Standard Stack / Code Examples | 低 — 任何 v2.4.x patch 与 SDK v2.4.2 兼容；planner 加 Wave-0 核对 |
| A2 | `ollama pull bge-m3` 暴露 `http://localhost:11434/v1/embeddings`（OpenAI 兼容） | Code Examples (.env) | 低 — ollama 默认 OpenAI 兼容端点；`.env.example` 注释已用此路径（现有 `.env.example:28`） |
| A3 | compose `${VAR:?msg}` 在 docker compose v2 生效（缺值 fail-fast） | Code Examples | 低 — compose v2 标准 shell 风格插值；`docker compose config` 可校验 |
| A4 | etcd/minio/milvus 具体镜像 tag（`v3.5.16`、`RELEASE.2024-09-13...`、`v2.4.17`） | Code Examples | 中 — 这些是我基于官方 compose 模式的合理推断，planner 应从 v2.4.x 官方 release asset 核对确切 tag |
| A5 | GitHub 仓库重命名 `ai-agent` → `skillforge` 后，旧 URL 自动 redirect | Open Questions | 低 — GitHub rename 标准行为；但 module path 已是 `github.com/skillforge/skillforge`（go.mod），无 Go 层影响 |
| A6 | `git-filter-repo` 与 `vhs` 可在操作机安装（pip / brew / go install） | Environment Availability | 中 — 两者均为知名上游工具，但环境未预装；planner 应 gate 在 checkpoint 后 |
| A7 | examples/calculator 用 `reads: [draft/**]` 占位能通过 validator（避免改 validator） | Code Examples Q2 | 中 — 若 validator 对占位 tier 有额外检查可能失败；executor 实测 |

## Open Questions (RESOLVED)

> All four questions are resolved and implemented in the Phase 7 plans (verified by gsd-plan-checker).

1. **DIST-09 的 "refusal accuracy" 指标如何精确产生？** — RESOLVED
   - What we know: evaluator（`internal/eval`）计算 4 个指标：faithfulness / context_precision / answer_relevance / context_recall。`Report.Overall` 与 `Report.PerCategory` 都只有这 4 个 key。
   - What's unclear: DIST-09 文案要 "refusal accuracy"，无字面对应字段。
   - Recommendation: 从 `Report.Cases` 派生（`refusal-required` 类目里 `refused==true` 的比例）。代码见 Example 6。这是一小段渲染逻辑，**不是新工具**。executor 写一个一次性脚本（或手工算）填进 README 快照。
   - **RESOLVED:** Plan 07-05 Task 1 ships `scripts/eval_snapshot` with `deriveRefusalAccuracy` (pure fn, tested via all-refused/partial/zero-refusal-required fixtures) producing the README snapshot.

2. **examples/calculator（纯工具型，reads 为空）如何通过 7-field validator？** — RESOLVED
   - What we know: `internal/skill.Validate` 要求 `reads` 非空（P-SKILL-3 default-deny）。
   - What's unclear: 纯工具型 Skill 没有 tier 可读，空 reads 会被拒。
   - Recommendation: calculator 声明一个语义占位 tier（如 `reads: [draft/**]`）并通过校验，README 注明"演示 writes 声明，纯工具运行时后续版本支持"。**不改 validator**（改 validator = 新能力，超范围）。
   - **RESOLVED:** Plan 07-04 calculator declares `reads: [draft/**]` placeholder tier; passes `internal/skill.Validate` unchanged (locked decision D7-10 honored).

3. **GitHub 用户名 / 仓库最终路径？** — RESOLVED
   - What we know: 公开名锁定 `skillforge`（D7-07）；module path 已是 `github.com/skillforge/skillforge`；当前 git **无 remote**。
   - What's unclear: 用户 GitHub 用户名（`ninedrunk`？CONTEXT 提及待确认）。
   - Recommendation: planner/executor 在 force-push 前 confirm 用户名；README 的 clone URL + badges 用最终路径。
   - **RESOLVED:** Plan 07-06 Task 3 (blocking-human checkpoint) confirms the GitHub username before force-push to the public `skillforge` remote.

4. **是否暴露 `cmd/skillforge validate <path>` 子命令？**（Claude's Discretion） — RESOLVED
   - What we know: CONTEXT 把此列为 discretion；现校验逻辑在 `internal/skill.Validate`（库），无独立 CLI。
   - Recommendation: **建议暴露**——一个轻量 `cmd/skillforge`（或 `cmd/server` 加 `validate` 子命令）让贡献者校验自己的 SKILL.md。成本低（复用 Validate），提升 CONTRIBUTING 体验。planner 决定。
   - **RESOLVED:** Planner chose to validate examples via `go test ./internal/skill -run TestExamplesValidate` (new `examples_validate_test.go`) rather than a new CLI subcommand — lighter surface, CONTRIBUTING references the test.

## Environment Availability

> 本阶段强依赖外部工具（docker / git-filter-repo / vhs）与外部服务（操作机的 Docker daemon）。

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker（compose v2） | DIST-04 全部（compose build/up/config） | ✗（`docker` 不在 PATH） | — | **无 fallback** —— DIST-04 硬依赖；planner 必须把"安装/启动 Docker Desktop"作为前置任务，或标注执行机需具备 |
| `git-filter-repo` | D7-08 / D7-09 历史重写 | ✗（未安装） | — | `pip install git-filter-repo`（发布期一次性）；planner 加安装步骤 + checkpoint |
| `vhs`（charmbracelet） | D7-05/06 终端 demo GIF | ✗（未安装，且仅能录终端） | — | 终端 GIF 可改用 `asciinema` + `agg`；UI walkthrough 只能屏录（ScreenToGif/OBS）—— **必选屏录**因 UI 是核心演示 |
| Go toolchain | `cmd/eval --frozen` 跑快照（D7-11） | ?（未探测；go.mod 要 1.25.0） | — | 用 Dockerfile.backend 的 `golang:1.25-alpine` 容器内跑 eval，规避操作机版本 |
| 在线 Milvus / BGE-M3 / GLM | `cmd/eval --frozen` 产出真实数字 | ?（依赖用户内网/密钥） | — | **无 fallback** —— eval 快照必须跑真实 stack；planner 标注为执行期依赖 |
| GitHub remote | D7-07 转公开 | ✗（`git remote -v` 为空） | — | 发布前 `git remote add origin <github>` |

**Missing dependencies with no fallback:**
- **Docker**：DIST-04 全部交付物硬依赖。执行机必须装 Docker Desktop（Windows）/ Docker Engine。`docker compose config`（Nyquist gate）也需 Docker。
- **在线 stack + GLM key**：D7-11 eval 快照必须跑 `cmd/eval --frozen=compliance`，需 Milvus + BGE-M3 + GLM 全部在线。无 key 则快照无法产出（用户明确拒绝无 key 路径，D7-06）。

**Missing dependencies with fallback:**
- `git-filter-repo`：`pip install` 即可（一次性）。
- 终端 GIF：vhs → asciinema+agg 替代；但 UI walkthrough 无屏录替代。

## Validation Architecture

> `.planning/config.json` 的 `workflow.nyquist_validation: true` —— 包含本节。

### Test Framework
| Property | Value |
|----------|-------|
| Framework | 本阶段**无新增单元测试框架**——交付物是声明式文件（compose / Dockerfile / docs / examples）。校验用"配置校验 + smoke build + schema 校验 + grep 审计"组合 |
| Config file | 无（复用 `internal/skill/validate_test.go` 校验 examples） |
| Quick run command | `docker compose config >/dev/null && echo OK` |
| Full suite command | 见下方 Phase Requirements → Test Map |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DIST-01 | README 含全部必含元素 | smoke（grep 清单） | `grep -cE 'badge\|value prop\|quick start\|docker compose\|architecture\|contributing\|license' README.md` | ❌ Wave 0 |
| DIST-02 | LICENSE 文件存在 + Apache-2.0 | smoke | `grep -c 'Apache License, Version 2.0' LICENSE` | ❌ Wave 0 |
| DIST-03 | `.env.example` 含所有必填变量 + 缺值 fail-fast | smoke | `docker compose config >/dev/null`（`${VAR:?}` 缺值即报错）+ grep 变量清单 | ✅（.env.example 存在，需改） |
| DIST-04 | compose 起全栈（backend+frontend+milvus） | smoke（config 校验）+ manual（实起） | `docker compose config >/dev/null`（语法）；实起需人工 + Docker | ❌ Wave 0 |
| DIST-05 | demo GIF 存在 | smoke | `ls docs/demo*.gif` / `ls *.gif` | ❌ Wave 0 |
| DIST-06 | sample docs 在 examples/data/ | smoke | `ls examples/data/*.md \| wc -l` ≥ 7 | ❌ Wave 0 |
| DIST-07 | CONTRIBUTING 含三节 | smoke | `grep -cE 'write a Skill\|add a tool\|add a knowledge tier' CONTRIBUTING.md` = 3 | ❌ Wave 0 |
| DIST-08 | examples/ 3 模板过 schema 校验 | unit（复用 validator） | `go test ./internal/skill -run TestExamplesValidate`（新建）或 `go run ./cmd/skillforge validate examples/skills/*` | ❌ Wave 0（validator 库 ✅） |
| DIST-09 | README eval 表含日期 + 指标 | smoke | `grep -E 'Faithfulness\|Refusal Accuracy' README.md` + 日期存在 | ❌ Wave 0 |
| DIST-10 | eval/sample.jsonl 含 50 例 | smoke | `wc -l eval/sample.jsonl` = 50 | ❌ Wave 0 |

### Additional Nyquist Gates（发布前硬 gate）

| Gate | Command | Pass Condition |
|------|---------|----------------|
| Dockerfile backend smoke build | `docker build -f Dockerfile.backend -t sf-be .` | exit 0 |
| Dockerfile frontend smoke build | `docker build -f Dockerfile.frontend -t sf-fe .` | exit 0 |
| compose config 校验 | `docker compose config >/dev/null` | exit 0 |
| schema validator 过 3 模板 | `go test ./internal/skill -run TestExamplesValidate` | PASS |
| filter-repo --dry-run 剥离 `.planning/` | `git filter-repo --dry-run --invert-paths --path .planning/` | 报告 .planning/ 被移除 |
| 全历史内网信息 grep 审计（filter-repo 后） | `git grep -nE '10\.62\.10\.34\|internal\.company\.com\|bge-m3\.internal' $(git rev-list --all)` | **零匹配** |
| `.planning/` 全历史移除确认 | `git log --oneline --all -- .planning/ \| wc -l` | = 0 |
| eval 快照产出 | `go run ./cmd/eval --frozen=compliance` 写 `eval/report.json` | report.json 存在 + Overall 有 4 指标 |

### Sampling Rate
- **Per task commit:** 相关 smoke/grep gate（如改 README → README grep；改 compose → `docker compose config`）
- **Per wave merge:** 全部 Nyquist gates 跑一遍
- **Phase gate（发布前，转 public 前）：** 全历史 grep 审计零匹配 + filter-repo --dry-run 确认 + 3 模板校验通过 + eval 快照存在

### Wave 0 Gaps
- [ ] `README.md` — covers DIST-01
- [ ] `LICENSE` — covers DIST-02
- [ ] `.env.example` 改（内网值 → compose 默认）— covers DIST-03
- [ ] `docker-compose.yml` + `Dockerfile.backend` + `Dockerfile.frontend` + `.dockerignore` — covers DIST-04
- [ ] `docs/demo.gif`（或根目录）— covers DIST-05
- [ ] `examples/data/*.md`（7 篇 sample）— covers DIST-06
- [ ] `CONTRIBUTING.md` — covers DIST-07
- [ ] `examples/skills/{compliance,simple-qa,calculator}/SKILL.md` — covers DIST-08
- [ ] `internal/skill/examples_validate_test.go`（新建，复用 Validate 校验 examples/）— covers DIST-08 gate
- [ ] README eval 节 + `eval/report.json`（跑 `cmd/eval --frozen`）— covers DIST-09
- [ ] `eval/sample.jsonl`（复制 golden）— covers DIST-10
- [ ] Docker Desktop / docker CLI 安装确认（执行机前置）— Nyquist gate 依赖

## Security Domain

> `config.json` 未显式禁用 security_enforcement（absent = enabled）。本阶段不开新攻击面，但发布治理含安全维度。

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no（发布物不含新 auth） | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes（间接） | examples/ SKILL.md 过 `internal/skill.Validate`（7-field） |
| V6 Cryptography | no | — |
| V14 Security Configuration | **yes（核心）** | `.env.example` 不含真实 secret；`${VAR:?}` 缺值 fail-fast；`.dockerignore` 排除 `.planning/`/`.git`/`*.exe` |
| V16 Infrastructure (容器) | **yes（核心）** | image pin 具体版本（非 latest）；多阶段构建无 build tool 残留；nginx 仅暴露必要端口 |

### Known Threat Patterns for 开源发布

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 内网信息泄漏（IP/域名）via git 历史 | Information Disclosure | D7-09 `git filter-repo --replace-text` + 全历史 grep 审计 gate（零匹配） |
| secret 泄漏（API key/connection string）via git 历史 | Information Disclosure | 已审计：仅 `GLM_API_KEY=unhappy` 哨兵值（非真实 key）；`.env` 被 `.gitignore`；`.dockerignore` 排除 `.env` [VERIFIED] |
| 供应链风险（恶意 base image） | Tampering | 全部 image 用 official / upstream publisher（golang/node/nginx=official；milvus/minio/etcd=upstream）；pin 具体 tag |
| 资源耗尽（Milvus 吃内存） | Denial of Service | README 标注最低资源要求（~8GB RAM） |

**已审计无泄漏 [VERIFIED]：** git 全历史 secret 扫描（`AKIA…` / `sk-…` / 密码连接串 / `@internal.company`）返回零匹配；唯一 `GLM_API_KEY=…` 字面量是哨兵值 `unhappy`（在 `cmd/spiked/e2e_test.go` + `integration/contract/contract_test.go`，强制离线测试用，非真实 key）。

## Sources

### Primary (HIGH confidence)
- **`go.mod`** — `go 1.25.0`、`milvus-sdk-go/v2 v2.4.2`、`eino v0.7.7`、`hertz v0.10.6`、`modernc.org/sqlite v1.56.0`（本会话 Read）
- **`web/package.json`** — `ai ^7.0.59`（AI SDK v7）、`vite ^5.4.0`、`react ^19.0.0`、`tailwindcss ^4.3.3`（本会话 Read）
- **`internal/skill/validate.go`** — 7-field validator 实现（SYSK-02/SYSK-08，含 temperature 指针）（本会话 Read）
- **`cmd/server/main.go`** — `envRequired()` fail-fast 路径（:102-108）、env 契约（本会话 Read）
- **`cmd/eval/main.go`** — `--frozen` flag 矩阵、`frozenSetPath`/`frozenLockPath` 布局（本会话 Read）
- **`internal/eval/dataset.go`** — `Report`/`CaseResult`/`LockFile` 结构（本会话 Read）
- **`eval/frozen/compliance.lock.json`** — `model_ver: glm-5.2`、`eval_set_version: compliance-2026-08-13-v1`、`prompt_hash: bd59cbe0…`（本会话 Read）
- **`eval/NOTICE`** — Apache-2.0 归因先例（Ragas faithfulness prompt port）（本会话 Read）
- **git 仓库审计**（本会话 Bash）— `.planning/` 追踪（182 files）、`.claude/` 未追踪、无 LICENSE/README/CONTRIBUTING、无 git remote、secret 扫描零真实 key

### Secondary (MEDIUM confidence)
- [milvus.io/docs/install_standalone-docker-compose.md](https://milvus.io/docs/install_standalone-docker-compose.md) — standalone 拓扑（3 容器、端口、卷、healthcheck、RocksMQ vs Woodpecker 版本矩阵）
- [github.com/newren/git-filter-repo](https://github.com/newren/git-filter-repo) + [git-filter-repo.txt](https://github.com/newren/git-filter-repo/blob/master/Documentation/git-filter-repo.txt) — `--invert-paths --path`、`--replace-text` 表格式、单次运行合并多操作
- [go.dev/doc/devel/release](https://go.dev/doc/devel/release) + [hub.docker.com/_/golang](https://hub.docker.com/_/golang) — Go 1.25 发布、`golang:1.25-alpine` 镜像可用
- [github.com/charmbracelet/vhs](https://github.com/charmbracelet/vhs) — declarative `.tape` → GIF（仅终端）

### Tertiary (LOW confidence — marked for validation)
- etcd/minio/milvus 具体 image patch tag（`v3.5.16` / `RELEASE.2024-09-13…` / `v2.4.17`）—— 基于 v2.4.x 官方 compose 模式的合理推断，planner 应从 Milvus v2.4.x release asset 核对确切 tag

## Metadata

**Confidence breakdown:**
- Standard stack（image/tool 版本）: **HIGH** — go.mod / package.json 直接验证；Milvus 版本由 SDK pin 推导（强约束）
- Architecture（compose 拓扑 / Dockerfile / filter-repo）: **HIGH** — 官方文档 + 项目约束交叉验证
- Pitfalls: **HIGH** — P-OSS-1..6 由 ROADMAP 定义 + 本会话环境审计（docker/filter-repo 未装、无 remote）实证
- examples/calculator 校验可行性: **MEDIUM** — validator 要求 reads 非空，纯工具型需占位 tier（Q2，待 executor 实测）
- refusal_accuracy 派生: **HIGH** — Report.Cases 结构明确，派生逻辑无歧义

**Research date:** 2026-08-13
**Valid until:** 2026-09-13（30 天；Milvus/Go/AI SDK 版本稳定区间。若 Milvus SDK 升级则 v2.4.x server pin 需重评）

## RESEARCH COMPLETE

**Phase:** 7 - Open-Source Release
**Confidence:** HIGH

### Key Findings
- **版本真相在 go.mod / package.json，非 CLAUDE.md**：后端 `go 1.25.0`（非 1.23）、前端 `ai@^7.0.59`（AI SDK **v7** 非 v5）/ `vite@^5.4.0`（非 v8）。Dockerfile 必须以此为准。
- **Milvus server 必须 v2.4.x**（非 milvus.io 当前默认 v3.0.0）：`go.mod` pin `milvus-sdk-go/v2 v2.4.2`，v2.4.x 用 RocksMQ 内嵌（3 容器），v3.0.0 切 Woodpecker 不兼容。
- **`refusal_accuracy` 不是直接算出的字段**：evaluator 算 faithfulness/context_precision/answer_relevance/context_recall 四项；DIST-09 的 "refusal accuracy" 须从 `Report.Cases`（`category=="refusal-required" && refused==true`）派生。
- **敏感信息审计已清洁**：git 全历史仅 `GLM_API_KEY=unhappy` 哨兵值（非真实 key）；内网 IP/域名（`10.62.10.34` / `*.internal.company.com` / `bge-m3.internal`）仅在注释 + `.env.example` + `.planning/`（随剥离消失）。
- **环境缺关键依赖**：`docker` 不在 PATH（DIST-04 硬依赖）、`git-filter-repo` 未装、无 git remote、`eval/report.json` 未生成（需在线 stack 跑）—— 全部需 planner 作为前置/执行期依赖处理。

### File Created
`D:\CODE\ClaudeCode\ai-agent\.planning\phases\07-open-source-release\07-RESEARCH.md`

### Confidence Assessment
| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | go.mod / package.json 直接验证；Milvus 版本由 SDK pin 强约束推导 |
| Architecture | HIGH | 官方 compose 拓扑 + 多阶段 Dockerfile 标准 + filter-repo 官方文档 |
| Pitfalls | HIGH | ROADMAP 定义 P-OSS-1..6 + 本会话环境实证（docker/filter-repo/remote 缺失） |
| examples/calculator 校验 | MEDIUM | validator 要求 reads 非空，纯工具型需占位（Q2 待实测） |
| refusal_accuracy 派生 | HIGH | Report.Cases 结构明确，派生逻辑无歧义 |

### Open Questions
1. DIST-09 refusal_accuracy 派生方式（已给推荐：从 Report.Cases 统计）
2. examples/calculator 空 reads 与 validator 冲突（已给推荐：占位 tier，不改 validator）
3. GitHub 用户名 / 最终仓库路径（待用户确认，转 public 前）
4. 是否暴露 `cmd/skillforge validate` 子命令（Claude's Discretion，建议暴露）

### Ready for Planning
Research complete. Planner can now create PLAN.md files. **关键提醒给 planner：**
- 必须在 PLAN 里锁定 ROADMAP SC-1/SC-5 + REQUIREMENTS DIST-04/DIST-05 的表述修订（D7-03/D7-06）
- 把"Docker 安装 / git-filter-repo 安装 / git remote 配置"作为前置任务（环境缺这三项）
- D7-11 eval 快照 + DIST-04 实起栈 需在线 stack（Milvus+BGE-M3+GLM），标注为执行期依赖
- examples/calculator 占位 tier（Q2）与 refusal_accuracy 派生（Q1）纳入对应 task
