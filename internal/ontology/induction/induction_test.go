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

package induction

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/ontology/store"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const testNS = "http://skillforge.local/ontology#"

// stubGenerator returns a fixed JSON payload regardless of input.
type stubGenerator struct{ payload string }

func (g stubGenerator) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage(g.payload, nil), nil
}

func newTestManager(t *testing.T, payload string) *Manager {
	t.Helper()
	kg := store.New()
	m, err := NewManager(testNS, t.TempDir(), stubGenerator{payload}, kg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestToPascalCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"business rule", "BusinessRule"},
		{"SQL Note", "SQLNote"},
		{"etl_table", "EtlTable"},
		{"  multiple   spaces  ", "MultipleSpaces"},
		{"已有中文 mixed EN words", "MixedENWords"},
		{"data quality!! check", "DataQualityCheck"},
	}
	for _, c := range cases {
		if got := ToPascalCase(c.in); got != c.want {
			t.Errorf("ToPascalCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGeneratePascalCaseAndBilingualLabel(t *testing.T) {
	m := newTestManager(t, `{"kind":"Class","description_zh":"业务规则","description_en":"business rule"}`)
	p, err := m.Generate(context.Background(), "compliance notes mention business rules")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if p.LocalName != "BusinessRule" {
		t.Errorf("LocalName = %q, want BusinessRule", p.LocalName)
	}
	if p.ProposedIRI != testNS+"BusinessRule" {
		t.Errorf("ProposedIRI = %q, want %s", p.ProposedIRI, testNS+"BusinessRule")
	}
	if p.Status != StatusPending {
		t.Errorf("Status = %q, want pending", p.Status)
	}
	joined := strings.Join(p.ProposedTriples, "\n")
	if !strings.Contains(joined, `"业务规则"@zh`) || !strings.Contains(joined, `"business rule"@en`) {
		t.Errorf("bilingual rdfs:label missing: %s", joined)
	}
	if !strings.Contains(joined, "owl:Class") {
		t.Errorf("type declaration missing: %s", joined)
	}
}

func TestGenerateSecondSameDescriptionGetsSuffix(t *testing.T) {
	m := newTestManager(t, `{"kind":"Class","description_zh":"业务规则","description_en":"business rule"}`)
	if _, err := m.Generate(context.Background(), "text one"); err != nil {
		t.Fatalf("Generate 1: %v", err)
	}
	p2, err := m.Generate(context.Background(), "text two")
	if err != nil {
		t.Fatalf("Generate 2: %v", err)
	}
	if p2.LocalName != "BusinessRule2" {
		t.Errorf("second LocalName = %q, want BusinessRule2", p2.LocalName)
	}
	if p2.ID == "prop-0001" {
		t.Errorf("IDs not advancing")
	}
}

func TestApproveIncrementsTBoxVersion(t *testing.T) {
	m := newTestManager(t, `{"kind":"Class","description_zh":"业务规则","description_en":"business rule"}`)
	p, err := m.Generate(context.Background(), "source text")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// Seed a v1 TBox so the approve pipeline has a base document.
	if err := os.WriteFile(filepath.Join(m.dataDir, "skillforge_tbox.ttl"), []byte("# v1 base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.dataDir, "tbox_version.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	approved, version, err := m.Approve(p.ID, "")
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if version != 2 {
		t.Errorf("version = %d, want 2", version)
	}
	if approved.Status != StatusApproved {
		t.Errorf("status = %q, want approved", approved.Status)
	}
	v2, err := os.ReadFile(filepath.Join(m.dataDir, "skillforge_tbox.v2.ttl"))
	if err != nil {
		t.Fatalf("v2 file missing: %v", err)
	}
	if !strings.Contains(string(v2), "BusinessRule") {
		t.Errorf("v2 TTL lacks proposed local name")
	}
	pointer, err := os.ReadFile(filepath.Join(m.dataDir, "skillforge_tbox.ttl"))
	if err != nil {
		t.Fatalf("pointer file missing: %v", err)
	}
	if string(pointer) != string(v2) {
		t.Errorf("pointer file != v2 (copy fallback contract)")
	}
	vf, _ := os.ReadFile(filepath.Join(m.dataDir, "tbox_version.txt"))
	if strings.TrimSpace(string(vf)) != "2" {
		t.Errorf("tbox_version.txt = %q, want 2", vf)
	}
	// KG store sees the new type triple.
	hits := m.kg.Query("", "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", "http://www.w3.org/2002/07/owl#Class")
	if len(hits) == 0 {
		t.Errorf("KG store missing inserted type triple")
	}
}

func TestApproveCustomLocalName(t *testing.T) {
	m := newTestManager(t, `{"kind":"Class","description_zh":"业务规则","description_en":"business rule"}`)
	p, _ := m.Generate(context.Background(), "src")
	_, version, err := m.Approve(p.ID, "ComplianceBusinessRule")
	if err != nil {
		t.Fatalf("Approve custom name: %v", err)
	}
	if version != 2 {
		t.Errorf("version = %d, want 2", version)
	}
	list := m.List()
	if len(list) != 1 || list[0].LocalName != "ComplianceBusinessRule" {
		t.Errorf("custom local name not applied: %+v", list)
	}
}

func TestRejectKeepsRecord(t *testing.T) {
	m := newTestManager(t, `{"kind":"Class","description_zh":"业务规则","description_en":"business rule"}`)
	p, _ := m.Generate(context.Background(), "src")
	rejected, err := m.Reject(p.ID)
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if rejected.Status != StatusRejected {
		t.Errorf("status = %q, want rejected", rejected.Status)
	}
	// Approving a rejected proposal must fail.
	if _, _, err := m.Approve(p.ID, ""); err == nil {
		t.Errorf("approve after reject should fail")
	}
}

func TestPreviewIRIConflictDetection(t *testing.T) {
	m := newTestManager(t, `{"kind":"Class","description_zh":"业务规则","description_en":"business rule"}`)
	// Seed a TBox containing skf:Skill.
	ttl := "@prefix skf: <http://skillforge.local/ontology#> .\nskf:Skill a owl:Class .\n"
	if err := os.WriteFile(filepath.Join(m.dataDir, "skillforge_tbox.ttl"), []byte(ttl), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.PreviewIRI("Skill"); err == nil {
		t.Errorf("PreviewIRI(Skill) should conflict")
	}
	if err := m.PreviewIRI("BrandNewName"); err != nil {
		t.Errorf("PreviewIRI(BrandNewName) should be free: %v", err)
	}
	// Queued pending proposal also conflicts.
	if _, err := m.Generate(context.Background(), "s"); err != nil {
		t.Fatal(err)
	}
	if err := m.PreviewIRI("BusinessRule"); err == nil {
		t.Errorf("PreviewIRI on queued local name should conflict")
	}
}

func TestQueuePersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	kg := store.New()
	m1, err := NewManager(testNS, dir, stubGenerator{`{"kind":"Class","description_zh":"规则","description_en":"rule"}`}, kg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m1.Generate(context.Background(), "first"); err != nil {
		t.Fatal(err)
	}
	m2, err := NewManager(testNS, dir, nil, kg)
	if err != nil {
		t.Fatalf("restart NewManager: %v", err)
	}
	list := m2.List()
	if len(list) != 1 || list[0].LocalName != "Rule" {
		t.Fatalf("queue did not survive restart: %+v", list)
	}
	if _, err := m2.Generate(context.Background(), "second"); err == nil {
		t.Errorf("Generate without LLM client should fail")
	}
}

// ---------------------------------------------------------------------------
// TestRollbackLatency — SC-4: rollback completes in < 5 seconds.
// D-LOG-03 + D-PROV-09: rollback writes v{N+1}, updates pointer,
// clears KG, reloads TTL, runs SHACL revalidation.
// ---------------------------------------------------------------------------

func TestRollbackLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	dir := t.TempDir()
	kg := store.New()

	// Setup: write v1 TTL + v2 TTL + pointer + version + proposals.
	v1TTL := `@prefix skf: <http://skillforge.local/ontology#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
skf:Thing a owl:Class .
`
	v2TTL := `@prefix skf: <http://skillforge.local/ontology#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
skf:Thing a owl:Class .
skf:NewClass a owl:Class .
`
	os.WriteFile(filepath.Join(dir, "skillforge_tbox.v1.ttl"), []byte(v1TTL), 0o644)
	os.WriteFile(filepath.Join(dir, "skillforge_tbox.v2.ttl"), []byte(v2TTL), 0o644)
	os.WriteFile(filepath.Join(dir, "skillforge_tbox.ttl"), []byte(v2TTL), 0o644)
	os.WriteFile(filepath.Join(dir, "tbox_version.txt"), []byte("2\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "proposals.json"), []byte(`{"next_id":1,"items":[]}`), 0o644)

	// Seed the KG with v2 content.
	kg.LoadFromTurtle(v2TTL)

	m, err := NewManager(testNS, dir, nil, kg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Measure rollback from v2 → v1.
	start := time.Now()
	newVersion, violations, err := m.RollbackTBox(1, "test rollback")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("RollbackTBox: %v", err)
	}
	if newVersion != 3 {
		t.Errorf("expected version 3 after rollback, got %d", newVersion)
	}
	if violations < 0 {
		t.Errorf("violations should be non-negative, got %d", violations)
	}

	// SC-4: rollback < 5s.
	if elapsed > 5*time.Second {
		t.Errorf("rollback took %v, expected < 5s (SC-4)", elapsed)
	}

	t.Logf("rollback took %v", elapsed)
}

// ---------------------------------------------------------------------------
// TestRollbackShaclParity — D-46-02 + D-PROV-09: post-rollback SHACL
// revalidation must consume the SAME shapes path list as boot, i.e. the
// SKILLFORGE_SHACL_SHAPES env list via shacl.ShapesPathsFromEnv(). Before
// this fix the rollback side hard-coded data/shacl/shapes.ttl (a directory
// that never exists), so revalidation was a silent no-op returning 0
// violations forever — this path had ZERO test coverage (case 1 below is
// the regression nail: it fails against the hard-coded pre-fix behavior).
//
// Case 2 pins the D-PROV-09 posture: a shapes load failure is NOT a
// rollback failure (warn-and-return-0, never an error).
//
// The env-UNSET case is deliberately not pinned: the default list is
// relative to the repo root while package tests run with the package dir
// as cwd, so the behavior would depend on the process cwd — asserting it
// here would make the suite environment-dependent. TestShapesPaths in the
// shacl package covers the default-list fallback instead.
// ---------------------------------------------------------------------------

func TestRollbackShaclParity(t *testing.T) {
	// Subtests are sequential (no t.Parallel) — t.Setenv forbids parallel
	// parents, and each case needs its own env value.
	t.Run("env_list_consumed_violations_reported", func(t *testing.T) {
		dir := t.TempDir()
		kg := store.New()

		// v1 TTL carries a dangling cross-entity reference: BadNote points
		// at skf:MissingTable, which is not a skf:TableSchema instance.
		v1TTL := `@prefix skf: <http://skillforge.local/ontology#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
skf:Thing a owl:Class .
skf:BadNote a skf:NoteChunk .
skf:BadNote skf:referencesTable skf:MissingTable .
`
		v2TTL := `@prefix skf: <http://skillforge.local/ontology#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
skf:Thing a owl:Class .
skf:GoodNote a skf:NoteChunk .
`
		// Rollback assembly mirrors TestRollbackLatency: v1/v2 TTL + pointer
		// + version + proposals, KG seeded with v2, manager without LLM.
		if err := os.WriteFile(filepath.Join(dir, "skillforge_tbox.v1.ttl"), []byte(v1TTL), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "skillforge_tbox.v2.ttl"), []byte(v2TTL), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "skillforge_tbox.ttl"), []byte(v2TTL), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "tbox_version.txt"), []byte("2\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "proposals.json"), []byte(`{"next_id":1,"items":[]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := kg.LoadFromTurtle(v2TTL); err != nil {
			t.Fatalf("LoadFromTurtle: %v", err)
		}

		// Shapes fixture: the global referencesTable PropertyShape — the
		// rule that flags BadNote's dangling reference as an error.
		shapesTTL := `@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .

skf:referencesTableShape a sh:PropertyShape ;
    sh:path skf:referencesTable ;
    sh:class skf:TableSchema .
`
		shapesPath := filepath.Join(dir, "parity-shapes.ttl")
		if err := os.WriteFile(shapesPath, []byte(shapesTTL), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("SKILLFORGE_SHACL_SHAPES", shapesPath)

		m, err := NewManager(testNS, dir, nil, kg)
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}

		newVersion, violations, err := m.RollbackTBox(1, "test rollback")
		if err != nil {
			t.Fatalf("RollbackTBox: %v", err)
		}
		if newVersion != 3 {
			t.Errorf("expected version 3 after rollback, got %d", newVersion)
		}
		// THE regression nail: pre-fix the revalidation read a hard-coded
		// non-existent path and always returned 0. If env is truly consumed
		// the dangling reference must surface as >= 1 violation.
		if violations <= 0 {
			t.Errorf("expected violations > 0 for dangling reference after rollback (env list must be consumed), got %d", violations)
		}
	})

	t.Run("missing_shapes_warn_and_return_zero", func(t *testing.T) {
		dir := t.TempDir()
		kg := store.New()

		v1TTL := `@prefix skf: <http://skillforge.local/ontology#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
skf:Thing a owl:Class .
`
		v2TTL := `@prefix skf: <http://skillforge.local/ontology#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
skf:Thing a owl:Class .
skf:NewClass a owl:Class .
`
		if err := os.WriteFile(filepath.Join(dir, "skillforge_tbox.v1.ttl"), []byte(v1TTL), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "skillforge_tbox.v2.ttl"), []byte(v2TTL), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "skillforge_tbox.ttl"), []byte(v2TTL), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "tbox_version.txt"), []byte("2\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "proposals.json"), []byte(`{"next_id":1,"items":[]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := kg.LoadFromTurtle(v2TTL); err != nil {
			t.Fatalf("LoadFromTurtle: %v", err)
		}

		// Env points at a path that does not exist — D-PROV-09: SHACL
		// failure is not a rollback failure (warn-and-return-0).
		t.Setenv("SKILLFORGE_SHACL_SHAPES", filepath.Join(dir, "no-such-shapes.ttl"))

		m, err := NewManager(testNS, dir, nil, kg)
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}

		_, violations, err := m.RollbackTBox(1, "test rollback")
		if err != nil {
			t.Errorf("RollbackTBox must not fail on SHACL load error (D-PROV-09), got: %v", err)
		}
		if violations != 0 {
			t.Errorf("expected 0 violations on shapes load failure (warn-and-return-0), got %d", violations)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRollbackPreservesABox — CR-01 regression nail: RollbackTBox replaces
// ONLY the TBox layer of the KG store. The pre-fix behavior iterated
// Query("","","") and DeleteSubject'ed EVERY subject, wiping the ECL ABox
// triples (note/table/entity, D-KG-02: must survive restarts) — and the
// post-rollback snapshot persisted that wipe to disk. After the fix an ABox
// note seeded before the rollback must still be present, a TBox subject
// removed by the rollback must be gone, and the changelog entry must report
// the removed TBox subject instead of an empty Removed list.
// ---------------------------------------------------------------------------

func TestRollbackPreservesABox(t *testing.T) {
	dir := t.TempDir()
	kg := store.New()

	v1TTL := `@prefix skf: <http://skillforge.local/ontology#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
skf:Thing a owl:Class .
`
	v2TTL := `@prefix skf: <http://skillforge.local/ontology#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
skf:Thing a owl:Class .
skf:NewClass a owl:Class .
`
	for name, content := range map[string]string{
		"skillforge_tbox.v1.ttl": v1TTL,
		"skillforge_tbox.v2.ttl": v2TTL,
		"skillforge_tbox.ttl":    v2TTL,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "tbox_version.txt"), []byte("2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proposals.json"), []byte(`{"next_id":1,"items":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.LoadFromTurtle(v2TTL); err != nil {
		t.Fatalf("LoadFromTurtle: %v", err)
	}

	// Seed ABox triples the way the ECL pipeline does: note/entity IRIs that
	// never appear in any TBox TTL (D-KG-02).
	aboxNote := testNS + "note_001"
	aboxEntity := testNS + "org_acme"
	kg.Insert(store.Triple{S: aboxNote, P: "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", O: testNS + "NoteChunk"})
	kg.Insert(store.Triple{S: aboxNote, P: testNS + "involvesOrganization", O: aboxEntity})
	kg.Insert(store.Triple{S: aboxEntity, P: "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", O: testNS + "Organization"})

	m, err := NewManager(testNS, dir, nil, kg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := m.RollbackTBox(1, "cr-01 regression test"); err != nil {
		t.Fatalf("RollbackTBox: %v", err)
	}

	// ABox triples must have survived the rollback.
	if got := len(m.kg.Query(aboxNote, "", "")); got != 2 {
		t.Errorf("ABox note triples = %d, want 2 (ABox must survive rollback)", got)
	}
	if got := len(m.kg.Query(aboxEntity, "", "")); got != 1 {
		t.Errorf("ABox entity triples = %d, want 1", got)
	}
	// TBox subject removed by the rollback must be gone from the KG.
	if got := len(m.kg.Query(testNS+"NewClass", "", "")); got != 0 {
		t.Errorf("removed TBox subject NewClass still in KG (%d triples), want 0", got)
	}
	// TBox subject kept by the rollback must be present exactly once.
	if got := len(m.kg.Query(testNS+"Thing", "", "")); got != 1 {
		t.Errorf("kept TBox subject Thing = %d triples, want 1", got)
	}

	// Changelog honesty: the rollback entry must report the removed TBox
	// subject (the pre-fix entry wrote Removed: []string{} unconditionally).
	entries, err := m.ListChangelog()
	if err != nil {
		t.Fatalf("ListChangelog: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("no changelog entries after rollback")
	}
	if len(entries[0].What.Removed) != 1 || entries[0].What.Removed[0] != testNS+"NewClass" {
		t.Errorf("changelog Removed = %v, want [%sNewClass]", entries[0].What.Removed, testNS)
	}
	if len(entries[0].What.Added) != 0 {
		t.Errorf("changelog Added = %v, want empty", entries[0].What.Added)
	}
}
