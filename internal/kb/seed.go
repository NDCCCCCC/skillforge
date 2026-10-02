// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// seed.go is the controlled-file bootstrap for the knowledge-base
// registry. knowledge_bases.yaml is a CONTROLLED FILE in the same sense
// as the KPI definitions: operator-authored, reviewable, diffable in git,
// and the only place the baseline knowledge bases are declared. Nothing
// else in the process may invent a baseline entry — a registry assembled
// any other way would be a registration backdoor around review.
//
// Posture mirrors the other controlled-file loaders: a malformed or
// invalid seed is an ERROR naming the file and the offending entry, and
// the caller decides fatality (the server boot treats it as fatal — a bad
// seed must not boot a half-known knowledge surface). An ABSENT file is
// the single legal degraded state: it yields no entries and no error, so
// a deployment can boot with an empty registry and every skill reads
// declaration then fails closed rather than guessing.
//
// Seed-vs-runtime semantics: this file declares STRUCTURE (slug, display
// name, role, description) plus the initial value of the retrieval
// switch. Once a deployment has a persisted registry, the runtime value
// of the switch wins, so an admin deactivation is never resurrected by a
// restart ("structure belongs to config, the switch belongs to ops").
package kb

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// seedFile is the on-disk shape of the controlled seed file. Decoding is
// strict: an unknown field is an error, so a typo'd key can never silently
// drop a declaration.
type seedFile struct {
	Version        string      `yaml:"version"`
	KnowledgeBases []seedEntry `yaml:"knowledge_bases"`
}

// seedEntry is one declared knowledge base. RetrievalEnabled is a pointer
// so a missing switch is distinguishable from an explicit false: a silent
// default would decide retrieval eligibility, which is exactly the kind of
// decision that must stay visible in the file.
type seedEntry struct {
	Slug             string `yaml:"slug"`
	DisplayName      string `yaml:"display_name"`
	Role             string `yaml:"role"`
	Description      string `yaml:"description"`
	RetrievalEnabled *bool  `yaml:"retrieval_enabled"`
}

// LoadSeedFile reads and validates the controlled seed file and returns
// its entries in declaration order. Validation is fail-closed per entry:
//
//   - slug must pass ValidateSlug (whitelist pattern + the channel-prefix
//     reservation) and be unique within the file;
//   - role must be a member of the closed role vocabulary;
//   - display_name must be non-empty;
//   - retrieval_enabled must be declared explicitly;
//   - the file must declare a version and at least one entry.
//
// An absent file returns (nil, nil) — the single legal degraded state.
// Every other failure returns a wrapped error naming the file, the entry
// index and the slug. Timestamps are stamped with the load time; a
// persisted runtime registry overwrites them with its own.
func LoadSeedFile(path string) ([]Entry, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Absent seed: the one legal degraded state.
			return nil, nil
		}
		return nil, fmt.Errorf("kb.LoadSeedFile: %s: %w", path, err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("kb.LoadSeedFile: %s is a symlink — the knowledge-base seed is a regular controlled file only (symlink-swap guard)", path)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("kb.LoadSeedFile: %s: %w", path, err)
	}

	var f seedFile
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true) // a typo'd key must error, never silently drop
	if err := dec.Decode(&f); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("kb.LoadSeedFile: %s: file is empty — a controlled seed must declare at least one knowledge base", path)
		}
		return nil, fmt.Errorf("kb.LoadSeedFile: %s: decode yaml: %w", path, err)
	}
	if strings.TrimSpace(f.Version) == "" {
		return nil, fmt.Errorf("kb.LoadSeedFile: %s: missing version — a controlled seed must declare its version", path)
	}
	if len(f.KnowledgeBases) == 0 {
		return nil, fmt.Errorf("kb.LoadSeedFile: %s: no `knowledge_bases:` entries — a controlled seed must declare at least one knowledge base", path)
	}

	now := time.Now().UTC()
	out := make([]Entry, 0, len(f.KnowledgeBases))
	seen := make(map[string]bool, len(f.KnowledgeBases))
	for i, e := range f.KnowledgeBases {
		entry, err := validateSeedEntry(e)
		if err != nil {
			return nil, fmt.Errorf("kb.LoadSeedFile: %s: entry %d (%q): %w", path, i+1, e.Slug, err)
		}
		if seen[entry.Slug] {
			return nil, fmt.Errorf("kb.LoadSeedFile: %s: entry %d (%q): duplicate slug within the seed file (a silent overwrite would hide an operator mistake)", path, i+1, entry.Slug)
		}
		seen[entry.Slug] = true
		entry.CreatedAt = now
		entry.UpdatedAt = now
		out = append(out, entry)
	}
	return out, nil
}

// validateSeedEntry applies the fail-closed per-entry rules and builds the
// Entry. Pure function — trivially unit-testable.
func validateSeedEntry(e seedEntry) (Entry, error) {
	if err := ValidateSlug(e.Slug); err != nil {
		return Entry{}, err
	}
	if !IsRole(e.Role) {
		return Entry{}, fmt.Errorf("role %q must be one of %v (roles are platform semantics, not configuration)", e.Role, Roles())
	}
	if strings.TrimSpace(e.DisplayName) == "" {
		return Entry{}, fmt.Errorf("display_name must be non-empty (it is the UI badge text and is pinned by the refusal copy)")
	}
	if e.RetrievalEnabled == nil {
		return Entry{}, fmt.Errorf("retrieval_enabled must be declared explicitly (a silent default would decide retrieval eligibility)")
	}
	return Entry{
		Slug:             e.Slug,
		DisplayName:      e.DisplayName,
		Role:             Role(e.Role),
		Description:      e.Description,
		RetrievalEnabled: *e.RetrievalEnabled,
	}, nil
}

// LoadSeedRegistry is the convenience constructor for commands and boot
// paths that want the seed view directly: LoadSeedFile followed by New.
// The returned registry is read-only (no Store wired).
func LoadSeedRegistry(path string) (Registry, error) {
	entries, err := LoadSeedFile(path)
	if err != nil {
		return nil, err
	}
	return New(entries), nil
}
