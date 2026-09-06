// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// views.go is the v1.1.1 controlled-file bootstrap for the DATA-10
// view allowlist. The files it loads are CONTROLLED FILES in the same
// sense as kpis/*.yaml (D16-A1): operator-authored, reviewable,
// diffable in git, and the ONLY source of view registrations at
// runtime. Nothing else in the process may register a view into the
// registry the server boots with — a registry assembled any other way
// would be a registration backdoor around the approval channel.
//
// Posture mirrors the KPI loader (D16-08-D): a malformed or invalid
// controlled file is an ERROR naming the file, and the caller decides
// fatality (cmd/server log.Fatalf's it — a bad view file must not boot
// a PII channel). An ABSENT directory, or a directory containing zero
// *.yaml files, is the single legal degraded state: it yields an empty
// registry and a nil error, so a deployment can ship the gate enabled
// with no views yet (every RequestApproval is then rejected
// view-not-in-registry, which is fail-closed by construction).
package externalauth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// viewsFile is the on-disk shape of one controlled view file in
// external_views/. Every field is validated fail-closed by
// LoadViewsDir before anything is registered.
type viewsFile struct {
	Views []viewsEntry `yaml:"views"`
}

type viewsEntry struct {
	ViewName string     `yaml:"view_name"`
	Source   string     `yaml:"source"`
	Columns  []struct { // v1.1.1: feeds the NL2SQL preview prompt
		Name string `yaml:"name"`
		Type string `yaml:"type"`
	} `yaml:"columns"`
	FieldMasks []struct {
		FieldName string `yaml:"field_name"`
		Kind      string `yaml:"kind"`
		KeepLen   int    `yaml:"keep_len"`
	} `yaml:"field_masks"`
}

// validMaskKinds is the closed set of DATA-05 mask kinds a controlled
// file may declare. Anything else is a typo today and a leak tomorrow,
// so it is rejected rather than defaulted (ApplyMask's unknown-kind
// full-mask remains the last-resort runtime behavior, but the loader
// refuses to bless it at boot).
var validMaskKinds = map[string]bool{
	"account": true,
	"phone":   true,
	"email":   true,
	"generic": true,
}

// LoadViewsDir walks dir for *.yaml controlled view files and returns
// the assembled ViewRegistry. Validation is fail-closed per entry:
//
//   - view_name must match ViewNamePattern and be unique across the
//     whole directory (a duplicate name is an operator mistake that a
//     silent overwrite would hide);
//   - source must be non-empty (the human-readable origin is what an
//     auditor reads first);
//   - every mask kind must be one of account/phone/email/generic;
//   - every mask field_name must exist in the entry's declared columns
//     (a mask on an undeclared column is a typo that silently no-ops);
//   - column names must be unique within a view.
//
// An absent directory or one with zero yaml files returns an EMPTY
// registry and a nil error — the single legal degraded state. Any
// malformed file returns a wrapped error naming the file path; the
// caller decides whether that is fatal (cmd/server treats it as
// fatal — controlled files are load-bearing).
func LoadViewsDir(dir string) (*ViewRegistry, error) {
	reg := NewViewRegistry()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Absent directory: the one legal degraded state.
			return reg, nil
		}
		return nil, fmt.Errorf("externalauth.LoadViewsDir: read %s: %w", dir, err)
	}

	loaded := 0
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".yaml") {
			// Only *.yaml is a controlled file; *.yml, *.json, README.md
			// and friends are skipped rather than guessed at.
			continue
		}
		path := filepath.Join(dir, e.Name())
		if err := loadViewsFile(reg, path); err != nil {
			return nil, err
		}
		loaded++
	}
	if loaded == 0 {
		// Zero yaml files: same degraded posture as an absent dir.
		return reg, nil
	}
	return reg, nil
}

// loadViewsFile decodes + validates one controlled file and registers
// every valid entry. Errors always name the file path.
func loadViewsFile(reg *ViewRegistry, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("externalauth.LoadViewsDir: %s: %w", path, err)
	}
	var f viewsFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return fmt.Errorf("externalauth.LoadViewsDir: %s: decode yaml: %w", path, err)
	}
	if len(f.Views) == 0 {
		return fmt.Errorf("externalauth.LoadViewsDir: %s: no `views:` entries — a controlled view file must declare at least one view", path)
	}
	for i, entry := range f.Views {
		v, err := validateViewsEntry(entry)
		if err != nil {
			return fmt.Errorf("externalauth.LoadViewsDir: %s: entry %d (%q): %w", path, i+1, entry.ViewName, err)
		}
		if _, exists := reg.Get(v.ViewName); exists {
			return fmt.Errorf("externalauth.LoadViewsDir: %s: duplicate view name %q (already registered)", path, v.ViewName)
		}
		reg.Register(v)
	}
	return nil
}

// validateViewsEntry applies the fail-closed per-entry rules and
// builds the PrecompiledView. Pure function — trivially unit-testable.
func validateViewsEntry(e viewsEntry) (PrecompiledView, error) {
	if !ViewNamePattern.MatchString(e.ViewName) {
		return PrecompiledView{}, fmt.Errorf("view_name %q must match %s", e.ViewName, ViewNamePattern.String())
	}
	if strings.TrimSpace(e.Source) == "" {
		return PrecompiledView{}, fmt.Errorf("source must be non-empty (the human-readable origin is the auditor's first read)")
	}

	seenColumns := make(map[string]bool, len(e.Columns))
	cols := make([]ViewColumn, 0, len(e.Columns))
	for _, c := range e.Columns {
		if strings.TrimSpace(c.Name) == "" {
			return PrecompiledView{}, fmt.Errorf("column with empty name")
		}
		if seenColumns[c.Name] {
			return PrecompiledView{}, fmt.Errorf("duplicate column %q", c.Name)
		}
		seenColumns[c.Name] = true
		cols = append(cols, ViewColumn{Name: c.Name, Type: c.Type})
	}

	masks := make([]MaskSpec, 0, len(e.FieldMasks))
	for _, m := range e.FieldMasks {
		if !validMaskKinds[m.Kind] {
			return PrecompiledView{}, fmt.Errorf("field_masks[%q]: kind %q must be one of account|phone|email|generic", m.FieldName, m.Kind)
		}
		if !seenColumns[m.FieldName] {
			return PrecompiledView{}, fmt.Errorf("field_masks[%q]: field_name is not declared in the view's columns list", m.FieldName)
		}
		masks = append(masks, MaskSpec{FieldName: m.FieldName, Kind: m.Kind, KeepLen: m.KeepLen})
	}

	return PrecompiledView{
		ViewName:   e.ViewName,
		Source:     e.Source,
		Columns:    cols,
		FieldMasks: masks,
	}, nil
}
