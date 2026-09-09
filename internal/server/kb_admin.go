// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Plan 26-05 / KBM-02 + KBM-03 — the knowledge-base registry admin surface
// and the public read-only projection the chat FE consumes.
//
// TRUST MODEL — deliberately different from kpi_admin.go. KPI definitions
// live in controlled files, so that surface is READ-ONLY BY CONSTRUCTION
// (D16-08-E) and its test source-greps the file for the ABSENCE of
// mutation verbs. Knowledge bases are operational content: KBM-02
// explicitly requires a runtime create/deactivate surface, so this file
// INTENTIONALLY carries POST verbs and kb_admin_test.go locks their
// PRESENCE. Copying the KPI posture here would lock the file into a
// contract KBM-02 forbids. The compensating controls are:
//
//   - every admin handler runs the X-Admin-Token gate (isAdminRequest —
//     crypto/subtle constant-time compare, unset env → 403 for every
//     caller). ADM-01: no new auth model is introduced;
//   - CHECK ORDER invariant (locked by TestKBAdminCheckOrder): the admin
//     check runs FIRST and only then the nil-registry 503 check — a 403
//     must never leak whether the KB registry is wired. Same discipline
//     as external_auth.go;
//   - every successful mutation writes ONE INSERT-only audit row
//     (action_kb_created / action_kb_deactivated) with the resulting
//     kb.Entry snapshot in FrozenArgs. No code path UPDATEs or DELETEs an
//     audit row, and there is NO delete endpoint: deactivation hides a
//     base, it never destroys it (KBM-02 scope is create/deactivate only);
//   - no caller input reaches SQL: mutations go through kb.Registry,
//     whose sqlite adapter is fully ?-parameterized (26-02), and slugs
//     cross the kb whitelist (kb.ValidateSlug) before anything else.
//
// Surfaces:
//
//	GET  /api/admin/kb                  → 200 {signature, bases:[kbAdminEntry]}   (deactivated included)
//	                                    → 403 | 503 (registry not wired)
//	POST /api/admin/kb                  → 201 kbAdminEntry | 400 (slug/role rule text)
//	                                    → 403 | 409 (slug exists, deactivated included) | 503
//	POST /api/admin/kb/:slug/deactivate → 200 {"slug":…,"active":false}
//	                                    → 403 | 404 (unknown slug) | 503
//	GET  /api/knowledge/bases           → 200 {bases:[{slug,display_name,role}]}  (NO token; active only)
//	                                    → 503 (registry not wired)
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/store"
)

// kbRegistryNotWiredError is the 503 body message when Deps.KBRegistry is
// nil — the server booted without a knowledge-base registry, so no KB
// surface can answer truthfully (fail-closed rather than an empty-looking
// 200). POST /api/skills' tier-channel gate in skills.go shares this exact
// wording through the same constant.
const kbRegistryNotWiredError = "KB registry not wired (server started without a knowledge-base registry)"

// kbAdminEntry is one registry entry as rendered for the admin view. The
// field names are the 26-08 FE wire contract — do not rename.
type kbAdminEntry struct {
	// Slug is the registry key and the knowledge/<slug>/ directory name.
	Slug string `json:"slug"`
	// DisplayName is the UI badge text (seed-authored).
	DisplayName string `json:"display_name"`
	// Role is the closed four-value semantic role.
	Role string `json:"role"`
	// Description is the operator-facing explanation from the seed.
	Description string `json:"description"`
	// Active mirrors the retrieval switch: false = deactivated (still
	// listed — the admin view is where a deactivated base stays visible).
	Active bool `json:"active"`
	// CreatedAt / UpdatedAt are RFC3339 UTC timestamps of the registry row.
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// kbAdminResponse is the 200 envelope for GET /api/admin/kb.
type kbAdminResponse struct {
	// Signature is the registry content hash — the same value every
	// resolution and audit row can be replayed against (26-01).
	Signature string         `json:"signature"`
	Bases     []kbAdminEntry `json:"bases"`
}

// kbCreateBody is the wire shape of POST /api/admin/kb.
type kbCreateBody struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Description string `json:"description"`
}

