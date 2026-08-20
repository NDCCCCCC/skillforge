// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// SPDX-License-Identifier: Apache-2.0

// main_test.go is Plan 12-04 / Task 4's spike-deep hermetic verification
// gate. The live RF-5 measurement is operator-runs-live (P12-L10 spike:
// requires GLM_API_KEY + BGE_M3_URL + MILVUS_ADDRESS); these tests
// prove the SPIKE INFRASTRUCTURE works WITHOUT a live stack.
//
// Coverage:
//
//   - TestSpikeDeepDryRun            — --dry-run exits 0 with "50 cases
//     loaded" message; proves the
//     golden load + schema validation
//     pipeline works end-to-end.
//   - TestSpikeDeepMissingEnv        — live mode without GLM_API_KEY
//     exits 1 with a clear error
//     naming the missing env vars.
//   - TestSpikeDeepGoldenSetSchema   — eval/golden/deep.jsonl carries
//     exactly 50 cases distributed
//     per the plan's contract
//     (25 multi-hop + 15 plan-style
//   - 10 refusal-required).
//   - TestSpikeDeepVerdictParseRateMeasurement
//     — SKELETON: skipped unless the
//     operator sets GLM_API_KEY. When
//     unskipped, asserts the
//     P12-L10 escalation floor
//     (direct parse rate >= 70%);
//     below 70% the test FAILS with
//     "RF-5 lock: structured-output
//     final step is mandatory".
//
// The dry-run + missing-env tests build the binary via `go build` into
// t.TempDir() so the test does NOT depend on `go run`'s hot-build
// behaviour; the schema test reads the golden file directly.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// exeSuffix returns the platform's executable suffix (".exe" on
// Windows; "" elsewhere). Go's `go build -o name` produces `name`
// verbatim on Unix and `name.exe` on Windows — exec.Command on
// Windows refuses to start a file without the .exe suffix.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// repoRoot returns the absolute path of the SkillForge repo root by
// walking up from this test file's directory until go.mod is found.
// `go test ./cmd/spike-deep/...` runs the test process with cwd =
// cmd/spike-deep, so relative paths to the repo's eval/ + skills/
// must be resolved against the repo root, not the package directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dir := filepath.Dir(file) // cmd/spike-deep
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not locate repo root (go.mod) walking up from %s", file)
	return ""
}

// buildSpikeDeepBinary builds the spike-deep command into dir/ and
// returns the absolute binary path. Mirrors the standard
// "go build into t.TempDir" pattern used by other operator-gated
// spike tests in the repo. The build cwd is the package directory
// (cmd/spike-deep); `go build -o bin .` resolves the local package
// against that cwd — the same resolution `go run ./cmd/spike-deep`
// uses.
func buildSpikeDeepBinary(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "spike-deep"+exeSuffix())
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(repoRoot(t), "cmd", "spike-deep")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build spike-deep: %v\n%s", err, out)
	}
	return bin
}

// runSpikeDeep executes bin with args, returning (exitCode, stdout,
// stderr). The cmd.Dir is the repo root so relative --golden /
// --output paths resolve against the repo (the spike's defaults
// assume the repo root as cwd, mirroring cmd/eval + cmd/spike-history).
func runSpikeDeep(t *testing.T, bin string, env []string, args ...string) (int, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("spike-deep %v timed out: stdout=%s stderr=%s", args, stdout.String(), stderr.String())
	}
	exit := 0
	if ee, ok := err.(*exec.ExitError); ok {
		exit = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("spike-deep %v: unexpected error: %v (stdout=%s stderr=%s)", args, err, stdout.String(), stderr.String())
	}
	return exit, stdout.String(), stderr.String()
}

