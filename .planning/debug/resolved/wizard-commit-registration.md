---
status: resolved
trigger: "Phase 5 UAT Test 9 (BLOCKER) — Skill Wizard commit closed loop broken: writing skills/expense-audit/SKILL.md + firing '已注册并热加载' toast but the new skill is NEVER registered (absent from GET /api/skills, absent from Skill Manager, not picked up even after manual 全部重载). Platform thesis defeated for the wizard creation path."
severity: blocker
phase_origin: 05-skill-platform-full-knowledge-boundaries
uat_test: 9
created: 2026-08-13
updated: 2026-08-13
tdd_mode: false
goal: find_and_fix
---

# Debug Session — wizard-commit-registration

## Symptoms

- **Expected:** A skill created via the Skill Wizard, on 确认并保存, is written to disk AND registered + hot-loaded so it immediately appears in Skill Manager (`GET /api/skills`) and is routable. The platform-thesis closed loop (write Skill file → agent adapts, no code change).
- **Actual:** Commit wrote `skills/expense-audit/SKILL.md` (valid, 1088 bytes, all 7 fields match the preview) and fired the success toast "Skill 已创建：expense-audit。已注册并热加载。" + full path. BUT the skill is NOT registered: absent from `GET /api/skills` (only 企业合规/制度审查 returned — 1 skill), absent from Skill Manager dropdown, and NOT picked up even after clicking 全部重载 (manual reload). The "已注册并热加载" toast is a FALSE claim. A wizard-created skill is dead-on-arrival until a server restart.
- **Error messages:** None (silent failure — HTTP 200 returned, toast fired, but no registration).
- **Timeline:** Observed during Phase 5 UAT, 2026-08-12 ~23:59. Reproducible.
- **Reproduction:** Run wizard (5 NL answers), 生成预览 → 200, 确认并保存 → POST /api/skills/wizard/commit → 200 + toast. Then GET /api/skills → still 1 skill. Click 全部重载 → GET /api/skills → still 1 skill.

## Evidence

