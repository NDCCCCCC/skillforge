---
phase: 07-open-source-release
plan: 02
subsystem: containerization
tags: [docker, compose, dockerfile, milvus, nginx, sse, multi-stage-build, dist-04]
requires:
  - "07-01-SUMMARY.md (.env.example scrubbed defaults + .dockerignore build-context boundary)"
  - "go.mod (go 1.25.0, milvus-sdk-go/v2 v2.4.2, modernc.org/sqlite v1.56.0 CGO-free)"
  - "web/package.json (ai ^7.0.59, vite ^5.4.0, react ^19.0.0, @types/node ^22)"
  - "D7-01 (Milvus standalone full topology), D7-02 (external embedding), D7-03 (SC-1 pluggable embedding), D7-04 (source build)"
provides:
  - "docker-compose.yml — one-command full-stack topology (etcd+minio+milvus+backend+frontend), Milvus v2.4.17 pinned, two-layer fail-fast env, healthcheck-gated depends_on"
  - "Dockerfile.backend — golang:1.25-alpine multi-stage build, alpine:3.20 runtime, ships skills/+knowledge/, CGO-free static binary"
  - "Dockerfile.frontend — node:22-alpine multi-stage build, nginx:1.27-alpine runtime, /api proxy with proxy_buffering off (SSE-friendly), SPA fallback"
affects:
  - "Plan 07-03 (README) — references docker compose up in quick-start; cites the 8080 entrypoint"
  - "Plan 07-06 (publication governance) — no filter-repo interaction (compose/Dockerfiles are NEW files with no history to scrub)"
  - "DIST-04 deliverable — artifacts committed; verification (compose config + image builds) PENDING Task 3 human-verify gate (Docker not in PATH on this machine)"
tech-stack:
  added:
    - "milvusdb/milvus:v2.4.17 (Milvus standalone, RocksMQ embedded — matches SDK v2.4.2)"
    - "quay.io/coreos/etcd:v3.5.16 (Milvus metadata)"
    - "minio/minio:RELEASE.2024-09-13T20-26-02Z (Milvus object storage)"
    - "golang:1.25-alpine (backend build stage)"
    - "alpine:3.20 (backend runtime stage)"
    - "node:22-alpine (frontend build stage)"
    - "nginx:1.27-alpine (frontend runtime stage)"
  patterns:
    - "Compose two-layer fail-fast: ${VAR:?msg} on BGE_M3_URL + GLM_API_KEY (layer 2; layer 1 is envRequired() in cmd/server/main.go)"
    - "healthcheck-gated depends_on chain: etcd+minio -> milvus (service_healthy) -> backend (service_healthy)"
    - "Multi-stage Dockerfile boundary: Go toolchain + node stay in build stage; runtime image has only ca-certificates + tzdata + the static binary + skills/+knowledge/ (backend) or nginx + web/dist/ (frontend)"
    - "nginx SSE-friendly proxy: proxy_buffering off + proxy_set_header Connection \"\" (load-bearing for UIMessageChunk streaming — without it, chunks buffer and the trust UI degrades)"
    - "Named volumes (etcd-data / minio-data / milvus-data / backend-data) keep the clone clean — no ./volumes/ host-directory litter"
key-files:
  created:
    - docker-compose.yml
    - Dockerfile.backend
    - Dockerfile.frontend
  modified: []
decisions:
  - "Milvus pinned to v2.4.17 (research-derived A1/A4 assumption — any v2.4.x patch satisfies SDK v2.4.2; human-verifier should confirm v2.4.17 is the latest v2.4.x at hub.docker.com/r/milvusdb/milvus/tags before building, and adjust if a newer v2.4.x exists)"
  - "Named volumes renamed to *-data suffix (etcd-data / minio-data / milvus-data) to disambiguate from service names — the plan's 5-services grep gate `^  (etcd|minio|...):` would otherwise double-count the volume keys"
  - "Backend SQLite path = /data/skillforge.db on the backend-data named volume (audit rows survive container restarts; SQLITE_DSN env set in compose)"
  - "nginx config written via single-line RUN printf (per plan: 'printf keeps it a single Dockerfile'); avoids both the Dockerfile line-continuation ambiguity and a bundled nginx.conf file"
metrics:
  duration: 5m
  completed: 2026-08-13
  tasks: 2     # of 3; Task 3 is checkpoint:human-verify (Docker not in PATH)
  commits: 2   # 798248d (Task 1) + 3d9f590 (Task 2); SUMMARY/state commit follows
  files_changed: 3
requirements_covered: []   # DIST-04 PENDING Task 3 human-verify (compose config + image builds under Docker)
---

# Phase 7 Plan 02: Containerization Summary

