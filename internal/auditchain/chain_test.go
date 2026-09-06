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

package auditchain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

type fakeStore struct {
	rows []store.AuditRow
}

func (f *fakeStore) AuditBySessionAt(_ context.Context, _ string, tUnixMs int64) ([]store.AuditRow, error) {
	out := make([]store.AuditRow, 0, len(f.rows))
	for _, r := range f.rows {
		if r.Timestamp.UnixMilli() <= tUnixMs {
			out = append(out, r)
		}
	}
	return out, nil
}

func encodeRulePK(t *testing.T, target, argsHash, skillID, scope string) []byte {
	t.Helper()
	b, err := json.Marshal(rulePKPayload{Target: target, ArgsHash: argsHash, SkillID: skillID, ScopeSessionID: scope})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return []byte(base64.RawURLEncoding.EncodeToString(b))
}

func at(sec int64) time.Time { return time.Unix(sec, 0) }

func TestChain_VerdictAndApprovalLifecycle(t *testing.T) {
	ctx := context.Background()
	fs := &fakeStore{rows: []store.AuditRow{
		{ID: 1, Timestamp: at(100), SessionID: "s", SkillID: "compliance", Action: "", ChunkIDs: "c1,c2"},
		{ID: 2, Timestamp: at(105), SessionID: "s", SkillID: "compliance", Action: "action_pending_approval", ToolCallID: "tc-1", ApprovalID: "tc-1", FrozenArgs: []byte(`{"hello":"world"}`)},
		{ID: 3, Timestamp: at(110), SessionID: "s", SkillID: "compliance", Action: "action_approved", ToolCallID: "tc-1", ApprovalID: "tc-1"},
		{ID: 4, Timestamp: at(115), SessionID: "s", SkillID: "compliance", Action: "action_rule_created", FrozenArgs: encodeRulePK(t, "tool_x", "h1", "compliance", "s")},
		{ID: 5, Timestamp: at(130), SessionID: "s", SkillID: "compliance", Action: "action_rule_revoked", FrozenArgs: encodeRulePK(t, "tool_x", "h1", "compliance", "s")},
	}}
	chain, err := ChainForSession(ctx, fs, "s", 100)
	if err != nil {
		t.Fatalf("chain: %v", err)
	}
	if len(chain) != 5 {
		t.Fatalf("expected 5 links, got %d", len(chain))
	}
	wantKinds := []LinkKind{LinkVerdict, LinkPendingApproval, LinkApproved, LinkRuleCreated, LinkRuleRevoked}
	for i, l := range chain {
		if l.Kind != wantKinds[i] {
			t.Errorf("link %d: kind=%s want %s", i, l.Kind, wantKinds[i])
		}
	}
	if chain[0].ChunkIDs != "c1,c2" {
		t.Errorf("verdict link missing chunk_ids: %+v", chain[0])
	}
	if chain[1].ToolCallID != "tc-1" || string(chain[1].FrozenArgsJSON) != `{"hello":"world"}` {
		t.Errorf("pending link lost fields: %+v", chain[1])
	}
}

func TestStateAtSession_RuleRevokedBeforeT_RuleHidden(t *testing.T) {
	ctx := context.Background()
	fs := &fakeStore{rows: []store.AuditRow{
		{ID: 1, Timestamp: at(100), SessionID: "s", SkillID: "compliance", Action: "action_rule_created", FrozenArgs: encodeRulePK(t, "tool_x", "h1", "compliance", "s")},
		{ID: 2, Timestamp: at(120), SessionID: "s", SkillID: "compliance", Action: "action_rule_revoked", FrozenArgs: encodeRulePK(t, "tool_x", "h1", "compliance", "s")},
	}}
	// Query at t=110 (between create and revoke): rule must be live.
	state, err := StateAtSession(ctx, fs, "s", at(110))
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if len(state.Rules) != 1 || state.Rules[0].Target != "tool_x" {
		t.Fatalf("expected rule live at t=110, got %+v", state.Rules)
	}
	// Query at t=130 (after revoke): rule must be gone.
	state, err = StateAtSession(ctx, fs, "s", at(130))
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if len(state.Rules) != 0 {
		t.Fatalf("expected rule revoked by t=130, got %+v", state.Rules)
	}
}

func TestStateAtSession_SkillScopeVisibleToOtherSessions(t *testing.T) {
	ctx := context.Background()
	// Rule created with scope_session_id="" (skill-scope) → visible to ALL sessions.
	fs := &fakeStore{rows: []store.AuditRow{
		{ID: 1, Timestamp: at(100), SessionID: "creator", SkillID: "compliance", Action: "action_rule_created", FrozenArgs: encodeRulePK(t, "tool_x", "h1", "compliance", "")},
	}}
	state, err := StateAtSession(ctx, fs, "other-session", at(200))
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if len(state.Rules) != 1 {
		t.Fatalf("skill-scope rule must be visible to other sessions, got %+v", state.Rules)
	}
}

func TestStateAtSession_SessionScopeHiddenFromOtherSessions(t *testing.T) {
	ctx := context.Background()
	fs := &fakeStore{rows: []store.AuditRow{
		{ID: 1, Timestamp: at(100), SessionID: "creator", SkillID: "compliance", Action: "action_rule_created", FrozenArgs: encodeRulePK(t, "tool_x", "h1", "compliance", "creator")},
	}}
	state, err := StateAtSession(ctx, fs, "other-session", at(200))
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if len(state.Rules) != 0 {
		t.Fatalf("session-scope rule must NOT be visible to other sessions, got %+v", state.Rules)
	}
}

func TestChain_EmptySession(t *testing.T) {
	ctx := context.Background()
	chain, err := ChainForSession(ctx, &fakeStore{}, "nope", 0)
	if err != nil {
		t.Fatalf("chain: %v", err)
	}
	if len(chain) != 0 {
		t.Fatalf("empty session should yield empty chain, got %d", len(chain))
	}
}

func TestChain_DeterministicOrdering(t *testing.T) {
	ctx := context.Background()
	fs := &fakeStore{rows: []store.AuditRow{
		{ID: 3, Timestamp: at(110), SessionID: "s", SkillID: "x", Action: ""},
		{ID: 1, Timestamp: at(100), SessionID: "s", SkillID: "x", Action: ""},
		{ID: 2, Timestamp: at(105), SessionID: "s", SkillID: "x", Action: ""},
	}}
	chain, err := ChainForSession(ctx, fs, "s", 0)
	if err != nil {
		t.Fatalf("chain: %v", err)
	}
	for i := 0; i < len(chain)-1; i++ {
		if chain[i].AuditID > chain[i+1].AuditID {
			t.Fatalf("chain not sorted by audit_id: %d > %d", chain[i].AuditID, chain[i+1].AuditID)
		}
	}
}
