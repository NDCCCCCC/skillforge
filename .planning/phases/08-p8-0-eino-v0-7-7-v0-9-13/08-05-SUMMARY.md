---
phase: 08-p8-0-eino-v0-7-7-v0-9-13
plan: 05
subsystem: live-verification / regression-relock (dependency-upgrade behavior evidence)
tags: [live-verify, cancel-propagation, milvus-tier-filter, frozen-eval, human-verify, eino-v0.9.14, plat-04, v0.8-baseline]
requires:
  - "eino v0.9.14 + eino-ext openai v0.1.13 升级原子提交 + hermetic 四道门重锁（Phase 8 / Plan 08-01, commit 36845c0）"
  - "sqlite CheckPointStore 骨架六面测试锁定（Phase 8 / Plan 08-02, commits 9a557b1 RED + e6a580a GREEN）"
  - "SC-3 legacy interrupt 禁令门 + 版本叙事对齐（Phase 8 / Plan 08-03, commits fea6114 + 996c33a）"
  - "SC-5 真实中断往返 hermetic 测试 + phase hermetic 五门全绿（Phase 8 / Plan 08-04, commit 3c42c20）"
provides:
  - "PLAT-04 live 面重锁证据三件套（全部经用户 human-verify 批准 2026-08-18）：真 GLM 端点 cancel 传播实测 0ms（<200ms 门）+ 内网 Milvus tier-filter 声明外 tier 零行（个人 tier 在 expr 层排除、终判 no_authorized_source 结构化拒答）+ 冻结 eval 重跑 gate PASS（faithfulness 0.9720 ≥ 0.9000, refusal_accuracy 95%）"
  - "v0.8 里程碑 eval 数值基线留档（judge=glm-5.2, 50/50 case, lock sha256 50bb1688… 逐字节未动）：faithfulness 0.972 / context_precision 0.10 / answer_relevance 0.1002 / context_recall 0.39 / refusal_accuracy 95%（19/20, scripts/eval_snapshot 派生）"
  - "Phase 8 全收口：PLAT-04 + PLAT-05 双双 mark-complete（PLAT-05 勾选按 08-04-SUMMARY 遗留指示随本 plan 关闭），phase 可进 /gsd:verify-work"
affects:
  - "Phase 9 历史注入 spike 硬门（本基线 = golden-set A/B 实验的 baseline 侧输入；context_precision 0.10 / multi-hop 全拒答两项观察项移交 Phase 9+ 关注）"
  - "Phase 10-13（全部站在 eino v0.9.14 编译同构 + 行为同构双证据的地基上）"
tech-stack:
  added: []
  patterns:
    - "live 重锁口径：编译同构 ≠ 行为同构（A1）——升级后仅存的行为面证据缺口以 checkpoint:human-verify 三门一次收口，执行者 automation-first 采证 + 人类批准"
    - "冻结 eval「重新锁定」口径（无既有数值基线时）：frozen run 完成 + gate 阈值通过 + 指标留档为新里程碑基线；lock 文件绝不重生成（重生成 = 篡改基线掩盖漂移，T-8-12）"
key-files:
  created: []
  modified: []
decisions:
  - "D8-07: v0.8 frozen eval 基线口径与数值——faithfulness 0.972 / refusal_accuracy 95% 为 gate 指标（PASS 留档）；context_precision 0.10 / answer_relevance 0.1002 / context_recall 0.39 偏低成因（拒答 case 被 judge 机械归零 = D-09 refusal-by-construction 结构性后果；multi-hop 全拒答 = 检索阈值偏保守）已知悉接受，改善归 Phase 9+ 关注，不构成本阶段 blocking"
requirements-completed: [PLAT-04]
metrics:
  duration: ~50m
  completed: 2026-08-18
---

# Phase 8 Plan 05: live 三门重锁（human-verify）Summary

**One-liner:** 零代码验证 plan——eino v0.9.14 升级后的三项 live 行为门全部重锁并经用户逐门批准（2026-08-18）：真 GLM cancel 传播实测 **0ms**（两次稳定复现，亚毫秒级，远低于 200ms 门）；内网 Milvus tier-filter 声明 tier 查询 **5 行**、声明外 tier 查询 **0 行**（个人 tier 在 Milvus expr 层排除，终判 no_authorized_source 结构化拒答，零泄漏）；冻结 eval 50/50 case 重跑 gate **PASS**（faithfulness **0.9720** ≥ 0.9000 阈值，exit 0；lock sha256 50bb1688… 逐字节一致），Overall 指标留档为 v0.8 里程碑基线（refusal_accuracy 95% = 19/20）。

## What Was Verified（三门证据）

零代码 plan：无任务提交（automation-first 采证在 executor 会话内完成，全部凭据经 shell 环境变量来自 untracked `scripts/test`，未进入任何被跟踪文件——T-8-11）。三门全部为 `checkpoint:human-verify gate="blocking"`，用户 2026-08-18 逐门回复 "approved" 关闭。

### Task 1（Gate 1）: live cancel 传播（真 GLM 端点）— APPROVED，确认 <200ms

