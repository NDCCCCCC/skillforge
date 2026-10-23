# Security Policy

SkillForge is an **execution-time trust boundary** for AI agents: tier-isolated
knowledge retrieval, a human approval gate for write actions, grounding-checked
citations, and refusal-by-construction. Security is a design input here, not an
afterthought — the full STRIDE register behind these mechanisms lives in
[`docs/threat-model/`](docs/threat-model/).

## Supported versions

| Version | Supported |
|---------|-----------|
| `main`  | ✅ (development) |
| v1.7.x tags | ✅ (latest release) |

Earlier milestones are historical development snapshots and do not receive
security patches.

## How the project thinks about security

- **Knowledge tiers are enforced at retrieval time.** A Skill's trust contract
  declares which tiers it may read; the retriever constrains candidates by tier
  *before* scoring, so an unauthorized tier can never influence an answer.
- **Write actions require a human approval** whose recorded arguments are
  byte-identical to the executed ones (approve-and-remember).
- **Every answer is grounded**: a citation whose span does not appear in the
  cited chunk is automatically downgraded.
- **Skills cannot ship regressed**: promotion runs through an in-tree eval gate
  (faithfulness / refusal-accuracy thresholds, fail-closed).

## Reporting a vulnerability

Please use **GitHub's private vulnerability reporting**
(**Security** tab → *Report a vulnerability*) — do **not** open a public issue
for security reports.

Include: affected path/commit, a minimal reproduction, and your assessment of
impact. We aim to acknowledge reports within **7 days** and will publish a fix
and a coordinated disclosure note once a patch is available.

## Scope

**In scope:** the production code paths under `internal/` and `cmd/`, the web
client under `web/src`, and the default shipped configuration (Docker Compose,
`.env.example`).

**Out of scope:** the demonstration corpus under `knowledge/` (it is sample
regulatory text, not real policy), known limitations of local development
modes that are documented in the README (for example the stub LLM mode's
labelled behaviour), and any deployment where the operator has disabled the
approval gate or admin token by explicit configuration.
