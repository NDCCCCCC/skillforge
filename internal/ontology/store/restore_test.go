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

package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTTL loads to this many triples (counted dynamically so TTL edits
// can't silently break the arithmetic below).
func testTTLLen(t *testing.T) int {
	t.Helper()
	s := New()
	n, err := s.LoadFromTurtle(testTTL)
	require.NoError(t, err)
	return n
}

// aboxTriple is an ABox statement the ECL pipeline would have written —
// present ONLY in the snapshot, never in the TBox TTL.
func aboxTriple() Triple {
	return Triple{S: ns + "note_001", P: ns + "referencesTable", O: ns + "tbl_etl_payment_due_t"}
}

// writeRestoreFixturesWithStore writes the snapshot + TTL files and
// returns their paths (the store's Snapshot() output becomes the snapshot
// file).
func writeRestoreFixturesWithStore(t *testing.T, dir string, s *Store, ttl string) (snapshotPath, ttlPath string) {
	t.Helper()
	snapshotPath = filepath.Join(dir, "skillforge_kg.json")
	ttlPath = filepath.Join(dir, "skillforge_tbox.ttl")
	require.NoError(t, s.WriteSnapshot(snapshotPath))
	require.NoError(t, os.WriteFile(ttlPath, []byte(ttl), 0o644))
	return snapshotPath, ttlPath
}

// TestRestore_SnapshotPreservesABox — Pitfall 3's core assertion: an ABox
// triple that exists only in the snapshot SURVIVES the restore (the
// pre-Phase-39 boot built the KG from the TTL alone, wiping it).
func TestRestore_SnapshotPreservesABox(t *testing.T) {
	dir := t.TempDir()
	ttlN := testTTLLen(t)

	s := New()
	_, err := s.LoadFromTurtle(testTTL)
	require.NoError(t, err)
	abox := aboxTriple()
	s.Insert(abox)
	snapshotPath, ttlPath := writeRestoreFixturesWithStore(t, dir, s, testTTL)

	restored, ttlLoaded, err := Restore(snapshotPath, ttlPath)
	require.NoError(t, err)
	assert.Equal(t, ttlN, ttlLoaded, "returned count = triples loaded from the TTL (boot-log continuity)")
	assert.Equal(t, ttlN+1, restored.Len(), "ABox triple must survive on top of the TBox set")
	assert.Len(t, restored.Query(abox.S, abox.P, abox.O), 1, "ABox triple queryable after restore")
}

// TestRestore_IdenticalTTLNoDuplicates — snapshot + identical TTL → no
// double-counting (InsertUnique dedupe), and repeated restores are stable.
func TestRestore_IdenticalTTLNoDuplicates(t *testing.T) {
	dir := t.TempDir()
	ttlN := testTTLLen(t)

	s := New()
	_, err := s.LoadFromTurtle(testTTL)
	require.NoError(t, err)
	snapshotPath, ttlPath := writeRestoreFixturesWithStore(t, dir, s, testTTL)

	restored, ttlLoaded, err := Restore(snapshotPath, ttlPath)
	require.NoError(t, err)
	assert.Equal(t, ttlN, ttlLoaded)
	assert.Equal(t, ttlN, restored.Len(), "identical TTL must add exactly 0 triples (dedupe)")

	// Repeat the restore from the same artifacts — Len must stay put.
	restored2, _, err := Restore(snapshotPath, ttlPath)
	require.NoError(t, err)
	assert.Equal(t, restored.Len(), restored2.Len(), "repeated restores must be Len-stable")
}

// TestRestore_TTLExtrasAddedOnce — a TTL carrying triples the snapshot
// lacks gets them added exactly once (schema growth between boots).
func TestRestore_TTLExtrasAddedOnce(t *testing.T) {
	dir := t.TempDir()

	// Snapshot holds ONLY the two ABox triples.
	s := New()
	s.Insert(aboxTriple())
	s.Insert(Triple{S: ns + "note_001", P: ns + "referencesTable", O: ns + "tbl_etl_claim_t"})
	snapshotPath, ttlPath := writeRestoreFixturesWithStore(t, dir, s, testTTL)
	ttlN := testTTLLen(t)

	restored, ttlLoaded, err := Restore(snapshotPath, ttlPath)
	require.NoError(t, err)
	assert.Equal(t, ttlN, ttlLoaded)
	assert.Equal(t, ttlN+2, restored.Len(), "TBox extras added exactly once on top of the 2 ABox triples")

	// And a second restore from the SAME (unchanged) snapshot stays at the
	// same total — the merge is deterministic, not accumulative.
	restored2, _, err := Restore(snapshotPath, ttlPath)
	require.NoError(t, err)
	assert.Equal(t, restored.Len(), restored2.Len())
}

// TestRestore_MissingSnapshotTTLOnly — first-boot fallback: no snapshot →
// TTL-only build (Phase-38-compatible), and no error.
func TestRestore_MissingSnapshotTTLOnly(t *testing.T) {
	dir := t.TempDir()
	ttlN := testTTLLen(t)

	snapshotPath := filepath.Join(dir, "absent_skillforge_kg.json")
	ttlPath := filepath.Join(dir, "skillforge_tbox.ttl")
	require.NoError(t, os.WriteFile(ttlPath, []byte(testTTL), 0o644))

	restored, ttlLoaded, err := Restore(snapshotPath, ttlPath)
	require.NoError(t, err)
	assert.Equal(t, ttlN, ttlLoaded)
	assert.Equal(t, ttlN, restored.Len())
	assert.Empty(t, restored.Query("", ns+"referencesTable", ""), "no ABox in TTL-only mode")
}

// TestRestore_UnreadableSnapshotErrors — a snapshot that exists but cannot
// be read/parsed must FAIL the restore, never silently fall back to
// TTL-only (that fallback is the exact ABox wipe this function prevents).
func TestRestore_UnreadableSnapshotErrors(t *testing.T) {
	dir := t.TempDir()
	snapshotPath := filepath.Join(dir, "skillforge_kg.json")
	ttlPath := filepath.Join(dir, "skillforge_tbox.ttl")
	require.NoError(t, os.WriteFile(snapshotPath, []byte("{not json"), 0o644))
	require.NoError(t, os.WriteFile(ttlPath, []byte(testTTL), 0o644))

	_, _, err := Restore(snapshotPath, ttlPath)
	require.Error(t, err, "corrupt snapshot must error, not fall back to TTL-only")
}
