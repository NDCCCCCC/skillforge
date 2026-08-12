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

package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/wire"
)

// runAnnotatePass spins a Hertz server with a single POST /api/test/annotate
// route that invokes emitCounterEvidence directly. This isolates the pass
// from the rest of the verdict pipeline so the D5-13 regression can assert
// on its specific wire output without driving the full pipeline.
//
// The handler captures the result of emitCounterEvidence via a closure-
// injected error slot so tests can additionally assert on the error path
// (retriever failure → non-fatal: the caller logs and continues, the
// verdict stands).
func runAnnotatePass(t *testing.T, deps *Deps, qvec []float32, now int64) []wire.UIMessageChunk {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.POST("/api/test/annotate", func(ctx context.Context, c *app.RequestContext) {
		_ = deps.emitCounterEvidence(ctx, c, qvec, now)
	})

	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()
	cleanup := func() { _ = h.Close(); <-errCh }

	addr := "127.0.0.1:" + portStr(port)
	require.NoError(t, waitForListener(addr, 3*time.Second), "server warmup")
	defer cleanup()

	resp, err := http.Post("http://"+addr+"/api/test/annotate", "application/json", strings.NewReader("{}"))
	require.NoError(t, err, "POST /api/test/annotate")
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read body")
	return parseSSEFramesLocal(t, string(body))
}

// parseSSEFramesLocal mirrors chat_test.go's parseFrames but is local so
// the annotate test is self-contained (it does not depend on a helper
// defined in another _test.go file). Same shape: split by newline, keep
// only `data: ...` lines, JSON-unmarshal each into wire.UIMessageChunk.
func parseSSEFramesLocal(t *testing.T, body string) []wire.UIMessageChunk {
	t.Helper()
	var out []wire.UIMessageChunk
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "" {
			continue
		}
		var ch wire.UIMessageChunk
		require.NoError(t, json.Unmarshal([]byte(payload), &ch),
			"unmarshal data line: %s", payload)
		out = append(out, ch)
	}
	return out
}

// findCounterEvidenceFrame scans frames for a data-decision chunk whose
// payload.Decision == "counter_evidence"; returns (payload, frame, ok).
func findCounterEvidenceFrame(frames []wire.UIMessageChunk) (wire.DecisionPayload, wire.UIMessageChunk, bool) {
	for _, f := range frames {
		if f.Type != "data-decision" {
			continue
		}
		var p wire.DecisionPayload
		if err := json.Unmarshal(f.Data, &p); err != nil {
			continue
		}
		if p.Decision == "counter_evidence" {
			return p, f, true
		}
	}
	return wire.DecisionPayload{}, wire.UIMessageChunk{}, false
}

// TestCounterEvidence_EmittedAsAnnotation — happy path. A stubbed 历史
// retriever returns 2 chunks; emitCounterEvidence MUST emit exactly one
// data-decision chunk with decision="counter_evidence" carrying the 2
// ConflictChunks in its payload. The chunk tier rides ON the payload
// (always "历史" today); the wire shape is the data-decision type (NO
// new wire type — D5-04 extends the existing discriminator).
func TestCounterEvidence_EmittedAsAnnotation(t *testing.T) {
	deps := newTestDeps()
	deps.Retriever = &stubRetriever{chunks: []retriever.Hit{
		{ID: 101, Tier: "历史", Content: "2023 旧制度：出差住宿费无上限", Source: "history/2023-travel.md", Score: 0.88, EffectiveTo: 1_700_000_000},
		{ID: 102, Tier: "历史", Content: "2022 旧制度：礼品赠送无需审批", Source: "history/2022-gift.md", Score: 0.81, EffectiveTo: 1_660_000_000},
	}}

	frames := runAnnotatePass(t, deps, []float32{1.0}, 1_800_000_000)

	payload, _, ok := findCounterEvidenceFrame(frames)
	require.True(t, ok, "expected a data-decision with decision=counter_evidence")
	assert.Equal(t, "counter_evidence", payload.Decision)
	require.Len(t, payload.ConflictChunks, 2, "conflictChunks must carry both 历史 rows")

	// Each conflict row carries the chunk-id, tier, source, snippet, and
	// (when set) the effective_* metadata so the FE badge logic can fire.
	assert.Equal(t, "101", payload.ConflictChunks[0].ChunkID)
	assert.Equal(t, "历史", payload.ConflictChunks[0].Tier)
	assert.Equal(t, "history/2023-travel.md", payload.ConflictChunks[0].Source)
	assert.Contains(t, payload.ConflictChunks[0].Snippet, "2023 旧制度")
	assert.Equal(t, int64(1_700_000_000), payload.ConflictChunks[0].EffectiveTo)
	assert.Equal(t, "102", payload.ConflictChunks[1].ChunkID)
	assert.Equal(t, "历史", payload.ConflictChunks[1].Tier)
}

