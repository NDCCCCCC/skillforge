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

// Wiring lock for the ChatHandler's per-request Runner assembly
// (CR-03, 26-REVIEW).
//
// T-26-05 states a nil registry is "never a production assembly state",
// and AgentConfig.KBRegistry's doc says the production registry is wired
// at boot. The approval_api and plan_api sites already thread
// s.KBRegistry; the ChatHandler BuildRunner did not, so a tier-channel
// Skill that declares `writes: [memory:facts]` failed hard on this path
// with "kb registry required" (BuildToolsForSkill refuses to build the
// HITL tool without a registry) — fail-closed, availability hole.
//
// The lock greps the assembly source itself because the failure is
// invisible to unit tests (they pass a registry explicitly).

import (
	"os"
	"strings"
	"testing"
)

// runnerSetupSource reads internal/server/runner_setup.go for the lock.
func runnerSetupSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("runner_setup.go")
	if err != nil {
		t.Fatalf("read runner_setup.go: %v", err)
	}
	return string(raw)
}

// structLiteralText returns the source text of the composite literal
// introduced by opener up to its MATCHING closing brace (nesting-aware).
func structLiteralText(src, opener string) (string, bool) {
	i := strings.Index(src, opener)
	if i < 0 {
		return "", false
	}
	depth := 0
	for j := i + len(opener) - 1; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[i : j+1], true
			}
		}
	}
	return "", false
}

// TestChatHandlerBuildRunnerWiresKBRegistry — the ChatHandler assembly
// must thread Deps.KBRegistry into the Runner, exactly as approval_api
// and plan_api already do. Without it every memory:facts Skill 500s on
// this path, and the accumulator gate stays inert for the tier channel.
func TestChatHandlerBuildRunnerWiresKBRegistry(t *testing.T) {
	src := runnerSetupSource(t)

	const opener = "agent.RunnerBuildConfig{"
	start := strings.Index(src, "func (s *Deps) ChatHandler")
	if start < 0 {
		t.Fatal("Deps.ChatHandler not found in runner_setup.go — update this lock to the new Runner-path assembly site")
	}
	lit, ok := structLiteralText(src[start:], opener)
	if !ok {
		t.Fatal("ChatHandler no longer constructs agent.RunnerBuildConfig — update this lock to the new construction site")
	}
	if !strings.Contains(lit, "KBRegistry: s.KBRegistry,") {
		t.Fatal("ChatHandler BuildRunner does not wire KBRegistry — a memory:facts Skill fails hard on this path (\"kb registry required\") and the citable gate stays inert (CR-03)")
	}
}
