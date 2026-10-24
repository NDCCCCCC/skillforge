---
gsd_state_version: "1.0"
milestone: v1.7
milestone_name: Open-Source Readiness
current_phase: 51
current_phase_name: 发布工程
status: planning
stopped_at: Phase 50 complete, ready to plan Phase 51
last_updated: "2026-10-23T18:19:16.579Z"
last_activity: 2026-10-24
last_activity_desc: Phase 50 complete, transitioned to Phase 51
state_head: 00470fe3a3dbd1e09c55d2425b0a9327718e712e
progress:
  total_phases: 6
  completed_phases: 4
  total_plans: 45
  completed_plans: 44
  percent: 80
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-10-13)

**Core value:** 同一个 agent 内核，通过可插拔的 Skill 文件被任意企业快速定制，无需改代码
**Current focus:** Phase 49 — Hermetic Demo

## Current Milestone

**v1.7 Open-Source Readiness** — 外部用户 clone→跑通→看懂→信任。
Phases: 47 清债基线 → (48 CI 矩阵 ∥ 49 Hermetic Demo) → 50 开源门面 → 51 发布工程
Coverage: 13/13 REQ（FACE×3 / CICD×3 / DEMO×3 / REL×4）

## Current Position

Phase: 51 — 发布工程
Plan: Not started
Status: Ready to plan
Last activity: 2026-10-24 — Phase 50 complete, transitioned to Phase 51

Progress: [████████░░] 80%

## Performance Metrics

**Velocity:**

- Total plans completed: 42 (v1.7, Phases 47–48)
- Average duration: —
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 47 清债与可复现基线 | 8 | — | — |
| 48 CI 矩阵 | 27 | — | — |
| 49 | 4 | - | - |
| 50 | 3 | - | - |

*Updated after each plan completion*
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 48 P01 | 13min | 4 tasks | 4 files |
| Phase 48 P03 | 2min | 3 tasks | 2 files |
| Phase 48 P02 | 5 min | 3 tasks | 2 files |
| Phase 48 P04 | 16min | 4 tasks | 1 files |
| Phase 48 P05 | 6 min | 3 tasks | 1 files |
| Phase 48 P06 | 3 min | 2 tasks | 1 files |
| Phase 48 P07 | 13 min | 3 tasks | 2 files |
| Phase 48 P08 | 7 min | 2 tasks | 2 files |
| Phase 48 P11 | 8 min | 3 tasks | 4 files |
| Phase 48 P10 | 12 min | 3 tasks | 4 files |
| Phase 48 P12 | 10min | 3 tasks | 2 files |
| Phase 48 P13 | 16 min | 3 tasks | 1 files |
| Phase 48 P14 | 5 min | 2 tasks | 3 files |
| Phase 48 P15 | 53 min | 3 tasks | 7 files (blocked-branch: 4 ci.yml runs on the public runner; 7 planning files reconciled; WR-06 closes on run-3) |
| Phase 48 P48-20 | 13 min | 2 tasks | 1 files |
| Phase 48 P48-21 | 16 min | 3 tasks | 1 files |
| Phase 48 P48-22 | 17 min | 3 tasks | 4 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
**历史 phase 决策（v1.4–v1.6，64 条，含 Phase 39/40/41 planner-pinned）**: `.planning/reports/v1.4-v1.6-DECISIONS-ARCHIVE.md` § Decisions —— 该清单原在本 STATE.md 中，被 v1.7 roadmap 会话的紧凑重写删除前未迁移，2026-10-19 归档保存。
Recent decisions affecting current work:

