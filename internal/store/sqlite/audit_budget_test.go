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

// Package sqlite_test — audit_budget_test.go is Phase 12 / Plan 12-05
// Task 1: the BudgetLayer column + ActionBudgetTriggered enum extension.
//
// Tests in this file:
//
//   - TestAuditBudgetTriggeredActionEnum — round-trips
//     action_budget_triggered + BudgetLayer=token through PutAudit +
//     QueryAudit (the column survives the SELECT).
//   - TestAuditBudgetLayerFilterable — three rows with different
//     budget_layers; the column-level scan discriminates them (T-3-SQLI
//     parameterized filter; the column is on the audit row, not a
//     JOINed table).
//   - TestAuditBudgetLayerInvalidRejected — PutAudit with
//     Action=action_budget_triggered + BudgetLayer="invalid" is rejected
//     by the BudgetLayerNames whitelist guard (T-12-05-6 spoofing
//     mitigation).
//   - TestAuditNonBudgetRowsBudgetLayerNull — verdict and HITL-04 rows
//     leave BudgetLayer empty; the audit table stays free of
//     accidental BudgetLayer leaks into non-budget rows.

package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditBudgetTriggeredActionEnum — Plan 12-05 Task 1: the new
// audit.action enum value action_budget_triggered + BudgetLayer=token
// MUST round-trip through PutAudit + QueryAudit. Same posture as
// TestAuditPlanTodoActionEnum for 12-02 — string-typed catalog
// extension, no DDL change to action column, but the NEW BudgetLayer
// column requires the row's BudgetLayer value to satisfy the
// BudgetLayerNames whitelist (T-12-05-6 mitigation).
func TestAuditBudgetTriggeredActionEnum(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// Row 1: action_budget_triggered + budget_layer=token.
	row := store.AuditRow{
		Timestamp:   time.Now().UTC(),
		SessionID:   "sess-budget",
		SkillID:     "compliance",
		Query:       "audit-budget-enum-test",
		PromptHash:  "p-hash",
		ModelVersion: "test",
		VerdictJSON: "{}",
		Action:      sqlite.ActionBudgetTriggered,
		BudgetLayer: "token",
	}
	id, err := s.PutAudit(ctx, row)
	require.NoError(t, err, "action_budget_triggered + budget_layer=token must be accepted by PutAudit")
	require.Greater(t, id, int64(0))

	// The row MUST surface via QueryAudit with both Action + BudgetLayer
	// preserved (the column is queryable, not silently coerced to "").
	rows, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID: "sess-budget",
		Action:    sqlite.ActionBudgetTriggered,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1,
		"action_budget_triggered filter must return exactly 1 row")
	assert.Equal(t, sqlite.ActionBudgetTriggered, rows[0].Action,
		"action enum must round-trip verbatim")
	assert.Equal(t, "token", rows[0].BudgetLayer,
		"budget_layer=token MUST round-trip verbatim (PLAN-04 / Plan 12-05 substrate)")
}

// TestAuditBudgetLayerFilterable — Plan 12-05 Task 1: the audit BudgetLayer
// column filter on QueryAudit MUST discriminate between budget_layer
// values within the action_budget_triggered enum. The audit_writer seam
// in 12-05 emits these rows in lockstep with the budget middleware's
// SSE forced_stop chunk; the 12-07 AuditTimelineTab will join on the
// (Action, BudgetLayer) tuple to render "budget exhausted: <layer>"
// events alongside approval/rejection events.
//
// The filter on BudgetLayer would require extending AuditFilter — for
// this scope, the discrimination is proven by filtering on Action
// (each row is a different layer) and reading the column back.
func TestAuditBudgetLayerFilterable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	base := time.Now().UTC()
	mk := func(layer string) store.AuditRow {
		return store.AuditRow{
			Timestamp:   base,
			SessionID:   "sess-budget-filter",
			SkillID:     "compliance",
			Query:       "audit-budget-filter-test",
			PromptHash:  "p-hash",
			ModelVersion: "test",
			VerdictJSON: "{}",
			Action:      sqlite.ActionBudgetTriggered,
			BudgetLayer: layer,
		}
	}
	_, err = s.PutAudit(ctx, mk("token"))
	require.NoError(t, err)
	_, err = s.PutAudit(ctx, mk("loop"))
	require.NoError(t, err)
	_, err = s.PutAudit(ctx, mk("max_iter"))
	require.NoError(t, err)

	// All three rows surface under the action filter; the layer column
	// is preserved row-by-row so per-layer queries can scan in SQL
	// (WHERE action='action_budget_triggered' AND budget_layer='loop').
	rows, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID: "sess-budget-filter",
		Action:    sqlite.ActionBudgetTriggered,
	})
	require.NoError(t, err)
	require.Len(t, rows, 3,
		"action_budget_triggered filter must return all 3 budget rows")

	got := map[string]bool{}
	for _, r := range rows {
		got[r.BudgetLayer] = true
	}
	assert.True(t, got["token"], "token row must surface with budget_layer preserved")
	assert.True(t, got["loop"], "loop row must surface with budget_layer preserved")
	assert.True(t, got["max_iter"], "max_iter row must surface with budget_layer preserved")
}

