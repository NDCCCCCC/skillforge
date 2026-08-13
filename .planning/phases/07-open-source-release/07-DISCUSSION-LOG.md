# Phase 7: Open-Source Release - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-13
**Phase:** 7-open-source-release
**Areas discussed:** Docker 打包与模型服务, Live demo 站点形态, 开源仓库发布形态, examples 与 eval 结果发布

---

## Docker 打包与模型服务

### 向量库形态

| Option | Description | Selected |
|--------|-------------|----------|
| Milvus standalone 全套 | etcd+minio+milvus，~2GB+ 镜像，与生产路径一致，忠实展示 tier-filter 护城河 | ✓ |
| 默认 mongo + Milvus 可选 | compose 默认走 mongo（轻量），Milvus 全套作备选 override | |
| standalone + 构建优化 | 镜像从源码构建，裁剪文档指引最低资源 | |

**User's choice:** Milvus standalone 全套
**Notes:** 与生产路径（milvus backend）100% 一致，最忠实展示 tier-filter 护城河（Phase 1 SC-2 核心机制）。资源开销可接受。

### BGE-M3 服务方式

| Option | Description | Selected |
|--------|-------------|----------|
| Ollama 容器自动拉取 | compose 起 ollama 容器，init 脚本 pull bge-m3，与内网 Ollama 路径一致 | |
| TEI 容器 | HuggingFace Text Embeddings Inference，生产级吞吐，镜像大、CPU 配置复杂 | |
| 要求外部服务 | compose 不内置 embedding，.env 指向外部 BGE_M3_URL | ✓ |

**User's choice:** 要求外部服务
**Notes:** embedding 服务不是核心价值；用户倾向外部化。

### SC-1 表述调和

| Option | Description | Selected |
|--------|-------------|----------|
| 外部默认 + 可选 Ollama override | .env 默认外部，另附 override 文件供一键全含 | |
| 坚持 SC-1 字面 | BGE-M3 必须容器内提供，回到 Ollama 方案 | |
| 修订 SC-1 表述 | compose 起 backend+frontend+Milvus+可插拔 embedding 端点，不强制内置模型服务 | ✓ |

**User's choice:** 修订 SC-1 表述
**Notes:** 合理裁剪——embedding 服务非核心价值。计划阶段需同步更新 ROADMAP SC-1 与 PROJECT.md。

### 镜像进入 compose 方式

| Option | Description | Selected |
|--------|-------------|----------|
| 纯源码构建 | 多阶段 Dockerfile，容器内 Go build + Vite build，无需 registry 账号 | ✓ |
| 预构建 GHCR 镜像 | Actions 构建推 GHCR，compose 拉预构建镜像，引入 registry 流程 | |
| 双路径并行 | GHCR 预构建 + dev override 源码构建，维护两套 | |

**User's choice:** 纯源码构建
**Notes:** 代码即真相，极简发布理念。

---

## Live demo 站点形态

### demo 形态

| Option | Description | Selected |
|--------|-------------|----------|
| 真实 demo + 硬限流 | 部署真实后端持有 GLM key + IP 限流 + 额度封顶 | |
| 检索真实 + 响应预录制 | demo 模式：检索走 Milvus，LLM 响应预录制，零成本零滥用 | |
| 只放 GIF 不部署 | 不部署在线服务，README 放高质量 GIF | ✓ |

**User's choice:** 只放 GIF 不部署
**Notes:** GLM key 公网暴露的成本与滥用风险不划算；GIF 足以传达核心价值。

### 无 key 用户体验

| Option | Description | Selected |
|--------|-------------|----------|
| 内置 demo 模式 | 发布版内置无 key 体验开关（预录裁决全渲染） | |
| 不设 demo 模式 | demo 只存在于 GIF，用真系统必须配 key | ✓ |
| You decide | Claude 评估 mock transport 复用成本后决定 | |

**User's choice:** 不设 demo 模式
**Notes:** 发布物保持精简；用户拒绝无 key 体验路径。

---

## 开源仓库发布形态

### 仓库载体

| Option | Description | Selected |
|--------|-------------|----------|
| 现有仓库转 public | 保留完整开发历史，GSD 工作流痕迹本身是工程能力展示 | ✓ |
| 新仓库干净发布 | 新建仓库从快照重新初始化，历史干净但丢失过程证据 | |
| 私有主干 + 公开镜像 | 现有仓库私有，定期推送代码（不含 .planning/）到公开镜像 | |

**User's choice:** 现有仓库转 public
**Notes:** GSD 工作流痕迹与个人品牌叙事一致。

