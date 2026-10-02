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

// Plan 06-02 / SC-1 + D6-13 — server-side tests for the
// /api/eval/* endpoints.
//
// Test families:
//
//   1. TestEvalReports_NilReporterReturns503 / TestEvalFrozen_Nil...
//      — Deps.EvalReporter == nil → 503 (mirrors knowledge / wizard).
//   2. TestEvalReports_Index — GET /api/eval/reports serves
//      EvalIndex JSON; missing INDEX.json is mapped to {"runs":[]}
//      (06-01 note: LoadIndex's os.IsNotExist error → empty index
//      200, NOT a 500).
//   3. TestEvalReports_ReportByTs — GET /api/eval/reports/:ts
//      serves the report; :ts regex-validated `^[0-9T:-]+Z$`
//      BEFORE filesystem access (T-6-PATH, ASVS V5).
//   4. TestEvalReports_PathTraversal — :ts='../etc/passwd',
//      '..\etc\passwd', 'subdir/file.json' all → 400.
//   5. TestEvalReports_InvalidTs — :ts='foo-bar' → 400.
//   6. TestEvalReports_NotFound — :ts valid format but no
//      matching report → 404.
//   7. TestEvalFrozen_List — GET /api/eval/frozen returns the
//      frozen-set list + lock metadata.
//   8. TestEvalReports_RouteRegistered — /api/eval/* never 404s.
//   9. TestEvalReports_TsRegexSourceCheck — defense-in-depth
//      grep on eval.go to pin the regex shape.

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/eval"
)

// stubEvalReporter is the test double for EvalReporter. Each
// method returns canned data; tests assert the handler's wire
// shape and the per-method dispatch.
type stubEvalReporter struct {
	indexFn     func() (eval.EvalIndex, error)
	reportFn    func(ts string) (eval.Report, error)
	frozenFn    func() ([]FrozenSetInfo, error)
	indexCalls  int
	reportCalls int
	frozenCalls int
}

func (s *stubEvalReporter) Index() (eval.EvalIndex, error) {
	s.indexCalls++
	if s.indexFn == nil {
		return eval.EvalIndex{Version: eval.IndexVersion, Runs: []eval.IndexRun{}}, nil
	}
	return s.indexFn()
}

func (s *stubEvalReporter) Report(ts string) (eval.Report, error) {
	s.reportCalls++
	if s.reportFn == nil {
		return eval.Report{}, errors.New("stub: Report not configured")
	}
	return s.reportFn(ts)
}

func (s *stubEvalReporter) FrozenSets() ([]FrozenSetInfo, error) {
	s.frozenCalls++
	if s.frozenFn == nil {
		return nil, nil
	}
	return s.frozenFn()
}

// Compile-time check that stubEvalReporter implements EvalReporter.
var _ EvalReporter = (*stubEvalReporter)(nil)

// newEvalHandlerHertz builds a fresh hertz instance with all
// three /api/eval/* routes registered.
func newEvalHandlerHertz(t *testing.T, deps *Deps) *server.Hertz {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	h.GET("/api/eval/reports", deps.evalIndexHandler)
	h.GET("/api/eval/reports/:ts", deps.evalReportHandler)
	h.GET("/api/eval/frozen", deps.evalFrozenHandler)
	return h
}

// invokeEvalReports fires GET /api/eval/reports (the index
// endpoint).
func invokeEvalReports(t *testing.T, deps *Deps) *ut.ResponseRecorder {
	t.Helper()
	h := newEvalHandlerHertz(t, deps)
	return ut.PerformRequest(h.Engine, http.MethodGet, "/api/eval/reports", nil)
}

// invokeEvalReport fires GET /api/eval/reports/<ts>. :ts is the
// raw path segment — pass "../etc/passwd" verbatim, etc.
func invokeEvalReport(t *testing.T, deps *Deps, ts string) *ut.ResponseRecorder {
	t.Helper()
	h := newEvalHandlerHertz(t, deps)
	return ut.PerformRequest(h.Engine, http.MethodGet, "/api/eval/reports/"+ts, nil)
}

// invokeEvalFrozen fires GET /api/eval/frozen.
func invokeEvalFrozen(t *testing.T, deps *Deps) *ut.ResponseRecorder {
	t.Helper()
	h := newEvalHandlerHertz(t, deps)
	return ut.PerformRequest(h.Engine, http.MethodGet, "/api/eval/frozen", nil)
}

// --- /api/eval/reports ---

