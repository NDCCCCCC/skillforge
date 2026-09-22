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
	"path/filepath"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/turtle"
)

const (
	rdfType = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
)

// loadGraph parses a Turtle string and returns the graph + triple count.
func loadGraph(t *testing.T, ttl string) (*graph.Graph, int) {
	t.Helper()
	g := graph.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(ttl)); err != nil {
		t.Fatalf("turtle.Parse: %v\nTTL head:\n%s", err, head(ttl, 400))
	}
	return g, g.Len()
}

func TestEmitSkillInstanceCount(t *testing.T) {
	root := projectRoot(t)
	e := New(filepath.Join(root, "skills"), filepath.Join(root, "knowledge"), DefaultNS)
	ttl, err := e.Emit()
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	g, n := loadGraph(t, ttl)
	t.Logf("emitted %d triples", n)
	skills := countByType(t, g, DefaultNS+"Skill")
	if skills != 8 {
		t.Errorf("Skill instances = %d, want 8", skills)
	}
}

func TestEmitTierInstanceCount(t *testing.T) {
	root := projectRoot(t)
	e := New(filepath.Join(root, "skills"), filepath.Join(root, "knowledge"), DefaultNS)
	ttl, err := e.Emit()
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	g, _ := loadGraph(t, ttl)
	tiers := countByType(t, g, DefaultNS+"KBCategory")
	if tiers != 5 {
		t.Errorf("KBCategory instances = %d, want 5", tiers)
	}
}

func TestEmitSubCategoryInstanceCount(t *testing.T) {
	root := projectRoot(t)
	e := New(filepath.Join(root, "skills"), filepath.Join(root, "knowledge"), DefaultNS)
	ttl, err := e.Emit()
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	g, _ := loadGraph(t, ttl)
	subs := countByType(t, g, DefaultNS+"SubCategory")
	if subs != 7 {
		t.Errorf("SubCategory instances = %d, want 7", subs)
	}
}

func TestEmitRoundtrip(t *testing.T) {
	root := projectRoot(t)
	e := New(filepath.Join(root, "skills"), filepath.Join(root, "knowledge"), DefaultNS)
	ttl1, err := e.Emit()
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	g1, n1 := loadGraph(t, ttl1)
	// Re-serialize → reparse.
	var buf bytes.Buffer
	if err := turtle.Serialize(g1, &buf); err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	_, n2 := loadGraph(t, buf.String())
	if n1 != n2 {
		t.Errorf("roundtrip triple count: %d → %d, want equal", n1, n2)
	}
}

func TestEmitNamespaceOverride(t *testing.T) {
	root := projectRoot(t)
	override := "https://override.example/onto#"
	e := New(filepath.Join(root, "skills"), filepath.Join(root, "knowledge"), override)
	ttl, err := e.Emit()
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	// Emit twice — same input must produce byte-identical output.
	ttl2, _ := e.Emit()
	if ttl != ttl2 {
		t.Errorf("emit not deterministic")
	}
	// The override namespace must appear in the @prefix declaration (the
	// emitter emits prefixed names like `skf:Skill`, not full IRIs).
	if !contains(ttl, "@prefix skf:  <https://override.example/onto#>") {
		t.Errorf("override namespace not applied; ttl snippet missing")
	}
	if contains(ttl, DefaultNS) {
		t.Errorf("default namespace still present in override output")
	}
}

