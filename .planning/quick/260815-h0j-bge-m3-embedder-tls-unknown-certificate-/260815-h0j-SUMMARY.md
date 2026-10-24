---
quick: 260815-h0j
type: execute
title: "BGE-M3 embedder TLS unknown certificate — configurable InsecureSkipVerify"
subsystem: embedder
tags: [go, bge, tls, internal-network, tdd]

# Dependency graph
requires: []
provides:
  - "BGE embedder accepts Config.InsecureSkipVerify for internal-network self-signed HTTPS"
  - "cmd/server wires BGE_M3_TLS_SKIP_VERIFY env var with operator-visible [WARN] log"
  - "Per-instance *http.Client construction (http.DefaultClient no longer shared)"
affects: [internal-network-deployments, bge-embedder, cmd-server-startup-log]

# Tech tracking
tech-stack:
  added: []           # stdlib only — crypto/tls + net/http; no new dependencies
  patterns:
    - "Per-instance *http.Client with http.Transport{TLSClientConfig} for TLS posture isolation"
    - "Operator opt-in via env var (case-insensitive EqualFold) + startup [WARN] log so the insecure posture is unmistakable"
    - "nolint:gosec on the InsecureSkipVerify literal — operator opt-in, not a default posture"

key-files:
  created:
    - internal/embedder/bge/bge_test.go
  modified:
    - internal/embedder/bge/bge.go
    - cmd/server/main.go

key-decisions:
  - "Default-false InsecureSkipVerify preserves the production safe posture; the field is purely additive so existing callers compile unchanged"
  - "Per-instance *http.Client (BGE.client) replaces http.DefaultClient; eliminates cross-package coupling on the global transport and isolates TLS posture per BGE"
  - "Operator-visible [WARN] log at startup is unconditional whenever BGE_M3_TLS_SKIP_VERIFY=true; the URL is interpolated so the operator sees WHICH endpoint is unverified"
  - "//nolint:gosec on the InsecureSkipVerify literal (G402) — operator opt-in via env var, not a default posture"

patterns-established:
  - "Pattern: per-instance http.Client construction in embedder-style wrappers — never share http.DefaultClient for client-payload TLS work"
  - "Pattern: opt-in security-relevant env vars (EqualFold true mapping) + unconditional [WARN] log on enabled — preserves the production safe default while making the insecure state unmistakable"

# Metrics
duration: ~3 min
completed: 2026-08-15
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: unknown
---

# Quick 260815-h0j: BGE-M3 embedder TLS unknown certificate — Summary

**Configurable InsecureSkipVerify for the BGE-M3 HTTP embedder (default-false; BGE_M3_TLS_SKIP_VERIFY env opt-in) + operator-visible [WARN] log + per-instance http.Client so http.DefaultClient is no longer shared.**

## Performance

- **Started:** 2026-08-15T12:19:00Z (approx)
- **Completed:** 2026-08-15T12:21:36Z
- **Duration:** ~3 min
- **Tasks:** 3/3 (RED + GREEN + wiring)
- **Files modified:** 3 (1 created, 2 modified)
- **Commits:** 3 (test + feat + feat) — TDD discipline visible: `d564e8e` RED → `47b45d8` GREEN → `896768b` wiring

## Accomplishments

- `Config.InsecureSkipVerify bool` field (append-only, default-false; production safe).
- Per-instance `*http.Client` constructed in `New(cfg)` with `http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}}` under `//nolint:gosec` (G402 — operator opt-in, not a default posture).
- `http.DefaultClient.Do(req)` call site GONE from the package; `embedOnce` now takes `client *http.Client` as its first arg and the retry loop in `Embed` passes `b.client`.
- `BGE_M3_TLS_SKIP_VERIFY` env var read in `cmd/server/main.go` via `strings.EqualFold(os.Getenv(...), "true")` (matches the existing `useAgentPath` parse style at line 153) and threaded into `bge.Config{InsecureSkipVerify: bgeTLSSkipVerify, ...}`.
- Operator-visible `[WARN] cmd/server: BGE_M3_TLS_SKIP_VERIFY=true — TLS certificate verification is DISABLED for %s (internal-network self-signed HTTPS only; NEVER use in production)` log emitted at startup when skip is true; the URL is interpolated so the operator sees WHICH endpoint is unverified. Silent default when false.
- Env-var doc comment in the `cmd/server/main.go` header (around line 52) describes the new variable alongside `BGE_M3_URL`.
- 3/3 bge tests PASS (`TestEmbed_TLS_InsecureSkipVerify_False` + `TestEmbed_TLS_InsecureSkipVerify_True` + `TestEmbed_HTTP_NoTLS_Regression`); `go vet ./internal/embedder/bge/...` clean; `go build ./...` clean.

