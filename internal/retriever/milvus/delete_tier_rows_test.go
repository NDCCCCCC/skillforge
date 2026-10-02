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

package milvus

// Hermetic tests for DeleteTierRows — the tier-scoped purge behind the
// boot-time reseed (sql-guides-duplicate-content, 2026-09-15). The purge
// is destructive by design (that is its job), so the properties under test
// mirror the recreate gate's discipline: it targets exactly the requested
// collection and tiers, degrades to a no-op when there is nothing to
// purge, and fails loudly instead of silently doing nothing.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

// purgeMock records the DML sequence. Unmocked methods of the embedded nil
// interface panic — the hermetic-mock discipline from recreateMockClient.
type purgeMock struct {
	client.Client
	calls  []string
	exists bool
}

func (m *purgeMock) HasCollection(_ context.Context, collName string) (bool, error) {
	m.calls = append(m.calls, "has:"+collName)
	return m.exists, nil
}

func (m *purgeMock) LoadCollection(_ context.Context, collName string, _ bool, _ ...client.LoadCollectionOption) error {
	m.calls = append(m.calls, "load:"+collName)
	return nil
}

func (m *purgeMock) Delete(_ context.Context, collName string, _ string, expr string) error {
	m.calls = append(m.calls, "delete:"+collName)
	m.calls = append(m.calls, "expr:"+expr)
	return nil
}

func (m *purgeMock) Flush(_ context.Context, collName string, _ bool, _ ...client.FlushOption) error {
	m.calls = append(m.calls, "flush:"+collName)
	return nil
}

func TestDeleteTierRowsTargetsCollectionAndTiers(t *testing.T) {
	mc := &purgeMock{exists: true}

	err := DeleteTierRows(context.Background(), mc, Config{CollectionName: DefaultCollectionName}, []string{"authority", "sql-guides"})
	if err != nil {
		t.Fatalf("DeleteTierRows: %v", err)
	}

	has := false
	loaded := false
	deleted := false
	flushed := false
	var expr string
	for _, c := range mc.calls {
		switch {
		case strings.HasPrefix(c, "has:"):
			has = true
		case strings.HasPrefix(c, "load:"):
			if !has {
				t.Fatalf("LoadCollection ran before HasCollection, calls = %v", mc.calls)
			}
			loaded = true
		case c == "delete:"+DefaultCollectionName:
			if !loaded {
				t.Fatalf("Delete ran before LoadCollection (DML needs a loaded collection), calls = %v", mc.calls)
			}
			deleted = true
		case strings.HasPrefix(c, "expr:"):
			expr = strings.TrimPrefix(c, "expr:")
		case c == "flush:"+DefaultCollectionName:
			if !deleted {
				t.Fatalf("Flush ran before Delete, calls = %v", mc.calls)
			}
			flushed = true
		}
	}
	if !deleted || !flushed {
		t.Fatalf("expected delete+flush against %s, calls = %v", DefaultCollectionName, mc.calls)
	}
	if expr != `tier in ["authority","sql-guides"]` {
		t.Fatalf("expr = %s, want the quoted ascending tier list", expr)
	}
	for _, c := range mc.calls {
		if strings.HasPrefix(c, "load:") && !strings.HasSuffix(c, DefaultCollectionName) {
			t.Fatalf("load targeted %q, want %q", c, DefaultCollectionName)
		}
	}
}

// TestDeleteTierRowsMissingCollectionNoOp: a fresh deploy has no collection
// and therefore nothing to purge — the no-op keeps the paired
// SeedDocuments' EnsureCollection as the only creator (no
// delete-on-nonexistent noise), and no other call may run.
func TestDeleteTierRowsMissingCollectionNoOp(t *testing.T) {
	mc := &purgeMock{exists: false}

	if err := DeleteTierRows(context.Background(), mc, Config{CollectionName: DefaultCollectionName}, []string{"authority"}); err != nil {
		t.Fatalf("DeleteTierRows on missing collection: %v", err)
	}
	for _, c := range mc.calls {
		if !strings.HasPrefix(c, "has:") {
			t.Fatalf("missing collection must be a has-only no-op, calls = %v", mc.calls)
		}
	}
}