func TestEmitSchemaCounts(t *testing.T) {
	root := projectRoot(t)
	e := New(filepath.Join(root, "skills"), filepath.Join(root, "knowledge"), DefaultNS)
	ttl, err := e.Emit()
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	g, _ := loadGraph(t, ttl)
	classes := 0
	objProps := 0
	dataProps := 0
	for tr := range g.Triples(nil, nil, nil) {
		obj := tr.Object.String()
		// Schema declarations use their own name as subject, not skill_/cat_/subcat_.
		subj := tr.Subject.String()
		if contains(subj, "#") && !contains(subj, "/skill_") && !contains(subj, "/cat_") && !contains(subj, "/subcat_") && !contains(subj, "/agent_") && !contains(subj, "/phrase_") {
			switch obj {
			case "http://www.w3.org/2002/07/owl#Class":
				classes++
			case "http://www.w3.org/2002/07/owl#ObjectProperty":
				objProps++
			case "http://www.w3.org/2002/07/owl#DatatypeProperty":
				dataProps++
			}
		}
	}
	if classes != 8 {
		t.Errorf("owl:Class = %d, want 8", classes)
	}
	if objProps != 8 {
		t.Errorf("owl:ObjectProperty = %d, want 8", objProps)
	}
	if dataProps != 8 {
		t.Errorf("owl:DatatypeProperty = %d, want 8", dataProps)
	}
}

// TestEmitECLSchemaDeclarations pins the D-TBox2-01 sentinel lines: the
// Phase 39 boot upgrade trigger keys on `skf:NoteChunk a owl:Class` — a
// persisted TBox lacking it is v1 and must be re-emitted.
func TestEmitECLSchemaDeclarations(t *testing.T) {
	root := projectRoot(t)
	e := New(filepath.Join(root, "skills"), filepath.Join(root, "knowledge"), DefaultNS)
	ttl, err := e.Emit()
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	for _, want := range []string{
		"skf:NoteChunk a owl:Class",
		"skf:TableSchema a owl:Class",
		"skf:belongsToNote a owl:ObjectProperty",
		"skf:referencesTable a owl:ObjectProperty",
	} {
		if !contains(ttl, want) {
			t.Errorf("emitted TTL missing sentinel %q", want)
		}
	}
	// Domain/range wiring for the two new object properties (exact blocks).
	for _, want := range []string{
		"skf:belongsToNote a owl:ObjectProperty ;\n    rdfs:domain skf:NoteChunk ;\n    rdfs:range skf:NoteChunk ;",
		"skf:referencesTable a owl:ObjectProperty ;\n    rdfs:domain skf:NoteChunk ;\n    rdfs:range skf:TableSchema ;",
	} {
		if !contains(ttl, want) {
			t.Errorf("emitted TTL missing property block:\n%s", want)
		}
	}
	// The emitter declares the ECL classes/properties but emits ZERO ABox
	// instances for them (ABox comes from the ECL pipeline; belongsToNote
	// stays declared-only this phase per open-question resolution #1).
	for _, forbidden := range []string{
		"a skf:NoteChunk",
		"a skf:TableSchema",
		" skf:belongsToNote skf:",
	} {
		if contains(ttl, forbidden) {
			t.Errorf("emitted TTL must not contain ABox usage %q", forbidden)
		}
	}
}

// --- helpers ---

func projectRoot(t *testing.T) string {
	t.Helper()
	// Tests run from package dir; project root is ../../../../
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("projectRoot: %v", err)
	}
	return root
}

// countByType returns the count of triples with `?s rdf:type <target>`,
// restricted to subjects whose name starts with the canonical instance prefix
// (skill_ / cat_ / subcat_). This excludes ObjectProperty/DatatypeProperty
// declarations that carry a `rdfs:domain skf:X` triple and would otherwise
// be double-counted.
func countByType(t *testing.T, g *graph.Graph, target string) int {
	t.Helper()
	prefixes := []string{"skill_", "cat_", "sub_"}
	count := 0
	for tr := range g.Triples(nil, nil, nil) {
		subj := tr.Subject.String()
		obj := tr.Object.String()
		if obj != target {
			continue
		}
		for _, p := range prefixes {
			if contains(subj, "#"+p) {
				count++
				break
			}
		}
	}
	return count
}

// head returns the first n bytes of s (for error diagnostics).
func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// contains reports whether sub appears in s.
func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
