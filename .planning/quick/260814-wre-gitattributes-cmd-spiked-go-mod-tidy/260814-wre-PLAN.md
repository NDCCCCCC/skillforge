---
phase: quick-260814-wre
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - .gitattributes
  - .gitignore
  - go.mod
  - go.sum
  - integration/contract/contract_test.go
  - scripts/spiked/*  # (moved from cmd/spiked/* via git mv)
  - internal/**  # comment-only path reference updates
  - eval/golden/README.md
autonomous: true
requirements: []
must_haves:
  truths:
    - "gofmt -l reports ZERO files across cmd/ internal/ integration/ scripts/ (CRLF blight eliminated)"
    - "git diff of the renormalize commit shows EOL-only changes (no content bytes changed)"
    - "cmd/ contains only eval/ and server/ (spiked gone); scripts/spiked/ compiles and its tests pass"
    - "go build ./... && go test ./cmd/... ./internal/... ./integration/... all green"
    - "Root debug artifacts (body_gbk_*, body_quantum.json, eval.exe, eval_snapshot.exe, skillforge.db*) are deleted and future body_gbk_* / body_quantum.json are gitignored"
    - "go mod tidy run; fsnotify sits in the DIRECT require block of go.mod"
    - "No stale 'cmd/spiked' references in tracked .go/.md outside .planning/ and .claude/"
  artifacts:
    - path: ".gitattributes"
      provides: "EOL policy (*.go text eol=lf etc.)"
      min_lines: 6
    - path: "scripts/spiked/main.go"
      provides: "spike regression binary at new in-module location"
    - path: "go.mod"
      provides: "tidied dependency graph with fsnotify as direct dep"
  key_links:
    - from: "integration/contract/contract_test.go"
      to: "github.com/skillforge/skillforge/scripts/spiked"
      via: "exec go build -o in buildSpiked"
      pattern: "skillforge/scripts/spiked"
---

<objective>
Backend repo hygiene, zero runtime-logic changes: (1) add .gitattributes and renormalize so Windows autocrlf CRLF blight on ~100 tracked .go files is fixed in one pure-EOL commit; (2) move cmd/spiked spike scaffolding out of the production cmd/ tree to scripts/spiked/ (same module, imports internal/ intact), update all references; (3) delete root debug artifacts and close .gitignore gaps; (4) go mod tidy to normalize fsnotify as a direct dependency.

Purpose: Phase 7 open-source release polish — a stranger forking the repo on any OS must get `gofmt -l` clean and a green build; the production cmd/ dir must contain only real entrypoints (eval, server).
Output: 3 atomic commits (EOL-only / content / go.mod tidy), all tests green.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.gitignore
@go.mod

Key repo facts (orchestrator-verified):
- Repo D:\CODE\ClaudeCode\ai-agent, module github.com/skillforge/skillforge, go 1.25.0. `core.autocrlf=true` → all ~100 tracked .go files have CRLF working-tree endings; `gofmt -l` flags every file. No .gitattributes exists.
- cmd/spiked: 10 .go files (~2.1k LOC), package main, imports internal/ packages → CANNOT become a nested module (.planning/spikes/* each have their own go.mod; that route would require a real refactor — rejected).
- Destination decision (D-quick-1): `scripts/spiked/` — precedent is `scripts/eval_snapshot` (main package inside the main module, runnable via `go run ./scripts/eval_snapshot`). Stays in module → internal/ imports keep working, `git mv` preserves history.
- `integration/contract/contract_test.go:498` builds spiked by full import path `github.com/skillforge/skillforge/cmd/spiked` (buildSpiked, cached via sync.Once; suite runs offline with GLM_API_KEY=unhappy). This line MUST be updated to the new path or the contract suite fails.
- Root clutter (all untracked, safe to delete): body_gbk_full.json, body_gbk_only.bin, body_gbk_prefix.json, body_quantum.json (NOT covered by .gitignore → show as ?? in git status), eval.exe, eval_snapshot.exe (covered by `*.exe`), skillforge.db / -shm / -wal (covered).
- go.mod: fsnotify v1.5.4 sits in the second require block WITHOUT `// indirect` — it IS direct (imported by internal/skill/registry.go hot-reload). `go mod tidy` will move it into the first require block.
- Mongo retriever is LIVE code (RETRIEVER_BACKEND fallback) — do not touch.
- HARD CONSTRAINT: 不改任何运行时逻辑。`go build ./... && go test ./cmd/... ./internal/...` must pass; EOL renormalization must NEVER be mixed with content edits in one commit.
</context>

<tasks>

<task type="auto">
  <name>Task 1: .gitattributes + pure-EOL renormalization commit</name>
  <files>.gitattributes</files>
  <action>
Do this FIRST so all subsequent content edits happen on LF-normalized files (clean diffs, gofmt usable as a gate).
1. Create .gitattributes with: `* text=auto` as the base rule, then explicit `text eol=lf` rules for *.go, *.mod, *.sum, *.md, *.json, *.yml, *.yaml, *.ts, *.tsx, *.css, *.js, *.sh, *.sql, Dockerfile* and *.dockerignore, and `*.png binary` / `*.gif binary` / `*.db binary` binary rules. (Repo is Go+web; no .bat files exist, so no eol=crlf rules needed.)
2. Stage renormalization: run `git add --renormalize .` then `git add .gitattributes`.
3. GATE — the staged diff must be EOL-only: `git diff --cached --ignore-cr-at-eol -- . ':(exclude).gitattributes'` must produce EMPTY output (proves zero content-byte changes). If it is non-empty, STOP and investigate before committing.
4. Commit alone, message: `chore: add .gitattributes and renormalize line endings (EOL-only)`.
5. Refresh the working tree so files materialize as LF per the new attributes: `git ls-files -z | xargs -0 rm` followed by `git checkout -- .` (safe — no uncommitted changes remain). Run from repo root.
6. Confirm `gofmt -l ./cmd ./internal ./integration ./scripts` outputs nothing (previously flagged ~100 files).
7. Confirm green: `go build ./... && go test ./cmd/... ./internal/...`.
</action>
<verify>gofmt -l ./cmd ./internal ./integration ./scripts is empty AND go build ./... && go test ./cmd/... ./internal/... exits 0 AND git show --stat HEAD shows only line-ending churn + .gitattributes</verify>
<done>.gitattributes committed; HEAD diff is pure EOL; gofmt clean; build+tests green</done>
</task>

<task type="auto">
  <name>Task 2: migrate cmd/spiked → scripts/spiked, update references, purge root artifacts, close .gitignore gaps</name>
  <files>cmd/spiked/*, scripts/spiked/*, integration/contract/contract_test.go, .gitignore, internal/wire/chunk.go, internal/retriever/milvus/config.go, internal/embedder/bge/bge.go, internal/knowledge/indexer.go, internal/server/chat_test.go, internal/server/chat.go, internal/server/pipeline.go, internal/eval/golden_test.go, internal/eval/dataset.go, internal/eval/chatmodel.go, internal/verdict/verdict_test.go, internal/skill/loader.go, internal/skill/loader_test.go, eval/golden/README.md</files>
  <action>
All changes in ONE content commit (working tree is LF-normalized after Task 1, so this diff stays content-only).
1. Move: `git mv cmd/spiked scripts/spiked`. Package name stays `main`; imports of internal/ are unchanged (same module). Per D-quick-1.
2. Update the functional build path: in integration/contract/contract_test.go line ~498 change the exec import path from `github.com/skillforge/skillforge/cmd/spiked` to `github.com/skillforge/skillforge/scripts/spiked`.
3. Update .gitignore: replace `/cmd/spiked/spiked` and `/cmd/spiked/spiked.exe` with `/scripts/spiked/spiked` and `/scripts/spiked/spiked.exe` (the stale top-level `/spiked` and `/spiked.exe` rules can stay — harmless). APPEND new rules: `/body_gbk_*` and `/body_quantum.json` (GBK/UTF-8 chat-body debug artifacts likely to reappear) with a one-line comment explaining their origin.
4. Update textual references: in tracked *.go and eval/golden/README.md, replace remaining `cmd/spiked` occurrences with `scripts/spiked` — these are comments/doc-prose only (e.g. internal/server/chat_test.go:57,85,580-581; internal/wire/chunk.go:53; internal/retriever/milvus/config.go:23,39; internal/embedder/bge/bge.go:23,195; internal/knowledge/indexer.go:108; scaffold_stubs.go self-references; milvus_test.go run-command comment). EXCLUDE .planning/ and .claude/ (historical records must not be rewritten) and body_* files (deleted in step 6). Zero code-logic edits — comment text and string literals inside moved files only.
5. Delete root debug artifacts (verify each is untracked first with `git status --porcelain` — must be `??` or absent): body_gbk_full.json, body_gbk_only.bin, body_gbk_prefix.json, body_quantum.json, eval.exe, eval_snapshot.exe, skillforge.db, skillforge.db-shm, skillforge.db-wal. NEVER touch tracked files.
6. Verify then commit: `go build ./... && go vet ./cmd/... ./internal/... ./integration/... ./scripts/... && go test ./cmd/... ./internal/... ./integration/...` (contract suite runs offline via GLM_API_KEY=unhappy; milvus integration tests in scripts/spiked skip without MILVUS_INTEGRATION=1). Commit message: `refactor: move cmd/spiked spike scaffolding to scripts/spiked, purge debug artifacts, close gitignore gaps`.
</action>
<verify>go build ./... && go vet ./cmd/... ./internal/... ./integration/... ./scripts/... && go test ./cmd/... ./internal/... ./integration/... all exit 0; grep -rn "cmd/spiked" --include="*.go" --include="*.md" . excluding .planning/ .claude/ returns 0 matches; git status --porcelain shows no body_* / *.exe / skillforge.db* files</verify>
<done>cmd/ contains only eval/ and server/; scripts/spiked compiles and contract suite passes at the new path; root clutter gone; .gitignore covers body_gbk_* and body_quantum.json; no stale references in tracked source</done>
</task>

<task type="auto">
  <name>Task 3: go mod tidy (fsnotify → direct require block)</name>
  <files>go.mod, go.sum</files>
  <action>
Run `go mod tidy` at repo root. Expected result: github.com/fsnotify/fsnotify v1.5.4 moves from the second require block into the FIRST (direct) require block, reflecting its real direct use by internal/skill/registry.go; no version bumps or removals of deps actually used (milvus-sdk-go, mongo-driver etc. must remain untouched). If tidy proposes removing a dependency that IS imported (would indicate an unused dep — not expected), STOP and report rather than force it. Then run `go build ./... && go test ./cmd/... ./internal/...` as the green gate. Commit message: `chore: go mod tidy (fsnotify promoted to direct require block)`. This commit must contain ONLY go.mod/go.sum changes.
</action>
<verify>go mod tidy && git diff --stat shows only go.mod/go.sum; go.mod first require block contains fsnotify; go build ./... && go test ./cmd/... ./internal/... exits 0</verify>
<done>go.mod normalized, fsnotify in direct require block, build+tests green, single tidy-only commit</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| working tree → git index | renormalization + .gitattributes control how bytes cross into history; a bad rule could corrupt binary files |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-q-01 | Tampering | .gitattributes renormalize | mitigate | `* text=auto` + explicit binary rules for png/gif/db; gate the staged diff with `--ignore-cr-at-eol` emptiness check before committing |
| T-q-02 | DoS/rep | go mod tidy | mitigate | inspect tidy diff — only go.mod/go.sum, no unexpected version changes or removals of live deps (mongo-driver, milvus-sdk-go) |
| T-q-SC | Tampering | package installs | accept | no new packages installed in this task; pure reorganization |
</threat_model>

<verification>
1. `gofmt -l ./cmd ./internal ./integration ./scripts` → empty
2. `go build ./... && go vet ./cmd/... ./internal/... ./integration/... ./scripts/...` → exit 0
3. `go test ./cmd/... ./internal/... ./integration/...` → all PASS (integration contract suite runs offline)
4. `git log --oneline -3` → 3 commits: EOL-only / spiked-move+cleanup / go-mod-tidy, in that order
5. `git show --ignore-cr-at-eol --stat HEAD~2` content-empty except .gitattributes (pure EOL proof)
6. `ls cmd/` → eval, server only
7. `grep -rn "cmd/spiked"` in tracked source outside .planning/.claude → 0 hits
8. Working tree clean: no body_gbk_*, body_quantum.json, *.exe, skillforge.db* at root
</verification>

<success_criteria>
- 3 atomic commits in order: EOL renormalization (pure), content (spiked move + refs + gitignore + artifact purge), go mod tidy
- Zero runtime-logic changes; all tests green (`go build ./... && go test ./cmd/... ./internal/... ./integration/...`)
- gofmt -l clean on Windows checkout; repo presentable for open-source release
</success_criteria>

<output>
Create `.planning/quick/260814-wre-gitattributes-cmd-spiked-go-mod-tidy/260814-wre-SUMMARY.md` when done
</output>
