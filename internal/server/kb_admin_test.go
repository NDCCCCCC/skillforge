// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Plan 26-05 / KBM-02 — tests for the knowledge-base admin surface.
//
// Two structural locks live here and they point in the OPPOSITE direction
// of kpi_admin_test.go's retrieval-only lock:
//
//  1. kb_admin.go MUST carry POST verbs (KBM-02 is a mutation surface) and
//     every mutation MUST land an INSERT-only audit row — asserted at
//     runtime through the recording store rather than by source-grep.
//  2. CHECK ORDER invariant: the admin-token 403 runs BEFORE the
//     nil-registry 503, so a caller without the token can never probe
//     whether the KB registry is wired (T-26-05-2). TestKBAdminCheckOrder
//     is that lock; TestKBAdminNilRegistry503 is its positive contrast.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/store"
)

// kbAdminSeedYAML is the one-entry controlled seed the KB admin tests
// assemble their registry from. The registry under test is the REAL
// store-backed assembly (kb.Open) — never a hand-rolled double — so the
// 409/404/active-only semantics the handlers rely on are the production
// semantics, not a test-local copy of them.
const kbAdminSeedYAML = `version: "test-26-05"
knowledge_bases:
  - slug: authority
    display_name: Authority
    role: authority
    description: baseline authority corpus
    retrieval_enabled: true
`

// memKBStore is an in-memory kb.Store: the mutation verbs APPLY to the map
// (so a created base is visible to the registry and a deactivation
// persists across a Lookup), which is exactly what the admin handlers need
// to prove. SetRetrievalEnabled fails closed on an unknown slug, mirroring
// the sqlite adapter (D26-02-D).
type memKBStore struct {
	mu   sync.Mutex
	rows map[string]kb.Entry
}

func newMemKBStore(rows ...kb.Entry) *memKBStore {
	m := &memKBStore{rows: make(map[string]kb.Entry, len(rows))}
	for _, r := range rows {
		m.rows[r.Slug] = r
	}
	return m
}

var _ kb.Store = (*memKBStore)(nil)

func (m *memKBStore) LoadBases(_ context.Context) ([]kb.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]kb.Entry, 0, len(m.rows))
	for _, e := range m.rows {
		out = append(out, e)
	}
	return out, nil
}

func (m *memKBStore) UpsertBase(_ context.Context, e kb.Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[e.Slug] = e
	return nil
}

func (m *memKBStore) SetRetrievalEnabled(_ context.Context, slug string, enabled bool, updatedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.rows[slug]
	if !ok {
		return fmt.Errorf("kb %q: unknown slug (fail closed, D26-02-D)", slug)
	}
	cur.RetrievalEnabled = enabled
	cur.UpdatedAt = updatedAt
	m.rows[slug] = cur
	return nil
}

// recordingAuditStore captures the INSERT-only audit rows the KB admin
// handlers write. The embedded nil interface makes every method the
// handlers must NOT call panic on first use (the same posture as
// fakeAdminAuditStore, minus two hundred lines of boilerplate).
type recordingAuditStore struct {
	store.Store
	mu   sync.Mutex
	rows []store.AuditRow
}

var _ store.Store = (*recordingAuditStore)(nil)

func (r *recordingAuditStore) PutAudit(_ context.Context, row store.AuditRow) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows = append(r.rows, row)
	return int64(len(r.rows)), nil
}

func (r *recordingAuditStore) auditRows() []store.AuditRow {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]store.AuditRow, len(r.rows))
	copy(out, r.rows)
	return out
}

// newKBAdminDeps wires a REAL store-backed registry (kb.Open over the
// one-entry seed) plus the audit recorder. extraRows are pre-persisted
// registry rows the seed does not declare (kb.Open keeps them — the seed
// is not a garbage collector), which is how the tests obtain a deactivated
// entry without hand-building a registry.
func newKBAdminDeps(t *testing.T, extraRows ...kb.Entry) (*Deps, *recordingAuditStore) {
	t.Helper()
	seedPath := filepath.Join(t.TempDir(), "knowledge_bases.yaml")
	require.NoError(t, os.WriteFile(seedPath, []byte(kbAdminSeedYAML), 0o644))

	reg, err := kb.Open(context.Background(), seedPath, newMemKBStore(extraRows...))
	require.NoError(t, err)

	audit := &recordingAuditStore{}
	return &Deps{KBRegistry: reg, Store: audit}, audit
}

