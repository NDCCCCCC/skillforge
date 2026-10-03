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

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/ontology/induction"
	"github.com/skillforge/skillforge/internal/ontology/store"
)

const ontologyTestNS = "http://test.local/onto#"

type stubLLM struct{ payload string }

func (g stubLLM) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage(g.payload, nil), nil
}

// newOntologyDeps builds a minimal wired OntologyDeps in a temp dir.
func newOntologyDeps(t *testing.T) *OntologyDeps {
	t.Helper()
	kg := store.New()
	if _, err := kg.LoadFromTurtle(testMiniTTL); err != nil {
		t.Fatalf("LoadFromTurtle: %v", err)
	}
	mgr, err := induction.NewManager(ontologyTestNS, t.TempDir(),
		stubLLM{`{"kind":"Class","description_zh":"业务规则","description_en":"business rule"}`}, kg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	// Seed the pointer TBox the tbox handler reads.
	if err := writeFileStr(t, mgr.DataDir(), "skillforge_tbox.ttl", testMiniTTL); err != nil {
		t.Fatal(err)
	}
	if err := writeFileStr(t, mgr.DataDir(), "tbox_version.txt", "1\n"); err != nil {
		t.Fatal(err)
	}
	return &OntologyDeps{Induction: mgr, KG: kg}
}

const testMiniTTL = `@prefix skf: <http://test.local/onto#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .

skf:Skill a owl:Class .

skf:skill_a a skf:Skill ;
    skf:skillName "a" .

skf:skill_b a skf:Skill ;
    skf:skillName "b" .
`

func writeFileStr(t *testing.T, dir, name, content string) error {
	t.Helper()
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

func ontologyRoutes(h *server.Hertz, deps *Deps) {
	h.GET("/api/admin/ontology/tbox", deps.adminOntologyTBoxHandler)
	h.GET("/api/admin/ontology/kg/stats", deps.adminOntologyKGStatsHandler)
	h.GET("/api/admin/ontology/kg/query", deps.adminOntologyKGQueryHandler)
	h.GET("/api/admin/ontology/graph", deps.adminOntologyGraphHandler)
	h.POST("/api/admin/ontology/proposals/generate", deps.adminOntologyProposalsGenerateHandler)
	h.GET("/api/admin/ontology/proposals", deps.adminOntologyProposalsListHandler)
	h.POST("/api/admin/ontology/proposals/:id/approve", deps.adminOntologyProposalsApproveHandler)
	h.POST("/api/admin/ontology/proposals/:id/reject", deps.adminOntologyProposalsRejectHandler)
	h.POST("/api/admin/ontology/snapshot", deps.adminOntologySnapshotHandler)
}

func invokeOntology(t *testing.T, deps *Deps, method, path, body string, token string) *ut.ResponseRecorder {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	ontologyRoutes(h, deps)
	var headers []ut.Header
	if token != "" {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: token})
	}
	if body != "" {
		headers = append(headers, ut.Header{Key: "Content-Type", Value: "application/json"})
	}
	var reqBody *ut.Body
	if body != "" {
		reqBody = &ut.Body{Body: strings.NewReader(body), Len: len(body)}
	}
	return ut.PerformRequest(h.Engine, method, path, reqBody, headers...)
}

func TestAdminOntologyRequiresToken(t *testing.T) {
	deps := &Deps{Ontology: newOntologyDeps(t)}
	paths := []struct{ method, path string }{
		{http.MethodGet, "/api/admin/ontology/tbox"},
		{http.MethodGet, "/api/admin/ontology/kg/stats"},
		{http.MethodGet, "/api/admin/ontology/kg/query"},
		{http.MethodGet, "/api/admin/ontology/graph"},
		{http.MethodPost, "/api/admin/ontology/proposals/generate"},
		{http.MethodGet, "/api/admin/ontology/proposals"},
		{http.MethodPost, "/api/admin/ontology/proposals/prop-0001/approve"},
		{http.MethodPost, "/api/admin/ontology/proposals/prop-0001/reject"},
		{http.MethodPost, "/api/admin/ontology/snapshot"},
	}
	for _, p := range paths {
		if code := invokeOntology(t, deps, p.method, p.path, "", "").Code; code != http.StatusForbidden {
			t.Errorf("%s %s without token: expected 403, got %d", p.method, p.path, code)
		}
	}
}

func TestAdminOntologyNilLayer503(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	deps := &Deps{}
	if code := invokeOntology(t, deps, http.MethodGet, "/api/admin/ontology/tbox", "", "tok").Code; code != http.StatusServiceUnavailable {
		t.Errorf("nil ontology layer: expected 503, got %d", code)
	}
}

