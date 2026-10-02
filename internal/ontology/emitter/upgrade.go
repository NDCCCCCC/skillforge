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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/skillforge/skillforge/internal/ontology/induction"
)

// noteChunkSentinel is the byte marker that identifies a Phase 39 (8-class)
// TBox: the literal `skf:NoteChunk a owl:Class` line the render loop emits
// for the D-TBox2-01 classes. Its presence in the pointer TTL is the
// ONE-TIME boot upgrade trigger (Plan 39-06 / RESEARCH Pitfall 4) — Plan
// 39-01 pinned this exact string as the cross-plan contract.
const noteChunkSentinel = "skf:NoteChunk a owl:Class"

// pointerFileName / versionFileName / tboxVersionFmt name the on-disk
// artifacts UpgradeIfNeeded manages. They mirror the Phase 38 boot layout
// (cmd/server main.go) and induction.Manager's own file discipline — the
// SAME names, so the upgrader and the approve pipeline stay interoperable
// on one dataDir.
const (
	pointerFileName     = "skillforge_tbox.ttl"
	versionFileName     = "tbox_version.txt"
	tboxVersionFmt      = "skillforge_tbox.v%d.ttl"
	firstTBoxVersion    = 1
	noteChunkDataPropV1 = 1 // version assigned on first boot
)

// UpgradeIfNeeded reconciles the on-disk TBox with the current emitter
// schema. It is the boot-time entry point (cmd/server calls it BEFORE the
// KG restore) and encodes the three legal states:
//
//  1. Pointer missing (fresh deploy / first boot): emit the current TBox
//     as v1 + pointer + version file — preserves Phase 38 first-boot
//     behavior byte-for-byte. Returns (true, 1, nil).
//
//  2. Pointer present AND contains the skf:NoteChunk sentinel: the TBox
//     already carries the Phase 39 classes — NO-OP. Returns
//     (false, currentVersion, nil). This is the anti-Pitfall-4 branch:
//     the pointer short-circuit exists so approved inductions and versioned
//     history survive restarts, so an up-to-date environment must NEVER
//     re-emit, even when skills/ or knowledge/ trees changed underneath.
//
//  3. Pointer present WITHOUT the sentinel (an environment that booted
//     Phase 38 with the 6-class TBox): ONE-TIME upgrade. Reads the current
//     version N, emits a fresh full TBox (8 classes), replays every
//     approved induction proposal's triples (induction.ApprovedTriples —
//     the human-approved mutations of the old TBox must not be silently
//     dropped by a re-emit), writes skillforge_tbox.v{N+1}.ttl, swaps the
//     pointer content, and bumps tbox_version.txt to N+1. Returns
//     (true, N+1, nil).
//
// There is deliberately NO unconditional re-emit path (RESEARCH 39
// anti-pattern: "Re-emitting the TBox unconditionally at boot drops
// induction-approved triples and rewrites versioned history").
//
// All writes are atomic (temp file + rename, mirroring induction's
// writeFileAtomic discipline). Errors are RETURNED, never log.Fatal'd —
// the caller (cmd/server) owns logging and fatal posture.
func UpgradeIfNeeded(dataDir, ns, skillsDir, kbDir string) (upgraded bool, version int, err error) {
	pointer := filepath.Join(dataDir, pointerFileName)
	_, statErr := os.Stat(pointer)
	if os.IsNotExist(statErr) {
		// State 1: fresh boot — current v1 emit path.
		ttlOut, emitErr := New(skillsDir, kbDir, ns).Emit()
		if emitErr != nil {
			return false, 0, fmt.Errorf("emitter: upgrade: v1 emit: %w", emitErr)
		}
		if werr := writeFileAtomic(filepath.Join(dataDir, fmt.Sprintf(tboxVersionFmt, firstTBoxVersion)), []byte(ttlOut)); werr != nil {
			return false, 0, fmt.Errorf("emitter: upgrade: write v1: %w", werr)
		}
		if werr := writeFileAtomic(pointer, []byte(ttlOut)); werr != nil {
			return false, 0, fmt.Errorf("emitter: upgrade: write pointer: %w", werr)
		}
		if werr := writeFileAtomic(filepath.Join(dataDir, versionFileName), []byte(fmt.Sprintf("%d\n", noteChunkDataPropV1))); werr != nil {
			return false, 0, fmt.Errorf("emitter: upgrade: write version file: %w", werr)
		}
		return true, noteChunkDataPropV1, nil
	}
	if statErr != nil {
		return false, 0, fmt.Errorf("emitter: upgrade: stat pointer %s: %w", pointer, statErr)
	}

	pointerBytes, rerr := os.ReadFile(pointer)
	if rerr != nil {
		return false, 0, fmt.Errorf("emitter: upgrade: read pointer %s: %w", pointer, rerr)
	}
	if bytes.Contains(pointerBytes, []byte(noteChunkSentinel)) {
		// State 2: up to date — never re-emit (Pitfall 4's no-op branch).
		return false, readVersion(dataDir), nil
	}

	// State 3: Phase-38 environment — one-time N → N+1 upgrade.
	current := readVersion(dataDir)
	next := current + 1
	ttlOut, emitErr := New(skillsDir, kbDir, ns).Emit()
	if emitErr != nil {
		return false, current, fmt.Errorf("emitter: upgrade: v%d emit: %w", next, emitErr)
	}
	fresh, ferr := appendApprovedTriples(ttlOut, dataDir)
	if ferr != nil {
		return false, current, ferr
	}
	if werr := writeFileAtomic(filepath.Join(dataDir, fmt.Sprintf(tboxVersionFmt, next)), []byte(fresh)); werr != nil {
		return false, current, fmt.Errorf("emitter: upgrade: write v%d: %w", next, werr)
	}
	if werr := writeFileAtomic(pointer, []byte(fresh)); werr != nil {
		return false, current, fmt.Errorf("emitter: upgrade: swap pointer: %w", werr)
	}
	if werr := writeFileAtomic(filepath.Join(dataDir, versionFileName), []byte(fmt.Sprintf("%d\n", next))); werr != nil {
		return false, current, fmt.Errorf("emitter: upgrade: bump version file: %w", werr)
	}
	return true, next, nil
}

// appendApprovedTriples normalizes the fresh TTL text and appends every
// approved induction proposal's Turtle lines (the replay that keeps
// human-approved schema mutations alive across the one-time upgrade).
func appendApprovedTriples(ttl, dataDir string) (string, error) {
	approved, err := induction.ApprovedTriples(dataDir)
	if err != nil {
		return "", fmt.Errorf("emitter: upgrade: read approved proposals: %w", err)
	}
	if len(approved) == 0 {
		return ttl, nil
	}
	var b strings.Builder
	b.WriteString(ttl)
	if !strings.HasSuffix(ttl, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n# === replayed approved induction proposals (TBox upgrade) ===\n")
	for _, line := range approved {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// readVersion parses tbox_version.txt with induction.Manager's semantics:
// missing or unparsable file → version 1 (the Phase 38 default).
func readVersion(dataDir string) int {
	data, err := os.ReadFile(filepath.Join(dataDir, versionFileName))
	if err != nil {
		return firstTBoxVersion
	}
	var v int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &v); err != nil || v < firstTBoxVersion {
		return firstTBoxVersion
	}
	return v
}

// writeFileAtomic writes via temp + rename (mirrors induction.writeFileAtomic
// and store.atomicWriteFile — one write discipline across the ontology
// packages so a crash mid-write never corrupts a TBox artifact).
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".emitter-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
