# Phase 26: 知识库注册表地基 (KB Registry Foundation) - Pattern Map

**Mapped:** 2026-09-07
**Files analyzed:** 34 (11 new + 23 modified) — grouped into 13 assignment clusters
**Analogs found:** 13 / 13 clusters matched (every mechanism already has a repo-proven isomorph)

**Headline:** This is a refactor phase whose every *mechanism* already exists in-repo. The research is explicit: "planner 的工作是把既有形态映射到新域，而不是发明新形态". The three dominant analogs are (1) `internal/externalauth/views.go` — the v1.1.1 controlled-file loader precedent, newest and closest; (2) `internal/server/kpi_admin.go` + `external_auth.go` — the admin-token-gate endpoint pair (GET-only vs POST-mutation split maps exactly to KB's read vs create/deactivate surfaces); (3) `internal/skill/registry.go` — the `atomic.Pointer` copy-on-write snapshot. New code should copy these shapes verbatim; inventing a new shape is what gets flagged by the Phase 30 zero-hardcode audit.

---

## File Classification

### New files

| New File | Role | Data Flow | Closest Analog | Match Quality |
|----------|------|-----------|----------------|---------------|
| `internal/kb/kb.go` | model (types + interface) | transform | `internal/kpi/registry.go` (type+interface block) | exact |
| `internal/kb/registry.go` | store (in-memory snapshot) | CRUD (read-heavy) | `internal/skill/registry.go:88-116` + `internal/kpi/registry.go:37-81` | exact |
| `internal/kb/seed.go` | loader (controlled file) | file-I/O | `internal/externalauth/views.go` (LoadViewsDir) | exact |
| `internal/kb/slug.go` | utility (validation) | transform | `internal/externalauth/views.go:144-183` + `internal/kpi/loader.go:413` | exact |
| `internal/kb/store.go` | model (persistence adapter) | CRUD | `internal/store/sqlite/schema.go` kpi_calls block + migrate tail | exact |
| `knowledge_bases.yaml` (new controlled seed) | config | file-I/O | `kpis/loans.yaml` header + `external_views/v_user_consumption.yaml` | exact |
| `internal/server/kb_admin.go` | controller | request-response (read + mutation) | `internal/server/kpi_admin.go` (read half) + `internal/server/external_auth.go` (mutation half) | exact |
| `internal/server/kb_admin_test.go` | test | request-response | `internal/server/kpi_admin_test.go` + `external_auth_test.go` | role-match |
| `internal/knowledge/indexer_registry_test.go` | test | batch | `internal/knowledge/indexer_test.go` (indexSeedHook seam) | role-match |
| `scripts/verify-phase-26.sh` | config (CI gate) | batch | `scripts/verify-phase-16.sh` Stage 09 | exact |
| `web/src/components/admin/KnowledgeBaseList.tsx` | component | request-response | `web/src/components/admin/KPIDefinitionList.tsx` | exact |

### Modified files (Consumer Inventory rows 1-28, grouped by analog)

| Modified File(s) | Role | Data Flow | Closest Analog | Match Quality |
|------------------|------|-----------|----------------|---------------|
| `internal/skill/tier.go` | utility (resolution) | transform | `internal/kpi/registry.go` narrow-interface discipline (DatasetLister, loader.go:39-51) | exact |
| `internal/skill/validate.go` | utility (validation) | transform | `internal/skill/validate.go:87-146` (its own XOR prescan — extend, don't rewrite) | exact (self) |
| `internal/skill/{registry,loader,manifest}.go` | store | CRUD | `internal/skill/registry.go` (inject interface into constructor) | exact (self) |
| `internal/knowledge/indexer.go` | service | batch / file-I/O | `internal/knowledge/indexer.go:68-168` (walker + indexSeedHook seam at :354) | exact (self) |
| `internal/knowledge/graph.go` | service | request-response | `internal/knowledge/graph.go:69-201` | exact (self) |
| `internal/retriever/{milvus/filter.go,mongo/mongo.go,mongo/seed.go,milvus/config.go}` | utility | transform | `internal/retriever/milvus/filter.go:57-78` + `mongo/mongo.go:346-366` | exact (self) |
| `internal/server/knowledge.go` | controller | request-response | `internal/server/knowledge.go:262-273,326-338` | exact (self) |
| `internal/server/annotate.go` | controller | event-driven (pipeline step) | `internal/server/annotate.go:71-82` | exact (self) |
| `internal/server/pipeline.go` | controller | streaming | `internal/server/pipeline.go:215,295-296,1006-1010` | exact (self) |
| `internal/agent/trust_floor.go` | service | event-driven | `internal/agent/trust_floor.go:113,147,197,227-235` | exact (self) |
| `internal/agent/hitl/hitl_memory_facts.go` | provider (tool schema) | request-response | `internal/agent/hitl/hitl_memory_facts.go:235,236,379-380` | exact (self) |
| `internal/agent/{tools/retrieve.go,fanout_tools.go}` + `internal/runtime/sub_agent_builder.go` | provider | transform | `internal/runtime/sub_agent_builder.go:99-135,217-227` | exact (self) |
| `internal/data/rules.go` + `datarules/loans.yaml` | loader (controlled file) | file-I/O | `internal/externalauth/views.go` (closed-set kind validation, :58-63 + :167-169) | exact |
| `cmd/server/main.go` | config (assembly) | request-response (boot) | `cmd/server/main.go:490-526` (LoadViewsDir → log.Fatalf ordering) | exact |
| `cmd/spike-fanout/main.go` + `eval/golden/fanout.jsonl` + `eval/frozen/fanout.lock.json` | test fixture | batch | `internal/eval/fanout_lock_test.go` sha256 re-sign discipline (D14-B4) | exact |
| `cmd/spike-history/long.go` | utility | batch | — (comment rewrite, no code analog; see No Analog Found) | none |
| `internal/store/sqlite/schema.go` + `internal/store/store.go` | model / migration | CRUD | `internal/store/sqlite/schema.go:256-296` (kpi_calls) + `store.go:74-90` (Action enum godoc) | exact |
| `web/src/types/verdict.ts` + `web/src/lib/chunk-lookup.ts` + `web/src/store/app-store.ts` + `web/src/types/knowledge.ts` | model (FE types) | transform | `web/src/components/common/TierBadge.tsx:13-18` | role-match |
| `web/src/components/common/TierBadge.tsx` | component | transform | `web/src/components/common/TierBadge.tsx` (self: Record lookup → fallback map) | exact (self) |
| `web/src/components/admin/AdminDashboard.tsx` | component | request-response | `web/src/components/admin/AdminDashboard.tsx:43-57,107-130` (SubrouteTab) | exact (self) |
| `README.md` + `CONTRIBUTING.md` | config (docs) | — | `CONTRIBUTING.md:117` + `README.md:246` (self) | exact (self) |

---

## Pattern Assignments

### `internal/kb/seed.go` (loader, file-I/O) — THE anchor pattern for this phase

**Analog:** `internal/externalauth/views.go` — the v1.1.1 precedent named explicitly in the phase brief. Copy its package-header posture comment, its "absent dir is the ONE legal degraded state" rule, and its per-entry fail-closed validation that always names the file.

**Package-header posture comment** (views.go:5-20 — copy this framing, s/external_views/knowledge_bases/):
```go
// views.go is the v1.1.1 controlled-file bootstrap for the DATA-10
// view allowlist. The files it loads are CONTROLLED FILES in the same
// sense as kpis/*.yaml (D16-A1): operator-authored, reviewable,
// diffable in git, and the ONLY source of view registrations at
// runtime. Nothing else in the process may register a view into the
// registry the server boots with...
// Posture mirrors the KPI loader (D16-08-D): a malformed or invalid
// controlled file is an ERROR naming the file, and the caller decides
// fatality (cmd/server log.Fatalf's it...). An ABSENT directory, or a
// directory containing zero *.yaml files, is the single legal degraded
// state.
```

**Directory walk + degraded-state handling** (views.go:83-113):
```go
func LoadViewsDir(dir string) (*ViewRegistry, error) {
	reg := NewViewRegistry()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Absent directory: the one legal degraded state.
			return reg, nil
		}
		return nil, fmt.Errorf("externalauth.LoadViewsDir: read %s: %w", dir, err)
	}
	loaded := 0
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".yaml") {
			continue // only *.yaml is a controlled file
		}
		path := filepath.Join(dir, e.Name())
		if err := loadViewsFile(reg, path); err != nil {
			return nil, err
		}
		loaded++
	}
	if loaded == 0 {
		return reg, nil // zero yaml files: same degraded posture
	}
	return reg, nil
}
```

**Closed-set field validation** (views.go:53-63 + :167-169 — this is the shape for validating `role:` values and for `ResolveReads`' `role:` prefix branch):
```go
// validMaskKinds is the closed set of DATA-05 mask kinds a controlled
// file may declare. Anything else is a typo today and a leak tomorrow,
// so it is rejected rather than defaulted.
var validMaskKinds = map[string]bool{"account": true, "phone": true, "email": true, "generic": true}
// ...in validateViewsEntry:
	if !validMaskKinds[m.Kind] {
		return PrecompiledView{}, fmt.Errorf("field_masks[%q]: kind %q must be one of account|phone|email|generic", m.FieldName, m.Kind)
	}
```

**Error form: always name the file + entry index** (views.go:129-138):
```go
	for i, entry := range f.Views {
		v, err := validateViewsEntry(entry)
		if err != nil {
			return fmt.Errorf("externalauth.LoadViewsDir: %s: entry %d (%q): %w", path, i+1, entry.ViewName, err)
		}
		if _, exists := reg.Get(v.ViewName); exists {
			return fmt.Errorf("externalauth.LoadViewsDir: %s: duplicate view name %q (already registered)", path, v.ViewName)
		}
		reg.Register(v)
	}
```

**Secondary analog** — `internal/kpi/loader.go:88-179` adds three elements views.go lacks and KB needs: the **symlink guard** (Stage 1, loader.go:143-150 `fi.Mode()&os.ModeSymlink != 0` → reject, "V12 symlink-swap guard"), the **sorted-deterministic content hash** (loader.go:255-260: `h := sha256.New(); for _, r := range results { h.Write(r.content) }` → `hex.EncodeToString(h.Sum(nil))` — this is the `Signature()` implementation for KB), and the **cross-file uniqueness check** (loader.go:182-199, duplicate → fail whole dir).

---

### `internal/kb/registry.go` (store, in-memory snapshot) + `internal/kb/kb.go`

**Analog A (concurrency):** `internal/skill/registry.go:88-116` — the atomic.Pointer copy-on-write model the research names directly (`internal/skill/registry.go:92`).
```go
type Registry struct {
	skillsDir string
	rootAbs   string
	embedder  embedder.Embedder
	snapshot  atomic.Pointer[map[string]*Entry] // immutable; copy-on-write
	// ...
	failed atomic.Pointer[map[string]string]
}
```
Doc contract to copy (registry.go:63-69): writers build a fresh map and atomically swap; in-flight readers that captured the previous snapshot keep their reference; readers call Snapshot() once per request and treat the returned map as read-only. KB's `Create`/`Deactivate` = read-snapshot → build new map with one entry changed → `atomic.Store`.

**Analog B (immutable read surface):** `internal/kpi/registry.go:37-81` — value-copy semantics + Signature.
```go
type Registry struct {
	defs  map[string]CompiledKPI // unexported: no external mutation
	order []string               // sorted KPI names for stable iteration
	sig   string                 // content-hash signature
}
// Lookup returns ... a value copy — callers cannot mutate the
// registry's internal state (defensive copy...)
func (r *Registry) Lookup(name string) (CompiledKPI, bool) { k, ok := r.defs[name]; return k, ok }
func (r *Registry) Names() []string {
	out := make([]string, len(r.order)); copy(out, r.order); return out
}
func (r *Registry) Signature() string { return r.sig }
```
Note the deliberate contrast: KPI's Registry is immutable-by-construction (no mutator methods, D16-A4) while KB's must expose Create/Deactivate — that difference is *intentional* and must be documented in the KB godoc (KBM-02 wants the mutation surface; D16-08-E's no-backdoor rule is KPI-specific and does not transfer).

**Interface placement (consumer-side interface-first, D18):** `internal/kpi/loader.go:39-51` shows the exact idiom for "leaf package declares narrow interface, production adapter assembled in main.go" — KB's `Registry` interface goes in `internal/kb` and consumers (`skill`, `knowledge`, `server`, `data`) depend on it, never on a concrete struct.
```go
// DatasetLister is the narrow interface for dataset existence checks.
// It returns the set of logical dataset names currently registered...
// internal/kpi does NOT import internal/store (interface isolation...)
// The production implementation adapts store.ListDatasets ... the
// adapter is assembled in main.go.
type DatasetLister interface {
	ListDatasets(ctx context.Context) ([]string, error)
}
```

---

### `internal/kb/slug.go` (utility, validation)

**Analog:** `internal/externalauth/views.go:145-147` (pattern-compile + error quoting the pattern) and `internal/kpi/loader.go:413` (package-level compiled regex const).
```go
// loader.go:409-413
// simpleIdentifierPattern matches a single SQL identifier (v1 KPI dialect).
// Only [A-Za-z_][A-Za-z0-9_]* is allowed — ...
var simpleIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
```
```go
// views.go:145-147
	if !ViewNamePattern.MatchString(e.ViewName) {
		return PrecompiledView{}, fmt.Errorf("view_name %q must match %s", e.ViewName, ViewNamePattern.String())
	}
```
Reserved-word handling mirrors views.go's `validMaskKinds` closed set: a `map[string]bool` of reserved slugs (`data`, plus the 4 role names) rejected with an error that names the conflict (`data` collides with the `data:` channel prefix, D16-C2). **One const, one home:** the slug max-length (19) must live in exactly one package — either `milvus.DefaultTierMaxLen` is read from `kb`, or `kb` exports the limit and milvus consumes it. Two copies is the new-hardcode the research forbids (Consumer row 11).

---

### `internal/server/kb_admin.go` (controller, request-response) — two-parent composite

**Analog A (read surface + envelope):** `internal/server/kpi_admin.go`. Copy the file-header trust-model block (:21-37), the nil-registry 503 const, the response envelope struct with the `signature` field, and the 403-first ordering.
```go
// :48-51
const kpiPipelineNotWiredError = "KPI pipeline not wired (server started without a KPI registry)"
// :93-103 — handler opening: admin gate, then nil check
func (s *Deps) kpiAdminHandler(ctx context.Context, c *app.RequestContext) {
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, map[string]string{"error": "admin token required"})
		return
	}
	if s.KPIRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kpiPipelineNotWiredError})
		return
	}
// :66-79 — envelope with registry signature
type kpiAdminResponse struct {
	Signature string `json:"signature"`
	KPIs []kpiCatalogEntry `json:"kpis"`
	LatestCalls map[string]string `json:"latest_calls"`
}
```
**Critical divergence from KPI (research Pattern R4):** KPI's read-only-by-construction header (:21-28 "This file therefore contains a single retrieval handler and no mutation verb") must **NOT** be copied — KB intentionally provides POST verbs (KBM-02). And `kpi_admin_test.go` contains a structural lock that source-greps kpi_admin.go for absence of mutation verbs; KB's test must assert the *opposite* (create/deactivate handlers exist and write audit rows). Copying the KPI test blind would lock the new file into a contract KBM-02 forbids.

**Analog B (mutation verbs + CHECK ORDER invariant):** `internal/server/external_auth.go:99-127` — the admin-gated POST with the ordering rule locked by tests.
```go
// :28-30 — the invariant
// CHECK ORDER invariant (locked by the tests): every gated handler
// runs the admin check FIRST and only then the nil-manager 503 check —
// a 403 must never leak whether the channel is configured.
// :101-112
func getExternalAuthPending(_ *Deps, mgr *externalauth.Manager) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// CHECK ORDER: admin check FIRST — a 403 must not leak whether
		// the channel is configured.
		if !isAdminRequest(c) {
			c.JSON(http.StatusForbidden, map[string]string{"error": "admin token required"})
			return
		}
		if mgr == nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "external auth not configured"})
			return
		}
```
Body-binding form (:143-147): `var body externalAuthResolveBody; if err := c.Bind(&body); err != nil { c.JSON(http.StatusBadRequest, ...) }`. Status codes for KB: 201 create / 200 deactivate / 409 duplicate slug / 400 invalid slug or body.

**Admin token gate (shared, do not reimplement):** `internal/server/audit.go:114-129`
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
Unset env → 403 for every caller (fail-closed). This is reused as-is; ADM-01 (no new auth model) is a locked v1.0 decision.

**Audit row write:** `internal/server/runner_setup.go:320-333` is the canonical PutAudit call shape.
```go
func (a *fanOutAuditAdapter) PutAudit(ctx context.Context, action, runPath, ...) error {
	_, err := a.store.PutAudit(ctx, store.AuditRow{
		Timestamp:            time.Now().UTC(),
		SessionID:            a.sessionID,
		SkillID:              a.skillID,
		Action:               action,
		...
	})
	return err
}
```
`AuditRow.Action` is an enumerated string whose godoc list must be extended: `internal/store/store.go:74-81` currently reads `One of "action_pending_approval" | "action_approved" | ... | "action_rule_revoked"`. Add `action_kb_created` / `action_kb_deactivated` (+ `action_kb_dir_unregistered` for the indexer path) to that doc list — A7 in the research flags that skipping this breaks AuditFilter consumers in the admin_audit FE view.

**Route registration:** `internal/server/server.go:462` (`h.GET("/api/admin/kpis", deps.kpiAdminHandler)`) and the external-auth block at :521-526 show both forms — deps-method handlers (`deps.kbAdminHandler`) and closure-bound handlers (`postExternalAuthRequest(deps, mgr)`). Register KB routes adjacent to the kpi/external-auth blocks with the same comment style.

---

### `internal/kb/store.go` + `internal/store/sqlite/schema.go` (model, migration)

**Analog:** the `kpi_calls` DDL block — the phase's named precedent for a separate INSERT-only audit companion, and the D14-E1 additive-migration discipline.
```go
// schema.go:281-296
-- INSERT-only discipline: rows are appended by PutKPICall; there is NO
-- mutation or removal path anywhere in kpi_calls.go (source-grep
-- structural lock in kpi_calls_test.go, mirroring the import_batches
-- anti-tampering posture).
CREATE TABLE IF NOT EXISTS kpi_calls (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ts              TEXT    NOT NULL,
    ...
    signature       TEXT    NOT NULL DEFAULT ''
);
```
The `knowledge_bases` table itself is a **mutable config table** (research Alternative table: `datasets` has real DELETE, store.go:718 — "配置≠证据" split is established). Note the deliberate asymmetry to state in the table's comment block: `knowledge_bases` is mutable (UPDATE retrieval_enabled), the KB audit rows land in the **existing** `audit` table via PutAudit (not a new table) — "kpi_calls 专用表先例仅在审计行装不下时才走" (research Don't-Hand-Roll table).

**Migration mechanics:** `schema.go:572-594` — new table goes into `schemaSQL` (CREATE TABLE IF NOT EXISTS = idempotent); new columns on existing tables go through the `columnExists` PRAGMA gate loop (:546-570). A brand-new table needs only the schemaSQL entry plus the `CREATE INDEX IF NOT EXISTS` in the migrate-tail position (:625-629 shows the "index created AFTER the ALTERs" convention — keep indexes at the tail even when no ALTER dependency exists, per the research's SQL comment).

---

### `knowledge_bases.yaml` (config, controlled seed file)

**Analog:** `kpis/loans.yaml:1-11` header form.
```yaml
# kpis/loans.yaml — KPI definitions for the loans dataset
# CONTROLLED FILE: versioned in git, changed via PR review (D16-A1).
# Data staff author this file; the server loads it fail-closed at startup.
# Signature = content hash rides every kpi_calls audit row so "which KPI
# version produced this result" is always provable (KPI-02).
```
On-disk struct shape comes from `internal/externalauth/views.go:35-51` (`viewsFile`/`viewsEntry` with `yaml:"..."` tags). The research's Code Examples section (:455-483) gives the full seed content including the load-bearing rule that must be unit-locked: **draft-role KBs are never retrievable even when `retrieval_enabled: true`** (role semantics outrank the entry switch — `TestDraftRoleNeverRetrievable`).

**Seed-vs-runtime semantics (Pitfall 6, locked recommendation):** "结构归配置、开关归运营" — YAML always upserts declared fields (slug/display_name/role/description) but `retrieval_enabled` defers to the sqlite current value. Write this into `seed.go`'s godoc and lock it with a "deactivate → restart → still deactivated" test.

---

### `internal/skill/tier.go` + `internal/skill/validate.go` (utility, transform — the breaking change)

**Analog (the function being replaced — its error shape is the contract to preserve):** `internal/skill/tier.go:120-162` `ResolveTiers`. The new `registry.ResolveReads` keeps this loop skeleton, swaps `TierEnum[top]` for `r.Lookup(top)`, and adds a `role:` prefix branch:
```go
// tier.go:140-160 — the loop to preserve
	for _, r := range reads {
		top := r
		if i := strings.Index(r, "/"); i >= 0 { top = r[:i] }
		tier, ok := TierEnum[top]
		if !ok {
			return nil, fmt.Errorf(
				"reads %q: unknown tier dir %q (known: %v)",
				r, top, knownTierDirs(),
			)
		}
		if !seen[tier] { seen[tier] = true; out = append(out, tier) }
	}
```
Keep: the `data:` defense-in-depth guard (:130-137 — reject data:-prefixed values reaching the tier resolver), the dedupe, the "unknown → error listing known values" error form (now listing active slugs). Keep the function name `ResolveTiers` per the research ("保函数名以缩小爆炸半径") and have it delegate to the registry.

**Allowlist/XOR shape to preserve:** `internal/skill/validate.go:87-146` — the data-channel prescan (:102-109) rejecting unknown `data:` modes, the XOR branch (:110-138), and the tier branch calling ResolveTiers (:139-146). Only the tier branch's callee changes; the prescan and XOR stay byte-identical. Empty registry + tier Skill → validation failure is the *correct* fail-closed behavior, which is why boot order (seed → store migrate → registry → skill loader) is load-bearing (Consumer row 2).

**Boot-order analog:** `cmd/server/main.go:490-526` — the LoadViewsDir wiring block is the exact template for the KB seed step (including the WARN-on-empty and log.Fatalf-on-error postures), and it must run **before** `skill.New("skills/", nil)` at :520.
```go
	viewsDir := envOr("SKILLFORGE_EXTERNAL_VIEWS_DIR", "external_views")
	extRegistry, err := externalauth.LoadViewsDir(viewsDir)
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: external view load from %s: %v (fix the controlled view file or remove it from the directory...)", viewsDir, err)
	}
	...
	reg, err := skill.New("skills/", nil)
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: skill registry: %v (fix the SKILL.md or remove it from skills/)", err)
	}
```

---

### `internal/knowledge/indexer.go` (service, batch / file-I/O) — walker convergence (Pitfall 4)

**Analog (self):** `internal/knowledge/indexer.go:68-168`. The `knownTierDirs` var (:68-73) and the sorted-dir walk (:132-159) are what get replaced by `os.ReadDir` + registry matching; the missing-registry fail-fast at :141-146 is the exact spirit that becomes "unknown dir → WARN + skip + audit":
```go
	for _, tierDir := range sortedTiers {
		tierVal, ok := skill.TierEnum[tierDir]
		if !ok {
			// This would indicate a code-level bug — the two lists drifted.
			// Fail fast; do not silently skip.
			return fmt.Errorf("knowledge.Index: tier %q missing from skill.TierEnum (loader + skill must stay in sync)", tierDir)
		}
		tierAbs := filepath.Join(knowledgeDir, tierDir)
		info, err := os.Stat(tierAbs)
		if err != nil {
			if os.IsNotExist(err) { continue } // Empty / missing tier dir IS valid (SC-4). Skip silently.
			return fmt.Errorf("knowledge.Index: stat %s: %w", tierAbs, err)
		}
```
Preserve: silent skip for *missing registered* dirs (SC-4 refusal mechanism still depends on empty-dir-validity), lexical `sort.Strings(matches)` for deterministic IDs, the maxDocBytes warn-and-skip (:57-66).

**Audit without coupling (Pitfall 7):** the test-seam idiom to copy is `indexSeedHook` (indexer.go:354 region) — an optional package-level hook var that production sets and hermetic tests stub. `Index` gains `AuditFn func(action string, detail string)` the same way; `cmd/server/main.go` binds it to PutAudit, tests bind an in-memory collector. Do **not** add `store.Store` as a parameter — it drags sqlite into cmd/eval and every test path.

**Walker convergence:** `internal/retriever/mongo/seed.go:66` holds a hand-copied duplicate of the dir list (its comment says "Mirrors internal/skill.TierEnum keys"). The fix is one shared walker in `internal/knowledge` (mongo/seed.go already imports internal/skill, so no new reverse dependency) with a shared behavior test on both sides.

---

### `internal/retriever/{milvus/filter.go, mongo/mongo.go, milvus/config.go}` (utility, transform)

**Signatures unchanged; values become slugs.** Copy nothing new — protect these two invariants.

Milvus empty-set default-deny + `%q` escaping (`internal/retriever/milvus/filter.go:57-78`):
```go
func BuildMilvusFilter(tiers []string, now int64) string {
	if len(tiers) == 0 {
		return "false" // matches zero rows; "" would be default-ALLOW — violates D-09
	}
	quoted := make([]string, len(tiers))
	for i, t := range tiers {
		quoted[i] = fmt.Sprintf("%q", t) // injection-safe for all tier values
	}
	tierClause := fmt.Sprintf("tier in [%s]", strings.Join(quoted, ","))
	effectiveClause := fmt.Sprintf("(effective_to == 0 or effective_to >= %d)", now)
	return tierClause + " and " + effectiveClause
}
```
Mongo `$in` empty-set default-deny (`internal/retriever/mongo/mongo.go:346-366`) — note the doc comment is the canonical statement of WHY the filter must always be built:
```go
	// {tier: {$in: tiers}}. Empty tiers → $in:[] → no matches → 0 hits
	// (D-09). This is THE trust boundary: building the filter any other
	// way (e.g. skipping the find on empty tiers) would still produce
	// 0 rows but would not be the DATABASE refusing — the structural
	// guarantee would be in Go, not in the storage layer. Always build
	// the filter, always run the find.
	filter := bson.D{
		{Key: "tier", Value: bson.D{{Key: "$in", Value: tiers}}},
		{Key: "$or", Value: bson.A{ ... }},
	}
```
Edits here are comment-only (CN-tier wording → slug wording) plus `config.go:44` `DefaultTierMaxLen = 20` re-homing decision (see slug.go above).

---

### `internal/server/annotate.go` + `internal/server/pipeline.go` + `internal/agent/trust_floor.go` (citation whitelist, KBM-03)

**Analog (self):** `internal/server/annotate.go:71-82` — today's hardcoded counter-evidence tier and its test pin. The replacement (`SlugsByRole(kb.RoleHistorical)`, empty → 0-hit search) must keep the test's *shape*: `annotate_test.go:236-248` asserts the literal `[]string{"历史"}` via `stubRetriever.LastTiers()`; the new assertion is "LastTiers() == registry SlugsByRole(historical)" and the `:163-212` chunksByID snapshot test must stay green.
```go
	// Retrieve against the 历史 tier ONLY. A leak to 权威 / 个人 here
	// would route grounded material into the annotation channel and
	// break the moat — TestCounterEvidence_PassesHistoryTierOnly pins
	// the literal tier slice via stubRetriever.LastTiers().
	hits, err := s.Retriever.Search(ctx, qvec, []string{"历史"}, 5, now)
```
The citability gate goes at the chunk-insertion point (`ChunkAccumulator.Add`, trust_floor.go:113/:147) — see research Pattern R3 excerpt for the nil-registry fail-open-to-legacy form. Warning to carry into the plan verbatim: non-authority chunks must never be placed in the prompt *outside* the annotation frame — LLM citation then triggers grounding stripping and innocent degradation (Pitfall 2).

---

### `internal/data/rules.go` + `datarules/loans.yaml` (loader, file-I/O)

**Analog:** `internal/externalauth/views.go` closed-set validation (shown above). `rules.go:146-149` validates `tier:` with `skill.TierEnumByValue` (excluding draft); the replacement is `kb.Registry.Lookup(slug)` + `Role != kb.RoleDraft`. The controlled file `datarules/loans.yaml:3` currently carries `tier: 历史` — this is one of only two data files that MUST change spelling (to `tier: historical`), the other being `eval/golden/fanout.jsonl`.

---

### `eval/golden/fanout.jsonl` + `eval/frozen/fanout.lock.json` + `cmd/spike-fanout/main.go` (test fixture, batch)

**Analog:** the frozen-lock re-sign discipline (`internal/eval/fanout_lock_test.go:37-104`; also visible as the sha256 `check()` helper in `scripts/verify-phase-16.sh:366`). `eval_set_sha256` pins the LF-normalized golden content byte-for-byte — changing golden without re-signing turns `TestFanoutLock*` red, and treating that red as "don't touch golden" leaves a half-migrated tree. Plan this as ONE continuous task: edit golden → recompute sha256 → update lock → record the re-sign reason in SUMMARY. `tier_ground_truth` values move from CN chars (`"权威"`) to slugs (`"authority"`); `cmd/spike-fanout/main_test.go:447` reads golden by relative path and its structural assertions follow the same edit.

---

### `scripts/verify-phase-26.sh` + Makefile target (config, CI gate)

**Analog:** `scripts/verify-phase-16.sh` Stage 09 (:319-334) — the `run_stage "..."` + `grep -q` source-lock form, and the Makefile target precedent from verify-phase-15/16.
```bash
run_stage "structural-constant source-grep locks (KMinDefault + ReservedParamAsOf) + cmd/server link-blocked-with-notes" \
    bash -c '
    grep -q 'KMinDefault = 5' internal/kpi/kmin.go || { echo 'FATAL: KMinDefault = 5 literal missing from internal/kpi/kmin.go'; exit 1; }
    grep -q 'ReservedParamAsOf = \"as_of\"' internal/kpi/executor.go || { echo 'FATAL: ...'; exit 1; }
    if [ -f kpis/loans.yaml ] || ...; then true; else echo 'FATAL: no kpis/*.yaml fixture present (D16-A1 受控文件)'; exit 1; fi
```
Three gates per research R7, with the false-positive hardening that is the gate's real engineering content (Pitfall 5):
- Gate 1 TODO: match only `TODO(` / `TODO:` — a bare `\bTODO\b` reds on 4 `deep.TODO` type references (`internal/agent/deep_audit.go:158`, `internal/store/sqlite/schema.go:171`, `internal/wire/chunk.go:270,273`). Write the convention into CONTRIBUTING.
- Gate 2: `grep -rn 'TierEnum' --include='*.go' internal cmd | grep -v '_test.go'` — deletion evidence.
- Gate 3: CN tier literal ban with an explicit whitelist (`internal/platformfloor/platform_floor.go:111,142` "个人/PII 信息"/"历史回复" and `internal/history/verdict_renderer.go:31` are natural-language, not tier values) + whitelist comments with a review date.
- Order: write the gate first, prove it reds, then clear the field (D8-04 negative-proof discipline).

---

### `web/src/components/admin/KnowledgeBaseList.tsx` (component, request-response)

**Analog (exact):** `web/src/components/admin/KPIDefinitionList.tsx`. Copy its skeleton: interface block mirroring the wire shape (:16-27), `useEffect` fetch with the admin token from localStorage (:69-94), the loading skeleton / error / empty-state trio (:45-62, :96-110), and the table layout (:112-121). Divergence: KB adds create + deactivate forms (KBM-02), so this view is the first admin view with mutation verbs — keep the fetch/token pattern, add a POST helper with the same `X-Admin-Token` header.
```tsx
	interface KPIAdminResponse {
	  signature: string;
	  kpis: KPIRow[];
	  latest_calls: Record<string, string>;
	}
	...
	useEffect(() => {
	  const token = localStorage.getItem('sf_admin_token') ?? '';
	  fetch('/api/admin/kpis', { headers: { 'X-Admin-Token': token } })
	    .then((resp) => { if (!resp.ok) { setError(true); setLoading(false); return null; } return resp.json() as Promise<KPIAdminResponse>; })
```

**Tab shell:** `web/src/components/admin/AdminDashboard.tsx:43-57` (`handleSubroute` union + URL sync) and :107-130 (`SubrouteTab` props). Adding a `bases` subroute means extending the `'batches' | 'kpis' | 'skills' | 'schedules'` union at :43 and the store's `adminSubroute` type.

**Tier consumption:** `web/src/components/common/TierBadge.tsx:13-18` — `TIER_LABELS: Record<Tier, string>` with 4 CN keys becomes `Record<string, string>` fed from the registry endpoint with a `` `[${tier}]` `` fallback (unknown slugs from historical audit replays must degrade, not crash; chunk-lookup.ts:131,148's hardcoded `tier: '权威'` fallback is deleted per the zero-hardcode standard). `web/src/types/verdict.ts:48-49` `Tier` union widens to `string`; GraphCanvas already loops `tiers.length` (GraphCanvas.tsx:58-62) so N-KB is naturally compatible — Phase 26 only guarantees no-break, Phase 28 owns the upgrade.

---

## Shared Patterns

### 1. Admin token gate — `isAdminRequest`
**Source:** `internal/server/audit.go:114-129`
**Apply to:** `internal/server/kb_admin.go` (all three handlers), `internal/server/kb_admin_test.go` (403 cases incl. unset-env)
```go
	expected := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	if expected == "" { return false }
	...subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
```
Plus the CHECK ORDER invariant from `external_auth.go:28-30`: admin check first, nil-registry 503 second — a 403 must never leak configuration state.

### 2. Controlled-file fail-closed loading
**Source:** `internal/externalauth/views.go:5-20` (posture), `:83-113` (walk + degraded state), `internal/kpi/loader.go:143-179` (symlink guard + sha256 + per-file naming errors)
**Apply to:** `internal/kb/seed.go`, `knowledge_bases.yaml`, and the boot wiring in `cmd/server/main.go` (log.Fatalf on bad controlled file; absent/empty dir is the only legal degraded state)

### 3. Atomic copy-on-write snapshot
**Source:** `internal/skill/registry.go:88-116`, `internal/kpi/registry.go:37-81`
**Apply to:** `internal/kb/registry.go` (readers get an immutable map ref; writers build-fresh-then-swap; all returned slices/maps are defensive copies; `Signature()` = sha256 over sorted seed content)

### 4. INSERT-only audit rows + Action enum godoc
**Source:** `internal/server/runner_setup.go:320-333` (call shape), `internal/store/store.go:74-81` (enum doc list to extend)
**Apply to:** create/deactivate handlers (`action_kb_created` / `action_kb_deactivated`), indexer unknown-dir path (`action_kb_dir_unregistered` via the AuditFn callback). Never UPDATE/DELETE an audit row; no new audit table unless a row genuinely doesn't fit.

### 5. Default-deny at the storage layer
**Source:** `internal/retriever/milvus/filter.go:57-66` (`return "false"`), `internal/retriever/mongo/mongo.go:346-366` (`$in:[]`, "Always build the filter, always run the find")
**Apply to:** every retriever touch — signatures and PRE-similarity position are frozen (D7); only the value source changes to registry slugs. Empty slug set → DB refuses → D-09 refusal. Unregistered slugs must be rejected earlier (ResolveReads / validateTiers) and never reach these functions.

### 6. Fail-closed error messages that list the valid set
**Source:** `internal/skill/tier.go:150-155` (`unknown tier dir %q (known: %v)`), `internal/kpi/loader.go:226-228` (names the registered set)
**Apply to:** `ResolveReads` unknown/deactivated slug errors and `validateTiers` replacement — the Skill author must see the active slug list, never a bare "invalid".

### 7. Structural source-grep gate
**Source:** `scripts/verify-phase-16.sh:319-334` (Stage 09 form), `:366` (sha256 lock form for the golden re-sign)
**Apply to:** `scripts/verify-phase-26.sh` + Makefile target; gate false-positive hardening per Pitfall 5 before clearing TODOs (RED → GREEN).

### 8. Test seams instead of dependency params
**Source:** `internal/knowledge/indexer.go` `indexSeedHook` (:354 region); `internal/server/*_test.go` fake/spy store structs (e.g. `admin_audit_test.go:82`)
**Apply to:** `Index`'s new `AuditFn` callback and `kb_admin_test.go`'s fake registry/store — hermetic tests must not construct sqlite or reach live Milvus/Mongo (env-intranet flakiness).

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `cmd/spike-history/long.go:54` | utility | batch | Comment rewrite, not code. No code analog exists for "dispose a stale TODO" — but the *content* is decided by the research: summarization middleware was never implemented (all `summariz*` hits are the spike's own `--summarize` reporting); Phase 10's actual escape hatch is `internal/history/projection.go` `TruncateToCharBudget`. Rewrite the line as a factual statement with zero TODO vocabulary (the gate would otherwise stay red). |
| `internal/agent/hitl/hitl_memory_facts.go` Enum replacement | provider | request-response | No existing "dynamic enum from a registry in a tool JSON schema" precedent — every other tool schema is static. Closest spirit is `internal/kpi/registry.go` Catalog() feeding a tool description. Planner should treat this as the one genuinely novel assembly: snapshot active-KB display names at BuildRunner construction, default = first authority-role KB slug. Hitl tests :63, :218-230, :281 carry literals that must follow. |
| `README.md:246` / `CONTRIBUTING.md:117` rewrite | config (docs) | — | Doc rewrite; the four-role semantics section (KBM-03) has no prior art in-repo. Content source = research KBM-03 definitions + `internal/kb/kb.go` `Roles()` godoc as the code-level anchor ("语义写入平台地板文档与 README"). |

---

## Metadata

**Analog search scope:** `internal/kpi`, `internal/externalauth`, `internal/server`, `internal/skill`, `internal/knowledge`, `internal/retriever/{milvus,mongo}`, `internal/store/{,sqlite}`, `cmd/server`, `cmd/spike-history`, `scripts`, `web/src/components/{admin,common}`, `kpis/`
**Files read in this session:** 17 full/partial (`kpi/loader.go`, `kpi/registry.go`, `externalauth/views.go`, `server/kpi_admin.go`, `server/audit.go`, `server/external_auth.go`, `server/annotate.go`, `server/runner_setup.go`, `skill/registry.go`, `skill/tier.go`, `skill/validate.go`, `knowledge/indexer.go`, `retriever/milvus/filter.go`, `retriever/mongo/mongo.go`, `store/sqlite/schema.go`, `store/store.go`, `cmd/server/main.go`, `cmd/spike-history/long.go`, `scripts/verify-phase-16.sh`, `kpis/loans.yaml`, `web/src/components/common/TierBadge.tsx`, `web/src/components/admin/KPIDefinitionList.tsx`, `web/src/components/admin/AdminDashboard.tsx`)
**Pattern extraction date:** 2026-09-07
**Research alignment:** every assignment cross-references a Consumer Inventory row # or Pattern R1-R7 in `26-RESEARCH.md`; line anchors in the research were re-verified against the working tree this session. No `internal/kb` package exists yet (Wave 0 confirmed).
