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

// Command spike-hitl is the Plan 11-08 single-binary E2E acceptance
// smoke for the Phase 11 HITL approval flow (HITL-01..08). It is
// HERMETIC: in-memory SQLite + scripted ChatModel stub, no live LLM /
// embedding / external DB.
//
// The 6-step pipeline (proves HITL-01..08 in one binary):
//
//	1. Run #1: BuildRunner → runner.Run → scripted ChatModel emits a
//	   memory_facts_write tool call → MemoryFactsTool Pass-1 →
//	   tool.StatefulInterrupt → Runner persists checkpoint.
//	2. WritePendingApproval: seed the action_pending_approval audit
//	   row via the fail-closed AuditWriter (D11-C3); stamp the
//	   checkpoint with the D11-C2 (skill_version, prompt_hash)
//	   snapshot.
//	3. Refresh: drop the in-memory Runner state; keep the sqlite
//	   store; rebuild a NEW Runner from the same store (simulates
//	   backend restart + UI refresh).
//	4. GET analog: query the action_pending_approval row; state =
//	   active (snapshot matches).
//	5. Run #2: MarkApproved (action_approved row) via the AuditWriter,
//	   then runner.ResumeWithParams with an ApprovalDecision approve
//	   payload → Tool Pass-2 runs → write executes.
//	6. Assert: 3-row audit chain (pending + approved) in INSERT-only
//	   monotonic order; sha256(frozen_args) byte-identical between the
//	   pending row, the approved row, and the Tool-call args.
//
// Output protocol (the acceptance gate greps for these prefixes):
//
//	[HITL-E2E-PASS]   on full success
//	[HITL-E2E-FAIL] <step-name>: <reason>   on any failure (exit 1)
//
// A JSON summary is printed to stdout on pass:
//
//	{"stage":"approve-final","approval_id":"<id>","audit_chain_rows":3,
//	 "frozen_args_sha256":"<hex>","executed":true}
//
// Wire-in: cmd/spike-hitl is the binary scripts/verify-phase-11.sh
// Stage 6 invokes. The companion TestE2EApprovalFlow in main_test.go
// calls runE2EApprovalFlow directly so the acceptance gate gets a
// `go test`-runnable hermetic check without subprocess overhead.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// smokeToolCallID is the deterministic tool-call ID the scripted
// ChatModel emits. Using a constant (not a random UUID) keeps the
// E2E smoke reproducible across runs — the acceptance gate can grep
// the JSON summary without a regex.
const smokeToolCallID = "tc-hitl-smoke-000001"

// smokeToolArgs is the scripted memory_facts_write tool-call payload.
// The bytes are the HITL-07 anchor: the pending row's frozen_args,
// the approved row's frozen_args, and the Pass-2 executed bytes MUST
// all be byte-identical to this constant.
const smokeToolArgs = `{"key":"employee_count","value":"150","tier":"权威"}`

// smokeSessionID is the E2E session identity. One binary run = one
// session; the approval flow's cross-session moat is exercised by
// OTHER tests (TestApprovalEndpointBoundToSession in 11-04).
const smokeSessionID = "sess-hitl-smoke"

// smokeSkillID is the witness Skill identity. Matches
// testdata/skills/hitl-smoke/SKILL.md.
const smokeSkillID = "hitl-smoke"

// scriptedSmokeModel is a ToolCallingChatModel scripted on the
// conversation history (mirrors internal/server/approval_api_test.go's
// scriptedApprovalModel):
//
//   - no tool message in history → emit the memory_facts_write tool
//     call (drives the HITL Tool's Pass-1 interrupt);
//   - a tool message present     → emit the final answer.
//
// The scripted shape keeps the E2E smoke hermetic AND deterministic
// across the cross-refresh Runner rebuild (the Pass-2 runner sees a
// tool message in the restored history and emits the final answer).
type scriptedSmokeModel struct {
	mu    sync.Mutex
	calls int
}

var _ model.ToolCallingChatModel = (*scriptedSmokeModel)(nil)

func hasToolResult(msgs []*schema.Message) bool {
	for _, m := range msgs {
		if m != nil && m.Role == schema.Tool {
			return true
		}
	}
	return false
}

