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
	"fmt"
	"net/http"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/skillforge/skillforge/internal/wire"
)

// chatRequest is the POST /api/chat body shape. Skill is optional —
// empty string defaults to "compliance" (the only Skill Phase 2
// supports; Phase 5 adds semantic routing).
type chatRequest struct {
	Query string `json:"query"`
	Skill string `json:"skill"`
}

// defaultSkill is the Phase 2-only Skill name. The pipeline refuses on
// any unknown skill with RefusalReasonOutOfScope — no implicit fallback.
const defaultSkill = "compliance"

// chatHandler is the SSE-emitting POST /api/chat handler. It is the
// single HTTP entry point in this package; pipeline.go owns the
// orchestration, this file owns the wire + the request decode + the
// skill-scope lookup.
//
// Behavior:
//  1. Decode JSON body — empty query → 400.
//  2. Resolve skill name → *skill.Manifest via deps.Loader.Skills().
//     Empty skill → defaultSkill. Unknown skill → emit out_of_scope
//     refusal + finish-step.
//  3. Delegate to VerdictPipeline (the 10-step orchestrator).
//
// Hertz's WithHostPorts(addr) + h.Spin() call shape (cmd/server/main.go)
// blocks here until SIGINT/SIGTERM — this handler runs on every
// request concurrently.
func (s *Deps) chatHandler(ctx context.Context, c *app.RequestContext) {
	var req chatRequest
	if err := c.BindAndValidate(&req); err != nil {
		// BindAndValidate decodes the JSON body and validates basic types.
		// A malformed body is a 400 — Hertz handles status before we get here.
		emitRefusal(c, verdict.Refusal{
			Reason:      verdict.RefusalReasonOutOfScope,
			CanRephrase: false,
		})
		return
	}
	if req.Query == "" {
		c.SetStatusCode(http.StatusBadRequest)
		_ = wire.EmitChunk(c, errorChunk("query is required"))
		return
	}
	if req.Skill == "" {
		req.Skill = defaultSkill
	}

	manifest := s.findSkill(req.Skill)
	if manifest == nil {
		// Unknown skill → out_of_scope refusal. The finish-step after the
		// data-policy is the canonical terminator — clients expect it.
		_ = emitRefusal(c, verdict.Refusal{
			Reason:      verdict.RefusalReasonOutOfScope,
			CanRephrase: false,
		})
		return
	}

	if err := s.VerdictPipeline(ctx, c, req.Query, manifest); err != nil {
		// Pipeline returned an error AFTER the first SSE frame was written
		// (we cannot change status now). Emit an error chunk so the client
		// sees something meaningful, then finish-step.
		_ = wire.EmitChunk(c, errorChunk(err.Error()))
		_ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
		return
	}
}

// findSkill returns the manifest for name in deps.Loader.Skills(), or
// nil if the Skill is not loaded (unknown name, or Loader was not
// initialized for tests).
//
// Phase 2 simplification: there is exactly ONE Skill (compliance).
// Phase 5 introduces a Name field on Manifest and a real map lookup
// (SYSK-05 routing). For now, the function matches name == "compliance"
// against the first loaded Skill, returning nil for any other name.
// Tests that bypass the Loader pass manifests directly to
// VerdictPipeline so this lookup is never exercised in the unit suite.
func (s *Deps) findSkill(name string) *skill.Manifest {
	if s.Loader == nil {
		return nil
	}
	if name != defaultSkill {
		// Phase 2 supports only "compliance"; unknown names are out of scope.
		return nil
	}
	for _, m := range s.Loader.Skills() {
		// The first loaded Skill is the compliance Skill (the only one
		// that ships in Phase 2).
		return m
	}
	return nil
}

