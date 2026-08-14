---
phase: quick-260814
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - README.md
  - CLAUDE.md
  - CONTRIBUTING.md
autonomous: true
requirements: [NARRATIVE-DECOMPLIANCE]
must_haves:
  truths:
    - "A reader scanning README headings and the intro paragraph sees a universal platform pitch (any document-grounded domain), not a compliance product"
    - "README Quick Start walks through a generic document-grounded Q&A demo, with compliance mentioned only as one of three example Skills"
    - "No selling point, heading, or main flow sentence in README/CLAUDE.md frames compliance as the product's primary use case"
    - "compliance still appears where it is factually required (file paths, eval frozen-set name, example Skill list) — those are pointers, not narrative"
  artifacts:
    - path: "README.md"
      provides: "Platform-first narrative with generic Quick Start demo"
    - path: "CLAUDE.md"
      provides: "Project section without compliance-as-showcase framing"
  key_links:
    - from: "README.md Quick Start"
      to: "examples/skills/simple-qa/SKILL.md"
      via: "generic demo path reference"
      pattern: "simple-qa"
---

<objective>
Remove compliance as the narrative spine of the reader/developer-facing docs (README.md, CLAUDE.md, CONTRIBUTING.md). The platform story becomes "one agent kernel, any business domain"; compliance survives only as a passing mention among the three example Skills and as factual pointers (file paths, frozen eval set name).

Purpose: Commit 9569120 already demoted compliance, but README's Quick Start ("Get a cited compliance verdict running"), intro paragraph, and demo flow still use compliance as the primary walkthrough. This plan finishes the job.

Output: Three edited docs, zero code changes, zero changes under examples/skills/, .planning/ROADMAP.md, .planning/PROJECT.md.

