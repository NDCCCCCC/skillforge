# Phase 39: ECL Pipeline — Section-aware Chunker + LLM Entity Extraction - Pattern Map

**Mapped:** 2026-09-21
**Files analyzed:** 22 (13 new + 9 modified, incl. tests)
**Analogs found:** 19 / 22 (3 have no in-repo analog; RESEARCH.md patterns cover them)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/knowledge/ecl/merge.go` | service | file-I/O + transform | `.planning/spikes/005c-ontology-aware-chunker/main.go` (walk L117-153, mergeByNote L155-199) | exact (sanctioned port) |
| `internal/knowledge/ecl/meta.go` | model | transform | spike `main.go` ChunkMeta L264-284 | exact |
| `internal/knowledge/ecl/chunk.go` | service | transform | `internal/retriever/milvus/seed.go` SeedDocuments column mapping L139-167 + `internal/knowledge/indexer.go` doc build L211-266 | role-match |
| `internal/knowledge/ecl/extract.go` | service | request-response (LLM) | `internal/ontology/induction/induction.go` Generate L128-188 + `internal/llm/extract.go` | exact |
| `internal/knowledge/ecl/f1.go` | utility | transform (pure) | none (pure set math) | none — use RESEARCH Pattern 5 |
| `internal/knowledge/ecl/manager.go` | service | event-driven (stage orchestrator) | `internal/ontology/induction/induction.go` Manager (mu + onChange L77-124) + `admin_ontology.go` 409 posture | role-match |
| `internal/knowledge/ecl/abox.go` | service | CRUD (KG triples) | `induction.go` Approve KG-insert loop L248-253 + `internal/ontology/store/store.go` Insert L67-77 | exact |
| `internal/knowledge/ecl/failures.go` | model (narrow interface) | CRUD (SQLite) | `induction.go` Generator narrow-interface precedent L49-53 + `internal/store/sqlite/` store pattern | role-match |
| `internal/knowledge/indexer.go` (MOD) | service | file-I/O | itself — per-file loop L211-229, seedDocs purge seam L443-451 | exact (self) |
| `internal/retriever/milvus/seed.go` (MOD) | model/storage | CRUD | itself — schema L70-88, SeedDocuments L129-180 | exact (self) |
| `internal/retriever/milvus/delete_expr.go` (NEW) | utility | CRUD (delete) | `seed.go` DeleteTierRows mechanics L216-252 | exact |
| `internal/ontology/store/store.go` (MOD) | model | CRUD | itself — Insert/Query indices L67-121 | exact (self) |
| `internal/ontology/emitter/emitter.go` (MOD) | config/model | transform | itself — classes L202-209, objProps L215-225 | exact (self) |
| `internal/store/sqlite/schema.go` (MOD) | migration | CRUD | itself — schemaSQL L51+ / migrate-tail convention L590-694 | exact (self) |
| `internal/server/admin_pipeline.go` (NEW) | controller | request-response | `internal/server/admin_ontology.go` (whole file) | exact |
| `cmd/server/main.go` (MOD) | config/boot | event-driven | itself — ontology boot L864-914, RECREATE gate L1120-1148 | exact (self) |
| `internal/server/server.go` (MOD) | config | — | itself — Deps.Ontology L461-467, route block L655-662 | exact (self) |
| `web/src/components/admin/pipeline/PipelineTab.tsx` (NEW) | component | event-driven (polling) | `web/src/components/admin/CapacityMonitor.tsx` + `web/src/hooks/useEvalReports.ts` + `AdminDashboard.tsx` tab registration | role-match |
| `web/src/components/admin/pipeline/pipelineApi.ts` (NEW) | utility (FE) | request-response | `CapacityMonitor.tsx` fetch block L76-96 (X-Admin-Token + localStorage) | role-match |
| `web/src/components/admin/pipeline/types.ts` (NEW) | model (FE) | — | — | none (trivial DTO mirror of status endpoint) |
| `eval/ecl-golden/sql-guides.jsonl` (NEW) | data | batch | `eval/golden/*.jsonl` convention (6 existing files) | role-match |
| `scripts/verify-phase-39.sh` (NEW) | test/verify | batch | `scripts/verify-phase-38.sh` (whole file) | exact |

## Pattern Assignments

### `internal/knowledge/ecl/merge.go` + `meta.go` (service/model, file-I/O)

**Analog:** `.planning/spikes/005c-ontology-aware-chunker/main.go` — VALIDATED spike; productionize as-is (D-ECL-02 "原样生产化").

**Filename regex** (spike main.go:57):
```go
var secRegex = regexp.MustCompile(`__sec(\d{2})_(.+)\.md$`)
```

**Walk + frontmatter parse** (spike main.go:117-153, parseFrontmatter L59-86): `filepath.Walk` over `knowledge/sql-guides`, keep only `.md` matching `secRegex`, parse YAML frontmatter (`gopkg.in/yaml.v3`) for `title` / `source_idx` / `section`, read body. Note: production should reuse `indexer.go`'s CRLF normalization discipline (see Shared Patterns) and descend subdirectories (spike source_paths confirm subdirs; mirror `walkTier` indexer.go:84-107 which uses `filepath.WalkDir` + skips hidden dirs).

**Merge core** (spike main.go:155-199 — copy verbatim):
```go
func mergeByNote(chunks []Chunk) []Note {
	bySrc := map[string][]Chunk{}
	for _, c := range chunks {
		bySrc[c.SourceIdx] = append(bySrc[c.SourceIdx], c)
	}
	notes := []Note{}
	for src, parts := range bySrc {
		// Stable sort: SecOrder ascending.
		sort.Slice(parts, func(i, j int) bool { return parts[i].SecOrder < parts[j].SecOrder })
		// Merge by Section name in first-seen order.
		bySection := map[string]*MergedSection{}
		var order []string
		for _, p := range parts {
			ms, ok := bySection[p.Section]
			if !ok {
				ms = &MergedSection{Section: p.Section}
				bySection[p.Section] = ms
				order = append(order, p.Section)
			}
			if strings.TrimSpace(p.Body) != "" {
				ms.Body += p.Body + "\n"
			}
			ms.SourcePaths = append(ms.SourcePaths, p.Path)
		}
		// ... assign SectionIdx by first-seen order; sort notes by numeric source_idx
	}
}
```

**ChunkMeta contract** (spike main.go:264-284 — field names are the D-ECL-02 contract):
```go
type ChunkMeta struct {
	SourceIdx   string   `json:"source_idx"`
	Title       string   `json:"title"`
	Section     string   `json:"section"`
	SectionIdx  int      `json:"section_idx"`
	Sections    int      `json:"section_count"`
	SourcePaths []string `json:"source_paths"`
	BodyBytes   int      `json:"body_bytes"`
}
```

**Boundary verification logic to port into tests** (spike main.go:322-331): total fragment-contributions (`sum(len(s.SourcePaths))`) must equal walked chunk count (185/185); note 001 must contain 3/3 corpus `update etl_\w+` targets (regex at spike main.go:205).

---

### `internal/knowledge/ecl/extract.go` (service, LLM request-response)

**Analog:** `internal/ontology/induction/induction.go` Generate (L128-188) — the LLM call shape; `internal/llm/extract.go` — defensive parsing + response-format option.

**Narrow interface for test stubs** (induction.go:49-53 — same seam pattern for `Extractor`):
```go
// Generator is the narrow LLM surface induction needs. It is satisfied by
// Eino's model.ToolCallingChatModel, which lets tests inject a stub.
type Generator interface {
	Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error)
}
```

**LLM call + defensive parse** (induction.go:135-152 — copy the shape, substitute D-Ext-01 prompt):
```go
resp, err := m.chat.Generate(ctx, []*schema.Message{
	schema.SystemMessage("You output only valid JSON."),
	schema.UserMessage(fmt.Sprintf(generatePrompt, text)),
}, model.WithTemperature(0.0))
if err != nil {
	return Proposal{}, fmt.Errorf("induction: llm generate: %w", err)
}
if resp == nil || resp.Content == "" {
	return Proposal{}, errors.New("induction: llm returned empty response")
}
raw := extractJSON(resp.Content)
if raw == "" {
	return Proposal{}, fmt.Errorf("induction: no JSON object in llm response: %s", resp.Content)
}
var shape llmProposalShape
if err := json.Unmarshal([]byte(raw), &shape); err != nil {
	return Proposal{}, fmt.Errorf("induction: unmarshal llm response: %w (raw: %s)", err, raw)
}
```
**Production must use `llm.ExtractJSON`** (extract.go:83-120), not induction's local copy — three call sites already standardize on it (RESEARCH "Don't Hand-Roll").

**response_format injection — CRITICAL CAVEAT** (`internal/llm/extract.go:185-192`): the helper is **unexported** (`withJSONObjectResponseFormat`). Either export a wrapper from `internal/llm` or inline this exact block in the ecl package (planner decision per RESEARCH Pattern 4; prefer exporting):
```go
func withJSONObjectResponseFormat() model.Option {
	return model.WrapImplSpecificOptFn(func(o *map[string]any) {
		if *o == nil {
			*o = map[string]any{}
		}
		(*o)["response_format"] = map[string]any{"type": "json_object"}
	})
}
```
Full call posture including the option (extract.go:230-239):
```go
opts := []model.Option{
	model.WithTemperature(0.0),
	withJSONObjectResponseFormat(),
}
resp, err := chatModel.Generate(ctx, []*schema.Message{
	schema.SystemMessage(systemPrompt),
	schema.UserMessage(userPrompt),
}, opts...)
```

**Fixed prompt schema (D-Ext-01)** — mirror induction's `generatePrompt` const style (induction.go:57-67): fixed JSON object shape `{tables: [{name, kind: "update|select|insert|create"}], related_notes: [{source_idx, relation}]}`; the mirrored struct (induction.go:70-74 `llmProposalShape`) is the unmarshal target; kind enum validated by switch (induction.go:153-158).

**Regex stub (doubles as golden-set drafter, D-GT-01)** — spike main.go:205-209 regexes extended to the kind enum:
```go
var sqlUpdateRegex = regexp.MustCompile(`(?im)^\s*update\s+(etl_\w+)\b`)
var sqlStartRegex = regexp.MustCompile(`(?im)^\s*(update|insert\s+into|create\s+or\s+replace|delete\s+from|merge\s+into|select\s+.*\s+into)\b`)
```

---

### `internal/knowledge/ecl/manager.go` (service, stage orchestrator)

**Analog:** `internal/ontology/induction/induction.go` Manager lifecycle + `admin_ontology.go` conflict posture.

**Manager struct + onChange hook** (induction.go:77-96 — copy the mutex + `SetOnChange` hook shape; ECL adds a `running` CAS flag and a status struct):
```go
type Manager struct {
	mu      sync.Mutex
	ns      string
	dataDir string
	chat    Generator
	kg      *store.Store
	queue   []Proposal
	nextID  int
	// onChange (when set) fires after every queue/TBox mutation so the
	// caller can schedule a debounced KG snapshot (D-KG-01/03).
	onChange func()
}

// SetOnChange registers the post-mutation callback (idempotent; the boot
// sequence wires the debounced snapshot writer here).
func (m *Manager) SetOnChange(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}
```

**409-on-double-run**: `admin_ontology.go:205-226` shows the error→HTTP mapping shape (`errorsIsIRIConflict` → 409 at L218-220); the ECL `run` handler maps `!mgr.CASRunning()` → 409 with UI-SPEC copy 「管线正在运行中，请等待完成」. Stage sentinels `idle / merge / extract / kg-load / milvus-load / done` are a FE contract (UI-SPEC §Stage Mapping) — do not rename.

**Per-note retry ×3 → dead-letter (D-RT-01/D-DL-01)**: no direct analog; the store's snapshot debounce (store.go:199-218) shows the project's timer/error-swallow posture (`return // never silently corrupt; skip this write`). Single-note failure must not abort the batch — structure the loop per-note with error capture into `ecl_failures`, not `return err`.

---

### `internal/knowledge/ecl/abox.go` (service, KG CRUD)

**Analog:** `internal/ontology/induction/induction.go` Approve KG-write loop; `internal/ontology/store/store.go`.

**KG insert after triple derivation** (induction.go:248-253):
```go
// Insert new triples into the KG store so queries see them immediately.
for _, t := range p.ProposedTriples {
	if trip, ok := parseTripleLine(t, m.ns); ok {
		m.kg.Insert(trip)
	}
}
```

**Store surface being extended** (store.go:67-77 — `Insert` does NOT dedupe, hence note-level replace needs `DeleteWhere` first):
```go
// Insert adds a triple and updates all three indices. O(1).
// Duplicate triples are stored as-is (caller deduplicates if needed).
func (s *Store) Insert(t Triple) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := len(s.all)
	s.all = append(s.all, t)
	s.byS[t.S] = append(s.byS[t.S], idx)
	s.byP[t.P] = append(s.byP[t.P], idx)
	s.byO[t.O] = append(s.byO[t.O], idx)
}
```
`DeleteWhere` (new) must maintain all three indices (byS/byP/byO) symmetrically — mirror how Insert appends to each. Note-level replace (D-Run-01): `DeleteWhere` on subject `skf:note_{source_idx}` (+ orphaned TableSchema cleanup), then Insert fresh triples.

**Triple literal convention** (store.go:39-47): S/P are full IRIs; O is an IRI or literal rendered `"value"`. IRI local names sanitized via `emitter.sanitizeLocal` (emitter.go:326-337) for extracted table names.

**Debounce trigger after writes** (main.go:911-912 wiring; store.go:199-218 impl):
```go
kgDebounce := ontologyKG.SnapshotDebounced(kgSnapshotPath, 5*time.Second)
ontologyManager.SetOnChange(kgDebounce)
```
ECL must fire the same debounce fn after KG writes (anti-pattern: KG writes without snapshot trigger — RESEARCH).

---

### `internal/knowledge/ecl/failures.go` (model, narrow interface over SQLite)

**Analog:** `induction.go:49-53` Generator narrow-interface precedent (recommended by RESEARCH Alternatives: narrow `ecl.FailureStore` interface implemented by the sqlite store, faked in tests — matches `retriever/backend.go:23-33` `IndexerBackend` precedent).

**DDL home** (`internal/store/sqlite/schema.go:51` `schemaSQL` const + migrate() L770-911): add `ecl_failures` as a `CREATE TABLE IF NOT EXISTS` block inside `schemaSQL` (auto-applied, idempotent on every `New()`). DDL sketch in RESEARCH Code Examples (`note_id TEXT PRIMARY KEY / stage / attempt_count / last_error / status pending|dead|resolved / updated_at`). Mutable status column is sanctioned here — NOT an audit table (CONTEXT D-DL-01 note). Status-transition reads/writes use parameterized placeholders via existing store helpers (T-3-SQLI discipline).

---

### `internal/knowledge/indexer.go` (MOD — routing branch)

**Analog:** itself. Two insertion points verified:

**Per-file loop where the branch goes** (indexer.go:211-229 — the ECL check must precede the `maxDocBytes` skip, or ECL files get silently starved):
```go
for _, path := range matches {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("knowledge.Index: read %s: %w", path, err)
	}

	body, fm := splitFrontmatter(raw)
	if len(body) == 0 {
		continue
	}
	if len(body) > maxDocBytes {   // <-- ECL branch goes BEFORE this guard
		...
		continue
	}
```
Branch shape (RESEARCH Pattern 1): `if eclEnabled && ecl.IsSecFragment(filepath.Base(path), fm) { continue }` — `IsSecFragment` = `__sec(\d{2})_(.+)\.md$` on base name AND non-empty `section` key in frontmatter (verified: non-sec `030_承保清单.md` has `title`+`source_idx` but no `section`).

**Purge seam where marker-scoped purge goes** (indexer.go:443-451):
```go
func seedDocs(ctx context.Context, mc client.Client, cfg milvus.Config, docs []retriever.Document) error {
	if indexSeedHook != nil {
		return indexSeedHook(ctx, mc, cfg, docs)
	}
	if err := milvus.DeleteTierRows(ctx, mc, cfg, seedTiers(docs)); err != nil {
		return fmt.Errorf("purge stale tier rows: %w", err)
	}
	return milvus.SeedDocuments(ctx, mc, cfg, docs)
}
```
With ECL on: swap `DeleteTierRows` for `milvus.DeleteByExpr(... section_count == 0 ...)` (legacy rows only). With ECL off: plain tier purge (legacy contract, also wipes stale ECL rows on rollback).

**Doc-build pattern to mirror for legacy zero-values (D-Milvus-02)** (indexer.go:256-264): `retriever.Document{ID, Tier, Content, Source, Embedding, EffectiveFrom, EffectiveTo}` — new fields default to zero values for legacy docs (`source_idx=""`, `section_idx=0`, `section_count=0`).

**Frontmatter CRLF handling** (indexer.go:494-516 `splitFrontmatter`): does `bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))` — ecl frontmatter parse must keep this discipline (Pitfall 7).

---

### `internal/retriever/milvus/seed.go` (MOD) + `delete_expr.go` (NEW)

**Analog:** itself.

**Schema extension point** (seed.go:70-80 — add 4 fields after `effective_to`, following the same builder chain):
```go
schema := entity.NewSchema().WithName(cfg.CollectionName).WithAutoID(false).
	WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(false)).
	WithField(entity.NewField().WithName("tier").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(DefaultTierMaxLen))).
	WithField(entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(DefaultContentMaxLen))).
	WithField(entity.NewField().WithName("source").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(DefaultSourceMaxLen))).
	WithField(entity.NewField().WithName("embedding").WithDataType(entity.FieldTypeFloatVector).WithDim(int64(cfg.EmbedDim))).
	WithField(entity.NewField().WithName("effective_from").WithDataType(entity.FieldTypeInt64)).
	WithField(entity.NewField().WithName("effective_to").WithDataType(entity.FieldTypeInt64))
```
Widths derive from single-home constants (zero-hardcode standard, config.go:43-62) — add `DefaultSourceIdxMaxLen` / `DefaultSectionMaxLen` consts in config.go. **VarChar columns use `entity.NewColumnVarChar`, never `NewColumnString`** (spike-findings skill: DataType String=20 vs FieldTypeVarChar=21 mismatch).

**Column-slice + Insert pattern to extend with 4 more columns** (seed.go:139-167):
```go
ids := make([]int64, len(docs))
tiers := make([]string, len(docs))
// ... one slice per column, filled in the docs loop ...
idCol := entity.NewColumnInt64("id", ids)
tierCol := entity.NewColumnVarChar("tier", tiers)
// ...
if _, err := mc.Insert(ctx, cfg.CollectionName, "", idCol, tierCol, contentCol, sourceCol, vecCol, effectiveFromCol, effectiveToCol); err != nil {
	return fmt.Errorf("Insert: %w", err)
}
```

**DeleteByExpr mechanics — copy DeleteTierRows exactly** (seed.go:216-252): HasCollection → no-op on fresh deploy; `LoadCollection` (DML requires loaded); `mc.Delete(ctx, cfg.CollectionName, "", expr)`; `Flush` (durable tombstones). Interpolate values with `strconv.Quote` / `%q` (L240-244 precedent; also filter.go:55-59).
```go
quoted := make([]string, len(tiers))
for i, t := range tiers {
	quoted[i] = strconv.Quote(t)
}
expr := "tier in [" + strings.Join(quoted, ",") + "]"
if err := mc.Delete(ctx, cfg.CollectionName, "", expr); err != nil { ... }
```

**RECREATE one-shot door already exists** (main.go:1120-1148): `milvus.ShouldRecreate(os.Getenv(milvus.RecreateEnvVar))` → `mc.RecreateCollection` + WARN receipt; boot Index at L1144 re-embeds. No new migration code needed — document the deploy sequence (RECREATE=1 boot → Run ECL → unset).

---

### `internal/ontology/emitter/emitter.go` (MOD — TBox 6→8)

**Analog:** itself. Two hardcoded lists are the single schema source (D-TBox2-01):

**Classes list** (emitter.go:202-209 — append 2 entries):
```go
classes := []struct{ Name, Label, Comment string }{
	{"Tier", "Tier", "Trust tier for KB content (权威/历史/个人/sql-guides/draft)."},
	// ... 4 more ...
	{"RefusalPhrase", "Refusal Phrase", "A refusal phrase a Skill is required to emit under specified conditions."},
	// Phase 39 (D-TBox2-01): + NoteChunk, TableSchema
}
```

**ObjectProperty list** (emitter.go:215-222 — append 2 entries with domain/range):
```go
objProps := []struct{ Name, Label, Domain, Range string }{
	{"belongsToTier", "category belongs to tier", "KBCategory", "Tier"},
	// ... 4 more ...
	// Phase 39: + belongsToNote (NoteChunk→Note-domain per planner), + referencesTable (NoteChunk→TableSchema)
}
```
Render loop emits `type + domain + range + label` per property (L223-225). ABox individual naming precedent: `skf:note_{source_idx}` / `skf:tbl_{name}` mirrors `skf:cat_%s` / `skf:sub_%s_%s` patterns (L249-252). Local-name sanitization: `sanitizeLocal` (L326-337).

**Boot pointer short-circuit is the extension blocker** (main.go:876-893):
```go
ontologyPointer := filepath.Join(ontologyDataDir, "skillforge_tbox.ttl")
if _, err := os.Stat(ontologyPointer); os.IsNotExist(err) {
	// emit v1 + pointer + tbox_version.txt
}
```
Pitfall 4 fix: add a one-time upgrade trigger — pointer exists but lacks `skf:NoteChunk a owl:Class` → emit v{N+1} + bump `tbox_version.txt` + swap pointer (never re-emit unconditionally).

**Boot KG-restore fix (Pitfall 3)** — current code (main.go:894-906) builds KG from pointer only and overwrites the snapshot; Phase 39 must implement D-KG-02: if snapshot exists → `store.LoadSnapshot` (store.go:181-191) → merge TBox triples WITH dedupe (`Insert` does not dedupe) → snapshot. `LoadSnapshot` returns a fresh `*Store` built via `Insert` per triple — the merge path needs the dedupe guard added this phase.

---

### `internal/server/admin_pipeline.go` (NEW controller) + `server.go` (MOD)

**Analog:** `internal/server/admin_ontology.go` — copy the whole file's shape.

**Route godoc header convention** (admin_ontology.go:17-37): every route + status code documented in the package comment before `package server`.

**Deps field** (server.go:461-467):
```go
// --- Phase 38: Ontology Layer (v1.4 / ONT-01..09) ---
// Ontology bundles the TBox emitter output (already persisted by boot),
// the memory-map KG store, and the schema-induction manager. nil = the
// server started without the ontology layer: every /api/admin/ontology/*
// route answers 503 (same nil-posture as Knowledge / Wizard). cmd/server
// wires it at boot: emit TBox → load KG → snapshot → debounce (5s).
Ontology *OntologyDeps
```
Add `ECL *ecl.Manager` (or `*ECLDeps`) beside it with the same nil-posture godoc.

**The gate — 403 before 503 (locked D-Route-01)** (admin_ontology.go:67-77):
```go
// requireOntology gates a handler on the ontology layer being wired.
func (s *Deps) requireOntology(c *app.RequestContext) (*OntologyDeps, bool) {
	if !requireAdmin(c) {
		return nil, false
	}
	if s.Ontology == nil || s.Ontology.Induction == nil || s.Ontology.KG == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "ontology layer not wired"})
		return nil, false
	}
	return s.Ontology, true
}
```
`requireAdmin` (audit.go:142-148) answers 403 `graphResponseError{Error: "admin token required"}`. ECL gate adds the 409 double-run check on `run` only.

**Error envelope** (knowledge.go:66-68):
```go
type graphResponseError struct {
	Error string `json:"error"`
}
```

**Handler shape: gate → bind → act → envelope** (admin_ontology.go:152-168):
```go
func (s *Deps) adminOntologyProposalsGenerateHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	var req generateRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid body: " + err.Error()})
		return
	}
	p, err := dep.Induction.Generate(ctx, req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "generate: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, proposalGenerateResponse{Proposal: p})
}
```
Path params via `c.Param("id")` (L215) → `c.Param("note_id")` for retry; validate `:note_id` against the known note/failure set before acting (V5 input validation, RESEARCH Security).

**Route registration** (server.go:655-662 — register the 3 pipeline routes in the same block):
```go
h.GET("/api/admin/ontology/tbox", deps.adminOntologyTBoxHandler)
// ... 7 more ontology routes ...
h.POST("/api/admin/ontology/snapshot", deps.adminOntologySnapshotHandler)
```

---

### `cmd/server/main.go` (MOD — boot wiring)

**Analog:** itself, ontology boot block L864-914 (order: mkdir dataDir → pointer check/emit → `onstore.New()` → `LoadFromTurtle` → snapshot → `induction.NewManager(..., cm, kg)` → debounce wire). ECL manager construction goes beside it: `ecl.NewManager(..., cm, kg, failureStore, mc/milvus deps...)` + `SetOnChange`-style debounce reuse + `deps.ECL = mgr`. Env gate `SKILLFORGE_ECL_ENABLED` parse: disabled only on explicit `off`/`false` (A3) — follow the `envOr` helper style. Boot does NOT auto-run ECL (D-Run-01). ecl_failures migration fires automatically via `sqlite.New` → `migrate()` (no boot code needed).

---

### `web/src/components/admin/pipeline/` (NEW FE component + api + types)

**Analogs:** `CapacityMonitor.tsx` (fetch + token), `useEvalReports.ts` (poll loop), `AdminDashboard.tsx` (tab registration), `ontology/OntologyTab.tsx` (subfolder tab precedent).

**Fetch + admin token** (CapacityMonitor.tsx:76-96):
```tsx
useEffect(() => {
  const token = localStorage.getItem('sf_admin_token') ?? '';
  fetch('/api/admin/table-counts', {
    headers: {
      'X-Admin-Token': token,
    },
  })
    .then((resp) => {
      if (!resp.ok) { setError(true); return null; }
      return resp.json() as Promise<TableCounts>;
    })
    .then((data) => { if (data) setCounts(data); })
    .catch(() => setError(true));
}, []);
```

**Polling loop — UI-SPEC mandates the injected-interval hook** (useEvalReports.ts:67-79; UI-SPEC §4 requires `vi.useFakeTimers` testability):
```tsx
useEffect(() => {
  let cancelled = false;
  void fetchOnce();
  const id = window.setInterval(() => {
    if (cancelled) return;
    void fetchOnce();
  }, pollIntervalMs);
  return () => {
    cancelled = true;
    window.clearInterval(id);
  };
}, [pollIntervalMs, reloadKey.current, fetchOnce]);
```
Cadence per D-Prog-01: 2s while `isRunning` (`stage !== 'idle' && stage !== 'done'`), 10s idle — implement as injected `pollIntervalMs` that re-arms on stage change (same effect-re-run mechanism as `reloadKey`).

**Tab registration — 9th tab** (AdminDashboard.tsx:185-195 SubrouteTab block + L218 `{adminSubroute === 'ontology' && <OntologyTab />}`):
```tsx
{/* Plan 38-05 — 本体 (the 8th tab): TBox browser + KG query + ... */}
<SubrouteTab
  ...
  testId="admin-tab-ontology"
  ...
