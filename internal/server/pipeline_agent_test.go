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
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/skill"
)

// TestPipelineAgentNameStable — the AgentName is stamped on every event
// and appears in Runner logs; a silent rename would break log-based
// triage that greps for it.
func TestPipelineAgentNameStable(t *testing.T) {
	pa := NewPipelineAgent(&Deps{}, nil, &skill.Manifest{}, "sess", "skill")
	assert.Equal(t, "skillforge-verdict-pipeline", pa.Name(context.Background()))
	assert.Equal(t, PipelineAgentName, pa.Name(context.Background()))
	assert.NotEmpty(t, pa.Description(context.Background()))
}

// TestPipelineAgentEmptyQueryFailsClosed — an AgentInput with no user
// message must surface as an error event, never as a silent success.
// This is the only branch reachable without a live Hertz RequestContext,
// which is why it is the one tested end-to-end here.
func TestPipelineAgentEmptyQueryFailsClosed(t *testing.T) {
	pa := NewPipelineAgent(&Deps{}, nil, &skill.Manifest{}, "sess", "skill")

	iter := pa.Run(context.Background(), &adk.AgentInput{})
	ev, ok := iter.Next()
	require.True(t, ok, "exactly one terminal event is expected")
	require.ErrorIs(t, ev.Err, errPipelineEmptyQuery)
	assert.Equal(t, PipelineAgentName, ev.AgentName)

	_, ok = iter.Next()
	assert.False(t, ok, "iterator must close after the terminal event")
}

// TestPipelineAgentNilInputFailsClosed — a nil AgentInput follows the
// same closed-failure path rather than panicking inside firstUserText.
func TestPipelineAgentNilInputFailsClosed(t *testing.T) {
	pa := NewPipelineAgent(&Deps{}, nil, &skill.Manifest{}, "sess", "skill")

	iter := pa.Run(context.Background(), nil)
	ev, ok := iter.Next()
	require.True(t, ok)
	require.ErrorIs(t, ev.Err, errPipelineEmptyQuery)
}

// TestFirstUserText — only the first user message is used, and it is
// trimmed. Assistant/system messages are skipped.
func TestFirstUserText(t *testing.T) {
	cases := []struct {
		name string
		in   *adk.AgentInput
		want string
	}{
		{"nil", nil, ""},
		{"empty", &adk.AgentInput{}, ""},
		{"user only", &adk.AgentInput{Messages: []*schema.Message{schema.UserMessage("  年假有几天  ")}}, "年假有几天"},
		{"skips system", &adk.AgentInput{Messages: []*schema.Message{
			schema.SystemMessage("sys"),
			schema.UserMessage("q"),
		}}, "q"},
		{"nil message skipped", &adk.AgentInput{Messages: []*schema.Message{
			nil,
			schema.UserMessage("q2"),
		}}, "q2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, firstUserText(tc.in))
		})
	}
}
