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
	"encoding/json"
	"testing"
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
		name      string
		in        string
		want      string // expected ExtractJSON return; "" means caller will see parse error
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
		ReviewRequired bool     `json:"review_required"`
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
