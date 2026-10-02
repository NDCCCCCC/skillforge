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

package report

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
)

// stubAudit is a minimal ReportAuditSink stub recording the row.
type stubAudit struct{ rows []store.ReportRow }

func (s *stubAudit) PutReport(_ context.Context, row store.ReportRow) error {
	s.rows = append(s.rows, row)
	return nil
}

func validTestTemplate() *ReportTemplate {
	return &ReportTemplate{
		Title: "2026-08 业务月报",
		Sections: []SectionTemplate{
			{Title: "业务概览", Slots: []Slot{{Type: "narrative"}}},
		},
	}
}

// chdirTemp moves the process into a fresh temp dir so publish writes
// never touch the repo working tree.
func chdirTemp(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// TestIsValidPeriod — CR-05: the traversal guard must accept the
// production period forms and reject every traversal / absolute /
// drive / empty form.
func TestIsValidPeriod(t *testing.T) {
	valid := []string{"2026-08", "2026-Q3", "2026_08", "aug"}
	for _, p := range valid {
		assert.True(t, IsValidPeriod(p), "period %q must be valid", p)
	}
	invalid := []string{
		"../scheduled-tasks", // parent traversal
		"..",                 // bare traversal
		"a/b",                // path separator
		`a\b`,                // windows separator
		"/etc",               // absolute
		"C:temp",             // windows drive fragment
		".",                  // dot
		"%2e%2e",             // encoded traversal characters rejected by class
		"",                   // empty
		"2026-08\n",          // newline escape
	}
	for _, p := range invalid {
		assert.False(t, IsValidPeriod(p), "period %q must be rejected", p)
	}
}

// TestPublishRejectsTraversalPeriod — CR-05: the publish boundary
// must fail closed BEFORE any MkdirAll/WriteFile for a traversal
// period, even though a path shaped like this would escape the
// reports/ directory if joined naively.
func TestPublishRejectsTraversalPeriod(t *testing.T) {
	chdirTemp(t)

	audit := &stubAudit{}
	res, err := Publish(context.Background(), validTestTemplate(), nil,
		"../scheduled-tasks", nil, "tc-1", nil, "monthly-report", audit)
	require.Error(t, err, "traversal period must be rejected at the publish boundary")
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "invalid period")

	_, statErr := os.Stat("scheduled-tasks")
	assert.True(t, os.IsNotExist(statErr), "no directory may be created outside reports/")
	_, statErr = os.Stat("reports")
	assert.True(t, os.IsNotExist(statErr), "publish must not have touched the filesystem")
	assert.Empty(t, audit.rows, "no audit row for a rejected publish")
}

// TestPublishWritesAuditRow — CR-04 store-side happy path from the
// report package's perspective: one row per publish with the digest +
// template version populated.
func TestPublishWritesAuditRow(t *testing.T) {
	chdirTemp(t)

	audit := &stubAudit{}
	res, err := Publish(context.Background(), validTestTemplate(), nil,
		"2026-08", nil, "tc-9", []int64{5}, "monthly-report", audit)
	require.NoError(t, err)
	require.Len(t, audit.rows, 1)
	row := audit.rows[0]
	assert.Equal(t, "2026-08", row.Period)
	assert.Equal(t, "monthly-report", row.SkillID)
	assert.Equal(t, 1, row.Version)
	assert.Equal(t, "tc-9", row.ApprovalAuditID)
	assert.Equal(t, "[5]", row.KPICallIDs)
	assert.Equal(t, res.Digest, row.ResultDigest)
	assert.Equal(t, res.TemplateVersion, row.TemplateVersion)
	assert.FileExists(t, row.FilePath)
}

// TestPublishRequiresAuditSink — CR-04 fail-closed half.
func TestPublishRequiresAuditSink(t *testing.T) {
	chdirTemp(t)

	res, err := Publish(context.Background(), validTestTemplate(), nil,
		"2026-08", nil, "tc-1", nil, "monthly-report", nil)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "audit sink not wired")
}
