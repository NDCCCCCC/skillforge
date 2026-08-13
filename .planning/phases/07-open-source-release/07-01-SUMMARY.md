---
phase: 07-open-source-release
plan: 01
subsystem: release-meta
tags: [license, dockerignore, gitignore, env, dist-docs, apache-2.0]
requires:
  - "D7-02 (external embedding decision)"
  - "D7-03 (SC-1 pluggable embedding endpoint revision)"
  - "D7-06 (SC-5 recorded demo GIF revision)"
  - "cmd/server/main.go envRequired() fail-fast path (Phase 2)"
provides:
  - "LICENSE — Apache-2.0 full text (DIST-02 closed)"
  - ".dockerignore — Docker build-context boundary (T-7-OSS-CONTAINER mitigated at HEAD)"
  - ".gitignore skillforge.db* — local SQLite audit store never committed (T-7-OSS-SECRET mitigated at HEAD)"
  - ".env.example — compose-friendly defaults + fail-fast contract doc (T-7-OSS-ENV mitigated at HEAD; DIST-03 readiness)"
  - "ROADMAP SC-1/SC-5 + REQUIREMENTS DIST-04/DIST-05 wording aligned with locked decisions"
affects:
  - "Plan 07-02 (containerization) — .env.example defaults + .dockerignore boundary are its inputs"
  - "Plan 07-03 (README) — references .env + compose quick-start; cites LICENSE"
  - "Plan 07-06 (publication governance) — .env.example history scrub via filter-repo (T-7-OSS-INFO still open until history rewrite)"
tech-stack:
  added: []
  patterns:
    - "SPDX canonical LICENSE (no hand-rolled text)"
    - "Two-layer fail-fast: envRequired() in Go + ${VAR:?} in compose"
    - "Docker build-context boundary = backend source + skills/ + knowledge/ + web/ only"
key-files:
  created:
    - LICENSE
    - .dockerignore
  modified:
    - .gitignore
    - .env.example
    - .planning/ROADMAP.md
    - .planning/REQUIREMENTS.md
decisions:
  - "D7-02 honored: .env.example points BGE_M3_URL at user-supplied endpoint (localhost:11434 ollama default), no embedding service baked into compose"
  - "D7-03 honored: ROADMAP SC-1 + REQUIREMENTS DIST-04 now say 'pluggable embedding endpoint' instead of 'BGE-M3'"
  - "D7-06 honored: ROADMAP SC-5 + REQUIREMENTS DIST-05 now say 'recorded demo GIF' instead of 'live demo site'"
  - "LICENSE only at repo root (no new per-file headers this task — cmd/*.go + internal/skill/*.go + eval/NOTICE already carry SPDX-License-Identifier: Apache-2.0)"
metrics:
  duration: 3m
  completed: 2026-08-13
  tasks: 3
  commits: 3
  files_changed: 6
requirements_covered:
  - DIST-02
  - DIST-03
---

# Phase 7 Plan 01: Release-Meta Foundations Summary

**One-liner:** Laid the release-meta foundation — verbatim Apache-2.0 LICENSE, Docker build-context `.dockerignore` boundary, `skillforge.db*` ignore hygiene, internal-IP scrub of `.env.example` with a documented two-layer fail-fast contract, and the four SC/DIST wording revisions (D7-03/D7-06) locked into ROADMAP + REQUIREMENTS.

## What Was Built

### Task 1 — LICENSE + ignore hygiene (`2a76533`)

