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

package sqlite_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentApproveDenyIdempotency — D11-C4 cross-device race moat.
//
// Ten goroutines race the SAME tool_call_id into the
// action_pending_approval state. The partial unique index
// idx_audit_pending_tcid lets exactly ONE win; the other nine get a
// UNIQUE constraint error, which the store returns rather than swallows
// (the caller reads it as "already pending" and short-circuits).
//
// The follow-up assertions prove the constraint is PENDING-scoped: the
// downstream lifecycle rows (approved / rejected) are free to reuse the
// same tool_call_id, which is what makes the audit chain joinable.
func TestConcurrentApproveDenyIdempotency(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "concurrent.db")
	s, err := sqlite.New(ctx, sqlite.Config{DSN: dsn, MaxOpenConns: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	var (
		wg        sync.WaitGroup
		start     = make(chan struct{})
		mu        sync.Mutex
		successes int
		failures  int
	)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, insErr := s.InsertPendingApproval(ctx, store.PendingApprovalRow{
				Timestamp:      time.Now().UTC(),
				SessionID:      "sess-race",
				UserID:         "user-1",
				SkillID:        "compliance",
				Query:          "memory:facts",
				PromptHash:     "p-hash",
				SkillVersion:   "v1",
				ModelVersion:   "glm-5.2",
				FrozenArgs:     []byte(`{"x":1}`),
				ToolCallID:     "tc-race",
				ApprovalID:     "tc-race",
				ScopeSessionID: "sess-race",
			})
			mu.Lock()
			if insErr == nil {
				successes++
			} else {
				failures++
			}
			mu.Unlock()
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, 1, successes, "exactly ONE of 10 concurrent pending INSERTs may succeed (D11-C4)")
	assert.Equal(t, 9, failures, "the other 9 must fail on the UNIQUE constraint")

	rows, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-race", Limit: 50})
	require.NoError(t, err)
	pendingCount := 0
	for _, r := range rows {
		if r.Action == "action_pending_approval" && r.ToolCallID == "tc-race" {
			pendingCount++
		}
	}
	assert.Equal(t, 1, pendingCount, "exactly 1 pending row survives the race")

	// The partial index is PENDING-scoped: the resolution rows may (and
	// must) carry the same tool_call_id.
	_, err = s.MarkApproved(ctx, store.ApprovalResolutionRow{
		Timestamp: time.Now().UTC(), SessionID: "sess-race", UserID: "user-1",
		SkillID: "compliance", Query: "memory:facts", PromptHash: "p-hash",
		SkillVersion: "v1", RefusalReason: "operator ok",
		FrozenArgs: []byte(`{"x":1}`), ToolCallID: "tc-race", ApprovalID: "tc-race",
		ScopeSessionID: "sess-race",
	})
	require.NoError(t, err, "an approved row may reuse the pending row's tool_call_id")

	_, err = s.MarkRejected(ctx, store.ApprovalResolutionRow{
		Timestamp: time.Now().UTC(), SessionID: "sess-race", UserID: "user-1",
		SkillID: "compliance", Query: "memory:facts", PromptHash: "p-hash",
		SkillVersion: "v1", RefusalReason: "operator changed their mind",
		FrozenArgs: []byte(`{"x":1}`), ToolCallID: "tc-race", ApprovalID: "tc-race",
		ScopeSessionID: "sess-race",
	})
	require.NoError(t, err, "the unique index is scoped to action_pending_approval only")
}
