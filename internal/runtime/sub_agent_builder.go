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

// Package runtime — sub_agent_builder.go is Phase 13 / Plan 13-01's
// per-Run sub-agent construction seam. It bakes the structural
// tier-closure (P13-L2 + FAN-02) into the Tool closure at
// CONSTRUCTION time, so the sub-agent's retrieve Tool cannot query
// outside the parent's ResolvedTiers() ∩ sub_skill.ResolvedTiers()
// intersection.
//
// Why structural (not prompt-level):
//   - Prompt injection cannot expand the sub-agent's tier boundary
//     because the tier slice is baked into the Tool's closure, NOT
//     into the system prompt. A user-controlled sub-query that
//     claims "tier_filter: [权威, 历史]" but the parent's
//     ResolvedTiers() is [权威] gets rejected at BuildSubAgent (empty
//     intersection → FAN-02 SC-2 load-time error).
//
// Why empty intersection = LOAD-TIME error (not runtime silence):
//   - FAN-02 SC-2 invariant: a sub-skill with no overlapping tier
//     MUST fail at sub-agent construction, not at runtime. A runtime
//     silence would let the sub-agent Run with an empty tier filter,
//     return zero chunks, and the trust floor would fire a refusal
//     — masking the structural mismatch from the operator.
//
// Why `subManifest.Writes = []string{}` at construction (P13-L4):
//   - Sub-agents are READ-ONLY by structural design (FAN-01 SC-1
//     says "compressed results, no HITL inside sub-agents"). Even
//     if a sub-skill declares writes in its SKILL.md, BuildSubAgent
//     zeros them — BuildToolsForSkill (Phase 11) intersects writes
//     with AllowedWriteTargets; empty writes → no HITL Tools in
//     the sub-agent's whitelist.
//
// Recursion guard (P13-L8): BuildSubAgent routes the constructed
// sub-agent through runtime.DispatchAgent with the subManifest's
// Agent field intact. Sub-agents always end up as chat or deep
// archetype — they cannot be orchestrator-archetype (we do NOT
// inspect the subManifest.Agent type here, but the orchestrator
// ALWAYS passes a sub-skill whose Agent is NOT "orchestrator").
// Structural recursion is impossible without an explicit attempt
// to construct orchestrator-of-orchestrator.
package runtime

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"

	"github.com/skillforge/skillforge/internal/skill"
)

