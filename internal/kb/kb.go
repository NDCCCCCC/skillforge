// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Package kb is the knowledge-base registry contract layer: the single
// source of truth for knowledge-base identity (slug set, semantic roles,
// deactivation switches) that every other package consumes through the
// Registry interface instead of a compiled-in enumeration (KBM-01), plus
// the code-level home of the citation whitelist (KBM-03).
//
// The package is a leaf: the persistence seam (Store) and the read
// contract (Registry) are declared here and assembled elsewhere (the
// sqlite adapter in 26-02, the boot wiring in 26-09), and the skill,
// knowledge, server and data packages depend on the interface only. No
// CN display
// name lives in this package — display names are data, and their single
// home is the controlled seed file knowledge_bases.yaml.
package kb

import (
	"context"
	"errors"
	"time"
)

// Role is the closed four-value semantic role vocabulary (KBM-03: exactly
// one role per knowledge base). Roles are platform semantics, not
// operator configuration: these four identifiers are the only knowledge
// tier vocabulary allowed to be hard-coded anywhere in the repo, and the
// phase 26 structural gates whitelist them here and only here. Everything
// else about a knowledge base — how many exist, their slugs, display
// names, descriptions, retrieval switches — is registry data.
type Role string

const (
	// RoleAuthority marks the citable evidence role: the citation
	// whitelist accepts chunks from authority-role KBs and nothing else
	// (KBM-03; the single source is CitableRole below).
	RoleAuthority Role = "authority"

	// RoleHistorical marks superseded revisions: kept for the record,
	// reachable only through an explicit reads declaration, never citable.
	RoleHistorical Role = "historical"

	// RolePersonal marks user-private material: excluded from retrieval
	// unless a skill explicitly declares it, never citable.
	RolePersonal Role = "personal"

	// RoleDraft marks not-yet-effective material: never retrievable
	// regardless of the entry's retrieval switch (role semantics outrank
	// the switch), never citable.
	RoleDraft Role = "draft"
)

// Roles returns the closed role vocabulary in its fixed deterministic
// order. The order is part of the contract: validation errors and
// documentation list roles in this order, so callers must not sort,
// filter or extend the returned slice.
func Roles() []Role {
	return []Role{RoleAuthority, RoleHistorical, RolePersonal, RoleDraft}
}

// IsRole reports whether s is a member of the closed role vocabulary.
// The comparison is exact and case-sensitive: an unknown role is a typo
// today and a trust-boundary hole tomorrow, so anything that is not an
// exact lowercase member is refused rather than defaulted.
func IsRole(s string) bool {
	switch Role(s) {
	case RoleAuthority, RoleHistorical, RolePersonal, RoleDraft:
		return true
	default:
		return false
	}
}

// CitableRole is the code-level single source of the citation whitelist
// (KBM-03): only knowledge bases whose role equals CitableRole may
// contribute citation evidence. Consumers (the trust floor, the verdict
// pipeline) must compare against this constant and never against a
// literal role name, so tightening or loosening citability stays a
// one-line change in one home.
const CitableRole = RoleAuthority

// MaxSlugLen is the single home of the knowledge-base slug length
// ceiling. A slug becomes a knowledge/<slug>/ directory path segment and
// a retriever filter value, and the milvus tier column width is projected
// from this constant, so every consumer derives from here — a second
// copy of the number would be a new hardcode.
const MaxSlugLen = 20