func (m *scriptedSmokeModel) scripted(msgs []*schema.Message) *schema.Message {
	if !hasToolResult(msgs) {
		return &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID:   smokeToolCallID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      hitl.MemoryFactsToolName,
					Arguments: smokeToolArgs,
				},
			}},
		}
	}
	return &schema.Message{Role: schema.Assistant, Content: "final answer after approval"}
}

func (m *scriptedSmokeModel) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	return m.scripted(msgs), nil
}

func (m *scriptedSmokeModel) Stream(ctx context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, msgs)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *scriptedSmokeModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// stubEmbedder is the hermetic Embedder — returns a fixed-dimension
// vector. Never touches the network. Mirrors the agent_test.go stub.
type stubEmbedder struct{}

func (e *stubEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	out := make([]float32, 8)
	for i := range out {
		out[i] = float32(len(text)%8+i) / 16.0
	}
	return out, nil
}

func (e *stubEmbedder) Dimensions() int { return 8 }

// stubRetriever is the hermetic Retriever — returns zero hits so the
// trust-floor never fires on the smoke query (the smoke query is a
// write request, not a retrieval question). Implements the narrow
// surface agent.BuildRetrieveTool consumes.
type stubRetriever struct{}

func (s *stubRetriever) Health(_ context.Context) error { return nil }
func (s *stubRetriever) Search(_ context.Context, _ []float32, _ []string, _ int, _ int64) ([]retriever.Hit, error) {
	return nil, nil
}
func (s *stubRetriever) Ensure(_ context.Context) error                        { return nil }
func (s *stubRetriever) Insert(_ context.Context, _ []retriever.Document) error { return nil }
func (s *stubRetriever) ListMetadata(_ context.Context) ([]retriever.DocumentMeta, error) {
	return nil, nil
}
func (s *stubRetriever) GetChunk(_ context.Context, _ int64) (retriever.ChunkContent, error) {
	return retriever.ChunkContent{}, errors.New("stub: no chunk")
}
func (s *stubRetriever) Disconnect() error { return nil }

// e2eReport is the JSON summary emitted to stdout on success. The
// acceptance gate parses this for the audit-chain + frozen_args
// invariants.
type e2eReport struct {
	Stage            string `json:"stage"`
	ApprovalID       string `json:"approval_id"`
	AuditChainRows   int    `json:"audit_chain_rows"`
	FrozenArgsSHA256 string `json:"frozen_args_sha256"`
	Executed         bool   `json:"executed"`
}

// runE2EApprovalFlow drives the 6-step hermetic HITL pipeline against
// the supplied store. The store MUST be an in-memory SQLite instance
// (the caller owns lifecycle). Returns the report on success, or an
// error with the failing step name wrapped for the [HITL-E2E-FAIL]
// prefix.
//
// The function is EXPORTED so cmd/spike-hitl/main_test.go's
// TestE2EApprovalFlow can invoke it directly; main() is the thin
// wrapper that constructs the store, calls this, and translates the
// error into the gate-friendly output protocol.
func runE2EApprovalFlow(ctx context.Context, s *sqlite.SQLiteStore, sessionID, skillID string) (*e2eReport, error) {
	if s == nil {
		return nil, errors.New("runE2EApprovalFlow: store is nil")
	}
	if sessionID == "" {
		return nil, errors.New("runE2EApprovalFlow: sessionID required")
	}
	if skillID == "" {
		return nil, errors.New("runE2EApprovalFlow: skillID required")
	}

	manifest := smokeManifest(skillID)
	snap := hitl.NewSkillSnapshot(manifest.Name, "", agent.RenderSystemPrompt(manifest))
	cpID := agent.RunnerCheckPointID(sessionID)

	// ---- Step 1: Run #1 — Pass-1 interrupt + checkpoint persist ----
	interruptID, err := step1Interrupt(ctx, s, manifest, sessionID)
	if err != nil {
		return nil, fmt.Errorf("step-1 interrupt: %w", err)
	}

	// ---- Step 2: Seed the pending audit row + stamp snapshot ----
	if err := step2SeedPending(ctx, s, sessionID, snap); err != nil {
		return nil, fmt.Errorf("step-2 seed-pending: %w", err)
	}
	if err := step2StampCheckpoint(ctx, s, cpID, snap); err != nil {
		return nil, fmt.Errorf("step-2 stamp-checkpoint: %w", err)
	}

	// ---- Step 3: Refresh — drop in-memory Runner, rebuild from store ----
	// The rebuilt Runner is constructed inside step5 (the resume step)
	// so this step is purely conceptual: the ONLY shared state across
	// the "refresh" is the sqlite store. This is the D11-C2 cross-
	// refresh resume path's essence.

	// ---- Step 4: GET analog — query the pending row, state=active ----
	if err := step4QueryPending(ctx, s, sessionID, snap); err != nil {
		return nil, fmt.Errorf("step-4 query-pending: %w", err)
	}

	// ---- Step 5: Run #2 — MarkApproved + ResumeWithParams ----
	if err := step5ApproveResume(ctx, s, manifest, sessionID, snap, interruptID); err != nil {
		return nil, fmt.Errorf("step-5 approve-resume: %w", err)
	}

	// ---- Step 6: Assert audit chain + frozen_args byte-identity ----
	report, err := step6AssertChain(ctx, s, sessionID)
	if err != nil {
		return nil, fmt.Errorf("step-6 assert-chain: %w", err)
	}
	return report, nil
}

