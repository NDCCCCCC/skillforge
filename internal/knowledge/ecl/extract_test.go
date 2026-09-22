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

package ecl

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// fakeGenerator records every Generate call so tests can assert the exact
// LLM call posture (messages + options) and simulate per-section response
// sequences for the oversized-note union path.
type fakeGenerator struct {
	contents    []string // per-call response content; the last entry repeats when exhausted
	nilResponse bool     // when set, Generate returns (nil, nil)
	err         error    // when set, Generate returns (nil, err)

	calls    int
	inputs   [][]*schema.Message
	opts     [][]model.Option
}

func (f *fakeGenerator) Generate(_ context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	f.calls++
	f.inputs = append(f.inputs, input)
	f.opts = append(f.opts, opts)
	if f.err != nil {
		return nil, f.err
	}
	if f.nilResponse {
		return nil, nil
	}
	idx := f.calls - 1
	if idx >= len(f.contents) {
		idx = len(f.contents) - 1
	}
	return schema.AssistantMessage(f.contents[idx], nil), nil
}

// userContent returns the user-message content of the i-th call (0-based).
func (f *fakeGenerator) userContent(call int) string {
	if call >= len(f.inputs) || len(f.inputs[call]) < 2 {
		return ""
	}
	return f.inputs[call][1].Content
}

// assertExtractionPosture pins the D-Ext-01 call shape on the most recent
// call: SystemMessage("You output only valid JSON.") + temperature 0 +
// response_format=json_object (the exported llm.JSONObjectResponseFormat).
func assertExtractionPosture(t *testing.T, f *fakeGenerator) {
	t.Helper()
	if f.calls == 0 {
		t.Fatal("fake generator received no calls")
	}
	input := f.inputs[f.calls-1]
	if len(input) != 2 {
		t.Fatalf("Generate input = %d messages, want 2 (system + user)", len(input))
	}
	if input[0].Content != "You output only valid JSON." {
		t.Errorf("system message = %q, want %q", input[0].Content, "You output only valid JSON.")
	}

	common := model.GetCommonOptions(&model.Options{}, f.opts[f.calls-1]...)
	if common.Temperature == nil {
		t.Error("Generate options missing temperature (want model.WithTemperature(0.0))")
	} else if *common.Temperature != 0.0 {
		t.Errorf("Generate temperature = %v, want 0", *common.Temperature)
	}

	impl := model.GetImplSpecificOptions[map[string]any](nil, f.opts[f.calls-1]...)
	if impl == nil || (*impl)["response_format"] == nil {
		t.Fatal("Generate options missing response_format (want llm.JSONObjectResponseFormat())")
	}
	rf, ok := (*impl)["response_format"].(map[string]any)
	if !ok || rf["type"] != "json_object" {
		t.Errorf("response_format = %#v, want {type: json_object}", (*impl)["response_format"])
	}
}

const fenceWrappedExtraction = "Here is the extraction:\n```json\n" +
	`{"tables":[{"name":"etl_payment_due_t","kind":"update"}],"related_notes":[]}` +
	"\n```\n"

// TestLLMExtractNoteFenceWrappedJSON covers the happy path: a GLM-style
// fence-wrapped response must parse into exactly one table of kind update,
// and the call must carry the pinned posture (temp 0, json_object,
// system-message contract).
func TestLLMExtractNoteFenceWrappedJSON(t *testing.T) {
	fake := &fakeGenerator{contents: []string{fenceWrappedExtraction}}
	ex := NewLLMExtractor(fake)

	got, err := ex.ExtractNote(context.Background(), noteWith("001", "UPDATE etl_payment_due_t SET a = 1;"))
	if err != nil {
		t.Fatalf("ExtractNote: %v", err)
	}
	if len(got.Tables) != 1 {
		t.Fatalf("tables = %#v, want exactly 1", got.Tables)
	}
	if got.Tables[0].Name != "etl_payment_due_t" || got.Tables[0].Kind != "update" {
		t.Errorf("table = %#v, want {etl_payment_due_t update}", got.Tables[0])
	}
	if len(got.RelatedNotes) != 0 {
		t.Errorf("related_notes = %#v, want empty", got.RelatedNotes)
	}
	assertExtractionPosture(t, fake)
	if fake.calls != 1 {
		t.Errorf("calls = %d, want 1 (single whole-note call under the cap)", fake.calls)
	}
	if !strings.Contains(fake.userContent(0), "UPDATE etl_payment_due_t SET a = 1;") {
		t.Errorf("prompt does not carry the full note body: %q", fake.userContent(0))
	}
	if !strings.Contains(fake.userContent(0), `"related_notes"`) {
		t.Errorf("prompt does not carry the D-Ext-01 schema instruction: %q", fake.userContent(0))
	}
}

