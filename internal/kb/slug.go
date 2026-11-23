// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package kb

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// slugPattern is the single slug whitelist: lowercase ascii, letter
// first, then letters/digits/hyphens, total length capped by MaxSlugLen.
// The bound is composed from the constant so this file holds no second
// copy of the number. A slug becomes a directory path segment and a
// retriever filter value, so anything this pattern refuses — traversal
// segments, underscores, uppercase, non-ascii — never reaches either.
var slugPattern = regexp.MustCompile(fmt.Sprintf(`^[a-z][a-z0-9-]{0,%d}$`, MaxSlugLen-1))

// reservedSlugs is the closed set of slugs a knowledge base may never
// claim, each mapped to the reason it is forbidden. Anything in here is
// refused rather than defaulted: a collision with another namespace is a
// typo today and a silent trust-boundary blur tomorrow.
//
// The four role names stay legal as slugs of EXISTING bases (the baseline
// seed KBs are named after their roles), but they are refused for NEW
// bases — see ValidateNewSlug. "data" is refused everywhere because it
// collides with the read-channel prefix.
var reservedSlugs = map[string]string{
	"data":       `collides with the "data:" read-channel prefix (D16-C2)`,
	"authority":  `names a semantic role and would blur the "role:" read wildcard`,
	"historical": `names a semantic role and would blur the "role:" read wildcard`,
	"personal":   `names a semantic role and would blur the "role:" read wildcard`,
	"draft":      `names a semantic role and would blur the "role:" read wildcard`,
}

// validateSlugSyntax applies the whitelist pattern and quotes it in the
// error so the author sees the rule, not just a verdict.
func validateSlugSyntax(s string) error {
	if !slugPattern.MatchString(s) {
		return fmt.Errorf("slug %q must match %s", s, slugPattern.String())
	}
	return nil
}

// ValidateSlug validates a slug in any authoring context: the controlled
// seed file, a skill reads declaration, or an admin create. It enforces
// the whitelist pattern and the channel-prefix reservation. Role names
// pass here on purpose — the baseline seed bases are named after their
// roles — so the seed file keeps loading.
func ValidateSlug(s string) error {
	if err := validateSlugSyntax(s); err != nil {
		return err
	}
	// Only the channel-prefix word is reserved in every context; role
	// names are filtered out of the reservation for existing bases.
	if reason, reserved := reservedSlugs[s]; reserved && !IsRole(s) {
		return fmt.Errorf("slug %q is reserved: %s", s, reason)
	}
	return nil
}

// ValidateNewSlug validates a slug for a NEW knowledge base (admin
// create). On top of ValidateSlug it refuses the four role names: a base
// named after a role would make the "role:<name>" read wildcard
// ambiguous, since the wildcard expands by role membership rather than by
// name. Creation time is the only gate that needs this — an existing base
// keeps its name.
func ValidateNewSlug(s string) error {
	if err := ValidateSlug(s); err != nil {
		return err
	}
	if reason, reserved := reservedSlugs[s]; reserved && IsRole(s) {
		return fmt.Errorf("slug %q is reserved: %s", s, reason)
	}
	return nil
}

// MaxFileStemLen caps a sanitized filename stem in bytes. The stem is
// composed into "<sha8>-<stem>.md" (8 + 1 + 96 + 3 = 108 bytes), which
// stays far below the 255-byte single filesystem component limit.
const MaxFileStemLen = 96

// FileStemFallback replaces a stem that sanitizes to nothing (e.g. a
// filename made only of non-whitelist characters). Uniqueness is carried
// by the sha8 prefix, so the fallback only needs to be a legal name.
const FileStemFallback = "doc"

// SanitizeFileStem reduces an uploaded filename to a path-safe stem
// (filename without extension) using the SAME character whitelist slugs
// use — lowercase a-z, digits, hyphen: A-Z folds down, every other rune
// becomes '-', boundary hyphens are trimmed. Post-sanitize the output is
// pure ASCII (whitelist guarantees every kept rune is one byte), so the
// byte cap cannot split a rune.
//
// It is deliberately NOT ValidateSlug: the stem never becomes a slug or a
// registry key — it only names the sha8-prefixed canonical file under
// knowledge/<slug>/ — so slug reservations and the letter-first rule do
// not apply. Phase 53-01 (D-53-01) uses it so an attacker-controlled
// upload filename can never escape the knowledge/<slug>/ directory or
// produce a second "." component.
func SanitizeFileStem(name string) string {
	stem := name
	if ext := filepath.Ext(name); ext != "" {
		stem = strings.TrimSuffix(name, ext)
	}
	var b strings.Builder
	for _, r := range stem {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r - 'A' + 'a')
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return FileStemFallback
	}
	if len(out) > MaxFileStemLen {
		out = out[:MaxFileStemLen]
	}
	return out
}
