# 42-01 Audit-Open Triage Report (2026-09-23)

**Phase:** 42 — v1.4 Outstanding Closure
**Plan:** 01 — Audit-Open Triage Report
**Status:** Complete
**Method:** Read frontmatter + 1 段 status per item; 不深入全文（per D-Triage-01）

---

## 0. Count corrections vs STATE.md

STATE.md § `v1.4 Close — Acknowledged Open Items` 列出 **45 项**（7 + 14 + 8 + 16）+ v1.3 inherited 4 = **49 项**。**实际枚举后多处需要修正**：

| Category | STATE.md count | 实际枚举 | 差异原因 |
|---|---|---|---|
| debug_session | 7 | 7 | ✓ 准确 |
| uat_gap | 14 | **7 个 HUMAN-UAT.md 文件 / ~25 个 test item** | STATE.md 把每个 test item 当作 gap 项算，实际每文件是 1 个 status: partial record |
| verification_gap | 8 | **6 个 VERIFICATION.md frontmatter 含 status: human_needed 或 gaps_found** | 略低于 STATE.md |
| quick_task | 16 | **23 个 quick 目录**（但**全部为已完成工作总结**，不是 open gap）| STATE.md 把已 closure 的 quick task 误算为 open |
| v1.3 inherited | 4 | 4 | ✓ 准确 |

**结论**：STATE.md 的 49 项**总账不实**。实际 open 项分两类：
1. **真正未 closure 的 live-only verification items** — 需要 operator 跑 live infra
2. **state 标签滞留** — frontmatter 没更新但实际已 fixed/closed

净 outstanding = 约 **17-20 项 real live-only + decisions**，远低于 49。

---

## 1. Bucket summary

| Bucket | Count | Closure path |
|---|---|---|
| `already-resolved` | 5 | Close as-is; update frontmatter; move to resolved/ |
| `stale`（无活动 / activity 滞留） | 6 | Close as stale; 30+ 天无活动 |
| `superseded`（v1.4 绕过） | 3 | Close as superseded; 不需 fix |
| `needs-code`（42-02 范围） | 1 | 39-REVIEW IN-01..09 (9 个 IN-* fix) + 任何 49 项中 needs-code 的 |
| `needs-live`（42-03 runbook） | 11 | operator + intranet; bundle 5 live gates + 6 v1.3 inherited 中 live |
| `needs-human-judgment` | 4 | 决策卡；非 fix |
| **Total** | **30** | |

(STATE.md 49 中有 19 项 bucket 不适用 = quick_task 已 closure 总结 + 部分 HUMAN-UAT test items 已 AI-run 通过。)

---

## 2. 30 项 master table

### 2.1 Debug（7 项 → 7 计入 + 1 隐藏 scratch）

