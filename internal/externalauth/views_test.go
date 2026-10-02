// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package externalauth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeViewsFile writes one controlled view file into a fresh temp dir
// and returns the dir. Helper for the loader matrix below.
func writeViewsFile(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return dir
}

// validViewsYAML is the canonical well-formed controlled file used as
// the happy-path fixture.
const validViewsYAML = `
views:
  - view_name: v_user_consumption
    source: read-only warehouse replica of the monthly consumption table
    columns:
      - name: user_id
        type: string
      - name: amount
        type: float
      - name: account_no
        type: string
      - name: phone
        type: string
      - name: email
        type: string
    field_masks:
      - field_name: account_no
        kind: account
        keep_len: 4
      - field_name: phone
        kind: phone
        keep_len: 4
      - field_name: email
        kind: email
        keep_len: 2
`

// TestLoadViewsDir_ValidFile — the happy path registers the view with
// its columns and masks intact.
func TestLoadViewsDir_ValidFile(t *testing.T) {
	dir := writeViewsFile(t, "v_user_consumption.yaml", validViewsYAML)

	reg, err := LoadViewsDir(dir)
	if err != nil {
		t.Fatalf("LoadViewsDir: %v", err)
	}
	if got := reg.Names(); len(got) != 1 || got[0] != "v_user_consumption" {
		t.Fatalf("registry names: %v", got)
	}
	v, ok := reg.Get("v_user_consumption")
	if !ok {
		t.Fatal("view missing from registry")
	}
	if v.Source == "" {
		t.Fatal("source must be carried through")
	}
	if len(v.Columns) != 5 {
		t.Fatalf("expected 5 columns, got %d (%+v)", len(v.Columns), v.Columns)
	}
	if v.Columns[2].Name != "account_no" || v.Columns[2].Type != "string" {
		t.Fatalf("column round-trip mismatch: %+v", v.Columns[2])
	}
	if len(v.FieldMasks) != 3 {
		t.Fatalf("expected 3 masks, got %d", len(v.FieldMasks))
	}
	if v.FieldMasks[0].FieldName != "account_no" || v.FieldMasks[0].Kind != "account" || v.FieldMasks[0].KeepLen != 4 {
		t.Fatalf("mask round-trip mismatch: %+v", v.FieldMasks[0])
	}
}

// TestLoadViewsDir_ShipsReferenceView — the file the repo actually
// ships under external_views/ must load through the production loader.
// This is the guard that keeps the shipped example honest: if someone
// edits the controlled file into an invalid state, the package tests
// fail exactly as the server boot would.
func TestLoadViewsDir_ShipsReferenceView(t *testing.T) {
	const shipped = "../../external_views"
	reg, err := LoadViewsDir(shipped)
	if err != nil {
		t.Fatalf("shipped external_views/ must load: %v", err)
	}
	v, ok := reg.Get("v_user_consumption")
	if !ok {
		t.Fatalf("shipped registry must contain v_user_consumption, got %v", reg.Names())
	}
	if len(v.Columns) == 0 || len(v.FieldMasks) == 0 {
		t.Fatalf("shipped view must declare columns + masks: %+v", v)
	}
}

// TestLoadViewsDir_MalformedYAMLErrors — a syntactically broken file
// is a wrapped error naming the file.
func TestLoadViewsDir_MalformedYAMLErrors(t *testing.T) {
	dir := writeViewsFile(t, "broken.yaml", "views: [ this is : not : valid yaml :")
	_, err := LoadViewsDir(dir)
	if err == nil {
		t.Fatal("expected an error for malformed yaml")
	}
	if !strings.Contains(err.Error(), "broken.yaml") {
		t.Fatalf("error must name the file, got: %v", err)
	}
}

