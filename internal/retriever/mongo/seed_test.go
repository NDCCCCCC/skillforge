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

// Seed-path tests for mongo.SeedFromKnowledgeDir (Phase 26 / Plan 26-03,
// KBM-06): the Mongo backend's corpus seeding now goes through the SAME
// registry-driven walker as knowledge.Index, so its three-state directory
// behavior must be byte-for-byte the same decision table.
//
// This file is the mongo-side half of the shared-behavior parity pair
// (26-RESEARCH Pitfall 4). It lives in the mongo package because a
// package knowledge test cannot import mongo (mongo imports knowledge);
// the matching knowledge-side half is internal/knowledge/walk_test.go,
// and BOTH sides use knowledge.WalkRegisteredDirs as the shared oracle on
// the SAME tree shape: two active dirs (authority, historical), one
// deactivated dir (personal), one unregistered dir (legacy-cn).
//
// Everything here is hermetic: the insert hook intercepts the Reset+Insert
// write phase (mirroring knowledge.indexSeedHook) and the audit hook
// collects rows in memory. No live MongoDB, no backend.

package mongo

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
)

// fakeSeedEmbedder is a minimal embedder.Embedder; the seed path only
// needs Embed to succeed (vector contents are irrelevant to the walk).
type fakeSeedEmbedder struct {
	mu    sync.Mutex
	calls []string
}

func (e *fakeSeedEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, text)
	return make([]float32, 4), nil
}

func (e *fakeSeedEmbedder) Dimensions() int { return 4 }

// Compile-time proof the fake satisfies the production interface.
var _ embedder.Embedder = (*fakeSeedEmbedder)(nil)

// baselineSeedRegistry mirrors the knowledge package's test registry: the
// four baseline slugs, with personal deactivated (the parity tree needs a
// registered-but-switched-off dir).
func baselineSeedRegistry() kb.Registry {
	roles := map[string]kb.Role{
		"authority":  kb.RoleAuthority,
		"historical": kb.RoleHistorical,
		"personal":   kb.RolePersonal,
		"draft":      kb.RoleDraft,
	}
	slugs := []string{"authority", "historical", "personal", "draft"}
	entries := make([]kb.Entry, 0, len(slugs))
	for _, slug := range slugs {
		enabled := slug != "personal" // personal is deactivated for the parity tree
		entries = append(entries, kb.Entry{
			Slug:             slug,
			DisplayName:      slug,
			Role:             roles[slug],
			RetrievalEnabled: enabled,
		})
	}
	return kb.New(entries)
}

// seedAuditRow is one collected audit callback invocation.
type seedAuditRow struct {
	action string
	detail string
}

// seedAuditCapture is the in-memory audit collector bound to
// mongo.SeedAuditHook's backing var for the duration of a test.
type seedAuditCapture struct {
	mu   sync.Mutex
	rows []seedAuditRow
}

func (c *seedAuditCapture) record(action, detail string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows = append(c.rows, seedAuditRow{action: action, detail: detail})
}

func (c *seedAuditCapture) all() []seedAuditRow {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]seedAuditRow, len(c.rows))
	copy(out, c.rows)
	return out
}

// withSeedInsertHook binds an insert-collecting hook over the write phase
// (Reset+Insert) for one test and restores the previous binding on cleanup.
func withSeedInsertHook(t *testing.T) *[]retriever.Document {
	t.Helper()
	var captured []retriever.Document
	prev := seedInsertHook
	seedInsertHook = func(_ context.Context, docs []retriever.Document) error {
		captured = make([]retriever.Document, len(docs))
		copy(captured, docs)
		return nil
	}
	t.Cleanup(func() { seedInsertHook = prev })
	return &captured
}

// withSeedAuditHook binds an in-memory audit collector for one test.
func withSeedAuditHook(t *testing.T) *seedAuditCapture {
	t.Helper()
	cap := &seedAuditCapture{}
	prev := seedAuditHook
	seedAuditHook = cap.record
	t.Cleanup(func() { seedAuditHook = prev })
	return cap
}

