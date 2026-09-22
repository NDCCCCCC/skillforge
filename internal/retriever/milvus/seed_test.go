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

package milvus

// Hermetic tests for EnsureCollection + SeedDocuments — the schema and
// insert path that Plan 39-02 extended with the four native ECL section
// fields (D-Milvus-01/02). The properties under test:
//
//   - the fresh-deploy schema carries exactly 11 fields, with widths
//     derived from the single-home consts (zero-hardcode standard);
//   - every Insert carries all 11 columns in schema order, with the
//     VarChar columns built via NewColumnVarChar (NEVER NewColumnString —
//     spike-findings DataType String=20 vs FieldTypeVarChar=21 mismatch);
//   - legacy-shaped Documents (D-Milvus-02) seed with zero-value section
//     fields, while populated ECL Documents seed their values intact and
//     over-width VarChar values are rune-truncated to the column widths.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/skillforge/skillforge/internal/retriever"
)

// seedMock records the schema/DML sequence and captures the payloads.
// Unmocked methods of the embedded nil interface panic — the
// hermetic-mock discipline from recreateMockClient.
type seedMock struct {
	client.Client
	calls  []string
	exists bool
	schema *entity.Schema
	cols   []entity.Column
}

func (m *seedMock) HasCollection(_ context.Context, collName string) (bool, error) {
	m.calls = append(m.calls, "has:"+collName)
	return m.exists, nil
}

func (m *seedMock) CreateCollection(_ context.Context, schema *entity.Schema, _ int32, _ ...client.CreateCollectionOption) error {
	m.calls = append(m.calls, "create:"+schema.CollectionName)
	m.schema = schema
	m.exists = true
	return nil
}

func (m *seedMock) CreateIndex(_ context.Context, collName string, _ string, _ entity.Index, _ bool, _ ...client.IndexOption) error {
	m.calls = append(m.calls, "index:"+collName)
	return nil
}

func (m *seedMock) Insert(_ context.Context, collName string, _ string, cols ...entity.Column) (entity.Column, error) {
	m.calls = append(m.calls, "insert:"+collName)
	m.cols = append(m.cols, cols...)
	return nil, nil
}

func (m *seedMock) Flush(_ context.Context, collName string, _ bool, _ ...client.FlushOption) error {
	m.calls = append(m.calls, "flush:"+collName)
	return nil
}

func (m *seedMock) LoadCollection(_ context.Context, collName string, _ bool, _ ...client.LoadCollectionOption) error {
	m.calls = append(m.calls, "load:"+collName)
	return nil
}

// Compile-time guard: the seed tests keep working against the Client
// interface they mock.
var _ client.Client = (*seedMock)(nil)

// seedTestCfg is the shared test config: a small embedding dim keeps the
// vector column light (withDefaults would otherwise fill in 1024).
func seedTestCfg() Config {
	return Config{CollectionName: DefaultCollectionName, EmbedDim: 4}
}

// seedTestDoc builds one 4-dim legacy-shaped Document.
func seedTestDoc(id int64) retriever.Document {
	return retriever.Document{
		ID:            id,
		Tier:          "sql-guides",
		Content:       fmt.Sprintf("chunk %d", id),
		Source:        "sql-guides/001_note.md",
		Embedding:     []float32{0.1, 0.2, 0.3, 0.4},
		EffectiveFrom: 0,
		EffectiveTo:   0,
	}
}

// wantSeedColumns is the 11-column schema/insert contract in order
// (Plan 39-02 / D-Milvus-01). The slice order is the single home for the
// column-order assertion — SeedDocuments must insert in this order so the
// collection schema and the insert payloads can never disagree.
var wantSeedColumns = []struct {
	name string
	typ  entity.FieldType
}{
	{"id", entity.FieldTypeInt64},
	{"tier", entity.FieldTypeVarChar},
	{"content", entity.FieldTypeVarChar},
	{"source", entity.FieldTypeVarChar},
	{"embedding", entity.FieldTypeFloatVector},
	{"effective_from", entity.FieldTypeInt64},
	{"effective_to", entity.FieldTypeInt64},
	{"source_idx", entity.FieldTypeVarChar},
	{"section", entity.FieldTypeVarChar},
	{"section_idx", entity.FieldTypeInt64},
	{"section_count", entity.FieldTypeInt64},
}

