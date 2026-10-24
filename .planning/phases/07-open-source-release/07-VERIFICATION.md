---
phase: 07-open-source-release
verified: 2026-08-13T21:30:00Z
status: human_needed
score: 11/16 must-haves verified by autonomous work; 5 routed to human/environment
overrides_applied: 0
human_verification:
  - test: "DIST-04 verify — install Docker Desktop + run compose config + both image builds"
    expected: "COMPOSE_OK printed; both `docker build -f Dockerfile.{backend,frontend}` exit 0; no :latest pulled"
    why_human: "Docker is not in this machine's PATH (RESEARCH §Environment Availability VERIFIED). The compose YAML parses + structural greps pass, but full validation needs the Docker daemon."
  - test: "DIST-09 publication — run `cmd/eval --frozen=compliance` against a live stack and paste `scripts/eval_snapshot` stdout into README ## Eval Results"
    expected: "eval/report.json exists with 50 cases; README table has a real date + real faithfulness/context_precision/refusal_accuracy numbers (no _TBD_ cells)"
    why_human: "Needs live Milvus + BGE-M3 + GLM_API_KEY stack (docker compose up + ollama pull bge-m3 + .env). The derivation tool (scripts/eval_snapshot) is shipped + 6/6 unit tests green, but the report.json + README publication require a real run."
  - test: "DIST-05 — record docs/demo.gif (cited verdict + structured refusal, <5MB) and verify README hero embed resolves"
    expected: "docs/demo.gif exists; `grep -c 'docs/demo.gif' README.md` = 1; GIF shows both a cited verdict card and a structured refusal card (D7-05/06 brand coverage)"
    why_human: "Needs (a) the stack from the previous item running at http://localhost:8080, and (b) a screen recorder (ScreenToGif / OBS / macOS Screen Recording) + optimizer (gifsicle / ffmpeg). VHS only records terminals; UI walkthrough must be screen capture."
  - test: "Plan 07-06 Task 1-2 — git filter-repo single-pass rewrite on a FRESH clone (strip .planning/ + scrub internal IPs/domains)"
    expected: "Post-rewrite: `git grep -nE '10\\.62\\.10\\.34|internal\\.company\\.com|bge-m3\\.internal' $(git rev-list --all)` returns ZERO matches; `git log --oneline --all -- .planning/ | wc -l` = 0"
    why_human: "Destructive + irreversible (rewrites all commit hashes). git-filter-repo IS installed (v2.47.0 VERIFIED), but the operation must run on a fresh clone and the user must confirm the scrub table + audit gates before publication."
  - test: "Plan 07-06 Task 3 — create public `skillforge` remote (confirm GitHub username) + force-push rewritten history + clean-clone verify"
    expected: "Public repo live at github.com/<username>/skillforge; clean clone has zero internal info, no .planning/, all release artifacts present; `go test ./internal/skill -run TestExamplesValidate` passes on the public clone"
    why_human: "Requires (a) user to confirm GitHub username (CONTEXT §specifics: `ninedrunk` '待确认'), (b) create the empty public repo, (c) authorize a destructive force-push (no remote currently configured: `git remote -v` empty)."
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: human_needed
---

# Phase 7: Open-Source Release Verification Report

