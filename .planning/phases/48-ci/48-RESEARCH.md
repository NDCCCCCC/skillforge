# Phase 48: CI 矩阵 - Research

**Researched:** 2026-10-19
**Domain:** GitHub Actions CI matrix (Go / FE / Docker), golangci-lint v2, fork-PR secret gating
**Confidence:** HIGH for versions, permissions, config schema and fork-PR semantics (verified against GitHub release metadata, action source, official docs, and a local v2.12.2 binary). MEDIUM for first-run Docker/Go-on-`ubuntu-latest` linking and for the exact runtime behavior of `only-new-issues` when `pull-requests: read` is absent (documented + source-read, not executed against GitHub).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Lint 基线策略
- **D-01：** lint 门禁采用「全量 linter 集 + changed-lines 过滤」——不引入 baseline 文件，也不留任何长期豁免清单。
- **D-02：** `.golangci.yml` 用 v2 格式：`default: standard`（含 govet / errcheck / staticcheck / ineffassign / unused）+ `revive` 配**显式规则表**（用户指定的 revive 规则会替换默认集，须显式列出）；`run.timeout` 必设（v2 默认无超时）。
- **D-03：** 启用 `std-error-handling` preset 吸收 `defer X.Close()` / print / `os.Setenv` 类噪声；`_test.go` 内排除 errcheck。**排除 `revive:unused-parameter`**——它已在 MCP 形 handler 上误报（实测 5 次），接口形 handler 天然如此。
- **D-04：** CI 侧用 `golangci-lint-action@v9` + `only-new-issues: true` + pin linter 版本（当前建议 v2.12.x）。**不得**开 `--whole-files`：`internal/server/chat.go` 之类千行文件会把陈年问题漏进来。
- **D-05：** lint job 必须设 `CGO_ENABLED=1` 并在有 gcc 的环境跑（实测 CGO 依赖 go-sqlite3 / go-duckdb 会让 Windows 宿主上的 lint 在 `internal/server/` cgo 预处理阶段直接失败）。
- **D-06：** lint 是**阻断门**，不得使用 `continue-on-error` 或跳过式 `if:`。建议型 job 已被明确否决。
- **D-07：** Go 版本走 `go-version-file: go.mod`（解析出精确 `1.25.5`，符合"零硬编码"标准），不用字面量。
- **D-08：** 仓库实测无 `Code generated ... DO NOT EDIT` 文件，Eino 的泛型密集代码类型检查无需特殊处理——**不引入 generated-code 排除规则**。

#### eval-gate 与 fork PR 的 secret 缺口
- **D-09：** `eval-gate.yml` 加 **job 级 `if:` 门禁**：仅在非 `pull_request` 事件，或 `head.repo.full_name == github.repository` 时运行；同时显式排除 `github.actor == 'dependabot[bot]'`（Dependabot 的分支在同仓，但同样拿不到 secrets）。
- **D-10：** 因**跳过的 job 在 GitHub 上报 "Success"**，必须另配一个可见的 `::notice::` skip-notice job 打破沉默，避免 required check 假绿。这是 Phase 47 HIGH-1（"没跑却报绿"）同类反模式在 CI 侧的对应防线。
- **D-11：** **显式禁止 `pull_request_target`**——它会用仓库 secrets + write token 去执行 fork 可控的 workflow 代码。任何触发器改动都不得引入它。
- **D-12：** ROADMAP SC-4 的措辞更正属**本 phase 内的文档修正**（非范围蔓延）：SC-4 前半句"live 门禁不进 CI"成立，后半句须从「eval-gate.yml 的 frozen-replay + faithfulness gate 是既有 hermetic 先例」改为「既有 **live** workflow，保持维护者 / 同仓行为不回归」。
- **D-13：** 上述 SC-4 更正的依据是已实测的代码事实：`cmd/eval` 的 `--frozen` 路径在打分**之前**就 bootstrap BGE-M3 / Milvus / GLM，缺 secret 时 `log.Fatalf` 致命退出（`cmd/eval/main.go` 中 `envRequired("BGE_M3_URL")` 一带）；只有 `--gate-check` 阶段 hermetic，而 replay 先崩时根本走不到。**`--frozen` 不是 hermetic 门。**
- **D-14：** 三条新线（Go / FE / Docker）**零 secret 引用**，除 `GITHUB_TOKEN` 外不得出现任何 `secrets.*`。

#### Go 门禁范围与版本
- **D-15：** 门禁范围取**字面 `go test ./...`**（CICD-01 / SC-1 不改），已实测无 secret 下 hermetic 通过（exit 0，39 包 ok / 7 包无测试文件；`integration/contract` 受 `//go:build contract` 门控、`internal/eval` 真实测试受 `//go:build liveeval` 门控）。
- **D-16：** **Go job 绝不声明 `MILVUS_ADDRESS` / `BGE_M3_URL` / `GLM_API_KEY` 任何入口**——`cmd/spike-deep` / `cmd/spike-fanout` 仅在这三者未设置时才 `t.Skip`，一旦设置就改走 live 路径。这是 D-15 成立的前提，两者不可拆。
- **D-17：** Go 版本统一走 `go-version-file: go.mod`；**`eval-gate.yml` 的 `setup-go@v5` + `go-version: '1.23'` 在同一 phase 内迁走**。理由是实测硬破坏：`setup-go@v6+` 会导出 `GOTOOLCHAIN=local`，于是 `go.mod` 的 `go 1.25.5` 直接报错 `go.mod requires go >= 1.25.5 (running go 1.23.x; GOTOOLCHAIN=local)`。
- **D-18：** 把 Phase 47 的确定性强度段搬进 CI：`-run '^TestDeriveEdgesDeterministic$' -count=100` + 对 knowledge / ontology 一次 `-shuffle=on`。实测合计约 11s（7.2s + 3.9s）。这是唯一能挡住"已修复的 flake 悄悄回归"的机制——Phase 47-01 测得修复前 flake 率 **17–25%**，且 `router_test` / fsnotify 防抖测试天然时序敏感、不响应 `-short`。
- **D-19：** `-shuffle=on` 的 seed 必须从 CI 日志捕获（失败不可复现即等于无价值）。
- **D-20：** CI 的 Go 门命令形态应**复刻 `verify-phase-47.sh` 的 P1 / P4 / P5**（本地门与 CI 门说同一件事，正是 Phase 47-08 的设计意图）。

#### Docker 门与工作流组织
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

### Deferred Ideas (OUT OF SCOPE)
- **把 eval-gate 改造成真 hermetic**（frozen replay 用 stub LLM / embedded 向量跑）——属 **Phase 49（Hermetic Demo）**。Phase 49 的 stub/embedded 基础设施建成后才可能做；本 phase 只保证 fork PR 不会因它变红。
- **镜像发布到 registry**（GHCR 等）——本 phase `push: false` 是硬边界；发布工程属 Phase 51。
- **把确定性强度段改 nightly / 按路径触发**——D-18 已定为每个 PR 都跑（约 11s）。若实测 Go job 总时长超出 PR 延迟预算，再作为后续调整项。
- **清理 lint 存量债**（约 175 处未检查调用、约 102 处裸 `defer X.Close()`、约 74 个导出符号缺文档注释、revive `unused-parameter` 误报）——不在本 phase。D-01 的 changed-lines 门禁使存量不阻断，可由后续 phase 顺带燃烧。
- **给 FE 加 eslint**——`web/` 当前无任何 eslint 配置，CICD-02 的门只有 `tsc --noEmit` + `vitest run`。引入 eslint 属新增能力，不在本 phase 的三条需求内。
- **CI 覆盖率门**——本 phase 不设覆盖率阈值；如需，最多作为非阻断上报（D 见 Claude's Discretion）。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CICD-01 | Go CI 进 GitHub Actions——golangci-lint + `go test ./...` + CGO_ENABLED=1 build+vet，PR/push 触发 | Go job design below; action pins verified; golangci v2 config verified locally; P1/P4/P5 command parity mapped; `go test ./...` measured exit 0 in WSL (47 packages ok, 8 no test files, 0 FAIL) |
| CICD-02 | FE CI——`tsc --noEmit` + `vitest run` 全量 | FE job design; `npx tsc --noEmit` exists via `typescript ^5.6.0`; `npx vitest run` measured 85 files / 689 tests passing in 27.7s |
| CICD-03 | Docker 镜像构建验证——backend/frontend 双 Dockerfile 在 CI 中可构建 | Docker job design with buildx + per-scope GHA cache; both Dockerfiles read and their context assumptions verified; Docker is unavailable on this host, so CI is the only execution venue |
</phase_requirements>

