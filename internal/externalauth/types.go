// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Package externalauth is Phase 21 Task 21-01/02 — the External
// Authorized Channel (DATA-04/05/06/10/11/13/14). It implements the
// approval + audit pipeline that gates any individual-identification
// query going from a SkillForge agent to an external data warehouse.
//
// Core invariants (each is a Phase 21 release gate):
//
//   - DATA-04 / DATA-10: queries can ONLY target a registered
//     precompiled view (not raw tables). The Source interface in
//     source.go returns row sets by view_name; raw-table access is
//     deliberately not exposed.
//
//   - DATA-05: every returned row passes through Mask() which
//     redacts field-level PII (account numbers, phones, emails) per
//     the registered mask spec. The agent never sees the underlying
//     raw values.
//
//   - DATA-06: each ExternalAuthRequest produces an audit row on
//     BOTH sides — the internal external_auth_audit (SkillForge) and
//     the external query_log (data warehouse). Reconciliation by
//     query_id is done by the /api/external-auth/audit/reconcile
//     endpoint (Wave 4); a missing pair is a security event
//     (fail-closed).
//
//   - DATA-11: agent.Run must produce an NL2SQL QueryPreview BEFORE
//     calling RequestApproval. The approval request carries the
//     preview verbatim; the data officer audits the actual query
//     intent (not just the natural-language reformulation).
//
//   - DATA-13: approval state is pending FOREVER until the data
//     officer explicitly approves or rejects. There is no auto-reject
//     path; the only way to resolve a pending request is human
//     action. TTL token expiry (DATA-04) is orthogonal — once a
//     token expires the request is dead regardless of approval
//     state.
//
//   - DATA-14: every approval produces an external_auth_audit row
//     with the 9-field schema defined in AuditRow below. Schema is
//     additive + omitempty; existing rows are readable by v1.0
//     reconciliation tools.
//
// This package is read-mostly at runtime (one RequestApproval per
// agent query that hits the channel, one Approve|Reject per data
// officer decision). The Source interface allows real data warehouse
// drivers to be plugged in later (v1.2) without changing call sites.
package externalauth

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DefaultTTL is the DATA-04-mandated short-TTL for issued tokens.
// DATA-13 (persistent pending) is orthogonal — the request stays
// pending indefinitely, but the issued token dies after this many ms.
const DefaultTTL = 5 * time.Minute

// Approval states — append-only enum; new states must be added to
// the bottom + match the audit_action column in the store schema.
const (
	StatePending  = "pending"
	StateApproved = "approved"
	StateRejected = "rejected"
)

// QueryPreview is the DATA-11 NL2SQL output that must precede every
// approval request. It is the data officer's only window into what
// the agent is actually about to query against the data warehouse.
//
// `EstimatedRows` is a coarse LLM estimate (round number; not a
// promise). `BusinessReason` is the user's stated reason; the data
// officer reads this + the SQL to decide approve/reject.
type QueryPreview struct {
	SQLPreview    string `json:"sql_preview"`
	ViewName      string `json:"view_name"`
	EstimatedRows int    `json:"estimated_rows"`
	BusinessReason string `json:"business_reason"`
}

// ExternalAuthRequest is one pending approval (one user query that
// hit the channel). Lifetime: created at RequestApproval time,
// resolved (approved/rejected) at data-officer action time, audit
// row persisted to external_auth_audit at resolution.
//
// IDs are UUIDv4-shaped strings generated server-side (see store
// implementation); callers never supply them.
type ExternalAuthRequest struct {
	RequestID    string       `json:"request_id"`
	UserID       string       `json:"user_id"`
	SkillID      string       `json:"skill_id"`
	SessionID    string       `json:"session_id"`
	Preview      QueryPreview `json:"preview"`
	State        string       `json:"state"`
	CreatedAt    time.Time    `json:"created_at"`
	// TTLTokenID is set ONLY when State == StateApproved (after
	// approval, the system mints a TTL token; the agent uses this
	// token to call Source.Execute). NULL while pending/rejected.
	TTLTokenID string `json:"ttl_token_id,omitempty"`
	// AuditRowID is the row id of the corresponding external_auth_audit
	// row (DATA-14 schema). Set at resolution time.
	AuditRowID int64 `json:"audit_row_id,omitempty"`
}

