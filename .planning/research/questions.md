
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
