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

// frozen_args.go is the HITL-07 byte-identity moat ("批的 A 不会执行成 B")
// and the D11-D1 rule-key anchor. The frozen tool-call args cross THREE
// representations — the ApprovalCard preview, the audit row's
// frozen_args column, and the bytes the Tool actually executes — and
// the invariant is that all three are BYTE-IDENTICAL. Everything in
// this file is pure: no I/O, no chat model, no clock.
package hitl

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// ErrFrozenArgsDrift is returned when the 3-way byte-equality assertion
// fails. The HITL Tool MUST surface this error verbatim (wrapped) so
// the operator sees the drift as a re-interrupt signal rather than a
// silent execution against tampered args.
var ErrFrozenArgsDrift = errors.New("hitl: frozen_args drift detected (preview != audit != executed) — re-interrupt required")

// ComputeFrozenArgsHash returns the hex SHA-256 of the raw args bytes.
// It is BOTH the D11-D1 approve-and-remember rule key AND the HITL-07
// byte-identity anchor.
//
// CRITICAL: the input is the verbatim byte slice — NEVER re-marshal
// through encoding/json before hashing. A re-marshal canonicalizes
// whitespace and key order, so two byte-identical semantic payloads
// with different wire bytes would hash equal, breaking the drift
// detection this hash exists to provide.
func ComputeFrozenArgsHash(args []byte) string {
	h := sha256.Sum256(args)
	return hex.EncodeToString(h[:])
}

// FrozenArgsBytesEqual reports byte-identity (not semantic equality).
// bytes.Equal treats nil == empty as equal, which matches the
// frozen_args contract (an absent snapshot and an empty snapshot are
// the same "nothing to compare" state upstream — the Tool never calls
// the assertion with an empty executed side).
func FrozenArgsBytesEqual(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// assertThreeWayByteEqual checks that the three representations of the
// same frozen_args are byte-identical:
//
//	previewBytes  — what the ApprovalCard rendered to the operator
//	auditFrozen   — what the audit row's frozen_args column holds
//	executedBytes — what the Tool is about to execute
//
// Returns ErrFrozenArgsDrift on ANY mismatch. All three pairs are
// checked (not transitively assumed) so a tampered middle
// representation cannot hide behind one equal pair. HITL-07 invariant.
func assertThreeWayByteEqual(previewBytes, auditFrozen, executedBytes []byte) error {
	if !FrozenArgsBytesEqual(previewBytes, executedBytes) {
		return ErrFrozenArgsDrift
	}
	if !FrozenArgsBytesEqual(auditFrozen, executedBytes) {
		return ErrFrozenArgsDrift
	}
	if !FrozenArgsBytesEqual(previewBytes, auditFrozen) {
		return ErrFrozenArgsDrift
	}
	return nil
}

// VerifyFrozenArgsRoundTrip is a defensive helper for tests and sanity
// checks: it proves the bytes are valid JSON and returns the
// re-marshaled form. Callers use it to DETECT canonicalization drift
// (round-tripped bytes != input bytes means a downstream consumer that
// re-marshals would break byte-identity); production code paths never
// execute frozen args through this function.
func VerifyFrozenArgsRoundTrip(raw []byte) ([]byte, error) {
	var anyValue any
	if err := json.Unmarshal(raw, &anyValue); err != nil {
		return nil, err
	}
	return json.Marshal(anyValue)
}
