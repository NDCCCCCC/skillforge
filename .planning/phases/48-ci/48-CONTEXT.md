# Phase 48: CI 矩阵 - Context

**Gathered:** 2026-10-19
**Status:** Ready for planning

<domain>
## Phase Boundary

把"666 Go 测试 + 85 FE 测试"的可信度从维护者本机 WSL 搬到**公共可见的 GitHub Actions**——三条质量门（Go / FE / Docker）在 PR 与 push 上跑出绿灯，外部贡献者无需任何内网服务或 API key 即可看到并信任这个信号。

**In scope：** `.github/workflows/` 下新增 CI 矩阵（Go 线含 lint + test + CGO build/vet、FE 线含 `tsc --noEmit` + `vitest run`、Docker 线含双 Dockerfile 构建）；新增 `.golangci.yml`；把 setup-go/setup-node/setup-buildx 抽成仓内 composite action 供 Phase 51 复用；修正 `eval-gate.yml` 的 fork-PR secret 缺口与 Go 版本 pin；更正 ROADMAP SC-4 的失实措辞；编写仓库根 `verify-phase-48.sh`。

**Out of scope：** 任何把内网服务（Milvus / BGE-M3 / GLM）引入 CI 的改动；把 eval-gate 改造成真 hermetic（属 Phase 49）；goreleaser / tag 触发 Release（属 Phase 51）；README / 架构图 / 社区模板（属 Phase 50）；镜像发布到 registry（本 phase `push: false`，永不在 CI 中发布镜像）。

</domain>

<decisions>
## Implementation Decisions

### Lint 基线策略
- **D-01：** lint 门禁采用「全量 linter 集 + changed-lines 过滤」——不引入 baseline 文件，也不留任何长期豁免清单。
- **D-02：** `.golangci.yml` 用 v2 格式：`default: standard`（含 govet / errcheck / staticcheck / ineffassign / unused）+ `revive` 配**显式规则表**（用户指定的 revive 规则会替换默认集，须显式列出）；`run.timeout` 必设（v2 默认无超时）。
- **D-03：** 启用 `std-error-handling` preset 吸收 `defer X.Close()` / print / `os.Setenv` 类噪声；`_test.go` 内排除 errcheck。**排除 `revive:unused-parameter`**——它已在 MCP 形 handler 上误报（实测 5 次），接口形 handler 天然如此。
- **D-04：** CI 侧用 `golangci-lint-action@v9` + `only-new-issues: true` + pin linter 版本（当前建议 v2.12.x）。**不得**开 `--whole-files`：`internal/server/chat.go` 之类千行文件会把陈年问题漏进来。
- **D-05：** lint job 必须设 `CGO_ENABLED=1` 并在有 gcc 的环境跑（实测 CGO 依赖 go-sqlite3 / go-duckdb 会让 Windows 宿主上的 lint 在 `internal/server/` cgo 预处理阶段直接失败）。
- **D-06：** lint 是**阻断门**，不得使用 `continue-on-error` 或跳过式 `if:`。建议型 job 已被明确否决。
- **D-07：** Go 版本走 `go-version-file: go.mod`（解析出精确 `1.25.5`，符合"零硬编码"标准），不用字面量。
- **D-08：** 仓库实测无 `Code generated ... DO NOT EDIT` 文件，Eino 的泛型密集代码类型检查无需特殊处理——**不引入 generated-code 排除规则**。

