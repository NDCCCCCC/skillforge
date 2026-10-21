---
phase: 48-ci
verified: 2026-10-21T09:51:51Z
status: passed
score: 15/15 must-haves verified
covered_files:
  - .github/actions/setup/action.yml
  - .github/workflows/ci.yml
  - .github/workflows/eval-gate.yml
  - .golangci.yml
  - .planning/REQUIREMENTS.md
  - .planning/ROADMAP.md
  - .planning/STATE.md
  - .planning/phases/48-ci/48-01-PLAN.md
  - .planning/phases/48-ci/48-01-SUMMARY.md
  - .planning/phases/48-ci/48-02-PLAN.md
  - .planning/phases/48-ci/48-02-SUMMARY.md
  - .planning/phases/48-ci/48-03-PLAN.md
  - .planning/phases/48-ci/48-03-SUMMARY.md
  - .planning/phases/48-ci/48-04-PLAN.md
  - .planning/phases/48-ci/48-04-SUMMARY.md
  - .planning/phases/48-ci/48-05-PLAN.md
  - .planning/phases/48-ci/48-05-SUMMARY.md
  - .planning/phases/48-ci/48-06-PLAN.md
  - .planning/phases/48-ci/48-06-SUMMARY.md
  - .planning/phases/48-ci/48-07-PLAN.md
  - .planning/phases/48-ci/48-07-SUMMARY.md
  - .planning/phases/48-ci/48-08-PLAN.md
  - .planning/phases/48-ci/48-08-SUMMARY.md
  - .planning/phases/48-ci/48-09-PLAN.md
  - .planning/phases/48-ci/48-09-SUMMARY.md
  - .planning/phases/48-ci/48-10-PLAN.md
  - .planning/phases/48-ci/48-10-SUMMARY.md
  - .planning/phases/48-ci/48-11-PLAN.md
  - .planning/phases/48-ci/48-11-SUMMARY.md
  - .planning/phases/48-ci/48-12-PLAN.md
  - .planning/phases/48-ci/48-12-SUMMARY.md
  - .planning/phases/48-ci/48-13-PLAN.md
  - .planning/phases/48-ci/48-13-SUMMARY.md
  - .planning/phases/48-ci/48-14-PLAN.md
  - .planning/phases/48-ci/48-14-SUMMARY.md
  - .planning/phases/48-ci/48-15-PLAN.md
  - .planning/phases/48-ci/48-15-SUMMARY.md
  - .planning/phases/48-ci/48-16-PLAN.md
  - .planning/phases/48-ci/48-16-SUMMARY.md
  - .planning/phases/48-ci/48-17-PLAN.md
  - .planning/phases/48-ci/48-17-SUMMARY.md
  - .planning/phases/48-ci/48-18-PLAN.md
  - .planning/phases/48-ci/48-18-SUMMARY.md
  - .planning/phases/48-ci/48-19-PLAN.md
  - .planning/phases/48-ci/48-19-SUMMARY.md
  - .planning/phases/48-ci/48-20-PLAN.md
  - .planning/phases/48-ci/48-20-SUMMARY.md
  - .planning/phases/48-ci/48-21-PLAN.md
  - .planning/phases/48-ci/48-21-SUMMARY.md
  - .planning/phases/48-ci/48-22-PLAN.md
  - .planning/phases/48-ci/48-22-SUMMARY.md
  - .planning/phases/48-ci/48-23-PLAN.md
  - .planning/phases/48-ci/48-23-SUMMARY.md
  - .planning/phases/48-ci/48-24-PLAN.md
  - .planning/phases/48-ci/48-24-SUMMARY.md
  - .planning/phases/48-ci/48-25-PLAN.md
  - .planning/phases/48-ci/48-25-SUMMARY.md
  - .planning/phases/48-ci/48-26-PLAN.md
  - .planning/phases/48-ci/48-26-SUMMARY.md
  - .planning/phases/48-ci/48-27-PLAN.md
  - .planning/phases/48-ci/48-27-SUMMARY.md
  - .planning/phases/48-ci/48-REVIEW-DISPOSITION.md
  - .planning/phases/48-ci/48-REVIEW.md
  - .planning/phases/48-ci/48-SECURITY.md
  - .planning/phases/48-ci/48-VALIDATION.md
  - Dockerfile.backend
  - Dockerfile.frontend
  - verify-phase-48.sh
  - web/src/components/chat/parts/Sparkline.test.tsx
covered_digest: "v3:sha256:c3174dca2179d21c0bd1c0a5c648145db6a8e3b42b6b850b34507e46c6b01e16"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 14/15
  gaps_closed:
    - "truth 12 / CR-01（BLOCKER）——P1 的触发判定改为解析优先 + 单一 quote-aware 剥离器；CR-01 形态的 flow-style `pull_request_target` 现在整个门禁 exit 1 并给出 file:line"
    - "truth 12 / WR-01（BLOCKER）——三条写权限规则各配隔离 fixture 与专属 class token；删除 WRITE_VALUE 行扫描或 job-level permissions walk 均 exit 1"
    - "WR-02（advisory→fixed）——P4 config-verify 证据行改为 append 前实测字节数，健康树打印 `tool output: 0 byte(s)`"
    - "IN-02（advisory→fixed）——P1 主循环对 rc ∉ {0,1} 显式 fail，两个 probe 都必须给出决定性结果"
  gaps_remaining: []
  regressions: []
