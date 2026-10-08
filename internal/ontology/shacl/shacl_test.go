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
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// D-46-03/D-46-04 constants: ABox business entities (D-NZK-04/05) and
	// the dangling-edge predicate family (abox.go:66-82, TBox v7).
	skfInsuranceProduct = "http://skillforge.local/ontology#InsuranceProduct"
	skfOrganization     = "http://skillforge.local/ontology#Organization"

	rdfsLabel                   = "http://www.w3.org/2000/01/rdf-schema#label"
	skfParentOrganization       = "http://skillforge.local/ontology#parentOrganization"
	skfInvolvesInsuranceProduct = "http://skillforge.local/ontology#involvesInsuranceProduct"
	skfInvolvesOrganization     = "http://skillforge.local/ontology#involvesOrganization"

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
        sh:severity sh:Warning ;
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

// ---------------------------------------------------------------------------
// writeTempShapes — temp-file fixture helper for the D-46-01/D-46-02 test
// group (os.CreateTemp("", "shacl-*.ttl") idiom shared with the tests above).
// ---------------------------------------------------------------------------

func writeTempShapes(t *testing.T, fixture string) string {
	t.Helper()
	tmp, err := os.CreateTemp("", "shacl-*.ttl")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if _, err := tmp.WriteString(fixture); err != nil {
		tmp.Close()
		t.Fatalf("write fixture: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp: %v", err)
	}
	t.Cleanup(func() { os.Remove(tmp.Name()) })
	return tmp.Name()
}

// ---------------------------------------------------------------------------
// TestShapesPaths — SKILLFORGE_SHACL_SHAPES path-list semantics (D-46-02).
// Covers VALIDATION Test Map SHACL-01 rows: env three-state (empty / single /
// multi) + default-list content + missing-path fail-closed (Pitfall 4/5:
// SplitList("") is [] not [""], and a missing path must never be silently
// skipped).
// ---------------------------------------------------------------------------

func TestShapesPaths(t *testing.T) {
	t.Run("empty env falls back to default two-file list", func(t *testing.T) {
		t.Setenv("SKILLFORGE_SHACL_SHAPES", "")
		got := ShapesPathsFromEnv()
		if len(got) != 2 {
			t.Fatalf("expected 2 default paths, got %d: %v", len(got), got)
		}
		if !strings.HasSuffix(got[0], "shapes.ttl") {
			t.Errorf("default[0] should end with shapes.ttl, got %q", got[0])
		}
		if !strings.HasSuffix(got[1], "entities.ttl") {
			t.Errorf("default[1] should end with entities.ttl, got %q", got[1])
		}
	})

	t.Run("single path env returns one-element list", func(t *testing.T) {
		one := filepath.Join(t.TempDir(), "custom.ttl")
		t.Setenv("SKILLFORGE_SHACL_SHAPES", one)
		got := ShapesPathsFromEnv()
		if len(got) != 1 || got[0] != one {
			t.Fatalf("expected [%s], got %v", one, got)
		}
	})

	t.Run("multi-path env splits on OS list separator", func(t *testing.T) {
		dir := t.TempDir()
		a := filepath.Join(dir, "a.ttl")
		b := filepath.Join(dir, "b.ttl")
		// Construct the env value with the OS list separator — never a
		// hard-coded ":" or ";" (the dual of filepath.SplitList).
		t.Setenv("SKILLFORGE_SHACL_SHAPES", strings.Join([]string{a, b}, string(filepath.ListSeparator)))
		got := ShapesPathsFromEnv()
		if len(got) != 2 || got[0] != a || got[1] != b {
			t.Fatalf("expected [%s %s], got %v", a, b, got)
		}
	})

	t.Run("missing single path fails validation", func(t *testing.T) {
		kg := store.New()
		missing := filepath.Join(t.TempDir(), "missing.ttl")
		report, err := Validate(kg, missing)
		if err == nil {
			t.Fatalf("expected error for missing shapes path %s, got report %+v", missing, report)
		}
	})

	t.Run("missing path in multi-file list fails whole validation", func(t *testing.T) {
		kg := store.New()
		kg.Insert(store.Triple{S: "http://skillforge.local/ontology#kb1", P: rdfType, O: skfKBCategory})
		kg.Insert(store.Triple{S: "http://skillforge.local/ontology#kb1", P: skfCategoryName, O: `"测试类别"`})

		valid := writeTempShapes(t, `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .

skf:KBCategoryShape a sh:NodeShape ;
    sh:targetClass skf:KBCategory ;
    sh:property [
        sh:path skf:categoryName ;
        sh:minCount 1 ;
        sh:datatype xsd:string
    ] .
`)
		missing := filepath.Join(t.TempDir(), "missing.ttl")
		// Pitfall 5: one bad path fails the WHOLE validation — a
		// half-populated shape set (missing path silently skipped) would
		// validate against incomplete rules.
		report, err := Validate(kg, valid, missing)
		if err == nil {
			t.Fatalf("expected error when one path in the list is missing, got report %+v", report)
		}
	})
}

