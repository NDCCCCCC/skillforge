// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// external_query_tool_test.go — v1.1.1 hermetic suite for
// BuildExternalQueryTool. Fully offline: the NL2SQL model is a local
// canned-response fake (no live LLM), the warehouse side is a seeded
// MemorySource, persistence is a MemoryStore, and the registry loads
// through the real LoadViewsDir from a temp-dir controlled file.
//
// Locked behaviors:
//
//	① fresh call → status "pending" with an xreq- id and the SQL
//	   preview VERBATIM from nl2sql (the tool is a pass-through —
//	   DATA-09);
//	② after Manager.Approve, a second call with query_id → status
//	   "executed" with rows masked by the view's field masks
//	   (DATA-04 + DATA-05);
//	③ re-invoking a still-pending request with its query_id →
//	   "pending" again (DATA-13: pending is a positive state — no
//	   auto-resolve, no poll, no block);
//	④ unknown query_id → structured refusal request_not_found;
//	⑤ a model emitting a non-select preview → structured refusal
//	   sql_preview_rejected (DATA-11);
//	⑥ structural lock: zero uppercase verb tokens in the tool source
//	   outside comment lines — the tool never writes a query itself.
package agenttools_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agenttools "github.com/skillforge/skillforge/internal/agent/tools"
	"github.com/skillforge/skillforge/internal/externalauth"
)

// extAuthFixtureView mirrors the shipped reference view schema.
const extAuthFixtureView = `
views:
  - view_name: v_user_consumption
    source: test fixture — read-only consumption replica
    columns:
      - name: user_id
        type: string
      - name: amount
        type: float
      - name: account_no
        type: string
      - name: phone
        type: string
      - name: email
        type: string
    field_masks:
      - field_name: account_no
        kind: account
        keep_len: 4
      - field_name: phone
        kind: phone
        keep_len: 4
      - field_name: email
        kind: email
        keep_len: 2
`

// cannedPreviewModel is the local fake model.ToolCallingChatModel:
// Generate returns the canned nl2sql-shaped message, everything else is
// unused by the tool under test.
type cannedPreviewModel struct{ content string }

func (m cannedPreviewModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: m.content}, nil
}

func (m cannedPreviewModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("cannedPreviewModel: Stream not implemented")
}

func (m cannedPreviewModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

var _ model.ToolCallingChatModel = cannedPreviewModel{}

// goodPreview is a nl2sql-shaped response naming a registered view.
const goodPreview = "VIEW: v_user_consumption\nSQL: SELECT user_id, account_no FROM v_user_consumption\nESTIMATE: 3\n"

// badPreview is a nl2sql-shaped response whose statement is a mutation
// attempt — nl2sql's gate must reject it.
const badPreview = "VIEW: v_user_consumption\nSQL: DROP VIEW v_user_consumption\nESTIMATE: 1\n"

// newExternalToolFixture builds the tool over a real manager + registry
// + seeded source, and returns the tool plus the manager (so tests can
// drive the approval path directly).
func newExternalToolFixture(t *testing.T, modelContent string) (tool.InvokableTool, *externalauth.Manager) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v_user_consumption.yaml"), []byte(extAuthFixtureView), 0o644))
	reg, err := externalauth.LoadViewsDir(dir)
	require.NoError(t, err)

	src := externalauth.NewMemorySource(reg)
	require.NoError(t, src.UpsertView("v_user_consumption", []map[string]string{
		{"user_id": "u-001", "amount": "1500.00", "account_no": "6225880137799001", "phone": "13812345678", "email": "alice@example.com"},
	}))
	mgr := externalauth.NewManager(externalauth.NewMemoryStore(), src, reg)

	tl := agenttools.BuildExternalQueryTool(agenttools.ExternalQueryToolConfig{
		Manager:   mgr,
		ChatModel: cannedPreviewModel{content: modelContent},
		Registry:  reg,
		SessionID: "sess-ext-1",
		SkillID:   "external-lookup",
	})
	require.NotNil(t, tl)
	info, err := tl.Info(context.Background())
	require.NoError(t, err)
	require.Equal(t, agenttools.ExternalQueryToolName, info.Name)
	return tl, mgr
}

type externalToolResponse struct {
	Status           string              `json:"status"`
	ReasonCode       string              `json:"reason_code"`
	HumanExplanation string              `json:"human_explanation"`
	QueryID          string              `json:"query_id"`
	SQLPreview       string              `json:"sql_preview"`
	View             string              `json:"view"`
	EstimatedRows    int                 `json:"estimated_rows"`
	Rows             []map[string]string `json:"rows"`
	Count            int                 `json:"count"`
	MaskApplied      []string            `json:"mask_applied"`
	Message          string              `json:"message"`
}

func invokeExternalTool(t *testing.T, tl tool.InvokableTool, argsJSON string) externalToolResponse {
	t.Helper()
	raw, err := tl.InvokableRun(context.Background(), argsJSON)
	require.NoError(t, err, "every refusal is a tool result, never a Go error (D16-B2)")
	require.NotEmpty(t, raw)
	var resp externalToolResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	return resp
}

// TestExternalTool_FreshCallPending — ① the preview path.
func TestExternalTool_FreshCallPending(t *testing.T) {
	tl, _ := newExternalToolFixture(t, goodPreview)

	resp := invokeExternalTool(t, tl, `{"query":"查一下上月每个员工的消费金额","business_reason":"费用复核"}`)
	assert.Equal(t, "pending", resp.Status, "body: %+v", resp)
	assert.True(t, strings.HasPrefix(resp.QueryID, "xreq-"), "query_id must be xreq-prefixed, got %q", resp.QueryID)
	// The preview is passed through VERBATIM from nl2sql (canonicalized
	// whitespace only) — the tool never authors it.
	assert.Equal(t, "SELECT user_id, account_no FROM v_user_consumption", resp.SQLPreview)
	assert.Equal(t, "v_user_consumption", resp.View)
	assert.Equal(t, 3, resp.EstimatedRows)
	assert.Contains(t, resp.MaskApplied, "account_no")
	assert.NotEmpty(t, resp.Message, "the pending result must instruct the agent to surface the approval card")
}

