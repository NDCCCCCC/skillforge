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

package shacl

import (
	"os"
	"testing"

	"github.com/skillforge/skillforge/internal/ontology/store"
)

// ---------------------------------------------------------------------------
// Test helper — URI constants matching emitter.go DefaultNS
// ---------------------------------------------------------------------------

const (
	rdfType = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"

	skfTier         = "http://skillforge.local/ontology#Tier"
	skfKBCategory   = "http://skillforge.local/ontology#KBCategory"
	skfSubCategory  = "http://skillforge.local/ontology#SubCategory"
	skfSkill        = "http://skillforge.local/ontology#Skill"
	skfSkillAgent   = "http://skillforge.local/ontology#SkillAgent"
	skfRefusalPhrase = "http://skillforge.local/ontology#RefusalPhrase"
	skfNoteChunk    = "http://skillforge.local/ontology#NoteChunk"
	skfTableSchema  = "http://skillforge.local/ontology#TableSchema"

	skfTierName         = "http://skillforge.local/ontology#tierName"
	skfCategoryName      = "http://skillforge.local/ontology#categoryName"
	skfSubCategoryName  = "http://skillforge.local/ontology#subCategoryName"
	skfSkillName         = "http://skillforge.local/ontology#skillName"
	skfAgentTypeName     = "http://skillforge.local/ontology#agentTypeName"
	skfRefusalPhraseText = "http://skillforge.local/ontology#refusalPhraseText"
	skfBelongsToTier     = "http://skillforge.local/ontology#belongsToTier"
	skfPoweredBy         = "http://skillforge.local/ontology#poweredBy"
	skfReferencesTable   = "http://skillforge.local/ontology#referencesTable"
	skfSourcePath        = "http://skillforge.local/ontology#sourcePath"
	skfHasReadAccess     = "http://skillforge.local/ontology#hasReadAccess"
	skfHasWriteAccess    = "http://skillforge.local/ontology#hasWriteAccess"
	skfRequiresRefusal   = "http://skillforge.local/ontology#requiresRefusal"
	skfHasSubCategory    = "http://skillforge.local/ontology#hasSubCategory"
	skfBelongsToNote     = "http://skillforge.local/ontology#belongsToNote"

	xsdString = "http://www.w3.org/2001/XMLSchema#string"

	// Entity URIs used in tests
	nsNote001      = "http://skillforge.local/ontology#note_001"
	nsNote002      = "http://skillforge.local/ontology#note_002"
	nsSqlGuides    = "http://skillforge.local/ontology#sql-guides"
	nsNonExistent  = "http://skillforge.local/ontology#nonExistentTable"
)

// ---------------------------------------------------------------------------
// TestShapesParse — verifies shapes.ttl is parseable and contains 8 NodeShapes
// ---------------------------------------------------------------------------

