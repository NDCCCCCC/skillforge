// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package nl2sql

import (
	"testing"
)

func TestExtractSQL_WithPrefix(t *testing.T) {
	in := "VIEW: v_user\nSQL: select * from v_user\nESTIMATE: 5\n"
	if got := extractSQL(in); got != "select * from v_user" {
		t.Fatalf("got %q", got)
	}
	if got := extractView(in); got != "v_user" {
		t.Fatalf("view: %q", got)
	}
	if got := extractEstimate(in); got != 5 {
		t.Fatalf("estimate: %d", got)
	}
}

func TestExtractSQL_FallbackNoPrefix(t *testing.T) {
	// Smaller models may emit raw SQL without the VIEW:/SQL: prefix.
	in := "select id from v_user"
	if got := extractSQL(in); got != "select id from v_user" {
		t.Fatalf("got %q", got)
	}
	if got := extractView(in); got != "" {
		t.Fatalf("view should be empty for un-prefixed output, got %q", got)
	}
}

func TestCanonicalizeSQL(t *testing.T) {
	in := "  SELECT  *  \n FROM   v_user ;\n\n"
	want := "SELECT * FROM v_user"
	if got := canonicalizeSQL(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildPromptIncludesSchema(t *testing.T) {
	s := Schema{Views: []ViewSchema{
		{ViewName: "v_user", Columns: []ColumnSpec{{Name: "id", Type: "int"}, {Name: "name", Type: "string"}}},
	}}
	got := buildPrompt(s, "list all users")
	if want := "v_user(id int, name string)"; !contains(got, want) {
		t.Fatalf("prompt missing schema: %q (looking for %q)", got, want)
	}
	if want := "QUERY: list all users"; !contains(got, want) {
		t.Fatalf("prompt missing query: %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
