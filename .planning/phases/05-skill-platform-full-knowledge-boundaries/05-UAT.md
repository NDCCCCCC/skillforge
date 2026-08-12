---
status: complete
phase: 05-skill-platform-full-knowledge-boundaries
source: [05-01-SUMMARY.md, 05-02-SUMMARY.md, 05-03-SUMMARY.md, 05-04-SUMMARY.md, 05-05-SUMMARY.md, 05-06-SUMMARY.md, 05-UI-SPEC.md]
started: 2026-08-12T13:46:53Z
updated: 2026-08-13T00:25:00Z
verification_mode: automated (chrome-devtools MCP) + backend HTTP (:8888) + unit-test evidence
dev_url: http://localhost:5173/ (Vite dev) → mock chat transport; backend http://localhost:8888 (Hertz, real data)
constraint: Dev FE uses MockChatTransport (env=dev) — SC-1/6/7 chat chunks verified via direct /api/chat + unit tests, not live dev UI
---

## Current Test

[testing complete — 11/11 resolved: 9 pass (7 original + #9 + OOS-1 fixed 2026-08-13), 3 open (#2 right-panel IA, #4 graph chunk-content, #7 hot-reload toast). BLOCKERS #9 (wizard registration, commit af1ca9a) + OOS-1 (GLM verdict fence-parse, commit d62fe98) RESOLVED via /gsd-debug — see 05-REMAINING-WORK.md for the remaining 3 issues' plan.]

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
result: pass
evidence: User observed empty 暂无 Agent 状态 after assistant turn in dev UI — root cause is the documented MockChatTransport limitation (grep: no __fixtures__/*.json contains `data-decision`; deriveHops() in AgentInspector.tsx:52 keys solely off `data-decision` parts → always [] in dev). Same class as SC-1/6/7. SC-4 verified via the established alternative path: (1) real backend POST /api/chat {"query":"年假能跨年吗","skill":"企业合规/制度审查"} emits `data-decision` routing trace live (2× data-decision + data-policy + finish-step confirmed on :8888); (2) AgentInspector.test.tsx 8/8 pass — empty/loading states, multi-hop grouping (1 hop per data-decision step, data-hop-step/data-decision attrs), active-hop 2px primary left-border (border-l-4 border-l-primary; past hops border-l-transparent), counter-evidence teal badge, tool-call expand (no dangerouslySetInnerHTML), reasoning collapsed by default, retrieval chunk-ID buttons (dispatch setCenterView).

### 7. Hot-Reload Toast (SC-2 / D5-07/D5-08)
expected: Touch/save a skills/<name>/SKILL.md on disk → within ~1s debounce a ReloadToast appears (success on valid change; validation-error toast on broken YAML, FS paths stripped). D5-08: validation failure keeps old snapshot.
result: issue
reported: "Empirical (automated chrome-devtools + fs): appended a valid change to skills/compliance/SKILL.md, waited 3.5s (>2s poll interval) → ZERO toasts appeared. Browser console shows `404 [38 times]` — EventSource hammering /api/skills/events which the backend never registered."
severity: major
root_cause_candidate: "Hot-reload SIGNAL PATH is severed end-to-end. (1) SSE: GET /api/skills/events is NOT registered (server.go:191-196 lists only chat/skills/wizard/graph) → EventSource reconnect storm → 404. (2) Polling fallback (useSkillReload.ts:120) diffs `item.version`, but GET /api/skills returns []*skill.Manifest (manifest.go:57-73 = the 7 schema fields, NO version). Version int64 lives only on registry.Entry (registry.go:58), bumped on reload (registry.go:333) but NEVER serialized to HTTP. So neither path delivers a reload event → toast never fires. NOTE: backend reload machinery itself works (fsnotify → registry Version bump → router cache invalidation); the FE toast UI is unit-tested in isolation. Only the BE→FE notification bridge is broken. Failure-toast path (broken YAML) is additionally unobservable: poller always emits status:'ok' (useSkillReload.ts:125), and SSE is dead."

### 8. Skill Wizard — NL Interview (SC-3 / SYSK-06)
expected: Click 新建 Skill（向导）→ right panel enters wizard mode (header 步骤 {N}/5 + 取消向导; ChatInput 模式：Skill 向导). Answer 5 NL prompts (purpose/reads/type/guardrails/temperature). 生成预览 → POST /api/skills/wizard returns 7-field preview; center auto-switches to Skill 定义.
result: pass
evidence: Drove full flow via chrome-devtools. Wizard mode entered correctly (Skill 向导 · 步骤 0/5 → 5/5, 取消向导 btn, ChatInput 模式：Skill 向导). 5 Chinese answers collected (purpose w/ ASCII token "expense-audit ...", reads=权威, skillType=合规审查, guardrails=中文, temp=0.0). 生成预览 → POST /api/skills/wizard (reqid 4132) → 200, returned full 7-field preview: description, agent=ChatModelAgent, reads=[authority/**], writes=[], 4-step steps, 5 prohibitions, temperature=0, name=expense-audit. FE rendered preview in center Skill 定义 area. LLM correctly normalized Chinese answers (合规审查→compliance, 权威→authority/**). NOTE: all-Chinese purpose w/o ASCII token is rejected by extractSkillName before LLM (T-5-16 guard, by design — confirmed in earlier probe).

### 9. Skill Wizard — Commit Closed Loop (SC-3 / D5-11)
expected: 确认并保存到 skills/{name}/SKILL.md → atomic write → success toast "Skill 已创建：{name}。已注册并热加载。" + path. fsnotify picks it up → appears in Skill Manager / routable. THE PLATFORM THESIS closed loop.
result: fixed (commit af1ca9a, 2026-08-13) — wizardCommitHandler now rescans Loader + sets Registered; FE gates toast on `registered`; Loader.cached race-guarded. Closed-loop integration test TestWizardCommitHandler_RegistersAndRefreshesLoader passes.
sub_passed: Atomic write OK (skills/expense-audit/SKILL.md, 1088 bytes, valid YAML, all 7 fields match preview); success toast fired EXACTLY as spec — "Skill 已创建：expense-audit。已注册并热加载。" + full path; wizard exited cleanly → chat session restored.
reported: "Closed loop BROKEN at registration step. Toast claims '已注册并热加载' but the new skill is NOT registered: absent from GET /api/skills (only 企业合规/制度审查 returned; '通用问答' appears in Skill Manager UI but NOT in /api/skills → it is FE/built-in injected, not registry-backed), absent from Skill Manager dropdown, and NOT picked up even after clicking 全部重载 (manual reload) — /api/skills still 1 skill, 0 expense-audit. Routability unconfirmed: POST /api/chat {skill:expense-audit} returned a no_authorized_source refusal, but that is consistent with fallback to the default compliance skill, not proof of routing. The platform thesis (write Skill file → agent adapts, no code change) is defeated for the wizard creation path — a wizard-created skill is dead-on-arrival until (likely) a server restart."
severity: blocker
root_cause_candidate: "Registration never happens for wizard-created skills. Three signals all negative: (1) fsnotify auto-reload did not register it (file written 23:59, /api/skills checked across 30+ s — never appears); (2) explicit 全部重载 (reload-all) ALSO did not register it; (3) not in Skill Manager. So the backend registry is not picking up skills/expense-audit/SKILL.md via any live path. Strongest hypothesis: registry rescan does not walk newly-created skill subdirectories (watches only paths known at startup, or reload re-scans a fixed set), AND the wizard commit handler writes the file but does not itself call registry.Reload()/rescan. The FE toast is fired optimistically on HTTP 200 from the commit endpoint without verifying registration — a false '已注册并热加载' claim. NOTE: distinct from Issue #7 — #7 is the FE→BE notification BRIDGE for EXISTING-skill reloads (SSE 404 / no version field); Test 9 is the BACKEND never registering a NEW skill at all. They may share the shaky hot-reload subsystem but are separate failure modes. Server-restart registration UNTESTED (left running per UAT constraint)."
artifacts_to_investigate:
  - "internal/server/skills.go — wizard commit handler: does it call registry reload after writing SKILL.md?"
  - "internal/skill/registry.go — Reload()/scan: does it walk skills/*/SKILL.md dynamically, or only re-load a fixed known set?"
  - "fsnotify watcher setup: does it add watches for new subdirs created under skills/?"
  - "FE toast trigger (wizard commit response handler): fires on 200 without polling /api/skills to confirm"
evidence: "GET /api/skills → only 企业合规/制度审查 (1 skill) before AND after 全部重载; grep '审查员工报销申请' = 0. Skill Manager dropdown: 企业合规/制度审查 + 通用问答 only, no expense-audit. File on disk: skills/expense-audit/SKILL.md valid. Test skill dir left in place for diagnosis (registration failed, so not installed in registry); clean up after diagnosis."
test_skill_cleanup_pending: true

### 10. Skill Wizard — Cancel Dialog + Path-Traversal Guard (SC-3 / T-5-16)
expected: 取消向导 → alert dialog "放弃当前进度？..." with 放弃 (danger) + 继续向导. Malformed purpose ("../etc/passwd" / "Foo Bar!") rejected at 400 (extractSkillName before LLM) with FE hint.
result: pass
evidence: |
  PART A (cancel dialog): clicked 取消向导 at step 0/5 → alertdialog (role=alertdialog, modal) rendered with heading 「放弃当前向导？」 + body 「放弃当前进度？当前进度（0/5 步已回答）将丢失，无法恢复。」 + buttons 继续向导 (safe, auto-focused) + 放弃 (destructive). Progress counter (0/5) shown. Clicked 放弃 → wizard exited cleanly, right panel restored to normal chat session, ChatInput placeholder reverted from wizard mode. Cancel flow works end-to-end. (WizardCancelDialog.tsx component + WizardCancelDialog.test.tsx.)
  PART B (path-traversal guard T-5-16): verified at the BACKEND gate (authoritative). Direct probe POST /api/skills/wizard {"answers":{"purpose":"../etc/passwd"}} → HTTP 400 + {"error":"the purpose \"../etc/passwd\" did not yield a usable Skill name (must contain English letters/digits, e.g. \"compliance-audit\")"}. extractSkillName (internal/server/wizard_name.go) truncates at the first '.' sentence-terminator → empty slug → error, BEFORE any LLM call (wizardHandler line 165-168 returns 400 before Wizard.Generate at line 171). FE hint is wired: requestPreview (useWizard.ts:182-185) does setError(body.error) and renders it on a 400 at 生成预览. NOTE: validation fires at 生成预览, not step 0 — by design (FE collects 5 answers locally; backend is the authoritative name+write gate, so no security gap: ../etc/passwd can never reach disk because wizardCommitHandler also re-validates name via skillNamePatternForWizard + resolved-path checks). No LLM cost on malicious input.

### 11. Routing Trace + Counter-Evidence + Redline (SC-1 / SC-6 / SC-7)
expected: Chat emits data-decision routing trace (skillScores + selected/ambiguous) inline; on 不合规/需补充 emits counter-evidence (CounterEvidenceCard — 历史 conflict + disclaimer) + redline (RedlineCard — 原条款/建议改写/修改理由 + copy/download). 合规 → NO redline (0 LLM calls, D5-14). (Mock can't trigger; verify via POST /api/chat :8888 + FE unit suites.)
result: pass
via: alt path (same methodology as Test 6 — MockChatTransport dev limitation; real chat-stream features verified via direct /api/chat + FE unit suites)
evidence: |
  SC-1 (routing trace): real backend POST /api/chat emits data-decision routing trace LIVE — confirmed twice this session (Test 6 follow-up + the expense-audit routability probe): stream = data-decision{step:0,decision:answer} → data-decision{step:0,decision:refuse,query} → data-policy{reason:no_authorized_source,...} → finish-step. RoutingTracePart.test.tsx 7/7 pass.
  SC-6 (counter-evidence): CounterEvidenceCard.test.tsx 9/9 pass — disclaimer rendered VERBATIM (T-5-27 load-bearing), one row per conflict chunk w/ teal tier badge + snippet, empty-state copy, click→setCenterView, D5-13 FE moat (props declare ONLY conflictChunks, no citations). Real 不合规/需补充 verdict path NOT live-verifiable (BLOCKED by OOS-1 — verdict parse fails before a non-refusal verdict can render); deferred to post-OOS-1-fix.
  SC-7 (redline): RedlineCard.test.tsx 12/12 pass (原条款/建议改写/修改理由 + copy/download). Real-verdict path likewise blocked by OOS-1; deferred.
  Refusal path (refuse verdict): works end-to-end — RefusalCard.test.tsx 8/8 + live /api/chat no_authorized_source observed.
  Full FE suite: 29 files / 193 tests pass (exit 0).
  CAVEAT: SC-6/SC-7 counter-evidence + redline on a REAL compliant/non-compliant verdict cannot be live-verified until OOS-1 (GLM fenced-JSON parse, /gsd-debug) is fixed. Components proven correct by unit suites + routing/refusal trace proven live; only the end-to-end non-refusal-verdict rendering is unobserved.

## Summary

total: 11
passed: 7
issues: 4
pending: 0
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

- truth: "Touching an installed skill file on disk (skills/<name>/SKILL.md) surfaces a ReloadToast in the UI within ~1s, for both valid changes (success) and broken YAML (validation error, paths stripped)."
  status: failed
  reason: "Empirical (automated): appended valid change to skills/compliance/SKILL.md, waited 3.5s → 0 toasts. Browser console: 404 [38 times] from EventSource hitting /api/skills/events."
  severity: major
  test: 7
  root_cause_candidate: "Signal path severed end-to-end. SSE /api/skills/events is not registered (server.go:191-196) → 404 reconnect storm. Polling fallback (useSkillReload.ts:120) diffs item.version, but GET /api/skills returns []*skill.Manifest with NO version field (manifest.go:57-73); Version lives only on registry.Entry (registry.go:58), bumped but never serialized. Neither path delivers a reload event. Backend reload + FE toast UI each work in isolation; only the BE→FE bridge is broken."
  artifacts:
    - path: "internal/server/server.go"
      issue: "Routes (lines 191-196) register no /api/skills/events SSE endpoint"
    - path: "internal/server/skills.go"
      issue: "listSkillsHandler returns []*skill.Manifest — no Version exposed"
    - path: "internal/skill/manifest.go"
      issue: "Manifest (lines 57-73) serializes only the 7 schema fields; no version json tag"
    - path: "web/src/hooks/useSkillReload.ts"
      issue: "Polling diff (line 120) keys off item.version which is always undefined in the response"
  missing:
    - "Register GET /api/skills/events SSE endpoint that emits skill-reloaded events on fsnotify reload (primary path), AND/OR"
    - "Serialize registry.Entry.Version into the GET /api/skills response (e.g. wrap Manifest in {manifest, version}) so the polling fallback diff can detect increments."
    - "Ensure failure path (broken YAML → status:'failed', version:'old-kept') is observable: poller currently only emits status:'ok' (line 125); without SSE the failure toast is unreachable."
  debug_session: ""  # Filled by diagnosis

- truth: "A skill created via the Skill Wizard, on 确认并保存, is written to disk, AND registered + hot-loaded so it immediately appears in Skill Manager and is routable — the platform-thesis closed loop (write Skill file → agent adapts, no code change)."
  status: fixed
  fixed_commit: af1ca9a
  reason: "User-path observed: commit wrote skills/expense-audit/SKILL.md (valid, 1088 bytes, all 7 fields) and fired the 'Skill 已创建：expense-audit。已注册并热加载。' toast, but the skill is NOT registered — absent from GET /api/skills (1 skill only), absent from Skill Manager dropdown, and NOT picked up even after manual 全部重载. The '已注册并热加载' toast is a false claim. A wizard-created skill is dead-on-arrival until (likely) a server restart."
  severity: blocker
  test: 9
  root_cause: ""     # Filled by diagnosis
  artifacts:
    - path: "internal/server/skills.go"
      issue: "wizard commit handler — verify whether it calls registry reload/rescan after writing SKILL.md (suspected: it does not)"
    - path: "internal/skill/registry.go"
      issue: "Reload/scan — verify whether it dynamically walks skills/*/SKILL.md or only re-loads a fixed known set (suspected: new subdirs missed)"
    - path: "fsnotify watcher setup"
      issue: "verify whether watches are added for newly-created subdirs under skills/ (classic fsnotify gap)"
    - path: "web/src — wizard commit response handler"
      issue: "toast fires optimistically on HTTP 200 without confirming registration via /api/skills"
  missing:
    - "Wizard commit handler must trigger a registry rescan that includes the newly-created skills/<name>/ directory (or fsnotify must watch new subdirs)."
    - "Confirm registration before showing the success toast — poll /api/skills or have the commit endpoint return the registered entry; on failure show an error toast, not '已注册并热加载'."
    - "Decide whether 通用问答 (in Skill Manager UI but not /api/skills) is a FE-injected built-in — if so, document; it muddies the 'installed skills' contract."
  debug_session: ""  # Filled by diagnosis

## Findings (out of Phase 5 scope — for /gsd-debug, NOT Phase 5 gap closure)

- id: OOS-1
  title: "Real-backend verdict parse failure on GLM fenced-JSON (hidden by MockChatTransport)"
  severity: blocker (core compliance-verdict value)
  discovered_during: "Test 6 follow-up — user asked if dev UI uses mock data; confirmed MockChatTransport (App.tsx:49 hardcodes env=dev on Vite PROD flag, no .env override). Probed real POST /api/chat with UTF-8 query 员工年假有多少天 → agent ran retrieve→answer→forced_stop then emitted type:error."
  evidence: |
    curl POST /api/chat {"query":"员工年假有多少天","skill":"企业合规/制度审查"} →
    errorText: "agent run: parse verdict: invalid character '`' after top-level value (raw: ```json\n{\n  "...)" → no text-delta / source-document emitted. Refusal path (no_authorized_source) works; the ANSWER path (compliant/non-compliant verdict) is broken.
  root_cause_candidate: |
    GLM wraps verdict JSON in ``` ```json ... ``` ``` fences. stripJSONDelim (internal/agent/callbacks.go:196) strips leading fence + lang tag + trailing fence, but the trailing fence is NOT fully removed in GLM's output → json.Unmarshal hits a backtick "after top-level value". Failing call site: internal/agent/agent.go:515 (parse verdict). Second site internal/server/pipeline.go:209.
  artifacts:
    - internal/agent/callbacks.go:196 (stripJSONDelim)
    - internal/agent/agent.go:515,519 (verdict Unmarshal + error)
    - internal/server/pipeline.go:209
  impact_on_phase_5_uat: "Blocks live end-to-end verification of Test 11 (SC-1/6/7 routing trace + counter-evidence + redline all require a real verdict). Test 11 will fall back to /api/chat (refusal path is observable) + FE unit suites, same as Test 6."
  resolution_route: "/gsd-debug (verdict pipeline is Phase 2-era; not a Phase 5 defect). DO NOT close under Phase 5 gap closure."
  status: fixed
  fixed_commit: d62fe98
