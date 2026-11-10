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

// Plan C (entry-point unification): adapt the deterministic
// VerdictPipeline to the adk.Agent contract.
//
// Scope — read this before changing behaviour here:
//
//	This unifies the ENTRY POINT, not the OUTPUT PATH. VerdictPipeline
//	still writes its SSE frames through the Hertz RequestContext held
//	by PipelineAgent; the AgentEvent stream produced by Run carries
//	only the terminal outcome (one event: Err on failure, an empty
//	assistant message on success). Callers MUST drain with
//	drainPipelineEvents — forwarding these events to the wire would
//	duplicate what the pipeline already emitted.
//
//	The reason that is acceptable: eino's Runner owns checkpoint,
//	cancellation and event plumbing, and both chat archetypes now
//	reach it the same way. Turning the pipeline into a true
//	event-driven agent needs an Emitter abstraction across ~90
//	EmitChunk call sites (pipeline.go + chat.go + redline.go); that is
//	a separate, larger change. The adapter is the seam it will plug
//	into.
package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/skill"
)

// errPipelineEmptyQuery is returned when the AgentInput carries no user
// message. It can only happen through a caller bug — chatHandler always
// seeds the input with schema.UserMessage(query).
var errPipelineEmptyQuery = errors.New("pipeline agent: no user message in AgentInput")

// PipelineAgentName is the AgentName stamped on every event and log line
// this adapter produces. Exported so tests can assert on it without
// duplicating the literal.
const PipelineAgentName = "skillforge-verdict-pipeline"

// PipelineAgent adapts (*Deps).VerdictPipeline to adk.Agent.
//
// Bind it per request: it holds the Hertz RequestContext (the pipeline
// writes SSE through it) and the resolved manifest/session identity.
// It is NOT safe for concurrent use.
type PipelineAgent struct {
	deps      *Deps
	reqCtx    *app.RequestContext
	manifest  *skill.Manifest
	sessionID string
	skillID   string
}

// NewPipelineAgent binds one chat turn's identity to the agent.
// manifest must be non-nil — chatHandler already refuses nil before
// reaching this seam (chat.go, the manifest == nil refusal branch).
func NewPipelineAgent(deps *Deps, c *app.RequestContext, manifest *skill.Manifest, sessionID, skillID string) *PipelineAgent {
	return &PipelineAgent{
		deps:      deps,
		reqCtx:    c,
		manifest:  manifest,
		sessionID: sessionID,
		skillID:   skillID,
	}
}

// Name implements adk.Agent.
func (p *PipelineAgent) Name(_ context.Context) string { return PipelineAgentName }

// Description implements adk.Agent.
func (p *PipelineAgent) Description(_ context.Context) string {
	return "SkillForge deterministic tier-channel pipeline (code-forced retrieval + verdict JSON)"
}

// Run implements adk.Agent. It executes the pipeline on a goroutine and
// emits exactly one terminal event, mirroring FanOutOrchestrator's
// shape (internal/runtime/orchestrator.go).
//
// The query is taken from the AgentInput rather than from a struct
// field so the Runner remains the single source of truth for what this
// run was asked to answer.
func (p *PipelineAgent) Run(ctx context.Context, input *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		// Defer order matters: Close must run LAST (registered first),
		// so the recovered-panic event still has a live generator to
		// send on.
		defer gen.Close()
		defer func() {
			if r := recover(); r != nil {
				// A panic escaping a goroutine kills the whole process —
				// in a chat handler that means the server dies instead of
				// returning one failed turn. FanOutOrchestrator applies
				// the same containment (runtime/orchestrator.go fanOut).
				gen.Send(&adk.AgentEvent{
					AgentName: PipelineAgentName,
					Err:       fmt.Errorf("pipeline agent: panic recovered: %v", r),
				})
			}
		}()

		query := firstUserText(input)
		if query == "" {
			gen.Send(&adk.AgentEvent{AgentName: PipelineAgentName, Err: errPipelineEmptyQuery})
			return
		}

		if err := p.deps.VerdictPipeline(ctx, p.reqCtx, query, p.manifest, p.sessionID, p.skillID); err != nil {
			gen.Send(&adk.AgentEvent{AgentName: PipelineAgentName, Err: err})
			return
		}

		gen.Send(&adk.AgentEvent{
			AgentName: PipelineAgentName,
			Output: &adk.AgentOutput{
				MessageOutput: &adk.MessageVariant{
					IsStreaming: false,
					Message:     &schema.Message{Role: schema.Assistant},
					Role:        schema.Assistant,
				},
			},
		})
	}()

	return iter
}

// firstUserText mirrors runtime.extractFirstUserText (orchestrator.go).
// Duplicated rather than exported because the runtime package must not
// grow an API surface for a two-line helper, and server already owns
// its own message-shape conventions.
func firstUserText(in *adk.AgentInput) string {
	if in == nil {
		return ""
	}
	for _, m := range in.Messages {
		if m == nil {
			continue
		}
		if m.Role == schema.User {
			return strings.TrimSpace(m.Content)
		}
	}
	return ""
}

// runPipelineAgent drives one tier-channel turn through adk.Runner.
//
// It drains the iterator and returns the pipeline error verbatim, so
// chatHandler's existing failure path (audit row + data-turn + the
// in-conversation error text) stays byte-identical: events are
// deliberately NOT forwarded to the SSE wire.
func (s *Deps) runPipelineAgent(ctx context.Context, c *app.RequestContext, query string, manifest *skill.Manifest, sessionID, skillID string) error {
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: NewPipelineAgent(s, c, manifest, sessionID, skillID),
		// The pipeline writes its own SSE frames; Runner must not try to
		// stream agent events onto the same response.
		EnableStreaming: false,
	})

	iter := runner.Run(ctx, []*schema.Message{schema.UserMessage(query)})
	for {
		ev, ok := iter.Next()
		if !ok {
			return nil
		}
		if ev == nil {
			continue
		}
		if ev.Err != nil {
			return ev.Err
		}
	}
}
