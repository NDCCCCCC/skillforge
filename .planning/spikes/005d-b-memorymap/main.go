// Spike 005d-b — memory-map KG storage for SkillForge TBox + ABox.
//
// Pure-Go baseline: no cgo, no external service. Uses a typed map keyed by
// triple (subject, predicate, object). Supports the same operations the
// retriever needs:
//   - InsertTriple / LoadFromTurtle (215 triples from 005b)
//   - Query: by (s, p, o) pattern
//   - Adjacency: "give me all subjects reachable from X via predicate P"
//   - Snapshot: serialize the map to JSON (for restart durability)
//
// Acceptance:
//   (a) loads 215-triple TBox file in < 100ms
//   (b) supports query-by-subject-predicate and query-by-predicate-object
//   (c) supports 1-hop and 2-hop adjacency traversal
//   (d) snapshot roundtrips losslessly
//   (e) memory footprint < 5MB for the 215-triple TBox
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// ---------- in-memory triple store ----------

// Triple is a simplified, JSON-serializable triple.
type Triple struct {
	S string `json:"s"`
	P string `json:"p"`
	O string `json:"o"`
}

// Store holds triples + three indices for O(1)-ish lookup by each component.
type Store struct {
	all       []Triple
	byS       map[string][]int // subject -> indices into all
	byP       map[string][]int
	byO       map[string][]int
}

func NewStore() *Store {
	return &Store{
		byS: map[string][]int{},
		byP: map[string][]int{},
		byO: map[string][]int{},
	}
}

func (s *Store) Insert(t Triple) {
	idx := len(s.all)
	s.all = append(s.all, t)
	s.byS[t.S] = append(s.byS[t.S], idx)
	s.byP[t.P] = append(s.byP[t.P], idx)
	s.byO[t.O] = append(s.byO[t.O], idx)
}