### .planning/ 处理

| Option | Description | Selected |
|--------|-------------|----------|
| 保留 .planning/ | ROADMAP/CONTEXT/SUMMARY 是 agent 工程方法论活标本 | |
| 剔除 .planning/ | git filter-repo 从公开历史剥离，只含产品代码 + 官方文档 | ✓ |
| 精选保留高层文档 | 只留 ROADMAP/REQUIREMENTS/PROJECT，剔除执行细节 | |

**User's choice:** 剔除 .planning/
**Notes:** 个人规划细节不属于发布物；公开仓库聚焦产品。

### 内网信息清理力度

| Option | Description | Selected |
|--------|-------------|----------|
| 全历史重写清理 | git filter-repo 重写：剥离 .planning/ + 替换内网 IP/域名为占位符 | ✓ |
| 只清工作树 + 历史审计 | 只清当前文件，历史内网信息确认占位级风险后接受残留 | |
| 先扫描再决定 | Claude 先跑全历史敏感信息扫描出报告再定力度 | |

**User's choice:** 全历史重写清理
**Notes:** 接受 commit hash 全变、强制推送重建远端的代价。

### 仓库名

| Option | Description | Selected |
|--------|-------------|----------|
| skillforge | 与项目品牌一致，README 标题 SkillForge + 一句话 value prop | ✓ |
| 其他名称 | 用户有其他命名想法 | |
| 给候选名再选 | Claude 给 2-3 候选名 | |

**User's choice:** skillforge
**Notes:** 当前目录 ai-agent 过于通用。

---

## examples 与 eval 结果发布

### examples/ Skill 模板程度

| Option | Description | Selected |
|--------|-------------|----------|
| 全部可运行 | 三模板都必须真实运行，calculator 需扩展运行时（超 Phase 7 范围） | |
| 校验通过即可（推荐） | compliance 可运行，simple QA + calculator 作格式范例，通过 schema 校验 | ✓ |
| 换成同类可运行模板 | 砍 calculator 换另一个 RAG 模板，保证可运行但多样性变弱 | |

**User's choice:** 校验通过即可（推荐）
**Notes:** calculator 纯工具型 Skill，强行支持执行超范围；DIST-08 字面只要求 schema 校验通过。

### DIST-09 eval 结果进 README

| Option | Description | Selected |
|--------|-------------|----------|
| 手工快照 + 日期标注（推荐） | 手工跑 frozen eval，指标表+日期+lock 版本写进 README，CI 自动化留 06-03 延期 | ✓ |
| 补齐 CI 自动更新 | 合并 eval-gate.yml，CI 跑 eval 自动更新 README badge | |
| 静态一次性表格 | 只放发布时指标静态表格，不承诺持续更新 | |

**User's choice:** 手工快照 + 日期标注（推荐）
**Notes:** 06-03 CI gate 已推迟，拉回 Phase 7 会范围膨胀；手工快照满足品牌叙事。

### DIST-10 eval 样本取样

| Option | Description | Selected |
|--------|-------------|----------|
| 用完整 50 例金标集 | 直接复制 eval/golden/compliance.jsonl 发布，内容完整 | ✓ |
| 精选 10-15 例子集 | 从 50 例精选有代表性子集发布 | |
| You decide | Claude 根据脱敏考量决定 | |

**User's choice:** 用完整 50 例金标集
**Notes:** 50 例已属 sample 规模，完整发布让陌生人能复现评估；合成测试数据无敏感顾虑。

---

## Claude's Discretion

- README 结构与章节顺序（hero 区 value prop + demo GIF + quick start，架构图与 contributing 放后半）
- CONTRIBUTING.md 内容深度（"how to write a Skill" / "add a tool" / "add a knowledge tier" 三节写作深度）
- GIF 录制内容脚本（录哪些场景、时长 30-60s）
- docker-compose 服务编排细节（healthcheck、依赖顺序、数据卷、端口、.env 内网占位替换为 compose 友好默认值）
- 架构图形态（倾向 Mermaid 源码内嵌 README）
- license 文件 + 包头版权头策略（参照 eval/NOTICE 归因先例）
- examples/ 校验器（复用 Phase 2 的 7-field validator，是否暴露 `cmd/skillforge validate` 子命令）

## Deferred Ideas

- CI 自动更新 README eval 指标（06-03 eval-gate.yml 合并）—— 后续版本
- 纯工具型 Skill 运行时支持（让 calculator 真正可运行）—— 新能力，后续版本
- 在线 live demo 服务 —— 用户明确拒绝部署；未来想做需重新评估 key 成本/滥用防护
