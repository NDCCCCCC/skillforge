---
phase: 48-ci
plan: 27
subsystem: ci
tags: [github-actions, ci-gate, verification, quote-aware-yaml, parsed-judge, evidence-binding, per-rule-fixtures, bash, python]

requires:
  - phase: 48-ci
    provides: verify-phase-48.sh with 11 partitions, per-assertion evidence tokens, the 48-23 attributing P11 fixture loop, the P3-INSTENV-OK env pin and the P4-CONFIG-OK binding; the round-6 verification report and its mutation table (D0, R1-R14, G1-G4, CR-01, D1, D2, WR-02)
provides:
  - P1 的触发判定改为解析优先：新增 `p1_prt_parsed_probe()`（按 mapping-key / sequence-element / bare-scalar 三形态读 `doc.get("on", doc.get(True))`），新 token `P1-NO-PRT-PARSED` 在解析判官与零命中断言之后才写入 `p1.log`，由 `ok_partition "P1"` 首个要求（CR-01）
  - 两个注释剥离点共用一个 quote-aware helper：`$WORK/lib_yaml_text.py` 的 `strip_yaml_comments(src)`（三状态机 NORMAL/SINGLE/DOUBLE），P1 文本层与 `scan_trust_surface` 各 import 一次；脚本中 `re.sub(r"(?m)#.*$"` 出现 0 次
  - 四个 canary 取代单一 canary：block 形态、flow 形态（引号内 `#`）、quoted-hash 负例、comment-only 负例，每个 canary 的判定消息固定可 grep（CR-01 / IN-R5-03 不回归）
  - 每条写权限规则各有一个只可能被它自己拒绝的 fixture 与专属 class token：`perm-write-line`→`P11-PERMSWRITE-OK`、`perm-job-write`→`P11-JOBPERMS-OK`、`perm-job-readall`→`P11-JOBPERMS-STR-OK`、`yaml-ext`→`P11-YAMLEXT-OK`；fixtures 14→17、class token 5→8，四处名字站点与三处数字站点全部对齐 17（4 vacuity + 13 per-file）（WR-01）
  - P4 的 config-verify 证据行改为实测字节数：`p4_cfg_bytes=$(wc -c < …)` 在任何 append 之前采样，输出 `tool output: N byte(s)`；"present" 与 "none (success is silent)" 两个措辞在脚本中各 0 次（WR-02）
  - Round-7 演练矩阵记录：37 个演练全部按预期落红/落绿，红色理由逐条点名 partition / fixture / token；旧矩阵 R1-R14 与 G1-G4 无一失效（G3 的红色理由变化已记旧因与新因）
affects: [48-ci round-7 verification, 48-SECURITY.md 的 T-48-83 / T-48-84 / T-48-85 三行（由 register 拥有者计划写入，本计划不碰）, 48-26]

actuals:
  tokens: 42000
  tasks: 4
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Quote-aware comment strip as a SHARED helper, not two copies: one `$WORK/lib_yaml_text.py` with a three-state machine (NORMAL / SINGLE / DOUBLE) imported by both call sites, so a fix to one can never leave the other quote-blind"
    - "Parsed document as the PRIMARY judge, text scan as the file:line evidence layer: the parsed `on:` mapping decides, the quote-aware text layer only reports where"
    - "One class token paid by exactly ONE fixture: a fixture that a sibling rule can also reject is not evidence for its own rule, so the isolation is a property of the fixture body, not of the registry"
    - "Evidence lines must MEASURE, not describe: a detector that appends its own bookkeeping into the log it then tests can only ever report itself; a byte count is a measurement, a word is a claim"
    - "A drill that cannot fail is worse than no drill: every marker range a drill deletes must be at COLUMN 0, otherwise the drill's anchor silently never matches and the mutation becomes a no-op that looks green"

key-files:
  created:
    - .planning/phases/48-ci/48-27-SUMMARY.md
  modified:
    - verify-phase-48.sh
    - .planning/ROADMAP.md

