---
phase: 51-release-engineering
status: complete
---

# Phase 51 Context — 发布工程（v1.7 收官）

## 用户决策

- **直接规划**（沿用 Phase 50 先例）：跳过 discuss/research，决策由编排者定并记录于此。
- 发布目标：push tag（`v1.7.0`）⇒ GitHub Actions 用 goreleaser 构建 + 建 Release + 结构化 notes，零手工步骤（ROADMAP SC1-SC4）。

## <decisions>

- **D-51-01 — 首版资产矩阵 = linux-amd64 + windows-amd64。** 两者都是 CGO_ENABLED=1 构建。Linux 在 ubuntu-latest（gcc 现成）。Windows 用 msys2 + mingw-w64 gcc —— GitHub runner 的 mingw 与宿主不同版本，宿主的 emutls/duckdb 链接失败不外推到 runner；**能否链接是 unknown**，用 goreleaser 的多构建定义让两个目标互不阻塞（一个失败另一个照常发布？——不：goreleaser 默认全或无 ⇒ 用 `builds` 细分 + nightly/skip 语义不可取，改为接受"全或无"，但把 windows 目标列为 flagged assumption，若 runner 链接失败则首版缩到 linux-amd64 并在 RELEASE NOTES 如实说明，workflow 不改）。
- **D-51-02 — 二进制 = 单一 `server` 可执行**（cmd/server 是唯一面向外部用户的入口；cmd/eval 是内部校准工具，不发布）。
- **D-51-03 — release notes 自动生成**：goreleaser `release: { header: 模板 }` + `- --generate-notes`？goreleaser 自己产 notes（changelog from commits）；结构化 = header 里给 quickstart 三行 + 资产说明 + 已知限制（stub 模式/Windows CGO）。模板写死在 .goreleaser.yaml。
- **D-51-04 — workflow = `.github/workflows/release.yml`**：`on: push: tags: ['v*']`；`goreleaser/goreleaser-action@v6`；权限 `contents: write`（goreleaser 建 release 需要；这是唯一一个 write 权限 workflow，与 D-26/D-29 的 ci.yml 最小权限约定**冲突点已知**，eval-gate.yml 已是先例？——不，eval-gate 只读。处理：release.yml 单独文件，permissions 只在该文件里 write，README/SECURITY 不承诺"全仓零 write"）；Windows 构建步骤 = msys2/action 安装 mingw-w64 gcc + env CGO_ENABLED=1 + `goreleaser build --single-target`？——goreleaser 的多平台跨 OS 构建不能单 job 完成 ⇒ **两 job（linux / windows）+ `--split`/merge？v2 的 builds 输出到 dist，release job 用 `goreleaser release` 直接跑在 matrix 里会建两个 release** ⇒ 正解：**goreleaser `--split` + `goreleaser continue`（v2 的分体发布）**，或简化：两个 job 各自 `goreleaser build` 产物上传 artifact，最后 release job 用 softprops/action-gh-release 汇总（绕开 goreleaser 的 release 步骤，release notes 用 gh api）。**采纳简化版**：build(linux)/build(windows)/release 三 job，release job 汇总 artifacts + 生成 notes（gh release create --generate-notes + body 模板）。
- **D-51-05 — 本地验证边界**：`goreleaser check`（配置语法）+ `goreleaser build --snapshot --clean`（linux 目标本地可跑；windows CGO 本地宿主链接必败——**在 WSL 里跑 linux 目标**）＝verify-phase-51.sh 的机械部分。真实 tag push 是 manual-only（H-51-1），在 RELEASE NOTES/README 记录。
- **D-51-06 — SC3（下载的二进制可跑 quickstart）**：无法在本地全真验证（要真 release 资产）。诚实降级：verify 里验证"构建产物在本机可启动到 /api/skills 200"（用 goreleaser build --snapshot 的产物 + 真实 embedding 端点 + 无 key = stub ⇒ 与 Phase 49 UAT 测试 1 同路径）；真实"从 Release 页下载"路径记 manual-only（H-51-2）。
- **D-51-07 — README 发布段**：README 增加"## Releases"小节（下载链接模板、哪个平台哪个资产、CGO 说明、或自建 quickstart）——与 Phase 50 的"README 与实况一致"闭环。
- **D-51-08 — Claude's Discretion**：.goreleaser.yaml 内部字段组织、workflow 内部步骤顺序、verify 分区命名、notes 措辞。

## Deferred

- macOS/arm64 资产（osxcross + CGO 复杂度，等有真实需求再说）。
- Docker 镜像发布（compose 路径已有，镜像发布未承诺）。
</decisions>

## <specifics>

- tag 触发词缀：`v*`（v1.7.0 是首个）。
- 二进制命名：`skillforge-server-<os>-<arch>`（goreleaser name_template）。
- release.yml 的 permissions：`contents: write` 仅 release job；build jobs `contents: read`。
- goreleaser 版本：v2（goreleaser-action@v6 默认）。
</specifics>

## <canonical_refs>

- ROADMAP.md § Phase 51（goal + 4 SC）
- .planning/REQUIREMENTS.md REL-03（tag 触发 GitHub Release：goreleaser 配置 + release notes）
- .github/workflows/ci.yml（权限与 runner 先例；D-26/D-29 约定）
- cmd/server/main.go（唯一发布入口；env 三件套）
- Makefile（verify-phase-* 目标惯例）
- README.md（Releases 小节落点）
</canonical_refs>

## <code_context>

- 现状：无 .goreleaser.yaml、无 release workflow、无 Releases 小节。
- ci.yml Go job：CGO_ENABLED=1 + golangci + test 先例（ubuntu-latest）。
- verify-phase-49.sh：真实端点 + stub 模式启动验证先例（SC3 的本地降级路径复用）。
</code_context>
