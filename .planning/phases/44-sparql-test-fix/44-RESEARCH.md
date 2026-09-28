# Phase 44 Research — SPARQL Test Failure Root Cause

**Researched:** 2026-09-28
**Revised:** 2026-09-28 (math table redriven per plan-checker BLOCK verdict; Path C expected counts corrected)
**Domain:** SkillForge SPARQL endpoint (`internal/server/admin_platform.go` + `internal/ontology/store/store.go`)
**Confidence:** HIGH (static analysis of fixture, implementation, and Phase 40 store tests; runtime test invocation not possible in this sandbox — Windows `cgo.exe` exits status 2 before any test code runs)

## 1. Test failure summary (verbatim from CGO build)

> **Sandbox limitation:** runtime test invocation blocked. `cgo.exe` in the embedded Go 1.25.5 toolchain exits status 2 under MSYS2 UCRT64 gcc 16.1.0 (`C:\Users\CPIC\go\pkg\mod\golang.org\toolchain@v0.0.1-go1.25.5.windows-amd64\pkg\tool\windows_amd64\cgo.exe: exit status 2`). WSL Ubuntu has no `go` binary installed. The Phase 43-01 runbook uses a Linux gcc with matching emutls ABI; that environment is not reachable here.

**Expected runtime output** (derived by tracing `SparqlQueryScoped` and `QueryScoped` byte-for-byte against the fixture below):

| Test | Expected | Actual (per static trace) | Failure mode |
|------|----------|---------------------------|--------------|
| `TestSparqlQueryScoped_AllWildcard` | 16 | **22** | count (test author miscounted) |
| `TestSparqlQueryScoped_SubjectFilter` (S=note_001, nil) | 3 | **4** | count (test author miscounted; inline comment already says 4) |
| `TestSparqlQueryScoped_Truncated` (1st, Limit=3) | 16 / TRUE / 3 | **22 / TRUE / 3** | count |
| `TestSparqlQueryScoped_Truncated` (2nd, Offset=20) | 16 / FALSE / 0 | **22 / FALSE / 0** | count + len |
| `TestSparqlQueryScoped_TierFilter` | 13 (passes), no note_002/note_003 | **15 (after fix) / no note_002/note_003** | count (test author treated label-of-visible-ABox as schema) |
| `TestSparqlQueryScoped_FailClosedNoBelongsToTier` (nil tier) | 2 | 2 | passes |
| `TestSparqlQueryScoped_FailClosedNoBelongsToTier` (sql-guides) | 0 | **0 (after fix; was 1 pre-fix)** | count + fail-closed |
| `TestSparqlQueryScoped_LimitCap` | 16 / FALSE / 16 | **22 / FALSE / 22** | count + len |
| `TestSparqlQueryScoped_DefaultLimit` | 16 / FALSE / 16 | **22 / FALSE / 22** | count + len |
| `TestSparqlQueryScoped_Pagination` | 16 / TRUE / 5 | **22 / TRUE / 5** | count |

The 8 listed failures split into three root-cause buckets (see §3 and §5):