key-decisions:
  - "The T-48-84 marker ranges are at column 0 (not indented inside the python heredoc): the drills anchor on `line.startswith('# T-48-84: …')`, and an indented marker makes the mutation a no-op — the drill would then 'pass' while the rule was still live. Caught by running R18/R19 for real, not by reading the plan."
  - "`yaml-ext` keeps the plan's (c) payload (`packages: write`, a publication token) and only its class token changes: a write-scope payload there would be caught by the WRITE_VALUE scan too, and since yaml-ext sits EARLIER in the fixture loop it would then red first and R18 would name yaml-ext instead of perm-write-line — the drill would still be red, but no longer on the rule it isolates."
  - "`perm-job-write` uses a QUOTED key (`\"contents\": write`) rather than the plan's inline flow mapping: the plan's premise ('WRITE_VALUE cannot match a flow mapping') does not hold — WRITE_VALUE is indentation-agnostic, so an UNQUOTED job-level `contents: write` is claimed by the text rule first. The quoted key makes the fixture invisible to WRITE_VALUE at any indent, which is a strictly stronger isolation, and the registered `want` string is identical either way."
  - "`perm-job-readall` carries `read-all`, not `write-all`: `read-all` is the scope the D-29 enumeration deliberately refuses, it is the branch the R19b drill deletes (so the drill can discriminate at all), and it is the literal the plan's own Task-4 verify greps for."
  - "D1 is recorded as two rows (D1-old / D1-new) plus D2, because the plan's action text and its acceptance criterion pull in opposite directions: the action wants D1 to show the old gate's blindness (green), the acceptance wants D1 red under the new gate. Both are recorded rather than one being silently picked."
  - "R21 is run on the REAL tree, not through the fixture loop: `tojson-secrets` sits BEFORE `quoted-hash-secret` in the loop and only the parsed walk can reject it, so deleting the parsed walk makes the loop stop at tojson-secrets before it ever reaches the quoted-`#` fixture. The real-tree file is named `quoted-hash-secret.yml` so the red names it."
  - "STATE.md and ROADMAP.md are written by the ORCHESTRATOR, not by the executor: the plan's non-negotiables forbid the executor from touching them, and the Round-7 header's reference to the non-existent `48-ANALYSIS.md` was corrected by the orchestrator to the real sources (48-VERIFICATION.md / 48-REVIEW.md / 48-REVIEW-DISPOSITION.md)."
---

## Accomplishments

