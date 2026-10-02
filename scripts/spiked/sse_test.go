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

// Package main — Plan 01-03 SC-1 layer (b)+(c) tests.
//
// TestUIMessageChunkShape pins the UIMessageChunk JSON shape per category
// and the wire.ValidTypes whitelist (T-1-11).
// TestSpikeSSERoundTrip spins a Hertz server on a dynamic port and proves
// the full Hertz → HTTP → JSON parse round-trip preserves chunk envelopes
// including the Content-Type=text/event-stream header and the `data: <json>`
// frame format expected by the AI SDK v5 DefaultChatTransport parser.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/skillforge/skillforge/internal/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shapeTypes enumerates one representative per UIMessageChunk category
// (text / reasoning / tool / source / custom-data / finish). Each MUST
// be a member of wire.ValidTypes so EmitChunk accepts it. This list
// mirrors the 6 categories the spiked sse CLI handler emits so the
// shape test + the live demo stay aligned on the contract surface.
var shapeTypes = []string{
	"text-delta",
	"reasoning-delta",
	"tool-input-available",
	"source-url",
	"data-policy",
	"finish-step",
}

// TestUIMessageChunkShape validates:
//  1. snake_case Type variants are rejected by IsValidType (Pitfall 6).
//  2. Every shapeTypes member is whitelisted in wire.ValidTypes.
//  3. The JSON shape per category matches the AI SDK v5 wire spec
//     (kebab-case `type` field, camelCase body field names).
//
// This is SC-1 layer (b): typed-parts serialization.
func TestUIMessageChunkShape(t *testing.T) {
	// 1. snake_case rejected, kebab-case accepted — Pitfall 6 protection.
	assert.False(t, wire.UIMessageChunk{Type: "text_delta"}.IsValidType(),
		"snake_case text_delta must be rejected by IsValidType")
	assert.False(t, wire.UIMessageChunk{Type: "source_url"}.IsValidType(),
		"snake_case source_url must be rejected by IsValidType")
	assert.True(t, wire.UIMessageChunk{Type: "text-delta"}.IsValidType(),
		"kebab-case text-delta must be accepted")
	assert.True(t, wire.UIMessageChunk{Type: "source-url"}.IsValidType(),
		"kebab-case source-url must be accepted")

	// 2. Every shapeType is a ValidTypes member (so EmitChunk won't reject
	//    the CLI handler's frames).
	for _, ty := range shapeTypes {
		t.Run("whitelist/"+ty, func(t *testing.T) {
			assert.True(t, wire.UIMessageChunk{Type: ty}.IsValidType(),
				"%q should be a member of wire.ValidTypes (T-1-11 whitelist)", ty)
		})
	}

	// 3. JSON shape per category — kebab-case `type`, camelCase body fields.
	t.Run("text-delta/json", func(t *testing.T) {
		b, err := json.Marshal(wire.UIMessageChunk{
			Type: "text-delta", ID: "x", Delta: "hi",
		})
		require.NoError(t, err)
		s := string(b)
		assert.Contains(t, s, `"type":"text-delta"`)
		assert.Contains(t, s, `"id":"x"`)
		assert.Contains(t, s, `"delta":"hi"`)
	})

	t.Run("tool-input-available/json", func(t *testing.T) {
		b, err := json.Marshal(wire.UIMessageChunk{
			Type:       "tool-input-available",
			ToolCallID: "tc-1",
			ToolName:   "search",
			Input:      json.RawMessage(`{"q":"policy"}`),
		})
		require.NoError(t, err)
		s := string(b)
		assert.Contains(t, s, `"type":"tool-input-available"`)
		assert.Contains(t, s, `"toolCallId":"tc-1"`)
		assert.Contains(t, s, `"toolName":"search"`)
		assert.Contains(t, s, `"input":{"q":"policy"}`)
	})

	t.Run("source-url/json", func(t *testing.T) {
		b, err := json.Marshal(wire.UIMessageChunk{
			Type:     "source-url",
			SourceID: "src-1",
			URL:      "https://policy.example/auth-001",
			Title:    "Authority Policy 1",
		})
		require.NoError(t, err)
		s := string(b)
		assert.Contains(t, s, `"type":"source-url"`)
		assert.Contains(t, s, `"sourceId":"src-1"`)
		assert.Contains(t, s, `"url":"https://policy.example/auth-001"`)
		assert.Contains(t, s, `"title":"Authority Policy 1"`)
	})

	t.Run("data-policy/json", func(t *testing.T) {
		b, err := json.Marshal(wire.UIMessageChunk{
			Type: "data-policy",
			Data: json.RawMessage(`{"verdict":"compliant"}`),
		})
		require.NoError(t, err)
		s := string(b)
		assert.Contains(t, s, `"type":"data-policy"`)
		assert.Contains(t, s, `"data":{"verdict":"compliant"}`)
	})
}