- 2026-08-12T16:05Z — `internal/server/skills.go` `wizardCommitHandler` (lines 198-236): after `s.Wizard.Commit` writes the file and returns `finalPath`, the handler returned `wizardCommitResponse{OK: true, Path: path}`. **No call to `Loader.Scan()` / `Registry.Reload()` / any rescan.** The docstring claimed "fsnotify watcher picks up the new file automatically (D5-11 closed loop)" — unverified documentation, not actual behavior.
- 2026-08-12T16:07Z — `cmd/server/main.go` Deps construction (lines 221-231) sets Loader/Embedder/Retriever/Runner/Agent/Store/Wizard/Knowledge/ChatModel. **`Deps.Registry` is NOT set** → nil in production. `skill.NewRegistry(...)` and `Registry.Watch(ctx)` are NEVER called from main.go. The fsnotify hot-reload subsystem the commit docstring leaned on is fictional in production.
- 2026-08-12T16:08Z — `listSkillsHandler` (skills.go lines 78-87) reads **only** `s.Loader.Skills()`. `Loader.cached` is the slice populated once by `Loader.Scan()` at startup (`cmd/server/main.go:124`); no code path mutated it after startup.
- 2026-08-12T16:10Z — `server.go` route table (192-197): `/api/chat`, `/api/skills` (GET+POST), `/api/skills/wizard`, `/api/skills/wizard/commit`, `/api/knowledge/graph`. **No `/api/skills/reload`** (so `全部重载` POSTs into a 404) and **no `/api/skills/events` SSE** (Issue #7).
- 2026-08-12T16:13Z — `wizardCommitResponse` (skills.go 63-70): fields OK/Path/Error only — no Registered/SkillsTotal, so FE could not distinguish "file written" from "skill registered".
- 2026-08-12T16:14Z — `web/src/hooks/useWizard.ts` `commit()` (206-245): on `body.ok === true`, dispatched `wizard-success` (toast) IMMEDIATELY with no follow-up GET /api/skills verification.
- 2026-08-12T16:15Z — `SkillManagerMenu.tsx` lists skills from `__fixtures__/skills.json` (static import), NOT `/api/skills`; `全部重载` POSTs to nonexistent `/api/skills/reload` → 404. (Skill Manager IA is Issue #2; out of scope here.)
- 2026-08-12T16:25Z — backend fix applied. `skills.go`: `wizardCommitResponse` gained `Registered bool` + `SkillsTotal int`; `wizardCommitHandler` now calls `s.Loader.Scan()` synchronously after `Wizard.Commit` and verifies `req.Name` appears in the rescanned cache before setting `Registered=true`. `loader.go`: `Scan()` stamps `m.Name = filepath.Base(filepath.Dir(path))` so the handler can match rescanned manifests to their dir id.
- 2026-08-12T16:30Z — frontend fix applied. `web/src/types/wizard.ts`: `WizardCommitResponse` gained `registered?: boolean` + `skillsTotal?: number`. `web/src/hooks/useWizard.ts`: `commit()` gates `wizard-success` dispatch on `body.registered === true`; on `ok=true && registered=false` surfaces an accurate error and stays in previewing phase.
- 2026-08-13T00:35Z — orchestrator review found a NEW data race introduced by the fix: `Loader.cached []*Manifest` had NO mutex; previously written once at startup then read-mostly. The fix makes `wizardCommitHandler` (request goroutine) call `Scan()` (writes `l.cached`) concurrent with `listSkillsHandler` (another request goroutine) calling `Skills()` (reads `l.cached`) → concurrent read/write data race, undetected by sequential tests. Fixed by adding `sync.RWMutex`: `Skills()` takes RLock; `Scan()` guards the `l.cached = out` swap with Lock. Added `TestLoader_ScanAndSkillsConcurrentNoRace` to pin it under `-race`.

## Eliminated

- (hypothesis) "fsnotify watches only startup-known dirs, missing new subdirs" — REFUTED: stronger root cause is that `Deps.Registry` is never constructed in `cmd/server/main.go` at all, so NO fsnotify watcher runs anywhere in production. The fix bypasses the fictional fsnotify path entirely by having the commit handler rescan the Loader synchronously.

## Resolution

- root_cause: Three-gap chain. (1) `cmd/server/main.go` never sets `Deps.Registry` → the fsnotify hot-reload subsystem is entirely absent in production (not just missing new-subdir watches). (2) `wizardCommitHandler` returned `ok:true` on file-write alone with no rescan. (3) `listSkillsHandler` reads only the startup-only `Loader.cached`. Net: no live path registered a wizard-created skill. The FE compounded it by firing the "已注册并热加载" toast optimistically on `body.ok === true`.
- fix:
  - Backend (`internal/server/skills.go`): `wizardCommitResponse` gained `Registered bool` + `SkillsTotal int`; `wizardCommitHandler` now calls `s.Loader.Scan()` synchronously after the atomic write and sets `Registered=true` only when `req.Name` actually appears in the rescanned cache. Loader-nil / scan-error / name-absent all yield `Registered=false` with a non-empty Error so the FE never shows a false success.
  - Backend (`internal/skill/loader.go`): `Scan()` stamps `Manifest.Name = filepath.Base(filepath.Dir(path))` (dir-id match for the handler). **Race fix:** added `sync.RWMutex` guarding `cached` — `Skills()` RLock, `Scan()` Lock around the `l.cached = out` publish (filesystem walk stays lock-free).
  - Frontend (`web/src/types/wizard.ts`, `web/src/hooks/useWizard.ts`): `WizardCommitResponse` gained `registered?` + `skillsTotal?`; `commit()` gates the `wizard-success` toast on `body.registered === true`, else surfaces an accurate inline error and stays in the previewing phase.
- verification: `go build ./...` exit 0; `go test ./internal/server/ ./internal/skill/` ok; `go test -race ./internal/skill/ ./internal/server/` ok (incl. new `TestLoader_ScanAndSkillsConcurrentNoRace` + `TestWizardCommitHandler_RegistersAndRefreshesLoader` closed-loop integration test + `TestWizardCommitHandler_RegisteredFalseWhenLoaderNil` + `TestLoader_ScanStampsManifestNameFromParentDir`); `go vet ./...` exit 0; FE `tsc --noEmit` exit 0; `vitest run` 29 files / 194 tests pass (incl. useWizard UAT Test 9 regression). The integration test reproduces the UAT scenario at the HTTP layer (POST commit → GET /api/skills lists the new skill without restart).
- files_changed: internal/server/skills.go, internal/server/skills_wizard_test.go, internal/skill/loader.go, internal/skill/loader_test.go, web/src/types/wizard.ts, web/src/hooks/useWizard.ts, web/src/__tests__/useWizard.test.tsx. Diagnostic test dir `skills/expense-audit/` removed (was untracked, left for diagnosis).
- live_browser_confirmation: OPTIONAL, not required for closure — the HTTP-layer integration test faithfully reproduces the failure mode (commit → rescan → GET /api/skills). The remaining browser-only surface (LLM preview generation → commit handoff) is unchanged in structure and unit-tested; the commit endpoint makes no LLM call.

## Out of scope (NOT touched, correctly deferred)

- Issue #7 — FE→BE reload bridge for *existing* skills (SSE 404 on `/api/skills/events`, no `/api/skills/reload` route, no version field on the HTTP payload).
- Issue #2 — Skill Manager menu reads from a static fixture `__fixtures__/skills.json`, not `/api/skills`.