**Phase Goal:** Ship a polished, documented, demo-able open-source release that a stranger can fork, run locally, and benchmark — the artifact that earns the personal-brand.
**Verified:** 2026-08-13T21:30:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
| -- | ----- | ------ | -------- |
| T1 | LICENSE exists at repo root with verbatim Apache-2.0 full text (DIST-02) | ✓ VERIFIED | `LICENSE` committed (10,280 bytes); `head -1` = "Apache License"; `grep -c 'Apache License'` = 4; SPDX canonical body. Commit `2a76533`. |
| T2 | `.env.example` contains NO internal IPs/domains, only compose-friendly defaults + documents `${VAR:?}` fail-fast (DIST-03) | ✓ VERIFIED | `grep -cE '10\.62\.10\.34\|internal\.company\.com\|bge-m3\.internal' .env.example` = 0; `MILVUS_ADDRESS=milvus:19530` + `BGE_M3_URL=http://localhost:11434/v1/embeddings` present; 10 cmd/server vars preserved; ollama one-liner + `host.docker.internal` Docker hint (WR-02 fix `5df872a`). Commit `e4e34ef`. |
| T3 | `.dockerignore` excludes `.planning/`, `.git/`, `.claude/`, `*.exe`, `node_modules/`, `web/dist/`, `.env`, `skillforge.db*`; `.gitignore` gained `skillforge.db*` (3 lines) | ✓ VERIFIED | `grep -cE '^\.(planning\|git\|claude)' .dockerignore` = 4; `grep -c 'skillforge.db' .gitignore` = 3. Commits `2a76533`. |
| T4 | ROADMAP SC-1 / SC-5 + REQUIREMENTS DIST-04 / DIST-05 wording matches locked decisions D7-03 / D7-06 | ✓ VERIFIED | `grep -c 'pluggable embedding endpoint' ROADMAP.md` = 1; `grep -ci 'live demo site' ROADMAP.md` = 0; REQUIREMENTS DIST-04 reads "pluggable embedding endpoint"; DIST-05 reads "Recorded demo GIF ... D7-06 reversed". Commit `418800b`. |
| T5 | `docker-compose.yml` defines 5 services; Milvus pinned to v2.4.x (matches SDK v2.4.2); `${VAR:?}` fail-fast on BGE_M3_URL + GLM_API_KEY; no `:latest` (DIST-04 files) | ✓ VERIFIED | `grep -cE 'milvusdb/milvus:v2\.4' docker-compose.yml` = 1; `:latest` = 0; `${VAR:?}` = 2; `^  (etcd\|minio\|milvus\|backend\|frontend):` = 5; YAML parses (UTF-8). Commit `798248d`. |
| T6 | `Dockerfile.backend` is multi-stage Go 1.25 → alpine, CGO_ENABLED=0, ships skills/+knowledge/ (DIST-04 files) | ✓ VERIFIED | `FROM golang:1.25` = 1; `CGO_ENABLED=0` = 3; `^FROM` = 2 (multi-stage); `COPY skills/` + `COPY knowledge/` present. Commit `3d9f590`. |
| T7 | `Dockerfile.frontend` is multi-stage node:22 → nginx with `/api` proxy + `proxy_buffering off` (SSE-friendly) + `proxy_read_timeout` (WR-01 fix) | ✓ VERIFIED | `FROM node:22` = 1; `FROM nginx:1.27` = 1; `proxy_buffering off` = 2; `proxy_pass http://backend:8888` = 1; `proxy_read_timeout` = 1 (WR-01 fix landed in `5df872a`); `^FROM` = 2. Commit `3d9f590` + `5df872a`. |
| T8 | README has all 7 DIST-01 elements (badges, value prop, demo GIF path, quick start, Mermaid arch diagram, contributing, license) | ✓ VERIFIED | `grep -cE 'badge\|Quick Start\|docker compose\|```mermaid\|Contributing\|Apache-2.0'` = 12; `docs/demo.gif` embed present; `ollama pull bge-m3` ×2; `localhost:8080` ×3; `## Eval Results` section exists (PLACEHOLDER). Commit `d35e7a6`. |
| T9 | CONTRIBUTING has 3 named sections (Write a Skill / Add a Tool / Add a Knowledge Tier) each with code examples (DIST-07) | ✓ VERIFIED | `grep -cE 'Write a Skill\|Add a Tool\|Add a Knowledge Tier'` = 3; TierEnum CN chars (权威/历史/个人/草稿) = 10; temperature documented; ChatModelAgent/ReActAgent referenced. Commit `4daade7`. |
| T10 | Zero internal IPs/domains in README + CONTRIBUTING (T-7-OSS-INFO at HEAD) | ✓ VERIFIED | `grep -cE '10\.62\.10\.34\|internal\.company\.com\|bge-m3\.internal' README.md CONTRIBUTING.md` = 0 across both. |
| T11 | `examples/` ships 3 Skill templates all passing `internal/skill.Validate` (DIST-08) | ✓ VERIFIED | `go test ./internal/skill -run TestExamplesValidate -v` → PASS (VERIFIED THIS SESSION); compliance is byte-identical to `skills/compliance/SKILL.md` (`diff -q` identical); calculator uses `reads: [draft/**]` placeholder tier (validator UNCHANGED — D7-10 honored); simple-qa `agent: ChatModelAgent`, calculator `agent: ReActAgent`. Commits `9f2b734` (RED) + `1eddc53` (GREEN). |
| T12 | `examples/data/` ships 7 sample docs verbatim from `knowledge/authority/` (DIST-06) | ✓ VERIFIED | `ls examples/data/*.md \| wc -l` = 7; `diff -q examples/data/annual_leave.md knowledge/authority/annual_leave.md` identical (spot-check). Commit `1eddc53`. |
| T13 | `eval/sample.jsonl` is a 50-case byte-identical copy of `eval/golden/compliance.jsonl` (DIST-10) | ✓ VERIFIED | `wc -l < eval/sample.jsonl` = 50; `diff -q eval/sample.jsonl eval/golden/compliance.jsonl` identical; categories: 20 adversarial + 20 refusal-required + 10 multi-hop. Commit `51b042f`. |
| T14 | `scripts/eval_snapshot` derives refusal_accuracy + is unit-tested (DIST-09 derivation) | ✓ VERIFIED | `go test ./scripts/eval_snapshot/ -v` → 6/6 PASS (VERIFIED THIS SESSION: AllRefused/HalfRefused/ZeroRefusalRequired/CategoryFilter/LockMetadata/NACell); `deriveRefusalAccuracy` pure fn present; stdlib-only imports; `cmd/eval` + `internal/eval` UNCHANGED. Commits `abf2f4d` (RED) + `4873213` (GREEN). |
| T15 | Full Go build + test suite regression-clean after Phase 7 additions | ✓ VERIFIED | `go build ./...` exit 0; `go test ./...` all packages ok (VERIFIED THIS SESSION — internal/skill, scripts/eval_snapshot, cmd/eval, cmd/spiked, internal/agent, internal/eval, internal/skill, internal/store, internal/verdict, internal/wire all green). |
| T16 | `docker compose config >/dev/null` + both `docker build -f Dockerfile.{backend,frontend}` exit 0 (DIST-04 verify / SC-1) | ? HUMAN_NEEDED | Docker is NOT in this machine's PATH (`command -v docker` empty — VERIFIED THIS SESSION). Files are structurally correct but boot validation requires Docker Engine/Desktop. Plan 07-02 Task 3 `checkpoint:human-verify gate=blocking` was correctly NOT bypassed. |
| T17 | `eval/report.json` exists from `cmd/eval --frozen=compliance`; README `## Eval Results` filled with real dated numbers (DIST-09 publication / SC-7) | ? HUMAN_NEEDED | `eval/report.json` does NOT exist (VERIFIED THIS SESSION); README lines 126-130 are explicit PLACEHOLDER (`_TBD (Plan 07-05)_` + `_TBD_` cells). Needs live Milvus + BGE-M3 + GLM stack. Derivation tool (T14) is shipped and ready. Plan 07-05 Task 2 `checkpoint:human-verify` was correctly NOT bypassed. |
| T18 | `docs/demo.gif` exists, <5MB, shows cited verdict AND structured refusal; README hero embed resolves (DIST-05 / SC-5) | ? HUMAN_NEEDED | `docs/` directory does NOT exist (VERIFIED THIS SESSION). Needs screen capture + running stack. Plan 07-05 Task 3 `checkpoint:human-verify` was correctly NOT bypassed. |
| T19 | Single combined `git filter-repo` run strips `.planning/` + scrubs internal IPs/domains across full history (D7-08 + D7-09 / T-7-OSS-INFO history) | ? HUMAN_NEEDED | `.planning/` still present in 178 commits (`git log --oneline --all -- .planning/ \| wc -l` = 178 — VERIFIED THIS SESSION). `git-filter-repo` v2.47.0 IS installed but the rewrite is destructive + must run on a fresh clone + needs user-confirmed scrub table. Plan 07-06 Task 2 `checkpoint:human-verify gate=blocking` was correctly NOT bypassed. |
| T20 | Public `skillforge` remote created; rewritten history force-pushed; clean clone passes gates (D7-07 publication) | ? HUMAN_NEEDED | `git remote -v` is empty (VERIFIED THIS SESSION). Username `ninedrunk` per CONTEXT §specifics but explicitly "待确认". Plan 07-06 Task 3 `checkpoint:human-verify gate=blocking` was correctly NOT bypassed. |