### eval-gate 与 fork PR 的 secret 缺口
- **D-09：** `eval-gate.yml` 加 **job 级 `if:` 门禁**：仅在非 `pull_request` 事件，或 `head.repo.full_name == github.repository` 时运行；同时显式排除 `github.actor == 'dependabot[bot]'`（Dependabot 的分支在同仓，但同样拿不到 secrets）。
- **D-10：** 因**跳过的 job 在 GitHub 上报 "Success"**，必须另配一个可见的 `::notice::` skip-notice job 打破沉默，避免 required check 假绿。这是 Phase 47 HIGH-1（"没跑却报绿"）同类反模式在 CI 侧的对应防线。
- **D-11：** **显式禁止 `pull_request_target`**——它会用仓库 secrets + write token 去执行 fork 可控的 workflow 代码。任何触发器改动都不得引入它。
- **D-12：** ROADMAP SC-4 的措辞更正属**本 phase 内的文档修正**（非范围蔓延）：SC-4 前半句"live 门禁不进 CI"成立，后半句须从「eval-gate.yml 的 frozen-replay + faithfulness gate 是既有 hermetic 先例」改为「既有 **live** workflow，保持维护者 / 同仓行为不回归」。
- **D-13：** 上述 SC-4 更正的依据是已实测的代码事实：`cmd/eval` 的 `--frozen` 路径在打分**之前**就 bootstrap BGE-M3 / Milvus / GLM，缺 secret 时 `log.Fatalf` 致命退出（`cmd/eval/main.go` 中 `envRequired("BGE_M3_URL")` 一带）；只有 `--gate-check` 阶段 hermetic，而 replay 先崩时根本走不到。**`--frozen` 不是 hermetic 门。**
- **D-14：** 三条新线（Go / FE / Docker）**零 secret 引用**，除 `GITHUB_TOKEN` 外不得出现任何 `secrets.*`。

### Go 门禁范围与版本
- **D-15：** 门禁范围取**字面 `go test ./...`**（CICD-01 / SC-1 不改），已实测无 secret 下 hermetic 通过（exit 0，39 包 ok / 7 包无测试文件；`integration/contract` 受 `//go:build contract` 门控、`internal/eval` 真实测试受 `//go:build liveeval` 门控）。
- **D-16：** **Go job 绝不声明 `MILVUS_ADDRESS` / `BGE_M3_URL` / `GLM_API_KEY` 任何入口**——`cmd/spike-deep` / `cmd/spike-fanout` 仅在这三者未设置时才 `t.Skip`，一旦设置就改走 live 路径。这是 D-15 成立的前提，两者不可拆。
- **D-17：** Go 版本统一走 `go-version-file: go.mod`；**`eval-gate.yml` 的 `setup-go@v5` + `go-version: '1.23'` 在同一 phase 内迁走**。理由是实测硬破坏：`setup-go@v6+` 会导出 `GOTOOLCHAIN=local`，于是 `go.mod` 的 `go 1.25.5` 直接报错 `go.mod requires go >= 1.25.5 (running go 1.23.x; GOTOOLCHAIN=local)`。
- **D-18：** 把 Phase 47 的确定性强度段搬进 CI：`-run '^TestDeriveEdgesDeterministic$' -count=100` + 对 knowledge / ontology 一次 `-shuffle=on`。实测合计约 11s（7.2s + 3.9s）。这是唯一能挡住"已修复的 flake 悄悄回归"的机制——Phase 47-01 测得修复前 flake 率 **17–25%**，且 `router_test` / fsnotify 防抖测试天然时序敏感、不响应 `-short`。
- **D-19：** `-shuffle=on` 的 seed 必须从 CI 日志捕获（失败不可复现即等于无价值）。
- **D-20：** CI 的 Go 门命令形态应**复刻 `verify-phase-47.sh` 的 P1 / P4 / P5**（本地门与 CI 门说同一件事，正是 Phase 47-08 的设计意图）。