advisory:
  - finding: "当前 HEAD 的 `verify-phase-48.sh` 尚未在公开 runner 上执行过：最新公共 run `39525757252`（head `c060c84`）跑的是 round-6 版门禁（14 fixtures、无 `P1-NO-PRT-PARSED`）"
    category: other
    reason: "`.github/` 在 `c060c84..HEAD` 的 diff 为空，三条 CI 线的定义逐字节未变，因此 SC-1..SC-3 仍由同一 run 背书；新门禁在本机（Git Bash + pinned golangci-lint + PyYAML，与 CI verify job 同一套依赖）exit 0 且恰好一条 ALL PASSED。48-27 计划显式判定无需新的公共 run（三项 gap 由本地变异演练证明）。下一次 push 会由 `verify` job 实际执行新脚本"
    evidence_status: "reproduced (gh run view + git diff --stat c060c84..HEAD -- .github/ 为空 + 本地门禁 exit 0)"
  - finding: "`.planning/ROADMAP.md:180` 的 `48-27-PLAN.md` 仍是 `- [ ]`（未勾选），Phase 48 抬头仍写 `Plans: 26/26 executed`——但 48-27 已执行（4 个提交 + 48-27-SUMMARY.md）"
    category: other
    reason: "计划登记的记账滞后，不影响阶段目标（CI 三线公开绿灯）与任何 must-have；48-26 等前序计划都带了 `- [x] …… ——已完成 <date>` 的收尾登记，48-27 缺这一步"
    evidence_status: "reproduced (git log fd6f082..HEAD -- .planning/ROADMAP.md 只有 3935876 一次改动，加入时即为 `- [ ]`；此后无提交再触碰该行)"
  - finding: "IN-03（round-6 延续）——secret 匹配器仍是字面拼写 `secrets\\.([A-Za-z_][A-Za-z0-9_]*)`（verify-phase-48.sh:1421），`secrets ['X']` / `secrets . X` 两种写法绕过文本层与解析层"
    category: security
    reason: "pre-existing 的匹配器宽度问题，round-7 未触及该正则（`diff` 中 :1421 未改）；解析层只覆盖无空格的 `secrets[`。非本 delta 引入，不构成 must-have 失败"
    evidence_status: "code read (verify-phase-48.sh:1421, :1606, :1618, :1638)"
  - finding: "IN-01（round-6 延续）——双引号 publication 拼写 `push: \"true\"` 有 token（verify-phase-48.sh:1398-1416）但无专属 fixture，删掉该 token 仍会 ALL PASSED"
    category: other
    reason: "同一断言组的第二个拼写，review 早已记为 Info；本轮 WR-01 的隔离化只覆盖三条权限规则，publication 组未扩展"
    evidence_status: "code read (verify-phase-48.sh:1398-1416, :2370)"
  - finding: "IN-04（round-6 延续）——`ci.yml` push 注释的 'A FORCE push to main has no common ancestor' 过度概括（rebase 型 force push 仍有共同祖先）"
    category: other
    reason: "注释措辞，无行为影响；round-5 的 IN-R5-04 已把运行结论更正为实证表述"
    evidence_status: "code read (.github/workflows/ci.yml:41-51)"
  - finding: "`48-27-PLAN.md:189-201` 给出的 R16 演练代码字面有缺陷：按 marker `# T-48-83: end quote-aware comment strip helper` 切段会连带吞掉 heredoc 终止符 `YAMLSTRIP`（它在 :205，end marker 在 :206），变异脚本退化为静默 exit 0 无 banner"
    category: other
    reason: "计划书里的演练脚手架缺陷，不是被验证代码的缺陷；48-27-SUMMARY 记录的红因（flow-style canary 返回 rc=0）与真实代码一致，说明执行者改用了 `\\nYAMLSTRIP\\n` 锚点。后续照抄该计划演练代码的人会踩同一个坑"
    evidence_status: "reproduced (verifier 首次尝试即命中该静默绿；改用 YAMLSTRIP 锚点后 R15-R21 全部按预期落红)"
---

# Phase 48: CI 矩阵 Verification Report (Re-verification, round 7)

**Phase Goal:** 外部贡献者的每次 PR 与维护者的每次 push 都被自动化质量门守护——Go / FE / Docker 三线在 GitHub Actions 上公开绿灯，把"666 Go 测试 + 84 FE 测试"的可信度从本机 WSL 搬到公共可见的 CI。
**Verified:** 2026-10-21T09:51:51Z
**Status:** passed
**Re-verification:** Yes — after gap-closure plan 48-27 (round 7)，工作树 HEAD `8f8cd6c`
**Previous report:** round 6（`gaps_found`, 14/15）正文另存为 `48-VERIFICATION-R6.md`，未删改。

---

## 覆盖集指纹（由 verifier 亲自跑，逐字抄录）

命令：

```bash
gsd_run query verification.fingerprint ".planning/phases/48-ci" <COVERED_FILES…>
```

输出：

