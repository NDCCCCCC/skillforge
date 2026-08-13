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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/skillforge/skillforge/internal/reprohash"
)

// TestComputePromptHash_BodyDrift is the server-side half of the Q1
// gate (its twin lives in internal/eval/frozen_test.go). It asserts the
// audit row's prompt_hash reacts to a prompt-body edit — the property
// Phase 3 explicitly deferred and Phase 6 delivers.
//
// Why it matters here: writeAuditAndTurn stamps this hash onto every
// audit row (GEVL-03), and internal/eval.AssertPin re-derives it to
// gate the frozen eval set (SC-6). If this test regresses, the frozen
// gate starts passing green over a drifted prompt.
func TestComputePromptHash_BodyDrift(t *testing.T) {
	const (
		skill = "compliance"
		model = "glm-5.2"
	)
	before := computePromptHash(skill, model, "You are a strict compliance auditor.")
	after := computePromptHash(skill, model, "You are a lenient compliance auditor.")

	assert.NotEqual(t, before, after,
		"changing ONLY the rendered prompt body MUST change prompt_hash (Q1)")
}

// TestComputePromptHash_Stable pins determinism + the delegation
// contract: computePromptHash MUST be byte-identical to
// reprohash.PromptHash, because internal/eval.AssertPin re-computes the
// pin through that shared package. Two divergent implementations would
// make the frozen pin decorative.
func TestComputePromptHash_Stable(t *testing.T) {
	const (
		skill = "compliance"
		model = "glm-5.2"
		body  = "You are a strict compliance auditor."
	)

	got := computePromptHash(skill, model, body)
	assert.Equal(t, computePromptHash(skill, model, body), got, "same inputs → same hash")
	assert.Equal(t, reprohash.PromptHash(skill, model, body), got,
		"computePromptHash MUST delegate to reprohash.PromptHash (byte-identity with eval.AssertPin)")
	assert.Len(t, got, 64, "SHA-256 hex digest is 64 chars")
}

// TestRenderedPromptBody_FallsBackWhenNoAgentOrSkill: a Deps with
// neither an Agent nor a resolvable Skill degrades to the empty body
// (hash still pins skill+model) rather than panicking. The audit write
// is best-effort and must never take down a request.
func TestRenderedPromptBody_FallsBackWhenNoAgentOrSkill(t *testing.T) {
	deps := &Deps{}
	assert.Equal(t, "", deps.renderedPromptBody("compliance"))
	assert.NotPanics(t, func() { _ = deps.renderedPromptBody("") })
}

// TestRenderedPromptBody_UsesWiredAgent: when an Agent is wired, the
// hash input is the agent's OWN rendering — literally the prompt the
// model saw for the request being audited.
func TestRenderedPromptBody_UsesWiredAgent(t *testing.T) {
	ag := newTestAgent(t, &cannedChatModel{})
	deps := &Deps{Agent: ag}

	body := deps.renderedPromptBody("compliance")
	assert.NotEmpty(t, body, "a wired Agent must yield its rendered system prompt")
	assert.Equal(t, ag.RenderedSystemPrompt(), body)
	assert.Contains(t, body, "compliance auditor", "sanity: this is the compliance system prompt")
}