// TestEvalReports_NilReporterReturns503 — Deps.EvalReporter ==
// nil → 503 with the standard error envelope.
func TestEvalReports_NilReporterReturns503(t *testing.T) {
	deps := &Deps{EvalReporter: nil}
	got := invokeEvalReports(t, deps)
	assert.Equal(t, http.StatusServiceUnavailable, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	assert.Contains(t, body.Error, "eval reporter not initialized")
}

// TestEvalReports_Index — happy path: EvalIndex JSON returned
// with shape {version, runs: [...]}.
func TestEvalReports_Index(t *testing.T) {
	stub := &stubEvalReporter{
		indexFn: func() (eval.EvalIndex, error) {
			return eval.EvalIndex{
				Version: eval.IndexVersion,
				Runs: []eval.IndexRun{{
					TS: "2026-08-13T10-05-21Z", Skill: "compliance",
					JudgeModel: "glm-5.2", Faithfulness: 0.92,
					ContextPrecision: 0.85, AnswerRelevance: 0.88,
					ContextRecall: 0.79, RefusalAccuracy: 0.95,
					FrozenVersion: "compliance-2026-08-13-v1",
					ReportPath:    "2026-08-13T10-05-21Z.json",
				}},
			}, nil
		},
	}
	deps := &Deps{EvalReporter: stub}

	got := invokeEvalReports(t, deps)
	assert.Equal(t, http.StatusOK, got.Code)
	var index eval.EvalIndex
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &index))
	assert.Equal(t, eval.IndexVersion, index.Version)
	require.Len(t, index.Runs, 1)
	assert.Equal(t, "compliance", index.Runs[0].Skill)
	assert.Equal(t, 1, stub.indexCalls, "Index invoked exactly once")
}

// TestEvalReports_Index_EmptyWhenMissing — when the reporter
// returns os.IsNotExist-equivalent, the handler maps it to an
// empty 200 response (NOT a 500). 06-01's executor flagged this
// explicitly: the dashboard's first-load case is "no history
// yet".
func TestEvalReports_Index_EmptyWhenMissing(t *testing.T) {
	stub := &stubEvalReporter{
		indexFn: func() (eval.EvalIndex, error) {
			return eval.EvalIndex{}, &os.PathError{Op: "open", Path: "eval/reports/INDEX.json", Err: os.ErrNotExist}
		},
	}
	deps := &Deps{EvalReporter: stub}

	got := invokeEvalReports(t, deps)
	assert.Equal(t, http.StatusOK, got.Code, "missing INDEX.json must NOT 500")
	var index eval.EvalIndex
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &index))
	assert.Equal(t, eval.IndexVersion, index.Version)
	assert.Empty(t, index.Runs, "empty index on missing INDEX.json")
}

// TestEvalReports_Index_OtherErrorReturns500 — non-NotExist
// errors surface as 500 generic (W-3).
func TestEvalReports_Index_OtherErrorReturns500(t *testing.T) {
	stub := &stubEvalReporter{
		indexFn: func() (eval.EvalIndex, error) {
			return eval.EvalIndex{}, errors.New("boom")
		},
	}
	deps := &Deps{EvalReporter: stub}

	got := invokeEvalReports(t, deps)
	assert.Equal(t, http.StatusInternalServerError, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	assert.NotContains(t, body.Error, "boom", "W-3: err.Error() must NOT leak to client")
}

// --- /api/eval/reports/:ts ---

// TestEvalReports_NilReporterReportReturns503 — :ts handler
// honors the 503 posture when EvalReporter is nil.
func TestEvalReports_NilReporterReportReturns503(t *testing.T) {
	deps := &Deps{EvalReporter: nil}
	got := invokeEvalReport(t, deps, "2026-08-13T10-05-21Z")
	assert.Equal(t, http.StatusServiceUnavailable, got.Code)
}

// TestEvalReports_ReportByTs — happy path: a valid ts returns
// the report JSON.
func TestEvalReports_ReportByTs(t *testing.T) {
	stub := &stubEvalReporter{
		reportFn: func(ts string) (eval.Report, error) {
			assert.Equal(t, "2026-08-13T10-05-21Z", ts, "handler must pass the URL segment verbatim")
			return eval.Report{
				Skill: "compliance",
				Overall: map[string]float64{
					"faithfulness":      0.92,
					"context_precision": 0.85,
					"answer_relevance":  0.88,
					"context_recall":    0.79,
				},
			}, nil
		},
	}
	deps := &Deps{EvalReporter: stub}

	got := invokeEvalReport(t, deps, "2026-08-13T10-05-21Z")
	assert.Equal(t, http.StatusOK, got.Code)
	var r eval.Report
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &r))
	assert.Equal(t, "compliance", r.Skill)
	assert.InDelta(t, 0.92, r.Overall["faithfulness"], 1e-9)
	assert.Equal(t, 1, stub.reportCalls)
}