// BuildSubAgent constructs the per-Run sub-agent with tier-closure
// inheritance (FAN-02 SC-2 + P13-L2 + P13-L4).
//
// Returns:
//   - *adk.Agent on success — the constructed sub-agent (chat or
//     deep archetype).
//   - error on empty intersection (FAN-02 SC-2 load-time failure),
//     nil parent/sub Manifest, empty parent.ResolvedTiers() /
//     sub_skill.Reads (P13-L2 invariants).
//
// The sub-agent's session ID is the parent's SessionID + ":" +
// SubQueryID (P13-L9 audit RunPath isolation — sub-agents cannot
// read parent's session values because the sub-session is isolated).
//
// Tier-closure invariant (FAN-02 SC-2):
//
//	subManifest.Reads = parent.ResolvedTiers() ∩ subSkill.Reads
//
// Empty intersection → load-time error with substring
// "tier intersection empty" (the FAN-02 SC-2 contract).
func BuildSubAgent(ctx context.Context, cfg SubAgentConfig) (adk.Agent, error) {
	if cfg.ParentManifest == nil || cfg.SubSkillManifest == nil {
		return nil, fmt.Errorf("%w: both manifests required (P13-L2 invariant)", ErrSubAgentFactory)
	}
	parentTiers := cfg.ParentManifest.ResolvedTiers()
	if len(parentTiers) == 0 {
		return nil, fmt.Errorf("%w: parent %q has empty ResolvedTiers() — orchestrator's parent must declare at least one tier",
			ErrSubAgentFactory, cfg.ParentManifest.Name)
	}
	// Sub-skill's ResolvedTiers (CN-char Milvus VARCHAR values).
	// Falls back to resolving subSkillManifest.Reads (ASCII patterns)
	// so the hermetic tests can populate tiers directly via
	// SetResolvedTiersForTest OR via ResolveTiers(reads).
	subTiers := cfg.SubSkillManifest.ResolvedTiers()
	if len(subTiers) == 0 {
		resolved, err := skill.ResolveTiers(cfg.SubSkillManifest.Reads)
		if err != nil {
			return nil, fmt.Errorf("%w: sub_skill %q ResolveTiers: %w", ErrSubAgentFactory, cfg.SubSkillManifest.Name, err)
		}
		subTiers = resolved
	}
	if len(subTiers) == 0 {
		return nil, fmt.Errorf("%w: sub_skill %q has empty ResolvedTiers() (P13-L2 requires explicit declaration)",
			ErrSubAgentFactory, cfg.SubSkillManifest.Name)
	}
	intersection := intersectTiers(parentTiers, subTiers)
	if len(intersection) == 0 {
		// FAN-02 SC-2: empty intersection = load-time error, NOT runtime.
		return nil, fmt.Errorf("%w: tier intersection empty (parent=%v, sub_skill=%v) — load-time validation failure per FAN-02 SC-2",
			ErrSubAgentFactory, parentTiers, subTiers)
	}

	// Translate the CN-char resolved-tier intersection back to the
	// ASCII tier-dir patterns that the Manifest's Reads field
	// expects. TierEnumByValue provides the reverse lookup; we
	// build a `<dir>/**` pattern for each intersected tier so the
	// downstream retriever sees a familiar pattern shape.
	readPatterns := resolvedTiersToReadPatterns(intersection)

	// Build subManifest with the intersection baked into Reads.
	subManifest, err := skill.NewManifestBuilder(cfg.SubSkillManifest).
		WithReads(readPatterns).
		Build()
	if err != nil {
		return nil, fmt.Errorf("%w: ManifestBuilder.Build: %w", ErrSubAgentFactory, err)
	}
	// Carry the resolved-tier slice forward so the sub-agent's
	// ResolvedTiers() returns the intersection (P13-L2 — the
	// sub-agent's Tool closure reads ONLY from this slice).
	subManifest.SetResolvedTiersForTest(intersection)

	// P13-L4: structural read-only. Zero out Writes BEFORE
	// BuildToolsForSkill (or any equivalent Tool-assembly path)
	// sees the subManifest — the per-turn Tool whitelist
	// intersects writes with AllowedWriteTargets, so empty writes
	// → no HITL Tools in the sub-agent's whitelist.
	subManifest.Writes = []string{}

	// P13-L8 structural recursion guard. The 13-01 comment above
	// assumed "the orchestrator ALWAYS passes a sub-skill whose Agent
	// is NOT orchestrator" — but 13-03's runSubAgent passes the PARENT
	// manifest as the sub-skill placeholder (orchestrator.go: "13-05
	// widens to per-sub-skill; 13-03 reuses parent"), whose Agent IS
	// "orchestrator". Without this rewrite the constructed sub-agent
	// dispatches to buildOrchestrator again → unbounded
	// orchestrator-of-orchestrator nesting at Run time (only bounded
	// by the 30s per-sub-agent timeout). Rewriting the field HERE
	// makes the guard STRUCTURAL: no caller can smuggle a recursive
	// orchestrator past construction, regardless of what manifest it
	// passes. Test lock: TestBuildSubAgentStructuralRecursionGuard
	// (sub_agent_builder_test.go, 13-05 Rule-1 deviation).
	if subManifest.Agent == string(ArchetypeOrchestrator) {
		subManifest.Agent = string(ArchetypeChat)
	}

	// Construct the sub-agent via the Phase 12 unified dispatcher.
	// The dispatcher routes by cfg.Manifest.Agent — sub-skills
	// declared as "chat" or "deep" route to buildChatModelAgent /
	// buildDeepAgent. Sub-skills declared as "orchestrator" are
	// ALLOWED here (the recursion-guard is structural — orchestrator
	// cannot reach this code because the orchestrator's
	// pre-fanout validation rejects sub-skills whose Agent field is
	// "orchestrator"). 13-03 may add an explicit guard; 13-01 ships
	// the natural-shape defensive behavior.
	subSessionID := cfg.SessionID + ":" + cfg.SubQueryID
	subAgent, err := DispatchAgent(ctx, DispatchConfig{
		Manifest:    subManifest,
		ChatModel:   cfg.ChatModel,
		Embedder:    cfg.Embedder,
		Retriever:   cfg.Retriever,
		SessionID:   subSessionID,
		Instruction: cfg.Instruction,
		Tools:       nil, // 13-03 fills the per-sub-agent Tool whitelist via BuildToolsForSkill(subManifest, ...)
	})
	if err != nil {
		return nil, fmt.Errorf("%w: DispatchAgent: %w", ErrSubAgentFactory, err)
	}
	return subAgent, nil
}

// intersectTiers returns the set intersection of parent and sub,
// order-preserving on the LEFT operand (parent's tiers first).
// O(n*m) — fine for small N (4 tiers max per Phase 3 / TierEnum).
//
// The order-preserving property matters because the orchestrator
// logs the parent's tier order; deterministic output simplifies
// audit replay.
func intersectTiers(parent, sub []string) []string {
	seen := make(map[string]bool, len(sub))
	for _, t := range sub {
		seen[t] = true
	}
	var out []string
	for _, t := range parent {
		if seen[t] {
			out = append(out, t)
		}
	}
	return out
}

// resolvedTiersToReadPatterns converts a CN-char resolved-tier slice
// (e.g. ["权威", "历史"]) back into the canonical ASCII Reads patterns
// (e.g. ["authority/**", "historical/**"]) that the Manifest schema
// expects. This is the reverse of ResolveTiers — we look up each
// CN-char value in TierEnumByValue and build the "<dir>/**" pattern.
//
// Any unknown tier value is silently dropped (defense-in-depth —
// TierEnumByValue returns ok=false for unknown values, and we have
// already passed ResolveTiers upstream so the input slice should
// always be valid; this guard catches future drift).
func resolvedTiersToReadPatterns(tiers []string) []string {
	out := make([]string, 0, len(tiers))
	for _, t := range tiers {
		dir, ok := skill.TierEnumByValue(t)
		if !ok {
			continue
		}
		out = append(out, dir+"/**")
	}
	return out
}