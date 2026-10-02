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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/skillforge/skillforge/internal/llm"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Generator is the narrow LLM surface the extractor needs (induction.go
// seam shape). It is satisfied by Eino's model.ToolCallingChatModel, which
// lets tests inject a fake.
type Generator interface {
	Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error)
}

// Generator is satisfied by Eino's chat models at compile time.
var _ Generator = (model.ToolCallingChatModel)(nil)

// Extractor produces a D-Ext-01 Extraction for one merged note. Two
// implementations share this seam: LLMExtractor (production, admin-run per
// D-Run-01) and StubExtractFunc (the Plan 03 deterministic regex stub that
// doubles as the golden-set drafter, D-GT-01).
type Extractor interface {
	ExtractNote(ctx context.Context, note Note) (Extraction, error)
}

// DefaultSectionCap is the whole-note prompt size guardrail in runes
// (RESEARCH Pitfall 6: note 039's merged body is ~88K chars; whole-note
// prompts above this cap fall back to per-section extraction with a result
// union).
const DefaultSectionCap = 40000

// extractionPromptTmpl is the fixed D-Ext-01 JSON schema instruction,
// mirroring induction's generatePrompt const style (induction.go:57-67).
// The output shape is locked: tables carry the validated kind enum and
// related_notes carry source_idx + a free-form relation tag.
//
// quick-261001-cz7 (2026-10-01 orchestrator decision "修 prompt 再归档"):
// the tables scope line + kind/range Rules bullets were rewritten from
// the original "every table the SQL touches" to the 39-09 golden policy-A
// WRITES-ONLY semantics — the live L1 gate measured the over-broad scope
// as the main F1 killer (read-only FROM/JOIN references extracted against
// a writes-only golden). The kind convention mirrors the stub's pinned
// mapping (39-03 Task 2): DELETE FROM / MERGE INTO normalize to "update",
// SELECT INTO to "select". The D-NZK-04 entities spec + entity rules
// remain strictly APPENDED below the tables/related contract.
const extractionPromptTmpl = `You are a data engineer analyzing a SkillForge knowledge note.
List every etl_* database table the note's SQL WRITES to, and every other note it cross-references.
WRITES-ONLY scope: list only DML TARGET tables — the table written by INSERT INTO, UPDATE, DELETE FROM, MERGE INTO, CREATE, or SELECT INTO. Tables that are only READ (appearing in SELECT, FROM, JOIN, or WHERE clauses without any write against them) MUST NOT be listed. If the note only queries tables and writes none, emit an empty tables array [].
Answer with a JSON object EXACTLY of the shape:
{"tables": [{"name": "<etl table name>", "kind": "update" | "select" | "insert" | "create"}],
 "related_notes": [{"source_idx": "<note id>", "relation": "<short relation tag>"}]}
Rules:
- "kind" is the DML type the note performs against the table, normalized: UPDATE → "update"; INSERT INTO → "insert"; CREATE → "create"; DELETE FROM or MERGE INTO → "update"; SELECT INTO → "select".
- Include only tables whose name starts with etl_. Report the BARE name without any schema/database prefix (e.g. "wuh_dw.etl_policy" → "etl_policy").
- If the note writes no etl_ tables or references no other notes, emit empty arrays.
- Output ONLY the JSON object, no fences, no prose.
The same JSON object also carries an "entities" array of business entities appearing in the note.
Each entity is a JSON object EXACTLY of the shape:
{"name": "<≤12字实体名>", "kind": "InsuranceProduct" | "Organization", "code": "<可选业务代码>", "parent": "<可选上级机构名,仅Organization>"}
Entity rules:
- Extract 险种 (insurance product) and 机构 (organization) names appearing in the note; "kind" is exactly "InsuranceProduct" or "Organization".
- "name" is at most 12 characters, written exactly as it appears in the note.
- "code" only when the note states a business code for the entity; "parent" only for an Organization and only when the note states its immediate parent organization name.
- If the note mentions no entities, emit an empty array [].

Note %s (title: %s):
%s`