// TestDeleteTierRowsEmptyTiersNoOp: an empty scope (the derivation of an
// empty batch) must not touch the backend at all — the purge can never
// fire without a matching reseed behind it.
func TestDeleteTierRowsEmptyTiersNoOp(t *testing.T) {
	mc := &purgeMock{exists: true}

	if err := DeleteTierRows(context.Background(), mc, Config{CollectionName: DefaultCollectionName}, nil); err != nil {
		t.Fatalf("DeleteTierRows(nil tiers): %v", err)
	}
	if len(mc.calls) != 0 {
		t.Fatalf("empty tier scope must be a zero-call no-op, calls = %v", mc.calls)
	}
}

// TestDeleteTierRowsNilClientErrors: the nil-client guard must name the
// purge instead of panicking in the SDK, so the boot log stays actionable.
func TestDeleteTierRowsNilClientErrors(t *testing.T) {
	err := DeleteTierRows(context.Background(), nil, Config{CollectionName: DefaultCollectionName}, []string{"authority"})
	if err == nil {
		t.Fatal("expected an error for a nil client")
	}
	if !strings.Contains(err.Error(), "DeleteTierRows") {
		t.Fatalf("error must name DeleteTierRows, got: %v", err)
	}
}

// TestDeleteTierRowsPropagatesDeleteError: a failed Delete surfaces wrapped
// (with the collection + expr named) so seedDocs aborts before the insert —
// a half-replaced tier is worse than an empty one.
func TestDeleteTierRowsPropagatesDeleteError(t *testing.T) {
	mc := &failingPurgeMock{purgeMock: purgeMock{exists: true}}

	err := DeleteTierRows(context.Background(), mc, Config{CollectionName: DefaultCollectionName}, []string{"authority"})
	if err == nil {
		t.Fatal("expected the delete error to propagate")
	}
	if !errors.Is(err, errDeleteBoom) {
		t.Fatalf("error chain must carry the SDK error, got: %v", err)
	}
	if !strings.Contains(err.Error(), DefaultCollectionName) {
		t.Fatalf("error must name the collection, got: %v", err)
	}
}

var errDeleteBoom = errors.New("delete boom")

type failingPurgeMock struct {
	purgeMock
}

func (m *failingPurgeMock) Delete(_ context.Context, collName string, _ string, _ string) error {
	m.calls = append(m.calls, "delete:"+collName)
	return errDeleteBoom
}

// Compile-time guard: the purge tests keep working against the Client
// interface they mock.
var _ client.Client = (*purgeMock)(nil)

// TestDeleteByExprPassesExprVerbatim: the Plan 05/06 usage contract at the
// helper level — the caller-built expression reaches mc.Delete VERBATIM
// (the helper never interpolates, never rewrites), and the mechanics
// mirror DeleteTierRows: HasCollection → LoadCollection → Delete → Flush.
// The expr used here is the ECL ownership-marker purge exactly as the ECL
// milvus-load stage will issue it (RESEARCH Pattern 3).
func TestDeleteByExprPassesExprVerbatim(t *testing.T) {
	mc := &purgeMock{exists: true}
	expr := `tier in ["sql-guides"] and section_count > 0`

	err := DeleteByExpr(context.Background(), mc, Config{CollectionName: DefaultCollectionName}, expr)
	if err != nil {
		t.Fatalf("DeleteByExpr: %v", err)
	}

	has := false
	loaded := false
	deleted := false
	flushed := false
	var gotExpr string
	for _, c := range mc.calls {
		switch {
		case strings.HasPrefix(c, "has:"):
			has = true
		case strings.HasPrefix(c, "load:"):
			if !has {
				t.Fatalf("LoadCollection ran before HasCollection, calls = %v", mc.calls)
			}
			loaded = true
		case c == "delete:"+DefaultCollectionName:
			if !loaded {
				t.Fatalf("Delete ran before LoadCollection (DML needs a loaded collection), calls = %v", mc.calls)
			}
			deleted = true
		case strings.HasPrefix(c, "expr:"):
			gotExpr = strings.TrimPrefix(c, "expr:")
		case c == "flush:"+DefaultCollectionName:
			if !deleted {
				t.Fatalf("Flush ran before Delete, calls = %v", mc.calls)
			}
			flushed = true
		}
	}
	if !deleted || !flushed {
		t.Fatalf("expected delete+flush against %s, calls = %v", DefaultCollectionName, mc.calls)
	}
	if gotExpr != expr {
		t.Fatalf("expr reached Delete as %q, want the caller-built string VERBATIM %q (the helper must never rewrite it)", gotExpr, expr)
	}
}

