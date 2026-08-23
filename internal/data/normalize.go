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

// Package data is the Phase 15 import-pipeline domain layer (P10.1):
// declarative rule-file loading with fail-closed validation, the L1
// pre-hash normalization profiles + HMAC engine, the L2 fixed-width
// bucketing, and the streaming decode pipeline for CSV uploads.
//
// Everything in this package is deterministic pure functions or
// single-file I/O — the mechanism base every import channel (CSV /
// SQL dump / API push, Plans 15-02..15-06) reuses. The encoding
// ordering iron law for the decode pipeline lives in decode.go:
// UTF-8 detection MUST run before the GB18030 fallback.
package data

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalization profiles (Phase 15 / D15-A1 / DATA-02). The profile
// names are part of the datarules/*.yaml rule-file contract (`profile:`
// field of a hash column). Unknown names are rejected fail-closed at
// rule-load time (rules.go) with the error pointing here.
//
// The chains below are the measured, LOCKED implementation spec
// (15-RESEARCH Pattern 2 — every behavioral fact was verified against
// x/text v0.28.0 on 2026-08-23; do not change chains without re-running
// TestHMACNormalizationJoinability):
//
//	person_name: NFC → TrimSpace   (NFKC FORBIDDEN — ①→1 compatibility
//	                                folding merges distinct values, Pitfall 5)
//	phone_cn:    NFKC → strip separators (spaces + hyphens) →
//	             conditional country-code strip (see normalizePhoneCN)
//	id_card_cn:  NFKC → ToUpper → remove all whitespace
//	default:     NFC → TrimSpace   (hash column with no explicit profile)
//
// strings.TrimSpace already covers U+3000 (full-width space) because
// unicode.IsSpace includes it — no hand-rolled rune stripping.
// ProfilePersonName normalizes CN person names: NFC composition plus
// whitespace trim. NFC (not NFKC) on purpose: name glyphs carry
// compatibility distinctions that must NOT be folded before hashing.
const ProfilePersonName = "person_name"

// ProfilePhoneCN normalizes CN mobile numbers: NFKC folds full-width
// digits/symbols (１３８→138, ＋→+), separators are stripped, and the
// +86/86 country code is removed only when the remainder is exactly
// 11 digits (the join-safe reading of "validate 11 digits" — a
// non-mobile shape is never forcibly stripped).
const ProfilePhoneCN = "phone_cn"

// ProfileIDCardCN normalizes CN resident ID numbers: NFKC folds
// full-width digits and the full-width Ｘ check character, ToUpper
// maps lowercase x→X, and all whitespace is removed — every surface
// form of one ID collapses to one string before hashing.
const ProfileIDCardCN = "id_card_cn"

// KnownProfiles returns the complete list of normalization profile
// names (sorted, freshly allocated). This is the SINGLE source of truth
// for what a rule file may declare in `profile:` — rules.go validates
// against this list so a typo'd or hallucinated profile name fails at
// load time, never at import time.
//
// Adding a profile = add the constant above AND extend this table AND
// extend Normalize's switch — consumers MUST NOT grow their own copies
// (mirrors the skill.validate KnownTargets discipline, validate.go:135-144).
func KnownProfiles() []string {
	return []string{ProfileIDCardCN, ProfilePersonName, ProfilePhoneCN}
}

// Normalize applies the named profile's normalization chain to value.
// An empty profile selects the default chain (NFC → TrimSpace) used by
// hash columns that declare no explicit profile. Any other unknown
// profile is a fail-closed error pointing at KnownProfiles().
func Normalize(profile, value string) (string, error) {
	switch profile {
	case ProfilePersonName:
		return norm.NFC.String(strings.TrimSpace(value)), nil
	case ProfilePhoneCN:
		return normalizePhoneCN(value), nil
	case ProfileIDCardCN:
		return normalizeIDCardCN(value), nil
	case "":
		return norm.NFC.String(strings.TrimSpace(value)), nil
	default:
		return "", fmt.Errorf(
			"data.Normalize: unknown profile %q (known profiles: %v); "+
				"the profile must appear in internal/data/normalize.go KnownProfiles() — "+
				"do NOT invent profile names in the rule file (D15-A1 single source of truth)",
			profile, KnownProfiles())
	}
}

// normalizePhoneCN implements the phone_cn chain. The country-code strip
// is conditional: only when the value carries a +86/86 prefix AND the
// remainder is exactly 11 digits. A landline like 86-10-1234567 keeps
// its 86 — stripping it would fork joins for non-mobile shapes.
func normalizePhoneCN(value string) string {
	s := norm.NFKC.String(value)
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' {
			return -1
		}
		return r
	}, s)
	for _, prefix := range []string{"+86", "86"} {
		if strings.HasPrefix(s, prefix) {
			rest := s[len(prefix):]
			if len(rest) == 11 && isASCIIDigits(rest) {
				return rest
			}
		}
	}
	return s
}

// normalizeIDCardCN implements the id_card_cn chain: NFKC (folds
// full-width digits and Ｘ) → ToUpper (x→X) → whitespace removal.
func normalizeIDCardCN(value string) string {
	s := norm.NFKC.String(value)
	s = strings.ToUpper(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	return s
}

// isASCIIDigits reports whether s is non-empty and all ASCII 0-9.
func isASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
