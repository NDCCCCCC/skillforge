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

package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeUpgradeEnv builds a temp deployment layout: an ontology dataDir plus
// empty skills/ and knowledge/ trees (Emit tolerates both being empty).
func makeUpgradeEnv(t *testing.T) (dataDir, skillsDir, kbDir string) {
	t.Helper()
	root := t.TempDir()
	dataDir = filepath.Join(root, "ontology")
	skillsDir = filepath.Join(root, "skills")
	kbDir = filepath.Join(root, "knowledge")
	for _, d := range []string{dataDir, skillsDir, kbDir} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	return dataDir, skillsDir, kbDir
}

// readTBoxArtifact reads a file, failing the test when missing.
func readTBoxArtifact(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "artifact %s must exist", path)
	return string(data)
}

// legacyPointerTTL approximates a Phase 38 6-class pointer: valid Turtle,
// no NoteChunk sentinel.
const legacyPointerTTL = `@prefix skf:  <http://skillforge.local/ontology#> .
@prefix owl:  <http://www.w3.org/2002/07/owl#> .

skf:Tier a owl:Class ;
    rdfs:label "Tier" .

skf:Skill a owl:Class ;
    rdfs:label "Skill" .
`

// TestUpgradeIfNeeded_FreshBootCreatesV1 — state 1: missing pointer emits
// the current TBox as v1 + pointer + version file.
func TestUpgradeIfNeeded_FreshBootCreatesV1(t *testing.T) {
	dataDir, skillsDir, kbDir := makeUpgradeEnv(t)

	upgraded, version, err := UpgradeIfNeeded(dataDir, DefaultNS, skillsDir, kbDir)
	require.NoError(t, err)
	assert.True(t, upgraded, "first boot must report an upgrade")
	assert.Equal(t, 1, version)

	pointer := readTBoxArtifact(t, filepath.Join(dataDir, pointerFileName))
	assert.Contains(t, pointer, noteChunkSentinel, "fresh emit carries the Phase 39 classes")
	assert.Contains(t, pointer, "skf:Tier a owl:Class")
	assert.Equal(t, pointer, readTBoxArtifact(t, filepath.Join(dataDir, "skillforge_tbox.v1.ttl")),
		"pointer content must equal the v1 artifact")
	assert.Equal(t, "1\n", readTBoxArtifact(t, filepath.Join(dataDir, versionFileName)))

	// Idempotence on the fresh path: a second boot is a no-op.
	upgraded2, version2, err := UpgradeIfNeeded(dataDir, DefaultNS, skillsDir, kbDir)
	require.NoError(t, err)
	assert.False(t, upgraded2)
	assert.Equal(t, 1, version2)
}

