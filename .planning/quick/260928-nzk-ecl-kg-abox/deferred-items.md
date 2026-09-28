# Deferred / Out-of-Scope Discoveries — quick 260928-nzk

## D-NZK-DEF-01: Live SPARQL tier-scoped visibility broken for ALL skills (pre-existing, Phase 41 surface)

**Found during:** quick-260928-nzk Task 4 (E2E live gate), 2026-09-28
**Severity:** high (blocks the positive half of D-NZK-02's live verification; affects all ABox data, not just entities)
**Disposition:** out of scope for this quick task — pre-existing defect in Phase 41/44-owned trust-boundary code; requires a human decision (Rule 4)

### Symptom

`POST /api/sparql` with any live `X-Skill-Name` returns only the schema passthrough (23 rows) for EVERY pattern — ABox rows for notes (`note_*`), tables (`tbl_*`), and the new entities (`ent_*`) are all filtered out. Verified: `p=referencesTable` and `s=note_006` with `X-Skill-Name: sql-guides` return schema-only, while the admin KG query shows the triples exist (e.g. `note_033 referencesTable tbl_etl_policy`).

### Root cause (evidence-backed)

- The KG wires skills as: `skill_sql-guides hasReadAccess → cat_sql-guides` — the object is a **KBCategory individual**, not a Tier individual (category `cat_sql-guides` carries `belongsToTier → sql-guides`).
- `resolveSkillTiersFromKG` (`internal/server/admin_platform.go:604-628`) passes the object through `strings.TrimPrefix(t.O, ns)` → tier name **`"cat_sql-guides"`**.
- `Store.QueryScoped` (`internal/ontology/store/store.go:525-536`) builds `visibleSubjects` by comparing belongsToTier objects (which are Tier IRIs like `<ns>sql-guides`, per D-TIER-02 ABox emission) against `allowedTiers` — `"sql-guides" ∉ {"cat_sql-guides"}` → `visibleSubjects` empty → every ABox row filtered (fail-closed).
- Phase 44's hermetic suite stayed green because tests hand-feed `allowedTiers` like `["sql-guides"]` (`admin_platform_test.go:136`) and never exercise the live resolver wiring.

### Why not fixed here

1. Out-of-scope discovery: pre-existing, in files this task does not own; fail-closed (the security-critical half) HOLDS — no skill sees anything it should not; the defect hides data, it never leaks it.
2. The fix (expand `cat_*` hasReadAccess objects through the category's `belongsToTier` edge, or normalize in the resolver) changes D-TIER trust-boundary resolution semantics of a shipped endpoint — an architectural decision, not an inline fix.
3. Adjacent locked decision exists: Phase 44 deferred **D-SPARQL-05** ("skill hasReadAccess visibility") to a v1.6 follow-up; this finding strengthens the case for that follow-up but re-opening the resolver here would silently widen a deferred decision's scope.

### Suggested follow-up (v1.6 / next quick task)

In `resolveSkillTiersFromKG`, normalize each `hasReadAccess` object: if the store has `(obj, rdf:type, skf:Tier)` keep the stripped name; else if `(obj, belongsToTier, tierIRI)` exists use the tier IRI's stripped name. Add a live-wiring regression test seeding the cat_ shape. Then re-run the 42-03 runbook SPARQL items (L-series) that were never live-executed.