```json
{
  "covered_files": [
    ".github/actions/setup/action.yml",
    ".github/workflows/ci.yml",
    ".github/workflows/eval-gate.yml",
    ".golangci.yml",
    ".planning/REQUIREMENTS.md",
    ".planning/ROADMAP.md",
    ".planning/STATE.md",
    ".planning/phases/48-ci/48-01-PLAN.md",
    ".planning/phases/48-ci/48-01-SUMMARY.md",
    ".planning/phases/48-ci/48-02-PLAN.md",
    ".planning/phases/48-ci/48-02-SUMMARY.md",
    ".planning/phases/48-ci/48-03-PLAN.md",
    ".planning/phases/48-ci/48-03-SUMMARY.md",
    ".planning/phases/48-ci/48-04-PLAN.md",
    ".planning/phases/48-ci/48-04-SUMMARY.md",
    ".planning/phases/48-ci/48-05-PLAN.md",
    ".planning/phases/48-ci/48-05-SUMMARY.md",
    ".planning/phases/48-ci/48-06-PLAN.md",
    ".planning/phases/48-ci/48-06-SUMMARY.md",
    ".planning/phases/48-ci/48-07-PLAN.md",
    ".planning/phases/48-ci/48-07-SUMMARY.md",
    ".planning/phases/48-ci/48-08-PLAN.md",
    ".planning/phases/48-ci/48-08-SUMMARY.md",
    ".planning/phases/48-ci/48-09-PLAN.md",
    ".planning/phases/48-ci/48-09-SUMMARY.md",
    ".planning/phases/48-ci/48-10-PLAN.md",
    ".planning/phases/48-ci/48-10-SUMMARY.md",
    ".planning/phases/48-ci/48-11-PLAN.md",
    ".planning/phases/48-ci/48-11-SUMMARY.md",
    ".planning/phases/48-ci/48-12-PLAN.md",
    ".planning/phases/48-ci/48-12-SUMMARY.md",
    ".planning/phases/48-ci/48-13-PLAN.md",
    ".planning/phases/48-ci/48-13-SUMMARY.md",
    ".planning/phases/48-ci/48-14-PLAN.md",
    ".planning/phases/48-ci/48-14-SUMMARY.md",
    ".planning/phases/48-ci/48-15-PLAN.md",
    ".planning/phases/48-ci/48-15-SUMMARY.md",
    ".planning/phases/48-ci/48-16-PLAN.md",
    ".planning/phases/48-ci/48-16-SUMMARY.md",
    ".planning/phases/48-ci/48-17-PLAN.md",
    ".planning/phases/48-ci/48-17-SUMMARY.md",
    ".planning/phases/48-ci/48-18-PLAN.md",
    ".planning/phases/48-ci/48-18-SUMMARY.md",
    ".planning/phases/48-ci/48-19-PLAN.md",
    ".planning/phases/48-ci/48-19-SUMMARY.md",
    ".planning/phases/48-ci/48-20-PLAN.md",
    ".planning/phases/48-ci/48-20-SUMMARY.md",
    ".planning/phases/48-ci/48-21-PLAN.md",
    ".planning/phases/48-ci/48-21-SUMMARY.md",
    ".planning/phases/48-ci/48-22-PLAN.md",
    ".planning/phases/48-ci/48-22-SUMMARY.md",
    ".planning/phases/48-ci/48-23-PLAN.md",
    ".planning/phases/48-ci/48-23-SUMMARY.md",
    ".planning/phases/48-ci/48-24-PLAN.md",
    ".planning/phases/48-ci/48-24-SUMMARY.md",
    ".planning/phases/48-ci/48-25-PLAN.md",
    ".planning/phases/48-ci/48-25-SUMMARY.md",
    ".planning/phases/48-ci/48-26-PLAN.md",
    ".planning/phases/48-ci/48-26-SUMMARY.md",
    ".planning/phases/48-ci/48-27-PLAN.md",
    ".planning/phases/48-ci/48-27-SUMMARY.md",
    ".planning/phases/48-ci/48-REVIEW-DISPOSITION.md",
    ".planning/phases/48-ci/48-REVIEW.md",
    ".planning/phases/48-ci/48-SECURITY.md",
    ".planning/phases/48-ci/48-VALIDATION.md",
    "Dockerfile.backend",
    "Dockerfile.frontend",
    "verify-phase-48.sh",
    "web/src/components/chat/parts/Sparkline.test.tsx"
  ],
  "covered_digest": "v3:sha256:c3174dca2179d21c0bd1c0a5c648145db6a8e3b42b6b850b34507e46c6b01e16"
}
```

> 说明：phase 目录的 27 对 PLAN/SUMMARY 由 fingerprint 工具按规则自动并入，本轮输入也显式列了它们（输出去重后 69 项）。`48-VERIFICATION.md` 自身不计入覆盖集。

---

## Goal Achievement

### 公开结果仍然成立，且本轮没有动 CI 定义