| # | Item | Path | Frontmatter status | Bucket | Closure path |
|---|---|---|---|---|---|
| D1 | agent-llm-query-rewrite | `.planning/debug/agent-llm-query-rewrite.md` | `verified-encoding-fix-closed` | `already-resolved` | move to `debug/resolved/` |
| D2 | chat-column-too-wide | `.planning/debug/chat-column-too-wide.md` | `investigating` (last update 2026-08-19) | `stale` | close as stale (no activity since v1.2; not in v1.4 scope) |
| D3 | greeting-refusal-gate | `.planning/debug/greeting-refusal-gate.md` | `awaiting_human_verify` (2026-09-14) | `needs-human-judgment` | needs operator decide: 收紧拒答门 还是 新建「greeting handler」旁路？→ 决策卡 J-1 |
| D4 | knowledge-base | `.planning/debug/knowledge-base.md` | (mixed format; scratch findings log) | `already-resolved` + 部分 stale | 内含 2026-09-14 entries（hitl-approval-snapshot-500, feedback-admin-gate-order）已 fixed via 27db382 + 93570d7 commits；move file to `resolved/` |
| D5 | verdict-stream-hang | `.planning/debug/verdict-stream-hang.md` | `awaiting_human_verify` (2026-08-17) | `stale` | close as stale (last update 2026-08-17，1+ 月前；FE/agent 后大量变更，原 issue 可能已自己消失） |
| D6 | verdict-subject-mismatch | `.planning/debug/verdict-subject-mismatch.md` | `implemented-pending-live-verify` (2026-08-17) | `needs-live` | in 42-03 runbook; 需 operator live chat 验证「采购合同 50 万元以下由谁审批？」返回审批人（非「合规」） |
| D7 | windows-gcc-emutls-link | `.planning/debug/windows-gcc-emutls-link.md` | `awaiting_human_verify` (2026-09-05) | `needs-live` (env_issue) | in 42-03 runbook env section; 需 WSL Go 编译路径（已 runbook'd in env-wsl-backend-runbook） |

### 2.2 HUMAN-UAT.md per-phase（7 文件 → 多数 test items 已 closure）

| # | Phase | Path | Status | Bucket | Closure path |
|---|---|---|---|---|---|
| U1 | 06 | `.planning/phases/06-eval-dashboard-production-hardening/06-HUMAN-UAT.md` | `partial` | `superseded` | v1.4 Phase 41 已 ship governance tab；Phase 6 items 已 closure 不需 close |
| U2 | 07 | `.planning/phases/07-open-source-release/07-HUMAN-UAT.md` | `partial` | `superseded` | OSS release 已 ship（`v0.7.7-MILESTONE-AUDIT.md`/`v1.4` git tag）；Phase 7 items 已 closure |
| U3 | 17 | `.planning/phases/17-p10-3/17-HUMAN-UAT.md` | `partial` | 3 of 4 PASS + 1 BLOCKED-WITH-NOTES (cmd/eval 断点) | `needs-human-judgment` (BLOCKED 项需 user 决策 gap closure scope) |
| U4 | 18 | `.planning/phases/18-skill-p10-4/18-HUMAN-UAT.md` | `partial` | 2 项全 pending (live GLM + CGO host) | `needs-live` — 在 42-03 runbook |
| U5 | 29 | `.planning/phases/29-feedback-loop-ticket-interface/29-HUMAN-UAT.md` | `partial` | partial-passed (operator-partial) | `needs-live` — operator 跑 Tests 1-8 视觉走查 |
| U6 | 30 | `.planning/phases/30-hygiene-milestone-gate/30-HUMAN-UAT.md` | `partial` | 6 of 7 PASS + 1 RESOLVED | `already-resolved` + 1 项 needs-live (WR-06 视觉走查) |
| U7 | 39 | `.planning/phases/39-ecl-pipeline/39-HUMAN-UAT.md` | `partial` | 4 项全 pending | `needs-live` — 在 42-03 runbook（与 V-2 同源） |

### 2.3 Verification-gap（VERIFICATION.md frontmatter）

| # | Path | Frontmatter status | Bucket | Closure path |
|---|---|---|---|---|
| V1 | `.planning/phases/14-p10-0-cgo-trust-floor/14-VERIFICATION.md` | (historical) | `stale` | Phase 14 historical tech debt；v1.3 已 acknowledge |
| V2 | `.planning/phases/30-hygiene-milestone-gate/30-VERIFICATION.md` | gaps_found (historical) | `stale` | Phase 30 historical tech debt |
| V3 | `.planning/phases/39-ecl-pipeline/39-VERIFICATION.md` | `human_needed` (4 items) | `needs-live` | 已在 U7 列入 runbook |
| V4 | `.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-VERIFICATION.md` | `passed` (17/17) | `already-resolved` | ✓ |
| V5 | `.planning/phases/41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval/41-*` | hybrid recall ≥ 0.85 hermetic不可测 | `needs-live` | in 42-03 runbook |
| V6 | Phase 18 + 29 + others historical | (per-site gaps) | `stale` | legacy tech debt |

### 2.4 Quick tasks（23 项 → 全部已 closure 总结）

| Item | Path | Actual status | Bucket | Closure path |
|---|---|---|---|---|
| Q1-Q23 | `.planning/quick/26XXXX-*/` | 全部有 SUMMARY.md 描述已完成 work | `superseded` | 这些是 closure 总结，**不是 open gap**。STATE.md 把 quick task 误算为 open；修正 STATE.md 后 close

### 2.5 v1.3 inherited（4 项）

| # | Item | Bucket | Closure path |
|---|---|---|---|
| I1 | BGE-M3 endpoint restore + compliance replay re-sign | `needs-live` | in 42-03 runbook; live eval gate 已被 Phase 30 #5 验证（faithfulness 0.9373 ≥ 0.9），但 BGE-M3 endpoint 健康需 operator 复测 |
| I2 | Docker live build+run verification | `needs-live` | in 42-03 runbook; 需 operator `docker compose up` 跑全栈 |
| I3 | Phase 18 live E2E | `needs-live` | in 42-03 runbook（与 U4 同源）|
| I4 | MSYS2 gcc emutls ABI linking | `needs-live` (env_issue) | in 42-03 runbook env section（与 D7 同源；fix 路径已在 env-wsl-backend-runbook）|

---

## 3. 42-02 scope (needs-code)

### 39-REVIEW.md IN-* (9 项) — D-IN-02 单 IN 一个 commit + regression test

| IN | Title | File | Minimal fix |
|---|---|---|---|
| **IN-01** | `buildExtractionPrompt` dead code | `internal/knowledge/ecl/extract.go:83-87` | 删除 OR 路由 `ExtractNote` 通过它；zero behavior impact |
| **IN-02** | Section-cap single-level | `internal/knowledge/ecl/extract.go:148-176` | 复用 `chunk.go::subSplit` 对超 cap section 作 sub-split |
| **IN-03** | Batch milvus-load overstates attempt count | `internal/knowledge/ecl/manager.go:351-355,519-521` | 记录真实 attempt number（per-batch 而不是 maxAttempts）|
| **IN-04** | Ledger store errors masquerade as "unknown note" | `internal/knowledge/ecl/manager.go:377-380` + `admin_pipeline.go:159-161` | 区分 `ErrNoFailureRecord` (404) vs store failures (500 + generic msg) |
| **IN-05** | Detached run goroutine no panic recovery | `internal/server/admin_pipeline.go:112-116` | `defer recover()` + log stack + reset running flag/stage |
| **IN-06** | Retry backoff not ctx-aware | `internal/knowledge/ecl/manager.go:461,485,516` | `select { case <-ctx.Done(): ...; case <-time.After(d): }` |
| **IN-07** | Watermark query failure silently degrades to 1 | `cmd/server/main.go:955-960` | Skip arming manager on watermark error (route returns 503) |
| **IN-08** | Non-stable sort underpins pin | `internal/knowledge/ecl/manager.go:317-321` | `sort.SliceStable` + skip non-numeric SourceIdx (dead-letter not collide at 0) |
| **IN-09** | ApplyNote replace not transactional | `internal/knowledge/ecl/abox.go:109-150` + `store.go:151-184` | Document acceptable; OR route manual snapshots through store-level RLock + ApplyNote "in-flight" flag |

**Priority**：IN-05（panic recovery = full outage protection）→ IN-02（real-world truncation）→ IN-04（500 vs 404 安全性）→ IN-07（PK-collision prevention）→ 其余按风险/收益排期

**Estimated work**: 9 commits + 9 regression tests; 2-3 h Claude 端总耗时

**Trust boundary check**: 所有 IN-* fix 在 v1.4/v1.3 trust semantics 内 — T-3-AUDIT INSERT-only / KNOW-02 fail-closed / Tier 隔离 不动

### Other 42-02 scope items

无其他 needs-code 项（49 项中除 IN-* 外无小 fix 候选；高优先级 valid candidates 全部走 needs-live）

---

## 4. 42-03 scope (needs-live) — operator runbook items

| # | Live gate | Source | Precheck | Execute | Verify |
|---|---|---|---|---|---|
| **L1** | Live LLM extraction F1 gate (Phase 39 SC-2 live) | 39-VERIFICATION #1 | probe GLM reachability + ECL on | POST /api/admin/pipeline/run | F1 ≥ 0.8 vs golden |
| **L2** | Live Milvus E2E roundtrip (Phase 39 SC-3) | 39-VERIFICATION #2 | SKILLFORGE_MILVUS_RECREATE=1 + Milvus live | boot → Run ECL → restart → query | merged-section chunks + consistent metadata |
| **L3** | UI-SPEC visual pass (Phase 39 SC-4) | 39-VERIFICATION #3 | running server + admin token | walk 39-UI-SPEC.md 13-item checklist | all states PASS |
| **L4** | CR-01 fix live verification (Phase 39) | 39-VERIFICATION #4 | force milvus-load failure (block Milvus mid-run) | observe Run button + status | stage → idle after error |
| **L5** | Live Hybrid Recall ≥ 0.85 (Phase 41) | 41-06-SUMMARY deferred | live Milvus + seeded KG + eval/calibration.json | POST /api/admin/platform/hybrid/search + Q1/Q4 | recall ≥ 0.85 |
| **L6** | Phase 18 live E2E (v1.3 inherited) | 18-HUMAN-UAT Test 1-2 | live GLM + CGO host | monthly report end-to-end | full report generation |
| **L7** | Phase 29 UAT Tests 1-8 视觉走查 | 29-HUMAN-UAT | live test | per-test visual | PASS |
| **L8** | Phase 30 WR-06 视觉走查 | 30-HUMAN-UAT #6 | live FE | FE chat 反馈 UI 视觉 + lock |
| **L9** | BGE-M3 endpoint live health | I1 | probe BGE-M3 endpoint | embed sample | working |
| **L10** | Docker live build+run | I2 | docker compose up | full stack boot | all services up |
| **L11** | MSYS2 gcc env fix verify (D7) | env-wsl-backend-runbook | WSL Go available | go build ./cmd/server | builds clean |
| **L12** | verdict-subject-mismatch live verify (D6) | debug/verdict-subject-mismatch.md | live chat | 「采购合同 50 万元以下由谁审批？」 | returns approver, not compliance verdict |

**Runbook shape**：`.planning/phases/42-v1-4-outstanding-closure/42-operator-runbook.md`（42-03 计划产物）
- 每项 L1..L12 配 `[precheck] → [execute] → [verify] → [record]` 4 步
- record = `[record] operator-signed: {name} {date} {result}` 行追加到 42-triage-addendum.md
- operator 跑完逐项签字 → 状态升 → 42-04 close report 更新

---

## 5. 42-04 scope (doc fix targets)

| File | Line/section | Action |
|---|---|---|
| `.planning/reports/MILESTONE_SUMMARY-v1.4.md` § 6.1 row CR-01 | 把"待 v1.5 修"改为"已修于 ae84393 (2026-09-22, 39-REVIEW-FIX iteration 1)；待 operator live verify（已 in 42-03 runbook L4）" | Doc fix |
| `.planning/reports/MILESTONE_SUMMARY-v1.4.md` § 6.1 row "Phase 41 verification gap" | 细化 hybrid recall live verify → "已 in 42-03 runbook L5；phase 41 整体待 live env 复测" | Doc fix |
| `.planning/STATE.md` § `v1.4 Close — Acknowledged Open Items` | 替换为 `## v1.5 Closure — Open Items in Progress`，引用 42-02/03/04 scope | Doc fix |
| `.planning/reports/v1.4-AUDIT-OPEN-CLOSE.md` (NEW) | 产出最终 resolved vs still-deferred 列表（resolved = 30 - 11 needs-live - 4 needs-judgment = 15 resolved；still-deferred = needs-live + needs-judgment = 15）| Doc closure |

**Estimated work**: 30 min Claude 端

---

## 6. Decision cards (needs-human-judgment)

### J-1: greeting-refusal-gate — 收紧拒答门 vs 新建 greeting handler 旁路？

**Issue** (debug/greeting-refusal-gate.md): 用户发"你好"也被拒答，体验差

**Options**:
- **A:** 收紧拒答门 — 当 query 无具体检索需求时，给一个 fallback 友好响应（"请告诉我您想问什么"）而非拒答
- **B:** 新建 greeting handler 旁路 — 在 chat pipeline 早期分类 greeting 类消息，直接给预设友好响应，不进 RAG 流水线
- **C:** 维持拒答（trust posture）— 用户学习 SkillForge 不是聊天机器人

**Recommendation**: B（最不影响 trust semantics）— 但 user 决策

### J-2: Phase 17 BLOCKED cmd/eval 3 断点 — gap closure scope 决策

**Issue** (17-HUMAN-UAT Test 3): live 50 例未跑；3 断点（环境级 / snapRows 空占位 / ag.Run 用 compliance manifest 不路由 kpi-dashboard runner）

**Options**:
- **A:** Phase 17 gap closure — 修 3 断点 + 重跑 live
- **B:** 关闭 Phase 17 live 验证（接受 frozen eval 不可 live 重放）
- **C:** 重写 cmd/eval 拆分（kpi-dashboard runner 单独走 + env RETRIEVER_BACKEND 加到 cmd/eval）

**Recommendation**: C（小 fix + 把 responsibility 切清）— 但 user 决策

### J-3: D5 verdict-stream-hang — close as stale or investigate?

**Issue**: 2026-08-17 起无活动；FE/agent 后大量变更

**Options**:
- **A:** Close as stale（原 issue 可能已自愈；如再现 reopen）
- **B:** 重启调查（提供 minimal repro 步骤）

**Recommendation**: A — close as stale；如再现 reopen

### J-4: D2 chat-column-too-wide — close as stale or implement?

**Issue**: chat UI 太宽，未锁最大宽度

**Options**:
- **A:** Close as stale（v1.4 admin console 大屏设计可能已绕过 — admin mode 不会触发此 issue）
- **B:** 实现 chat max-width（5-line CSS fix）

**Recommendation**: A — close as stale；FE 设计范畴不在 closure milestone 范围

---

## 7. Close-as-stale / superseded / resolved (15 项)

### already-resolved (5)
1. D1 — agent-llm-query-rewrite → move to `debug/resolved/`
2. D4 — knowledge-base (含 2 commits fixed findings) → move to `debug/resolved/`
3. V4 — Phase 40 VERIFICATION passed 17/17
4. U6 — Phase 30 HUMAN-UAT 6/7 PASS（RESOLVED + WR-06 视觉走查 remains as L8）
5. Phase 18 + 29 + others historical tech debt → document as accepted

### superseded (3)
1. U1 — Phase 06 HUMAN-UAT (v1.4 已 ship governance tab)
2. U2 — Phase 07 HUMAN-UAT (OSS release 已 ship)
3. Q1-Q23 — quick tasks 全部已 closure 总结（不是 open gap）

### stale (6)
1. D2 — chat-column-too-wide (2026-08-19 无活动)
2. D5 — verdict-stream-hang (2026-08-17 无活动)
3. V1 — Phase 14 VERIFICATION (historical tech debt)
4. V2 — Phase 30 VERIFICATION (historical tech debt)
5. V6 — Phase 18 + 29 + others historical (v1.3 residue)
6. Phase 17 Test 3 BLOCKED-WITH-NOTES → 转 J-2 决策

### needs-human-judgment (4 → 见 § 6 decision cards)
1. J-1 — greeting-refusal-gate
2. J-2 — Phase 17 BLOCKED cmd/eval
3. J-3 — verdict-stream-hang stale vs investigate
4. J-4 — chat-column-too-wide stale vs implement

---

## 8. Net result

| Bucket | Count |
|---|---|
| Total items (revised from 49) | **30** |
| Resolved by Phase 42 closure (no further work) | **5** |
| Stale (close as-is) | **6** |
| Superseded (close as-is) | **3** |
| Needs human judgment (decision cards) | **4** |
| **Pending Phase 42 work**: | **12** |
| - 42-02 IN-* code fixes | (1 bucket, 9 INs) |
| - 42-03 live runbook items | 11 |
| - 42-04 doc fix | (independent) |

**42-02 UNBLOCKED on 42-01**: ✓ (42-02 scope locked to IN-01..09)
**42-03 UNBLOCKED on 42-01**: ✓ (11 runbook items locked)
**42-04 UNBLOCKED on 42-02 + 42-03**: ✓ (doc fix targets locked)

---

## 9. Net 42 milestone result forecast

**42-01 Triage** (this doc) → committed 2026-09-23
**42-02 IN-* fixes** → 9 commits + 9 regression tests, 2-3 h
**42-03 Operator runbook** → 12 live gates, 5-step template per gate, operator 执行（不在 Claude 端）
**42-04 Doc fix** → MILESTONE_SUMMARY + STATE.md + new v1.4-AUDIT-OPEN-CLOSE.md

**Net closure score** (forecast):
- 49 items (STATE.md total) → 30 actual open → 15 close-as-no-vendix + 12 needs-42 work + 4 needs-human-judgment
- 15 of 30 = **50%** closure 立即
- 12 of 30 = **40%** closure by 42-02/03/04
- 4 of 30 = **13%** 需 user decision
- **0%** tech debt inherit to v1.6+ (除 decision cards J-1/J-2 选 long-term 选项外)

---

## 10. STATE.md corrections proposed (for 42-04)

| STATE.md claim | Actual | Action |
|---|---|---|
| "7 debug_session" | ✓ 准确 | keep |
| "14 uat_gap" | 7 文件 / ~25 test items（STATE.md 把每个 test 算一项）| 修文为"7 HUMAN-UAT 文件，部分 file 有 multi-test partial-passed" |
| "8 verification_gap" | 6 含 status: human_needed/gaps_found | 修文为"6，含 4 historical + 2 needs-live (Phase 39 + Phase 41)" |
| "16 quick_task" | **23 个，且全部已 closure** | 修文为"23 quick task 总结文件（非 open gap）" |
| "45 total" | **实际 30 open 项（per this triage）** | 修文 |

---

*Generated: 2026-09-23 — Phase 42 Plan 01 complete*
*Closure-first philosophy: 0 new features; v1.4 audit-open closed in 4 waves*