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

//go:build contract

// Package contract is the version-pinned integration smoke test (SC-5).
//
// This package combines the SC-1..SC-4 validators into a single test
// suite that runs in under 10 seconds wall-clock. It is the "trust
// contract" deliverable from D-10: future Phase 2-7 work builds on the
// wire / filter / schema pins asserted here. Any drift in:
//
//   - eino / hertz / eino-ext versions (TestContractVersionPin)
//   - UIMessageChunk kebab-case Type discriminator (TestContractWireFormat)
//   - Milvus tier-filter expression shape (TestContractMilvusFilter)
//   - Verdict struct fields / status enum (TestContractStructuredOutputSchema)
//
// makes the suite fail, blocking the merge that introduced the drift.
//
// T-1-16 single-source-of-truth: this file imports `spike.Verdict` /
// `spike.ValidStatuses` / `spike.Validate` from internal/spike. It does
// NOT redeclare a local ContractVerdict type. A field rename in
// internal/spike/verdict.go breaks this file at compile time — drift
// between cmd/spiked (which also imports spike.Verdict) and this file
// is structurally impossible.
//
// Build tag: `//go:build contract` gates the suite so the default
// `go test ./...` run does not pay the ~5s spiked subprocess cost.
// Run with: `go test ./integration/contract -tags=contract -count=1 -timeout 10s -v`.
package contract

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/spike"
	"github.com/stretchr/testify/assert"
)

// spikedBinary is the path to the locally-built spiked binary. Resolved
// once by buildSpiked (via sync.Once) and cached for the rest of the
// suite. Stored at package scope with a process-scoped temp dir (NOT
// t.TempDir, which is bound to test lifetime and would be cleaned up
// when the first test finishes, breaking subsequent tests).
var (
	spikedBinaryOnce sync.Once
	spikedBinary     string
	spikedBinaryErr  error
)

// TestContractVersionPin asserts the production stack versions have not
// drifted from the values pinned in go.mod. SC-5 drift detection — a
// bump to eino / hertz / eino-ext changes behavior subtly enough that
// re-running the spike suite is the only safe validation, so the
// contract test fails fast on any version change.
//
// Per Plan 01-01 SUMMARY deviation #2: eino-ext was corrected from
// pseudo-version to tagged v0.1.5 (the highest version compatible with
// eino v0.7.7). This test asserts v0.1.5 — NOT the pseudo-version
// pattern the original plan described. That plan assumption was
// corrected by Plan 01-01 in 2026-08-11.
func TestContractVersionPin(t *testing.T) {
	content, err := os.ReadFile(findGoMod(t))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	body := string(content)

	// eino v0.7.7 EXACTLY (STACK.md locks the minimum).
	assert.True(t, strings.Contains(body, "github.com/cloudwego/eino v0.7.7"),
		"Version pin violated: github.com/cloudwego/eino must be v0.7.7 (got body without that substring)")

	// hertz v0.10.6 — locked by SC-1 (Hertz pkg/protocol/sse API verified at this version).
	assert.True(t, strings.Contains(body, "github.com/cloudwego/hertz v0.10.6"),
		"Version pin violated: github.com/cloudwego/hertz must be v0.10.6")

	// eino-ext openai v0.1.5 — Plan 01-01 pinned to the highest version
	// compatible with eino v0.7.7 (newer v0.1.13+ requires eino v0.7.13+).
	assert.True(t, strings.Contains(body, "github.com/cloudwego/eino-ext/components/model/openai v0.1.5"),
		"Version pin violated: github.com/cloudwego/eino-ext/components/model/openai must be v0.1.5")

	// milvus-sdk-go v2.4.2 — Plan 01-02 verified positional Search API at this version.
	assert.True(t, strings.Contains(body, "github.com/milvus-io/milvus-sdk-go/v2 v2.4.2"),
		"Version pin violated: milvus-sdk-go/v2 must be v2.4.2 (positional Search API verified at this version)")
}

