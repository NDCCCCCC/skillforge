---
phase: 05-skill-platform-full-knowledge-boundaries
fixed_at: 2026-08-12T13:38:00Z
review_path: .planning/phases/05-skill-platform-full-knowledge-boundaries/05-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 5
status: all_fixed
---

# Phase 5 Code Review Fix Report

**Fixed at:** 2026-08-12T13:38:00Z
**Source review:** `.planning/phases/05-skill-platform-full-knowledge-boundaries/05-REVIEW.md`
**Iteration:** 1
**Fix scope:** `critical_warning` (Warning findings only — no Critical findings were raised)

**Summary:**
- Findings in scope: 3 (W-1, W-2, W-3)
- Fixed: 3
- Skipped: 5 (I-1..I-5 — out of scope per `fix_scope: critical_warning`)

## Fixed Issues

### W-1 — Wizard commit endpoint silently overwrites existing Skills

**Files modified:** `internal/skill/wizard.go`, `internal/skill/wizard_test.go`, `internal/server/skills.go`, `internal/server/skills_wizard_test.go`
**Commit:** `5d497b6`
**Test result:** `go test ./internal/skill/... ./internal/server/...` → PASS (skill 1.987s, server 0.920s). Full `go test ./internal/...` → all packages PASS.

**Applied fix (default-deny option (a) from the review):**

- `internal/skill/wizard.go`: added exported sentinel `ErrSkillAlreadyExists`. `Wizard.Commit` now runs a fourth gate — `os.Stat(finalPath)` on `skills/<name>/SKILL.md` — immediately after the existing belt-and-braces `HasPrefix` check and BEFORE `MkdirAll` / temp-file / rename. If the file exists, `Commit` returns a wrapped `ErrSkillAlreadyExists` with the operator-guiding message `skill %q already exists — pick a different name or remove the existing directory`. The refusal therefore runs before any disk write, so the originally-committed `SKILL.md` is preserved byte-for-byte. Updated the `Commit` docstring to document the fourth gate. The check targets the FILE (not the directory): an orphan `skills/<name>/` directory without `SKILL.md` (e.g. left by a previously-failed commit) does NOT block the name — only the `SKILL.md` file is the existence signal. `MkdirAll` remains idempotent.
- `internal/server/skills.go`: `wizardCommitHandler` now does `errors.Is(err, skill.ErrSkillAlreadyExists)` and returns HTTP **409 Conflict** with the existing `{ok:false, error:"..."}` envelope. The 409 status distinguishes "name taken, pick another" from the 200 + `OK:false` validation-failure path so the FE can surface the right inline copy. The atomic-rename behavior for the genuinely-new case is preserved unchanged.
- `internal/skill/wizard_test.go`: added `TestWizard_Commit_RefusesExistingSkill` (second commit refuses with `errors.Is(err, ErrSkillAlreadyExists)`; originally-committed body preserved byte-for-byte) and `TestWizard_Commit_AllowsOrphanDirectory` (only the `SKILL.md` file is the existence signal — an orphan dir without the file does not block).
- `internal/server/skills_wizard_test.go`: added `TestWizardCommitHandler_RefusesExistingSkillWith409` (end-to-end: first commit 200+OK:true, second commit 409+OK:false with descriptive error, original file preserved byte-for-byte).

Existing tests `TestWizardCommitHandler_HappyPath`, `TestWizardCommitHandler_RejectsPathEscape`, `TestWizardCommitHandler_RejectsInvalidManifest`, and `TestWizard_Commit_AtomicWrite` continue to pass unchanged — the new gate is transparent for the genuinely-new path and only fires on a real name collision.

**Note on status code choice:** the existing convention in `skills.go` is HTTP 200 + `{OK:false, Error:...}` for validation failures (bad name, invalid manifest). The prompt guidance and REVIEW.md both explicitly said "409-style" for the conflict case, and conflict is semantically distinct from validation failure (Conflict = "request well-formed but conflicts with current resource state", vs. 200-with-OK:false = "validation result IS the payload"). I used HTTP 409 with the same `{ok:false, error:...}` envelope shape — honors "409-style" while reusing the existing envelope helper.

### W-2 — `downloadMarkdown` filename built from LLM-controlled `clauseId` without sanitization

**Files modified:** `web/src/components/verdict/RedlineCard.tsx`, `web/src/__tests__/RedlineCard.test.tsx`
**Commit:** `a135a98`
**Test result:** `npx vitest run src/__tests__/RedlineCard.test.tsx` → 12/12 PASS (9 original + 3 new). Full FE suite `npx vitest run` → 29 files / 193 tests PASS. `npx tsc --noEmit -p tsconfig.json` → clean.

**Applied fix (sanitization at the filename-construction point in RedlineCard):**

- `web/src/components/verdict/RedlineCard.tsx`: in `handleDownload`, replaced the raw `(data.clauseId ? data.clauseId : 'redline') + '.md'` with the review-specified sanitization chain:
  ```ts
  const safeClauseId = (data.clauseId ?? '')
    .replace(/[\\/:*?"<>|\x00-\x1f]/g, '-')   // path separators + control chars
    .replace(/^-+|-+$/g, '')                   // trim leading / trailing dashes
    .slice(0, 64)                              // cap length
    || 'redline';                              // fall back when nothing usable remains
  ```
  Applied at the point where the filename is built (`RedlineCard.handleDownload`) rather than inside `clipboard.downloadMarkdown`. The `clipboard.ts` helper retains its simple string-in/string-out contract — this matches the review's "in `RedlineCard.handleDownload`" recommendation and keeps `downloadMarkdown` reusable for future callers that may pass already-sanitized names.
