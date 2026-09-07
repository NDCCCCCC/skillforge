# Phase 27 PATTERNS

**Mapped:** 2026-09-07
**Files analyzed:** 10
**Analogs found:** 10 / 10

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/server/doc_upload.go` | controller | request-response / streaming | `internal/server/import_api.go` | exact |
| (admin-token gate) | middleware | request-response | `internal/server/kb_admin.go` | exact |
| (registry walker) | service | file-I/O | `internal/knowledge/walk.go` | exact |
| (chunk pipeline) | service | file-I/O / CRUD | `internal/knowledge/indexer.go` | exact |
| (audit hook) | hook | event-driven | `internal/knowledge/indexer.go` | exact |
| `web/src/components/admin/DocUpload.tsx` | component | request-response | `web/src/components/admin/KnowledgeBaseList.tsx` | exact |
| (validation) | utility | transform | `internal/skill/loader.go` | role-match |
| (Document shape) | model | CRUD | `internal/retriever/retriever.go` | exact |
| (schema) | config | batch | `internal/store/sqlite/schema.go` | role-match |
| (kb.Citable path) | service | event-driven | `internal/agent/trust_floor.go` | exact |

---

## Pattern 1: Streaming Multipart Admin Surface

**Closest analog:** `internal/server/import_api.go`

**Handler entry point** (`import_api.go:115`):
```go
func (s *Deps) importPreviewHandler(ctx context.Context, c *app.RequestContext) {
    start := time.Now()
    if !isAdminRequest(c) {
        c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
        return
    }
    if s.Importer == nil {
        c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: importDisabledError})
        return
    }
    if err := parseImportForm(c, func(form *importForm, file io.Reader) error {
        // ...
    }); err != nil {
        c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid form: " + err.Error()})
    }
}
```

**parseImportForm streaming pattern** (`import_api.go:325-398`):
```go
func parseImportForm(c *app.RequestContext, onFile func(form *importForm, file io.Reader) error) error {
    boundary := string(c.Request.Header.MultipartFormBoundary())
    if boundary == "" {
        return fmt.Errorf("missing multipart boundary")
    }
    mr := multipart.NewReader(c.Request.BodyStream(), boundary)
    for {
        part, perr := mr.NextPart()
        if perr == io.EOF { break }
        name := part.FormName()
        if name == "file" {
            return onFile(form, part)  // stream file directly, never touch disk
        }
        // switch on field names...
    }
    return fmt.Errorf("missing file part")
}
```

**Reuse vs adapt for Phase 27:**
- Mirror verbatim: `isAdminRequest` gate, nil-check 503, multipart streaming via `BodyStream()`, callback pattern
- Change: no DuckDB engine, no preview/commit/push state machine -- just one-shot parse-embed-insert; file is a `.md` or PDF passed directly to `eino-ext` parsers (not a CSV with a staging path)
- Pitfalls to avoid: the GB18030 UTF-8 guard from T-15-12 is import-specific (CSV encoding); doc upload deals with UTF-8 source files only; the per-route body-size split is server-wide (server.go `WithStreamBody` config); the no-raw-on-disk invariant is the same -- parse in memory via the part reader

**Phase 27 reuse:** adapted verbatim

---

## Pattern 2: Admin Token Gate with Check-Order

**Closest analog:** `internal/server/kb_admin.go` + `internal/server/audit.go`

**isAdminRequest** (`audit.go:114-129`):
```go
func isAdminRequest(c *app.RequestContext) bool {
    expected := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
    if expected == "" {
        return false
    }
    var got string
    c.Request.Header.VisitAll(func(k, v []byte) {
        if string(k) == "X-Admin-Token" { got = string(v) }
    })
    return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
```

**Check-order pattern in kb_admin.go** (`kb_admin.go:136-146`, `kb_admin.go:167-177`):
```go
// CHECK ORDER: admin check FIRST — a 403 must not leak whether the
// KB registry is wired.
if !isAdminRequest(c) {
    c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
    return
}
if s.KBRegistry == nil {
    c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kbRegistryNotWiredError})
    return
}
```

**Phase 27 reuse:** verbatim -- the doc upload endpoint follows the identical 403-then-503 check order, using the same `isAdminRequest` and `graphResponseError` envelope.

---

## Pattern 3: Registry-Driven Walker

**Closest analog:** `internal/knowledge/walk.go`

**WalkRegisteredDirs** (`walk.go:85-142`):
```go
func WalkRegisteredDirs(knowledgeDir string, reg kb.Registry, audit DirAuditFn) ([]string, error) {
    // Fail-closed on nil registry
    if reg == nil {
        return nil, errors.New("knowledge.WalkRegisteredDirs: registry is required (fail-closed)")
    }
    entries, err := os.ReadDir(knowledgeDir)
    // ...
    deactivated := make(map[string]bool)
    for _, e := range reg.List() { deactivated[e.Slug] = true }
    for _, entry := range entries {
        name := entry.Name()
        if !entry.IsDir() { continue }
        if _, ok := reg.Lookup(name); ok { out = append(out, name); continue }
        if deactivated[name] { log.Printf("[WARN] ...deactivated... skipping"); continue }
        // unregistered: WARN + audit
        if audit != nil { audit(actionKBDirUnregistered, ...) }
    }
    sort.Strings(out)
    return out, nil
}
```

**Reuse:** The upload endpoint's `kb_slug` form field must pass the same `kb.Registry.Lookup` gate before any indexing work begins. The three-state logic (active/deactivated/unregistered) is handled by the walker; the upload handler just calls `reg.Lookup(slug)` to confirm the target base exists and is active.

**Phase 27 reuse:** verbatim -- the upload handler validates `kb_slug` via `KBRegistry.Lookup` before invoking the indexer.

---

## Pattern 4: Chunk Emission Pipeline

**Closest analog:** `internal/knowledge/indexer.go`

**seedDocs call chain** (`indexer.go:116-256`):
```go
// Registry-driven walk
dirs, err := WalkRegisteredDirs(knowledgeDir, reg, indexAuditHook)

