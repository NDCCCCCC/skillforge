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

// Package reprohash owns the ONE canonical reproducibility hash used by
// SkillForge. It exists to break an import cycle that would otherwise
// exist between the two consumers of the hash:
//
//   - internal/server (chat.go::computePromptHash) writes the hash onto
//     every audit row (GEVL-03 reproducibility).
//   - internal/eval (frozen.go::AssertPin) RE-computes the hash at eval
//     time and asserts byte-identity against the frozen set's
//     .lock.json pin (SC-6 / D6-13).
//
// Both packages import this leaf package, so the hash function has
// exactly ONE definition. Duplicating it in two packages would let the
// two copies silently drift — and a drifted pin is worse than no pin,
// because it fails green.
//
// Phase 6 / Q1 resolution: PromptHash covers the RENDERED SYSTEM-PROMPT
// BODY, not just (skillName, modelVersion). Before Phase 6 the hash was
// SHA-256(skillName + modelVersion), which meant editing a Skill's
// prompt body without renaming it produced an UNCHANGED hash — the
// frozen-set gate would pass while the system had actually drifted.
// See internal/server/chat.go::computePromptHash for the migration note.
package reprohash

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
)

// PromptHash returns the SHA-256 hex digest that pins one reproducible
// agent configuration: the Skill name, the rendered system-prompt body,
// and the model version.
//
// Canonical field order in the DIGEST is:
//
//	skillName → renderedPromptBody → modelVersion
//
// (The PARAMETER order is (skillName, modelVersion, renderedPromptBody)
// so the two pre-existing arguments keep their original positions at
// every call site. The digest order is fixed independently and MUST NOT
// be changed — any change invalidates every published .lock.json.)
//
// Each field is length-prefixed (8-byte little-endian) before being
// written into the digest. Without the prefix, ("ab", "c") and
// ("a", "bc") would hash identically — a trivially-forgeable collision
// class that would let a Skill rename masquerade as an unchanged
// configuration.
//
// Determinism contract: same three inputs → byte-identical output, on
// every platform, in every process. This is what makes the frozen-set
// pin (SC-6) meaningful.
func PromptHash(skillName, modelVersion, renderedPromptBody string) string {
	h := sha256.New()
	writeField(h, skillName)
	writeField(h, renderedPromptBody)
	writeField(h, modelVersion)
	return hex.EncodeToString(h.Sum(nil))
}

// writeField writes one length-prefixed field into the digest. The
// 8-byte little-endian length prefix makes the concatenation
// unambiguous (see PromptHash's collision note).
func writeField(h hash.Hash, s string) {
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], uint64(len(s)))
	_, _ = h.Write(n[:])
	_, _ = h.Write([]byte(s))
}
