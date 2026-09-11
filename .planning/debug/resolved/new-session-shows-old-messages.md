---
slug: new-session-shows-old-messages
status: resolved
created: 2026-09-11
updated: 2026-09-12
resolved: 2026-09-12
trigger: "用户新开一个会话，出现旧对话记录（截图：会话里显示「生成2026-08 月报」+ 回复渲染为「收到未识别的消息负载（可能是版本差异，不影响其他内容）」卡片）"
---

# Debug Session: new-session-shows-old-messages

## Symptoms

- **Expected**: 点击「新建会话」后应得到空白对话区（新 session，无历史消息）。
- **Actual**: 点击「新建会话」后：
  1. 头部 skill 选择器变为 `monthly-report`、会话标题变为「生成 2026-08 月报」（一个 8 分钟前的旧会话的标题）；
  2. 对话区仍显示上一个会话（「你好」/年假问答 ×2，即 local-session-3 的 history）的消息，未清空；
  3. 详情面板同样残留旧会话的结论。
- **Errors**: 浏览器控制台 React 报错 `Encountered two children with the same key, local-session-2`（重复 16 次）。
- **Timeline**: 不确定何时开始；用户在测试 monthly-report skill（「生成 2026-08 月报」）后注意到。
- **Reproduction**（orchestrator 已用 chrome-devtools 稳定复现，2026-09-11）:
  1. 打开 http://localhost:5173/ （页面加载即恢复最近会话 local-session-3）；
  2. 点侧栏「会话列表」→「新建会话」；
  3. 头部变为 monthly-report /「生成 2026-08 月报」，对话区/详情面板仍显示 local-session-3 的旧消息。
- **主修复验收（2026-09-12 human-verify round 1 PASSED）**: 硬刷新 → 新建会话 → 对话区为空，头部「会话 1」+ simple-qa；无 duplicate-key 报错；新会话 id 为 UUID（local-session-fd577332-3de8-4f1e-b34b-67999d46bb34），roster dupIds=[]；发消息「新会话独立持久化验证」完整走管线（5 候选→超范围拒答）并独立持久化成会话。
- **残留验收（2026-09-12 human-verify round 2 PASSED，confirmed fixed）**: 见 Evidence 2026-09-12 round 2 条目 —— 拒答单写、boot 闸门、createSession 幂等三项全部通过。

## Evidence

- timestamp: 2026-09-12 (human-verify round 2 — 前置)
  - orchestrator 按 runbook 重建并重启 :8888：杀掉用户终端里的 `go run`（pid 2890/2944，二进制名 main）→ `go build -o skillforge-server-dev-new ./cmd/server` + setsid + source .env 启动成功（日志 [OK] listening on :8888, backend=milvus）。
  - 注意：用户旧终端里的 go run 进程被杀，会显示进程退出；后端现由 runbook 的 skillforge-server-dev-new 二进制承载。