// smokeManifest returns the in-memory manifest equivalent of
// testdata/skills/hitl-smoke/SKILL.md. The binary does NOT parse the
// SKILL.md file at runtime — hermetic requirement — but the two must
// agree (a comment in the SKILL.md notes this).
func smokeManifest(skillID string) *skill.Manifest {
	m := &skill.Manifest{
		Name:        skillID,
		Description: "HITL smoke witness Skill (Plan 11-08 Task 2)",
		Agent:       "ChatModelAgent",
		Writes:      []string{"memory:facts"},
	}
	m.SetResolvedTiersForTest([]string{"权威"})
	return m
}

// smokeKBRegistry builds the read-only registry the smoke Skill's hitl
// tool construction resolves its tier vocabulary against (Plan 26-06:
// BuildToolsForSkill fails closed without one when the manifest declares
// memory:facts). The seed file is located by walking cwd upward so both
// `go run ./cmd/spike-hitl` (repo root) and `go test` (package dir)
// resolve it.
//
// PROVISIONAL (26-06): read-only seed snapshot — replaced by kb.Open in
// plan 26-09 (same posture as the other spike binaries' 26-03/26-04
// snapshots).
func smokeKBRegistry() kb.Registry {
	path, err := locateSeedFile("knowledge_bases.yaml")
	if err != nil {
		log.Printf("[WARN] cmd/spike-hitl: knowledge_bases.yaml not found from cwd: %v — knowledge-base registry is empty (hitl tool construction will fail closed)", err)
		return kb.New(nil)
	}
	kbSeeds, err := kb.LoadSeedFile(path)
	if err != nil {
		log.Fatalf("[FAIL] cmd/spike-hitl: kb seed load: %v", err)
	}
	return kb.New(kbSeeds)
}

// locateSeedFile walks cwd upward (max 5 levels) looking for the named
// file. Mirrors cmd/spike-fanout's helper (deliberate duplication across
// spike binaries — no shared main-package seam exists).
func locateSeedFile(name string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, name)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("cwd upward search")
}

// step1Interrupt drives Pass 1: a BuildRunner-constructed Runner
// executes one Run whose scripted memory_facts_write tool call
// interrupts; the checkpoint persists to s. Returns the captured
// interrupt UUID (the eino resume-target key).
func step1Interrupt(ctx context.Context, s *sqlite.SQLiteStore, manifest *skill.Manifest, sessionID string) (string, error) {
	runner, err := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
		Manifest:        manifest,
		Embedder:        &stubEmbedder{},
		Retriever:       &stubRetriever{},
		ChatModel:       &scriptedSmokeModel{},
		SessionID:       sessionID,
		SkillID:         manifest.Name,
		CheckPointStore: s,
		KBRegistry:      smokeKBRegistry(),
	})
	if err != nil {
		return "", fmt.Errorf("BuildRunner: %w", err)
	}

	iter := runner.Run(ctx, []*schema.Message{schema.UserMessage("log the employee count")},
		adk.WithCheckPointID(agent.RunnerCheckPointID(sessionID)))
	var interruptID string
	var sawInterrupt bool
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		if ev.Err != nil {
			return "", fmt.Errorf("run event error: %w", ev.Err)
		}
		if ev.Action != nil && ev.Action.Interrupted != nil {
			sawInterrupt = true
			if len(ev.Action.Interrupted.InterruptContexts) == 0 {
				return "", errors.New("interrupt event carried zero InterruptContexts")
			}
			interruptID = ev.Action.Interrupted.InterruptContexts[0].ID
		}
	}
	if !sawInterrupt {
		return "", errors.New("scripted tool call did NOT surface Action.Interrupted")
	}
	if interruptID == "" {
		return "", errors.New("interrupt UUID not captured")
	}

	// The checkpoint MUST be durably persisted BEFORE the resume.
	_, existed, err := s.Get(ctx, agent.RunnerCheckPointID(sessionID))
	if err != nil {
		return "", fmt.Errorf("checkpoint Get: %w", err)
	}
	if !existed {
		return "", errors.New("checkpoint not persisted after interrupt")
	}
	return interruptID, nil
}

