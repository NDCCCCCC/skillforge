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

// Package store implements a pure-Go in-memory RDF triple store with
// byS/byP/byO indices and JSON snapshot durability.
//
// Reference: spike 005d-b (memorymap-kg) — 215 triples load 7.71ms,
// query 0.00ms, ~43KB memory. No cgo, no external process (ONT-09).
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// Triple is one RDF triple in simplified, JSON-serializable form.
//
// S/P are full IRI strings; O is a full IRI string or a literal rendered
// as `"value"` (matching spike 005d-b's convention).
type Triple struct {
	S string `json:"s"`
	P string `json:"p"`
	O string `json:"o"`
}

// Store is a concurrency-safe in-memory triple store with three indices.
type Store struct {
	mu  sync.RWMutex
	all []Triple
	byS map[string][]int
	byP map[string][]int
	byO map[string][]int
}

// New returns an empty Store.
func New() *Store {
	return &Store{
		byS: map[string][]int{},
		byP: map[string][]int{},
		byO: map[string][]int{},
	}
}

// Insert adds a triple and updates all three indices. O(1).
// Duplicate triples are stored as-is — use InsertUnique when the caller
// cannot guarantee dedupe upstream (e.g. boot-time TBox→KG merges).
func (s *Store) Insert(t Triple) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := len(s.all)
	s.all = append(s.all, t)
	s.byS[t.S] = append(s.byS[t.S], idx)
	s.byP[t.P] = append(s.byP[t.P], idx)
	s.byO[t.O] = append(s.byO[t.O], idx)
}

// InsertUnique inserts t only if no identical triple (same S, P and O) is
// already stored, and reports whether the triple was inserted. The
// existence check and the append share one write-lock acquisition, so the
// check-then-insert sequence cannot race (no TOCTOU window): byS[t.S]
// holds the position of every stored candidate sharing the subject, and an
// exact match can only live among them.
//
// This is the dedupe guard for boot-time merges: replaying the TBox into a
// snapshot-restored KG must not double-count triples.
func (s *Store) InsertUnique(t Triple) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, idx := range s.byS[t.S] {
		if c := s.all[idx]; c.P == t.P && c.O == t.O {
			return false
		}
	}
	idx := len(s.all)
	s.all = append(s.all, t)
	s.byS[t.S] = append(s.byS[t.S], idx)
	s.byP[t.P] = append(s.byP[t.P], idx)
	s.byO[t.O] = append(s.byO[t.O], idx)
	return true
}

// DeleteSubject removes every triple whose subject equals subject and
// returns the number of triples removed. This is the note-level replace
// primitive (D-Boot-01): removing all triples of skf:note_{idx} before
// re-inserting a refreshed note, and pruning orphaned TableSchema rows.
//
// byS/byP/byO hold positions into s.all, so a partial compaction would
// corrupt the other two indices; the surviving triples are compacted in
// order and all three indices are rebuilt from scratch in the same pass.
// O(n) rebuild is fine at KG scale (<10k triples). Removing a subject that
// owns no triples mutates nothing and returns 0.
func (s *Store) DeleteSubject(subject string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]Triple, 0, len(s.all))
	removed := 0
	for _, t := range s.all {
		if t.S == subject {
			removed++
			continue
		}
		kept = append(kept, t)
	}
	if removed == 0 {
		return 0
	}
	s.all = kept
	s.byS = make(map[string][]int, len(s.byS))
	s.byP = make(map[string][]int, len(s.byP))
	s.byO = make(map[string][]int, len(s.byO))
	for idx, t := range s.all {
		s.byS[t.S] = append(s.byS[t.S], idx)
		s.byP[t.P] = append(s.byP[t.P], idx)
		s.byO[t.O] = append(s.byO[t.O], idx)
	}
	return removed
}

// Len returns the number of stored triples.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.all)
}

// Query returns triples matching the (s, p, o) pattern; empty string is a
// wildcard. The most selective bound component drives candidate lookup.
func (s *Store) Query(subj, pred, obj string) []Triple {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var cands []int
	switch {
	case subj != "":
		cands = s.byS[subj]
	case pred != "":
		cands = s.byP[pred]
	case obj != "":
		cands = s.byO[obj]
	default:
		cands = make([]int, len(s.all))
		for i := range cands {
			cands[i] = i
		}
	}
	var out []Triple
	for _, i := range cands {
		t := s.all[i]
		if subj != "" && t.S != subj {
			continue
		}
		if pred != "" && t.P != pred {
			continue
		}
		if obj != "" && t.O != obj {
			continue
		}
		out = append(out, t)
	}
	return out
}

// Adjacent1 returns deduplicated, sorted objects reachable from subj via pred
// (1-hop forward).
func (s *Store) Adjacent1(subj, pred string) []string {
	ts := s.Query(subj, pred, "")
	return dedupSortObjects(ts)
}

// Adjacent1Reverse returns deduplicated, sorted subjects that reach obj via
// pred (1-hop reverse).
func (s *Store) Adjacent1Reverse(pred, obj string) []string {
	ts := s.Query("", pred, obj)
	return dedupSortSubjects(ts)
}