func seedColumnByName(cols []entity.Column, name string) entity.Column {
	for _, c := range cols {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

// TestSeedDocumentsLegacyDocSeedsZeroValueSectionFields: a legacy-shaped
// Document (D-Milvus-02) must seed with the zero-value contract —
// source_idx="", section="", section_idx=0, section_count=0 — so the
// retrieval filter semantics of existing rows are unchanged and the
// ownership marker (section_count > 0) is not set spuriously.
func TestSeedDocumentsLegacyDocSeedsZeroValueSectionFields(t *testing.T) {
	mc := &seedMock{exists: true}
	docs := []retriever.Document{seedTestDoc(1), seedTestDoc(2)}

	if err := SeedDocuments(context.Background(), mc, seedTestCfg(), docs); err != nil {
		t.Fatalf("SeedDocuments: %v", err)
	}

	// Exactly one Insert carrying all 11 columns.
	if got := len(mc.cols); got != len(wantSeedColumns) {
		t.Fatalf("Insert carried %d columns, want %d", got, len(wantSeedColumns))
	}

	// Column order + types.
	for i, want := range wantSeedColumns {
		col := mc.cols[i]
		if col.Name() != want.name {
			t.Fatalf("column %d = %q, want %q (columns must arrive in schema order)", i, col.Name(), want.name)
		}
		if col.Type() != want.typ {
			t.Fatalf("column %q type = %v, want %v", want.name, col.Type(), want.typ)
		}
		if col.Len() != len(docs) {
			t.Fatalf("column %q len = %d, want %d", want.name, col.Len(), len(docs))
		}
	}

	// D-Milvus-02 zero-value contract for every legacy row.
	for i := range docs {
		if v, err := seedColumnByName(mc.cols, "source_idx").GetAsString(i); err != nil || v != "" {
			t.Fatalf("row %d source_idx = (%q, %v), want (\"\", nil)", i, v, err)
		}
		if v, err := seedColumnByName(mc.cols, "section").GetAsString(i); err != nil || v != "" {
			t.Fatalf("row %d section = (%q, %v), want (\"\", nil)", i, v, err)
		}
		if v, err := seedColumnByName(mc.cols, "section_idx").GetAsInt64(i); err != nil || v != 0 {
			t.Fatalf("row %d section_idx = (%d, %v), want (0, nil)", i, v, err)
		}
		if v, err := seedColumnByName(mc.cols, "section_count").GetAsInt64(i); err != nil || v != 0 {
			t.Fatalf("row %d section_count = (%d, %v), want (0, nil) — 0 is the legacy marker, never set it for legacy rows", i, v, err)
		}
	}

	// Insert → Flush → Load sequence (unchanged legacy behavior).
	insertAt, flushAt, loadAt := -1, -1, -1
	for i, c := range mc.calls {
		switch {
		case strings.HasPrefix(c, "insert:"):
			insertAt = i
		case strings.HasPrefix(c, "flush:"):
			if insertAt < 0 {
				t.Fatalf("Flush ran before Insert, calls = %v", mc.calls)
			}
			flushAt = i
		case strings.HasPrefix(c, "load:"):
			if flushAt < 0 {
				t.Fatalf("LoadCollection ran before Flush, calls = %v", mc.calls)
			}
			loadAt = i
		}
	}
	if insertAt < 0 || flushAt < 0 || loadAt < 0 {
		t.Fatalf("expected insert+flush+load sequence, calls = %v", mc.calls)
	}
}

// TestSeedDocumentsSectionFieldsPopulated: an ECL Document seeds its native
// section metadata intact, and over-width VarChar values are rune-truncated
// to the single-home column widths (the same truncateRunes discipline the
// content/source columns follow).
func TestSeedDocumentsSectionFieldsPopulated(t *testing.T) {
	mc := &seedMock{exists: true}
	doc := seedTestDoc(100)
	doc.SourceIdx = "001"
	doc.Section = "SQL 基础"
	doc.SectionIdx = 2
	doc.SectionCount = 5
	doc.SourceIdx += strings.Repeat("x", DefaultSourceIdxMaxLen) // force truncation to 64
	doc.Section += strings.Repeat("y", DefaultSectionMaxLen+100) // force truncation to 256
	wantSourceIdx := string([]rune(doc.SourceIdx)[:DefaultSourceIdxMaxLen])
	wantSection := string([]rune(doc.Section)[:DefaultSectionMaxLen])

	if err := SeedDocuments(context.Background(), mc, seedTestCfg(), []retriever.Document{doc}); err != nil {
		t.Fatalf("SeedDocuments: %v", err)
	}

	if got := len(mc.cols); got != len(wantSeedColumns) {
		t.Fatalf("Insert carried %d columns, want %d", got, len(wantSeedColumns))
	}
	if v, err := seedColumnByName(mc.cols, "source_idx").GetAsString(0); err != nil {
		t.Fatalf("source_idx: %v", err)
	} else if v != wantSourceIdx {
		t.Fatalf("source_idx = %q, want the %d-rune-truncated value %q", v, DefaultSourceIdxMaxLen, wantSourceIdx)
	}
	if v, err := seedColumnByName(mc.cols, "section").GetAsString(0); err != nil {
		t.Fatalf("section: %v", err)
	} else if v != wantSection {
		t.Fatalf("section = %q, want the CJK prefix rune-truncated to %d runes %q", v, DefaultSectionMaxLen, wantSection)
	}
	if v, err := seedColumnByName(mc.cols, "section_idx").GetAsInt64(0); err != nil || v != 2 {
		t.Fatalf("section_idx = (%d, %v), want (2, nil)", v, err)
	}
	if v, err := seedColumnByName(mc.cols, "section_count").GetAsInt64(0); err != nil || v != 5 {
		t.Fatalf("section_count = (%d, %v), want (5, nil) — > 0 marks the row ECL-owned", v, err)
	}
}

// TestEnsureCollectionCreatesElevenFieldSchema: the fresh-deploy schema
// carries exactly the 11 planned fields in order, with VarChar widths
// derived from the single-home consts (zero-hardcode standard — the test
// compares against the consts, never literals) and sentinel-0 INT64
// section fields.
func TestEnsureCollectionCreatesElevenFieldSchema(t *testing.T) {
	mc := &seedMock{exists: false}

	if err := EnsureCollection(context.Background(), mc, seedTestCfg()); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	if mc.schema == nil {
		t.Fatal("CreateCollection was never called — no schema captured")
	}

	if got := len(mc.schema.Fields); got != len(wantSeedColumns) {
		t.Fatalf("schema has %d fields, want %d (Plan 39-02 D-Milvus-01: 7 legacy + 4 ECL)", got, len(wantSeedColumns))
	}
	for i, want := range wantSeedColumns {
		f := mc.schema.Fields[i]
		if f.Name != want.name {
			t.Fatalf("field %d = %q, want %q", i, f.Name, want.name)
		}
		if f.DataType != want.typ {
			t.Fatalf("field %q type = %v, want %v", want.name, f.DataType, want.typ)
		}
	}

	// Widths derive from the single-home consts — no literal widths.
	if got := mc.schema.Fields[1].TypeParams[entity.TypeParamMaxLength]; got != fmt.Sprint(DefaultTierMaxLen) {
		t.Fatalf("tier max_length = %q, want %q (single-home const)", got, fmt.Sprint(DefaultTierMaxLen))
	}
	if got := mc.schema.Fields[3].TypeParams[entity.TypeParamMaxLength]; got != fmt.Sprint(DefaultSourceMaxLen) {
		t.Fatalf("source max_length = %q, want %q (single-home const)", got, fmt.Sprint(DefaultSourceMaxLen))
	}
	if got := mc.schema.Fields[7].TypeParams[entity.TypeParamMaxLength]; got != fmt.Sprint(DefaultSourceIdxMaxLen) {
		t.Fatalf("source_idx max_length = %q, want %q (DefaultSourceIdxMaxLen single home)", got, fmt.Sprint(DefaultSourceIdxMaxLen))
	}
	if got := mc.schema.Fields[8].TypeParams[entity.TypeParamMaxLength]; got != fmt.Sprint(DefaultSectionMaxLen) {
		t.Fatalf("section max_length = %q, want %q (DefaultSectionMaxLen single home)", got, fmt.Sprint(DefaultSectionMaxLen))
	}
	// INT64 fields must NOT carry a max_length param.
	for _, idx := range []int{0, 5, 6, 9, 10} {
		if got := mc.schema.Fields[idx].TypeParams[entity.TypeParamMaxLength]; got != "" {
			t.Fatalf("field %q carries max_length %q — INT64 fields are width-less", mc.schema.Fields[idx].Name, got)
		}
	}
	// id stays the explicit (non-auto) primary key.
	if !mc.schema.Fields[0].PrimaryKey || mc.schema.Fields[0].AutoID {
		t.Fatalf("id field: PK=%v AutoID=%v, want PK=true AutoID=false (caller assigns numeric IDs)", mc.schema.Fields[0].PrimaryKey, mc.schema.Fields[0].AutoID)
	}
}
