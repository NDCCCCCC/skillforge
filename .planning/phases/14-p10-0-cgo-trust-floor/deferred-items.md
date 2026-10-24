# Phase 14 — Deferred Items (out-of-scope discoveries)

## 2026-08-23 — Plan 14-09: `findSkill` Phase-2 literal gate vs `simple-qa` deployment

- **Status:** acknowledged

- **File:** `internal/server/chat.go:660` (`findSkill`), `const defaultSkill = "compliance"` (chat.go:96)
- **What:** `findSkill` accepts ONLY the literal `"compliance"` and returns the first loaded
  Skill's manifest regardless of its actual name. The deployed `skills/` dir contains exactly
  one Skill, `simple-qa`, so `GET /api/admin/platform-prompt?skill=simple-qa` returns 404 and
  `POST /api/chat {"skill":"simple-qa"}` returns an out_of_scope refusal — even though
  simple-qa IS the loaded Skill. The live system works because every caller (FE default,
  curl UAT scripts) sends the literal `"compliance"`: 107/115 legacy audit rows carry
  `skill_id='compliance'`.
- **Why deferred:** Pre-existing Phase-2 simplification (documented in the function comment;
  "Phase 5 introduces … a real map lookup (SYSK-05 routing)") — not caused by 14-09's
  schema.go change and outside this plan's `files_modified`. Touching it changes chat
  routing semantics for the FE default and the accumulated audit `skill_id` vocabulary.
- **Suggested owner:** Phase 20 admin / next skill-registry pass (SYSK-05) — make findSkill
  match on `Manifest.Name` with a compatibility alias for the `"compliance"` literal.
- **Impact if unfixed:** none functional (live contract is the literal); cosmetic 404 for
  callers who use the directory name.