**Score:** 15/20 truths fully VERIFIED by autonomous work + this session's regression run; 5 truths route to human/environment (T16, T17, T18, T19, T20).

(The 16/20 headline in the frontmatter collapses T15 as a meta-check; the explicit count is 15 of 20 observable truths VERIFIED, 5 HUMAN_NEEDED, 0 FAILED.)

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `LICENSE` | Apache-2.0 verbatim full text (DIST-02) | ✓ VERIFIED | 10,280 bytes; SPDX canonical; `Apache License` ×4. |
| `.env.example` | Compose-friendly defaults + fail-fast contract (DIST-03) | ✓ VERIFIED | 0 internal-info matches; `host.docker.internal` hint added (WR-02). |
| `.dockerignore` | Build-context boundary excludes sensitive paths | ✓ VERIFIED | `.planning/`, `.git/`, `.claude/`, `*.exe`, `skillforge.db*`, etc. |
| `.gitignore` | Adds `skillforge.db*` (3 lines) | ✓ VERIFIED | Local SQLite audit store never committed. |
| `docker-compose.yml` | 5-service full-stack topology; Milvus v2.4.x (DIST-04) | ✓ VERIFIED (file) | YAML parses; all 10 structural greps pass. Boot verification → T16 (human). |
| `Dockerfile.backend` | Multi-stage Go 1.25-alpine build, CGO-free (DIST-04) | ✓ VERIFIED (file) | `^FROM` = 2; ships skills/+knowledge/. Build verification → T16 (human). |
| `Dockerfile.frontend` | node:22 → nginx + `/api` proxy_buffering off + SPA fallback (DIST-04) | ✓ VERIFIED (file) | `proxy_read_timeout` added (WR-01 fix `5df872a`). Build verification → T16 (human). |
| `README.md` | All 7 DIST-01 elements + eval placeholder + 3 moats | ✓ VERIFIED | 12 element-grep matches; Mermaid diagram inline; zero internal info. |
| `CONTRIBUTING.md` | 3 named sections w/ code examples (DIST-07) | ✓ VERIFIED | Write a Skill / Add a Tool / Add a Knowledge Tier all present w/ fenced blocks. |
| `examples/skills/{compliance,simple-qa,calculator}/SKILL.md` | 3 templates passing 7-field validator (DIST-08) | ✓ VERIFIED | `TestExamplesValidate` PASS; compliance byte-identical to canonical; calculator placeholder tier per D7-10. |
| `examples/data/*.md` (×7) | Pre-bundled sample compliance docs (DIST-06) | ✓ VERIFIED | 7 files; spot-check `diff -q` identical to `knowledge/authority/`. |
| `internal/skill/examples_validate_test.go` | DIST-08 schema-validation Go test | ✓ VERIFIED | GREEN this session. |
| `eval/sample.jsonl` | 50-case published eval sample (DIST-10) | ✓ VERIFIED | Byte-identical to golden; 20/20/10 category split. |
| `scripts/eval_snapshot/{main.go,main_test.go}` | refusal_accuracy derivation tool (DIST-09 derivation) | ✓ VERIFIED | 6/6 hermetic tests PASS this session; stdlib-only imports. |
| `eval/report.json` | Frozen-run metric output (DIST-09 publication) | ✗ MISSING (human) | Needs `go run ./cmd/eval --frozen=compliance` against live stack. |
| `eval/calibration.json` | Calibration output alongside report.json | ✗ MISSING (human) | Same — produced by `cmd/eval --frozen`. |
| `docs/demo.gif` | Recorded demo (cited verdict + structured refusal, <5MB) (DIST-05) | ✗ MISSING (human) | Needs screen capture + running stack. |
| `scrub.txt` | Release-time filter-repo replace-text table (Plan 07-06) | ✗ MISSING (human) | Untracked release-time artifact; produced on fresh clone before rewrite. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `.env.example` | `docker-compose.yml` | `${VAR:?}` interpolation of BGE_M3_URL / GLM_API_KEY / MILVUS_ADDRESS | ✓ WIRED | `${BGE_M3_URL:?...}` + `${GLM_API_KEY:?...}` in compose env stanza; MILVUS_ADDRESS uses compose DNS `milvus:19530`. |
| `ROADMAP.md` | `REQUIREMENTS.md` | SC-1/SC-5 + DIST-04/DIST-05 wording revisions | ✓ WIRED | Both files updated consistently per D7-03/D7-06. |
| `docker-compose.yml` milvus service | `go.mod` milvus-sdk-go/v2 v2.4.2 | image tag pin v2.4.17 (RocksMQ embedded) | ✓ WIRED | Pin matches SDK; not v3 / not :latest. |
| `Dockerfile.frontend` nginx config | `backend:8888` | `proxy_pass` + `proxy_buffering off` + `proxy_read_timeout 3600s` | ✓ WIRED | SSE-friendly (WR-01 fix applied). |
| `README.md` Quick Start | `docker-compose.yml` + `.env.example` | `docker compose up` + `cp .env.example .env` | ✓ WIRED | Both referenced from README; `localhost:8080` entrypoint documented. |
| `README.md` hero | `docs/demo.gif` | `![SkillForge demo](docs/demo.gif)` embed | ⚠ PARTIAL | Embed wired in README (1 match); target file does NOT exist (T18 human). |
| `README.md` Eval Results | `eval/report.json` via `scripts/eval_snapshot` | metric values trace to report fields | ⚠ PARTIAL | Script + reproduce command wired; report.json MISSING + table cells are `_TBD_` (T17 human). |
| `CONTRIBUTING.md` Write a Skill | `internal/skill.Validate` + `examples/skills/*` | `go test ./internal/skill -run TestExamplesValidate` | ✓ WIRED | Test exists + GREEN; examples/ tree validates. |
| `scripts/eval_snapshot/main.go` | `internal/eval.LoadReport` + `LoadLock` | Go import + function calls | ✓ WIRED | `eval.LoadReport` + `eval.LoadLock` each present (2 matches); `internal/eval` UNCHANGED. |
| `internal/skill/examples_validate_test.go` | `internal/skill.Validate` via `skill.NewLoader(...).Scan()` | Loader → parseAndValidate → Validate + ResolveTiers | ✓ WIRED | Test PASS this session exercises the full path. |
| Plan 07-06 scrub.txt → filter-repo | full-history internal-string audit | `git grep -nE '...' $(git rev-list --all)` zero-match gate | ✗ NOT_WIRED (human) | Rewrite not executed; .planning/ still in 178 commits. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `scripts/eval_snapshot` markdown output | `deriveRefusalAccuracy(report)` ratio | `eval.LoadReport("eval/report.json")` | n/a — script is the derivation tool, not a renderer of live data | ✓ VERIFIED (logic) — 6/6 hermetic tests pin the math; produces real data once `eval/report.json` is fed in (T17 human blocks the feeding step). |
| `README.md` Eval Results table | faithfulness / context_precision / refusal_accuracy / case count | `eval/report.json` (via script paste) | No — `_TBD_` cells; `eval/report.json` MISSING | ⚠ HOLLOW — wiring (script + reproduce command) is in place; data production blocked by live-stack dependency (T17). |
| `examples_validate_test.go` assertions | manifest count + zero Validate error | `skill.NewLoader("../../examples/skills").Scan()` | Yes — 3 manifests validate against the real 7-field validator | ✓ FLOWING. |
| `docker-compose.yml` backend env | `BGE_M3_URL`, `GLM_API_KEY`, `MILVUS_ADDRESS` | `.env` (user-supplied) + compose `${VAR:?}` interpolation | Will flow once user populates `.env` and runs `docker compose up` | ✓ WIRED (config) — boot verification is T16 (human). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| DIST-08 schema gate | `go test ./internal/skill -run TestExamplesValidate -v` | `PASS: TestExamplesValidate (0.00s)` | ✓ PASS |
| DIST-09 derivation unit tests | `go test ./scripts/eval_snapshot/ -v` | 6/6 `--- PASS` (AllRefused/HalfRefused/ZeroRefusalRequired/CategoryFilter/LockMetadata/NACell) | ✓ PASS |
| Full Go test suite regression | `go test ./...` | all 14 test-bearing packages `ok` (cached); zero FAIL | ✓ PASS |
| Go build smoke (all cmd + scripts) | `go build ./cmd/... ./scripts/eval_snapshot/` | exit 0 | ✓ PASS |
| Compose YAML parse (structural only) | `python -c "import yaml; yaml.safe_load(open('docker-compose.yml', encoding='utf-8'))"` | `YAML OK (utf-8)` | ✓ PASS |
| `docker compose config` (DIST-04 boot gate) | `docker compose config >/dev/null` | SKIPPED — `docker` not in PATH | ? SKIP (T16 human) |
| Image builds (DIST-04 boot gate) | `docker build -f Dockerfile.{backend,frontend} -t sf-{be,fe} .` | SKIPPED — `docker` not in PATH | ? SKIP (T16 human) |
| Live eval run (DIST-09 publication) | `go run ./cmd/eval --frozen=compliance` | SKIPPED — no live Milvus + BGE-M3 + GLM stack | ? SKIP (T17 human) |

