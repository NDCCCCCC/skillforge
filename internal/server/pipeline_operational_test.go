// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Regression tests for the sql-guides-needs-input debug session
// (.planning/debug/sql-guides-needs-input.md, 2026-09-14).
//
// The bug: a Skill whose resolved reads contain NO authority-role KB
// (sql-guides: role=personal) grounded against an EMPTY citation
// closure — citableChunksByID keeps authority-role chunks only (KBM-03,
// commit 8cbe270) — so every LLM citation failed GroundingCheck and
// ApplyDowngrade deterministically rewrote the verdict to
// needs_input @0.89 + review_required, then the grounded-filter
// stripped every citation. Audit row 443 is the live fingerprint. The
// redline gate then fired on Status==needs_input for ANY skill,
// producing a compliance clause-rewrite card on a SQL 取数 query.
//
// The fix introduces OPERATIONAL mode: a skill with zero authority
// tiers grounds against ALL retrieved chunks (anti-fabrication, D2-01)
// instead of the citable whitelist, and never triggers the redline
// pass. EVIDENCE mode (≥1 authority tier) keeps the citable-only
// closure byte-for-byte — the moat is untouched.

package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/verdict"
)

// operationalTestChunk is the standing personal-role chunk for these
// tests. Mirrors the live shape: tier=sql-guides, operational SQL guide
// content.
const operationalTestChunk = "公司业务部相关数据的取数口径：从 nonauto_item_list_t 表取数，按 org_code 过滤机构，按险种代码 risk_code 汇总保费"

// operationalTestDeps wires the default-path Deps for the operational
// tests: real on-disk skills tree (so postChat("…", "sql-guides")
// resolves the real personal-only SKILL.md), stub retriever/runner, and
// the seed registry (which carries sql-guides at role=personal —
// mirrors knowledge_bases.yaml AND the runtime sqlite row).
func operationalTestDeps(t *testing.T, chunks []retriever.Hit, cannedVerdict string) (*Deps, *stubChatModel) {
	t.Helper()
	deps := newTestDeps()
	loader := skill.NewLoader(skillsDirPath(), testKBReg())
	_, err := loader.Scan()
	require.NoError(t, err, "load real skills/ tree (incl. sql-guides)")
	deps.Loader = loader
	deps.Retriever = &stubRetriever{chunks: chunks}
	deps.Runner = newStubRunner(cannedVerdict)
	// The redline seam: a counting stub so the tests can assert the
	// operational mode makes ZERO compliance rewrite LLM calls.
	redlineStub := &stubChatModel{cannedJSON: `{"originalClause":"x","proposedRewrite":"y","rationale":"z"}`}
	deps.ChatModel = redlineStub
	return deps, redlineStub
}

// findVerdictFrame scans frames for the data-policy Verdict payload.
func findVerdictFrame(t *testing.T, body string) *verdict.Verdict {
	t.Helper()
	var v *verdict.Verdict
	for _, f := range parseFrames(t, body) {
		if f.Type == "data-policy" {
			var pv verdict.Verdict
			require.NoError(t, json.Unmarshal(f.Data, &pv))
			v = &pv
		}
	}
	return v
}

// TestVerdictPipeline_OperationalSkillGroundsAgainstRetrievedChunks —
// THE bug lock for audit row 443. A personal-only skill (sql-guides)
// whose LLM verdict cites a retrieved sql-guides chunk with a verbatim
// span MUST produce status=ok with the citation intact — NOT the
// deterministic needs_input@0.89 + stripped-citations downgrade.
//
// Pre-fix this test fails on every assertion: grounding ran against an
// empty citable closure, so ApplyDowngrade rewrote status/confidence/
// review_required and the grounded-filter emptied Citations.
func TestVerdictPipeline_OperationalSkillGroundsAgainstRetrievedChunks(t *testing.T) {
	v := verdict.Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Citations: []verdict.Citation{
			{ChunkID: "7", Span: "从 nonauto_item_list_t 表取数", Tier: "sql-guides"},
		},
		ReviewRequired:   false,
		Answer:           "公司业务部相关数据从 nonauto_item_list_t 表取数，按机构代码过滤",
		ComplianceStatus: verdict.NotApplicableStatus,
	}
	canned, _ := json.Marshal(v)
	deps, redlineStub := operationalTestDeps(t, []retriever.Hit{
		{ID: 7, Tier: "sql-guides", Content: operationalTestChunk, Source: "sql-guides/业务部/取数.md", Score: 0.9},
	}, string(canned))

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "公司业务部相关数据查询", "sql-guides")
	got := findVerdictFrame(t, body)
	require.NotNil(t, got, "data-policy Verdict must be on the wire")

	// 2026-11-10 方案 D：operational Skill 走 emitOperationalAnswer 直出
	// 通道 —— LLM 产出 markdown（经 guardOperationalSQL 代码校验），verdict
	// 由代码固定组装（ok / 0.9 / not_applicable / 空 citations）。canned
	// verdict 不再参与；本测试锁定的是「直出 verdict 不经过 ApplyDowngrade
	// 语义（citation 清空降级链）」这一不变量 + 零 redline 调用。
	assert.Equal(t, "ok", got.Status,
		"operational direct-out verdict is ok (no downgrade chain)")
	assert.Equal(t, 0.9, got.Confidence,
		"operational direct-out confidence is the fixed 0.9 (not the 0.89 downgrade cap)")
	assert.False(t, got.ReviewRequired,
		"operational direct-out must not force review_required")
	assert.Empty(t, got.Citations,
		"operational direct-out carries no citations (the quoted guide text IS the answer)")
	assert.Equal(t, verdict.NotApplicableStatus, got.ComplianceStatus,
		"no downgrade → ComplianceStatus keeps the LLM's not_applicable (never the ApplyDowngrade overwrite)")
	assert.Equal(t, 0, redlineStub.callCount(),
		"operational skill must make ZERO redline LLM calls")
}