// TestAuditBudgetLayerInvalidRejected — Plan 12-05 Task 1 (T-12-05-6
// spoofing guard): PutAudit with Action=action_budget_triggered +
// BudgetLayer="invalid" MUST be rejected by the BudgetLayerNames
// whitelist. The audit table is a durable record of what happened; an
// invalid layer value would let an adversary understate or misrepresent
// the budget event.
//
// The sentinel is sqlite.ErrInvalidBudgetLayer — tests assert
// errors.Is() so future wrapping layers don't silently swallow the
// signal.
func TestAuditBudgetLayerInvalidRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// Empty budget_layer is rejected for action_budget_triggered.
	_, err = s.PutAudit(ctx, store.AuditRow{
		Timestamp:   time.Now().UTC(),
		SessionID:   "sess-spoof",
		SkillID:     "compliance",
		Query:       "audit-spoof-test",
		PromptHash:  "p-hash",
		ModelVersion: "test",
		VerdictJSON: "{}",
		Action:      sqlite.ActionBudgetTriggered,
		BudgetLayer: "",
	})
	require.Error(t, err, "empty BudgetLayer on a budget row MUST be rejected")
	assert.True(t, errors.Is(err, sqlite.ErrInvalidBudgetLayer),
		"empty-budget_layer rejection must wrap ErrInvalidBudgetLayer (T-12-05-6): %v", err)

	// Unknown budget_layer value is rejected.
	_, err = s.PutAudit(ctx, store.AuditRow{
		Timestamp:   time.Now().UTC(),
		SessionID:   "sess-spoof",
		SkillID:     "compliance",
		Query:       "audit-spoof-test",
		PromptHash:  "p-hash",
		ModelVersion: "test",
		VerdictJSON: "{}",
		Action:      sqlite.ActionBudgetTriggered,
		BudgetLayer: "hypothetical",
	})
	require.Error(t, err, "unknown BudgetLayer value MUST be rejected")
	assert.True(t, errors.Is(err, sqlite.ErrInvalidBudgetLayer),
		"unknown-budget_layer rejection must wrap ErrInvalidBudgetLayer: %v", err)

	// Sanity: the rejected rows must NOT appear in QueryAudit.
	rows, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-spoof"})
	require.NoError(t, err)
	assert.Empty(t, rows,
		"rejected budget rows must not have been written to the audit table")
}

// TestAuditNonBudgetRowsBudgetLayerNull — Plan 12-05 Task 1: verdict
// rows and HITL-04 lifecycle rows (action_pending_approval,
// action_approved, action_rejected) MUST leave BudgetLayer empty —
// the column is reserved for action_budget_triggered rows only. A
// stray non-empty BudgetLayer on a non-budget row would confuse the
// 12-07 AuditTimelineTab render (which groups events by layer).
func TestAuditNonBudgetRowsBudgetLayerNull(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// Three rows: one verdict (action=""), one pending_approval,
	// one rejected — none is action_budget_triggered.
	cases := []struct {
		name   string
		action string
	}{
		{"verdict", ""},
		{"pending", sqlite.ActionPendingApproval},
		{"rejected", sqlite.ActionRejected},
	}
	base := time.Now().UTC()
	for _, tc := range cases {
		_, err := s.PutAudit(ctx, store.AuditRow{
			Timestamp:   base,
			SessionID:   "sess-nonbudget",
			SkillID:     "compliance",
			Query:       "audit-nonbudget-test",
			PromptHash:  "p-hash",
			ModelVersion: "test",
			VerdictJSON: "{}",
			Action:      tc.action,
		})
		require.NoError(t, err, "row %q must be accepted", tc.name)
	}

	rows, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-nonbudget"})
	require.NoError(t, err)
	require.Len(t, rows, 3)
	for _, r := range rows {
		assert.Empty(t, r.BudgetLayer,
			"non-budget row (action=%q) MUST have empty BudgetLayer", r.Action)
	}
}