**One-liner:** Committed the one-command full-stack topology — `docker-compose.yml` (Milvus v2.4.17 standalone + Go backend + nginx frontend, two-layer `${VAR:?}` fail-fast, healthcheck-gated `depends_on`) and both multi-stage Dockerfiles (Go 1.25-alpine build -> alpine runtime; node 22 build -> nginx with `proxy_buffering off` for SSE) — with the docker-build verification cleanly checkpointed because Docker is not in PATH on this machine.

## What Was Built

### Task 1 — docker-compose.yml (`798248d`)

Five services wired as a healthcheck-gated dependency chain, Milvus pinned to the SDK-compatible v2.4.x line, no embedding service container (D7-02 — embedding is external via `BGE_M3_URL`):

| Service | Image (pin) | Ports | Role |
|---------|-------------|-------|------|
| `etcd` | `quay.io/coreos/etcd:v3.5.16` | internal | Milvus metadata |
| `minio` | `minio/minio:RELEASE.2024-09-13T20-26-02Z` | 9001 (console) | Milvus object storage |
| `milvus` | `milvusdb/milvus:v2.4.17` | 19530 (gRPC) + 9091 (metrics) | Vector DB standalone (RocksMQ embedded) |
| `backend` | built from `Dockerfile.backend` | 8888 | Go Hertz server |
| `frontend` | built from `Dockerfile.frontend` | 8080 -> 80 | nginx static + /api proxy |

**Dependency chain:** `etcd`+`minio` (各自 healthcheck) -> `milvus` (`condition: service_healthy` on both) -> `backend` (`condition: service_healthy` on milvus) -> `frontend` (plain depends_on backend).

**Fail-fast env (DIST-03 layer 2):** backend env stanza uses `${BGE_M3_URL:?BGE_M3_URL is required — see .env.example}` and `${GLM_API_KEY:?GLM_API_KEY is required — see .env.example}` — compose itself aborts at startup if either is unset. Optional vars (`BGE_M3_MODEL`, `GLM_BASE_URL`) use `${VAR:-default}`. `MILVUS_ADDRESS: milvus:19530` uses the compose service DNS (not localhost). `SQLITE_DSN: /data/skillforge.db` rides the `backend-data` named volume so audit rows survive restarts.

**Named volumes:** `etcd-data`, `minio-data`, `milvus-data`, `backend-data` (renamed to `*-data` suffix — see Deviations). `restart: unless-stopped` on backend + frontend. Top-level `name: skillforge`. No `:latest` anywhere. No ollama/bge-m3 service (D7-02).

**Verify gates (10/10 pass at HEAD):** YAML parses (UTF-8); Milvus v2.4.x pin = 1; `:latest` = 0; `${VAR:?}` fail-fast = 2; `condition: service_healthy` = 3; `MILVUS_ADDRESS: milvus:19530` = 1; embedding-container grep = 0; 5 services = 5; `name: skillforge` = 1; `restart: unless-stopped` = 2.

### Task 2 — Dockerfile.backend + Dockerfile.frontend (`3d9f590`)

**Dockerfile.backend** (multi-stage, T-7-OSS-CONTAINER mitigation):
- Build stage `golang:1.25-alpine`: `COPY go.mod go.sum` -> `go mod download` -> `COPY . .` -> `CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/skillforge ./cmd/server`. CGO-free is safe because `modernc.org/sqlite v1.56.0` is pure Go.
- Runtime stage `alpine:3.20`: `apk add --no-cache ca-certificates tzdata`, copies the binary to `/app/skillforge`, copies `skills/` + `knowledge/` (runtime data the loader + indexer scan — `skill.NewLoader("skills/")` + `knowledge.Index` over `"knowledge/"` in `cmd/server/main.go`). `EXPOSE 8888`, `ENTRYPOINT ["/app/skillforge"]`.

**Dockerfile.frontend** (multi-stage, SSE load-bearing):
- Build stage `node:22-alpine`: `COPY web/package.json web/package-lock.json` -> `npm ci` -> `COPY web/ .` -> `npm run build` (emits `web/dist/`).
- Runtime stage `nginx:1.27-alpine`: copies `web/dist/` to `/usr/share/nginx/html`, writes `default.conf` via a single-line `RUN printf` with `location / { try_files $uri /index.html; }` (SPA fallback) + `location /api { proxy_pass http://backend:8888; proxy_http_version 1.1; proxy_set_header Connection ""; proxy_buffering off; }` (SSE-friendly — `proxy_buffering off` is load-bearing for UIMessageChunk streaming). `EXPOSE 80`.