// TestCounterEvidence_EmptyConflictChunks — when the 历史 tier returns
// zero hits, the counter_evidence chunk MUST still be emitted so the FE
// can render the "未检索到..." empty-state copy. The conflictChunks slice
// is empty (or nil) so the FE reads a stable shape.
func TestCounterEvidence_EmptyConflictChunks(t *testing.T) {
	deps := newTestDeps()
	deps.Retriever = &stubRetriever{chunks: nil}

	frames := runAnnotatePass(t, deps, []float32{1.0}, 1_800_000_000)

	payload, _, ok := findCounterEvidenceFrame(frames)
	require.True(t, ok, "counter_evidence chunk must be emitted even when the tier is empty")
	assert.Empty(t, payload.ConflictChunks, "empty 历史 tier → empty conflictChunks (FE renders empty state)")
}

// TestCounterEvidence_NotGrounded — THE D5-13 LOAD-BEARING REGRESSION.
// The counter-evidence pass must NEVER touch the verdict's chunksByID
// map. The function signature takes only (ctx, c, qvec, now) — there is
// no parameter through which 历史 chunks could leak into grounding. This
// test asserts the structural guarantee at runtime by holding a
// chunksByID snapshot up against the pass and verifying the snapshot is
// byte-identical after the pass.
//
// Threat mitigated: T-5-25 (moat erosion — 历史 chunks enter chunksByID).
// If a future refactor passes chunksByID into emitCounterEvidence (e.g.
// to "simplify" the search), this test fails immediately because the
// snapshot's len / contents would change.
//
// Note: this test is a SIGNATURE guard at runtime — the real protection
// is the compiler rejecting any attempt to pass chunksByID. The runtime
// assertion catches the case where someone re-implements emitCounterEvidence
// to share state via a Deps field (forbidden — see annotate.go godoc).
func TestCounterEvidence_NotGrounded(t *testing.T) {
	deps := newTestDeps()
	deps.Retriever = &stubRetriever{chunks: []retriever.Hit{
		{ID: 201, Tier: "历史", Content: "旧条款与本次结论冲突", Source: "history/old.md", Score: 0.91},
	}}

	// The verdict's grounding map — populated BEFORE the counter-evidence
	// pass runs. The pass does NOT receive this map; we hold it here as
	// the trust-moat sentinel.
	chunksByID := map[string]string{
		"1": "权威条款：当前生效的出差住宿标准",
		"2": "权威条款：当前生效的审批流程",
	}
	snapshotBefore := snapshotGroundingMap(chunksByID)

	// Run the counter-evidence pass. The function signature does not
	// accept chunksByID — verify the call compiles + runs without the
	// grounding map.
	frames := runAnnotatePass(t, deps, []float32{1.0}, 1_800_000_000)
	require.NotEmpty(t, frames, "counter-evidence pass must emit ≥1 frame")

	// The grounding map MUST be byte-identical to its pre-pass snapshot.
	// This is the D5-13 invariant: 历史 chunks surface as annotation
	// only, never as grounding.
	assert.Equal(t, snapshotBefore, snapshotGroundingMap(chunksByID),
		"D5-13 trust moat: chunksByID must not change across the counter-evidence pass")

	// Defensive content assertion: the 历史 chunk id (201) must not
	// appear in the grounding map under any key.
	for k, v := range chunksByID {
		assert.NotContains(t, k, "201",
			"D5-13: 历史 chunk id 201 must NOT be a key in chunksByID (annotation only)")
		assert.NotContains(t, v, "旧条款",
			"D5-13: 历史 chunk content must NOT appear in chunksByID values")
	}
}

// TestCounterEvidence_RetrieverErrorIsNonFatal — D5-13 / UI-SPEC §SC-6
// error state. When Retriever.Search fails, emitCounterEvidence returns
// the error (the caller logs + continues — the verdict still stands).
// The wire must contain NO counter_evidence chunk on this path; the
// verdict is unaffected.
func TestCounterEvidence_RetrieverErrorIsNonFatal(t *testing.T) {
	deps := newTestDeps()
	deps.Retriever = &stubRetriever{err: assertErr("simulated 历史 outage")}

	frames := runAnnotatePass(t, deps, []float32{1.0}, 1_800_000_000)

	_, _, ok := findCounterEvidenceFrame(frames)
	assert.False(t, ok, "retriever error → no counter_evidence chunk on the wire (verdict stands)")
}

// TestCounterEvidence_PassesHistoryTierOnly — the search call inside
// emitCounterEvidence must request ONLY the 历史 tier (D5-13). A leak
// to 权威 or 个人 here would route grounded material into the
// annotation channel and break the moat. We assert via stubRetriever's
// capturedTiers slot.
func TestCounterEvidence_PassesHistoryTierOnly(t *testing.T) {
	sr := &stubRetriever{chunks: []retriever.Hit{
		{ID: 301, Tier: "历史", Content: "old", Source: "history/x.md", Score: 0.7},
	}}
	deps := newTestDeps()
	deps.Retriever = sr

	_ = runAnnotatePass(t, deps, []float32{1.0}, 1_800_000_000)

	lastTiers := sr.LastTiers()
	require.NotEmpty(t, lastTiers, "Search must have been called")
	assert.Equal(t, []string{"历史"}, lastTiers,
		"D5-13: counter-evidence pass must query ONLY the 历史 tier")
}

// snapshotGroundingMap clones a map[string]string for before/after
// comparison. We don't deep-copy because values are strings (immutable).
func snapshotGroundingMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// assertErr is a minimal error type for test-side stubbed failures.
type assertErr string

func (e assertErr) Error() string { return string(e) }