// TestLLMExtractNoteInvalidJSON proves a malformed model response surfaces
// as an error (never a panic, never a partial Extraction).
func TestLLMExtractNoteInvalidJSON(t *testing.T) {
	fake := &fakeGenerator{contents: []string{`{"tables": [{"name":"etl_x_t","kind":`}}
	_, err := NewLLMExtractor(fake).ExtractNote(context.Background(), noteWith("001", "body"))
	if err == nil {
		t.Fatal("ExtractNote with malformed JSON should fail")
	}
	if !strings.Contains(err.Error(), "unmarshal") && !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should mention unmarshal/parse, got: %v", err)
	}
}

// TestLLMExtractNoteNoJSONObject covers the prose-only response shape: no
// braces at all means ExtractJSON cannot isolate an object — explicit error.
func TestLLMExtractNoteNoJSONObject(t *testing.T) {
	fake := &fakeGenerator{contents: []string{"I cannot answer that question."}}
	_, err := NewLLMExtractor(fake).ExtractNote(context.Background(), noteWith("001", "body"))
	if err == nil {
		t.Fatal("ExtractNote with no JSON object should fail")
	}
	if !strings.Contains(err.Error(), "no JSON object") {
		t.Errorf("error should mention the missing JSON object, got: %v", err)
	}
}

// TestLLMExtractNoteBadKind proves ValidateExtraction gates the model
// output: an out-of-enum kind must be rejected before any ABox write.
func TestLLMExtractNoteBadKind(t *testing.T) {
	fake := &fakeGenerator{contents: []string{
		`{"tables":[{"name":"etl_x_t","kind":"drop"}],"related_notes":[]}`,
	}}
	_, err := NewLLMExtractor(fake).ExtractNote(context.Background(), noteWith("001", "body"))
	if err == nil {
		t.Fatal("ExtractNote with kind=drop should fail ValidateExtraction")
	}
	if !strings.Contains(err.Error(), "invalid extraction kind") {
		t.Errorf("error should carry the ValidateExtraction voice, got: %v", err)
	}
}

// TestLLMExtractNoteEmptyResponse mirrors induction.go:117-119 posture: an
// empty (or nil) model response is an explicit error, never a zero
// Extraction.
func TestLLMExtractNoteEmptyResponse(t *testing.T) {
	empty := &fakeGenerator{contents: []string{""}}
	if _, err := NewLLMExtractor(empty).ExtractNote(context.Background(), noteWith("001", "body")); err == nil {
		t.Fatal("empty response content should fail")
	} else if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention the empty response, got: %v", err)
	}

	nilResp := &fakeGenerator{nilResponse: true}
	if _, err := NewLLMExtractor(nilResp).ExtractNote(context.Background(), noteWith("001", "body")); err == nil {
		t.Fatal("nil response should fail")
	} else if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention the empty response, got: %v", err)
	}

	genErr := &fakeGenerator{err: errors.New("connection refused")}
	if _, err := NewLLMExtractor(genErr).ExtractNote(context.Background(), noteWith("001", "body")); err == nil {
		t.Fatal("transport error should propagate")
	} else if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("transport error should be wrapped with context, got: %v", err)
	}
}