### Probe Execution

Phase 7 declares no `scripts/*/tests/probe-*.sh` probes; validation is configured as grep gates + Go tests + Docker-driven smoke (the latter deferred to human). All configured Go-test probes (TestExamplesValidate, scripts/eval_snapshot tests) executed and PASSED this session.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| DIST-01 | 07-03 | README with badges, value prop, demo GIF, quick start, architecture diagram, contributing, license | ✓ SATISFIED | 12 element-grep matches; Mermaid diagram inline; all 7 elements present. |
| DIST-02 | 07-01 | Apache-2.0 license file present from day 1 | ✓ SATISFIED | LICENSE 10,280 bytes; verbatim SPDX Apache-2.0. |
| DIST-03 | 07-01 | `.env.example` with every required variable; missing values → clear startup error | ✓ SATISFIED | 10 cmd/server vars; 0 internal-info matches; `${VAR:?}` fail-fast documented + layered with `envRequired()` in cmd/server/main.go. |
| DIST-04 | 07-02 | `docker compose up` brings up full stack (revised: backend + frontend + Milvus + pluggable embedding endpoint) | ⚠ NEEDS HUMAN (artifacts ✓) | Compose + 2 Dockerfiles committed and structurally correct (5 services, Milvus v2.4.17, `${VAR:?}`, multi-stage, SSE-friendly); boot verification requires Docker daemon not in env. |
| DIST-05 | 07-05 Task 3 | Recorded demo GIF in README hero (cited verdict + structured refusal flow) — D7-06 reversed the live-demo decision | ⚠ NEEDS HUMAN | `docs/demo.gif` does not exist; README embed is in place (1 match); needs screen capture + running stack. |
| DIST-06 | 07-04 | Pre-bundled sample compliance docs (works without API key for retrieval) | ✓ SATISFIED | `examples/data/*.md` ×7 byte-identical to `knowledge/authority/`. |
| DIST-07 | 07-03 | `CONTRIBUTING.md` with "write a Skill", "add a tool", "add a knowledge tier" | ✓ SATISFIED | 3 named sections; fenced code blocks ×18; TierEnum + validAgentTypes + retrieve_tool referenced. |
| DIST-08 | 07-04 | `examples/` folder with 2-3 Skill templates each passing schema validator | ✓ SATISFIED | 3 templates; `TestExamplesValidate` GREEN this session; calculator uses placeholder tier (validator UNCHANGED — D7-10). |
| DIST-09 | 07-05 Task 1 (derivation ✓) + Task 2 (publication ⚠) | Published eval results in README (faithfulness %, citation precision %, refusal accuracy %) | ⚠ NEEDS HUMAN | Derivation tool shipped + 6/6 unit tests green; README publication (real `eval/report.json` + dated numbers in table) blocked by live stack. |
| DIST-10 | 07-04 | Eval set sample included in repo (`eval/sample.jsonl`, 50 cases) | ✓ SATISFIED | `wc -l` = 50; byte-identical to `eval/golden/compliance.jsonl`; 20/20/10 category split. |

