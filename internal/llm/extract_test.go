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

package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// TestExtractJSON is the regression-lock for OOS-1 (glm-verdict-fence-parse).
// Every case is a shape ExtractJSON must handle; each is also fed through
// json.Unmarshal to prove the result parses cleanly (or, for the malformed
// cases, errors cleanly without panicking).
//
// The cases mirror the verification constraint in the debug session:
// the exact GLM shape, no language tag, no adjacent newlines, trailing
// whitespace/CRLF, prose around the block, no fences (passthrough), and
// malformed/empty input.
func TestExtractJSON(t *testing.T) {
	// canonicalObject is the JSON object every "should parse" case must
	// recover, regardless of how the model wrapped it.
	const canonicalObject = `{"status":"合规","confidence":0.95,"review_required":false}`

	type tc struct {
		name       string
		in         string
		want       string // expected ExtractJSON return; "" means caller will see parse error
		wantParses bool   // true iff json.Unmarshal(out) must succeed
	}
	cases := []tc{
		// --- Passthrough (no fences) — regression guard ---
		{
			name:       "no_fences_passthrough",
			in:         canonicalObject,
			want:       canonicalObject,
			wantParses: true,
		},
		{
			name:       "no_fences_with_surrounding_whitespace",
			in:         "  \t\n" + canonicalObject + "\n\t  ",
			want:       canonicalObject,
			wantParses: true,
		},

		// --- The exact GLM shape from the OOS-1 production error ---
		// ExtractJSON preserves the JSON body's internal formatting
		// verbatim (it strips fences/prose only); the want value mirrors
		// the input body exactly.
		{
			name:       "glm_basic_json_lang_tag",
			in:         "```json\n{\n  \"status\": \"合规\",\n  \"confidence\": 0.95,\n  \"review_required\": false\n}\n```",
			want:       "{\n  \"status\": \"合规\",\n  \"confidence\": 0.95,\n  \"review_required\": false\n}",
			wantParses: true,
		},

		// --- Fence with no language tag ---
		{
			name:       "fence_no_language_tag",
			in:         "```\n" + canonicalObject + "\n```",
			want:       canonicalObject,
			wantParses: true,
		},

		// --- No newlines adjacent to fences ---
		{
			name:       "no_newline_after_lang_tag",
			in:         "```json" + canonicalObject + "```",
			want:       canonicalObject,
			wantParses: true,
		},
		{
			name:       "no_newline_no_lang_tag",
			in:         "```" + canonicalObject + "```",
			want:       canonicalObject,
			wantParses: true,
		},

		// --- Trailing whitespace / CRLF after the closing fence ---
		{
			name:       "trailing_newline_after_closing_fence",
			in:         "```json\n" + canonicalObject + "\n```\n",
			want:       canonicalObject,
			wantParses: true,
		},
		{
			name:       "trailing_spaces_after_closing_fence",
			in:         "```json\n" + canonicalObject + "\n```   ",
			want:       canonicalObject,
			wantParses: true,
		},
		{
			name:       "crlf_line_endings_throughout",
			in:         "```json\r\n" + canonicalObject + "\r\n```",
			want:       canonicalObject,
			wantParses: true,
		},
		{
			name:       "crlf_after_closing_fence",
			in:         "```json\n" + canonicalObject + "\n```\r\n",
			want:       canonicalObject,
			wantParses: true,
		},

		// --- Leading/trailing prose around the fenced block ---
		{
			name:       "prose_prefix_and_suffix",
			in:         "Here is the verdict:\n```json\n" + canonicalObject + "\n```\nThanks!",
			want:       canonicalObject,
			wantParses: true,
		},
		{
			name:       "prose_prefix_only",
			in:         "Sure, here you go:\n```json\n" + canonicalObject + "\n```",
			want:       canonicalObject,
			wantParses: true,
		},
		{
			name:       "prose_suffix_only",
			in:         "```json\n" + canonicalObject + "\n```\nLet me know if you need more.",
			want:       canonicalObject,
			wantParses: true,
		},
		{
			name:       "prose_no_fences",
			in:         "The verdict is: " + canonicalObject + " — hope that helps.",
			want:       canonicalObject,
			wantParses: true,
		},

		// --- Uppercase / mixed-case language tags ---
		{
			name:       "uppercase_JSON_lang_tag",
			in:         "```JSON\n" + canonicalObject + "\n```",
			want:       canonicalObject,
			wantParses: true,
		},
		{
			name:       "javascript_lang_tag",
			in:         "```javascript\n" + canonicalObject + "\n```",
			want:       canonicalObject,
			wantParses: true,
		},

		// --- Nested objects + brace-in-string (heuristic safety) ---
		{
			name:       "nested_objects",
			in:         "```json\n{\"a\":{\"b\":{\"c\":1}}}\n```",
			want:       `{"a":{"b":{"c":1}}}`,
			wantParses: true,
		},
		{
			name:       "brace_inside_string_value",
			in:         "```json\n{\"span\":\"this } is literal\"}\n```",
			want:       `{"span":"this } is literal"}`,
			wantParses: true,
		},

		// --- Malformed / empty input (must error cleanly, NOT panic) ---
		{name: "empty_string", in: "", want: "", wantParses: false},
		{name: "only_whitespace", in: "   \n\t\r\n  ", want: "", wantParses: false},
		{name: "only_fences_no_json", in: "```\n```", want: "", wantParses: false},
		{name: "prose_no_json_at_all", in: "I cannot help with that.", want: "", wantParses: false},
		{name: "opening_brace_only", in: "```json\n{", want: "", wantParses: false},
		{name: "closing_brace_only", in: "```json\n}", want: "", wantParses: false},
		{
			name:       "reversed_braces",
			in:         "}{",
			want:       "",
			wantParses: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := ExtractJSON(c.in)
			if got != c.want {
				t.Errorf("ExtractJSON(%q) = %q; want %q", c.in, got, c.want)
				return
			}
			// Prove the result parses (or errors cleanly) under json.Unmarshal.
			var v map[string]any
			err := json.Unmarshal([]byte(got), &v)
			if c.wantParses && err != nil {
				t.Errorf("ExtractJSON(%q): result %q failed json.Unmarshal: %v", c.in, got, err)
			}
			if !c.wantParses && err == nil && got != "" {
				// Non-empty result that unexpectedly parsed is fine only if
				// it matched want; otherwise flag. Empty result + no error
				// is impossible (Unmarshal("") errors), so no false positive.
				t.Errorf("ExtractJSON(%q): malformed-case result %q unexpectedly parsed", c.in, got)
			}
		})
	}
}

