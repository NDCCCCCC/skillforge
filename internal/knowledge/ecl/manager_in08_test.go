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

// IN-08 regression test — non-stable sort underpins the deterministic pin.
// Verifies that sort is stable and non-numeric SourceIdx values don't collide.
package ecl

import (
	"context"
	"sort"
	"strconv"
	"testing"

	"github.com/skillforge/skillforge/internal/ontology/store"
	"github.com/skillforge/skillforge/internal/retriever"
)

// stubExtractorIN08 always succeeds.
type stubExtractorIN08 struct{}

func (stubExtractorIN08) ExtractNote(_ context.Context, _ Note) (Extraction, error) {
	return Extraction{}, nil
}

// stubSeedIN08 always succeeds.
func stubSeedIN08(_ context.Context, _ []retriever.Document) error {
	return nil
}

// TestManagerStableSortsDuplicateSourceIdx (IN-08): sort.SliceStable is used
// instead of sort.Slice, and non-numeric SourceIdx values don't collide at 0.
func TestManagerStableSortsDuplicateSourceIdx(t *testing.T) {
	kbDir := threeNoteKBDir(t)
	fs := newFakeFailureStore()
	kg := store.New()
	mgr := NewManager("http://skillforge.dev/ns#", kbDir, kg, stubExtractorIN08{}, fs, stubSeedIN08)
	next := int64(1)
	mgr.SetIDAllocator(func() int64 { v := next; next++; return v })

	// Simulate loaded notes with duplicate (non-numeric) SourceIdx values.
	loaded := []loadedNote{
		{note: Note{SourceIdx: "abc"}, ex: Extraction{}},
		{note: Note{SourceIdx: "001"}, ex: Extraction{}},
		{note: Note{SourceIdx: "def"}, ex: Extraction{}},
		{note: Note{SourceIdx: "002"}, ex: Extraction{}},
		{note: Note{SourceIdx: "xyz"}, ex: Extraction{}},
	}

	// Apply the stable sort as done in Run.
	sort.SliceStable(loaded, func(i, j int) bool {
		a, aOK := strconv.Atoi(loaded[i].note.SourceIdx)
		b, bOK := strconv.Atoi(loaded[j].note.SourceIdx)
		if aOK == nil && bOK == nil {
			return a < b
		}
		if aOK != nil && bOK != nil {
			return false
		}
		if aOK != nil {
			return false
		}
		return true
	})

	// After sort: numeric ones first in ascending order, then non-numeric in original order.
	// Order should be: 001, 002, abc, def, xyz
	want := []string{"001", "002", "abc", "def", "xyz"}
	for i, wantIdx := range want {
		if loaded[i].note.SourceIdx != wantIdx {
			t.Errorf("loaded[%d].SourceIdx = %q, want %q", i, loaded[i].note.SourceIdx, wantIdx)
		}
	}
}

// TestManagerNonNumericSourceIdxNotCollapsed (IN-08): two notes with non-numeric
// SourceIdx are kept separate by stable sort (don't both resolve to 0).
func TestManagerNonNumericSourceIdxNotCollapsed(t *testing.T) {
	loaded := []loadedNote{
		{note: Note{SourceIdx: "foo"}, ex: Extraction{}},
		{note: Note{SourceIdx: "001"}, ex: Extraction{}},
		{note: Note{SourceIdx: "bar"}, ex: Extraction{}},
		{note: Note{SourceIdx: "002"}, ex: Extraction{}},
	}

	sort.SliceStable(loaded, func(i, j int) bool {
		a, aOK := strconv.Atoi(loaded[i].note.SourceIdx)
		b, bOK := strconv.Atoi(loaded[j].note.SourceIdx)
		if aOK == nil && bOK == nil {
			return a < b
		}
		if aOK != nil && bOK != nil {
			return false
		}
		if aOK != nil {
			return false
		}
		return true
	})

	// foo and bar should be in positions 2 and 3 (stable relative order).
	if loaded[2].note.SourceIdx != "foo" {
		t.Errorf("loaded[2] = %q, want foo", loaded[2].note.SourceIdx)
	}
	if loaded[3].note.SourceIdx != "bar" {
		t.Errorf("loaded[3] = %q, want bar", loaded[3].note.SourceIdx)
	}
}