// Per-file loop
for _, path := range matches {
    raw, err := os.ReadFile(path)
    body, fm := splitFrontmatter(raw)
    // maxDocBytes guard
    if len(body) > maxDocBytes { fmt.Fprintf(os.Stderr, "[WARN] ...skipping\n"); continue }
    vec, err := emb.Embed(ctx, string(body))
    rel, _ := filepath.Rel(knowledgeDir, path)
    ef, et := parseEffectiveWindow(fm)
    docs = append(docs, retriever.Document{
        ID:            nextID,
        Tier:          slug,
        Content:       string(body),
        Source:        relSlash,
        Embedding:     vec,
        EffectiveFrom: ef,
        EffectiveTo:   et,
    })
    nextID++
}

// Tier + effective window validation BEFORE backend write
if err := validateTiers(docs, reg); err != nil { return err }
if err := validateEffective(docs); err != nil { return err }

// Seed in one batch
if len(docs) == 0 { return nil }
if err := seedDocs(ctx, mc, cfg, docs); err != nil { return err }
```

**What changes for Phase 27:**
- Source = single uploaded file (not all `knowledge/<slug>/*.md` from a walk)
- Target = append into existing client (not re-walk from disk); no full re-seed
- `splitFrontmatter`, `validateTiers`, `nextID` assignment pattern stays identical
- `validateEffective` still runs on the single doc's frontmatter
- `indexAuditHook` still fires for the upload action

**Phase 27 reuse:** adapted verbatim -- same pipeline functions; different source (single file upload vs. directory walk) and target (append vs. full re-seed).

---

## Pattern 5: Audit Hook

**Closest analog:** `internal/knowledge/indexer.go`

**indexAuditHook package var** (`indexer.go:356-374`):
```go
var indexAuditHook func(action, detail string)

func SetIndexAuditHook(fn func(action, detail string)) { indexAuditHook = fn }
```

**Walker fires it** (`walk.go:134-138`):
```go
if audit != nil {
    audit(actionKBDirUnregistered, fmt.Sprintf(`{"dir":%q,"knowledge_dir":%q}`, name, knowledgeDir))
}
```

**KB admin audit row** (`kb_admin.go:330-348`):
```go
func (s *Deps) putKBAdminAuditRow(ctx context.Context, action string, entry kb.Entry) {
    if s.Store == nil { return }
    payload, err := json.Marshal(entry)
    if err != nil { log.Printf("[WARN] ...row skipped"); return }
    if _, err := s.Store.PutAudit(ctx, store.AuditRow{
        Timestamp:  time.Now().UTC(),
        UserID:     "admin",
        Action:     action,
        ActorType:  "admin",
        FrozenArgs: payload,
    }); err != nil {
        log.Printf("[WARN] ...best-effort, mutation not rolled back")
    }
}
```

**Phase 27 reuse:** verbatim -- bind `SetIndexAuditHook` at boot with the same `store.PutAudit` wrapper; fire `action_doc_uploaded` with `FrozenArgs` carrying the upload metadata `{slug, filename, size_bytes, chunk_count}`.

---

## Pattern 6: KB Admin FE Form

**Closest analog:** `web/src/components/admin/KnowledgeBaseList.tsx`

**X-Admin-Token fetch pattern** (`KnowledgeBaseList.tsx:87-113`):
```tsx
const loadBases = useCallback(() => {
    const token = localStorage.getItem('sf_admin_token') ?? '';
    return fetch('/api/admin/kb', {
        headers: { 'X-Admin-Token': token },
    })
    .then((resp) => {
        if (!resp.ok) { setError(true); setLoading(false); return null; }
        return resp.json() as Promise<KBAdminResponse>;
    })
    .then((d) => { if (d) { setData(d); setError(false); } setLoading(false); })
    .catch(() => { setError(true); setLoading(false); });
}, []);
```

**Form submission with error rendering** (`KnowledgeBaseList.tsx:119-168`):
```tsx
function handleCreate() {
    setSlugError('');
    setActionError('');
    // UX pre-flight
    if (!KB_SLUG_PATTERN.test(slug)) { setSlugError(KB_SLUG_RULE); return; }
    // ...
    fetch('/api/admin/kb', {
        method: 'POST',
        headers: { 'X-Admin-Token': token, 'Content-Type': 'application/json' },
        body: JSON.stringify({ slug, display_name: displayName, role, description }),
    })
    .then(async (resp) => {
        if (resp.ok) { /* reset form, reload */ return; }
        const body = (await resp.json().catch(() => ({}))) as { error?: string };
        if (resp.status === 409) { setActionError('slug 已存在'); }
        else if (resp.status === 403) { setActionError('创建失败：缺少或错误的 X-Admin-Token'); }
        else if (resp.status === 503) { setActionError('创建失败：服务端知识库注册表未启用'); }
        else { setActionError(body.error ?? `创建失败（HTTP ${resp.status}）`); }
    })
    // ...
}
```

**Field error rendering** (`KnowledgeBaseList.tsx:355-359`):
```tsx
{slugError && (
    <div data-testid="kb-slug-error" role="alert" className="text-xs text-danger">
        {slugError}
    </div>
)}
```

**Phase 27 reuse:** adapted verbatim -- same `localStorage.getItem('sf_admin_token')` read, same `headers: { 'X-Admin-Token': token }` pattern, same 403/409/503 dispatch, same `actionError` + `role="alert"` rendering. Extend with an upload panel inside the same admin route group; multipart/form-data fetch instead of JSON body.

---

## Pattern 7: Chunk Document Shape

**Closest analog:** `internal/retriever/retriever.go`

**retriever.Document struct** (`retriever/retriever.go:70-78`):
```go
type Document struct {
    ID            int64
    Tier          string       // slug, e.g. "authority"
    Content       string       // body text (markdown or plain)
    Source        string       // slash-normalized path, e.g. "travel_expense.md"
    Embedding     []float32    // BGE-M3 vector
    EffectiveFrom int64        // epoch seconds; 0 = perpetual
    EffectiveTo   int64        // epoch seconds; 0 = open-ended/perpetual
}
```

**Phase 27 reuse:** verbatim -- uploaded documents produce `retriever.Document` rows with the same seven fields. `Tier` = the `kb_slug` from the upload form. `Source` = the uploaded filename. `ID` = nextID assigned from theMilvus client's current max + 1. `EffectiveFrom/EffectiveTo` = parsed from optional YAML frontmatter (0,0 if absent).

---

## Pattern 8: Validation Contract

**Closest analog:** `internal/skill/validate.go`

**Validate returns structured errors** (`validate.go:58-79`):
```go
func Validate(m *Manifest, reg kb.Registry) error {
    if m == nil {
        return fmt.Errorf("manifest is nil (SYSK-02: 7-field SKILL.md required)")
    }
    if strings.TrimSpace(m.Description) == "" {
        return fmt.Errorf("description is required and must be non-empty (SKILL_SCHEMA field 1)")
    }
    if !validAgentTypes[m.Agent] {
        return fmt.Errorf(
            "agent type %q is not supported (SKILL_SCHEMA field 2; valid: chat, deep, ...)",
            m.Agent,
        )
    }
    // ... more checks, each returns a wrapped fmt.Errorf
    return nil
}
```

**parseAndValidate callers handle errors** (`loader.go:141-145`):
```go
m, tiers, err := parseAndValidate(path, l.kbReg)
if err != nil {
    return err  // propagates with file path + rule text
}
```

**Phase 27 reuse:** adapted verbatim -- parser errors (file too large, malformed frontmatter, invalid tier) should return `fmt.Errorf` messages that carry the rule-keyword text (e.g., `"document exceeds maxDocBytes (4000 bytes)"`). The FE renders `err.Error()` directly as the `actionError` text, same as `KnowledgeBaseList.tsx` does. Keep the `{Code, Message, Path}` shape only if Phase 27 introduces a structured error type; plain wrapped `fmt.Errorf` is the existing convention and is sufficient.

---

## Pattern 9: nextID Coordination

**Closest analog:** `internal/knowledge/indexer.go` + `internal/retriever/milvus/client.go`

**Indexer assigns IDs** (`indexer.go:138,217`):
```go
nextID := int64(1)
// ... inside file loop
docs = append(docs, retriever.Document{ ID: nextID, ... })
nextID++
```

**seedDocs calls Milvus** (`seed.go:119`):
```go
func SeedDocuments(ctx context.Context, mc client.Client, cfg Config, docs []retriever.Document) error {
    // Uses docs[].ID directly as Milvus INT64 PK
```

**Decision surface:** There is no `GetMaxID` helper in `internal/retriever/milvus/client.go`. The upload pipeline must decide where the upload-only `nextID` comes from:

- Option A: Query `GetCollectionStatistics` or `Query` to find the current max ID from Milvus, then `max+1` as the starting nextID.
- Option B: The uploader always appends at the Milvus layer with auto-ID disabled (caller-assigned PK, the same convention as `seedDocs`).
- The safe approach is to query the existing max ID before inserting, using Milvus's `Query` with `output_fields=["id"]` and `limit=1` sorted by `id DESC`, then add 1.

**Phase 27 reuse:** new code for the max-ID discovery step; reuse the `ID: nextID` assignment pattern from `indexer.go` verbatim once `nextID` is established.

---

## Pattern 10: kb.Citable Path (Trust Invariant)

**Closest analog:** `internal/agent/trust_floor.go:119` + `internal/server/pipeline.go:1051` + `internal/runtime/trust_stage.go:251` + `internal/runtime/fanout_runtime.go:250`

**trust_floor.go citable gate** (`trust_floor.go:113-120`):
```go
func (a *ChunkAccumulator) citable(h retriever.Hit) bool {
    return kb.Citable(a.kb, h.Tier)
}
```

**pipeline.go citableChunksByID** (`pipeline.go:1048-1057`):
```go
func citableChunksByID(reg kb.Registry, chunks []retriever.Hit) map[string]string {
    byID := make(map[string]string, len(chunks))
    for _, ch := range chunks {
        if !kb.Citable(reg, ch.Tier) { continue }
        byID[strconv.FormatInt(ch.ID, 10)] = ch.Content
    }
    return byID
}
```

**trust_stage.go citableChunksByID** (`trust_stage.go:248-257`):
```go
func citableChunksByID(reg kb.Registry, chunks []retriever.Hit) map[string]string {
    byID := make(map[string]string, len(chunks))
    for _, ch := range chunks {
        if !kb.Citable(reg, ch.Tier) { continue }
        byID[chunkKey(ch.ID)] = ch.Content
    }
    return byID
}
```

**fanout_runtime.go Add** (`fanout_runtime.go:250-252`):
```go
if !kb.Citable(s.kb, hit.Tier) {
    return false // non-authority material never enters the grounding union
}
```

**Reuse:** Upload produces chunks of the exact same `retriever.Document` shape. The existing four insertion points (`ChunkAccumulator.citable`, `citableChunksByID` in pipeline/trust_stage, `SharedChunkAccumulator.Add` in fanout) all gate citability using `kb.Citable(reg, tier)`. No new gate is needed -- uploaded chunks automatically participate in the same citable/non-citable split the existing trust floor enforces. The upload pipeline does NOT need to check `kb.Citable` at insert time; that check lives at retrieval-time grounding, not at index-time.

**Phase 27 reuse:** verbatim -- no changes to the citability machinery. Uploaded documents enter Milvus as `retriever.Document` rows; the existing trust floor gates them at query time.

---

## Shared Patterns

### Authentication
**Source:** `internal/server/audit.go:114-129` (`isAdminRequest`)
**Apply to:** All admin controller files (doc_upload.go, import_api.go, kb_admin.go, etc.)
```go
func isAdminRequest(c *app.RequestContext) bool {
    expected := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
    if expected == "" { return false }
    var got string
    c.Request.Header.VisitAll(func(k, v []byte) {
        if string(k) == "X-Admin-Token" { got = string(v) }
    })
    return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
```

### Error Envelope
**Source:** `internal/server/kb_admin.go` (used in all admin handlers)
**Apply to:** All controller JSON error responses
```go
type graphResponseError struct { Error string `json:"error"` }
// 403: c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
// 503: c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kbRegistryNotWiredError})
// 400: c.JSON(http.StatusBadRequest, graphResponseError{Error: err.Error()})
```

### Audit Row Contract
**Source:** `internal/server/kb_admin.go:330-348` + `store.AuditRow` struct
**Apply to:** All INSERT-only audit rows
```go
store.AuditRow{
    Timestamp:  time.Now().UTC(),
    UserID:     "admin",
    Action:     "action_doc_uploaded",   // Phase 27 specific
    ActorType:  "admin",
    FrozenArgs: payload,                  // JSON-serialized upload metadata
}
```

### SQL Parameterization
**Source:** `internal/store/sqlite/schema.go` (all SQL in store layer uses `?` placeholders)
**Apply to:** Any new store layer additions for `doc_uploads` table

---

## No Exact Analog Found

All 10 patterns have existing analogs. The closest-match summary:

| Pattern | Best analog file | Key difference |
|---|---|---|
| Pattern 9 (nextID discovery) | `internal/knowledge/indexer.go` | No `GetMaxID` exists -- new Milvus query logic needed |
| Pattern 8 (validation shape) | `internal/skill/validate.go` | Plain `fmt.Errorf` vs. structured `{Code,Message,Path}` -- recommend staying with `fmt.Errorf` for consistency |

---

## Metadata

**Analog search scope:** `internal/server/`, `internal/knowledge/`, `internal/retriever/`, `internal/store/sqlite/`, `internal/agent/`, `internal/skill/`, `internal/runtime/`, `web/src/components/admin/`
**Files scanned:** ~30
**Pattern extraction date:** 2026-09-07