**ORPHANED requirements:** None. All 10 DIST IDs (DIST-01..10) are claimed by ≥1 plan and accounted for above.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| `README.md` | 126, 130 | `_TBD (Plan 07-05)_` + `_TBD_` cells in Eval Results table | ℹ️ Info (intentional placeholder) | Documented in prose ("Placeholder — this section is populated by Plan 07-05 ... Do not treat the table as measured yet."). Not a debt marker; it is the expected state pending Plan 07-05 Task 2. Resolves when T17 human item closes. |
| `README.md` | (hero) | `![SkillForge demo](docs/demo.gif)` embed resolves to a missing file | ℹ️ Info (intentional placeholder) | Plan 07-03 wrote the embed; Plan 07-05 Task 3 ships the GIF. Resolves when T18 human item closes. |
| `examples/skills/calculator/SKILL.md` | 5, 8, 22, 45 | "PLACEHOLDER tier" wording | ℹ️ Info (intentional design) | D7-10 + RESEARCH Q2: calculator is validate-only; `reads: [draft/**]` is the documented placeholder that lets it pass `internal/skill.Validate` without weakening P-SKILL-3 default-deny. Validator UNCHANGED — VERIFIED. Not a stub. |
| `internal/skill/examples_validate_test.go` | 46 | "// PLACEHOLDER tier" comment | ℹ️ Info (doc) | Same as above — documents the D7-10 design choice. |
| `CONTRIBUTING.md` | 99, 180 | "placeholder tier" wording | ℹ️ Info (doc) | Contributor guidance for tool-only Skills; mirrors D7-10. |