// TestExtractJSONRealisticVerdictShape proves the full OOS-1 scenario
// end-to-end at the helper level: the verdict JSON GLM actually emits,
// wrapped in fences, must parse into the production verdict.Verdict shape.
// This is the closest unit-level analog to the production failure without
// a live LLM call (which needs a provider API key absent in this env).
func TestExtractJSONRealisticVerdictShape(t *testing.T) {
	// Realistic GLM-wrapped verdict output (matches the OOS-1 raw bytes
	// pattern: ```json\n{\n  "field": ...\n}\n```).
	raw := "```json\n" + `{
  "status": "合规",
  "confidence": 0.92,
  "citations": [
    {"chunk_id": "7", "span": "员工连续工作1年以上，享受带薪年休假", "tier": "权威"}
  ],
  "review_required": false
}` + "\n```"

	extracted := ExtractJSON(raw)

	// Use an anonymous struct mirroring verdict.Verdict's JSON tags so this
	// test stays in the llm package (no internal/verdict import cycle).
	var v struct {
		Status         string  `json:"status"`
		Confidence     float64 `json:"confidence"`
		ReviewRequired bool    `json:"review_required"`
	}
	if err := json.Unmarshal([]byte(extracted), &v); err != nil {
		t.Fatalf("realistic GLM verdict failed to parse after ExtractJSON: %v (extracted=%q)", err, extracted)
	}
	if v.Status != "合规" {
		t.Errorf("status = %q, want %q", v.Status, "合规")
	}
	if v.Confidence != 0.92 {
		t.Errorf("confidence = %v, want 0.92", v.Confidence)
	}
	if v.ReviewRequired != false {
		t.Errorf("review_required = %v, want false", v.ReviewRequired)
	}
}

// ---------------------------------------------------------------------------
// ExtractVerdictJSONWithStructuredOutput (Phase 12 / 12-04, RF-5)
// ---------------------------------------------------------------------------

// stubExtractModel is a hermetic model.ToolCallingChatModel that returns a
// canned content string (or error) and RECORDS the options each Generate
// call received, so tests can prove the temperature-0 +
// response_format=json_object signal is actually propagated.
type stubExtractModel struct {
	content  string
	genErr   error
	gotOpts  []model.Option
	gotMsgs  []*schema.Message
	genCalls int
}

func (m *stubExtractModel) Generate(_ context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.genCalls++
	m.gotMsgs = msgs
	m.gotOpts = opts
	if m.genErr != nil {
		return nil, m.genErr
	}
	return schema.AssistantMessage(m.content, nil), nil
}

