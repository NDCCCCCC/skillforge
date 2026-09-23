# 42-Decisions — Audit-Open Decision Cards Resolution (2026-09-23)

**Phase:** 42 — v1.4 Outstanding Closure
**Source:** `42-triage.md` § 6 Decision cards (J-1..J-4)
**Resolved by:** User approval ("按你的推荐来" — 2026-09-23)
**Status:** All 4 decisions LOCKED

---

## J-1: greeting-refusal-gate — LEN/MISS/维持拒答？

**Issue:** debug/greeting-refusal-gate.md — 用户发"你好"被拒答"无法给出结论：未检索到可回答该问题的授权知识源"，拒答门对非知识型问题过激，体验差

**Recommendation:** **B — 新建 greeting handler 旁路**

**Rationale:**
- 不削弱 KNOW-02 fail-closed trust posture（拒答门本身不动）
- 最小侵入 — 在 chat pipeline 早期分类 greeting 类消息，直接给预设友好响应
- greeting handler 旁路应配置驱动（`SKILLFORGE_GREETING_ENABLED`），如未来想收紧可一键 disable

**为什么不是 A（收紧拒答门）**: 拒答门是 KNOW-02 的核心 trust contract，改它影响整个 RAG pipeline；greeting handler 是 additive 隔离层
**为什么不是 C（维持拒答）**: 体验问题已被 operator 多次反馈（2026-09-14 status: awaiting_human_verify 反映此 issue 不止一次）

**Closure path:**
- Status 改为 `decided: option-B` 
- 转 `42-02-closure` 范围 — 写新 IN-* (IN-10) 加 39-REVIEW 范围？OR 新建 .planning/phases/43-greeting-handler/（v1.6 候选）
- 当前：status → `awaiting_implementation`；42-02 不含此（IN-* fix 限定 39-REVIEW IN-01..09）；放 v1.6 backlog

**Locked decision:** B (新建 greeting handler 旁路，配置驱动；实现待 v1.6)

---

## J-2: Phase 17 BLOCKED cmd/eval 3 断点 — gap closure 决策

**Issue:** 17-HUMAN-UAT Test 3 BLOCKED-WITH-NOTES — live 50 例未跑；3 断点：
1. **环境级**: cmd/eval 的 retriever 构造不认 `RETRIEVER_BACKEND=none`
2. **代码级**: `runKPIMode` 的 snapRows 为空占位
3. **代码级**: `ag.Run` 用 compliance manifest（firstManifest），不路由 kpi-dashboard runner

**Recommendation:** **C — 重写 cmd/eval 拆分（kpi-dashboard runner 单独走 + env RETRIEVER_BACKEND 加到 cmd/eval）**

**Rationale:**
- 3 断点都是 cmd/eval 单 binary 跨多个 runner 责任不清的结果
- C 方案：(a) cmd/eval 加 `RETRIEVER_BACKEND=none` env support（移植自 cmd/server）→ (b) kpi-dashboard runner 拆为 cmd/kpi-eval 独立 binary（OR `--runner=kpi-dashboard` flag）→ (c) snapRows 真实注入（用 BuildRunner 数据通道）
- A = gap closure 在 Phase 17 范围内修，但 Phase 17 已 shipped 14 天，无 momentum
- B = 关闭 Phase 17 live 验证，把 eval 责任让给 cmd/eval；这放弃 KPI 准确率门（GEVL-09 KPI 准确率 eval gate 是 v1.0 已有需求）

**为什么不是 A（Phase 17 gap closure）**: Phase 17 已 archived 1+ 月；reopen 破坏 v1.0 archive
**为什么不是 B（关闭 live 验证）**: GEVL-09 KPI 准确率门已 v1.0 ship；放弃它破坏 trust posture

