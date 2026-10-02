// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package externalauth

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Source is the data warehouse side of the External Authorized
// Channel. Real implementations (PostgreSQL / MySQL / Oracle /
// Hive) would implement this interface; the v1.1 default is
// MemorySource (in-process mock) used for tests + local UAT.
//
// DATA-10 enforcement: every Execute call must verify the view is
// in the registry (Source.Get) before any query runs. Unregistered
// views MUST return ErrViewNotAllowed — never silently fall through
// to a default/empty result.
//
// DATA-04 enforcement: tokens are checked for Valid() at every
// Execute. An expired token MUST return ErrTokenExpired — never
// silently downgrade to read-only-without-audit.
type Source interface {
	// Execute runs the (already previewed, already approved) query
	// against the underlying data warehouse. The TTL token binds
	// the query to (view_name, user_id). Returned rows are masked
	// per the view's FieldMasks spec; the agent never sees raw
	// values. `now` is the caller-injected clock (production:
	// time.Now(); tests: a frozen value); the Source MUST use it
	// for TTL expiry checks so the Manager's SetClock override
	// flows through deterministically.
	Execute(ctx context.Context, token TTLToken, viewName string, now time.Time) ([]map[string]string, error)

	// AppendExternalAudit is the DATA-06 dual-audit hook. The data
	// warehouse's own query_log is the EXTERNAL side; we surface
	// its rows here so the reconcile endpoint (Wave 4) can pair
	// them with our internal external_auth_audit rows.
	AppendExternalAudit(ctx context.Context, row AuditRow) error
	ListExternalAudit(ctx context.Context, fromUnixMs, toUnixMs int64) ([]AuditRow, error)
}

// MemorySource is the v1.1 reference implementation of Source.
// It holds a per-view table of rows in-process; queries return
// all rows for the view (no real SQL parser — this is UAT +
// golden-set scaffolding only). Real v1.2 drivers (Postgres /
// MySQL) plug into the Source interface unchanged.
type MemorySource struct {
	mu      sync.RWMutex
	registry *ViewRegistry
	rows    map[string][]map[string]string // view_name -> rows
	extLog  []AuditRow                     // DATA-06 dual audit
}

// NewMemorySource constructs an empty source bound to a registry.
// Tables are populated via UpsertView (used by tests and the
// data warehouse bootstrap path).
func NewMemorySource(registry *ViewRegistry) *MemorySource {
	return &MemorySource{
		registry: registry,
		rows:     make(map[string][]map[string]string),
	}
}

// UpsertView seeds the in-memory table for one view. The view MUST
// already be registered (caller registers the contract first via
// registry.Register, then seeds rows). A view name that fails
// ViewNamePattern is rejected — DATA-10 surface area stays small.
func (m *MemorySource) UpsertView(viewName string, rows []map[string]string) error {
	if !ViewNamePattern.MatchString(viewName) {
		return fmt.Errorf("MemorySource: invalid view name %q", viewName)
	}
	if _, ok := m.registry.Get(viewName); !ok {
		return fmt.Errorf("MemorySource: view %q not in registry (DATA-10)", viewName)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// Defensive copy — caller can mutate their slice after UpsertView.
	stored := make([]map[string]string, len(rows))
	for i, r := range rows {
		stored[i] = copyRow(r)
	}
	m.rows[viewName] = stored
	return nil
}

// Execute implements Source. Returns the rows for viewName after
// applying the view's mask spec; errors per the Source contract.
func (m *MemorySource) Execute(_ context.Context, token TTLToken, viewName string, now time.Time) ([]map[string]string, error) {
	if _, ok := m.registry.Get(viewName); !ok {
		return nil, ErrViewNotAllowed
	}
	if !token.Valid(now) {
		return nil, ErrTokenExpired
	}
	m.mu.RLock()
	src, ok := m.rows[viewName]
	m.mu.RUnlock()
	if !ok {
		// View registered but unseeded → empty result, not an error.
		return []map[string]string{}, nil
	}
	view, _ := m.registry.Get(viewName)
	out := make([]map[string]string, len(src))
	for i, r := range src {
		out[i] = view.Mask(r)
	}
	return out, nil
}

// AppendExternalAudit implements Source.
func (m *MemorySource) AppendExternalAudit(_ context.Context, row AuditRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.extLog = append(m.extLog, row)
	return nil
}

// ListExternalAudit implements Source.
func (m *MemorySource) ListExternalAudit(_ context.Context, fromUnixMs, toUnixMs int64) ([]AuditRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AuditRow, 0, len(m.extLog))
	for _, r := range m.extLog {
		ts := r.Timestamp.UnixMilli()
		if ts < fromUnixMs || ts >= toUnixMs {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// HasQueryID is a test helper — confirms the external query_log
// contains the matching pair for an internal audit row (DATA-06
// reconcile).
func (m *MemorySource) HasQueryID(queryID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.extLog {
		if r.QueryID == queryID {
			return true
		}
	}
	return false
}

// Copy a row map defensively so caller mutation can't poison the
// in-memory table.
func copyRow(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// timeNow is the clock used by TTL token Valid() checks.
// Indirected through a package-level variable so tests can swap it
// without pulling time.Now into every Source call. Production
// default is time.Now; tests in types_test.go use a frozen clock.
var timeNow = func() time.Time { return realTimeNow() }

// realTimeNow is the default clock implementation. Defined as a
// variable so tests can override without touching the production
// path.
var realTimeNow = func() time.Time { return time.Now() }

// RejectNonSelect is the DATA-11 enforcement: the preview's
// SQL must be SELECT-only. Comment-prefixes and whitespace are
// stripped before the keyword check so ` -- select * from ...`
// still passes. INSERT/UPDATE/DELETE/MERGE/REPLACE/CREATE/DROP/
// ALTER/TRUNCATE/GRANT/REVOKE are all rejected; CALL is rejected
// (no procedure calls through the channel — those go through a
// different approval flow). SELECT is the only allowed leading
// verb.
//
// The check is deliberately conservative — true SQL grammar
// parsing is not the goal here. A real data warehouse driver
// would re-validate with its own parser; this is the channel
// boundary gate.
func RejectNonSelect(sql string) error {
	trimmed := strings.TrimSpace(sql)
	// Strip line comments.
	for {
		i := strings.Index(trimmed, "--")
		if i < 0 {
			break
		}
		end := strings.Index(trimmed[i:], "\n")
		if end < 0 {
			trimmed = trimmed[:i]
			break
		}
		trimmed = trimmed[:i] + trimmed[i+end:]
	}
	trimmed = strings.TrimSpace(trimmed)
	upper := strings.ToUpper(trimmed)
	// Find the first keyword.
	first := upper
	if sp := strings.IndexAny(upper, " \t\n("); sp >= 0 {
		first = upper[:sp]
	}
	switch first {
	case "SELECT", "WITH":
		return nil
	default:
		return fmt.Errorf("%w: leading verb %q", ErrSQLReadOnly, first)
	}
}
