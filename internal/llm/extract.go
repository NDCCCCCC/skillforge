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

// Package llm holds small, dependency-free utilities for post-processing
// raw LLM output that is consumed across multiple SkillForge packages
// (agent, server, eval). Centralizing these here prevents each consumer
// from reimplementing fragile fence-stripping logic — the exact defect
// that caused OOS-1 (glm-verdict-fence-parse): the agent and eval packages
// each carried a private stripJSONDelim with subtly different blind spots,
// and the server pipeline carried none at all.
package llm

import "strings"

// ExtractJSON extracts a JSON OBJECT from a raw model response.
//
// LLMs (notably GLM, Claude-family via OpenAI-compatible shims, and many
// others) frequently wrap JSON output in markdown code fences even when
// response_format=json_object is set. The wrapping varies across models
// and even across calls of the same model:
//
//   - Leading ```json or ``` fence, with or without a trailing newline.
//   - Trailing ``` fence, with or without a preceding newline.
//   - Leading/trailing prose ("Here is the verdict:\n```json ... ```").
//   - Mixed Unix LF / Windows CRLF line endings.
//   - Extra backticks (4+) or doubled fences from misbehaving shims.
//   - No fence at all (passthrough must remain intact).
//
// ExtractJSON handles all of these by:
//
//  1. Trimming surrounding whitespace.
//  2. Stripping a leading code fence (``` + optional language tag up to a
//     newline) if present.
//  3. Stripping a trailing code fence (```) if present.
//  4. Extracting the substring from the first '{' to the last '}' — this
//     uniformly removes any residual fences or prose and isolates the JSON
//     object body.
//
// ExtractJSON returns the extracted JSON object substring (without
// surrounding fences or prose), or "" if no '{' ... '}' pair is found.
// A "" return surfaces upstream as a clean "unexpected end of JSON input"
// error from json.Unmarshal rather than a confusing fence-related parse
// error such as "invalid character '`' after top-level value".
//
// ExtractJSON does NOT validate JSON; it only isolates the putative JSON
// object. The caller's json.Unmarshal is authoritative.
//
// Scope note: the function targets JSON OBJECTS (the only top-level shape
// SkillForge's verdict / critique / eval prompts request). A bare JSON
// array (`[...]`) yields "" because there is no '{' ... '}' pair; that is
// intentional — an array is the wrong shape for these call sites and the
// upstream Unmarshal error correctly rejects it.
//
// Heuristic safety: for a single top-level JSON object the first-'{'-to-
// last-'}' scan is exact. It correctly handles nested objects and '}'
// characters inside JSON string values (the FIRST '{' is the object's
// opening brace; the LAST '}' is its closing brace). The only failure mode
// is a '}' appearing in trailing prose AFTER the object — rare for the
// structured-output prompts SkillForge uses, and mitigated by the
// fence-stripping passes that usually remove prose before the brace scan.
func ExtractJSON(s string) string {
	s = strings.TrimSpace(s)

	// Step 1: strip a leading code fence + optional language tag.
	// Handles ```json\n, ```\n, ```json (no newline), and bare ```.
	// When the fence has no trailing newline (e.g. ```json{...}) we do
	// NOT consume past the fence — the brace scan below isolates the
	// object regardless. This is the fix for the OOS-1 failure mode
	// where the previous byte-index loop ate the entire input.
	if strings.HasPrefix(s, "```") {
		s = s[len("```"):]
		if nl := strings.IndexByte(s, '\n'); nl >= 0 {
			// Drop the language tag (json, JSON, javascript, etc.) and
			// the newline that terminates it.
			s = s[nl+1:]
		}
	}

	// Step 2: strip a trailing code fence. Trim whitespace first so a
	// trailing newline/space after the closing fence does not defeat
	// HasSuffix.
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-len("```")]
	}

	// Step 3: isolate the JSON object body — first '{' to last '}'.
	// This is the robust catch-all that handles any residual fence or
	// prose the targeted strips above did not remove (notably prose-
	// wrapped output where the leading fence was not at the absolute
	// start of the trimmed string).
	first := strings.IndexByte(s, '{')
	last := strings.LastIndexByte(s, '}')
	if first < 0 || last < 0 || last < first {
		return ""
	}
	return s[first : last+1]
}
