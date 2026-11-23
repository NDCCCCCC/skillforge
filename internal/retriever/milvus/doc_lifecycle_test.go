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

// Hermetic tests for the retriever.DocChunkStore implementation on
// MilvusClient (Phase 53-03 / D-53-03). The properties under test mirror
// the purge-mock discipline (delete_tier_rows_test.go): the expression is
// injection-escaped AND tier-scoped, the mechanics keep the
// HasCollection → LoadCollection → DML → Flush order, a missing
// collection degrades to the empty/no-op posture, and a window-filling
// result set fails loudly (WR-02) instead of under-reporting the doc's
// rows.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// lifecycleMock records the DML/Query sequence. Unmocked methods of the
// embedded nil interface panic — the hermetic-mock discipline from
// purgeMock.
type lifecycleMock struct {
	client.Client
	calls       []string
	exists      bool
	queryResult client.ResultSet
	queryErr    error
}

func (m *lifecycleMock) HasCollection(_ context.Context, collName string) (bool, error) {
	m.calls = append(m.calls, "has:"+collName)
	return m.exists, nil
}

func (m *lifecycleMock) LoadCollection(_ context.Context, collName string, _ bool, _ ...client.LoadCollectionOption) error {
	m.calls = append(m.calls, "load:"+collName)
	return nil
}

func (m *lifecycleMock) Query(_ context.Context, collName string, _ []string, expr string, outputFields []string, _ ...client.SearchQueryOptionFunc) (client.ResultSet, error) {
	m.calls = append(m.calls, "query:"+collName)
	m.calls = append(m.calls, "expr:"+expr)
	m.calls = append(m.calls, "fields:"+strings.Join(outputFields, ","))
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return m.queryResult, nil
}

func (m *lifecycleMock) Delete(_ context.Context, collName string, _ string, expr string) error {
	m.calls = append(m.calls, "delete:"+collName)
	m.calls = append(m.calls, "expr:"+expr)
	return nil
}

func (m *lifecycleMock) Flush(_ context.Context, collName string, _ bool, _ ...client.FlushOption) error {
	m.calls = append(m.calls, "flush:"+collName)
	return nil
}

// Compile-time guard: the lifecycle tests keep working against the
// Client interface they mock.
var _ client.Client = (*lifecycleMock)(nil)

// docIDExprWant is the exact expression both lifecycle primitives must
// produce for the shared fixture — %q-escaped (injection-safe) and
// tier-scoped (the trust boundary stays load-bearing on the lifecycle
// path).
const (
	lifecycleTier  = "authority"
	lifecycleDocID = "authority/abc12345-policy.md"
	docIDExprWant  = `tier == "authority" and doc_id == "authority/abc12345-policy.md"`
)

