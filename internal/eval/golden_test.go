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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoldenCategoryCoverage is the GEVL-05 regression gate. It loads
// the production golden set (eval/golden/compliance.jsonl) and asserts:
//
//   - Exactly 50 cases total.
//   - Exactly 20 adversarial + 20 refusal-required + 10 multi-hop.
//   - The 5 spike-anchor cases are present (reused from
//     scripts/spiked/e2e.go:76-82 for cross-phase comparability).
//   - Every case has a non-empty query + category.
//
// Pinned by 03-VALIDATION.md. If this test fails, the golden set has
// drifted from the GEVL-05 contract — fix the JSONL before re-running
// the eval CLI.
//
// NOTE: the test uses a relative path (../../eval/golden/compliance.jsonl)
// because internal/eval's test working directory is internal/eval/. The
// path is stable under `go test` (which does not chdir).
func TestGoldenCategoryCoverage(t *testing.T) {
	path := filepath.Join("..", "..", "eval", "golden", "compliance.jsonl")
	cases, err := LoadGolden(path)
	require.NoError(t, err, "load production golden set")
	require.Len(t, cases, 50, "GEVL-05 contract: exactly 50 cases")

	counts := map[string]int{
		"adversarial":      0,
		"refusal-required": 0,
		"multi-hop":        0,
	}
	spikeAnchors := 0
	ids := make(map[string]struct{}, len(cases))
	for _, c := range cases {
		assert.NotEmpty(t, c.Query, "case %s has empty query", c.ID)
		assert.NotEmpty(t, c.Category, "case %s has empty category", c.ID)
		assert.Contains(t, counts, c.Category, "case %s has unknown category %q", c.ID, c.Category)
		counts[c.Category]++
		_, dup := ids[c.ID]
		assert.False(t, dup, "duplicate case id %q", c.ID)
		ids[c.ID] = struct{}{}
		// The "source" field is optional metadata; LoadGolden does not
		// surface it on Case (the field is not in the struct). We
		// detect spike-anchors by re-decoding the raw line here.
	}

	// Re-scan the raw file to count spike-anchor tags (the Case struct
	// does not carry the "source" field — it's metadata, not a metric
	// input).
	spikeAnchors = countSpikeAnchors(t, path)
	assert.Equal(t, 5, spikeAnchors, "expected 5 spike-anchor cases (scripts/spiked/e2e.go:76-82)")

	assert.Equal(t, 20, counts["adversarial"],
		"GEVL-05: exactly 20 adversarial cases (got %d)", counts["adversarial"])
	assert.Equal(t, 20, counts["refusal-required"],
		"GEVL-05: exactly 20 refusal-required cases (got %d)", counts["refusal-required"])
	assert.Equal(t, 10, counts["multi-hop"],
		"GEVL-05: exactly 10 multi-hop cases (got %d)", counts["multi-hop"])
}

// countSpikeAnchors re-reads the golden file and counts lines tagged
// "source":"spike-anchor". Used because the Case struct intentionally
// does not surface the "source" metadata field (it is not a metric
// input). The count is asserted by TestGoldenCategoryCoverage.
func countSpikeAnchors(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var probe struct {
			Source string `json:"source"`
		}
		if err := json.Unmarshal([]byte(line), &probe); err == nil && probe.Source == "spike-anchor" {
			n++
		}
	}
	require.NoError(t, scanner.Err())
	return n
}