## Summary

The phase is a workflow-authoring phase, not a code phase. The critical work is (1) getting three jobs green on the public `ubuntu-latest` runner without any intranet service or secret, (2) making the lint gate meaningful without letting ~175 legacy errcheck findings flood every PR, and (3) closing the eval-gate fork-PR hole without turning a skipped live gate into a false green. All six pinned action majors named in CONTEXT exist and are current: `actions/checkout@v7` (v7.0.1), `actions/setup-go@v7` (v7.0.0), `actions/setup-node@v7` (v7.1.0), `docker/setup-buildx-action@v4` (v4.4.1), `docker/build-push-action@v7` (v7.4.0), `golangci/golangci-lint-action@v9` (v9.3.0). No pin has moved or disappeared.

Two locked decisions cannot hold exactly as written and must be amended before planning (details in **Decision Conflicts**): D-04 + D-26 conflict because `only-new-issues` fetches the PR diff through `pulls.get`, which requires `pull-requests: read`; with `contents: read` only, the action *silently* falls back to whole-repo lint (source-read, not a hard error). D-22 conflicts with the local-action resolution rules because a `uses: ./.github/actions/setup` step cannot itself perform the first `actions/checkout` — the runner resolves local actions from the workspace, so checkout must precede the composite invocation.

Everything else in CONTEXT is confirmed: setup-go v7 unconditionally exports `GOTOOLCHAIN=local` (source-read), so `go-version-file: go.mod` is both necessary and sufficient to install 1.25.5; skipped jobs do report "Success" (official docs) while path-filtered workflows stay "Pending"; `type=gha` scale/scope semantics and the cache-mount limitation are confirmed by Docker docs; and `permissions` does **not** need `actions: write` for cache export from PR/push runs (cache access is event-scoped, not `permissions`-scoped).

**Primary recommendation:** Write one `.github/workflows/ci.yml` with `go` / `fe` / `docker` jobs, a checkout-first composite setup action containing only the three toolchain setups, a verified v2 `.golangci.yml` pinned to golangci-lint v2.12.2, and a `verify-phase-48.sh` that asserts the invariants statically plus proves the linter has teeth in a temp module. Grant the go job `pull-requests: read` (read-only) and move checkout out of the composite — both are amendments to D-26/D-22, not silent workarounds.

## Decision Conflicts

> These are explicit conflicts between locked decisions and verified platform behavior. The planner must surface these for user approval rather than silently work around them.

### ⚠ Conflict 1 — D-04 (`only-new-issues`) vs D-26 (`contents: read` only)

**D-26 says:** `permissions:` 仅授 `contents: read`.

**What verified behavior says:**

- `golangci-lint-action@v9` implements `only-new-issues` by calling `octokit.rest.pulls.get({ owner, repo, pull_number, mediaType: { format: 'diff' } })` for `pull_request` events (`src/patch.ts`). The action README's permission block states: `# Optional: allow read access to pull requests. Use with only-new-issues option.` / `pull-requests: read` [CITED: github.com/golangci/golangci-lint-action v9 README].
- GitHub's permission reference states: "If you specify the access for any of these permissions, all of those that are not specified are set to `none`." [CITED: github.com/github/docs `data/reusables/actions/github-token-scope-descriptions.md`]. A workflow/job block of `contents: read` therefore yields `pull-requests: none`.
- On failure, the action **does not fail**: `fetchPullRequestPatch` catches the error, logs `console.warn`, and returns `""`; `src/run.ts` only adds `--new-from-patch` when the patch path is truthy. The result is a full-repo lint run with no error — precisely the "silent scope change" D-01 and D-06 forbid [CITED: golangci-lint-action v9 `src/patch.ts` + `src/run.ts`].
- Consequence if unchanged: the first PR touching any Go file reports the full legacy set (CONTEXT estimates ~175 unchecked calls / ~74 missing doc comments), so the blocking lint gate is red by construction.

**Recommended amendment (read-only, preserves D-26's intent):** workflow-level `permissions: contents: read`; the `go` job adds `pull-requests: read`. No write scope is introduced. This is compatible with fork PRs: GitHub's docs state that the `permissions` key can add/remove **read** permissions for forked repositories (it only blocks granting write) [CITED: github/docs `content/actions/reference/workflows-and-actions/workflow-syntax.md` §Using the `permissions` key for forked repositories]. The verify script should assert: every `permissions` scope in `.github/workflows/` is one of `contents: read` / `pull-requests: read`, and no `write` scope exists anywhere. If the user refuses the amendment, the only alternatives are (a) drop D-04 and run whole-repo lint (contradicts D-01), or (b) use `--new-from-rev` with `fetch-depth: 0` (changes push-event semantics and still requires git history, not the GitHub API).

### ⚠ Conflict 2 — D-22 ("composite action 含 checkout") vs local-action resolution

**D-22 says:** 抽出仓内 composite action（`.github/actions/setup`，含 checkout + setup-go + setup-node + setup-buildx）。

**What verified behavior says:** a local action is resolved from `github.workspace`; if the repository has not been checked out, the runner throws `Can't find 'action.yml', 'action.yaml' or 'Dockerfile' under '<path>'. Did you forget to run actions/checkout before running your local action?` [CITED: actions/runner `src/Runner.Worker/ActionManager.cs`]. The official local-action documentation example also checks out first, then uses `./.github/actions/hello-world-action` [CITED: github/docs `data/reusables/actions/workflows/section-referencing-an-action-from-the-same-repository.md`]. A composite action cannot bootstrap its own checkout.

**Recommended amendment:** each job does `uses: actions/checkout@v7` first, then `uses: ./.github/actions/setup`; the composite contains setup-go / setup-node / setup-buildx (optionally gated by inputs). D-22's stated purpose — one place for toolchain versions, reusable by Phase 51 — is fully preserved; only the checkout line moves to the three jobs (or to Phase 51's workflow). Do **not** leave a redundant checkout inside the composite; a second checkout would reset the workspace and add time.

### ⚠ Premise correction — "GHA 缓存需 `actions: write`"

CONTEXT `<specifics>` states: "GHA 缓存需 `actions: write` 权限，若与 D-26 的'仅 contents: read'冲突，planner 须显式取舍并记录."

**Verified:** Docker's official GHA cache docs describe cache access in terms of **events and refs**, not `permissions`: "Regular `pull_request` runs using a merge ref retain read-write cache access by default"; only events like `issue_comment`, `pull_request_target`, and `pull_request [closed]` get read-only cache access [CITED: docker/docs `content/manuals/build/ci/github-actions/cache.md` §Cache write restrictions]. GitHub's cache docs likewise scope cache access by branch/merge-ref, not by the `permissions` block [CITED: github/docs `content/actions/reference/workflows-and-actions/dependency-caching.md` §Restrictions for accessing a cache]. `actions: write` is the permission for cache **management/deletion** APIs (e.g. `gh cache delete`), not for `cache-to: type=gha` in a normal PR/push run.

**Conclusion:** D-26 does **not** conflict with Docker cache export. Keep `contents: read` (plus the Conflict-1 amendment). If the first run ever prints `failed to reserve cache`, check the event/ref first (e.g. `pull_request: [closed]`), not the permission block.

### Non-conflicts / clarifications

