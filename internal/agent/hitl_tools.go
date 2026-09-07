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
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/cloudwego/eino/components/tool"

	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/skill"
)

// BuildToolsForSkill is the D11-B1 pure-function whitelist recompute.
// Per turn (per Runner construction), the LLM-visible Tool list is
// intersection(AllowedWriteTargets, manifest.Writes) — read-only
// Skills see ZERO write Tool schemas (HITL-05 load-bearing invariant:
// a Skill with writes:[] has no way to reach a write Tool, regardless
// of how the LLM is prompted or what the user pastes into the chat).
//
// WHY THIS FILE LIVES IN package agent (not package hitl):
//
// Cycle resolution (Task 5 read_first). The hitl package is the
// D11-B3 single source of truth for AllowedWriteTargets +
// hitlHandlers. The BuildToolsForSkill function takes
// *skill.Manifest as a parameter, which would force `hitl` to import
// `skill`. Meanwhile skill/validate.go must import `hitl`
// (for hitl.KnownTargets). The Go compiler rejects this package
// cycle.
//
// Resolution: place BuildToolsForSkill in package agent. The agent
// package already imports both skill (for the manifest) and hitl
// (for the registry). Adding the function here creates NO new
// import edges — the agent package is the natural join point. The
// source of truth (AllowedWriteTargets, hitlHandlers,
// KnownTargets, LookupHandler, Contains) STAYS in package hitl and
// is unchanged.
//
// CALL SITE (per Runner construction):
//
//	tools, err := agent.BuildToolsForSkill(ctx, manifest, sessionID, skillID, ruleMatcher, reg)
//	// → BuildRunner appends retrieveTool after, constructs the Agent.
//
// The registry argument (Plan 26-06) threads the knowledge-base
// registry into the hitl tool construction: memory:facts' tier
// vocabulary is the registry's ActiveSlugs snapshot, and a Skill whose
// writes declare memory:facts CANNOT build its tool without one (nil →
// error, fail-closed — never a silent fallback to a compiled-in Enum;
// T-26-06-2). Read-only Skills never construct a hitl tool, so their
// nil-registry call stays legal.
//
// D11-B1 INVARIANTS (test by internal/agent/hitl_tools_test.go):
//
//   - Same (ctx, manifest) → same []tool.BaseTool. No caching. No
//     global state. The function is pure. (26-06: the registry bind on
//     the shared handler follows the same last-write-wins posture
//     WithContext and InjectDeps already document — the whitelist
//     recompute itself stays stateless.)
//   - Read-only Skills (writes: nil or len==0) → 0-element slice.
//     The LLM never sees a write Tool schema.
//   - Unknown targets (writes: ["external_api"]) → silently skipped.
//     The manifest validator (skill/validate.go) already warned at
//     load time; BuildToolsForSkill does NOT re-error — Skills with
//     forward-compatibility targets would crash-loop otherwise.
//   - Handlers that are nil in hitlHandlers (e.g. "review_report"
//     until 11-04) → silently skipped. The LLM does NOT see a Tool
//     schema whose handler is unregistered (no half-built schema).
//   - Returned slice sorted by Tool.Name (stable, deterministic).
//
// D11-B4: BuildToolsForSkill is called per-Runner-construction, not
// per-session. The Runner itself is per-session singleton (see
// runner_setup.go's BuildRunner) but every call to BuildRunner
// re-pure-computes the whitelist. This is the correct synthesis:
// Skill-A's HITL Tool cannot leak into Skill-B's session even via
// Runner cross-reuse, AND a future Skill hot-reload invalidates the
// Tools automatically.
//
// 11-05 EXTENSION (HITL-08 / D11-D1): the signature carries the
// per-session context (sessionID, skillID, ruleMatcher) so each
// whitelisted HITL Tool is bound to the CURRENT session + Skill + rule
// store via WithContext. A nil ruleMatcher degrades to
// hitl.NoopRuleMatcher (no auto-pass — every write interrupts; the
// fail-closed direction).
func BuildToolsForSkill(_ context.Context, manifest *skill.Manifest, sessionID, skillID string, ruleMatcher hitl.RuleMatcher, reg kb.Registry) ([]tool.BaseTool, error) {
	if manifest == nil {
		return nil, errors.New("agent.BuildToolsForSkill: manifest is nil")
	}

	// D11-B1: per-turn whitelist = intersection(AllowedWriteTargets,
	// manifest.Writes) mapped via hitlHandlers. Read-only Skills
	// (writes: []) get an empty slice so the LLM NEVER sees a
	// write-tool JSON schema (HITL-05 prompt-injection resistance —
	// structural defense, not policy check).
	writes := manifest.Writes
	if len(writes) == 0 {
		return nil, nil
	}

	// Plan 26-06 / T-26-06-2: bind the registry BEFORE any schema is
	// published. Only Skills that actually declare the memory:facts
	// write target need the vocabulary, which keeps read-only Skill
	// construction registry-free. A nil registry (or one without an
	// authority-role KB) aborts the whitelist here — the error path is
	// the fail-closed contract, not a degraded Enum.
	if slices.Contains(writes, hitl.MemoryFactsTarget) {
		if err := hitl.BindKBRegistry(reg); err != nil {
			return nil, fmt.Errorf("agent.BuildToolsForSkill: %w", err)
		}
	}

	// Collect (name, tool) pairs so we can sort by tool name without
	// calling Info() twice per pair. ctx is context.Background()
	// here because Info() is required to be stateless by the
	// tool.BaseTool contract (verified at eino@v0.9.14/components/
	// tool/tool.go); real per-Run ctx is not needed for the schema
	// lookup.
	type pair struct {
		name string
		t    tool.BaseTool
	}
	pairs := make([]pair, 0, len(writes))
	seenNames := map[string]bool{}
	for _, target := range writes {
		// Unknown target → silently skip. The manifest validator
		// already warned at load time; re-erroring here would crash
		// a Skill loaded with a forward-compatibility target
		// (e.g. Phase 12 plan_approve referenced by a Skill ahead
		// of the Phase 12 deployment).
		if !hitl.Contains(hitl.AllowedWriteTargets, target) {
			continue
		}
		// Handler not yet registered (e.g. "review_report" before
		// 11-04 lands) → silently skip. The LLM does NOT see a
		// Tool schema for an unregistered handler — see Task 2's
		// hitlHandlers var declaration comment.
		h := hitl.LookupHandler(target)
		if h == nil {
			continue
		}
		info, err := h.Info(context.Background())
		if err != nil {
			return nil, fmt.Errorf(
				"agent.BuildToolsForSkill: tool %s Info: %w", target, err,
			)
		}
		if seenNames[info.Name] {
			continue
		}
		seenNames[info.Name] = true

		// 11-05: bind the per-session context (sessionID + skillID +
		// ruleMatcher) so the Tool's Pass-1 D11-D1 rule lookup and
		// Pass-2 audit rows are scoped to THIS session + Skill. nil
		// ruleMatcher → the Tool's own NoopRuleMatcher default
		// (fail-closed toward human review).
		if mft, ok := h.(*hitl.MemoryFactsTool); ok {
			mft.WithContext(sessionID, skillID, ruleMatcher)
		}
		pairs = append(pairs, pair{name: info.Name, t: h})
	}

	// Stable order: sort by Tool name so the LLM-visible schema
	// list is deterministic across calls (avoids spurious Tool-call
	// drift in test fixtures; matches registry ordering convention).
	sort.SliceStable(pairs, func(i, j int) bool {
		return pairs[i].name < pairs[j].name
	})
	out := make([]tool.BaseTool, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, p.t)
	}
	return out, nil
}