### Docker 门与工作流组织
- **D-21：** 采用**单个 `.github/workflows/ci.yml` + 三个 job（go / fe / docker）**，不拆三个 workflow 文件。理由：branch protection 匹配的是 **job 名而非 workflow 文件名**；单文件已能通过 UI 按 job 重跑与 `gh run rerun --job` 给到按线重跑；拆文件会带来三份触发器/concurrency/版本同步负担与 job 名全局唯一约束。
- **D-22：** 抽出仓内 composite action（`.github/actions/setup`，含 checkout + setup-go + setup-node + setup-buildx），供 **Phase 51 的 release workflow 复用**（Phase 51 SC 明确依赖 Phase 48 的构建矩阵为基础）。版本升级只落一处。
- **D-23：** Docker job 用 buildx + `cache-from: type=gha` / `cache-to: type=gha,mode=max`，**按镜像分 `scope=`（如 `backend` / `frontend`）**——默认 scope `buildkit` 会让第二个镜像挤掉第一个的缓存。
- **D-24：** `push: false` 且不 `load`：CI 永不发布镜像，也永不产出本地镜像。**任何按线跳过都必须用 job 级 `if:`，绝不能用 `paths:`**——后者会让 required check 卡在 expected 阻塞 PR。
- **D-25：** **不使用 `docker compose build`**。已核实：compose 内含 `${GLM_API_KEY:?}` / `${BGE_M3_URL:?}` 插值，一旦解析 CI 就必须伪造 secret；且 etcd / minio / milvus 是 `image:` 引用、从来不可 build。
- **D-26：** `permissions:` 仅授 `contents: read`；`.github/workflows/` 下**零出现** `docker/login-action`、`registry:`、`GHCR` / `DOCKERHUB`、`packages: write`。
- **D-27：** 已知缓存收益边界（写给 planner 省一轮试错）：两个 Dockerfile 都是 `COPY . .`，每次提交都让编译层失效，故 GHA 缓存**只对 `go mod download` / `npm ci` 层有效**；且 `type=gha` 不保留 `RUN --mount=type=cache` 的内容。
- **D-28：** `Dockerfile.backend` 的 `golang:1.25-alpine` 是浮动 tag，落后补丁时 go.mod 的 `go 1.25.5` 会触发工具链下载——CI 冷构建耗时需按此预期（Go job 整体预期 5–10 分钟；`./...` 本地 warm ≈146s vs `./internal/...` ≈52s，差额主因 `cmd/spike-deep` 37.7s）。

### Claude's Discretion
- 单个 `ci.yml` 内的 job 命名与 `concurrency` 组的具体写法（只要 job 名全局唯一、且并发取消不破坏 required check 语义）。
- `.golangci.yml` 中 `std-error-handling` 之外是否额外加少量 preset。
- `verify-phase-48.sh` 的分片编号（P1..PN）与具体断言措辞（但必须满足下方"门禁自证"要求）。
- 是否为 FE 线加 `--coverage` 上报（仅限**非阻断**的上报，不作门禁）。

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 定义与需求
- `.planning/ROADMAP.md` §"Phase 48: CI 矩阵" — Goal / Depends on / Success Criteria 1-5 的原文；SC-4 的措辞将在本 phase 内被更正（D-12）
- `.planning/REQUIREMENTS.md` — CICD-01（CICD L15-17 区间）/ CICD-02 / CICD-03 逐条原文与 Phase 映射表
- `.planning/PROJECT.md` — 技术栈与 v1.7 里程碑定位（v1.7 Open-Source Readiness：外部用户 clone→跑通→看懂→信任）
- `.planning/STATE.md` — 当前 phase 位置与 Phase 47 完成状态（8/8 plans，验证 26/28，2 PARTIAL，0 FAILED）

### 前序 phase 的门禁契约（本 phase 的地基）
- `verify-phase-47.sh` — 本地零化门。P1 = build+vet（CGO_ENABLED=1）、P4 = `go test ./internal/...` 全量、P5 = 确定性强度段（`-count=100` + `-shuffle=on`）、P6 = FE（`tsc --noEmit` + `vitest run`，唯一允许非致命分支的分片，且 `P6_ran` 是收尾 banner 的唯一真相源）
- `.planning/phases/47-ci/47-CONTEXT.md` — Phase 47 的决策集（本 phase 的口径来源）
- `.planning/phases/47-ci/47-DEBT-DISPOSITION.md` — v1.6 遗留项处置记录（"每个警告有去向"的标准在本 phase 沿用）
- `.planning/phases/47-ci/47-PATTERNS.md` — Phase 47 建立的模式（空 waiver、空日志必须失败等）

