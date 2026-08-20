// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"

	"github.com/skillforge/skillforge/internal/skill"
)

// findModuleRoot walks upward from the current working directory until
// it finds a go.mod. Returns the absolute path of the file joined to
// the module root. Tests use this to source-grep files relative to
// the module root regardless of cwd (gopls / terminal / CI may
// differ).
func findModuleRoot(relPath string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir + "/" + relPath, nil
		}
		parent := dir
		// stop at filesystem root
		if dir == "/" || dir[len(dir)-1] == ':' {
			return "", fmt.Errorf("go.mod not found above %s", wd)
		}
		// walk up
		for i := len(dir) - 1; i >= 0; i-- {
			if dir[i] == '/' || dir[i] == '\\' {
				dir = dir[:i]
				if dir == "" {
					dir = "/"
				}
				break
			}
		}
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", wd)
		}
	}
}

// TestBuildDeepAgent_RequiresChatModel mirrors the dispatcher's nil
// ChatModel guard at the deep-agent construction site.
func TestBuildDeepAgent_RequiresChatModel(t *testing.T) {
	_, err := BuildDeepAgent(context.Background(), DispatchConfig{
		Manifest:  &skill.Manifest{Name: "compliance", Agent: "deep"},
		SessionID: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "ChatModel is required") {
		t.Fatalf("expected ChatModel-required error, got %v", err)
	}
}

// TestBuildDeepAgent_RequiresManifest locks the Manifest nil-guard.
func TestBuildDeepAgent_RequiresManifest(t *testing.T) {
	_, err := BuildDeepAgent(context.Background(), DispatchConfig{
		ChatModel: stubChatModel{},
		SessionID: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "Manifest is required") {
		t.Fatalf("expected Manifest-required error, got %v", err)
	}
}

// TestMaxIterationHardCap_Literal10 is the P12-L2 source-grep lock.
// It reads deep_agent.go and confirms the literal `MaxIteration: 10`
// appears in the deep.New call. A test assertion against runtime
// state would be insufficient — the source-grep is what makes the
// cap structural (config-driven knobs can drift; a code literal
// cannot).
func TestMaxIterationHardCap_Literal10(t *testing.T) {
	// Find module root via go.mod upward walk — tests may run from
	// any cwd (gopls variant cwd, terminal cwd, etc.).
	path, err := findModuleRoot("internal/runtime/deep_agent.go")
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	src := string(content)
	if !strings.Contains(src, "MaxIteration: 10") {
		t.Fatalf("P12-L2 lock broken: %s must contain the literal `MaxIteration: 10`", path)
	}
	if !strings.Contains(src, "WithoutGeneralSubAgent: true") {
		t.Fatalf("P12-L1 hardening broken: %s must set WithoutGeneralSubAgent: true (Phase 13 territory; hardened now against sub-agent injection)", path)
	}
	if !strings.Contains(src, "WithoutWriteTodos: true") {
		t.Fatalf("Option C broken: %s must set WithoutWriteTodos: true (custom audited write_todos Tool ships in 12-02)", path)
	}
}

// TestDefaultMaxIterationEqualsHardCap locks the contract between the
// exported default constant and the source-grep'd hard cap. They
// MUST stay in sync — the source-grep test catches drift in either
// direction.
func TestDefaultMaxIterationEqualsHardCap(t *testing.T) {
	if DefaultMaxIteration != 10 {
		t.Fatalf("DefaultMaxIteration = %d, want 10 (P12-L2)", DefaultMaxIteration)
	}
}

// TestBuildDeepAgent_Smoke constructs a DeepAgent with the minimal
// required fields. The test does not exercise the LLM; it just
// confirms the deep.New call site compiles + succeeds + returns a
// ResumableAgent (which Phase 12's Runner integration depends on).
func TestBuildDeepAgent_Smoke(t *testing.T) {
	agent, err := BuildDeepAgent(context.Background(), DispatchConfig{
		Manifest:  &skill.Manifest{Name: "compliance", Agent: "deep", Description: "compliance review"},
		ChatModel: stubChatModel{},
		SessionID: "sess-deep-1",
		Tools:     []tool.BaseTool{},
	})
	if err != nil {
		t.Fatalf("BuildDeepAgent: %v", err)
	}
	if agent == nil {
		t.Fatalf("BuildDeepAgent returned nil")
	}
}

// TestBuildDeepAgent_InstructionOverride ensures the per-archetype
// system prompt is wired through DispatchConfig.Instruction (Phase 12
// caller-side: BuildRunner calls runtime.renderDeepSystemPrompt for the
// deep archetype).
func TestBuildDeepAgent_InstructionOverride(t *testing.T) {
	const custom = "PLAN-12-TEST instruction override marker"
	agent, err := BuildDeepAgent(context.Background(), DispatchConfig{
		Manifest:    &skill.Manifest{Name: "compliance", Agent: "deep"},
		ChatModel:   stubChatModel{},
		SessionID:   "sess-deep-2",
		Instruction: custom,
	})
	if err != nil {
		t.Fatalf("BuildDeepAgent: %v", err)
	}
	if agent == nil {
		t.Fatalf("BuildDeepAgent returned nil")
	}
	// We can't peek at the prompt directly, but the construction
	// succeeded with the override; the lock is at the Instruction
	// field mapping.
}