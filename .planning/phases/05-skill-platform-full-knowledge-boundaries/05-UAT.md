---
status: partial
phase: 05-skill-platform-full-knowledge-boundaries
source: [05-01-SUMMARY.md, 05-02-SUMMARY.md, 05-03-SUMMARY.md, 05-04-SUMMARY.md, 05-05-SUMMARY.md, 05-06-SUMMARY.md, 05-UI-SPEC.md]
started: 2026-08-12T13:46:53Z
updated: 2026-08-12T15:23:44Z
verification_mode: automated (chrome-devtools MCP) + backend HTTP (:8888) + unit-test evidence
dev_url: http://localhost:5173/ (Vite dev) → mock chat transport; backend http://localhost:8888 (Hertz, real data)
constraint: Dev FE uses MockChatTransport (env=dev) — SC-1/6/7 chat chunks verified via direct /api/chat + unit tests, not live dev UI
---

## Current Test

[testing paused — 6 items outstanding (Tests 6–11); 2 issues found: #2 right-panel IA (major), #4 graph chunk-content (major). Resume with `/gsd-verify-work 5` — first pending test is #6 (Agent Inspector).]

## Tests

### 1. Cold Start — Both Servers Boot Clean
expected: Backend :8888 + Vite :5173 up; Registry scanned skills/, Router + fsnotify wired; /api/skills + /api/knowledge/graph serve real data; app shell renders, no console crash.
result: pass
evidence: Backend GET /api/skills → 企业合规/制度审查 manifest; GET /api/knowledge/graph → real 4-tier tree (权威 7 docs/7 chunks). Frontend :5173 title `SkillForge — Skill 驱动的企业合规审查`, three-panel shell rendered. Console: 1 msg — harmless 404 ×3 (resource, likely favicon; same as Phase 04 baseline), no blocking/crash errors. (Verified live instance; no forced kill+restart.)

### 2. App Shell + 5-Tab CenterPanel (SC-1/2/4/5 entry)
expected: Three-panel layout. Center shows 5 tabs — 审查结论 / 引用原文 / Skill 定义 / Agent 状态 / 知识图谱. Topbar has SkillForge brand + Skill Manager menu + collapse buttons. Statusbar reads 就绪. (Phase 5 lifts D-02: tabs are bidirectional — clicking any tab switches center view.)
result: issue
sub_passed: 5 center tabs present + bidirectional nav verified (clicking Agent 状态 → selected + center switches to inspector empty state); topbar SkillForge + Skill 管理 + collapse; statusbar 就绪.
reported: "选择skill的下拉框在右上角，而他应该在下方显示'未选择skill'那里，会话列表应该是个抽屉，和chat互相切换，下拉框换成新建会话按钮。"
severity: major

### 3. Skill Manager Menu (SC-2 / SYSK-04)
expected: Topbar dropdown opens and lists every installed Skill with per-skill status. Includes the 新建 Skill（向导）entry point that enters wizard mode.
result: pass
evidence: Clicked topbar `Skill 管理` → dropdown rendered: 已安装 Skill heading; 企业合规/制度审查 (已加载); 通用问答 (已加载); `全部重载` button; `新建 Skill（向导）` button (wizard entry). Per-skill status + wizard entry both present.

### 4. Knowledge Graph Browser (SC-5)
expected: Click 知识图谱 tab → 3-level Accordion tree (tier → doc → chunk) loads live from GET /api/knowledge/graph. 4 tiers (权威 7 docs / 历史 / 个人 / 草稿). Tier label primary color. Filter narrows by filename/chunk ID. Clicking a chunk row opens ChunkSourceView in center.
result: issue
sub_passed: 3-level tree renders live (权威 7 docs/7 chunks; 草稿/历史/个人 0/0 with empty states); tier label primary; filter narrows (typed "annual" → 权威 7→1 doc, empty tiers switch 暂无文档→无匹配文档).
reported: "Clicking a graph chunk opens ChunkSourceView (center switches to 引用原文, tier [权威] + 版本:当前有效 + 生效日期:未知 render) but the body shows '(chunk not found in fixture)' — verbatim chunk text is NOT displayed when navigating from the graph browser."
severity: major
root_cause_candidate: "web/src/lib/chunk-lookup.ts:48 NOT_FOUND_TEXT='(chunk not found in fixture)'; content lookup only resolves fixture/verdict citations. Backend has NO chunk-content endpoint (routes: /api/chat, /api/skills, /api/skills/wizard[+/commit], /api/knowledge/graph). Graph endpoint is metadata-only per T-5-22 → graph-browsed chunks have no client-side content."

### 5. Version Badges + Filtered-Out Notice (SC-8)
expected: ChunkSourceView metadata row shows VersionBadge (当前有效 / 已失效) + 生效日期 (always) + 失效日期 (only when effective_to set). Superseded chunk → FilteredOutNotice banner. (Live corpus all-perpetual → 当前有效 observed; 已失效/FilteredOutNotice pinned by unit tests.)
result: pass
evidence: Graph chunk rows + ChunkSourceView show `当前有效` VersionBadge + `生效日期: 未知` (effectiveFrom=0 sentinel → 未知). 已失效 / 失效日期 / FilteredOutNotice not observable live (corpus is all effectiveTo=0 perpetual) — pinned by VersionBadge.test.tsx (5 cases) + FilteredOutNotice.test.tsx (2 cases) + ChunkSourceView.test.tsx SC-8 cases.

### 6. Agent Inspector (SC-4)
expected: Click Agent 状态 tab → Accordion hop tree derived from the latest assistant turn. Empty state 暂无 Agent 状态. With a streamed turn: hops grouped by data-decision step; active hop 2px primary left-border; tool calls expandable; reasoning collapsed; retrieval rows clickable. (messages via props, NOT Zustand — D14.)
result: [pending]

### 7. Hot-Reload Toast (SC-2 / D5-07/D5-08)
expected: Touch/save a skills/<name>/SKILL.md on disk → within ~1s debounce a ReloadToast appears (success on valid change; validation-error toast on broken YAML, FS paths stripped). D5-08: validation failure keeps old snapshot.
result: [pending]

### 8. Skill Wizard — NL Interview (SC-3 / SYSK-06)
expected: Click 新建 Skill（向导）→ right panel enters wizard mode (header 步骤 {N}/5 + 取消向导; ChatInput 模式：Skill 向导). Answer 5 NL prompts (purpose/reads/type/guardrails/temperature). 生成预览 → POST /api/skills/wizard returns 7-field preview; center auto-switches to Skill 定义.
result: [pending]

### 9. Skill Wizard — Commit Closed Loop (SC-3 / D5-11)
expected: 确认并保存到 skills/{name}/SKILL.md → atomic write → success toast "Skill 已创建：{name}。已注册并热加载。" + path. fsnotify picks it up → appears in Skill Manager / routable. THE PLATFORM THESIS closed loop.
result: [pending]

### 10. Skill Wizard — Cancel Dialog + Path-Traversal Guard (SC-3 / T-5-16)
expected: 取消向导 → alert dialog "放弃当前进度？..." with 放弃 (danger) + 继续向导. Malformed purpose ("../etc/passwd" / "Foo Bar!") rejected at 400 (extractSkillName before LLM) with FE hint.
result: [pending]

### 11. Routing Trace + Counter-Evidence + Redline (SC-1 / SC-6 / SC-7)
expected: Chat emits data-decision routing trace (skillScores + selected/ambiguous) inline; on 不合规/需补充 emits counter-evidence (CounterEvidenceCard — 历史 conflict + disclaimer) + redline (RedlineCard — 原条款/建议改写/修改理由 + copy/download). 合规 → NO redline (0 LLM calls, D5-14). (Mock can't trigger; verify via POST /api/chat :8888 + FE unit suites.)
result: [pending]

## Summary

total: 11
passed: 3
issues: 2
pending: 6
skipped: 0
blocked: 0

## Gaps

- truth: "Right panel (chat sidebar) information architecture matches design intent: the Skill selector lives in the chat-input area (where the 未选择 Skill hint is), NOT as a separate top-right dropdown; the session list is a drawer that mutually toggles with the chat view; a 新建会话 button replaces the session/skill dropdown. (Sketch source-of-truth: session list + file tree + skill list belong in the LEFT sidebar; right panel is 'chat with Skill'.)"
  status: failed
  reason: "User reported: 选择skill的下拉框在右上角，而他应该在下方显示'未选择skill'那里；会话列表应该是个抽屉，和chat互相切换；下拉框换成新建会话按钮。"
  severity: major
  test: 2
  root_cause: ""     # Filled by diagnosis
  artifacts: []      # Filled by diagnosis
  missing:
    - "Consolidate the Skill selector into the chat-input area (near the 未选择 Skill hint); remove the redundant top-right 选择 Skill button."
    - "Convert the session list into a drawer/overlay that mutually toggles with the chat view (not a persistent inline button)."
    - "Replace the session dropdown with a 新建会话 (New Session) button."
    - "Reconcile with sketch source-of-truth: sessions + file tree + skill list belong in the LEFT sidebar; decide final IA (left-sidebar vs right-drawer) before implementing."
  debug_session: ""  # Filled by diagnosis

- truth: "Clicking a chunk in the SC-5 Knowledge Graph Browser opens ChunkSourceView and displays the chunk's verbatim text (not just metadata)."
  status: failed
  reason: "User-path observed: graph chunk click → ChunkSourceView shows tier + 当前有效 badge + 生效日期 but body renders '(chunk not found in fixture)'. chunk-lookup.ts only resolves fixture/verdict citations; backend exposes no chunk-content endpoint; graph endpoint is metadata-only (T-5-22)."
  severity: major
  test: 4
  root_cause: ""     # Filled by diagnosis
  artifacts:
    - path: "web/src/lib/chunk-lookup.ts"
      issue: "NOT_FOUND_TEXT fallback; content lookup limited to fixture/verdict citations — no resolution path for graph-browsed (non-cited) chunks"
  missing:
    - "Add a chunk-content fetch path (e.g. GET /api/knowledge/chunk/:id) OR have the graph browser hydrate ChunkSourceView content, so graph-browsed chunks display verbatim text."
    - "Decide SC-5 browse contract: is full-text view from the graph in-scope (needs content endpoint) or metadata-only (then chunk click should not promise a verbatim view)?"
  debug_session: ""  # Filled by diagnosis
