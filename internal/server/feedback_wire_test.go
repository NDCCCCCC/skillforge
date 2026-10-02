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

package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/wire"
)

// Phase 29 / Plan 29-04 Task 1 — the D15 paired data-turn registration
// + emitTurnID emission helper. Three locks:
//
//  1. TestValidTypes_ContainsDataTurn   — the 24th wire type is
//     registered kebab-case only (snake_case stays rejected, T-1-11);
//     mirrors TestValidTypes_Twenty in internal/wire.
//  2. TestEmitTurnID — table-driven over the degradation guard
//     (turnID<=0 → zero bytes out) and the happy path (turnId=7 → one
//     frame, correct Type + payload, IsValidType true).

// TestValidTypes_ContainsDataTurn — Plan 29-04 Task 1: `data-turn` is
// the 24th registered wire type. The kebab-case discriminator is the
// ONLY accepted spelling; the snake_case variant must stay rejected so
// a typo'd emitter fails loudly at the pre-write gate instead of being
// silently dropped by the AI SDK v5 parser (T-1-11).
func TestValidTypes_ContainsDataTurn(t *testing.T) {
	assert.True(t, wire.ValidTypes["data-turn"],
		"data-turn MUST be registered (24th wire type, FBK-01 turn identity, Plan 29-04)")
	assert.False(t, wire.ValidTypes["data_turn"],
		"snake_case data_turn MUST NOT be in ValidTypes (T-1-11 silent-drop regression)")

	// Round-trip the payload shape: a single camelCase turnId field, no
	// extra keys (the TS mirror TurnPayload reads exactly this).
	raw, err := json.Marshal(wire.TurnPayload{TurnID: 42})
	require.NoError(t, err)
	assert.JSONEq(t, `{"turnId":42}`, string(raw),
		"TurnPayload must marshal to exactly {turnId} (FE vote-key contract)")

	// The emitted chunk must pass the pre-write gate.
	chunk := wire.UIMessageChunk{Type: "data-turn", Data: raw}
	assert.True(t, chunk.IsValidType(),
		"a data-turn chunk MUST pass EmitChunk's ValidTypes gate")
}

// emitTurnIDViaHertz runs emitTurnID inside a real Hertz handler on a
// dynamic listener and returns the raw SSE response body, so the
// assertion covers the actual bytes a client receives. A real listener
// (not ut.PerformRequest) is required because sse.NewWriter hijacks the
// response writer — the recorder does not capture hijacked writes
// (mirrors newTestHertz's dynamic-port pattern in chat_test.go).
func emitTurnIDViaHertz(t *testing.T, turnID int64) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.GET("/api/test/emit-turn-id", func(ctx context.Context, c *app.RequestContext) {
		if err := emitTurnID(c, turnID); err != nil {
			t.Logf("emitTurnID error: %v", err)
		}
	})
	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()
	defer func() { _ = h.Close(); <-errCh }()

	addr := "127.0.0.1:" + portStr(port)
	require.NoError(t, waitForListener(addr, 3*time.Second), "server warmup")

	resp, err := http.Get("http://" + addr + "/api/test/emit-turn-id")
	require.NoError(t, err, "GET /api/test/emit-turn-id")
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read body")
	return string(raw)
}

// TestEmitTurnID — the degradation + happy-path contract:
//
//   - turnID <= 0 writes ZERO bytes: no frame, no partial JSON, nothing
//     for the FE to mistake for a usable vote key (idempotent replay /
//     store degradation must not fabricate identity, T-29-06).
//   - turnID > 0 writes exactly one frame whose Type is "data-turn" and
//     whose payload is {"turnId":N}.
func TestEmitTurnID(t *testing.T) {
	t.Run("zero_id_emits_nothing", func(t *testing.T) {
		body := emitTurnIDViaHertz(t, 0)
		assert.Equal(t, "", body,
			"turnID=0 (replay/degradation) must emit zero bytes — no fabricated identity (T-29-06)")
		assert.NotContains(t, body, "data-turn", "no data-turn frame may appear")
	})

	t.Run("negative_id_emits_nothing", func(t *testing.T) {
		body := emitTurnIDViaHertz(t, -3)
		assert.Equal(t, "", body,
			"turnID<0 must emit zero bytes (defensive mirror of the zero guard)")
	})

	t.Run("positive_id_emits_one_frame", func(t *testing.T) {
		body := emitTurnIDViaHertz(t, 7)
		frames := parseFrames(t, body)
		require.Len(t, frames, 1,
			"exactly one SSE frame expected; body=%q", body)
		assert.Equal(t, "data-turn", frames[0].Type,
			"frame Type must be the kebab-case discriminator")
		var p wire.TurnPayload
		require.NoError(t, json.Unmarshal(frames[0].Data, &p),
			"payload must unmarshal into wire.TurnPayload; raw=%s", string(frames[0].Data))
		assert.Equal(t, int64(7), p.TurnID,
			"payload must carry the caller's turn id verbatim")
		assert.True(t, strings.Contains(body, `"turnId":7`),
			"wire payload must be the camelCase turnId field (FE contract); body=%q", body)
	})
}