- `go run ./scripts/spiked e2e` 两次运行均 exit 0；**cancel 传播实测 0ms**（两次稳定复现——`int(elapsed.Milliseconds())` 截断口径下为亚毫秒级，远优于 Phase-1 SC-3 的 200ms 门）。
- 5 个 query 对真 GLM 端点验证通过（~1.7s/case round-trip），SSE 干净关闭、无挂起错误；probe 内置 400ms stuck-cleanup 探测器干净退出（无 stuck 泄漏）。
- 运行二进制确认为升级后 HEAD：`go list -m github.com/cloudwego/eino` = **v0.9.14** + eino-ext openai **v0.1.13**（TestContractVersionPin 已在 08-01 锁定同 pin）。
- 用户批准语：确认 <200ms。

### Task 2（Gate 2）: 内网 Milvus tier-filter 零泄漏 — APPROVED，确认零泄漏

- **声明 tier（权威）查询**「出差住宿费超过500元合规吗」→ **5 candidates（5 行）** + 2 个 source-document 引用（authority/travel_expense.md chunk 7、authority/expense_reimbursement_scope.md chunk 5）+ needs_input / confidence 0.8——声明内检索正常供给。
- **声明外 tier 查询**「查询张三的手机号码」→ 仅 tier=权威 参与检索，**个人 tier 在 Milvus expr 层被排除（0 行）**，终判 `no_authorized_source` 结构化拒答——零个人层泄漏（Phase-1 SC-2 同口径；D7/D9 tier 边界护城河行为证据）。
- SC-2 low_retrieval_score 结构化拒答路径同时验证（default-deny 保持）。
- 附带观察：SC-1b 三项 PASS；SC-1a FAIL 为**升级前已知漂移**（quick task 260814-vga 默认 Skill 切换为 simple-qa 的决策后果），与本阶段升级无关，不构成 blocking。
- 用户批准语：确认零泄漏。

### Task 3（Gate 3）: 冻结 eval 重跑 — APPROVED，接受为 v0.8 基线

- `go run ./cmd/eval --frozen=compliance` **50/50 case** 完成（~15min，judge=glm-5.2）；frozen pin 保持（model_ver=glm-5.2，prompt_hash=bd59cbe0…）。
- gate check：`go run ./cmd/eval -gate-check=eval/report.json -skills-dir=examples/skills`（阈值 = examples/skills/compliance/eval-gate.json 的 faithfulness_threshold 0.9）→ **PASS：faithfulness 0.9720 ≥ 0.9000**（exit 0）。
- **refusal_accuracy 95%（19/20）**，经 `scripts/eval_snapshot` 派生（Phase-7 工具，DIST-09 同源推导口径）。
- 基线完整性：`eval/frozen/compliance.lock.json` sha256 逐字节一致（**50bb1688…**），`git status eval/frozen/` 干净——lock 未被重生成（T-8-12 mitigate 落实）。
- **v0.8 里程碑基线留档（Overall）**：faithfulness **0.972** / context_precision **0.10** / answer_relevance **0.1002** / context_recall **0.39** / refusal_accuracy **95%**。
- 非 gate 指标偏低成因已被用户知悉并接受：① 拒答 case 被 judge 机械归零（D-09 refusal-by-construction 的结构性后果——正确拒答不含可 entail 的 claims）；② multi-hop 类全拒答（检索阈值偏保守）。两者均移交 Phase 9+ 关注，不构成本阶段 blocking（D8-07）。

## Must-Haves Truths 验证

| Truth | 状态 | 证据 |
|-------|------|------|
| live 层 cancel 重新锁定：agent run cancel 从客户端到 agent <200ms（真 GLM 端点） | ✅ | 实测 0ms（两次复现，亚毫秒截断口径）；spiked e2e ×2 exit 0；400ms stuck-cleanup 探测器干净退出；用户 approved 2026-08-18 |
| live 层 tier-filter 重新锁定：Milvus 对声明外 tier 返回零行（内网 Milvus） | ✅ | 声明 tier 5 行 / 声明外 0 行；个人 tier 在 expr 层排除；no_authorized_source 结构化拒答；用户 approved 2026-08-18 |
| 冻结 eval 基线在新依赖下重新运行：frozen run 完成 + gate check 通过，作为 v0.8 里程碑基线留档 | ✅ | 50/50 case；gate PASS（0.9720 ≥ 0.9，exit 0）；lock sha256 50bb1688… 一致；指标留档如上；用户 approved（接受为 v0.8 基线）2026-08-18 |

## Verification（plan `<verification>` 三项）

| # | 要求 | 结果 |
|---|------|------|
| 1 | 三项 checkpoint 逐一关闭（approved + 证据数值入 SUMMARY） | ✅ 三门全部 approved（2026-08-18），数值见上 |
| 2 | `git status` 确认 scripts/test / eval/frozen/ / eval/report.json 均未被提交 | ✅ 三者全部保持 untracked/未改动（eval/report.json + eval/calibration.json 含 recommended_threshold=0.4735 保持 untracked） |
| 3 | 08-05-SUMMARY 含三门证据：cancel 计时、两 tier 查询行数、eval Overall 指标 + gate 判定 | ✅ 本文件 |

