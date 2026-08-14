---
phase: quick-260814-wre
plan: 01
subsystem: repo-hygiene
tags: [gitattributes, eol, gofmt, spiked, gitignore, go-mod-tidy]
requires: []
provides:
  - "LF-normalized working tree on any OS (gofmt -l clean)"
  - "scripts/spiked regression binary location (contract suite build path)"
  - ".gitignore coverage for body_gbk_* / body_quantum.json debug artifacts"
affects: []
tech-stack:
  added: []
  patterns:
    - "text eol=lf gitattributes policy for Go+web repo with core.autocrlf=true"
key-files:
  created:
    - .gitattributes
  modified:
    - .gitignore
    - go.mod
    - integration/contract/contract_test.go
    - internal/** (comment-only path references)
    - eval/golden/README.md
    - web/src/components/graph/KnowledgeGraphBrowser.tsx
  moved:
    - "cmd/spiked/* -> scripts/spiked/* (git mv, history preserved)"
decisions:
  - "D-quick-1 honored: scripts/spiked (same module, precedent scripts/eval_snapshot)"
metrics:
  duration: ~10m
  completed: 2026-08-14
---

# Phase quick-260814-wre Plan 01: Backend repo hygiene Summary

One-liner: .gitattributes LF policy + gofmt normalization (119 flagged files → 0), cmd/spiked → scripts/spiked migration with contract-suite path/signature repair, .gitignore artifact-gap closure, and fsnotify promoted to the direct require block — zero runtime-logic changes, all suites green.

## What Was Built

1. **EOL policy + pure-EOL commit (`4f4e631`)** — `.gitattributes` with `* text=auto`, explicit `text eol=lf` for *.go/*.mod/*.sum/*.md/*.json/*.yml/*.yaml/*.ts/*.tsx/*.css/*.js/*.sh/*.sql/Dockerfile*/*.dockerignore, binary rules for *.png/*.gif/*.db. `git add --renormalize .` was a no-op beyond the new file (the repository blobs were already LF — autocrlf only polluted the working tree), so the commit is trivially EOL-only. Working tree refreshed via delete-all + checkout so files materialize LF; gofmt CRLF hits dropped 119 → 55.

2. **gofmt normalization (`1f1edd6`, deviation — see below)** — the 55 remaining gofmt hits were NOT CRLF but genuine Go-toolchain formatting diffs (numbered-list doc-comment blank lines, const/map alignment). `gofmt -w` applied across cmd/internal/integration/scripts; comment/whitespace-only, zero runtime logic.

3. **spiked migration + cleanup (`c4b34a8`)** — `git mv cmd/spiked scripts/spiked` (100%-similarity renames preserved, same module so internal/ imports intact). Contract suite exec path updated to `github.com/skillforge/skillforge/scripts/spiked`. All comment/doc `cmd/spiked` references updated across internal/**, eval/golden/README.md, scripts/spiked self-references, and one user-facing string in `web/src/components/graph/KnowledgeGraphBrowser.tsx` (stale run command shown in UI). .gitignore: `/cmd/spiked/*` rules replaced with `/scripts/spiked/*`; appended `/body_gbk_*` + `/body_quantum.json` with an origin comment. Root debug artifacts (body_gbk_*, body_quantum.json, *.exe, skillforge.db*) are absent from this worktree (they exist only as untracked files in the main checkout; the new ignore rules cover them post-merge).

4. **go.mod normalization (`b1ded0f`)** — `go mod tidy` was already a no-op (fsnotify correctly required without `// indirect`); the fsnotify v1.5.4 line was moved into the first (direct) require block manually to satisfy the must-have, then re-verified: tidy still no-op, build+tests green, go.sum untouched.

## Commits

| # | Hash | Type | Subject |
|---|------|------|---------|
| 1 | 4f4e631 | chore | add .gitattributes and renormalize line endings (EOL-only) |
| 2 | 1f1edd6 | style | gofmt across cmd/internal/integration/scripts (deviation: extra commit) |
| 3 | c4b34a8 | refactor | move cmd/spiked spike scaffolding to scripts/spiked, purge debug artifacts, close gitignore gaps |
| 4 | b1ded0f | chore | go mod tidy (fsnotify promoted to direct require block) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] gofmt normalization required beyond EOL fix (extra `style` commit `1f1edd6`)**
- **Found during:** Task 1 step 6
- **Issue:** Plan assumed all 119 gofmt-flagged files were CRLF blight. After EOL normalization 55 files remained flagged — genuine Go 1.25 toolchain formatting diffs (doc-comment numbered-list blank lines, alignment). Hard gate demands `gofmt -l` empty.
- **Fix:** `gofmt -w ./cmd ./internal ./integration ./scripts` (55 files, 241+/237-; comment/whitespace-only). Committed separately as `style:` to keep the EOL commit pure and the content commit focused.
- **Files modified:** 55 .go files across cmd/ internal/ integration/ scripts/
- **Commit:** 1f1edd6

**2. [Rule 1 - Bug] Contract suite pre-existing compile break at BuildMilvusFilter call sites**
- **Found during:** Task 2 step 6 (contract suite run)
- **Issue:** Commit 92b908c (Plan 05-02 effective_to PRE-filter) changed `milvus.BuildMilvusFilter(tiers)` → `(tiers, now int64)` but never updated `integration/contract/contract_test.go` — the `-tags=contract` suite has failed to compile since 05-02 landed. Plan required this suite green.
- **Fix:** Test-only updates — 6 call sites given `now=0` (per filter.go godoc "Tests may pass 0 to assert expression shape"), equality/suffix assertions updated for the `and (effective_to == 0 or effective_to >= 0)` clause, doc comment refreshed. No production code touched.
- **Files modified:** integration/contract/contract_test.go
- **Commit:** c4b34a8

**3. [Rule 2 - Consistency] One .tsx stale reference updated beyond the plan's *.go/*.md scope**
- **Found during:** Task 2 step 4
- **Issue:** `web/src/components/graph/KnowledgeGraphBrowser.tsx:104` shows users a `cmd/spiked index` run command that no longer exists after the move.
- **Fix:** Path string updated to `scripts/spiked index` (UI text only, no logic).
- **Files modified:** web/src/components/graph/KnowledgeGraphBrowser.tsx
- **Commit:** c4b34a8

**4. [Rule 3 - Blocking] go mod tidy no-op → manual block placement**
- **Found during:** Task 3
- **Issue:** `go mod tidy` produced zero diff — fsnotify was already a correct direct require (no `// indirect`); tidy does not reorganize existing block placement. Must-have requires fsnotify in the direct (first) require block.
- **Fix:** Moved the `github.com/fsnotify/fsnotify v1.5.4` line into the first require block; re-ran tidy (still no-op — placement accepted), build+tests green, go.sum untouched, no version changes or dep removals (T-q-02 clean).
- **Files modified:** go.mod
- **Commit:** b1ded0f

### Plan-exact items

- Root artifact deletion was a no-op in this worktree (files absent — plan allowed "??" or absent). The main checkout's untracked copies are covered by the new .gitignore rules once this branch merges.

## Verification

1. `gofmt -l ./cmd ./internal ./integration ./scripts` → empty (was 119) — PASS
2. `go build ./...` + `go vet ./cmd/... ./internal/... ./scripts/...` → exit 0 — PASS
3. `go test ./cmd/... ./internal/...` → 12 packages ok; `GLM_API_KEY=unhappy go test -tags=contract ./integration/...` → ok (offline) — PASS
4. `git log` shows EOL → style → content → tidy order (4 commits; 3 planned + 1 deviation) — PASS w/ deviation
5. `git show --ignore-cr-at-eol --stat` on EOL commit → only .gitattributes (24 insertions) — PASS
6. `ls cmd/` → eval, server only — PASS
7. `grep -rn "cmd/spiked"` in tracked *.go/*.md outside .planning/.claude → 0 hits — PASS
8. Working tree clean; no body_gbk_* / body_quantum.json / *.exe / skillforge.db* — PASS

## Self-Check: PASSED

- Commits verified on branch worktree-agent-a1267184e6610fefc: 4f4e631, 1f1edd6, c4b34a8, b1ded0f
- .gitattributes, scripts/spiked/main.go exist; go.mod first require block contains fsnotify