// emitVerdictStream is the SSE-emission tail of the verdict pipeline.
// Per Pattern 2 (02-RESEARCH.md §lines 299-323):
//   - one source-document chunk per citation (SC-6; chunk_id rides in
//     SourceID, tier rides in the data-policy Verdict payload's
//     Citations[].Tier field — keeps source-document AI-SDK-v5-clean).
//   - one data-policy chunk carrying the entire Verdict JSON
//     (CMPL-02 / CMPL-05).
//   - one finish-step terminator.
//
// The tier info stays in the data-policy payload rather than the
// source-document metadata because the AI SDK v5 UIMessageChunk
// discriminated union has no "tier" field on the source-document
// variant; stuffing it in Data would not parse through the v5
// client parser. The Verdict payload is the canonical place.
func emitVerdictStream(c *app.RequestContext, v verdict.Verdict, chunks []retriever.TierSearchResult) error {
	// 1. source-document per citation (SC-6).
	for _, cit := range v.Citations {
		if err := wire.EmitChunk(c, wire.UIMessageChunk{
			Type:     "source-document",
			ID:       "src-" + cit.ChunkID,
			SourceID: cit.ChunkID,
			Title:    sourcePathFor(chunks, cit.ChunkID),
		}); err != nil {
			return fmt.Errorf("emit source-document for %s: %w", cit.ChunkID, err)
		}
	}
	// 2. data-policy carrying the entire Verdict.
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal verdict: %w", err)
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{
		Type: "data-policy",
		Data: payload,
	}); err != nil {
		return fmt.Errorf("emit data-policy: %w", err)
	}
	// 3. finish-step terminator (AGNT-03).
	return wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
}

// emitRefusal emits a single data-policy chunk carrying the Refusal,
// then a finish-step. This is the canonical "refusal path" stream —
// both the SC-2 (no_authorized_source), GEVL-01 (low_retrieval_score),
// and D2-13 (out_of_scope) reasons use the same shape (ARAG-05).
//
// Returns the last wire.EmitChunk error (always nil in practice —
// writer.go:157 flushes immediately so a writer error is catastrophic).
func emitRefusal(c *app.RequestContext, r verdict.Refusal) error {
	payload, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal refusal: %w", err)
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{
		Type: "data-policy",
		Data: payload,
	}); err != nil {
		return err
	}
	return wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
}

// sourcePathFor returns the original Source path for the chunk whose
// numeric ID matches chunkID (rendered as a decimal string). If no
// chunk matches, the chunk ID itself is returned as the title (still
// useful for the citation badge, even if the source filename is
// unknown to the handler).
func sourcePathFor(chunks []retriever.TierSearchResult, chunkID string) string {
	for _, ch := range chunks {
		if fmt.Sprintf("%d", ch.ID) == chunkID {
			return ch.Source
		}
	}
	return chunkID
}

// drainFinalContent consumes a Runner.Query iterator until close,
// returning the concatenated assistant message content. For a no-tools
// ChatModelAgent the iterator emits exactly one AgentEvent whose
// Output.MessageOutput carries the final assistant message.
//
// This function graduates from cmd/spiked/e2e.go:341-374 — the proven
// shape from the Phase 1 spike. Any future streaming work (Phase 3+)
// will iterate over multiple events; for the Phase 2 single-shot
// verdict path the final message is all we need.
//
// If any event has a non-nil Err, drain returns that error immediately
// after concatenating whatever content was buffered (so the caller can
// surface a meaningful error without losing partial output).
func drainFinalContent(iter *adk.AsyncIterator[*adk.AgentEvent]) (string, error) {
	var content []string
	var firstErr error
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		if ev.Err != nil && firstErr == nil {
			firstErr = ev.Err
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil {
			msg, mErr := ev.Output.MessageOutput.GetMessage()
			if mErr != nil && firstErr == nil {
				firstErr = mErr
			}
			if msg != nil && msg.Content != "" {
				content = append(content, msg.Content)
			}
		}
	}
	return joinStrings(content), firstErr
}

// joinStrings is the local string-join helper. It avoids importing
// "strings" just for one call; we already use strings in pipeline.go
// for the prompt builder. Kept local for clarity.
func joinStrings(parts []string) string {
	out := ""
	for _, p := range parts {
		out += p
	}
	return out
}

// errorChunk renders an error UIMessageChunk for the post-pipeline
// failure path. ErrorText is the only field set; the AI SDK v5 parser
// surfaces it as a "something went wrong" message in the chat UI.
//
// The error message MUST NOT contain the API key or any other
// T-1-13-sensitive value. The pipeline already enforces this — it
// passes our own wrapped errors whose messages never include env
// values.
func errorChunk(msg string) wire.UIMessageChunk {
	return wire.UIMessageChunk{Type: "error", ErrorText: msg}
}

// Compile-time guarantee: schema.UserMessage is imported for future
// streaming hooks (Phase 3+); this package will not silently lose the
// import if a future refactor removes the explicit use.
var _ = schema.UserMessage