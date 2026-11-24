// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 57-03 (ONT-03) -- entity grounding pins.
package verdict

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMatcher struct{ known map[string]bool }

func (f fakeMatcher) Known(name string) bool { return f.known[name] }

func TestCheckEntityGroundingNilMatcherDidNotRun(t *testing.T) {
	assert.Nil(t, CheckEntityGrounding("差旅住宿费标准", nil),
		"no KG wired == grounding did not run (nil, NOT an empty failure report)")
}

func TestCheckEntityGroundingEmptyCandidatesIsARealResult(t *testing.T) {
	rep := CheckEntityGrounding("", fakeMatcher{})
	require.NotNil(t, rep)
	assert.NotNil(t, rep.Verified)
	assert.NotNil(t, rep.Unverified)
	assert.Empty(t, rep.Unverified)
}

func TestCheckEntityGroundingSplitsVerifiedFromUnverified(t *testing.T) {
	rep := CheckEntityGrounding(
		"根据「差旅住宿费标准」，员工住宿由「虚构的审批委员会」批准。",
		fakeMatcher{known: map[string]bool{"差旅住宿费标准": true}},
	)
	require.NotNil(t, rep)
	assert.Contains(t, rep.Verified, "差旅住宿费标准")
	assert.Contains(t, rep.Unverified, "虚构的审批委员会")
}

func TestCheckEntityGroundingCapsReportedEntities(t *testing.T) {
	var answer string
	for i := 0; i < 60; i++ {
		answer += "「实体" + string(rune('A'+i%26)) + string(rune('A'+i/26)) + "」"
	}
	rep := CheckEntityGrounding(answer, fakeMatcher{})
	require.NotNil(t, rep)
	assert.LessOrEqual(t, len(rep.Unverified), MaxReportedEntities,
		"an adversarial answer cannot inflate the payload unbounded")
}

func TestExtractEntityCandidatesIgnoresShortFragmentsAndIsStable(t *testing.T) {
	got := ExtractEntityCandidates("好的，根据「差旅住宿费标准」和《年假制度》。")
	assert.Contains(t, got, "差旅住宿费标准")
	assert.Contains(t, got, "年假制度")
	// Sorted + deduped: the same input always yields the same list.
	again := ExtractEntityCandidates("好的，根据「差旅住宿费标准」和《年假制度》。")
	assert.Equal(t, got, again)
	for _, name := range got {
		assert.GreaterOrEqual(t, len([]rune(name)), minEntityRunes,
			"1-2 char fragments would make the check meaningless")
	}
}

func TestApplyEntityGroundingDowngradesOnUnverified(t *testing.T) {
	v := Verdict{Status: "ok", Confidence: 0.97, Citations: []Citation{}, ComplianceStatus: "compliant"}
	out := ApplyEntityGrounding(v, &EntityGrounding{Unverified: []string{"虚构实体"}})
	assert.True(t, out.ReviewRequired)
	assert.LessOrEqual(t, out.Confidence, ConfidenceFloor)
	assert.Equal(t, DowngradeStatus, out.Status)
	assert.Equal(t, DowngradeStatus, out.ComplianceStatus,
		"the compliance badge must not keep saying compliant while the platform status is needs_input")
}

func TestApplyEntityGroundingNoOps(t *testing.T) {
	v := Verdict{Status: "ok", Confidence: 0.97, Citations: []Citation{}}
	assert.Equal(t, v, ApplyEntityGrounding(v, nil), "nil report == not run")
	assert.Equal(t, v, ApplyEntityGrounding(v, &EntityGrounding{Verified: []string{"差旅住宿费标准"}}),
		"an all-verified report must not downgrade")
	// Idempotent on the failure path.
	once := ApplyEntityGrounding(v, &EntityGrounding{Unverified: []string{"x"}})
	assert.Equal(t, once, ApplyEntityGrounding(once, &EntityGrounding{Unverified: []string{"x"}}))
}