- **D-15 vs D-20 (P4 scope).** `verify-phase-47.sh` P4 runs `go test ./internal/...`; CICD-01/D-15 require literal `go test ./...`. The CI command is a superset and satisfies both. Do not run both P4 and `./...` (wasteful); the verify script should assert the `./...` command and that P1/P5 strings are present.
- **D-18/D-19 (shuffle seed).** Go only prints the seed line with `-v` (measured: `-test.shuffle 1792392631789960520` appears as the first line with `-v`; no seed line without it). Add `-v` to the shuffle step or the seed is unrecoverable from CI logs.
- **D-04 linter pin.** CONTEXT suggests v2.12.x; the local verified binary is **v2.12.2** (built 2026-05-06), while the newest release is v2.15.0 (2026-10-16, three days old at research time). Pin **v2.12.2** so `verify-phase-48.sh` can verify the same schema/behavior locally; upgrading is a separate, low-risk change.
- **Test-count wording drift (not a locked decision).** ROADMAP line 113 says "666 Go 测试 + 84 FE 测试"; the actual FE suite is **85 test files / 689 tests**, and the Go suite is **47 packages ok / 8 no test files / 0 FAIL** (`go test ./...` in WSL, 2026-10-19). Do not hard-code these counts in the gate. D-12 only mandates the SC-4 hermetic wording; count corrections are optional and should not expand scope.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| PR/push trigger, concurrency, required-check identity | GitHub Actions control plane | — | Trigger and job identity are repo-level, not code-level |
| Checkout and toolchain provisioning | Runner (`ubuntu-latest`) | Composite action | Native gcc/node/docker; composite shares version pins |
| Go build + vet + `go test ./...` | Runner | — | Linux runner is the CI equivalent of the local WSL `CGO_ENABLED=1` path |
| Changed-lines lint filtering | Runner + GitHub REST API | — | `only-new-issues` fetches the PR diff via `pulls.get` |
| FE install, typecheck, unit tests | Runner (Node 22) | — | `npm ci` + `tsc --noEmit` + `vitest run` |
| Docker image build + cache | Runner (BuildKit container) | GitHub Actions cache service | buildx docker-container driver; per-image `scope` |
| Fork/Dependabot secret gating | Workflow control flow | — | Job-level `if`; skipped job reports Success |
| Suppression visibility | Skip-notice job | — | `::notice::` annotation breaks the silent-green anti-pattern |
| Local CI-invariant proof | Developer host (Git Bash + WSL) | `verify-phase-48.sh` | Static workflow assertions + linter sentinel + config verify |
| Live eval gate | Intranet-dependent workflow | — | Stays gated; not made hermetic in this phase (Phase 49) |

## Standard Stack

### Core (GitHub Actions)

All versions verified on 2026-10-19 via `gh api repos/<owner>/<repo>/releases/latest`; every repo below is the official upstream.

| Action | Pin | Latest verified | Published | Purpose | Why this version |
|--------|-----|-----------------|-----------|---------|------------------|
| `actions/checkout` | `@v7` | v7.0.1 | 2026-07-20 | Repository checkout (local action prerequisite) | v7 adds a fork-PR guard for `pull_request_target`/`workflow_run`; node24 |
| `actions/setup-go` | `@v7` | v7.0.0 | 2026-07-16 | Go 1.25.5 from `go.mod`, module/build cache | Always exports `GOTOOLCHAIN=local`; `go-version-file` installs the exact version |
| `actions/setup-node` | `@v7` | v7.1.0 | 2026-10-08 | Node 22 + npm cache | node24 runtime; npm cache keyed by `web/package-lock.json` |
| `docker/setup-buildx-action` | `@v4` | v4.4.1 | 2026-09-16 | docker-container builder (cache export capable) | node24; ESM |
| `docker/build-push-action` | `@v7` | v7.4.0 | 2026-09-15 | Dual Dockerfile build, `push: false` | node24; GHA cache backend |
| `golangci/golangci-lint-action` | `@v9` | v9.3.0 | 2026-06-29 | Lint with annotations, caching, `only-new-issues` | Requires node24; compatible with golangci-lint ≥ v2.1.0 |
| `actions/upload-artifact` | `@v7` (eval-gate migration) | v7.0.2 | — | Eval report artifact | Current eval-gate uses `@v4` (node20; runner force-upgrades with an annotation) |

### Core (linter binary)

| Tool | Pin | Verified | Purpose |
|------|-----|----------|---------|
| `golangci-lint` | **v2.12.2** | Local binary reports `2.12.2`; schema verified with `config verify` exit 0 and three sentinel runs | Full linter set + changed-lines filtering |

> Latest is v2.15.0 (2026-10-16). Pinning v2.12.2 is a deliberate local-verifiability choice; the config keys used here exist in both.

### Supporting (verify-time tools already present)

| Tool | Version | Purpose | Availability |
|------|---------|---------|--------------|
| Python + PyYAML | 3.13 / 6.0.3 | Parse workflow YAML in `verify-phase-48.sh` | Present on host (`python`) |
| `gh` CLI | authenticated (`repo`, `workflow` scopes) | Trigger/inspect CI runs, verify action versions | Present |
| WSL Ubuntu-22.04 Go | `go1.25.5 linux/amd64` + gcc 11.4.0 | Local Go gate parity | Present |
| Node/npx | host Node available | FE gate parity | Present |
| Docker | — | Local image build | **Absent** (Git Bash and WSL) |
| `actionlint` / `yq` | — | Workflow schema lint | Absent (PyYAML + explicit assertions substitute) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Single `ci.yml` + 3 jobs (D-21) | Three workflow files | Job names are what branch protection matches; three files triplicate triggers/concurrency/version pins. Keep D-21. |
| `only-new-issues` | `--new-from-rev` after `fetch-depth: 0` | Avoids the `pull-requests: read` amendment but changes push-event semantics and is git-history based; contradicts D-04. |
| `docker compose build` | `build-push-action` per Dockerfile | Compose interpolates `${GLM_API_KEY:?}` / `${BGE_M3_URL:?}` and references non-buildable service images. D-25 is correct. |
| `actions: write` | — | Not needed for cache export from PR/push merge refs; only for cache management APIs. |

**Installation:** none. This phase adds no npm/PyPI/crates dependencies; all runtime units are GitHub Actions fetched by the runner.

## Package Legitimacy Audit

**Registry packages: one.** The supply-chain units below are GitHub Actions, verified against GitHub's release API; the single registry package this phase installs (pip `pyyaml`, added to the CI `verify` job by 48-11) is row-listed in the same table.

| Unit | Source Repo | Latest | Official Org | Verdict | Disposition |
|------|-------------|--------|--------------|---------|-------------|
| `actions/checkout` | github.com/actions/checkout | v7.0.1 | Yes | OK | Approved `@v7` |
| `actions/setup-go` | github.com/actions/setup-go | v7.0.0 | Yes | OK | Approved `@v7` |
| `actions/setup-node` | github.com/actions/setup-node | v7.1.0 | Yes | OK | Approved `@v7` |
| `actions/upload-artifact` | github.com/actions/upload-artifact | v7.0.2 | Yes | OK | Approved `@v7` (eval-gate) |
| `docker/setup-buildx-action` | github.com/docker/setup-buildx-action | v4.4.1 | Yes | OK | Approved `@v4` |
| `docker/build-push-action` | github.com/docker/build-push-action | v7.4.0 | Yes | OK | Approved `@v7` |
| `golangci/golangci-lint-action` | github.com/golangci/golangci-lint-action | v9.3.0 | Yes | OK | Approved `@v9` |
| `golangci-lint` binary | github.com/golangci/golangci-lint | v2.12.2 pinned | Yes | OK | Approved |
| `pyyaml` | github.com/yaml/pyyaml | 6.0.3 | Yes | OK | Pinned `==6.0.3` in ci.yml's gate job (48-12 Task 2) — the version already validated by every local verifier run |

**Packages removed due to SLOP verdict:** none.
**Packages flagged suspicious:** none.
**Note on pinning:** CONTEXT pins moving major tags. Full-SHA pinning is a stronger supply-chain posture but is not a locked decision and would complicate the "version upgrade in one place" goal. If desired, SHA-pin inside the composite action only.

## Architecture Patterns