// TestExternalTool_ApproveThenExecuteMaskedRows — ② the post-approval
// continuation returns masked rows.
func TestExternalTool_ApproveThenExecuteMaskedRows(t *testing.T) {
	tl, mgr := newExternalToolFixture(t, goodPreview)

	pending := invokeExternalTool(t, tl, `{"query":"上月员工消费","business_reason":"费用复核"}`)
	require.Equal(t, "pending", pending.Status)

	_, _, _, err := mgr.Approve(context.Background(), pending.QueryID, "officer-1", "verified", nil)
	require.NoError(t, err)

	executed := invokeExternalTool(t, tl, `{"query":"上月员工消费","query_id":"`+pending.QueryID+`"}`)
	assert.Equal(t, "executed", executed.Status, "body: %+v", executed)
	require.Len(t, executed.Rows, 1)
	acct := executed.Rows[0]["account_no"]
	assert.Equal(t, "9001", acct[len(acct)-4:], "the last 4 digits stay visible")
	assert.True(t, strings.Trim(acct[:len(acct)-4], "*") == "",
		"account number must be masked beyond the last 4, got %q", acct)
	assert.Equal(t, 1, executed.Count)
	assert.Contains(t, executed.MaskApplied, "account_no")
}

// TestExternalTool_PendingStaysPending — ③ DATA-13: re-invoking a
// pending request returns the pending shape again.
func TestExternalTool_PendingStaysPending(t *testing.T) {
	tl, _ := newExternalToolFixture(t, goodPreview)

	first := invokeExternalTool(t, tl, `{"query":"上月员工消费"}`)
	require.Equal(t, "pending", first.Status)

	again := invokeExternalTool(t, tl, `{"query":"上月员工消费","query_id":"`+first.QueryID+`"}`)
	assert.Equal(t, "pending", again.Status, "pending must be a positive state — no auto-resolve")
	assert.Equal(t, first.QueryID, again.QueryID)
	assert.NotEmpty(t, again.Message)
}

// TestExternalTool_UnknownQueryIDRefused — ④ a fabricated query_id is a
// refusal, never an error.
func TestExternalTool_UnknownQueryIDRefused(t *testing.T) {
	tl, _ := newExternalToolFixture(t, goodPreview)
	resp := invokeExternalTool(t, tl, `{"query":"上月员工消费","query_id":"xreq-99999999"}`)
	assert.Equal(t, "refused", resp.Status)
	assert.Equal(t, "request_not_found", resp.ReasonCode)
	assert.NotEmpty(t, resp.HumanExplanation)
}

// TestExternalTool_NonSelectPreviewRefused — ⑤ DATA-11: the model
// emitting a mutation attempt never reaches the approval pipeline.
func TestExternalTool_NonSelectPreviewRefused(t *testing.T) {
	tl, _ := newExternalToolFixture(t, badPreview)
	resp := invokeExternalTool(t, tl, `{"query":"把消费表删了"}`)
	assert.Equal(t, "refused", resp.Status)
	assert.Equal(t, "sql_preview_rejected", resp.ReasonCode)
	assert.NotEmpty(t, resp.HumanExplanation)
}

// TestExternalTool_MissingQueryRefused — empty query is a structured
// invalid_arguments refusal.
func TestExternalTool_MissingQueryRefused(t *testing.T) {
	tl, _ := newExternalToolFixture(t, goodPreview)
	resp := invokeExternalTool(t, tl, `{"query":"   "}`)
	assert.Equal(t, "refused", resp.Status)
	assert.Equal(t, "invalid_arguments", resp.ReasonCode)
}

// TestExternalTool_NilManagerRefused — defense in depth: even if a
// caller bypasses BuildRunner's fail-fast, the tool refuses instead of
// panicking.
func TestExternalTool_NilManagerRefused(t *testing.T) {
	tl := agenttools.BuildExternalQueryTool(agenttools.ExternalQueryToolConfig{
		Manager:   nil,
		ChatModel: cannedPreviewModel{content: goodPreview},
	})
	resp := invokeExternalTool(t, tl, `{"query":"上月员工消费"}`)
	assert.Equal(t, "refused", resp.Status)
	assert.Equal(t, "external_auth_not_configured", resp.ReasonCode)
}

// TestExternalToolNoSQLSurface is the DATA-09 structural lock: the tool
// source file must contain zero uppercase statement-verb tokens outside
// comment lines. Comment lines are stripped BEFORE matching so the lock
// cannot trip on its own prose. The canned model response inside THIS
// test file is deliberately exempt — the lock targets the tool source
// only.
func TestExternalToolNoSQLSurface(t *testing.T) {
	src, err := os.ReadFile("external_query_tool.go")
	require.NoError(t, err)
	var kept []string
	for _, line := range strings.Split(string(src), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		kept = append(kept, line)
	}
	code := strings.Join(kept, "\n")
	for _, verb := range []string{"SELECT ", "INSERT ", "UPDATE ", "DELETE "} {
		assert.NotContains(t, code, verb,
			"external_query_tool.go contains %q outside comments — the tool must never author a statement itself (DATA-09 pass-through invariant)", verb)
	}
}
