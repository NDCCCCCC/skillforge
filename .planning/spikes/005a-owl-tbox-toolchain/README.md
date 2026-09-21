---
spike: "005a"
name: owl-tbox-toolchain
type: standard
validates: "Given a minimal OWL/Turtle TBox file (Class + ObjectProperty + DatatypeProperty + Restriction), when loaded via tggo/goRDFlib v0.1.0 and walked, then (a) all Classes enumerated with label + subClassOf hierarchy, (b) ObjectProperty domain/range extracted, (c) DatatypeProperty domain/range extracted, (d) Restriction 'P some D' enumerated, (e) roundtrip lossless"
verdict: VALIDATED
related: []
tags: [owl, rdf, turtle, tbox, goRDFlib, spike-005]
---

# Spike 005a: Go OWL TBox Toolchain

## What This Validates

The SkillForge spike series (005a–005e) explores whether ontology modeling (OWL/RDF TBox + ABox) can be layered onto the existing Knowledge Base (currently 5 tiers + flat markdown files). The first gate is: **can Go itself load and walk an OWL TBox?** If not, every downstream spike (005b/005c/005d/005e) collapses.

Given/When/Then:
- **(a) parser:** Given a `.ttl` file with 5 Classes + 3 ObjectProperty + 2 DatatypeProperty + 1 Restriction, when loaded via `turtle.Parse`, then `graph.Len()` returns 50 triples without error.
- **(b) Classes:** then `?x a owl:Class` enumerates 6 distinct subjects with rdfs:label readable.
- **(c) Hierarchy:** then `?x rdfs:subClassOf ?y` enumerates parent relations (including blank-node Restriction parents).
- **(d) ObjectProperty:** then `?p a owl:ObjectProperty` enumerates with `rdfs:domain` + `rdfs:range` resolved.
- **(e) DatatypeProperty:** same for `owl:DatatypeProperty`.
- **(f) Restriction:** then `?r a owl:Restriction` blank nodes are enumerated and their `owl:onProperty` + `owl:someValuesFrom` walked to produce "P some D" strings.
- **(g) roundtrip:** then serialize back to Turtle and reparse produces the same triple count.

## Research

### Library selection

| Library | Lang | CGO? | Maturity | Verdict |
|---|---|---|---|---|
| `github.com/deiu/rdf2go` | Go | **YES (librdf/redland C lib)** | medium | rejected — cgo + `apt install librdf-dev` blows the spike-004 single-binary contract |
| `github.com/semantix/owlgo` | Go | YES (calls into librdf-style API for input ontology) | medium | rejected — same cgo dependency chain |
| `github.com/tggo/goRDFlib` | Go | **NO** | v0.1.0, Apache-2.0, 2026 | **chosen** — pure Go, parser for Turtle/N-Triples/N-Quads/RDF/XML/JSON-LD + triple-store + graph operations |

`goRDFlib` is the only Go-native (no cgo) library that covers the four formats we need for ontology ingestion (Turtle is primary; JSON-LD covers web sources later). v0.1.0 is fresh but the API surface is small and the README is honest about its scope.

### API shape (learned by reading source — pkg.go.dev fetch failed)

- `term.MustURIRef(s) URIRef` (value type, not interface) — well-known constants are cheap
- `term.NewLiteral(v, opts...) Literal` — `.Value()` returns `any`, requires `.(string)` assertion
- `graph.NewGraph()` → `*Graph`
- `turtle.Parse(g *Graph, r io.Reader) error` — note: takes Graph pointer, NOT a count return
- `turtle.Serialize(g, w io.Writer) error`
- `g.Triples(s term.Subject, p *URIRef, o term.Term) TripleIterator` — `TripleIterator = iter.Seq[Triple]` (Go 1.23+ range-over-func); predicate uses pointer for nil-as-wildcard
- `g.Len() int`
- `Triple.Subject term.Subject` (= `URIRef | BNode`), `Triple.Predicate URIRef`, `Triple.Object term.Term`

### TBox example structure

`example_tbox.ttl` declares 6 Classes (with 1 named subClassOf + 1 anonymous-Restriction parent), 3 ObjectProperties, 2 DatatypeProperties, 3 Tier instances, and 1 Restriction. Total 50 triples. Roundtrips with no triple loss.

### Gotchas hit during the spike

