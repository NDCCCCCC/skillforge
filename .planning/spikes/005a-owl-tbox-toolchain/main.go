// Spike 005a — validate the Go OWL/RDF toolchain (tggo/goRDFlib v0.1.0).
//
// Given a minimal OWL TBox file (Classes + ObjectProperties +
// DatatypeProperties + Restrictions), the spike loads it into a graph and
// prints the enumerated schema (entity types + relations + their domain/range).
//
// Acceptance criteria (from spike manifest):
//   (1) parser reads the .ttl file into a graph without error
//   (2) we can enumerate every owl:Class
//   (3) we can enumerate every owl:ObjectProperty with its domain/range
//   (4) we can enumerate every owl:DatatypeProperty with its domain/range
//   (5) we can enumerate every owl:Restriction of the form "P some D"
//   (6) the graph roundtrips: serialize back to .ttl and reload produces
//       identical triple count (no info loss)
package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// Well-known IRI constants (value-typed).
var (
	rdfType = term.MustURIRef("http://www.w3.org/1999/02/22-rdf-syntax-ns#type")
	rdfsDom = term.MustURIRef("http://www.w3.org/2000/01/rdf-schema#domain")
	rdfsRng = term.MustURIRef("http://www.w3.org/2000/01/rdf-schema#range")
	rdfsSub = term.MustURIRef("http://www.w3.org/2000/01/rdf-schema#subClassOf")
	rdfsLbl = term.MustURIRef("http://www.w3.org/2000/01/rdf-schema#label")
	owlCls  = term.MustURIRef("http://www.w3.org/2002/07/owl#Class")
	owlOP   = term.MustURIRef("http://www.w3.org/2002/07/owl#ObjectProperty")
	owlDP   = term.MustURIRef("http://www.w3.org/2002/07/owl#DatatypeProperty")
	owlRes  = term.MustURIRef("http://www.w3.org/2002/07/owl#Restriction")
	owlOnP  = term.MustURIRef("http://www.w3.org/2002/07/owl#onProperty")
	owlSV   = term.MustURIRef("http://www.w3.org/2002/07/owl#someValuesFrom")

	skfNS = "http://skillforge.local/ontology#"
)

var prefixes = []struct{ ns, p string }{
	{"http://www.w3.org/1999/02/22-rdf-syntax-ns#", "rdf:"},
	{"http://www.w3.org/2000/01/rdf-schema#", "rdfs:"},
	{"http://www.w3.org/2002/07/owl#", "owl:"},
	{"http://www.w3.org/2001/XMLSchema#", "xsd:"},
	{skfNS, "skf:"},
}

func compact(s string) string {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p.ns) {
			return p.p + s[len(p.ns):]
		}
	}
	return "<" + s + ">"
}

func termStr(t term.Term) string {
	if t == nil {
		return "<nil>"
	}
	switch v := t.(type) {
	case term.URIRef:
		return compact(v.Value())
	case term.Literal:
		if s, ok := v.Value().(string); ok {
			return fmt.Sprintf("\"%s\"", s)
		}
		return fmt.Sprintf("%v", v.Value())
	case term.BNode:
		return fmt.Sprintf("_:%s", v.Value())
	}
	return t.String()
}

// matchTriples returns triples matching (?, ?p?, ?o?) where predicate/object
// are matched only when non-nil and equal.
func matchTriples(g *graph.Graph, subj term.Subject, pred *term.URIRef, obj term.Term) []term.Triple {
	var out []term.Triple
	for tr := range g.Triples(subj, pred, obj) {
		out = append(out, tr)
	}
	return out
}

func typeSet(g *graph.Graph, cls term.URIRef) []term.URIRef {
	seen := map[string]term.URIRef{}
	for tr := range g.Triples(nil, &rdfType, cls) {
		if s, ok := tr.Subject.(term.URIRef); ok {
			seen[s.Value()] = s
		}
	}
	out := make([]term.URIRef, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value() < out[j].Value() })
	return out
}

func labelOf(g *graph.Graph, s term.Subject) string {
	for _, tr := range matchTriples(g, s, &rdfsLbl, nil) {
		if l, ok := tr.Object.(term.Literal); ok {
			if v, ok := l.Value().(string); ok {
				return v
			}
		}
	}
	return ""
}

func domainOf(g *graph.Graph, p term.URIRef) string {
	for _, tr := range matchTriples(g, p, &rdfsDom, nil) {
		return termStr(tr.Object)
	}
	return ""
}

func rangeOf(g *graph.Graph, p term.URIRef) string {
	for _, tr := range matchTriples(g, p, &rdfsRng, nil) {
		return termStr(tr.Object)
	}
	return ""
}

