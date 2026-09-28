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

package ecl

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
)

// asciiEntityNamePattern gates the PascalCase fast path: names made only of
// [0-9A-Za-z_] get readable local names; everything else (the CJK business
// vocabulary this corpus actually yields — 车险, 分公司, 湖北分公司…) takes
// the hash path.
var asciiEntityNamePattern = regexp.MustCompile(`^[0-9A-Za-z_]+$`)

// EntityIRI renders the ABox individual for an extracted business entity
// (D-NZK-05): {ns}ent_{local}.
//
// Determinism is the D-NZK-01 identity contract: entity identity = the
// exact (trimmed) name, so the same name MUST always yield the same IRI —
// across notes, sections, pipeline re-runs, and boots. That determinism is
// what makes store.InsertUnique dedupe entity individuals globally when
// several notes mention 车险.
//
// Why emitter.SanitizeLocal alone is insufficient: it rewrites every
// non-ASCII rune to "_", so 车险 collapses to "___" — and any two CJK names
// of equal length (车险 vs 农险) would fuse into ONE individual, silently
// merging distinct business vocabulary. entityLocalName therefore splits:
//
//   - pure-ASCII names: a deterministic PascalCase transform (readable
//     local names for the rare Latin entity)
//   - everything else: "e" + first 12 hex chars of sha256(name) — a
//     fixed-width, deterministic, collision-free-for-practical-purposes
//     encoding; no raw LLM text ever reaches the IRI (T-NZK-02, same
//     defense as TableIRI's sanitizer)
//
// Same-batch collisions (two DISTINCT names whose local names coincide,
// e.g. "ab" vs "AB" → "Ab") are resolved by BuildTriples with ordinal
// suffixes (-2, -3, …) within that batch; cross-batch, the CJK path's
// 48-bit hash space makes fusion negligible and the corpus yields no
// ASCII-named entities in practice. stdlib only (T-NZK-SC).
func EntityIRI(ns, name string) string {
	return ns + "ent_" + entityLocalName(name)
}

// entityLocalName computes the entity individual's local name (see
// EntityIRI for the split rationale).
func entityLocalName(name string) string {
	if asciiEntityNamePattern.MatchString(name) {
		return asciiPascalCase(name)
	}
	sum := sha256.Sum256([]byte(name))
	return "e" + hex.EncodeToString(sum[:])[:12]
}

// asciiPascalCase splits on "_" and title-cases each part (first rune
// upper, remaining runes lower). Deterministic by construction: "che_xian"
// → "CheXian", "ABC" → "Abc", "a_b_c" → "ABC".
func asciiPascalCase(name string) string {
	var b strings.Builder
	for _, part := range strings.Split(name, "_") {
		if part == "" {
			continue
		}
		runes := []rune(part)
		b.WriteRune(unicode.ToUpper(runes[0]))
		for _, r := range runes[1:] {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
