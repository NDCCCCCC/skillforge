---
phase: 50-open-source-facade
plan: 01
status: complete
subsystem: docs
requires:
  - "ROADMAP Phase 50 (face truth goal)"
provides:
  - "README facade truth-sync (skill count, honest badges, lanes, architecture link, CGO claim)"
  - "docs/architecture.md — Mermaid four-layer map + request lifecycle, zero-fabrication path loop"
affects:
  - "README.md"
  - "docs/architecture.md"
  - ".planning/ROADMAP.md (Phase 50 wording: Dify/RAGFlow -> WeKnora/Semantica)"
tech-stack:
  - markdown
  - mermaid
key-files:
  - README.md
  - docs/architecture.md
key-decisions:
  - "Comparison lanes follow the README's current positioning (WeKnora / Semantica), not the outdated ROADMAP wording (Dify/RAGFlow)."
actuals:
  tokens: 22000
  tasks: 3
  commits: 2
---

## Summary

README facade synced to repo reality: the template claim now lists all
seven example skills by name; the decorative static-green CI badge
(`eval-gate-22-09-06-green`) was replaced by an informational badge
pointing at the real workflow file; the badge block gained explicit links
to the new Security / Code of Conduct / Contributing / Architecture
artifacts; the CGO story is stated as it is (required for the production
build). `docs/architecture.md` is new: a Mermaid four-layer map (agent
kernel / knowledge / governance / platform), a five-step request
lifecycle, and a zero-fabrication loop asserting every backticked repo
path in the doc exists. ROADMAP's Phase 50 wording was aligned from
Dify/RAGFlow to the WeKnora/Semantica lanes (the historical Source line
was kept verbatim).

The comparison table itself pre-existed and was already accurate — only
the architecture link was added.

## Commits

- `5b64efa` docs(50-01): README truth-sync + architecture doc + ROADMAP lanes
- `a5bdbfb` docs(50-01): README template count — three -> seven (residue)
