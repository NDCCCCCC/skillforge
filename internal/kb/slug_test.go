// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package kb

import (
	"strings"
	"testing"
)

// TestValidateSlugAccepts locks the slug whitelist happy path, including
// the exact upper bound MaxSlugLen. Role-named slugs are legal here: the
// baseline seed KBs are named after their roles (see ValidateNewSlug for
// the creation-time reservation).
func TestValidateSlugAccepts(t *testing.T) {
	accepts := []string{
		"authority",
		"historical",
		"kb-2026",
		"a",
		"a0",
		strings.Repeat("a", MaxSlugLen), // exact upper bound must pass
	}
	for _, s := range accepts {
		if err := ValidateSlug(s); err != nil {
			t.Errorf("ValidateSlug(%q) = %v, want nil", s, err)
		}
	}
}

// TestValidateSlugRejects locks every fail-closed rejection: pattern
// violations and reserved words. Each reserved-word rejection must name
// the conflict reason, not just say "invalid".
func TestValidateSlugRejects(t *testing.T) {
	rejects := []string{
		"",
		"Authority",                       // uppercase
		"1abc",                            // digit-leading
		"a_b",                             // underscore
		"a b",                             // whitespace
		strings.Repeat("a", MaxSlugLen+1), // over the bound
		"./escape",
		"../cmd", // traversal attempt
		"数据",     // non-ascii
	}
	for _, s := range rejects {
		if err := ValidateSlug(s); err == nil {
			t.Errorf("ValidateSlug(%q) = nil, want error", s)
		} else if !strings.Contains(err.Error(), "must match") {
			t.Errorf("ValidateSlug(%q) error %q must quote the whitelist pattern", s, err)
		}
	}

	// "data" is reserved in every authoring context: it collides with the
	// read-channel prefix, so it must never name a knowledge base.
	err := ValidateSlug("data")
	if err == nil {
		t.Fatal("ValidateSlug(\"data\") = nil, want error (channel-prefix reservation)")
	}
	if !strings.Contains(err.Error(), "data:") {
		t.Errorf("reserved-word error %q must name the channel-prefix conflict", err)
	}

	// Role names are refused at creation time (ValidateNewSlug): a KB
	// named after a role would blur the role wildcard.
	for _, r := range Roles() {
		err := ValidateNewSlug(string(r))
		if err == nil {
			t.Errorf("ValidateNewSlug(%q) = nil, want error (role-name reservation)", r)
			continue
		}
		if !strings.Contains(err.Error(), "role") {
			t.Errorf("ValidateNewSlug(%q) error %q must name the role-wildcard conflict", r, err)
		}
	}
}