// TestVerdictPipeline_OperationalSkillSuppressesRedlineOnNeedsInput —
// the redline-card half of the bug. When the LLM itself emits
// needs_input for an operational skill (a legitimate conditional case:
// the guide needs a parameter the user did not give), the verdict must
// keep its grounded citation and confidence, and the compliance
// clause-rewrite pass must NOT fire — a policy-drafting card is
// undefined for a skill that reads no authority KB.
//
// Pre-fix this test fails: the empty closure downgraded the verdict
// (0.89 cap, citation stripped, ComplianceStatus overwritten) AND the
// redline gate fired on Status==needs_input.
func TestVerdictPipeline_OperationalSkillSuppressesRedlineOnNeedsInput(t *testing.T) {
	v := verdict.Verdict{
		Status:     verdict.DowngradeStatus,
		Confidence: 0.92,
		Citations: []verdict.Citation{
			{ChunkID: "7", Span: "按 org_code 过滤机构", Tier: "sql-guides"},
		},
		ReviewRequired: true,
	}
	canned, _ := json.Marshal(v)
	deps, redlineStub := operationalTestDeps(t, []retriever.Hit{
		{ID: 7, Tier: "sql-guides", Content: operationalTestChunk, Source: "sql-guides/业务部/取数.md", Score: 0.9},
	}, string(canned))

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "公司业务部相关数据查询", "sql-guides")
	got := findVerdictFrame(t, body)
	require.NotNil(t, got, "data-policy Verdict must be on the wire")

	// 2026-11-10 方案 D：operational 通道不再调用 verdict LLM —— canned
	// needs_input 被直接绕过，verdict 由代码固定组装。本测试锁定的是
	// 「即使 LLM 声称 needs_input，operational Skill 也绝不触发 compliance
	// clause-rewrite（redline）」，这是 needs_input 场景在新架构下的残留
	// 语义。
	assert.Equal(t, "ok", got.Status,
		"operational direct-out verdict is ok regardless of the canned needs_input")
	assert.Equal(t, 0.9, got.Confidence,
		"operational direct-out confidence is the fixed 0.9 (no 0.89 confidence cap)")
	assert.Empty(t, got.Citations,
		"operational direct-out carries no citations")
	assert.Equal(t, "not_applicable", got.ComplianceStatus,
		"no downgrade overwrite path is reachable")
	assert.Equal(t, 0, redlineStub.callCount(),
		"operational skill must make ZERO redline LLM calls even on needs_input")
}

// TestVerdictPipeline_MixedReadsKeepCitableOnlyClosure — the moat lock.
// A skill WITH an authority tier (compliance) whose LLM cites ONLY a
// non-citable (personal) chunk must still be downgraded and stripped:
// operational mode requires ZERO authority tiers, so mixed-read skills
// keep the KBM-03 citable-only grounding closure byte-for-byte. This
// pins the fix's blast radius — it must never re-widen the closure for
// evidence-mode skills (the exact regression 8cbe270 closed).
func TestVerdictPipeline_MixedReadsKeepCitableOnlyClosure(t *testing.T) {
	v := verdict.Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Citations: []verdict.Citation{
			{ChunkID: "2", Span: "个人笔记里的私有口径", Tier: "personal"},
		},
		ReviewRequired:   false,
		ComplianceStatus: verdict.NotApplicableStatus,
	}
	canned, _ := json.Marshal(v)
	deps, redlineStub := operationalTestDeps(t, []retriever.Hit{
		{ID: 1, Tier: "authority", Content: "权威条款：差旅住宿费超标 200 元以内由部门负责人审批", Source: "authority/travel.md", Score: 0.95},
		{ID: 2, Tier: "personal", Content: "个人笔记里的私有口径：该场景建议直接找数据组", Source: "personal/notes.md", Score: 0.9},
	}, string(canned))

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "出差住宿费超标怎么审批", "compliance")
	got := findVerdictFrame(t, body)
	require.NotNil(t, got, "data-policy Verdict must be on the wire")

	assert.Equal(t, verdict.DowngradeStatus, got.Status,
		"evidence-mode citation of a non-citable chunk is still the D2-02 downgrade")
	assert.Equal(t, 0.89, got.Confidence,
		"downgrade caps confidence at ConfidenceFloor")
	assert.Empty(t, got.Citations,
		"non-citable citation is stripped (KBM-03 whitelist unchanged for mixed reads)")
	assert.Equal(t, verdict.DowngradeStatus, got.ComplianceStatus,
		"ApplyDowngrade propagates to ComplianceStatus exactly as before the fix")
	assert.Equal(t, 1, redlineStub.callCount(),
		"evidence-mode needs_input still fires the redline pass (D5-14 unchanged)")
}

// TestLooksLikeOperationalAnswer pins the operational-channel shape gate
// (2026-11-25): the branch used to stamp Status="ok" + Confidence=0.9 on
// ANY non-empty text, so a bare refusal phrase rendered as a confident
// answer.
func TestLooksLikeOperationalAnswer(t *testing.T) {
	for _, ok := range []string{
		"```sql\nselect * from t\n```",
		"SELECT policy_no FROM etl_payment_due_t",
		"with v_case_1 as (select ...) select * from v_case_1",
	} {
		assert.True(t, looksLikeOperationalAnswer(ok), "expected code-shaped: %q", ok)
	}
	for _, bad := range []string{
		"no_relevant_source",
		"抱歉，我无法回答这个问题。",
		"请提供更具体的取数口径。",
	} {
		assert.False(t, looksLikeOperationalAnswer(bad), "expected NOT code-shaped: %q", bad)
	}
}