**Verify gates (all pass at HEAD):** backend `FROM golang:1.25` = 1, `CGO_ENABLED=0` >= 1, `COPY skills/` = 1, `COPY knowledge/` = 1, `^FROM` = 2; frontend `FROM node:22` = 1, `npm run build` >= 1, `FROM nginx:1.27` = 1, `proxy_buffering off` >= 1, `proxy_pass http://backend:8888` >= 1, `try_files $uri /index.html` >= 1, `^FROM` = 2; cross-file `:latest` audit clean (0 in each).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking grep-gate mismatch] Named volumes renamed to `*-data` suffix**
- **Found during:** Task 1 verification
- **Issue:** The plan's 5-services grep gate `grep -cE '^  (etcd|minio|milvus|backend|frontend):' docker-compose.yml = 5` initially returned 8 because named volumes (`etcd:`, `minio:`, `milvus:`) share those keys at the same 2-space indent as the services.
- **Fix:** Renamed the volume keys to `etcd-data`, `minio-data`, `milvus-data` (and updated the per-service `volumes:` references). `backend-data` was already disambiguated. Now the gate returns exactly 5 (services) + 0 (volumes).
- **Files modified:** `docker-compose.yml` (pre-commit, same Task 1 commit `798248d`)
- **Commit:** `798248d`

**2. [Rule 3 - Blocking grep-gate mismatch] Rephrased comments to avoid blunt-grep false positives**
- **Found during:** Task 1 + Task 2 verification
- **Issue:** (a) Task 1's "embedding container" gate `grep -ci 'ollama|bge-m3.*image:|bge-m3.*container' = 0` initially returned 2 because the header comment mentioned "ollama pull bge-m3" as a stranger-flow hint and "BGE-M3" (hyphenated) in prose. (b) Task 2's `:latest` audit initially flagged both Dockerfiles because the header comments literally said "No :latest tags" (the string `:latest` appears in the documentation of its absence).
- **Fix:** (a) Rephrased the compose header comment to reference `.env.example` for the embedding minimum instead of restating `ollama pull bge-m3`. (b) Rephrased both Dockerfile header comments from "No :latest tags" to "All base images are pinned to specific tags". No functional change — the prose intent is preserved; only the literal trigger strings were removed.
- **Files modified:** `docker-compose.yml`, `Dockerfile.backend`, `Dockerfile.frontend` (all pre-commit)
- **Commits:** `798248d` (Task 1), `3d9f590` (Task 2)

No other deviations. The compose topology, Dockerfile stages, image pins, healthcheck chain, fail-fast env, and SSE proxy config all match the plan's `<behavior>` and RESEARCH Examples 1/2/3 verbatim.

## Task 3 — Checkpoint (NOT executed; awaiting human verification)

Task 3 is `checkpoint:human-verify` (gate="blocking"). The verification commands (`docker compose config`, `docker build -f Dockerfile.backend`, `docker build -f Dockerfile.frontend`) require Docker Engine / Docker Desktop with compose v2 — **which is NOT in this machine's PATH** (confirmed: `command -v docker` returns nothing; `docker --version` fails). This matches the orchestrator's `critical_notes` prediction exactly.

The checkpoint is returned in the completion message with the exact commands the human must run. No code changes happen in Task 3. The plan is **2/3 tasks complete**; DIST-04 stays pending until the human-verify gate passes.

## Threat Model — Disposition Status

| Threat ID | Component | Plan Disposition | HEAD Status after 07-02 |
|-----------|-----------|------------------|-------------------------|
| T-7-OSS-CONTAINER (Tampering) | base images | mitigate | **Mitigated** — all 7 images pinned to specific tags (`golang:1.25-alpine`, `node:22-alpine`, `nginx:1.27-alpine`, `alpine:3.20`, `milvusdb/milvus:v2.4.17`, `quay.io/coreos/etcd:v3.5.16`, `minio/minio:RELEASE.2024-09-13T20-26-02Z`); zero `:latest` (verified by cross-file grep audit). All from official / upstream publishers. |
| T-7-OSS-CONTAINER (EoP) | runtime image | mitigate | **Mitigated** — both Dockerfiles are 2-stage; Go toolchain + node stay in the build stage; backend runtime = alpine + ca-certificates + tzdata + binary + skills/+knowledge/ only; frontend runtime = nginx + web/dist/ only. |
| T-7-OSS-ENV | compose env stanza | mitigate | **Mitigated** — `${VAR:?}` on BGE_M3_URL + GLM_API_KEY fail-fasts at compose-up if unset; `MILVUS_ADDRESS` uses compose service DNS (`milvus:19530`), not an internal IP. |
| T-7-OSS-INFO | Dockerfile build context | mitigate (Plan 01 .dockerignore) | **Inherited from 07-01** — `.dockerignore` (Plan 01) excludes `.planning/`, `.git/`, `.claude/`, `.env`, `*.exe`, `node_modules/`, `web/dist/`, `skillforge.db*`, `Dockerfile*`, `docker-compose*.yml`. No new build-context surface introduced. |
| T-7-SC | Milvus/etcd/minio image provenance | accept | **Accepted** (out of slopcheck scope) — all three from upstream publishers (milvus-io, etcd-io, minio). |

