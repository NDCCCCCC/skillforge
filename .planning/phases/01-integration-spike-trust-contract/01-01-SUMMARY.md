---
phase: 01-integration-spike-trust-contract
plan: 01
subsystem: infra
tags: [eino, hertz, milvus, bge-m3, glm, sse, spike, wave-0, internal-api]

# Dependency graph
requires:
  - phase: pre-phase
    provides: SKILL_SCHEMA.md v1.1 + spike findings (001/002/002b/003) + locked stack (eino v0.7.7, hertz v0.10.6, milvus-sdk-go v2)
provides:
  - "Wave 0 runnable Go skeleton (cmd/spiked/ with subcommand router)"
  - "spike wire subcommand (SC-1 layer a) — Eino ChatModel → mock OpenAI round-trip"
  - "4 SKILL.md fixtures (authority/historical/personal/draft) per SKILL_SCHEMA.md v1.1"
  - "go.mod with pinned deps (eino v0.7.7, hertz v0.10.6, milvus v2.4.2, eino-ext openai v0.1.5, testify v1.11.1, yaml.v3 v3.0.1)"
  - "Internal API strategy: Milvus + BGE-M3 on 内网, GLM on 公网 (no Docker)"
  - "docker-compose.yml.template as Phase 7 packaging artifact (not executed in spike)"
affects:
  - "01-02 (spike milvus — uses internal Milvus address from .env.example)"
  - "01-03 (spike sse — uses Hertz wire already in go.mod)"
  - "01-04 (spike e2e — uses real GLM key, deferred to Plan 04)"
  - "Phase 2 (production code reuses cmd/spiked/ structure)"

# Tech tracking
tech-stack:
  added:
    - "github.com/cloudwego/eino v0.7.7 (agent framework)"
    - "github.com/cloudwego/hertz v0.10.6 (HTTP + SSE)"
    - "github.com/milvus-io/milvus-sdk-go/v2 v2.4.2 (vector DB)"
    - "github.com/cloudwego/eino-ext/components/model/openai v0.1.5 (OpenAI adapter, eino v0.7.7-compatible)"
    - "github.com/cloudwego/eino-ext/libs/acl/openai v0.1.2 (transitive)"
    - "github.com/stretchr/testify v1.11.1 (assertions)"
    - "gopkg.in/yaml.v3 v3.0.1 (SKILL.md frontmatter)"
  patterns:
    - "Subcommand router pattern: switch on os.Args[1] with runX(ctx, args) signatures"
    - "Dynamic port allocation: net.Listen(tcp, 127.0.0.1:0) — no hardcoded ports"
    - "Project-root path resolution: walk up to go.mod to find fixtures regardless of CWD"
    - "Mock OpenAI server pattern: in-process HTTP handler on dynamic port, captures request body to passed-in buffer"
    - "Apache-2.0 license header on every .go file (skeleton prepended, copy-paste for new files)"

key-files:
  created:
    - "go.mod — pinned 6 dependencies (eino v0.7.7, hertz v0.10.6, milvus v2.4.2, eino-ext openai v0.1.5, testify v1.11.1, yaml.v3 v3.0.1)"
    - "go.sum — dependency checksums"
    - ".env.example — internal API placeholders (Milvus internal, BGE-M3 internal, GLM public)"
    - ".gitignore — .env excluded, anchored to /spiked /spiked.exe /cmd/spiked/spiked{,.exe} to avoid Windows case-insensitive collisions"
    - "cmd/spiked/main.go — subcommand router (4 slots: wire/milvus/sse/e2e)"
    - "cmd/spiked/scaffold_stubs.go — runMilvus/runSSE/runE2E stubs (Plans 02-04 replace)"
    - "cmd/spiked/wire.go — runWire + parseSKILL + startMockServer (dynamic port)"
    - "cmd/spiked/wire_test.go — TestSpikeWireMock + TestMockServerReceivesWireExtras"
    - "testdata/skills/authority/SKILL.md — tier=权威, temperature=0, sample authority clause"
    - "testdata/skills/historical/SKILL.md — tier=历史, superseded clause example"
    - "testdata/skills/personal/SKILL.md — tier=个人, individual record example"
    - "testdata/skills/draft/SKILL.md — tier=草稿, [DRAFT] marker, not-yet-effective"
    - ".planning/phases/01-integration-spike-trust-contract/docker-compose.yml.template — Phase 7 packaging artifact (NOT executed in spike)"
  modified: []

