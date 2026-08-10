# PITFALLS.md — SkillForge (Skill-Driven Enterprise Agent Platform)

**Domain:** Skill-driven enterprise Agent / Agentic RAG platform
**Demo Skill:** 企业合规 / 制度审查 (compliance review)
**Researched:** 2026-08-10
**Confidence:** MEDIUM-HIGH (mix of official Anthropic/OWASP/Eino guidance + WebSearch verification; gaps flagged)

---

## 0. Cross-Cutting Critical Pitfalls (Read First)

Three pitfalls are **show-stoppers for this project specifically** because they directly attack the demo Skill's value proposition (可信、合规、不臆测):

1. **P-COMPL-1: Compliance Hallucination with Fake Citations** — agent cites clauses that don't exist in the authoritative doc. Highest legal/financial risk.
2. **P-RAG-2: Knowledge Tier Bleed** — agent pulls 个人笔记 / 历史观点 to answer 权威 question, producing plausible-but-wrong output.
3. **P-EVAL-1: No Measurable Eval Pipeline** — ship without RAGAS/faithfulness metrics = no way to prove "we reduce hallucination", killing the demo's story.

Everything else is secondary to these three.

---

## 1. Skill Design Pitfalls (SKILL.md 6-field spec)

### P-SKILL-1: Over-specified Skill (brittle, low reuse)
**What goes wrong:** SKILL.md hardcodes filenames, regex patterns, exact prompts, or step-by-step control flow that works for one doc set but breaks on the next enterprise's data.
**Why:** Author confuses "explicit" with "brittle". 6-field spec invites copy-paste step lists.
**Warning signs:**
- `steps` field reads like imperative code (`if doc contains "Article 5.2" then...`)
- Changing the doc structure breaks the skill (manual test fails)
- New enterprise needs >30 min to adapt
**Prevention:**
- Keep `steps` declarative (intent, not control flow); let the agent decide tool order
- Reference data via `reads` patterns, not literal filenames
- Add a "skill self-test" command in the Skill loader that runs 3 example queries
- Use Anthropic Skills guidance: description states *when/how*, body stays general
**Phase:** Phase 2 (Skill system design) — make the spec enforce this via lint

### P-SKILL-2: Under-specified Skill (loses guardrails)
**What goes wrong:** Skill is too vague, agent uses personal knowledge or refuses valid queries because it can't tell what's in scope.
**Why:** Over-correction from P-SKILL-1; author writes "answer questions helpfully" without boundaries.
**Warning signs:**
- Eval shows agent answering 权威 questions from general knowledge
- Refusal rate on in-scope queries >20%
- `reads`/`writes` empty or trivial
**Prevention:**
- Make `guardrails` field required (not optional)
- Require ≥3 negative examples in the Skill spec (queries that must be refused)
- Default-deny: if `reads` is empty, agent must refuse, not improvise
**Phase:** Phase 2 — enforce in `SKILL.md` schema validator

### P-SKILL-3: Over-permissive reads/writes (knowledge bleed)
**What goes wrong:** Skill declares `reads: ["*"]` or omits the field; agent retrieves from any vector store, including user notes / drafts / outdated versions.
**Why:** Convenience — author wants the skill to "just work". Mirrors OWASP LLM08 (Excessive Agency).
**Warning signs:**
- Audit log shows retrievals from non-`reads` stores
- `description` includes "all documents" / "any data"
- Skill works equally well across unrelated doc sets (smell: zero specificity)
**Prevention:**
- `reads` is a whitelist of tier+path patterns (`authority/policies/**`); reject empty/wildcard at load time
- Tool layer (not prompt layer) enforces: skill's retriever literally cannot query outside its declared tier
- Map to Anthropic's "narrow tool definitions" principle from agentic misalignment research
**Phase:** Phase 2 (Skill spec) + Phase 4 (RAG tier enforcement at tool layer)

### P-SKILL-4: Over-restrictive Skill (no real capability)
**What goes wrong:** Skill refuses too aggressively; reads only one doc; can't handle follow-ups.
**Why:** Over-correction from P-SKILL-3; author writes Skill that handles 1 query and ships it.
**Warning signs:**
- Eval shows refusal rate >40% on in-scope queries
- No multi-hop or follow-up handling
- Skill cannot answer 50% of plausible questions in its own domain
**Prevention:**
- Eval set must include ≥10 "happy path" queries per Skill before shipping
- `guardrails` field distinguishes "must refuse" from "must answer"
- Track refusal rate as a KPI, not a safety signal alone
**Phase:** Phase 5 (Eval pipeline) — automated refusal tracking