// TestLLMExtractNoteOversizedUnion covers the prompt-size guardrail
// (RESEARCH Pitfall 6): a note whose body exceeds the section cap is
// extracted per-section with a union — NOT one full-note call. Dedupe by
// table name keeps the first kind; related notes dedupe by source_idx+
// relation.
func TestLLMExtractNoteOversizedUnion(t *testing.T) {
	sectionBody := strings.Repeat("SELECT a FROM b WHERE c = 1;\n", 4) // > 10 runes
	note := Note{
		SourceIdx: "001",
		Title:     "t",
		Sections: []MergedSection{
			{Section: "概述", SectionIdx: 0, Body: sectionBody},
			{Section: "SQL / 正文", SectionIdx: 1, Body: sectionBody},
		},
	}
	fake := &fakeGenerator{contents: []string{
		`{"tables":[{"name":"etl_a","kind":"update"},{"name":"etl_shared","kind":"insert"}],"related_notes":[{"source_idx":"006","relation":"mentions-note"}]}`,
		`{"tables":[{"name":"etl_shared","kind":"select"},{"name":"etl_b","kind":"create"}],"related_notes":[{"source_idx":"006","relation":"mentions-note"},{"source_idx":"007","relation":"mentions-note"}]}`,
	}}

	got, err := NewLLMExtractor(fake, 10).ExtractNote(context.Background(), note)
	if err != nil {
		t.Fatalf("ExtractNote: %v", err)
	}

	if fake.calls != len(note.Sections) {
		t.Fatalf("calls = %d, want %d (per-section calls, not one full-note call)", fake.calls, len(note.Sections))
	}
	if strings.Contains(fake.userContent(0), sectionBody[:len(sectionBody)/2]) &&
		strings.Count(fake.userContent(0), "SELECT a FROM b") != 1 {
		t.Errorf("section call 0 should carry only its own body once, got %q", fake.userContent(0))
	}

	wantTables := []ExtractedTable{
		{Name: "etl_a", Kind: "update"},
		{Name: "etl_shared", Kind: "insert"}, // first kind wins on dedupe
		{Name: "etl_b", Kind: "create"},
	}
	if len(got.Tables) != len(wantTables) {
		t.Fatalf("union tables = %#v, want %d entries", got.Tables, len(wantTables))
	}
	for i, w := range wantTables {
		if got.Tables[i] != w {
			t.Errorf("union table[%d] = %#v, want %#v", i, got.Tables[i], w)
		}
	}

	wantRelated := []RelatedNote{
		{SourceIdx: "006", Relation: "mentions-note"},
		{SourceIdx: "007", Relation: "mentions-note"},
	}
	if len(got.RelatedNotes) != len(wantRelated) {
		t.Fatalf("union related = %#v, want %d entries (deduped)", got.RelatedNotes, len(wantRelated))
	}
	for i, w := range wantRelated {
		if got.RelatedNotes[i] != w {
			t.Errorf("union related[%d] = %#v, want %#v", i, got.RelatedNotes[i], w)
		}
	}
}

// TestLLMExtractorDefaults pins the cross-plan constructor contract:
// NewLLMExtractor(chat) defaults the section cap to 40000 runes.
func TestLLMExtractorDefaults(t *testing.T) {
	if got := NewLLMExtractor(&fakeGenerator{}).sectionCap; got != 40000 {
		t.Errorf("default sectionCap = %d, want 40000", got)
	}
	if got := NewLLMExtractor(&fakeGenerator{}, 1234).sectionCap; got != 1234 {
		t.Errorf("explicit sectionCap = %d, want 1234", got)
	}
}

// TestExtractNoteViaStubFunc proves the Plan 03 regex stub sits behind the
// same Extractor seam as the LLM implementation (D-Ext-01's dual impl).
func TestExtractNoteViaStubFunc(t *testing.T) {
	var ex Extractor = StubExtractFunc(StubExtract)
	got, err := ex.ExtractNote(context.Background(), noteWith("001", "UPDATE etl_payment_due_t SET a = 1;\n"))
	if err != nil {
		t.Fatalf("StubExtractFunc.ExtractNote: %v", err)
	}
	if len(got.Tables) != 1 || got.Tables[0].Name != "etl_payment_due_t" || got.Tables[0].Kind != "update" {
		t.Errorf("stub extraction = %#v, want [{etl_payment_due_t update}]", got.Tables)
	}
}
