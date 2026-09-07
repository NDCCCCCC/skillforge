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

package main

// Wiring lock for the eval CLI's agent assembly (CR-03, 26-REVIEW).
//
// T-26-05 states a nil registry is "never a production assembly state".
// The eval CLI already assembles the PRODUCTION registry (openKBRegistry
// over the deployment's sqlite store, 26-09) and resolves skill reads
// against it — but buildAgent never passed it to agent.AgentConfig, so
// the eval harness measured the ungated posture while approval_api /
// plan_api served the gated one: golden and frozen numbers calibrated
// against a DIFFERENT trust contract than the one production runs.
//
// The lock greps the assembly source itself because the drift is
// invisible to unit tests (they pass a registry explicitly).

import (
	"os"
	"strings"
	"testing"
)

// buildAgentLiteral returns the agent.AgentConfig literal inside
// buildAgent (the only production construction site in cmd/eval).
func buildAgentLiteral(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	src := string(raw)
	start := strings.Index(src, "func buildAgent(")
	if start < 0 {
		t.Fatal("buildAgent not found in cmd/eval/main.go — update this lock to the new eval agent assembly site")
	}
	end := strings.Index(src[start:], "\nfunc ")
	if end < 0 {
		t.Fatal("cannot delimit buildAgent in cmd/eval/main.go")
	}
	body := src[start : start+end]
	i := strings.Index(body, "agent.AgentConfig{")
	if i < 0 {
		t.Fatal("buildAgent no longer constructs agent.AgentConfig — update this lock")
	}
	depth := 0
	for j := i + len("agent.AgentConfig{") - 1; j < len(body); j++ {
		switch body[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return body[i : j+1]
			}
		}
	}
	t.Fatal("agent.AgentConfig literal in buildAgent is unterminated")
	return ""
}

// TestEvalBuildAgentWiresKBRegistry — the eval agent assembly must carry
// the registry so the numbers it publishes describe the gated trust
// contract production serves.
func TestEvalBuildAgentWiresKBRegistry(t *testing.T) {
	lit := buildAgentLiteral(t)
	if !strings.Contains(lit, "KBRegistry: kbReg,") {
		t.Fatal("buildAgent does not wire KBRegistry — the eval CLI measures the ungated posture while approval_api/plan_api serve the gated one (CR-03)")
	}
}
