// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 57-02/57-03 (ONT-02/ONT-03) -- the orthogonality + degradation
// invariants, pinned as tests:
//
//	A. A missing KG degrades OPENLY: Available=false, no fragment, no
//	   crafted citations -- the chat keeps running on pure vector.
//	B. A working KG with no matching entity is NOT a degradation.
//	C. Anchored entities produce a PROMPT fragment only -- nothing from
//	   the KG can enter v.Citations (the citation-laundering red line).
//	D. Unverified answer entities downgrade the verdict (review_required
//	   + confidence cap + needs_input).
package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/ontology/anchor"
	"github.com/skillforge/skillforge/internal/ontology/store"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/verdict"
)

const labelPred = anchor.LabelPredicate

func kgDeps() *Deps {
	kg := store.New()
	kg.Insert(store.Triple{
		S: "http://skillforge.local/ontology#tbl_travel",
		P: labelPred,
		O: `"差旅住宿费标准"`,
	})
	kg.Insert(store.Triple{
		S: "http://skillforge.local/ontology#tbl_travel",
		P: "http://skillforge.local/ontology#belongsToTier",
		O: "http://skillforge.local/ontology#authority",
	})
	return &Deps{Ontology: &OntologyDeps{KG: kg}}
}

func travelManifest() *skill.Manifest {
	return &skill.Manifest{
		Name:  "compliance",
		Reads: []string{"authority"},
	}
}

func TestBuildKGContextDegradesOpenlyWithoutKG(t *testing.T) {
	// No ontology at all.
	kgc := (&Deps{}).buildKGContext(context.Background(), "差旅住宿费标准是多少", travelManifest())
	assert.False(t, kgc.Available)
	assert.Equal(t, 0, kgc.EntitiesMatched)
	assert.Empty(t, kgc.PromptFragment,
		"no KG must not produce a context block")
}

func TestBuildKGContextWorkingKGWithoutMatchIsNotDegradation(t *testing.T) {
	kgc := kgDeps().buildKGContext(context.Background(), "今天天气如何", travelManifest())
	assert.True(t, kgc.Available,
		"a working KG that found no entity is an honest zero, not a degradation")
	assert.Equal(t, 0, kgc.EntitiesMatched)
	assert.Empty(t, kgc.PromptFragment)
}

func TestBuildKGContextAnchorsAndRendersPromptOnly(t *testing.T) {
	kgc := kgDeps().buildKGContext(context.Background(), "差旅住宿费标准是多少", travelManifest())
	require.True(t, kgc.Available)
	require.Equal(t, 1, kgc.EntitiesMatched)
	assert.Equal(t, "差旅住宿费标准", kgc.Anchors[0].Label)
	assert.Contains(t, kgc.PromptFragment, "Knowledge-graph context")
	assert.Contains(t, kgc.PromptFragment, "差旅住宿费标准")
	assert.Contains(t, kgc.PromptFragment, "Cite only chunk sources",
		"the fragment must tell the model the KG is context, not evidence")
}

func TestKGFragmentNeverBecomesACitation(t *testing.T) {
	// This is the red-line test: the KG produces a prompt fragment, and a
	// verdict built from vector hits keeps EXACTLY its own citations.
	kgc := kgDeps().buildKGContext(context.Background(), "差旅住宿费标准是多少", travelManifest())
	require.NotEmpty(t, kgc.PromptFragment)

	// A verdict whose citations came from the vector path only.
	v := verdict.Verdict{
		Status:     "ok",
		Confidence: 0.9,
		Citations: []verdict.Citation{
			{ChunkID: "1", Span: "员工出差住宿费超过标准额度", Tier: "权威"},
		},
	}
	chunksByID := map[string]string{"1": "员工出差住宿费超过标准额度时，须分管副总审批。"}
	out := verdict.GroundingChecker(v, chunksByID)
	assert.Len(t, out.Citations, 1)
	assert.Equal(t, "1", out.Citations[0].ChunkID,
		"KG context must never append a citation of its own")
}

func TestApplyEntityGroundingDowngradesUnverifiedAnswerEntities(t *testing.T) {
	deps := kgDeps()
	v := verdict.Verdict{
		Status:     "ok",
		Confidence: 0.97,
		Answer:     "根据「差旅住宿费标准」，由「虚构的审批委员会」批准。",
		Citations:  []verdict.Citation{},
	}
	out := deps.applyEntityGrounding(v)
	require.NotNil(t, out.Entities)
	assert.Contains(t, out.Entities.Verified, "差旅住宿费标准")
	assert.Contains(t, out.Entities.Unverified, "虚构的审批委员会")
	assert.True(t, out.ReviewRequired)
	assert.LessOrEqual(t, out.Confidence, verdict.ConfidenceFloor)
	assert.Equal(t, verdict.DowngradeStatus, out.Status)
}