func TestAdminOntologyTBoxEndpoint(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	deps := &Deps{Ontology: newOntologyDeps(t)}
	resp := invokeOntology(t, deps, http.MethodGet, "/api/admin/ontology/tbox", "", "tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var body struct {
		Version     int    `json:"version"`
		TripleCount int    `json:"triple_count"`
		TTL         string `json:"ttl"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Version != 1 {
		t.Errorf("version = %d, want 1", body.Version)
	}
	if body.TripleCount == 0 {
		t.Errorf("triple_count = 0, want KG count")
	}
	if !strings.Contains(body.TTL, "skf:skill_a") {
		t.Errorf("ttl body missing skill_a")
	}
}

func TestAdminOntologyKGQueryEndpoint(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	deps := &Deps{Ontology: newOntologyDeps(t)}
	path := "/api/admin/ontology/kg/query?p=" +
		"http://www.w3.org/1999/02/22-rdf-syntax-ns%23type&o=" +
		strings.ReplaceAll(ontologyTestNS+"Skill", "#", "%23")
	resp := invokeOntology(t, deps, http.MethodGet, path, "", "tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var body struct {
		Matches []store.Triple `json:"matches"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Matches) != 2 {
		t.Errorf("matches = %d, want 2 (skill_a + skill_b)", len(body.Matches))
	}
}

