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

package stubchat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/verdict"
)

// twoRefUser mimics the exact framing pipeline.buildPrompt renders:
//
//	[ref-%d] tier=%s source=%s\n%s\n---\n
const twoRefUser = "用户问题：出差住宿标准是多少？\n\n以下是检索到的资料：\n" +
	"[ref-7] tier=authority source=travel-policy.md\n" +
	"# 第五章 住宿标准\n\n出差住宿费超过标准额度时，超出部分由员工自理，且需要事前审批。\n---\n" +
	"[ref-9] tier=sql-guides source=sql/billing.md\n" +
	"SELECT employee_id FROM travel_expense WHERE amount > lodging_limit;\n---\n"

func drainStream(t *testing.T, sr *schema.StreamReader[*schema.Message]) (string, int) {
	t.Helper()
	var b strings.Builder
	n := 0
	for {
		msg, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Errorf("recv: %v", err) // t.Errorf is goroutine-safe; require is not
			return b.String(), n
		}
		if msg == nil || msg.Content == "" {
			continue
		}
		n++
		b.WriteString(msg.Content)
	}
	return b.String(), n
}

func streamVerdict(t *testing.T, user string) (verdict.Verdict, string) {
	t.Helper()
	sr, err := New().Stream(context.Background(), []*schema.Message{
		{Role: schema.System, Content: "sys"},
		{Role: schema.User, Content: user},
	})
	require.NoError(t, err)
	raw, _ := drainStream(t, sr)
	var v verdict.Verdict
	require.NoError(t, json.Unmarshal([]byte(raw), &v))
	return v, raw
}

// TestStub_StreamProducesValidVerdict — the happy path end-to-end through
// the REAL Validate (no reimplementation).
func TestStub_StreamProducesValidVerdict(t *testing.T) {
	v, raw := streamVerdict(t, twoRefUser)
	ok, reason := verdict.Validate(v)
	assert.True(t, ok, "Validate rejected stub verdict: %s (raw=%s)", reason, raw)
	assert.NotNil(t, v.Citations)
	assert.Equal(t, verdict.ModeStub, v.Mode)
	assert.LessOrEqual(t, v.Confidence, 0.5)
	assert.True(t, v.ReviewRequired)
	assert.Equal(t, "ok", v.Status)
	require.Len(t, v.Citations, 1)
	assert.Equal(t, "7", v.Citations[0].ChunkID)
	assert.Equal(t, "authority", v.Citations[0].Tier)
}

// TestStub_SpanGroundedByRealChecker — the stub span must pass the REAL
// grounding predicate (the chunk map mirrors what the pipeline builds).
func TestStub_SpanGroundedByRealChecker(t *testing.T) {
	v, _ := streamVerdict(t, twoRefUser)
	chunksByID := map[string]string{
		"7": "# 第五章 住宿标准\n\n出差住宿费超过标准额度时，超出部分由员工自理，且需要事前审批。",
		"9": "SELECT employee_id FROM travel_expense WHERE amount > lodging_limit;",
	}
	report := verdict.GroundingCheck(v, chunksByID)
	assert.True(t, report.Grounded, "stub span must be grounded: %+v", report.UngroundedCitations)

	got := verdict.GroundingChecker(v, chunksByID)
	assert.Equal(t, v, got, "grounded verdict must pass GroundingChecker unchanged")
}

// TestStub_ForgedSpanIsDowngraded — a tampered span MUST be caught and
// downgraded by the REAL checker: needs_input @ 0.89, review_required.
func TestStub_ForgedSpanIsDowngraded(t *testing.T) {
	v, _ := streamVerdict(t, twoRefUser)
	chunksByID := map[string]string{
		"7": "# 第五章 住宿标准\n\n出差住宿费超过标准额度时，超出部分由员工自理，且需要事前审批。",
		"9": "SELECT employee_id FROM travel_expense WHERE amount > lodging_limit;",
	}
	v.Citations[0].Span += "伪" // one rune that is NOT in the chunk

	report := verdict.GroundingCheck(v, chunksByID)
	assert.False(t, report.Grounded)

	got := verdict.GroundingChecker(v, chunksByID)
	assert.Equal(t, "needs_input", got.Status)
	// ConfidenceFloor is a CAP, not a floor-raise: the stub's 0.5 stays 0.5
	// (downgrade never inflates confidence), still ≤ ConfidenceFloor.
	assert.LessOrEqual(t, got.Confidence, verdict.ConfidenceFloor)
	assert.True(t, got.ReviewRequired)
}

