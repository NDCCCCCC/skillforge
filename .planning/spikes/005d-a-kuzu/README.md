---
spike: "005d-a"
name: kuzu-embedded-kg
type: standard
validates: "Given Kuzu v0.11.3 (cgo binding with pre-built libkuzu.so), when opening in-memory DB + creating 2 node tables (Skill/Tier) + 1 rel table (READS_FROM) + inserting 8 Skills + 5 Tiers + edges + running MATCH query, then total elapsed < 500ms and 8 rows returned"
verdict: VALIDATED
related: [005d-kg-storage-eval, 005d-b-memorymap]
tags: [kg, kuzu, cgo, embedded, spike-005]
---

# Spike 005d-a: Kuzu Embedded KG (cgo binding)

Sub-spike of `005d-kg-storage-eval` (comparison: Kuzu vs memory map).

See **[005d-kg-storage-eval/README.md](../005d-kg-storage-eval/README.md)** for the full comparison and verdict.

## Local Results

```
=== spike 005d-a: Kuzu embedded KG (cgo binding) ===
[1] open in-memory DB + connection: 16.07 ms
[2] created 2 node tables + 1 rel table (35.65 ms total)
[3] inserted 5 Tiers + 8 Skills (with READS_FROM edges): 60.80 ms total
[4] MATCH (s)-[:READS_FROM]->(t) results (8 rows, 10.07 ms):
    - compliance -> authority
    - compliance-orchestrator -> authority
    - simple-qa -> authority
    - calculator -> draft
    - external-lookup -> personal
    - kpi-dashboard -> personal
    - monthly-report -> personal
    - sql-guides -> sql-guides
[5] total elapsed: 70.96 ms
=== VERDICT: VALIDATED ===
```

## Headline takeaway

Kuzu works as advertised: 71ms for the full open+create+insert+query+close cycle on 13 nodes + 8 edges. Cypher is ergonomic.

But: 005d-b (memory map) is **9× faster**, **1500× lighter on disk** (no `libkuzu.so`), and **single-binary**. Kuzu wins on query expressiveness; memory map wins on everything else for SkillForge scale.

See 005d-kg-storage-eval for the full comparison.
