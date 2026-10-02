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

package hitl

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFrozenArgsThreeWayByteEquality — HITL-07 byte-identity invariant
// ("批的 A 不会执行成 B"). Locks:
//
//  1. sha256 equality across the three representations (preview ==
//     audit == executed) when the bytes are identical.
//  2. ANY edit produces a different sha256 (the D11-D1 rule key is
//     sensitive to the exact bytes, never re-marshaled).
//  3. assertThreeWayByteEqual passes on identical triples.
//  4. assertThreeWayByteEqual returns ErrFrozenArgsDrift when the
//     executed side drifts.
//  5. assertThreeWayByteEqual returns ErrFrozenArgsDrift when the
//     audit side is tampered (the middle representation cannot hide
//     behind one equal pair).
func TestFrozenArgsThreeWayByteEquality(t *testing.T) {
	previewJSON := []byte(`{"key":"employee_count=150","reason":"approved budget"}`)
	auditFrozen := previewJSON
	executedArgs := json.RawMessage(previewJSON)

	h1 := ComputeFrozenArgsHash(previewJSON)
	h2 := ComputeFrozenArgsHash(auditFrozen)
	h3 := ComputeFrozenArgsHash(executedArgs)
	assert.Equal(t, h1, h2, "preview sha256 == audit sha256")
	assert.Equal(t, h1, h3, "preview sha256 == executed sha256")

	editedArgs := json.RawMessage(`{"key":"employee_count=150","reason":"approved budget (edited)"}`)
	h4 := ComputeFrozenArgsHash(editedArgs)
	assert.NotEqual(t, h1, h4, "edited args MUST produce different sha256")

	assert.Len(t, h1, 64, "sha256 hex is 64 chars")

	require.NoError(t, assertThreeWayByteEqual(previewJSON, auditFrozen, executedArgs))

	err := assertThreeWayByteEqual(previewJSON, auditFrozen, editedArgs)
	require.Error(t, err)
	assert.Equal(t, ErrFrozenArgsDrift, err)

	tamperedAudit := []byte(`{"key":"tampered","reason":"x"}`)
	err = assertThreeWayByteEqual(previewJSON, tamperedAudit, previewJSON)
	require.Error(t, err)
	assert.Equal(t, ErrFrozenArgsDrift, err)

	// ApplyFrozenArgs — the Pass-2 executed-bytes selector.
	stored := `{"key":"k","value":"v"}`
	decision := &ApprovalDecision{Decision: "approve", ApprovalID: "a1"}
	assert.Equal(t,
		json.RawMessage(stored),
		ApplyFrozenArgs(decision, stored),
		"empty FrozenArgs falls back to the stored args",
	)
	decision.FrozenArgs = json.RawMessage(`{"key":"k","value":"edited"}`)
	assert.Equal(t,
		json.RawMessage(`{"key":"k","value":"edited"}`),
		ApplyFrozenArgs(decision, stored),
		"non-empty FrozenArgs wins (HITL-07 edit-before-approve)",
	)
	assert.Equal(t,
		json.RawMessage(stored),
		ApplyFrozenArgs(nil, stored),
		"nil decision falls back to the stored args",
	)
}