- **`gh run view 39525757252`**（head `c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67`，event `push`，2026-10-21T01:10:28Z→01:13:49Z）结论 `success`。逐 job 经 Actions API 复核：**`go=success`、`fe=success`、`docker=success`、`verify=success`**，四个 job 内**所有 step** 均 `success`（无 skipped）。
- **`gh run view 39380373930`**（head `630f8751ab65a07d71d5fa6f25fd8e13c3838ef7`，event `push`）同样四绿全 step success——这是 REQUIREMENTS.md 两条记录引用的 first all-green run。
- **CI 定义未变**：`git diff --stat c060c84..HEAD -- .github/` 输出为空；`c060c84..HEAD` 的完整 diff 只有 13 个文件——12 个 `.planning/**` 记录 + `verify-phase-48.sh`（+419/-47）。这正是 48-27 计划第 2 条 prohibition 的要求，也是"三线绿灯仍由同一次运行背书"的前提。
- **CICD-01/02/03 状态未被重翻**：`.planning/REQUIREMENTS.md:17-19` 三条 `[x]`，`:69-71` 三行均 `Complete (run 39380373930)`；`:84` Coverage 17/17 mapped items。REQUIREMENTS.md 在 `c060c84..HEAD` 的 diff 为空（不在 13 个改动文件里）。

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC-1 / CICD-01：Go 线在公开触发上运行且发布检查为绿 | ✓ VERIFIED | Run `39525757252` `go=success`，其 step 全 success：`build + vet (P1)`、`lint (changed lines)`、`go test ./... (CICD-01 / D-15)`、`determinism -count=100 (P5a)`、`determinism -shuffle=on with seed (P5b)`。 |
| 2 | SC-2 / CICD-02：FE 线在公开触发上运行且发布检查为绿 | ✓ VERIFIED | 同一 run `fe=success`，step 全 success：`install`、`typecheck`、`unit tests`。本轮本地复核 `cd web && npx vitest run` → **85 test files / 689 tests 全绿**，exit 0。 |
| 3 | SC-3 / CICD-03：两个 Dockerfile 在 CI 中构建 | ✓ VERIFIED | 同一 run `docker=success`，`build backend image` 与 `build frontend image` 两步均 success。 |
| 4 | 在公共默认分支上存在真实的 `ci.yml` 运行，且触发方式保留门禁 | ✓ VERIFIED | `gh run list --workflow ci.yml`：run `39380373930`（push, 2026-10-20T06:26:30Z, success）与 `39525757252`（push, 2026-10-21T01:10:28Z, success）均为 push 到 `main`，head 分别为 `630f875` / `c060c84`。 |
| 5 | 仓库 owner 的历史决策已记录并执行 | ✓ VERIFIED | `c060c84..HEAD` 无 force 改写（12 个 `.planning` + 1 个门禁脚本的线性提交）；round-6 的快进推送记录与 48-15 的一次性例外记录不变。 |
| 6 | CICD-01/02/03 只在可佐证的四绿 run 之后推进 | ✓ VERIFIED | REQUIREMENTS 两处均 `Complete (run 39380373930)`；该 run 本轮经 `gh run view` 独立复核为四绿全 step success。 |
| 7 | 没有为制造绿灯而弱化门禁 | ✓ VERIFIED | `ci.yml` 无 `continue-on-error`、`paths:`/`paths-ignore:`、`pull_request_target`、`\|\| true`、无 job/step 级 `if:`、无 `secrets.`；回归变异 G1（fe job 加 job 级 `if:`）、G4（`Gate check` 步加 `env:` 块）本轮重跑仍 exit 1。 |
| 8 | WR-06（push 路径 lint 范围）由运行证据裁决 | ✓ VERIFIED | round-5 的 run-3（`39353593172`）fast-forward push 上 lint 绿色；`lint (changed lines)` step 在 run `39525757252` 中为 success。 |
| 9 | 门禁 bootstrap 不打开未记录的供应链面；register == code | ✓ VERIFIED | P3 的 parsed 精确等值断言 + `P3-INSTENV-OK` 在健康树通过；R13（`GOTOOLCHAIN: local`→`auto`）本轮重跑 exit 1，消息 `FAIL P2/P3: the go job's structural contract is violated`；P11 registry == call sites == begin markers == success tokens（11/11/11）。 |
| 10 | 实时门禁只把 secrets 授给消费它的 step | ✓ VERIFIED | P9 在基线通过；G4（给 `Gate check` 加三 secret `env:`）本轮重跑 exit 1，消息点名 `the 'Gate check' step carries an env: key`。 |
| 11 | workflow 侧 Class-B 闭合，含防重命名 determinism 守卫 | ✓ VERIFIED | materialised-list 形态在位（P5a 写 `$RUNNER_TEMP/go-test-list.txt`）；R11（删 quoted-paths heredoc）exit 1 报 vacuous pin；R9（删 parsed path-filter walk）exit 1 报 `'quoted-paths' fixture`。 |
| 12 | **verifier 拒绝把空洞或被中和的检查折叠为 PASS** | ✓ VERIFIED（本轮由 FAILED 升为 VERIFIED） | CR-01 与 WR-01 的两个 BLOCKER 全部闭合，独立复现见下方 Mutation Evidence：D2（flow-style `pull_request_target`）exit 1 并给 file:line；R15/R15b/R16/R17（删解析判官 / 删 token / 退回 quote-blind 剥离 / 删文本 probe）逐条 exit 1 点名 P1；R18/R19/R19b（删 WRITE_VALUE 行扫描 / 删 job-level walk / 放宽标量分支）逐条 exit 1 点名 `perm-write-line` / `perm-job-write` / `perm-job-readall`。 |
| 13 | SC-4：实时 eval-gate 从不伪造绿灯；fork/Dependabot 边界、secrets 与单一 Go 版本源成立 | ✓ VERIFIED | P9 通过；G4 exit 1；`.github/workflows/eval-gate.yml` 在 `c060c84..HEAD` 未改动。 |
| 14 | SC-5：`bash verify-phase-48.sh` exit 0 | ✓ VERIFIED | 当前提交树实测（`C:\Program Files\Git\bin\bash.exe`）**EXIT=0**，恰好一条 `=== Phase 48 Gate: ALL PASSED ===`，`NOT VERIFIED` 0 命中；`17 fixtures rejected for their registered reasons: 4 vacuity + 13 per-file`；`P4: config verify accepted .golangci.yml (tool output: 0 byte(s))`。 |
| 15 | 每条断言 run/transport 状态的记录都被对账到同一结果 | ✓ VERIFIED | STATE.md / 48-VALIDATION.md / 48-SECURITY.md 三处都带同一 canonical fragment（`total runs: 7` / first all-green `39380373930` / run 7 = `39525757252`）；REQUIREMENTS.md 未被编辑；`48-27-SUMMARY.md` 的 37 条演练矩阵与本轮独立结果一致（见下）。 |

**Score:** 15/15 truths verified（0 present-but-behavior-unverified，0 failed）

---

## Mutation Evidence（独立沙箱，仓库未被修改）

演练在 `mktemp` 沙箱副本（`verify-phase-48.sh` + `.github/` + `.golangci.yml` + `.planning/ROADMAP.md`）上执行；每个变异对锚点先断言存在且唯一，no-op 无法伪造结果。**verifier 自己写的 harness**（`drills.sh` + `mutate.py`），未采用 `48-27-PLAN.md` 里的演练代码片段（原因见 advisory 末条）。

