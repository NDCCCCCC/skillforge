# Phase 48: CI 矩阵 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-19
**Phase:** 48-CI 矩阵
**Areas discussed:** Lint 基线策略, Fork PR 与 eval-gate secrets, Go 门禁范围与版本, Docker 门深度与工作流组织

**Mode:** discuss（config 确认）· ADVISOR_MODE=true · 校准档 `standard`（USER-PROFILE 评级 `pragmatic-fast`）· NON_TECHNICAL_OWNER=false

---

## Lint 基线策略

| Option | Description | Selected |
|--------|-------------|----------|
| A 全量 linter + changed-lines 门禁 | `.golangci.yml` v2，`default: standard` + revive 显式规则表 + `std-error-handling` preset，测试文件排除 errcheck；action `only-new-issues: true` | ✓ |
| B 全树门禁 + 提交排除清单 | 先跑一次全树 lint，转写进 `linters.exclusions.rules/paths` 并提交 | |
| C 先修存量再全树阻断 | 专项修复提交清掉全部存量，再裸跑，无豁免 | |
| D 先上建议型 job | `continue-on-error: true`，先量噪声 | |

**User's choice:** A — 全量 linter + changed-lines 门禁

**Notes:** 研究员实测的存量规模构成了决策依据：521 个 Go 文件 / 55 包；`internal/` 约 175 处未检查调用、约 102 处裸 `defer X.Close()`、约 74 个导出符号缺文档注释；revive `unused-parameter` 已在 MCP 形 handler 上误报 5 次。CGO 依赖（go-sqlite3 / go-duckdb）使 lint 在 Windows 宿主上于 `internal/server/` 的 cgo 预处理直接失败 → 决定 D-05（lint 必须设 `CGO_ENABLED=1` 且在有 gcc 的环境跑）。选项 C 被否决的核心理由是：在一个纯 CI 管道的 phase 里做跨数十文件的大规模清理属范围蔓延，且易催生 `nolint` 泛滥。选项 D 被否决因其**不满足 CICD-01 与 phase 成功标准**，且正是 Phase 47 刚修掉的"没跑却报绿"反模式。

---

## Fork PR 与 eval-gate secrets

| Option | Description | Selected |
|--------|-------------|----------|
| B job 级 `if:` 门禁 + skip notice | 非 pull_request 事件或 `head.repo.full_name == github.repository` 才跑，另配可见 `::notice::` job；排除 dependabot[bot] | ✓ |
| A 不动 eval-gate | eval-gate.yml 零改动，只加三条 hermetic workflow | |
| C 收窄触发器 | 触发器改为 `push: main` + `workflow_dispatch`，彻底不跑 fork | |
| D 推迟到 Phase 49 | 本 phase 只记债，等 hermetic 模式建成再改造 | |

**User's choice:** B — job 级 `if:` 门禁 + skip notice

**Notes:** 决策被一项代码实证推翻：ROADMAP SC-4 把 eval-gate 称作"既有 hermetic 先例"**不成立**——`cmd/eval` 的 `--frozen` 路径在打分之前就 bootstrap BGE-M3 / Milvus / GLM，缺 secret 时 `log.Fatalf` 致命退出；只有 `--gate-check` 阶段 hermetic，而 replay 先崩时根本走不到该阶段。结合 `CONTRIBUTING.md` 把 fork + PR 定为主贡献路径，每个外部 PR 触发一次必红的 eval-gate 直接违反 Phase 48 "外部贡献者的每次 PR 公开绿灯"。选定 B 的关键理由是它是**既保住维护者侧 live 行为、又消除外部 PR 必红、改动最小**的一刀；A 的致命伤是若 eval-gate 成为 required check 则外部 PR 永远合不进去；C 的致命伤是若它是 required check 则不触发的 run 会让 check 卡在 **Pending** 阻塞合并（与 job skip 报 Success 的行为不同）。`pull_request_target` 在全部选项中被**显式禁止**（它会用仓库 secrets + write token 执行 fork 可控代码）。另注意 Dependabot：其分支在同仓但同样拿不到 secrets，`head.repo.full_name` 抓不到，须单独排除。

### 跟进追问：SC-4 措辞更正的归属

| Option | Description | Selected |
|--------|-------------|----------|
| 当 phase 内文档修正改掉 | 改 ROADMAP.md 措辞 + `verify-phase-48.sh` 加断言（旧措辞不存在、新措辞存在） | ✓ |
| 只记录不动文字 | 仅在 CONTEXT.md / 处置记录里记下事实，ROADMAP.md 不动 | |

**User's choice:** 当 phase 内文档修正改掉

**Notes:** 判据是 v1.7 里程碑继承的标准之一「文档表述与构建现实一致」。SC-4 前半句「live 门禁不进 CI」成立、无需动；后半句的事实错误才是更正对象。归为 phase 内修正而非范围蔓延，是因为若不改字，一个明知不实的表述会继续误导 Phase 51（其 SC 显式依赖 Phase 48 的构建矩阵）。

---

## Go 门禁范围与版本

| Option | Description | Selected |
|--------|-------------|----------|
| A + C 字面 `./...` + 确定性段进 CI | `go-version-file: go.mod` + `go test ./... -count=1`，并把 `-count=100` + `-shuffle=on` 搬进 CI | ✓ |
| B + C 用 `./internal/...` 门禁 | 与 `verify-phase-47.sh` P4 逐字节一致 + `go build ./cmd/...` 冒烟，省约 90s | |
| A 不搬确定性段 | 字面 `./...` 同上，但确定性段只在本地门跑 | |
| A + 确定性段转 nightly | 确定性段改为 schedule 跑 | |