// TestLoadViewsDir_UnknownMaskKindErrors — an unlisted mask kind fails
// closed instead of silently defaulting.
func TestLoadViewsDir_UnknownMaskKindErrors(t *testing.T) {
	dir := writeViewsFile(t, "bad_kind.yaml", `
views:
  - view_name: v_users
    source: test source
    columns:
      - name: acct
        type: string
    field_masks:
      - field_name: acct
        kind: mystery
`)
	_, err := LoadViewsDir(dir)
	if err == nil {
		t.Fatal("expected an error for unknown mask kind")
	}
	if !strings.Contains(err.Error(), "mystery") {
		t.Fatalf("error must echo the bad kind, got: %v", err)
	}
}

// TestLoadViewsDir_MaskFieldNotInColumnsErrors — a mask on an
// undeclared column would silently no-op at execution time, so the
// loader rejects it.
func TestLoadViewsDir_MaskFieldNotInColumnsErrors(t *testing.T) {
	dir := writeViewsFile(t, "bad_field.yaml", `
views:
  - view_name: v_users
    source: test source
    columns:
      - name: acct
        type: string
    field_masks:
      - field_name: ssn
        kind: generic
        keep_len: 4
`)
	_, err := LoadViewsDir(dir)
	if err == nil {
		t.Fatal("expected an error for a mask field missing from columns")
	}
	if !strings.Contains(err.Error(), "ssn") {
		t.Fatalf("error must echo the offending field, got: %v", err)
	}
}

// TestLoadViewsDir_DuplicateViewNameErrors — the same name declared
// twice (even across two files) is an operator mistake, not a silent
// overwrite.
func TestLoadViewsDir_DuplicateViewNameErrors(t *testing.T) {
	merged := t.TempDir()
	fixtures := map[string]string{
		"a.yaml": validViewsYAML,
		"b.yaml": validViewsYAML,
	}
	for name, body := range fixtures {
		if err := os.WriteFile(filepath.Join(merged, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}

	_, err := LoadViewsDir(merged)
	if err == nil {
		t.Fatal("expected an error for a duplicate view name")
	}
	if !strings.Contains(err.Error(), "duplicate view name") {
		t.Fatalf("error must name the duplicate, got: %v", err)
	}
}

// TestLoadViewsDir_AbsentDirEmptyRegistry — the single legal degraded
// state: no directory at all yields an empty registry + nil error.
func TestLoadViewsDir_AbsentDirEmptyRegistry(t *testing.T) {
	reg, err := LoadViewsDir(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("absent dir must not error, got %v", err)
	}
	if reg == nil {
		t.Fatal("registry must be non-nil")
	}
	if got := reg.Names(); len(got) != 0 {
		t.Fatalf("absent dir must yield an empty registry, got %v", got)
	}
}

// TestLoadViewsDir_ZeroYamlDirEmptyRegistry — a directory with other
// files but zero *.yaml files is the same legal degraded state.
func TestLoadViewsDir_ZeroYamlDirEmptyRegistry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("no views yet"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.yml"), []byte("views: []"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	reg, err := LoadViewsDir(dir)
	if err != nil {
		t.Fatalf("zero-yaml dir must not error, got %v", err)
	}
	if got := reg.Names(); len(got) != 0 {
		t.Fatalf("zero-yaml dir must yield an empty registry, got %v", got)
	}
}

// TestLoadViewsDir_DuplicateColumnErrors — column names must be unique
// within a view.
func TestLoadViewsDir_DuplicateColumnErrors(t *testing.T) {
	dir := writeViewsFile(t, "dupcol.yaml", `
views:
  - view_name: v_users
    source: test source
    columns:
      - name: acct
        type: string
      - name: acct
        type: int
`)
	if _, err := LoadViewsDir(dir); err == nil {
		t.Fatal("expected an error for duplicate column names")
	}
}

// TestLoadViewsDir_EmptyViewNameErrors — a view_name failing
// ViewNamePattern is rejected before registration.
func TestLoadViewsDir_EmptyViewNameErrors(t *testing.T) {
	dir := writeViewsFile(t, "noname.yaml", `
views:
  - view_name: "Bad-Name"
    source: test source
`)
	if _, err := LoadViewsDir(dir); err == nil {
		t.Fatal("expected an error for a view_name failing ViewNamePattern")
	}
}