### Round-7 新增（CR-01 / WR-01 / WR-02）

| # | Mutation | Result | Verdict |
|---|----------|--------|---------|
| D0 | baseline（当前提交树） | exit 0，恰好一条 `ALL PASSED`，11/11 registry/begin/token，17/17 fixtures | control |
| D2 | 在 `.github/workflows/zz-cr01-flow.yml` 注入 `on: {push: {branches: ['main#hotfix']}, pull_request_target: {}}` | **exit 1** — `FAIL P1 pull_request_target found (D-11) — reported by the parsed on: mapping and by quote-aware comment-stripped text; listed above with file:line`，且日志含 `zz-cr01-flow.yml:` 的 file:line 证据 | **CR-01 BITES** |
| R15 | 删 `# T-48-83: P1 parsed trigger judge` 段 + 把调用点改 `true` | **exit 1** — `FAIL P1 the block-style positive canary returned rc=1 (text) / rc=127 (parsed), want 1 (hit) from both — a probe that cannot produce a decisive result must not deposit PASS (R2-WR-03)` | BITES |
| R15b | 只删 `printf 'P1-NO-PRT-PARSED\n' >> "$WORK/p1.log"` | **exit 1** — `FAIL partition P1 cannot deposit its success tokens: … does not contain the probe-emitted token(s) 'P1-NO-PRT-PARSED' (required: P1-PARSED P1-COMPOSITE-OK P1-NO-PRT P1-NO-PRT-PARSED)` | BITES（正是计划预期的红因） |
| R16 | 把 helper body 退回 quote-blind `re.sub(r"(?m)#.*$", "", src)` | **exit 1** — `FAIL P1 the flow-style positive canary returned rc=0 (text) / rc=1 (parsed), want 1 (hit) from both — the quote-blind strip hid a flow-style trigger after a quoted '#' (CR-01) …` | BITES |
| R17 | 删 `p1_prt_probe "$f" >> …` + `p1_prt_rc=$?` + 其 case 块 | **exit 1** — `FAIL P1 the comment-stripped pull_request_target scan produced a decisive result for 0 of 2 enumerated file(s) …` | BITES |
| R18 | 删 `# T-48-84: WRITE_VALUE line scan` 段 | **exit 1** — `FAIL P11 fixture 'perm-write-line' rejected for the wrong reason: wanted 'contains a write scope' (WR-R5-01)` | **WR-01 BITES，点名 perm-write-line** |
| R19 | 删 `# T-48-84: job-level permissions walk` 段 | **exit 1** — `FAIL P11 verifier is vacuous on an empty partition: scan_trust_surface returned success on the 'perm-job-write' fixture` | **WR-01 BITES，点名 perm-job-write** |
| R19b | `if value in ("read-all", "write-all"):` → `("write-all",)` | **exit 1** — `FAIL P11 fixture 'perm-job-readall' rejected for the wrong reason: wanted 'job 'build' permissions grants 'read-all'' (WR-R5-01)`（理由落到 unknown-string 分支） | BITES |
| R20 | config-verify 换成 `: > "$WORK/p4-config.log"` + `p4_cfg_rc=0` | exit 0，`allpassed=1`，证据行 `tool output: 0 byte(s)` | 绿是设计内（测的是诚实取样，不是变红） |
| R21 | 删 `# T-48-04: parsed secret walk` 段 + 向真实树副本注入 `quoted-hash-secret.yml` | **exit 1**，日志含 `references secrets.DEPLOY_KEY` 且**不含** `parsed value references` → 消息来自文本层，证明第二个调用点确实共享 quote-aware 剥离 | BITES（共享性成立） |

### Round-6 及更早的回归抽测（无回归）

| # | Mutation | Result | Verdict |
|---|----------|--------|---------|
| R9 | 删 `# T-48-03: parsed path-filter walk` 段 | exit 1 — `returned success on the 'quoted-paths' fixture` | BITES |
| R11 | 删 quoted-paths fixture 的 heredoc | exit 1 — `P11 fixture 'quoted-paths' has no workflow file to scan — a deleted or never-written fixture is a vacuous pin, not evidence (WR-R5-01)` | BITES |
| R13 | `GOTOOLCHAIN: local` → `auto`（用缩进版锚点，避开 ci.yml 注释里的同名字面） | exit 1 — `FAIL P2/P3: the go job's structural contract is violated` | BITES |
| G1 | `fe` job 加 job-level `if:` | exit 1 — `the fe job carries a job-level if:` | BITES |
| G4 | `Gate check` 步加 `env:` 块 | exit 1 — `the 'Gate check' step carries an env: key` | BITES |

### 计划验收 grep（逐条 PASS）

`P1-SHARED-STRIP-OK`、`P11-ISOLATED-FIXTURES-OK`、`P4-MEASURED-EVIDENCE-OK`、`ROUND7-MESSAGES-PRESENT`、`MATRIX-RECORDED` 全部通过。计数核对：`17 fixtures` 出现 2 次、`13 per-file` 出现 2 次、字面 `re.sub(r"(?m)#.*$"` 在 `verify-phase-48.sh` 中出现 **0** 次、`tool output: present` 出现 **0** 次、`lib_yaml_text import strip_yaml_comments` 出现 **2** 次（P1 与 `scan_trust_surface` 各一次，共享同一 helper）。

---

## 48-27 must_haves 逐条判定（9 条 truths）