### P-SKILL-5: Vague description (bad intent routing)
**What goes wrong:** Two Skills with similar `description`; agent routes to wrong one, or routes to none ("I'll just answer from general knowledge"). Real bug: GitHub Issue #10768 (Anthropic Skills System NL intent matching broken).
**Why:** `description` field used as the routing signal. Generic descriptions collide.
**Warning signs:**
- Eval shows >10% routing errors
- Two Skills overlap in description keywords
- Skill never invoked even when query is in-scope
**Prevention:**
- `description` must include (a) trigger phrases (b) negative scope (c) tier/source keywords
- Routing test: write 5 queries that should hit each Skill, 5 that should miss — fail CI if miss > 2
- Consider semantic embedding-based router for top-K (not just keyword); fall back to LLM pick only when confidence < threshold
**Phase:** Phase 2 (router) + Phase 5 (eval includes routing accuracy)

### P-SKILL-6: Missing or weak guardrails field
**What goes wrong:** `guardrails` is one vague line ("be careful") instead of structured constraints; agent silently ignores.
**Why:** Authors treat it as documentation, not enforcement.
**Warning signs:**
- No structured refusal examples in `guardrails`
- Agent produces answers that contradict guardrail text
- No automated check that guardrail phrases appear in agent's refusal messages
**Prevention:**
- Define `guardrails` schema: `{prohibitions: [...], required_refusal_phrases: [...], tier_limits: [...]}`
- Run guardrail assertions in CI eval
**Phase:** Phase 2 (schema) + Phase 5 (CI eval)

---

## 2. True-Source Separation (Tiered RAG) Pitfalls

### P-RAG-1: Mixing knowledge tiers (knowledge bleed)
**What goes wrong:** Vector store flattens 权威 / 历史 / 个人 / 草稿 into one index; agent retrieves from any tier to answer any question.
**Why:** Single embedding model + single collection = no tier awareness. Common RAG antipattern.
**Warning signs:**
- Eval query like "what is current policy on X" returns content from `personal/notes/` or `archive/2023/`
- Citation tier not displayed to user
- Retrieval score doesn't include tier weight
**Prevention:**
- Separate vector indexes per tier; metadata `tier` is mandatory on every chunk
- Retrieval pipeline: filter by tier BEFORE similarity search
- Per-Skill `reads` maps to a tier allowlist (ties back to P-SKILL-3)
- Surface tier in UI citation: `[权威]` vs `[个人笔记]`
**Phase:** Phase 4 (RAG pipeline) — must be designed in, not bolted on

### P-RAG-2: Missing tier in retrieval (silent hallucination)
**What goes wrong:** Query targets 权威 tier but retrieved chunks have no `tier` metadata; agent doesn't know to refuse.
**Why:** Indexing pipeline dropped metadata; or new docs indexed without tier tag.
**Warning signs:**
- `tier IS NULL` ratio > 1% in production retrieval logs
- Agent answers confidently when no tier-matched docs found (no refusal)
**Prevention:**
- Tier is required at index time (insert fails if tier absent)
- Agent must emit "no tier-matched context" signal when retrieval is empty; UI shows refusal
- Denova-style: each Skill declares tier priority order
**Phase:** Phase 4 + monitoring in Phase 7 (production)

### P-RAG-3: Over-trusting vector similarity
**What goes wrong:** Top-K chunks returned by cosine similarity look topically related but are wrong (different article, outdated clause, off-topic memo).
**Why:** Embedding models conflate lexical/topic similarity with semantic correctness. Worse with Chinese legal/financial terms that share many characters.
**Warning signs:**
- Manual review: top-1 chunk is wrong >15% of the time
- Citation text doesn't match retrieved chunk text exactly
- Agent cites a "right-sounding" clause that isn't actually relevant
**Prevention:**
- Hybrid retrieval: BM25 + dense embedding (Chinese text especially benefits)
- Re-rank with cross-encoder; require top chunk score ≥ threshold (else refuse)
- Use CRAG-style corrective grading: agent must classify each chunk as `supports / irrelevant / contradicts`
- For compliance: keyword-must-match layer (e.g., clause number must appear in retrieved chunk)
**Phase:** Phase 4 (retrieval) + Phase 5 (eval includes retrieval precision)

