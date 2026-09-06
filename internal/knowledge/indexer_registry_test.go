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

// Registry-driven index-path tests (Phase 26 / Plan 26-03, KBM-05 + KBM-06).
//
// The knowledge/<slug>/ directory walk is registry-driven from this plan on,
// with one three-state decision table shared by every corpus consumer:
//
//	active slug       → indexed (tier = the slug itself)
//	deactivated slug  → skipped with a WARN, NO audit row (a deactivation is
//	                    a known, operator-authored state, not an anomaly)
//	unregistered dir  → skipped with a WARN + an INSERT-only audit row
//	                    (action_kb_dir_unregistered) delivered through the
//	                    indexAuditHook callback seam (Pitfall 7: callback
//	                    injection, never a store.Store parameter)
//
// The service never fails on an unknown directory (SC-5). The KNOW-02 main
// gate (validateTiers) now resolves through kb.Registry.Lookup, so an
// unknown OR deactivated tier value fails closed with the active slug set
// in the message.
//
// Everything here is hermetic: the seedDocs hook intercepts the Milvus
// write and the audit hook collects rows in memory. No CGO, no backend.

package knowledge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
)

// testBaselineSlugs mirrors the four baseline slugs declared by the
// controlled seed (knowledge_bases.yaml). They double as the
// knowledge/<slug>/ directory names, which is exactly the property the
// walker depends on.
var testBaselineSlugs = []string{"authority", "historical", "personal", "draft"}

// testSlugRoles maps each baseline slug to its semantic role. The role is
// registry identity data, not tier semantics — the indexer never branches
// on it (only kb does).
var testSlugRoles = map[string]kb.Role{
	"authority":  kb.RoleAuthority,
	"historical": kb.RoleHistorical,
	"personal":   kb.RolePersonal,
	"draft":      kb.RoleDraft,
}

// baselineEntries builds one active Entry per baseline slug. DisplayName is
// the slug itself: display names are data whose home is the seed file, and
// no assertion in this package may depend on them.
func baselineEntries() []kb.Entry {
	entries := make([]kb.Entry, 0, len(testBaselineSlugs))
	for _, slug := range testBaselineSlugs {
		entries = append(entries, kb.Entry{
			Slug:             slug,
			DisplayName:      slug,
			Role:             testSlugRoles[slug],
			RetrievalEnabled: true,
		})
	}
	return entries
}

// baselineRegistry is the read-only registry over the four baseline slugs,
// all active. Equivalent to what kb.LoadSeedRegistry yields for the shipped
// seed file.
func baselineRegistry() kb.Registry { return kb.New(baselineEntries()) }

// deactivatedRegistry returns a registry in which slug still EXISTS but is
// switched off: Lookup fails closed on it, List still shows it.
func deactivatedRegistry(slug string) kb.Registry {
	entries := baselineEntries()
	for i := range entries {
		if entries[i].Slug == slug {
			entries[i].RetrievalEnabled = false
		}
	}
	return kb.New(entries)
}

// registryWithExtra returns the baseline registry plus additional entries
// (used to prove the graph node set is N-dynamic, not fixed at 4).
func registryWithExtra(extra ...kb.Entry) kb.Registry {
	return kb.New(append(baselineEntries(), extra...))
}

// auditRow is one collected audit callback invocation.
type auditRow struct {
	action string
	detail string
}

// auditCapture is the in-memory audit collector hermetic tests bind to
// indexAuditHook (mirrors indexSeedCapture for the seedDocs seam).
type auditCapture struct {
	mu   sync.Mutex
	rows []auditRow
}

func (c *auditCapture) record(action, detail string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows = append(c.rows, auditRow{action: action, detail: detail})
}

func (c *auditCapture) all() []auditRow {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]auditRow, len(c.rows))
	copy(out, c.rows)
	return out
}

// withAuditHook binds the in-memory collector to indexAuditHook for the
// duration of one test and restores the previous binding on cleanup.
func withAuditHook(t *testing.T) *auditCapture {
	t.Helper()
	cap := &auditCapture{}
	prev := indexAuditHook
	indexAuditHook = cap.record
	t.Cleanup(func() { indexAuditHook = prev })
	return cap
}

// makeTree creates the named subdirectories under a fresh temp dir and
// returns the temp dir path (the knowledge root under test).
func makeTree(t *testing.T, dirs ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0o755))
	}
	return root
}

// --- WalkRegisteredDirs: the three-state decision table ---

