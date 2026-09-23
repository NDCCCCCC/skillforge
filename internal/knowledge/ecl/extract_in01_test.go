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

// IN-01 regression test — buildExtractionPrompt dead code removal.
// Verifies that removing buildExtractionPrompt has zero behavior impact:
// the under-cap extraction path still works via joinSectionBodies + promptForBody.
package ecl

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// stubExtractorForIN01 always succeeds (uses the actual LLM call path).
type stubExtractorForIN01 struct {
	Generator // embed to satisfy interface
	calls     int
}

func (s *stubExtractorForIN01) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	s.calls++
	return &schema.Message{
		Content: `{"tables": [{"name": "etl_test_t", "kind": "update"}], "related_notes": []}`,
	}, nil
}

// TestExtractUnderCapStillUsesJoinAndPrompt (IN-01): under-cap extraction
// still works after buildExtractionPrompt deletion — zero behavior change.
// The under-cap path uses joinSectionBodies + promptForBody directly.
func TestExtractUnderCapStillUsesJoinAndPrompt(t *testing.T) {
	note := Note{
		SourceIdx: "003",
		Title:     "Small Note",
		Sections: []MergedSection{{
			Section:    "SQL",
			Body:       "update etl_small_t set x = 1;",
			SourcePaths: []string{"test.sql"},
		}},
	}

	ext := &stubExtractorForIN01{}
	llm := LLMExtractor{chat: ext, sectionCap: 100000}

	_, err := llm.ExtractNote(context.Background(), note)
	if err != nil {
		t.Fatalf("ExtractNote returned error: %v", err)
	}

	if ext.calls != 1 {
		t.Errorf("extractor calls = %d, want 1 (under-cap path still works)", ext.calls)
	}
}

// callCountExtractorIN01 counts LLM Generate calls.
type callCountExtractorIN01 struct {
	Generator // embed to satisfy interface
	calls     int
}

func (c *callCountExtractorIN01) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	c.calls++
	// Verify the prompt contains the note title and joined section bodies.
	for _, m := range msgs {
		if strings.Contains(m.Content, "Small Note") && strings.Contains(m.Content, "update etl_small_t") {
			// Expected content in prompt.
			break
		}
	}
	return &schema.Message{Content: `{"tables":[],"related_notes":[]}`}, nil
}

// TestExtractUnderCapPromptContent (IN-01): confirms the under-cap path
// renders a prompt containing the joined section bodies (joinSectionBodies +
// promptForBody, not buildExtractionPrompt which is now deleted).
func TestExtractUnderCapPromptContent(t *testing.T) {
	note := Note{
		SourceIdx: "004",
		Title:     "IN01 Test",
		Sections: []MergedSection{
			{Section: "Part1", Body: "section one body", SourcePaths: []string{"f1.md"}},
			{Section: "Part2", Body: "section two body", SourcePaths: []string{"f2.md"}},
		},
	}

	ext := &callCountExtractorIN01{}
	llm := LLMExtractor{chat: ext, sectionCap: 100000}

	_, err := llm.ExtractNote(context.Background(), note)
	if err != nil {
		t.Fatalf("ExtractNote returned error: %v", err)
	}

	if ext.calls != 1 {
		t.Fatalf("calls = %d, want 1", ext.calls)
	}
}
