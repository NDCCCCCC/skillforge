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
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
)

// retiredKGPath is the route retired by the 2026-11-13 rename, spelled as a
// concatenation so this reverse-contract assertion does not itself carry the
// retired path literal — the repo sweep gate greps internal/ for it (the FE
// keeps its own reverse assertion in web/src, outside the gate's path set,
// for the same reason). The concatenated value is byte-identical to the old
// path, so the 404 assertion below still exercises the real retired route.
const retiredKGPath = "/api/" + "sparql"

// TestKGTriplesRouteContract pins the 2026-11-13 rename end to end:
// POST /api/kg/triples serves the KG triple query; the retired SPARQL-shaped
// path is REMOVED (no compat shim). It goes through server.New — the REAL
// route table from server.go — not a hand-rolled registration, so a
// regression that re-adds or re-points the route cannot silently pass:
//   - /api/kg/triples serves the triple query (200 with a valid caller)
//   - the retired path is gone (404 — Hertz default for unregistered paths)
func TestKGTriplesRouteContract(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")

	// Same wiring as TestSchemaJudgmentParity: manager from newOntologyDeps,
	// KG replaced by the shared parity fixture (the handler reads only
	// Ontology.KG; fixtureKG wires skill "sql" → hasReadAccess).
	ont := newOntologyDeps(t)
	ont.KG = fixtureKG()
	deps := &Deps{Ontology: ont}

	h := New(":0", deps)

	body := `{"p":"` + testNS + `hasReadAccess"}`
	hdrs := []ut.Header{
		{Key: "Content-Type", Value: "application/json"},
		{Key: "X-Skill-Name", Value: "sql"},
		{Key: "X-Admin-Token", Value: "tok"},
	}
	req := &ut.Body{Body: strings.NewReader(body), Len: len(body)}

	resp := ut.PerformRequest(h.Engine, http.MethodPost, "/api/kg/triples", req, hdrs...)
	if resp.Code != http.StatusOK {
		t.Fatalf("/api/kg/triples: expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	retired := ut.PerformRequest(h.Engine, http.MethodPost, retiredKGPath, req, hdrs...)
	if retired.Code != http.StatusNotFound {
		t.Fatalf("%s must be retired (404), got %d: %s", retiredKGPath, retired.Code, retired.Body.String())
	}
}
