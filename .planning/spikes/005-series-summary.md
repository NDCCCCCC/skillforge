# Spike Series 005: Ontology in Knowledge Bases

**Date:** 2026-09-21
**Spikes processed:** 5 (005a / 005b / 005c / 005d-a / 005d-b / 005e)
**Verdicts:** 5 ✓ VALIDATED (one with a winner pick: 005d-b memory map)
**Skill output:** this summary + spike artifacts (committed to `.planning/spikes/`)

## The user's question

> 研究本体论在知识库方面的应用，和当前项目可以如何结合？

with a clarifying sub-question that turned out to be the real test:

> 验证 milvus 的必要性，使用本体论之后是否还需要向量数据库？

## Verdicts

| # | Spike | What | Verdict |
|---|---|---|---|
| 005a | owl-tbox-toolchain | Go can load OWL/RDF TBox (tggo/goRDFlib v0.1.0, no cgo) | ✓ VALIDATED |
| 005b | skillschema-rdfs-lift | SKILL.md 7 fields + 5-tier KB → 215-triple RDFS/OWL TBox | ✓ VALIDATED |
| 005c | ontology-aware-chunker | h2/section-aware markdown chunker recovers complete SQL (185 fragments → 26 notes, 100% boundary) | ✓ VALIDATED |
| 005d-a | kuzu-embedded-kg | Kuzu v0.11.3 cgo binding — 71ms for 13-node graph | ✓ VALIDATED |
| 005d-b | memorymap-kg | Pure-Go memory map — 8ms for 215 triples, ~43KB | ✓ VALIDATED |
| **005d** | kg-storage-eval | **memory map wins** (9× faster, 1500× lighter, single binary) | ✓ VALIDATED (pick 005d-b) |
| 005e | post-ontology-vector-necessity | KG + vectors are **complementary**, not substitutes | ✓ VALIDATED |

## The final answer

**Yes, Milvus is still necessary.** But its role shifts:

```
BEFORE (naive):
  query → Milvus+BGE → top-K chunks → LLM

AFTER (005 series):
  query → [005d-b KG: candidate gate by entity/SubCategory/tier]
         ↓
         ~10-50 candidate chunks
         ↓
         [Milvus+BGE: rerank by semantic similarity]
         ↓
         top-K final chunks → LLM
```

KG narrows the search space (precise relations, ontology-aware filters).
Vectors rank within that space (fuzzy semantic matches that KG can't capture).

This matches the 2026 literature consensus (OntoRAG / LightRAG / Semantica):
- **Hybrid** (KG filter + vector rerank) beats either alone
- Neither KG nor vector can fully substitute the other

## What gets added to the project

| Spike | Production artifact | Integration point |
|---|---|---|
| 005a | `tggo/goRDFlib v0.1.0` dependency (pure Go) | `internal/ontology/tbox.go` (new) |
| 005b | TBox emitter (YAML → Turtle) | `internal/ontology/lift.go` (new) — runs at server boot |
| 005c | Section-aware markdown chunker | `internal/knowledge/indexer.go` (replace byte-cap chunker for sql-guides) |
| 005d-b | Pure-Go memory-map KG + JSON snapshot | `internal/ontology/store.go` (new) |
| 005e | Hybrid retrieval orchestrator (KG filter → Milvus rerank) | `internal/retriever/hybrid.go` (new) |

## Key discoveries

1. **Go-native OWL/RDF exists**: `tggo/goRDFlib v0.1.0` is pure Go, Apache-2.0, and roundtrips Turtle losslessly. Rejected alternatives (`rdf2go`, `owlgo`) both require cgo + librdf.
2. **CJK sub-category names** (取数口径, 操作手册, 汽车险) round-trip cleanly through Turtle IRIs.
3. **The "7 SQL statements" assumption was wrong** — sql-guides corpus has 3 distinct target tables, with multiple update clauses per table. The repetition of `## SQL / 正文` across sec03–sec08 was a visual symptom of byte-bucket splitting, not evidence of multiple SQL statements.
4. **Kuzu is feature-complete but overhead-heavy** for our scale. Memory map wins 9× faster, 1500× lighter, single binary. Kuzu's Cypher is its only advantage.
5. **discovered third resource class** in SKILL.md reads/writes: `data:external:authorized`, `skill_state`, `plan_approve` — non-tier targets that don't fit the current Tier model. Worth modelling as `skf:Capability` in v2.
6. **HYBRID wins on average but not on every query** — the right strategy depends on query type. KG is precise for entity mentions, VEC is needed for fuzzy natural-language, HYBRID shines when there's a SubCategory narrowing.

## Known limitations

- **005d-b is not concurrent-safe** (production needs RWMutex)
- **005c section heuristic assumes frontmatter** — KBs without `section:` field fall back to byte-cap chunker
- **005e mock vectors are hash-based**, not real BGE-M3 embeddings. Real-world recall will differ but the *relative ordering* of strategies is what matters.
- **005e HYBRID sometimes loses to VEC** when KG filter is too narrow (Q4). Real fix is 2-hop subgraph expansion + cross-encoder reranker.

## Open questions (not blocking, recorded for future spikes)

1. Should `skf:Capability` become a first-class class in v2 TBox? (handles non-tier reads/writes)
2. KG-2-hop subgraph expansion depth — what's the right default?
3. Cross-encoder reranker integration (BGE-reranker-v2-m3 self-host vs API)?
4. Cold-start: how to bootstrap TBox from corpus when KG is empty? (OntoRAG's schema-induction stage)

## Files produced

```
.planning/spikes/
├── MANIFEST.md                          ← updated with 005 series
├── 005-series-summary.md                ← this file
├── 005a-owl-tbox-toolchain/             ← ✓ VALIDATED
│   ├── README.md, main.go, example_tbox.ttl, 005a-output.log
├── 005b-skillschema-rdfs-lift/          ← ✓ VALIDATED
│   ├── README.md, main.go, skillforge_tbox.ttl, 005b-output.log
├── 005c-ontology-aware-chunker/         ← ✓ VALIDATED
│   ├── README.md, main.go, chunk_metadata.json, 005c-output.log
│   └── merged_notes/                    ← 26 reconstructed notes
├── 005d-a-kuzu/                         ← ✓ VALIDATED (loser)
├── 005d-b-memorymap/                    ← ✓ VALIDATED (winner)
├── 005d-kg-storage-eval/                ← comparison summary
└── 005e-post-ontology-vector-necessity/ ← ✓ VALIDATED
    ├── README.md, main.go, 005e-output.log, 005e-results.json
```

## Commits

```
ed9cbd1 docs(spike-005e): VALIDATED — KG + vectors complement, not substitute
0d74b40 docs(spike-005d): VALIDATED — pick memory map for SkillForge KG
9d379e2 docs(spike-005c): VALIDATED — ontology-aware chunker recovers complete SQL
a84186f docs(spike-005b): VALIDATED — SKILL.md + KB lifted to RDFS/OWL TBox
ead3320 docs(spike-005a): VALIDATED — Go OWL TBox toolchain via tggo/goRDFlib v0.1.0
```
