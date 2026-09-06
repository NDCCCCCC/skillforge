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

// Quick plan 260815-0xb Task 1 (TDD) — VerdictPipeline per-stage
// data-subprogress streaming.
//
// The FE renderer (web/src/components/chat/parts/DataSubprogressPart.tsx,
// FEUI-06) already exists and renders `data-subprogress` chunks; the mock
// fixture __fixtures__/sub-progress.json proved the chunk order
// (data-subprogress* before text-start) is useChat-compatible. These
// tests pin the BACKEND half of that channel:
//
//  1. Test 1 lives in internal/wire/chunk_test.go (TestValidTypes_Twenty)
//     — data-subprogress must be in the EmitChunk whitelist.
//  2. Test 2 (verdict path): the 4 progress chunks appear IN ORDER
//     (retrieve → found_candidates → generate → self_verify), all BEFORE
//     text-start; found_candidates carries count=len(chunks) + tier.
//  3. Test 3 (refusal path): the refusal short-circuit emits ZERO
//     data-subprogress chunks (fast return — progress for a path that
//     never reaches the LLM would be noise, and SC-2/GEVL-01 must stay
//     snappy).
//
// RED phase: these tests FAIL before the pipeline change because
// VerdictPipeline never emits data-subprogress. The GREEN commit adds
// emitSubprogress + the 4 emit sites.

package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/skillforge/skillforge/internal/wire"
)

// subprogressFrames extracts the data-subprogress chunks from an SSE
// body, in wire order, decoded as wire.SubprogressStep payloads.
func subprogressFrames(t *testing.T, body string) []wire.SubprogressStep {
	t.Helper()
	var out []wire.SubprogressStep
	for _, f := range parseFrames(t, body) {
		if f.Type != "data-subprogress" {
			continue
		}
		var p wire.SubprogressStep
		require.NoError(t, json.Unmarshal(f.Data, &p),
			"data-subprogress Data must unmarshal to wire.SubprogressStep: %s", string(f.Data))
		out = append(out, p)
	}
	return out
}

// TestVerdictPipeline_SubprogressOrder — the happy-path verdict stream
// MUST carry the 4 progress chunks in stage order, all BEFORE the
// text-start that opens the assistant message. The user's "LLM 生成期间
// 长时间空白" experience is fixed only if the progress lands on the wire
// DURING the wait (retrieve before Search, generate before Runner.Query,
// self_verify before GroundingChecker), which is exactly what the
// ordering assertion pins.
func TestVerdictPipeline_SubprogressOrder(t *testing.T) {
	deps := newVerdictPipelineDeps(t)
	chunkText := "员工累计工作已满1年不满10年的，年假5天。"
	chunks := []retriever.Hit{
		{ID: 1, Tier: "authority", Content: chunkText, Source: "authority/annual_leave.md", Score: 0.65},
		{ID: 2, Tier: "authority", Content: "备用条款", Source: "authority/other.md", Score: 0.55},
	}
	deps.Retriever = &stubRetriever{chunks: chunks}
	deps.Runner = newStubRunner(cannedVerdictJSON(verdict.CompliantStatus, 0.95, []string{"1"}, chunkText, "authority"))

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, "sess-subprogress-verdict", "年假有几天")
	require.NoError(t, err, "POST /api/chat (verdict path)")
	body := drainBody(t, resp)

	frames := parseFrames(t, body)
	progress := subprogressFrames(t, body)

	// (a) Exactly the 4 stages, in pipeline order.
	require.Len(t, progress, 4,
		"verdict path must emit exactly 4 data-subprogress chunks (retrieve/found_candidates/generate/self_verify)")
	assert.Equal(t, "retrieve", progress[0].Step)
	assert.Equal(t, "found_candidates", progress[1].Step)
	assert.Equal(t, "generate", progress[2].Step)
	assert.Equal(t, "self_verify", progress[3].Step)

	// (b) found_candidates carries count=len(chunks) + the search tier.
	assert.Equal(t, 2, progress[1].Count,
		"found_candidates.count must equal the retrieved chunk count")
	assert.Equal(t, "authority", progress[1].Tier,
		"found_candidates.tier must carry the first resolved search tier")
	assert.Equal(t, "authority", progress[0].Tier,
		"retrieve.tier must carry the first resolved search tier")

	// (c) Every data-subprogress chunk precedes text-start (the assistant
	// message anchor) — progress streams DURING the wait, never after.
	textStartIdx := -1
	lastProgressIdx := -1
	for i, f := range frames {
		switch f.Type {
		case "text-start":
			if textStartIdx < 0 {
				textStartIdx = i
			}
		case "data-subprogress":
			lastProgressIdx = i
		}
	}
	require.GreaterOrEqual(t, textStartIdx, 0, "verdict path must emit text-start")
	assert.Less(t, lastProgressIdx, textStartIdx,
		"all data-subprogress chunks must precede text-start (FEUI-06 channel contract)")
}

// TestVerdictPipeline_RefusalNoSubprogress — the refusal short-circuit
// (0 chunks, SC-2) is a fast structured return. It MUST NOT emit any
// data-subprogress chunk: the retrieval found nothing to report progress
// about, and decorating the refusal path would delay the very UX the
// refusal-by-construction design optimizes for (snappy, deterministic).
func TestVerdictPipeline_RefusalNoSubprogress(t *testing.T) {
	deps := newVerdictPipelineDeps(t)
	deps.Retriever = &stubRetriever{chunks: nil}
	deps.Runner = newStubRunner("")

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, "sess-subprogress-refusal", "查询员工联系方式")
	require.NoError(t, err, "POST /api/chat (refusal path)")
	body := drainBody(t, resp)

	progress := subprogressFrames(t, body)
	assert.Empty(t, progress,
		"refusal short-circuit must emit ZERO data-subprogress chunks (got %d)", len(progress))
}