| # | must_have（摘要） | Status | Evidence |
|---|-------------------|--------|----------|
| 1 | P1 对被禁事件按**解析后的 `on:` 映射**在三形态（mapping key / sequence element / bare scalar）判定；flow 单行映射里引号内 `#` 会让整个门禁 exit 1 并给 file:line | ✓ VERIFIED | `p1_prt_parsed_probe()`（:421-463）读 `doc.get("on", doc.get(True))`，三分支 + 非 dict 显式 exit 1；变异 **D2** exit 1 且日志含 file:line。 |
| 2 | 两个剥离点共用**一个** quote-aware helper `strip_yaml_comments`，脚本中不再出现字面 `re.sub(r"(?m)#.*$")` | ✓ VERIFIED | `:98-206` 写出 `$WORK/lib_yaml_text.py`；两个调用点 `:371-419` 与 `:1367-1380` 各自 `from lib_yaml_text import strip_yaml_comments`；`grep -c 're\.sub(r"(?m)#\.\*\$"'` = **0**；变异 **R16** 退回旧写法后 flow canary 变红。 |
| 3 | 文本层自身的引号感知由**四个 canary** 现场证明（block 正例 rc1 / flow 引号内 `#` 正例 rc1 / quoted-hash 负例 rc0 / comment-only 负例 rc0） | ✓ VERIFIED | `:464-510` 四个 canary 文件，text 与 parsed **两层**都要求规定 rc；变异 **R15/R16** 红在 canary 上并带固定可 grep 的消息。 |
| 4 | P1 的 deposit 需要新 token `P1-NO-PRT-PARSED`，且只在解析判官的决定性结果 + 零命中断言之后 append | ✓ VERIFIED | `:550-557`：三条断言（两个 scanned 等值 + `p1_hits == 0`）之后才 `printf` 两个 token；`ok_partition "P1"`（:559）把它列为首个必需 token；变异 **R15b** 只删该 `printf` 即 exit 1。 |
| 5 | 每条写权限断言由**自己的隔离 fixture** 缴纳自己的 token（`perm-write-line`→`P11-PERMSWRITE-OK`、`perm-job-write`→`P11-JOBPERMS-OK`、`perm-job-readall`→`P11-JOBPERMS-STR-OK`、`yaml-ext`→`P11-YAMLEXT-OK`） | ✓ VERIFIED | 注册表 :2362-2378 一一绑定，未注册 fixture 直接 `fail … has no registered rejection reason (WR-R5-01)`；健康树逐条打印各自的首行理由（见下"Behavioral Spot-Checks"）。 |
| 6 | 删 WRITE_VALUE 行扫描 → 红并点名 `perm-write-line`；删 job-level permissions walk → 红并点名 `perm-job-write`；两者都到不了 ALL PASSED | ✓ VERIFIED | 变异 **R18** / **R19** 分别 exit 1，消息逐字点名对应 fixture。 |
| 7 | fixture 计数断言、循环列表、mkdir 列表、deposit token 列表、done_partition 文案、收尾 banner **六处**都说 17 fixtures（4 vacuity + 13 per-file） | ✓ VERIFIED | `:2414` 等值断言 `[ "$P11_FIXTURE_COUNT" -eq 17 ] \|\| fail …`；`:2443` 与 `:2476` 文案一致；健康树打印 `17 fixtures rejected for their registered reasons: 4 vacuity + 13 per-file`；`grep -c "17 fixtures"`=2、`grep -c "13 per-file"`=2。 |
| 8 | P4 config-verify 证据行报告**工具实测字节数**（append 前采样），0 字节时打印 `tool output: 0 byte(s)`，`present` 不再出现 | ✓ VERIFIED | `:903-922`：`p4_cfg_bytes=$(wc -c < … \| tr -d ' \r\n')` 在 `{ echo … } >> "$WORK/p4-config.log"` **之前**；健康树实测输出 `INFO P4: config verify accepted .golangci.yml (tool output: 0 byte(s))`；`grep -c 'tool output: present'` = **0**。 |
| 9 | 健康树仍 exit 0 恰一条 ALL PASSED；矩阵保留旧结果（D0 绿、R1-R14 与 G1-G4 全 exit 1）并新增 R15-R21 | ✓ VERIFIED | 本轮实测 EXIT=0 / `ALL PASSED` 计数 = 1 / `NOT VERIFIED` 计数 = 0；回归抽测 R9/R11/R13/G1/G4 全部 exit 1（见上表）；`48-27-SUMMARY.md` 含 R15/R16/R17/R18/R19/R19b/R20/R21/D0/R1/R14/G1/G4 的记录。 |

**48-27 must_haves：9/9 通过。** 三条 prohibition 同样成立：门禁未为造绿而削弱（每个非 D0 演练都点名 partition / fixture / token，`P11_FIXTURE_COUNT` 仍是等值断言）；`git diff --stat c060c84..HEAD` 只碰 `verify-phase-48.sh` + `.planning/**`，未触碰 `.github/workflows/*`、`.golangci.yml`、Dockerfiles、REQUIREMENTS.md；证据不再说谎（`tool output: present` 0 命中，`P1-NO-PRT-PARSED` 与 `P4-CONFIG-OK` 都在断言之后 printf）。

---

## Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `verify-phase-48.sh` | 11 partition、每断言自带证据 token 的零 waiver 门禁 | ✓ 存在且可用 | 2476 行；健康树 exit 0，`registry 11/11, begin markers 11/11, success tokens 11/11`。 |
| `$WORK/lib_yaml_text.py`（`strip_yaml_comments`） | 单一 quote-aware 剥离器，落在 `# T-48-83: quote-aware comment strip helper` 标记区间内 | ✓ 存在 | `:98-206`；三状态机 NORMAL/SINGLE/DOUBLE，未闭合引号整行保留（fail toward more text）。 |
| `p1_prt_parsed_probe()` | 解析优先的 `on:` 判官 | ✓ 存在 | `:421-463`；三形态 + 非 mapping 显式拒绝。 |
| `$WORK/p1-prt-canary{,-flow,-quoted-hash,-comment}.yml` | 四个 canary | ✓ 存在 | `:464-510`；每层各跑一次并要求规定 rc。 |
| `$WORK/p11-fixtures/perm-write-line`、`perm-job-write`、`perm-job-readall` | 三条写权限规则的隔离 fixture | ✓ 存在 | `:2054-2068` / `:2070-2095` / `:2097-2116`；fixtures 14→17。 |
| `.github/workflows/ci.yml` | Go / FE / Docker / verify 四 job | ✓ 未改动 | `git diff c060c84..HEAD -- .github/` 为空。 |
| `.github/workflows/eval-gate.yml` | live 门禁（不进 CI 背书线） | ✓ 未改动 | 同上。 |
| `.github/actions/setup/action.yml`、`.golangci.yml`、`Dockerfile.backend`、`Dockerfile.frontend` | bootstrap / lint / 双镜像 | ✓ 存在且 CI 内构建成功 | run `39525757252` 的 `docker` job 两步 success。 |

## Key Link Verification

| From | To | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| workflow 文件 | 解析判官 | `yaml.safe_load` → `doc.get("on", doc.get(True))` → 三形态拒绝 | ✓ | `P1-NO-PRT-PARSED` 在断言之后 append，删解析判官即 R15 红。 |
| workflow 行 | file:line 证据 | `strip_yaml_comments`（引号状态机）→ 逐行匹配 | ✓ | 报告保持精确而假阴性洞关闭；D2 演练日志含 `zz-cr01-flow.yml:` 行号。 |
| fixture 主体 | class token | `scan_trust_surface` 规则序（publication → WRITE_VALUE 行 → workflow perms → job perms）→ 注册 `want` 子串 → class token | ✓ | 一个 token 只由一个 fixture 缴纳，同胞无法代缴；R18/R19/R19b 逐条证红。 |
| config verify 字节数 | P4 证据行 | `wc -c`（append 前采样）→ `tool output: N byte(s)` → `p4-config.log` → `ok_partition P4` | ✓ | 健康树实测 0 byte(s)。 |
| `.github/`（CI 定义） | 公共绿灯 run | push → ci.yml → 四 job | ✓ | 定义自 `c060c84` 起未变，run `39525757252` 四绿。 |

## Data-Flow Trace

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `p1.log` | `P1-NO-PRT` / `P1-NO-PRT-PARSED` | 两个 probe 的决定性结果 + `p1_hits == 0` | ✓（不是自我声明，删断言即红） |
| `p11-fixtures.log` | 8 个 class token | 每个 fixture **首个命中规则**的实测输出 | ✓（实测理由见 spot-check 表） |
| `p4-config.log` | `tool output: N byte(s)` | `golangci-lint config verify` 的真实 stdout/stderr 字节数 | ✓（0 字节如实打印 0） |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| 健康树整体门禁 | `C:\Program Files\Git\bin\bash.exe -lc "bash verify-phase-48.sh"` | EXIT=0；`grep -c "ALL PASSED"` = 1；`grep -c "NOT VERIFIED"` = 0 | ✓ |
| P4 证据诚实性 | 同上输出第 17 行 | `INFO P4: config verify accepted .golangci.yml (tool output: 0 byte(s))` | ✓ |
| P11 注册完整性 | 同上输出第 90 行 | `INFO P11: registry 11/11, begin markers 11/11, success tokens 11/11` | ✓ |
| 四个新 fixture 各自按注册理由被拒 | 同上输出 84-89 行 | `perm-write-line` → `ci.yml:6 contains a write scope (D-26/D-29)`；`perm-job-write` → `in job 'build' grants 'contents'='write'`；`perm-job-readall` → `job 'build' permissions grants 'read-all'`；`yaml-ext` → `contains forbidden registry/publication token 'packages: write'` | ✓ |
| P1 双层判定文案 | 同上输出第 3 / 96 行 | `zero pull_request_target in the parsed on: mapping AND in quote-aware comment-stripped text; per-assertion success tokens P1-PARSED P1-COMPOSITE-OK P1-NO-PRT P1-NO-PRT-PARSED` | ✓ |

## Probe Execution

| Probe | Command | Result | Status |
| ----- | ------- | ------ | ------ |
| Go 全量回归 | `wsl.exe -e bash -lc "cd /mnt/d/CODE/ClaudeCode/ai-agent && go test ./..."` | exit 0，`go version go1.25.5 linux/amd64`（与 CI `ubuntu-latest` 同构），全包 `ok` | ✓ |
| FE 全量回归 | `cd web && npx vitest run` | exit 0，**85 test files / 689 tests** 全绿（24.56s） | ✓ |
| 公共 CI 运行 | `gh run view 39525757252` / `gh run view 39380373930` | 两次 run 均 `success`，各 4 job 全 `success`、全 step `success` | ✓ |
| 记录边界 | `git diff --stat c060c84..HEAD -- .github/` | 空；完整 diff 只含 12 个 `.planning/**` + `verify-phase-48.sh`（+419/-47） | ✓ |
| 计划执行记账 | `git log --oneline fd6f082..HEAD -- .planning/ROADMAP.md` | 只有 `3935876` 一次改动，`48-27-PLAN.md` 加入时即为 `- [ ]`，此后无人再勾 | ⚠️ 见 advisory #2 |