### P-RAG-4: No fallback path when retrieval is weak
**What goes wrong:** Retrieved docs are weak/empty; agent still answers (fabricates) instead of asking clarifying question or refusing.
**Why:** Prompt says "answer helpfully"; no explicit refusal training in system prompt.
**Warning signs:**
- Eval shows answers produced when `top_score < 0.5`
- User can probe with adversarial queries and get confident-but-wrong answers
**Prevention:**
- System prompt must include: "If top retrieval score < threshold OR tier doesn't match, refuse with structured refusal message"
- Eval set must include ≥5 "no good retrieval" cases; passing requires correct refusal
**Phase:** Phase 4 + Phase 5

---

## 3. Agentic RAG Pitfalls

### P-AGENTIC-1: Over-retrieval (token waste, noise)
**What goes wrong:** Agent loops, retrieving the same chunks repeatedly; conversation context bloats with duplicate retrieved docs; answer quality drops.
**Why:** No deduplication; no retrieval budget; agent doesn't track what it has.
**Warning signs:**
- Avg retrievals per query > 3
- Same chunk ID appears >2 times in conversation history
- Token cost per query grows linearly with conversation length
**Prevention:**
- Hard cap: max N retrievals per query (e.g., 3)
- Dedup by chunk ID in conversation state
- Agent state machine: retrieve → assess → either retrieve-more-on-different-angle or answer
- Track retrievals-per-query as a metric
**Phase:** Phase 4 (Agentic RAG state) + Phase 5 (metric)

### P-AGENTIC-2: Refusal too aggressive (bad UX)
**What goes wrong:** Agent refuses valid queries because guardrails are too tight or thresholds too high.
**Why:** Hallucination fear → over-conservative prompts.
**Warning signs:**
- User reports "the bot never answers"
- Eval shows <60% answer rate on in-scope queries
- Help desk / FAQ tier queries refused
**Prevention:**
- Two-tier refusal: "I don't know" (genuine missing info) vs "I won't answer" (out-of-scope) — different UX
- Tune thresholds against eval set; refusal rate is a tracked KPI
- Allow "partial answer" with explicit uncertainty markers when only some aspects are supported
**Phase:** Phase 5 (tune via eval) + Phase 6 (UX polish)

### P-AGENTIC-3: Missing self-check loop (hallucination slip)
**What goes wrong:** Agent answers without verifying its own answer against retrieved context. Common in single-shot RAG.
**Why:** No "critique → revise" step in the agent loop.
**Warning signs:**
- Answer contains facts not in retrieved context (caught by faithfulness eval)
- Agent never re-retrieves after first pass
- No self-verification log in audit trail
**Prevention:**
- Self-RAG / CRAG pattern: after draft answer, agent re-reads retrieved docs and checks each claim
- Add explicit self-check tool: `verify_answer_against_sources(answer, chunks)` returns confidence per claim
- If any claim is unsupported → either retrieve more or strip the claim
- Log every self-check call for audit
**Phase:** Phase 4 (Agent loop) — mandatory before compliance demo

### P-AGENTIC-4: Citation hallucination (fake clause reference)
**What goes wrong:** Agent cites a specific clause number / paragraph that doesn't exist in any retrieved doc. Highest legal risk.
**Why:** LLM generates fluent-but-fabricated specifics; no grounding check.
**Warning signs:**
- Eval set includes queries with known correct answers; agent cites wrong clause in >5% of cases
- Citation text doesn't appear verbatim in any retrieved chunk
- Manual audit: "agent cited §3.2 but §3.2 doesn't say what it claims"
**Prevention:**
- **Hard rule:** agent can only cite chunks it actually retrieved (chunk IDs in prompt context)
- Citation = chunk ID + span, not free text
- Post-generation check: for each cited chunk ID, verify the cited span exists in that chunk's text
- If no chunk supports a claim, agent must remove the claim or refuse
- This is the single most important eval metric for the compliance demo
**Phase:** Phase 4 (mandatory) + Phase 5 (citation-precision is the headline metric)

---

## 4. Eino ADK Misuse Pitfalls

