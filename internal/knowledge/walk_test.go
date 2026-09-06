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

// Walker unit tests plus the knowledge-side half of the shared-behavior
// parity pair (26-RESEARCH Pitfall 4). One walker now serves every corpus
// consumer; the parity contract is asserted once per side with the walker
// as the shared oracle:
//
//   - this file proves knowledge.Index obeys the walker's decision table;
//   - internal/retriever/mongo/seed_test.go proves
//     mongo.SeedFromKnowledgeDir obeys the SAME table on the SAME tree
//     shape (that test lives in the mongo package because a package
//     knowledge test cannot import mongo — mongo imports knowledge).
//
// The shared tree shape is identical in both files: two active dirs
// (authority, historical), one deactivated dir (personal), one
// unregistered dir (legacy-cn).

package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kb"
)

// parityTreeDirs lists the directory names of the shared parity tree shape.
var parityTreeDirs = []string{"authority", "historical", "personal", "legacy-cn"}

// makeParityTree builds the shared parity tree: authority + historical
// active, personal registered-but-deactivated, legacy-cn unregistered.
// Every dir carries exactly one doc so the expected doc set is derivable
// from the active slug set alone.
func makeParityTree(t *testing.T) (root string, reg kb.Registry) {
	t.Helper()
	root = makeTree(t, parityTreeDirs...)
	writeDoc(t, root, "authority", "a.md", "现行制度。")
	writeDoc(t, root, "historical", "h.md", "被取代的旧版制度。")
	writeDoc(t, root, "personal", "p.md", "用户私有材料。")
	writeDoc(t, root, "legacy-cn", "old.md", "CN-era leftover that must not be indexed")
	return root, deactivatedRegistry("personal")
}

// TestWalkRegisteredDirsAbsentRoot: a knowledgeDir that does not exist at
// all is the same legal degraded posture as an empty one (SC-4) — empty
// result, no error, no audit rows.
func TestWalkRegisteredDirsAbsentRoot(t *testing.T) {
	root := t.TempDir()
	absent := root + "/does-not-exist"
	cap := &auditCapture{}

	got, err := WalkRegisteredDirs(absent, baselineRegistry(), cap.record)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Empty(t, cap.all())
}

// TestWalkRegisteredDirsNilRegistry: the walker fails closed on a nil
// registry — it must never silently classify every directory as
// unregistered.
func TestWalkRegisteredDirsNilRegistry(t *testing.T) {
	root := makeTree(t, "authority")
	_, err := WalkRegisteredDirs(root, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry is required")
}

// TestWalkRegisteredDirsEmptyDirArg: an empty knowledgeDir argument is a
// programming error, not an empty corpus.
func TestWalkRegisteredDirsEmptyDirArg(t *testing.T) {
	_, err := WalkRegisteredDirs("", baselineRegistry(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "knowledgeDir is empty")
}

// TestWalkerParityWithIndex — the knowledge-side half of the parity pair.
// On the shared tree the walker reports exactly the two active slugs, and
// knowledge.Index obeys that verdict end to end: identical doc set (slug
// tiers, ascending source order), identical skip decisions (nothing from
// the deactivated or the unregistered dir), identical audit surface
// (exactly one unregistered row, nothing for the deactivated dir).
func TestWalkerParityWithIndex(t *testing.T) {
	root, reg := makeParityTree(t)
	cap := &auditCapture{}

	active, err := WalkRegisteredDirs(root, reg, cap.record)
	require.NoError(t, err)
	assert.Equal(t, []string{"authority", "historical"}, active,
		"walker oracle: only the two active slugs, ascending")

	seed := withSeedHook(t)
	auditCap := withAuditHook(t)
	require.NoError(t, driveIndex(t, root, newFakeEmbedder(), reg))

	// The docs Index produced must be exactly the active dirs' docs.
	got := make([]string, 0, len(seed.docs))
	for _, d := range seed.docs {
		got = append(got, d.Tier)
	}
	assert.ElementsMatch(t, active, got,
		"Index must index exactly the dirs the walker returned, one doc per dir")

	// The audit surface must be exactly the walker's: one unregistered row
	// naming legacy-cn, and nothing for the deactivated dir.
	rows := auditCap.all()
	require.Len(t, rows, 1)
	assert.Equal(t, actionKBDirUnregistered, rows[0].action)
	assert.Contains(t, rows[0].detail, "legacy-cn")
}

// Index must never write a doc whose tier is outside the walker's active
// set — the property the KNOW-02 gate upstream of seedDocs depends on.
func TestIndexDocsStaysWithinWalkerActiveSet(t *testing.T) {
	root, reg := makeParityTree(t)

	seed := withSeedHook(t)
	require.NoError(t, driveIndex(t, root, newFakeEmbedder(), reg))

	active, err := WalkRegisteredDirs(root, reg, nil)
	require.NoError(t, err)
	allowed := make(map[string]bool, len(active))
	for _, slug := range active {
		allowed[slug] = true
	}
	for _, d := range seed.docs {
		assert.Truef(t, allowed[d.Tier],
			"doc tier %q must be in the walker's active set %v (KBM-05: value source is the registry)", d.Tier, active)
		assert.NoError(t, validateTiers(seed.docs, reg))
	}
}

// TestWalkRegisteredDirsRegistryIsTheOnlySource: the walker's output is a
// pure function of the registry — the same tree under a registry with a
// DIFFERENT active set yields a different walk, proving no directory
// enumeration survives in code (the fixed 4-dir list is gone).
func TestWalkRegisteredDirsRegistryIsTheOnlySource(t *testing.T) {
	root := makeTree(t, "authority", "historical")
	custom := kb.New([]kb.Entry{
		{Slug: "authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	got, err := WalkRegisteredDirs(root, custom, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"authority"}, got,
		"with historical unregistered, only authority is walked — the registry decides, not a code list")

	// Same tree, both active → both walked.
	got, err = WalkRegisteredDirs(root, baselineRegistry(), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"authority", "historical"}, got)
}