func TestApplyEntityGroundingWithoutKGLeavesVerdictUntouched(t *testing.T) {
	v := verdict.Verdict{
		Status:     "ok",
		Confidence: 0.97,
		Answer:     "由「虚构的审批委员会」批准。",
		Citations:  []verdict.Citation{},
	}
	out := (&Deps{}).applyEntityGrounding(v)
	assert.Nil(t, out.Entities, "no KG == grounding did not run; the payload stays unchanged")
	assert.Equal(t, v.Status, out.Status)
	assert.Equal(t, v.Confidence, out.Confidence)
	assert.False(t, out.ReviewRequired)
}

// TestEntityGroundingDoesNotDowngradeWithoutDictionaryCoverage is the
// 2026-11-25 live regression: the corpus KG covers the insurance
// business domain and NOTHING of the HR/policy domain the authority
// tier answers, so every policy answer produced unverified-only
// reports and was downgraded to needs_input @0.89 -- with byte-exact
// citations. The gate must report the finding without downgrading.
func TestEntityGroundingDoesNotDowngradeWithoutDictionaryCoverage(t *testing.T) {
	// A KG that knows ONLY an unrelated insurance entity.
	kg := store.New()
	kg.Insert(store.Triple{
		S: "http://skillforge.local/ontology#ent_chexian",
		P: labelPred,
		O: `"车险"`,
	})
	kg.Insert(store.Triple{
		S: "http://skillforge.local/ontology#ent_chexian",
		P: "http://skillforge.local/ontology#belongsToTier",
		O: "http://skillforge.local/ontology#sql-guides",
	})
	deps := &Deps{Ontology: &OntologyDeps{KG: kg}}

	v := verdict.Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Answer:     "住宿费超标 200 元以内的部分由部门负责人审批；超过 200 元的部分须附书面情况说明。",
	}
	out := deps.applyEntityGrounding(v)

	// The report is still attached (the finding stays visible + auditable).
	require.NotNil(t, out.Entities)
	assert.Empty(t, out.Entities.Verified)

	// ...but the verdict is NOT downgraded: zero dictionary coverage
	// means the check has no discriminating power over this answer.
	assert.Equal(t, "ok", out.Status,
		"zero dictionary coverage must not downgrade a byte-exact-cited answer")
	assert.False(t, out.ReviewRequired)
	assert.Equal(t, 0.95, out.Confidence)
}

// TestEntityGroundingDowngradesWhenDictionaryHasCoverage keeps the
// ORIGINAL red line intact: when the dictionary DOES confirm at least
// one entity, an unverified remainder is a real signal.
func TestEntityGroundingDowngradesWhenDictionaryHasCoverage(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{
		S: "http://skillforge.local/ontology#tbl_travel",
		P: labelPred,
		O: `"差旅住宿费标准"`,
	})
	kg.Insert(store.Triple{
		S: "http://skillforge.local/ontology#tbl_travel",
		P: "http://skillforge.local/ontology#belongsToTier",
		O: "http://skillforge.local/ontology#sql-guides",
	})
	deps := &Deps{Ontology: &OntologyDeps{KG: kg}}

	v := verdict.Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Answer:     "依据「差旅住宿费标准」，员工住宿由「虚构的审批委员会」批准。",
	}
	out := deps.applyEntityGrounding(v)

	require.NotNil(t, out.Entities)
	assert.Contains(t, out.Entities.Verified, "差旅住宿费标准")
	assert.Contains(t, out.Entities.Unverified, "虚构的审批委员会")
	assert.Equal(t, verdict.DowngradeStatus, out.Status,
		"with dictionary coverage, an unverified entity is a real risk signal")
	assert.True(t, out.ReviewRequired)
	assert.LessOrEqual(t, out.Confidence, verdict.ConfidenceFloor)
}

// TestOnlyEntityDowngradeDistinguishesPolicyGaps pins the predicate the
// redline pass uses to avoid attaching a fabricated clause rewrite to a
// correct answer.
func TestOnlyEntityDowngrade(t *testing.T) {
	assert.False(t, onlyEntityDowngrade(verdict.Verdict{Status: "ok"}),
		"a healthy verdict has no downgrade to explain")
	assert.False(t, onlyEntityDowngrade(verdict.Verdict{
		Status: verdict.DowngradeStatus,
	}),
		"needs_input WITHOUT an entity report is a citation/other gap, not entity-only")
	assert.False(t, onlyEntityDowngrade(verdict.Verdict{
		Status:   verdict.DowngradeStatus,
		Entities: &verdict.EntityGrounding{Verified: []string{"x"}, Unverified: []string{"y"}},
	}),
		"a verified entity means the dictionary had coverage -- a real signal")
	assert.True(t, onlyEntityDowngrade(verdict.Verdict{
		Status:   verdict.DowngradeStatus,
		Entities: &verdict.EntityGrounding{Unverified: []string{"住宿费超标"}},
	}),
		"unverified-only + needs_input is exactly the no-coverage shape")
}