- **CR-01 closed at the source.** P1 no longer decides anything with a regex. The parsed `on:` document is the primary judge and rejects the banned event in all three shapes it can legally take (mapping key, sequence element, bare scalar), reading the trigger as `doc.get("on", doc.get(True))` because PyYAML 1.1 loads a bare `on:` as the boolean `True`. The text layer survives as the file:line evidence layer and is now quote-aware: both call sites import ONE helper (`$WORK/lib_yaml_text.py`, three-state machine over NORMAL / SINGLE / DOUBLE, `#` starts a comment only at line start or after whitespace outside quotes, an unterminated quote leaves the line intact so the failure direction is always seeing MORE text). The literal `re.sub(r"(?m)#.*$")` occurs zero times in the script.
- **WR-01 closed at the source.** Each write-permission rule now has a fixture only it can reject, and each pays its own class token: `perm-write-line` (workflow-level `contents: write`, claimed by the WRITE_VALUE line scan) → `P11-PERMSWRITE-OK`; `perm-job-write` (job-level QUOTED key `"contents": write`, invisible to WRITE_VALUE's key pattern `[A-Za-z0-9_-]+`) → `P11-JOBPERMS-OK`; `perm-job-readall` (job-level scalar `"permissions": read-all`, invisible to the mapping walk) → `P11-JOBPERMS-STR-OK`; `yaml-ext` (job-level `packages: write`, a publication token — it certifies the `*.yaml` enumeration) → `P11-YAMLEXT-OK`. Fixtures 14→17, class tokens 5→8; the mkdir list, the loop list, the case registry, the deposit list and the class-token list all carry the three new names, and the count assertion, the `done_partition` text and the closing banner all say 17 (4 vacuity + 13 per-file).
- **WR-02 closed at the source.** P4's config-verify evidence line reports the tool's real output size: `p4_cfg_bytes=$(wc -c < "$WORK/p4-config.log" | tr -d ' \r\n')` is sampled AFTER the rc capture and BEFORE any append, and the block prints `tool output: ${p4_cfg_bytes} byte(s)`. A successful `golangci-lint config verify` genuinely emits 0 bytes, so the healthy tree reads `tool output: 0 byte(s)` — 0 bytes IS the evidence. The old qualitative word `present` and its unreachable alternative occur 0 times in the script, and the INFO line carries the number to stdout as well because `$WORK` is removed by the exit trap.
- **Regression duty discharged.** The full matrix was re-run on the final tree: 37 drills, every one with its mutation anchored to a unique string, every one landing where it was expected to land. No previously recorded drill stopped biting.

## Round-7 Drill Matrix (final tree, sandbox copies, anchors asserted)

Every drill ran in its own sandbox copy of `verify-phase-48.sh` + `.github/` + `.golangci.yml` + `.planning/ROADMAP.md`. The repository was never modified by a drill. The matrix is SEQUENTIAL: `golangci-lint` takes a global lock, so a parallel run makes the P4 sentinels fail with `parallel golangci-lint is running` — a false red, not a finding.

| Drill | Mutation | Exit | Named partition / fixture / token | Note |
|---|---|---|---|---|
| base-pre | 旧脚本（`git show fd6f082:verify-phase-48.sh`）+ 当前树 | 0 | 1× `ALL PASSED` | pre-change baseline: 旧门禁在当前树上也是绿的（14 fixtures 时代） |
| base-post | 当前脚本 + 当前树 | 0 | 1× `ALL PASSED` | post-change baseline: 绿，17/17 fixtures，registry/begin/token 11/11/11 |
| D0 | 无 | 0 | 1× `ALL PASSED` | control |
| D1-old | **旧脚本** + flow-style 文件 `zz-review-probe.yml` | 0 | 1× `ALL PASSED` | 旧门禁对 CR-01 形态完全失明（历史 CR-01 记录原样复现） |
| D1-new | 当前脚本 + **quote-blind helper 变异** + 同一 flow-style 文件 | 1 | `P1 the flow-style positive canary returned rc=0 (text) / rc=1 (parsed), want 1 (hit) from both — the quote-blind strip hid a flow-style trigger after a quoted '#' (CR-01)` | 同一变异在旧门禁下是绿的、在新门禁下变红 = 记录在案的反转 |
| D2 | 当前脚本 + 同一 flow-style 文件 | 1 | `P1 …/zz-review-probe.yml: on: has a 'pull_request_target' key (D-11)` → `FAIL P1 pull_request_target found (D-11) — reported by the parsed on: mapping and by quote-aware comment-stripped text; listed above with file:line` | 解析层判定 + file:line 证据 |
| R1 | 删 P2 整段断言块 | 1 | `partition P2 cannot deposit its success tokens: … 'P2-CMDS-OK P2-DET-OK P2-SHUFFLE-OK P2-HERMETIC-OK'` | 历史红因不变 |
| R2 | 删 P3 段 | 1 | `partition P3 cannot deposit … 'P3-BLOCKING-OK P3-LINT-OK P3-VERIFYJOB-OK P3-INSTENV-OK'` | 历史红因不变（P3-STEP-IF-OK 在 lead-in 段，不随 P3 段删除） |
| R3 | 删 P1 composite 组 | 1 | `partition P1 cannot deposit … 'P1-COMPOSITE-OK'` | 历史红因不变 |
| R4 | 删 P4 sentinel A | 1 | `partition P4 cannot deposit … 'P4-A-ERRCHECK'` | 历史红因不变 |
| R5 | 删 P6 job-if 组 | 1 | `partition P6 cannot deposit … 'P6-JOB-IF-OK'` | 历史红因不变 |
| R6 | 删 P7 cache 组 | 1 | `partition P7 cannot deposit … 'P7-CACHE-OK'` | 历史红因不变 |
| R7 | 删 P9 notice 组 | 1 | `partition P9 cannot deposit … 'P9-NOTICE-OK'` | 历史红因不变 |
| R8 | banner 移到 else 之后 | 1 | `P11 the passing banner sits after an else: it is no longer inside the RAN-gated branch` | 历史红因不变 |
| R9 | 删 parsed path-filter walk | 1 | `P11 verifier is vacuous on an empty partition: scan_trust_surface returned success on the 'quoted-paths' fixture` | 历史红因不变 |
| R10 | 删 parsed secret walk | 1 | `… returned success on the 'tojson-secrets' fixture` | 历史红因不变 |
| R11 | 删 quoted-paths heredoc | 1 | `P11 fixture 'quoted-paths' has no workflow file to scan — a deleted or never-written fixture is a vacuous pin, not evidence (WR-R5-01)` | 历史红因不变 |
| R12 | path walk + quoted-paths + space-paths fixtures（复合） | 1 | `P11 fixture 'quoted-paths' has no workflow file to scan …` | 点名 `quoted-paths` ✓；与 R11 消息相同（变异不同：R12 另删了 path walk 与 space-paths） |
| R13 | `GOTOOLCHAIN: local` → `auto` | 1 | `P3: the linter install step env is {'GOTOOLCHAIN': 'auto', 'GOSUMDB': 'sum.golang.org', 'GOPROXY': 'https://proxy.golang.org,direct'}, want exactly {'GOTOOLCHAIN': 'local', …}` | 历史红因不变 |
| R14 | 删整个 install-step `env:` 块 | 1 | `P3: the linter install step env is {}, want exactly {'GOTOOLCHAIN': 'local', …}` | 历史红因不变 |
| G1 | fe job 加 job-level `if:` | 1 | `P6: the fe job carries a job-level if: "github.event_name == 'push'" - the gate is skippable (R2-WR-01)` | 无回归 |
| G2 | docker job 加 job-level `if:` | 1 | `P7: the docker job carries a job-level if: "github.event_name == 'push'" - the gate is skippable (R2-WR-01)` | 无回归 |
| G3 | 恢复 `go test -list \| grep -qx`（`LIST=` 行保留） | 1 | `P2: the -count=100 determinism step pipes the -list output into grep - 'grep -q' closes the pipe early and the producer takes SIGPIPE -> exit 141 on Linux (SC-1); materialise the list and grep the file instead` | **红色理由变了**（旧：`does not materialise the -list output to go-test-list.txt`），原因见下 |
| G3b | 同上，但连 `LIST="$RUNNER_TEMP/go-test-list.txt"` 行一起删（48-24 记录的形态） | 1 | `P2: the -count=100 determinism step does not materialise the -list output to go-test-list.txt - the guard must not run through a pipe (R2-WR-04 / SC-1)` | 补跑，复现历史红因 |
| G4 | 给 `Gate check` 加三 secret env | 1 | `P9 the 'Gate check' step carries an env: key - the offline threshold comparison consumes no secret, so any env block there is dead credential surface (R2-WR-05)` | 无回归 |
| R15 | 删解析判官（`p1_prt_parsed_probe` 整个函数 + marker 段） | 1 | `P1 the block-style positive canary returned rc=1 (text) / rc=127 (parsed), want 1 (hit) from both — a probe that cannot produce a decisive result must not deposit PASS (R2-WR-03)` | **红色理由与计划记录不同**（计划预期：缺 `P1-NO-PRT-PARSED`），原因见下 |
| R15b | 只删 `printf 'P1-NO-PRT-PARSED\n'` | 1 | `partition P1 cannot deposit its success tokens: … 'P1-NO-PRT-PARSED'` | 补跑，复现计划记录的旧红因 |
| R16 | helper body 退回 quote-blind 正则 | 1 | `P1 the flow-style positive canary returned rc=0 (text) / rc=1 (parsed), want 1 (hit) from both — the quote-blind strip hid a flow-style trigger after a quoted '#' (CR-01), and a probe that cannot produce a decisive result must not deposit PASS (R2-WR-03)` | 计划记录的旧红因 |
| R17 | 删文本扫描调用 + 其决定性结果计数 case | 1 | `P1 the comment-stripped pull_request_target scan produced a decisive result for 0 of 2 enumerated file(s) — a neutralised or deleted probe cannot deposit PASS (R2-WR-03)` | 计划记录的旧红因 |
| R18 | 删 WRITE_VALUE 行扫描（T-48-84 段） | 1 | `P11 fixture 'perm-write-line' rejected for the wrong reason: wanted 'contains a write scope' (WR-R5-01)` | 计划要求的点名 ✓（`perm-write-line`），且红色理由是"被兄弟规则以别的理由拒绝" |
| R19 | 删 job 级 permissions 遍历（T-48-84 段） | 1 | `P11 verifier is vacuous on an empty partition: scan_trust_surface returned success on the 'perm-job-write' fixture` | 计划要求的点名 ✓ |
| R19b | 只删 `judge_permissions_string` 的 `read-all` 分支 | 1 | `P11 fixture 'perm-job-readall' rejected for the wrong reason: wanted 'job 'build' permissions grants 'read-all'' (WR-R5-01)` | 红色理由变了（见下） |
| R20 | config-verify 换成 `: > "$WORK/p4-config.log"`（rc=0） | 0 | 证据行 `tool output: 0 byte(s)` | 绿是设计内的：这条演练判的是"证据行",不是退出码 |
| R20b | 恢复 pre-round-7 的证据写法（先 append `exit code:` 再 `[ -s ]` → 恒 `present`） | 0 | 证据行 `tool output: present` | 绿是设计内的：它证明旧写法不可证伪（工具 0 字节也写 `present`） |
| R21 | 删 parsed secret walk（T-48-04 段）+ 真实树注入 `quoted-hash-secret.yml` | 1 | `trust-surface: …/.github/workflows/quoted-hash-secret.yml references secrets.DEPLOY_KEY; only secrets.GITHUB_TOKEN is allowed outside eval-gate.yml (T-48-04)` | **TEXT 层措辞**（无 `parsed value`）——证明第二调用点的 quote-aware 剥离看得见引号内的 `#` |
| R21-control | 不删 parsed walk，仅注入同一文件 | 1 | `trust-surface: … quoted-hash-secret.yml parsed value references secrets.DEPLOY_KEY; only secrets.GITHUB_TOKEN is allowed outside eval-gate.yml (T-48-04)` | **解析层措辞** —— 与 R21 对照即层切换 |
| R21-counter | 删 parsed walk + 第二调用点退回 quote-blind + 注入同一文件 | 1 | 红在 `P11 verifier is vacuous … returned success on the 'tojson-secrets' fixture`；输出中**不出现** `references secrets.DEPLOY_KEY` | 判别力对照：R21 的红确实来自 quote-aware 剥离，不是来自删掉解析层这件事本身 |

### 红色理由变化（同时记旧因与新因）

1. **G3 — 旧因 `does not materialise the -list output to go-test-list.txt`，新因 `pipes the -list output into grep`**。原因：P2 探针的检查顺序是「`-list` 形式 → `go-test-list.txt` 物化 → `grep -qx` → 管道形态」。G3 只把重定向改成管道、`LIST=` 行仍在，于是物化检查先通过，管道检查才命中。G3b 用 48-24 记录的形态（连 `LIST=` 行一起删）复现了旧因。两个演练都落红、都点名 P2，**没有演练失效**。
2. **R19b — 计划预期"删 `read-all` 分支后门禁不变红"，实测落红，但理由变了**：`read-all` 落到同一函数的 unknown-string 兜底臂，措辞从 `grants 'read-all'; 'read-all' exceeds D-29's … ceiling` 变成 `grants unknown string 'read-all'`，P11 的注册理由不匹配 → 红在 `rejected for the wrong reason`。verdict 不变，措辞变——正是"红色理由变了但没消失"这一类。
3. **R15 — 计划记录的红因是"缺 `P1-NO-PRT-PARSED`"，实测红在更早的 canary 块**（`rc=127`，判官被删后命令找不到）。原因：canary 判定块刻意放在 deposit 之前，用来抓被中和的判官——这是它该起的作用。R15b 只删 `P1-NO-PRT-PARSED` 的 printf，复现了计划记录的旧红因。两个演练都落红、都点名 P1。

## 关键机制与计数

- **helper 目录进入 `scan_trust_surface` 的机制**：一次性环境变量前缀赋值 `P48_STRIP_DIR="$(native "$WORK")" python - "${args[@]}"`，heredoc 内 `sys.path.insert(0, os.environ["P48_STRIP_DIR"])` 后 `from lib_yaml_text import strip_yaml_comments`。**不是**额外的 argv 元素（`paths = sys.argv[1:]` 会把 helper 目录当成 workflow 文件去扫，并破坏零文件拒绝），**也不是**全局 `export`（会漏进本脚本派生的每一个 linter 子进程）。
- **P11 计数**：17 个 fixture（4 vacuity + 13 per-file）、8 个 class token、registry / begin markers / success tokens 三者 11/11/11。四个数字站点（`-eq 17` 断言、`done_partition` 文本、closing banner、fixture 列表）与四个名字站点（mkdir 列表、循环列表、case registry、class token 列表）全部对齐。
- **quote-aware 剥离是否暴露了真实 workflow 发现**：**没有**。真实树（`.github/workflows/`）在 pre-change 与 post-change 两个基线下都是绿的；P8 的 trust-surface 扫描在 quote-aware 剥离下没有新增红点。剥离只会看到更多文本，所以这是"没变差"的证据，不是"没变好"的证明——但也没有需要上报的真发现。

## Deviations

1. **[Rule 1 — Bug] T-48-84 的四个 marker 原本缩进在 python heredoc 里**
   - **问题**：计划 Task 2 (a) 要求"column-0 markers"，但实现把 `# T-48-84: WRITE_VALUE line scan` 等四行缩进了 4 个空格。R18/R19 的锚点是 `line.startswith("# T-48-84: …")`，缩进的 marker 永远匹配不上 → 变异变成 no-op → 演练"绿"而规则其实还在。这是"本该失败的演练却通过了"，比没有演练更糟。
   - **修复**：四个 marker 移到第 0 列（heredoc 内第 0 列的 `#` 仍是合法 Python 注释；同文件里 `# T-48-03: parsed path-filter walk` 早就是第 0 列，是既有惯例）。**修的是源码，不是演练。**
   - **验证**：R18 红在 `perm-write-line`、R19 红在 `perm-job-write`，两个点名都命中。
2. **[Rule 1 — Bug] `yaml-ext` 的载荷被改成了写权限，导致 R18 点名错对象**
   - **问题**：实现把 `yaml-ext` 的载荷从 `packages: write`（publication token）改成 workflow 级 `contents: write`，理由是"这样扩展名规则才可单独证明"。但这样一来 `yaml-ext` 也被 WRITE_VALUE 行扫描捕获，而它在 fixture 循环里排在 `perm-write-line` 前面 —— 删掉 WRITE_VALUE 扫描时它抢先变红，R18 点名的是 `yaml-ext` 而不是 `perm-write-line`。演练仍然落红，却不再指向它要隔离的规则。
   - **修复**：按计划 Task 2 (c) 恢复 `yaml-ext` 的载荷与注册理由（`packages: write` / `want='registry/publication token'`），只换 class token 为 `P11-YAMLEXT-OK`。扩展名规则的可证性改由 EXT 演练证明：把 `*.yaml` 从枚举里去掉（3 个站点）→ 红在 `yaml-ext`。
   - **验证**：R18 点名 `perm-write-line` ✓；EXT 演练红在 `yaml-ext` ✓。
3. **[Rule 2 — 计划前提错误] 计划说 WRITE_VALUE 匹配不到 flow mapping**
   - **问题**：计划 Task 2 (b) 断言 `perm-job-write` 用 inline flow mapping `permissions: {contents: write}`，理由是"WRITE_VALUE 匹配不到（冒号后面是花括号）"。实测 WRITE_VALUE 是**缩进无关**的，未加引号的 job 级 `contents: write` 会被文本规则先认领，fixture 就不再只被一条规则拒绝。
   - **修复**：`perm-job-write` 改用**加引号的键** `"contents": write` —— WRITE_VALUE 的键模式 `[A-Za-z0-9_-]+` 匹配不了带引号的键，于是文本规则在任何缩进下都看不见它，只有解析后的 job 级映射遍历能拒。注册理由串不变。
   - **验证**：R19 红在 `perm-job-write` ✓（删掉 job 级遍历后没有任何规则能拒它 → vacuity）。
4. **[Rule 3 — 与计划字面量对齐] `perm-job-readall` 的载荷从 `write-all` 改回 `read-all`**
   - **问题**：计划 Task 2 (b) 与 Task 4 的 verify 都要求 `read-all`（`job 'build' permissions grants 'read-all'`），实现用了 `write-all`。而且只有 `read-all` 才是 D-29 明令拒绝、也是 R19b 要删的那条分支 —— 载荷若是 `write-all`，R19b 会变成完全无效果的变异。
   - **修复**：载荷与注册理由都改为 `read-all`。
   - **验证**：R19b 红在 `perm-job-readall`（理由变了，verdict 不变）✓；计划 Task 4 的 `ROUND7-MESSAGES-PRESENT` 七个字面量现已全部命中。
5. **[Rule 4 — 计划的演练编号语义] D1 拆成 D1-old / D1-new 两行**
   - **问题**：计划 action 要求 D1 展示"旧门禁会打印 ALL PASSED"（绿），acceptance 又要求"D1 and D2 are both red under the new gate"（红）。二者不能同时由一行满足。
   - **修复**：记三行 —— D1-old（旧脚本 + flow-style 文件 → 绿，ALL PASSED）、D1-new（当前脚本 + quote-blind 变异 + 同一文件 → 红在 flow-style canary）、D2（当前脚本 + 同一文件 → 红，带 file:line）。**没有把任何一行改名或省略。**
6. **[Rule 5 — 计划前提错误] R21 无法经由 fixture 循环证明**
   - **问题**：计划要求"删 parsed secret walk 后 `quoted-hash-secret` 被 TEXT 层以 `references secrets.DEPLOY_KEY` 拒绝"。但循环顺序里 `tojson-secrets` 排在 `quoted-hash-secret` 之前，而 `toJSON(secrets)` 没有点号后的名字，只有解析层能拒它 —— 删掉解析层后循环先停在 tojson-secrets，根本走不到 quoted-hash-secret。
   - **修复**：R21 改在**真实树**上做：注入与 fixture 同体的 `quoted-hash-secret.yml`，删 T-48-04 解析层 → P8（真实树扫描，排在 P11 之前）以 **TEXT 层措辞**（`references secrets.DEPLOY_KEY`，无 `parsed value`）落红；R21-control 不删解析层 → 落红在**解析层措辞**（`parsed value references secrets.DEPLOY_KEY`）；R21-counter 再把第二调用点退回 quote-blind → 输出里不再出现该字面量，判别力成立。
   - **验证**：三行全部按预期落红/落绿。
7. **[Rule 6 — 计划自相矛盾] R20 是"绿"的演练**
   - **问题**：计划说"每个非 D0 演练必须 exit 1"，但 R20（把 config 日志逼成 0 字节）的验收写的是"门禁保持绿并报 `tool output: 0 byte(s)`"。
   - **修复**：按验收执行，R20 记为绿（退出码 0），判定项是**证据行**而不是退出码；并补 R20b（恢复 pre-round-7 的措辞写法）证明旧写法不可证伪——工具输出 0 字节时它照样写 `present`。
8. **[Rule 7 — 计划引用修正] ROADMAP 的 Round 7 标题引用了不存在的 `48-ANALYSIS.md`**
   - **问题**：`.planning/phases/48-ci/` 下没有 `48-ANALYSIS.md` 这个文件。
   - **修复**：改为真实出处 —— `48-VERIFICATION.md`（round-5 再验证 truth 12 FAILED，行 171 与变异表 205-209）+ `48-REVIEW.md`（round-6 review）+ `48-REVIEW-DISPOSITION.md`（CR-01/WR-01/WR-02 三行 open，行 43-45）。
   - **验证**：`grep -rn "48-ANALYSIS" .planning/` 现在 0 命中。

## TRUE FINDING

**无。** 真实树在 pre-change 与 post-change 两个基线下都绿；quote-aware 剥离没有让任何真实 `.github/workflows/` 文件变红，因此没有需要上报的 workflow 发现，也没有任何 workflow 被编辑（`git diff --stat` 只涉及 `verify-phase-48.sh` 与 `.planning/` 的工件）。

## Task Commits

1. **Task 1: parsed-first P1 trigger judge + one shared quote-aware stripper (CR-01)** — `a4cb04c` (fix)
2. **Task 2: one isolating fixture and one class token per write-permission rule (WR-01)** — `09d1444` (fix)
3. **Task 3: report P4's config-verify evidence as a measured byte count (WR-02)** — `6bbdcf8` (fix)
4. **Task 4: 复跑并记录 round-7 演练矩阵 + 写本 SUMMARY** — 本文件（fixup 提交另计）

## Verification

- `bash verify-phase-48.sh` → exit 0，恰好一条 `=== Phase 48 Gate: ALL PASSED ===`，无 `NOT VERIFIED`；P1 `2 workflow(s)`；P4 `tool output: 0 byte(s)`；P11 `registry 11/11, begin markers 11/11, success tokens 11/11`；17/17 fixture 按注册理由拒绝。
- 计划的 Task 4 verify 三条：①健康树绿；②`MATRIX-RECORDED` —— 本文件含 `R15 R16 R17 R18 R19 R20 R21` 与 `D0 R1 R14 G1 G4`；③`ROUND7-MESSAGES-PRESENT` —— 七个字面量在 `verify-phase-48.sh` 中全部命中。
- `git diff --stat` 只涉及 `verify-phase-48.sh`（本计划唯一允许改的源码文件）与 `.planning/` 工件；未编辑任何 `.github/workflows/*`、`.golangci.yml`、Dockerfile 或 `48-SECURITY.md`。
- 未改任何 requirement 状态行（REQUIREMENTS.md 本来就带 Complete）；未重开 branch-protection checkpoint（STATE.md 的 Option A 未动）。

## 回归门（execute-phase `regression_gate`）记录

- **命令解析**：`workflow.test_command` 未配置 → 根 Makefile 无 `^test:` 目标 → 根目录无 `package.json` → 有 `go.mod` ⇒ 解析为 `go test ./...`（与 CI 的 `go test ./... -count=1` 同形）。前序 phase 的 VERIFICATION.md 共 25 份、其中提取出的测试路径已写入 GSD handoff 文件 `regression-files`（Go 侧命令按项目语言解析，不消费该清单）。
- **Windows 宿主首跑红**：`cmd/server`、`internal/data`、`internal/duckdb`、`internal/kpi/wsl`、`internal/server` 五个包 `[build failed]`。根因是**本机工具链**，不是回归：`duckdb-go-bindings` 预编译的 Windows 静态库与本机 msys2 ucrt64 gcc 16.1.0 的 TLS ABI 不匹配 —— `ld.exe: libicu_extension.a: undefined reference to __emutls_v._ZSt11__once_call` / `__emutls_v._ZSt15__once_callable`。**本 phase 的 diff 零行 Go 代码**（`git diff --name-only 80990b1..HEAD` 无 `*.go`、无 `go.mod`/`go.sum`），编译输入与前序 phase 完全相同。
- **同构环境复跑绿**：同一条 `go test ./...` 在 WSL2 Ubuntu-22.04（go1.25.5 linux/amd64、gcc 11.4.0，与 CI 的 ubuntu-latest 同构）跑，**全绿** —— 上述五个 duckdb 依赖包同样 `ok`。
- **前端回归**：`cd web && npx vitest run` → **85 个测试文件 / 689 个测试全部通过**（138.34s）。
- **结论**：跨阶段回归门**通过**。Windows 侧的红是宿主链接器的既有限制，与本 phase 无关；后续在本机做全量 Go 回归须走 WSL 或 CI。

## Notes for the next agent

- `48-SECURITY.md` 的 **T-48-83 / T-48-84 / T-48-85** 三行由 register 拥有者计划（48-25 / 48-26 血统）写入，本计划按其登记说明**不碰**该文件。写入时还必须更正 **T-48-82** 的 mitigation 单元：它声称"真实触发器仍会以 file:line 变红"，而这对 CR-01 形态并不成立（`on: {push: {branches: ['main#hotfix']}, pull_request_target: {}}` 在 round-5 的剥离下会走到 `ALL PASSED`）。该行的 block-style 证据仍成立，泛化不成立。
- 演练矩阵的驱动器留在 `.planning/tmp/t4_matrix.py`（每个变异的锚点唯一性断言就写在里面），逐演练日志在 `.planning/tmp/t4-<id>.log`，合并结果在 `.planning/tmp/t4-consolidated.json`。**矩阵必须顺序跑**：`golangci-lint` 取全局锁，并行会让 P4 哨兵红在 `parallel golangci-lint is running`。
- Windows 上 `bash` 在 PATH 里是 WSL launcher（无 python / golangci-lint）；跑门禁必须显式用 `C:\Program Files\Git\bin\bash.exe`。