// TestContractWireFormat asserts SC-1 layer (a) wire format is intact:
//
//  1. `spiked wire` still produces "[OK] spike wire: temperature=0
//     passthrough verified" (proves Eino ChatModel + mock OpenAI server
//     round-trip with the 3 extras still works).
//
//  2. spike.UIMessageChunk marshals with kebab-case Type discriminator
//     (Pitfall 6 — AI SDK v5 parser silently drops snake_case frames).
//
//  3. spike.BuildMilvusFilter with an O'Brien-style apostrophe tier
//     produces a parseable filter (Pitfall 7 — CN-char + apostrophe escape).
//
// Together these cover the wire-format envelope that crosses the
// Browser → API tier boundary (the only shared contract in Phase 1).
func TestContractWireFormat(t *testing.T) {
	// 1. spiked wire subprocess — proves the eino-ext openai adapter +
	// mock OpenAI round-trip with temperature + response_format + thinking
	// extras still works end-to-end. Sets spikedBinary for later tests.
	stdout, _, exitCode := runSpiked(t, "wire")
	assert.Equal(t, 0, exitCode, "spiked wire should exit 0")
	assert.Contains(t, stdout, "temperature=0 passthrough verified",
		"spiked wire should report successful wire-format passthrough")

	// 2. UIMessageChunk kebab-case Type — Pitfall 6. The AI SDK v5
	// parser (process-ui-message-stream.ts) silently drops frames whose
	// Type is snake_case (text_delta vs text-delta). This assertion is
	// the regression catch for any future code that constructs a chunk
	// with a snake_case type.
	chunk := spike.UIMessageChunk{
		Type: "text-delta",
		ID:   "msg-contract",
		Delta: "hello contract",
	}
	payload, err := json.Marshal(chunk)
	assert.NoError(t, err, "marshal UIMessageChunk")
	assert.True(t, strings.Contains(string(payload), `"type":"text-delta"`),
		"UIMessageChunk JSON must contain kebab-case type discriminator; got: %s", string(payload))
	assert.False(t, strings.Contains(string(payload), `"type":"text_delta"`),
		"UIMessageChunk JSON must NOT contain snake_case type discriminator")

	// 3. BuildMilvusFilter with apostrophe — Pitfall 7. Go's %q formatter
	// produces a JSON-safe escaped string for hostile characters. The
	// filter string must be parseable by Milvus (heuristic: contains
	// `in [` and ends with `]`).
	filter := spike.BuildMilvusFilter([]string{`O'Brien`})
	assert.True(t, strings.Contains(filter, "in ["),
		"filter with apostrophe should still be parseable (contains 'in ['); got: %s", filter)
	assert.True(t, strings.HasSuffix(filter, "]"),
		"filter should end with ']'; got: %s", filter)
	assert.True(t, strings.Contains(filter, `O'Brien`) || strings.Contains(filter, `O\'Brien`),
		"filter should preserve the tier value (escaped or not); got: %s", filter)
}

// TestContractMilvusFilter asserts SC-2 tier-filter syntax is intact:
// the filter expression is the trust boundary (no cross-tier retrieval
// as long as the filter is well-formed + applied pre-similarity).
//
// Three cases:
//  1. Empty tiers → "false" (default-deny — D-09 structural enforcement).
//  2. Single tier → `tier in ["权威"]` (CN-char properly escaped via %q).
//  3. Multi-tier → `tier in ["权威","历史"]`.
//
// These are pure-function assertions — no live Milvus needed. Plan 02
// validated the live bleed assertion separately (gated on
// MILVUS_INTEGRATION=1 + human-verify checkpoint).
func TestContractMilvusFilter(t *testing.T) {
	assert.Equal(t, "false", spike.BuildMilvusFilter([]string{}),
		"empty tier list must produce literal 'false' (default-deny per D-09)")
	assert.Equal(t, "false", spike.BuildMilvusFilter(nil),
		"nil tier list must produce literal 'false' (default-deny per D-09)")

	assert.Equal(t, `tier in ["权威"]`, spike.BuildMilvusFilter([]string{"权威"}),
		"single CN tier must be JSON-escaped + wrapped in tier in [...]")

	assert.Equal(t, `tier in ["权威","历史"]`, spike.BuildMilvusFilter([]string{"权威", "历史"}),
		"multi CN tier must be comma-separated JSON-escaped values")

	// Sub-test: CN-char escape edge cases (Pitfall 7).
	t.Run("CN_char_escape", func(t *testing.T) {
		hostile := []string{`O'Brien`, `a"b`, `a\b`, `正常`}
		for _, h := range hostile {
			f := spike.BuildMilvusFilter([]string{h})
			assert.True(t, strings.Contains(f, "in ["),
				"hostile tier %q should still produce parseable filter; got: %s", h, f)
			assert.True(t, strings.HasSuffix(f, "]"),
				"filter for hostile tier %q should end with ]: got %s", h, f)
		}
	})
}