**Debt-marker gate (TBD / FIXME / XXX):** Only 2 matches, both in `README.md` `_TBD_` cells explicitly documented as Plan 07-05 Task 2 deferral. Zero `FIXME`. Zero `XXX`. Zero unreferenced markers.

**Stub classification:** All grep matches are intentional documented placeholders with a clear resolution path routed to specific human-verification items (T17, T18). None are stubs in the sense of "incomplete implementation masquerading as done."

### Human Verification Required

The following items need human testing or environment setup. The autonomous work is committed and structurally verified; these items complete the publication narrative.

#### 1. DIST-04 verify — Docker compose config + both image builds

**Test:** Install Docker Desktop (Windows) or Docker Engine + compose v2. From repo root: `docker compose config >/dev/null && echo COMPOSE_OK` then `docker build -f Dockerfile.backend -t sf-be .` then `docker build -f Dockerfile.frontend -t sf-fe .`.
**Expected:** `COMPOSE_OK` printed; both builds exit 0; no `:latest` pulled; Milvus tag `v2.4.17` resolves (if not, bump to latest v2.4.x patch — any v2.4.x satisfies SDK v2.4.2).
**Why human:** Docker is not in this machine's PATH (VERIFIED `command -v docker` empty). The compose YAML parses (Python yaml.safe_load OK) and all 10 structural greps pass, but the daemon-driven validation cannot be automated here. Recommended: also run `docker compose up` + open `http://localhost:8080` + ask "出差住宿费超标 200 元，能批吗？" — expect a cited verdict (this is the brand-narrative smoke test).