### System Architecture Diagram

```
Developer / external contributor
        │
        ├── push to main ─────────────┐
        └── pull_request (target main)┘
                                      │
                    ┌─────────────────▼──────────────────┐
                    │ .github/workflows/ci.yml           │
                    │ permissions: contents: read        │
                    │ concurrency: cancel superseded runs│
                    └───┬──────────────┬─────────────┬───┘
                        │              │             │
              ┌─────────▼───┐  ┌───────▼─────┐ ┌─────▼──────────┐
              │ go job      │  │ fe job      │ │ docker job     │
              │             │  │             │ │                │
              │ checkout@v7 │  │ checkout@v7 │ │ checkout@v7    │
              │ composite:  │  │ composite:  │ │ composite:     │
              │  setup-go   │  │  setup-node │ │  setup-buildx  │
              │             │  │             │ │                │
              │ build ./... │  │ npm ci      │ │ build backend  │
              │ vet (P1)    │  │ tsc --noEmit│ │  cache scope=  │
              │ lint (new)  │  │ vitest run  │ │  backend       │
              │ test ./...  │  │             │ │ build frontend │
              │ count=100   │  │             │ │  cache scope=  │
              │ shuffle -v  │  │             │ │  frontend      │
              └─────────────┘  └─────────────┘ │ push: false    │
                                                └────────────────┘
                    ┌──────────────────────────────────────────┐
                    │ .github/workflows/eval-gate.yml          │
                    │ job if: not PR OR same-repo OR not       │
                    │        dependabot                        │
                    │ skip-notice job: ::notice:: on fork/bot  │
                    │ (skipped job reports Success — hence the │
                    │  visible notice job)                     │
                    └──────────────────────────────────────────┘

Local proof: verify-phase-48.sh
  parse workflows → assert invariants → verify .golangci.yml → sentinel errcheck
```

### Recommended Project Structure

```
.github/
├── actions/
│   └── setup/
│       └── action.yml          # setup-go + setup-node + setup-buildx (checkout stays in jobs)
└── workflows/
    ├── ci.yml                  # go / fe / docker (new)
    └── eval-gate.yml           # fork gate + skip notice + go-version-file (modified)
.golangci.yml                   # v2 schema (new)
verify-phase-48.sh              # repo root, P1..PN fail-fast (new)
```

### Pattern 1: Single workflow, three always-on jobs

**What:** One `ci.yml`; no job-level `if`, no `paths:` filters. Every push/PR produces all three check results.
**When to use:** Always for required checks. A skipped job reports Success, and a path-filtered workflow leaves checks Pending (both documented), so job-level skips are the only honest way to skip — and the three quality gates are never skipped.
**Evidence:** Skipped job → "Success"; path/branch filtering → "Pending" [CITED: github/docs `content/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks.md` §Handling skipped but required checks].

### Pattern 2: Checkout-first + composite toolchain setup

**What:** `actions/checkout@v7` in the job, then `./.github/actions/setup` with inputs selecting go/node/buildx.
**Why:** Local actions require an existing workspace (Conflict 2). Conditional inputs let the docker job skip Go/Node and the FE job skip Go, while keeping version pins in one file.
**Example:** see Code Examples §2.

### Pattern 3: Changed-lines lint with an explicit permission

**What:** `golangci-lint-action@v9` + `only-new-issues: true` + `version: v2.12.2`; job adds `pull-requests: read`.
**Why:** The gate must not flood on legacy debt, and it must not silently change scope when the diff fetch fails.
**Verification:** Source-read of the action plus local sentinel runs (below).

### Pattern 4: Gate the live workflow, then break the silence

**What:** `eval-gate` job-level `if` excludes fork PRs and Dependabot; a sibling job runs only for those cases and emits `::notice::`.
**Why:** A skipped job reports Success, so without the notice the check is indistinguishable from a real green live gate.
**Example:** Code Examples §4.

### Pattern 5: Per-image GHA cache scopes

**What:** `cache-from: type=gha,scope=backend` / `cache-to: type=gha,mode=max,scope=backend`; same for `frontend`.
**Why:** Default scope `buildkit` makes the second build overwrite the first's cache [CITED: docker/docs `content/manuals/build/cache/backends/gha.md` §Scope]. Cache mounts are not preserved [CITED: docker/docs `content/manuals/build/ci/github-actions/cache.md` §Cache mounts].

### Anti-Patterns to Avoid

- **`pull_request_target`:** runs with base-repo secrets + write token; banned by D-11 and, from 2026-11-02, GitHub's default policy for public repos begins enforcement [CITED: github/docs `content/actions/reference/security/securely-using-pull_request_target.md`].
- **`paths:` filters on required workflows:** checks stay Pending and block merges (D-24).
- **`continue-on-error` / skip-`if` on lint:** turns a blocking gate into a suggestion (D-06).
- **Faking compose secrets to run `docker compose build`:** unnecessary and violates D-25.
- **`go-version: '1.23'` literals:** setup-go sets `GOTOOLCHAIN=local`, so the `go.mod` 1.25.5 requirement becomes a hard failure.
- **Whole-repo lint on every PR:** the legacy debt makes the gate permanently red.
- **Relying on `verify-phase-47.sh` exit code alone:** it exits 0 with `PARTIAL PASS — FRONTEND PARTITION NOT VERIFIED` when npx is absent (STATE A2).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Toolchain install + caching | Custom download scripts | `actions/setup-go@v7` / `actions/setup-node@v7` | Correct cache keys, proxy/mirror handling, manifest fallbacks |
| Lint diff computation | `git diff` parsing in a shell step | `golangci-lint-action` `only-new-issues` | Uses the PR/compare API, handles forks, `--new-from-patch` |
| Docker layer caching | Tar/upload cache directories | buildx `type=gha` | BuildKit content-addressed cache, scope separation |
| Workflow YAML validation | Grep-only structural checks | PyYAML parse + explicit key assertions | Grep cannot see nesting or trigger shape |
| Go version discovery | `go-version: '1.25.5'` literal | `go-version-file: go.mod` | Single source of truth; avoids the GOTOOLCHAIN trap |
| Live-secret gating | `if: secrets.X != ''` | Job-level `if` on `head.repo.full_name` / actor | Secrets are empty strings in expressions on forks; the job still starts |
| Cache permission juggling | `actions: write` | Nothing — event/ref already grants write for PR merge refs | Adding write broadens the token for no benefit |

**Key insight:** In this domain every hand-rolled mechanism has a platform-native equivalent whose failure modes are documented; the value of the phase is in composing those primitives correctly, not in writing shell logic.

## Runtime State Inventory

This is not a rename/refactor phase. The only persistent runtime state is GitHub-side:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no renamed identifiers, DB keys, or user IDs | None |
| Live service config | Repo secrets `GLM_API_KEY`, `BGE_M3_URL`, `MILVUS_ADDRESS` already configured for eval-gate (workflow references them; values out of repo) | None; do not read/echo them |
| OS-registered state | None | None |
| Secrets/env vars | The three live secrets above; `GITHUB_TOKEN` is automatic | No key rename; CI lines reference none of the live secrets |
| Build artifacts | No existing GHA workflow caches (first ci.yml run); eval-gate uploads `eval-gate-report-<run_id>` artifacts | New cache scopes `backend` / `frontend`; no migration |
| Branch protection | `GET /repos/NDCCCCCC/skillforge/branches/main/protection` returned 404 "Branch not protected" | None this phase; note that required-check names are not yet enforced |

**Canonical question:** after the workflow files are committed, nothing outside git holds the old state. The only operational change is that new GHA caches will be created under `backend` / `frontend` scopes.

## Common Pitfalls

### Pitfall 1: `only-new-issues` silently becomes whole-repo lint
**What goes wrong:** Every PR turns red with legacy findings, or (worse) the team adds `continue-on-error` to compensate.
**Why it happens:** Missing `pull-requests: read`; the action warns and runs without `--new-from-patch` instead of failing.
**How to avoid:** Add `pull-requests: read` at the `go` job level; assert it in `verify-phase-48.sh`.
**Warning signs:** Workflow log contains `failed to fetch pull request patch` and the issue count jumps to legacy levels.

