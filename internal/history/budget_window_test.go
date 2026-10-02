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
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTotalChars sums correctly across multiple messages.
func TestTotalChars(t *testing.T) {
	msgs := []schema.Message{
		{Role: schema.User, Content: "abc"},
		{Role: schema.Assistant, Content: "defgh"},
		{Role: schema.User, Content: ""},
	}
	assert.Equal(t, 8, totalChars(msgs))
}

// TestTruncateToCharBudget_NoTruncationWhenSmall — small slices fit
// unchanged.
func TestTruncateToCharBudget_NoTruncationWhenSmall(t *testing.T) {
	msgs := makeMessages(4, 100)
	kept, dropped, truncated := TruncateToCharBudget(msgs, 1000)
	assert.Equal(t, 4, len(kept))
	assert.Equal(t, 0, dropped)
	assert.False(t, truncated)
}

// TestTruncateToCharBudget_BudgetPrimaryKeepsMoreThanThreePairs —
// D10-B3 / T-10-03 primary contract. budget>0 with headroom keeps ALL
// messages regardless of message count (16 messages = 8 pairs in this
// case). This is the B1 regression test that fails if anyone wires the
// legacy 3-pair cap back into TruncateToCharBudget.
func TestTruncateToCharBudget_BudgetPrimaryKeepsMoreThanThreePairs(t *testing.T) {
	msgs := makeMessages(16, 40) // 16 × 40 = 640 chars total
	kept, dropped, truncated := TruncateToCharBudget(msgs, DefaultCharBudget)
	assert.Equal(t, 16, len(kept),
		"budget=DefaultCharBudget (8000) must keep all 16 messages (total=640); the legacy 3-pair cap is forbidden on this code path")
	assert.Equal(t, 0, dropped)
	assert.False(t, truncated)
}

// TestTruncateToCharBudget_OldestFirstDrop — when the slice overflows
// the budget, the OLDEST messages are dropped; the most-recent
// messages are preserved in original order.
func TestTruncateToCharBudget_OldestFirstDrop(t *testing.T) {
	msgs := makeMessages(10, 1200) // 10 × 1200 = 12000 chars
	kept, dropped, truncated := TruncateToCharBudget(msgs, DefaultCharBudget)
	assert.Equal(t, 6, len(kept),
		"6 × 1200 = 7200 fits in 8000; the oldest 4 must be dropped")
	assert.Equal(t, 4, dropped)
	assert.True(t, truncated)
	// Verify order: kept[0] should be the original messages[4].
	assert.Equal(t, msgs[4].Content, kept[0].Content)
	assert.Equal(t, msgs[9].Content, kept[5].Content)
}

// TestTruncateToCharBudget_SafeguardFloor — when the budget is too
// tight to honor, TruncateToCharBudget stops at SafeguardFloor (2
// messages) rather than collapsing to zero (T-10-03 DoS mitigation).
func TestTruncateToCharBudget_SafeguardFloor(t *testing.T) {
	msgs := makeMessages(100, 100) // 100 × 100 = 10000
	kept, _, truncated := TruncateToCharBudget(msgs, 10)
	assert.Equal(t, SafeguardFloor, len(kept),
		"the safeguard floor (2) is the absolute minimum; budget=10 cannot shrink the slice below 2 messages")
	assert.True(t, truncated)
	// Verify the kept messages are the most-recent 2.
	assert.Equal(t, msgs[98].Content, kept[0].Content)
	assert.Equal(t, msgs[99].Content, kept[1].Content)
}

// TestTruncateToCharBudget_ZeroBudgetLegacyFallback — budget<=0
// triggers the LEGACY fallback path. The most-recent
// FallbackMessageCeiling (6) messages are kept regardless of budget.
// This is the D9-B3 cap's only surviving application in Phase 10.
func TestTruncateToCharBudget_ZeroBudgetLegacyFallback(t *testing.T) {
	msgs := makeMessages(20, 100)
	kept, dropped, truncated := TruncateToCharBudget(msgs, 0)
	assert.Equal(t, FallbackMessageCeiling, len(kept),
		"budget=0 must fall back to FallbackMessageCeiling (6) messages")
	assert.Equal(t, 20-FallbackMessageCeiling, dropped)
	assert.True(t, truncated)
	// Order: the last 6 in original order.
	for i := 0; i < FallbackMessageCeiling; i++ {
		assert.Equal(t, msgs[20-FallbackMessageCeiling+i].Content, kept[i].Content)
	}
}

// TestTruncateToCharBudget_NegativeBudgetLegacyFallback — same as
// the zero case; any non-positive budget activates the fallback.
func TestTruncateToCharBudget_NegativeBudgetLegacyFallback(t *testing.T) {
	msgs := makeMessages(20, 100)
	kept, _, _ := TruncateToCharBudget(msgs, -1)
	assert.Equal(t, FallbackMessageCeiling, len(kept))
}

// TestTruncateToCharBudget_EmptyInput — no messages, no work.
func TestTruncateToCharBudget_EmptyInput(t *testing.T) {
	kept, dropped, truncated := TruncateToCharBudget(nil, 1000)
	assert.Nil(t, kept)
	assert.Equal(t, 0, dropped)
	assert.False(t, truncated)
}

// TestTruncateToCharBudget_ExactBudgetFit — when total == budget
// exactly, no truncation (the budget gate is inclusive).
func TestTruncateToCharBudget_ExactBudgetFit(t *testing.T) {
	msgs := makeMessages(4, 100) // 4 × 100 = 400
	kept, dropped, truncated := TruncateToCharBudget(msgs, 400)
	assert.Equal(t, 4, len(kept))
	assert.Equal(t, 0, dropped)
	assert.False(t, truncated)
}

// TestTruncateToCharBudget_JustOverBudget — one message over the
// budget drops exactly one from the head.
func TestTruncateToCharBudget_JustOverBudget(t *testing.T) {
	msgs := makeMessages(4, 100) // 4 × 100 = 400
	kept, dropped, truncated := TruncateToCharBudget(msgs, 399)
	assert.Equal(t, 3, len(kept))
	assert.Equal(t, 1, dropped)
	assert.True(t, truncated)
}

// makeMessages creates n alternating user/assistant messages each
// with content of exactly contentLen characters (deterministic and
// easy to reason about in assertions). The content is the digit
// pattern "0123456789..." repeated and trimmed, so the byte-length
// equals contentLen and is a stable input to totalChars.
func makeMessages(n, contentLen int) []schema.Message {
	msgs := make([]schema.Message, n)
	pattern := strings.Repeat("0123456789", contentLen/10+1)
	for i := 0; i < n; i++ {
		role := schema.User
		if i%2 == 1 {
			role = schema.Assistant
		}
		msgs[i] = schema.Message{
			Role:    role,
			Content: pattern[:contentLen],
		}
	}
	return msgs
}

// TestBudgetConstants_Pinned — the constants are the D10-B3 contract;
// CI guards pin them so an accidental edit trips a test rather than
// silently changing behavior.
func TestBudgetConstants_Pinned(t *testing.T) {
	require.Equal(t, 8000, DefaultCharBudget)
	require.Equal(t, 7500, ReservedForChunks)
	require.Equal(t, 600, ReservedForOutput)
	require.Equal(t, 2500, ReservedForSystemPrompt)
	require.Equal(t, 2000, ReservedForQueryAndUser)
	require.Equal(t, 6, FallbackMessageCeiling)
	require.Equal(t, 2, SafeguardFloor)
}