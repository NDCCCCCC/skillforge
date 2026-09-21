---
spike: "005d-b"
name: memorymap-kg
type: standard
validates: "Given the 215-triple SkillForge TBox (005b), when loaded into a pure-Go memory store with byS/byP/byO indices + JSON snapshot, then load < 100ms, query < 1ms, snapshot roundtrip lossless, memory < 5MB"
verdict: VALIDATED
related: [005d-kg-storage-eval, 005d-a-kuzu, 005b-skillschema-rdfs-lift]
tags: [kg, memory-map, pure-go, snapshot, spike-005]
---

# Spike 005d-b: Memory Map KG Storage

Sub-spike of `005d-kg-storage-eval` (comparison: Kuzu vs memory map).

See **[005d-kg-storage-eval/README.md](../005d-kg-storage-eval/README.md)** for the full comparison and verdict (this spike is the **winner**).

## Local Results

```
=== spike 005d-b: memory-map KG storage (pure Go baseline) ===
[1] loaded 215 triples in 7.71 ms (7.71 ms)
[1a] memory estimate: ~43 KB
[2a] ?x a skf:Skill -> 8 instances in 0.00 ms
[2b] cat_sql-guides ->belongsToTier-> [http://skillforge.local/ontology#sql-guides]
[2c] ?x hasReadAccess cat_sql-guides -> 1 readers in 0.00 ms
    - sql-guides
[3a] snapshot: 40678 bytes in 0.30 ms
[3b] reload snapshot: 215 triples in 0.42 ms
=== VERDICT: VALIDATED — memory-map KG storage meets SkillForge baseline ===
```

## API

```go
type Store struct { /* ... */ }
func NewStore() *Store
func (s *Store) Insert(t Triple)
func (s *Store) Query(subj, pred, obj string) []Triple   // pattern match (empty = wildcard)
func (s *Store) Adjacent1(subj, pred string) []string    // 1-hop forward
func (s *Store) Adjacent2(subj, pred1, pred2 string) []string
func (s *Store) Snapshot() ([]byte, error)               // JSON
func LoadSnapshot(data []byte) (*Store, error)
```

## Headline takeaway

Pure Go. 7.71ms to load 215 triples. ~43 KB. Zero dependencies beyond stdlib + goRDFlib.

This is the **recommended KG store** for SkillForge at current scale (< 10k nodes). See 005d-kg-storage-eval for the full comparison and migration criteria (when to switch to Kuzu).
