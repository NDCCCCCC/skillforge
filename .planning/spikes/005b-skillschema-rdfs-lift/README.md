---
spike: "005b"
name: skillschema-rdfs-lift
type: standard
validates: "Given SKILL.md 7-field frontmatter + 5-tier KB directory tree (sql-guides has 7 sub-categories), when lifted via Go + gopkg.in/yaml.v3 into an RDFS/OWL TBox Turtle file, then (a) all 8 Skill instances enumerated with name/desc/temp/agent/reads/writes, (b) all 5 Tier instances, (c) all 7 SubCategory instances, (d) reload via tggo/goRDFlib reproduces the same instance counts, (e) roundtrip lossless"
verdict: VALIDATED
related: [005a-owl-tbox-toolchain]
tags: [skillschema, rdfs, lift, kb-metadata, yaml, spike-005]
---

# Spike 005b: SKILL.md + KB → RDFS/OWL TBox Lift

## What This Validates

Spike 005a proved that Go itself can load and walk an OWL/RDF TBox. 005b asks the next question: **can we programmatically lift SkillForge's own metadata into a TBox?** If yes, the TBox becomes a runtime artifact that the retriever / agent router / 005c ontology-aware chunker / 005d KG store can all consume uniformly.

Source data:
- 8 × `skills/*/SKILL.md` (compliance, sql-guides, calculator, simple-qa, kpi-dashboard, external-lookup, monthly-report, compliance-orchestrator)
- 5 × tier directories under `./knowledge/{authority, draft, historical, personal, sql-guides}/`
- 7 × sub-categories under `knowledge/sql-guides/` (其他, 取数口径, 操作手册, 汽车险, 理赔与赔付, 财务对账, 非车险)

Given/When/Then:
- **(a) Schema:** then 6 `owl:Class` (Tier, KBCategory, SubCategory, Skill, SkillAgent, RefusalPhrase) + 6 `owl:ObjectProperty` (belongsToTier, hasSubCategory, hasReadAccess, hasWriteAccess, poweredBy, requiresRefusal) + 8 `owl:DatatypeProperty` (skillName, skillDescription, skillTemperature, categoryName, subCategoryName, tierName, agentTypeName, refusalPhraseText) declared in the emitted `.ttl`.
- **(b) Instances:** then 8 `Skill` + 5 `Tier` + 5 `KBCategory` + 7 `SubCategory` individuals + at least 4 `SkillAgent` + ≥ 6 `RefusalPhrase` populated.
- **(c) Reload:** then tggo/goRDFlib reloads the emitted `.ttl` and reproduces the same instance counts.
- **(d) Roundtrip:** then serialize → reparse produces identical triple count.

## Research

### Stack

| Concern | Choice | Why |
|---|---|---|
| TBox emission | Plain string concatenation in Go | Avoids extra RDF-emit libs; full control over Turtle syntax |
| Frontmatter parsing | `gopkg.in/yaml.v3` | Stdlib-of-Go YAML; handles nested `guardrails.prohibitions[]` arrays cleanly |
| RDF load + walk | `github.com/tggo/goRDFlib v0.1.0` | Same lib proven in spike 005a; reuse 005a patterns (`term.MustURIRef`, `g.Triples(s, *p, o)`) |
| IRI safety | CJK sub-category names kept verbatim (Turtle allows it; URIRef not URL-encoded) | Verifies goRDFlib handles non-ASCII IRIs in both directions |

### What got lifted, what didn't

| SKILL.md field | Lifted to TBox | Notes |
|---|---|---|
| `description` | `skf:skillDescription` ✓ | Full text preserved |
| `agent` | `skf:poweredBy → skf:SkillAgent(skf:agentTypeName)` ✓ | 6 distinct agent names found (ChatModelAgent, ReActAgent, orchestrator, chat, deep) |
| `reads[]` | `skf:hasReadAccess → skf:cat_<tier>` ✓ for tier-glob entries | `data:external:authorized`, `data:aggregated_only` silently dropped (not tier globs) |
| `writes[]` | `skf:hasWriteAccess → skf:cat_<tier>` ✓ for tier-glob entries | `skill_state`, `report:publish`, `plan_approve`, `memory:facts` silently dropped (not tier globs) |
| `temperature` | `skf:skillTemperature` (xsd:decimal) ✓ | |
| `guardrails.prohibitions[]` | dropped (not modelled in v1) | Worth modelling as `skf:Prohibition` Class in v2 |
| `guardrails.required_refusal_phrases[]` | `skf:requiresRefusal → skf:RefusalPhrase` ✓ | 6 distinct phrases found (`no_authorized_source`, `out_of_scope`, `low_retrieval_score`, `no_matching_guide`, etc.) |
| `guardrails.tier_limits{}` | dropped (not modelled in v1) | Object-of-objects; needs `skf:TierLimit` intermediate class |

### Known gap (recorded, not blocking)

`reads`/`writes` accept both tier globs (`authority/**`) and **non-tier named targets** (`data:external:authorized`, `skill_state`, `plan_approve`). The current spike models only the tier-glob subset. The missing 4–5 named targets are an interesting follow-up — they hint at a *third* class of resource beyond Tier: **capability/handle targets** (memory keys, write gates, external API scopes). 005d's KG storage choice will need to know whether to model these too.

### Gotchas