**User's choice:** A + C — 字面 `go test ./...` + 确定性强度段进 CI + 修正 eval-gate 版本 pin

**Notes:** 三个决定性实测事实：
1. **`./...` 在无任何 secret 下实测通过**（exit 0，39 包 ok / 7 包无测试文件；`integration/contract` 受 `//go:build contract` 门控、`internal/eval` 真实测试受 `//go:build liveeval` 门控）——所以取字面范围不必改需求文本，选项 B 的"必须同步改 requirement"代价不成立。
2. **但 `cmd/spike-deep` / `cmd/spike-fanout` 仅在 `GLM_API_KEY` / `BGE_M3_URL` / `MILVUS_ADDRESS` 未设置时才 `t.Skip`** → Go job 绝不声明这三个变量是选项 A 成立的前提，两者不可拆（落为 D-16）。
3. **版本漂移是硬破坏且已在本仓库复现**：`setup-go@v6+` 导出 `GOTOOLCHAIN=local`，于是 `go.mod` 的 `go 1.25.5` 直接报错 `go.mod requires go >= 1.25.5 (running go 1.23.x; GOTOOLCHAIN=local)`。因此 `eval-gate.yml` 的 `@v5` + `'1.23'` 必须在同一 phase 迁走——即使有人主张它"超出范围"也应记录漂移而非留雷（选项 D 被明确否决）。

确定性段的取舍依据：实测成本仅约 11s（`-count=100` 7.2s + `-shuffle=on` 3.9s），而它买到的是对 Phase 47-01 实测 **17–25% flake 率**那类回归的唯一防护；`router_test` / fsnotify 防抖测试天然时序敏感且不响应 `-short`，没有这道门就只能靠本地门。且 `-shuffle=on` 的 seed 必须从 CI 日志捕获，否则失败不可复现。

---

## Docker 门深度与工作流组织

| Option | Description | Selected |
|--------|-------------|----------|
| C 单 ci.yml + composite action 复用 | `ci.yml` 三 job + `.github/actions/setup`（checkout + setup-go + setup-node + setup-buildx），供 Phase 51 复用 | ✓ |
| A 单 ci.yml，不抽 composite | 同样三 job + buildx + GHA 缓存，但 setup 三块会被 Phase 51 复制粘贴 | |
| B 三个独立 workflow 文件 | go.yml / frontend.yml / docker.yml，各自独立入口与历史 | |
| D 裸 docker build，无缓存 | 单 job 两次 `docker build`，无 buildx | |

**User's choice:** C — 单 `ci.yml` 三 job + 仓内 composite action 供 Phase 51 复用

**Notes:** 决定性事实：`docker compose build` **不可行**（compose 含 `${GLM_API_KEY:?}` / `${BGE_M3_URL:?}` 插值，解析即要求 CI 伪造 secret；且 etcd/minio/milvus 是 `image:` 引用、从来不可 build）→ 选项集里直接排除该方向。选 A 而非 B 的理由是 GitHub 的机制事实：**branch protection 匹配的是 job 名而非 workflow 文件名**，跳过的 job 报 "Success"，而按 job 重跑（UI 按钮 + `gh run rerun --job`）已能满足按线重跑；拆文件反而带来三份触发器/concurrency/版本同步负担与 job 名全局唯一约束。选 C 而非 A 的理由是 Phase 51 的 SC 显式写着"Depends on: Phase 48（CI 构建矩阵为 release workflow 提供基础）"——C 是唯一让 release workflow 直接复用 toolchain / cache 设置的选项。选 C 而非 D 的理由是 D 每次全冷拉取（`apk add build-base` + `go mod download` + CGO 链接 + `npm ci` + vite build），docker 腿会主导 PR 墙钟时间，且不给 Phase 51 留任何 builder / 缓存地基。

已记录的缓存收益边界（写给 planner 省一轮试错）：两个 Dockerfile 都是 `COPY . .`，每次提交都让编译层失效，故 GHA 缓存**只对 `go mod download` / `npm ci` 层有效**；`type=gha` 不保留 `RUN --mount=type=cache` 的内容；默认 scope `buildkit` 会让第二个镜像挤掉第一个，必须按镜像分 `scope=`。

---

## Claude's Discretion

以下留给 planner / 执行期决定，未在讨论中锁死：

- 单个 `ci.yml` 内的 job 命名与 `concurrency` 组写法（约束：job 名全局唯一、并发取消不破坏 required check 语义）
- `.golangci.yml` 中 `std-error-handling` 之外是否额外加少量 preset
- `verify-phase-48.sh` 的分片编号（P1..PN）与断言措辞（但必须满足 Specific Ideas 的门禁自证清单）
- 是否为 FE 线加**非阻断**的 `--coverage` 上报
- GHA 缓存所需 `actions: write` 与 D-26「`permissions` 仅 `contents: read`」之间的取舍——planner 须显式取舍并记录，**不得静默放宽**

## Deferred Ideas

- 把 eval-gate 改造成真 hermetic（frozen replay 用 stub LLM / embedded 向量）→ **Phase 49**
- 镜像发布到 registry（GHCR 等）→ **Phase 51**；本 phase `push: false` 是硬边界
- 确定性强度段改 nightly / 按路径触发 → 若实测 Go job 总时长超出 PR 延迟预算再议（本 phase 已定为每 PR 跑）
- 清理 lint 存量债（约 175 处未检查调用、约 102 处裸 `defer X.Close()`、约 74 个导出符号缺文档注释、revive `unused-parameter` 误报）→ 后续 phase 顺带燃烧；本 phase 的 changed-lines 门禁使存量不阻断
- 给 `web/` 引入 eslint → 新增能力，不在本 phase 的三条需求内
- CI 覆盖率门 → 本 phase 不设阈值