#### 2. DIST-09 publication — live-stack eval run + README snapshot

**Test:** Bring up the stack from item 1 + `ollama pull bge-m3` + populate `.env` with `GLM_API_KEY` + `BGE_M3_URL=http://host.docker.internal:11434/v1/embeddings` (or `localhost:11434` if running outside Docker). Then run `go run ./cmd/eval --frozen=compliance` from repo root. Capture stdout of `go run ./scripts/eval_snapshot`. Paste the markdown block into README.md under `## Eval Results`, replacing the `_TBD_` row.
**Expected:** `eval/report.json` exists with 50 cases; `eval/calibration.json` exists; the snapshot block has a real date + real faithfulness/context_precision/refusal_accuracy numbers (e.g. `92% / 85% / 95% (19/20)`). Every README number must trace to a `report.json` field (refusal_accuracy traces to the derived `Cases` ratio).
**Why human:** Needs live Milvus + BGE-M3 + GLM endpoints (RESEARCH §Environment Availability — no fallback for the eval stack). The derivation logic is shipped + 6/6 hermetic unit tests pin the math; only the live-run + paste step is unautomated. If a live stack is genuinely unavailable this session, leave the README placeholder intact (do NOT fabricate numbers — T-7-OSS-REP mitigation).

#### 3. DIST-05 — record `docs/demo.gif`

**Test:** With the stack from item 2 still running at `http://localhost:8080`, open the URL in a browser, select the **compliance** Skill, start a screen recorder (ScreenToGif / OBS / macOS Screen Recording). Ask a question that yields a CITED VERDICT (e.g. "出差住宿费超标 200 元，能批吗？请引用具体制度条款。"). Then ask an out-of-scope question that triggers a STRUCTURED REFUSAL (`no_authorized_source` / out-of-scope card). Stop recording, export as GIF (≤60s, optimize to <5MB via `gifsicle --optimize=3 --colors 128` or `ffmpeg`). Save as `docs/demo.gif`.
**Expected:** `docs/demo.gif` exists; size <5MB; the GIF shows BOTH a cited verdict card AND a structured refusal card (D7-05/06 brand-narrative coverage); `grep -c 'docs/demo.gif' README.md` = 1.
**Why human:** Needs (a) the live stack from item 2, and (b) a screen recorder — neither is automatable in this environment. VHS (`.tape` declarative terminal GIF) only records terminals; the UI walkthrough is the load-bearing content.

#### 4. Plan 07-06 Task 1-2 — `git filter-repo` single-pass rewrite

**Test:** On a FRESH clone (`git clone <current-repo> skillforge-release && cd skillforge-release`), create `scrub.txt` with these 4 literal lines (order matters — FQDNs first):