// llmExtractionShape mirrors the fixed prompt output contract (the unmarshal
// target — keeps unknown model fields out of the public Extraction type,
// same discipline as induction's llmProposalShape).
type llmExtractionShape struct {
	Tables       []ExtractedTable  `json:"tables"`
	RelatedNotes []RelatedNote     `json:"related_notes"`
	Entities     []ExtractedEntity `json:"entities"` // D-NZK-04
}

// promptForBody renders the D-Ext-01 instruction for an arbitrary body
// (whole note or a single section on the oversized path).
func promptForBody(sourceIdx, title, body string) string {
	return fmt.Sprintf(extractionPromptTmpl, sourceIdx, title, body)
}

// joinSectionBodies concatenates the note's section bodies in order.
func joinSectionBodies(note Note) string {
	var b strings.Builder
	for i, s := range note.Sections {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(s.Body)
	}
	return b.String()
}

// LLMExtractor is the production Extractor: one deterministic (temperature
// 0, response_format=json_object) LLM call per note, parsed through the
// shared llm.ExtractJSON defensive path.
type LLMExtractor struct {
	chat       Generator
	sectionCap int
}

// Compile-time pin: both implementations satisfy the Extractor seam.
var (
	_ Extractor = LLMExtractor{}
	_ Extractor = StubExtractFunc(nil)
)

// NewLLMExtractor constructs an LLMExtractor. This exact name and signature
// are a cross-plan contract (Plan 06 Task 3 calls ecl.NewLLMExtractor(cm)) —
// do not rename it or change the parameter shape. The optional sectionCap
// overrides DefaultSectionCap (40000 runes) when positive.
func NewLLMExtractor(chat Generator, sectionCap ...int) LLMExtractor {
	cap := DefaultSectionCap
	if len(sectionCap) > 0 && sectionCap[0] > 0 {
		cap = sectionCap[0]
	}
	return LLMExtractor{chat: chat, sectionCap: cap}
}

// ExtractNote produces the D-Ext-01 Extraction for one merged note.
//
// Under the section cap the whole note body goes out in a single call; over
// the cap each section is extracted separately and the results are unioned
// (tables deduped by name keeping the first kind; related notes deduped by
// source_idx+relation) — RESEARCH Pitfall 6's guardrail against oversized
// prompts.
//
// Every failure path returns a zero Extraction and a wrapped error: no
// partial Extractions ever escape (the caller either gets a validated
// result or a dead-letterable failure).
func (e LLMExtractor) ExtractNote(ctx context.Context, note Note) (Extraction, error) {
	if e.chat == nil {
		return Extraction{}, errors.New("ecl: extract: no LLM client configured")
	}
	if utf8.RuneCountInString(joinSectionBodies(note)) <= e.sectionCap {
		return e.extractBody(ctx, note.SourceIdx, note.Title, joinSectionBodies(note))
	}

	var union Extraction
	seenTables := map[string]bool{}
	seenRelated := map[string]bool{}
	seenEntities := map[string]bool{} // D-NZK-01: entity identity = exact name, keep-first
	for _, s := range note.Sections {
		// IN-02: sections over MinSplitRunes are sub-split so every LLM prompt
		// stays under the cap. Bodies within the cap go directly to extractBody.
		var bodies []string
		if utf8.RuneCountInString(s.Body) > MinSplitRunes {
			splitter, err := buildSplitter()
			if err != nil {
				return Extraction{}, fmt.Errorf("ecl: extract: build splitter: %w", err)
			}
			parts, err := subSplit(splitter, s.Body)
			if err != nil {
				return Extraction{}, fmt.Errorf("ecl: extract: sub-split oversize section %s: %w", s.Section, err)
			}
			bodies = parts
		} else {
			bodies = []string{s.Body}
		}
		for _, body := range bodies {
			part, err := e.extractBody(ctx, note.SourceIdx, note.Title, body)
			if err != nil {
				return Extraction{}, err
			}
			for _, t := range part.Tables {
				if seenTables[t.Name] {
					continue // first kind wins
				}
				seenTables[t.Name] = true
				union.Tables = append(union.Tables, t)
			}
			for _, r := range part.RelatedNotes {
				key := r.SourceIdx + "\x00" + r.Relation
				if seenRelated[key] {
					continue
				}
				seenRelated[key] = true
				union.RelatedNotes = append(union.RelatedNotes, r)
			}
			for _, ent := range part.Entities {
				// part.Entities already passed sanitizeEntities (trimmed),
				// so the exact trimmed name is the dedupe key (D-NZK-01).
				if seenEntities[ent.Name] {
					continue
				}
				seenEntities[ent.Name] = true
				union.Entities = append(union.Entities, ent)
			}
		}
	}
	return union, nil
}

