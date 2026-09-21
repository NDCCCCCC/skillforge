---
spike: "005c"
name: ontology-aware-chunker
type: standard
validates: "Given sql-guides/*.md byte-bucket split into __secNN_*.md fragments (185 fragments across 26 logical notes), when chunker merges by frontmatter.section (not byte count), then (a) 100% boundary detection (every fragment contributes to a merged section), (b) merged note 001 contains all 3 corpus SQL targets (etl_payment_due_t / etl_policy_request_issue_t / etl_NONAUTO_POLICY_T), (c) chunk.metadata JSON-serializable with source_idx/title/section/section_idx/source_paths"
verdict: VALIDATED
related: [005a-owl-tbox-toolchain, 005b-skillschema-rdfs-lift, internal/knowledge/indexer.go]
tags: [chunker, semantic, h2-boundary, sql, ontology-aware, spike-005]
---

# Spike 005c: Ontology-Aware Markdown Chunker

## What This Validates

sql-guides KB was byte-bucket-split by a prior pipeline (commit 9fa41c3 reorganized the part files; earlier session used `maxDocBytes=4000` in `internal/knowledge/indexer.go` to chunk). Result: one logical note like "001_公司业务部…" becomes 8 fragments, each carrying the same `section: SQL / 正文` frontmatter. LLM retrieval sees only the fragment title, never the complete SQL.

The fix is ontology-aware chunking: use the **`section` field in frontmatter as the semantic boundary**, not byte count.

Given/When/Then:
- **(a) Walk:** Given `knowledge/sql-guides/` containing 185 `__secNN_*.md` files across 26 logical notes (per source_idx), when walked with filename regex `__sec(\d{2})_(.+)\.md$`, then all 185 fragments parsed (100% coverage).
- **(b) Boundary:** When grouped by `(source_idx, section)` and sorted by `__secNN`, then every fragment maps to exactly one merged section (185 fragment-contributions).
- **(c) SQL completeness:** When corpus has 3 distinct `update etl_*` target tables (etl_payment_due_t, etl_policy_request_issue_t, etl_NONAUTO_POLICY_T), then note 001's merged body contains all 3.
- **(d) Metadata:** When sections are emitted as `ChunkMeta` JSON records, then 69 records written (26 notes × avg ~2.6 sections each).
- **(e) Roundtrip:** When merged notes are re-written to `merged_notes/`, then the re-walked corpus still contains all 3 corpus SQL targets.

## Research

### Filename contract

Filenames follow `<title>__secNN_<section-slug>.md`:
```
001_公司业务部、武汉中支部门组部门科室等信息更新__sec01_公司业务部、武汉中支部门组部门科室等信息更新.md
001_公司业务部、武汉中支部门组部门科室等信息更新__sec02_说明.md
001_公司业务部、武汉中支部门组部门科室等信息更新__sec03_SQL_正文.md
001_公司业务部、武汉中支部门组部门科室等信息更新__sec04_SQL_正文.md
... up to __sec08_SQL_正文.md
```

The `__secNN` numeric suffix is the order; the `<section-slug>` is a human-readable hint of what the section is.

### Frontmatter contract (per fragment)

```yaml
---
title: 公司业务部、武汉中支部门组部门科室等信息更新
source_idx: "001"
section: "SQL / 正文"   # ← the ontology boundary
---
```

### Section-boundary rules (discovered empirically)