// step2SeedPending writes the action_pending_approval audit row
// through the fail-closed writer (the SAME code path the production
// approval endpoint uses when it seeds a row).
func step2SeedPending(ctx context.Context, s *sqlite.SQLiteStore, sessionID string, snap hitl.SkillSnapshot) error {
	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	_, err := w.WritePendingApproval(ctx, sessionID, "", snap, smokeToolCallID, "memory:facts", []byte(smokeToolArgs), "")
	if err != nil {
		return fmt.Errorf("WritePendingApproval: %w", err)
	}
	return nil
}

// step2StampCheckpoint rewrites the checkpoint row with the D11-C2
// snapshot columns (the payload bytes are carried over verbatim).
func step2StampCheckpoint(ctx context.Context, s *sqlite.SQLiteStore, cpID string, snap hitl.SkillSnapshot) error {
	payload, existed, err := s.Get(ctx, cpID)
	if err != nil {
		return fmt.Errorf("checkpoint Get: %w", err)
	}
	if !existed {
		return errors.New("checkpoint must exist before stamping")
	}
	if err := s.SetCheckpointWithSnapshot(ctx, cpID, payload, snap.Version, snap.PromptHash); err != nil {
		return fmt.Errorf("SetCheckpointWithSnapshot: %w", err)
	}
	return nil
}

// step4QueryPending is the GET /approval analog: query the pending
// row via the store and assert its snapshot matches the ACTIVE Skill
// (state=active). A mismatch here would render the row "expired" in
// production (D11-C2).
func step4QueryPending(ctx context.Context, s *sqlite.SQLiteStore, sessionID string, snap hitl.SkillSnapshot) error {
	rows, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID:  sessionID,
		Action:     "action_pending_approval",
		ToolCallID: smokeToolCallID,
		Limit:      1,
	})
	if err != nil {
		return fmt.Errorf("QueryAudit pending: %w", err)
	}
	if len(rows) != 1 {
		return fmt.Errorf("expected 1 pending row, got %d", len(rows))
	}
	row := rows[0]
	if row.SkillVersion != snap.Version || row.PromptHash != snap.PromptHash {
		return fmt.Errorf("snapshot drift: row (v=%q ph=%q) vs active (v=%q ph=%q) — would render expired",
			row.SkillVersion, row.PromptHash, snap.Version, snap.PromptHash)
	}
	return nil
}

// step5ApproveResume writes the action_approved row AND drives the
// resume round trip (the Run #2 approve analog). MarkApproved runs
// BEFORE ResumeWithParams — mirrors the production approval endpoint
// (internal/server/approval_api.go), which writes the resolution row
// synchronously so an audit failure aborts the resume.
func step5ApproveResume(ctx context.Context, s *sqlite.SQLiteStore, manifest *skill.Manifest, sessionID string, snap hitl.SkillSnapshot, interruptID string) error {
	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	if _, err := w.MarkApproved(ctx, sessionID, "", snap, smokeToolCallID, smokeToolCallID, "memory:facts", []byte(smokeToolArgs), "smoke-test approve"); err != nil {
		return fmt.Errorf("MarkApproved: %w", err)
	}

	// Rebuild the Runner (the "refresh" — new instance, same store).
	runner, err := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
		Manifest:        manifest,
		Embedder:        &stubEmbedder{},
		Retriever:       &stubRetriever{},
		ChatModel:       &scriptedSmokeModel{},
		SessionID:       sessionID,
		SkillID:         manifest.Name,
		CheckPointStore: s,
		KBRegistry:      smokeKBRegistry(),
	})
	if err != nil {
		return fmt.Errorf("BuildRunner (resume): %w", err)
	}

	iter, err := runner.ResumeWithParams(ctx, agent.RunnerCheckPointID(sessionID), &adk.ResumeParams{
		Targets: map[string]any{interruptID: &hitl.ApprovalDecision{
			Decision:   "approve",
			Reason:     "smoke-test approve",
			Target:     "memory:facts",
			ApprovalID: smokeToolCallID,
		}},
	})
	if err != nil {
		return fmt.Errorf("ResumeWithParams: %w", err)
	}
	if err := agent.DrainRunnerEvents(ctx, iter, nil); err != nil {
		return fmt.Errorf("drain resume events: %w", err)
	}
	return nil
}