// extractBody runs ONE LLM call for one body chunk and defensively parses
// the response (induction.go Generate posture: fixed system contract,
// temperature 0, response_format=json_object, empty-response guard,
// ExtractJSON, strict unmarshal, enum validation).
func (e LLMExtractor) extractBody(ctx context.Context, sourceIdx, title, body string) (Extraction, error) {
	resp, err := e.chat.Generate(ctx, []*schema.Message{
		schema.SystemMessage("You output only valid JSON."),
		schema.UserMessage(promptForBody(sourceIdx, title, body)),
	}, model.WithTemperature(0.0), llm.JSONObjectResponseFormat())
	if err != nil {
		return Extraction{}, fmt.Errorf("ecl: extract: llm generate: %w", err)
	}
	if resp == nil || resp.Content == "" {
		return Extraction{}, errors.New("ecl: extract: llm returned empty response")
	}
	raw := llm.ExtractJSON(resp.Content)
	if raw == "" {
		return Extraction{}, fmt.Errorf("ecl: extract: no JSON object in llm response: %s", resp.Content)
	}
	var shape llmExtractionShape
	if err := json.Unmarshal([]byte(raw), &shape); err != nil {
		return Extraction{}, fmt.Errorf("ecl: extract: unmarshal llm response: %w (raw: %s)", err, raw)
	}
	// D-NZK-04: entity drop-gates run BEFORE ValidateExtraction so the
	// tables hard-error gate semantics stay untouched — sanitizeEntities
	// cannot fail, it only drops entries (T-NZK-01).
	// quick-261001-cz7: schema-qualified names are de-qualified BEFORE the
	// gate — the ^etl_\w+$ contract still rejects anything that fails
	// after the strip (contract not relaxed, only de-qualified).
	for i := range shape.Tables {
		shape.Tables[i].Name = stripSchemaPrefix(shape.Tables[i].Name)
	}
	ex := Extraction{Tables: shape.Tables, RelatedNotes: shape.RelatedNotes, Entities: sanitizeEntities(shape.Entities)}
	if err := ValidateExtraction(ex); err != nil {
		return Extraction{}, fmt.Errorf("ecl: extract: %w", err)
	}
	return ex, nil
}

// stripSchemaPrefix de-qualifies an LLM-extracted table name before the
// corpus-contract gate: "wuh_dw.etl_policy" → "etl_policy". The gate
// itself (^etl_\w+$) is NOT relaxed — a name that still fails after the
// strip is rejected as before. LastIndexByte (not strings.Split) so
// multi-part qualifiers ("a.b.etl_c") reduce to the bare local name.
// (quick-261001-cz7: the live L1 run showed glm-5.3-flash qualifying
// names with the wuh_dw schema even when told not to — strip, don't
// trust.)
func stripSchemaPrefix(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i+1:]
	}
	return name
}

// StubExtractFunc adapts the Plan 03 regex stub function (StubExtract) to
// the Extractor interface, keeping Plan 03's public surface untouched while
// letting it share the seam (compile-time pin: var _ Extractor =
// StubExtractFunc(nil)). It never errors — the regex scan is total.
type StubExtractFunc func(n Note) Extraction

// ExtractNote implements Extractor via the deterministic regex scan.
func (f StubExtractFunc) ExtractNote(_ context.Context, n Note) (Extraction, error) {
	return f(n), nil
}