## Task Commits

Each task was committed atomically (TDD discipline: test first, then impl, then wiring):

1. **Task 1: RED — failing TLS tests against httptest.NewTLSServer** - `d564e8e` (test)
2. **Task 2: GREEN — Config.InsecureSkipVerify + b.client swap** - `47b45d8` (feat)
3. **Task 3: env wiring + WARN log in cmd/server/main.go** - `896768b` (feat)

## Test Evidence (RED → GREEN)

### RED (`d564e8e`)

The test file references `Config.InsecureSkipVerify`, which does not exist yet. The Go compiler reports the missing field at both call sites — a compile-time RED that names the missing contract exactly:

```
internal\embedder\bge\bge_test.go:55:31: unknown field InsecureSkipVerify
  in struct literal of type Config
internal\embedder\bge\bge_test.go:78:31: unknown field InsecureSkipVerify
  in struct literal of type Config
FAIL  github.com/skillforge/skillforge/internal/embedder/bge [build failed]
```

### GREEN (`47b45d8`)

All 3 tests pass after the implementation. Note: the plan's verify step says "TestEmbed_TLS_InsecureSkipVerify_False still fails (this is correct — we want it to fail, that proves the config is being honored)" — that wording is misleading. The test ASSERTS the expected `x509`/`certificate` error and PASSES when the embedder correctly rejects the self-signed cert. That IS the correct green outcome for the verify-on default. The matrix below is the truth:

| Test | Config | Server | Expected | Result |
|------|--------|--------|----------|--------|
| `TestEmbed_TLS_InsecureSkipVerify_False` | `InsecureSkipVerify: false` | httptest.NewTLSServer (self-signed) | x509/certificate error (verify-on rejects) | **PASS** (default posture safe) |
| `TestEmbed_TLS_InsecureSkipVerify_True` | `InsecureSkipVerify: true` | httptest.NewTLSServer (self-signed) | 1024-dim vector (opt-in succeeds) | **PASS** (operator-facing opt-in works) |
| `TestEmbed_HTTP_NoTLS_Regression` | (TLS unset) | httptest.NewServer (plain HTTP) | 1024-dim vector (no regression) | **PASS** (common case unaffected) |

```
=== RUN   TestEmbed_TLS_InsecureSkipVerify_False
--- PASS: TestEmbed_TLS_InsecureSkipVerify_False (2.10s)
=== RUN   TestEmbed_TLS_InsecureSkipVerify_True
--- PASS: TestEmbed_TLS_InsecureSkipVerify_True (0.01s)
=== RUN   TestEmbed_HTTP_NoTLS_Regression
--- PASS: TestEmbed_HTTP_NoTLS_Regression (0.00s)
PASS
ok  	github.com/skillforge/skillforge/internal/embedder/bge	3.353s
```

## Files Created/Modified

- `internal/embedder/bge/bge_test.go` *(created, 104 lines)* — 3 TDD tests: TLS-false x509 guard, TLS-true opt-in success, HTTP regression. Uses `package bge` (not `bge_test`) for symmetric access to `embedOnce`; uses a shared `makeOpenAIShapeHandler()` factory that emits 1024-dim OpenAI-shape responses.
- `internal/embedder/bge/bge.go` *(modified, +30/-6)* — added `crypto/tls` import; extended `Config` with `InsecureSkipVerify bool`; extended `BGE` with `client *http.Client`; rewrote `New(cfg)` to construct a dedicated `*http.Client` with `http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}}` under `//nolint:gosec`; updated `embedOnce` signature to take `client *http.Client` as first arg; replaced the `http.DefaultClient.Do(req)` call site with `client.Do(req)`; updated the BGE struct doc to note the per-instance TLS posture isolation.
- `cmd/server/main.go` *(modified, +13/-1)* — added the `BGE_M3_TLS_SKIP_VERIFY` env-var doc line; parsed `bgeTLSSkipVerify := strings.EqualFold(os.Getenv("BGE_M3_TLS_SKIP_VERIFY"), "true")` next to the other BGE env reads; passed `InsecureSkipVerify: bgeTLSSkipVerify` to `bge.New`; emitted a `[WARN]` log line at startup with the unverified URL interpolated when skip is true.

## Decisions Made

- **Default-false `InsecureSkipVerify`** preserves the production safe posture. The field is purely additive (no callers change when unset), so existing operators see zero behavioral change.
- **Per-instance `*http.Client`** in `BGE.client` replaces the shared `http.DefaultClient`. This eliminates cross-package coupling on the global transport and isolates the TLS posture per BGE — a deliberate hardening beyond the plan's "make it work" minimum.
- **Unconditional `[WARN]` log on enabled** (not conditional on a verbosity flag). The plan says "operator-visible signal" — that signal must fire whenever the insecure mode is active, with no chance of being suppressed by log-level tuning.
- **`//nolint:gosec` on the `InsecureSkipVerify` literal** is the right comment style. G402 is a known gosec lint hit; the comment justifies it as an operator opt-in (env-var gated, default off, startup-log announced), not a default posture.