- timestamp: 2026-09-12 (human-verify round 2 #1 拒答单写 PASS)
  - compliance skill 下发「量子力学第一性原理简介」→ 拒答（范围内检索置信度不足）→ 服务端 history 恰好 2 条（user+assistant 各 1），UI 显示与 DB 一致。会话 local-session-4fc9c30e-2920-4606-bd5c-68b21dc1865e。
- timestamp: 2026-09-12 (human-verify round 2 #2 boot 闸门 PASS)
  - initScript 从文档起点每 50ms 采样：t=676ms composer 挂载即 disabled（sendDisabled=true），t≈1.7s 水合完成后启用（2s fail-open 内）。
  - 对抗性测试：initScript 在建议按钮出现瞬间（t=749ms，btnDisabledAtClick=true）自动 click → 无任何发送，近 2 分钟 roster 零新增 → 实证 SUGGESTED_PROMPTS[0]/defaultSkill 兜底的代码级定位。
- timestamp: 2026-09-12 (human-verify round 2 #3 幂等 PASS)
  - 页内对「新建会话」按钮同步双击（back-to-back，快于人手）→ 仅新增一个「会话 2」（此前单击已铸「会话 1」），roster 291→291（纯前端铸造不发请求），控制台零 React 报错。
- timestamp: 2026-09-12 (code: internal/server/pipeline.go — 残留 #1 根因)
  - `VerdictPipeline` 分类器拒答分支在 255 行调用 `writeAuditAndTurn`（审计行 + user/assistant turns），随后返回到 `emitRefusalWithChunks` —— 后者在 336 行做了同样的写入。拒答路径上一次 POST = 2 份审计 + 2 组 turns。
  - 验证：`TestVerdictPipeline_ClassifierRefuses` 修复前 RED（"should have 1 item(s), but has 2"），与 live DB（audit 414/415、turns 693-696）吻合。
  - 前端双发已排除：ChatInput `setText('')` 提交时序、AI SDK `makeRequest` 无重试、未设置 `sendAutomaticallyWhen` —— 单次 Enter 不可能双发。
- timestamp: 2026-09-12 (code: ChatColumn.tsx + BE defaultSkill — 残留 #2 根因)
  - 幽灵文本「年假未休完的部分如何折算或结转？」精确等于 `web/src/components/layout/ChatColumn.tsx:35` 的 `SUGGESTED_PROMPTS[0]`（唯一出现处）。
  - `skill_id=compliance` 是 BE `defaultSkill` 兜底——仅在 `currentSkillId` 为 null 时触发；composer 只在水合前的 boot Chat 上这样绑定。两个条件只在 GET /api/sessions（~220ms）期间同时成立 → 窗口内发送落进随后被抛弃的 boot 会话（d812e7d5），以幽灵形式出现在 roster 顶部。
  - SDK 自动重发已排除：应用以 `new Chat({id, transport})` 构造，未设置 `sendAutomaticallyWhen`，resume=false。
- timestamp: 2026-09-12 (code: app-store.ts createSession — 残留 #3 根因)
  - StrictMode 从不双调用事件 handler；composer 与 store 同一 render 派生，"UI fd577332 / POST 71158e9c" 分裂只是 active id 在 GET 与 POST 之间推进的结果（fd577332 从未到服务端，DB 不存在）。
  - Prod 实际风险：快速双击使 `createSession` 跑两次。
- timestamp: 2026-09-12 (chrome-devtools live 残留 #1 同轮双写 — 原始观察)
  - 消息落在 `local-session-71158e9c-0f3c-4809-86b9-7dfbaa87e82b`，同一轮 Q&A 写两遍（user+assistant ×2，ts=1789143516255 / 1789143516274，差 19ms）。人工发送（local-session-3 三轮）无双写。
- timestamp: 2026-09-12 (chrome-devtools live 残留 #2 幽灵自动重发 — 原始观察)
  - roster 顶部出现 `local-session-d812e7d5-6fb3-4eac-af31-31dc558fd758`（boot 会话 INITIAL_SESSION_ID），内容为 local-session-3 最后一问 + 新生成的 assistant 回答，ts=1789143518057（用户消息完成后 ~1.8s），skill_id=compliance。操作者从未输入过这句话。
- timestamp: 2026-09-12 (chrome-devtools live 残留 #3 新建会话双铸 — 原始观察)
  - 点一次「新建会话」铸两个 UUID（fd577332 与 71158e9c）；UI 会话状态（approval/history 请求）打 fd577332，消息落 71158e9c。
- timestamp: 2026-09-12 (chrome-devtools human-verify round 1 — 主修复验收通过)
  - 硬刷新 → 新建会话 → 对话区为空，头部「会话 1」+ simple-qa；无 duplicate-key；新会话 id 为 UUID，roster dupIds=[]；发消息完整走管线并独立持久化。
- timestamp: 2026-09-11 (code: web/src/store/app-store.ts)
  - checked: id generation. Lines 572-573: module-level `let sessionCounter = 1`; `INITIAL_SESSION_ID = local-session-${sessionCounter++}` (consumes 1 → counter=2 at boot, so the FIRST createSession after any page load mints exactly `local-session-2`). `createSession` (796) and `deleteSession` fallback (816) mint from the same counter, appending to `sessions` WITHOUT dedupe (799).
  - implication: after every page reload the counter restarts; first 新建会话 deterministically re-mints `local-session-2` → collides with the persisted server session. Matches repro exactly. [FIXED by UUID mint — verified round 1]
- timestamp: 2026-09-11 (code: web/src/store/app-store.ts + web/src/transport/RealChatTransport.ts)
  - checked: why server DB contains `local-session-*` ids despite the B3 comment claiming the namespace is "structurally disjoint from server roster ids" (app-store.ts:565-571).
  - found: RealChatTransport.sendMessages sends `x-session-id: options.chatId` (RealChatTransport.ts:366) where chatId === the client-minted `local-session-N`; the BE resolves/creates the session partition from this header.
  - implication: the B3 namespace-disjointness assumption is false. Counter reset on reload guarantees eventual collision. [FIXED by UUID mint]
- timestamp: 2026-09-11 (code: web/src/App.tsx)
  - checked: messages reset on session switch. `historySnapshot` state (139) held ONE un-keyed `SessionHistoryResponse`; the inject effect (193-212) wrote the stale snapshot into the newly-active session's empty Chat and permanently marked it applied (`appliedSessionsRef`), blocking the new session's own (empty) history (195).
  - implication: stale old-session messages appear in the brand-new session AND persist — symptom 2's mechanism, independent of the id collision. [FIXED by keyed snapshot — verified round 1]
- timestamp: 2026-09-11 (code: web/src/App.tsx + components)
  - found: collision appends a second roster row (dup React keys ×16); header/meta via `sessions.find(id)` first-match (`SessionPicker.tsx:22`, `SessionDrawer.tsx:169`) shows the roster row's old title; skill via `useSkillAutoSwitch(historySnapshot?.skill_id)` (App.tsx:159). [FIXED]
- timestamp: 2026-09-11 (code: web/src/main.tsx)
  - found: StrictMode is ON — dev double-invokes effects; mount-time ordering races are live. [主注入面已由 keyed snapshot 修复；残留 #2/#3 已单独根因定位并修复]
- timestamp: 2026-09-11 (chrome-devtools live repro — 原始主诉)
  - 点击「新建会话」无 POST；id 前端本地生成；GET history/approval-rules 直接打新 id。
- timestamp: 2026-09-11 (fetch /api/sessions)
  - 服务端已有 `local-session-1`（空）、`local-session-2`（「生成 2026-08 月报」）、`local-session-3`（「你好」）；前端恰铸 `local-session-2` → 撞 ID。
- timestamp: 2026-09-11 (localStorage dump)
  - 无任何会话 id/计数器 key → 计数器内存态。[已确认：app-store.ts 模块级计数器，已改 UUID]
- timestamp: 2026-09-11 (fetch /api/session/{id}/history)
  - local-session-2 的 assistant content 空串 + verdict 全零 → DataPolicyPart 兜底文案来源（web/src/components/chat/parts/DataPolicyPart.tsx:81）；local-session-3 为「你好」+ 年假问答 ×2。 [主诉机制已确认+修复；空 assistant 消息成因属 BE SSE 中断问题，范围外]
- timestamp: 2026-09-11 (fetch /api/sessions 全量)
  - 库中 300+ 条历史会话，大量 e2e/压测残留；部分 `last_active_ms` 为 Go 零值。[卫生问题，范围外]

## Current Focus

hypothesis: （已全部证实并修复）主诉 = 计数器撞 ID + un-keyed historySnapshot 注入；残留 #1 = BE 拒答路径双写；残留 #2 = boot 水合窗口发送（SUGGESTED_PROMPTS + defaultSkill 兜底）；残留 #3 = createSession 非幂等。
test: round 2 human-verify 全部 PASS（2026-09-12，WSL 后端重建重启后）。
expecting: 无遗留。
next_action: ARCHIVED (2026-09-12) — coordinator round-2 verdict confirmed fixed。lint 清理已完成（emitRefusalWithChunks 移除 unused classifierReason 参数 + 5 处调用点；WSL go build/vet OK，TestVerdictPipeline_ClassifierRefuses PASS）；文件已移入 .planning/debug/resolved/。会话关闭。
reasoning_checkpoint:
  hypothesis: "Primary: collision (reload-resetting counter re-mints persisted ids) + un-keyed historySnapshot injection. Residual #1: BE double-write on classifier-refusal path. Residual #2: boot-hydration-window send into doomed boot Chat (SUGGESTED_PROMPTS[0] + defaultSkill fallback). Residual #3: non-idempotent createSession under rapid re-trigger."
  confirming_evidence:
    - "Round-1 verify: empty new session, UUID id, dupIds=[] — primary fix confirmed."
    - "TestVerdictPipeline_ClassifierRefuses RED before dedup fix ('should have 1 item(s), but has 2'), matches live DB audit 414/415 + turns 693-696."
    - "Ghost text === SUGGESTED_PROMPTS[0] (ChatColumn.tsx:35); skill_id=compliance === BE defaultSkill fallback only when currentSkillId null (pre-hydration boot Chat)."
    - "Round-2 verify: refusal history exactly 1 turn; initScript sampling t=676ms composer disabled, adversarial auto-click at t=749ms → zero sends in ~2min; double-click → exactly one new row, roster 291→291, zero console errors."
  falsification_test: "All three residual mechanisms had RED→GREEN regression coverage (Go classifier test; web composer gate tests; store idempotency test) — if the mechanisms were misidentified, the live adversarial tests would have sent/replicated; they did not."
  fix_rationale: "BE: single ownership of refusal persistence. FE: gate sends until roster hydration completes (2s fail-open); make createSession idempotent within 250ms. All mechanism-level, minimal surface."
  blind_spots: "Out of scope (recorded, non-blocking): 300+ e2e/load-test residue rows in sessions table; pre-fix ghost session d812e7d5 + double-written rows remain in DB as historical data; empty assistant content in old rows (BE SSE-interrupt family, memory fix-runner-sse-interrupt-chunk); web/.env.local leaking VITE_REAL_BACKEND=1 into vitest (now asserted-around, may deserve its own cleanup)."

## Eliminated

- hypothesis: App.test.tsx failures (null useMemo dispatcher) are caused by the debug fix / regression tests
  evidence: `git stash push -- src/__tests__/App.test.tsx` → pristine HEAD file also fails all 8 tests identically; `src/components/ApprovalCard.test.tsx` (any React-rendering test) fails the same way; tracked lockfile is `package-lock.json` (npm) while untracked `pnpm-lock.yaml` + `pnpm-workspace.yaml` show node_modules was recently reinstalled via pnpm (react 19.2.8). Pre-existing environmental regression from the pnpm switch, unrelated to this bug.
  timestamp: 2026-09-12
- hypothesis: 残留 #1 双写由前端 submit 双触发造成
  evidence: ChatInput `setText('')` 在下一次离散事件前提交；AI SDK `makeRequest` 无重试；未设置 `sendAutomaticallyWhen`；人工发送无双写。真实根因在 BE pipeline.go 双写。
  timestamp: 2026-09-12
- hypothesis: 残留 #2 由 Vercel AI SDK `sendAutomaticallyWhen` 自动重发造成
  evidence: 应用以 `new Chat({id, transport})` 构造，未设置该回调，resume=false；幽灵文本精确匹配 SUGGESTED_PROMPTS[0]，真实根因是水合窗口内的人工/建议按钮发送 + boot Chat 被抛弃。
  timestamp: 2026-09-12
- hypothesis: 残留 #3 由 StrictMode 双调用 createSession handler 造成
  evidence: StrictMode 仅双调用 effects/render，不双调用事件 handler；composer 与 store 同 render 派生，无垂直分裂；fd577332 从未到达服务端。真实风险是快速双击。
  timestamp: 2026-09-12

## Resolution

root_cause: Five defects total — two primary (frontend) + three sibling residuals:
  (P1) Session-id collision — `web/src/store/app-store.ts` minted `local-session-${n}` from a module-level counter that resets on every page load; ids are persisted server-side via `x-session-id` (`web/src/transport/RealChatTransport.ts:366`), so the re-minted id deterministically collided with an existing roster session (dup React keys, wrong header title/skill).
  (P2) Stale history injection — `web/src/App.tsx` held `historySnapshot` un-keyed by session; on switch, the previous session's snapshot was injected into the new session's empty Chat and `appliedSessionsRef` permanently blocked the new session's own history.
  (R1) BE refusal double-write — `internal/server/pipeline.go`: the classifier-refusal branch called `writeAuditAndTurn` (255) and then fell through to `emitRefusalWithChunks`, which performs the same writes (336) → one POST persisted audit rows + turn pairs twice.
  (R2) Boot-hydration-window send — composer/suggested prompts were live before roster hydration finished; a send in that ~220ms window (suggested prompt or typed) landed on the doomed boot Chat with `currentSkillId=null` → BE `defaultSkill` fallback (compliance), surfacing later as a ghost roster row the user never authored.
  (R3) Non-idempotent createSession — rapid re-trigger (fast double-click) minted two ids and split UI state vs the Chat the composer actually sends on.
fix: 
  (P1) app-store.ts — `mintSessionId()` returns `local-session-${crypto.randomUUID()}` (time+random fallback); separate display-only `sessionNameCounter`.
  (P2) App.tsx — `historySnapshot` keyed `{sessionId, resp}`; inject effect and `useSkillAutoSwitch` only act when the snapshot belongs to the active session.
  (R1) internal/server/pipeline.go — duplicate write removed; persistence owned solely by `emitRefusalWithChunks`; regression test locks turn-pair count. (Archive cleanup: unused `classifierReason` parameter removed from emitRefusalWithChunks + 5 call sites — behavior-neutral, WSL build/vet/tests green.)
  (R2) App.tsx + web/src/components/layout/ChatColumn.tsx — boot-hydration send gate (`bootReady` flips after roster fetch completes, 2s fail-open); composer + suggested prompts disabled until then.
  (R3) app-store.ts — `createSession` double-dispatch guard: within 250ms, if the new mint is still active, return the existing id.
verification: 
  Round 1 (primary, 2026-09-12 human-verify): PASSED — empty new session, fresh 「会话 1」+simple-qa, UUID id, roster dupIds=[], independent persistence through full pipeline.
  Self-tests: web 608/608 (75 files, incl. 3 new/updated regression tests), `tsc --noEmit` clean, `vite build` OK; Go classifier test RED→GREEN, `go build ./...` OK (pre-existing unrelated failure `TestIsBackendUnavailable_NetErrorTimeout` on original branch).
  Round 2 (residuals, 2026-09-12 human-verify after WSL backend rebuild/restart to skillforge-server-dev-new): ALL PASS — (R1) refusal history exactly 1 turn (user+assistant), UI=DB; (R2) composer disabled at mount t=676ms, enabled ~1.7s, adversarial auto-click at t=749ms produced zero sends / zero roster growth in ~2min; (R3) fast double-click minted exactly one row, roster 291→291, zero console errors.
  Coordinator verdict: confirmed fixed → archive.
  NOTE: test harness had been silently broken by an untracked pnpm install; restored with `npm ci`. `web/.env.local` (`VITE_REAL_BACKEND=1`) leaks into vitest — old composer tests were false greens; now gate-aware + transport-level assertions.
files_changed:
  - web/src/store/app-store.ts (mintSessionId UUID mint; sessionNameCounter; createSession idempotency guard + call sites)
  - web/src/App.tsx (KeyedHistorySnapshot {sessionId, resp}; inject-effect identity guard; keyed useSkillAutoSwitch; bootReady gate)
  - web/src/components/layout/ChatColumn.tsx (composer + suggested prompts disabled until bootReady)
  - internal/server/pipeline.go (refusal double-write removed; persistence owned by emitRefusalWithChunks)
  - web/src/store/app-store.test.ts (regression: minted ids cannot collide; createSession idempotency)
  - web/src/__tests__/App.test.tsx (regression: new session shows empty surface; boot gate)
  - internal/server/pipeline_history_test.go (TestVerdictPipeline_ClassifierRefuses asserts single turn pair)

## Notes

- 相关历史记忆：`fix-data-policy-unknown-payload`（DataPolicyPart 兜底分支的排查方法）、`fix-runner-sse-interrupt-chunk`（SSE 中断导致 chunk 丢失——旧会话空 assistant 消息的可能成因，范围外）。
- 环境注意：后端只能用 WSL Go 启动（见 env-wsl-backend-runbook）。
- 【用户须知】2026-09-12 验收时 orchestrator 杀掉了用户终端里的 `go run` 后端进程（pid 2890/2944），改用 runbook 的 `skillforge-server-dev-new` 二进制启动（setsid + source .env，listening :8888, backend=milvus）。用户旧终端会显示进程退出，属预期，不是故障。
- 范围外遗留（不阻塞归档）：DB 里 300+ 条 e2e/压测残留会话的清理；修复前产生的幽灵会话 d812e7d5 与双写数据留在库里属历史数据。
- pipeline.go:338 unused parameter `classifierReason`（去重写入后遗留，info 级 lint）——已于归档时清理（移除参数 + 更新 5 处调用点；WSL go build ./... / go vet / TestVerdictPipeline_ClassifierRefuses 全绿）。
- 【归档时发现的额外预存失败】`TestVerdictPipeline_SubprogressOrder`（pipeline_subprogress_test.go:101，期望 4 个 subprogress chunk 实得 3 个、缺 retrieve）在 pristine HEAD 上同样失败（stash 差分验证）——预存问题，与本会话修复无关，未修，留待单独排查。
- web 测试框架曾被未跟踪 pnpm install 静默破坏，已用 `npm ci` 恢复；若有意切 pnpm 需单独立项排查。
