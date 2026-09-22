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
	"fmt"
	"os"
)

// Restore rebuilds the KG store at boot in the D-KG-02 order — snapshot
// FIRST, then a deduplicated TBox merge:
//
//  1. Snapshot present: LoadSnapshot(snapshotPath) restores the full KG
//     (TBox triples AND any ABox the ECL pipeline wrote since the last
//     boot), then the current TBox TTL is parsed into a temporary store and
//     every triple is InsertUnique'd into the restored one. Insert does NOT
//     dedupe (store.go) — replaying the TBox without the guard would
//     double-count every TBox triple on every boot.
//  2. Snapshot absent (first boot / Phase-38-compatible behavior): fresh
//     New() + LoadFromTurtle only.
//
// This function is the Pitfall 3 fix (RESEARCH 39): the pre-Phase-39 boot
// built the KG from the TBox pointer alone and then OVERWROTE the snapshot,
// silently wiping every ABox triple on every restart. Restoring
// snapshot-first makes ABox durable; the caller's subsequent WriteSnapshot
// then persists the MERGED store (snapshot == merged state again).
//
// Returns the store and the number of triples loaded FROM THE TTL (the
// boot log's continuity metric with the pre-Phase-39 LoadFromTurtle line —
// NOT the store's total length). Errors are returned, never swallowed: an
// unreadable snapshot must fail the boot loudly rather than silently fall
// back to a TTL-only rebuild (that fallback IS the ABox wipe this function
// exists to prevent).
func Restore(snapshotPath, ttlPath string) (*Store, int, error) {
	ttlBytes, err := os.ReadFile(ttlPath)
	if err != nil {
		return nil, 0, fmt.Errorf("store: restore: read TBox TTL %s: %w", ttlPath, err)
	}

	_, statErr := os.Stat(snapshotPath)
	if os.IsNotExist(statErr) {
		// First boot: no snapshot yet — TTL-only (Phase-38-compatible).
		s := New()
		n, lerr := s.LoadFromTurtle(string(ttlBytes))
		if lerr != nil {
			return nil, 0, fmt.Errorf("store: restore: load TBox into fresh store: %w", lerr)
		}
		return s, n, nil
	}
	if statErr != nil {
		return nil, 0, fmt.Errorf("store: restore: stat snapshot %s: %w", snapshotPath, statErr)
	}

	snapBytes, rerr := os.ReadFile(snapshotPath)
	if rerr != nil {
		return nil, 0, fmt.Errorf("store: restore: read snapshot %s: %w", snapshotPath, rerr)
	}
	s, lerr := LoadSnapshot(snapBytes)
	if lerr != nil {
		return nil, 0, fmt.Errorf("store: restore: load snapshot: %w", lerr)
	}

	// Dedupe-guarded TBox merge: parse into a TEMP store (enumeration
	// needs a store; the TTL parse count is what we report), then fold its
	// triples into the restored one via InsertUnique.
	temp := New()
	n, terr := temp.LoadFromTurtle(string(ttlBytes))
	if terr != nil {
		return nil, 0, fmt.Errorf("store: restore: parse TBox for merge: %w", terr)
	}
	for _, t := range temp.Query("", "", "") {
		s.InsertUnique(t)
	}
	return s, n, nil
}
