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
