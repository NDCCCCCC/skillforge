---
phase: 29-feedback-loop-ticket-interface
plan: 09
subsystem: infra
tags: [verify-gate, makefile, threat-model, stride, uat-runbook, fbk-05, structural-lock]

# Dependency graph
requires:
  - phase: 29-01..29-08 (all 8 prior plans of Phase 29)
    provides: turn-id 地基 / feedback+tickets 存储层 / HTTP 端点 / FE 反馈与 admin 双 tab —— 四道门的断言对象
  - phase: 28-graph-dynamic-relations
    provides: verify-phase-28.sh 3-stage 门骨架 + Makefile phase-28 target + STRIDE/28-UAT runbook 惯例
  - phase: 26-p10-3-hygiene
    provides: TODO 正则惯例（TODO(/TODO:/FIXME(/FIXME: 才计数，deep.TODO 不误伤）
provides:
  - scripts/verify-phase-29.sh 四阶段结构门（TODO 清零 / FBK-05 信任红线 / D15 配对 / DDL 纪律）+ 负向自证记录
  - make phase-29 target（.PHONY 已纳入）
  - docs/threat-model/STRIDE-29.md（7 信任边界 + 10 威胁行全 Disposition）
  - .planning/phases/29-feedback-loop-ticket-interface/29-UAT.md（9 段 operator runbook，含 Phase 27→29 联合验收）
affects: [phase-30-closeout, HYG-02, HYG-03, milestone-v1.2-verification]

# Tech tracking
tech-stack:
  added: [] # 零新增依赖（bash/grep/make 全既有）
  patterns:
    - "4-stage verify 门：set -uo pipefail + 逐 stage echo PASS/FAIL + 首个 FAIL exit 1 + 结尾 PASS banner（28 骨架扩展）"
    - "结构门负向自证：注入违禁 token → FAIL 点名文件行号 → 撤销恢复 PASS，证明门真实把守"
    - "门断言先读门对象：写 Stage 前先 grep 实测计数，不为凑 PASS 弱化断言"

key-files:
  created:
    - scripts/verify-phase-29.sh
    - docs/threat-model/STRIDE-29.md
    - .planning/phases/29-feedback-loop-ticket-interface/29-UAT.md
  modified:
    - Makefile

key-decisions:
  - "Stage 4 DO NOTHING 断言跨 schema/feedback/tickets 三文件写表面计数（实测 2+3+2=7 ≥4）——ON CONFLICT 子句住在 DML 文件，schema.go 只带 DDL；计划原稿只数 schema.go 会把 INSERT-only 纪律的真相位置搞错"
  - "Stage 1 沿 verify-phase-26 TODO 正则（TODO(/TODO:/FIXME(/FIXME:）+ 反误伤断言：deep.TODO 在本 phase 文件列表内恰有 4 处引用、0 处误配"
  - "负向自证注入真实违禁 token 'UPDATE '（计划原稿示例 'UPDATED' 不含尾随空格、不会触发 'UPDATE ' 模式）——严格更强的自证"
  - "STRIDE-29.md 落 docs/threat-model/（沿 STRIDE-11..14/26/27 主流惯例 6/7；STRIDE-28 落在 phase 目录是离群点）；29-UAT 段落命名 '### Test N' 使验收 grep '^### Test' 可计数"

patterns-established:
  - "Phase 29 收口惯例 = verify-phase-29.sh + make phase-29 + STRIDE-29 + 29-UAT（对齐 26-10/27-06/28-07）"
  - "结构门负向自证记录进 SUMMARY：门的 PASS 必须伴随一次可复现的 FAIL 证据"

requirements-completed: [FBK-01, FBK-02, FBK-03, FBK-04, FBK-05, TKT-02, TKT-03, TKT-04]

# Metrics
duration: 7min
completed: 2026-09-08
---

# Phase 29 Plan 09: 收尾卫生门 Summary

**verify-phase-29.sh 四阶段结构门（FBK-05 信任红线 source-grep + D15 双侧配对 + DDL 纪律，附负向自证）+ make phase-29 目标 + STRIDE-29 威胁模型（10 威胁全处置）+ 29-UAT operator runbook（9 段含 Phase 27→29 联合验收）**

## Performance

- **Duration:** 7 min
- **Started:** 2026-09-07T22:05:02Z
- **Completed:** 2026-09-07T22:11:44Z
- **Tasks:** 2
- **Files modified:** 4（3 created + 1 modified）

## Accomplishments
- Phase 29 的 FBK-05「反馈绝不自动改信任」与 D15「data-turn Go/TS 配对」从一次性测试升级为可持续把守的门：Stage 2 对 feedback 三文件断言零 `UPDATE `/`DELETE `/`Insert(`、retriever 唯一引用 GetChunk（4 处）、零 internal/knowledge 引用；Stage 3 对 wire/chunk.go（5 处）与 wire-types.ts（4 处）双侧存在性断言——任一侧漂移即 FAIL。
- 门有效性经负向自证证明：向 internal/server/feedback.go 注入一行含 `UPDATE ` 的注释 → Stage 2a FAIL 点名文件+行号（exit 1）；撤销后恢复 PASS（exit 0）。
- STRIDE-29 登记 7 条信任边界（含「门脚本→代码库」弱化面）+ 10 个威胁（T-29-01..09 + T-29-SC），每行 Disposition 非空且 Mitigation 指到具体文件:行或门（全部锚点经 grep 实测核实，如 TestFeedbackNoTrustMutation@feedback_test.go:1082、UNIQUE(session_id,turn_id)@schema.go:405/470、isAdminRequest 首语句@feedback.go:436/535/663）。
- 29-UAT.md 9 段编号测试覆盖投票→幂等回显→复核处置→重开→指标→工单双 provider→Phase 27→29 联合验收（处置零 KB 变更、知识库变更仅由上传产生）→结构门；每步含命令 + 期望结果 + 实测记录列；live/webhook 项标 [operator]。

## Task Commits

Each task was committed atomically:

1. **Task 1: scripts/verify-phase-29.sh 四阶段门 + Makefile phase-29 目标** - `b6bbf0c` (feat)
2. **Task 2: STRIDE-29.md 威胁模型 + 29-U.md operator runbook** - `3dd2384` (docs)

**Plan metadata:** `(docs commit, see final commit)`

## Files Created/Modified
- `scripts/verify-phase-29.sh` - 4-stage 结构门（hermetic，grep-only，零 Go 工具链依赖）
- `Makefile` - phase-29 target（镜像 phase-28）+ .PHONY 追加
- `docs/threat-model/STRIDE-29.md` - 威胁模型登记表 + 缓解摘要
- `.planning/phases/29-feedback-loop-ticket-interface/29-UAT.md` - operator 验收 runbook

## Decisions Made
- Stage 1 TODO 扫描覆盖 Go + TS/TSX 全部 phase 文件（29 是 FE/BE 同 phase），并沿 verify-phase-26 正则——裸 `TODO` 会误伤 eino 类型名 `deep.TODO`（本 phase 文件内恰 4 处）；门的反误伤断言反而比 28 版更强。
- Stage 4 断言对象修正为「schema.go + feedback.go + tickets.go 三文件写表面」：零 `DO UPDATE`、`DO NOTHING` 总数 7 ≥ 4、4 张 additive 新表各 `CREATE TABLE IF NOT EXISTS`。
- make phase-29 经 WSL 执行（Windows host 无 make，Go/构建工具链的既有 repo 惯例）；29-UAT Test 9 已写明该 operator 指引。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Stage 4 DO NOTHING 断言位置修正（schema.go 实测仅 2 处，计划阈值 ≥4 不可满足）**
- **Found during:** Task 1（写门前先读门对象）
- **Issue:** 计划规定「schema.go 的 DO NOTHING 计数 ≥4」，实测 schema.go 仅 2 处；4+ 处 ON CONFLICT DO NOTHING 实际分布在 DML 文件（feedback.go 3 + tickets.go 2）。把 DML 写进 schema.go 才能满足原断言 = 为凑 PASS 改代码，方向错误。
- **Fix:** Stage 4 断言跨三文件写表面计数（2+3+2=7 ≥4）+ 零 DO UPDATE + 4 张新表 DDL 各 ≥1——INSERT-only 纪律的语义完整保留，且比原稿覆盖面更宽（DML 文件也被门住）。
- **Files modified:** scripts/verify-phase-29.sh
- **Verification:** `bash scripts/verify-phase-29.sh` Stage 4 PASS（4b: 7 DO NOTHING clauses）
- **Committed in:** b6bbf0c

**2. [Rule 1 - Bug] Stage 1 裸 TODO 正则会误伤 eino 类型名 deep.TODO**
- **Found during:** Task 1（实测扫描发现 4 处 `deep.TODO` 引用位于本 phase 文件列表内）
- **Issue:** 计划的裸 `TODO|FIXME` grep 会把 `deep.TODO.ActiveForm`（schema.go:172）等 4 处类型引用计为 TODO，门必假红；verify-phase-26 已为此建立正则惯例并写进 Makefile 注释。
- **Fix:** 采用 `TODO\(|TODO:|FIXME\(|FIXME:` 正则 + 新增反误伤断言（deep.TODO 引用存在但 0 误配，引用消失则 WARN）。
- **Files modified:** scripts/verify-phase-29.sh
- **Verification:** Stage 1 PASS 双行（0 markers + deep.TODO 4 refs not matched）
- **Committed in:** b6bbf0c

**3. [Rule 3 - Blocking] 负向自证注入 token 修正（计划示例 'UPDATED' 不触发门）**
- **Found during:** Task 1（执行验收时推演）
- **Issue:** 计划验收要求「注入 'UPDATED' 注释后 Stage 2 变 FAIL」，但 Stage 2 模式是尾随空格的 `UPDATE `——`UPDATED ` 不含该子串，按原字面执行自证会假绿（自证本身失效）。
- **Fix:** 注入真实违禁 token `UPDATE `（注释 `// ...feedback UPDATE session_id = ? ...`）→ Stage 2a FAIL 并点名行号 exit 1；`git checkout` 恢复后 PASS。严格更强的自证，结果记录于 29-UAT.md Test 9 步骤 3。
- **Files modified:** internal/server/feedback.go（临时注入后已原样恢复，git diff 为空）
- **Verification:** FAIL 输出含 `819:// NEGATIVE-SELF-TEST: ...` 行号；恢复后 exit 0
- **Committed in:** b6bbf0c（门文件）；注入物零残留（diff 干净）

### Planning-path corrections（非代码偏差）

- 计划 read_first 引用的 `docs/threat-model/STRIDE-28.md` 与 `.planning/phases/28-graph-dynamic-relations/28-UAT.md` 实际不存在；真实位置为 `.planning/phases/28-dynamic-knowledge-graph/STRIDE-28.md` 与 `.../28-UAT.md`（STRIDE-28 落在 phase 目录，与 docs/threat-model/ 下的 11/12/13/14/26/27 相比是离群点）。本 plan 的 STRIDE-29.md 按其 frontmatter artifacts 显式路径落 `docs/threat-model/`（主流惯例 6/7）。

---

**Total deviations:** 3 auto-fixed（1 bug / 2 blocking）
**Impact on plan:** 三处均为门断言与自证的「真相位置」修正，无一弱化断言——Stage 4 覆盖面更宽、Stage 1 反误伤更强、负向自证从假绿变真红。无 scope creep。

## Issues Encountered
- Windows host 无 `make` → `make phase-29` 经 WSL 执行成功（exit 0），与 Go 构建工具链同惯例；已在 29-UAT.md 记录 operator 指引。

## User Setup Required
None - 无外部服务配置。29-UAT.md 的 Test 1–8 为 operator-facing（需 live 服务 + 浏览器 + 本地假 webhook），Test 9 为 hermetic 任何人可跑。

## Known Stubs
None - 本 plan 纯文档/门产物，零运行时代码，无 stub 面向 UI。

## Next Phase Readiness
- Phase 29 全部 9 plans 执行完毕；四道结构门可重跑 PASS 且负向自证在案——Phase 30 的 HYG-02/03 里程碑级验收可直接消费 verify-phase-29.sh 的 PASS 记录。
- 待 operator：按 29-UAT.md Test 1–8 完成 live 验收（投票/幂等/复核/指标/工单双 provider/Phase 27→29 联合）；Test 9 已由本 plan 代跑并留档。
- 全量回归：WSL `go test ./... -count=1` exit 0 + FE 74 文件/595 用例全绿 + `tsc --noEmit` 干净（与 29-08 基线一致，零漂移）。

---
*Phase: 29-feedback-loop-ticket-interface*
*Completed: 2026-09-08*