### P-EINO-1: Wrong agent composition (circular calls / infinite loops)
**What goes wrong:** Two agents call each other; no termination condition; infinite loop. Or workflow agent routes to itself.
**Why:** Graph composition without cycles check; manual routing missing terminal nodes.
**Warning signs:**
- Logs show agent A → agent B → agent A repeated
- Token cost per query is unbounded
- Timeout in production
**Prevention:**
- Use ADK's built-in patterns (`plan-react`, `workflow`, `manual`) — don't hand-roll graph edges
- Static graph validation: reject cycles at startup (Eino provides graph compile-time check)
- Set per-query step limit (e.g., 10 agent calls)
**Phase:** Phase 3 (Eino agent kernel) — encode in framework

### P-EINO-2: State bleeding between turns
**What goes wrong:** Variables set in turn 1 (user identity, retrieved docs, partial answers) leak into turn 2 of a different user/session.
**Why:** Global state, package-level variables, or shared Eino Runner instance without per-session scoping.
**Warning signs:**
- User A sees content from User B's session
- Conversation history shows cross-session contamination
- Compliance: wrong tier docs returned to wrong user
**Prevention:**
- All per-session state in a `SessionState` struct passed explicitly; no globals
- Eino Runner instantiated per-session, or use context propagation for state
- Add session isolation tests (multi-tenant fuzz)
**Phase:** Phase 3 (kernel design) + Phase 7 (multi-user testing)