// TestSpikeSSERoundTrip proves the Hertz → HTTP → parse round-trip
// preserves UIMessageChunk envelopes end-to-end (SC-1 layers b + c).
//
// It spins a Hertz server on a dynamic port with a handler that emits
// 3 chunks (text-start, text-delta "hi", finish-step), then makes a
// real HTTP request via stdlib http.Get and asserts:
//   - Content-Type response header contains "text/event-stream"
//   - Body contains a `data: {"type":"text-delta",...}` frame
//   - Re-parsing the body yields exactly 3 chunks with the right types
//
// This validates that Hertz's chunked-body SSE encoding de-chunks
// cleanly through the stdlib HTTP client and that the JSON inside each
// data: line round-trips into the UIMessageChunk struct.
func TestSpikeSSERoundTrip(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.GET("/api/spike/chat", func(ctx context.Context, c *app.RequestContext) {
		chunks := []wire.UIMessageChunk{
			{Type: "text-start", ID: "msg-1"},
			{Type: "text-delta", ID: "msg-1", Delta: "hi"},
			{Type: "finish-step"},
		}
		for _, ch := range chunks {
			if err := wire.EmitChunk(c, ch); err != nil {
				t.Logf("emit err (%s): %v", ch.Type, err)
				return
			}
		}
	})

	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()
	defer func() {
		_ = h.Close()
		<-errCh
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := waitForListener(context.Background(), addr); err != nil {
		t.Fatalf("server warmup: %v", err)
	}

	url := fmt.Sprintf("http://%s/api/spike/chat", addr)
	resp, err := http.Get(url)
	require.NoError(t, err, "GET /api/spike/chat")
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	assert.Contains(t, ct, "text/event-stream",
		"Content-Type must be text/event-stream (auto-set by sse.NewWriter at writer.go:46)")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read response body")
	bodyStr := string(body)

	assert.Contains(t, bodyStr, `data: {"type":"text-delta","id":"msg-1","delta":"hi"}`,
		"frame format must be `data: <json>\\n` (verified writer.go:140-150)")

	frames, err := parseSSEFrames(body)
	require.NoError(t, err, "parse frames from body")

	require.Len(t, frames, 3, "expected exactly 3 frames (text-start + text-delta + finish-step)")
	assert.Equal(t, "text-start", frames[0].Type)
	assert.Equal(t, "text-delta", frames[1].Type)
	assert.Equal(t, "hi", frames[1].Delta)
	assert.Equal(t, "finish-step", frames[2].Type)
}

// TestEmitChunkRejectsUnknownType pins the T-1-11 mitigation: EmitChunk
// must reject unknown or snake_case Type values before any wire write.
// Without this guard, a typo'd type would serialize to the wire and be
// silently dropped by the AI SDK v5 parser, producing a confusing
// "frame missing" client-side bug.
func TestEmitChunkRejectsUnknownType(t *testing.T) {
	ctx := context.Background()
	// Register a handler that ATTEMPTS to emit a bad chunk; the handler
	// should observe an error from EmitChunk and abort before writing.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	emitErr := make(chan error, 1)
	h.GET("/api/spike/chat", func(_ context.Context, c *app.RequestContext) {
		// snake_case variant — must be rejected.
		err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "text_delta"})
		emitErr <- err
	})

	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()
	defer func() {
		_ = h.Close()
		<-errCh
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	require.NoError(t, waitForListener(ctx, addr), "server warmup")

	resp, err := http.Get(fmt.Sprintf("http://%s/api/spike/chat", addr))
	require.NoError(t, err)
	defer resp.Body.Close()

	select {
	case err := <-emitErr:
		require.Error(t, err, "EmitChunk must error on snake_case type")
		assert.Contains(t, err.Error(), "text_delta")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for handler to report EmitChunk error")
	}
}
