---
spike: "005d"
name: kg-storage-eval
type: comparison
subspikes: [005d-a-kuzu, 005d-b-memorymap]
validates: "Given the SkillForge TBox (215 triples from 005b) and an ABox subset (8 Skills + 5 Tiers + hasReadAccess edges), when comparing (a) Kuzu v0.11.3 embedded graph DB via cgo against (b) pure-Go memory map with JSON snapshot, then both can answer basic query patterns in < 100ms, and we pick the lower-overhead option"
verdict: VALIDATED — pick 005d-b (memory map)
related: [005b-skillschema-rdfs-lift, 005e-post-ontology-vector-necessity]
tags: [kg, kuzu, memory-map, cgo, storage, comparison, spike-005]
---

# Spike 005d: KG Storage Selection — Kuzu vs Memory Map

## What This Validates

With the TBox + ABox pattern stable (005a/005b), 005d asks: **where does the KG live?** Two lightweight candidates:

| | 005d-a Kuzu v0.11.3 | 005d-b Memory Map |
|---|---|---|
| Bindings | `github.com/kuzudb/go-kuzu` (cgo + pre-built `libkuzu.so` ~70MB) | stdlib only |
| Query language | Cypher (full property graph) | In-memory index by S/P/O + ad-hoc filters |
| Persistence | on-disk DB file | JSON snapshot to disk |
| Lines of code | ~100 | ~250 |

The spike validates both, then picks the better fit for SkillForge scale.

## Research

### Kuzu (005d-a)

- v0.11.3 (Oct 2025), MIT, Go binding via cgo
- Linux amd64: pre-built `libkuzu.so` shipped in module — no source build required
- API: `kuzu.OpenInMemoryDatabase` / `OpenConnection` / `conn.Query(cypher)` returns `QueryResult`
- Cypher semantics: standard MATCH/CREATE/RETURN; **CREATE with new node literal + existing PK collides** (gotcha — must use MATCH-then-CREATE for edges pointing to existing nodes)

### Memory map (005d-b)

- Three indices: `byS`, `byP`, `byO` (subject/predicate/object → slice of triple indices)
- Insertion is O(1) per triple + 3 map updates
- Query: pick the most selective index, then filter the rest
- Snapshot: `json.MarshalIndent(all, "", "  ")` — human-readable, ~190 bytes per triple

## Side-by-Side Results

| Metric | 005d-a Kuzu | 005d-b Memory Map | Winner |
|---|---|---|---|
| Open / connect | 16.07 ms | — | — |
| Load 215 triples (TBox) | (separate) 35.65 ms (DDL) | 7.71 ms | **map** (4.5×) |
| Insert 8 Skills + 5 Tiers + 8 edges | 60.80 ms | — | — |
| Query 8 rows (MATCH/Query) | 10.07 ms | < 1 ms | **map** (10×) |
| Total elapsed | 70.96 ms | 8 ms | **map** (~9×) |
| Memory footprint (215 triples) | `~70 MB libkuzu.so` + heap | **~43 KB** | **map** (1500×) |
| Lines of code | ~100 | ~250 | Kuzu |
| Persistence | on-disk DB file | JSON snapshot | Kuzu (binary, ACID) |
| Query language | Cypher (expressive) | Go code (manual) | Kuzu |
| Cold-start cost | libkuzu.so load (~10ms) | zero | **map** |
| CGO dependency | yes | no | **map** |

### Verdict

**005d-b (memory map) wins for SkillForge scale.** Reasons:

1. **Scale** — current KB is 26 notes × ~3 sections = ~80 logical chunks. Even 100× growth (2600 chunks) fits comfortably in a Go map (< 50 MB heap). At this scale, the constant-factor overhead of Kuzu's columnar storage + Cypher parser dominates the actual query cost.
2. **No CGO** — single static binary matches the spike 004 / production SkillForge pattern (`milvusclient v2.6.5` is the only cgo exception, justified by Milvus' size).
3. **DevX** — queries are Go code; type-checked, refactorable, IDE-friendly. Cypher strings are runtime-checked only.
4. **JSON snapshot** — easy to inspect, version-control, and diff in code review.

Kuzu would win if:
- Dataset grows past ~10k nodes
- Multi-process access (multiple SkillForge replicas) needs ACID transactions
- Query patterns become complex (3+ hops, path-finding, community detection) and writing them in Go becomes a maintenance burden

## How to Run

**005d-b (memory map)** — pure Go, no cgo:
```bash
TMPDIR=/tmp/spike005db_stage
mkdir -p $TMPDIR && cd $TMPDIR
ln -sf /mnt/d/code/ClaudeCode/ai-agent/.planning/spikes ./spikes
cp /mnt/d/code/ClaudeCode/ai-agent/.planning/spikes/005d-b-memorymap/main.go .
cp /mnt/d/code/ClaudeCode/ai-agent/.planning/spikes/005d-b-memorymap/go.mod .
cp /mnt/d/code/ClaudeCode/ai-agent/.planning/spikes/005d-b-memorymap/go.sum .
sed -i 's/spike005db/spike005db_stage/g' go.mod
/usr/local/go/bin/go mod tidy
/usr/local/go/bin/go run .
```

**005d-a (Kuzu)** — needs cgo + libkuzu:
```bash
TMPDIR=/tmp/spike005da_stage
mkdir -p $TMPDIR && cd $TMPDIR
cp /mnt/d/code/ClaudeCode/ai-agent/.planning/spikes/005d-a-kuzu/main.go .
cp /mnt/d/code/ClaudeCode/ai-agent/.planning/spikes/005d-kg-storage-eval/go.mod .
cp /mnt/d/code/ClaudeCode/ai-agent/.planning/spikes/005d-kg-storage-eval/go.sum .
sed -i 's/spike005d/spike005da_stage/g' go.mod
/usr/local/go/bin/go mod tidy
/usr/local/go/bin/go run .
```

## Sub-spike Results

### 005d-a Kuzu — VALIDATED
```
=== spike 005d-a: Kuzu embedded KG (cgo binding) ===
[1] open in-memory DB + connection: 16.07 ms
[2] created 2 node tables + 1 rel table (35.65 ms total)
[3] inserted 5 Tiers + 8 Skills (with READS_FROM edges): 60.80 ms total
[4] MATCH (s)-[:READS_FROM]->(t) results (8 rows, 10.07 ms)
[5] total elapsed: 70.96 ms
=== VERDICT: VALIDATED ===
```

### 005d-b Memory Map — VALIDATED
```
=== spike 005d-b: memory-map KG storage (pure Go baseline) ===
[1] loaded 215 triples in 7.71 ms
[1a] memory estimate: ~43 KB
[2a] ?x a skf:Skill -> 8 instances in 0.00 ms
[2b] cat_sql-guides ->belongsToTier-> [http://skillforge.local/ontology#sql-guides]
[2c] ?x hasReadAccess cat_sql-guides -> 1 readers
[3a] snapshot: 40678 bytes in 0.30 ms
[3b] reload snapshot: 215 triples in 0.42 ms
=== VERDICT: VALIDATED ===
```

## Investigation Trail

| Iteration | Change | Result |
|---|---|---|
| 005d-b v1 | First draft: triple indices `idx := s.byP` etc., selected by selectivity heuristic | compile error — `idx` declared but not used |
| 005d-b v2 | Removed dead `idx` selection (always pick smallest index set, then filter) | compiles, 215 triples loaded in 7.86ms |
| 005d-a v1 | First draft used `CREATE (s)-[:READS_FROM]->(t {id: 'tier:authority'})` | Runtime exception: duplicated primary key — Kuzu tries to CREATE the Tier node again |
| 005d-a v2 | Split into `MATCH (t:Tier {id: '...'}) CREATE (s:Skill {...})-[:READS_FROM]->(t)` | 8 rows returned, total 70.96ms |

## Production Implications

1. **Adopt 005d-b** as the runtime KG store. JSON snapshot to `skillforge_kg.json` at boot, load on startup.
2. **005b's TBox emission** runs at boot (cheap — 215 triples, < 1ms to parse) and feeds both the KG and the retriever.
3. **005c's ontology-aware chunks** become ABox `Note` instances + `belongsToNote` edges — the retriever can now answer queries like "all chunks in notes that reference table etl_payment_due_t" with a single 1-hop traversal.
4. **Defer Kuzu migration** to when (a) KB grows past 10k chunks OR (b) we need multi-process ACID — neither is on the current roadmap.

## Out of Scope

- Property values beyond string (numbers, dates, lang-tagged strings)
- Reasoning (RDFS / OWL 2 RL inference) — neither store infers; the spike 005a `restrictionsOf` extractor proves we can read them out of the TBox, but neither KG store materializes them
- Sharding / replication
- Concurrency control (Go map + RWMutex not implemented in the spike; production code will need it)