key-decisions:
  - "Internal API strategy (user checkpoint response): Milvus + BGE-M3 on 内网独立服务器 via gRPC/REST, GLM on 公网 Coding Plan endpoint. Skips Docker entirely. Provides full Milvus feature set (not Lite subset), LAN latency (1-5ms) more production-realistic than 0ms in-container, BGE-M3 stays on internal network for data residency, zero install time."
  - "GLM API key validation deferred to Plan 04 (spike e2e). Plan 01 wire spike uses mock OpenAI server (per D-02), so real GLM key is not needed for SC-1 layer (a). Saves 1 free-tier API call."
  - "eino-ext pinned to tagged v0.1.5 (compatible with eino v0.7.7). Plan's 'pseudo-version' approach is no longer needed since eino-ext is now tagged. v0.1.5 is the highest eino-ext openai version compatible with eino v0.7.7 (newer v0.1.13+ requires eino v0.7.13+)."
  - "Dynamic port allocation (Pitfall 7 mitigation): net.Listen(tcp, 127.0.0.1:0) instead of hardcoded :18080. Avoids port collision on shared hosts. 1ms grace period for goroutine startup (no race)."
  - "Project-root path resolution: findProjectRoot walks up to go.mod so SKILL.md fixtures resolve regardless of CWD (works from both project root and cmd/spiked/)."

patterns-established:
  - "Pattern: Subcommand router — switch on os.Args[1] with case for each subcommand, default 'unknown' to stderr exit 2, error path to stderr exit 1"
  - "Pattern: In-process mock OpenAI server — net.Listen dynamic port, /v1/chat/completions handler, captured body via passed-in bytes.Buffer pointer"
  - "Pattern: openai.WithExtraFields map for response_format + thinking + custom fields (Eino v0.1.13+ API, replaces deprecated RequestBodyModifier)"
  - "Pattern: parseSKILL — YAML frontmatter between --- markers, struct tags for Name/Description/Agent/Reads/Writes/Temperature"

requirements-completed: []

# Environment decisions (per plan 01-01 must_haves)
docker-mode: internal-api  # NOT docker / milvus-lite / remote-milvus
glm-key-status: deferred   # Plan 04 (spike e2e) will validate via glm-4-flash free tier

# Metrics
duration: ~25min
completed: 2026-08-11
---

# Phase 1 Plan 01: Wave 0 Scaffold + Spike Wire Summary

**Wave 0 Go skeleton with subcommand router + `spike wire` proving Eino v0.7.7 + eino-ext v0.1.5 serialize temperature/response_format/thinking extras to mock OpenAI server on a dynamic port — internal API strategy, no Docker.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-08-11T03:42:44Z
- **Completed:** 2026-08-11T04:08:00Z
- **Tasks:** 2 (1 scaffold + 1 wire impl)
- **Files created:** 13 (8 in scaffold commit, 2 wire + 1 test + 1 .env.example + 1 docker-compose template in subsequent commits)
- **Commits:** 4 atomic (Wave 0 scaffold, CLI router, internal API strategy, wire impl + test)

## Accomplishments

- **Wave 0 runnable Go skeleton:** `cmd/spiked/` with subcommand router dispatching to 4 slots (wire/milvus/sse/e2e). No-args → exit 2 with usage; bogus → exit 2 with "unknown subcommand"; each subcommand → exit 0 (wire WIP, others stub returning nil).
- **SC-1 layer (a) verified:** `spike wire` exits 0 with `[OK] spike wire: temperature=0 passthrough verified`. Eino ChatModel → mock OpenAI server round-trip proves `temperature: 0`, `response_format: {type: json_object}`, and `thinking: {type: disabled}` all reach the wire through openai.WithExtraFields (Eino v0.1.13+ API).
- **4 SKILL.md fixtures per SKILL_SCHEMA.md v1.1:** authority/historical/personal/draft tiers, each with tier-matching `reads: ["<tier>"]`, `temperature: 0`, `reasoning: false`, `guardrails.refuse_on_missing_tier: true`. Sample clauses in each markdown body.
- **Internal API strategy (user checkpoint):** Milvus (内网, gRPC) + BGE-M3 (内网, REST) + GLM Coding Plan (公网). Zero Docker. `.env.example` updated to internal API placeholders; `docker-compose.yml.template` added as Phase 7 packaging artifact (not executed in spike).
- **T-1-01 secret mitigation verified:** `.env` in `.gitignore` (anchored to avoid Windows case-insensitive collisions); `git ls-files | grep .env` returns empty.

