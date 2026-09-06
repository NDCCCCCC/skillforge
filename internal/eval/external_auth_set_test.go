// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// external_auth_set_test.go — v1.1.1 loader-compatibility lock for the
// Phase 21 Wave-5 golden set (eval/frozen/external-auth.jsonl,
// backfilled by the v1.1.1 cleanup). The set is UNPINNED by design: no
// lock sidecar exists until an authorized live replay mints one
// (D14-B4), so this file locks the SET SHAPE, not pinned metrics.
package eval

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// externalAuthDATARe extracts the DATA number from the
// external-auth-<NN>-<facet> category convention, so the coverage
// assertion keys on the requirement number rather than the whole
// category string (which carries a free-form facet suffix).
var externalAuthDATARe = regexp.MustCompile(`^external-auth-(\d+)-`)

// TestExternalAuthGoldenSet locks the 10-case shape: parses through the
// production loader, unique ids, the external-auth- prefix on every
// category, coverage of all seven DATA requirements, exactly three
// refusal-required cases, and non-empty query + ground_truth everywhere.
func TestExternalAuthGoldenSet(t *testing.T) {
	cases, err := LoadGolden("../../eval/frozen/external-auth.jsonl")
	require.NoError(t, err, "the golden set must parse through the production loader")
	require.Len(t, cases, 10, "the Wave-5 deliverable is exactly 10 cases")

	// Ids unique.
	seen := make(map[string]bool, len(cases))
	for _, c := range cases {
		require.False(t, seen[c.ID], "duplicate case id %q", c.ID)
		seen[c.ID] = true
	}

	// Category convention + per-case completeness.
	dataSeen := make(map[int]bool)
	refusalCount := 0
	for _, c := range cases {
		assert.True(t, len(c.Category) >= len("external-auth-") && externalAuthDATARe.MatchString(c.Category),
			"case %q: category %q must carry the external-auth-<DATA>- prefix", c.ID, c.Category)
		assert.NotEmpty(t, c.Query, "case %q must carry a non-empty query", c.ID)
		assert.NotEmpty(t, c.GroundTruth, "case %q must carry a non-empty ground_truth", c.ID)

		if m := externalAuthDATARe.FindStringSubmatch(c.Category); len(m) >= 2 {
			n, convErr := strconv.Atoi(m[1])
			require.NoError(t, convErr, "case %q: DATA number must parse", c.ID)
			dataSeen[n] = true
		}
		if regexp.MustCompile(`-refusal$`).MatchString(c.ID) {
			refusalCount++
		}
	}

	// All seven DATA requirements the channel implements are covered:
	// 04 (ttl) / 05 (masking) / 06 (dual audit) / 10 (precompiled view)
	// / 11 (sql preview) / 13 (pending) / 14 (audit schema).
	for _, want := range []int{4, 5, 6, 10, 11, 13, 14} {
		assert.True(t, dataSeen[want],
			"category set must cover DATA-%02d; seen %v", want, dataSeen)
	}
	assert.Len(t, dataSeen, 7, "exactly seven distinct DATA requirements are exercised")

	// Exactly three refusal-required cases: the second sql-preview case
	// (non-select intent) and BOTH precompiled-view cases (unregistered
	// view; raw table outside the allowlist).
	assert.Equal(t, 3, refusalCount,
		"exactly 3 cases must be refusal-required (id carries the -refusal suffix)")
}

// TestExternalAuthGoldenSetUnpinned is the honest-state lock: the set
// ships WITHOUT a lock sidecar until an authorized live replay mints
// one (D14-B4 — regenerating a lock by hand is baseline tampering).
func TestExternalAuthGoldenSetUnpinned(t *testing.T) {
	_, statErr := os.Stat("../../eval/frozen/external-auth.lock.json")
	assert.True(t, os.IsNotExist(statErr),
		"external-auth.lock.json must not exist until an authorized live replay mints it")
}