// knowledgeBasePublicEntry is one entry of the PUBLIC projection. The
// three fields are exactly what a tier badge renders — anything more
// (description, the retrieval switch, timestamps, deactivated entries)
// would widen the unauthenticated surface for no UI gain (T-26-05-2).
type knowledgeBasePublicEntry struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// knowledgeBasesPublicResponse is the 200 envelope for
// GET /api/knowledge/bases.
type knowledgeBasesPublicResponse struct {
	Bases []knowledgeBasePublicEntry `json:"bases"`
}

// newKBAdminEntry projects a registry entry onto the admin wire shape
// (RFC3339 UTC timestamps, role as its string form).
func newKBAdminEntry(e kb.Entry) kbAdminEntry {
	return kbAdminEntry{
		Slug:        e.Slug,
		DisplayName: e.DisplayName,
		Role:        string(e.Role),
		Description: e.Description,
		Active:      e.RetrievalEnabled,
		CreatedAt:   e.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   e.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// kbAdminHandler serves GET /api/admin/kb — the admin view of the
// registry: the content signature plus EVERY entry, deactivated included
// (a deactivated base must stay visible somewhere, and this is that
// somewhere).
func (s *Deps) kbAdminHandler(ctx context.Context, c *app.RequestContext) {
	// CHECK ORDER: admin check FIRST — a 403 must not leak whether the
	// KB registry is wired.
	if !requireAdmin(c) {
		return
	}
	if s.KBRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kbRegistryNotWiredError})
		return
	}

	entries := s.KBRegistry.List()
	bases := make([]kbAdminEntry, 0, len(entries))
	for _, e := range entries {
		bases = append(bases, newKBAdminEntry(e))
	}
	c.JSON(http.StatusOK, kbAdminResponse{
		Signature: s.KBRegistry.Signature(),
		Bases:     bases,
	})
}

// kbCreateHandler serves POST /api/admin/kb — the KBM-02 create verb.
//
// Order of gates: admin token → wired registry → body decode → slug
// whitelist → role vocabulary → duplicate slug → registry.Create. The
// slug/role pre-checks exist so the author sees the RULE (the whitelist
// pattern, the closed role list) rather than a bare verdict; the registry
// re-runs the same checks inside Create, which is defense in depth, not a
// second source of truth.
func (s *Deps) kbCreateHandler(ctx context.Context, c *app.RequestContext) {
	// CHECK ORDER: admin check FIRST — a 403 must not leak whether the
	// KB registry is wired.
	if !requireAdmin(c) {
		return
	}
	if s.KBRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kbRegistryNotWiredError})
		return
	}

	var body kbCreateBody
	if err := c.Bind(&body); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid body: " + err.Error()})
		return
	}

	// Slug whitelist gate: the 400 carries the rule text so the operator
	// can fix the slug without a round trip to the docs.
	if err := kb.ValidateSlug(body.Slug); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: err.Error()})
		return
	}
	// Closed role vocabulary: an unknown role is a typo today and a trust
	// blur tomorrow, so it is refused with the four-value list.
	if !kb.IsRole(body.Role) {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: fmt.Sprintf("role %q must be one of %v (roles are platform semantics, not configuration)", body.Role, kb.Roles()),
		})
		return
	}
	// Duplicate slug → 409, checked across ACTIVE AND DEACTIVATED entries:
	// a deactivated base still owns its slug, and re-creating it would
	// silently reset a registry identity that audit rows reference.
	for _, e := range s.KBRegistry.List() {
		if e.Slug == body.Slug {
			c.JSON(http.StatusConflict, graphResponseError{
				Error: fmt.Sprintf("kb %q is already registered (deactivate it instead of re-creating)", body.Slug),
			})
			return
		}
	}

	entry, err := s.KBRegistry.Create(kb.CreateInput{
		Slug:        body.Slug,
		DisplayName: body.DisplayName,
		Role:        kb.Role(body.Role),
		Description: body.Description,
	})
	if err != nil {
		if errors.Is(err, kb.ErrReadOnly) {
			// A registry opened without a Store is a wiring defect, not bad
			// input — the same family as a nil registry.
			c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: err.Error()})
			return
		}
		// Remaining failures are validation (reserved slug for NEW bases,
		// empty display name). The message is author-facing rule text.
		c.JSON(http.StatusBadRequest, graphResponseError{Error: err.Error()})
		return
	}

	s.putKBAdminAuditRow(ctx, "action_kb_created", entry)
	c.JSON(http.StatusCreated, newKBAdminEntry(entry))
}