func TestAdminOntologyProposalApproveFlow(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	deps := &Deps{Ontology: newOntologyDeps(t)}

	// 1. generate
	resp := invokeOntology(t, deps, http.MethodPost, "/api/admin/ontology/proposals/generate",
		`{"text":"compliance notes reference business rules"}`, "tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("generate: %d %s", resp.Code, resp.Body.String())
	}
	var gen struct {
		Proposal induction.Proposal `json:"proposal"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &gen); err != nil {
		t.Fatalf("unmarshal generate: %v", err)
	}
	if gen.Proposal.Status != "pending" || gen.Proposal.LocalName != "BusinessRule" {
		t.Fatalf("generate produced %+v", gen.Proposal)
	}

	// 2. list contains it
	resp = invokeOntology(t, deps, http.MethodGet, "/api/admin/ontology/proposals", "", "tok")
	var list struct {
		Proposals []induction.Proposal `json:"proposals"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(list.Proposals) != 1 {
		t.Fatalf("list = %d, want 1", len(list.Proposals))
	}

	// 3. approve → tbox_version 2
	resp = invokeOntology(t, deps, http.MethodPost,
		"/api/admin/ontology/proposals/"+gen.Proposal.ID+"/approve", `{}`, "tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", resp.Code, resp.Body.String())
	}
	var appr struct {
		TBoxVersion int `json:"tbox_version"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &appr); err != nil {
		t.Fatalf("unmarshal approve: %v", err)
	}
	if appr.TBoxVersion != 2 {
		t.Errorf("tbox_version = %d, want 2", appr.TBoxVersion)
	}

	// 4. tbox endpoint reports version 2
	resp = invokeOntology(t, deps, http.MethodGet, "/api/admin/ontology/tbox", "", "tok")
	var tb struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &tb); err != nil {
		t.Fatalf("unmarshal tbox: %v", err)
	}
	if tb.Version != 2 {
		t.Errorf("tbox version after approve = %d, want 2", tb.Version)
	}
}

func TestAdminOntologySnapshot(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	deps := &Deps{Ontology: newOntologyDeps(t)}
	resp := invokeOntology(t, deps, http.MethodPost, "/api/admin/ontology/snapshot", "", "tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("snapshot: %d %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "skillforge_kg.json") {
		t.Errorf("snapshot response missing path: %s", resp.Body.String())
	}
}

// invokeParity mirrors invokeOntology but registers BOTH the admin ontology
// routes AND POST /api/sparql on ONE engine, so TestSchemaJudgmentParity can
// issue the two parity requests against the same handler set. /api/sparql is
// NOT under the admin token group (server.go registers it bare — its caller
// context is the X-Skill-Name header, not X-Admin-Token).
//
// DO NOT fold this into invokeOntology: six existing tests depend on
// invokeOntology/ontologyRoutes exact behavior (45-02 acceptance criteria
// forbid modifying them — T-45-06 anti-second-implementation guard).
func invokeParity(t *testing.T, deps *Deps, method, path, body string, headers ...ut.Header) *ut.ResponseRecorder {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	ontologyRoutes(h, deps)
	h.POST("/api/sparql", deps.adminPlatformSparqlHandler)
	var hdrs []ut.Header
	if body != "" {
		hdrs = append(hdrs, ut.Header{Key: "Content-Type", Value: "application/json"})
	}
	hdrs = append(hdrs, headers...)
	var reqBody *ut.Body
	if body != "" {
		reqBody = &ut.Body{Body: strings.NewReader(body), Len: len(body)}
	}
	return ut.PerformRequest(h.Engine, method, path, reqBody, hdrs...)
}

// parityHasTriple reports whether rows contain the (s, p, o) triple.
func parityHasTriple(rows []store.Triple, s, p, o string) bool {
	for _, t := range rows {
		if t.S == s && t.P == p && t.O == o {
			return true
		}
	}
	return false
}

// parityHasSubject reports whether any row has the given subject.
func parityHasSubject(rows []store.Triple, s string) bool {
	for _, t := range rows {
		if t.S == s {
			return true
		}
	}
	return false
}

// TestSchemaJudgmentParity pins SPARQL-03 / D-45-03 at the handler level:
// /api/sparql (X-Skill-Name, skill-scoped surface) and
// /api/admin/ontology/kg/query?skill= (X-Admin-Token, admin surface) make
// the IDENTICAL schema/ABox judgment on the SAME store — both route through
// QueryScoped Path B and its single isSchemaTriple call site (store.go, the
// only schema-classification implementation in the codebase). The test only
// OBSERVES the two real handlers' responses; no judgment logic exists here.
//
// Parity assertions on fixtureKG (query pattern p = skf:hasReadAccess,
// skill = "sql"):
//   - BOTH responses contain (skill_sql, hasReadAccess, sql-guides) — the
//     D-SPARQL-05 β-visible row
//   - NEITHER response contains any note_003 row — the fail-closed
//     D-TIER-07 subject stays invisible on both surfaces
//
// Resolver-parity precondition (45-RESEARCH A4 / BL-01): fixtureKG wires
// hasReadAccess → sql-guides where sql-guides carries rdf:type skf:Tier —
// the Case-1 direct-Tier wiring that resolveSkillTiersFromKG
// (admin_platform.go) and resolveSkillTiers (admin_ontology.go) resolve
// IDENTICALLY. This keeps the test measuring the shared store judgment, not
// resolver divergence.
func TestSchemaJudgmentParity(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")

	// requireOntology gates the kg/query handler on a non-nil Induction, so
	// the manager from newOntologyDeps is wired — then its testMiniTTL KG is
	// REPLACED by the shared parity fixture (45-02 Task 2 mandates
	// fixtureKG; the kg/query handler reads only Ontology.KG).
	ont := newOntologyDeps(t)
	ont.KG = fixtureKG()
	deps := &Deps{Ontology: ont}

	// --- Request A: skill-scoped surface (POST /api/sparql).
	respA := invokeParity(t, deps, http.MethodPost, "/api/sparql",
		`{"p":"`+testNS+`hasReadAccess"}`,
		ut.Header{Key: "X-Skill-Name", Value: "sql"})
	if respA.Code != http.StatusOK {
		t.Fatalf("/api/sparql: expected 200, got %d: %s", respA.Code, respA.Body.String())
	}
	var sparql SparqlResponse
	if err := json.Unmarshal(respA.Body.Bytes(), &sparql); err != nil {
		t.Fatalf("/api/sparql unmarshal: %v", err)
	}
	rowsA := make([]store.Triple, 0, len(sparql.Results))
	for _, r := range sparql.Results {
		rowsA = append(rowsA, store.Triple{S: r.S, P: r.P, O: r.O})
	}

	// --- Request B: admin surface (GET /api/admin/ontology/kg/query?skill=).
	path := "/api/admin/ontology/kg/query?p=" +
		strings.ReplaceAll(testNS+"hasReadAccess", "#", "%23") + "&skill=sql"
	respB := invokeParity(t, deps, http.MethodGet, path, "",
		ut.Header{Key: "X-Admin-Token", Value: "tok"})
	if respB.Code != http.StatusOK {
		t.Fatalf("kg/query: expected 200, got %d: %s", respB.Code, respB.Body.String())
	}
	var admin kgQueryResponse
	if err := json.Unmarshal(respB.Body.Bytes(), &admin); err != nil {
		t.Fatalf("kg/query unmarshal: %v", err)
	}

	// --- Parity assertions: same judgment on both surfaces.
	wantS, wantP, wantO := testNS+"skill_sql", testNS+"hasReadAccess", testNS+"sql-guides"
	if !parityHasTriple(rowsA, wantS, wantP, wantO) {
		t.Errorf("/api/sparql missing the beta-visible row (%s, %s, %s); got %+v",
			wantS, wantP, wantO, rowsA)
	}
	if !parityHasTriple(admin.Matches, wantS, wantP, wantO) {
		t.Errorf("kg/query missing the beta-visible row (%s, %s, %s); got %+v",
			wantS, wantP, wantO, admin.Matches)
	}
	noteX := testNS + "note_003"
	if parityHasSubject(rowsA, noteX) {
		t.Errorf("/api/sparql leaked a note_003 row (fail-closed violation, D-TIER-07): %+v", rowsA)
	}
	if parityHasSubject(admin.Matches, noteX) {
		t.Errorf("kg/query leaked a note_003 row (fail-closed violation, D-TIER-07): %+v", admin.Matches)
	}
	// Row-count parity is an extra observational signal: the same pattern on
	// the same store through the same QueryScoped call site must yield the
	// same total (schema passthrough is tier-agnostic → both hasReadAccess
	// rows on each surface).
	if sparql.Count != len(admin.Matches) {
		t.Errorf("count parity broken: /api/sparql count=%d, kg/query matches=%d",
			sparql.Count, len(admin.Matches))
	}
}