> Windows 宿主上 `go test ./...` 的 5 个 duckdb 依赖包链接失败（`undefined reference to __emutls_v._ZSt11__once_call`）在本轮复核为**宿主工具链限制**：本机 msys2 ucrt64 gcc 与 duckdb-go-bindings 预编译 Windows 静态库的 TLS ABI 不匹配；`c060c84..HEAD` 的 diff 是**零行 Go 代码**，且 WSL（同 CI 的 linux/amd64）全绿，因此不是回归。

## Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| CICD-01 | 48-01（`go` job）、48-02、48-16、48-26、48-27 | Go CI 进 GitHub Actions——golangci-lint + `go test ./...` + CGO_ENABLED=1 build+vet，PR/push 触发 | ✓ Complete | REQUIREMENTS.md:17 `[x]`、:69 `Complete (run 39380373930)`；run `39525757252` `go=success`（5 个 step 全 success）。 |
| CICD-02 | 48-02（`fe` job）、48-26、48-27 | FE CI——`tsc --noEmit` + `vitest run` 全量 | ✓ Complete | REQUIREMENTS.md:18 `[x]`、:70 同引用；run `39525757252` `fe=success`（typecheck + unit tests success）；本地 `vitest run` 689/689 绿。 |
| CICD-03 | 48-02（`docker` job）、48-16、48-26、48-27 | Docker 镜像构建验证——backend/frontend 双 Dockerfile 在 CI 中可构建 | ✓ Complete | REQUIREMENTS.md:19 `[x]`、:71 同引用；run `39525757252` `docker=success`（两个 build step 均 success）。 |

**需求追溯核对**：27 份 PLAN（48-01..48-27）的 frontmatter 全部带 `requirements:`，取值均为 `{CICD-01, CICD-02, CICD-03}` 的子集；**三个 ID 全覆盖，无孤立 ID，无 plan 声明了不存在的 ID**。`48-27-PLAN.md` 的三条 `flagged_assumptions` 明确"不重翻 CICD 状态、不编辑任何 requirement 行"——本轮复核确认 `REQUIREMENTS.md` 在 `c060c84..HEAD` 的 diff 为空，遵守。

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| — | — | 本轮 delta 未发现新的反模式 | — | 三个 `fix(48-ci)` 提交都是"加严"方向：无新增 waiver、无 `|| true`、无被中和的断言、无自我声明的证据行。 |

## Human Verification Required

N/A —— 基础设施/CI phase，无用户可见元素；五条 success criteria 全部可程序化验证，本轮实测覆盖 SC-1..SC-5。M2（真实 fork PR 触发 live eval-gate）、M3（Dependabot PR 按 fork 处理）、M5（第二次绿灯 docker run 的 cache hit）仍是已声明的 manual-only 项，按设计 Open，不构成 must-have 失败；`48-VERIFICATION-R6.md` 已逐项列出其理由，本轮无变化。

## Gaps Summary

**阶段目标在本轮达成，round-6 的两个 BLOCKER 与一个 WARNING 全部闭合，无回归。**

- **CR-01 闭合**：P1 的触发判定改为解析优先（`doc.get("on", doc.get(True))`，mapping-key / sequence-element / bare-scalar 三形态），两个注释剥离点共用一个 quote-aware helper `strip_yaml_comments`。独立复现：把 round-6 的攻击形态 `on: {push: {branches: ['main#hotfix']}, pull_request_target: {}}` 注入沙箱副本后，门禁 **exit 1** 并给出 `zz-cr01-flow.yml:` 的 file:line；把 helper 退回 quote-blind 写法（R16）或删掉解析判官（R15）或删掉新 token（R15b）或删掉文本 probe（R17），四条都各自 exit 1 并点名 P1 与具体 token。
- **WR-01 闭合**：`P11-PERMSWRITE-OK` 只由 `perm-write-line` 缴纳，`P11-JOBPERMS-OK` 只由 `perm-job-write` 缴纳，`P11-JOBPERMS-STR-OK` 只由 `perm-job-readall` 缴纳，`yaml-ext` 改缴 `P11-YAMLEXT-OK`。独立复现：删 WRITE_VALUE 行扫描（R18）与删 job-level permissions walk（R19）分别 exit 1 并**点名对应 fixture**；放宽标量分支（R19b）exit 1 点名 `perm-job-readall`。注册表还拒绝任何"未登记拒绝理由的 fixture"，杜绝同类漏洞上移。
- **WR-02 闭合**：P4 证据行改为 append 前 `wc -c` 实测，健康树实测打印 `tool output: 0 byte(s)`（工具本轮就是静默的），脚本中 `tool output: present` 0 命中。IN-02 也随之闭合：P1 主循环对 rc ∉ {0,1} 显式 `fail`，两个 probe 都必须给出决定性结果。
- **CI 定义未动**：`git diff --stat fd6f082..HEAD` 与 `git diff --stat c060c84..HEAD` 均未触碰 `.github/workflows/**`；两次公共 run 四绿全 step success，本轮本地 Go（WSL go1.25.5 linux/amd64）与 FE（689 测试）全绿。
- **记账滞后一项（advisory，非 gap）**：`ROADMAP.md:180` 的 `48-27-PLAN.md` 仍是 `- [ ]`、Phase 48 抬头仍写 `26/26 executed`。这只影响计划登记的可读性，不影响阶段目标与任何 must-have；修它需要改 `ROADMAP.md`，而本轮 verifier 的权限是只读工件，故作为 advisory 交回编排者处置。

---

_Verified: 2026-10-21T09:51:51Z_
_Verifier: DeepSeek Harness (gsd-verifier)，round-7 独立复核_