### 现有 CI 资产
- `.github/workflows/eval-gate.yml` — 唯一存量 workflow。L74 附近的 `setup-go@v5` + `go-version: '1.23'` 是必须迁走的版本 pin（D-17）；`on:` 块是 D-09 的改动目标
- `cmd/eval/main.go` — `--frozen` 在打分前 bootstrap live 依赖并在缺 secret 时 `log.Fatalf` 的实证来源（D-13）
- `CONTRIBUTING.md` — 把 fork + PR 定为主贡献路径，是"外部贡献者公开绿灯"必须成立的依据

### 工具链与容器
- `go.mod` — `go 1.25.5`，无 `toolchain` 行（D-17 的版本真相源）
- `web/package.json` — `typescript ^5.6.0` + `vitest ^2.0.0`，但**无 `tsc` script**（CI 需显式 `npx tsc --noEmit`）；`test` 即 `vitest run`
- `Dockerfile.backend` — `golang:1.25-alpine`，`build-base` 已装，`CGO_ENABLED=1` 对 go-duckdb v2.4.3 / mattn-go-sqlite3 正确
- `Dockerfile.frontend` — `node:22-alpine` + `npm ci`，需要 npm registry
- `.dockerignore` — 两个 Dockerfile 均以**仓库根为 build context**
- `docker-compose.yml` — 含 `${GLM_API_KEY:?}` / `${BGE_M3_URL:?}` 插值，是 D-25 排除 `compose build` 的实证

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`verify-phase-47.sh` 的分片骨架**：P1..P7 的 fail-fast 分片 + 每分片独立日志 + 空日志判失败（WR-06）结构可直接复制为 `verify-phase-48.sh` 的骨架，planner 无需重新设计门禁形态。
- **`P6_ran` 单一真相源模式**：FE 分片是唯一允许非致命的，且收尾 banner 由"它到底跑没跑"决定而非由测试名决定。CI 侧的 lint job / skip-notice job 应沿用同一"不许把未跑折叠成 PASS"的精神。
- **`examples/skills/*/eval-gate.json` 阈值契约**：eval-gate 的门槛从配置文件读而非硬编码——`verify-phase-48.sh` 若要断言 eval-gate 行为，应读该配置而非写字面量。

### Established Patterns
- **零 waiver**（Phase 47 退役）：残余失败只能落成显式记录，不得用 `t.Skip` / `WAIVED` / `continue-on-error` 维持表面绿灯。
- **文档表述与构建现实一致**（v1.7 继承标准）：`CGO_ENABLED=1` 是 go-duckdb 生产构建的现实路径，不得宣称无 cgo 单 binary。同理，SC-4 的失实措辞必须改掉而非绕过（D-12）。
- **本地门与 CI 门说同一件事**：Phase 47-08 的设计意图，本 phase 必须延续（D-20）。
- **Go handler ≤200 行 / 不静默吞错**：作为 lint 规则集的隐含约束背景。

### Integration Points
- **`.github/workflows/ci.yml` ↔ `verify-phase-48.sh`**：脚本须能**证明 CI 门会真跑**——断言 lint job 无 `continue-on-error`、无跳过 `if:`、设 `CGO_ENABLED=1`、pin v2 action + linter 版本、`golangci-lint config verify` exit 0，并在临时模块里跑一个哨兵 errcheck 违规证明 linter 真会触发；工具缺失时打印 `LINT NOT VERIFIED` 而**非**折叠进 PASS。
- **`.github/workflows/ci.yml` ↔ `go.mod`**：Go 版本单一真相源（`go-version-file: go.mod`），任何 workflow 不得再出现 `go-version: '1.2…'` 字面量。
- **`.github/actions/setup` ↔ Phase 51**：composite action 是本 phase 交给下个 phase 的接口（D-22），其稳定性是跨 phase 契约。
- **`.github/workflows/ci.yml` ↔ `eval-gate.yml`**：eval-gate 的 `if:` 门禁与三条新线同处 `.github/workflows/`，`verify-phase-48.sh` 须扫全目录校验 D-11 / D-14 / D-26 三条不变式（且发现结果为空必须判失败）。