// TestWalkRegisteredDirsActive: active slug directories are returned in
// ascending order; a non-directory entry (even one whose name looks like a
// slug) is ignored, not an error.
func TestWalkRegisteredDirsActive(t *testing.T) {
	root := makeTree(t, "historical", "authority")
	require.NoError(t, os.WriteFile(filepath.Join(root, "personal.d"), []byte("a file, not a dir"), 0o644))

	got, err := WalkRegisteredDirs(root, baselineRegistry(), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"authority", "historical"}, got,
		"active slug dirs are returned ascending; non-directory entries are ignored")
}

// TestWalkRegisteredDirsUnregistered: an unregistered directory is not
// indexed AND produces exactly one INSERT-only audit row naming it — but
// the walker does NOT fail (SC-5: the service never fails on an unknown
// directory).
func TestWalkRegisteredDirsUnregistered(t *testing.T) {
	root := makeTree(t, "authority", "legacy-cn")
	cap := &auditCapture{}

	got, err := WalkRegisteredDirs(root, baselineRegistry(), cap.record)
	require.NoError(t, err, "SC-5: an unregistered directory must not fail the walk")
	assert.Equal(t, []string{"authority"}, got, "the unregistered dir must not be returned")

	rows := cap.all()
	require.Len(t, rows, 1, "exactly one audit row for the one unregistered dir")
	assert.Equal(t, actionKBDirUnregistered, rows[0].action)
	// The detail must be a valid JSON object naming both the directory and
	// the knowledge root — cmd/server stores it verbatim in the audit
	// row's FrozenArgs column, so it has to stay parseable JSON.
	var detail struct {
		Dir          string `json:"dir"`
		KnowledgeDir string `json:"knowledge_dir"`
	}
	require.NoError(t, json.Unmarshal([]byte(rows[0].detail), &detail),
		"audit detail must be a JSON object (it lands in the FrozenArgs column)")
	assert.Equal(t, "legacy-cn", detail.Dir)
	assert.Equal(t, root, detail.KnowledgeDir)
}

// TestWalkRegisteredDirsDeactivated: a registered-but-deactivated directory
// is skipped WITHOUT an audit row — deactivation is a known operator state,
// not the anomaly the audit row exists for. The skip is proven by the
// returned set (the WARN goes to the log).
func TestWalkRegisteredDirsDeactivated(t *testing.T) {
	root := makeTree(t, "authority", "historical")
	cap := &auditCapture{}

	got, err := WalkRegisteredDirs(root, deactivatedRegistry("historical"), cap.record)
	require.NoError(t, err)
	assert.Equal(t, []string{"authority"}, got, "the deactivated dir must not be returned")
	assert.Empty(t, cap.all(), "deactivated != unregistered: no audit row")
}

// TestWalkRegisteredDirsMissingDirIsSilent: an empty (or wholly absent of
// registered dirs) knowledge root yields an empty result set with no error
// and no audit rows — the SC-4 refusal mechanism depends on the empty tree
// being a legal steady state, not a failure.
func TestWalkRegisteredDirsMissingDirIsSilent(t *testing.T) {
	root := makeTree(t) // empty knowledge root
	cap := &auditCapture{}

	got, err := WalkRegisteredDirs(root, baselineRegistry(), cap.record)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Empty(t, cap.all(), "an empty tree is legal, not an anomaly")
}

// --- Index: registry wiring + nil-registry fail-closed guard ---