// TestDeleteByExprMissingCollectionNoOp: a fresh deploy has no collection
// and therefore nothing to purge — the has-only no-op keeps the paired
// SeedDocuments' EnsureCollection as the only creator.
func TestDeleteByExprMissingCollectionNoOp(t *testing.T) {
	mc := &purgeMock{exists: false}

	if err := DeleteByExpr(context.Background(), mc, Config{CollectionName: DefaultCollectionName}, "section_count > 0"); err != nil {
		t.Fatalf("DeleteByExpr on missing collection: %v", err)
	}
	for _, c := range mc.calls {
		if !strings.HasPrefix(c, "has:") {
			t.Fatalf("missing collection must be a has-only no-op, calls = %v", mc.calls)
		}
	}
}

// TestDeleteByExprEmptyExprRejected: the caller owns the expression; an
// empty one must be rejected before any backend call — Milvus would
// reject it server-side, but the error must name the helper duty instead
// of surfacing as an SDK failure.
func TestDeleteByExprEmptyExprRejected(t *testing.T) {
	mc := &purgeMock{exists: true}

	err := DeleteByExpr(context.Background(), mc, Config{CollectionName: DefaultCollectionName}, "  ")
	if err == nil {
		t.Fatal("expected an error for an empty expr")
	}
	if !strings.Contains(err.Error(), "DeleteByExpr") {
		t.Fatalf("error must name DeleteByExpr, got: %v", err)
	}
	if len(mc.calls) != 0 {
		t.Fatalf("no call may run before the expr check, calls = %v", mc.calls)
	}
}

// TestDeleteByExprNilClientErrors: the nil-client guard must name the
// helper instead of panicking in the SDK.
func TestDeleteByExprNilClientErrors(t *testing.T) {
	err := DeleteByExpr(context.Background(), nil, Config{CollectionName: DefaultCollectionName}, "section_count > 0")
	if err == nil {
		t.Fatal("expected an error for a nil client")
	}
	if !strings.Contains(err.Error(), "DeleteByExpr") {
		t.Fatalf("error must name DeleteByExpr, got: %v", err)
	}
}

// TestDeleteByExprPropagatesDeleteError: a failed Delete surfaces wrapped
// (with the collection + expr named) so the caller aborts before the
// paired insert — a half-purged tier is worse than an untouched one.
func TestDeleteByExprPropagatesDeleteError(t *testing.T) {
	mc := &failingPurgeMock{purgeMock: purgeMock{exists: true}}

	err := DeleteByExpr(context.Background(), mc, Config{CollectionName: DefaultCollectionName}, "section_count > 0")
	if err == nil {
		t.Fatal("expected the delete error to propagate")
	}
	if !errors.Is(err, errDeleteBoom) {
		t.Fatalf("error chain must carry the SDK error, got: %v", err)
	}
	if !strings.Contains(err.Error(), DefaultCollectionName) {
		t.Fatalf("error must name the collection, got: %v", err)
	}
}