// TestUpgradeIfNeeded_SentinelIsNoOp — state 2: an 8-class pointer NEVER
// re-emits, even when the skills/knowledge source trees change underneath
// (the anti-Pitfall-4 regression guard).
func TestUpgradeIfNeeded_SentinelIsNoOp(t *testing.T) {
	dataDir, skillsDir, kbDir := makeUpgradeEnv(t)
	pointerPath := filepath.Join(dataDir, pointerFileName)

	// Seed an up-to-date environment at version 3.
	require.NoError(t, os.WriteFile(pointerPath, []byte(legacyPointerTTL+"\n"+noteChunkSentinel+" ;\n    rdfs:label \"seed\" .\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, versionFileName), []byte("3\n"), 0o644))
	before := readTBoxArtifact(t, pointerPath)
	versionBefore := readTBoxArtifact(t, filepath.Join(dataDir, versionFileName))

	upgraded, version, err := UpgradeIfNeeded(dataDir, DefaultNS, skillsDir, kbDir)
	require.NoError(t, err)
	assert.False(t, upgraded, "sentinel-bearing pointer must be a no-op")
	assert.Equal(t, 3, version, "no-op reports the current version")

	// Mutate the source trees the way a live deployment would.
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "compliance"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "compliance", "SKILL.md"),
		[]byte("---\nname: compliance\ndescription: changed after the TBox was emitted\nagent: ChatModelAgent\nwrites: []\n---\nbody"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(kbDir, "authority"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(kbDir, "authority", "new.md"), []byte("new corpus file"), 0o644))

	upgraded2, version2, err := UpgradeIfNeeded(dataDir, DefaultNS, skillsDir, kbDir)
	require.NoError(t, err)
	assert.False(t, upgraded2, "changed source trees must NOT trigger a re-emit")
	assert.Equal(t, 3, version2)
	assert.Equal(t, before, readTBoxArtifact(t, pointerPath), "pointer bytes unchanged across boots")
	assert.Equal(t, versionBefore, readTBoxArtifact(t, filepath.Join(dataDir, versionFileName)))
}

// TestUpgradeIfNeeded_LegacyPointerUpgradesOnce — state 3: a Phase-38
// (sentinel-less) pointer emits v{N+1}, bumps the version, swaps the
// pointer, and keeps the old classes.
func TestUpgradeIfNeeded_LegacyPointerUpgradesOnce(t *testing.T) {
	dataDir, skillsDir, kbDir := makeUpgradeEnv(t)
	pointerPath := filepath.Join(dataDir, pointerFileName)
	require.NoError(t, os.WriteFile(pointerPath, []byte(legacyPointerTTL), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, versionFileName), []byte("1\n"), 0o644))

	upgraded, version, err := UpgradeIfNeeded(dataDir, DefaultNS, skillsDir, kbDir)
	require.NoError(t, err)
	assert.True(t, upgraded, "Phase-38 pointer must trigger the one-time upgrade")
	assert.Equal(t, 2, version)

	v2 := readTBoxArtifact(t, filepath.Join(dataDir, "skillforge_tbox.v2.ttl"))
	assert.Contains(t, v2, noteChunkSentinel, "v2 must declare the NoteChunk class")
	assert.Contains(t, v2, "skf:Tier a owl:Class", "old classes intact")
	assert.Equal(t, v2, readTBoxArtifact(t, pointerPath), "pointer swapped to v2 content")
	assert.Equal(t, "2\n", readTBoxArtifact(t, filepath.Join(dataDir, versionFileName)))
	assert.NotContains(t, v2, "replayed approved induction proposals", "empty queue → no replay block")

	// Exactly once: the next boot is a no-op.
	upgraded2, version2, err := UpgradeIfNeeded(dataDir, DefaultNS, skillsDir, kbDir)
	require.NoError(t, err)
	assert.False(t, upgraded2)
	assert.Equal(t, 2, version2)
}

// TestUpgradeIfNeeded_ReplaysApprovedProposals — state 3 with a populated
// induction queue: the approved proposal's Turtle lines must appear in the
// fresh v{N+1} TTL (approved schema mutations survive the upgrade).
func TestUpgradeIfNeeded_ReplaysApprovedProposals(t *testing.T) {
	dataDir, skillsDir, kbDir := makeUpgradeEnv(t)
	pointerPath := filepath.Join(dataDir, pointerFileName)
	require.NoError(t, os.WriteFile(pointerPath, []byte(legacyPointerTTL), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, versionFileName), []byte("1\n"), 0o644))

	// An approved + a rejected proposal in the induction queue shape
	// (proposals.json / queueFile). Only the APPROVED one replays.
	proposalsJSON := `{
  "next_id": 3,
  "items": [
    {
      "id": "prop-0001",
      "kind": "Class",
      "description_zh": "审批件",
      "description_en": "approval case",
      "local_name": "ApprovalCase",
      "proposed_iri": "http://skillforge.local/ontology#ApprovalCase",
      "proposed_triples": [
        "<http://skillforge.local/ontology#ApprovalCase> a owl:Class ;",
        "    rdfs:label \"审批件\"@zh , \"approval case\"@en ."
      ],
      "status": "approved",
      "source_text": "approved text",
      "created_at": "2026-09-21T00:00:00Z",
      "decided_at": "2026-09-21T01:00:00Z"
    },
    {
      "id": "prop-0002",
      "kind": "DatatypeProperty",
      "description_zh": "驳回",
      "description_en": "rejected prop",
      "local_name": "RejectedProp",
      "proposed_iri": "http://skillforge.local/ontology#RejectedProp",
      "proposed_triples": [
        "<http://skillforge.local/ontology#RejectedProp> a owl:DatatypeProperty ;",
        "    rdfs:label \"驳回\"@zh , \"rejected prop\"@en ."
      ],
      "status": "rejected",
      "source_text": "rejected text",
      "created_at": "2026-09-21T00:00:00Z"
    }
  ]
}`
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "proposals.json"), []byte(proposalsJSON), 0o644))

	upgraded, version, err := UpgradeIfNeeded(dataDir, DefaultNS, skillsDir, kbDir)
	require.NoError(t, err)
	assert.True(t, upgraded)
	assert.Equal(t, 2, version)

	v2 := readTBoxArtifact(t, filepath.Join(dataDir, "skillforge_tbox.v2.ttl"))
	assert.Contains(t, v2, "<http://skillforge.local/ontology#ApprovalCase> a owl:Class ;",
		"approved proposal triples replayed into the fresh TBox")
	assert.Contains(t, v2, "rdfs:label \"审批件\"@zh , \"approval case\"@en .")
	assert.NotContains(t, v2, "RejectedProp", "rejected proposals must NOT replay")
}

// TestUpgradeIfNeeded_PointerReadErrorIsReturned — a pointer that exists
// but cannot be read is an ERROR, never a silent re-emit (the upgrader
// must not guess at un-readable state; the caller owns the fatal posture).
func TestUpgradeIfNeeded_PointerReadErrorIsReturned(t *testing.T) {
	dataDir, skillsDir, kbDir := makeUpgradeEnv(t)
	// A directory where the pointer file should be: Stat succeeds, Read fails.
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, pointerFileName), 0o755))

	_, _, err := UpgradeIfNeeded(dataDir, DefaultNS, skillsDir, kbDir)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "read pointer"),
		"error should name the pointer read, got: %v", err)
}