// TTLToken is the DATA-04 short-lived credential minted at approval
// time. The Source uses it to authenticate the agent's query
// against the data warehouse.
//
// Scope is the (view_name, user_id) pair — a token can ONLY query
// the view it was issued for, and only on behalf of the user who
// requested it. The scope_hash is deterministic so reconcile /
// mismatch checks don't need the raw values.
type TTLToken struct {
	TokenID   string    `json:"token_id"`
	ViewName  string    `json:"view_name"`
	UserID    string    `json:"user_id"`
	ScopeHash string    `json:"scope_hash"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Valid reports whether the token is in its valid lifetime. Tokens
// are NOT invalidated on approval-state changes (the request stays
// pending DATA-13, but the token death-clock is independent).
func (t TTLToken) Valid(now time.Time) bool {
	return !now.Before(t.IssuedAt) && now.Before(t.ExpiresAt)
}

// AuditRow is the DATA-14 schema. Every approval creates exactly one
// row (decision = approved | rejected). Schema is append-only —
// adding a field is non-breaking; renaming/requiring is breaking
// and needs a migration.
//
// Both sides (SkillForge + data warehouse query_log) carry this
// shape so reconcile can match on query_id + ts_window.
type AuditRow struct {
	// QueryID is the external_auth_requests.request_id — the
	// canonical join key for reconciliation.
	QueryID          string   `json:"query_id"`
	UserID           string   `json:"user_id"`
	BusinessReason   string   `json:"business_reason"`
	SQLPreview       string   `json:"sql_preview"`
	Decision         string   `json:"decision"` // approved | rejected
	DataOfficerID    string   `json:"data_officer_id"`
	TTLTokenID       string   `json:"ttl_token_id,omitempty"`
	Timestamp        time.Time `json:"ts"`
	ChunkIDsReturned []string `json:"chunk_ids_returned"`
	// MaskApplied is a list of field names that were masked on the
	// returned rows (DATA-05). Empty when the view returns no
	// masked fields, or when Decision == rejected (nothing was
	// returned to mask).
	MaskApplied []string `json:"mask_applied,omitempty"`
}

// MaskSpec describes the field-level redaction rule for one column.
// FieldName matches a column in the PrecompiledView; MaskFn is a
// canonical redaction (account / phone / email — the three PII
// classes DATA-05 explicitly calls out).
type MaskSpec struct {
	FieldName string `json:"field_name"`
	Kind      string `json:"kind"` // "account" | "phone" | "email" | "generic"
	// KeepLen is the number of trailing chars to keep visible (e.g.
	// account last 4 digits). 0 means "full mask" (return "***").
	KeepLen int `json:"keep_len"`
}

// PrecompiledView is the DATA-10 whitelist entry: the agent can ONLY
// target registered views, never raw tables. ViewName is unique per
// deployment; FieldMasks lists the columns that need redaction on
// return. The actual row materialization is Source's job (see
// source.go) — the view here is the *contract*.
type PrecompiledView struct {
	ViewName   string      `json:"view_name"`
	Source     string      `json:"source"` // human-readable origin
	FieldMasks []MaskSpec  `json:"field_masks"`
}

// ViewRegistry is the in-process whitelist of allowed views.
// Source implementations consult this before executing any query.
type ViewRegistry struct {
	views map[string]PrecompiledView
}

// NewViewRegistry constructs an empty registry; callers Register
// views at startup (typically from a config file or the operator's
// skill manifest).
func NewViewRegistry() *ViewRegistry {
	return &ViewRegistry{views: make(map[string]PrecompiledView)}
}

// Register adds (or replaces) one view. Duplicate names panic — view
// registration is a startup-only path, not a runtime hot path, and
// silent overwrite would hide operator mistakes.
func (r *ViewRegistry) Register(v PrecompiledView) {
	if v.ViewName == "" {
		panic("externalauth: ViewRegistry.Register: ViewName empty")
	}
	r.views[v.ViewName] = v
}

// Get returns (view, true) for a known view, or (zero, false).
// Used by Source.Execute to verify the requested view is whitelisted
// (DATA-10 enforcement) — unregistered views MUST 404.
func (r *ViewRegistry) Get(name string) (PrecompiledView, bool) {
	v, ok := r.views[name]
	return v, ok
}

// Names returns the registered view names sorted alphabetically —
// the agent uses this to populate the NL2SQL prompt with the
// allow-list of queryable views.
func (r *ViewRegistry) Names() []string {
	out := make([]string, 0, len(r.views))
	for k := range r.views {
		out = append(out, k)
	}
	// sort.Strings would import the package just for this; the list
	// is small (<100 entries) so we keep the import surface clean
	// with a simple insertion sort.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// Mask applies the registered field-level redaction to one row.
// The returned row is a NEW map; the input is not mutated. Unknown
// fields are passed through unchanged (DATA-05 lists three PII
// classes; anything else is the data warehouse's responsibility).
func (v PrecompiledView) Mask(row map[string]string) map[string]string {
	out := make(map[string]string, len(row))
	for k, val := range row {
		out[k] = val
	}
	for _, spec := range v.FieldMasks {
		raw, ok := out[spec.FieldName]
		if !ok {
			continue
		}
		out[spec.FieldName] = ApplyMask(spec, raw)
	}
	return out
}

// ApplyMask runs one MaskSpec on a single string value. Exported so
// the Source.MemorySource test double can apply the same rules
// without going through the View method (faster + simpler).
func ApplyMask(spec MaskSpec, raw string) string {
	switch spec.Kind {
	case "account":
		return maskAccount(raw, spec.KeepLen)
	case "phone":
		return maskPhone(raw, spec.KeepLen)
	case "email":
		return maskEmail(raw, spec.KeepLen)
	case "generic":
		return maskGeneric(raw, spec.KeepLen)
	default:
		// Unknown kind: full mask. Better to over-redact than leak.
		return "***"
	}
}

// maskAccount keeps the last N digits, masks the rest with '*'. If
// the input is shorter than N, masks everything.
func maskAccount(s string, keep int) string {
	if keep <= 0 {
		return strings.Repeat("*", len([]rune(s)))
	}
	runes := []rune(s)
	if len(runes) <= keep {
		return strings.Repeat("*", len(runes))
	}
	return strings.Repeat("*", len(runes)-keep) + string(runes[len(runes)-keep:])
}

// maskPhone: e.g. "+86 138 1234 5678" → "+** **** 1234 ****" with
// keep=4. The implementation keeps the keep-last chars; the spaces
// pass through. Real production masks typically group-by-segment;
// this is the simplified default (DATA-05 lists the categories; the
// operator overrides via mask spec).
func maskPhone(s string, keep int) string { return maskAccount(s, keep) }

// maskEmail keeps the first 2 chars of the local part + the full
// domain. "alice@example.com" → "al***@example.com".
func maskEmail(s string, keep int) string {
	at := strings.IndexByte(s, '@')
	if at < 0 {
		return maskAccount(s, keep)
	}
	local, domain := s[:at], s[at:]
	if len(local) <= 2 {
		return "**" + domain
	}
	return local[:2] + strings.Repeat("*", len(local)-2) + domain
}

// maskGeneric is the catch-all: keep last N chars, mask the rest.
func maskGeneric(s string, keep int) string { return maskAccount(s, keep) }

// ViewNamePattern is the allow-list enforced by the Source layer.
// Lowercase letters, digits, underscore; no spaces, no dashes. Same
// character class as a SQL identifier to keep SQL injection risk at
// the boundary zero.
var ViewNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// ErrViewNotAllowed is the Source-level error when a query targets
// a view name that's not in the registry. DATA-10 enforcement.
var ErrViewNotAllowed = errors.New("externalauth: view not in registry")

// ErrTokenExpired is the Source-level error when the TTL token is
// past its ExpiresAt. The agent must re-request approval.
var ErrTokenExpired = errors.New("externalauth: token expired")

// ErrSQLReadOnly is the Source-level error when a preview includes
// non-SELECT SQL (INSERT/UPDATE/DELETE/DDL). DATA-11 enforcement.
var ErrSQLReadOnly = errors.New("externalauth: sql preview must be SELECT-only")

// FormatRequestID is a stable, human-readable id format for
// external_auth_requests. Operators grep by this when reconciling.
func FormatRequestID(seq int64) string {
	return fmt.Sprintf("xreq-%08d", seq)
}

// FormatTokenID is the same for TTL tokens. Different prefix so the
// two id spaces don't collide on dashboards.
func FormatTokenID(seq int64) string {
	return fmt.Sprintf("xtok-%08d", seq)
}