- `web/src/__tests__/RedlineCard.test.tsx`: added 3 tests covering (1) path-separator collapse (`'a/b\\c:d'` → `'a-b-c-d.md'`), (2) all-bad fallback (`'/\\:'` → `'redline.md'`), and (3) 64-char cap (`'a'.repeat(100)` → 64 a's + `.md`). The two pre-existing tests (`'C-7.3.2.md'` happy path and `'redline.md'` absent-clauseId fallback) continue to pass unchanged — the sanitization is transparent for already-safe input.

**Note on FE test runner:** the worktree had no `node_modules/` (worktree shares tracked files but not gitignored dirs). I installed a directory junction from the worktree's `web/node_modules/` to the main repo's pre-installed `node_modules/` (no copy, no lockfile mutation, gitignored so it does not appear in the commit). `npm ci` against the worktree's own `package-lock.json` fails with `Missing: lightningcss-*` — a pre-existing platform-optional-deps issue in the committed lockfile, unrelated to this fix.

### W-3 — Knowledge graph 500 envelope echoes internal metadata to the client

**Files modified:** `internal/server/knowledge.go`, `internal/server/knowledge_test.go`
**Commit:** `e85a44c`
**Test result:** `go test ./internal/server/... -run TestGraphHandler -v` → 4/4 PASS. Confirmed server-side `[WARN] server: knowledge graph build failed: milvus unavailable` log emission in the test output. Full `go test ./internal/...` → all packages PASS.

**Applied fix (log full error server-side, generic message to client):**

- `internal/server/knowledge.go`: added `log` import. The 500 path now logs `log.Printf("[WARN] server: knowledge graph build failed: %v", err)` (mirrors the `[WARN]` convention used in `pipeline.go:267,270` and `chat.go:257,350,360`) and returns the generic `Error: "knowledge graph build failed — see server logs"` to the FE. The 503 "not initialized" copy is unchanged — it carries no internal data and the review explicitly said to keep it verbatim. Updated the file's top-level wire-contract docstring to reflect the new 500 message and the rationale (internal metadata leak risk).
- `internal/server/knowledge_test.go`: updated `TestGraphHandler_BuildErrorReturns500` to assert the generic message is returned AND that internal detail (`"milvus"`) does NOT leak. Test header docstring updated to pin the new acceptance shape (#3 in the file's "4 acceptance assertions" list).

## Skipped Issues

The following Info findings were skipped because they are **out of scope** for this run (`fix_scope: critical_warning`). The orchestrator launched this fixer with explicit instructions: "OUT OF SCOPE (skip, do NOT fix): all five Info findings I-1 through I-5. Document them as 'skipped — out of scope (Info)' in REVIEW-FIX.md."

### I-1 — Doc comment mis-describes V8 enforcement mechanism

**File:** `internal/wire/redline.go:121-123`
**Reason:** skipped — out of scope (Info)
**Original issue:** The `UnmarshalRedline` doc comment claims the required-field check (V8 core) happens automatically inside `json.Unmarshal` because `OriginalClause`/`ProposedRewrite`/`Rationale` have no `omitempty` tag. This is incorrect — `encoding/json` does not error on missing non-pointer fields regardless of `omitempty`. The actual V8 enforcement lives in `RedlinePayload.UnmarshalJSON` (lines 84-112) via the pointer-sentinel pattern. Risk: a future maintainer could remove `UnmarshalJSON` as "redundant" and silently break V8.

### I-2 — Chunk IDs in GraphBuilder are sorted lexically, not numerically

**File:** `internal/knowledge/graph.go:192-195`
**Reason:** skipped — out of scope (Info)
**Original issue:** `sort.SliceStable` uses string `<` comparison on `ChunkNode.ID`, producing `1, 10, 100, 2, 20` ordering — confusing for a human browsing chunks in the SC-5 Accordion. Cosmetic only; no correctness impact.

### I-3 — Wizard "Body is data (never executed)" framing is technically imprecise

**File:** `.planning/.../05-04-SUMMARY.md` T-5-17 disposition; `internal/skill/wizard.go:88-103`
**Reason:** skipped — out of scope (Info)
**Original issue:** The T-5-17 mitigation claim undersells the surface: the generated `Manifest.Steps` field IS injected into the agent's runtime prompt context when the Skill is loaded. So prompt-injection content authored into `steps` reaches future LLM calls. Acceptable under the documented operator-trust threat model (the wizard user previews the SKILL.md before commit), but the framing should be tightened so future maintainers don't relax `steps` sanitization. Recommended remediation is a 1-line comment, no code change.

### I-4 — `int64ToChunkID` comment is wrong about `strconv.FormatInt`

**File:** `internal/knowledge/graph.go:211-234`
**Reason:** skipped — out of scope (Info)
**Original issue:** The comment claims `strconv.FormatInt` is heavy and that the hand-rolled version "avoids an extra import". Both claims are inaccurate (`strconv` is neither heavy nor an extra import — `time`/`sort` already pull it in transitively). The hand-rolled code is correct; only the justification is wrong. Style/DOC only.

### I-5 — Unchecked `row.tier as Tier` cast in CounterEvidenceCard

**File:** `web/src/components/verdict/CounterEvidenceCard.tsx:86`
**Reason:** skipped — out of scope (Info)
**Original issue:** `ConflictChunk.tier` is typed `string` on the wire; the `row.tier as Tier` cast is unchecked. The BE only emits "历史" today (pinned by `TestCounterEvidence_PassesHistoryTierOnly`), so production-safe. But the `ConflictChunk.Tier` godoc at `internal/wire/chunk.go:91` explicitly anticipates "a future cross-tier expansion (e.g. 草稿)" — an unexpected value would silently propagate into the store.

---

_Fixed: 2026-08-12T13:38:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