// Entry is one registered knowledge base. Entries are value types: the
// registry hands out copies, so mutating a returned Entry never changes
// registry state.
type Entry struct {
	// Slug is the stable identifier. It doubles as the knowledge/<slug>/
	// directory name and the retriever filter value, so the slug
	// whitelist (ValidateSlug) is the only gate a slug crosses before it
	// reaches a filesystem path or a filter expression.
	Slug string

	// DisplayName is the UI badge text. Its single home is the controlled
	// seed file; refusal-suggestion copy and frozen eval assertions
	// depend on these strings byte-for-byte.
	DisplayName string

	// Role is the semantic role from the closed vocabulary (see Role).
	Role Role

	// Description is the operator-facing explanation of what the base
	// holds.
	Description string

	// RetrievalEnabled is the deactivation switch: false hides the base
	// from Lookup and from every retrieval-eligible set without deleting
	// any data (KBM-02: deactivate, never destroy). Role semantics still
	// outrank this switch — a draft-role entry is never retrievable even
	// when this is true.
	RetrievalEnabled bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Registry is the single source of truth for knowledge-base identity: the
// slug set, the semantic roles and the deactivation switches are all
// queryable here and nowhere else (KBM-01). Consumers declare the narrow
// slice of this interface they need; nothing outside this package may
// reach into registry internals.
//
// Trust-model note — deliberately different from the KPI registry. The
// KPI registry (D16-A4/D16-08-E) is immutable by construction because any
// HTTP surface that could redefine a KPI would be a registration backdoor
// around the approval channel. Knowledge bases are operational content,
// not load-bearing SQL definitions, and KBM-02 explicitly requires a
// runtime create/deactivate surface — so this interface carries Create and
// Deactivate. Every mutation still lands in an INSERT-only audit row
// written by the CALLER (the kb package never writes audit rows itself),
// and no delete verb exists: deactivation hides a base, it never destroys
// data.
//
// A Registry opened without a Store (see New) is read-only: Create and
// Deactivate fail closed with ErrReadOnly instead of mutating memory
// only, because an unpersisted mutation would silently vanish on restart.
type Registry interface {
	// Lookup returns the active entry for slug. A deactivated or
	// unregistered slug is indistinguishable from absent (fail-closed):
	// callers must treat ok=false as "no such knowledge base".
	Lookup(slug string) (Entry, bool)

	// List returns every entry including deactivated ones (the admin
	// view), sorted by slug ascending. The slice is a defensive copy.
	List() []Entry

	// ActiveSlugs returns the slugs of all active entries sorted
	// ascending. This is the skill-validation surface: a deactivated base
	// disappears from it, so skills referencing it stop loading.
	ActiveSlugs() []string

	// SlugsByRole returns the slugs of active entries with role r, sorted
	// ascending. An unknown role yields an empty slice. Note that a
	// draft-role entry is active but never retrieval-eligible — this
	// method answers "what exists", not "what is searchable".
	SlugsByRole(r Role) []string

	// RetrievalEligible returns the slugs that may enter a retrieval
	// filter set: active, retrieval-enabled, and not draft-role. The
	// draft exclusion is unconditional — role semantics outrank the
	// entry switch, so a seed slip can never make draft material
	// searchable (A3).
	RetrievalEligible() []string

	// Signature returns the deterministic content hash of the entry set.
	// Audit rows carry it so "which registry version produced this
	// answer" stays replayable.
	Signature() string

	// ResolveReads expands a skill's reads declarations into the set of
	// slugs the retriever may query (KBM-04). Unknown or deactivated
	// slugs fail closed; the draft role is dropped from the resolved set
	// rather than granted.
	ResolveReads(reads []string) ([]string, error)

	// Create registers a new knowledge base. It fails closed on an
	// invalid slug, an unknown role, an empty display name, or a slug
	// that already exists (including a deactivated one), and with
	// ErrReadOnly when no Store is wired.
	Create(in CreateInput) (Entry, error)

	// Deactivate flips the retrieval switch of slug to false (KBM-02):
	// the entry stays listed for admins, vanishes from Lookup and from
	// every retrieval-eligible set, and its data is untouched. Fails
	// closed on an unknown slug and with ErrReadOnly when no Store is
	// wired.
	Deactivate(slug string) (Entry, error)
}

// Store is the registry persistence seam: 26-02 adapts it over the sqlite
// knowledge_bases table. This package does not depend on the store layer —
// the production adapter is assembled at boot, mirroring the narrow
// dataset-lister discipline in the KPI package. The kb package itself never
// writes audit rows; callers persist the INSERT-only audit trail for each
// mutation they drive through this seam.
type Store interface {
	// LoadBases returns every persisted entry, active and deactivated.
	LoadBases(ctx context.Context) ([]Entry, error)

	// UpsertBase writes the declared fields of one entry.
	UpsertBase(ctx context.Context, e Entry) error

	// SetRetrievalEnabled flips only the deactivation switch.
	SetRetrievalEnabled(ctx context.Context, slug string, enabled bool, updatedAt time.Time) error
}

// CreateInput is the admin-facing payload of Registry.Create.
type CreateInput struct {
	Slug        string
	DisplayName string
	Role        Role
	Description string
}

// ErrReadOnly is returned by Create and Deactivate on a registry opened
// without a Store: a mutation that could not be persisted must not happen
// at all, because it would silently vanish on the next restart.
var ErrReadOnly = errors.New("kb: registry opened read-only (no store wired)")