// TestIndexRejectsNilRegistry: a nil registry is a programming error and
// must fail closed — silently indexing NOTHING (every dir unregistered)
// would look like a healthy empty corpus.
func TestIndexRejectsNilRegistry(t *testing.T) {
	dir := makeKnowledgeDir(t)
	err := Index(context.Background(), dir, newFakeEmbedder(), nil, milvus.Config{CollectionName: "t"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry is required")
}

// TestIndexUnknownDirWarnSkipAudit: end-to-end through Index, an
// unregistered directory contributes no docs, produces exactly one audit
// row via the hook, and Index still returns nil (SC-5).
func TestIndexUnknownDirWarnSkipAudit(t *testing.T) {
	root := makeTree(t, "authority", "legacy-cn")
	writeDoc(t, root, "authority", "a.md", "差旅审批制度正文。")
	writeDoc(t, root, "legacy-cn", "old.md", "leftover content that must not be indexed")

	seed := withSeedHook(t)
	auditCap := withAuditHook(t)

	require.NoError(t, driveIndex(t, root, newFakeEmbedder(), baselineRegistry()))

	require.Len(t, seed.docs, 1, "only the registered dir's doc is indexed")
	assert.Equal(t, "authority", seed.docs[0].Tier, "tier = the slug itself")

	rows := auditCap.all()
	require.Len(t, rows, 1)
	assert.Equal(t, actionKBDirUnregistered, rows[0].action)
	assert.Contains(t, rows[0].detail, "legacy-cn")
}

// TestIndexDeactivatedDirSkipped: a deactivated base's documents never
// enter the docs slice and no audit row is written (deactivation is known
// state, not an anomaly).
func TestIndexDeactivatedDirSkipped(t *testing.T) {
	root := makeTree(t, "authority", "historical")
	writeDoc(t, root, "authority", "a.md", "现行制度。")
	writeDoc(t, root, "historical", "h.md", "被取代的旧版制度。")

	seed := withSeedHook(t)
	auditCap := withAuditHook(t)

	require.NoError(t, driveIndex(t, root, newFakeEmbedder(), deactivatedRegistry("historical")))

	require.Len(t, seed.docs, 1)
	assert.Equal(t, "authority", seed.docs[0].Tier)
	assert.Empty(t, auditCap.all(), "no audit row for a deactivated directory")
}

// --- validateTiers: the KNOW-02 main gate through kb.Registry.Lookup ---

// TestValidateTiersRegistry locks the migrated gate semantics: unknown AND
// deactivated tier values fail closed with the active slug set in the
// message, an active slug passes, and the empty-tier KNOW-02 branch is
// preserved verbatim.
func TestValidateTiersRegistry(t *testing.T) {
	t.Run("unknown_tier_lists_active_slugs", func(t *testing.T) {
		docs := []retriever.Document{
			{ID: 1, Tier: "kb-nope", Source: "authority/bad.md", Content: "bad tier"},
		}
		err := validateTiers(docs, baselineRegistry())
		require.Error(t, err)
		msg := err.Error()
		assert.Contains(t, msg, "KNOW-02", "the gate must keep citing the requirement")
		assert.Contains(t, msg, "kb-nope", "the error must echo the offending value")
		for _, slug := range testBaselineSlugs {
			assert.Containsf(t, msg, slug, "the error must list active slug %q so the author can self-correct", slug)
		}
	})

	t.Run("active_slug_passes", func(t *testing.T) {
		docs := []retriever.Document{
			{ID: 1, Tier: "authority", Source: "authority/a.md", Content: "ok"},
		}
		assert.NoError(t, validateTiers(docs, baselineRegistry()))
	})

	t.Run("empty_tier_keeps_know02_error", func(t *testing.T) {
		docs := []retriever.Document{
			{ID: 1, Tier: "", Source: "authority/missing.md", Content: "no tier"},
		}
		err := validateTiers(docs, baselineRegistry())
		require.Error(t, err)
		msg := err.Error()
		assert.Contains(t, msg, "empty tier")
		assert.Contains(t, msg, "KNOW-02")
		assert.Contains(t, msg, "authority/missing.md")
	})

	t.Run("deactivated_slug_rejected", func(t *testing.T) {
		docs := []retriever.Document{
			{ID: 1, Tier: "historical", Source: "historical/h.md", Content: "deactivated base"},
		}
		err := validateTiers(docs, deactivatedRegistry("historical"))
		require.Error(t, err, "a deactivated slug must be rejected at index time too")
		msg := err.Error()
		assert.Contains(t, msg, "historical", "the error echoes the rejected value")
		assert.Contains(t, msg, "authority", "the error still lists the remaining active slugs")
	})

	t.Run("nil_registry_fails_closed", func(t *testing.T) {
		docs := []retriever.Document{
			{ID: 1, Tier: "authority", Source: "authority/a.md", Content: "ok"},
		}
		err := validateTiers(docs, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry is required")
	})
}

// TestDeterministicChunkIDs: two Index runs over the same tree produce a
// byte-identical docs slice — same order, same 1-based IDs, same sources
// and slug tiers (regression for the sorted-walk contract the FE chunk IDs
// and citation labels depend on).
func TestDeterministicChunkIDs(t *testing.T) {
	root := makeTree(t, "personal", "historical", "authority")
	writeDoc(t, root, "authority", "alpha.md", "A 类差旅政策。")
	writeDoc(t, root, "authority", "bravo.md", "B 类差旅政策。")
	writeDoc(t, root, "historical", "legacy.md", "旧版差旅政策。")
	writeDoc(t, root, "personal", "note.md", "部门内部备注。")

	first := withSeedHook(t)
	require.NoError(t, driveIndex(t, root, newFakeEmbedder(), baselineRegistry()))
	second := withSeedHook(t)
	require.NoError(t, driveIndex(t, root, newFakeEmbedder(), baselineRegistry()))

	require.Len(t, first.docs, 4)
	require.Len(t, second.docs, 4)
	assert.Equal(t, first.docs, second.docs,
		"two runs over the same tree must produce identical docs (order, IDs, sources, tiers)")
	for i, d := range first.docs {
		assert.Equal(t, int64(i+1), d.ID, "IDs stay 1-based and monotonically increasing")
	}
}