// TestEvalReports_PathTraversal — :ts with /, \, or .. is
// REJECTED before any filesystem read (T-6-PATH / ASVS V5).
//
// The rejection layer is layered: hertz's router rejects paths
// with literal `/` (or URL-encoded `%2F`) at the routing stage
// (returning 404 — the path doesn't match /api/eval/reports/:ts
// because hertz treats `/` as a segment boundary), and the
// handler's `validTimestamp` regex rejects everything else
// (returning 400). Both are valid rejections per the security
// goal; what matters is that EvalReporter.Report is never
// invoked.
//
// `..\\etc\\passwd` (raw `..\etc\passwd`) and `..` are special:
// hertz's URL parser may strip the `..` segment, returning 404
// without invoking the handler. Other inputs that reach the
// handler (e.g. `subdir/file.json` may be routed, depending on
// the path) are 400'd by the regex.
func TestEvalReports_PathTraversal(t *testing.T) {
	stub := &stubEvalReporter{
		reportFn: func(ts string) (eval.Report, error) {
			t.Fatalf("Report MUST NOT be invoked for invalid :ts (got %q)", ts)
			return eval.Report{}, nil
		},
	}
	deps := &Deps{EvalReporter: stub}

	// The full set of bad inputs the spec demands we reject.
	// `..` is the path-traversal attack vector (ASVS V5).
	bad := []string{
		"..",
		"../etc/passwd",
		"..\\etc\\passwd",
		"subdir/file.json",
		"a/b",
		"a%2Fb",
	}
	for _, b := range bad {
		t.Run(b, func(t *testing.T) {
			got := invokeEvalReport(t, deps, b)
			// Either 400 (handler regex) or 404 (router) is a
			// valid rejection. What MUST NOT happen is 200 — a
			// successful response would mean the file was read.
			assert.NotEqual(t, http.StatusOK, got.Code, "bad :ts %q must be rejected, got 200", b)
			assert.True(t, got.Code == http.StatusBadRequest || got.Code == http.StatusNotFound,
				"bad :ts %q must be 400 or 404, got %d", b, got.Code)
		})
	}
}

// TestEvalReports_InvalidTs — :ts that fails the regex (not
// just path-separator) → 400. Note: an empty :ts is a router
// concern (hertz may or may not match it); we exclude it from
// the strict 400 assertion because the security goal is
// "rejected", not "specific status code".
func TestEvalReports_InvalidTs(t *testing.T) {
	stub := &stubEvalReporter{}
	deps := &Deps{EvalReporter: stub}

	// The "" case is a routing concern (hertz may 404 on the
	// empty path), not a handler concern. We exclude it from
	// the strict 400 list but still require "not 200".
	cases := []struct {
		input    string
		wantCode int // 0 = any non-200
	}{
		{"foo-bar", http.StatusBadRequest},
		{"2026-08-13T10:05:21Z", http.StatusBadRequest}, // colons illegal
		{"2026-08-13", http.StatusBadRequest},           // no Z suffix
		{"2026-08-13T10-05-21", http.StatusBadRequest},  // no Z suffix
		{"2026-08-13T10-05-21z", http.StatusBadRequest}, // lowercase z
	}
	for _, c := range cases {
		t.Run("bad="+c.input, func(t *testing.T) {
			got := invokeEvalReport(t, deps, c.input)
			if c.wantCode == 0 {
				assert.NotEqual(t, http.StatusOK, got.Code, "bad :ts %q must be rejected", c.input)
			} else {
				assert.Equal(t, c.wantCode, got.Code, "bad :ts %q", c.input)
			}
		})
	}
}

