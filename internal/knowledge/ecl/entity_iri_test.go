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

package ecl

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"testing"
)

// TestEntityIRIDeterministic — EntityIRI is deterministic (D-NZK-01: entity
// identity = exact name ⇒ same name → same IRI across notes and re-runs,
// which is what makes cross-note InsertUnique dedupe work) and always
// prefixed ns + "ent_".
func TestEntityIRIDeterministic(t *testing.T) {
	a := EntityIRI(testNS, "车险")
	b := EntityIRI(testNS, "车险")
	if a != b {
		t.Fatalf("EntityIRI not deterministic: %q vs %q", a, b)
	}
	if len(a) <= len(testNS)+len("ent_") || a[:len(testNS)+4] != testNS+"ent_" {
		t.Fatalf("EntityIRI = %q, want the %q<local> shape", a, testNS+"ent_")
	}
}

// TestEntityIRICJKHashLocalName pins the CJK local-name algorithm exactly:
// "e" + first 12 hex chars of sha256(name) (T-NZK-02 — no raw LLM text ever
// reaches the IRI; emitter.SanitizeLocal would strip CJK to empty).
func TestEntityIRICJKHashLocalName(t *testing.T) {
	sum := sha256.Sum256([]byte("车险"))
	want := testNS + "ent_e" + hex.EncodeToString(sum[:])[:12]
	if got := EntityIRI(testNS, "车险"); got != want {
		t.Errorf("EntityIRI(车险) = %q, want %q", got, want)
	}
	if !regexp.MustCompile(`^e[0-9a-f]{12}$`).MatchString(EntityIRI(testNS, "车险")[len(testNS)+4:]) {
		t.Error("CJK local name is not e+12hex")
	}
}

// TestEntityIRIASCIIPascalCase pins the ASCII fast path: pure
// [0-9A-Za-z_] names take a deterministic PascalCase transform (snake and
// uniform-case inputs converge on the same readable form).
func TestEntityIRIASCIIPascalCase(t *testing.T) {
	cases := map[string]string{
		"che_xian": "CheXian",
		"ABC":      "Abc",
		"ab":       "Ab",
		"Entity":   "Entity",
		"a_b_c":    "ABC",
	}
	for in, want := range cases {
		if got := EntityIRI(testNS, in); got != testNS+"ent_"+want {
			t.Errorf("EntityIRI(%q) = %q, want %q", in, got, testNS+"ent_"+want)
		}
	}
}

// TestEntityIRIDistinctNamesDistinctIRIs — distinct CJK entity names map to
// distinct IRIs (the sha256 path is injective for practical purposes), so
// exact-name identity never fuses two entities cross-batch.
func TestEntityIRIDistinctNamesDistinctIRIs(t *testing.T) {
	seen := map[string]string{}
	for _, name := range []string{"车险", "农险", "健康险", "意外险", "学平险", "湖北分公司", "武汉中支"} {
		iri := EntityIRI(testNS, name)
		if prev, ok := seen[iri]; ok {
			t.Fatalf("IRI collision: %q and %q both → %q", prev, name, iri)
		}
		seen[iri] = name
	}
}
