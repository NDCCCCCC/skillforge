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

// 2026-11-11 — 取数通道的"真实流式正文"。
//
// 背景（实测）：一条取数问题，5 条 data-subprogress 全在 0.30s 发完，
// 之后 66 秒没有任何帧，text-start 要等到 66.44s —— 因为 streamOnce
// 虽然逐 token 从 Eino 的 StreamReader 收到了内容，却只发
// text-delta-draft（草稿卡），正文要等整段生成 + JSON 解包 + guard
// 校验完才一次吐出。
//
// 这组测试锁住改造后的行为：
//  1. 取数通道的答案逐 token 以 text-delta 流出，且第一个 text-delta
//     出现在 data-policy 之前（用户先看到内容，再看到结构化结论）；
//  2. 只有一次 text-start（正文与收尾共用同一个 text part）；
//  3. 模型仍把答案包成 JSON 时，探针放弃直出，回到原来的整段解包路径
//     —— 正文里绝不出现 {"answer": ...} 的原始 JSON；
//  4. CR 归一：模型吐 \r\n 时正文里不留 CR（否则 web 上 SQL 会挤成一行）。
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/wire"
)

// postChatSkill 是 postChatRaw 的变体：可以指定 Skill（取数通道需要
// sql-guides，postChatRaw 固定用 compliance）。
func postChatSkill(t *testing.T, ctx context.Context, addr, sessionID, skillName, query string) (*http.Response, error) {
	t.Helper()
	body, _ := json.Marshal(chatRequest{Query: query, Skill: skillName})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+addr+"/api/chat", bytes.NewReader(body))
	require.NoError(t, err, "build request")
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.Header.Set("X-Session-Id", sessionID)
	}
	return http.DefaultClient.Do(req)
}

// operationalDeps 构造一个走取数通道的 Deps：sql-guides Skill + 一条
// 取数手册 chunk + 逐 token 的流式 LLM。
func operationalDeps(t *testing.T, tokens []string) *Deps {
	t.Helper()
	deps := newVerdictPipelineDeps(t)
	deps.Retriever = &stubRetriever{chunks: []retriever.Hit{{
		ID:      1,
		Tier:    "sql-guides",
		Content: "030 承保清单：主表 wuh_dw.etl_nonauto_policy_t，时间口径 inception_year。",
		Source:  "sql-guides/取数口径/030_承保清单.md",
		Score:   0.9,
	}}}
	deps.PipelineChatModel = &stubStreamingChatModel{tokens: tokens}
	return deps
}

// textDeltas 按 wire 顺序收集正文增量。
func textDeltas(frames []wire.UIMessageChunk) []string {
	var out []string
	for _, f := range frames {
		if f.Type == "text-delta" {
			out = append(out, f.Delta)
		}
	}
	return out
}

func indexOfType(frames []wire.UIMessageChunk, typ string) int {
	for i, f := range frames {
		if f.Type == typ {
			return i
		}
	}
	return -1
}

func countType(frames []wire.UIMessageChunk, typ string) int {
	n := 0
	for _, f := range frames {
		if f.Type == typ {
			n++
		}
	}
	return n
}

func TestOperationalAnswer_StreamsTokensAsTextDeltas(t *testing.T) {
	tokens := []string{
		"按 030 承保清单口径：",
		"主表 wuh_dw.etl_nonauto_policy_t，",
		"时间口径用 inception_year = '2024'。",
	}
	deps := operationalDeps(t, tokens)
	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	resp, err := postChatSkill(t, context.Background(), addr, "sess-stream-text", "sql-guides",
		"2024年襄阳的学平险保单怎么提？")
	require.NoError(t, err, "POST /api/chat (operational path)")
	body := drainBody(t, resp)
	frames := parseFrames(t, body)

	deltas := textDeltas(frames)
	require.Greater(t, len(deltas), 1,
		"正文必须逐 token 分多帧流出，而不是等整段生成完再一次性发 (got %d text-delta frames)", len(deltas))
	assert.Equal(t, strings.Join(tokens, ""), strings.Join(deltas, ""),
		"流出的正文必须与模型输出逐字一致")

	firstDelta := indexOfType(frames, "text-delta")
	policy := indexOfType(frames, "data-policy")
	require.Greater(t, firstDelta, -1, "must emit text-delta")
	require.Greater(t, policy, firstDelta,
		"正文必须在 data-policy 之前就开始流（用户先看到内容，再看到结构化结论）")

	assert.Equal(t, 1, countType(frames, "text-start"),
		"正文与收尾必须共用同一个 text part（只开一次 text-start）")
	assert.Equal(t, 1, countType(frames, "text-end"), "exactly one text-end")
	// 有正文时不能再整段重发一遍 —— 那会让同一段答案出现两次。
	assert.Greater(t, policy, firstDelta)
}

func TestOperationalAnswer_JSONWrappedAnswerFallsBackToWholeText(t *testing.T) {
	// 模型无视 "Do NOT wrap in JSON"，仍然返回 {"answer": "..."}。
	// 探针在头几个字符就该判定出来，正文必须回退到整段解包路径，
	// 绝不能把原始 JSON 流进对话。
	wrapped := `{"answer": "按 030 承保清单口径取数。", "status": "completed"}`
	deps := operationalDeps(t, []string{wrapped[:1], wrapped[1:]})
	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	resp, err := postChatSkill(t, context.Background(), addr, "sess-stream-json", "sql-guides",
		"2024年襄阳的学平险保单怎么提？")
	require.NoError(t, err)
	body := drainBody(t, resp)
	frames := parseFrames(t, body)
	deltas := textDeltas(frames)
	joined := strings.Join(deltas, "")

	require.NotEmpty(t, deltas, "must still emit the answer text")
	assert.NotContains(t, joined, `"answer"`,
		"原始 JSON 绝不能流进正文（探针必须回退到整段解包路径）")
	assert.Contains(t, joined, "按 030 承保清单口径取数",
		"解包后的 answer 字段才是正文")
	assert.Equal(t, 1, countType(frames, "text-start"),
		"回退路径仍然只开一次 text-start")
}

func TestOperationalAnswer_NormalizesCarriageReturnsWhileStreaming(t *testing.T) {
	// 模型被要求"逐字复制"时会吐 CR。CR 在 web 上不换行，会让 SQL 挤成
	// 一行、相邻 token 粘连（"awhere"）。归一必须在流式阶段就做。
	tokens := []string{"第一行\r\n", "第二行\r\n", "select a,\r", "\n b from t"}
	deps := operationalDeps(t, tokens)
	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	resp, err := postChatSkill(t, context.Background(), addr, "sess-stream-cr", "sql-guides",
		"2024年襄阳的学平险保单怎么提？")
	require.NoError(t, err)
	body := drainBody(t, resp)
	joined := strings.Join(textDeltas(parseFrames(t, body)), "")

	assert.NotContains(t, joined, "\r", "正文里不能留 CR（会让 SQL 挤成一行）")
	assert.Contains(t, joined, "第一行\n第二行\nselect a,\n b from t")
}