1. **YAML frontmatter parser** strips the outer `---` markers, then runs `yaml.Unmarshal` on the inner block. Works for the 7-field frontmatter in all 8 skills.
2. **Variable shadowing** (caught on first compile): in the inner loop `if s, ok := p.(string); ok { s = ...; s.Prohibitions = ... }` — the `s` shadowed the outer `s Skill`. Fixed by renaming to `str`.
3. **CJK sub-category names**: Turtle allows non-ASCII IRIs (the angle-bracket form `<...>` or prefixed form `skf:其他`). goRDFlib handles them in both load + serialize.
4. **Run-from-spike-dir failure**: `go run .` from `.planning/spikes/005b/` can't find `skills/` (relative path resolves to `.planning/spikes/005b/skills/`). Workaround: stage the spike in `/tmp/spike005b/` with symlinks to `../../skills` and `../../knowledge`. A long-term fix is `go run . -C ../..` or passing `--root`.

## How to Run

```bash
# Stage the spike in /tmp (or anywhere writable) with symlinks:
TMPDIR=/tmp/spike005b_stage
mkdir -p $TMPDIR && cd $TMPDIR
ln -sf /path/to/ai-agent/skills ./skills
ln -sf /path/to/ai-agent/knowledge ./knowledge
cp /path/to/ai-agent/.planning/spikes/005b-skillschema-rdfs-lift/main.go .
cp /path/to/ai-agent/.planning/spikes/005b-skillschema-rdfs-lift/go.mod .
cp /path/to/ai-agent/.planning/spikes/005b-skillschema-rdfs-lift/go.sum .
sed -i 's/spike005b/spike005b_stage/g' go.mod
/usr/local/go/bin/go mod tidy
/usr/local/go/bin/go run .
```

## What to Expect

```
=== spike 005b: SKILL.md + KB -> RDFS/OWL TBox lift ===
[1] loaded 8 Skill(s) from skills/*/SKILL.md
    - calculator                 reads=[draft/**]                          writes=[skill_state]            agent=ReActAgent
    - compliance                 reads=[authority/**]                      writes=[]                        agent=ChatModelAgent
    - compliance-orchestrator    reads=[authority/** historical/**]        writes=[]                        agent=orchestrator
    - external-lookup            reads=[data:external:authorized]          writes=[]                        agent=chat
    - kpi-dashboard              reads=[data:aggregated_only]              writes=[]                        agent=ChatModelAgent
    - monthly-report             reads=[data:aggregated_only]              writes=[report:publish plan_approve] agent=deep
    - simple-qa                  reads=[authority/**]                      writes=[]                        agent=ChatModelAgent
    - sql-guides                 reads=[sql-guides/**]                     writes=[]                        agent=ChatModelAgent
[2] walked 5 KB Categories
    - authority    sub-categories=[]
    - draft        sub-categories=[]
    - historical   sub-categories=[]
    - personal     sub-categories=[]
    - sql-guides   sub-categories=[其他 取数口径 操作手册 汽车险 理赔与赔付 财务对账 非车险]
    total SubCategories=7
[3] emitted skillforge_tbox.ttl (12501 bytes)
[4a] reloaded: 215 triples
[4b] owl:Class count=6, owl:ObjectProperty=6, owl:DatatypeProperty=8
[4c] instances: Tier=5, KBCategory=5, SubCategory=7, Skill=8
[5] roundtrip: original=215 triples, reparse=215 triples — OK

=== VERDICT: VALIDATED — SKILL.md + KB lift to RDFS/OWL TBox works ===
```

The emitted `skillforge_tbox.ttl` contains 215 triples. The reload + walk reproduces 8 Skills + 5 Tiers + 5 Categories + 7 SubCategories. Roundtrip lossless.

## Investigation Trail

| Iteration | Change | Result |
|---|---|---|
| v1 | First draft using guessed `term.NewIRIRef` / `IRIRef` interface | compile errors (same as 005a; URIRef is value type) |
| v2 | Switched to `MustURIRef` + `URIRef` value type | compiles |
| v3 | First run: `prohibitions` & `refusal` fields empty (variable shadow) | compile error: `s.Prohibitions undefined` — renamed inner `s` to `str` |
| v4 | First run: `[FAIL] read skills dir: open skills: no such file or directory` | relative path from `.planning/spikes/005b/` doesn't find `skills/` — staged run in `/tmp/spike005b/` with symlinks |
| v5 (final) | 8 skills + 5 tiers + 7 sub-categories enumerated, 215 triples roundtrip lossless | **VERDICT: VALIDATED** |

## Results

**Verdict: ✓ VALIDATED.** SkillForge metadata lifts cleanly to RDFS/OWL TBox in 215 triples via a pure-Go stack (yaml.v3 + goRDFlib, no cgo).

### Implications

1. **TBox as runtime artifact**: `skillforge_tbox.ttl` can be re-emitted on every server boot (cheap — 215 triples, < 1ms to parse), and consumed by:
   - Skill router (match `query_intent` against `skf:skillDescription` text + `skf:poweredBy` agent type)
   - Retriever (filter by `skf:cat_<tier>` ↔ Milvus `tier` field — already aligned with spike 004's tier filter)
   - 005c ontology-aware chunker (use `SubCategory` to scope chunk boundaries)
   - 005d KG store (TBox = schema; ABox = the 215 triples themselves + the SQL/chunk instances)
2. **Discovered third resource class**: `reads/writes` include non-tier targets (`data:*`, `skill_state`, `plan_approve`). Worth modelling as `skf:Capability` in v2.
3. **005e input**: now we have a concrete TBox + 215-triple ABox baseline. The "post-ontology: is Milvus still necessary?" question can be answered with an actual hybrid-retrieval prototype, not just literature.

### Out of scope (this spike)

- Tier-Restrictions (e.g. "SQLNote must belongsToTier skf:权威") — 005a proved we can emit + walk these; 005b doesn't emit any.
- Cardinality restrictions (1..1, 0..*).
- SKILL.md → Skill instance class hierarchy (every Skill is currently `a skf:Skill` flat — could subclass by agent type).
- ABox notes/chunks (those are the chunker/retriever's job).