// TestSpikeDeepDryRun — --dry-run exits 0 with the "50 cases loaded"
// message; the schema validation + golden loader work end-to-end
// without any env vars.
func TestSpikeDeepDryRun(t *testing.T) {
	bin := buildSpikeDeepBinary(t, t.TempDir())

	// No env-set: dry-run MUST NOT require GLM_API_KEY / BGE_M3_URL
	// / MILVUS_ADDRESS. Explicitly clear them so the test runs in a
	// hermetic CI lane without leaking operator secrets.
	env := []string{
		"GLM_API_KEY=",
		"BGE_M3_URL=",
		"MILVUS_ADDRESS=",
	}

	exit, stdout, stderr := runSpikeDeep(t, bin, env, "parse-rate", "--dry-run", "--golden=eval/golden/deep.jsonl")
	if exit != 0 {
		t.Fatalf("--dry-run exit %d (want 0); stderr=%s stdout=%s", exit, stderr, stdout)
	}
	// log.Printf writes to stderr — check the combined stream so the
	// assertions hold regardless of log routing.
	combined := stdout + stderr
	// The [OK] line with the case count is the verification gate's
	// primary signal — it must surface the plan-locked "50 cases".
	if !strings.Contains(combined, "50 cases loaded") {
		t.Errorf("--dry-run output missing '50 cases loaded' message:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	// And it must name the golden file the test fed it (proves the
	// --golden flag is wired, not silently ignored).
	if !strings.Contains(combined, "eval/golden/deep.jsonl") {
		t.Errorf("--dry-run output missing the golden path:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
}

// TestSpikeDeepMissingEnv — live mode (no --dry-run) without
// GLM_API_KEY / BGE_M3_URL / MILVUS_ADDRESS exits 1 with a clear
// error naming the missing vars. The spike is operator-gated; a
// silent success on a half-configured machine would corrupt the
// measurement.
func TestSpikeDeepMissingEnv(t *testing.T) {
	bin := buildSpikeDeepBinary(t, t.TempDir())

	// Clear all three required env vars; live mode MUST fail.
	env := []string{
		"GLM_API_KEY=",
		"BGE_M3_URL=",
		"MILVUS_ADDRESS=",
	}

	exit, stdout, stderr := runSpikeDeep(t, bin, env, "parse-rate", "--golden=eval/golden/deep.jsonl")
	if exit == 0 {
		t.Fatalf("live mode without env vars must exit non-zero; got 0 (stdout=%s stderr=%s)", stdout, stderr)
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "GLM_API_KEY") {
		t.Errorf("error message missing GLM_API_KEY:\n%s", combined)
	}
	if !strings.Contains(combined, "BGE_M3_URL") {
		t.Errorf("error message missing BGE_M3_URL:\n%s", combined)
	}
	if !strings.Contains(combined, "MILVUS_ADDRESS") {
		t.Errorf("error message missing MILVUS_ADDRESS:\n%s", combined)
	}
	// The error message must also point at the operator-actionable
	// remediation (dry-run as the escape hatch).
	if !strings.Contains(combined, "--dry-run") {
		t.Errorf("error message missing the --dry-run remediation hint:\n%s", combined)
	}
}

// TestSpikeDeepGoldenSetSchema — eval/golden/deep.jsonl carries
// exactly 50 cases distributed per the plan's contract. The schema
// validation lives in the spike binary itself (loadDeepGolden +
// validateCase); this test asserts the file's shape from the
// outside, so a refactor that accidentally bypasses the loader's
// checks still gets caught.
func TestSpikeDeepGoldenSetSchema(t *testing.T) {
	goldenPath := filepath.Join(repoRoot(t), "eval", "golden", "deep.jsonl")
	f, err := os.Open(goldenPath)
	if err != nil {
		t.Fatalf("open %s: %v", goldenPath, err)
	}
	defer f.Close()

	type rawCase struct {
		ID                 string         `json:"id"`
		Category           string         `json:"category"`
		Query              string         `json:"query"`
		Skill              string         `json:"skill"`
		GroundTruth        map[string]any `json:"groundTruth"`
		Difficulty         string         `json:"difficulty"`
		RequiresWriteTodos bool           `json:"requiresWriteTodos,omitempty"`
		ExpectedPlanSteps  int            `json:"expectedPlanSteps,omitempty"`
	}

	byCat := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	total := 0
	seenIDs := map[string]bool{}
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		var c rawCase
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			t.Fatalf("line %d decode: %v", total+1, err)
		}
		if c.ID == "" {
			t.Errorf("line %d missing id", total+1)
		}
		if seenIDs[c.ID] {
			t.Errorf("duplicate id %q at line %d", c.ID, total+1)
		}
		seenIDs[c.ID] = true
		if c.Query == "" {
			t.Errorf("line %d (%s) missing query", total+1, c.ID)
		}
		if c.Skill == "" {
			t.Errorf("line %d (%s) missing skill", total+1, c.ID)
		}
		if len(c.GroundTruth) == 0 {
			t.Errorf("line %d (%s) missing groundTruth", total+1, c.ID)
		}
		switch c.Category {
		case "multi-hop", "plan-style", "refusal-required":
			byCat[c.Category]++
		default:
			t.Errorf("line %d (%s) unknown category %q", total+1, c.ID, c.Category)
		}
		total++
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	// 50 cases distributed per the plan's contract.
	if total != 50 {
		t.Errorf("total cases = %d, want 50", total)
	}
	if byCat["multi-hop"] != 25 {
		t.Errorf("multi-hop cases = %d, want 25", byCat["multi-hop"])
	}
	if byCat["plan-style"] != 15 {
		t.Errorf("plan-style cases = %d, want 15", byCat["plan-style"])
	}
	if byCat["refusal-required"] != 10 {
		t.Errorf("refusal-required cases = %d, want 10", byCat["refusal-required"])
	}
}

// TestSpikeDeepVerdictParseRateMeasurement — SKELETON live test. By
// default the GLM_API_KEY env is unset, so the test SKIPS — CI must
// never depend on a live stack. When the operator sets GLM_API_KEY
// (and BGE_M3_URL + MILVUS_ADDRESS), the test runs the live parse-rate
// measurement and asserts the P12-L10 escalation floor:
// direct_rate >= 0.70. Below 70% the test FAILS with "RF-5 lock:
// structured-output final step is mandatory" (the threshold that
// 12-08 acceptance gate escalates to).
//
// The operator runs:
//
//	GLM_API_KEY=... BGE_M3_URL=... MILVUS_ADDRESS=... \
//	  go test ./cmd/spike-deep/ -run TestSpikeDeepVerdictParseRateMeasurement -v
func TestSpikeDeepVerdictParseRateMeasurement(t *testing.T) {
	if os.Getenv("GLM_API_KEY") == "" {
		t.Skip("RF-5 live spike requires GLM_API_KEY (operator gate; P12-L10 — 12-08 acceptance gate reports the actual rate)")
	}
	if os.Getenv("BGE_M3_URL") == "" {
		t.Skip("RF-5 live spike requires BGE_M3_URL (operator gate)")
	}
	if os.Getenv("MILVUS_ADDRESS") == "" {
		t.Skip("RF-5 live spike requires MILVUS_ADDRESS (operator gate)")
	}

	bin := buildSpikeDeepBinary(t, t.TempDir())
	// --output inside t.TempDir so the test does NOT write to the
	// repo's reports/ directory (T-9-RESULT-LOSS guard).
	out := filepath.Join(t.TempDir(), "rf5-deep.jsonl")
	env := []string{
		"GLM_API_KEY=" + os.Getenv("GLM_API_KEY"),
		"BGE_M3_URL=" + os.Getenv("BGE_M3_URL"),
		"MILVUS_ADDRESS=" + os.Getenv("MILVUS_ADDRESS"),
	}
	exit, stdout, stderr := runSpikeDeep(t, bin, env,
		"parse-rate", "--golden=eval/golden/deep.jsonl", "--output="+out, "--cost-limit=10.00")
	if exit != 0 {
		t.Fatalf("live parse-rate exit %d; stderr=%s stdout=%s", exit, stderr, stdout)
	}

	// Parse the summary row from the JSONL (last line) — the
	// assertion target is the direct_rate field.
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output %q: %v", out, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		t.Fatalf("output JSONL has %d lines, want at least 2 (cases + summary)", len(lines))
	}
	var sum parseRateRow
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &sum); err != nil {
		t.Fatalf("unmarshal summary row: %v", err)
	}
	if sum.Type != "summary" {
		t.Fatalf("last row type = %q, want summary", sum.Type)
	}
	// P12-L10 escalation floor: direct parse rate MUST be >= 70%.
	// Below 70% triggers the verdict-suffix + forced_stop escalation
	// (out of scope for 12-04 — the spike measures, 12-08 reports).
	if sum.DirectRate < 0.70 {
		t.Fatalf("RF-5 lock: direct_rate=%.3f < 0.70 (P12-L10 escalation floor) — verdict-suffix fallback + forced_stop required. Full summary: %+v", sum.DirectRate, sum)
	}
}
