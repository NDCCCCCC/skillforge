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
	"time"
)

const (
	ns      = "http://skillforge.local/ontology#"
	rdfType = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	testTTL = `@prefix skf: <http://skillforge.local/ontology#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .

skf:Skill a owl:Class ;
    rdfs:label "Skill" .

skf:skill_a a skf:Skill ;
    skf:skillName "a" ;
    skf:hasReadAccess skf:cat_authority .

skf:skill_b a skf:Skill ;
    skf:hasReadAccess skf:cat_authority .

skf:skill_c a skf:Skill ;
    skf:hasReadAccess skf:cat_historical .

skf:cat_authority a skf:KBCategory ;
    skf:belongsToTier skf:authority .
`
)

func TestStoreInsertAndLen(t *testing.T) {
	s := New()
	s.Insert(Triple{S: "a", P: "b", O: "c"})
	s.Insert(Triple{S: "a", P: "b", O: "d"})
	if s.Len() != 2 {
		t.Errorf("Len = %d, want 2", s.Len())
	}
}

func TestStoreQueryByType(t *testing.T) {
	s := New()
	if _, err := s.LoadFromTurtle(testTTL); err != nil {
		t.Fatalf("LoadFromTurtle: %v", err)
	}
	skills := s.Query("", rdfType, ns+"Skill")
	if len(skills) != 3 {
		t.Errorf("?x a skf:Skill = %d, want 3", len(skills))
	}
}

func TestStoreQueryWildcard(t *testing.T) {
	s := New()
	if _, err := s.LoadFromTurtle(testTTL); err != nil {
		t.Fatalf("LoadFromTurtle: %v", err)
	}
	all := s.Query("", "", "")
	if len(all) != s.Len() {
		t.Errorf("wildcard query = %d, want %d", len(all), s.Len())
	}
}

func TestStoreAdjacent1(t *testing.T) {
	s := New()
	if _, err := s.LoadFromTurtle(testTTL); err != nil {
		t.Fatalf("LoadFromTurtle: %v", err)
	}
	tiers := s.Adjacent1(ns+"cat_authority", ns+"belongsToTier")
	if len(tiers) != 1 || tiers[0] != ns+"authority" {
		t.Errorf("Adjacent1(cat_authority, belongsToTier) = %v, want [%s]", tiers, ns+"authority")
	}
}

func TestStoreAdjacent1Reverse(t *testing.T) {
	s := New()
	if _, err := s.LoadFromTurtle(testTTL); err != nil {
		t.Fatalf("LoadFromTurtle: %v", err)
	}
	readers := s.Adjacent1Reverse(ns+"hasReadAccess", ns+"cat_authority")
	if len(readers) != 2 {
		t.Errorf("Adjacency1Reverse hasReadAccess→cat_authority = %v, want 2 readers", readers)
	}
}

func TestStoreAdjacent2(t *testing.T) {
	s := New()
	if _, err := s.LoadFromTurtle(testTTL); err != nil {
		t.Fatalf("LoadFromTurtle: %v", err)
	}
	// skill_a --hasReadAccess--> cat_authority --belongsToTier--> authority
	out := s.Adjacent2(ns+"skill_a", ns+"hasReadAccess", ns+"belongsToTier")
	if len(out) != 1 || out[0] != ns+"authority" {
		t.Errorf("Adjacent2 = %v, want [%s]", out, ns+"authority")
	}
}

func TestStoreAdjacent2Reverse(t *testing.T) {
	s := New()
	if _, err := s.LoadFromTurtle(testTTL); err != nil {
		t.Fatalf("LoadFromTurtle: %v", err)
	}
	// ?s --hasReadAccess--> ?cat --belongsToTier--> authority
	out := s.Adjacent2Reverse(ns+"belongsToTier", ns+"hasReadAccess", ns+"authority")
	if len(out) != 2 {
		t.Errorf("Adjacent2Reverse = %v, want 2 skills", out)
	}
}

func TestStoreSnapshotRoundtrip(t *testing.T) {
	s := New()
	n, err := s.LoadFromTurtle(testTTL)
	if err != nil {
		t.Fatalf("LoadFromTurtle: %v", err)
	}
	data, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	s2, err := LoadSnapshot(data)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if s2.Len() != n {
		t.Errorf("roundtrip = %d triples, want %d", s2.Len(), n)
	}
	// Query equivalence both directions.
	for _, q := range [][3]string{
		{"", rdfType, ns + "Skill"},
		{ns + "skill_a", "", ""},
		{"", ns + "hasReadAccess", ""},
	} {
		a := s.Query(q[0], q[1], q[2])
		b := s2.Query(q[0], q[1], q[2])
		if len(a) != len(b) {
			t.Errorf("query %v diverged after roundtrip: %d vs %d", q, len(a), len(b))
		}
	}
}

func TestStoreLoadSpeed(t *testing.T) {
	// ONT-06: 215-triple-scale load < 100ms (spike measured 7.71ms).
	big := New()
	for i := 0; i < 215; i++ {
		big.Insert(Triple{S: ns + "s", P: ns + "p", O: ns + "o"})
	}
	data, _ := big.Snapshot()
	start := time.Now()
	if _, err := LoadSnapshot(data); err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("load 215 triples took %v, want < 100ms", d)
	}
}

func TestStoreWriteSnapshotAtomic(t *testing.T) {
	s := New()
	s.Insert(Triple{S: "a", P: "b", O: "c"})
	dir := t.TempDir()
	path := filepath.Join(dir, "kg.json")
	if err := s.WriteSnapshot(path); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	s2, err := LoadSnapshot(data)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if s2.Len() != 1 {
		t.Errorf("reloaded len = %d, want 1", s2.Len())
	}
}

func TestStoreDebounceBehavior(t *testing.T) {
	s := New()
	dir := t.TempDir()
	path := filepath.Join(dir, "kg.json")

	trigger := s.SnapshotDebounced(path, 100*time.Millisecond)

	// 10 rapid inserts + trigger each — must collapse to ONE write at the end.
	for i := 0; i < 10; i++ {
		s.Insert(Triple{S: ns + "s", P: ns + "p", O: ns + "o"})
		trigger()
		time.Sleep(5 * time.Millisecond)
	}

	// Before debounce window closes: no file yet.
	if _, err := os.Stat(path); err == nil {
		t.Errorf("snapshot written before debounce window closed")
	}

	// After the window: exactly one file, containing the final state.
	time.Sleep(250 * time.Millisecond)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("debounced snapshot missing: %v", err)
	}
	s2, err := LoadSnapshot(data)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if s2.Len() != 10 {
		t.Errorf("debounced snapshot has %d triples, want 10", s2.Len())
	}
}

func TestStoreStats(t *testing.T) {
	s := New()
	if _, err := s.LoadFromTurtle(testTTL); err != nil {
		t.Fatalf("LoadFromTurtle: %v", err)
	}
	st := s.Stats()
	if st.TripleCount != s.Len() {
		t.Errorf("Stats.TripleCount = %d, want %d", st.TripleCount, s.Len())
	}
	if st.BySSize == 0 || st.ByPSize == 0 || st.ByOSize == 0 {
		t.Errorf("Stats index sizes all zero: %+v", st)
	}
}