- **LICENSE** (new): verbatim Apache-2.0 full text fetched from the SPDX canonical source (`https://spdx.org/licenses/Apache-2.0.txt`). First line `Apache License`; body contains both `Version 2.0, January 2004` and `Apache License, Version 2.0`. Closes **DIST-02**. No new per-file Go headers added this task — `cmd/*.go`, `internal/skill/*.go`, and `eval/NOTICE` already carry `SPDX-License-Identifier: Apache-2.0` (confirmed in 07-RESEARCH.md §Code Examples Example 8).
- **.gitignore** (modified): appended `skillforge.db`, `skillforge.db-shm`, `skillforge.db-wal`. The local SQLite audit store was untracked-but-not-ignored; this closes the gap before the repo goes public (**T-7-OSS-SECRET**). Existing entries (`.env`, `*.exe`, `/spiked`, `.DS_Store`, `node_modules/`, `web/dist/`) preserved.
- **.dockerignore** (new): boundary = backend source + `skills/` + `knowledge/` + `web/` only. Excludes `.planning/`, `.git/`, `.gitignore`, `.claude/`, `.env`, `.env.local`, `skillforge.db*`, `*.exe`, `eval.exe`, `spike-*trace.json`, `web/dist/`, `web/node_modules/`, `node_modules/`, `docs/demo*.gif`, `Dockerfile*`, `docker-compose*.yml`, `.DS_Store` (**T-7-OSS-CONTAINER**). Verified `cmd/`, `internal/`, `skills/`, `knowledge/`, `web/src/`, `web/package.json`, `go.mod`, `go.sum` are NOT excluded.

### Task 2 — ROADMAP + REQUIREMENTS SC revisions (`418800b`)

Exactly four targeted edits (Edit tool, not rewrites) per **D7-03** + **D7-06**:

| File | Before | After |
|------|--------|-------|
| ROADMAP SC-1 | `backend, frontend, Milvus, BGE-M3 (DIST-04)` | `backend, frontend, Milvus, pluggable embedding endpoint (DIST-04)` |
| ROADMAP SC-5 | `A live demo site on free-tier hosting responds to a sample compliance question and returns a cited verdict (DIST-05)` | `A recorded demo GIF in the README hero walks through a sample compliance question and returns a cited verdict + structured refusal (DIST-05)` |
| REQUIREMENTS DIST-04 | `(Go backend + Milvus + BGE-M3 + frontend)` | `(Go backend + Milvus + frontend + pluggable embedding endpoint)` |
| REQUIREMENTS DIST-05 | `Live demo site deployed (HuggingFace Spaces / fly.io / free tier) with sample compliance docs` | `Recorded demo GIF in README hero (cited verdict + structured refusal flow) — D7-06 reversed the live-demo decision` |

No other SC or DIST text touched. `git diff --stat` confirmed only the two files changed.

### Task 3 — .env.example scrub + fail-fast contract (`e4e34ef`)

Rewrote `.env.example` per 07-RESEARCH.md Example 5:

- **Scrubbed every internal IP/domain** (**T-7-OSS-ENV**):
  - `milvus.internal.company.com:19530` → `milvus:19530` (compose service name)
  - `http://bge-m3.internal.company.com/v1/embeddings` → `http://localhost:11434/v1/embeddings` (ollama default)
  - `mongodb://10.62.10.34:27017` (MONGO_URI comment example) → `mongodb://localhost:27017`
- **Documented the two-layer fail-fast contract** so a stranger knows missing `GLM_API_KEY` / `BGE_M3_URL` aborts startup:
  - Layer 1: `envRequired()` in `cmd/server/main.go` (`log.Fatalf("[FAIL] cmd/server: <VAR> env var not set ...")` + non-zero exit) — already exists, unchanged.
  - Layer 2: `docker-compose.yml` `${VAR:?}` interpolation (Plan 07-02 will consume this).
- **Dropped the stale spike-era header** `# Spike uses INTERNAL services — no Docker.`
- **Added the one-liner** `ollama pull bge-m3` (D7-02 external-embedding minimum bar).
- **Preserved all 10 cmd/server vars** (`RETRIEVER_BACKEND`, `MILVUS_ADDRESS`, `MILVUS_TOKEN`, `MONGO_URI`, `BGE_M3_URL`, `BGE_M3_MODEL`, `BGE_M3_AUTH`, `GLM_API_KEY`, `GLM_BASE_URL`, `SQLITE_DSN`) — none added or removed. `GLM_API_KEY=` and `BGE_M3_AUTH=` stay blank (no real secret introduced).