### P-EINO-3: Tool permission too broad (excessive agency)
**What goes wrong:** Skill's tools can write files, call arbitrary URLs, execute shell — agent uses these for in-scope queries it shouldn't.
**Why:** "Just give the agent everything; it will figure it out." Mirrors OWASP LLM08.
**Warning signs:**
- Skill's `tools` list includes shell exec, file write, network
- Audit log shows writes happening during read-only queries
- Compliance: agent wrote to 草稿 tier during 权威 query
**Prevention:**
- Tools are tier-scoped (compliance Skill can't have write tools at all)
- Per-tool policy: `read_only`, `requires_confirmation`, `tier_restricted`
- Eino: wrap tools with permission check middleware
- Default: read-only tools only; writes require explicit Skill opt-in
**Phase:** Phase 3 (tool wrapper) + Phase 5 (policy assertions in CI)

---

## 5. Eval & Guardrails Pitfalls

### P-EVAL-1: Missing eval pipeline (ship unmeasured quality)
**What goes wrong:** No metrics; "looks good in demo" becomes "we shipped hallucination as a feature."
**Why:** Eval feels like extra work; demo-ability prioritized.
**Warning signs:**
- No `eval/` directory or only a single hand-picked query
- No CI step running eval
- "Quality" tracked via vibes, not numbers
**Prevention:**
- Phase 5 must ship RAGAS (or equivalent) integration: faithfulness, context_precision, answer_relevancy
- Eval set must include 50+ queries per Skill: easy, hard, refusal-required, multi-hop, adversarial
- Eval runs on every PR; regression alerts
**Phase:** Phase 5 (mandatory; blocks Phase 8 ship)

### P-EVAL-2: Wrong metrics (optimize fluency not faithfulness)
**What goes wrong:** Eval tracks BLEU / cosine similarity / human-rated fluency; model improves at sounding good while hallucination worsens.
**Why:** Easy metrics get picked first.
**Warning signs:**
- No "faithfulness" or "groundedness" metric in dashboard
- Eval metric improves while manual QA finds more hallucinations
**Prevention:**
- Headline metrics must be faithfulness (RAGAS) + citation precision (custom)
- Track refusal accuracy (right refusal vs wrong refusal)
- Include human-spot-check rate in dashboard
**Phase:** Phase 5

### P-EVAL-3: Eval set bias (golden set ≠ real distribution)
**What goes wrong:** Eval set is hand-crafted by authors who know the system; doesn't represent real user queries. Model overfits to eval set.
**Why:** Convenience, time pressure.
**Warning signs:**
- All eval queries match templates agent was tuned on
- No adversarial / typo / multi-lingual cases
- Production queries diverge significantly from eval
**Prevention:**
- Eval set composition: 40% real-user queries (logged), 30% hand-crafted, 30% synthetic adversarial
- Include: typos, code-switching, ambiguous queries, follow-ups
- Refresh eval set quarterly with production samples
**Phase:** Phase 5 + ongoing maintenance in Phase 7

### P-EVAL-4: LLM-as-judge bias
**What goes wrong:** Eval uses GPT-4 to judge; model favors its own style / has known bias on length / formatting. RAGAS's faithfulness is itself LLM-judge-based and has documented limitations.
**Why:** Cheap automated eval; no human ground truth.
**Warning signs:**
- Human eval disagrees with automated eval >15% of time
- Long verbose answers score higher than concise correct ones
**Prevention:**
- Use multiple judge models + human spot-check on 10% of samples
- Report inter-judge agreement
- Calibrate LLM judge against human-labeled set before trusting
**Phase:** Phase 5 + Phase 7

---

## 6. Compliance Demo Skill Specific Pitfalls

### P-COMPL-1: Compliance hallucination (fabricated clauses) — TOP RISK
**What goes wrong:** Agent cites §3.2 of a policy that says something different, or invents a clause. User (compliance officer) trusts it. Legal/financial damage.
**Why:** LLM's natural fluency + lack of citation grounding check.
**Warning signs:**
- Citation-precision eval <95%
- Manual audit of 50 outputs finds ≥1 fabricated reference
- Agent answers confidently on queries where no tier-matched doc exists
**Prevention:** (This is the demo's core; budget disproportionate time)
- **Citations are chunk IDs, not free text** (per P-AGENTIC-4)
- Mandatory self-check pass: every cited chunk must contain the cited claim verbatim
- Refusal when no chunk supports the answer
- UI shows the actual retrieved text alongside the citation (user can verify)
- Eval must include adversarial queries that try to elicit fabricated clauses
**Phase:** Phase 4 (architecture) + Phase 5 (eval) + Phase 6 (UI must show source text)

### P-COMPL-2: Refusing valid queries
**What goes wrong:** Agent refuses "Can an employee work remotely 3 days/week?" because it can't find exact phrasing in the handbook.
**Why:** Over-conservative guardrails; no semantic match; literal clause lookup.
**Warning signs:**
- Refusal rate on plausibly in-scope queries >25%
- Help-desk style queries get refused
**Prevention:**
- Use semantic + paraphrase retrieval (BM25 misses "WFH" vs "remote work")
- Allow "based on related clause X, here's the likely answer" with uncertainty marker
- Track refusal accuracy in eval; tune threshold
**Phase:** Phase 4 + Phase 5

### P-COMPL-3: Citing wrong clauses (real clause, wrong content)
**What goes wrong:** Agent cites §5.1 (correct clause number) but paraphrases what §5.1 says incorrectly.
**Why:** LLM paraphrases retrieved text rather than quoting it.
**Warning signs:**
- Eval: cited clause number exists, but cited text doesn't match retrieved text exactly
- Paraphrase accuracy <90%
**Prevention:**
- Quote retrieved text verbatim for the critical sentence; mark paraphrase explicitly when needed
- UI shows retrieved text and agent's claim side-by-side
**Phase:** Phase 4 + Phase 6 (UI)

### P-COMPL-4: Missing audit trail
**What goes wrong:** No record of which doc / which chunk / which prompt / which model version produced this answer. Can't defend in audit.
**Why:** Logging treated as infra; not part of demo scope.
**Warning signs:**
- No structured log per query
- Logs lack: query, retrieved chunk IDs, model version, prompt template hash
- Can't answer "what did the agent tell user X about topic Y on date Z"
**Prevention:**
- Every query logs: `{ts, user, query, skill, chunk_ids[], model_ver, prompt_hash, answer, refusal_reason}`
- Log is queryable by compliance officer
- Retention policy meets typical regulatory requirement (≥1 year)
**Phase:** Phase 7 (production hardening) — but design hooks in Phase 4

### P-COMPL-5: No human-in-loop for high-stakes
**What goes wrong:** Agent answers "is this contract compliant?" with high confidence, no human review.
**Why:** "Fully automated" demo story.
**Warning signs:**
- Skill returns `compliant: true|false` as final answer
- No escalation path
- No confidence threshold for human review
**Prevention:**
- For compliance verdicts: require human review if confidence <0.9 OR if claim is novel
- UI shows "needs human review" badge
- Skill output is structured (`status`, `confidence`, `citations`, `review_required`) — never just free text
**Phase:** Phase 6 (UX) + Phase 4 (Skill output schema)

### P-COMPL-6: Mixing policy versions (current vs superseded)
**What goes wrong:** Agent retrieves from old policy (replaced 2024) and treats it as current.
**Why:** Tier metadata missing `effective_date`; no version filter.
**Warning signs:**
- Same clause appears with different text in different docs
- Agent can't tell which version is current
**Prevention:**
- Chunk metadata: `{tier, doc_id, version, effective_from, effective_to}`
- Retrieval filters out superseded versions unless explicitly asked
- UI citation shows `effective_from`
**Phase:** Phase 4 (indexing schema)

---

## 7. Frontend AI Integration Pitfalls

### P-FE-1: Streaming UX broken on tool calls
**What goes wrong:** Tool-call blocks stream → UI freezes / shows empty spinner for seconds → answer appears all at once. Or reasoning text bleeds into final answer. Known issue: Open WebUI Issue #23173.
**Why:** Frontend doesn't render typed stream parts separately; awaits full response before rendering.
**Warning signs:**
- Tool calls cause long pauses with no feedback
- Reasoning chain invisible to user
- UX feels like "blank → wall of text"
**Prevention:**
- Use AI SDK 4.x typed message parts: `{type: 'reasoning' | 'tool-invocation' | 'text' | 'source'}`
- Render each type in its own UI component; show progress
- Tool-invocation panel shows: "Searching compliance docs... [3 chunks]"
- Show reasoning chain collapsible (transparency for compliance!)
**Phase:** Phase 6 (frontend) — design decision before coding

### P-FE-2: Reasoning chain not shown (transparency loss)
**What goes wrong:** User sees final answer, not the steps. For compliance, this is *exactly* what the user needs to verify.
**Why:** Most chat UIs hide reasoning by default to save space.
**Warning signs:**
- User can't see why agent gave a particular answer
- No "show reasoning" toggle
- Auditability lost
**Prevention:**
- Reasoning chain is collapsible but visible by default in compliance mode
- Each reasoning step linked to retrieved chunk
- User can expand to see exact prompt sent to model
**Phase:** Phase 6 (UX)

### P-FE-3: Citation UI hidden / not interactive
**What goes wrong:** Agent cites "Section 3.2" as plain text; user can't click to see what §3.2 actually says.
**Why:** Frontend treats citation as decoration, not verification surface.
**Warning signs:**
- Citations are `[1]` superscripts with no link
- User can't see cited text in one click
- No source panel
**Prevention:**
- Citation is a clickable link → opens side panel with retrieved chunk verbatim
- Source panel shows: tier, doc title, chunk text, version, effective date
- "Compare" view: agent claim vs source text side-by-side
**Phase:** Phase 6 (UX) — this is the demo's hero feature

### P-FE-4: No loading state during long tool calls
**What goes wrong:** Agent runs for 8s with no feedback; user closes tab.
**Why:** Streaming tool calls don't emit intermediate events.
**Warning signs:**
- User drop-off correlates with first-tool-call latency
- No progress indicator during RAG retrieval
**Prevention:**
- Show sub-progress: "Searching 权威 tier... Found 12 candidates... Re-ranking..."
- Use Eino's intermediate event emission
**Phase:** Phase 6

### P-FE-5: Refusal UX is silent or apologetic
**What goes wrong:** Agent refuses without saying why, or with a generic "I can't help with that."
**Why:** Refusal prompt is bare.
**Warning signs:**
- User can't tell if refusal is correct or a bug
- No suggestion for rephrasing
**Prevention:**
- Structured refusal: `{reason, can_user_rephrase, suggested_alternative_query}`
- UI distinguishes "out-of-scope" (clear message) from "I don't know" (offer to escalate)
**Phase:** Phase 6

---

## 8. Open-Source Distribution Pitfalls

### P-OSS-1: Weak README (kills adoption)
**What goes wrong:** README is a paragraph, no GIF, no quick start, no demo link. Stars plateau at <50.
**Why:** Authors assume readers will figure it out.
**Warning signs:**
- No "What / Why / Demo / Quick Start" sections
- No animated GIF of the demo Skill in action
- No architecture diagram
**Prevention:**
- README template: badges → 1-line value prop → demo GIF → features → quick start (Docker one-liner) → architecture diagram → contributing → license
- Hero image showing compliance demo with citations
**Phase:** Phase 8 (release readiness)

### P-OSS-2: No live demo (kills curiosity)
**What goes wrong:** User must clone + set API keys + run to see anything. Most won't.
**Why:** Deployment isn't free; demo hosting skipped.
**Warning signs:**
- No demo URL in README
- Setup requires >5 minutes
**Prevention:**
- Deploy a demo instance (HuggingFace Spaces / fly.io / a free tier VPS)
- Demo is read-only with sample compliance docs
- "Try the demo first, clone if interested"
**Phase:** Phase 8

### P-OSS-3: Hard to run locally (kills evaluation)
**What goes wrong:** Setup requires manual config of Go, Node, API keys, vector DB, embedding model. User gives up at step 3.
**Why:** "It works on my machine" syndrome.
**Warning signs:**
- Setup requires >10 manual steps
- No Docker Compose
- No `.env.example`
**Prevention:**
- `docker-compose up` brings up the full stack
- `.env.example` with every required variable
- Pre-bundled sample compliance docs (so it works offline / without API key for retrieval)
- Document common setup failures + fixes
**Phase:** Phase 8

### P-OSS-4: No CONTRIBUTING / unclear architecture (kills contributors)
**What goes wrong:** Someone wants to add a Skill; can't figure out the SKILL.md schema, can't find the extension point.
**Why:** Code is the only documentation.
**Warning signs:**
- No CONTRIBUTING.md
- No "How to write a Skill" tutorial
- No example Skill other than compliance
**Prevention:**
- CONTRIBUTING.md with: how to add a Skill, how to add a tool, how to add a knowledge tier
- `examples/` folder with 2-3 Skill templates (compliance, simple QA, calculator)
- Architecture doc in `docs/architecture.md`
**Phase:** Phase 8

### P-OSS-5: License / IP unclear (kills enterprise adoption)
**What goes wrong:** Apache-2.0 declared but uses a non-compatible dep. Or no license. Enterprise legal team blocks adoption.
**Why:** License is an afterthought.
**Warning signs:**
- No LICENSE file
- Dependencies with GPL/AGPL/SSPL pulled in without notice
**Prevention:**
- License file present from day 1
- CI check: fail if new dep has incompatible license
- README explicitly states: "Apache-2.0; no telemetry; no phone-home"
**Phase:** Phase 1 (project init) + Phase 8 (release)

### P-OSS-6: Missing evaluation results (kills credibility)
**What wrong:** No published metrics. "Reduces hallucination" — by how much? On what set?
**Why:** Eval results are internal; not seen as marketing.
**Warning signs:**
- README makes claims without numbers
- No `eval/results/` directory in repo
**Prevention:**
- Publish eval results in README: faithfulness %, citation precision %, refusal accuracy %
- Include the eval set (or a sample) in repo
- Compare: SkillForge vs naive RAG baseline
**Phase:** Phase 8 (release) — but build the data in Phase 5

---

## Phase Mapping Summary

| Phase | Primary Pitfalls to Address |
|-------|-----------------------------|
| **Phase 1 (Foundation)** | P-OSS-5 (license), P-SKILL-1..6 spec design |
| **Phase 2 (Skill system)** | P-SKILL-1, 2, 3, 4, 5, 6 (all Skill design) |
| **Phase 3 (Eino kernel)** | P-EINO-1, 2, 3 (composition, state, permissions) |
| **Phase 4 (Agentic RAG)** | P-RAG-1..4, P-AGENTIC-1, 3, 4, P-COMPL-1, 3, 6 |
| **Phase 5 (Eval pipeline)** | P-EVAL-1, 2, 3, 4, P-COMPL-1 (metric), P-SKILL-4 (refusal metric) |
| **Phase 6 (Frontend)** | P-FE-1..5 (streaming, citations, refusal UX) |
| **Phase 7 (Production hardening)** | P-RAG-2 (tier monitoring), P-COMPL-4 (audit trail), P-EINO-2 (multi-tenant) |
| **Phase 8 (Release)** | P-OSS-1..6 (README, demo, Docker, contributing, eval results) |

---

## Confidence Assessment

| Pitfall Group | Confidence | Notes |
|---------------|------------|-------|
| Skill design (P-SKILL-*) | MEDIUM | Anthropic Skills guidance + GitHub Issue #10768 (real bug evidence); denova pattern adapted; LOW on long-term maintenance cost |
| RAG tiered sources (P-RAG-*) | MEDIUM-HIGH | Self-RAG / CRAG / tiered RAG all documented in literature |
| Agentic RAG (P-AGENTIC-*) | MEDIUM | RAGAS docs + WebSearch on hallucination patterns |
| Eino ADK (P-EINO-*) | MEDIUM | Eino official docs + general ADK patterns; LOW on Eino-specific quirks (limited public case studies) |
| Eval (P-EVAL-*) | MEDIUM-HIGH | RAGAS limitations well-documented (context precision deprecated; LLM-judge bias known) |
| Compliance (P-COMPL-*) | HIGH | Legal/financial AI hallucination is well-publicized; OWASP LLM08; citation fabrication documented in multiple sources |
| Frontend (P-FE-*) | MEDIUM | AI SDK 4.x typed parts + Open WebUI Issue #23173 (real bug evidence) |
| Open-source (P-OSS-*) | HIGH | Generic OSS distribution best practices; well-established |

---

## Open Questions / Research Gaps

- **Eino ADK specific failure modes**: Public case studies on Eino production deployments are sparse. Worth deeper research during Phase 3 if issues emerge.
- **Chinese-language compliance text retrieval**: Embedding quality for Chinese legal/financial terms is uneven. May need fine-tuned embedding for the compliance demo specifically.
- **Cross-tier conflict resolution**: When 权威 and 个人 tiers disagree, who wins? Need a policy framework — research during Phase 4.
- **Eval set sourcing for compliance**: Real enterprise compliance queries are sensitive; cannot be reused publicly. May need synthetic-data strategy — research during Phase 5.

---

## Source Confidence Summary

**HIGH (Context7 / official docs):** Eino GitHub (`github.com/cloudwego/eino`), RAGAS docs (`docs.ragas.io`), OWASP LLM Top 10, Anthropic agentic misalignment research.

**MEDIUM (multiple credible WebSearch sources):** Self-RAG / CRAG / tiered RAG patterns, Vercel AI SDK 4.x typed parts, citation hallucination in RAG.

**LOW (single-source / training-data-based):** Denova-specific patterns (project forked but limited public docs), some Eino production case studies.

Sources referenced:
- [Eino GitHub](https://github.com/cloudwego/eino) and [Eino ADK](https://github.com/cloudwego/eino/tree/main/adk)
- [OWASP Top 10 for LLM Applications (LLM08 Excessive Agency)](https://owasp.org/www-project-top-10-for-large-language-model-applications/)
- [Anthropic Securing AI Agent Tool Use / agentic misalignment research](https://www.anthropic.com/research/agentic-misalignment)
- [RAGAS Faithfulness Metric docs](https://docs.ragas.io/en/stable/concepts/metrics/faithfulness.html) and [RAGAS paper](https://arxiv.org/pdf/2309.15217.pdf)
- [Fabricated Citations in LLM Outputs (Medium)](https://medium.com/@sliao-ai/fabricated-citations-in-llm-outputs-2c98a4a8f810)
- [The Hidden Crisis of RAG: Why Citations Can Be Sources of Hallucination](https://pub.towardsai.net/the-hidden-crisis-of-rag-why-citations-can-be-sources-of-hallucination-36894df3fbf3)
- [Self-RAG / CRAG corrective retrieval patterns](https://docs.ragas.io/) (via literature)
- [Anthropic Claude Skills Bug: NL intent matching (Issue #10768)](https://github.com/anthropics/claude-code/issues/10768)
- [Open WebUI streaming reasoning truncation (Issue #23173)](https://github.com/open-webui/open-webui/issues/23173)
- [AI agent routing intelligence / failure modes](https://www.e2enetworks.com/blog/ai-agent-routing-intelligence)
- [Vercel AI SDK 4.2 reasoning / typed parts (Juejin)](https://juejin.cn/post/7484078291248594980)
- [Patterns for Building a RAG Frontend](https://medium.com/@vishalmakwana/patterns-for-building-a-rag-frontend-key-architectural-decisions-and-design-considerations-bf0e9a3b66dd)
- [NIST Principle of Least Privilege](https://csrc.nist.gov/glossary/term/least_privilege)
- [AutoRAG PR removing context precision (RAGAS limitation)](https://github.com/Marker-Inc-Korea/AutoRAG/pull/455/commits)