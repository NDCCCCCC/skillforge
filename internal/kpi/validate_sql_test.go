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

package kpi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSerializer implements SQLSerializer for hermetic testing.
// It returns a pre-recorded JSON string (from probe fixtures or inline).
type fakeSerializer struct {
	json string // raw JSON to return
	err  error  // Go error to return (infra failure)
}

func (f *fakeSerializer) SerializeSQL(_ context.Context, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.json, nil
}

// loadProbeFixture loads a probe-N.json fixture from the wsl/testdata directory.
func loadProbeFixture(t *testing.T, n int) string {
	t.Helper()
	// Probe fixtures live in internal/kpi/wsl/testdata/ relative to this test file
	// which is in internal/kpi/. Navigate up and down.
	path := filepath.Join("wsl", "testdata", fmt.Sprintf("probe-%d.json", n))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read probe fixture %s: %v", path, err)
	}
	return string(data)
}

func TestValidateSQLAST_Probe1_ValidSelect(t *testing.T) {
	// Probe 1: valid SELECT with ds_loans.data → should pass, 1 reference
	ser := &fakeSerializer{json: loadProbeFixture(t, 1)}
	refs, err := ValidateSQLAST(context.Background(), ser,
		`SELECT status AS status, count(*) AS row_count FROM ds_loans.data GROUP BY status`,
		"loans")
	require.NoError(t, err)
	assert.Equal(t, []string{"ds_loans.data"}, refs, "should find exactly 1 table reference ds_loans.data")
}

func TestValidateSQLAST_Probe2_SyntaxError(t *testing.T) {
	// Probe 2: invalid SQL → error:true envelope → Rule 1 reject
	ser := &fakeSerializer{json: loadProbeFixture(t, 2)}
	_, err := ValidateSQLAST(context.Background(), ser,
		`TOTALLY NOT VALID SQL`,
		"loans")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SQL parse error")
	assert.Contains(t, err.Error(), "parser")
}

func TestValidateSQLAST_Probe3_InsertRejected(t *testing.T) {
	// Probe 3: INSERT → error:true envelope with "not implemented" → Rule 1 reject
	// (A1 lock: INSERT returns error:true, stronger than assumed — both Rule 1 and
	// Rule 3 dual-safety, but Rule 1 fires first)
	ser := &fakeSerializer{json: loadProbeFixture(t, 3)}
	_, err := ValidateSQLAST(context.Background(), ser,
		`INSERT INTO ds_loans.data VALUES (1)`,
		"loans")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SQL parse error")
}

func TestValidateSQLAST_Probe4_CrossAliasJoin(t *testing.T) {
	// Probe 4: cross-alias JOIN (ds_loans + ds_cards) → Rule 4 reject with ds_cards
	ser := &fakeSerializer{json: loadProbeFixture(t, 4)}
	_, err := ValidateSQLAST(context.Background(), ser,
		`SELECT count(*) FROM ds_loans.data JOIN ds_cards.data ON true`,
		"loans")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ds_cards.data")
	assert.Contains(t, err.Error(), "not matching declared dataset")
}

func TestValidateSQLAST_Probe5_EmptyFrom(t *testing.T) {
	// Probe 5: SELECT 42 (empty FROM) → Rule 4 reject (zero references)
	ser := &fakeSerializer{json: loadProbeFixture(t, 5)}
	_, err := ValidateSQLAST(context.Background(), ser,
		`SELECT 42`,
		"loans")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zero table references")
}

func TestValidateSQLAST_Probe6_Subquery(t *testing.T) {
	// Probe 6: subquery in select list with ds_loans.data → should pass, 1 reference
	ser := &fakeSerializer{json: loadProbeFixture(t, 6)}
	refs, err := ValidateSQLAST(context.Background(), ser,
		`SELECT (SELECT count(*) FROM ds_loans.data) AS c`,
		"loans")
	require.NoError(t, err)
	assert.Equal(t, []string{"ds_loans.data"}, refs, "subquery table ref should be collected")
}

func TestValidateSQLAST_MultipleStatements(t *testing.T) {
	// Inline construction: envelope.Statements length 2 → Rule 2 reject
	envelope := map[string]any{
		"error":      false,
		"statements": []map[string]any{{"node": map[string]any{"type": "SELECT_NODE"}}, {"node": map[string]any{"type": "SELECT_NODE"}}},
	}
	raw, _ := json.Marshal(envelope)
	ser := &fakeSerializer{json: string(raw)}
	_, err := ValidateSQLAST(context.Background(), ser, `SELECT 1; SELECT 2`, "loans")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected exactly 1 statement")
	assert.Contains(t, err.Error(), "2")
}

func TestValidateSQLAST_InfraError(t *testing.T) {
	// SerializeSQL returns Go error (infra failure) → transparent passthrough
	ser := &fakeSerializer{err: fmt.Errorf("connection refused")}
	_, err := ValidateSQLAST(context.Background(), ser, `SELECT 1`, "loans")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SerializeSQL infra error")
	assert.Contains(t, err.Error(), "connection refused")
}