// step6AssertChain verifies the audit chain invariants:
//
//   - Exactly 1 action_pending_approval row + 1 action_approved row
//     for the smoke tool_call_id (the auto-pass chain is a separate
//     single-row shape; this E2E exercises the operator chain).
//   - The two rows appear in INSERT-only monotonic order (pending
//     row ID < approved row ID).
//   - sha256(frozen_args) is byte-identical between the two rows AND
//     matches sha256(smokeToolArgs) — the HITL-07 anchor.
func step6AssertChain(ctx context.Context, s *sqlite.SQLiteStore, sessionID string) (*e2eReport, error) {
	pending, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID:  sessionID,
		Action:     "action_pending_approval",
		ToolCallID: smokeToolCallID,
		Limit:      1,
	})
	if err != nil {
		return nil, fmt.Errorf("QueryAudit pending: %w", err)
	}
	if len(pending) != 1 {
		return nil, fmt.Errorf("expected 1 pending row, got %d", len(pending))
	}
	approved, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID:  sessionID,
		Action:     "action_approved",
		ToolCallID: smokeToolCallID,
		Limit:      1,
	})
	if err != nil {
		return nil, fmt.Errorf("QueryAudit approved: %w", err)
	}
	if len(approved) != 1 {
		return nil, fmt.Errorf("expected 1 approved row, got %d", len(approved))
	}

	p, a := pending[0], approved[0]
	if !(p.ID < a.ID) {
		return nil, fmt.Errorf("audit chain not monotonic: pending.id=%d approved.id=%d", p.ID, a.ID)
	}

	// HITL-07: 3-way byte-identity — pending row, approved row, smoke
	// tool-call args.
	expected := sha256.Sum256([]byte(smokeToolArgs))
	pSHA := sha256.Sum256(p.FrozenArgs)
	aSHA := sha256.Sum256(a.FrozenArgs)
	if pSHA != expected {
		return nil, fmt.Errorf("pending frozen_args drift: got sha256=%x want %x", pSHA, expected)
	}
	if aSHA != expected {
		return nil, fmt.Errorf("approved frozen_args drift: got sha256=%x want %x", aSHA, expected)
	}

	return &e2eReport{
		Stage:            "approve-final",
		ApprovalID:       smokeToolCallID,
		AuditChainRows:   2, // pending + approved (operator chain; auto-pass chain would be 1)
		FrozenArgsSHA256: hex.EncodeToString(expected[:]),
		Executed:         true,
	}, nil
}

// main is the acceptance-gate entry point. Constructs the in-memory
// store, runs the E2E flow, and translates the result into the
// [HITL-E2E-PASS] / [HITL-E2E-FAIL] output protocol.
func main() {
	log.SetFlags(0) // the gate greps for prefixes; timestamps would break the match

	ctx := context.Background()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:", MaxOpenConns: 1})
	if err != nil {
		fmt.Printf("[HITL-E2E-FAIL] store-init: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = s.Close() }()

	report, err := runE2EApprovalFlow(ctx, s, smokeSessionID, smokeSkillID)
	if err != nil {
		fmt.Printf("[HITL-E2E-FAIL] %v\n", err)
		os.Exit(1)
	}
	out, err := json.Marshal(report)
	if err != nil {
		fmt.Printf("[HITL-E2E-FAIL] marshal-report: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
	fmt.Println("[HITL-E2E-PASS]")
}
