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

package hitl

import (
	"sort"

	"github.com/cloudwego/eino/components/tool"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/report"
)

// AllowedWriteTargets is the D11-B3 SINGLE SOURCE OF TRUTH for the
// list of write-target names a Skill may declare in its `writes:`
// frontmatter. Three call sites read from this slice; if you add a
// new write target, this is the ONLY place you need to extend.
//
// The three call sites (each is a separate package, all read from the
// same variable here):
//
//  1. AGENT CONSTRUCTION — internal/agent/hitl_tools.go's
//     BuildToolsForSkill intersects manifest.Writes with this slice
//     to compute the LLM-visible tool whitelist per turn (D11-B1
//     pure-function invariant).
//
//  2. APPROVE-AND-REMEMBER RULE MATCH — internal/agent/hitl/rule_matcher.go
//     (11-05) reads this slice to validate the rule-key target field
//     at INSERT time. An unknown target would create a rule that can
//     never match — rejected at insert.
//
//  3. PHASE 12 plan_approve add-point — appending "plan_approve" to
//     this slice AND registering &planApproveTool{} in hitlHandlers
//     (Plan 12-03 / PLAN-03 / D11-B3) is the only change required to
//     add a new write target. NO other file in the codebase needs
//     touching.
//
// Order matters: each Skill's writes:[] is filtered against this
// slice in iteration order, and the resulting tool whitelist is
// sorted lexicographically by Tool.Name before being passed to the
// LLM (deterministic schema order — see BuildToolsForSkill).
//
// DEVIATION from the 11-02 PLAN's literal 2-element list:
//
//	`skill_state` is a deprecated placeholder referenced by the
//	existing examples/skills/calculator SKILL.md (D7-10 validate-only
//	template). It is NOT a Phase 11 target — no HITL Tool ships for
//	it; build_tools.go silently skips a nil handler. Including it
//	keeps the existing TestLoader_ParseComplianceCanonicalSkill test
//	green without rewriting the canonical compliance/calculator
//	fixtures. Phase 12 may prune this entry when calculator moves out
//	of the validate-only demo state.
//
// PHASE 12 add (Plan 12-03 / PLAN-03): "plan_approve" is the third
// production write target — the DeepAgent plan confirmation gate.
// Skills with `writes: ["plan_approve"]` (or `writes: ["*"]`) get
// the plan_approve_review Tool in their LLM-visible whitelist.
//
// PHASE 18 add (Plan 18-01 / D18-A4): "report:publish" is the fourth
// production write target — the monthly-report Skill publication gate.
// Skills with `writes: ["report:publish"]` get the report_publish Tool
// in their LLM-visible whitelist.
var AllowedWriteTargets = []string{
	"memory:facts",    // 11-02: first write target (HITL-06)
	"review_report",   // 11-04: second write target — handler nil until 11-04
	"plan_approve",    // 12-03: third write target — DeepAgent plan confirmation gate (PLAN-03)
	"report:publish",  // 18-01: fourth write target — monthly-report publication gate (D18-A4)
	"skill_state",     // deprecated placeholder for calculator (D7-10); no handler
}

// hitlHandlers is the D11-B3 map from write-target name to the
// InvokableTool that handles the write. The map is READ-ONLY after
// package init — there is no mutex; reads are lock-free. If a future
// Skill hot-reload needs to swap handlers at runtime, the swap is a
// wholesale map replacement under a write lock (out of scope for
// 11-02).
//
// A nil value at a key means "target is allowed but the handler has
// not been registered yet". BuildToolsForSkill skips nil handlers
// when computing the Tool whitelist, so the LLM never sees a write-
// tool schema for a target whose handler is not yet implemented.
//
// CRITICAL — D11-B3 architecture:
//   - DO NOT add an init() function that registers handlers at
//     package load. The CONTEXT explicitly rejects "global init()
//     registration" (rationale: global init() is debugging-hostile).
//   - DO register via the direct struct-literal at this variable's
//     declaration so the wiring is grep-able in one place.
//   - DO NOT add a sync.RWMutex — the registry is process-wide
//     constant after init; reads are lock-free.
var hitlHandlers = map[string]tool.InvokableTool{
	"memory:facts":   &MemoryFactsTool{},    // 11-02 — this file's companion hitl_memory_facts.go (exported in 11-05 for BuildToolsForSkill's WithContext injection)
	"review_report":  &reviewReportTool{},   // 11-04 — hitl_review_report.go (HITL-06 second write target)
	"plan_approve":   &planApproveTool{},   // 12-03 — hitl_plan_approve.go (PLAN-03 DeepAgent plan confirmation gate)
	"report:publish": &reportPublishTool{},   // 18-01 — hitl_report_publish.go (D18-A4 monthly-report publication gate)
	"skill_state":    nil,                   // deprecated placeholder (D7-10 calculator). nil handler = BuildToolsForSkill silently skips.
}