```
10.62.10.34==>127.0.0.1
milvus.internal.company.com==>milvus
bge-m3.internal.company.com==>localhost
bge-m3.internal==>localhost
```

Run `git filter-repo --invert-paths --path .planning/ --replace-text scrub.txt`. Then run the 4 audit gates:
- Gate 1: `git grep -nE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' $(git rev-list --all)` → ZERO matches
- Gate 2: `git log --oneline --all -- .planning/ | wc -l` → 0
- Gate 3: `git grep -nE 'AKIA[0-9A-Z]{16}|sk-[a-zA-Z0-9]{20,}' $(git rev-list --all)` → ZERO real keys (only `GLM_API_KEY=unhappy` test sentinel permitted)
- Gate 4: `grep -cE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' .env.example cmd/server/main.go internal/retriever/mongo/mongo.go internal/embedder/bge/bge.go scripts/live-verify.sh` → 0 across all 5 files

**Expected:** All 4 gates pass with zero matches; filter-repo reports a full rewrite.
**Why human:** Destructive + irreversible (rewrites every commit hash). `git-filter-repo` v2.47.0 IS installed (VERIFIED this session), but the operation must run on a fresh clone and the user must confirm the scrub table covers all internal-string variants before publication. Plan 07-06 explicitly marks this as `gate="blocking-human"`.

#### 5. Plan 07-06 Task 3 — create public `skillforge` remote + force-push + clean-clone verify

**Test:** Confirm GitHub username with the user (CONTEXT §specifics says `ninedrunk` but flagged "待确认"). Create empty public repo `github.com/<username>/skillforge` (do NOT initialize with README/LICENSE — force-push supplies everything). In the skillforge-release clone: `git remote add origin git@github.com:<username>/skillforge.git` then `git push -u --force origin master` (current branch is `master`). Then in a NEW empty directory: `git clone git@github.com:<username>/skillforge.git skillforge-verify` and re-run the gates: `git grep -nE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' $(git rev-list --all)` → ZERO; `ls .planning 2>/dev/null` → no such directory; confirm LICENSE + README + docker-compose.yml + examples/ present; (optional) `docker compose config >/dev/null` + `go test ./internal/skill -run TestExamplesValidate` pass on the public clone.
**Expected:** Public repo live; clean clone has zero internal info + no `.planning/` + all release artifacts; validator test green on public clone.
**Why human:** Requires (a) user to confirm GitHub username, (b) create the empty public repo, (c) authorize a destructive force-push (no remote currently configured: `git remote -v` empty — VERIFIED this session). Plan 07-06 explicitly marks this as `gate="blocking"`.

### Gaps Summary

**No BLOCKER gaps.** All autonomous work claimed by the SUMMARYs is verifiable in the codebase: every committed artifact exists at the expected path, is substantive (not a stub), and is internally consistent. The two reviewer warnings (WR-01 nginx read-timeout + WR-02 `.env.example` Docker host hint) were both fixed in commit `5df872a`. The Go test suite is regression-clean. The DIST-08 schema gate and the DIST-09 derivation tool both PASS this session.

**5 truths route to human/environment (T16-T20).** These are not implementation failures — they are the explicit `autonomous:false` / `checkpoint:human-verify` tasks that the plans deliberately deferred because the required environment (Docker daemon, live Milvus+BGE-M3+GLM stack, screen capture, destructive git operations, public remote) is not available in this planning environment. The plans' blocking-gate design was respected — at no point did the executor fabricate eval numbers, fake a GIF, or bypass the filter-repo gate.

**The expected publication sequence:** close items 1 → 2 → 3 (boot → eval → GIF) in any order, then 4 → 5 (history rewrite + force-push). Items 1-3 update the dev repo's HEAD (real eval numbers in README + real GIF); items 4-5 consume the resulting HEAD when rewriting history for publication.

**Note on Plan 07-06:** although `git-filter-repo` v2.47.0 turned out to be installed (the SUMMARY claimed it was not), the publication gate remains human-driven because (a) the rewrite must run on a fresh clone, (b) the user must confirm the GitHub username + create the public repo, and (c) the force-push is destructive and irreversible. This is consistent with the plan's `autonomous:false` declaration and `gate="blocking-human"` design — the environment probe was a stale assumption, not a verifier-blocking gap.

---

_Verified: 2026-08-13T21:30:00Z_
_Verifier: Claude (gsd-verifier)_