## Operator-Facing Env Var

| Var | Required | Default | Effect |
|-----|----------|---------|--------|
| `BGE_M3_TLS_SKIP_VERIFY` | optional | unset (verify-on) | `"true"` (case-insensitive) disables TLS cert verification for the BGE-M3 endpoint. Internal-network self-signed HTTPS ONLY; logs `[WARN] cmd/server: BGE_M3_TLS_SKIP_VERIFY=true — TLS certificate verification is DISABLED for <URL> (internal-network self-signed HTTPS only; NEVER use in production)` at startup. Any other value (including empty / unset) keeps verify-on. |

## Deviations from Plan

**None — plan executed exactly as written.**

The only minor adjustment was the RED-vs-GREEN interpretation: the plan's verify step said the TLS-false test "still fails (this is correct)". The test is an assertion-based test that PASSES when the embedder correctly rejects the self-signed cert — which is the correct green outcome. The "still fails" wording in the plan describes runtime behavior (the embedder fails to connect) not test-suite status. Both readings are consistent: the test passes when the embedder correctly fails, which IS the contract under test.

## Issues Encountered

None. All 3 commits landed cleanly. `go test ./internal/embedder/bge/... -v` → 3/3 PASS, `go vet` clean, `go build ./...` clean.

## Grep Gate Verification

| Gate | Status |
|------|--------|
| `grep InsecureSkipVerify internal/embedder/bge/bge.go` | PASS (5 mentions: doc + Config field + struct field + New + transport) |
| `grep http.DefaultClient.Do internal/embedder/bge/bge.go` | PASS (0 active calls; only documentation references explaining the deliberate non-use) |
| `grep BGE_M3_TLS_SKIP_VERIFY cmd/server/main.go` | PASS (4 mentions: doc header + read + WARN log + comment) |
| `grep InsecureSkipVerify: cmd/server/main.go` | PASS (1 mention: passed to `bge.New` at line 171) |
| `grep \[WARN\] cmd/server/main.go` | PASS (4 mentions: the new TLS line + 3 pre-existing) |
| No diff outside the three declared `files_modified` | PASS (`git diff --stat e60346e HEAD` → exactly 3 files: `cmd/server/main.go`, `internal/embedder/bge/bge.go`, `internal/embedder/bge/bge_test.go`) |
| Zero touches to `internal/wire/*`, `internal/skill/*`, `internal/server/chat*`, `internal/server/pipeline*` | PASS (no related files changed) |

## Next Phase Readiness

- Internal-network HTTPS BGE-M3 deployments (e.g., `https://10.62.10.33:63803` behind a self-signed reverse proxy) now work with `BGE_M3_TLS_SKIP_VERIFY=true`.
- Production deployments (no env var) continue to verify TLS — zero behavioral change for current operators.
- The pattern (per-instance http.Client + opt-in env var + unconditional [WARN] log) is reusable for any future outbound-HTTP client that may need to talk to internal self-signed endpoints (e.g., a MongoDB-over-TLS or Milvus-over-TLS shim, if internal-network TLS gets rolled out beyond the BGE-M3 endpoint).
- No follow-up required for this quick task.

## Self-Check: PASSED

All claimed artifacts verified on disk and in git history:

- `internal/embedder/bge/bge_test.go` — FOUND (3 test functions, 104 lines)
- `internal/embedder/bge/bge.go` — FOUND (modified, +30/-6)
- `cmd/server/main.go` — FOUND (modified, +13/-1)
- `.planning/quick/260815-h0j-bge-m3-embedder-tls-unknown-certificate-/260815-h0j-SUMMARY.md` — FOUND (untracked, per constraint)
- `d564e8e` (RED test) — FOUND in git log
- `47b45d8` (GREEN impl) — FOUND in git log
- `896768b` (wiring) — FOUND in git log
- TDD gate sequence: test commit precedes feat commit precedes feat commit (RED → GREEN → wiring) ✓
- `go test ./internal/embedder/bge/... -v` → 3/3 PASS
- `go vet ./internal/embedder/bge/...` → clean
- `go vet ./cmd/server/...` → clean
- `go build ./...` → clean
- Grep gates: InsecureSkipVerify present, http.DefaultClient.Do absent (0 active calls), BGE_M3_TLS_SKIP_VERIFY present, InsecureSkipVerify: present in main.go, [WARN] present in main.go

---

*Quick: 260815-h0j*
*Completed: 2026-08-15*