// KnownTargets returns a sorted copy of AllowedWriteTargets.
//
// Why a defensive copy: callers MUST NOT mutate the underlying slice
// (skill/validate.go uses it for the unknown-target check; a future
// mutation would silently poison every Skill reload). The copy is
// sorted lexicographically so error messages from
// skill/validate.go are stable for test fixtures.
//
// The O(n log n) cost is irrelevant at the small N (currently 2;
// Phase 12 may grow to 3). DO NOT swap this for a map[string]bool —
// the "two named slices" shape is the D11-B3 source-of-truth
// contract. Phase 12's add-point is append on AllowedWriteTargets,
// NOT an edit to a map literal.
func KnownTargets() []string {
	out := make([]string, len(AllowedWriteTargets))
	copy(out, AllowedWriteTargets)
	sort.Strings(out)
	return out
}

// auditBinder is implemented by registry handlers that accept a
// production audit hook (WR-01 18-REVIEW). The unexported method keeps
// the setter invisible outside this package; InjectDeps (same package)
// performs the type assertion.
type auditBinder interface{ setAudit(hook auditWriterHook) }

// reportSinkBinder is implemented by handlers that need the CR-04
// reports-table audit sink (currently report:publish).
type reportSinkBinder interface{ setReportSink(sink report.ReportAuditSink) }

// reportEmitBinder is implemented by handlers that need the WR-02 SSE
// emission hook (currently report:publish).
type reportEmitBinder interface{ setEmit(fn ReportPublishEmitFunc) }

// kbBinder is implemented by handlers whose LLM-visible schema carries
// registry-derived vocabulary (currently memory:facts — its tier Enum
// is the kb registry ActiveSlugs snapshot, Plan 26-06).
type kbBinder interface{ bindKBRegistry(reg kb.Registry) error }

// sessionCloner is implemented by handlers that carry per-request
// bindable state (sessionID / skillID / ruleMatcher / tier vocabulary —
// currently memory:facts). CR-04 (26-REVIEW): those fields must never be
// written on the shared singleton, because BuildToolsForSkill runs per
// HTTP request on its own goroutine; the handler is asked for a
// request-scoped copy instead.
type sessionCloner interface {
	cloneForSession(reg kb.Registry, sessionID, skillID string, ruleMatcher RuleMatcher) (tool.InvokableTool, error)
}

// HandlerForSession returns the write handler for target, scoped to the
// per-request identity. Handlers WITHOUT bindable state are returned as
// the shared singleton (they are read-only after init, and the D11-B3
// map is the source of truth); handlers WITH bindable state return a
// fresh copy carrying (reg, sessionID, skillID, ruleMatcher), so two
// concurrent requests never share mutable fields and an interrupt's
// audit row can only name the session that actually issued it.
//
// Fail-closed contract (T-26-06-2): a nil registry, or one without an
// authority-role KB, aborts the clone for handlers that need the
// vocabulary — the returned error is the caller's signal to refuse tool
// construction, never a fallback to a compiled-in tier list. A nil
// handler (unregistered target) returns (nil, nil) and stays the
// caller's silent-skip case.
func HandlerForSession(target string, reg kb.Registry, sessionID, skillID string, ruleMatcher RuleMatcher) (tool.InvokableTool, error) {
	h := hitlHandlers[target]
	if h == nil {
		return nil, nil
	}
	if c, ok := h.(sessionCloner); ok {
		return c.cloneForSession(reg, sessionID, skillID, ruleMatcher)
	}
	return h, nil
}