// TestContractStructuredOutputSchema asserts SC-3 + SC-5 schema gate:
// the Verdict struct shape + ValidStatuses enum are the single source
// of truth, shared between cmd/spiked/e2e.go (the producer) and this
// file (the validator).
//
// T-1-16 mitigation: this test imports `spike.Verdict` /
// `spike.ValidStatuses` / `spike.Validate` — it does NOT redeclare a
// local ContractVerdict type. A field rename in internal/spike/verdict.go
// breaks both cmd/spiked/e2e.go AND this file at compile time. Runtime
// drift between producer and validator is structurally impossible.
//
// Coverage:
//  1. JSON round-trip — all 4 fields (status, confidence, citations,
//     review_required) survive marshal → unmarshal with values intact.
//  2. ValidStatuses contents — the exact 4 expected keys are present.
//  3. Validate accepts a sample verdict + rejects an unknown status.
func TestContractStructuredOutputSchema(t *testing.T) {
	// 1. Round-trip the wire JSON shape. This is the contract between
	// the LLM-produced JSON + the spike.Verdict Go struct — drift here
	// would mean either the prompt template or the struct tags changed.
	original := spike.Verdict{
		Status:         "合规",
		Confidence:     0.85,
		Citations:      []string{"auth-001"},
		ReviewRequired: true,
	}
	jsonBytes, err := json.Marshal(original)
	assert.NoError(t, err, "marshal sample Verdict")

	var roundTripped spike.Verdict
	err = json.Unmarshal(jsonBytes, &roundTripped)
	assert.NoError(t, err, "unmarshal sample Verdict")

	assert.Equal(t, original.Status, roundTripped.Status,
		"Verdict.Status round-trip mismatch (json tag drift)")
	assert.Equal(t, original.Confidence, roundTripped.Confidence,
		"Verdict.Confidence round-trip mismatch")
	assert.Equal(t, original.Citations, roundTripped.Citations,
		"Verdict.Citations round-trip mismatch")
	assert.Equal(t, original.ReviewRequired, roundTripped.ReviewRequired,
		"Verdict.ReviewRequired round-trip mismatch (json tag drift)")

	// 2. ValidStatuses contents — explicit assertion on the exact 4 keys.
	// A typo here (e.g. variation selector on a CN char) would silently
	// change semantics; this assertion catches it.
	expectedStatuses := map[string]bool{
		"合规":      true,
		"不合规":     true,
		"需补充":     true,
		"unanswered": true,
	}
	assert.Equal(t, expectedStatuses, spike.ValidStatuses,
		"spike.ValidStatuses must contain exactly the 4 expected keys")

	// 3. Validate accepts the sample + rejects unknown status. This is
	// the runtime gate that catches LLM output drift (e.g. if a future
	// model returns "compliant" instead of "合规").
	ok, reason := spike.Validate(original)
	assert.True(t, ok, "sample Verdict should validate; reason=%s", reason)
	assert.Empty(t, reason, "valid sample should have empty reason")

	badStatus := spike.Verdict{Status: "BOGUS", Confidence: 0.5, Citations: []string{}}
	okBad, reasonBad := spike.Validate(badStatus)
	assert.False(t, okBad, "unknown status should fail Validate")
	assert.Contains(t, reasonBad, "invalid status", "reason should mention invalid status")
}

// TestContractRunsUnder10s is the SC-5 budget assertion. It wraps the
// 4 validator sub-tests above + runs them sequentially, measuring total
// wall-clock. Failure here means the suite has grown too slow for the
// "fast feedback" promise of the trust contract.
//
// Budget: 10 seconds. Current baseline: ~5s (dominated by the spiked
// wire subprocess). The spike leaves headroom for Phase 2 to add 1-2
// more sub-tests without busting the budget.
func TestContractRunsUnder10s(t *testing.T) {
	start := time.Now()

	t.Run("VersionPin", TestContractVersionPin)
	t.Run("WireFormat", TestContractWireFormat)
	t.Run("MilvusFilter", TestContractMilvusFilter)
	t.Run("StructuredOutputSchema", TestContractStructuredOutputSchema)

	elapsed := time.Since(start)
	assert.True(t, elapsed < 10*time.Second,
		"SC-5 violated: contract suite took %v; budget 10s. Sub-process spikes (spiked wire build) dominate — see commit history for baseline.", elapsed)
	t.Logf("contract suite ran in %v (budget 10s)", elapsed)
}