### Pitfall 2: Composite action cannot check out itself
**What goes wrong:** Runner fails with `Can't find 'action.yml' ... Did you forget to run actions/checkout before running your local action?`
**How to avoid:** Checkout in each job before `uses: ./.github/actions/setup` (Conflict 2).

### Pitfall 3: `GOTOOLCHAIN=local` + wrong Go literal
**What goes wrong:** `go.mod requires go >= 1.25.5 (running go 1.23.x; GOTOOLCHAIN=local)`.
**Why:** setup-go v6+ (including v7) unconditionally sets `GOTOOLCHAIN=local` (`src/main.ts::setGoToolchain`).
**How to avoid:** `go-version-file: go.mod` in every workflow; ban `go-version:` literals in the verifier.

### Pitfall 4: Shuffle seed invisible in logs
**What goes wrong:** A `-shuffle=on` failure cannot be reproduced because the seed was never logged.
**Why:** Go prints `-test.shuffle <seed>` only with `-v` (measured).
**How to avoid:** Add `-v` to the shuffle step and keep full logs.

### Pitfall 5: GHA cache scope collision
**What goes wrong:** The frontend build evicts the backend cache; every build is cold.
**How to avoid:** Distinct `scope=backend` / `scope=frontend` on both `cache-from` and `cache-to`.

### Pitfall 6: Expecting cache mounts to survive `type=gha`
**What goes wrong:** The Dockerfiles' `RUN --mount=type=cache` (if added later) provides no cross-run benefit.
**Why:** BuildKit does not preserve cache mounts in the GHA cache by default [CITED: docker/docs cache.md §Cache mounts].
**How to avoid:** Rely only on `go mod download` / `npm ci` layer caching; D-27 already documents this.

### Pitfall 7: `paths:` filter blocks required checks
**What goes wrong:** A required check never runs and stays Pending, blocking the PR.
**How to avoid:** No `paths:` under `on:`; use job-level `if` if a skip is ever needed (D-24).

### Pitfall 8: Fork PR / Dependabot jobs fail on empty secrets
**What goes wrong:** `envRequired("BGE_M3_URL")` calls `log.Fatalf`; the job is red for every external contributor.
**Why:** GitHub withholds secrets from fork PRs and Dependabot runs [CITED: github/docs `data/reusables/actions/workflow-runs-dependabot-note.md`; events docs for fork PR read-only token].
**How to avoid:** D-09 job-level `if`; D-10 notice job.

### Pitfall 9: A skipped job is not a failed job
**What goes wrong:** The live gate is skipped on a fork PR but the required check shows green.
**Why:** "A job that is skipped will report its status as 'Success'." [CITED: github/docs `data/reusables/actions/workflows/skipped-job-status-checks-passing.md`]
**How to avoid:** The notice job is not optional; make it part of the D-10 contract.

### Pitfall 10: `push` on every branch triggers full-repo lint
**What goes wrong:** A push to a brand-new branch has an all-zero `before` SHA; the compare API call fails, the action warns, and lint runs whole-repo → red.
**How to avoid:** Prefer `push: branches: [main]` (matches eval-gate) and let PRs cover feature branches. Omit `workflow_dispatch` from `ci.yml`, or accept full lint on manual runs.

### Pitfall 11: New Go files need a package comment
**What goes wrong:** `revive:package-comments` is in the default rule set; the Apache license header does **not** satisfy it (measured).
**Why:** New files are entirely in the diff, so the package clause is "new".
**How to avoid:** Add `// Package <name> ...` above `package` in new files, or explicitly disable `package-comments` in the revive rule table (currently not in D-03). Note that existing files without a package comment are unaffected unless their package line changes.

### Pitfall 12: Docker not available on the maintainer host
**What goes wrong:** `verify-phase-48.sh` cannot prove CICD-03 locally.
**Why:** Docker is absent from both Git Bash and WSL on this machine.
**How to avoid:** Verify the Dockerfile/docker job statically in the script; prove CICD-03 with the first real `gh run` (see Validation Architecture).

### Pitfall 13: PyYAML parses `on:` as boolean `True`
**What goes wrong:** `doc['on']` raises `KeyError`.
**How to avoid:** Normalize with `doc.get('on', doc.get(True))` in the verifier.

### Pitfall 14: Delegating to `verify-phase-47.sh` by exit code
**What goes wrong:** `PARTIAL PASS` still exits 0.
**How to avoid:** If Phase 48 delegates, grep stdout for `ALL PASSED` and fail otherwise.

### Pitfall 15: Cold Docker build time surprises
**What goes wrong:** The Docker job times out or delays PR feedback.
**Why:** `golang:1.25-alpine` may be behind 1.25.5 (toolchain download), CGO duckdb is heavy, and `COPY . .` invalidates the compile layer each commit.
**How to avoid:** `timeout-minutes: 45`; expect only dependency-layer cache benefit (D-27/D-28).

## Code Examples

### 1. `.golangci.yml` (v2 schema, verified locally)

Verified with `golangci-lint config verify` exit 0 on v2.12.2, and with three sentinel modules: `json.Unmarshal` unchecked → errcheck fires; `defer f.Close()` → suppressed by `std-error-handling`; unused function parameter → **not** reported (revive `unused-parameter` disabled). Source references: golangci-lint v2 reference config keys (`version`, `linters.default`, `linters.enable`, `linters.settings`, `linters.exclusions.presets`, `linters.exclusions.rules`, `run.timeout`) and revive's `Normalize`/`GetLintingRules` semantics.

```yaml
version: "2"

run:
  timeout: 10m

linters:
  default: standard          # errcheck, govet, ineffassign, staticcheck, unused
  enable:
    - revive
  settings:
    revive:
      # Default rules stay on; an explicit rule entry below overrides one.
      enable-default-rules: true
      rules:
        - name: unused-parameter
          disabled: true
  exclusions:
    presets:
      - std-error-handling
    rules:
      - path: _test\.go
        linters:
          - errcheck
```