## Deviations from Plan

None — plan executed exactly as written. One minor adjustment during Task 1: the initial `.dockerignore` included explicit `spiked` / `cmd/spiked/spiked` binary entries (mirroring `.gitignore`); these were trimmed because (a) the plan's behavior list did not include them, (b) `*.exe` already covers the `.exe` variants, and (c) the Task 1 acceptance criterion `grep -E '^cmd/' .dockerignore` must return nothing. The trimmed version matches the plan's exclusion list verbatim. No functional difference — the spike binaries are host-built artifacts not needed in the image.

## Threat Model — Disposition Status

| Threat ID | Component | Plan Disposition | HEAD Status after 07-01 |
|-----------|-----------|------------------|-------------------------|
| T-7-OSS-ENV | .env.example | mitigate | **Mitigated at HEAD** (zero internal IP/domain matches; `${VAR:?}` contract documented). History scrub remains Plan 07-06. |
| T-7-OSS-CONTAINER | .dockerignore boundary | mitigate | **Mitigated** (.planning/, .git/, .claude/, *.exe, node_modules/, web/dist/, .env, skillforge.db* all excluded) |
| T-7-OSS-SECRET | .gitignore (skillforge.db*) | mitigate | **Mitigated** (3 lines added: skillforge.db, .db-shm, .db-wal) |
| T-7-OSS-INFO | .env.example history | accept (this task) / mitigate (Plan 06) | **Accepted for 07-01** — HEAD scrub done; full-history `git filter-repo --replace-text` scrub is Plan 07-06's gate. `.env.example` flagged as a known hotspot for the release gate. |

## Threat Flags

None. All file changes this plan either introduce no new trust boundary (LICENSE / ROADMAP / REQUIREMENTS are docs) or *tighten* an existing boundary (.dockerignore, .gitignore, .env.example scrub). No new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries.

## Known Stubs

None. This plan ships no runtime code — all artifacts are declarative files (license / ignore lists / env template) and doc edits. Nothing flows to UI rendering; no placeholder data.

## Open Items Handed to Downstream Plans

- **Plan 07-02** (containerization): consumes the `.env.example` defaults (`milvus:19530`, `localhost:11434`) as compose env-stanza interpolation targets, and inherits the `.dockerignore` boundary as its build-context contract.
- **Plan 07-03** (README): references `.env` + compose in quick-start; cites LICENSE.
- **Plan 07-06** (publication governance): owns the **T-7-OSS-INFO** mitigation — `git filter-repo --replace-text` must scrub the full git history of the internal strings this plan removed from HEAD. The post-rewrite audit gate (`git grep -nE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' $(git rev-list --all)` → zero matches) is the release gate that closes T-7-OSS-INFO.

## Self-Check: PASSED

**Files exist:** LICENSE, .dockerignore, .gitignore, .env.example, .planning/ROADMAP.md, .planning/REQUIREMENTS.md, .planning/phases/07-open-source-release/07-01-SUMMARY.md — all FOUND.

**Commits exist:** 2a76533 (Task 1), 418800b (Task 2), e4e34ef (Task 3) — all FOUND in `git log --all`.

**Plan verification gates (all pass):**
- `grep -c 'Apache License' LICENSE` = 4 (≥1) ✓
- `grep -cE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' .env.example` = 0 ✓
- `grep -c 'pluggable embedding endpoint' .planning/ROADMAP.md` = 1 (≥1) ✓
- `grep -c 'pluggable embedding endpoint' .planning/REQUIREMENTS.md` = 1 (≥1) ✓
- `grep -c 'skillforge.db' .gitignore` = 3 (≥1) ✓
- `grep -c '.planning' .dockerignore` = 1 (≥1) ✓

**Deletion check:** zero file deletions across all 3 task commits.