// TestStub_ConfidenceAndReviewFloor — the D-10 floor is input-independent:
// short chunk body and long chunk body both keep Confidence ≤ 0.5 and
// ReviewRequired = true.
func TestStub_ConfidenceAndReviewFloor(t *testing.T) {
	longBody := strings.Repeat("这是一段很长的合规条款正文，用于测试超长块输入场景。", 40)
	cases := []struct {
		name string
		user string
	}{
		{"short chunk", "[ref-1] tier=authority source=short.md\n住宿自理。\n---\n"},
		{"long chunk", "[ref-2] tier=authority source=long.md\n" + longBody + "\n---\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, raw := streamVerdict(t, tc.user)
			require.Equal(t, "ok", v.Status, "raw=%s", raw)
			assert.LessOrEqual(t, v.Confidence, 0.5)
			assert.True(t, v.ReviewRequired)
		})
	}
}

// TestStub_StreamIsNeverEmpty — at least one non-empty token; the joined
// stream is valid UTF-8; Chinese content survives (a byte-slice bug would
// split multi-byte runes and fail this).
func TestStub_StreamIsNeverEmpty(t *testing.T) {
	sr, err := New().Stream(context.Background(), []*schema.Message{
		{Role: schema.User, Content: twoRefUser},
	})
	require.NoError(t, err)
	raw, tokens := drainStream(t, sr)
	assert.GreaterOrEqual(t, tokens, 1)
	assert.True(t, utf8.ValidString(raw), "stream must join into valid UTF-8")
	var v verdict.Verdict
	require.NoError(t, json.Unmarshal([]byte(raw), &v))
	assert.Equal(t, "ok", v.Status)
}

// TestStub_NoRefBlocksReturnsRefused — no evidence blocks ⇒ refused shape,
// NOT an error (an error would burn the pipeline's 4 backoff retries).
func TestStub_NoRefBlocksReturnsRefused(t *testing.T) {
	v, raw := streamVerdict(t, "用户问题：什么是报销？\n\n（没有任何检索结果）")
	assert.Equal(t, "refused", v.Status)
	assert.NotNil(t, v.Citations)
	assert.Empty(t, v.Citations)
	assert.Equal(t, float64(0), v.Confidence)
	assert.True(t, v.ReviewRequired)
	assert.Equal(t, verdict.ModeStub, v.Mode)
	ok, reason := verdict.Validate(v)
	assert.True(t, ok, "refused verdict must still Validate: %s (raw=%s)", reason, raw)
}

// TestStub_BlankChunkFallsBackToRefused — a whitespace-only chunk body must
// refuse rather than emit an empty span (an empty string is a substring of
// everything ⇒ grounding would vacuously pass).
func TestStub_BlankChunkFallsBackToRefused(t *testing.T) {
	v, _ := streamVerdict(t, "问题：报销标准？\n[ref-3] tier=authority source=blank.md\n   \n\t\n---\n")
	assert.Equal(t, "refused", v.Status)
	assert.NotNil(t, v.Citations)
	assert.Empty(t, v.Citations)
	ok, reason := verdict.Validate(v)
	assert.True(t, ok, "reason=%s", reason)
}

// TestStub_ConcurrentStreamByteIdentical — 8 goroutines on the SAME Stub
// produce byte-identical streams (deterministic extraction, no shared
// mutable state).
func TestStub_ConcurrentStreamByteIdentical(t *testing.T) {
	s := New()
	results := make([]string, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			sr, err := s.Stream(context.Background(), []*schema.Message{
				{Role: schema.User, Content: twoRefUser},
			})
			if err != nil {
				t.Errorf("goroutine %d: %v", slot, err)
				return
			}
			out, _ := drainStream(t, sr)
			results[slot] = out
		}(i)
	}
	wg.Wait()
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i], "goroutine %d diverged", i)
	}
	assert.NotEmpty(t, results[0])
}