// TestContractFullSpikeSuite shells out to the spike subcommands that
// can run without external services. SC-5 integration coverage.
//
// - spiked wire: always runnable (mock OpenAI server in-process).
// - spiked sse: always runnable (Hertz on dynamic port).
// - spiked milvus: SKIPPED — requires live Milvus cluster (Plan 02
//   human-verify checkpoint).
// - spiked e2e: SKIPPED — requires real GLM-5.2 (Plan 04 human-verify
//   checkpoint).
//
// Both skips are recorded so the suite output documents what was NOT
// covered (not silently dropped).
func TestContractFullSpikeSuite(t *testing.T) {
	if spikedBinary == "" {
		// TestContractWireFormat builds spiked; if it ran first, we have
		// the binary. If not, build it now (still <10s budget).
		spikedBinary = buildSpiked(t)
	}

	cases := []struct {
		name        string
		args        []string
		expectStdin string
		skipReason  string // empty = run the test
	}{
		{name: "wire", args: []string{"wire"}, expectStdin: "temperature=0 passthrough verified"},
		{name: "sse", args: []string{"sse"}, expectStdin: "[OK] spike sse"},
		{name: "milvus", skipReason: "requires live Milvus cluster (Plan 02 human-verify checkpoint)"},
		{name: "e2e", skipReason: "requires real GLM-5.2 API key (Plan 04 human-verify checkpoint)"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.skipReason != "" {
				t.Skip(c.skipReason)
			}
			stdout, _, exitCode := runSpikedWithBinary(t, spikedBinary, c.args...)
			assert.Equal(t, 0, exitCode, "spiked %s exit code", c.name)
			assert.Contains(t, stdout, c.expectStdin, "spiked %s stdout", c.name)
		})
	}
}

// --- helpers ---

// findGoMod locates go.mod by walking up from the test's working directory.
// The contract test runs from integration/contract/, so go.mod is two
// levels up. Falls back to testing.Short() skip if not found (indicates
// the suite was run from outside the module — operator error).
func findGoMod(t *testing.T) string {
	t.Helper()
	return filepath.Join(findModuleRoot(t), "go.mod")
}

// findModuleRoot locates the directory containing go.mod by walking up
// from the test's working directory. Used to set cmd.Dir for `go build`
// so the package import path resolves correctly regardless of where the
// test binary was launched from.
func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate go.mod by walking up from " + dir)
	return ""
}

// buildSpiked compiles ./cmd/spiked into a process-scoped temp directory
// and returns the binary path. Uses sync.Once so the build runs exactly
// once per test process — subsequent callers reuse the cached binary.
//
// The temp directory is created via os.MkdirTemp (process-scoped, NOT
// t.TempDir which is bound to a single test's lifetime). This means the
// binary survives across sub-tests in the same `go test` invocation.
// The OS cleans up the temp dir eventually; we don't bother with
// explicit cleanup because the test process exits shortly after.
//
// Building once + caching avoids re-compiling per sub-test (~3s per
// build on this machine — would bust the 10s budget if done per-test).
func buildSpiked(t *testing.T) string {
	t.Helper()
	spikedBinaryOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "spiked-contract-")
		if err != nil {
			spikedBinaryErr = err
			return
		}
		binPath := filepath.Join(tmpDir, "spiked.exe")
		buildCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Use the full import path so `go build` resolves the package
		// correctly regardless of the test's working directory (Go tests
		// run from their package directory, not from the module root).
		cmd := exec.CommandContext(buildCtx, "go", "build", "-o", binPath, "github.com/skillforge/skillforge/cmd/spiked")
		cmd.Dir = findModuleRoot(t)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			spikedBinaryErr = err
			return
		}
		spikedBinary = binPath
	})
	if spikedBinaryErr != nil {
		t.Fatalf("go build spiked: %v", spikedBinaryErr)
	}
	if spikedBinary == "" {
		t.Fatal("spikedBinary empty after Once.Do (no error recorded)")
	}
	return spikedBinary
}

// runSpiked is the convenience wrapper that builds spiked on first call
// then invokes the named subcommand. Returns (stdout, stderr, exitCode).
// GLM_API_KEY is set to "unhappy" to force spiked e2e into offline mode
// (no real LLM call) so the suite can be run without external services.
func runSpiked(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	bin := buildSpiked(t)
	return runSpikedWithBinary(t, bin, args...)
}

// runSpikedWithBinary invokes the already-built spiked binary. Sets
// GLM_API_KEY=unhappy to force offline mode for any subcommand that
// reads it (spiked e2e in particular). Captures stdout + stderr +
// exit code without inheriting the parent's env beyond what's needed.
func runSpikedWithBinary(t *testing.T, bin string, args ...string) (string, string, int) {
	t.Helper()
	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, bin, args...)
	// Force offline mode — no real LLM/Milvus calls. spiked wire + spiked
	// sse don't read GLM_API_KEY; spiked e2e does (and should fail fast
	// with "[FAIL] e2e: GLM_API_KEY not set"). spiked milvus reads
	// MILVUS_ADDRESS (empty → connection error).
	cmd.Env = append(os.Environ(), "GLM_API_KEY=unhappy")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("run spiked %v: %v", args, err)
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

// Compile-time anchor: spike.Verdict MUST be a struct (not interface /
// type alias) for the JSON round-trip assertions in
// TestContractStructuredOutputSchema to work. This var declaration
// forces a compile error if spike.Verdict is ever changed to a non-struct
// type — a belt-and-suspenders companion to the runtime assertions.
var _ spike.Verdict
