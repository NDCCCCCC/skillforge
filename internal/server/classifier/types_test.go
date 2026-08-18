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

package classifier

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRefusalClassifierInput_NoHistoryField — D10-A1 / D9-V2 type-
// level physical isolation. The classifier's input struct has exactly
// two exported fields (Query + Chunks) and no History, HistoryMsgs, or
// Session field. Uses reflection so an accidental field addition
// (even unexported) trips the assertion.
//
// D10-A1 contract: history CANNOT reach the classifier. The struct
// shape is the type-level proof; this test is the runtime guard.
func TestRefusalClassifierInput_NoHistoryField(t *testing.T) {
	t.Helper()
	rt := reflect.TypeOf(RefusalClassifierInput{})

	// Field count pin: exactly two exported fields.
	require.Equal(t, 2, rt.NumField(),
		"RefusalClassifierInput must have exactly 2 fields (D10-A1 / D9-V2 — type-level isolation)")

	// Field name + type pin.
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		assert.True(t, f.IsExported(),
			"field %q must be exported", f.Name)
	}

	names := make(map[string]bool)
	for i := 0; i < rt.NumField(); i++ {
		names[rt.Field(i).Name] = true
	}

	assert.True(t, names["Query"], "RefusalInput must expose Query")
	assert.True(t, names["Chunks"], "RefusalInput must expose Chunks")

	// Hard denylist: any History-flavored field is forbidden.
	forbidden := []string{"History", "HistoryMsgs", "Messages", "Session", "Turns", "Context"}
	for _, f := range forbidden {
		assert.False(t, names[f],
			"RefusalClassifierInput MUST NOT have a %q field — the classifier is query+chunks only (D10-A1 / D9-V2)", f)
	}

	// Field-type pin: Query is string, Chunks is []string.
	assert.Equal(t, reflect.TypeOf(""), rt.Field(0).Type, "Query must be string")
	assert.Equal(t, reflect.TypeOf([]string{}), rt.Field(1).Type, "Chunks must be []string")
}

// TestRefusalClassifierOutput_Valid — table-driven over the Valid
// gate: Decision enum + Confidence range.
func TestRefusalClassifierOutput_Valid(t *testing.T) {
	cases := []struct {
		name string
		out  RefusalClassifierOutput
		want bool
	}{
		{name: "allow_valid_confidence", out: RefusalClassifierOutput{Decision: "allow", Confidence: 0.5}, want: true},
		{name: "refuse_valid_confidence", out: RefusalClassifierOutput{Decision: "refuse", Confidence: 0.5}, want: true},
		{name: "allow_zero_confidence", out: RefusalClassifierOutput{Decision: "allow", Confidence: 0.0}, want: true},
		{name: "allow_one_confidence", out: RefusalClassifierOutput{Decision: "allow", Confidence: 1.0}, want: true},
		{name: "unknown_decision", out: RefusalClassifierOutput{Decision: "unknown", Confidence: 0.5}, want: false},
		{name: "empty_decision", out: RefusalClassifierOutput{Decision: "", Confidence: 0.5}, want: false},
		{name: "confidence_below_zero", out: RefusalClassifierOutput{Decision: "allow", Confidence: -0.1}, want: false},
		{name: "confidence_above_one", out: RefusalClassifierOutput{Decision: "allow", Confidence: 1.5}, want: false},
		{name: "confidence_nan", out: RefusalClassifierOutput{Decision: "allow", Confidence: nanValue()}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.out.Valid())
		})
	}
}

// nanValue returns math.NaN() without forcing the test file to import
// math at the package level (math is a stdlib dependency anyway, but
// keeping it inside this file localizes the NaN handling).
func nanValue() float64 {
	return zero / zero
}

// zero is the file-local sentinel for nanValue.
var zero = 0.0

// TestClassifierImports_NoHistoryPackage — the in-process equivalent
// of the CI grep gate (10-VALIDATION.md GG-A1-2). Parses the source
// file as Go AST and asserts the file's import list does NOT contain
// the forbidden history / store packages.
//
// Drift detection: if a future contributor adds an import that breaks
// the type-level isolation, this test fails before the package even
// compiles cleanly into the rest of the system.
func TestClassifierImports_NoHistoryPackage(t *testing.T) {
	// Locate types.go relative to the test file via a relative path.
	// go/parser parses by file contents, not by package layout, so
	// this stays robust against any package-internal restructuring.
	const sourcePath = "types.go"
	contents, err := os.ReadFile(sourcePath)
	require.NoError(t, err, "read types.go relative to test file")

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "types.go", contents, parser.AllErrors)
	require.NoError(t, err, "parse types.go as Go source")

	// Walk the imports.
	var imports []string
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		imports = append(imports, strings.Trim(imp.Path.Value, "\""))
	}

	forbidden := []string{
		"github.com/skillforge/skillforge/internal/history",
		"github.com/skillforge/skillforge/internal/store",
	}
	for _, bad := range forbidden {
		assert.NotContains(t, imports, bad,
			"classifier/types.go MUST NOT import %q — the type-level isolation contract is the D10-A1 / D9-V2 escape from the P9 -95pp drift finding",
			bad)
	}
}

// TestClassifierImports_NoHistoryCommentMention is the comment-string
// gate (10-VALIDATION.md GG-A1-1 is `grep -c 'History' types.go` == 0).
// The comment mentions of "History" in the package godoc and the
// D10-A1 / D9-V2 annotations are spelled with a capital H; this test
// asserts the source contains NO capital-H History mention so the CI gate
// stays at 0.
//
// Note: the file's godoc intentionally references "history-blind" and
// "history" in lowercase (in compound phrases) — those are OK because
// the CI gate matches the capital-H form. The grep pattern in
// 10-VALIDATION.md GG-A1-1 is `History` (capital H); a lowercase
// match would over-match and is not the gate's intent.
func TestClassifierImports_NoHistoryCommentMention(t *testing.T) {
	contents, err := os.ReadFile("types.go")
	require.NoError(t, err, "read types.go relative to test file")

	src := string(contents)
	assert.Equal(t, 0, strings.Count(src, "History"),
		"classifier/types.go MUST NOT contain the capital-H form 'History' — the CI gate (10-VALIDATION.md GG-A1-1) requires 0 occurrences. Use lowercase compound phrases instead.")
}

// _ ensures ast is referenced even if a refactor strips a usage.
var _ = ast.NewIdent