// Adjacent2 returns deduplicated, sorted objects reachable from subj via
// pred1 → pred2 (2-hop forward).
func (s *Store) Adjacent2(subj, pred1, pred2 string) []string {
	mid := s.Adjacent1(subj, pred1)
	seen := map[string]bool{}
	out := []string{}
	for _, m := range mid {
		for _, t := range s.Query(m, pred2, "") {
			if !seen[t.O] {
				seen[t.O] = true
				out = append(out, t.O)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Adjacent2Reverse returns deduplicated, sorted subjects that reach obj via
// pred1 → pred2 (2-hop reverse: subject --pred1--> mid --pred2--> obj).
func (s *Store) Adjacent2Reverse(pred2, pred1, obj string) []string {
	mids := s.Adjacent1Reverse(pred2, obj)
	seen := map[string]bool{}
	out := []string{}
	for _, m := range mids {
		for _, t := range s.Query("", pred1, m) {
			if !seen[t.S] {
				seen[t.S] = true
				out = append(out, t.S)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Snapshot serializes the store to indented JSON.
func (s *Store) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(s.all, "", "  ")
}

// LoadSnapshot rebuilds a Store from a Snapshot-produced JSON document.
func LoadSnapshot(data []byte) (*Store, error) {
	var triples []Triple
	if err := json.Unmarshal(data, &triples); err != nil {
		return nil, fmt.Errorf("store: unmarshal snapshot: %w", err)
	}
	s := New()
	for _, t := range triples {
		s.Insert(t)
	}
	return s, nil
}

// SnapshotDebounced returns a trigger function; each call schedules a
// snapshot write to path after delay of quiet time. Rapid successive calls
// collapse into one write (the timer resets). Safe for concurrent use.
//
// D-KG-03: Phase 38 uses 5s; Phase 39's high-frequency ECL inserts merge
// naturally without extra logic.
func (s *Store) SnapshotDebounced(path string, delay time.Duration) func() {
	var mu sync.Mutex
	var timer *time.Timer
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(delay, func() {
			data, err := s.Snapshot()
			if err != nil {
				// WR-04: this debounced writer is the ONLY recurring
				// durability trigger for ECL ABox writes and induction
				// mutations — a silent failure turns every later write into
				// an unexplained no-op, and the triples since the last
				// successful snapshot are lost on the next crash/restart
				// with nothing in the operator's log to explain it. The
				// stderr WARN matches the manager's ledger-write posture.
				fmt.Fprintf(os.Stderr, "[WARN] ontology: debounced snapshot %s: marshal: %v\n", path, err)
				return
			}
			if err := atomicWriteFile(path, data); err != nil {
				fmt.Fprintf(os.Stderr, "[WARN] ontology: debounced snapshot %s: write: %v\n", path, err)
				return
			}
		})
	}
}

// WriteSnapshot persists an immediate snapshot to path (atomic: temp + rename).
func (s *Store) WriteSnapshot(path string) error {
	data, err := s.Snapshot()
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data)
}

// LoadFromTurtle parses a Turtle document into the store, returning the
// number of triples inserted. Literals render as `"value"`.
func (s *Store) LoadFromTurtle(ttl string) (int, error) {
	g := graph.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(ttl)); err != nil {
		return 0, fmt.Errorf("store: parse turtle: %w", err)
	}
	n := 0
	for tr := range g.Triples(nil, nil, nil) {
		s.Insert(Triple{
			S: subjectString(tr.Subject),
			P: tr.Predicate.Value(),
			O: objectString(tr.Object),
		})
		n++
	}
	return n, nil
}

// Stats reports index sizes for the admin KG stats endpoint.
type Stats struct {
	TripleCount int `json:"triple_count"`
	BySSize     int `json:"byS_size"`
	ByPSize     int `json:"byP_size"`
	ByOSize     int `json:"byO_size"`
	MemoryKB    int `json:"memory_kb_estimate"`
}

// Stats returns a point-in-time stats snapshot (ONT-06 admin surface).
func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Rough estimate mirroring spike 005d-b: avg 60 bytes × 3 strings + map overhead.
	memKB := (len(s.all)*3*60 + (len(s.byS)+len(s.byP)+len(s.byO))*32) / 1024
	return Stats{
		TripleCount: len(s.all),
		BySSize:     len(s.byS),
		ByPSize:     len(s.byP),
		ByOSize:     len(s.byO),
		MemoryKB:    memKB,
	}
}

// --- helpers ---

func dedupSortObjects(ts []Triple) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range ts {
		if !seen[t.O] {
			seen[t.O] = true
			out = append(out, t.O)
		}
	}
	sort.Strings(out)
	return out
}

func dedupSortSubjects(ts []Triple) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range ts {
		if !seen[t.S] {
			seen[t.S] = true
			out = append(out, t.S)
		}
	}
	sort.Strings(out)
	return out
}

func subjectString(sub term.Subject) string {
	switch v := sub.(type) {
	case term.URIRef:
		return v.Value()
	case term.BNode:
		return "_:" + v.Value()
	default:
		return sub.String()
	}
}

func objectString(obj term.Term) string {
	switch v := obj.(type) {
	case term.URIRef:
		return v.Value()
	case term.Literal:
		if vs, ok := v.Value().(string); ok {
			return `"` + vs + `"`
		}
		return fmt.Sprintf("%v", v.Value())
	case term.BNode:
		return "_:" + v.Value()
	default:
		return obj.String()
	}
}

// atomicWriteFile writes data to path via temp file + rename so a crash
// mid-write never corrupts an existing snapshot.
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".snapshot-*")
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
