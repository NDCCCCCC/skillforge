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

// Package spike — Hertz SSE writer wrapper.
//
// This file enforces the two wire-contract invariants from
// 01-RESEARCH.md §Pitfall 2 + §Pitfall 6:
//
//  1. Use the ACTUAL Hertz v0.10.6 SSE API: sse.NewWriter(c) +
//     w.WriteEvent(id, type, data). The Gin/Echo-style "publish an
//     sse.Event" / "construct a stream" helpers shown in PROJECT.md
//     §2.2 do NOT exist in Hertz (verified by reading writer.go — the
//     Writer type exposes only NewWriter + WriteEvent + Write + Close;
//     there is no event-publisher method and no stream-constructor
//     function). PROJECT.md §2.2 misremembered the API.
//  2. Validate Type against the kebab-case whitelist BEFORE writing so
//     a snake_case or unknown Type cannot reach the AI SDK v5 parser
//     (where it would be silently dropped, producing a confusing
//     "frame missing" client-side bug).
//
// Verified API anchors (hertz@v0.10.6/pkg/protocol/sse/writer.go):
//   - line 41: func NewWriter(c *app.RequestContext) *Writer
//   - line 46:     c.Response.Header.SetContentType("text/event-stream; charset=utf-8")
//   - line 64: func (w *Writer) WriteEvent(id, eventType string, data []byte) error
//   - line 157:    return w.w.Flush()  // each WriteEvent flushes immediately
//   - line 160: func (w *Writer) Close() error  // finalizes chunked encoding
package spike

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
)

// EmitChunk writes one UIMessageChunk as an SSE frame to the response.
//
// EmitChunk validates the chunk type (T-1-11 mitigation: reject unknown
// or snake_case types before any wire write), marshals the chunk to
// JSON, and emits the frame via sse.NewWriter(c).WriteEvent. The empty
// id + "message" eventType pair produces the canonical SSE frame:
//
//	event: message
//	data: {"type":"text-delta","id":"msg-1","delta":"hi"}
//
// (followed by a blank line separator; see writer.go:122-151 for the
// exact byte layout).
//
// EmitChunk does NOT call w.Close() — the Hertz framework finalizes the
// chunked response automatically when the handler returns (verified in
// hertz@v0.10.6 example_test.go:48-50 comment: "hertz will do it after
// handler returns"). Each WriteEvent flushes immediately (writer.go:157),
// so the client sees each frame without waiting for Close. Calling
// Close prematurely would write the chunked terminator (0\r\n\r\n)
// mid-stream and break subsequent frames.
//
// sse.NewWriter is idempotent: repeated calls within one handler return
// fresh *Writer structs wrapping the same hijacked underlying writer
// (writer.go:47-52 — GetHijackWriter reuses the existing hijack). Safe
// to call EmitChunk in a loop.
func EmitChunk(c *app.RequestContext, chunk UIMessageChunk) error {
	if !chunk.IsValidType() {
		return fmt.Errorf("unknown chunk type %q: must be kebab-case member of spike.ValidTypes", chunk.Type)
	}
	payload, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("marshal UIMessageChunk: %w", err)
	}
	w := sse.NewWriter(c) // sets Content-Type + Cache-Control, hijacks response (idempotent)
	if err := w.WriteEvent("", "message", payload); err != nil {
		return fmt.Errorf("write sse event: %w", err)
	}
	return nil
}

// StreamChunks writes a channel of UIMessageChunks as a contiguous SSE
// stream, calling w.Close() to finalize the chunked encoding when the
// channel closes. Use this when the handler's sole job is to drain a
// channel; use EmitChunk directly when interleaving with other handler
// logic or when the framework should finalize on handler return.
//
// Type validation happens per-chunk (same T-1-11 gate as EmitChunk).
// On the first invalid chunk or write error, StreamChunks returns
// immediately without draining the rest of the channel — the caller is
// responsible for closing the channel to unblock the producer.
func StreamChunks(c *app.RequestContext, chunks <-chan UIMessageChunk) error {
	w := sse.NewWriter(c)
	for chunk := range chunks {
		if !chunk.IsValidType() {
			return fmt.Errorf("unknown chunk type %q: must be kebab-case member of spike.ValidTypes", chunk.Type)
		}
		payload, err := json.Marshal(chunk)
		if err != nil {
			return fmt.Errorf("marshal UIMessageChunk: %w", err)
		}
		if err := w.WriteEvent("", "message", payload); err != nil {
			return fmt.Errorf("write sse event: %w", err)
		}
	}
	return w.Close()
}

// ContentTypeIsSSE reports whether the response's Content-Type header
// is `text/event-stream` (set automatically by sse.NewWriter at
// writer.go:46). Used in tests to verify Hertz's auto-header behavior
// without parsing the full response.
func ContentTypeIsSSE(c *app.RequestContext) bool {
	return strings.Contains(c.Response.Header.Get("Content-Type"), "text/event-stream")
}
