---
phase: 50-open-source-facade
plan: 02
status: complete
subsystem: docs, community
requires:
  - "50-CONTEXT.md D-50-03 (SECURITY.md distilled from docs/threat-model)"
  - "50-CONTEXT.md D-50-04 (minimal template set)"
provides:
  - "SECURITY.md (STRIDE distillation + private reporting + 7-day response)"
  - "CODE_OF_CONDUCT.md (Contributor Covenant 2.1)"
  - ".github/ISSUE_TEMPLATE/{bug_report,feature_request,config}.yml + PULL_REQUEST_TEMPLATE.md"
  - "README '## Community & Security' block"
affects:
  - "SECURITY.md"
  - "CODE_OF_CONDUCT.md"
  - ".github/ISSUE_TEMPLATE/"
  - ".github/PULL_REQUEST_TEMPLATE.md"
  - "README.md"
tech-stack:
  - markdown
  - yaml
key-files:
  - SECURITY.md
  - CODE_OF_CONDUCT.md
key-decisions:
  - "SECURITY.md scope: documented stub behavior and demo corpus are out-of-scope; secrets hygiene is in-scope."
  - "Templates must never elicit secrets (bug template carries a redaction warning)."
actuals:
  tokens: 18000
  tasks: 3
  commits: 1
---
## Accomplishments
- SECURITY.md distils the STRIDE register into four design stances with a private-vulnerability reporting path and a 7-day response commitment; CODE_OF_CONDUCT.md adopts Contributor Covenant 2.1.
- Four community templates landed (.github ISSUE_TEMPLATE bug/feature + config + PULL_REQUEST_TEMPLATE) with a redaction warning in the bug form, and README gained a Community & Security block linking all of them.


## Summary

The community infrastructure that was missing entirely is now in place:
`SECURITY.md` distills the existing STRIDE threat model into the four
design stances, documents GitHub private vulnerability reporting with a
7-day response commitment, and draws the scope line (documented stub
behavior and the demo corpus are out of scope; secrets hygiene is in
scope). `CODE_OF_CONDUCT.md` adopts Contributor Covenant 2.1. The issue
templates (bug form with a redaction warning, feature form, config with
blank issues disabled and security contact pointing at the private
channel) and the pull-request template (What/Why/How-tested plus a
no-secrets checklist) are the minimal honest set. README links all four
from a new `## Community & Security` block.

## Commits

- `e637d0c` docs(50-02): community infrastructure (security/coc/templates/README block)
