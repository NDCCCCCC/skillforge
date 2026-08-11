
## 2026-08-10 — Finance Compliance Data Architecture

Q1. **NL2SQL safety**: how to ensure agent cannot write `SELECT *` to bypass desensitization?
   Options: capability restriction / query-only-KPI-functions / sandbox / SQL parser whitelist.

Q2. **Aggregation granularity**: k-anonymity vs differential privacy vs simple bucketing
   — which matches Chinese financial compliance regulations?

Q3. **External authorized channel protocol**: read-only DB user per Skill + TTL token + audit log?
   Or proxy service (Hasura / PostgREST) with row-level security?

Q4. **Two-tier audit design**: internal audit log (SkillForge) + external query log (data warehouse) —
   how to reconcile timestamps and IDs across systems?

Q5. **Re-identification risk**: even with k-anonymity, joining multiple aggregates can re-identify individuals.
   How to detect / prevent? (DPIA-style analysis)

## 2026-08-11 — NL2SQL Security Deep Dive

Q6. **NL2SQL accuracy**: agent generates SQL preview before approval — how to ensure it's accurate?
   Options: self-verify with second LLM / dry-run with EXPLAIN / static analysis.

Q7. **Approval SLA optimization**: 持续挂起 puts pressure on data officers under load.
   Phase 6 to evaluate: queue priority + escalation rules + bulk-approval for similar queries.

Q8. **TTL token design**: agent calls external query interface with TTL token.
   Token lifetime / revocation / rotation / scope — what are best practices for financial compliance?