// restrictionsOf enumerates every blank node ?r where (?r a owl:Restriction)
// (?r owl:onProperty ?prop) and (?r owl:someValuesFrom ?cls).
func restrictionsOf(g *graph.Graph) []string {
	seen := map[string]term.Subject{}
	for tr := range g.Triples(nil, &rdfType, owlRes) {
		seen[termStr(tr.Subject)] = tr.Subject
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []string
	for _, k := range keys {
		s := seen[k]
		var onProp, svf term.Term
		for _, tr := range matchTriples(g, s, nil, nil) {
			if tr.Predicate.Equal(owlOnP) {
				onProp = tr.Object
			}
			if tr.Predicate.Equal(owlSV) {
				svf = tr.Object
			}
		}
		if onProp != nil && svf != nil {
			out = append(out, fmt.Sprintf("%s some %s", termStr(onProp), termStr(svf)))
		}
	}
	return out
}

func classHierarchy(g *graph.Graph) map[string][]string {
	hier := map[string][]string{}
	for _, c := range typeSet(g, owlCls) {
		var parents []string
		for _, tr := range matchTriples(g, c, &rdfsSub, nil) {
			parents = append(parents, termStr(tr.Object))
		}
		if len(parents) > 0 {
			sort.Strings(parents)
			hier[termStr(c)] = parents
		}
	}
	return hier
}

func loadTurtle(path string) (*graph.Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	g := graph.NewGraph()
	if err := turtle.Parse(g, f); err != nil {
		return nil, fmt.Errorf("turtle parse: %w", err)
	}
	return g, nil
}

func loadTurtleBytes(s string) (*graph.Graph, error) {
	g := graph.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(s)); err != nil {
		return nil, fmt.Errorf("reparse: %w", err)
	}
	return g, nil
}

func serializeTurtle(g *graph.Graph) (string, error) {
	var buf bytes.Buffer
	if err := turtle.Serialize(g, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func main() {
	const tboxPath = "example_tbox.ttl"

	fmt.Println("=== spike 005a: Go OWL TBox toolchain (tggo/goRDFlib v0.1.0) ===")

	g, err := loadTurtle(tboxPath)
	if err != nil {
		fmt.Println("[FAIL] load:", err)
		os.Exit(1)
	}
	n := g.Len()
	fmt.Printf("[1] parsed graph: %d triples from %s\n", n, tboxPath)

	// (2) enumerate owl:Class
	classes := typeSet(g, owlCls)
	fmt.Printf("[2] owl:Class count=%d\n", len(classes))
	for _, c := range classes {
		fmt.Printf("    - %-20s label=%q\n", termStr(c), labelOf(g, c))
	}

	// Class hierarchy
	hier := classHierarchy(g)
	keys := make([]string, 0, len(hier))
	for k := range hier {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println("[2.5] class hierarchy (subClassOf):")
	for _, k := range keys {
		fmt.Printf("    - %s subClassOf %v\n", k, hier[k])
	}

	// (3) enumerate owl:ObjectProperty
	objProps := typeSet(g, owlOP)
	fmt.Printf("[3] owl:ObjectProperty count=%d\n", len(objProps))
	for _, p := range objProps {
		fmt.Printf("    - %-20s domain=%-18s range=%-18s label=%q\n",
			termStr(p), domainOf(g, p), rangeOf(g, p), labelOf(g, p))
	}

	// (4) enumerate owl:DatatypeProperty
	dataProps := typeSet(g, owlDP)
	fmt.Printf("[4] owl:DatatypeProperty count=%d\n", len(dataProps))
	for _, p := range dataProps {
		fmt.Printf("    - %-20s domain=%-18s range=%-18s label=%q\n",
			termStr(p), domainOf(g, p), rangeOf(g, p), labelOf(g, p))
	}

	// (5) enumerate owl:Restriction
	restrs := restrictionsOf(g)
	fmt.Printf("[5] owl:Restriction count=%d\n", len(restrs))
	for _, r := range restrs {
		fmt.Printf("    - %s\n", r)
	}

	// (6) roundtrip
	serialized, err := serializeTurtle(g)
	if err != nil {
		fmt.Println("[FAIL] serialize:", err)
		os.Exit(1)
	}
	fmt.Printf("[6a] serialized back: %d triples, %d bytes\n", g.Len(), len(serialized))

	g2, err := loadTurtleBytes(serialized)
	if err != nil {
		fmt.Println("[FAIL] reparse:", err)
		os.Exit(1)
	}
	n3 := g2.Len()
	fmt.Printf("[6b] reparsed: %d triples\n", n3)
	if n == n3 {
		fmt.Println("[6c] roundtrip lossless: OK (n == n3)")
	} else {
		fmt.Printf("[6c] roundtrip LOSSY: n=%d n3=%d (FAIL)\n", n, n3)
		os.Exit(1)
	}

	// Verdict
	fmt.Println()
	allOK := len(classes) >= 4 && len(objProps) >= 3 && len(dataProps) >= 2 && n == n3
	if allOK {
		fmt.Println("=== VERDICT: VALIDATED — Go OWL TBox toolchain is viable ===")
	} else {
		fmt.Println("=== VERDICT: INVALIDATED — schema enumeration or roundtrip failed ===")
		os.Exit(1)
	}
}