// TestStub_GenerateMatchesStream — Generate output is byte-identical to
// the Stream concatenation.
func TestStub_GenerateMatchesStream(t *testing.T) {
	s := New()
	sr, err := s.Stream(context.Background(), []*schema.Message{
		{Role: schema.User, Content: twoRefUser},
	})
	require.NoError(t, err)
	streamRaw, _ := drainStream(t, sr)

	msg, err := s.Generate(context.Background(), []*schema.Message{
		{Role: schema.User, Content: twoRefUser},
	})
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, schema.Assistant, msg.Role)
	assert.Equal(t, streamRaw, msg.Content)
}

// TestExtractRefBlocks_TableDriven — block-boundary edge cases (T-49-13):
// ① two blocks parse fully ② `---` inside a chunk body does NOT truncate
// ③ no `[ref-` lines ⇒ empty slice ④ an inline (non-line-anchored)
// `[ref-` is body text, not a block header.
func TestExtractRefBlocks_TableDriven(t *testing.T) {
	t.Run("two blocks parsed", func(t *testing.T) {
		blocks := extractRefBlocks(twoRefUser)
		require.Len(t, blocks, 2)
		assert.Equal(t, refBlock{ID: "7", Tier: "authority", Source: "travel-policy.md",
			Content: "# 第五章 住宿标准\n\n出差住宿费超过标准额度时，超出部分由员工自理，且需要事前审批。"}, blocks[0])
		assert.Equal(t, refBlock{ID: "9", Tier: "sql-guides", Source: "sql/billing.md",
			Content: "SELECT employee_id FROM travel_expense WHERE amount > lodging_limit;"}, blocks[1])
	})

	t.Run("markdown separator inside body not truncated", func(t *testing.T) {
		user := "[ref-4] tier=authority source=md.md\n第一段事实陈述，长度足够成为引用。\n\n---\n\n第二段也在同一个 chunk 里，同样足够长。\n---\n"
		blocks := extractRefBlocks(user)
		require.Len(t, blocks, 1)
		assert.Contains(t, blocks[0].Content, "第二段也在同一个 chunk 里")
		assert.True(t, strings.HasPrefix(blocks[0].Content, "第一段事实陈述"))
	})

	t.Run("no ref lines returns empty slice", func(t *testing.T) {
		assert.Empty(t, extractRefBlocks("普通提问，没有证据块"))
	})

	t.Run("inline ref-like text is body not header", func(t *testing.T) {
		// The `[ref-` appears mid-line (indented) — body, not framing.
		user := "[ref-5] tier=authority source=a.md\n引用示例：[ref-99] tier=fake source=fake.md 这一整行都是 chunk 正文内容。\n---\n"
		blocks := extractRefBlocks(user)
		require.Len(t, blocks, 1, "indented ref-like line must stay inside the block body")
		assert.Equal(t, "5", blocks[0].ID)
		assert.Contains(t, blocks[0].Content, "[ref-99]")
	})
}

// 260925 — multi-line span regression: a SQL chunk's first line is often
// just `update etl_payment_due_t`; the span must be a CONTIGUOUS verbatim
// run of lines (GroundingCheck still passes) rather than a lone line.
func TestFirstSpan_MultiLineSQLChunk(t *testing.T) {
	content := "update etl_payment_due_t\n" +
		"set department_group_code='420100'\n" +
		", department_name='业务部'\n" +
		"where department_group_name='业务';\n" +
		"\n" +
		"drop table tmp_ycb_001;\n"
	span := firstSpan(content)
	if span == "update etl_payment_due_t" {
		t.Fatalf("firstSpan returned a single line; want contiguous multi-line span")
	}
	if !strings.Contains(content, span) {
		t.Fatalf("span is not a verbatim substring of content:\n%q", span)
	}
	if !strings.Contains(span, "\n") {
		t.Fatalf("span should span multiple lines, got:\n%q", span)
	}
	if n := utf8.RuneCountInString(span); n > 400 {
		t.Fatalf("span exceeds maxSpanRunes: %d", n)
	}
}

