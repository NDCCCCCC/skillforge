---
phase: 7
slug: open-source-release
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-13
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Phase 7 is a packaging/release phase — deliverables are declarative files (compose / Dockerfile / docs / examples). Validation = config validation + smoke build + schema validation + grep audit (no new unit-test framework).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | No new unit framework. Reuses `go test ./internal/skill` (existing validator) for DIST-08 + `docker compose config` smoke for DIST-04 + grep audits for docs. |
| **Config file** | none — reuses `internal/skill/validate_test.go`; new `internal/skill/examples_validate_test.go` for examples/ |
| **Quick run command** | `docker compose config >/dev/null && echo OK` |
| **Full suite command** | `docker compose config >/dev/null && go test ./internal/skill -run TestExamplesValidate && grep -cE 'write a Skill|add a tool|add a knowledge tier' CONTRIBUTING.md` |
| **Estimated runtime** | ~15 seconds (compose config + go test); Dockerfile smoke builds ~60-180s each |

> **Environment caveat (RESEARCH §Environment Availability):** `docker` is NOT currently in PATH on this machine, and `git-filter-repo` is not installed. Nyquist gates that invoke these tools are marked **[manual/env]** — they cannot run in the automated loop until Docker Desktop + git-filter-repo are installed. The planner must mark such tasks `autonomous: false` or gate them behind a prerequisite task.

---

## Sampling Rate

- **After every task commit:** Run the smoke/grep gate relevant to the file touched (README→README grep; compose→`docker compose config`; examples→validator test)
- **After every plan wave:** Run full suite command
- **Before `/gsd:verify-work`:** Full suite green + pre-publication gates (filter-repo dry-run + full-history grep audit zero-match + eval snapshot exists)
- **Max feedback latency:** ~15 seconds (non-Docker gates)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 07-01-01 | 01 | 1 | DIST-02 | T-7-OSS-2 | LICENSE present + Apache-2.0 full text | smoke | `grep -c 'Apache License, Version 2.0' LICENSE` | ❌ W0 | ⬜ pending |
| 07-01-02 | 01 | 1 | DIST-03 | T-7-OSS-1 | `.env.example` lists all vars + `${VAR:?}` fail-fast | smoke | `docker compose config >/dev/null` (fails on unset) + grep var list | ✅ exists, needs edit | ⬜ pending |
| 07-01-03 | 01 | 1 | DIST-04 | T-7-OSS-4 | compose config valid syntax | smoke | `docker compose config >/dev/null` | ❌ W0 | ⬜ pending |
| 07-02-01 | 02 | 1 | DIST-01 | — | README contains all required elements | smoke | `grep -cE 'badge|value prop|quick start|docker compose|architecture|contributing|license' README.md` | ❌ W0 | ⬜ pending |
| 07-02-02 | 02 | 1 | DIST-07 | — | CONTRIBUTING has the three sections | smoke | `grep -cE 'write a Skill|add a tool|add a knowledge tier' CONTRIBUTING.md` = 3 | ❌ W0 | ⬜ pending |
| 07-02-03 | 02 | 1 | DIST-05 | — | demo GIF exists | smoke | `ls docs/demo*.gif docs/*.gif 2>/dev/null` | ❌ W0 | ⬜ pending |
| 07-03-01 | 03 | 2 | DIST-08 | T-7-OSS-8 | 3 example templates pass schema validator | unit | `go test ./internal/skill -run TestExamplesValidate` | ❌ W0 (validator lib ✅) | ⬜ pending |
| 07-03-02 | 03 | 2 | DIST-06 | — | ≥7 sample docs in examples/data/ | smoke | `ls examples/data/*.md \| wc -l` ≥ 7 | ❌ W0 | ⬜ pending |
| 07-03-03 | 03 | 2 | DIST-10 | — | eval/sample.jsonl has 50 cases | smoke | `wc -l < eval/sample.jsonl` = 50 | ❌ W0 | ⬜ pending |
| 07-04-01 | 04 | 2 | DIST-09 | — | README eval table + date | smoke | `grep -E 'Faithfulness|Refusal Accuracy' README.md` + date present | ❌ W0 | ⬜ pending |
| 07-04-02 | 04 | 2 | DIST-04 | T-7-OSS-4 | Dockerfile backend smoke build [manual/env] | smoke | `docker build -f Dockerfile.backend -t sf-be .` | ❌ W0 | ⬜ pending |
| 07-04-03 | 04 | 2 | DIST-04 | T-7-OSS-4 | Dockerfile frontend smoke build [manual/env] | smoke | `docker build -f Dockerfile.frontend -t sf-fe .` | ❌ W0 | ⬜ pending |
| 07-05-01 | 05 | 3 | D7-08/09 | T-7-OSS-INFO | filter-repo dry-run strips .planning/ [manual/env] | smoke | `git filter-repo --dry-run --invert-paths --path .planning/` reports removal | n/a | ⬜ pending |
| 07-05-02 | 05 | 3 | D7-09 | T-7-OSS-INFO | full-history internal-info grep audit zero-match [manual/env] | smoke | `git grep -nE '10\.62\.10\.34|internal\.company\.com|bge-m3\.internal' $(git rev-list --all)` → 0 | n/a | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `README.md` — covers DIST-01, DIST-09
- [ ] `LICENSE` — covers DIST-02
- [ ] `.env.example` (edited: internal placeholders → compose-friendly defaults) — covers DIST-03
- [ ] `docker-compose.yml` + `Dockerfile.backend` + `Dockerfile.frontend` + `.dockerignore` — covers DIST-04
- [ ] `docs/demo.gif` (or repo root) — covers DIST-05
- [ ] `examples/data/*.md` (7 sample docs) — covers DIST-06
- [ ] `CONTRIBUTING.md` — covers DIST-07
- [ ] `examples/skills/{compliance,simple-qa,calculator}/SKILL.md` — covers DIST-08
- [ ] `internal/skill/examples_validate_test.go` (new, reuses Validate) — DIST-08 gate
- [ ] README eval section + `eval/report.json` (run `cmd/eval --frozen`) — covers DIST-09
- [ ] `eval/sample.jsonl` (copy of golden) — covers DIST-10
- [ ] Docker Desktop / docker CLI install (execution-machine prerequisite) — Nyquist gate dependency

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `docker compose up` brings full stack live on clean laptop | DIST-04 | docker not in PATH; needs real Milvus+backend+frontend boot + a sample compliance Q&A | Install Docker Desktop → `docker compose up` → wait healthchecks → open frontend → ask sample question → expect cited verdict |
| Full git history rewrite + force-push to public remote | D7-07/08/09 | destructive + remote-mutating; git-filter-repo not installed; no remote configured yet | Install git-filter-repo → run combined strip+scrub → `git grep` audit zero-match → create `ninedrunk/skillforge` remote → force-push → confirm clean clone |
| Record demo GIF | DIST-05 | requires a running stack + screen capture | After stack is live: capture a cited-verdict + refusal flow with vhs/asciinema/OBS → export ≤60s GIF → commit under docs/ |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 20s (non-Docker gates)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
