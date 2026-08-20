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

// main_test.go is Plan 11-08 Task 2's companion test for the
// spike-hitl binary. TestE2EApprovalFlow invokes the exported
// runE2EApprovalFlow directly (no subprocess) so the acceptance
// gate's Stage 6 runs as a plain `go test` with no shell-out cost.

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// TestE2EApprovalFlow — Plan 11-08 / HITL-01..08 single-binary E2E
// smoke. Builds an in-memory sqlite store, runs the 6-step pipeline,
// and asserts:
//
//   - The run completes without error.
//   - The audit chain contains exactly 1 pending + 1 approved row
//     for the smoke tool_call_id, in INSERT-only monotonic order
//     (pending.id < approved.id).
//   - sha256(actual_args) == sha256(audit_row.frozen_args) for the
//     approved row (HITL-07 byte-identity anchor).
//   - The report fields match the expected approval_id, row count,
//     frozen_args sha256, and executed=true.
//
// Hermetic: in-memory sqlite + scripted ChatModel + stub embedder /
// retriever; NO live LLM / embedding / external DB.
func TestE2EApprovalFlow(t *testing.T) {
	ctx := context.Background()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:", MaxOpenConns: 1})
	require.NoError(t, err, "sqlite.New(:memory:) must succeed")
	defer func() { _ = s.Close() }()

	report, err := runE2EApprovalFlow(ctx, s, smokeSessionID, smokeSkillID)
	require.NoError(t, err, "runE2EApprovalFlow must complete the 6-step pipeline")
	require.NotNil(t, report, "report must be non-nil on success")

	// Report shape — the acceptance gate parses these fields.
	assert.Equal(t, "approve-final", report.Stage)
	assert.Equal(t, smokeToolCallID, report.ApprovalID)
	assert.True(t, report.Executed, "the approve path must execute the write")

	// Audit chain — pending + approved rows in monotonic order.
	pending, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID:  smokeSessionID,
		Action:     "action_pending_approval",
		ToolCallID: smokeToolCallID,
		Limit:      1,
	})
	require.NoError(t, err)
	require.Len(t, pending, 1, "exactly one action_pending_approval row")

	approved, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID:  smokeSessionID,
		Action:     "action_approved",
		ToolCallID: smokeToolCallID,
		Limit:      1,
	})
	require.NoError(t, err)
	require.Len(t, approved, 1, "exactly one action_approved row")

	assert.Less(t, pending[0].ID, approved[0].ID,
		"INSERT-only monotonic order: pending.id must precede approved.id")

	// HITL-07 byte-identity: sha256(approved.frozen_args) == sha256(actual_args).
	expectedSHA := sha256.Sum256([]byte(smokeToolArgs))
	actualSHA := sha256.Sum256(approved[0].FrozenArgs)
	assert.Equal(t, hex.EncodeToString(expectedSHA[:]), hex.EncodeToString(actualSHA[:]),
		"sha256(actual_args) must equal sha256(audit_row.frozen_args) for the approved row")

	// The report's FrozenArgsSHA256 matches the same anchor.
	assert.Equal(t, hex.EncodeToString(expectedSHA[:]), report.FrozenArgsSHA256,
		"report.FrozenArgsSHA256 must equal sha256(smokeToolArgs)")

	// Row count — the operator chain is 2 rows (pending + approved);
	// the auto-pass chain (not exercised here) would be 1.
	assert.Equal(t, 2, report.AuditChainRows,
		"operator chain audit rows = pending + approved")
}

// TestE2EApprovalFlowRejectsNilStore — negative proof: the flow must
// fail loudly (not panic) on a nil store so the acceptance gate's
// [HITL-E2E-FAIL] prefix fires with a usable step name.
func TestE2EApprovalFlowRejectsNilStore(t *testing.T) {
	ctx := context.Background()
	report, err := runE2EApprovalFlow(ctx, nil, smokeSessionID, smokeSkillID)
	require.Error(t, err, "nil store must surface an error")
	assert.Nil(t, report)
	assert.Contains(t, err.Error(), "store is nil")
}

// TestE2EApprovalFlowRejectsEmptyIdentity — negative proof: sessionID
// and skillID are required (the audit chain + checkpoint ID derive
// from them).
func TestE2EApprovalFlowRejectsEmptyIdentity(t *testing.T) {
	ctx := context.Background()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:", MaxOpenConns: 1})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	if _, err := runE2EApprovalFlow(ctx, s, "", smokeSkillID); err == nil {
		t.Fatal("empty sessionID must error")
	}
	if _, err := runE2EApprovalFlow(ctx, s, smokeSessionID, ""); err == nil {
		t.Fatal("empty skillID must error")
	}
}