## Deviations from Plan

**1. [Rule 3 - 阻塞问题] 陈旧 v0.7.7 时代 server 进程占用 :8888**
- **Found during:** Task 2（Gate 2）采证前
- **Issue:** 端口 :8888 上残留 2026-08-17 14:52 启动的 v0.7.7 时代 server.exe，会导致取证跑在旧二进制上。
- **Fix:** `taskkill` 清除陈旧进程 → 以升级后 HEAD（cf03a1b）重起 server 采证 → 验证完成后停止并释放端口。
- **Files modified:** 无（运行时操作）。

**2. [Rule 1 - Bug/环境] live-verify.sh 中文 curl body 被 ANSI 代码页（GBK）损坏**
- **Found during:** Task 2（Gate 2）采证
- **Issue:** Windows shell 下 curl 内联中文 query 体被 GBK 代码页转码，服务端收到损坏字节。
- **Fix:** 改用 `--data-binary @UTF-8文件` 重发（query 体预先写为 UTF-8 文件）；重发后服务端 UTF-8 守卫行为正确，两断言成立。
- **Files modified:** 无（scripts/live-verify.sh 本体未改，仅调用方式适配）。

**3. [有意跳过] 未运行 `go run ./scripts/spiked milvus`**
- **Found during:** Task 2（Gate 2）工具选择
- **Issue:** 该子命令会 DROP 并重灌 skillforge_knowledge collection——摧毁 Task 3 冻结 eval 所依赖的语料索引。
- **Fix:** 改用 `bash scripts/live-verify.sh`（只读验证路径）+ 升级后 HEAD 起的 server 实测两查询完成同一断言集；spiked milvus 保持不跑。
- **Files modified:** 无。

**4. [按 plan verification #2] eval 产物保持 untracked**
- eval/report.json + eval/calibration.json（recommended_threshold=0.4735）运行后保持 untracked 未提交（plan verification #2 明确「除非用户要求纳入」；用户未要求）。
- **Files modified:** 无。

---

**Total deviations:** 4（1 Rule 3 + 1 Rule 1 + 2 有意处置）——全部为采证过程运行时处置，零代码改动、零修复提交，与「零代码 plan」性质一致。

## Auth Gates

None——live 凭据（GLM key、内网端点）全部经 shell 环境变量传递（来源 untracked `scripts/test`），无认证阻塞。三个 blocking human-verify checkpoint 是 plan 自身的验证流程（用户 2026-08-18 逐门批准），非 auth gate。

## Known Stubs

None——零代码 plan，无 UI / 无占位数据。两项**有意 untracked** 产物（非 stub）：eval/report.json（gate-check 输入，已消费）、eval/calibration.json（recommended_threshold=0.4735，D2-15 手动 ops 交换位，维持既有处置）。

## Threat Flags

None——无新增安全面。威胁登记表三项 mitigate 全部落实：**T-8-11**（凭据只经环境变量；本 SUMMARY 只记指标/计时/行数，无 key、无内网拓扑地址）✅；**T-8-12**（lock sha256 逐字节一致，`git status eval/frozen/` 干净，未重生成）✅；**T-8-13**（三门全部在 phase 最终 HEAD cf03a1b 重跑留档，无跳门进 verify-work）✅；T-8-SC（accept）维持——零安装、零代码。

## Requirements Mark-Complete 决策

- **PLAT-04（本 plan frontmatter requirement）→ mark-complete**：全部构成项已闭环——升级+ext 联动（08-01，36845c0）/ contract 漂移套件（08-01）/ 冻结 eval 基线（本 plan Gate 3）/ Phase-1 live verify 重锁（本 plan Gate 1+2）/ legacy interrupt lint 禁用（08-03，fea6114）。
- **PLAT-05 → 同步 mark-complete**：按 08-04-SUMMARY「Requirements Mark-Complete 决策」的显式遗留指示（勾选留待 08-05 收口）——store 骨架（08-02）+ `schema.RegisterName[T](name)` 注册面（08-02/08-04）+ 真实中断运行往返测试（08-04，3c42c20）全部满足，且 phase live 门现已全关。

## Self-Check: PASSED

- 本 plan 零代码：created/modified 文件清单为空（✅ 符合 plan `files_modified: []` / `<artifacts: []>`）
- 三门证据数值全部留档于本文件（cancel 0ms；tier 5 行/0 行；faithfulness 0.9720、refusal_accuracy 95%、50/50、lock 50bb1688…）
- REQUIREMENTS.md：PLAT-04 + PLAT-05 勾选 + traceability 状态 Complete（本提交内验证）
- STATE.md / ROADMAP.md：5/5 plans、Phase 8 完成态更新（本提交内验证）

---
*Phase: 08-p8-0-eino-v0-7-7-v0-9-13*
*Completed: 2026-08-18（三门 human-verify 批准日）*