// newKBAdminHertz binds exactly the four routes this plan registers —
// private re-binding (not New) keeps the harness free of the full route
// table's other deps while still exercising the real handlers.
func newKBAdminHertz(deps *Deps) *server.Hertz {
	h := server.Default(server.WithHostPorts(":0"))
	h.GET("/api/admin/kb", deps.kbAdminHandler)
	h.POST("/api/admin/kb", deps.kbCreateHandler)
	h.POST("/api/admin/kb/:slug/deactivate", deps.kbDeactivateHandler)
	h.GET("/api/knowledge/bases", deps.knowledgeBasesHandler)
	return h
}

// kbRequest performs one request against the harness. jsonBody "" sends no
// body; token "" sends no admin header.
func kbRequest(h *server.Hertz, method, path, jsonBody, token string) *ut.ResponseRecorder {
	headers := []ut.Header{{Key: "Content-Type", Value: "application/json"}}
	if token != "" {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: token})
	}
	var body *ut.Body
	if jsonBody != "" {
		body = &ut.Body{Body: strings.NewReader(jsonBody), Len: len(jsonBody)}
	}
	return ut.PerformRequest(h.Engine, method, path, body, headers...)
}

// TestKBAdminCreate201 — the happy mutation path: a correct token plus a
// valid body answers 201 with the created entry, the registry immediately
// serves the new base, and exactly ONE INSERT-only audit row
// (action_kb_created) carries the entry snapshot in FrozenArgs.
func TestKBAdminCreate201(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	deps, audit := newKBAdminDeps(t)
	h := newKBAdminHertz(deps)

	resp := kbRequest(h, http.MethodPost, "/api/admin/kb",
		`{"slug":"kb-2026","display_name":"KB 2026","role":"personal","description":"ops-created"}`,
		testAdminToken)
	require.Equal(t, http.StatusCreated, resp.Code, "body: %s", resp.Body.String())

	var entry kbAdminEntry
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &entry))
	assert.Equal(t, "kb-2026", entry.Slug)
	assert.Equal(t, "personal", entry.Role)
	assert.Equal(t, "KB 2026", entry.DisplayName)
	assert.True(t, entry.Active, "a freshly created base is active")

	// The created base is live in the registry (Lookup, not just List).
	got, ok := deps.KBRegistry.Lookup("kb-2026")
	require.True(t, ok, "a created base must be immediately resolvable")
	assert.Equal(t, kb.RolePersonal, got.Role)

	// Exactly one INSERT-only audit row, snapshot in FrozenArgs.
	rows := audit.auditRows()
	require.Len(t, rows, 1, "one mutation = one audit row")
	assert.Equal(t, "action_kb_created", rows[0].Action)
	assert.Contains(t, string(rows[0].FrozenArgs), "kb-2026", "FrozenArgs must carry the entry snapshot")
	assert.Equal(t, "admin", rows[0].ActorType, "admin mutations are surfaced by the admin audit sub-view (ADM-06)")
}

// TestKBAdminCreateSlugValidation — invalid slugs 400 with the whitelist
// rule spelled out, and an unknown role 400 with the closed four-value
// vocabulary. The author sees the rule, never just a verdict.
func TestKBAdminCreateSlugValidation(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	deps, audit := newKBAdminDeps(t)
	h := newKBAdminHertz(deps)

	for _, slug := range []string{"Bad_Slug", "../cmd", ""} {
		payload := fmt.Sprintf(`{"slug":%q,"display_name":"x","role":"personal"}`, slug)
		resp := kbRequest(h, http.MethodPost, "/api/admin/kb", payload, testAdminToken)
		require.Equal(t, http.StatusBadRequest, resp.Code, "slug %q must 400 (body: %s)", slug, resp.Body.String())
		assert.Contains(t, resp.Body.String(), "must match",
			"the 400 must carry the slug whitelist rule text")
	}

	resp := kbRequest(h, http.MethodPost, "/api/admin/kb",
		`{"slug":"kb-fine","display_name":"x","role":"admin"}`, testAdminToken)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "must be one of", "the 400 must list the role vocabulary")
	for _, role := range kb.Roles() {
		assert.Contains(t, resp.Body.String(), string(role), "the role list must name %q", role)
	}

	assert.Empty(t, audit.auditRows(), "a rejected create writes no audit row")
}