func TestDocChunkRowsReturnsFullProjection(t *testing.T) {
	mc := &lifecycleMock{
		exists: true,
		queryResult: client.ResultSet{
			entity.NewColumnInt64("id", []int64{7, 9}),
			entity.NewColumnInt64("chunk_index", []int64{0, 1}),
			entity.NewColumnVarChar("content_hash", []string{"hash-a", "hash-b"}),
			entity.NewColumnFloatVector("embedding", 4, [][]float32{{1, 0, 0, 0}, {0, 1, 0, 0}}),
		},
	}
	m := Wrap(mc, Config{CollectionName: DefaultCollectionName})

	rows, err := m.DocChunkRows(context.Background(), lifecycleTier, lifecycleDocID)
	if err != nil {
		t.Fatalf("DocChunkRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].ID != 7 || rows[0].ChunkIndex != 0 || rows[0].ContentHash != "hash-a" {
		t.Fatalf("row 0 = %+v, want {7 0 hash-a}", rows[0])
	}
	if rows[1].ID != 9 || rows[1].ChunkIndex != 1 || rows[1].ContentHash != "hash-b" {
		t.Fatalf("row 1 = %+v, want {9 1 hash-b}", rows[1])
	}
	// The embedding is the load-bearing field: the upsert re-inserts
	// unchanged chunks with their ORIGINAL vector (the re-embed skip).
	if len(rows[0].Embedding) != 4 || rows[0].Embedding[0] != 1 {
		t.Fatalf("row 0 embedding = %v, want the stored 4-dim vector", rows[0].Embedding)
	}
	if len(rows[1].Embedding) != 4 || rows[1].Embedding[1] != 1 {
		t.Fatalf("row 1 embedding = %v, want the stored 4-dim vector", rows[1].Embedding)
	}

	var expr, fields string
	for _, c := range mc.calls {
		switch {
		case strings.HasPrefix(c, "expr:"):
			expr = strings.TrimPrefix(c, "expr:")
		case strings.HasPrefix(c, "fields:"):
			fields = strings.TrimPrefix(c, "fields:")
		}
	}
	if expr != docIDExprWant {
		t.Fatalf("expr = %s, want the escaped tier-scoped form %s", expr, docIDExprWant)
	}
	for _, want := range []string{"id", "chunk_index", "content_hash", "embedding"} {
		if !strings.Contains(fields, want) {
			t.Fatalf("output fields = %s, must request %q (the upsert needs the stored vector to skip re-embedding)", fields, want)
		}
	}
}

func TestDocChunkRowsNoRowsReturnsEmpty(t *testing.T) {
	// A result set with columns but zero rows is how a doc_id with no
	// rows comes back — empty slice, nil error (first upload).
	mc := &lifecycleMock{
		exists: true,
		queryResult: client.ResultSet{
			entity.NewColumnInt64("id", []int64{}),
		},
	}
	m := Wrap(mc, Config{CollectionName: DefaultCollectionName})

	rows, err := m.DocChunkRows(context.Background(), lifecycleTier, lifecycleDocID)
	if err != nil {
		t.Fatalf("DocChunkRows on empty doc: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %d, want 0 (idCol present but empty)", len(rows))
	}
}

func TestDocChunkRowsMissingCollectionEmptyNoQuery(t *testing.T) {
	mc := &lifecycleMock{exists: false}
	m := Wrap(mc, Config{CollectionName: DefaultCollectionName})

	rows, err := m.DocChunkRows(context.Background(), lifecycleTier, lifecycleDocID)
	if err != nil {
		t.Fatalf("DocChunkRows on missing collection: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(rows))
	}
	for _, c := range mc.calls {
		if strings.HasPrefix(c, "query:") {
			t.Fatalf("missing collection must be a has-only no-op, calls = %v", mc.calls)
		}
	}
}

func TestDocChunkRowsTruncationGuard(t *testing.T) {
	// WR-02 analog: a window-filling result set may hide rows beyond it —
	// the upsert would decide reuse/delete over an incomplete view of the
	// doc. Fail loudly instead.
	full := make([]int64, maxIDQueryWindow)
	for i := range full {
		full[i] = int64(i + 1)
	}
	mc := &lifecycleMock{
		exists:      true,
		queryResult: client.ResultSet{entity.NewColumnInt64("id", full)},
	}
	m := Wrap(mc, Config{CollectionName: DefaultCollectionName})

	rows, err := m.DocChunkRows(context.Background(), lifecycleTier, lifecycleDocID)
	if err == nil {
		t.Fatalf("DocChunkRows on a window-filling result set = %d rows, want a loud truncation error", len(rows))
	}
	if rows != nil {
		t.Fatalf("under truncation suspicion no rows may be returned, got %d", len(rows))
	}
	if !strings.Contains(err.Error(), "10000") {
		t.Fatalf("error must carry the window size for the operator, got: %v", err)
	}
	if !strings.Contains(err.Error(), "DocChunkRows") {
		t.Fatalf("error must name the op, got: %v", err)
	}
}

func TestDocChunkRowsQueryErrorPropagates(t *testing.T) {
	mc := &lifecycleMock{
		exists:   true,
		queryErr: errors.New("query boom"),
	}
	m := Wrap(mc, Config{CollectionName: DefaultCollectionName})

	if _, err := m.DocChunkRows(context.Background(), lifecycleTier, lifecycleDocID); err == nil {
		t.Fatal("expected the query error to propagate")
	}
}

func TestDeleteDocRowsExprVerbatimAndMechanics(t *testing.T) {
	mc := &lifecycleMock{exists: true}
	m := Wrap(mc, Config{CollectionName: DefaultCollectionName})

	if err := m.DeleteDocRows(context.Background(), lifecycleTier, lifecycleDocID); err != nil {
		t.Fatalf("DeleteDocRows: %v", err)
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
	if expr != docIDExprWant {
		t.Fatalf("expr = %s, want the escaped tier-scoped form %s", expr, docIDExprWant)
	}
}

func TestDeleteDocRowsMissingCollectionNoOp(t *testing.T) {
	mc := &lifecycleMock{exists: false}
	m := Wrap(mc, Config{CollectionName: DefaultCollectionName})

	if err := m.DeleteDocRows(context.Background(), lifecycleTier, lifecycleDocID); err != nil {
		t.Fatalf("DeleteDocRows on missing collection: %v", err)
	}
	for _, c := range mc.calls {
		if !strings.HasPrefix(c, "has:") {
			t.Fatalf("missing collection must be a has-only no-op, calls = %v", mc.calls)
		}
	}
}

func TestDeleteDocRowsDeleteErrorPropagates(t *testing.T) {
	mc := &failingLifecycleMock{lifecycleMock: lifecycleMock{exists: true}}
	m := Wrap(mc, Config{CollectionName: DefaultCollectionName})

	err := m.DeleteDocRows(context.Background(), lifecycleTier, lifecycleDocID)
	if err == nil {
		t.Fatal("expected the delete error to propagate")
	}
	if !errors.Is(err, errDeleteBoom) {
		t.Fatalf("error chain must carry the SDK error, got: %v", err)
	}
	if !strings.Contains(err.Error(), lifecycleDocID) {
		t.Fatalf("error must name the doc_id, got: %v", err)
	}
}

type failingLifecycleMock struct {
	lifecycleMock
}

func (m *failingLifecycleMock) Delete(_ context.Context, collName string, _ string, _ string) error {
	m.calls = append(m.calls, "delete:"+collName)
	return errDeleteBoom
}