/>
```
Add `admin-tab-pipeline` SubrouteTab + conditional render line beside the ontology ones; put PipelineTab in a `pipeline/` subfolder mirroring `ontology/OntologyTab.tsx`. Error/503 rendering follows CapacityMonitor's `text-danger` block (L98-104) with the copy contract from 39-UI-SPEC.

---

### `scripts/verify-phase-39.sh` (NEW verify script)

**Analog:** `scripts/verify-phase-38.sh` — copy the skeleton verbatim and swap stages. Key structural elements (verify-phase-38.sh:35-50, 200-213):
```bash
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT="$(pwd)"
WORK="$(mktemp -d /tmp/verify38-XXXXXX)"
trap 'rm -rf "$WORK"' EXIT
pass_count=0
fail_count=0
ok() { echo "[SC-PASS] $1"; pass_count=$((pass_count + 1)); }
fail() { echo "[SC-FAIL] $1"; fail_count=$((fail_count + 1)); }
# ... stages: /usr/local/go/bin/go test <pkg> -run '<Test>' -count=1 -v >"$WORK/stageN.log" 2>&1 && grep -q -- "--- PASS: ..." ...
echo "=== PHASE 38 HERMETIC GATE: $pass_count passed, $fail_count failed ==="
```
Conventions to keep: WSL toolchain path `/usr/local/go/bin/go`; hermetic (no GLM/MILVUS env required — stub-driven); per-SC stage naming `[SC-nn-...]`; WSL header comment. Phase 39 stages: (a) merge boundary 185/185 + note 001 3/3; (b) F1 harness (regex stub vs golden ≈1.0); (c) ABox dual-write + dead-letter flow (stub).

---

### `eval/ecl-golden/sql-guides.jsonl` (NEW data)

**Analog:** `eval/golden/*.jsonl` convention (6 existing files: compliance.jsonl, hitl.jsonl, kpi.jsonl, ...). Row shape per CONTEXT specifics:
```json
{"source_idx": "001", "tables": [{"name": "etl_payment_due_t", "kind": "update"}], "related": ["006"]}
```
26 rows. NOTE (A5): D-GT-01 locks the path `eval/ecl-golden/sql-guides.jsonl` even though `eval/golden/` is the established convention — keep the CONTEXT path, flag in plan.

## Shared Patterns

### Admin gate: 403 → 503 → (409 on run)
**Source:** `internal/server/admin_ontology.go:67-77` + `internal/server/audit.go:142-148`
**Apply to:** `admin_pipeline.go` all 3 handlers (run additionally checks `CASRunning()` → 409)
```go
if !requireAdmin(c) { return nil, false }                       // 403 "admin token required"
if s.ECL == nil /* or not enabled */ {
	c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "ECL pipeline not wired"})
	return nil, false                                           // 503 after 403 — locked order
}
```

### LLM call posture: temp-0 + response_format + ExtractJSON + strict unmarshal
**Source:** `internal/ontology/induction/induction.go:135-158` + `internal/llm/extract.go:83-120, 185-192`
**Apply to:** `ecl/extract.go` (LLM impl); stub impl shares the `Extractor` interface (induction.go:51-53 seam). CAVEAT: `withJSONObjectResponseFormat` is unexported — export from `internal/llm` or copy the `WrapImplSpecificOptFn` block.

### Store mutation → debounced snapshot hook
**Source:** `internal/ontology/induction/induction.go:92-124` (SetOnChange/fireChange) + `cmd/server/main.go:911-912` (wiring) + `internal/ontology/store/store.go:199-218` (SnapshotDebounced)
**Apply to:** `ecl/manager.go` + `ecl/abox.go` — every KG write fires the shared debounce fn; ABox is not durable without it.

### Milvus DML mechanics: HasCollection → Load → DML → Flush, %q-escape values
**Source:** `internal/retriever/milvus/seed.go:216-252` (DeleteTierRows), `:129-180` (SeedDocuments insert+flush+load)
**Apply to:** `milvus/delete_expr.go` (new helper), `seed.go` (+4 columns), `ecl/chunk.go` (column discipline: `NewColumnVarChar` for strings, never `NewColumnString`).

### SQLite migration: additive CREATE TABLE IF NOT EXISTS in schemaSQL + migrate()-tail indexes
**Source:** `internal/store/sqlite/schema.go:51` (schemaSQL), `:770-911` (migrate), newest-table precedent `knowledge_bases` L337-373 / `feedback` L375-406
**Apply to:** `ecl_failures` DDL. Mutable `status` column sanctioned (not an audit table). If an index is needed, add it as a migrate()-tail const (L650-694 convention), never inline in schemaSQL.

### SPDX header + route-godoc documentation discipline
**Source:** every in-repo file (e.g. `admin_ontology.go:1-15` full Apache-2.0 block; `admin_ontology.go:17-43` wire-contract godoc)
**Apply to:** ALL new files (`ecl/*.go`, `admin_pipeline.go`, `delete_expr.go`, FE files use the 2-line variant `// Copyright 2026 SkillForge Authors` + `// SPDX-License-Identifier: Apache-2.0` per CapacityMonitor.tsx:1-2).

### Narrow interface + injected seam for hermetic tests
**Source:** `induction.go:49-53` (Generator), `indexer.go:421-441` (indexSeedHook/indexAuditHook callback seams)
**Apply to:** `Extractor` (LLM vs regex stub) and `FailureStore` (sqlite vs test fake); never add mock branches inside production paths.

### FE polling hook with injected interval + cancellation
**Source:** `web/src/hooks/useEvalReports.ts:33-89`
**Apply to:** `pipeline/` polling (2s running / 10s idle per D-Prog-01); token fetch per CapacityMonitor.tsx:79-84; tab registration per AdminDashboard.tsx:185-195 + 218.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/knowledge/ecl/chunk.go` (sub-split call) | service | transform | `eino-ext splitter/recursive` has NO in-repo Go usage (grep: only go.mod/go.sum + planning docs). Use eino-ext docs for the splitter API; column mapping analog is seed.go:139-167 |
| `internal/knowledge/ecl/f1.go` | utility | transform (pure) | No set-similarity helper exists; RESEARCH Pattern 5 pins the exact math (per-note table-name sets, macro-averaged) |
| `web/src/components/admin/pipeline/types.ts` | model (FE) | — | Trivial DTO mirror of the status endpoint shape `{stage, current_file, done, total, queue_len, failures[]}` |

## Metadata

**Analog search scope:** `internal/knowledge/`, `internal/retriever/milvus/`, `internal/ontology/{store,induction,emitter}/`, `internal/llm/`, `internal/server/`, `internal/store/sqlite/`, `cmd/server/`, `web/src/components/admin/`, `web/src/hooks/`, `scripts/`, `.planning/spikes/005c-ontology-aware-chunker/`
**Files scanned:** 16 analog files read in full + 4 targeted reads (main.go boot/RECREATE, server.go routes) + 6 greps (tab registration, envelope type, splitter usage)
**Pattern extraction date:** 2026-09-21
**Drift check vs CONTEXT canonical refs:** all 6 listed analogs verified present and current; RESEARCH line anchors spot-checked and accurate (admin_ontology.go:68-77, seed.go:70-88, main.go ontology block, emitter lists, store.go Insert, llm/extract.go).