- **Bucket A — nil-tier path returns raw triples with duplicates** (6 tests after Revision: `AllWildcard`, `Truncated` x2, `LimitCap`, `DefaultLimit`, `Pagination`; all expect 16, get 22 unique after fixture fix + dedup)
- **Bucket B — SubjectFilter asserts 3 but fixture inserts 4** (1 test, off-by-one in the assertion only — test author's inline comment at line 106 already says 4. REVISION NOTE: this is INDEPENDENT of Bucket C fix because SubjectFilter uses `nil` allowedTiers, so `QueryScoped` early-returns to `s.Query` which never consults `isSchemaTriple`)
- **Bucket C — `isSchemaTriple` treats `label` predicate as always-schema, leaking ABox labels through tier filter** (2 tests: `TierFilter` count, `FailClosedNoBelongsToTier` sql-guides branch)

## 2. Fixture audit

`internal/server/admin_platform_test.go:42-84` inserts **23 raw `Triple` values** via `s.Insert(...)` (which does NOT dedup — `internal/ontology/store/store.go:117`).

| # | Line | Subject | Predicate | Object | Schema? | Visible @ `["sql-guides"]`? |
|---|------|---------|-----------|--------|---------|------------------------------|
| 1 | 46 | `testNS+"Class"` | `rdfType` | `testNS+"owl:Class"` | NO — `isSchemaTriple` requires `O==ns+"Class"` (`http://...#Class`); fixture has `http://...#owl:Class` (store.go:570-575) | NO (subject not in `visibleSubjects`) |
| 2 | 47 | `testNS+"ObjectProperty"` | `rdfType` | `testNS+"owl:ObjectProperty"` | NO — same IRI mismatch | NO |
| 3 | 48 | `testNS+"Skill"` | `rdfType` | `testNS+"owl:Class"` | NO — same IRI mismatch | NO |
| 4 | 49 | `testNS+"Tier"` | `rdfType` | `testNS+"owl:Class"` | NO — same IRI mismatch | NO |
| 5 | 52 | `testNS+"skill_sql"` | `rdfType` | `skfSkill` (`ns+"Skill"`) | YES — `schemaClasses[ns+"Skill"]` (store.go:600) | YES (always-schema) |
| 6 | 53 | `testNS+"skill_sql"` | `ns+"hasReadAccess"` | `testNS+"sql-guides"` | NO — predicate not in schema predicate list | NO |
| 7 | 54 | `testNS+"skill_compliance"` | `rdfType` | `skfSkill` | YES | YES |
| 8 | 55 | `testNS+"skill_compliance"` | `ns+"hasReadAccess"` | `testNS+"authority"` | NO | NO |
| 9 | 58 | `testNS+"Tier"` | `rdfType` | `testNS+"owl:Class"` | NO (same as #1) — also **DUPLICATE of #4** (same S,P,O) | NO |
| 10 | 59 | `testNS+"sql-guides"` | `rdfType` | `skfTier` (`ns+"Tier"`) | YES — `schemaClasses[ns+"Tier"]` | YES |
| 11 | 60 | `testNS+"authority"` | `rdfType` | `skfTier` | YES | YES |
| 12 | 64 | `testNS+"note_001"` | `rdfType` | `skfNote` (`ns+"NoteChunk"`) | NO — `NoteChunk` NOT in `schemaClasses` (intentional, store.go:616-617 comment) | YES (subject in `visibleSubjects`) |
| 13 | 65 | `testNS+"note_001"` | `ns+"belongsToTier"` | `testNS+"sql-guides"` | NO — predicate not in schema predicate list | YES |
| 14 | 66 | `testNS+"note_001"` | `ns+"label"` | `"SQL指南第一章"` | **YES via predicate check** (store.go:585) — pre-fix | YES (always-schema) — pre-fix |
| 15 | 67 | `testNS+"note_001"` | `ns+"referencesTable"` | `testNS+"tbl_payment_t"` | NO | YES |
| 16 | 70 | `testNS+"note_002"` | `rdfType` | `skfNote` | NO | NO (subject not visible — tier=`authority`) |
| 17 | 71 | `testNS+"note_002"` | `ns+"belongsToTier"` | `testNS+"authority"` | NO | NO |
| 18 | 72 | `testNS+"note_002"` | `ns+"label"` | `"合规政策V2"` | **YES via predicate check** — pre-fix | **YES (LEAK)** — pre-fix |
| 19 | 75 | `testNS+"note_003"` | `rdfType` | `skfNote` | NO | NO (subject has no `belongsToTier`) |
| 20 | 76 | `testNS+"note_003"` | `ns+"label"` | `"无归属笔记"` | **YES via predicate check** — pre-fix | **YES (LEAK — fail-closed violated)** — pre-fix |
| 21 | 79 | `testNS+"tbl_payment_t"` | `rdfType` | `skfTable` | NO — `TableSchema` NOT in `schemaClasses` | YES |
| 22 | 80 | `testNS+"tbl_payment_t"` | `ns+"belongsToTier"` | `testNS+"sql-guides"` | NO | YES |
| 23 | 81 | `testNS+"tbl_payment_t"` | `ns+"label"` | `"支付表"` | **YES via predicate check** — pre-fix | YES |

**Counts (REDRIVEN per Revision 3):**
- Raw `Insert` calls: **23**
- Unique triples (after S|P|O dedup): **22** (#9 = #4 dup)
- After Tasks 3+4 (owl-IRI fix + duplicate removal): fixture produces **22 unique triples** (the duplicate becomes a single triple after D-SPARQL-02 dedup is applied during query)
- `isSchemaTriple` returns true at runtime (post D-SPARQL-03): 8 (#1, #2, #3, #4 after owl-IRI fix via rdf:type+Class/ObjectProperty arm; #5, #7, #10, #11 via schemaClasses arm)
- ABox triples (post D-SPARQL-03, subject-visibility-checked on Path B): 14 (#6, #8, #12-#15, #16-#18, #19-#20, #21-#23)
- **Path A (nil allowedTiers) total**: ALL 22 unique triples (no filtering applied)
- **Path B (allowedTiers=["sql-guides"]) total**: 8 schema + 4 note_001 (incl. label) + 3 tbl_payment_t (incl. label) = **15**

**Fixture-count errors in previous version of this section (REVISION NOTE):**

The previous version claimed: "Raw `Insert` calls: 23" → "Unique triples: 22" → then silently divided into "8 schema + 14 ABox = 22" which is correct, but the "expected Path A = 16" derivation skipped 6 triples (2 hasReadAccess, 2 note_002 triples (rdf:type + belongsToTier), 2 note_003 triples... wait, note_003 only has 2 triples so all 2 are included). The correct Path A count is **22**, not 16. The previous research incorrectly reported 16.

The new Section 5 ("Recommended fix path") and the new Section 6 ("Decision locks") below reflect this correction.

## 3. Implementation behavior

### 3.1 `SparqlQueryScoped` (admin_platform.go:189-252)

```
1. validate Limit/Offset >= 0
2. default Limit=50; cap at 500
3. triples := kg.QueryScoped(S, P, O, allowedTiers)
4. optional substring filter on S/P/O (case-insensitive)
5. count := len(triples)
6. page := triples[offset:min(offset+limit, count)]
7. Truncated := count > offset+limit
```

All `Count` failures trace to step 3 (QueryScoped returning more than expected). Step 7 logic is correct.

### 3.2 `Store.Query` (store.go:273-306)

- No dedup. When all three of S,P,O are empty, returns `s.all` verbatim via the `default` branch (line 286-289) — including the raw duplicate at line 58.

### 3.3 `Store.QueryScoped` (store.go:487-566)

Two paths:

**Path A — `allowedTiers` is nil or empty** (line 492-494):
```go
if len(allowedTiers) == 0 {
    return s.Query(subj, pred, obj)
}
```
- Returns ALL triples matching pattern, INCLUDING duplicates.
- No `seen` dedup applied.
- **REVISION NOTE:** `isSchemaTriple` is NEVER consulted on this path. This is why SubjectFilter (S=note_001, nil) returns 4 triples (rdf:type + belongsToTier + label + referencesTable) regardless of the D-SPARQL-03 fix to `isSchemaTriple`.
- This is what fires for `AllWildcard`, `SubjectFilter`, `Truncated`, `LimitCap`, `DefaultLimit`, `Pagination` (all pass `nil` for allowedTiers).

**Path B — `allowedTiers` non-empty** (line 496-554):
- Builds `allowedTierSet` from `allowedTiers` (short names).
- Walks `s.byP[ns+"belongsToTier"]` to build `visibleSubjects` (D-TIER-06).
- Iterates `s.all` and applies:
  1. `seen` dedup by `S|P|O` (line 517-524).
  2. `isSchemaTriple(t)` short-circuit (line 527-531) — schema triples are added unconditionally regardless of `visibleSubjects`.
  3. ABox visibility check (line 533-536) — skip if `t.S` not in `visibleSubjects`.
  4. S/P/O pattern filter (line 539-547).

This is what fires for `TierFilter` and `FailClosedNoBelongsToTier` (sql-guides branch).

### 3.4 `isSchemaTriple` (store.go:568-592) — the trust leak

```go
if t.P == rdfType {
        if t.O == ns+"Class" || t.O == ns+"ObjectProperty" ||
            t.O == ns+"DatatypeProperty" || t.O == ns+"Property" ||
            t.O == ns+"NamedIndividual" {
            return true
        }
        if schemaClasses[t.O] {            // ns+"Skill", ns+"Tier", etc.
            return true
        }
}

// Ontology metadata predicates — always schema layer.
if t.P == ns+"label" || t.P == ns+"comment" ||
    t.P == ns+"domain" || t.P == ns+"range" ||
    t.P == ns+"subClassOf" || t.P == ns+"subPropertyOf" {
    return true
}
```

The predicate-based schema gate treats **any** triple with `label`/`comment`/`domain`/`range`/`subClassOf`/`subPropertyOf` predicate as schema, regardless of subject. This is **inconsistent with the D-TIER-03 docstring** above (line 564-567), which says schema layer = TBox declarations + Skill/KBCategory/etc individuals.

For `label`: if `note_003` (no `belongsToTier`) has a `label` triple, that triple is returned by `QueryScoped(..., ["sql-guides"])` even though `note_003` itself is fail-closed. The `TestSparqlQueryScoped_FailClosedNoBelongsToTier` test (line 168-180) directly asserts the opposite behavior — **this is a trust-boundary violation, not just a test bug.**

`TestQueryScopedFailClosedNoBelongsToTier` in store_test.go:615-634 does NOT exercise this path because it uses the `referencesTable` predicate (which is not in the schema predicate list).

### 3.5 Inconsistency between `Query` and `QueryScoped`

| API | Dedup behavior | Trust semantics |
|-----|----------------|-----------------|
| `Query` (admin baseline) | NO — returns raw duplicates | "all" |
| `QueryScoped(allowedTiers=nil)` | NO — delegates to `Query` | "all" |
| `QueryScoped(allowedTiers=non-empty)` | YES via `seen` map (store.go:517) | filtered |

The user prompt correctly flags this inconsistency. It is defensible (admin baseline is "show everything, even if malformed") but it's the proximate cause of the count mismatch in 6/8 tests.

## 4. Schema-triple classification (current implementation)

| Predicate pattern | Object pattern | Schema? | Notes |
|-------------------|----------------|---------|-------|
| `rdfType` | `ns+"Class"` / `ns+"ObjectProperty"` / `ns+"DatatypeProperty"` / `ns+"Property"` / `ns+"NamedIndividual"` | YES | lines 570-575 |
| `rdfType` | `schemaClasses[O]` (`Skill`/`Tier`/`KBCategory`/`SubCategory`/`SkillAgent`/`RefusalPhrase`) | YES | line 579 |
| `rdfType` | anything else (e.g., `NoteChunk`, `TableSchema`, `owl:Class`, `owl:ObjectProperty` from fixture) | NO | ABox — also fixtures bug |
| `ns+"label"` | ANY | **YES (BUG)** | line 585 — leaks ABox labels |
| `ns+"comment"` | ANY | **YES (potential BUG)** | line 585 — not exercised by SPARQL tests |
| `ns+"domain"` / `ns+"range"` / `ns+"subClassOf"` / `ns+"subPropertyOf"` | ANY | YES (likely intentional — ontology metadata) | lines 586-587 |
| anything else | — | NO | ABox — subject visibility applied |

**Fixture IRI mismatch** (`owl:Class` / `owl:ObjectProperty`):
The fixture (lines 46-49) uses `testNS+"owl:Class"` and `testNS+"owl:ObjectProperty"` as OBJECT values, but `isSchemaTriple` requires `ns+"Class"` (without `owl:` prefix) as OBJECT. This is intentional in the store (it's checking the in-house ontology namespace's class IRIs, not the OWL vocabulary IRIs), so the fixture is wrong.

The production TBox emitter (`internal/ontology/emitter`) emits `(skf:Class, rdf:type, skf:Class)` style triples — NOT `(skf:Class, rdf:type, owl:Class)`. The fixture author appears to have copy-pasted a Turtle TTL example that used the OWL vocabulary.

## 5. Recommended fix path

**Recommended: Path C (both fixture AND implementation).**

Path A (fixture-only, change `16 → 22`) preserves the implementation but **leaves the trust-boundary violations in place** (Bucket C failures). Path B (implementation-only) makes the wrong test expectations in `SubjectFilter` and Bucket A counts impossible to reconcile. The user's "Path A" suggestion in the prompt only addresses the count math; the trust-violation tests (`FailClosedNoBelongsToTier`, `TierFilter` negative assertions) cannot be made green without touching `isSchemaTriple`.

### Path C — concrete changes

#### C.1 Implementation changes (`internal/ontology/store/store.go`)

**C.1.1 Add dedup to nil-tier path** (store.go:492-494).

Replace the early-return with a dedup-aware call:

```go
// D-TIER-01: empty tiers = no filtering (admin baseline).
if len(allowedTiers) == 0 {
    // Use the same dedup logic as Path B so admin baseline doesn't
    // return raw duplicates that the tier path would have collapsed.
    if subj == "" && pred == "" && obj == "" {
        seen := map[string]bool{}
        out := make([]Triple, 0, len(s.all))
        for _, t := range s.all {
            key := t.S + "|" + t.P + "|" + t.O
            if seen[key] { continue }
            seen[key] = true
            out = append(out, t)
        }
        return out
    }
    return s.Query(subj, pred, obj)
}
```

This eliminates the duplicate at fixture line 58 without changing fixture semantics — `Insert` deliberately does not dedup, so the store should.

**C.1.2 Remove `label`/`comment` from always-schema predicate list** (store.go:585).

`label` and `comment` should be schema only when the subject is itself a schema individual. Two acceptable options:

**Option α (subject-based, no API change):** add a `schemaSubjects` set populated at `Insert` time whenever `(s, rdf:type, o)` with `schemaClasses[o]` is inserted; `isSchemaTriple` then returns true if `t.S ∈ schemaSubjects` OR `t.P ∈ {domain, range, subClassOf, subPropertyOf}`.

**Option β (drop label/trade predicate, keep subject-based via in-place subject check):** drop the predicate clause entirely and rely on subject being a schema individual (the `rdfType`+`schemaClasses` arm already pulls in those individuals' `rdf:type` triples; for their label/comment, the subject check pulls them in).

Option β is the smaller patch:

```go
// Ontology metadata predicates — applied only when subject is schema.
if t.P == ns+"domain" || t.P == ns+"range" ||
    t.P == ns+"subClassOf" || t.P == ns+"subPropertyOf" {
    return true
}
// label/comment fall through to ABox path: subject must be visible.
```

This preserves schema-subject labels (e.g., `sql-guides rdfs:label "SQL guides"` would be schema only if `sql-guides` is itself a schema individual — which it is via `rdf:type skf:Tier`). It also leaks zero labels for ABox subjects that are fail-closed.

#### C.2 Fixture changes (`internal/server/admin_platform_test.go`)

**C.2.1 Fix owl-IRI mismatch** (lines 46-49). Change `testNS+"owl:Class"` → `testNS+"Class"`, `testNS+"owl:ObjectProperty"` → `testNS+"ObjectProperty"`. Four single-token edits.

**C.2.2 Remove duplicate at line 58.** Either delete the line or replace with `s.InsertUnique(...)`.

**C.2.3 Update `SubjectFilter` count** (line 106). **REVISION NOTE:** With C.1.2 applied, `note_001` STILL returns 4 triples (rdf:type, belongsToTier, referencesTable, label) — because `SubjectFilter` uses `nil` allowedTiers so `QueryScoped` delegates to `s.Query(note_001, "", "")` which never consults `isSchemaTriple`. **The assertion MUST be updated to 4** (per D-SPARQL-04b row 2 and D-SPARQL-08). This is independent of C.1.2.

#### C.3 Expected results after Path C (REDRIVEN per Revision 3)

With both C.1.1 (dedup), C.1.2 (label-as-ABox), C.2.1 (IRI fix), and C.2.2 (duplicate removal) applied:

| Test | Expected | After Path C | Match? |
|------|----------|--------------|--------|
| `AllWildcard` (Path A, nil) | 16 | **22** (4 schema defs + 4 schema individuals + 2 hasReadAccess + 4 note_001 + 3 note_002 + 2 note_003 + 3 tbl_payment_t) | **NO — assertion update required (16→22)** |
| `SubjectFilter` (note_001, nil) | 3 | **4** (rdf:type + belongsToTier + label + referencesTable — Path A delegates to `s.Query`, never consults `isSchemaTriple`) | **NO — assertion update required (3→4) INDEPENDENT of C.1.2** |
| `Pagination` (Path A, nil) | 16 | **22** | **NO — assertion update required (16→22)** |
| `Truncated` (Limit=3, Path A) | 16/TRUE/3 | **22/TRUE/3** | **NO — assertion update required (16→22)** |
| `Truncated` (Limit=5, Offset=20, Path A) | 16/FALSE/0 | **22/FALSE/0** | **NO — assertion update required (16→22)** |
| `TierFilter` (sql-guides) | 13, no note_002/note_003 | **15 (8 schema + 4 note_001 incl. label + 3 tbl_payment_t incl. label)**, no note_002/note_003 | **NO — assertion update required (13→15)** |
| `FailClosedNoBelongsToTier` (nil) | 2 | 2 (note_003 rdf:type + label) | YES |
| `FailClosedNoBelongsToTier` (sql-guides) | 0 | 0 (note_003 label no longer schema; subject not visible) | YES |
| `LimitCap` (Path A, nil) | 16/FALSE/16 | **22/FALSE/22** | **NO — assertion update required (16→22)** |
| `DefaultLimit` (Path A, nil) | 16/FALSE/16 | **22/FALSE/22** | **NO — assertion update required (16→22)** |
| `Filter` (Path A) | 1/1/0 | 1/1/0 | YES |
| `InvalidLimit` | error | error | YES |

**Path A (nil allowedTiers) total = 22 unique fixture triples derivation:**
- 4 schema class definitions (after D-SPARQL-04 IRI fix: Class/ObjectProperty/Skill/Tier rdf:type)
- 4 schema individuals (skill_sql/skill_compliance rdf:type skf:Skill + sql-guides/authority rdf:type skf:Tier)
- 2 skill hasReadAccess triples (skill_sql→sql-guides, skill_compliance→authority) — NOT schema (predicate not in schema list, subject not in visibleSubjects) but INCLUDED on Path A (no filter)
- 4 note_001 triples (rdf:type + belongsToTier + label + referencesTable) — all visible on Path A
- 3 note_002 triples (rdf:type + belongsToTier + label) — all visible on Path A (note_002's authority membership doesn't matter on Path A)
- 2 note_003 triples (rdf:type + label) — all visible on Path A (no belongsToTier doesn't matter on Path A)
- 3 tbl_payment_t triples (rdf:type + belongsToTier + label) — all visible on Path A
- = **22 unique triples** ✓

**Path B (allowedTiers=["sql-guides"]) total = 15 derivation:**
- Schema (always visible, D-TIER-03): 4 class defs + 4 individuals = 8
- ABox (subject in visibleSubjects={note_001, tbl_payment_t, sql-guides, authority, skill_sql, skill_compliance}):
  - note_001: rdf:type + belongsToTier + label + referencesTable = 4
  - tbl_payment_t: rdf:type + belongsToTier + label = 3
  - note_002: subject not visible (authority tier ≠ sql-guides) → EXCLUDED
  - note_003: subject not visible (no belongsToTier) → EXCLUDED (fail-closed)
  - skill_sql/skill_compliance: subject is in visibleSubjects (schema individual) but no ABox triples besides rdf:type which is schema; hasReadAccess is ABox but predicate not schema AND subject in visibleSubjects... wait, are skill_sql/skill_compliance in visibleSubjects?

**REVISION CORRECTION (CRITICAL):** Re-checking Path B subject visibility. `visibleSubjects` is built from `s.byP[ns+"belongsToTier"]` (store.go:496-510), which collects subjects that have a `belongsToTier` triple. Skill individuals (skill_sql, skill_compliance) have NO `belongsToTier` triple (they have `hasReadAccess` instead). Therefore:
- skill_sql: NOT in `visibleSubjects` → hasReadAccess triple (subject is skill_sql, predicate not in schema list) → EXCLUDED on Path B
- skill_compliance: NOT in `visibleSubjects` → hasReadAccess triple → EXCLUDED on Path B
- sql-guides, authority: schema individuals (rdf:type skf:Tier), but they are NOT in `visibleSubjects` either (no belongsToTier for Tier individuals). Their only triples are rdf:type (schema via schemaClasses arm) — already counted in schema total.
- note_001: in visibleSubjects (belongsToTier sql-guides) → all 4 ABox triples included
- tbl_payment_t: in visibleSubjects (belongsToTier sql-guides) → all 3 ABox triples included
- note_002: in visibleSubjects (belongsToTier authority) — but allowedTiers is ["sql-guides"] not ["authority"], so note_002 is filtered by `allowedTierSet` check on line 533-536 (the visibility check: `t.S` must be in `visibleSubjects` AND the subject's tier must be in `allowedTiers`). EXCLUDED.

So Path B ABox count = 4 (note_001) + 3 (tbl_payment_t) = 7. Total = 8 + 7 = **15**. Confirmed.

**Path A vs Path B summary:**
- Path A returns ALL 22 unique triples (no filtering).
- Path B returns 15 (8 schema + 7 ABox for sql-guides).
- Path B with S=note_003 returns 0 (subject fail-closed).
- Path A with S=note_003 returns 2 (no filter, both triples for note_003).
- Path A with S=note_001 returns 4 (no filter, all 4 triples for note_001).

> **Caveat:** skill `hasReadAccess` is NOT schema after C.1.2 because its subject `skill_sql` is not a schema individual in `isSchemaTriple`'s view (rdf:type matches `schemaClasses` only via the O-equals-schemaClass arm, not via subject). This matches the test author's mental model (skill hasReadAccess is "skill metadata", neither schema nor user-data ABox) and matches Phase 40's `TestQueryScopedSchemaLayerNeverFiltered` scope (line 571-590 checks only `rdf:type skf:Skill` passes through, not `hasReadAccess`).

### Path A alternative (NOT recommended for closure)

- Remove duplicate at line 58, fix owl IRIs at lines 46-49.
- Update count expectations: `AllWildcard`, `Truncated`, `LimitCap`, `DefaultLimit`, `Pagination` 16 → 22.
- Update `SubjectFilter` 3 → 4.
- **CANNOT fix `FailClosedNoBelongsToTier` sql-guides branch** — label leak is an implementation bug.
- **CANNOT fix `TierFilter` negative assertions** — same root cause.

This path violates the prompt constraint: "without weakening tier-filtering semantics or trust boundaries".

### Path B alternative (NOT recommended)

- Add dedup to nil-tier path.
- Remove label predicate from `isSchemaTriple`.
- Update `SubjectFilter` 3 → 4 (Path A — independent of label-as-ABox change).
- Update `AllWildcard` count 16 → ?? (depends on whether skill `hasReadAccess` is schema or not).

This path requires counting carefully against the locked Phase 40 semantics and may conflict with the `D-TIER-03` docstring.

## 6. Decision locks needed

- **D-SPARQL-02:** `Store.QueryScoped` with `allowedTiers=nil` MUST dedup by `S|P|O` before returning. Closes Bucket A (6 tests on Path A).
- **D-SPARQL-03:** `isSchemaTriple` predicate gate for `label`/`comment` is REMOVED. Labels are schema only when the subject is a schema individual (rdf:type matches `schemaClasses[O]`). Closes Bucket C and the fail-closed trust violation. Reaffirms `D-TIER-03` (Phase 40-04 ROADMAP.md:196).
- **D-SPARQL-04:** Fixture cleanup is non-semantic — fix owl-IRI mismatch at admin_platform_test.go:46-49 and remove duplicate at line 58. Test expectations MUST be updated (per D-SPARQL-08) to match the actual 22-unique-triple fixture.
- **D-SPARQL-04b (REVISION EXTENDED):** **7** test-author assertion miscounts MUST be updated:
  - `AllWildcard` line 94, 96: `16 → 22`
  - `SubjectFilter` line 106, 107: `3 → 4` (independent of Bucket C fix — Path A never consults `isSchemaTriple`)
  - `Pagination` line 121: `16 → 22`
  - `Truncated` line 132: `16 → 22`
  - `Truncated` line 140: `16 → 22`
  - `TierFilter` line 158: `13 → 15`
  - `LimitCap` line 224, 226: `16 → 22`
  - `DefaultLimit` line 236, 238: `16 → 22`
- **D-SPARQL-05 (deferred / discuss-phase):** Whether skill `hasReadAccess` triples are "schema layer visible to admin baseline" or "skill metadata excluded from SPARQL response". Phase 40 locked decision does not explicitly cover it; current implementation includes them as ABox (subject not visible). Path C as drafted excludes them from admin baseline too. If maintainer wants them visible, switch to C.1.2 Option α.
- **D-SPARQL-08 (NEW per Revision):** Test-author assertion miscounts are valid closure-phase scope when the assertion contradicts the actual fixture triple count. Update with godoc rationale documenting the count derivation; do not silently change implementation to match.

## 7. Risks + open questions

### 7.1 Sandbox cannot verify the fix at runtime

`cgo.exe` in the embedded Go 1.25.5 toolchain is broken on this Windows host (exits status 2). The Phase 43-01 closure path used a Linux gcc with matching emutls ABI. **Operator must re-run `go test -run TestSparqlQueryScoped -v` under the Phase 43-01 build environment** to confirm Bucket A/B/C actually turn green after the patch.

### 7.2 `comment` predicate has the same trust leak as `label`

The fix removes `label` from `isSchemaTriple`'s predicate gate (C.1.2). If the maintainer wants `comment` to behave the same way (fail-closed ABox comments excluded), the fix should remove it too. Current SPARQL tests do not exercise `comment`, so this is a latent risk, not a blocking failure. Recommend grouping the cleanup in the same patch.

### 7.3 Subject-based schema detection is per-triple O(1) extra lookup

If C.1.2 Option α is chosen (full subject-based detection for all schema predicates), `isSchemaTriple` will perform one extra `byS` lookup per triple. At KG scale (<10k triples, per `DeleteSubject` comment line 161) this is negligible. For the production hot path (chat sessions → SPARQL queries at most a few per request) it is invisible.

### 7.4 HybridRetrieval KG expansion is also affected by `isSchemaTriple`

`HybridSearchScoped` (admin_platform.go:348-547) calls `kg.QueryScoped(...)` for label and tier collection (lines 389-390, 440, 456, 473). Removing `label` from `isSchemaTriple` means label triples for non-visible ABox subjects will NOT be returned by these calls. This is the **correct** behavior (we want labels for visible entities only), but worth confirming the `HybridKGNode.Label` field in the FE does not silently degrade. The current code (admin_platform.go:441) falls back to the entity IRI as label when no label triple is found, so the behavior is graceful.

### 7.5 `TestQueryScopedVisibleEntityAllEdgesIncluded` may shift after C.1.2

store_test.go:636-664 inserts no label/comment/domain/range/subClassOf/subPropertyOf triples, so its assertions remain green. But the broader Phase 40 hermetic gate (`verify-phase-40.sh`) should be re-run to confirm no regression.

## 8. Source of truth references

| Claim | Source |
|-------|--------|
| 8 failing tests list | `.planning/STATE.md` § "v1.5 Phase 43 Discovery" (lines 113-115) |
| Fixture triple-by-triple | `internal/server/admin_platform_test.go:42-84` |
| Test expectations | `internal/server/admin_platform_test.go:88-239` |
| `SparqlQueryScoped` logic | `internal/server/admin_platform.go:189-252` |
| `Store.QueryScoped` paths | `internal/ontology/store/store.go:487-566` |
| `Store.Query` (no dedup) | `internal/ontology/store/store.go:273-306` |
| `isSchemaTriple` (label leak) | `internal/ontology/store/store.go:568-592` |
| `schemaClasses` (NoteChunk NOT included) | `internal/ontology/store/store.go:597-619` |
| `Insert` (no dedup) vs `InsertUnique` (dedup) | `internal/ontology/store/store.go:117-150` |
| Phase 40 trust semantics D-TIER-01/03/06/07 | `ROADMAP.md:196` |
| Phase 40 QueryScoped test patterns | `internal/ontology/store/store_test.go:569-744` |
| Phase 41 SPARQL original plan | `.planning/phases/41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval/41-02-PLAN.md` |
| Phase 41 SPARQL decisions summary | `.planning/phases/41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval/41-02-SUMMARY.md` |

## 9. Metadata

**Confidence breakdown:**
- Fixture triple count (23 raw / 22 unique): HIGH (line-by-line table above)
- Implementation behavior (Path A returns 22 raw after fixture fix + dedup, Path B returns 15 after isSchemaTriple fix): HIGH (full read of `QueryScoped`, `Query`, `isSchemaTriple`)
- Trust-boundary violation via `label` predicate: HIGH (Bucket C traced end-to-end)
- Bucket A nil-tier path: HIGH (line 492-494 is unambiguous early-return)
- Path C expected outputs (22 / 4 / 15): HIGH (re-derived from the trace above)
- Runtime test outputs: LOW (sandbox cannot run; static-trace substitutes)

**Research date:** 2026-09-28
**Revised:** 2026-09-28 (Path C expected outputs table redriven per plan-checker BLOCK verdict)
**Valid until:** 30 days (stable — no upstream library churn expected)

**Operator action required:** Phase 44 plan executor must run `CGO_ENABLED=1 go test ./internal/server/ -run TestSparqlQueryScoped -v -count=1` under the Phase 43-01 build environment (Linux gcc with emutls ABI) to confirm Bucket A/B/C actually turn green. Sandbox here cannot execute `cgo.exe` so the static trace stands in for runtime confirmation.