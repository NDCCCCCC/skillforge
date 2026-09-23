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

// IN-02 regression test — section-cap sub-split.
// Verifies that a section exceeding MinSplitRunes is split into multiple
// LLM calls rather than one oversized prompt.
package ecl

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// callCountExtractor counts how many times Generate was invoked (mimics
// the Generator interface the LLMExtractor requires).
type callCountExtractor struct {
	Generator // embed to satisfy the interface; override Generate only
	calls     int
}

func (c *callCountExtractor) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	c.calls++
	return &schema.Message{
		Content: `{"tables": [{"name": "etl_test_t", "kind": "update"}], "related_notes": []}`,
	}, nil
}

// TestExtractOversizeSectionSubSplits (IN-02): a section whose body
// exceeds MinSplitRunes triggers sub-splitting and multiple LLM calls,
// rather than one oversized prompt.
func TestExtractOversizeSectionSubSplits(t *testing.T) {
	// Build a section body larger than MinSplitRunes (3500 runes).
	// Research found a 93,993-byte section (~25k runes) in the corpus.
	largeBody := strings.Repeat("a", 4000) // 4000 chars >> MinSplitRunes

	note := Note{
		SourceIdx: "001",
		Title:     "Test Note",
		Sections: []MergedSection{{
			Section:     "SQL",
			Body:        largeBody,
			SourcePaths: []string{"test.sql"},
		}},
	}

	extractor := &callCountExtractor{}
	llm := LLMExtractor{chat: extractor, sectionCap: 1000} // section cap is above whole-note but below section

	_, err := llm.ExtractNote(context.Background(), note)
	if err != nil {
		t.Fatalf("ExtractNote returned error: %v", err)
	}

	// The section is 4000 chars, MinSplitRunes is 3500 — it must be sub-split.
	// Expect at least 2 calls (floor(4000/3500) + 1 for remainder).
	if extractor.calls < 2 {
		t.Errorf("extractor calls = %d, want >= 2 (section at %d chars must be sub-split across multiple LLM calls)",
			extractor.calls, len(largeBody))
	}
}

// TestExtractUnderCapSectionGoesDirectly (IN-02 sanity): a section below
// MinSplitRunes produces exactly one LLM call.
func TestExtractUnderCapSectionGoesDirectly(t *testing.T) {
	smallBody := "update etl_alpha_t set x = 1;"

	note := Note{
		SourceIdx: "002",
		Title:     "Small Note",
		Sections: []MergedSection{{
			Section:     "SQL",
			Body:        smallBody,
			SourcePaths: []string{"test.sql"},
		}},
	}

	extractor := &callCountExtractor{}
	llm := LLMExtractor{chat: extractor, sectionCap: 100000}

	_, err := llm.ExtractNote(context.Background(), note)
	if err != nil {
		t.Fatalf("ExtractNote returned error: %v", err)
	}

	if extractor.calls != 1 {
		t.Errorf("extractor calls = %d, want 1 (small section goes direct)", extractor.calls)
	}
}
