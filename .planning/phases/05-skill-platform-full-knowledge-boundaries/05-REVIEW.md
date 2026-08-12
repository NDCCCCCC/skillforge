---
phase: 05-skill-platform-full-knowledge-boundaries
review: code-review
status: issues
reviewer: gsd-code-reviewer
date: 2026-08-12
scope: [05-04, 05-05, 05-06]
depth: standard
---

# Phase 5 Code Review — Skill Platform & Full Knowledge Boundaries

## Verdict

**status: issues** — All seven critical security/trust focus areas are correctly implemented at the code level and verified by load-bearing regression tests. No Critical findings. Three Warning-class issues worth addressing before the open-source release (mostly around deployment posture and LLM-output hardening), plus five Info-class observations.

The headline trust claims hold up under direct inspection:

- **D5-13 trust moat** — `emitCounterEvidence(ctx, c, qvec, now)` genuinely does NOT receive `chunksByID` (signature-enforced). `pipeline.go:266` calls it without the grounding map. `TestCounterEvidence_NotGrounded` snapshots `chunksByID` and asserts byte-identity after the pass. FE moat mirrored: `CounterEvidenceCardProps` accepts ONLY `conflictChunks`. **Verified.**
- **D5-14 cost control** — The first executable line of `emitRedline` is the status gate (`redline.go:108`), running BEFORE the `ChatModel == nil` check and before any `Generate` call. `TestRedline_Conditional/compliant_skips_LLM` asserts `stubCM.count == 0` on 合规. **Verified.**
- **Path traversal** — Three gates in `Wizard.Commit` (regex `^[a-z0-9-]+$` → `EvalSymlinks(skillsRoot)` → `Validate`) plus a belt-and-braces `HasPrefix(skillDir, skillsRoot+sep)` check. Regex forbids `/` and `..`, so join is single-segment. **Verified.**
- **XSS** — Grep confirms the only mentions of `dangerouslySetInnerHTML` in `web/src` are in comments/tests asserting its absence. InspectorToolCall, CounterEvidenceCard, RedlineCard, ChunkSourceView all render LLM/retriever text via React's default text-node escaping. **Verified.**
- **T-5-22 graph content leak** — `knowledge.ChunkNode` (Go) carries only `{ID, EffectiveFrom, EffectiveTo, Version}`. TS mirror at `web/src/types/knowledge.ts` has the same shape — no `content` field exists for a future BE bug to populate. **Verified.**
- **V8 wire smuggling** — `RedlinePayload.UnmarshalJSON` uses pointer-field sentinels (`*string`) to distinguish absent from empty and rejects missing required fields. `UnmarshalRedline` adds `DisallowUnknownFields()`. **Verified** (with a doc-comment nit below).

## Findings

### Warning

#### W-1 — Wizard commit endpoint is unauthenticated and silently overwrites existing Skills
**Files:** `internal/server/skills.go:193-222`, `internal/skill/wizard.go:256-342`, `internal/server/server.go:189-198`

`POST /api/skills/wizard/commit` has no auth middleware (confirmed: `server.New()` registers routes directly with no auth layer), no "name already exists" check (grep confirms zero matches for `os.IsExist` / "already exists" / `existing skill` across `internal/`), and uses `os.CreateTemp` + `os.Rename` which atomically REPLACES any existing `skills/<name>/SKILL.md`. Combined with `MkdirAll(skillDir, 0o755)` (idempotent — never fails on existing dir), any network-reachable instance allows:

1. **Integrity tampering** — an attacker or careless operator can overwrite the canonical `skills/compliance/SKILL.md` (the trust root for the compliance verdicts) with a generated variant. The fsnotify watcher picks it up immediately and the next verdict uses the new definition.
2. **Skill squatting** — commit a Skill named `compliance` with relaxed guardrails (<3 prohibitions won't pass Validate, but exactly 3 weak ones will).

The CLAUDE.md deployment model ("v1 本地启动 + Docker Compose") makes this acceptable for the local-dev case, but the project is explicitly an open-source release vehicle ("开源发布，用于建个人技术品牌"). If an operator exposes the port publicly (a very common Docker Compose mistake), the trust root is remotely mutable.

**Recommended remediation:**
- Add a guard in `Wizard.Commit`: if `finalPath` exists, either (a) refuse with a 409-style `{ok:false, error:"skill '<name>' already exists — pick a different name or remove the existing directory"}`, or (b) require an explicit `overwrite bool` field in `wizardCommitRequest`. Option (a) matches the "default-deny" philosophy in CLAUDE.md.
- Document in the README / deploy guide that `/api/skills/wizard*` and `/api/skills` POST must not be exposed beyond localhost (Docker `ports: 127.0.0.1:8888:8888` not `8888:8888`).

#### W-2 — `downloadMarkdown` filename is built from LLM-controlled `clauseId` without sanitization
**File:** `web/src/components/verdict/RedlineCard.tsx:67`, `web/src/lib/clipboard.ts:40-52`

```ts
const filename = (data.clauseId ? data.clauseId : 'redline') + '.md';
```

`data.clauseId` originates from the redline LLM's JSON output (`wire.RedlinePayload.ClauseID`). The string flows unvalidated into `a.download = filename`. Modern evergreen browsers (Chrome/Firefox/Safari) sanitize path separators in the `download` attribute, so the practical exploit surface is narrow. However:

- The redline LLM is a fresh producer whose output the project has not yet seen in production — defense-in-depth is warranted.
- A `clauseId` containing `../`, `\\`, NUL, or control characters could in principle be interpreted by file managers or older browser engines when the operator clicks "show in folder" after download.
- The V8 mitigation (`UnmarshalRedline` rejects unknown fields) does NOT validate the CONTENT of known fields; only their presence.

**Recommended remediation** in `RedlineCard.handleDownload`:
```ts
const safeClauseId = (data.clauseId ?? '')
  .replace(/[\\/:*\?"<>\|\x00-\x1f]/g, '-')
  .replace(/^-+|-+$/g, '')
  .slice(0, 64) || 'redline';
const filename = safeClauseId + '.md';
```

#### W-3 — Knowledge graph 500 envelope echoes internal metadata to the client
**File:** `internal/server/knowledge.go:74-78`, `internal/knowledge/graph.go:58-75`

`graphHandler` returns `err.Error()` directly in the 500 body. `ErrInvalidTier` and `ErrEmptySource` embed the doc's numeric ID, source path, and tier value. This pattern matches `skills.go` (which echoes Go json unmarshal errors), so it's consistent with the existing posture — and for a trusted-operator tool this is genuinely useful for debugging.

However, the same open-source-deployment concern as W-1 applies: if `/api/knowledge/graph` is exposed, an attacker can probe corpus shape (numeric IDs hint at row counts; source paths leak the corpus filename taxonomy) by triggering validation errors via a hand-edited retriever fixture or a corpus migration.

**Recommended remediation:** Log the full `err.Error()` server-side at `[WARN]`; return a generic `"knowledge graph build failed — see server logs"` to the FE on 500. Keep the 503 "not initialized" copy verbatim (it carries no internal data).

### Info

#### I-1 — Doc comment mis-describes V8 enforcement mechanism
**File:** `internal/wire/redline.go:121-123`

```go
// The required-field check (V8 core) happens automatically inside
// json.Unmarshal because OriginalClause / ProposedRewrite / Rationale
// have no omitempty tag — missing fields return an error.
```

This is incorrect — `encoding/json` does NOT error on missing non-pointer fields regardless of `omitempty`; it leaves them at zero value. The actual V8 enforcement lives in `RedlinePayload.UnmarshalJSON` (lines 84-112) via the pointer-sentinel pattern. The comment there is correct.

The risk: if a future maintainer reads the `UnmarshalRedline` comment, believes the protection is structural to `json.Unmarshal`, and removes `UnmarshalJSON` as "redundant", V8 silently breaks (a payload missing `rationale` would decode to an empty string and the FE would render a blank rationale block).

**Recommended remediation:** Replace the misleading comment with: "The required-field check (V8 core) is enforced by `RedlinePayload.UnmarshalJSON` via pointer-field sentinels — see that method. `DisallowUnknownFields` here adds defense-in-depth against unexpected producer-added fields."

#### I-2 — Chunk IDs in GraphBuilder are sorted lexically, not numerically
**File:** `internal/knowledge/graph.go:192-195`

```go
sort.SliceStable(acc.chunks, func(i, j int) bool {
    return acc.chunks[i].ID < acc.chunks[j].ID
})
```

`ChunkNode.ID` is a `string` (the wire shape). Lexical ordering puts chunks `1, 10, 100, 2, 20` in that order, which is confusing for a human browsing a doc's chunks in the SC-5 Accordion. The summary's "deterministic order" claim is technically true but reads as numeric-intent. Cosmetic — no correctness impact.

**Recommended remediation:** Parse to int64 for the comparator, fall back to lexical on parse error:
```go
sort.SliceStable(acc.chunks, func(i, j int) bool {
    ni, ei := strconv.ParseInt(acc.chunks[i].ID, 10, 64)
    nj, ej := strconv.ParseInt(acc.chunks[j].ID, 10, 64)
    if ei == nil && ej == nil { return ni < nj }
    return acc.chunks[i].ID < acc.chunks[j].ID
})
```

#### I-3 — Wizard "Body is data (never executed)" framing is technically imprecise
**File:** `.planning/.../05-04-SUMMARY.md` Threat-Model Disposition T-5-17, plus implicit in `internal/skill/wizard.go:88-103`

The T-5-17 mitigation claim ("Body is data, never executed") undersells the actual surface. The generated `Manifest.Steps` field IS injected into the agent's runtime prompt context when the Skill is loaded (see `internal/server/pipeline.go:71-92` `systemPromptTemplate` and `internal/agent/*` consumption). So prompt-injection content authored into `steps` reaches future LLM calls — it is "instructions" not "data" in the strict sense the summary implies.

This is **acceptable under the documented operator-trust threat model** (the wizard user previews the SKILL.md before commit, and is the same user who could hand-edit any existing Skill). But the framing should be tightened so future maintainers don't relax `steps` sanitization on the assumption that it's inert data.

**Recommended remediation:** Add a 1-line comment to `wizardSystemPrompt` noting that `steps` becomes part of the agent's runtime prompt and pointing at the operator-preview gate as the trust boundary. No code change.

#### I-4 — `int64ToChunkID` comment is wrong about `strconv.FormatInt`
**File:** `internal/knowledge/graph.go:211-234`

The comment claims "strconv.FormatInt is heavy; for IDs < 2^31 this is sufficient and avoids an extra import". `strconv.FormatInt` is neither heavy nor allocates more than the hand-rolled version; the "avoids an import" justification is also moot since `strconv` is already imported elsewhere in the package's transitive deps (and the file imports `time`, `sort`, etc.). The hand-rolled code is correct but the justification is inaccurate. Style/DOC only.

**Recommended remediation:** Replace with `return strconv.FormatInt(id, 10)` and drop the helper. (Or keep the helper and fix the comment to "kept local to keep graph.go dependency-light" — but it already imports `time`/`sort` so even that is weak.) Low priority.

#### I-5 — Unchecked `row.tier as Tier` cast in CounterEvidenceCard
**File:** `web/src/components/verdict/CounterEvidenceCard.tsx:86`

```tsx
tier: row.tier as Tier,
```

`ConflictChunk.tier` is typed `string` on the wire; the cast to `Tier` is unchecked. The BE only emits "历史" today (`emitCounterEvidence` queries the 历史 tier only, pinned by `TestCounterEvidence_PassesHistoryTierOnly`), so this is safe in production. But if a future cross-tier expansion lands (the `ConflictChunk.Tier` godoc at `internal/wire/chunk.go:91` explicitly anticipates "a future cross-tier expansion (e.g. 草稿)"), an unexpected value would silently propagate into the store and could surprise `ChunkSourceView` / `TierBadge`.

**Recommended remediation:** Validate at the dispatcher:
```ts
const tier = (['权威','历史','个人','草稿'] as const).includes(row.tier as Tier)
  ? (row.tier as Tier)
  : '历史';
```

## Areas verified clean (no findings)

- **`emitCounterEvidence` error handling** — best-effort; `pipeline.go:266-271` logs `[WARN]` and returns nil. Verdict already emitted in step 10; SSE is append-only. **Clean.**
- **`emitRedline` LLM error / invalid JSON paths** — both non-fatal; no chunk emitted; verdict + counter-evidence stand. Tests cover. **Clean.**
- **`snippetOf` rune/byte length handling** — correctly handles multi-byte CJK; the byte-length short-circuit followed by rune-length check is correct. **Clean.**
- **`parseWizardManifest` markdown-fence stripping** — defensive strip of ` ```json ... ``` ` + brace-delimiter check; never injects the raw LLM output anywhere unsafe. **Clean.**
- **`yamlQuoteIfNeeded`** — escapes `\\`, `"`, `\n` and quotes strings containing `:`, `#`, `"`, leading `-`, `*`, `&`, `!`. The generated SKILL.md round-trips through `Validate`. **Clean** (no YAML injection of attacker-controlled structure into the manifest's own fields).
- **`CounterEvidenceCard` disclaimer** — hardcoded JSX literal, no prop can override. T-5-27 satisfied. **Clean.**
- **`useWizard` busy-guard** — `requestPreview` and `commit` both gate on `if (busy) return` and reset in `finally`. **Clean.**
- **`safeStringify` in `InspectorToolCall`** — catches circular-reference exceptions, renders via `<code>{string}</code>` text node. **Clean.**
- **`isChunkActive` / `FilteredOutNotice` / `VersionBadge`** — defensive null/0 guards; correct epoch-seconds vs epoch-ms comparison. **Clean.**

## Files inspected

Backend (Go):
- `internal/skill/wizard.go` (459 lines)
- `internal/skill/validate.go`
- `internal/server/wizard_name.go`
- `internal/server/skills.go`
- `internal/server/annotate.go` + `annotate_test.go`
- `internal/server/redline.go` + `redline_test.go`
- `internal/server/pipeline.go`
- `internal/server/knowledge.go`
- `internal/server/server.go`
- `internal/knowledge/graph.go`
- `internal/wire/chunk.go`
- `internal/wire/redline.go`

Frontend (TS/TSX):
- `web/src/components/verdict/CounterEvidenceCard.tsx`
- `web/src/components/verdict/RedlineCard.tsx`
- `web/src/components/inspector/AgentInspector.tsx`
- `web/src/components/inspector/InspectorToolCall.tsx`
- `web/src/components/chat/MessageBubble.tsx`
- `web/src/components/center/ChunkSourceView.tsx`
- `web/src/lib/clipboard.ts`
- `web/src/hooks/useWizard.ts`
- `web/src/types/knowledge.ts`

Plus targeted greps across `web/src` (dangerouslySetInnerHTML) and `internal/` (auth, overwrite checks).

## Summary

The Phase 5 trust architecture is sound. The two load-bearing moats (D5-13 counter-evidence grounding isolation, D5-14 redline cost control) are enforced at the signature / first-line level — not merely by tests — and survive direct code inspection. The three Warning findings are deployment-hardening items appropriate to address before the Apache-2.0 open-source release (which CLAUDE.md flags as the project's purpose) rather than correctness defects. The Info findings are documentation/cosmetic polish.