**Closure path:**
- 转 v1.6 backlog 候选（v1.5 范围不修改 shipped Phase 17）
- 42-04 doc fix：在 `v1.4-AUDIT-OPEN-CLOSE.md` 标注此卡 deferred to v1.6
- 在 42-04 节增加 "Phase 17 BLOCKED cmd/eval" → "v1.6 eval runner refactor"

**Locked decision:** C (重写 cmd/eval 拆分 — defer to v1.6 as Phase 43+ 候选；当前 Phase 17 status 不动)

---

## J-3: D5 verdict-stream-hang — close as stale or investigate?

**Issue:** debug/verdict-stream-hang.md — 2026-08-17 起 1+ 月无活动；FE/agent 后大量变更，原 issue 可能已自愈

**Recommendation:** **A — close as stale**

**Rationale:**
- 1+ 月无活动 + agent 后多次大变更（Eino 升级、streamAndAssemble 重构、wrappers、agent runner 等）
- 无 user/operator recent reproduce 反馈
- 闭环路径：close as stale；如再现 reopen + 转 `.planning/debug/resolved/stale-attic/` 备查

**为什么不是 B（重启调查）**: 无 reproduce steps、无新证据；当前无法证伪 issue 是否存在

**Closure path:**
- 42-01 立即 closure（已 in `42-triage.md` § 7 stale bucket）
- 不需 42-02/03/04 范围 work

**Locked decision:** A (close as stale)

---

## J-4: D2 chat-column-too-wide — close as stale or implement?

**Issue:** debug/chat-column-too-wide.md — 2026-08-19 起 1+ 月无活动；chat UI 太宽，未锁最大宽度

**Recommendation:** **A — close as stale**

**Rationale:**
- 1+ 月无活动
- v1.4 admin console 大屏设计（4 个 admin tab + 11th platform tab）后，chat mode 可能已被调整为单独面板（侧栏可关）— 原 issue 触发场景可能已绕过
- 5-line CSS fix 在 v1.6+ 候选；当前 closure milestone 范围内不改 UI 设计

**为什么不是 B（实现 chat max-width）**: FE 设计范畴；closure milestone 范围不动新 UI（per 42-CONTEXT "0 new features" hard constraint）

**Closure path:**
- 42-01 立即 closure（已 in `42-triage.md` § 7 stale bucket）
- 不需 42-02/03/04 范围 work

**Locked decision:** A (close as stale)

---

## Decision summary table

| Card | Title | Recommendation | LOCKED | Scope impact |
|------|-------|----------------|--------|--------------|
| J-1 | greeting-refusal-gate | **B** (新建 greeting handler 旁路) | ✓ | v1.6 backlog |
| J-2 | Phase 17 cmd/eval 3 断点 | **C** (拆分 cmd/eval → defer v1.6) | ✓ | v1.6 backlog |
| J-3 | verdict-stream-hang stale | **A** (close as stale) | ✓ | 42-01 closure |
| J-4 | chat-column-too-wide stale | **A** (close as stale) | ✓ | 42-01 closure |

## Scope impact on 42-02/03/04

| Plan | Original scope | After J-1..J-4 LOCKED |
|------|----------------|----------------------|
| 42-02 IN-* fixes | 9 items | **9 items** (无变化 — IN-* 不含 J-1/J-2) |
| 42-03 live runbook | 11 items | **11 items** (无变化 — D5 verdict-stream-hang closure 不需 live; D2 chat-column 不需 live) |
| 42-04 doc fix | 3 targets | **4 targets** (+ "v1.6 backlog decision cards" section) |

## v1.6 backlog net addition

- **greeting handler 旁路** (J-1)
- **cmd/eval 拆分 refactor** (J-2)

(2 items; v1.5 closure 不包含新功能 — 全 defer 到 v1.6)

## Signed

User: approved "按你的推荐来" (2026-09-23)
Claude: 记录 + commit + 推进 42-02 plan-phase

---

*This document LOCKED 2026-09-23 — 42-04 阶段将引用本文件作为决策依据。*