- **v1.7 roadmap（2026-10-13）**: REL 拆两处——清债（REL-01/02/04）前置 Phase 47，goreleaser（REL-03）收官 Phase 51；flaky 修复必须先于 CI 全量门与 hermetic 套件（否则闪红）
- **CI 约束**: live 门禁不进 CI（eval-gate.yml frozen-replay 是既有 hermetic 先例）；ubuntu runner 原生 gcc = WSL CGO_ENABLED=1 惯例的 CI 等价物
- **REL-04 对象**: v1.7 Traceability 100% 填充（roadmap 时已填）+ v1.6 遗留警告处置记录；不改 `.planning/milestones/` 归档
- **文档与构建现实一致**: CGO_ENABLED=1 是 go-duckdb 生产构建现实路径，README/goreleaser 表述必须如实（v1.4 前的"无 cgo 单 binary"标准已被现实路径取代）
- **Phase 46 遗留（D-45/46-01..04）**: severity 归位 shapes 数据 + ShapesPathsFromEnv 单点（详见 PROJECT.md Key Decisions）
- [Phase 48]: P3 比较 go job permissions map 用「精确相等」而非包含：一个多余的 scope 和一个缺失的 scope 同样是 scope 变更（D-29）
- [Phase 48]: P2/P3 共用一个 Python 探针判定：两个探针读同一个文件可能给出不一致结论，而两份「工作流没问题」的断言之间的分歧无法与「工作流确实没问题」区分
- [Phase 48]: verify-phase-48.sh 在运行时拼接被禁的 CI 关键字：校验脚本自身源码若写出该关键字，全仓 grep 就无法区分工作流文本与校验器文本；ALL PASSED 与 LINT NOT VERIFIED 各只出现一次同理
- [Phase 48]: Phase 48-03: fork/Dependabot exclusion is a job-level if: — the on: trigger block stays byte-identical, so no pull_request_target can slip in (D-09/D-11).
- [Phase 48]: Phase 48-03: eval-gate-skip-notice runs on the complement and emits ::notice::; the three secret mappings stay asserted, so hardening cannot silently turn the live gate into a non-live one (D-10).
- [Phase 48]: Phase 48-03: ROADMAP SC-4 now states --frozen bootstraps BGE-M3 / Milvus / GLM before scoring; only --gate-check is hermetic (D-12/D-13).
- [Phase 48]: 48-02: the fe and docker jobs inherit the workflow-level contents: read ceiling — neither declares job-level permissions, so the mutable-scope surface stays exactly the go job's pull-requests: read.
- [Phase 48]: 48-02: the forbidden 'docker compose' literal is kept out of ci.yml even in prose, so a raw-text ban in P7/P8 can distinguish the workflow from commentary about it.
- [Phase 48]: 48-02: Docker is statically asserted only on this host (absent from Git Bash and WSL); CICD-03 closes on the first real CI run (VALIDATION M1), not on a local build claim.
- [Phase 48]: Branch protection required status checks: Option A — leave protection off. The three CI jobs run and are publicly visible; enforcement is deferred until they have been stable for a few merges, and the absence of enforcement is a recorded decision rather than an oversight.
- [Phase 48]: 48-05: the secret allowlist is explicit and named — secrets.GITHUB_TOKEN anywhere; GLM_API_KEY, BGE_M3_URL and MILVUS_ADDRESS only in eval-gate.yml.
- [Phase 48]: 48-05: read-all stays rejected — D-29 enumerates the ceiling as exactly {contents: read, pull-requests: read}; accepting read-all would be a D-29 amendment first and a code change second.
- [Phase 48]: 48-05: every mutation probe writes under a trap and asserts git status --porcelain -- .github/workflows is empty, so a leaked fixture fails the probe rather than passing it.
- [Phase 48]: 48-06: keep `set -o pipefail` in the replay run body even though `shell: bash` implies it, so the exit-code intent survives a later removal of the shell key (WR-03).
- [Phase 48]: 48-06: live credentials live on the two consuming steps (replay + gate check), not job-level env; job-level keeps configuration only (WR-08).
- [Phase 48]: 48-06: no waiver token added — zero continue-on-error, `|| true`, or `exit 0`; the phase gate stays fully enforcing (T-48-28).
- [Phase 48]: 48-07: a step-level `if:` is rejected by key presence on every go/fe/docker gate step - no allowlist; false, always() and success() all decide whether the gate runs.
- [Phase 48]: 48-07: the count rule constrains the gap between the count word and the number, not the digits; a loose newline-bounded window matches the file own `go test ./... (CICD-01 / D-15)` step name.
- [Phase 48]: 48-07: persist-credentials: false is scoped to the go checkout (the only job with an extra scope), and P3 asserts the parsed value is exactly False.
- [Phase 48]: 48-07: CICD-01/02/03 remain unmarked - hardening the verifier does not substitute for the first real GitHub Actions run owned by 48-11.
- [Phase 48]: 48-08: P9 compares the whitespace-normalized eval-gate if: to the exact D-09 literal with no substring fallback, so || true, outer negation and clause removal all fail.
- [Phase 48]: 48-08: the skip notice is asserted structurally as notice == '!(' + gate + ')', so the two expressions cannot drift apart.
- [Phase 48]: 48-11: WR-06 = option D (decide from the first real run evidence) — The push-path lint outcome is unmeasured (golangci-lint-action could not be executed on a push event locally); whole-repo config measured at 45 findings on this tree. Both candidate fixes are recorded (A: args --new-from-rev diff base; B: PR-only lint step = a D-06 amendment, not chosen); no lint-step code change was made.
- [Phase 48]: 48-11: IN-08 = option F (document the closed pull_request base set [main, phase-*] in the ci.yml header next to invariant 1) — No trigger behavior change; the Pending-forever hazard for PRs based outside the closed set is now a documented decision, and widening it is a deliberate header+trigger edit, never a silent filter drop.
- [Phase 48]: 48-11: IN-07 = option A (new independent verify job running bash verify-phase-48.sh in CI) — Checkout first, composite with go true, pinned golangci-lint v2.12.2 via the official install script, PyYAML, then the gate script. No job-level permissions (inherits contents: read ceiling), no step-level if, no continue-on-error. The lint action is deliberately not reused so the verify job never inherits the push-run whole-repo fallback.
- [Phase 48]: 48-11: M1 = BLOCKED (historical entry — the parenthetical cause "no git transport to github.com from this host" was FALSIFIED by 48-VERIFICATION.md round 2: transport succeeds and git ls-remote exits 0; origin/main unrelated history, merge-base empty; ci.yml was never on the remote default branch at the time) — see the 48-15 entries below for the history decision and runs 1-5; CICD-01/02/03 stayed Pending honestly. The full run recipe (history decision, transport-capable host push, gate-preserving trigger, gh run watch, red-run classification, report format) is persisted in 48-11-SUMMARY.md for a future host, and the deferral handoff to plan 48-10 (48-SECURITY.md What this audit does NOT establish) is recorded there.
- [Phase 48]: Phase 48 register re-closed on mutation evidence: threats_open 9->0, status verified; all nine falsified-hold rows cite the 48-05..48-09 assertion plus the mutation that proves it — Every mutation re-closure cites a named partition assertion plus its mutation probe; string-absence alone no longer closes a row
- [Phase 48]: Review dispositions final: 18 fixed, 1 deferred (WR-06 option D — push-path lint scope unmeasured, decision re-opens on the first real ci.yml run) — IN-07/IN-08 recorded as implemented (48-11 options A/F), not deferred; CICD-01/02/03 stay Pending on M1 BLOCKED
- [Phase 48]: 48-12: R2-CR-01 closed with the module-proxy install shape (go install @v2.12.2 + [ -x ] postcondition), not SHA-pinned install.sh — single command means no pipe and R2-WR-02 closes by construction, so no pipefail is cargo-culted in
- [Phase 48]: 48-12: P5a determinism guard (-list | grep -qx) runs under shell: bash + explicit pipefail because GitHub default bash -e {0} has none; both P2-pinned fragments stay byte-present (D-18/D-19)
- [Phase 48]: 48-12: Gate check documented as the offline threshold comparison holding zero secret surface — cmd/eval --gate-check short-circuits before bootstrap (main.go:463-467); the three live mappings stay on the replay step only (R2-WR-05)
- [Phase 48]: 48-13: P11's registry count runs LAST (banner checks -> fixtures -> P11 token -> four-way count); the count demands P11's own success token, so the reverse order would be a self-failing gate
- [Phase 48]: 48-13: no :- default on a variable whose setter is the probe under test — TRUST_SURFACE_COUNT aborts under set -u instead of printing a plausible zero (M8 now bites); success tokens deposit only after each probe's exit-code assertion, and done_partition requires begin + ok
- [Phase 48]: 48-13: the verify job is pinned by the same P2/P3 probe that judges the go job (existence, if-freeness, gate terminus, persist-credentials false, module-proxy install); the banned fetch/script tokens are printf-assembled argv, never spelled joined — M9/M10 and both 48-12-fix reintroductions now red with the partition named
- [Phase 48]: 48-13: CICD-01/02/03 stay Pending — hardening the verifier does not substitute for the first real GitHub Actions run (48-15); requirements mark-complete deliberately not run
- [Phase 48]: 48-14: supply-chain surface re-recorded as closed register row T-48-47 (Tampering/ci.yml verify job install step/critical/mitigate) citing 48-12's module-proxy install + 48-13's P2/P3 pin and reintroduction drill — 48-11-SUMMARY's threat_flag: supply-chain finally lands; register 21 rows, 0 open, status verified
- [Phase 48]: 48-14: transport-truth refresh in 48-SECURITY.md + ROADMAP Blocked-on — git ls-remote origin HEAD succeeds (exit 0, 369b08a); remaining path is the origin/main unrelated-history decision + one push from this host, owned by 48-15; CICD-01/02/03 deliberately stay Pending (no run exists)
- [Phase 48]: 48-15: history decision recorded with fresh evidence — option a force-push to main, with v1.5-archive tag preservation (369b08a); plan 48-11's no-transport cause already falsified by 48-VERIFICATION round 2 — full recipe step 1 captured in 48-15-SUMMARY.md
- [Phase 48]: 48-15: ci.yml ran six times on public main (runs 1-5 red; run 6 of 6 four-green per the 48-19 entry below) — run-1 (head d168512, id 39350901689), run-2 (head 62b6663 recipe candidate A, id 39352480525), run-3 (head a2a4c86 recipe-revert, id 39353593172), run-4 (head 304dcc4 final reconciliation commit, id 39355131431), run-5 (head e161436, id 39378733451), run-6 (head 630f875, id 39380373930, event push); at the time of 48-15 the outcome was `Run outcome: blocked` — three classified persistent real reds (go P5a is a workflow pipeline shape: `go test -list | grep -qx` under `set -o pipefail` closes the pipe at the first match, the producer takes SIGPIPE and exits 141 — fixed by 48-16 materialising the list to $RUNNER_TEMP; fe Sparkline path hardcoded to a Windows absolute path, docker Dockerfile.backend alpine-musl missing glibc symbols backtrace/malloc_trim/__res_init required by static duckdb); WR-06 closes positively on run-3 evidence (lint step green with only-new-issues: true alone, fast-forward push); CICD-01/02/03 stay Pending per the recipe blocked-branch rule (do NONE of the flips unless the run-level conclusion is success)
- [Phase 48]: 48-15: WR-06 reconciles from `option D / deferred` (48-11) to `closed-by-evidence on run-3` (48-15) in both disposition ledgers; candidate A (`args: --new-from-rev` on the lint step) was tested in run-2 and rejected as incompatible with `only-new-issues: true`; no lint-step code change applied to production; cumulative workflow diff vs the pre-plan BASE_SHA = 0
- [Phase 48]: 48-15: append to 48-11's `M1 = BLOCKED` (line 116) — M1 was reclassified to M1 partial by plan 48-15; the runner did execute, but M1 in the strict all-three-jobs-green sense stays open because three new M-class rows (M1a / M1b / M1c) document the three classified persistent real reds that prevent the four-green run conclusion; full close requires source-side fixes for the Sparkline test path, Dockerfile.backend base image, and the P5a SIGPIPE capture shape
- [Phase 48]: 48-15: append to 48-11's `Review dispositions final` (line 118) — WR-06 advances from `deferred` to `fixed — closed-by-evidence on run-3`; M2 / M3 / M4 / M5 unchanged except M4 closed by 48-15 evidence (only-new-issues works on a fast-forward push); CICD-01/02/03 do not advance in this branch per the recipe blocked-branch "do NONE of the flips" rule
- [Phase 48]: 48-19: ci.yml run 39380373930 (head 630f875, 2026-10-20) is the first four-green run — `go=success`, `fe=success`, `docker=success`, `verify=success`; derived total 6 (runs 1-5 red). CICD-01/02/03 flip to Complete in both REQUIREMENTS.md places against that run id, and ROADMAP/VALIDATION/SECURITY/STATE carry the canonical fragment `ci.yml total runs: 6; first all-green: 39380373930`. The three classified build reds were cleared by 48-16 (materialised `-list` output, module-relative Sparkline path, glibc backend image); the run-5 red was a load-sensitive DuckDB reader-block ratio flake, fixed at the source with best-of-5 minimum sampling (50 readers and the 3.00x threshold unchanged)
- [Phase 48]: 48-20: ok_partition now takes one-or-more required per-assertion-group tokens and reports every missing token at once — a partition's PASS deposit requires every one of its own assertion groups to have run (WR-03/M13 closed for P1/P2/P3/P4) — P3-STEP-IF-OK stays attached to the go-job step-level if: loop per the plan must_haves, so the P3-section deletion drill reds on P3-BLOCKING-OK/P3-LINT-OK; drills grep the output for a specific token, so ok_partition quotes the full required set
- [Phase 48]: P6/P7/P9 probes emit one success token per assertion group (4/6/6 tokens) and ok_partition requires the full list at deposit — WR-03 is closed phase-wide; deleting any single assertion group reds the gate naming the partition and the missing token — A single per-probe token let an assertion group be deleted while its sibling's evidence still deposited PASS; per-group tokens bind PASS to each group's own evidence, and the sandbox drills prove every deletion exits 1 with the partition named
- [Phase 48]: The P11 else-detector captures the printed gate-to-banner range into BANNER_RANGE before a non-quiet grep reads it to EOF, removing the SIGPIPE false negative (WR-01 / verification advisory 1) — Under set -o pipefail a quiet grep -q closes the pipe at its first match; if the range outgrew the pipe buffer the producer would take EPIPE and read as 'no else found' exactly when a banner had been moved after an else
- [Phase 48]: 48-22: both parsed trust-surface walks layer OVER the retained textual rules (not replacements) - the pre-existing two-files/stray-secret fixtures stay rejected with a walk deleted, so each deletion drill's fixture loop reaches the first fixture only the deleted walk was catching (quoted-paths resp. tojson-secrets) and attributes exactly
- [Phase 48]: 48-22: drill markers for the parsed walks sit at column 0 inside the indented Python probe - comment-only lines do not participate in Python indentation, so the markers are valid probe syntax and startswith-matchable range delimiters; indented markers delete nothing (first drill attempt proved it)
- [Phase 48]: 48-22: requirements mark-complete deliberately not run - ready-ids returned all three but REQUIREMENTS.md already carries Complete (run 39380373930) in both places and the plan forbids requirement-status changes; marking would overwrite the canonical run-id status text (plan's expected no-op)
- [Phase 48]: 48-26: round-6 Class-A run obtained — the round-6 tree (head `c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67`, carrying 48-23..48-25's gate and record work) was pushed to public main as a fast-forward (`630f875..c060c84`, no force, HTTPS origin transport) and run 7 of 7 (id `39525757252`, event `push`, 2026-10-21T01:10:28Z→01:13:50Z) concluded `go=fe=docker=verify=success` — the first public run of the round-6 gate, closing the round-5 local-green-only record boundary; go/fe/docker job spans are byte-identical between `630f875` and `c060c84` (ci.yml delta = push-trigger comment block + verify-job header comment block + verify install-step declared env map), so CICD-01/02/03 stay Complete (run 39380373930) without re-flipping and REQUIREMENTS.md is untouched; canonical fragment: `ci.yml total runs: 7; first all-green: 39380373930; run 7 (head c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67, id 39525757252, event push) is the first public run of the round-6 gate (four-green: go=success fe=success docker=success verify=success)` — total derived live from `gh run list`, M2/M3/M5 stay open by design

### Pending Todos

None yet.

### Blockers/Concerns

- **内网服务间歇断**（Milvus/BGE/GLM）: live verify 沿用"探测先行、断线记 env-SKIP"惯例；v1.7 的 hermetic 化正是为降低此依赖
- **Phase 49 是最大工作量 phase**: 真实代码改动（embedded 向量兜底 + stub LLM chat），建议 plan-phase 时重点拆分
- **【A2 → Phase 48 必读】`verify-phase-47.sh` 前端分区被跳过时仍 exit 0**: 输出已无歧义（`PARTIAL PASS — FRONTEND PARTITION NOT VERIFIED`、P6 不在已验证列表、零 `ALL PASSED`），但退出码仍是 0。Phase 48 的 CI **不得**只看退出码，必须同时 grep `PARTIAL PASS`。runner 上没有 node 的那一刻就会踩到。
- **【A3】P4 覆盖 `./internal/...`，但仓库另有 18 个已跟踪测试文件在其外**: `go test ./...` 实测 exit 0（8 包全绿），所以**当前没有任何东西被藏起来**；但这是偶然而非结构性保证——明天在 `cmd/` 下加一个测试，它会被 CI 跑却不被本门覆盖。非 SC-4 违规（SC-4 字面定义 P4 为 `./internal/...`）。
- **MEDIUM-2（已裁定接受，`graph_edges.go:232-241`）**: 边方向已确定化，但 `keyword_cooccur` 边的**挂载**仍取 `docs` 的首命中 per-source 代表 chunk；Milvus `ListMetadata` 返回行序未定义且不去重 → 同一语料可能挂到不同节点。验证者复核确认这是**输入置换敏感**（不是固定切片上的跑间不确定），因此 `-count=100`/`-shuffle=on` 门的作用域是诚实的。
- **LOW-1（已裁定接受，`GovernanceTab.tsx:167`）**: REL-02 的 FE 连线本身无直测（删掉 prop 三个新测试照样过）。
- **D-05 已知限制（明确接受，非隐式修复）**: 启动校验会 quarantine 坏实体、rollback 不会，故回滚确认框与横幅的 `N 个实体已隔离` 都夸大了 rollback 的实际行为。三处显式记录，从未作为已修复呈现。
- **既有 flake（非 Phase 47 引入，`audit_writer_test.go:59`）**: `TestAuditWriterPreconditions` 对文件型 SQLite store 开库时套 5s `context.WithTimeout`，CPU 超卖下会超时。串行全量 8/8 绿（Phase 47 验证者自行复跑 3/3 绿），8 份并发则全红。`git log -S` 定源于 `9613d4d`（2026-08-20, Phase 11）。Phase 48 每线单 job 不会触发。
- **CICD-01/02/03 status as of plan 48-19 (2026-10-20): Complete.** Runner executed ci.yml six times on the public default branch (runs 1-5 red; run 6 of 6 — id 39380373930 / head `630f875` — concluded `go=fe=docker=verify=success`, event `push`). Canonical fragment: `ci.yml total runs: 6; first all-green: 39380373930`. The earlier classified reds were cleared at the source: the go job's P5a SIGPIPE (48-16 materialised the `-list` output), the fe job's Sparkline Windows path (48-16 module-relative resolution), and the docker alpine-musl duckdb linkage (48-16 glibc builder/runtime); the run-5 red was a load-sensitive DuckDB reader-block ratio flake fixed by 48-19 (best-of-5 minimum sampling). WR-06 closes positively on run-3 evidence (lint step green with `only-new-issues: true` alone on the fast-forward push). M2-M5 remain open by design. **Round-6 update (2026-10-21, plan 48-26):** runner has now executed ci.yml seven times — run 7 of 7 (id `39525757252`, head `c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67`, fast-forward push `630f875..c060c84`) is four-green and is the first public run of the round-6 gate (the 48-23/48-24 verifier hardening and 48-25 record corrections are runner-executed); go/fe/docker job spans are byte-identical `630f875..c060c84`, so the Complete statuses stand against run 39380373930 without re-flipping. Canonical fragment: `ci.yml total runs: 7; first all-green: 39380373930`. M2/M3/M5 remain open by design.

## Deferred Items

Items acknowledged and carried forward from previous milestone close.

**本表是索引，不是权威记录（D-03c）。** 每个遗留项的处置与证据的唯一权威来源是
[`.planning/phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md`](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md)；
下表每行一个 ID、状态格内链回该主表，不复制证据。本表曾在本 STATE.md 的紧凑重写中丢过一次内容，故索引必须保持可解析（每 ID 一行，不合并成区间行）。

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| tech_debt | D-46-DEF-02 `TestDeriveEdgesDeterministic` flaky | `v1.7 修复`（Phase 47 / REL-01，已闭合；根因 = 边方向 map 迭代序，非浮点 Weight）— 证据见 [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-46-DEF-02` | v1.6 |
| tech_debt | WARNING-1 rollback SHACL 缓存刷新缺口 | `v1.7 修复`（Phase 47 / REL-02，已闭合；BE `e7ca19a` + FE `197421e`/`bf1c1e0`）— 证据见 [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `WARNING-1` | v1.6 |
| tech_debt | BL-01 双 resolver 去重（`resolveSkillTiers` vs `resolveSkillTiersFromKG`） | `延期+理由`（带行为语义的重构，无当前缺陷，须单独立项）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `BL-01` | v1.6 |
| tech_debt | BL-02 server 包 lint 清扫 | `延期+理由`（需 Phase 48 的 `golangci-lint` 报告才有可定义范围）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `BL-02` | v1.6 |
| tech_debt | BL-03 live 抽取 prompt 对齐 writes-only 的残余微调 | `延期+理由`（needs-live 内网 LLM/BGE）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `BL-03` | v1.6 |
| tech_debt | BL-04 DetailPanel / emitter 两个 deferred 测试 | `v1.7 修复`（Phase 47 已闭合：emitter = plans 47-03，DetailPanel = plan 47-05）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `BL-04` | v1.6 |
| tech_debt | D-45-DEF-01 emitter `TestEmitSubCategoryInstanceCount` 6 vs 7 | `v1.7 修复`（Phase 47 已闭合；修断言非实现，`emitter.go` 未改）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-45-DEF-01` | v1.6 |
| tech_debt | D-45-DEF-02 未跟踪的 runtime-stale `verify_main.go` | `v1.7 修复`（清理工作树由 plan 47-08 执行；harness 重写未立项，因无消费者）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-45-DEF-02` | v1.6 |
| tech_debt | D-45-DEF-03 45-REVIEW 的 WR-01 / WR-02 / IN-02 | `延期+理由`（count 语义变更、fail-open 姿态对齐、IRI 族对齐，各需独立决策记录）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-45-DEF-03` | v1.6 |
| tech_debt | D-46-DEF-01 DetailPanel SC-5 空态 hero 测试确定性失败 | `v1.7 修复`（Phase 47 已闭合；断言重定向到「文档」mode，生产组件未改）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-46-DEF-01` | v1.6 |
| tech_debt | WARNING-2 `owl:Class`-vs-`skf:Class` 声明 quirk（skill 视图 tier-invisible） | `延期+理由`（schemaClasses 可见性语义决策，影响面覆盖两条路径；admin baseline 不受影响）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `WARNING-2` | v1.6 |
| known_limit | D-05 rollback 不执行 quarantine → banner「N 个实体已隔离」在回滚路径失真 | `延期+理由`（已知限制，本 phase 明确不修；修它会改变 rollback 副作用面）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-05` | Phase 47 |
| future | 身份体系/RBAC、多租户、OTel、OpenAPI、rate limiting | v1.8+ 商业分期 | v1.7 规划 |

## Session Continuity

Last session: 2026-10-22T16:14:06.999Z
Stopped at: Phase 50 complete, ready to plan Phase 51
Resume file: .planning/phases/49-hermetic-demo/49-VERIFICATION.md
Resume command: `/gsd-plan-phase 49` — Phase 48 (CI 矩阵) is complete: 27/27 plans executed, round-7 verification `passed` (15/15 must-haves, `48-VERIFICATION.md`, coverage set 69 files, digest `v3:sha256:c3174dca2179d21c0bd1c0a5c648145db6a8e3b42b6b850b34507e46c6b01e16`), the 37-threat security sheet has 0 open, and the public four-green run 39380373930 backstops CICD-01/02/03. Phase 49 (Hermetic Demo) is next — start with its context (`/gsd-discuss-phase 49`). Round-8 candidates already recorded in ROADMAP.md「Round-7 review 遗留的 6 条 open 发现」: the workflow-level `permissions:` mapping-value branch still has no isolated fixture, the quote-aware stripper's quote-state has no canary, the P4 byte count is measured but not asserted, and the secret matcher still misses spaced/bracket forms.
