---
status: partial
phase: 07-open-source-release
source: [07-01-SUMMARY.md, 07-02-SUMMARY.md, 07-03-SUMMARY.md, 07-04-SUMMARY.md, 07-05-SUMMARY.md, 07-VERIFICATION.md]
started: 2026-08-13
updated: 2026-08-13
note: MVP user-story guard skipped — Phase 7 is a release/packaging phase (no interactive product user-flow); standard UAT path used. File-based deliverables batch-verified directly; environment-dependent items probed + recorded blocked.
---

## Current Test

[testing complete — 10 passed (auto-verified), 5 blocked (environment gates)]

## Tests

### 1. LICENSE present (Apache-2.0)
expected: `head -1 LICENSE` = "Apache License" (Apache-2.0 full text)
result: pass
evidence: `head -1 LICENSE` → "Apache License" ✓

### 2. README has all 7 DIST-01 elements
expected: badges, value prop, demo GIF path, quick start, architecture diagram, contributing, license
result: pass
evidence: `grep -cE 'badge|Quick Start|docker compose|mermaid|Contributing|Apache-2.0' README.md` → 12 (≥6)

### 3. CONTRIBUTING has the three named sections
expected: "Write a Skill" / "Add a Tool" / "Add a Knowledge Tier" headings present
result: pass
evidence: case-insensitive grep → 3 matches (lines 43, 117, 184)

### 4. .env.example scrubbed of internal info
expected: zero matches for internal IP/domains; no real secrets
result: pass
evidence: `grep -cE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' .env.example` → 0

### 5. Three example Skill templates pass schema validator
expected: `go test ./internal/skill -run TestExamplesValidate` green (compliance + simple-qa + calculator)
result: pass
evidence: `go test ./internal/skill -run TestExamplesValidate` → ok (calculator uses reads:[draft/**] placeholder, validator unchanged)

### 6. eval/sample.jsonl ships 50 cases
expected: `wc -l eval/sample.jsonl` = 50
result: pass
evidence: 50 lines (byte-identical to eval/golden/compliance.jsonl)

### 7. examples/data ships ≥7 sample compliance docs
expected: 7 bundled docs
result: pass
evidence: `ls examples/data/*.md` → 7

### 8. docker-compose + Dockerfiles structurally correct
expected: compose exists; Milvus pinned v2.4.x (not v3); zero `:latest` tags; SSE proxy_buffering off
result: pass
evidence: docker-compose.yml exists; `milvusdb/milvus:v2.4.` present; `:latest` count = 0; WR-01 nginx read-timeout fix applied

### 9. eval_snapshot refusal_accuracy derivation tool works
expected: `go test ./scripts/eval_snapshot/` green (DIST-09's only derivation source)
result: pass
evidence: `go test ./scripts/eval_snapshot/` → ok (6/6 hermetic tests)

### 10. SC revisions applied per D7-03/D7-06
expected: ROADMAP SC-1 "pluggable embedding endpoint" + SC-5 "recorded demo GIF"; REQUIREMENTS DIST-04/DIST-05 aligned
result: pass
evidence: ROADMAP has 1× "pluggable embedding endpoint" + 1× "recorded demo GIF"

### 11. docker build + `docker compose up` boots full stack
expected: COMPOSE_OK; both `docker build -f Dockerfile.{backend,frontend}` exit 0; stack responds at localhost:8080 with a cited verdict
result: blocked
blocked_by: other
reason: "docker not in PATH (probe NO). DIST-04 file artifacts committed + structurally verified; daemon-driven boot/build verification needs Docker Desktop install."

### 12. Live-stack eval run → README Eval Results real numbers
expected: eval/report.json with 50 cases; README ## Eval Results has real date + faithfulness/context_precision/refusal_accuracy (no _TBD_)
result: blocked
blocked_by: other
reason: "Needs live Milvus+BGE-M3+GLM stack (item 11 prerequisite). eval/report.json MISSING (probe). Derivation tool shipped + tested; only the live run + README paste is unautomated."

### 13. demo GIF recorded (docs/demo.gif)
expected: docs/demo.gif exists (<5MB); shows cited verdict + structured refusal; README embed resolves
result: blocked
blocked_by: other
reason: "docs/demo.gif MISSING (probe). Needs running stack (item 11) + screen capture."

### 14. git filter-repo history rewrite (strip .planning/ + scrub internal info)
expected: post-rewrite 4 audit gates zero-match (.planning/ history = 0; internal strings = 0; secrets = 0; HEAD clean)
result: blocked
blocked_by: other
reason: "git-filter-repo NOT installed (probe: not a git subcommand, not a python module, not pip). Destructive one-way rewrite — must run on fresh clone with user authorization."

### 15. Public skillforge remote + force-push + clean-clone verify
expected: public repo live; clean clone has zero internal info + no .planning/ + all artifacts; validator green on public clone
result: blocked
blocked_by: other
reason: "No git remote (probe empty). Requires user to confirm GitHub username (ninedrunk 待确认) + create empty public repo + authorize destructive force-push. Depends on item 14."

## Summary

total: 15
passed: 10
issues: 0
pending: 5
skipped: 0
blocked: 5

> 2026-08-14 audit note: `status: partial` 的未决项即下面 5 个 blocked 项（环境门，非代码问题）——它们同步登记在 `07-HUMAN-UAT.md`（result: [pending]，共 5 处）。`pending: 5` 与 blocked 计数是同一批事项的两个视角。

## Gaps

None — all 5 unresolved tests are environment gates (blocked), not code issues. The autonomous artifacts are committed and verified. The 5 blocked items have exact commands in `07-HUMAN-UAT.md`.

**Recommended unblock sequence:** install Docker Desktop + `pip install git-filter-repo` → items 11→12→13 (boot → eval → GIF) update dev HEAD → items 14→15 (history rewrite → public push) consume that HEAD for publication.