Notes:
- `linters.default: standard` is the correct v2 key (v1's `disable-all` → `linters.default: none`).
- `run.timeout` is top-level; v2's default is 0 (disabled), so D-02's requirement is real.
- `formatters:` is **not** needed. It only feeds `golangci-lint fmt`; `run` does not check formatting. Leaving it out uses standard Go formatting.
- `revive`'s default rules include `package-comments`, `exported`, `unused-parameter`, etc. With `enable-default-rules: true`, an explicit `disabled: true` entry is honored (verified in `config.go::Normalize`: default rules are added only when the name is absent).
- If the planner prefers a fully enumerated rule table, list revive's 24 defaults explicitly and omit `enable-default-rules`; the concise form above is equivalent and less brittle.

### 2. `.github/actions/setup/action.yml` (checkout excluded — see Conflict 2)

```yaml
name: setup
description: Shared toolchain setup for SkillForge workflows (call checkout first)

inputs:
  go:
    description: Install Go from go.mod
    default: "true"
  node:
    description: Install Node 22 with the web/ npm cache
    default: "true"
  buildx:
    description: Create a BuildKit docker-container builder
    default: "false"

runs:
  using: composite
  steps:
    - if: inputs.go == 'true'
      uses: actions/setup-go@v7
      with:
        go-version-file: go.mod
        cache: true

    - if: inputs.node == 'true'
      uses: actions/setup-node@v7
      with:
        node-version: "22"
        cache: npm
        cache-dependency-path: web/package-lock.json

    - if: inputs.buildx == 'true'
      uses: docker/setup-buildx-action@v4
```

No `run:` steps → no `shell:` requirement. `inputs` is available in composite step `if:` expressions.

### 3. `.github/workflows/ci.yml` (skeleton)

```yaml
name: CI

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

permissions:
  contents: read

concurrency:
  group: ci-${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  go:
    name: go
    runs-on: ubuntu-latest
    timeout-minutes: 20
    permissions:
      contents: read
      pull-requests: read        # required by only-new-issues (Decision Conflict 1)
    env:
      CGO_ENABLED: "1"
    steps:
      - uses: actions/checkout@v7
      - uses: ./.github/actions/setup
        with:
          node: "false"

      - name: build + vet (P1)
        run: |
          go build ./...
          go vet ./internal/ontology/... ./internal/server/... ./internal/knowledge/...

      - name: lint (changed lines)
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.12.2
          only-new-issues: true

      - name: go test ./... (CICD-01 / D-15)
        run: go test ./... -count=1

      - name: determinism -count=100 (P5a)
        run: go test ./internal/knowledge/ -run '^TestDeriveEdgesDeterministic$' -count=100

      - name: determinism -shuffle=on with seed (P5b)
        run: go test ./internal/knowledge/ ./internal/ontology/... -count=1 -shuffle=on -v

  fe:
    name: fe
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@v7
      - uses: ./.github/actions/setup
        with:
          go: "false"
      - name: install
        working-directory: web
        run: npm ci
      - name: typecheck
        working-directory: web
        run: npx tsc --noEmit
      - name: unit tests
        working-directory: web
        run: npx vitest run

  docker:
    name: docker
    runs-on: ubuntu-latest
    timeout-minutes: 45
    steps:
      - uses: actions/checkout@v7
      - uses: ./.github/actions/setup
        with:
          go: "false"
          node: "false"
          buildx: "true"

      - name: build backend image
        uses: docker/build-push-action@v7
        with:
          context: .
          file: Dockerfile.backend
          push: false
          cache-from: type=gha,scope=backend
          cache-to: type=gha,mode=max,scope=backend

      - name: build frontend image
        uses: docker/build-push-action@v7
        with:
          context: .
          file: Dockerfile.frontend
          push: false
          cache-from: type=gha,scope=frontend
          cache-to: type=gha,mode=max,scope=frontend
```

**Do not add** `workflow_dispatch` unless the planner accepts a full-repo lint run on manual invocation (the action has no diff for that event). **Do not add** `paths:` (D-24). If this repo opens PRs whose **base** is a `phase-*` branch, widen `pull_request.branches` to `[main, 'phase-*']` to match the existing eval-gate trigger (line 34); a PR targeting a base not listed here would never start CI.

### 4. eval-gate fork gate + skip notice

```yaml
jobs:
  eval-gate:
    name: Frozen compliance replay + gate
    if: >
      (github.event_name != 'pull_request' ||
       github.event.pull_request.head.repo.full_name == github.repository) &&
      github.actor != 'dependabot[bot]'
    # ... existing steps ...

  eval-gate-skip-notice:
    name: eval-gate skipped (fork / Dependabot)
    if: >
      github.event_name == 'pull_request' &&
      (github.event.pull_request.head.repo.full_name != github.repository ||
       github.actor == 'dependabot[bot]')
    runs-on: ubuntu-latest
    steps:
      - run: |
          echo "::notice::eval-gate skipped: live secrets are unavailable to fork and Dependabot PRs (D-09/D-10). This is NOT a live-gate pass."
```

Also migrate in the same file:
```yaml
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
          cache: true
```
and upgrade `actions/upload-artifact@v4` → `@v7` (node20 is force-upgraded with an annotation today, but v7 removes the warning and matches the other pins).

### 5. `verify-phase-48.sh` — YAML parsing gotcha and linter sentinel

```bash
# PyYAML loads the YAML 1.1 key `on:` as boolean True.
norm_on() { python -c '
import sys, yaml
d = yaml.safe_load(open(sys.argv[1], encoding="utf-8"))
on = d.get("on", d.get(True))
print(on)
' "$1"; }

# Prove the linter has teeth without linting the real repo (Windows host has no cgo-safe lint).
D="$(mktemp -d)"
( cd "$D" && printf 'module sentinel\n\ngo 1.21\n' > go.mod \
  && printf '// Package main is a sentinel.\npackage main\n\nimport "encoding/json"\n\nfunc main() { var v any; json.Unmarshal([]byte("{}"), &v) }\n' > main.go \
  && golangci-lint run --config "$ROOT/.golangci.yml" ./... ) >"$WORK/lint-sentinel.log" 2>&1
[ $? -ne 0 ] || fail "lint sentinel: errcheck did not fire — the config has no teeth"
grep -q 'errcheck' "$WORK/lint-sentinel.log" || fail "lint sentinel: failure was not errcheck"
```

`golangci-lint config verify --config "$ROOT/.golangci.yml"` must exit 0. If the binary is absent or its version differs from the pinned v2.12.2, print `LINT NOT VERIFIED` and exit non-zero — never print `ALL PASSED`.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| node20 actions | node24 actions; runner force-upgrades node20 with an annotation | Runner v2.333.0 / 2026-06-16 enforcement | v7-era actions are the clean path; v4 still runs |
| `pull_request_target` for fork CI | `pull_request` + read-only token + gated jobs; GitHub default policy blocks `pull_request_target` in public repos from 2026-11-02 | 2025–2026 | Reinforces D-11 |
| `go-version: '1.23'` literals | `go-version-file: go.mod` | setup-go v5+ | Avoids `GOTOOLCHAIN=local` hard failures |
| Whole-repo lint baselines | Changed-lines filtering (`only-new-issues` / `--new-from-patch`) | golangci-lint v1.57+ | Lets a gate ship without cleaning legacy debt |
| Single `type=gha` cache scope | Per-image `scope=` | buildx | Prevents cache eviction between images |
| Path-filtered required workflows | Always-on jobs with job-level skips | GitHub documented behavior | Avoids Pending required checks |

**Deprecated/outdated:**
- `pull_request_target`: banned (D-11) and policy-blocked.
- `golangci-lint` v1 config (`disable-all`, top-level `exclude-rules`): replaced by v2 `linters.default`, `linters.exclusions`.
- `actions/upload-artifact@v4`: node20; migrate to v7.
- Whole-repo lint with a baseline file: no baseline in this repo (D-01).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `go-duckdb v2.4.3` links on `ubuntu-latest` with the runner's system gcc/g++ | Standard Stack / Docker | Docker/Go job fails on first run; fix is likely an apt package or pinned runner image |
| A2 | `cache-to: type=gha` succeeds with only `contents: read` (+ `pull-requests: read`) on PR/push merge refs | Decision Conflicts / Docker | Cache export warns and builds are cold; not fatal (build still succeeds) |
| A3 | `pulls.get` with `contents: read`-only is denied (making `pull-requests: read` necessary) | Decision Conflicts | If GitHub still allows public PR reads, the amendment is harmless; if denied as expected, D-26 must be amended |
| A4 | Node 22 on the runner reproduces the local vitest behavior (85 files / 689 tests) | Validation Architecture | FE job red on a platform-specific test; first CI run reveals it |
| A5 | `golangci-lint` v2.12.2's binary on the runner analyzes a Go 1.25.5 module without a toolchain download | Standard Stack | Lint job fails to typecheck; fallback is bumping the linter pin |
| A6 | No branch protection is configured (verified 404), so job names are free choices | Runtime State Inventory | If protection is enabled later, renaming `go`/`fe`/`docker` would require a settings update |
| A7 | The first Docker build succeeds despite `golang:1.25-alpine` potentially lagging 1.25.5 | Pitfalls | Toolchain auto-download adds time; GOTOOLCHAIN default (auto) handles it inside the image |

## Open Questions (RESOLVED 2026-10-20 — dispositions below; see 48-CONTEXT.md)

1. **Approve the D-26 amendment (`pull-requests: read`)?**
   - What we know: `only-new-issues` requires the PR diff API; without it, the action silently degrades.
   - What's unclear: whether the user considers any non-`contents` permission a violation of D-26's intent.
   - Recommendation: amend D-26 to "read-only scopes only; `pull-requests: read` permitted for the lint job; zero write scopes." This preserves the security invariant.
   - **RESOLVED** (2026-10-20) — decided by **D-29** (48-CONTEXT.md): read-only scopes only, `pull-requests: read` permitted for the lint job, zero write scopes; P3 asserts the `go` job's permissions map is exactly `{contents: read, pull-requests: read}` and P8 asserts every value is `read`/`none`.

2. **Approve the D-22 amendment (checkout outside the composite)?**
   - What we know: local actions cannot bootstrap checkout.
   - What's unclear: whether "版本升级只落一处" is intended to include checkout.
   - Recommendation: keep setup-go/node/buildx in the composite; checkout at job top. Phase 51 copies the same three lines.
   - **RESOLVED** (2026-10-20) — decided by **D-30** (48-CONTEXT.md): checkout moves to the top of each job; the composite keeps setup-go/setup-node/buildx only, and no job carries a redundant second checkout. Phase 51 copies the same three composite lines.

3. **Should `verify-phase-48.sh` also run the full local Go/FE gates?**
   - What we know: Phase 47 already owns those suites; rerunning `go test ./...` takes ~2.5 min locally.
   - What's unclear: whether SC-5 means "CI config is correct" or "the same suites pass locally".
   - Recommendation: the script asserts CI invariants + lint teeth by default; add an opt-in `--full` mode that delegates to `verify-phase-47.sh` and greps `ALL PASSED`. Do not make the default path depend on WSL+node+Docker.
   - **RESOLVED** (2026-10-20) — decided under **Discretion** as shipped: `verify-phase-48.sh` asserts CI invariants + lint teeth by default, the Go/FE suites are the `go`/`fe` jobs' own subject matter in CI, and no `--full` delegation mode was built.

4. **Should eval-gate's `checkout`/`upload-artifact` be upgraded to v7 in this phase?**
   - What we know: v4 is node20 and currently force-upgraded with a warning annotation.
   - What's unclear: scope appetite for files beyond the secret gate.
   - Recommendation: upgrade both while touching the file; it is a two-line change that removes deprecation noise.
   - **RESOLVED** (2026-10-20) — decided by the shipped tree: `.github/workflows/eval-gate.yml` carries `actions/checkout@v7` (line 83) and `actions/upload-artifact@v7` (line 130), both upgraded while the file was touched.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| WSL Ubuntu-22.04 Go | Local Go gate parity | ✓ | go1.25.5 linux/amd64 | — |
| gcc (WSL) | CGO build/test | ✓ | 11.4.0 | — |
| Node / npx (host) | Local FE gate | ✓ | host Node present | Skip with explicit `NOT VERIFIED` |
| golangci-lint (Windows host) | Config verify + sentinel | ✓ | 2.12.2 | `LINT NOT VERIFIED`, non-zero exit |
| Python + PyYAML | Workflow parsing in verifier | ✓ | 3.13 / 6.0.3 | Fail loudly; do not grep-only fall back |
| `gh` CLI | CI run inspection | ✓ | authenticated, `repo` + `workflow` | Manual GitHub UI |
| Docker | Local Docker build proof | ✗ | — | Prove via first CI run |
| `actionlint` | Workflow schema lint | ✗ | — | PyYAML + explicit assertions |
| `yq` | YAML querying | ✗ | — | PyYAML |
| Network to github.com | Action fetch, push, CI | ✓ | — | — |

**Missing dependencies with no fallback:**
- Docker locally → CICD-03 can only be proven by a real GitHub Actions run. The planner must include a post-push verification task (`gh run watch <run-id>` / `gh pr checks`).

**Missing dependencies with fallback:**
- `actionlint`/`yq` → PyYAML structural assertions.

## Validation Architecture

> `workflow.nyquist_validation` is absent in `.planning/config.json` → treat as enabled. This phase adds no application code; the "test framework" is the verifier plus the real CI run.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (existing) + Vitest 2.x (existing) + `verify-phase-48.sh` (new harness) + GitHub Actions |
| Config file | `web/vitest.config.ts`; no Go test config |
| Quick run command | `bash verify-phase-48.sh` (static invariants + lint sentinel) |
| Full suite command | `wsl.exe -d Ubuntu-22.04 -- bash -c "cd /mnt/d/CODE/ClaudeCode/ai-agent && CGO_ENABLED=1 /usr/local/go/bin/go test ./... -count=1"` and `cd web && npx tsc --noEmit && npx vitest run` |
| CI proof | `gh run watch <run-id>` / `gh pr checks <pr>` after pushing the branch |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| CICD-01a | `go build ./...` + targeted vet pass with CGO | integration (CI) | CI `go` job step "build + vet (P1)" | ❌ Wave 0 (`ci.yml`) |
| CICD-01b | Lint gate runs and only fails on new lines | integration (CI) | CI `go` job step "lint (changed lines)" | ❌ Wave 0 |
| CICD-01c | `go test ./...` hermetic, exit 0 | integration (CI) | CI `go` job step "go test ./..." | ❌ Wave 0 |
| CICD-01d | Determinism strength (`-count=100` + `-shuffle=on`) | integration (CI) | CI `go` job steps P5a/P5b | ❌ Wave 0 |
| CICD-02 | `tsc --noEmit` + `vitest run` full | integration (CI) | CI `fe` job | ❌ Wave 0 |
| CICD-03 | Both Dockerfiles build | integration (CI) | CI `docker` job (two build steps) | ❌ Wave 0 |
| D-11 | No `pull_request_target` anywhere | static | `verify-phase-48.sh` trigger scan | ❌ Wave 0 |
| D-09/D-10 | eval-gate job `if` + skip-notice job | static | verifier parses `eval-gate.yml` | ❌ Wave 0 |
| D-14 | `ci.yml` has no `secrets.*` beyond `GITHUB_TOKEN` | static | verifier scans `ci.yml` | ❌ Wave 0 |
| D-20 | CI Go commands match P1/P4/P5 core strings | static | verifier compares strings against `verify-phase-47.sh` | ❌ Wave 0 |
| D-26 | No write scopes / registry / login action | static | verifier parses all workflows | ❌ Wave 0 |
| D-01..D-08 | `.golangci.yml` schema + linter teeth | static + sentinel | `golangci-lint config verify` + errcheck sentinel | ❌ Wave 0 |
| D-12 | SC-4 old wording absent, new present | static | verifier greps `ROADMAP.md` | ❌ Wave 0 |

### Concrete verifiable predicates (for VALIDATION.md generation)

| # | Predicate | Command / check |
|---|-----------|-----------------|
| 1 | Every workflow YAML parses | PyYAML `safe_load` on `.github/workflows/*.yml`; non-zero on failure |
| 2 | No dangerous trigger | No `pull_request_target` key in any parsed workflow (and no non-comment raw occurrence) |
| 3 | eval-gate has the approved gate | `eval-gate` job `if` contains all three: `github.event_name != 'pull_request'`, `head.repo.full_name == github.repository`, `github.actor != 'dependabot[bot]'` |
| 4 | Skip is visible | A job whose `if` is the inverse exists and a step contains `::notice::` |
| 5 | New CI has zero live secrets | `ci.yml` contains no `secrets.GLM_API_KEY` / `BGE_M3_URL` / `MILVUS_ADDRESS` / any `secrets.` except `secrets.GITHUB_TOKEN` |
| 6 | Go uses the single version source | `setup-go` step in every workflow has `go-version-file: go.mod`; no `go-version:` literal anywhere |
| 7 | Go job is hermetic | `go` job env contains `CGO_ENABLED: "1"` and none of `GLM_API_KEY`, `BGE_M3_URL`, `MILVUS_ADDRESS` |
| 8 | Go commands match the local gate | `ci.yml` contains `go build ./...`, `go vet ./internal/ontology/... ./internal/server/... ./internal/knowledge/...`, `go test ./... -count=1`, `-run '^TestDeriveEdgesDeterministic$' -count=100`, `-shuffle=on` |
| 9 | Seed is capturable | The shuffle step includes `-v` |
| 10 | Lint is blocking and pinned | `golangci-lint-action@v9`, `version: v2.12.2`, `only-new-issues: true`, no `continue-on-error`, no job `if` |
| 11 | Lint can read the diff | `go` job grants `pull-requests: read` (only if Conflict 1 is approved) |
| 12 | FE commands are exact | `ci.yml` contains `npm ci`, `npx tsc --noEmit`, `npx vitest run` under the `fe` job |
| 13 | Docker builds both images without publishing | `ci.yml` contains `file: Dockerfile.backend`, `file: Dockerfile.frontend`, two `push: false`; no `load: true` |
| 14 | Docker cache scopes differ | `cache-from`/`cache-to` use `type=gha` with `scope=backend` and `scope=frontend` |
| 15 | No registry surface | Zero `docker/login-action`, `registry:`, `GHCR`, `DOCKERHUB`, `packages: write` in `.github/workflows/` |
| 16 | Permissions are read-only | Every parsed `permissions` mapping has values `read` (or `none`); no `write` |
| 17 | No path filters | No `paths` / `paths-ignore` under any trigger |
| 18 | Composite exists and is referenced | `.github/actions/setup/action.yml` exists; `ci.yml` references `./.github/actions/setup`; callers checkout first |
| 19 | Linter config is valid | `golangci-lint config verify --config .golangci.yml` exit 0 |
| 20 | Linter has teeth | Sentinel module with unchecked `json.Unmarshal` exits non-zero and names `errcheck` |
| 21 | Linter noise is absorbed | Sentinel with `defer f.Close()` exits 0; sentinel with unused parameter does not report `unused-parameter` |
| 22 | SC-4 is corrected | `ROADMAP.md` does not contain the old hermetic phrase and does contain the replacement wording |
| 23 | Verifier itself is honest | Empty file list / missing tooling prints `NOT VERIFIED` and exits non-zero; `ALL PASSED` appears only when every partition ran |

### Sampling Rate
- **Per task commit:** `bash verify-phase-48.sh`
- **Per wave merge:** `verify-phase-48.sh` + the local Go `./...` and FE suite
- **Phase gate:** `verify-phase-48.sh` exit 0 **and** a real GitHub Actions run where `go`, `fe`, `docker` are all green; plus one fork-PR/Dependabot observation (or a documented reasoning check) showing `eval-gate` skipped and the notice job green

### Wave 0 Gaps
- [ ] `.github/workflows/ci.yml` — CICD-01/02/03
- [ ] `.github/actions/setup/action.yml` — D-22
- [ ] `.golangci.yml` — D-01..D-03
- [ ] `verify-phase-48.sh` — SC-5 and D-09/D-11/D-14/D-20/D-26
- [ ] `.github/workflows/eval-gate.yml` modifications — D-09/D-10/D-17
- [ ] `.planning/ROADMAP.md` SC-4 wording — D-12
- [ ] No unit-test framework install needed: existing Go and Vitest suites cover CICD-01/02.
- [ ] Docker local proof is unavailable; the first CI run is the Wave-0 gap closer for CICD-03.

## Security Domain

`security_enforcement` is absent → enabled.

### Applicable ASVS Categories (adapted to CI supply chain)

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | Indirect | No new auth surface; `GITHUB_TOKEN` is automatic; live secrets remain untouched |
| V3 Session Management | No | No sessions in CI |
| V4 Access Control | Yes | Least-privilege `permissions`; job-level gate; no `pull_request_target`; no write scopes |
| V5 Input Validation | Yes | PyYAML parse + explicit assertions; trigger shape is validated, not grepped blindly |
| V6 Cryptography | No (CI config) | No hand-rolled crypto; action pins are the supply-chain control |
| V14 Configuration | Yes | Workflow config is the security boundary; `verify-phase-48.sh` is the config gate |

### Known Threat Patterns for GitHub Actions

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Pwn request (`pull_request_target` + checkout of PR head) | Elevation of Privilege | Ban `pull_request_target`; use `pull_request` + read-only token (D-11) |
| Fork PR secret exfiltration | Information Disclosure | Fork PRs get no secrets; gate the live job (D-09) and make the skip visible (D-10) |
| False-green required check | Spoofing | Skipped jobs report Success; the `::notice::` job distinguishes skip from pass (D-10) |
| Cache poisoning via untrusted triggers | Tampering | Only trusted events write default-branch cache; `pull_request_target` read-only; per-image scopes |
| Action tag mutability / supply chain | Tampering | Official orgs, pinned majors (optionally SHAs); no third-party actions |
| Accidental image publication | Tampering | `push: false`, no login action, no `packages: write` (D-24/D-26) |
| Lint scope silently widening | Repudiation | Assert `pull-requests: read` and fail the verifier if absent; sentinel proves teeth |
| Live endpoint accidentally configured in CI | Information Disclosure / Availability | Verifier bans live secret names in `ci.yml` (D-14/D-16) |

## Sources

### Primary (HIGH confidence)
- GitHub REST/GraphQL release API via `gh api` — latest tags and publish dates for `actions/checkout`, `actions/setup-go`, `actions/setup-node`, `actions/upload-artifact`, `docker/setup-buildx-action`, `docker/build-push-action`, `golangci/golangci-lint-action`, `golangci/golangci-lint` (checked 2026-10-19).
- `actions/setup-go` v7 `src/main.ts` (`setGoToolchain` unconditionally exports `GOTOOLCHAIN=local`) and `src/installer.ts`; `docs/advanced-usage.md` (`go-version-file` reads `toolchain` then `go`; patch versions supported; `go-version` wins if both are set).
- `golangci/golangci-lint-action` v9 `src/patch.ts` (`pulls.get` diff; failures return `""`), `src/run.ts` (`--new-from-patch` only when a patch exists), `action.yml`, README permissions and `only-new-issues` sections.
- `golangci-lint` v2.12.2 `.golangci.reference.yml` (run.timeout, linters.default, exclusions.presets/rules, formatters), `pkg/lint/lintersdb/builder_linter.go` (standard group = errcheck, govet, ineffassign, staticcheck, unused), `pkg/commands/config_verify.go`.
- `mgechev/revive` `config/config.go` (`defaultRules`, `Normalize`, `GetLintingRules`).
- `actions/runner` `src/Runner.Worker/ActionManager.cs` (local-action checkout error) and PR #4303 (node20 → node24 force-upgrade).
- `docker/docs` `content/manuals/build/cache/backends/gha.md` (`scope`, default `buildkit`) and `content/manuals/build/ci/github-actions/cache.md` (cache write restrictions, cache mounts).
- `github/docs` reusables: `skipped-job-status-checks-passing.md`, `workflow-runs-dependabot-note.md`, `github-token-scope-descriptions.md`, `section-referencing-an-action-from-the-same-repository.md`; `troubleshooting-required-status-checks.md`; `workflow-commands.md`; `securely-using-pull_request_target.md`.
- Local execution on 2026-10-19: `golangci-lint` 2.12.2 `config verify` exit 0; three sentinel modules; `go test ./...` in WSL exit 0 (47 ok / 8 no-test / 0 FAIL); `npx vitest run` 85 files / 689 tests pass in 27.74s.

### Secondary (MEDIUM confidence)
- First-run `ubuntu-latest` CGO linkage for `go-duckdb v2.4.3` (assumed from the WSL gcc 11 success; not executed on the runner).
- GHA cache export under `contents: read` (docs describe event/ref scoping; not executed in this repo).

### Tertiary (LOW confidence)
- None used for decisions. The phase-goal test counts (666 / 84 / 85) were not reproducible as stated; actual measured counts are recorded above.

## Metadata

**Confidence breakdown:**
- Standard stack / action pins: HIGH — every version checked against GitHub release metadata.
- golangci-lint v2 config: HIGH — schema verified and behavior probed locally.
- Fork-PR / skipped-check semantics: HIGH — official docs and action source.
- Permission model: HIGH for the `pull-requests: read` requirement; MEDIUM for cache export without `actions: write` (docs-based, not executed).
- Docker build on the runner: MEDIUM — Docker is absent locally, so the first CI run is the proof.

**Research date:** 2026-10-19
**Valid until:** 2026-11-18 (action majors and GitHub semantics move; re-check pins before Phase 51 reuses the composite action)
