---
phase: quick-260814
plan: 01
subsystem: docs
tags: [readme, narrative, de-compliance, docs]
requires: []
provides:
  - "Platform-first README/CLAUDE.md narrative with generic document-QA Quick Start"
affects: [README.md, CLAUDE.md, CONTRIBUTING.md]
key-files:
  modified:
    - README.md
    - CLAUDE.md
    - CONTRIBUTING.md
decisions:
  - "Quick Start adds one verified `cp -r examples/skills/simple-qa skills/` step (cmd/server loads skills/ only)"
  - "CONTRIBUTING 'canonical reference' pointer neutralized to cite the three templates collectively"
metrics:
  duration: 2m
  completed: 2026-08-14
---

# Quick Task 260814-tsr: Remove compliance-as-narrative from reader-facing docs Summary

Compliance demoted from narrative spine to passing example across README/CLAUDE.md/CONTRIBUTING.md; Quick Start now demos the simple-qa Skill with a verified copy step and a document-grounded question.

## What Changed

### README.md (commit `f30d7d1`)

- **Intro (L7):** removed the "Compliance is just one Skill — not the product" bolded showcase paragraph and the "harshest trust test" justification. Now opens with the universal pitch (legal review, HR policy, IT helpdesk, document-grounded QA, any domain) and names the three templates (`simple-qa`, `calculator`, `compliance`) in one sentence.
- **Quick Start (L19):** "Get a cited compliance verdict running" → "Get a document-grounded, cited answer running in your browser in under 10 minutes."
- **Steps (L46–49):** added step 4 — `cp -r examples/skills/simple-qa skills/` — with a comment explaining the kernel loads whatever is under `skills/`. See Deviations.
- **Try it (L58–66):** primary demo is now the **simple-qa** Skill with a document-grounded question (差旅住宿费的报销标准是什么？ — travel_expense.md is in `knowledge/authority/`). Expected outcome: cited answer or `no_relevant_source` structured refusal. Calculator/compliance presented as swappable templates on the same kernel.
- **Architecture (L100):** "The compliance demo is replaceable" → "The demo Skill is replaceable"; "no compliance code left to remove" → "no domain-specific logic left to remove". `skills/compliance/SKILL.md` retained as the concrete file-path illustration (plan-permitted).
- **Moat 1 (L110):** "a compliance Skill physically cannot see" → "an authority-only Skill physically cannot see".
- **Skills section (L126–127):** dropped the "Showcase:" line. Neutral template list plus a factual "Loaded Skills" line (kernel loads `skills/`; repo default is `skills/compliance/SKILL.md`).
- **Eval Results:** unchanged (factual frozen-set references, populated by Plan 07-05).
- Badges, mermaid diagram, tone, license attribution preserved.

### CLAUDE.md (commit `2c3f7f8`)

- Project section paragraph only: removed 内置"企业合规/制度审查"Skill 作为样板间 showcase framing. Now: platform adapts to any domain (法律审查、HR 制度、IT 服务台、文档问答) via SKILL.md; three example Skills named under `examples/skills/` as the proof.
- Technology Stack research sections untouched (their compliance mentions are factual eval/RAG reference material).

### CONTRIBUTING.md (commit `2c3f7f8`)

- One edit: L113 "For the canonical reference, see skills/compliance/SKILL.md" → cites the three validated templates under `examples/skills/` plus the loaded `skills/` copy.
- L101 "temperature: 0.0 for compliance/data Skills" kept (generic guidance, plan-permitted).

## Deviations from Plan

**1. [Rule 3 - Blocking] Quick Start gained a `cp -r examples/skills/simple-qa skills/` step**
- **Found during:** Task 1 re-verification of the simple-qa demo path (mandated by the plan before finalizing wording).
- **Issue:** the plan expected "pick the simple-qa Skill" to work in the picker, but `cmd/server/main.go:146,257` loads **only `skills/`** (`skill.NewLoader("skills/")` + `skill.New("skills/", nil)`) — at repo default that directory contains only `compliance`. `examples/skills/` templates are not loaded by the running stack.
- **Fix:** added one verified copy step before `docker compose up` (Dockerfile.backend ships `skills/`; the startup scan picks it up). All three templates pass `skill.Validate` (regression test `internal/skill/examples_validate_test.go`), so the copied Skill loads cleanly. This is a real command grounded in code inspection, not an invented one — the plan's fallback clause ("without inventing commands") did not trigger.
- **Files modified:** README.md
- **Commit:** f30d7d1

## Verification

- Task 1 grep: remaining `compliance` lines in README.md (post-filter) are exactly two — the Moat 2 negation ("from a single 'compliance app' into a **platform**") and the eval-table factual row; the plan's `^\| compliance` exclusion did not strip the table row only because `grep -n` prefixes line numbers. Neither is showcase framing. `grep -c -i "showcase" README.md` = 0.
- Task 2 grep: CLAUDE.md hits confined to Technology Stack research sections + the single example-list mention in Project; CONTRIBUTING.md hits are Apache license boilerplate / generic guidance / file paths.
- Task 3: `git diff --name-only <base> HEAD | sort` = exactly `CLAUDE.md CONTRIBUTING.md README.md`; working tree clean; no orphaned "compliance verdict / pick the compliance Skill / built-in showcase" references; domain examples consistent between intro and Moat 2.
- Manual spot-check: intro + Quick Start read as a universal Skill-driven platform with a generic document-Q&A demo; compliance reads as one swappable example among three.

## Known Stubs

None. (README Eval Results table and `docs/demo.gif` remain pre-existing intentional placeholders owned by Plan 07-05's blocking checkpoints — untouched by this task, per plan.)

## Self-Check: PASSED

- Commits found: f30d7d1 (README), 2c3f7f8 (CLAUDE.md + CONTRIBUTING.md) — verified on branch `worktree-agent-aeac750d93f196556`.
- All three modified files exist and are the only tracked changes vs base `cad4e25`.