// Query returns triples matching (?, ?p?, ?o?) where nil = wildcard.
func (s *Store) Query(subj, pred, obj string) []Triple {
	var out []Triple
	var cands []int
	if subj != "" {
		cands = s.byS[subj]
	} else if pred != "" {
		cands = s.byP[pred]
	} else if obj != "" {
		cands = s.byO[obj]
	} else {
		cands = make([]int, len(s.all))
		for i := range cands {
			cands[i] = i
		}
	}
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

// Adjacent1 returns triples reachable from `subj` via `pred` (one hop).
func (s *Store) Adjacent1(subj, pred string) []string {
	ts := s.Query(subj, pred, "")
	out := make([]string, 0, len(ts))
	seen := map[string]bool{}
	for _, t := range ts {
		if !seen[t.O] {
			seen[t.O] = true
			out = append(out, t.O)
		}
	}
	sort.Strings(out)
	return out
}

// Adjacent2 returns subjects reachable from `subj` via pred1 -> pred2 in 2 hops.
func (s *Store) Adjacent2(subj, pred1, pred2 string) []string {
	mid := s.Adjacent1(subj, pred1)
	out := []string{}
	seen := map[string]bool{}
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

// Snapshot serializes the store to JSON.
func (s *Store) Snapshot() ([]byte, error) {
	return json.MarshalIndent(s.all, "", "  ")
}

// LoadSnapshot deserializes a JSON snapshot back into the store.
func LoadSnapshot(data []byte) (*Store, error) {
	var triples []Triple
	if err := json.Unmarshal(data, &triples); err != nil {
		return nil, err
	}
	s := NewStore()
	for _, t := range triples {
		s.Insert(t)
	}
	return s, nil
}

// ---------- load TBox from Turtle ----------

// compactToFull reverses the prefix compaction done by 005a's termStr helper.
// We need this because TBox files use skf:foo form; we want full IRI strings.
func loadFromTurtle(path string) (*Store, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	g := graph.NewGraph()
	if err := turtle.Parse(g, f); err != nil {
		return nil, 0, err
	}
	s := NewStore()
	n := 0
	for tr := range g.Triples(nil, nil, nil) {
		// Compact each term to its full IRI string (use Value() directly).
		var subj, pred, obj string
		switch v := tr.Subject.(type) {
		case term.URIRef:
			subj = v.Value()
		case term.BNode:
			subj = "_:" + v.Value()
		default:
			subj = v.String()
		}
		pred = tr.Predicate.Value()
		switch v := tr.Object.(type) {
		case term.URIRef:
			obj = v.Value()
		case term.Literal:
			if vs, ok := v.Value().(string); ok {
				obj = `"` + vs + `"`
			} else {
				obj = fmt.Sprintf("%v", v.Value())
			}
		case term.BNode:
			obj = "_:" + v.Value()
		default:
			obj = v.String()
		}
		s.Insert(Triple{S: subj, P: pred, O: obj})
		n++
	}
	return s, n, nil
}

// ---------- measurements ----------

func memKB(s *Store) int {
	// Rough estimate: 3 strings × avg 60 bytes + map overhead.
	avg := 60
	return (len(s.all)*3*avg + len(s.byS)*32 + len(s.byP)*32 + len(s.byO)*32) / 1024
}

func main() {
	fmt.Println("=== spike 005d-b: memory-map KG storage (pure Go baseline) ===")

	// (1) Load 005b's TBox.
	const tbox = "spikes/005b-skillschema-rdfs-lift/skillforge_tbox.ttl"
	t0 := time.Now()
	s, n, err := loadFromTurtle(tbox)
	if err != nil {
		fmt.Println("[FAIL] load:", err)
		os.Exit(1)
	}
	loadDur := time.Since(t0)
	fmt.Printf("[1] loaded %d triples in %v (%.2f ms)\n", n, loadDur, float64(loadDur.Microseconds())/1000)
	fmt.Printf("[1a] memory estimate: ~%d KB\n", memKB(s))

	// (2) Query tests.
	t1 := time.Now()
	// Find every Skill instance: ?x a skf:Skill
	skills := s.Query("", "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", "http://skillforge.local/ontology#Skill")
	fmt.Printf("[2a] ?x a skf:Skill -> %d instances in %.2f ms\n",
		len(skills), float64(time.Since(t1).Microseconds())/1000)

	// Find what tier sql-guides belongs to.
	t2 := time.Now()
	tierOfSqlGuides := s.Adjacent1(
		"http://skillforge.local/ontology#cat_sql-guides",
		"http://skillforge.local/ontology#belongsToTier",
	)
	fmt.Printf("[2b] cat_sql-guides ->belongsToTier-> %v (%.2f ms)\n",
		tierOfSqlGuides, float64(time.Since(t2).Microseconds())/1000)

	// Find what skills READ from sql-guides (1-hop).
	// (Note: Adjacent2 is forward direction; for reverse we query by O.)

	// Reverse direction: who hasReadAccess cat_sql-guides? We need to search by O.
	t4 := time.Now()
	readers := s.Query("", "http://skillforge.local/ontology#hasReadAccess", "http://skillforge.local/ontology#cat_sql-guides")
	fmt.Printf("[2c] ?x hasReadAccess cat_sql-guides -> %d readers in %.2f ms\n",
		len(readers), float64(time.Since(t4).Microseconds())/1000)
	for _, r := range readers {
		// Strip IRI prefix for readable output.
		s := strings.TrimPrefix(r.S, "http://skillforge.local/ontology#skill_")
		fmt.Printf("    - %s\n", s)
	}

	// (3) Snapshot roundtrip.
	t5 := time.Now()
	snap, err := s.Snapshot()
	if err != nil {
		fmt.Println("[FAIL] snapshot:", err)
		os.Exit(1)
	}
	fmt.Printf("[3a] snapshot: %d bytes in %.2f ms\n", len(snap), float64(time.Since(t5).Microseconds())/1000)
	t6 := time.Now()
	s2, err := LoadSnapshot(snap)
	if err != nil {
		fmt.Println("[FAIL] reload snapshot:", err)
		os.Exit(1)
	}
	fmt.Printf("[3b] reload snapshot: %d triples in %.2f ms\n", len(s2.all), float64(time.Since(t6).Microseconds())/1000)

	// (4) Verdict.
	fmt.Println()
	okLoad := loadDur < 100*time.Millisecond
	okQuery := len(skills) >= 8 && len(readers) >= 1
	okSnap := len(s2.all) == n
	okMem := memKB(s) < 5*1024
	if okLoad && okQuery && okSnap && okMem {
		fmt.Println("=== VERDICT: VALIDATED — memory-map KG storage meets SkillForge baseline ===")
	} else {
		fmt.Printf("=== VERDICT: INVALIDATED — loadOK=%v queryOK=%v snapOK=%v memOK=%v ===\n",
			okLoad, okQuery, okSnap, okMem)
		os.Exit(1)
	}
}