## Task Commits

Each task was committed atomically:

1. **Task 1 scaffold (part 1):** `0920584` — `feat(01-01): Wave 0 scaffold — go.mod pinned, 4 SKILL.md fixtures, CLI router`
2. **Task 1 scaffold (part 2):** `d4f084d` — `feat(01-01): CLI router + scaffold stubs + Windows-safe .gitignore`
3. **Task 1 strategy update (per user checkpoint):** `176e417` — `feat(01-01): internal API strategy — Milvus/BGE-M3 内网, GLM 公网`
4. **Task 2 wire impl + test:** `ae7dcae` — `feat(01-01): spike wire — Eino ChatModel → mock OpenAI round-trip (SC-1 layer a)`

## Files Created/Modified

- `go.mod` — Pinned: eino v0.7.7, hertz v0.10.6, milvus-sdk-go/v2 v2.4.2, eino-ext openai v0.1.5, testify v1.11.1, yaml.v3 v3.0.1
- `.env.example` — Milvus/BGE-M3 internal API placeholders + GLM public endpoint
- `.gitignore` — `.env` excluded; `/spiked`, `/spiked.exe`, `/cmd/spiked/spiked{,.exe}` anchored
- `cmd/spiked/main.go` — Subcommand router (4 slots + usage/unknown dispatch)
- `cmd/spiked/scaffold_stubs.go` — runMilvus/runSSE/runE2E stubs (return nil, replaced in Plans 02-04)
- `cmd/spiked/wire.go` — parseSKILL + startMockServer (dynamic port) + runWire (full round-trip)
- `cmd/spiked/wire_test.go` — TestSpikeWireMock (end-to-end) + TestMockServerReceivesWireExtras (wire format)
- `testdata/skills/authority/SKILL.md` — tier=权威, temperature=0, sample authority clause
- `testdata/skills/historical/SKILL.md` — tier=历史, superseded clause example
- `testdata/skills/personal/SKILL.md` — tier=个人, individual record example
- `testdata/skills/draft/SKILL.md` — tier=草稿, [DRAFT] marker
- `.planning/phases/01-integration-spike-trust-contract/docker-compose.yml.template` — Phase 7 packaging artifact (NOT executed in spike)

## Decisions Made

### 1. Internal API strategy (user checkpoint A)

**Decision:** Use internal Milvus (gRPC `milvus.internal.company.com:19530`) + internal BGE-M3 (REST `http://bge-m3.internal.company.com/v1/embeddings`) + public GLM Coding Plan (`https://open.bigmodel.cn/api/coding/paas/v4`). No Docker.

**Rationale (per user):**
- Real Milvus (full filter syntax) vs Lite subset — full tier-filter validation
- BGE-M3 stays on internal network — no data residency / cross-border concern
- LAN latency (1-5ms) more production-realistic than 0ms in-container
- Zero install time

**Impact:**
- Plans 02-04 must use `MILVUS_ADDRESS` + `BGE_M3_URL` env vars (no Docker)
- `docker-compose.yml.template` added as Phase 7 packaging artifact (OSS release)
- Plan 02 must include retry/timeout (5s dial, 3x exponential backoff 1s/2s/4s) for network resilience

### 2. eino-ext version selection (auto-fix deviation)

**Decision:** Pin eino-ext openai to tagged **v0.1.5** (not pseudo-version).

