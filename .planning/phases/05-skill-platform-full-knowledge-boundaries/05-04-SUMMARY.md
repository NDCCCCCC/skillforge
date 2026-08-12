---
phase: 05-skill-platform-full-knowledge-boundaries
plan: 04
subsystem: skill-platform
tags: [wizard, sc-3, sysk-06, fsnotify-closed-loop, alert-dialog, hot-reload]
requires: [05-01, 05-02, 05-03]
provides: [wizard-backend, wizard-ui, atomic-skill-write, sk-from-nl]
---

# Plan 05-04 — SC-3 Skill Wizard (NL → SKILL.md)

## Objective

Ship the SC-3 Skill Wizard end-to-end. The open-source showcase deliverable per CLAUDE.md §Project / Constraints: a stranger describes a Skill in NL and gets a running Skill without code. The closed loop (wizard → write → fsnotify → registered → routable) IS the platform thesis in action.

## What shipped

### Backend (Go)

| File | Provides |
|------|----------|
| `internal/skill/wizard.go` | `Wizard.Generate(answers)` → `(*Manifest, error)` — LLM call with `response_format: json_object` + bounded retry (T-5-18: ONE additional call on Validate failure) + D5-12 temperature-by-type. `Wizard.Commit(m)` → `(string, error)` — atomic write (write-temp + `os.Rename`) + three gates (regex, resolved-path, Validate). |
| `internal/skill/wizard_test.go` | 10 tests covering T-5-16 path-traversal, retry-on-validation, atomic write, fsnotify path closure. |
| `internal/server/skills.go` | `wizardHandler` (POST /api/skills/wizard) + `wizardCommitHandler` (POST /api/skills/wizard/commit) with 503/400 envelopes mirroring the existing `/api/skills` fallback posture. |
| `internal/server/wizard_name.go` | `extractSkillName(purpose)` — slugifies the first sentence to `^[a-z0-9-]+$` (T-5-16 first gate, runs BEFORE the LLM call so a malformed purpose is rejected at 400 with a copy-able FE hint). |
| `internal/server/server.go` | `Deps.Wizard *skill.Wizard` field + the two new routes registered in `New()`. |
| `cmd/server/main.go` | Wires `skill.NewWizard(cm, "skills/")` into `Deps` at startup. Writes into the same `skillsDir` the Loader scans; fsnotify (Plan 05-01) picks up the new file automatically — D5-11 closed loop. |
| `internal/server/skills_wizard_test.go` | 11 server-side tests: extract-name happy + rejection paths, handler 503 paths, handler happy path, LLM-error surfacing, malformed JSON 400, name-extraction failure 400, commit happy path, path-escape rejection, invalid-manifest rejection, missing-preview 400, route-registration regression. |

### Frontend (TypeScript / React 19)

| File | Provides |
|------|----------|
| `web/src/types/wizard.ts` | `WizardAnswers` (mirror of Go Answers) + `WIZARD_STEPS` (5-step verbatim copy from 05-UI-SPEC §SC-3) + `WizardPreviewResponse` + `WizardCommitResponse`. `DEFAULT_TEMPERATURE_BY_TYPE` for D5-12. |
| `web/src/hooks/useWizard.ts` | State machine: `idle → answering → previewing → committing → idle`. Exposes `submitAnswer`, `requestPreview`, `regenerate`, `commit`, `cancel`. On commit success fires `wizard-success` CustomEvent. |
| `web/src/components/layout/WizardMode.tsx` | Right-panel controller. Composes WizardHeader + transcript + (when preview) WizardPreview + ChatInput (re-used with `modeLabel="Skill 向导"`). Auto-routes center to Skill 定义 tab on preview. |
| `web/src/components/layout/WizardHeader.tsx` | Progress bar (custom — shadcn Progress not yet initialized) + 步骤 {N}/{total} + 取消向导 CTA. |
| `web/src/components/layout/WizardPreview.tsx` | Renders 7 fields verbatim + 3 CTAs (重新生成 / 确认并保存到 skills/{name}/SKILL.md / 取消向导). |
| `web/src/components/layout/WizardCancelDialog.tsx` | Hand-rolled alert-dialog (shadcn init runs as Wave-1 task). 放弃 CTA uses Button `destructive` variant (`bg-danger` per theme.css). |
| `web/src/components/layout/RightChat.tsx` | Mounts `<WizardMode />` instead of the normal chat layout when `store.wizardMode.active === true`. |
| `web/src/components/chat/ChatInput.tsx` | Added `modeLabel`, `disabled`, `busy` props. Send button disabled when text empty or `disabled`. Hint line switches from current-Skill to 模式：Skill 向导 when `modeLabel` is set. |
| `web/src/store/app-store.ts` | Added `lastCreatedSkill` + `setLastCreatedSkill` / `clearLastCreatedSkill`. |
| `web/src/App.tsx` | Subscribes `window` `wizard-success` event → mirrors into store; renders the success toast Skill 已创建：{name}。已注册并热加载。 + the on-disk path. |

