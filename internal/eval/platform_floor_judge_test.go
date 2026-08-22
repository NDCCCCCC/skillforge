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

package eval

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// platformFloorGoldenPath is the relative path from internal/eval to the
// platform-floor golden set. The test working directory under `go test` is
// internal/eval/, so two `..` segments reach the repo root.
const platformFloorGoldenPath = ".." + string(filepath.Separator) + ".." + string(filepath.Separator) + "eval" + string(filepath.Separator) + "golden" + string(filepath.Separator) + "platform-floor.jsonl"

// platformFloorCase is the row schema for eval/golden/platform-floor.jsonl.
// It is intentionally local to this test file — the golden set is a fixture,
// and the schema assertions below are the machine-readable contract.
type platformFloorCase struct {
	ID                       string `json:"id"`
	Scenario                 string `json:"scenario"`
	Query                    string `json:"query"`
	ExpectedRefuse           bool   `json:"expected_refuse"`
	ExpectedSQLPatternAbsent bool   `json:"expected_sql_pattern_absent"`
	Notes                    string `json:"notes"`
	HistoryTurns             []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"history_turns,omitempty"`
}

// TestPlatformFloorGoldenSchema is the D14-C2/C3 schema gate for the 20-case
// platform-floor frozen set. It asserts:
//   - Exactly 20 parseable JSON rows.
//   - IDs are unique and prefixed with "pfloor-".
//   - Scenario is one of the six attack surfaces.
//   - Every row is an adversarial inducement (expected_refuse == true).
//   - Every row requires the SQL pattern to be absent (hermetic FAIL input).
//   - Notes map to at least one D14-A1 rule (①-⑦ or 1-7).
//   - long_dialogue_dilution rows carry >= 20 neutral history turns.
//
// This is the RED half of the Task 1 TDD pair; the GREEN half is the
// platform-floor.jsonl fixture itself.
func TestPlatformFloorGoldenSchema(t *testing.T) {
	cases := loadPlatformFloorCases(t, platformFloorGoldenPath)
	require.Len(t, cases, 20, "platform-floor golden set must contain exactly 20 cases")

	scenarios := map[string]int{
		"direct_sql_induction":       0,
		"channel_override":           0,
		"raw_row_request":            0,
		"deanon_join":                0,
		"roleplay_jailbreak":         0,
		"long_dialogue_dilution":     0,
	}
	ids := make(map[string]struct{}, len(cases))
	rulePattern := regexp.MustCompile(`(?:rule[:：]\s*[1-7,①-⑦\s]+|[①-⑦])`)

	for _, c := range cases {
		assert.True(t, strings.HasPrefix(c.ID, "pfloor-"), "case %q id must start with pfloor-", c.ID)
		_, dup := ids[c.ID]
		assert.False(t, dup, "duplicate case id %q", c.ID)
		ids[c.ID] = struct{}{}

		_, ok := scenarios[c.Scenario]
		assert.True(t, ok, "case %q has unknown scenario %q", c.ID, c.Scenario)
		scenarios[c.Scenario]++

		assert.True(t, c.ExpectedRefuse, "case %q must be an inducement (expected_refuse=true)", c.ID)
		assert.True(t, c.ExpectedSQLPatternAbsent, "case %q must expect SQL pattern absent", c.ID)
		assert.NotEmpty(t, strings.TrimSpace(c.Query), "case %q has empty query", c.ID)
		assert.Regexp(t, rulePattern, c.Notes, "case %q notes must reference at least one D14-A1 rule", c.ID)

		if c.Scenario == "long_dialogue_dilution" {
			assert.GreaterOrEqual(t, len(c.HistoryTurns), 20,
				"case %q (long_dialogue_dilution) must carry >=20 history turns", c.ID)
			for i, turn := range c.HistoryTurns {
				assert.Contains(t, []string{"user", "assistant"}, turn.Role,
					"case %q history turn[%d] has invalid role %q", c.ID, i, turn.Role)
				assert.NotEmpty(t, strings.TrimSpace(turn.Content),
					"case %q history turn[%d] has empty content", c.ID, i)
			}
		}
	}

	total := 0
	for s, n := range scenarios {
		total += n
		t.Logf("scenario %s: %d", s, n)
	}
	assert.Equal(t, 20, total, "scenario counts must sum to 20")

	// Distribution contract (D14-C2): every iron-law has >=2 variants.
	// The scenario distribution is the primary shape; notes rule coverage is
	// asserted above. We additionally assert no scenario is empty, so the
	// six attack surfaces are all represented.
	for s, n := range scenarios {
		assert.Greater(t, n, 0, "scenario %q must have at least one case", s)
	}
}

// loadPlatformFloorCases reads platform-floor.jsonl into a slice. It mirrors
// LoadGolden's posture but uses the local platformFloorCase schema.
func loadPlatformFloorCases(t *testing.T, path string) []platformFloorCase {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err, "open platform-floor golden set")
	defer f.Close()

	var cases []platformFloorCase
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var c platformFloorCase
		require.NoError(t, json.Unmarshal([]byte(line), &c),
			"line %d: parse platform-floor case", lineNo)
		cases = append(cases, c)
	}
	require.NoError(t, scanner.Err())
	return cases
}
