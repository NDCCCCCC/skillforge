# Phase 30 — Threat Model (STRIDE-30)

**Phase:** 30 — 工程卫生收尾 (Hygiene & Milestone Gate)
**Authored:** 2026-09-08 (Plan 30-07)
**Status:** active — locks the trust surface for Phase 30
**Gate:** `bash scripts/verify-phase-30.sh` / `make phase-30`

---

## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| client → POST /api/feedback | Public write; per-session rate limit (WR-05, T-30-06) |
| client → POST /api/tickets | Public write; per-session rate limit + ownership校验 (WR-05, T-30-06) |
| server → SSE emit | CR-01: persisted flag prevents double-write on emit failure |
| feedback refs → store | WR-01: idempotent backfill via ON CONFLICT DO NOTHING |
| ticket dispatch → restart | WR-02: startup sweep re-dispatches queued tickets |
| webhook client → redirect | WR-03: CheckRedirect policy denies cross-host redirects |
| feedback attribution | WR-04: Action whitelist + ts tie-break prevents misbinding |
| Phase 30 → inherited surfaces | No new attack surface introduced |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation |
|-----------|----------|-----------|-------------|------------|
| T-30-01 | Tampering | CR-01: SSE emit failure double-write | mitigate | Persisted flag prevents second audit/turn write (pipeline.go + chat.go); error branch writes only if `!persisted` |
| T-30-02 | Tampering | WR-01: feedback refs backfill gap | mitigate | ON CONFLICT DO NOTHING idempotent backfill on replay (feedback.go + sqlite/feedback.go) |
| T-30-03 | Denial of Service | WR-02: queued ticket restart loss | mitigate | Startup sweep re-dispatches queued tickets from prior run (tickets.go + sqlite/tickets.go) |
| T-30-04 | Information Disclosure | WR-03: webhook redirect secret leak | mitigate | CheckRedirect policy denies cross-host redirects; max 3 redirects cap (webhook.go) |
| T-30-05 | Tampering | WR-04: attribution misbinding | mitigate | Action whitelist + ts+id tie-break prevents concurrent-request misbinding (feedback.go + audit.go) |
| T-30-06 | Denial of Service | WR-05: anonymous webhook relay abuse | mitigate | Per-session rate limit + session/turn ownership validation (tickets.go + feedback.go) |
| T-30-07 | Tampering | WR-06: late votedVerdict | mitigate | useEffect syncs server truth to lock state (FeedbackRow.tsx); server truth wins on late arrival |
| T-30-SC | Supply Chain | Documentation artifacts | N/A | No new packages introduced |

## Trust Boundaries — Phase 30 Inherited

Phase 30 introduces **zero new attack surface**. All boundaries below were established in prior phases and remain unchanged in their trust posture:

| Boundary | Established | Trust Posture |
|----------|-------------|---------------|
| client → GET /api/knowledge/graph | Phase 28 | Public read; no admin token |
| client → GET /api/knowledge/graph/neighbors | Phase 28 | Public read |
| client → GET /api/knowledge/graph/path | Phase 28 | Public read |
| client → admin endpoints | Phase 29 | admin token gate; 403 before 503 |
| SkillForge → external webhook | Phase 29 | URL from env only; body-only payload; optional secret header |
| feedback → knowledge base | Phase 29 | STRUCTURALLY FORBIDDEN (FBK-05 red line) |
| retriever → EdgeGraph | Phase 28 | Deterministic; zero LLM |

## Residual Risk

All 7 threat dispositions are **mitigate** — no **accept** or **transfer** dispositions in this phase. Phase 30 is a hardening phase; residual risk is bounded by the mitigations implemented in plans 30-01 through 30-06.

---

_Mitigation summary:_

- **T-30-01**: Persisted flag (`persisted bool`) from `writeAuditAndTurn` gates the error branch in `chatHandler`; only one audit/turn pair written per request even on SSE emit failure.
- **T-30-02**: `ON CONFLICT(feedback_id, chunk_id) DO NOTHING` makes refs backfill idempotent; replay after partial failure heals the gap.
- **T-30-03**: `SweepQueuedTickets()` runs in a background goroutine at server boot; only sweeps when webhook provider is wired (never on "none" provider).
- **T-30-04**: `CheckRedirect` returns error when `req.URL.Host != via[0].URL.Host`; custom headers (including `X-SkillForge-Ticket-Secret`) are not forwarded to cross-origin redirect targets.
- **T-30-05**: Attribution query filters `action = 'chat_verdict'` + `chunk_ids != ''` and orders `ORDER BY ts DESC, id DESC`; concurrent same-session requests cannot win the near-lookup.
- **T-30-06**: Per-session token-bucket rate limiter (`rate.Limit(10)`, burst 5) on feedback/ticket endpoints; session/turn ownership validated server-side before write.
- **T-30-07**: `useEffect([votedVerdict])` syncs server truth to local lock on every prop change; mount-time seed is overwritten by server value when part arrives late.

---

_See also:_ `docs/threat-model/STRIDE-29.md` (Phase 29 trust contract), `docs/threat-model/STRIDE-28.md` (Phase 28 graph contract), `verify-phase-30.sh` (structural gate), `.planning/phases/30-hygiene-milestone-gate/30-CONTEXT.md` (D-30-03/D-30-04 decisions).
