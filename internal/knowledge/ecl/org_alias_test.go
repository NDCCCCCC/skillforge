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

// Quick 260929-h8k — extraction-side organization-alias normalization.
//
// Within ONE extraction batch, a bare org name (武汉) and its longer
// suffixed form (武汉中心支公司) are the same institution; the short form is
// dropped so the note's involves-organization edge collapses onto the long
// entity, and Parent references naming the short form are re-pointed so
// same-batch parent resolution (abox.go orgByName, exact-name lookup)
// keeps resolving. Cross-batch splits stay covered by InsertUnique plus the
// server-side projection merge (internal/server/entity_alias.go).

import (
	"testing"
)

// TestSanitizeEntitiesOrgAliasNormalization — batch merge + parent rewrite
// + product isolation.
func TestSanitizeEntitiesOrgAliasNormalization(t *testing.T) {
	raw := []ExtractedEntity{
		{Name: "武汉", Kind: "Organization"},                 // short alias → dropped
		{Name: "武汉中心支公司", Kind: "Organization"},            // canonical long name survives
		{Name: "某支公司", Kind: "Organization", Parent: "武汉"}, // parent ref → rewritten
		{Name: "责任险", Kind: "InsuranceProduct"},            // product pair must NOT merge
		{Name: "雇主责任险", Kind: "InsuranceProduct"},
	}
	got := sanitizeEntities(raw)
	if len(got) != 4 {
		t.Fatalf("sanitizeEntities = %#v, want 4 survivors (武汉 dropped, products untouched)", got)
	}
	if got[0].Name != "武汉中心支公司" {
		t.Errorf("first survivor = %q, want the canonical long name 武汉中心支公司", got[0].Name)
	}
	for _, e := range got {
		if e.Name == "武汉" {
			t.Errorf("short alias 武汉 must be dropped, got %#v", e)
		}
		if e.Name == "某支公司" && e.Parent != "武汉中心支公司" {
			t.Errorf("child Parent = %q, want rewritten to 武汉中心支公司", e.Parent)
		}
	}
	// Product isolation: 责任险 and 雇主责任险 are distinct products (the org
	// suffix grammar does not apply to InsuranceProduct names).
	var sawShort, sawLong bool
	for _, e := range got {
		switch e.Name {
		case "责任险":
			sawShort = true
		case "雇主责任险":
			sawLong = true
		}
	}
	if !sawShort || !sawLong {
		t.Errorf("products must never merge via the org rule: 责任险=%v 雇主责任险=%v", sawShort, sawLong)
	}
}

// TestSanitizeEntitiesOrgAliasNoMergeCases — the rule's restraint paths:
// no long form in batch → short survives; suffix-ending names are terminal
// even when a longer sibling exists; multiple candidates → longest wins.
func TestSanitizeEntitiesOrgAliasNoMergeCases(t *testing.T) {
	// Short org without its long form in the SAME batch survives (cross-
	// batch convergence is the projection merge's job, not this gate's).
	got := sanitizeEntities([]ExtractedEntity{{Name: "武汉", Kind: "Organization"}})
	if len(got) != 1 || got[0].Name != "武汉" {
		t.Fatalf("sanitizeEntities = %#v, want 武汉 untouched (no long form in batch)", got)
	}

	// A suffix-ending name is terminal: 公安支公司 stays even though a longer
	// sibling (公安支公司分部) is present — it is a DIFFERENT institution.
	got = sanitizeEntities([]ExtractedEntity{
		{Name: "公安支公司", Kind: "Organization"},
		{Name: "公安支公司分部", Kind: "Organization"},
	})
	if len(got) != 2 {
		t.Fatalf("sanitizeEntities = %#v, want both terminal names kept", got)
	}

	// Multiple long candidates → the LONGEST wins.
	got = sanitizeEntities([]ExtractedEntity{
		{Name: "武汉", Kind: "Organization"},
		{Name: "武汉支公司", Kind: "Organization"},
		{Name: "武汉中心支公司", Kind: "Organization"},
	})
	if len(got) != 2 {
		t.Fatalf("sanitizeEntities = %#v, want 武汉 dropped, both terminal long forms kept", got)
	}
	for _, e := range got {
		if e.Name == "武汉" {
			t.Fatalf("short alias 武汉 must be dropped when its long form is in batch: %#v", got)
		}
	}
}
