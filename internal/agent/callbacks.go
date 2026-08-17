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

package agent

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	template "github.com/cloudwego/eino/utils/callbacks"

	"github.com/skillforge/skillforge/internal/llm"
)

// Decision constants (D3-03 / SC-1). The data-decision SSE chunk's
// payload uses these as the Decision field's enum values. Emitted by
// newDecisionCallback based on which Eino callback fired:
//
//   - **retrieve** — Tool OnStart fired (the agent invoked
//     retrieve_compliance_chunks on this hop).
//   - **answer**   — ChatModel OnEnd fired with a message that has no
//     tool calls (the model produced a final answer).
//
// The fourth constant, forced_stop, is emitted by the Run method itself
// when MaxStep is hit (the react loop bails before ChatModel OnEnd can
// fire with a clean answer). It is NOT emitted from inside the callback
// handler — Run handles it directly.
const (
	DecisionRetrieve   = "retrieve"
	DecisionAnswer     = "answer"
	DecisionRefuse     = "refuse"
	DecisionForcedStop = "forced_stop"
)

// DecisionRecord is the in-memory representation of one agent decision.
// Production code converts this to a wire.DecisionPayload at emit time;
// tests capture DecisionRecords directly via a stub AgentConfig.Emit.
//
// The reason this package does NOT use wire.DecisionPayload directly
// is to keep internal/agent decoupled from internal/wire — the wire
// package is Hertz-aware (it knows about *app.RequestContext), while
// internal/agent is pure Go logic. The conversion happens at the
// internal/server chat handler boundary (Plan 03-05).
type DecisionRecord struct {
	Step       int
	Decision   string
	ToolCallID string
	Query      string
	Confidence float64
}

// stepCounter is the mutex-guarded per-Run hop counter. The Tool OnStart
// callback increments it once per retrieve invocation; ChatModel OnEnd
// reads it to label the answer with the current hop number. Mutex-
// guarded because Eino may fire callbacks from multiple goroutines when
// the agent issues parallel tool calls (Pitfall 2 pattern, copied from
// the chunkAccumulator idiom).
type stepCounter struct {
	mu sync.Mutex
	n  int
}

func (s *stepCounter) inc() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
}

func (s *stepCounter) current() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.n
}

// newDecisionCallback constructs the Eino callbacks.Handler that emits
// data-decision chunks per agent hop. Returns (handler, stepCounter)
// so Run can read the final step count for the forced_stop edge case.
//
// If emit is nil, returns (nil, nil) — caller (Run) skips the
// compose.WithCallbacks option entirely (batch mode for Plan 03-06
// eval sidecar).
//
// The handler uses the canonical Eino callback template pattern
// (template.NewHandlerHelper().ChatModel(modelHandler).Tool(toolHandler).Handler())
// per 03-RESEARCH.md §Code Examples lines 696-710. Callback signatures
// follow eino@v0.9.14/utils/callbacks/template.go:525-528 (ModelCallbackHandler)
// and 596-600 (ToolCallbackHandler) — both return context.Context
// (NOT error, as the original plan / research note assumed).
//
// Import-graph note: there are TWO callbacks packages in Eino:
//   - `github.com/cloudwego/eino/callbacks` (parent — defines Handler, RunInfo)
//   - `github.com/cloudwego/eino/utils/callbacks` (template helpers — defines
//     HandlerHelper, ModelCallbackHandler, ToolCallbackHandler)
//
// We import the parent as `callbacks` and the template helpers as
// `template` (the canonical Eino alias). The Handler() method returns
// the parent's callbacks.Handler type.
func newDecisionCallback(emit DecisionEmitFunc) (callbacks.Handler, *stepCounter) {
	if emit == nil {
		return nil, nil
	}

	sc := &stepCounter{}

	modelHandler := &template.ModelCallbackHandler{
		OnEnd: func(ctx context.Context, _ *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
			if output == nil || output.Message == nil {
				return ctx
			}
			// ChatModel produced a final message. If it has no tool
			// calls, the model is done — emit the answer decision.
			// If it has tool calls, the ReAct loop will route through
			// the ToolsNode next (the Tool OnStart will fire).
			if len(output.Message.ToolCalls) == 0 {
				emit(sc.current(), DecisionAnswer, "", "", tryParseConfidence(output.Message.Content))
			}
			return ctx
		},
	}

	toolHandler := &template.ToolCallbackHandler{
		OnStart: func(ctx context.Context, _ *callbacks.RunInfo, input *tool.CallbackInput) context.Context {
			sc.inc()
			query := ""
			if input != nil {
				query = tryParseQuery(input.ArgumentsInJSON)
			}
			emit(sc.current(), DecisionRetrieve, "", query, 0.0)
			return ctx
		},
	}

	return template.NewHandlerHelper().
			ChatModel(modelHandler).
			Tool(toolHandler).
			Handler(),
		sc
}

// tryParseConfidence makes a best-effort attempt to extract a confidence
// number from the model's JSON output. Used to populate the optional
// Confidence field on the answer decision payload. Returns 0.0 on any
// parse failure (the field is optional; UI treats 0.0 as "not reported").
func tryParseConfidence(raw string) float64 {
	raw = llm.ExtractJSON(raw)
	if raw == "" {
		return 0.0
	}
	var probed struct {
		Confidence float64 `json:"confidence"`
	}
	if json.Unmarshal([]byte(raw), &probed) == nil {
		return probed.Confidence
	}
	return 0.0
}

// tryParseQuery extracts the query argument from a retrieve tool-call's
// JSON arguments. Best-effort — returns "" on any parse failure (the
// query field is informational, not authoritative; the actual retrieve
// call uses its own parsed args).
func tryParseQuery(argsJSON string) string {
	if argsJSON == "" {
		return ""
	}
	var probed struct {
		Query string `json:"query"`
	}
	if json.Unmarshal([]byte(argsJSON), &probed) == nil {
		return probed.Query
	}
	return ""
}

// stripJSONDelim previously lived here as a fragile byte-index fence
// stripper. It was the root cause of OOS-1 (glm-verdict-fence-parse): its
// language-tag loop `for s[0] != '\n'` consumed the entire input when no
// newline followed the tag, and its leading-fence check required the
// fence at the absolute start (prose-wrapped output defeated it). It has
// been replaced by the shared, robust llm.ExtractJSON helper
// (internal/llm/extract.go), which is used at every LLM-output parse site:
//
//   - internal/agent/agent.go (verdict parse, the OOS-1 failing site)
//   - internal/agent/callbacks.go tryParseConfidence (above)
//   - internal/agent/critique.go (critique LLM JSON parse)
//   - internal/server/pipeline.go (Phase 2 linear verdict parse)
//
// See internal/llm/extract.go godoc and TestExtractJSON for the full
// matrix of fence/prose/newline shapes the helper handles.

// emitForcedStop is the helper Run uses when the MaxStep budget is hit
// before the ChatModel produces a clean answer. It synthesizes a
// forced_stop decision so the UI can surface "the agent gave up after
// N hops" rather than silently truncating.
func emitForcedStop(emit DecisionEmitFunc, sc *stepCounter) {
	if emit == nil || sc == nil {
		return
	}
	emit(sc.current(), DecisionForcedStop, "", "", 0.0)
}
