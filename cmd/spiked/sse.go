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

// Package main — sse subcommand (SC-1 layers b + c).
//
// runSSE spins a Hertz server on a dynamic port, emits one chunk from
// each UIMessageChunk category (text / reasoning / tool-input / source-url
// / custom-data-policy / finish-step) as an SSE frame, then parses the
// response back and reports the frame count + types. This proves the AI
// SDK v5 UIMessageChunk envelope survives a Hertz → HTTP → JSON parse
// round-trip, which is SC-1 layer (b) typed-parts serialization +
// layer (c) client-side parse.
//
// Layer (a) wire format (text/event-stream + data: framing) is verified
// by the accompanying TestSpikeSSERoundTrip via direct header + body
// assertions on the raw HTTP response.
//
// Verified API anchors (hertz@v0.10.6):
//   - server.Default(opts...) *Hertz          (app/server/hertz.go:48)
//   - server.WithListener(ln net.Listener)    (app/server/option.go:176)
//   - (*Hertz).Run() error                    (embedded *route.Engine.Run)
//   - (*Hertz).Close() error                  (embedded *route.Engine.Close)
// Run blocks until Close; Close unblocks Run (verified via Spin() in
// hertz.go:56-79 which calls Run in a goroutine then waits on a signal
// before calling Shutdown/Close).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/skillforge/skillforge/internal/wire"
)

// runSSE is the Plan 01-03 spike entry point. It is wired into the
// subcommand router in main.go.
func runSSE(ctx context.Context, args []string) error {
	// Allocate a dynamic port (Pitfall 7: no hardcoded ports on shared
	// hosts). The listener is handed directly to Hertz via WithListener
	// so Hertz serves on the already-bound socket — no close-and-rebind
	// race, no port-reuse window.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.GET("/api/spike/chat", sseChatHandler)

	// Serve in background; h.Run() blocks until h.Close() is called.
	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()

	defer func() {
		_ = h.Close()
		<-errCh // let Run() return so the goroutine exits cleanly
	}()

	// Poll the listener via TCP dial until Hertz is accepting (no
	// fixed time.Sleep — startup latency varies by host).
	if err := waitForListener(ctx, fmt.Sprintf("127.0.0.1:%d", port)); err != nil {
		return fmt.Errorf("server warmup: %w", err)
	}

	// Stream the response. Use a per-call context with a generous
	// timeout so a hung server cannot wedge the spike indefinitely.
	getCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://127.0.0.1:%d/api/spike/chat", port)
	req, err := http.NewRequestWithContext(getCtx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		return fmt.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	frames, err := parseSSEFrames(body)
	if err != nil {
		return fmt.Errorf("parse frames: %w", err)
	}

	typeNames := make([]string, 0, len(frames))
	for _, f := range frames {
		typeNames = append(typeNames, f.Type)
	}

	fmt.Printf("[OK] spike sse: parsed %d frames (types: %s)\n",
		len(frames), strings.Join(typeNames, ", "))
	return nil
}

// sseChatHandler emits 6 representative UIMessageChunk frames spanning
// every category used in the SkillForge wire contract:
//
//  1. text-delta            — incremental assistant text
//  2. reasoning-delta       — model's reasoning trace (Phase 4 collapsible)
//  3. tool-input-available  — tool-call parsed args (search query here)
//  4. source-url            — citation URL chip (SC-4 clickable <a>)
//  5. data-policy           — SkillForge custom Verdict payload
//  6. finish-step           — end-of-step marker
//
// The 6 types match TestUIMessageChunkShape's shapeTypes iteration set
// exactly so the CLI demonstration and the unit-test assertions stay
// aligned on the contract surface.
//
// Inter-frame latency (20ms) simulates a real agent stream so the
// client observes actual streaming behavior; total stream budget
// (~120ms) stays well under the 10s spike wall-clock cap.
func sseChatHandler(ctx context.Context, c *app.RequestContext) {
	chunks := []wire.UIMessageChunk{
		{Type: "text-delta", ID: "msg-1", Delta: "Hello "},
		{Type: "reasoning-delta", ID: "r-1", Delta: "evaluating policy..."},
		{Type: "tool-input-available", ID: "tc-1", ToolCallID: "tc-1", ToolName: "search",
			Input: json.RawMessage(`{"q":"policy"}`)},
		{Type: "source-url", ID: "src-1",
			URL: "https://policy.example/auth-001", Title: "Authority Policy 1"},
		{Type: "data-policy",
			Data: json.RawMessage(`{"verdict":"compliant","confidence":0.95}`)},
		{Type: "finish-step"},
	}
	for i, ch := range chunks {
		if err := wire.EmitChunk(c, ch); err != nil {
			// Once the first frame is on the wire we cannot change the
			// response status; log to stderr and abort the stream.
			fmt.Fprintf(os.Stderr, "[FAIL] sse emit chunk %d (%s): %v\n", i, ch.Type, err)
			return
		}
		// Real-agent pacing. Keep tiny so the spike stays well under the
		// 10s budget even on slow CI runners.
		time.Sleep(20 * time.Millisecond)
	}
}

// parseSSEFrames scans raw SSE bytes and returns the UIMessageChunk
// value from each `data: <json>` line. Non-data lines (event:, id:,
// retry:, comments) are ignored per the SSE spec — clients reassemble
// events based on data: payload + the discriminated `type` field inside.
//
// This mirrors what the AI SDK v5 DefaultChatTransport parser does
// client-side: it reads data: lines and JSON-parses each into a
// UIMessageChunk. A successful round-trip here (Hertz marshal → wire
// → parse → struct) proves the wire contract holds end-to-end.
//
// Malformed JSON on a data: line is a hard error: it would indicate
// either a Hertz serialization bug or an in-spec but unexpected chunk
// shape, both of which would break the real client parser too.
func parseSSEFrames(body []byte) ([]wire.UIMessageChunk, error) {
	var out []wire.UIMessageChunk
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "" {
			continue // SSE keep-alive or terminator
		}
		var ch wire.UIMessageChunk
		if err := json.Unmarshal([]byte(payload), &ch); err != nil {
			return nil, fmt.Errorf("unmarshal data line %q: %w", payload, err)
		}
		out = append(out, ch)
	}
	return out, nil
}

// waitForListener polls the TCP addr with a dial until it succeeds or
// the context is cancelled. Used instead of a fixed time.Sleep so the
// spike pays only the actual startup latency (Hertz is typically ready
// within a few milliseconds of Run() being called).
func waitForListener(ctx context.Context, addr string) error {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	return fmt.Errorf("listener %s not accepting after 3s", addr)
}
