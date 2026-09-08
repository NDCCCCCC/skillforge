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

	"github.com/skillforge/skillforge/internal/platformfloor"
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

// longestCommonPrefix and longestCommonSuffix are byte-level prefix/
// suffix helpers shared with the platform-floor tests. Inlined here
// rather than exported to keep the helper local to the runtime test
// surface (avoids an internal/testutil package for two trivial funcs).
func longestCommonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
}

func longestCommonSuffix(a, b string) string {
	la, lb := len(a), len(b)
	n := la
	if lb < n {
		n = lb
	}
	i := 0
	for i < n && a[la-1-i] == b[lb-1-i] {
		i++
	}
	return a[la-i:]
}

// TestRenderDeepSystemPromptPrependsFloor is the deep-architecture
// mirror of internal/agent's TestPlatformFloorPrependBothPaths (Plan
// 14-01 Task 2 Test 4 deep side). It locks the D14-A4 invariant that
// the deep archetype's rendered system prompt carries the platform
// floor at BOTH ends.
//
// Why package runtime (not agent): renderDeepSystemPrompt is unexported
// in internal/runtime, so the assertion must live in the same package.
// The agent-package chat-side mirror (TestPlatformFloorPrependBothPaths)
// imports this through its structural lock; together they pin the
// dual-path invariant.
//
// Why not via BuildRunner: runner_setup.go (in internal/agent) renders
// the chat prompt and passes it via DispatchConfig.Instruction, which
// short-circuits the fallback renderDeepSystemPrompt inside
// BuildDeepAgent. The fallback is what runs when a caller does NOT
// pre-render (tests / future spike binaries). Both paths are
// structurally equivalent (the prefix+base concatenation logic lives
// inside renderDeepSystemPrompt itself); locking the fallback path
// covers both because the prefix+base layout is the only place the
// floor can be (or fail to be) injected for the deep archetype.
func TestRenderDeepSystemPromptPrependsFloor(t *testing.T) {
	// Derive floor header/tail from two distinct sentinels using the
	// leaf ComposePlatformPrompt (same technique as the agent-side
	// TestComposePlatformPromptShape / TestPlatformFloorPrependBothPaths).
	const (
		sentinelA = "\x02DEEP_SENTINEL_A\x02"
		sentinelB = "\x03DEEP_SENTINEL_B\x03"
	)
	composedA := platformfloor.ComposePlatformPrompt(sentinelA)
	composedB := platformfloor.ComposePlatformPrompt(sentinelB)
	header := longestCommonPrefix(composedA, composedB)
	tail := longestCommonSuffix(composedA, composedB)
	if header == "" || tail == "" {
		t.Fatalf("could not derive floor header/tail from sentinels (collision or empty floor?)")
	}

	m := &skill.Manifest{Name: "compliance", Description: "compliance review", Agent: "deep", Writes: []string{}}
	rendered := renderDeepSystemPrompt(m)

	if !strings.HasPrefix(rendered, header) {
		t.Fatalf("renderDeepSystemPrompt (deep path) must start with the platform floor header\n got prefix: %q\nwant header (first 200): %q",
			rendered[:min(len(rendered), 200)], header[:min(len(header), 200)])
	}
	if !strings.HasSuffix(rendered, tail) {
		t.Fatalf("renderDeepSystemPrompt (deep path) must end with the platform floor tail\n got suffix: %q\nwant tail (last 200):    %q",
			rendered[max(0, len(rendered)-200):], tail[max(0, len(tail)-200):])
	}

	// The deep prompt's middle band must contain the deep-specific
	// prefix (telling the model about write_todos / plan_approve /
	// MaxIteration=10) and the deep base body — confirming the deep
	// prefix+base body still lives in the middle after floor wrapping.
	middleStart := len(header)
	middleEnd := len(rendered) - len(tail)
	middle := rendered[middleStart:middleEnd]
	if !strings.Contains(middle, "SkillForge DeepAgent") {
		t.Errorf("renderDeepSystemPrompt middle must contain the deep-archetype prefix (SkillForge DeepAgent marker); got middle prefix=%q",
			middle[:min(len(middle), 200)])
	}
	if !strings.Contains(middle, "compliance auditor") {
		t.Errorf("renderDeepSystemPrompt middle must contain the deep base body (auditor persona); got middle=%q",
			middle)
	}
}

// TestDeepArchetypePrefix_DataChannelLocksFinalStepGuard — D18-D5
// source-lock regression (2026-09-08 live observation). The LLM
// occasionally emitted the full monthly report as prose in its final
// assistant message instead of calling report_publish, producing zero
// audit + zero published artifact. The deep-archetype data-channel
// prefix is one of two redundant contracts that pin the model to call
// report_publish as its FINAL tool call (the other contract lives in
// hitl_report_publish.go's Info().Desc — both are locked by sibling
// tests). This test constructs a data-channel manifest (HasDataChannel
// = true + non-nil Template) and asserts the rendered prompt carries
// the MUST / NEVER / plain-text / final-step phrases.
//
// Mirrors the source-grep pattern of TestMaxIterationHardCap_Literal10:
// we cannot peek at the live ChatModel's instruction directly, so we
// render the prompt via the same code path the Runner uses and grep
// the rendered output. A test against runtime state alone would miss
// a regression where someone softens the prefix wording.
func TestDeepArchetypePrefix_DataChannelLocksFinalStepGuard(t *testing.T) {
	m := &skill.Manifest{
		Name:        "monthly-report",
		Description: "monthly KPI report",
		Agent:       "deep",
		Reads:       []string{"data:aggregated_only"},
		Writes:      []string{"report:publish"},
		Template:    &struct{}{}, // non-nil; only the type assertion matters
	}
	rendered := renderDeepSystemPrompt(m)

	// Direct contract: the data-channel branch of deepArchetypePrefix
	// (D18-A4 + D18-D5) MUST tell the model that report_publish is the
	// mandatory final step. We grep the whole rendered prompt — the
	// floor wrapper / base body never contain these phrases in this
	// shape, so a positive hit localizes to the data-channel prefix.
	// Phrases that span a wrapped newline in the source are split into
	// separate assertions ("FINAL" + "tool call") to avoid the brittle
	// mid-line break test.
	assertContainsAll(t, rendered,
		[]string{
			"D18-D5",                       // the contract marker
			"MANDATORY FINAL STEP",         // declares finality
			"MUST call report_publish",     // names the tool + obligation
			"FINAL",                        // position: final step
			"tool call",                    // position: tool call (newline-agnostic)
			"NEVER emit",                   // forbids the alternate path
			"plain text",                   // calls out the text-output anti-pattern
			"operator",                     // rationalizes the why (HITL)
			"HITL approval",                // (newline-agnostic)
		},
		"renderDeepSystemPrompt must carry the D18-D5 final-step guard in the data-channel branch")
}

// assertContainsAll is a small helper that fails the test with a single
// combined message listing every required phrase that was missing — saves
// the test author (and CI reruns) from chasing one missing phrase at a
// time.
func assertContainsAll(t *testing.T, haystack string, needles []string, what string) {
	t.Helper()
	var missing []string
	for _, n := range needles {
		if !strings.Contains(haystack, n) {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("%s — missing phrases: %v\n\nrendered prompt (first 2000 chars):\n%s",
			what, missing, haystack[:min(len(haystack), 2000)])
	}
}
