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

// Plan 26-09 Task 2 — the SKILLFORGE_MILVUS_RECREATE re-seed gate, tested
// hermetically. The gate's whole point is that a MISTAKE is expensive (a
// full re-embed) or a disaster (a CN legacy vector library silently
// refusing every query, Pitfall 1), so the two properties under test are:
//
//   - the sequence is drop-then-ensure, and the drop targets exactly the
//     production collection (never "whatever collection happens to be
//     configured");
//   - the gate is CONSERVATIVE: only the literal "1" opens it, so a
//     `RECREATE=true` in an ops runbook cannot wipe a corpus by accident.

import (
	"context"
	"strings"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// recreateMockClient records the sequence of schema-mutating calls. Every
// other method of client.Client stays on the embedded nil interface: if
// the code under test reaches one, the test panics instead of silently
// passing — which is the point of a hermetic mock.
type recreateMockClient struct {
	client.Client
	calls []string
	exists bool
}

func (m *recreateMockClient) record(call string) {
	m.calls = append(m.calls, call)
}

func (m *recreateMockClient) DropCollection(_ context.Context, collName string, _ ...client.DropCollectionOption) error {
	m.record("drop:" + collName)
	m.exists = false // the drop removes the collection
	return nil
}

func (m *recreateMockClient) HasCollection(_ context.Context, collName string) (bool, error) {
	m.record("has:" + collName)
	return m.exists, nil
}

func (m *recreateMockClient) CreateCollection(_ context.Context, schema *entity.Schema, _ int32, _ ...client.CreateCollectionOption) error {
	m.record("create:" + schema.CollectionName)
	m.exists = true
	return nil
}

func (m *recreateMockClient) CreateIndex(_ context.Context, collName string, _ string, _ entity.Index, _ bool, _ ...client.IndexOption) error {
	m.record("index:" + collName)
	return nil
}

func (m *recreateMockClient) indexOf(needle string) int {
	for i, c := range m.calls {
		if strings.HasPrefix(c, needle) {
			return i
		}
	}
	return -1
}

// TestRecreateCollectionSequence locks the drop → ensure order and the
// collection identity: the drop names the production collection and the
// recreate ends with a fresh, indexed collection of the same name.
func TestRecreateCollectionSequence(t *testing.T) {
	mc := &recreateMockClient{exists: true}
	c := Wrap(mc, Config{CollectionName: DefaultCollectionName})

	if err := c.RecreateCollection(context.Background(), DefaultCollectionName); err != nil {
		t.Fatalf("RecreateCollection: %v", err)
	}

	dropAt := mc.indexOf("drop:")
	if dropAt < 0 {
		t.Fatalf("no drop recorded, calls = %v", mc.calls)
	}
	if got := mc.calls[dropAt]; got != "drop:"+DefaultCollectionName {
		t.Fatalf("drop targeted %q, want the requested collection %q", got, DefaultCollectionName)
	}
	createAt := mc.indexOf("create:")
	if createAt < 0 {
		t.Fatalf("no collection re-created, calls = %v (the recreate must end with a usable collection)", mc.calls)
	}
	if createAt < dropAt {
		t.Fatalf("sequence broken: create (call %d) ran before the drop (call %d) — calls = %v", createAt, dropAt, mc.calls)
	}
	if mc.indexOf("index:") < 0 {
		t.Fatalf("no IVF_FLAT index rebuilt, calls = %v (a recreated collection without its index cannot serve search)", len(mc.calls))
	}
	for _, c := range mc.calls {
		if strings.HasPrefix(c, "create:") && !strings.HasSuffix(c, DefaultCollectionName) {
			t.Fatalf("recreate produced %q, want %q (a differently named collection would silently strand the corpus)", c, "create:"+DefaultCollectionName)
		}
	}
}

// TestRecreateCollectionRejectsEmptyName: the collection argument is the
// destructive target; an empty one must not fall through to a default and
// silently drop a collection the caller did not name.
func TestRecreateCollectionRejectsEmptyName(t *testing.T) {
	mc := &recreateMockClient{exists: true}
	c := Wrap(mc, Config{CollectionName: DefaultCollectionName})

	if err := c.RecreateCollection(context.Background(), ""); err == nil {
		t.Fatal("expected an error for an empty collection name (the drop target must be explicit)")
	}
	if len(mc.calls) != 0 {
		t.Fatalf("no call may run before the name check, calls = %v", mc.calls)
	}
}

// TestRecreateEnvGateParsing is T-26-09-2's mitigation: the gate opens on
// the literal "1" and nothing else. "true"/"yes"/"on" and any padded or
// cased variant stay closed, so a sloppy ops line cannot trigger a full
// corpus re-embed — and the default (unset) is closed.
func TestRecreateEnvGateParsing(t *testing.T) {
	closed := []string{"", "0", "false", "no", "off", "true", "yes", "on", "TRUE", "True", "1 ", " 1", "01", "1.0", "recreate"}
	for _, v := range closed {
		if ShouldRecreate(v) {
			t.Errorf("ShouldRecreate(%q) = true, want false (the gate opens on the literal \"1\" only)", v)
		}
	}
	if !ShouldRecreate("1") {
		t.Error(`ShouldRecreate("1") = false, want true`)
	}

	// The gate's single home must be named by the env var constant, so the
	// read point and the implementation cannot drift apart.
	if RecreateEnvVar != "SKILLFORGE_MILVUS_RECREATE" {
		t.Fatalf("RecreateEnvVar = %q, want SKILLFORGE_MILVUS_RECREATE", RecreateEnvVar)
	}
}

// TestDropOnlyOnRecreatePath is the structural half of T-26-09-2: inside
// this package, the destructive call exists ONLY on the recreate path —
// regular boot (Ensure) must never drop anything.
func TestDropOnlyOnRecreatePath(t *testing.T) {
	mc := &recreateMockClient{exists: true}
	c := Wrap(mc, Config{CollectionName: DefaultCollectionName})

	// The routine boot path (what buildRetriever calls every start).
	if err := c.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	for _, call := range mc.calls {
		if strings.HasPrefix(call, "drop:") {
			t.Fatalf("Ensure dropped the collection: calls = %v (a routine boot must never be destructive)", mc.calls)
		}
	}
}