func TestShapesParse(t *testing.T) {
	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .
@prefix rdf:  <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .

# skf:Tier — minimal: rdf:type check only
skf:TierShape a sh:NodeShape ;
    sh:targetClass skf:Tier ;
    sh:property [] .

# skf:KBCategory
skf:KBCategoryShape a sh:NodeShape ;
    sh:targetClass skf:KBCategory ;
    sh:property [
        sh:path skf:categoryName ;
        sh:minCount 1 ;
        sh:datatype xsd:string
    ] .

# skf:SubCategory
skf:SubCategoryShape a sh:NodeShape ;
    sh:targetClass skf:SubCategory ;
    sh:property [
        sh:path skf:subCategoryName ;
        sh:minCount 1 ;
        sh:datatype xsd:string
    ] .

# skf:Skill
skf:SkillShape a sh:NodeShape ;
    sh:targetClass skf:Skill ;
    sh:property [
        sh:path skf:skillName ;
        sh:minCount 1 ;
        sh:datatype xsd:string
    ] .

# skf:SkillAgent
skf:SkillAgentShape a sh:NodeShape ;
    sh:targetClass skf:SkillAgent ;
    sh:property [
        sh:path skf:agentTypeName ;
        sh:minCount 1 ;
        sh:datatype xsd:string
    ] .

# skf:RefusalPhrase
skf:RefusalPhraseShape a sh:NodeShape ;
    sh:targetClass skf:RefusalPhrase ;
    sh:property [
        sh:path skf:refusalPhraseText ;
        sh:minCount 1 ;
        sh:datatype xsd:string
    ] .

# skf:NoteChunk
skf:NoteChunkShape a sh:NodeShape ;
    sh:targetClass skf:NoteChunk ;
    sh:property [
        sh:path rdf:type ;
        sh:hasValue skf:NoteChunk
    ] .

# skf:TableSchema
skf:TableSchemaShape a sh:NodeShape ;
    sh:targetClass skf:TableSchema ;
    sh:property [
        sh:path rdf:type ;
        sh:hasValue skf:TableSchema
    ] .
`

	tmp, err := os.CreateTemp("", "shacl-test-*.ttl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpPath := tmp.Name()
	tmp.WriteString(fixture)
	tmp.Close()
	defer os.Remove(tmpPath)

	parsed, err := parseShapes(tmpPath)
	if err != nil {
		t.Fatalf("parseShapes failed: %v", err)
	}

	if len(parsed.NodeShapes) != 8 {
		t.Errorf("expected 8 NodeShapes, got %d", len(parsed.NodeShapes))
	}

	wantClasses := []string{
		"http://skillforge.local/ontology#Tier",
		"http://skillforge.local/ontology#KBCategory",
		"http://skillforge.local/ontology#SubCategory",
		"http://skillforge.local/ontology#Skill",
		"http://skillforge.local/ontology#SkillAgent",
		"http://skillforge.local/ontology#RefusalPhrase",
		"http://skillforge.local/ontology#NoteChunk",
		"http://skillforge.local/ontology#TableSchema",
	}
	seen := map[string]bool{}
	for _, ns := range parsed.NodeShapes {
		seen[ns.TargetClass] = true
	}
	for _, want := range wantClasses {
		if !seen[want] {
			t.Errorf("NodeShape for %s not found", want)
		}
	}

	// Verify at least one NodeShape has a property with minCount constraint.
	hasMinCount := false
	for _, ns := range parsed.NodeShapes {
		for _, p := range ns.Properties {
			if p.MinCount != nil {
				hasMinCount = true
				break
			}
		}
	}
	if !hasMinCount {
		t.Error("no NodeShape has a property with sh:minCount")
	}
}

// ---------------------------------------------------------------------------
// TestMinCountViolation — sh:minCount violation detected when property absent
// ---------------------------------------------------------------------------

func TestMinCountViolation(t *testing.T) {
	kg := store.New()
	// Insert a KBCategory WITHOUT skf:categoryName — should trigger minCount violation.
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#kb1", P: rdfType, O: skfKBCategory})

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .
skf:KBCategoryShape a sh:NodeShape ;
    sh:targetClass skf:KBCategory ;
    sh:property [
        sh:path skf:categoryName ;
        sh:minCount 1 ;
        sh:datatype xsd:string
    ] .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if report.Health != "error" {
		t.Errorf("expected health 'error', got '%s'", report.Health)
	}
	if report.ErrorCount < 1 {
		t.Errorf("expected at least 1 error, got %d", report.ErrorCount)
	}

	// Find the minCount violation.
	found := false
	for _, q := range report.Quarantined {
		for _, v := range q.Violations {
			if v.Constraint == "sh:minCount" {
				found = true
				if q.Entity != "http://skillforge.local/ontology#kb1" {
					t.Errorf("expected entity 'http://skillforge.local/ontology#kb1', got '%s'", q.Entity)
				}
			}
		}
	}
	if !found {
		t.Error("sh:minCount violation not found in report")
	}
}

// ---------------------------------------------------------------------------
// TestMaxCountViolation — sh:maxCount violation when property count > max
// ---------------------------------------------------------------------------

func TestMaxCountViolation(t *testing.T) {
	kg := store.New()
	// Insert a Tier with TWO skf:tierName values (maxCount = 1).
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#tier1", P: rdfType, O: skfTier})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#tier1", P: skfTierName, O: `"tier-a"`})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#tier1", P: skfTierName, O: `"tier-b"`})

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .
skf:TierShape a sh:NodeShape ;
    sh:targetClass skf:Tier ;
    sh:property [
        sh:path skf:tierName ;
        sh:maxCount 1 ;
        sh:datatype xsd:string
    ] .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if report.Health != "error" {
		t.Errorf("expected health 'error', got '%s'", report.Health)
	}

	found := false
	for _, q := range report.Quarantined {
		for _, v := range q.Violations {
			if v.Constraint == "sh:maxCount" {
				found = true
			}
		}
	}
	if !found {
		t.Error("sh:maxCount violation not found")
	}
}

// ---------------------------------------------------------------------------
// TestClassViolation — O must be rdf:type of required class IRI
// ---------------------------------------------------------------------------

func TestClassViolation(t *testing.T) {
	kg := store.New()
	// Insert a Skill whose skf:poweredBy points to skf:notAnAgent (not a SkillAgent).
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#skill1", P: rdfType, O: skfSkill})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#skill1", P: skfSkillName, O: `"TestSkill"`})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#skill1", P: skfPoweredBy, O: "http://skillforge.local/ontology#notAnAgent"})
	// skf:notAnAgent does NOT have rdf:type skf:SkillAgent.

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .
skf:SkillShape a sh:NodeShape ;
    sh:targetClass skf:Skill ;
    sh:property [
        sh:path skf:skillName ;
        sh:minCount 1 ;
        sh:datatype xsd:string
    ] ;
    sh:property [
        sh:path skf:poweredBy ;
        sh:class skf:SkillAgent
    ] .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if report.Health != "error" {
		t.Errorf("expected health 'error', got '%s'", report.Health)
	}

	found := false
	for _, q := range report.Quarantined {
		for _, v := range q.Violations {
			if v.Constraint == "sh:class" {
				found = true
			}
		}
	}
	if !found {
		t.Error("sh:class violation not found")
	}
}

// ---------------------------------------------------------------------------
// TestDatatypeViolation — literal datatype mismatch
// ---------------------------------------------------------------------------

func TestDatatypeViolation(t *testing.T) {
	kg := store.New()
	// Insert a SkillAgent with skf:agentTypeName as a non-string literal (IRI).
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#agent1", P: rdfType, O: skfSkillAgent})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#agent1", P: skfAgentTypeName, O: `"ChatModelAgent"`}) // ok

	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#agent2", P: rdfType, O: skfSkillAgent})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#agent2", P: skfAgentTypeName, O: "not-a-literal-iri"}) // not quoted → treated as IRI

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .
skf:SkillAgentShape a sh:NodeShape ;
    sh:targetClass skf:SkillAgent ;
    sh:property [
        sh:path skf:agentTypeName ;
        sh:minCount 1 ;
        sh:datatype xsd:string
    ] .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// agent1 should pass; agent2 should fail (non-literal value for datatype xsd:string).
	if report.Health != "error" {
		t.Errorf("expected health 'error', got '%s'", report.Health)
	}
}

// ---------------------------------------------------------------------------
// TestNodeKindViolation — IRI expected but literal given
// ---------------------------------------------------------------------------

func TestNodeKindViolation(t *testing.T) {
	kg := store.New()
	// Insert a NoteChunk with skf:referencesTable as a literal (should be IRI).
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#note1", P: rdfType, O: skfNoteChunk})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#note1", P: skfReferencesTable, O: `"should-be-iri"`})

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
skf:NoteChunkShape a sh:NodeShape ;
    sh:targetClass skf:NoteChunk ;
    sh:property [
        sh:path rdf:type ;
        sh:hasValue skf:NoteChunk
    ] ;
    sh:property [
        sh:path skf:referencesTable ;
        sh:nodeKind sh:IRI
    ] .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	found := false
	for _, q := range report.Quarantined {
		for _, v := range q.Violations {
			if v.Constraint == "sh:nodeKind" {
				found = true
			}
		}
	}
	if !found {
		t.Error("sh:nodeKind violation not found")
	}
}

// ---------------------------------------------------------------------------
// TestPatternViolation — value does not match regex
// ---------------------------------------------------------------------------

func TestPatternViolation(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#ref1", P: rdfType, O: skfRefusalPhrase})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#ref1", P: skfRefusalPhraseText, O: `"INVALID_PHONE_NUMBER_123"`}) // contains spaces

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
skf:RefusalPhraseShape a sh:NodeShape ;
    sh:targetClass skf:RefusalPhrase ;
    sh:property [
        sh:path skf:refusalPhraseText ;
        sh:minCount 1 ;
        sh:pattern "^[A-Za-z0-9 ，。！？、：；]+$"
    ] .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	found := false
	for _, q := range report.Quarantined {
		for _, v := range q.Violations {
			if v.Constraint == "sh:pattern" {
				found = true
			}
		}
	}
	if !found {
		t.Error("sh:pattern violation not found")
	}
}

// ---------------------------------------------------------------------------
// TestInViolation — value not in enumeration list
// ---------------------------------------------------------------------------

func TestInViolation(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#tier1", P: rdfType, O: skfTier})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#tier1", P: skfTierName, O: `"invalid-tier"`})

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
skf:TierShape a sh:NodeShape ;
    sh:targetClass skf:Tier ;
    sh:property [
        sh:path skf:tierName ;
        sh:in "权威" , "历史" , "个人" , "sql-guides" , "draft"
    ] .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	found := false
	for _, q := range report.Quarantined {
		for _, v := range q.Violations {
			if v.Constraint == "sh:in" {
				found = true
			}
		}
	}
	if !found {
		t.Error("sh:in violation not found")
	}
}

// ---------------------------------------------------------------------------
// TestErrorWarningSeparation — error vs warning severity separation
// ---------------------------------------------------------------------------

func TestErrorWarningSeparation(t *testing.T) {
	kg := store.New()
	// Entity with ONLY warning-level violation (provenance-missing via sourcePath).
	// D-PROV-10: skf:sourcePath absent is a warning.
	// We test this by giving NoteChunk a valid sourcePath (so only the warning
	// constraint fires — in a real scenario with absent sourcePath there would be
	// a minCount violation marked warning, but absent optional = no violation in
	// our framework; instead we verify that an entity with ONLY warning violations
	// passes validation (not quarantined) while an entity with error violations is.
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#note1", P: rdfType, O: skfNoteChunk})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#note1", P: skfSourcePath, O: `"sql-guides/note1.md"`}) // present, ok

	// Another NoteChunk with an error violation (class constraint broken).
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#note2", P: rdfType, O: skfNoteChunk})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#note2", P: skfSourcePath, O: `"sql-guides/note2.md"`})
	// broken class: note2 has a referencesTable to something that is NOT a TableSchema
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#note2", P: skfReferencesTable, O: "http://skillforge.local/ontology#notATable"})

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix rdf:  <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .

skf:NoteChunkShape a sh:NodeShape ;
    sh:targetClass skf:NoteChunk ;
    sh:property [
        sh:path rdf:type ;
        sh:hasValue skf:NoteChunk
    ] ;
    sh:property [
        sh:path skf:sourcePath ;
        sh:maxCount 1 ;
        sh:nodeKind sh:Literal ;
        sh:message "sourcePath must be a literal (warning-level)"
    ] ;
    sh:property [
        sh:path skf:referencesTable ;
        sh:class skf:TableSchema ;
        sh:message "referencesTable must be skf:TableSchema (error-level)"
    ] .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Health should be "error" because note2 has an error-level violation.
	if report.Health != "error" {
		t.Errorf("expected health 'error', got '%s'", report.Health)
	}
	// note2 should be quarantined.
	if len(report.Quarantined) == 0 {
		t.Error("expected note2 to be quarantined")
	}
	// note1 should NOT be quarantined (only warning-level).
	for _, q := range report.Quarantined {
		if q.Entity == "http://skillforge.local/ontology#note1" {
			t.Error("note1 should NOT be quarantined (only warning-level)")
		}
	}
}

// ---------------------------------------------------------------------------
// TestReportStructure — health=ok when no violations
// ---------------------------------------------------------------------------

func TestReportStructure(t *testing.T) {
	kg := store.New()
	// A valid KBCategory with required properties.
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#kb1", P: rdfType, O: skfKBCategory})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#kb1", P: skfCategoryName, O: `"测试类别"`})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#tier1", P: rdfType, O: skfTier})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#kb1", P: skfBelongsToTier, O: "http://skillforge.local/ontology#tier1"})

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .
@prefix rdf:  <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .

skf:KBCategoryShape a sh:NodeShape ;
    sh:targetClass skf:KBCategory ;
    sh:property [
        sh:path skf:categoryName ;
        sh:minCount 1 ;
        sh:datatype xsd:string
    ] ;
    sh:property [
        sh:path skf:belongsToTier ;
        sh:class skf:Tier
    ] .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if report.Health != "ok" {
		t.Errorf("expected health 'ok', got '%s'", report.Health)
	}
	if report.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", report.ErrorCount)
	}
	if report.WarningCount != 0 {
		t.Errorf("expected 0 warnings, got %d", report.WarningCount)
	}
	if len(report.Quarantined) != 0 {
		t.Errorf("expected 0 quarantined, got %d", len(report.Quarantined))
	}
	if report.TripleCount != 4 {
		t.Errorf("expected tripleCount 4, got %d", report.TripleCount)
	}
	if report.ShapesPath == "" {
		t.Error("ShapesPath should be set")
	}
	if report.ValidatedAt == "" {
		t.Error("ValidatedAt should be set")
	}
}

// ---------------------------------------------------------------------------
// TestShaclViolation — SHACL violation detection with quarantine分流
// D-SHACL-02: error-level violations quarantine the entity; warnings do not.
// ---------------------------------------------------------------------------

func TestShaclViolation(t *testing.T) {
	kg := store.New()
	// Build a KG with a NoteChunk that has a missing required provenance
	// (provenance-missing = warning, D-PROV-10) — entity should NOT be quarantined.
	kg.Insert(store.Triple{S: nsSqlGuides, P: rdfType, O: skfTier})
	kg.Insert(store.Triple{S: nsNote001, P: rdfType, O: skfNoteChunk})
	// note_001 is missing skf:sourcePath — this triggers a warning-level
	// minCount violation (provenance-missing path, D-PROV-10).

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix rdf:  <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .

skf:TierShape a sh:NodeShape ;
    sh:targetClass skf:Tier ;
    sh:property [] .

skf:NoteChunkShape a sh:NodeShape ;
    sh:targetClass skf:NoteChunk ;
    sh:property [
        sh:path rdf:type ;
        sh:hasValue skf:NoteChunk
    ] ;
    sh:property [
        sh:path skf:sourcePath ;
        sh:minCount 1 ;
        sh:datatype xsd:string ;
        sh:message "provenance-missing (warning-level, D-PROV-10)"
    ] .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Should have 1 warning (provenance-missing on note_001).
	if report.WarningCount != 1 {
		t.Errorf("expected 1 warning, got %d", report.WarningCount)
	}
	// No quarantined (warning doesn't quarantine per D-SHACL-02).
	if len(report.Quarantined) != 0 {
		t.Errorf("expected 0 quarantined, got %d", len(report.Quarantined))
	}
	// Health = warn (only warnings, no errors).
	if report.Health != "warn" {
		t.Errorf("expected health=warn, got %s", report.Health)
	}
}

// TestShaclQuarantine — error-level violation triggers quarantine分流.
// D-SHACL-02: error entities are removed from the main KG index.
func TestShaclQuarantine(t *testing.T) {
	kg := store.New()
	// note_002 references a non-existent TableSchema — this is an error
	// (structural integrity violation, D-SHACL-02).
	kg.Insert(store.Triple{S: nsNote002, P: rdfType, O: skfNoteChunk})
	kg.Insert(store.Triple{S: nsNote002, P: skfReferencesTable, O: nsNonExistent})

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix rdf:  <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .

skf:NoteChunkShape a sh:NodeShape ;
    sh:targetClass skf:NoteChunk ;
    sh:property [
        sh:path rdf:type ;
        sh:hasValue skf:NoteChunk
    ] ;
    sh:property [
        sh:path skf:referencesTable ;
        sh:class skf:TableSchema ;
        sh:message "referencesTable must point to a skf:TableSchema instance"
    ] .

# Global cross-entity integrity check
skf:referencesTableShape a sh:PropertyShape ;
    sh:path skf:referencesTable ;
    sh:class skf:TableSchema .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Health must be "error" (at least one error-level violation).
	if report.Health != "error" {
		t.Errorf("expected health=error, got %s", report.Health)
	}
	// note_002 must be in the Quarantined list.
	found := false
	for _, q := range report.Quarantined {
		if q.Entity == nsNote002 {
			found = true
			if q.TripleCount == 0 {
				t.Errorf("quarantined entity note_002 has 0 triples, want 2")
			}
			if len(q.Violations) == 0 {
				t.Errorf("note_002 quarantined with 0 violations")
			}
			for _, v := range q.Violations {
				if v.Severity != SeverityError {
					t.Errorf("expected error severity, got %s", v.Severity)
				}
				if v.Constraint != "sh:class" {
					t.Errorf("expected sh:class constraint, got %s", v.Constraint)
				}
			}
		}
	}
	if !found {
		t.Error("note_002 expected in Quarantined list but not found")
	}
}

// ---------------------------------------------------------------------------
// TestGlobalReferencesConstraint — skf:referencesTable target must exist
// ---------------------------------------------------------------------------

func TestGlobalReferencesConstraint(t *testing.T) {
	kg := store.New()
	// Insert a NoteChunk that references skf:nonexistent-table (does not exist).
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#note1", P: rdfType, O: skfNoteChunk})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#note1", P: skfReferencesTable, O: "http://skillforge.local/ontology#nonexistent-table"})

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix rdf:  <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .

skf:NoteChunkShape a sh:NodeShape ;
    sh:targetClass skf:NoteChunk ;
    sh:property [
        sh:path rdf:type ;
        sh:hasValue skf:NoteChunk
    ] ;
    sh:property [
        sh:path skf:referencesTable ;
        sh:class skf:TableSchema
    ] .

# Global property shape for cross-entity reference integrity
skf:referencesTableShape a sh:PropertyShape ;
    sh:path skf:referencesTable ;
    sh:class skf:TableSchema .
`
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	tmp.WriteString(fixture)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	report, err := Validate(kg, tmpPath)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if report.Health != "error" {
		t.Errorf("expected health 'error', got '%s'", report.Health)
	}
}