// TestGetMaxIDCollectionAlwaysTrueExprAndMax: the watermark is
// collection-wide — the expr is the always-true `id >= 0` (NOT a tier
// filter; Pitfall 5) and the returned max spans mock rows that
// conceptually belong to different tiers (one Query sees them all).
func TestGetMaxIDCollectionAlwaysTrueExprAndMax(t *testing.T) {
	tc := &getMaxIDMockClient{
		resultRows: []client.ResultSet{
			// Rows from multiple tiers arrive in ONE result set because
			// there is no tier filter: authority 7, sql-guides 42,
			// historical 100 — the collection-wide max is 100 even though
			// the per-tier max of any single tier would under-report it.
			client.ResultSet{idCol([]int64{7, 42, 100})},
		},
	}
	mc := Wrap(tc, Config{CollectionName: DefaultCollectionName})

	id, err := mc.GetMaxIDCollection(context.Background())
	if err != nil {
		t.Fatalf("GetMaxIDCollection: %v", err)
	}
	if id != 100 {
		t.Fatalf("GetMaxIDCollection = %d, want 100 (collection-wide max across mixed-tier rows)", id)
	}
	if len(tc.calls) != 1 {
		t.Fatalf("expected exactly one Query round-trip, calls = %v", tc.calls)
	}
	if tc.calls[0].expr != "id >= 0" {
		t.Fatalf("expr = %q, want the always-true \"id >= 0\" (a tier filter would under-estimate the watermark, Pitfall 5)", tc.calls[0].expr)
	}
	if tc.calls[0].coll != DefaultCollectionName {
		t.Fatalf("Query targeted %q, want %q", tc.calls[0].coll, DefaultCollectionName)
	}
}

// TestGetMaxIDCollectionEmptyReturnsZero: an empty collection has no
// watermark — 0 with no error, matching GetMaxID's empty posture.
func TestGetMaxIDCollectionEmptyReturnsZero(t *testing.T) {
	tc := &getMaxIDMockClient{
		resultRows: []client.ResultSet{client.ResultSet{}},
	}
	mc := Wrap(tc, Config{CollectionName: DefaultCollectionName})

	id, err := mc.GetMaxIDCollection(context.Background())
	if err != nil {
		t.Fatalf("GetMaxIDCollection on empty collection: %v", err)
	}
	if id != 0 {
		t.Fatalf("GetMaxIDCollection = %d, want 0 for an empty collection", id)
	}
}

// TestGetMaxIDCollectionTruncationGuard (WR-02): a result set that fills
// the 10000-row Query window may hide higher IDs beyond it — the watermark
// scan must fail loudly instead of answering a silently under-read max
// (an under-read watermark makes ECL allocate from below existing PKs, and
// Milvus does not dedupe Insert: the duplicate-generation incident class).
func TestGetMaxIDCollectionTruncationGuard(t *testing.T) {
	full := make([]int64, 10000)
	for i := range full {
		full[i] = int64(i + 1)
	}
	tc := &getMaxIDMockClient{
		resultRows: []client.ResultSet{client.ResultSet{idCol(full)}},
	}
	mc := Wrap(tc, Config{CollectionName: DefaultCollectionName})

	id, err := mc.GetMaxIDCollection(context.Background())
	if err == nil {
		t.Fatalf("GetMaxIDCollection on a window-filling result set = %d, want a loud truncation error", id)
	}
	if id != 0 {
		t.Fatalf("under truncation suspicion no watermark may be returned, got %d", id)
	}
	if !strings.Contains(err.Error(), "10000") {
		t.Fatalf("error must carry the window size for the operator, got: %v", err)
	}
	if !strings.Contains(err.Error(), "GetMaxIDCollection") {
		t.Fatalf("error must name the scanning op, got: %v", err)
	}
}

// TestGetMaxIDTruncationGuard (WR-02): the same guard on the per-tier
// scan — a tier whose scannable rows fill the window errors instead of
// under-reporting.
func TestGetMaxIDTruncationGuard(t *testing.T) {
	full := make([]int64, 10000)
	for i := range full {
		full[i] = int64(i + 1)
	}
	tc := &getMaxIDMockClient{
		resultRows: []client.ResultSet{client.ResultSet{idCol(full)}},
	}
	mc := Wrap(tc, Config{CollectionName: DefaultCollectionName})

	if _, err := mc.GetMaxID(context.Background(), "sql-guides"); err == nil {
		t.Fatal("GetMaxID on a window-filling result set: want a loud truncation error")
	}
}
