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

package server

import (
	"fmt"
	"regexp"
	"strings"
)

// extractSkillName derives a Skill directory name from the first
// sentence of the wizard's purpose field (SC-3 / D5-10). The result
// matches ^[a-z0-9-]+$ — the same regex the wizard's Commit gate uses
// (T-5-16). If no extractable name satisfies the regex, an error is
// returned so the FE can prompt the user to clarify (UI-SPEC §SC-3).
//
// Slugification rules (deterministic, language-agnostic on the ASCII
// pass — Chinese input still produces a name as long as SOME ASCII
// alphanumerics or dashes are present):
//
//  1. Trim leading/trailing whitespace.
//  2. Take everything up to the first sentence terminator
//     (。！？.!?\n). If no terminator, take the whole trimmed string.
//  3. Lowercase ASCII letters; strip non-ASCII-alphanumeric + non-dash
//     characters; collapse runs of dashes to a single dash; trim
//     leading/trailing dashes.
//  4. If the slug is empty or non-matching, return an error with the
//     UI hint. The FE shows this copy so the user can rephrase the
//     purpose to include an English keyword (or an explicit Latin
//     identifier).
var (
	wizardNameSlugNonAllowed = regexp.MustCompile(`[^a-z0-9-]+`)
	wizardNameSlugDash       = regexp.MustCompile(`-+`)
)

// extractSkillName implements the slugification described above.
// Extracted to a separate file so tests can hit it directly (no HTTP
// wiring required).
func extractSkillName(purpose string) (string, error) {
	trimmed := strings.TrimSpace(purpose)
	if trimmed == "" {
		return "", fmt.Errorf("purpose is required and must contain an English keyword or Latin identifier (the Skill name is derived from the first sentence)")
	}

	// 2. Sentence terminator detection. We treat both CN and EN
	// terminators as cut points; the slug is whatever appears before
	// the FIRST terminator.
	cut := -1
	for i, r := range trimmed {
		if r == '。' || r == '！' || r == '？' ||
			r == '.' || r == '!' || r == '?' || r == '\n' {
			cut = i
			break
		}
	}
	if cut >= 0 {
		trimmed = trimmed[:cut]
	}

	// 3. Walk the runes, mapping each into one of: lowercase ASCII letter
	// / digit / dash. Any non-allowed rune between two allowed ones is
	// collapsed to a dash so "My QA" → "my-qa" and "审批流 Skill" →
	// "skill" (the dash acts as a word boundary).
	var b strings.Builder
	prevAllowed := false
	for _, r := range trimmed {
		if r >= 'A' && r <= 'Z' {
			r = r - 'A' + 'a'
		}
		allowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		switch {
		case allowed && r == '-':
			// Preserve an explicit dash only if the previous emitted
			// rune was an allowed non-dash (so we don't double up).
			if prevAllowed && b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteRune('-')
			}
			// prevAllowed stays true so two explicit dashes in a row
			// still produce a single dash after the collapse regex.
		case allowed:
			b.WriteRune(r)
			prevAllowed = true
		default:
			// Non-allowed char between two allowed runes → emit a dash
			// (word-boundary heuristic). At the very start or after
			// another non-allowed char, emit nothing.
			if prevAllowed && b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteRune('-')
			}
			// prevAllowed stays false until next allowed char.
		}
	}
	slug := b.String()
	// Drop a trailing dash before the collapse (a non-allowed char at
	// the end would otherwise leave "foo-" intact).
	slug = strings.TrimSuffix(slug, "-")
	// Collapse runs of dashes — defensive (single dash is allowed at
	// this point; only run-collapse is needed).
	slug = wizardNameSlugDash.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if slug == "" {
		return "", fmt.Errorf(
			"the purpose %q did not yield a usable Skill name (must contain English letters/digits, e.g. %q)",
			purpose, "compliance-audit",
		)
	}
	// Defensive: confirm the final slug matches the wizard regex. If
	// it doesn't (which would mean a future regex change), the wizard
	// commit path will surface a clearer error — but we still report
	// the failure here so the FE gets it before the user re-confirms.
	if !skillNamePatternForWizard.MatchString(slug) {
		return "", fmt.Errorf("derived Skill name %q does not match ^[a-z0-9-]+$", slug)
	}
	return slug, nil
}

// skillNamePatternForWizard is the wizard's Skill name regex. Kept in
// this file so extractSkillName can re-verify the slug without an
// import into internal/skill. The same regex is defined inside
// internal/skill/wizard.go — duplication is intentional and small (a
// few characters) because the wizard name gate is a server-layer
// concern (it's enforced before the FE ever sees the preview) and the
// skill layer's gate is the actual writer (defense in depth).
var skillNamePatternForWizard = regexp.MustCompile(`^[a-z0-9-]+$`)