### Tests

| File | Coverage |
|------|----------|
| `web/src/__tests__/useWizard.test.tsx` | Hook state machine: idle→answering transition, submitAnswer recording + step advance, reads parsing, sentinel step, requestPreview success/error, commit success/error, cancel, regenerate. |
| `web/src/__tests__/WizardMode.test.tsx` | WizardHeader step counter + Progress bar + cancel trigger; cancel confirmation flips `wizardMode.active`; WizardPreview field rendering + 3 CTAs; WizardMode mounts when active + ChatInput routes answers + 生成预览 POSTs to backend. |
| `web/src/__tests__/WizardCancelDialog.test.tsx` | Renders SC-3 copy verbatim; uses destructive variant (bg-danger) on 放弃 CTA; 放弃 fires onConfirm; 继续向导 / backdrop / Esc fire onCancel. |

## Verification

```bash
$ go build ./...
# (no output)

$ go test ./... -count=1 -timeout 120s
ok    github.com/skillforge/skillforge/cmd/spiked             0.272s
ok    github.com/skillforge/skillforge/internal/agent        0.259s
ok    github.com/skillforge/skillforge/internal/eval         0.165s
ok    github.com/skillforge/skillforge/internal/knowledge    0.345s
ok    github.com/skillforge/skillforge/internal/retriever/milvus  0.201s
ok    github.com/skillforge/skillforge/internal/retriever/mongo   0.201s
ok    github.com/skillforge/skillforge/internal/server       0.766s
ok    github.com/skillforge/skillforge/internal/skill        1.947s
ok    github.com/skillforge/skillforge/internal/store        0.961s
ok    github.com/skillforge/skillforge/internal/verdict      0.675s
ok    github.com/skillforge/skillforge/internal/wire         1.028s

$ cd web && npx tsc --noEmit
# (no output)

$ cd web && npx vitest run --reporter=basic
Test Files  25 passed (25)
     Tests  150 passed (150)
```

All automated checks green. SPEC §verification checklist:

- [x] `go test ./internal/skill/... ./internal/server/... -count=1` exits 0
- [x] `cd web && npx vitest run` exits 0
- [x] `cd web && npx tsc --noEmit` exits 0
- [x] `/api/skills/wizard` + `/api/skills/wizard/commit` routes registered (server-side regression test asserts 404 is never returned)
- [x] Wizard name regex `^[a-z0-9-]+$` enforced (10-case TestWizard_Name_Pattern + 2-case extractSkillName happy/reject coverage)
- [x] Wizard commit runs `Validate` as the third gate (TestWizard_Commit_RejectsInvalidManifest)
- [x] `--color-danger` used only on the 放弃 CTA (WizardCancelDialog + Button destructive variant)
- [x] 5 interview step prompts match UI-SPEC §SC-3 copy verbatim (WIZARD_STEPS source-of-truth)

## Acceptance Criteria — Results