// ---------------------------------------------------------------------------
// TestSeverity — sh:severity three-value mapping (D-46-01, W3C SHACL §2.1.4).
// Covers VALIDATION Test Map SHACL-02 row: sh:Warning / sh:Info → warning
// (allowed through, 0 quarantined, health=warn); unspecified / sh:Violation →
// error (quarantined, health=error). Severity is read at PropertyShape
// granularity; the engine derives nothing from the property path.
// ---------------------------------------------------------------------------

func TestSeverity(t *testing.T) {
	// One KBCategory individual missing its required skf:categoryName —
	// the sh:minCount 1 constraint fires under every severity variant.
	// Literals are quoted per the :204-206 KG-construction idiom.
	kg := store.New()
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#kb1", P: rdfType, O: skfKBCategory})

	fixtureFmt := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .

skf:KBCategoryShape a sh:NodeShape ;
    sh:targetClass skf:KBCategory ;
    sh:property [
        sh:path skf:categoryName ;
        sh:minCount 1 ;
        sh:datatype xsd:string ;
        %s
        sh:message "severity probe (D-46-01)"
    ] .
`

	cases := []struct {
		name              string
		severityDirective string
		wantQuarantined   bool
		wantHealth        string
	}{
		{name: "sh:Warning passes through unquarantined", severityDirective: "sh:severity sh:Warning ;", wantQuarantined: false, wantHealth: "warn"},
		{name: "sh:Info passes through unquarantined", severityDirective: "sh:severity sh:Info ;", wantQuarantined: false, wantHealth: "warn"},
		{name: "unspecified defaults to error", severityDirective: "", wantQuarantined: true, wantHealth: "error"},
		{name: "sh:Violation is error", severityDirective: "sh:severity sh:Violation ;", wantQuarantined: true, wantHealth: "error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTempShapes(t, fmt.Sprintf(fixtureFmt, tc.severityDirective))
			report, err := Validate(kg, path)
			if err != nil {
				t.Fatalf("Validate failed: %v", err)
			}
			if report.Health != tc.wantHealth {
				t.Errorf("expected health %q, got %q", tc.wantHealth, report.Health)
			}
			if tc.wantQuarantined {
				if report.ErrorCount < 1 {
					t.Errorf("expected at least 1 error, got %d", report.ErrorCount)
				}
				found := false
				for _, q := range report.Quarantined {
					if q.Entity == "http://skillforge.local/ontology#kb1" {
						found = true
					}
				}
				if !found {
					t.Error("expected kb1 in Quarantined (error-level violation)")
				}
			} else {
				if report.ErrorCount != 0 {
					t.Errorf("expected 0 errors, got %d", report.ErrorCount)
				}
				if report.WarningCount != 1 {
					t.Errorf("expected exactly 1 warning, got %d", report.WarningCount)
				}
				if len(report.Quarantined) != 0 {
					t.Errorf("expected 0 quarantined, got %d", len(report.Quarantined))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestSeveritySourcePathRetired — retirement nail for the D-PROV-10 engine
// special case (D-46-01). A sh:path skf:sourcePath PropertyShape WITHOUT an
// explicit sh:severity must produce an ERROR-level violation: behaviour is
// decided by shapes data alone, never by the engine's former path-name
// hard-coding (path==sourcePath → warning).
// ---------------------------------------------------------------------------

func TestSeveritySourcePathRetired(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#note1", P: rdfType, O: skfNoteChunk})

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .

skf:NoteChunkShape a sh:NodeShape ;
    sh:targetClass skf:NoteChunk ;
    sh:property [
        sh:path skf:sourcePath ;
        sh:minCount 1 ;
        sh:datatype xsd:string ;
        sh:message "sourcePath missing — no explicit severity declared"
    ] .
`
	path := writeTempShapes(t, fixture)
	report, err := Validate(kg, path)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if report.Health != "error" {
		t.Errorf("expected health=error (sourcePath engine special case retired), got %s", report.Health)
	}
	if report.ErrorCount < 1 {
		t.Errorf("expected at least 1 error, got %d", report.ErrorCount)
	}
	found := false
	for _, q := range report.Quarantined {
		if q.Entity == "http://skillforge.local/ontology#note1" {
			found = true
		}
	}
	if !found {
		t.Error("expected note1 quarantined (unspecified severity on a sourcePath shape = error)")
	}
}