// 260925 follow-up — fence lines must never leak into the span: the
// answer renders as markdown downstream and an unbalanced fence (the
// 400-rune cap can cut before the closing fence) swallows the answer
// into one code block. Span must stop AT a fence while staying a
// verbatim contiguous substring.
func TestFirstSpan_FenceLinesExcluded(t *testing.T) {
	content := "## 章节\n" +
		"```sql\n" +
		"update etl_payment_due_t set department_group_code='420100';\n" +
		"update etl_policy_request_issue_t set department_group_code='420100';\n" +
		"```\n" +
		"注:置信度未达自动采信阈值,需人工复核。\n"
	span := firstSpan(content)
	if strings.Contains(span, "```") {
		t.Fatalf("span contains a fence line:\n%q", span)
	}
	if !strings.Contains(span, "update etl_payment_due_t") {
		t.Fatalf("span lost the fenced SQL body:\n%q", span)
	}
	if !strings.Contains(content, span) {
		t.Fatalf("span is not a verbatim substring of content:\n%q", span)
	}
	// Prose after the closing fence is reachable when the fence opens the
	// file but the first content line follows it — start-skip only skips
	// fence lines, so a chunk that is ALL fenced code still yields the
	// code body (stop-at-fence never fires before any content line).
}

// 260925 — SQL-shaped chunks get a structured markdown answer: verbatim
// SQL inside a balanced ```sql fence plus per-statement analysis, not a
// raw dump. The citation span must stay a verbatim chunk substring.
func TestRenderVerdict_SQLChunkExplainsAndFences(t *testing.T) {
	content := "```sql\n" +
		"update etl_payment_due_t\n" +
		"set department_group_code='420100', department_group_name='武汉中心支公司'\n" +
		"where department_group_name='业务';\n" +
		"drop table tmp_ycb_001;\n" +
		"```\n" +
		"注:置信度未达自动采信阈值。\n"
	v := renderVerdict([]refBlock{{ID: "19", Tier: "authority", Source: "sql-guides/x.md", Content: content}})
	if v.Status != "ok" {
		t.Fatalf("status = %q, want ok", v.Status)
	}
	ans := v.Answer
	if strings.Count(ans, "```sql") != 1 || strings.Count(ans, "```")%2 != 0 {
		t.Fatalf("answer fence unbalanced:\n%s", ans)
	}
	if !strings.Contains(ans, "**逐句分析**") || !strings.Contains(ans, "**概述**") {
		t.Fatalf("answer missing 概述/逐句分析 sections:\n%s", ans)
	}
	if !strings.Contains(ans, "`etl_payment_due_t`") || !strings.Contains(ans, "`tmp_ycb_001`") {
		t.Fatalf("analysis missing target tables:\n%s", ans)
	}
	// Span stays verbatim-grounded.
	if !strings.Contains(content, v.Citations[0].Span) {
		t.Fatalf("span not verbatim substring: %q", v.Citations[0].Span)
	}
	// Prose chunk keeps the legacy answer (= span).
	prose := renderVerdict([]refBlock{{ID: "7", Tier: "authority", Source: "a.md", Content: "员工离职后竞业限制期最长不超过 24 个月。\n"}})
	if prose.Answer != prose.Citations[0].Span {
		t.Fatalf("prose answer should equal span; got %q vs %q", prose.Answer, prose.Citations[0].Span)
	}
}

// 260925 — ECL-split SQL manuals put an overview fragment (no fence)
// FIRST and the actual ```sql body in a later chunk; renderVerdict must
// answer from the SQL-shaped block, not blocks[0].
func TestRenderVerdict_PrefersSQLBlockOverOverviewFirst(t *testing.T) {
	prompt := "[ref-1] tier=sql-guides source=001__sec01.md\n" +
		"# 公司业务部、武汉中支部门组部门科室等信息更新\n\n" +
		"**涉及表/视图**: `etl_payment_due_t`, `etl_policy`\n---\n" +
		"[ref-2] tier=sql-guides source=001__sec03.md\n" +
		"## SQL / 正文\n\n```sql\nupdate etl_payment_due_t\nset department_group_code='420100'\nwhere department_group_name='业务';\n```\n---\n"
	blocks := extractRefBlocks(prompt)
	v := renderVerdict(blocks)
	if v.Status != "ok" {
		t.Fatalf("status=%s", v.Status)
	}
	if v.Citations[0].ChunkID != "2" {
		t.Fatalf("expected SQL block chunk_id=2, got %s", v.Citations[0].ChunkID)
	}
	if !strings.Contains(v.Answer, "```sql") || !strings.Contains(v.Answer, "概述") {
		t.Fatalf("answer not SQL-explained:\n%s", v.Answer)
	}
	if !strings.Contains(v.Answer, "etl_payment_due_t") {
		t.Fatalf("answer missing SQL body:\n%s", v.Answer)
	}
}