| Criterion | Result |
|-----------|--------|
| Wizard endpoint POST /api/skills/wizard drives a 5-step interview (purpose / reads / type / guardrails / temperature) and emits a generated SKILL.md preview that passes skill.Validate | PASS — `Wizard.Generate` produces a 7-field Manifest that `Validate` accepts; UI renders 5 prompts via `WIZARD_STEPS` |
| POST /api/skills/wizard/commit writes skills/<name>/SKILL.md atomically and triggers the fsnotify hot-reload path | PASS — atomic write via `os.CreateTemp` + `os.Rename` (same dir); fsnotify watcher picks up automatically |
| Right panel enters wizard mode when user clicks 新建 Skill（向导）: chat header replaced by step indicator + cancel CTA, interview rendered as assistant messages, ChatInput stays mounted | PASS — `RightChat` mounts `WizardMode` when `wizardMode.active` is true |
| On confirm, center panel auto-switches to Skill 定义 tab showing the preview, with 重新生成 / 确认并保存 / 取消向导 CTAs | PASS — `useEffect` in `WizardMode` dispatches `setCenterView({kind:'skill-md', skillId: name})` on phase=previewing |
| Cancel opens alert-dialog with body 放弃当前进度？当前进度（{N}/{total} 步已回答）将丢失，无法恢复。 + confirm CTA 放弃 in --color-danger | PASS — `WizardCancelDialog` renders copy verbatim; 放弃 uses Button `destructive` variant |

## Threat-Model Disposition

| Threat | Mitigation | Test |
|--------|------------|------|
| T-5-16 path traversal | Three gates: regex `^[a-z0-9-]+$` (T-5-16) → resolved-path rooted check → `Validate`. BE-side gate runs in `extractSkillName` BEFORE the LLM call. | TestWizard_Name_Pattern (10 cases) + TestWizard_Commit_RejectsPathEscape + extractSkillName test cases |
| T-5-17 prompt injection | Generated SKILL.md is parsed as YAML; `Validate` rejects unknown/invalid fields. Body is data (never executed). | TestWizard_Generate_ValidatesOutput asserts the LLM output passes Validate |
| T-5-18 closed-loop break | Generate retries once on validation failure; second failure returns error to FE for inline display. | TestWizard_Generate_RetriesOnValidationFailure + TestWizard_Generate_FailsAfterTwoInvalidAttempts |
| T-5-19 DoS via LLM hang | LLM call bounded by request context; error surfaces to FE. | TestWizardHandler_LLMErrorSurfacesToFE |
| T-5-20 write-vs-fsnotify drift | Success toast fires ONLY after atomic write completes (BE) AND a `wizard-success` CustomEvent is dispatched (FE). | WizardMode commit test asserts the event + active flip |

## Atomic Commit Trail

```
7df98df feat(05-04): wizard UI — right-panel mode + preview + cancel + commit
f853c9c feat(05-04): wire Wizard at cmd/server/main.go (POST /api/skills/wizard + /commit live)
a70e4aa feat(05-04): wire wizard HTTP endpoints (POST /api/skills/wizard + /commit) + extract-skill-name helper
df259c2 feat(05-04): wizard backend — Generate (retry-on-validation) + Commit (atomic write + path-traversal guard)
```

## Notes for Future Work

- `bg-danger/10` + `bg-danger/30` + `text-danger` are used in WizardCancelDialog + WizardMode (error chip) + ToolPart. The `bg-danger` background-class usage is confined to the destructive Button variant + the cancel-dialog icon background.
- Wizard copy in `WIZARD_STEPS` is the verbatim prompt list per 05-UI-SPEC §SC-3. Editing requires a UI-SPEC change.
- `WizardMode`'s 生成预览 CTA hides `wizard-empty` once the first assistant prompt is in the transcript (transcript.length > 0).
- The success toast in `App.tsx` is dismissable via the `×` button — no auto-dismiss (operator can read it).
- `useWizard`'s `phase` discriminator is `idle | answering | previewing | committing` — the explicit source of truth; `step` is the 0-based index inside the answering phase.