---
phase: 18
slug: skill-p10-4
status: verified
threats_open: 0
asvs_level: 1
created: 2026-08-29
---

# Phase 18 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| HITL tool boundary → operator | report_publish tool crosses agent runtime to human operator | Frozen tool args (period, title, template hash, KPI snapshot) |
| Operator → report file system | Approved report written to reports/<period>/v<N>.html | HTML content (sanitized via goldmark + bluemonday) |
| Static file serving | GET /reports/* serves published HTML to operator browser | HTML file read from disk |
| DB audit trail | reports table INSERT after successful file write | Report metadata (period, version, title, approval audit ID) |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-18-B1 | Information Disclosure | GET /reports/*filepath | mitigate | `reportPathAllowlist` regex `^[A-Za-z0-9_-]+/v[0-9]+\.html$` — rejects `../`, `%2e%2e`, absolute paths; 404 on non-match (server.go:78,93) | closed |
| T-18-B2 | Tampering | reports/<period>/vN.html | mitigate | `getNextVersion()` scans directory for MAX(N)+1 — every publish creates a new immutable version, no overwrite (publish.go:198) | closed |
| T-18-B3 | Repudiation | reports table INSERT failure | accept | DB error logged; file already on disk; caller retries — audit row may lag but file is durable (18-02 SUMMARY.md) | closed |
| T-18-C1 | Information Disclosure | HITL tool args (period field) | mitigate | `IsValidPeriod()` regex `^[A-Za-z0-9_-]+$` enforced at HITL Pass-1 before interrupt — reject traversal/absolute/drive forms at write boundary (publish.go:41,46) | closed |
| T-18-C2 | Spoofing / Repudiation | HITL approve/deny/request_change | mitigate | Two-pass contract: InsertPendingApproval (Pass-1) → StatefulInterrupt → ApprovalDecision injection (Pass-2) — frozen args byte-equality invariant (D11-C4) via `tool.GetInterruptState` / `tool.GetResumeContext` (hitl_report_publish.go:182-294) | closed |
| T-18-C3 | Information Disclosure | PrevRunContext injection | mitigate | `PrevRunContext` struct carries only metadata (period, template version, prev version, mod notes) — no raw KPI data, no skill credentials (factory.go, deep_agent.go:87-89) | closed |

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| T-18-B3 | T-18-B3 | DB INSERT failure after file write: file is durable on disk; audit row may lag but operator can retry; no data loss — only audit completeness affected | Phase 18 execution | 2026-08-28 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-29 | 6 | 6 | 0 | gsd-secure-phase |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-29