// ---------------------------------------------------------------------------
// writeTempEntities — entity-shapes fixture helper (46-02 test group). The
// fixture lands in a temp dir under the EXACT name entities.ttl: the engine
// derives the violation layer from the file basename (D-46-02), so a
// randomly named file would silently flip layer assertions to tbox.
// ---------------------------------------------------------------------------

func writeTempEntities(t *testing.T, fixture string) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "shacl-entity-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "entities.ttl")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatalf("write entities.ttl fixture: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// TestViolationLayer — three-layer violation stamping (D-46-03).
// VALIDATION Test Map rows 46-02-T1/T3: tbox violations carry Layer=tbox +
// the NodeShape's target class; entity violations carry Layer=entity
// (fixture MUST be named entities.ttl — layer derives from the basename);
// global cross-entity violations carry LayerGlobal with an EMPTY
// TargetClass (omitempty → absent in JSON = FE dashed-badge sentinel,
// 46-UI-SPEC §Component 2).
// ---------------------------------------------------------------------------

func TestViolationLayer(t *testing.T) {
	t.Run("tbox layer stamped from non-entities shapes file", func(t *testing.T) {
		kg := store.New()
		kg.Insert(store.Triple{S: nsNote001, P: rdfType, O: skfNoteChunk})

		fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .

skf:NoteChunkShape a sh:NodeShape ;
    sh:targetClass skf:NoteChunk ;
    sh:property [
        sh:path skf:sourcePath ;
        sh:minCount 1 ;
        sh:datatype xsd:string ;
        sh:message "sourcePath missing (error-level, unspecified severity)"
    ] .
`
		// writeTempShapes → random basename ≠ entities.ttl → LayerTBox.
		path := writeTempShapes(t, fixture)
		report, err := Validate(kg, path)
		if err != nil {
			t.Fatalf("Validate failed: %v", err)
		}
		stamped := false
		for _, q := range report.Quarantined {
			for _, v := range q.Violations {
				if v.Layer != LayerTBox {
					t.Errorf("expected layer %q, got %q", LayerTBox, v.Layer)
				}
				if v.TargetClass != skfNoteChunk {
					t.Errorf("expected target_class %q, got %q", skfNoteChunk, v.TargetClass)
				}
				stamped = true
			}
		}
		if !stamped {
			t.Error("expected at least one stamped violation (note_001 missing sourcePath)")
		}
	})

	t.Run("entity layer stamped from entities.ttl basename", func(t *testing.T) {
		kg := store.New()
		kg.Insert(store.Triple{S: "http://skillforge.local/ontology#prod1", P: rdfType, O: skfInsuranceProduct})

		fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

skf:InsuranceProductShape a sh:NodeShape ;
    sh:targetClass skf:InsuranceProduct ;
    sh:property [
        sh:path rdfs:label ;
        sh:minCount 1 ;
        sh:message "skf:InsuranceProduct must have at least one rdfs:label"@en
    ] .
`
		// File name must be exactly entities.ttl — basename-derived layer.
		path := writeTempEntities(t, fixture)
		report, err := Validate(kg, path)
		if err != nil {
			t.Fatalf("Validate failed: %v", err)
		}
		stamped := false
		for _, q := range report.Quarantined {
			for _, v := range q.Violations {
				if v.Layer != LayerEntity {
					t.Errorf("expected layer %q, got %q", LayerEntity, v.Layer)
				}
				if v.TargetClass != skfInsuranceProduct {
					t.Errorf("expected target_class %q, got %q", skfInsuranceProduct, v.TargetClass)
				}
				stamped = true
			}
		}
		if !stamped {
			t.Error("expected at least one stamped violation (prod1 missing label)")
		}
	})

	t.Run("global sentinel has no target class", func(t *testing.T) {
		kg := store.New()
		kg.Insert(store.Triple{S: nsNote001, P: rdfType, O: skfNoteChunk})
		kg.Insert(store.Triple{S: nsNote001, P: skfReferencesTable, O: nsNonExistent})

		fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .

# Global cross-entity integrity PropertyShape only — no NodeShape here.
skf:referencesTableShape a sh:PropertyShape ;
    sh:path skf:referencesTable ;
    sh:class skf:TableSchema .
`
		path := writeTempShapes(t, fixture)
		report, err := Validate(kg, path)
		if err != nil {
			t.Fatalf("Validate failed: %v", err)
		}
		stamped := false
		for _, q := range report.Quarantined {
			for _, v := range q.Violations {
				if v.Layer != LayerGlobal {
					t.Errorf("expected layer %q, got %q", LayerGlobal, v.Layer)
				}
				// Sentinel semantics: global violations belong to no single
				// NodeShape class — TargetClass stays "" (omitempty drops it
				// in JSON, which is the FE's no-chip signal).
				if v.TargetClass != "" {
					t.Errorf("expected empty target_class (global sentinel), got %q", v.TargetClass)
				}
				stamped = true
			}
		}
		if !stamped {
			t.Error("expected at least one global violation (referencesTable → non-existent TableSchema)")
		}
	})
}

// ---------------------------------------------------------------------------
// TestDanglingEdges — dangling_edge_count per quarantined entity (D-46-04).
// VALIDATION Test Map rows 46-02-T1/T3: incoming edges across the
// involvesInsuranceProduct / involvesOrganization / parentOrganization
// family are counted per quarantined entity (reverse byO lookup, A4 timing:
// counted during validation, BEFORE any quarantine), and an isolated
// entity with zero in-edges reports 0 (field has no omitempty — BE always
// ships it).
// ---------------------------------------------------------------------------

func TestDanglingEdges(t *testing.T) {
	kg := store.New()
	prodX := "http://skillforge.local/ontology#prod_x"
	orgY := "http://skillforge.local/ontology#org_y"
	orgZ := "http://skillforge.local/ontology#org_z"

	// All three entities lack rdfs:label → each violates the entities.ttl
	// minCount and is quarantined; only their in-edge counts differ.
	kg.Insert(store.Triple{S: prodX, P: rdfType, O: skfInsuranceProduct})
	kg.Insert(store.Triple{S: orgY, P: rdfType, O: skfOrganization})
	kg.Insert(store.Triple{S: orgZ, P: rdfType, O: skfOrganization})
	kg.Insert(store.Triple{S: nsNote001, P: rdfType, O: skfNoteChunk})
	// In-edges: note_001 → prodX, note_001 → orgY, orgZ → orgY.
	kg.Insert(store.Triple{S: nsNote001, P: skfInvolvesInsuranceProduct, O: prodX})
	kg.Insert(store.Triple{S: nsNote001, P: skfInvolvesOrganization, O: orgY})
	kg.Insert(store.Triple{S: orgZ, P: skfParentOrganization, O: orgY})

	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

skf:InsuranceProductShape a sh:NodeShape ;
    sh:targetClass skf:InsuranceProduct ;
    sh:property [
        sh:path rdfs:label ;
        sh:minCount 1 ;
        sh:message "label required"@en
    ] .

skf:OrganizationShape a sh:NodeShape ;
    sh:targetClass skf:Organization ;
    sh:property [
        sh:path rdfs:label ;
        sh:minCount 1 ;
        sh:message "label required"@en
    ] .
`
	path := writeTempEntities(t, fixture)
	report, err := Validate(kg, path)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	wantDangling := map[string]int{
		prodX: 1, // note_001 involvesInsuranceProduct
		orgY:  2, // note_001 involvesOrganization + orgZ parentOrganization
		orgZ:  0, // no in-edges — count must still be reported (no omitempty)
	}
	seen := map[string]bool{}
	for _, q := range report.Quarantined {
		want, tracked := wantDangling[q.Entity]
		if !tracked {
			t.Errorf("unexpected quarantined entity %s", q.Entity)
			continue
		}
		seen[q.Entity] = true
		if q.DanglingEdgeCount != want {
			t.Errorf("%s: expected DanglingEdgeCount %d, got %d", q.Entity, want, q.DanglingEdgeCount)
		}
	}
	for entity := range wantDangling {
		if !seen[entity] {
			t.Errorf("expected %s in Quarantined (all three lack labels)", entity)
		}
	}
}

// ---------------------------------------------------------------------------
// TestEntitiesShapesParse — the shipped entities.ttl asset (D-46-01).
// VALIDATION Test Map row 46-02-T2: exactly 2 NodeShapes
// (InsuranceProduct/Organization); type PropertyShape hasValue, label
// minCount 1 with NO datatype constraint (Pitfall 2), tier/parent
// if-present class checks with no minCount; every PropertyShape at error
// severity; zero entityCode PropertyShapes (zero-constraint deviation).
// Then the constructive first-run-green assertion: a KG of legal entities
// validates with health=ok, 0 errors, 0 quarantined (hermetic — no live
// snapshot counts, Pitfall 7).
// ---------------------------------------------------------------------------

func TestEntitiesShapesParse(t *testing.T) {
	// go test runs with cwd = package dir, so the relative path resolves.
	parsed, err := parseShapes("entities.ttl")
	if err != nil {
		t.Fatalf("parseShapes(entities.ttl) failed: %v", err)
	}
	if len(parsed.NodeShapes) != 2 {
		t.Fatalf("expected exactly 2 NodeShapes, got %d", len(parsed.NodeShapes))
	}

	byTarget := map[string]nodeShape{}
	for _, s := range parsed.NodeShapes {
		byTarget[s.TargetClass] = s
	}
	product, ok := byTarget[skfInsuranceProduct]
	if !ok {
		t.Fatalf("expected a NodeShape targeting skf:InsuranceProduct, got %d shapes", len(byTarget))
	}
	org, ok := byTarget[skfOrganization]
	if !ok {
		t.Fatalf("expected a NodeShape targeting skf:Organization, got %d shapes", len(byTarget))
	}

	propByPath := func(s nodeShape, path string) *propertyConstraint {
		for i := range s.Properties {
			if s.Properties[i].Path == path {
				return &s.Properties[i]
			}
		}
		return nil
	}

	// type PropertyShape: hasValue matches the target class.
	if p := propByPath(product, rdfType); p == nil || p.HasValue != skfInsuranceProduct {
		t.Errorf("InsuranceProduct rdf:type shape missing or wrong hasValue: %+v", p)
	}
	if p := propByPath(org, rdfType); p == nil || p.HasValue != skfOrganization {
		t.Errorf("Organization rdf:type shape missing or wrong hasValue: %+v", p)
	}

	// label PropertyShape: minCount 1, and NO datatype constraint (Pitfall 2).
	for name, s := range map[string]nodeShape{"InsuranceProduct": product, "Organization": org} {
		p := propByPath(s, rdfsLabel)
		if p == nil {
			t.Fatalf("%s label PropertyShape missing", name)
		}
		if p.MinCount == nil || *p.MinCount != 1 {
			t.Errorf("%s label: expected MinCount 1, got %v", name, p.MinCount)
		}
		if p.Datatype != "" {
			t.Errorf("%s label: expected no datatype constraint (Pitfall 2), got %q", name, p.Datatype)
		}
	}

	// tier PropertyShape: if-present class check against skf:Tier, no minCount.
	if p := propByPath(product, skfBelongsToTier); p == nil || p.Class != skfTier || p.MinCount != nil {
		t.Errorf("InsuranceProduct tier shape wrong: %+v", p)
	}
	if p := propByPath(org, skfBelongsToTier); p == nil || p.Class != skfTier || p.MinCount != nil {
		t.Errorf("Organization tier shape wrong: %+v", p)
	}

	// parent PropertyShape: Organization only, class skf:Organization, no minCount.
	if p := propByPath(product, skfParentOrganization); p != nil {
		t.Errorf("InsuranceProduct must not constrain parentOrganization: %+v", p)
	}
	if p := propByPath(org, skfParentOrganization); p == nil || p.Class != skfOrganization || p.MinCount != nil {
		t.Errorf("Organization parent shape wrong: %+v", p)
	}

	// Every PropertyShape in the file is error severity (explicit
	// sh:severity sh:Violation per Pitfall 3).
	for _, s := range parsed.NodeShapes {
		for i := range s.Properties {
			if s.Properties[i].Severity != SeverityError {
				t.Errorf("%s property %s: expected error severity, got %s",
					s.TargetClass, s.Properties[i].Path, s.Properties[i].Severity)
			}
		}
	}

	// Zero-constraint deviation: NO PropertyShape targets entityCode.
	for _, s := range parsed.NodeShapes {
		for i := range s.Properties {
			if s.Properties[i].Path == pf("entityCode") {
				t.Errorf("entityCode PropertyShape found on %s — zero-constraint deviation violated", s.TargetClass)
			}
		}
	}

	// Constructive first-run-green: legal Tier individual + legal Product
	// (type+label+tier) + legal Org → zero quarantine. Hermetic fixture
	// only — never live snapshot counts (Pitfall 7).
	kg := store.New()
	kg.Insert(store.Triple{S: nsSqlGuides, P: rdfType, O: skfTier})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#prod_legal", P: rdfType, O: skfInsuranceProduct})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#prod_legal", P: rdfsLabel, O: `"产品甲"`})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#prod_legal", P: skfBelongsToTier, O: nsSqlGuides})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#org_legal", P: rdfType, O: skfOrganization})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#org_legal", P: rdfsLabel, O: `"机构乙"`})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#org_legal", P: skfBelongsToTier, O: nsSqlGuides})

	report, err := Validate(kg, "entities.ttl")
	if err != nil {
		t.Fatalf("Validate(entities.ttl) failed: %v", err)
	}
	if report.Health != "ok" {
		t.Errorf("expected health ok for constructive legal entities, got %s", report.Health)
	}
	if report.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", report.ErrorCount)
	}
	if len(report.Quarantined) != 0 {
		t.Errorf("expected 0 quarantined, got %d", len(report.Quarantined))
	}
}

// ---------------------------------------------------------------------------
// TestEntityQuarantine — SC2 formal semantics (D-46-04): a quarantined
// entity becomes invisible in SUBJECT position, while its incoming edges
// remain visible in OBJECT position (subject-only quarantine; cascading
// quarantine rejected — store.Quarantine is subject-keyed and snapshots
// serialize only the main store). Also pins the cross-boot stable loop
// (Pitfall 6: boot snapshot is written BEFORE validate, so every boot
// re-validates the same graph and reports the same violations) and the
// dangling-edge disclosure (DanglingEdgeCount 1 for prod2).
// VALIDATION Test Map rows 46-02-T3 / SHACL-02.
// ---------------------------------------------------------------------------

func TestEntityQuarantine(t *testing.T) {
	fixture := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

skf:InsuranceProductShape a sh:NodeShape ;
    sh:targetClass skf:InsuranceProduct ;
    sh:property [
        sh:path rdf:type ;
        sh:hasValue skf:InsuranceProduct
    ] ;
    sh:property [
        sh:path rdfs:label ;
        sh:minCount 1 ;
        sh:message "skf:InsuranceProduct must have at least one rdfs:label"@en
    ] .
`
	buildKG := func() *store.Store {
		kg := store.New()
		// Tier individual (prod1's belongsToTier target must type-check).
		kg.Insert(store.Triple{S: nsSqlGuides, P: rdfType, O: skfTier})
		// prod1: healthy (type + label + tier).
		kg.Insert(store.Triple{S: "http://skillforge.local/ontology#prod1", P: rdfType, O: skfInsuranceProduct})
		kg.Insert(store.Triple{S: "http://skillforge.local/ontology#prod1", P: rdfsLabel, O: `"产品一"`})
		kg.Insert(store.Triple{S: "http://skillforge.local/ontology#prod1", P: skfBelongsToTier, O: nsSqlGuides})
		// prod2: violates (type only, missing label).
		kg.Insert(store.Triple{S: "http://skillforge.local/ontology#prod2", P: rdfType, O: skfInsuranceProduct})
		// note_001 —involvesInsuranceProduct→ prod2.
		kg.Insert(store.Triple{S: nsNote001, P: rdfType, O: skfNoteChunk})
		kg.Insert(store.Triple{S: nsNote001, P: skfInvolvesInsuranceProduct, O: "http://skillforge.local/ontology#prod2"})
		return kg
	}

	path := writeTempEntities(t, fixture)
	kg := buildKG()
	report, err := Validate(kg, path)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	prod1 := "http://skillforge.local/ontology#prod1"
	prod2 := "http://skillforge.local/ontology#prod2"
	var prod2Count int
	for _, q := range report.Quarantined {
		if q.Entity == prod2 {
			prod2Count++
			if q.DanglingEdgeCount != 1 {
				t.Errorf("prod2: expected DanglingEdgeCount 1 (note_001 in-edge), got %d", q.DanglingEdgeCount)
			}
			for _, v := range q.Violations {
				if v.Layer != LayerEntity {
					t.Errorf("prod2 violation: expected layer %q, got %q", LayerEntity, v.Layer)
				}
			}
		}
		if q.Entity == prod1 {
			t.Error("prod1 is healthy and must NOT be quarantined")
		}
	}
	if prod2Count != 1 {
		t.Fatalf("expected prod2 in Quarantined exactly once, got %d", prod2Count)
	}

	// Boot-style quarantine loop — main.go:937-941 replicated verbatim
	// (Query(q.Entity,"","") → Quarantine(t); the inner t shadows the
	// testing var exactly as in main).
	for _, q := range report.Quarantined {
		for _, t := range kg.Query(q.Entity, "", "") {
			kg.Quarantine(t)
		}
	}

	// SC2: subject position — prod2 is invisible.
	if got := kg.Query(prod2, "", ""); len(got) != 0 {
		t.Errorf("expected prod2 invisible in subject position after quarantine, got %d triples", len(got))
	}
	// SC2: object position — the incoming edge is intentionally disclosed.
	if got := kg.Query("", skfInvolvesInsuranceProduct, prod2); len(got) != 1 {
		t.Errorf("expected the note_001→prod2 edge visible in object position, got %d", len(got))
	}

	// Cross-boot stable loop (Pitfall 6): rebuild from the same fixture and
	// re-validate — prod2 must be quarantined again (each boot validates
	// the full snapshot, so quarantine never converges the graph clean).
	report2, err := Validate(buildKG(), path)
	if err != nil {
		t.Fatalf("re-Validate failed: %v", err)
	}
	stable := false
	for _, q := range report2.Quarantined {
		if q.Entity == prod2 {
			stable = true
		}
	}
	if !stable {
		t.Error("expected prod2 quarantined again on rebuild (stable cross-boot loop)")
	}
}
