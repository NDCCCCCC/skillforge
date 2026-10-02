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
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// quick-261001-cz7 follow-up (orchestrator decision "修 prompt 再归档"):
// the live L1 run (F1=0.3938) surfaced two root causes against the 39-09
// writes-only golden policy A — (1) the prompt asked for every table the
// SQL "touches", so read-only references were extracted; (2) schema-
// qualified names (wuh_dw.etl_policy) hard-errored the whole note at the
// corpus-contract gate. These tests pin both fixes.

// writesOnlyFakeGen returns a canned LLM response, standing in for the real
// chat model (induction-test posture).
type writesOnlyFakeGen struct{ content string }

func (f writesOnlyFakeGen) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return &schema.Message{Content: f.content}, nil
}

// TestExtractionPromptWritesOnly pins the 39-09 policy-A semantics in the
// D-Ext-01 prompt text: DML TARGETS only; read-only references excluded.
func TestExtractionPromptWritesOnly(t *testing.T) {
	prompt := extractionPromptTmpl

	// The writes-only scope rule must be explicit.
	for _, want := range []string{
		"INSERT", "UPDATE", "DELETE", "MERGE", "CREATE",
		"WRITES-ONLY",
		"MUST NOT be listed",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("extractionPromptTmpl missing writes-only token %q — the prompt must state that only DML target tables are extracted", want)
		}
	}

	// The old over-broad wording invited read-only extraction.
	if strings.Contains(prompt, "SQL touches") {
		t.Errorf("extractionPromptTmpl still says %q — over-broad scope is the measured cause of the F1=0.3938 live gate failure", "SQL touches")
	}

	// The kind convention must mirror the stub's pinned mapping (39-03
	// Task 2): DELETE FROM / MERGE INTO normalize to "update", SELECT INTO
	// to "select" — otherwise live kinds mismatch the golden's kinds.
	for _, want := range []string{`DELETE FROM`, `MERGE INTO`, `SELECT INTO`} {
		if !strings.Contains(prompt, want) {
			t.Errorf("extractionPromptTmpl missing kind-convention token %q (stub-pinned mapping)", want)
		}
	}
}

// TestStripSchemaPrefix is the pure-function contract for the schema-prefix
// strip applied before the corpus-contract gate.
func TestStripSchemaPrefix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"wuh_dw.etl_policy", "etl_policy"},   // live-failure case
		{"etl_policy", "etl_policy"},          // already clean
		{"a.b.etl_c", "etl_c"},                // multi-part: LastIndex wins
		{"etl_NONAUTO_POLICY_T", "etl_NONAUTO_POLICY_T"}, // case kept
		{"", ""},                              // empty stays empty (gate rejects)
	}
	for _, c := range cases {
		if got := stripSchemaPrefix(c.in); got != c.want {
			t.Errorf("stripSchemaPrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestLLMExtractorStripsSchemaPrefix runs the fake-LLM path end to end: a
// schema-qualified table name must now EXTRACT cleanly (previously a hard
// error killed the whole note), while a non-etl name must STILL be
// rejected — the corpus contract is not relaxed, only de-qualified.
func TestLLMExtractorStripsSchemaPrefix(t *testing.T) {
	ext := NewLLMExtractor(writesOnlyFakeGen{content: `{"tables":[{"name":"wuh_dw.etl_policy","kind":"update"}],"related_notes":[],"entities":[]}`})
	ex, err := ext.ExtractNote(context.Background(), Note{SourceIdx: "T1", Title: "t"})
	if err != nil {
		t.Fatalf("schema-qualified etl_ name must extract after prefix strip: %v", err)
	}
	if len(ex.Tables) != 1 || ex.Tables[0].Name != "etl_policy" {
		t.Fatalf("tables = %+v, want single etl_policy", ex.Tables)
	}

	ext = NewLLMExtractor(writesOnlyFakeGen{content: `{"tables":[{"name":"dim_user","kind":"update"}],"related_notes":[],"entities":[]}`})
	if _, err := ext.ExtractNote(context.Background(), Note{SourceIdx: "T2", Title: "t"}); err == nil {
		t.Fatal("non-etl name after strip must still violate the corpus contract (contract not relaxed)")
	}
}