// TestKBAdminCreateDuplicate409 — a slug that already exists conflicts,
// INCLUDING a deactivated one: re-creating would silently reset a base's
// identity, so deactivation owns the slug forever (KBM-02 has no delete).
func TestKBAdminCreateDuplicate409(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	deps, audit := newKBAdminDeps(t, kb.Entry{
		Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical,
		RetrievalEnabled: false, // deactivated, but still the owner of its slug
	})
	h := newKBAdminHertz(deps)

	resp := kbRequest(h, http.MethodPost, "/api/admin/kb",
		`{"slug":"authority","display_name":"Authority","role":"authority"}`, testAdminToken)
	assert.Equal(t, http.StatusConflict, resp.Code, "an active duplicate must 409")

	resp = kbRequest(h, http.MethodPost, "/api/admin/kb",
		`{"slug":"historical","display_name":"Historical","role":"historical"}`, testAdminToken)
	assert.Equal(t, http.StatusConflict, resp.Code, "a DEACTIVATED duplicate must 409 too")

	assert.Empty(t, audit.auditRows(), "a conflicted create writes no audit row")
}

// TestKBAdminDeactivate — deactivation flips the switch, writes the
// action_kb_deactivated audit row, keeps the entry in the admin List (the
// deactivate-never-destroy contract) and removes it from Lookup (the
// skill-validation / retrieval surface loses it).
func TestKBAdminDeactivate(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	deps, audit := newKBAdminDeps(t)
	h := newKBAdminHertz(deps)

	resp := kbRequest(h, http.MethodPost, "/api/admin/kb/authority/deactivate", "", testAdminToken)
	require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

	var body struct {
		Slug   string `json:"slug"`
		Active bool   `json:"active"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, "authority", body.Slug)
	assert.False(t, body.Active)

	// Audit row landed.
	rows := audit.auditRows()
	require.Len(t, rows, 1)
	assert.Equal(t, "action_kb_deactivated", rows[0].Action)
	assert.Contains(t, string(rows[0].FrozenArgs), "authority")

	// Admin view still lists it — deactivated, not deleted.
	stillListed := false
	for _, e := range deps.KBRegistry.List() {
		if e.Slug == "authority" {
			stillListed = true
			assert.False(t, e.RetrievalEnabled, "the listed entry carries the deactivation")
		}
	}
	assert.True(t, stillListed, "deactivate must never remove the admin-visible entry")

	// ...while the validation/retrieval surface lost it.
	_, ok := deps.KBRegistry.Lookup("authority")
	assert.False(t, ok, "a deactivated base must vanish from Lookup (KBM-02)")
}

// TestKBAdminDeactivateUnknown404 — deactivating an unregistered slug is a
// 404 naming the active set, never a silent 200.
func TestKBAdminDeactivateUnknown404(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	deps, audit := newKBAdminDeps(t)
	h := newKBAdminHertz(deps)

	resp := kbRequest(h, http.MethodPost, "/api/admin/kb/ghost/deactivate", "", testAdminToken)
	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Contains(t, resp.Body.String(), "ghost", "the 404 names the offending slug")
	assert.Empty(t, audit.auditRows(), "a failed deactivation writes no audit row")
}

// TestKBAdminUnsetEnv403 — unset SKILLFORGE_ADMIN_TOKEN denies ALL THREE
// admin routes (fail-closed, isAdminRequest's existing semantics). The
// token header is still attached: an unset env beats any presented token.
func TestKBAdminUnsetEnv403(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "")
	deps, _ := newKBAdminDeps(t)
	h := newKBAdminHertz(deps)

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/kb"},
		{http.MethodPost, "/api/admin/kb"},
		{http.MethodPost, "/api/admin/kb/authority/deactivate"},
	} {
		resp := kbRequest(h, tc.method, tc.path, `{"slug":"kb-2026"}`, testAdminToken)
		assert.Equal(t, http.StatusForbidden, resp.Code, "%s %s must 403 with the env unset", tc.method, tc.path)
	}
}

// TestKBAdminCheckOrder — THE CHECK ORDER LOCK. A nil registry AND no
// token must answer 403, never 503: the admin gate runs FIRST, so an
// unauthenticated caller cannot probe whether the KB registry is wired
// (T-26-05-2 information-disclosure posture, external_auth.go discipline).
func TestKBAdminCheckOrder(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "")
	deps, _ := newKBAdminDeps(t)
	deps.KBRegistry = nil // unwired registry behind the closed gate
	h := newKBAdminHertz(deps)

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/kb"},
		{http.MethodPost, "/api/admin/kb"},
		{http.MethodPost, "/api/admin/kb/authority/deactivate"},
	} {
		resp := kbRequest(h, tc.method, tc.path, `{"slug":"kb-2026"}`, "")
		require.Equal(t, http.StatusForbidden, resp.Code,
			"%s %s: 403 must PRECEDE the nil-registry 503 (CHECK ORDER invariant)", tc.method, tc.path)
		assert.NotContains(t, resp.Body.String(), "not wired",
			"a 403 must not leak configuration state")
	}
}

// TestKBAdminNilRegistry503 — the positive contrast to the CHECK ORDER
// lock: with a VALID token the nil registry surfaces as 503
// (kbRegistryNotWiredError) — fail-closed, never an empty-looking 200.
func TestKBAdminNilRegistry503(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	deps, _ := newKBAdminDeps(t)
	deps.KBRegistry = nil
	h := newKBAdminHertz(deps)

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/kb"},
		{http.MethodPost, "/api/admin/kb"},
		{http.MethodPost, "/api/admin/kb/authority/deactivate"},
	} {
		resp := kbRequest(h, tc.method, tc.path, `{"slug":"kb-2026"}`, testAdminToken)
		require.Equal(t, http.StatusServiceUnavailable, resp.Code, "%s %s", tc.method, tc.path)
		assert.Contains(t, resp.Body.String(), kbRegistryNotWiredError)
	}
}

// TestKnowledgeBasesPublic — the public projection needs NO admin token
// (the chat FE renders tier badges from it) and is information-minimized:
// active entries only, slug/display_name/role only. Description, the
// retrieval switch and deactivated entries never leave the server.
func TestKnowledgeBasesPublic(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	deps, _ := newKBAdminDeps(t, kb.Entry{
		Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical,
		RetrievalEnabled: false,
	})
	h := newKBAdminHertz(deps)

	resp := kbRequest(h, http.MethodGet, "/api/knowledge/bases", "", "")
	require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

	var body struct {
		Bases []knowledgeBasePublicEntry `json:"bases"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Len(t, body.Bases, 1, "deactivated entries must not appear")
	assert.Equal(t, "authority", body.Bases[0].Slug)
	assert.Equal(t, "Authority", body.Bases[0].DisplayName)
	assert.Equal(t, "authority", body.Bases[0].Role)

	// Wire minimization, asserted on the raw payload: no description, no
	// retrieval switch, no timestamps, no deactivated slug anywhere.
	raw := resp.Body.String()
	assert.NotContains(t, raw, "description")
	assert.NotContains(t, raw, "enabled")
	assert.NotContains(t, raw, "created_at")
	assert.NotContains(t, raw, "historical")
}

// TestKBAdminGetList — the admin view answers 200 with the registry
// signature and EVERY entry (deactivated included, active:false).
func TestKBAdminGetList(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	deps, _ := newKBAdminDeps(t, kb.Entry{
		Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical,
		RetrievalEnabled: false,
	})
	h := newKBAdminHertz(deps)

	resp := kbRequest(h, http.MethodGet, "/api/admin/kb", "", testAdminToken)
	require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

	var body kbAdminResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, deps.KBRegistry.Signature(), body.Signature, "signature must be the registry content hash")
	assert.NotEmpty(t, body.Signature)

	require.Len(t, body.Bases, 2, "the admin view lists deactivated entries too")
	bySlug := make(map[string]kbAdminEntry, len(body.Bases))
	for _, e := range body.Bases {
		bySlug[e.Slug] = e
	}
	assert.True(t, bySlug["authority"].Active)
	assert.False(t, bySlug["historical"].Active, "the deactivated entry is flagged, not hidden")
	assert.Equal(t, "authority", bySlug["authority"].Role)
}