func (m *stubExtractModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, msgs, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *stubExtractModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// TestExtractVerdictJSONWithStructuredOutput — the happy path: a stub model
// WITH response_format=json_object support (i.e. one that returns a JSON
// object, even fence-wrapped) yields the parsed verdict map. Also proves
// the structured-output signal is propagated: temperature 0 (common option)
// + the response_format impl-specific option both reach the model.
func TestExtractVerdictJSONWithStructuredOutput(t *testing.T) {
	m := &stubExtractModel{
		content: "```json\n{\"status\":\"ok\",\"confidence\":0.91,\"citations\":[{\"chunk_id\":\"7\"}]}\n```",
	}
	got, err := ExtractVerdictJSONWithStructuredOutput(
		context.Background(),
		"经核查，差旅住宿费超过 500 元不符合规定（prose final message, no JSON）",
		m,
		`{"status":"string","confidence":"number"}`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["status"] != "ok" {
		t.Errorf("status = %v, want ok", got["status"])
	}
	if got["confidence"] != 0.91 {
		t.Errorf("confidence = %v, want 0.91", got["confidence"])
	}

	if m.genCalls != 1 {
		t.Fatalf("Generate calls = %d, want exactly 1 final-step call", m.genCalls)
	}
	// Temperature 0 propagated (deterministic extraction).
	common := model.GetCommonOptions(nil, m.gotOpts...)
	if common.Temperature == nil || *common.Temperature != 0.0 {
		t.Errorf("temperature option not propagated: %+v", common.Temperature)
	}
	// The response_format=json_object signal propagated on the
	// impl-specific channel.
	impl := model.GetImplSpecificOptions(&map[string]any{}, m.gotOpts...)
	rf, ok := (*impl)["response_format"].(map[string]any)
	if !ok || rf["type"] != "json_object" {
		t.Errorf("response_format=json_object signal not propagated: %+v", *impl)
	}
	// The Phase 2 system prompt rendered VERBATIM (T-12-04-7) + the
	// schema interpolated.
	if len(m.gotMsgs) != 2 || m.gotMsgs[0].Role != schema.System || m.gotMsgs[1].Role != schema.User {
		t.Fatalf("messages = %+v, want [system, user]", m.gotMsgs)
	}
	if !strings.Contains(m.gotMsgs[0].Content, "You are a strict compliance auditor.") ||
		!strings.Contains(m.gotMsgs[0].Content, "NO other text. ONLY the JSON object.") {
		t.Errorf("system prompt is not the verbatim Phase 2 template: %q", m.gotMsgs[0].Content)
	}
	if !strings.Contains(m.gotMsgs[0].Content, `"status":"string"`) {
		t.Errorf("verdict schema not interpolated into the system prompt: %q", m.gotMsgs[0].Content)
	}
	if !strings.Contains(m.gotMsgs[1].Content, "DeepAgent final message:") {
		t.Errorf("user prompt missing the final-message framing: %q", m.gotMsgs[1].Content)
	}
}

// TestExtractVerdictJSONWithStructuredOutputEmpty — empty finalMessage is
// rejected BEFORE any model call.
func TestExtractVerdictJSONWithStructuredOutputEmpty(t *testing.T) {
	m := &stubExtractModel{content: `{"status":"ok"}`}
	if _, err := ExtractVerdictJSONWithStructuredOutput(context.Background(), "", m, ""); err == nil {
		t.Fatal("empty finalMessage must error")
	} else if !strings.Contains(err.Error(), "finalMessage is empty") {
		t.Errorf("error = %v, want finalMessage-is-empty", err)
	}
	if m.genCalls != 0 {
		t.Errorf("Generate calls = %d, want 0 (gate fires before the call)", m.genCalls)
	}
}

// TestExtractVerdictJSONWithStructuredOutputNilModel — nil chatModel is
// rejected BEFORE any call (nil-pointer guard).
func TestExtractVerdictJSONWithStructuredOutputNilModel(t *testing.T) {
	if _, err := ExtractVerdictJSONWithStructuredOutput(context.Background(), "some final message", nil, ""); err == nil {
		t.Fatal("nil chatModel must error")
	} else if !strings.Contains(err.Error(), "chatModel is required") {
		t.Errorf("error = %v, want chatModel-is-required", err)
	}
}

// TestExtractVerdictJSONWithStructuredOutputInvalidJSON — a stub WITHOUT
// structured-output support that returns prose ("not json") yields an
// empty result + error (the caller falls back / records the miss).
func TestExtractVerdictJSONWithStructuredOutputInvalidJSON(t *testing.T) {
	m := &stubExtractModel{content: "I cannot produce JSON for that message."}
	got, err := ExtractVerdictJSONWithStructuredOutput(context.Background(), "final prose message", m, "")
	if err == nil {
		t.Fatal("unparseable response must error")
	}
	if !strings.Contains(err.Error(), "parse failed") {
		t.Errorf("error = %v, want parse-failed", err)
	}
	if got != nil {
		t.Errorf("result = %v, want nil on parse failure", got)
	}
}

// TestExtractVerdictJSONWithStructuredOutputNonObject — a stub that returns
// a JSON ARRAY (the wrong top-level shape) yields nil + error: ExtractJSON
// isolates objects only, so the array never reaches Unmarshal as an object.
func TestExtractVerdictJSONWithStructuredOutputNonObject(t *testing.T) {
	m := &stubExtractModel{content: `["not","an","object"]`}
	got, err := ExtractVerdictJSONWithStructuredOutput(context.Background(), "final prose message", m, "")
	if err == nil {
		t.Fatal("non-object response must error")
	}
	if !strings.Contains(err.Error(), "not a JSON object") && !strings.Contains(err.Error(), "no JSON object") {
		t.Errorf("error = %v, want not-a-JSON-object", err)
	}
	if got != nil {
		t.Errorf("result = %v, want nil for non-object", got)
	}
}