// BindKBRegistry snapshots the knowledge-base registry into every
// handler whose schema needs it (T-26-06-2). A nil registry or a
// registry without an authority-role KB fails closed — the returned
// error is the caller's signal to refuse construction, never to fall
// back to a compiled-in tier list.
//
// CR-04 (26-REVIEW): this is NO LONGER on the per-request path.
// BuildToolsForSkill now obtains a request-scoped clone via
// HandlerForSession instead of mutating the shared singletons — the
// "last bind wins" posture was a data race plus a cross-session audit
// mis-attribution once BuildRunner ran per HTTP request. This helper
// stays for boot-time / test construction that binds explicitly.
func BindKBRegistry(reg kb.Registry) error {
	for _, h := range hitlHandlers {
		if b, ok := h.(kbBinder); ok {
			if err := b.bindKBRegistry(reg); err != nil {
				return err
			}
		}
	}
	return nil
}

// InjectDeps wires the production audit hooks into the registry's
// handlers. Called ONCE per Runner construction (BuildRunner) with the
// session-bound writer; the handler instances in hitlHandlers are
// shared singletons, so the LAST InjectDeps wins — the same
// last-write-wins posture MemoryFactsTool.WithContext documents, and
// the same single-threaded Agent-construction call path.
//
// WR-01 (18-REVIEW): without this wiring the report:publish and
// plan_approve handlers kept a nil audit field, auditOrDefault()
// degraded to noopAuditWriter, and Pass-1 interrupts produced NO
// action_pending_approval row — the operator-facing approval surface
// (fed exclusively from those rows) could never see the request and
// the flow dead-ended. The D11-C3 fail-closed INSERT invariant was
// silently inverted to fail-silent. WR-02 extends the same seam with
// the SSE emission hook for the report_published card. nil arguments
// are skipped so unit-test construction without a DB stays a noop.
func InjectDeps(audit auditWriterHook, reports report.ReportAuditSink, emit ReportPublishEmitFunc) {
	if audit != nil {
		for _, h := range hitlHandlers {
			if b, ok := h.(auditBinder); ok {
				b.setAudit(audit)
			}
		}
	}
	if reports != nil {
		for _, h := range hitlHandlers {
			if b, ok := h.(reportSinkBinder); ok {
				b.setReportSink(reports)
			}
		}
	}
	if emit != nil {
		for _, h := range hitlHandlers {
			if b, ok := h.(reportEmitBinder); ok {
				b.setEmit(emit)
			}
		}
	}
}

// LookupHandler returns the InvokableTool registered for the given
// target name, or nil if absent. Wraps the raw map lookup so the
// call sites (BuildToolsForSkill in agent/, rule_matcher.go in 11-05)
// don't need to know the hitlHandlers key namespace.
//
// Forward-compat note: 11-05's rule_matcher.go will use this helper
// for "if h := hitl.LookupHandler(rule.Target); h == nil { /* no-op:
// handler not yet registered (forward-compat placeholder) */ }" —
// the nil-allowed pattern is intentional, not a bug.
func LookupHandler(target string) tool.InvokableTool {
	return hitlHandlers[target]
}

// Contains returns true if needle is in haystack. Linear scan;
// the slice is bounded by AllowedWriteTargets (currently 2 entries;
// Phase 12 may grow to 3-4) so O(n) is fine. DO NOT swap for a
// map[string]bool — see KnownTargets comment for the rationale
// (the two-named-slices shape is the D11-B3 source-of-truth
// contract).
//
// EXPORTED (capital C) so the agent package's BuildToolsForSkill
// (internal/agent/hitl_tools.go) can call across the package
// boundary. The function is package-internal in spirit — no other
// package should consume it — but Go does not allow cross-package
// access to unexported identifiers and the dependency direction
// (agent → hitl) is one-way.
func Contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