1. **Constructor naming**: First draft used `term.NewIRIRef` — wrong, the package exports `URIRef` (not `IRIRef`). Correct: `term.MustURIRef("...")` or `term.NewURIRefUnsafe("...")`.
2. **`URIRef` is a value type, not an interface**: had to switch from `term.IRI` assertions to `term.URIRef`.
3. **`Literal.Value()` returns `any`**: requires double assertion `if v, ok := l.Value().(string); ok` — first spike build failed on this.
4. **BNode subject in Restriction**: `typeSet(g, owlRes)` originally did `tr.Subject.(term.URIRef)` which silently dropped BNodes → 0 restrictions extracted. Fix: collect `term.Subject` (interface) into a `map[string]term.Subject`, iterate by string key.

## How to Run

```bash
cd .planning/spikes/005a-owl-tbox-toolchain
/usr/local/go/bin/go run . 2>&1 | tee 005a-output.log
```

## What to Expect

```
=== spike 005a: Go OWL TBox toolchain (tggo/goRDFlib v0.1.0) ===
[1] parsed graph: 50 triples from example_tbox.ttl
[2] owl:Class count=6
    - skf:AuthoritativeSQLNote label="Authoritative SQL Note"
    - skf:BusinessUnit     label="Business Unit"
    - skf:KnowledgeNote    label="Knowledge Note"
    - skf:SQLNote          label="SQL Note"
    - skf:TableSchema      label="Table Schema"
    - skf:Tier             label="Tier"
[2.5] class hierarchy (subClassOf):
    - skf:AuthoritativeSQLNote subClassOf [_:... skf:SQLNote]
    - skf:SQLNote subClassOf [skf:KnowledgeNote]
[3] owl:ObjectProperty count=3
    - skf:belongsToTier    domain=skf:KnowledgeNote  range=skf:Tier           label="belongs to tier"
    - skf:partOf           domain=skf:BusinessUnit   range=skf:BusinessUnit   label="part of (organisational hierarchy)"
    - skf:referencesTable  domain=skf:SQLNote        range=skf:TableSchema    label="references table"
[4] owl:DatatypeProperty count=2
    - skf:noteContent      domain=skf:KnowledgeNote  range=xsd:string         label="note content"
    - skf:noteTitle        domain=skf:KnowledgeNote  range=xsd:string         label="note title"
[5] owl:Restriction count=1
    - skf:belongsToTier some skf:权威
[6a] serialized back: 50 triples, 2168 bytes
[6b] reparsed: 50 triples
[6c] roundtrip lossless: OK (n == n3)

=== VERDICT: VALIDATED — Go OWL TBox toolchain is viable ===
```

## Investigation Trail

| Iteration | Change | Result |
|---|---|---|
| v1 | First draft using guessed API names (`NewIRIRef`, `IRIRef` interface) | compile errors — package exports `URIRef` value type |
| v2 | Fixed to `MustURIRef` + `URIRef` assertions + `Literal.Value()` type assertion | compiles, runs, but `[5] Restriction count=0` |
| v3 | Realised `typeSet` filtered out BNode subjects via `tr.Subject.(term.URIRef)` cast | rewrote `restrictionsOf` to collect `term.Subject` directly, ordered by string key |
| v4 (final) | Restriction correctly extracted: `skf:belongsToTier some skf:权威` | **VERDICT: VALIDATED** |

## Results

**Verdict: ✓ VALIDATED.** The Go OWL TBox toolchain is viable on pure-Go `tggo/goRDFlib v0.1.0`. The blocker (no Go-native RDF lib) was a false alarm — `goRDFlib` covers our needs for spike 005b (lift SKILL.md + KB dir tree to RDFS/OWL).

### Implications for downstream spikes

- **005b** can now proceed: write a small Go generator that emits a `.ttl` file mapping SKILL.md frontmatter (7 fields) + KB directory structure + tier labels into RDFS/OWL.
- **005c** stays independent — ontology-aware chunking is a markdown parser task, not an OWL one.
- **005d** KG storage picks up where 005b leaves off: the TBox becomes the schema; ABox (notes/tables) goes into Kuzu/memory-map.
- **005e** can be answered honestly: with 005a–005c proven, we can evaluate "Milvus necessity" against an actual ontology-aware retrieval baseline, not just literature.
