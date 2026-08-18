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

// Package history contains the Phase 9 history projection contract.
// D9-B1 keeps only user/assistant text; tool/system metadata cannot reach the LLM.
// D9-B2 replaces references in user text and preserves assistant citations.
// D9-B3 keeps the latest three pairs and removes an orphan assistant after filtering.
// D9-B4 records the contract as executable fixtures for Phase 10; chat wiring is deferred.
// D9-B2 intentionally leaves [REF-N] unpadded: mixed Chinese text and Normalize's
// whitespace collapse make padding semantically unnecessary and it would be discarded.
package history

import (
	"regexp"
	"strconv"

	"github.com/cloudwego/eino/schema"
	"github.com/skillforge/skillforge/internal/store"
)

var chunkRefPattern = regexp.MustCompile(`(?i)\[(?:CHUNK|CH-|id:)\s*([A-Za-z0-9_-]+)\s*\]`)

// Project returns a text-only, bounded Eino history for Phase 10 callers.
func Project(prev []store.SessionMessage) []schema.Message {
	filtered := make([]store.SessionMessage, 0, len(prev))
	for _, m := range prev {
		if m.Role == "user" || m.Role == "assistant" {
			filtered = append(filtered, m)
		}
	}
	for i, m := range filtered {
		if m.Role == "user" {
			filtered[i].Content = scrubChunkRefs(m.Content)
		}
	}
	if len(filtered) > 6 {
		filtered = filtered[len(filtered)-6:]
	}
	if len(filtered) < 2 {
		return nil
	}
	if filtered[0].Role == "assistant" {
		filtered = filtered[1:]
	}
	if len(filtered) < 2 {
		return nil
	}
	out := make([]schema.Message, len(filtered))
	for i, m := range filtered {
		role := schema.User
		if m.Role == "assistant" {
			role = schema.Assistant
		}
		out[i] = schema.Message{Role: role, Content: m.Content}
	}
	return out
}

func scrubChunkRefs(content string) string {
	ref := 0
	return chunkRefPattern.ReplaceAllStringFunc(content, func(string) string {
		ref++
		return "[REF-" + strconv.Itoa(ref) + "]"
	})
}
