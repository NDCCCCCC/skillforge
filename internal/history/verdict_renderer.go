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

package history

import (
	"encoding/json"
	"fmt"

	"github.com/skillforge/skillforge/internal/verdict"
)

// unrenderableConclusionSentinel is the deterministic placeholder returned
// to direct callers when the verdict JSON is malformed. Project() only
// rewrites on parse SUCCESS, so this sentinel never enters projected
// history — its purpose is to keep direct callers from crashing if they
// cannot afford to fail. Locked by T-10-01.
const unrenderableConclusionSentinel = "[历史结论不可解析]"

// RenderConclusion parses verdictJSON as a verdict.Verdict and returns the
// platform-appropriate plain-text body for the agent-path history. Order of
// preference (260817-vsm verdict-subject-mismatch + D10-A2):
//
//  1. Answer — direct factual answer when populated (simple-qa /
//     compliance-factual path).
//  2. ComplianceStatus — Compliance-Skill-specific verdict code
//     ({合规, 不合规, 不适用}).
//  3. Status — platform-neutral code ({ok, needs_input, refused, error}).
//
// Returns an error when the JSON is malformed; the direct caller can then
// decide to fall back to the unrenderableConclusionSentinel. Project()
// treats parse failure as a pass-through (the message is either a bare
// refusal code handled by isRefusalRound or plain assistant prose — neither
// needs conclusion rendering).
//
// This function is the ONLY renderer in Project() (D10-A2, T-10-02b). It
// must never produce refusal-semantic text. The Phase 9 spike proved
// refusal-semantic history is the -95pp refusal-drift source; the
// companion RenderRefusal exists for the user-facing handler path ONLY.
func RenderConclusion(verdictJSON string) (string, error) {
	if verdictJSON == "" {
		return "", fmt.Errorf("empty verdict JSON")
	}
	var v verdict.Verdict
	if err := json.Unmarshal([]byte(verdictJSON), &v); err != nil {
		return "", fmt.Errorf("parse verdict JSON: %w", err)
	}
	if v.Answer != "" {
		return v.Answer, nil
	}
	if v.ComplianceStatus != "" {
		return v.ComplianceStatus, nil
	}
	return v.Status, nil
}

// RenderConclusionOrSentinel is the direct-caller convenience wrapper
// around RenderConclusion. On error it substitutes
// the unrenderableConclusionSentinel so callers that cannot afford to
// propagate an error still receive a renderable string. NEVER call this
// from Project() — Project() handles parse failure by passing the message
// through unchanged (callers must NOT reach the sentinel inside history).
func RenderConclusionOrSentinel(verdictJSON string) string {
	s, err := RenderConclusion(verdictJSON)
	if err != nil {
		return unrenderableConclusionSentinel
	}
	return s
}

// RenderRefusal renders a user-facing refusal sentence for the given
// refusal-reason code. OUTPUT CONTAINS REFUSAL-SEMANTIC WORDS BY DESIGN
// — visual parity with the live chat refusal stream
// (internal/server/chat.go refusalText — D2-13 / FEUI-05).
//
// THIS FUNCTION MUST NEVER BE CALLED FROM Project() OR ANY CODE PATH
// FEEDING THE LLM HISTORY PREFIX.
//
// Reason: the Phase 9 spike (D9-V1, H2 anchor) demonstrated that
// refusal-semantic text in the history prefix causes the LLM to anchor
// on prior refusal language and replicate it — the measured drift was
// -95 percentage points on the refusal metric. Refusal rounds in agent
// history must render as the neutral placeholder (neutral_placeholder.go);
// refusal text rendered here is reserved exclusively for the user-facing
// session-history restore handler (Plan 10-03), where visual consistency
// with the live chat stream is a deliberate UX choice.
//
// Decisions honored:
//   - D10-A2 (agent-path neutral placeholder).
//   - D10-C2 (rewrite output excluded from generation evidence chain;
//     refusal text likewise excluded — the agent path sees neither).
//
// The map mirrors internal/server/chat.go refusalReasonText verbatim so
// the restored history shows the exact sentences they saw live; unknown
// codes fall through to the generic fallback (matching chat.go's
// behavior). The rephrase-suggestion suffix is omitted because
// session_turn stores only the bare reason code (not the
// SuggestedAlternativeQuery) — restoring it would require speculative
// reconstruction.
func RenderRefusal(refusalReason string) string {
	msg, ok := refusalReasonText[refusalReason]
	if !ok {
		msg = refusalGenericFallback
	}
	return fmt.Sprintf("无法给出结论：%s。", msg)
}

// refusalReasonText is the mirror of internal/server/chat.go
// refusalReasonText. It is duplicated (not imported) to keep the
// history package free of any import cycle to internal/server — and to
// make the Phase 10 handler-only contract grep-verifiable from a single
// file (RenderRefusal's caller set is bounded to the handler, period).
var refusalReasonText = map[string]string{
	verdict.RefusalReasonNoAuthorizedSource: "未检索到可回答该问题的授权知识源",
	verdict.RefusalReasonLowRetrievalScore:  "检索到的条款与该问题相关度不足",
	verdict.RefusalReasonOutOfScope:         "该问题超出当前 Skill 的处理范围",
}

// refusalGenericFallback is the unknown-reason fallback (mirrors
// chat.go refusalText's miss branch verbatim).
const refusalGenericFallback = "无法基于已授权的知识源回答该问题"