// kbDeactivateHandler serves POST /api/admin/kb/:slug/deactivate — the
// KBM-02 deactivate verb. The entry stays listed for admins and its data
// is untouched; it vanishes from Lookup (so skills referencing it stop
// loading and retrieval loses it) — deactivate, never destroy.
func (s *Deps) kbDeactivateHandler(ctx context.Context, c *app.RequestContext) {
	// CHECK ORDER: admin check FIRST — a 403 must not leak whether the
	// KB registry is wired.
	if !requireAdmin(c) {
		return
	}
	if s.KBRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kbRegistryNotWiredError})
		return
	}

	slug := c.Param("slug")

	// Unknown slug → 404 naming the active set (the operator sees what
	// IS deactivateable), decided from List() so the pre-check cannot be
	// confused with a persistence failure.
	known := false
	for _, e := range s.KBRegistry.List() {
		if e.Slug == slug {
			known = true
			break
		}
	}
	if !known {
		c.JSON(http.StatusNotFound, graphResponseError{
			Error: fmt.Sprintf("kb %q is not registered (active: %v)", slug, s.KBRegistry.ActiveSlugs()),
		})
		return
	}

	entry, err := s.KBRegistry.Deactivate(slug)
	if err != nil {
		if errors.Is(err, kb.ErrReadOnly) {
			c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: err.Error()})
			return
		}
		// A persistence failure is an internal fault: W-3 posture — the
		// detail is logged, the envelope stays generic.
		log.Printf("[WARN] server: kb deactivate %q failed: %v", slug, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "kb deactivate failed — see server logs"})
		return
	}

	s.putKBAdminAuditRow(ctx, "action_kb_deactivated", entry)
	c.JSON(http.StatusOK, map[string]any{"slug": entry.Slug, "active": false})
}

// knowledgeBasesHandler serves GET /api/knowledge/bases — the PUBLIC
// read-only projection. Deliberately NO admin gate: the chat FE needs the
// slug → display_name mapping to render tier badges, and the payload
// carries nothing sensitive (active entries only; slug, display name and
// role only). Information minimization replaces authentication here —
// description, the retrieval switch, timestamps and deactivated entries
// never leave the server (T-26-05-2).
//
// A nil registry still answers 503: an empty-looking 200 would have the FE
// render "no knowledge bases", which is a different operational fact.
func (s *Deps) knowledgeBasesHandler(ctx context.Context, c *app.RequestContext) {
	if s.KBRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kbRegistryNotWiredError})
		return
	}

	entries := s.KBRegistry.List()
	bases := make([]knowledgeBasePublicEntry, 0, len(entries))
	for _, e := range entries {
		if !e.RetrievalEnabled {
			continue // deactivated bases are invisible to the public surface
		}
		bases = append(bases, knowledgeBasePublicEntry{
			Slug:        e.Slug,
			DisplayName: e.DisplayName,
			Role:        string(e.Role),
		})
	}
	c.JSON(http.StatusOK, knowledgeBasesPublicResponse{Bases: bases})
}

// putKBAdminAuditRow writes the INSERT-only audit row for one KB mutation
// (action_kb_created / action_kb_deactivated) with the resulting entry
// snapshot in FrozenArgs and ActorType "admin" so GET /api/admin/audit
// (ADM-06) surfaces the row.
//
// Best-effort, mirroring every other audit write in this package (chat.go,
// runner_setup.go): a failed write is logged at WARN and NEVER rolls the
// mutation back — the registry change is already persisted and swapped, and
// turning a successful deactivation into a 500 because its audit row failed
// would leave the registry and the operator's intent out of sync. The
// threat register accepts this posture (T-26-05-5) for repo-wide
// consistency; a nil Store means no audit backend is configured (degraded
// mode, same as elsewhere).
func (s *Deps) putKBAdminAuditRow(ctx context.Context, action string, entry kb.Entry) {
	if s.Store == nil {
		return
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		log.Printf("[WARN] server: kb audit row marshal (%s slug=%s): %v (row skipped)", action, entry.Slug, err)
		return
	}
	if _, err := s.Store.PutAudit(ctx, store.AuditRow{
		Timestamp:  time.Now().UTC(),
		UserID:     "admin",
		Action:     action,
		ActorType:  "admin",
		FrozenArgs: payload,
	}); err != nil {
		log.Printf("[WARN] server: kb audit row write (%s slug=%s): %v (best-effort, mutation not rolled back)", action, entry.Slug, err)
	}
}