</code_context>

<specifics>
## Specific Ideas

`verify-phase-48.sh` 的门禁自证清单（D-09/D-11/D-14/D-20/D-26 的可执行化，已定方向，逐条措辞留给 planner）：

1. **禁止危险触发器**：解析全部 `.github/workflows/*.yml`，`pull_request_target` 出现即失败并报 file:line（D-11）。
2. **live secret 只许在门后**：任何触发 `pull_request` 且引用 `GLM_API_KEY|BGE_M3_URL|MILVUS_ADDRESS` 的 workflow，必须带已批准的 job 级 `if:`（`head.repo.full_name == github.repository` + Dependabot 排除）与可见 skip-notice job，否则失败（D-09/D-10）。
3. **触发器回归检查**：eval-gate 的 `on:` 块须为已批准的形状，防止静默放宽或删除（D-09）。
4. **新线零 secret**：三条新 workflow 除 `GITHUB_TOKEN` 外无任何 `secrets.*`；发现文件列表为空必须判失败（D-14）。
5. **Go 线**：断言用 `go-version-file: go.mod`、任何 workflow 无 `go-version: '1.2…'` 字面量、测试命令串与选定范围一致、确定性段（`-count=100` + `-shuffle=on`）存在、Go job 不声明 live 服务 secret、`eval-gate.yml` 不再 pin 1.23（D-15~D-20）。
6. **Docker 线**：断言 `dockerfile: Dockerfile.backend` 与 `dockerfile: Dockerfile.frontend` 两个构建调用都在、`push: false`、`cache-from`/`cache-to: type=gha` 且 scope 不同、`permissions` 仅 `contents: read`、全目录零 `docker/login-action` / `registry:` / `GHCR` / `DOCKERHUB` / `packages: write`（D-23/D-24/D-26）。
7. **lint 线**：见 Integration Points 的自证要求（D-01~D-08）。
8. **SC-4 措辞**：断言 ROADMAP.md 中旧措辞已不存在、新措辞存在（D-12）。

**当前 majors 记录**（planner 落地时按此 pin，版本漂移属实现期重新核对项）：`actions/checkout@v7`、`actions/setup-go@v7`、`actions/setup-node@v7`、`docker/setup-buildx-action@v4`、`docker/build-push-action@v7`、`golangci-lint-action@v9`（需 node24；v8 配 golangci-lint ≥ v2.1.0）。GHA 缓存需 `actions: write` 权限，若与 D-26 的"仅 contents: read"冲突，planner 须显式取舍并记录，不得静默放宽。

</specifics>

<deferred>
## Deferred Ideas

- **把 eval-gate 改造成真 hermetic**（frozen replay 用 stub LLM / embedded 向量跑）——属 **Phase 49（Hermetic Demo）**。Phase 49 的 stub/embedded 基础设施建成后才可能做；本 phase 只保证 fork PR 不会因它变红。
- **镜像发布到 registry**（GHCR 等）——本 phase `push: false` 是硬边界；发布工程属 Phase 51。
- **把确定性强度段改 nightly / 按路径触发**——D-18 已定为每个 PR 都跑（约 11s）。若实测 Go job 总时长超出 PR 延迟预算，再作为后续调整项。
- **清理 lint 存量债**（约 175 处未检查调用、约 102 处裸 `defer X.Close()`、约 74 个导出符号缺文档注释、revive `unused-parameter` 误报）——不在本 phase。D-01 的 changed-lines 门禁使存量不阻断，可由后续 phase 顺带燃烧。
- **给 FE 加 eslint**——`web/` 当前无任何 eslint 配置，CICD-02 的门只有 `tsc --noEmit` + `vitest run`。引入 eslint 属新增能力，不在本 phase 的三条需求内。
- **CI 覆盖率门**——本 phase 不设覆盖率阈值；如需，最多作为非阻断上报（D 见 Claude's Discretion）。

</deferred>

---

*Phase: 48-CI 矩阵*
*Context gathered: 2026-10-19*