OUT OF SCOPE (do not touch): any Go/TS source, examples/skills/**, skills/**, .planning/** except this quick dir, LICENSE, eval/**.
</objective>

<context>
@./CLAUDE.md
@./README.md
@./CONTRIBUTING.md

## Current compliance-narrative hotspots (mapped 2026-08-14)

README.md:
- L7: intro paragraph leads with "The built-in showcase is enterprise compliance / policy review (企业合规 / 制度审查)..." — rewrite to lead with universality; compliance gets at most one clause as an example.
- L19: "Get a cited compliance verdict running in your browser in under 10 minutes." — genericize.
- L53–63 "Try it": tells reader to pick the compliance Skill and ask "出差住宿费超标 200 元，能批吗？" — switch primary demo to simple-qa (see Verification of simple-qa below).
- L95: "**The compliance demo is replaceable, not the product.** ... Delete it and SkillForge has no compliance code left to remove." — good sentiment, compliance-heavy delivery; reframe as "the demo Skill is replaceable" using the compliance file only as the concrete illustration (one mention is acceptable here — it is a factual file reference).
- L103–105 (Moat 1): "so a compliance Skill physically cannot see draft or personal notes" — generalize to "so an authority-only Skill physically cannot see draft or personal notes".
- L121–122 (Skills section): "Showcase: skills/compliance/SKILL.md — the built-in compliance review Skill loaded by default." — demote to a neutral list of the three templates; do not label compliance the "Showcase".
- L128–140 (Eval Results): KEEP AS-IS. The frozen eval set is factually named `compliance` (`--frozen=compliance`, `eval/frozen/compliance.lock.json`, placeholder table). This is a factual pointer, not narrative; Plan 07-05 populates it.

CLAUDE.md (Project section only — lines ~3–14):
- L6 paragraph: "内置'企业合规/制度审查'Skill 作为样板间" and any framing of compliance as the built-in showcase — rewrite: platform adapts to any enterprise domain via SKILL.md; mention the three example templates (compliance, simple-qa, calculator) in one breath, or cite a range (legal review, HR policy, IT helpdesk).
- DO NOT touch the Technology Stack research sections (L100, L106, L164, L179 mention compliance in eval/RAG research context — factual reference material, out of narrative scope). Keep Constraints/tech-stack facts intact.

CONTRIBUTING.md (audit, expect minimal edits):
- L101 "temperature: 0.0 for compliance/data Skills" — acceptable (generic guidance), may keep.
- L113 "For the canonical reference, see skills/compliance/SKILL.md" — acceptable as a pointer; optionally neutralize to "see the Skills under examples/skills/ and skills/". Only edit if it reads as showcase framing.

## simple-qa suitability (pre-verified by planner)

examples/skills/simple-qa/SKILL.md exists and is a generic authority-tier QA template (ChatModelAgent, reads authority/**, refuses with `no_relevant_source`). It runs on the same kernel and the same docker compose stack — no extra setup. Note: the indexed knowledge base currently contains compliance-policy documents, so an honest generic demo question is document-grounded phrasing, e.g. "What does the policy say about travel expense reimbursement?" style — framed as "ask a question about the loaded documents", not as a compliance verdict. Executor MUST re-verify before writing the final wording (see Task 1 action).
</context>

<tasks>

<task type="auto">
  <name>Task 1: Rewrite README.md narrative to platform-first with a generic Quick Start</name>
  <files>README.md</files>
  <action>
Rework README.md so the narrative spine is "one kernel, any domain":

1. L7 intro: open with the universal pitch — the same kernel serves legal review, HR policy, IT helpdesk, doc-grounded QA, or any domain via a SKILL.md file. Mention the three ready-to-run templates (simple-qa, calculator, compliance) in one sentence; compliance is named only inside that list. Remove the "harshest trust test" compliance justification prose (or compress it to at most one clause).
2. Quick Start L19: replace "Get a cited compliance verdict running" with a generic framing, e.g. "Get a document-grounded, cited answer running in your browser in under 10 minutes."
3. "Try it" section (L53–63): BEFORE writing the final wording, verify the simple-qa demo path is real — read examples/skills/simple-qa/SKILL.md and confirm it needs nothing beyond the existing docker compose flow (it reads authority/**, which the default stack indexes; confirmed at planning time, re-verify). Then rewrite: pick the **simple-qa** Skill and ask a document-grounded question about the loaded documents (e.g. about the travel-expense policy content already in knowledge/). Expected outcome: a cited answer with chunk references, or a structured refusal (`no_relevant_source`) — never a fabricated answer. Keep the note that swapping the Skill in the picker (calculator, compliance) changes the behavior on the same kernel. If verification finds simple-qa would NOT work out of the box, fall back to "generic document Q&A demo" wording without inventing commands and note the discrepancy in the SUMMARY.
4. Architecture L95: reframe as "The demo Skill is replaceable, not the product" — keep the concrete illustration that all demo behavior lives in a 30-line Markdown file and can be deleted; compliance appears here only as the file path being cited.
5. Moat 1 L105: change "a compliance Skill physically cannot see" to authority-generic phrasing ("an authority-only Skill physically cannot see draft or personal notes").
6. Skills section L121–122: drop the "Showcase:" line labeling compliance as the built-in showcase; present the three templates neutrally (simple-qa, calculator, compliance — one line each at most). The link to skills/compliance/SKILL.md may remain as a factual pointer, not as "the default".
7. Eval Results section: NO changes (factual frozen-set references).
8. Keep English language, existing tone, badges, mermaid diagram (diagram node labels are fine — "cited verdict / refusal card" is already generic).

Preserve all factual content (stack, moats, eval repro command, license attribution).
  </action>
  <verify>
    <automated>grep -n -i "compliance" README.md | grep -v -E "examples/skills|skills/compliance|--frozen|eval/frozen|compliance-2026|^\| compliance" — remaining lines must each be a passing mention (example-list clause or file-path citation), none in a heading, none framing compliance as the product or default demo</automated>
  </verify>
  <done>README has no compliance-framed heading, selling point, or main-flow demo; Quick Start demo is generic (simple-qa / document Q&A); compliance appears only in the example-template list, file-path citations, and the Eval Results factual block</done>
</task>

<task type="auto">
  <name>Task 2: Fix CLAUDE.md Project narrative; audit CONTRIBUTING.md</name>
  <files>CLAUDE.md, CONTRIBUTING.md</files>
  <action>
CLAUDE.md — edit ONLY the Project section paragraph (around line 6):
- Remove "内置'企业合规/制度审查'Skill 作为样板间" showcase framing. Rewrite the closing sentences to: the platform ships three ready-to-run example Skills (simple-qa, calculator, compliance) under examples/skills/, demonstrating that the same kernel adapts to any business domain without code changes.
- Keep everything factual: tech stack, Apache-2.0, Docker Compose deployment, open-source goal, denova referential constraint. Do not touch the Technology Stack / research sections below (their compliance mentions are reference facts about eval/RAG tooling).

CONTRIBUTING.md — audit every compliance mention; expected disposition:
- File-path pointers (skills/compliance/SKILL.md, examples/skills/) and generic guidance ("temperature: 0.0 for compliance/data Skills") may stay — they are not showcase framing.
- If any line reads as "compliance is the flagship/demo Skill", neutralize it to reference the example templates collectively. Expect zero-to-one small edits here.
  </action>
  <verify>
    <automated>grep -n "合规\|compliance" CLAUDE.md CONTRIBUTING.md — CLAUDE.md hits must be confined to the Technology Stack research sections (eval/RAG facts) plus at most one example-list mention in Project; CONTRIBUTING.md hits must be file paths or generic guidance only</automated>
  </verify>
  <done>CLAUDE.md Project section pitches the universal platform with three example Skills; no doc positions compliance as the showcase/flagship; tech-stack facts untouched</done>
</task>

<task type="auto">
  <name>Task 3: Cross-document consistency audit</name>
  <files>README.md</files>
  <action>
Final pass over README.md after Tasks 1–2:
1. Read the full file top to bottom; confirm the story flows: universal pitch → generic Quick Start → architecture (domain-agnostic moats) → Skills-as-platform → eval (factual placeholder) → contributing.
2. Confirm no orphaned references: any place that still says "pick the compliance Skill", "compliance verdict" as the promised outcome, or "the built-in showcase" must already be gone from Task 1 — fix if found.
3. Confirm the three domain examples used for universality (legal review, HR policy, IT helpdesk, doc-grounded QA) are used consistently, not contradicting the Quick Start demo wording.
4. Confirm no changes leaked outside the three allowed files: git status must show only README.md, CLAUDE.md, CONTRIBUTING.md modified.
  </action>
  <verify>
    <automated>git diff --name-only | sort — must equal exactly: CLAUDE.md, CONTRIBUTING.md, README.md; plus grep -c -i "showcase" README.md returns 0</automated>
  </verify>
  <done>Only the three permitted docs are modified; README narrative is coherent end-to-end with zero "showcase" framing</done>
</task>

</tasks>

<verification>
- All three tasks' automated checks pass.
- Manual spot-check (executor, not a checkpoint): re-read README intro + Quick Start; a stranger should understand SkillForge as a universal Skill-driven agent platform and get a generic document-Q&A demo running, with compliance perceived as one swappable example among three.
</verification>

<success_criteria>
- README.md, CLAUDE.md, CONTRIBUTING.md edited; nothing else in the working tree changed.
- Compliance appears only as: (a) one item in the three-template example list, (b) factual file paths, (c) the frozen eval-set block in README Eval Results.
- Quick Start promises and demonstrates a generic document-grounded Q&A flow (simple-qa), verified against the actual example before wording is finalized.
- English language and existing formatting/tone preserved.
</success_criteria>

<output>
Create `.planning/quick/260814-tsr-remove-compliance-wording/260814-tsr-SUMMARY.md` when done
</output>