**Rationale:** eino-ext is now tagged (plan assumed pseudo-version). v0.1.5 is the highest version compatible with eino v0.7.7 (newer v0.1.13+ requires eino v0.7.13+ which would violate plan's "eino v0.7.7 EXACTLY" requirement).

### 3. GLM API key validation deferred (user checkpoint B)

**Decision:** Do not validate GLM key in Plan 01. Defer to Plan 04 (spike e2e).

**Rationale:** Plan 01 wire spike uses mock OpenAI server (per D-02). Real GLM-5.2 validation is needed only for `spike e2e` in Plan 04 Task 1, where the user has indicated they will validate via glm-4-flash free-tier curl.

### 4. Dynamic port allocation (Pitfall 7)

**Decision:** `net.Listen("tcp", "127.0.0.1:0")` for the mock server. 1ms grace period (no `time.Sleep(200ms)` like spike 001).

**Rationale:** Hardcoded :18080 may collide on shared hosts (spike 001 first run failed for this reason per temperature-field-validation.md Pitfall 7). Dynamic port + 1ms grace is the standard fix.

### 5. Project-root path resolution

**Decision:** `findProjectRoot()` walks up to `go.mod` for fixture path resolution.

**Rationale:** Tests run from `cmd/spiked/` (package directory); CLI can run from project root OR `cmd/spiked/`. The acceptance criteria explicitly tests `cd cmd/spiked && ./spiked wire` — requires CWD-independent fixture resolution.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Windows case-insensitive `.gitignore` pattern collision**
- **Found during:** Task 1 (scaffold)
- **Issue:** Pattern `spiked` in `.gitignore` matched `cmd/spiked/main.go` (and all other files in `cmd/spiked/`) because Windows filesystem is case-insensitive. `git add` rejected with "The following paths are ignored".
- **Fix:** Anchored patterns to root: `/spiked`, `/spiked.exe`, `/cmd/spiked/spiked`, `/cmd/spiked/spiked.exe`. Now only the actual binary outputs are ignored.
- **Files modified:** `.gitignore`
- **Verification:** `git check-ignore -v cmd/spiked/main.go` returns empty (not ignored); `git check-ignore -v /cmd/spiked/spiked.exe` confirms binary IS ignored.
- **Committed in:** `d4f084d` (part of CLI router commit)

**2. [Rule 3 - Blocking] eino-ext version incompatibility**
- **Found during:** Task 1 (scaffold dependency pinning)
- **Issue:** `go get github.com/cloudwego/eino-ext/components/model/openai@latest` upgraded eino from v0.7.7 → v0.7.13 (transitive dep). Plan required "eino v0.7.7 EXACTLY".
- **Fix:** Explicitly `go get github.com/cloudwego/eino@v0.7.7` to downgrade; this automatically downgraded eino-ext openai to v0.1.5 (the highest version compatible with eino v0.7.7).
- **Files modified:** `go.mod`, `go.sum`
- **Verification:** `grep "cloudwego/eino " go.mod` returns `github.com/cloudwego/eino v0.7.7`.
- **Committed in:** `0920584` (scaffold commit)

**3. [Rule 2 - Missing Critical] Test path resolution**
- **Found during:** Task 2 (wire test run)
- **Issue:** `TestSpikeWireMock` failed with "open testdata/skills/authority/SKILL.md: The system cannot find the path specified." because Go tests run from package directory (`cmd/spiked/`), not project root.
- **Fix:** Added `findProjectRoot()` helper that walks up to `go.mod`; `parseSKILL` uses it to resolve project-relative paths.
- **Files modified:** `cmd/spiked/wire.go` (added findProjectRoot, updated parseSKILL imports)
- **Verification:** `TestSpikeWireMock` passes (0.01s); `cd cmd/spiked && ./spiked.exe wire` exits 0; `./cmd/spiked/spiked.exe wire` from project root exits 0.
- **Committed in:** `ae7dcae` (wire impl commit)

**4. [Rule 3 - Blocking] Hardcoded port reference in comments**
- **Found during:** Task 2 (final acceptance check)
- **Issue:** `grep -c "18080" cmd/spiked/wire.go` returned 2 (acceptance criteria requires 0). Both instances were in comments documenting Pitfall 7.
- **Fix:** Replaced literal `:18080` references in comments with generic "(hardcoded port)" / "no hardcoded port" wording. Code itself already uses dynamic port allocation.
- **Files modified:** `cmd/spiked/wire.go` (comment text only)
- **Verification:** `grep -c "18080" cmd/spiked/wire.go` returns 0; all tests still pass.
- **Committed in:** `ae7dcae` (wire impl commit)

---

**Total deviations:** 4 auto-fixed (1 Rule 1 bug, 1 Rule 2 missing-critical, 2 Rule 3 blocking)
**Impact on plan:** All auto-fixes necessary for correctness/cross-platform compatibility. No scope creep.

## Issues Encountered

None — all issues handled as auto-fix deviations above.

## User Setup Required

**External services require manual configuration.** See `.planning/phases/01-integration-spike-trust-contract/.env.example` for:
- `MILVUS_ADDRESS` — internal Milvus gRPC endpoint (default: `milvus.internal.company.com:19530`)
- `MILVUS_TOKEN` — optional auth token (empty by default)
- `BGE_M3_URL` — internal BGE-M3 embedding service (default: `http://bge-m3.internal.company.com/v1/embeddings`)
- `BGE_M3_AUTH` — optional bearer token (empty by default)
- `GLM_API_KEY` — GLM Coding Plan key (deferred to Plan 04; needed for spike e2e only)
- `GLM_BASE_URL` — public GLM endpoint (default: `https://open.bigmodel.cn/api/coding/paas/v4`)

**Verification (Plan 02 will run):**
```bash
curl -sSf --max-time 5 -o /dev/null -w "milvus: %{http_code}\n" $MILVUS_ADDRESS/health
curl -sSf --max-time 5 -X POST $BGE_M3_URL -H "Authorization: Bearer $BGE_M3_AUTH" -d '{"input":"test"}'
```

## Spike Limitations

1. **BGE-M3 internal API:** Spike assumes internal BGE-M3 service is reachable. If service is down or unreachable, `spike milvus` (Plan 02) will fail. Plan 02 must include retry/timeout (5s dial, 3x exponential backoff 1s/2s/4s per user instructions).

2. **Milvus internal API:** Spike assumes internal Milvus cluster. Full filter syntax validated (not Lite subset). gRPC endpoint must be reachable from machine running spike.

3. **GLM API key deferred:** Plan 01 wire spike does NOT call real GLM. Real GLM-5.2 validation deferred to Plan 04 (spike e2e). Run the glm-4-flash smoke curl before executing Plan 04.

4. **docker-compose.yml.template:** This file is a Phase 7 packaging artifact, NOT executed in Phase 1. OSS users running `docker compose up` will fail (template has placeholder service configs). Phase 7 will:
   - Copy template → `docker-compose.yml` at repo root
   - Fill in healthchecks, volume mounts, network config, GPU reservations
   - Add OSS-README explaining Phase 1 internal API mode

5. **No Tier-filter E2E in Plan 01:** Plan 01 only validates SC-1 layer (a) (wire format). SC-2 (Milvus tier-filter) is in Plan 02. SC-1 layers (b) SSE wire format + (c) UIMessageChunk alignment are in Plan 03.

## Next Phase Readiness

**Plan 01-02 (spike milvus, Wave 2):**
- Reuses `cmd/spiked/scaffold_stubs.go` `runMilvus` slot (replace stub with real impl)
- Connects to `MILVUS_ADDRESS` from `.env`
- Seeds 4 tiers (≥3 docs each) + asserts `tier in ["权威"]` returns zero personal/草稿 chunks
- Must include retry/timeout per user checkpoint instructions (5s dial, 3x backoff 1s/2s/4s)
- Depends on `.planning/SKILL_SCHEMA.md` tier definitions (already established)

**Plan 01-03 (spike sse, Wave 2):**
- Reuses `cmd/spiked/scaffold_stubs.go` `runSSE` slot
- Validates Hertz `pkg/protocol/sse` → Vercel AI SDK v5 UIMessageChunk alignment
- Three layers: (a) text/event-stream wire format, (b) typed parts (reasoning/tool-calls/sources), (c) UIMessageChunk parse

**Plan 01-04 (spike e2e, Wave 2):**
- Reuses `cmd/spiked/scaffold_stubs.go` `runE2E` slot
- Validates GLM-5.2 end-to-end (deferred from Plan 01)
- 5 compliance queries × structured output (per D-12 trace log)
- 5s smoke curl with `glm-4-flash` BEFORE this plan to validate key

**Blockers / concerns:**
- Internal Milvus + BGE-M3 service reachability must be verified at Plan 02 execution time (not validated in Plan 01)
- No automated Milvus integration test in Plan 01 (out of scope; covered by Plan 02)

---
*Phase: 01-integration-spike-trust-contract*
*Plan: 01*
*Completed: 2026-08-11*