// makeParityTree builds the SAME tree shape as the knowledge package's
// parity test (see internal/knowledge/walk_test.go makeParityTree).
func makeParityTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"authority", "historical", "personal", "legacy-cn"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	}
	writeSeedDoc(t, root, "authority", "a.md", "现行制度。")
	writeSeedDoc(t, root, "historical", "h.md", "被取代的旧版制度。")
	writeSeedDoc(t, root, "personal", "p.md", "用户私有材料。")
	writeSeedDoc(t, root, "legacy-cn", "old.md", "CN-era leftover that must not be indexed")
	return root
}

func writeSeedDoc(t *testing.T, root, dir, name, body string) {
	t.Helper()
	p := filepath.Join(root, dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
}

// TestSeedFromKnowledgeDirWalkerParity — the mongo-side half of the parity
// pair. With the walker as the shared oracle on the shared tree, the Mongo
// seed must reach the SAME verdict as knowledge.Index: identical doc set
// (slug tiers over exactly the active dirs), identical skip decisions
// (nothing from the deactivated or the unregistered dir), identical audit
// surface (exactly one unregistered row, none for the deactivated dir).
func TestSeedFromKnowledgeDirWalkerParity(t *testing.T) {
	root := makeParityTree(t)
	reg := baselineSeedRegistry()
	c := newClientWithFake(&fakeFinder{})

	// Shared oracle: what the walker says is what BOTH backends must do.
	active, err := knowledge.WalkRegisteredDirs(root, reg, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"authority", "historical"}, active)

	inserted := withSeedInsertHook(t)
	auditCap := withSeedAuditHook(t)

	require.NoError(t, c.SeedFromKnowledgeDir(context.Background(), root, &fakeSeedEmbedder{}, reg))

	// Same doc set as the walker's active set: one doc per active dir,
	// tier = the slug, sources relative and slash-normalized.
	got := make([]string, 0, len(*inserted))
	for _, d := range *inserted {
		got = append(got, d.Tier)
		assert.NotContains(t, d.Source, "\\", "source paths stay slash-normalized")
	}
	assert.ElementsMatch(t, active, got,
		"mongo seed must index exactly the dirs the walker returned, one doc per dir")
	for _, d := range *inserted {
		assert.NotEqual(t, "personal", d.Tier, "deactivated dir must not be seeded")
		assert.NotEqual(t, "legacy-cn", d.Tier, "unregistered dir must not be seeded")
	}

	// Same audit surface: exactly one unregistered row naming legacy-cn and
	// none for the deactivated dir.
	rows := auditCap.all()
	require.Len(t, rows, 1)
	assert.Equal(t, "action_kb_dir_unregistered", rows[0].action)
	assert.Contains(t, rows[0].detail, "legacy-cn")
}

// TestSeedFromKnowledgeDirNilRegistry: the Mongo seed fails closed on a
// nil registry, mirroring knowledge.Index — silently seeding nothing would
// read as a healthy empty collection.
func TestSeedFromKnowledgeDirNilRegistry(t *testing.T) {
	c := newClientWithFake(&fakeFinder{})
	err := c.SeedFromKnowledgeDir(context.Background(), t.TempDir(), &fakeSeedEmbedder{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry is required")
}

// TestSeedFromKnowledgeDirEmptyTreeNoWrite: an empty knowledge tree is a
// legal no-op — the write phase (Reset+Insert) must not run at all, so an
// idle boot never wipes a collection it did not refill.
func TestSeedFromKnowledgeDirEmptyTreeNoWrite(t *testing.T) {
	root := t.TempDir() // no subdirectories at all
	c := newClientWithFake(&fakeFinder{})

	inserted := withSeedInsertHook(t)
	require.NoError(t, c.SeedFromKnowledgeDir(context.Background(), root, &fakeSeedEmbedder{}, baselineSeedRegistry()))
	assert.Empty(t, *inserted, "no docs → no write phase (the existing idempotent Reset+Insert pair stays untouched in production)")
}
