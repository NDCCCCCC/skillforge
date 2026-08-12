---
status: resolved
trigger: "OOS-1 (BLOCKER) — Real-backend POST /api/chat answer-path verdict parse fails: GLM wraps verdict JSON in ```json ... ``` fences; stripJSONDelim does not fully strip the trailing fence → json.Unmarshal hits a backtick 'after top-level value'. The compliant/non-compliant ANSWER/verdict path is broken end-to-end; the refusal path works."
severity: blocker
phase_origin: discovered-during-phase-05-UAT (verdict pipeline is Phase 2-era; resolution_route = /gsd-debug, NOT Phase 5 gap closure)
uat_finding: OOS-1
created: 2026-08-13
updated: 2026-08-13
tdd_mode: false
goal: find_and_fix
---

# Debug Session — glm-verdict-fence-parse (OOS-1) — RESOLVED

## Symptoms

- **Expected:** For a compliant/non-compliant query, the agent emits a verdict JSON that parses cleanly, then streams `text-delta` + `source-document`. (Refusal path already works.)
- **Actual:** Real-backend `POST /api/chat {"query":"员工年假有多少天","skill":"企业合规/制度审查"}` → agent runs retrieve→answer→forced_stop then emits `type:error`. No `text-delta`/`source-document`. ANSWER path broken.
- **Error messages:** `agent run: parse verdict: invalid character '`' after top-level value (raw: ```json\n{\n  "...)"`.
- **Reproduction:** `curl -N -X POST localhost:8888/api/chat ... -d '{"query":"员工年假有多少天","skill":"企业合规/制度审查"}'` → stream ends `type:error`. (Needs a live LLM provider.)

## Evidence

- 2026-08-13 #1 — `internal/agent/callbacks.go:196-219` (stripJSONDelim): byte-index impl. Leading-fence strip gated on `s[:3]=="```"` AFTER whitespace trim → ANY prose before the fence ("Here is the verdict:\n```json…") defeats leading detection. Lang-tag loop `for len(s)>0 && s[0]!='\n' { s=s[1:] }` consumes the WHOLE remaining string when no `\n` follows the tag (`` ```json{...}``` `` → `""`). Trailing-fence strip matches exactly 3 backticks → extra/misplaced fences leave a residual backtick → "invalid character '`' after top-level value". Fragile across multiple common LLM shapes, not one.
- 2026-08-13 #2 — `internal/server/pipeline.go:201-210` (second parse site): `raw, err := drainFinalContent(iter)` then `json.Unmarshal([]byte(raw), &v)` — NO stripJSONDelim call. ANY fenced verdict fails here regardless of the agent helper. (Phase 2-era linear pipeline parse; agent.go:515 is the Phase 3 ReAct path.)
- 2026-08-13 #3 — `internal/agent/critique.go:164-178` (third parse site): `json.Unmarshal([]byte(out.Content), &parsed)` — also NO stripping. Same defect class (latent — critique is best-effort additive strictness).
- 2026-08-13 #4 — grep across internal/: TWO separate stripJSONDelim implementations — agent/callbacks.go (byte-index, fragile) and eval/judge.go:362 (TrimPrefix/Suffix, also misses prose-wrapped). No shared helper. agent.go:515 + callbacks.go:160 use the agent version; eval/judge.go:135 the eval version; pipeline.go:208 + critique.go:175 use NONE.
- 2026-08-13 #5 — `internal/eval/faithfulness_test.go:147-158` (TestStripJSONDelim): 4 simple cases, all BASIC shapes the helpers happen to pass. No no-newline / prose-wrapped / CRLF / malformed coverage → false confidence.
- 2026-08-13 #6 (orchestrator) — independent verification of the applied fix: `go build ./...`=0, `go vet ./...`=0, `go test ./...`=all ok, `go test -race ./internal/{llm,agent,server}/`=clean. Confirmed all 4 production parse sites route through `llm.ExtractJSON` (agent.go:516, callbacks.go:162, critique.go:176, pipeline.go:209); `stripJSONDelim` removed from agent/server runtime (comments only). Reviewed the `ExtractJSON` algorithm by hand across GLM-shape / no-tag / no-newline / prose-wrapped / brace-in-string / nested / empty / reversed cases — all correct; degrades to clean `""`→`unexpected end of JSON input` on malformed input.

## Eliminated

- (hypothesis) "the second site shares the agent helper" — REFUTED: pipeline.go:208 had NO stripping at all (pre-diagnosis was right that it lacked stripping; the agent's investigation confirmed it does not share the helper).

## Resolution

- root_cause: TWO cooperating defects. (1) `callbacks.go` `stripJSONDelim` was fragile in multiple ways — unguarded lang-tag loop ate the whole input when no newline followed the tag; leading-fence detection required the fence at the absolute post-trim start (prose-wrapped output defeated it); trailing-fence match required exactly 3 backticks (extra/misplaced fences left a residual backtick). (2) `pipeline.go:208` (Phase 2 linear verdict parse) had NO fence stripping at all, and `critique.go:175` had the same no-stripping defect. Either defect alone broke the GLM answer path.
- fix: New shared helper `internal/llm.ExtractJSON` (new leaf package, zero deps). Algorithm: TrimSpace → strip leading `` ```[lang]\n `` (only consume the lang tag when a newline terminates it, so `` ```json{...}``` `` does not lose the object) → strip trailing `` ``` `` → fall back to first-`{`-to-last-`}` brace scan (exact for a single top-level object; isolates the body from residual fences/prose). Returns `""` on malformed input so `json.Unmarshal` surfaces a clean error. The fragile `stripJSONDelim` was removed; all 4 production parse sites (agent.go:516, callbacks.go:162, critique.go:176, pipeline.go:209) now route through it.
- verification: `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./...` all packages ok; `go test -race ./internal/llm/ ./internal/agent/ ./internal/server/` clean. New `internal/llm/extract_test.go`: 25-case table (basic GLM, no lang tag, no newlines, CRLF, trailing whitespace, prose-wrapped both sides, uppercase/js tags, nested objects, brace-in-string, empty/whitespace/only-fences/reversed-braces) + realistic full-GLM-verdict parse. New `agent_test.go` end-to-end regression drives real `agent.Run` → agent.go:516 with fenced + prose-wrapped verdicts via `multiTurnStubChatModel`; both parse cleanly (prose-wrapped would have FAILED on old code). Live GLM repro NOT run (no provider API key in this environment, per the preset verification constraint); the end-to-end agent.Run regression with the exact GLM byte shape is the faithful substitute.
- files_changed: internal/llm/extract.go (NEW), internal/llm/extract_test.go (NEW), internal/agent/callbacks.go (removed stripJSONDelim; tryParseConfidence → ExtractJSON), internal/agent/agent.go (verdict parse → ExtractJSON), internal/agent/agent_test.go (+2 OOS-1 regression tests), internal/agent/critique.go (critique parse → ExtractJSON), internal/server/pipeline.go (Phase 2 verdict parse → ExtractJSON).
- live_backend_confirmation: OPTIONAL — with a configured GLM provider, replay the curl repro and confirm the stream now emits text-delta/source-document for a compliant/non-compliant verdict instead of terminating type:error.

## Out of scope (flagged for follow-up, NOT touched)

- `internal/eval/judge.go:362` carries its own `stripJSONDelim` copy with the same prose-wrapped blind spot (and `eval/faithfulness_test.go` pins only basic shapes). NOT on the user-facing verdict path; candidate for future consolidation onto `llm.ExtractJSON` + expanded test coverage. A model that wraps eval-judge output in prose+fences would silently degrade eval metrics.
