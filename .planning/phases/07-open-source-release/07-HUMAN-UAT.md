---
status: partial
phase: 07-open-source-release
source: [07-VERIFICATION.md]
started: 2026-08-13
updated: 2026-08-13
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  gap_snapshot: "partial::scenarios=5"
---

## Current Test

[awaiting human testing — 5 items; autonomous artifacts committed]

## Tests

### 1. DIST-04 verify — Docker compose config + both image builds

expected: `COMPOSE_OK` printed; both `docker build -f Dockerfile.{backend,frontend}` exit 0; no `:latest` pulled; Milvus tag `v2.4.17` resolves (any v2.4.x patch satisfies SDK v2.4.2). Bonus: `docker compose up` → `http://localhost:8080` → ask "出差住宿费超标 200 元，能批吗？" → expect cited verdict.
result: [pending]

Commands (repo root):

```bash
docker compose config >/dev/null && echo COMPOSE_OK
docker build -f Dockerfile.backend -t sf-be .
docker build -f Dockerfile.frontend -t sf-fe .

# optional end-to-end:

docker compose up   # then open http://localhost:8080
```

### 2. DIST-09 publication — live-stack eval run + README snapshot

expected: `eval/report.json` exists with 50 cases; README `## Eval Results` has a real date + real faithfulness/context_precision/refusal_accuracy numbers (no `_TBD_`). Every number traces to `report.json`; refusal_accuracy traces to the derived `Cases` ratio.
result: [pending]

Commands (after item 1 stack is up + `ollama pull bge-m3` + `.env` filled with `GLM_API_KEY` + `BGE_M3_URL=http://host.docker.internal:11434/v1/embeddings`):

```bash
go run ./cmd/eval --frozen=compliance
test -f eval/report.json && python -c "import json;r=json.load(open('eval/report.json'));assert len(r['cases'])==50;print('report OK')"
go run ./scripts/eval_snapshot   # paste stdout into README ## Eval Results, replacing the _TBD_ row
```

NOTE: if a live stack is unavailable, leave the placeholder intact — do NOT fabricate numbers (T-7-OSS-REP).

### 3. DIST-05 — record docs/demo.gif

expected: `docs/demo.gif` exists (<5MB); GIF shows BOTH a cited verdict card AND a structured refusal card (D7-05/06 brand coverage); `grep -c 'docs/demo.gif' README.md` = 1.
result: [pending]

Steps (stack from item 2 running at `http://localhost:8080`): pick **compliance** Skill → start screen recorder (ScreenToGif/OBS/macOS) → ask cited-verdict question → ask out-of-scope question (structured refusal) → stop, export GIF ≤60s, optimize <5MB (`gifsicle --optimize=3 --colors 128` or `ffmpeg`) → save `docs/demo.gif`.

### 4. Plan 07-06 T1-2 — git filter-repo single-pass rewrite (FRESH clone)

expected: post-rewrite all 4 audit gates pass with ZERO matches; filter-repo reports a full rewrite.
result: [pending]

On a FRESH clone (`git clone <current-repo> skillforge-release && cd skillforge-release`), create `scrub.txt` (4 literal lines, order matters — FQDNs first):

```
10.62.10.34==>127.0.0.1
milvus.internal.company.com==>milvus
bge-m3.internal.company.com==>localhost
bge-m3.internal==>localhost
```

Run + 4 audit gates:

```bash
git filter-repo --invert-paths --path .planning/ --replace-text scrub.txt

# Gate 1 — internal strings zero-match across ALL history:

git grep -nE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' $(git rev-list --all)   # → ZERO

# Gate 2 — .planning/ fully stripped:

git log --oneline --all -- .planning/ | wc -l   # → 0

# Gate 3 — no real secrets (only GLM_API_KEY=unhappy test sentinel allowed):

git grep -nE 'AKIA[0-9A-Z]{16}|sk-[a-zA-Z0-9]{20,}' $(git rev-list --all)   # → ZERO

# Gate 4 — HEAD source files clean:

grep -rcE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' .env.example cmd/server/main.go internal/retriever/mongo/mongo.go internal/embedder/bge/bge.go scripts/live-verify.sh   # → 0 each
```

### 5. Plan 07-06 T3 — public `skillforge` remote + force-push + clean-clone verify

expected: public repo live at `github.com/<username>/skillforge`; clean clone has zero internal info + no `.planning/` + all release artifacts; `go test ./internal/skill -run TestExamplesValidate` green on public clone.
result: [pending]

Steps: (a) confirm GitHub username with user (CONTEXT flagged `ninedrunk` 待确认); (b) create EMPTY public repo `github.com/<username>/skillforge` (do NOT init with README/LICENSE — force-push supplies everything); (c) in the skillforge-release clone:

```bash
git remote add origin git@github.com:<username>/skillforge.git
git push -u --force origin master

# then in a NEW empty dir, clean-clone verify:

git clone git@github.com:<username>/skillforge.git skillforge-verify && cd skillforge-verify
git grep -nE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' $(git rev-list --all)   # → ZERO
ls .planning 2>/dev/null   # → no such directory
go test ./internal/skill -run TestExamplesValidate   # → PASS
```

## Summary

total: 5
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps

None — all autonomous artifacts committed + structurally verified (VERIFICATION.md: 15/20 truths verified, 0 blockers). These 5 items are the explicit `autonomous:false` publication/environment tasks deliberately deferred.

**Recommended sequence:** items 1→2→3 (boot → eval snapshot → GIF) update dev HEAD, then 4→5 (history rewrite → public push) consume that HEAD for publication.
