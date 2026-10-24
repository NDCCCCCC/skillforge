---
phase: "51"
slug: "release-engineering"
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-10-24"
---

# Nyquist Validation — Phase 51

## Test Infrastructure

| Layer | Command | Notes |
| --- | --- | --- |
| gate | `bash verify-phase-51.sh` | 5 partitions + 4 mutation drills |
| goreleaser | `goreleaser check` (WSL) | config schema |
| snapshot | `goreleaser build --snapshot --single-target` (WSL) | linux/amd64 ELF |
| stub smoke | snapshot binary + real embedding endpoint | /api/skills 200, data-policy `"mode":"stub"` |

## Sampling Rate

| When | What runs |
| --- | --- |
| Per task commit | plan-specified verify commands |
| Phase close | full `verify-phase-51.sh` (where applicable) + go test / vitest |

## Per-Task Verification Map

| Task | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 51-W0-01 | 51-01 | 1 | REL-03 | T-51-01.. | packaging | automated | `goreleaser check` (WSL) | ✅ | PASSING |
| 51-W0-02 | 51-01 | 1 | REL-03 | T-51-01.. | snapshot build | automated | `goreleaser build --snapshot --single-target` | ✅ | PASSING |
| 51-W0-03 | 51-01 | 1 | REL-03 | T-51-05 | stub smoke of packaged binary | automated | run binary -> GET /api/skills == 200 | ✅ | PASSING |
| 51-W0-04 | 51-02 | 2 | REL-03 | T-51-06.. | workflow structure/permissions | automated | `bash verify-phase-51.sh` (P-WORKFLOW) | ✅ | PASSING |
| 51-W0-05 | 51-03 | 3 | REL-03 | T-51-04 | gate drills bite | automated | `bash verify-phase-51.sh` (P-SELF, D1..D4 red) | ✅ | PASSING |

## Wave 0 Requirements

All Wave 0 gaps were closed during execution (see plans' SUMMARY files).

## Manual-Only Verifications

- **H-51-1/H-51-2**: the real tag push -> GitHub runners -> Release page -> download-and-run loop (ROADMAP SC1/SC2/SC3 remote leg). One command (`git tag v1.7.0 && git push --tags`) triggers the locally-verified workflow; recording the resulting Release run URL closes these.


## Validation Sign-Off

- [x] Every Wave 0 gap is either closed or explicitly deferred
- [x] Every requirement maps to at least one automated command or a documented manual-only entry
- [x] Sampling rate defined for phase close

Approval: orchestrator-audited (2026-10-24) — documentation/CI-only content; all automated commands were executed in-session and their outputs recorded in the phase VERIFICATION.md.

