---
phase: 1
slug: integration-spike-trust-contract
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-11
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `testify/assert` + Node `vitest` for the React stub |
| **Config file** | `go.mod` (no separate config); Node `package.json` with vitest |
| **Quick run command** | `go test ./cmd/spiked/... ./integration/contract/... -run TestSpike -count=1 -timeout 30s` |
| **Full suite command** | `docker compose up -d && go test ./... -count=1 -timeout 60s && docker compose down` |
| **Estimated runtime** | ~30s (quick, no Docker); ~60s (full with Docker); SC-5 contract test < 10s |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/spiked -run TestSpike -count=1 -timeout 30s` (no Docker required — covers SC-1, SC-3)
- **After every plan wave:** Run `go test ./... -count=1 -timeout 60s` (with `docker compose up -d` first; covers SC-2, SC-4)
- **Before `/gsd:verify-work`:** Full suite must be green + SC-5 contract test < 10s
- **Max feedback latency:** 30s for quick run; 60s for full suite

---

## Per-Success-Criterion Verification Map

> Phase 1 is a spike — no v1 REQ-IDs. Map directly to the 5 success criteria (SC-1..SC-5) from ROADMAP.md §Phase 1.

| SC ID | Plan | Wave | Success Criterion | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|-------|------|------|-------------------|------------|-----------------|-----------|-------------------|-------------|--------|
| SC-1 | 01 | 1 | Hertz SSE → React client UIMessageChunk envelope, no wire-format drift | T-1-01 (SSE chunk injection) | Hertz `WriteEvent` EOL validation splits multi-line `data:` safely | Integration (in-process Hertz + AI SDK parser) | `go test ./cmd/spiked -run TestSpikeSSE -v` | ❌ W0 | ⬜ pending |
| SC-2 | 02 | 1 | Milvus tier-filter returns ONLY tier="权威" against 4-tier seeded dataset | T-1-02 (tier filter bypass) | `allowedTiers` bound at retriever construction; no per-call filter | Integration (Milvus container) | `go test ./cmd/spiked -run TestSpikeMilvus -v` | ❌ W0 | ⬜ pending |
| SC-3 | 03 | 2 | ChatModelAgent + SequentialAgent 2-step E2E; client→agent cancel < 200ms | T-1-03 (cancellation propagation) | `context.WithTimeout` propagates through `Runner.Run` iterator | Integration | `go test ./cmd/spiked -run TestSpikeAgent -v` | ❌ W0 | ⬜ pending |
| SC-4 | 04 | 2 | "Fake cited answer" demo with hardcoded chunk citations as clickable elements | T-1-04 (React XSS via source URL) | Phase 1 stub uses `<a href={p.url}>` trusting URL — flag for Phase 4 sanitization | E2E (Hertz + React stub) | `cd web && npm run test:e2e` | ❌ W0 | ⬜ pending |
| SC-5 | 05 | 3 | Version-pinned integration test runs SC-1..SC-4 in < 10s; fails on wire/filter drift | — | Combines SC-1..SC-4 with timing assertion | Integration (smoke) | `go test ./integration/contract -count=1 -timeout 10s` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/spiked/main.go` — subcommand router (`spike wire`, `spike sse`, `spike milvus`, `spike e2e`)
- [ ] `cmd/spiked/wire.go` — mock OpenAI server + Eino ChatModel round-trip (proves Wire SC-1 layer a)
- [ ] `cmd/spiked/sse.go` — Hertz SSE writer + AI SDK v5 parser round-trip (proves SC-1 layers b+c)
- [ ] `cmd/spiked/milvus.go` — 4-tier seed + tier-filter assertion (proves SC-2)
- [ ] `cmd/spiked/e2e.go` — full pipeline trace log generator (proves SC-3, SC-4 + D-12)
- [ ] `integration/contract/contract_test.go` — version-pinned smoke test running SC-1..SC-4 in < 10s (proves SC-5)
- [ ] `testdata/skills/{authority,historical,personal,draft}/SKILL.md` — 4 fixtures
- [ ] `testdata/docs/{tier}/*.md` — ≥ 3 docs per tier (12 docs total)
- [ ] `docker-compose.yml` — milvus + bge-m3 services (with Milvus Lite fallback option)
- [ ] `web/src/App.tsx` — minimal React stub with `useChat` + citation rendering
- [ ] `web/src/__tests__/citations.test.tsx` — `@testing-library/react` test for citation clickability
- [ ] `.env.example` — `GLM_API_KEY=` placeholder (no real value)
- [ ] `go.mod` — pinned versions: eino v0.7.7, hertz v0.10.6, milvus-sdk-go-go-sdk v2.x, eino-ext pseudo-version pinned post-first-build

**Wave 0 gate:** All files above MUST exist before Wave 1 plan begins.

---

## Environment Constraints (from RESEARCH.md §Environment Availability)

| Dependency | Status | Impact on Validation |
|------------|--------|----------------------|
| Go 1.24.5 | ✓ | Tests compile + run |
| Node.js v22.17.1 | ✓ | React stub tests run |
| Docker | **✗** | **BLOCKER for SC-2, SC-4** — `docker compose up` for Milvus + BGE-M3 cannot run |
| GLM API key | unknown | Real API call tests (SC-1, SC-3) may fail with 401 if key invalid |

**Mitigation:**
- **Docker blocker:** Wave 0 MUST include a `checkpoint:human-verify` task titled "Verify Docker availability and decide Milvus deployment mode". Options surfaced to user: (a) Install Docker Desktop now, (b) Use Milvus Lite embedded binary, (c) Use remote Milvus (Zilliz Cloud free tier).
- **API key blocker:** Wave 0 MUST include `.env.example` with `GLM_API_KEY=` placeholder + a `checkpoint:human-verify` task to confirm key is valid before SC-1, SC-3 run.

---

## Manual-Only Verifications

| Behavior | SC | Why Manual | Test Instructions |
|----------|----|-----------|--------------------|
| Real GLM-5.2 API call streams response | SC-1, SC-3 | Cannot mock real network + cost | Set `GLM_API_KEY`, run `spike wire`, observe trace log shows response from `https://open.bigmodel.cn/api/coding/paas/v4` |
| Click citation in React stub opens verbatim chunk text | SC-4 | E2E browser interaction | Run `web && npm run dev`, query "test query", click citation chip, confirm chunk text panel opens with verbatim text |
| Cancellation round-trip feels responsive to human | SC-3 | Latency < 200ms is observable but needs human perception | Run `spike agent`, press Ctrl+C mid-stream, observe SSE connection closes cleanly |

*All other behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All 5 SCs have `<automated>` verify OR manual instructions OR Wave 0 dependencies
- [ ] Sampling continuity: SC-1 + SC-2 + SC-3 + SC-5 cover quick-run path; SC-4 adds full-suite path
- [ ] Wave 0 covers all MISSING references above
- [ ] No watch-mode flags in test commands (`-count=1` enforces fresh runs)
- [ ] Quick run feedback latency < 30s; full suite < 60s; contract test < 10s
- [ ] Docker blocker resolved (via checkpoint task) before SC-2 / SC-4 marked ✅
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