// TestEvalReports_NotFound — :ts passes the regex but no
// matching report exists → 404.
func TestEvalReports_NotFound(t *testing.T) {
	stub := &stubEvalReporter{
		reportFn: func(ts string) (eval.Report, error) {
			return eval.Report{}, os.ErrNotExist
		},
	}
	deps := &Deps{EvalReporter: stub}

	got := invokeEvalReport(t, deps, "2026-08-13T10-05-21Z")
	assert.Equal(t, http.StatusNotFound, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	assert.Equal(t, "report not found", body.Error)
}

// TestEvalReports_OtherErrorReturns500 — non-NotExist errors
// surface as 500 generic.
func TestEvalReports_OtherErrorReturns500(t *testing.T) {
	stub := &stubEvalReporter{
		reportFn: func(ts string) (eval.Report, error) {
			return eval.Report{}, errors.New("disk failure")
		},
	}
	deps := &Deps{EvalReporter: stub}

	got := invokeEvalReport(t, deps, "2026-08-13T10-05-21Z")
	assert.Equal(t, http.StatusInternalServerError, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	assert.NotContains(t, body.Error, "disk failure")
}

// --- /api/eval/frozen ---

// TestEvalFrozen_NilReporterReturns503 — nil seam → 503.
func TestEvalFrozen_NilReporterReturns503(t *testing.T) {
	deps := &Deps{EvalReporter: nil}
	got := invokeEvalFrozen(t, deps)
	assert.Equal(t, http.StatusServiceUnavailable, got.Code)
}

// TestEvalFrozen_List — returns the frozen-set list with each
// entry carrying Skill + Path + Lock metadata.
func TestEvalFrozen_List(t *testing.T) {
	stub := &stubEvalReporter{
		frozenFn: func() ([]FrozenSetInfo, error) {
			return []FrozenSetInfo{{
				Skill: "compliance",
				Path:  "eval/frozen/compliance.jsonl",
				Lock: eval.LockFile{
					ModelVersion:   "glm-5.2",
					PromptHash:     "deadbeef",
					Seed:           42,
					EvalSetVersion: "compliance-2026-08-13-v1",
					SkillName:      "compliance",
				},
			}}, nil
		},
	}
	deps := &Deps{EvalReporter: stub}

	got := invokeEvalFrozen(t, deps)
	assert.Equal(t, http.StatusOK, got.Code)
	var sets []FrozenSetInfo
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &sets))
	require.Len(t, sets, 1)
	assert.Equal(t, "compliance", sets[0].Skill)
	assert.Equal(t, "eval/frozen/compliance.jsonl", sets[0].Path)
	assert.Equal(t, "glm-5.2", sets[0].Lock.ModelVersion)
	assert.Equal(t, int64(42), sets[0].Lock.Seed)
	assert.Equal(t, 1, stub.frozenCalls)
}

// TestEvalFrozen_Empty — no frozen sets is not an error;
// returns an empty list (200).
func TestEvalFrozen_Empty(t *testing.T) {
	stub := &stubEvalReporter{
		frozenFn: func() ([]FrozenSetInfo, error) {
			return nil, nil
		},
	}
	deps := &Deps{EvalReporter: stub}

	got := invokeEvalFrozen(t, deps)
	assert.Equal(t, http.StatusOK, got.Code)
	var sets []FrozenSetInfo
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &sets))
	assert.Empty(t, sets)
}

// --- Route registration ---

// TestEvalReports_RouteRegistered — the /api/eval/* routes
// MUST never 404. Mirrors TestGraphHandler_RouteRegistered.
func TestEvalReports_RouteRegistered(t *testing.T) {
	stub := &stubEvalReporter{}
	deps := &Deps{EvalReporter: stub}
	h := New(":0", deps)

	for _, path := range []string{
		"/api/eval/reports",
		"/api/eval/frozen",
	} {
		w := ut.PerformRequest(h.Engine, http.MethodGet, path, nil)
		assert.NotEqual(t, http.StatusNotFound, w.Code,
			"GET %s must be registered (got 404)", path)
	}
}

// TestEvalReports_TsRegexSourceCheck — T-6-PATH: the regex
// ^[0-9T:-]+Z$ must be a top-level package var named
// validTimestamp so this grep finds it. A future refactor
// that inlines the regex and forgets the boundaries is
// caught here.
func TestEvalReports_TsRegexSourceCheck(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must succeed in a test")
	srcPath := filepath.Join(filepath.Dir(thisFile), "eval.go")
	src, err := os.ReadFile(srcPath)
	require.NoError(t, err, "read eval.go at %s", srcPath)
	body := string(src)
	assert.Contains(t, body, "validTimestamp",
		"eval.go must export validTimestamp as a top-level var (defense-in-depth)")
	assert.Contains(t, body, "^[0-9T-]+Z$",
		"validTimestamp regex must anchor on ^[0-9T-]+Z$ (T-6-PATH)")
}

// Keep the strings import live across refactors (the source-check
// test currently uses it; this is a tripwire).
var _ = strings.Contains