## Threat Flags

None. The three new files introduce no new network endpoint (the backend `/api/chat` SSE endpoint already exists from Phase 2; nginx just proxies it), no new auth path, no new file-access pattern beyond the existing `skills/` + `knowledge/` reads. The compose env stanza references existing env-var names (contract from `cmd/server/main.go`); no schema change at any trust boundary.

## Known Stubs

None. All three files are complete declarative artifacts. The compose file names five real services with real image pins and real healthchecks; the Dockerfiles define complete multi-stage builds with real `COPY` / `RUN` / `ENTRYPOINT`. No placeholder values, no TODOs, no mock data. The only thing "pending" is the Docker-driven verification (Task 3), not the file contents.

## Open Items Handed to Downstream

- **Task 3 human-verify gate (blocking this plan):** A human must install Docker Desktop (or Docker Engine) with compose v2, then run (from repo root):
  1. `docker compose config >/dev/null && echo COMPOSE_OK` — validates the `${VAR:?}` fail-fast and the overall compose schema
  2. `docker build -f Dockerfile.backend -t sf-be .` — backend image builds from source (Go 1.25 multi-stage)
  3. `docker build -f Dockerfile.frontend -t sf-fe .` — frontend image builds (Vite emits `web/dist/`)
  4. (Optional, recommended) `docker compose up` with a real embedding endpoint (`ollama pull bge-m3`) + a valid `GLM_API_KEY` in `.env`, then open http://localhost:8080 and ask "出差住宿费超标 200 元，能批吗？" — expect a cited verdict
  Resume signal: "approved" (steps 1-3 pass) or describe what failed (e.g. "Milvus tag v2.4.17 not found — used v2.4.16") so the pin can be amended.
- **Milvus v2.4.17 pin verification:** The pin is a research-derived assumption (07-RESEARCH A1/A4 — "any v2.4.x patch satisfies SDK v2.4.2"). The human-verifier should confirm `v2.4.17` is still the latest v2.4.x patch at `hub.docker.com/r/milvusdb/milvus/tags?page=1&name=v2.4` before building; if a newer v2.4.x exists, bump the pin (no other change needed). The resume-signal explicitly anticipates this case.
- **Plan 07-03 (README):** References `docker compose up` in quick-start and the `http://localhost:8080` entrypoint. Cites the compose `${VAR:?}` fail-fast as the DIST-03 layer-2 story.
- **DIST-04 requirement:** Marked complete only after Task 3 resume signal is "approved". Until then the artifacts exist but the "stranger can compose-up" claim is unverified.

## Self-Check: PASSED

**Files exist:**
- `docker-compose.yml` — FOUND
- `Dockerfile.backend` — FOUND
- `Dockerfile.frontend` — FOUND
- `.planning/phases/07-open-source-release/07-02-SUMMARY.md` — FOUND (this file)

**Commits exist (verified in `git log`):**
- `798248d` — `feat(07-02): add docker-compose.yml (Milvus standalone + backend + frontend)` — FOUND
- `3d9f590` — `feat(07-02): add multi-stage Dockerfile.backend + Dockerfile.frontend` — FOUND

**Verify gates (all pass at HEAD):**
- Task 1 (10/10): YAML parse OK, Milvus v2.4.x pin = 1, `:latest` = 0, `${VAR:?}` = 2, `condition: service_healthy` = 3, `MILVUS_ADDRESS: milvus:19530` = 1, embedding-container = 0, 5 services = 5, `name: skillforge` = 1, `restart: unless-stopped` = 2
- Task 2 (all pass): backend `FROM golang:1.25` = 1, `CGO_ENABLED=0` = 3 (>= 1), `COPY skills/` = 1, `COPY knowledge/` = 1, `^FROM` = 2; frontend `FROM node:22` = 1, `npm run build` = 1, `FROM nginx:1.27` = 1, `proxy_buffering off` = 2 (>= 1), `proxy_pass http://backend:8888` = 1, `try_files $uri /index.html` = 1, `^FROM` = 2; cross-file `:latest` audit clean (0 in each file)

**Deletion check:** zero file deletions across both task commits.

**Plan status:** 2/3 tasks complete and committed; Task 3 is a blocking human-verify gate (Docker not in PATH). Plan is IN PROGRESS — not advanced to next plan until the checkpoint resume signal is "approved".
