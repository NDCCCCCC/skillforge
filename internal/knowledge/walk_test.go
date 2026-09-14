// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// SPDX-License-Identifier: Apache-2.0

// Regression tests for Phase 999.2 walker recursion. The walker
// (internal/knowledge/walkTier) replaced a flat filepath.Glob with
// filepath.WalkDir so KB directories may now contain subdirectories for
// visual / git organization. These tests pin the contract that
// walk.go's three-state decision table requires:

//   - .md files are discovered at ANY depth (root + nested + deep)
//   - Hidden directories (entries whose name starts with '.') are skipped
//   - Non-markdown files are skipped
//   - The returned slice is sorted in lexical order so chunk ID assignment
//     remains stable across server restarts (Phase 999.2 / RESEARCH.md F2)
//   - Empty directories return an empty slice (not an error)
//
// The walker does NOT consult the KB registry — that's the responsibility
// of WalkRegisteredDirs at a higher layer. walkTier sees only filesystem
// state.

package knowledge

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// makeFile creates an empty file at the given relative path under root.
// Intermediate directories are created as needed.
func makeFile(t *testing.T, root, rel string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte("# "+filepath.Base(rel)+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

// TestWalk_DiscoversNestedFiles verifies that .md files at multiple
// nesting depths (root + one level + two levels) are all discovered.
// Pre-999.2 walker would have missed the nested files.
func TestWalk_DiscoversNestedFiles(t *testing.T) {
	root := t.TempDir()
	makeFile(t, root, "root.md")
	makeFile(t, root, "policies/hr/hr.md")
	makeFile(t, root, "policies/finance/finance.md")
	makeFile(t, root, "policies/finance/sub/deep.md")

	got, err := walkTier(root)
	if err != nil {
		t.Fatalf("walkTier: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 files, got %d: %v", len(got), got)
	}
	// Every returned path must be under root and end in .md. Use
	// strings.HasPrefix on the (already-cleaned) absolute paths rather than
	// the deprecated filepath.HasPrefix — both root and p come from
	// filepath.WalkDir which yields clean paths, so a raw prefix check
	// is correct and intentional.
	for _, p := range got {
		if !strings.HasPrefix(p, root) {
			t.Errorf("path %q not under root %q", p, root)
		}
		if filepath.Ext(p) != ".md" {
			t.Errorf("path %q is not .md", p)
		}
	}
}

// TestWalk_SkipsHiddenDirs verifies that directories whose name starts
// with '.' are skipped (e.g. .git, .vscode, .idea). A hidden dir at any
// nesting depth is skipped, but the rest of the tree continues.
func TestWalk_SkipsHiddenDirs(t *testing.T) {
	root := t.TempDir()
	makeFile(t, root, "visible.md")
	makeFile(t, root, ".git/HEAD")
	makeFile(t, root, ".hidden/file.md")
	makeFile(t, root, "policies/.secret/file.md")
	makeFile(t, root, "policies/.DS_Store")
	makeFile(t, root, "policies/regular.md")

	got, err := walkTier(root)
	if err != nil {
		t.Fatalf("walkTier: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 visible files, got %d: %v", len(got), got)
	}
	for _, p := range got {
		base := filepath.Base(filepath.Dir(p))
		if len(base) > 0 && base[0] == '.' {
			t.Errorf("path %q is under a hidden directory", p)
		}
	}
}

// TestWalk_SkipsNonMarkdown verifies that non-.md files are not indexed.
// KnowledgeBase tier directories may legitimately contain .txt or
// image files for documentation; only .md is corpus.
func TestWalk_SkipsNonMarkdown(t *testing.T) {
	root := t.TempDir()
	makeFile(t, root, "doc.md")
	makeFile(t, root, "readme.txt")
	makeFile(t, root, "script.py")
	makeFile(t, root, "data.json")
	makeFile(t, root, "image.png")
	makeFile(t, root, "no_extension")

	got, err := walkTier(root)
	if err != nil {
		t.Fatalf("walkTier: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 .md file, got %d: %v", len(got), got)
	}
	if filepath.Base(got[0]) != "doc.md" {
		t.Errorf("expected doc.md, got %s", got[0])
	}
}

// TestWalk_LexicalSort verifies that the returned slice is in lexical
// order so chunk ID assignment (1-based, walk-order) is stable across
// server restarts. Without sort stability, citation labels would drift.
func TestWalk_LexicalSort(t *testing.T) {
	root := t.TempDir()
	// Insert in deliberately scrambled on-disk order.
	makeFile(t, root, "c.md")
	makeFile(t, root, "a.md")
	makeFile(t, root, "b/sub.md")
	makeFile(t, root, "b/top.md")

	got, err := walkTier(root)
	if err != nil {
		t.Fatalf("walkTier: %v", err)
	}
	want := []string{
		filepath.Join(root, "a.md"),
		filepath.Join(root, "b/sub.md"),
		filepath.Join(root, "b/top.md"),
		filepath.Join(root, "c.md"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sort order mismatch\n got: %v\nwant: %v", got, want)
	}
	// Defensive: explicitly confirm the result is also what raw sort would
	// produce, to catch any future change to the sort key.
	if !sort.StringsAreSorted(got) {
		t.Errorf("walkTier returned unsorted slice: %v", got)
	}
}

// TestWalk_DeterministicAcrossCalls verifies that calling walkTier twice
// on the same directory yields the same result. This guards against
// future changes that introduce non-determinism (e.g. parallel walk
// without merging).
func TestWalk_DeterministicAcrossCalls(t *testing.T) {
	root := t.TempDir()
	makeFile(t, root, "x.md")
	makeFile(t, root, "y/z.md")
	makeFile(t, root, "a/b/c/d.md")

	first, err := walkTier(root)
	if err != nil {
		t.Fatalf("walkTier first: %v", err)
	}
	second, err := walkTier(root)
	if err != nil {
		t.Fatalf("walkTier second: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("non-deterministic walker\nfirst:  %v\nsecond: %v", first, second)
	}
}

// TestWalk_EmptyDirReturnsEmpty verifies that an empty directory produces
// an empty slice and no error (rather than nil and an error). The caller
// (Index) relies on this to skip empty slug dirs without spurious failures.
func TestWalk_EmptyDirReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	got, err := walkTier(root)
	if err != nil {
		t.Fatalf("walkTier: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}