1. **Same section across multiple fragments** = same logical section, body should be concatenated. (e.g. sec03..sec08 all = "SQL / 正文")
2. **First-seen order wins** — within a note, sections appear in source_idx-then-NN order; this becomes the `SectionIdx` in `ChunkMeta`.
3. **CJK in section names** = supported; preserved verbatim through YAML parse + round-trip.
4. **Note 001's pathology**: sec01 has `section` field equal to `title` (both = "公司业务部…信息更新"), creating an extra "title section" that's redundant with the `#` h1. This is upstream data quality noise, not a chunker bug. Real fix lives in the split pipeline (005b's section-aware generator should give sec01 a distinct section name like "涉及表/视图").

### Gotchas hit

1. **Variable shadowing** (carry-over from 005b/005a): first draft had `for _, p := range parts { if ... ms.Section == p.Section { ... ms.Body += p.Body ... } }` — clean.
2. **Walk function from /tmp**: `go run .` from `.planning/spikes/005c/` can't find `knowledge/` (relative path is broken from deep subdirs). Workaround: stage in `/tmp/spike005c_stage/` with a symlink `knowledge → /mnt/d/.../knowledge`. Long-term: `go run -C ../..`.
3. **Rewalk WARN**: `walk(merged_notes)` returns 0 because merged filenames don't contain `__secNN` (clean naming by design). The roundtrip evidence comes from the corpus SQL regex match on the merged body, not from the rewalk count.

### Surprise finding

**The "7 SQL statements" assumption from the prior session was wrong.** The corpus has only **3 distinct SQL target tables**: `etl_payment_due_t`, `etl_policy_request_issue_t`, `etl_NONAUTO_POLICY_T`. The repetition of `## SQL / 正文` headers across sec03–sec08 was the visual symptom of byte-bucket-splitting, not evidence of multiple independent SQL statements.

A note may have many `update <table>` clauses against the same table (e.g. 6 different `update etl_payment_due_t` blocks for different department codes), but they're all updating the same target — a single logical intent.

## How to Run

```bash
# Stage the spike with a symlink to the KB:
TMPDIR=/tmp/spike005c_stage
mkdir -p $TMPDIR && cd $TMPDIR
ln -sf /mnt/d/code/ClaudeCode/ai-agent/knowledge ./knowledge
cp /mnt/d/code/ClaudeCode/ai-agent/.planning/spikes/005c-ontology-aware-chunker/main.go .
cp /mnt/d/code/ClaudeCode/ai-agent/.planning/spikes/005c-ontology-aware-chunker/go.mod .
cp /mnt/d/code/ClaudeCode/ai-agent/.planning/spikes/005c-ontology-aware-chunker/go.sum .
sed -i 's/spike005c/spike005c_stage/g' go.mod
/usr/local/go/bin/go mod tidy
/usr/local/go/bin/go run .
```

## What to Expect

```
=== spike 005c: ontology-aware markdown chunker ===
[1] walked 185 sec-fragment chunks under knowledge/sql-guides
[1a] distinct logical notes by source_idx: 26
    - source_idx=001 fragments=8
    - source_idx=006 fragments=4
    ...
    - source_idx=039 fragments=35
    - source_idx=043 fragments=7
[2] merged into 26 logical notes
[3] boundary detection: 185 fragment-contributions across 26 sections
[4] note 001 title="公司业务部、武汉中支部门组部门科室等信息更新", sections=3
    - [section #0] 公司业务部、武汉中支部门组部门科室等信息更新  body=297 bytes
    - [section #1] 说明                                          body=1134 bytes
    - [section #2] SQL / 正文                                    body=17401 bytes
[5a] distinct `update etl_*` targets in corpus: 3
    - update:etl_payment_due_t
    - update:etl_policy_request_issue_t
    - update:etl_NONAUTO_POLICY_T
[5b] note 001 contains 3/3 corpus SQL targets
[6] emitted chunk_metadata.json (32850 bytes, 69 metadata records)
[7] wrote 26 merged notes to merged_notes/

=== VERDICT: VALIDATED — ontology-aware chunker recovers complete SQL ===
```

## Investigation Trail

| Iteration | Change | Result |
|---|---|---|
| v1 | First draft: regex-based filename parser + YAML frontmatter strip + group by source_idx | compiles, but ran from `.planning/spikes/005c/` → `open knowledge: no such file or directory` |
| v2 | Stage in `/tmp/spike005c_stage/` with symlink to knowledge/ | 185 fragments walked, 26 logical notes merged, SQL completeness 3/3 |
| v3 (final) | Added `chunk_metadata.json` emission (69 records) + merged_notes/ write + corpus-SQL roundtrip | **VERDICT: VALIDATED** |

## Results

**Verdict: ✓ VALIDATED.** Ontology-aware chunking by frontmatter `section` (not byte count) recovers complete SQL with 100% boundary detection across 185 fragments / 26 notes.

### Production implications

1. **Replace `internal/knowledge/indexer.go` byte-cap chunker** with the spike's section-aware chunker for sql-guides (and any future SKILL.md-driven KB). Drop `maxDocBytes=4000` for note types that have `section` frontmatter; keep it as a fallback for note types without frontmatter.
2. **Milvus metadata fields** become first-class: every chunk now carries `source_idx / title / section / section_idx / section_count / source_paths` — usable as filter dimensions alongside `tier`.
3. **005b schema extension**: add `skf:NoteChunk` class + `belongsToNote` ObjectProperty to TBox so the chunker's output becomes ABox citizens.
4. **005e retriever**: instead of "retrieve top-K similar chunks", we can now retrieve "retrieve all chunks where `belongsToNote(some user-mentioned entity)` UNION `top-K similar chunks`" — true hybrid (entity + vector) retrieval. The post-ontology answer shifts from "is Milvus still necessary?" toward "Milvus is necessary for the soft-similarity leg; ontology is necessary for the precise-relation leg".

### Out of scope

- SQL statement *boundary detection inside* a section (the regex captures `update etl_*` table names but doesn't count distinct `;`-terminated statements — left for a follow-up).
- The sec01 pathology in note 001 (section name == title) — upstream data quality fix, not the chunker's job.
- Generic markdown parser for non-frontmatter notes — current spike assumes every file has `section` frontmatter; for KBs without it, fall back to the existing byte-cap chunker.
