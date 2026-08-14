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

// Plan 06-02 / SC-1 + D6-13 — the /api/eval/* HTTP surface.
//
// Wire contract:
//
//	GET /api/eval/reports
//	    → 200 {version, runs: [...]}     EvalIndex JSON
//	    → 200 {version, runs: []}        empty index on missing file
//	    → 500 {error: "..."}             other load failure (W-3)
//	    → 503 {error: "..."}             Deps.EvalReporter == nil
//
//	GET /api/eval/reports/:ts
//	    → 200 {skill, overall, ...}      single report
//	    → 400 {error: "..."}             bad :ts (path traversal / format)
//	    → 404 {error: "..."}             no matching report
//	    → 500 {error: "..."}             other load failure
//	    → 503 {error: "..."}             Deps.EvalReporter == nil
//
//	GET /api/eval/frozen
//	    → 200 [{skill, path, lock: {...}}]  frozen-set list + lock metadata
//	    → 503 {error: "..."}                  Deps.EvalReporter == nil
//
// Threat model (06-02 threat_model):
//
//   - T-6-PATH — :ts is regex-validated `^[0-9T:-]+Z$` BEFORE any
//     filesystem access. The regex rejects /, \, ., and all
//     non-timestamp characters implicitly. A `..` in :ts is the
//     path-traversal attack vector (ASVS V5). The reporter
//     implementation (filesystem-backed) MUST NOT add a path
//     prefix that escapes the eval/reports/ directory; the
//     regex gate is the load-bearing defense.
//
// Design choice — EvalReporter seam (RESEARCH Pattern 4): the
// handlers do NOT read eval/reports/ directly off the disk. They
// depend on an EvalReporter interface (mirrors KnowledgeBuilder
// on Deps); the production wiring at cmd/server/main.go
// constructs a filesystem-backed impl, tests inject a stub.
// This keeps the handler testable without standing up a real
// filesystem layout, and it puts the path-traversal defense in
// one place (the regex here, NOT scattered across the impl).

package server

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/eval"
)

// validTimestamp is the strict regex for the :ts path segment
// on /api/eval/reports/:ts. The format mirrors
// eval.ArchiveTimestampLayout (YYYY-MM-DDTHH-MM-SSZ) — hyphens
// in the time portion because colons are illegal in Windows
// filenames. The regex intentionally EXCLUDES `:` from the
// character class so a payload like "2026-08-13T10:05:21Z" (with
// real colons) is rejected at the boundary rather than
// surfacing as a 500 from the filesystem layer.
//
// Why a top-level package var (instead of inlining): the
// eval_test.go source-check (TestEvalReports_TsRegexSourceCheck)
// greps the source for `validTimestamp` and the exact regex
// string as a defense-in-depth against a future refactor that
// drops the boundaries and accidentally matches a
// path-traversal payload.
var validTimestamp = regexp.MustCompile(`^[0-9T-]+Z$`)

// EvalReporter is the seam the /api/eval/* handlers depend on
// (RESEARCH Pattern 4). Mirrors KnowledgeBuilder's pattern: an
// interface in this package, a filesystem-backed implementation
// constructed in cmd/server/main.go, an in-memory stub for
// handler tests.
//
// Index reads eval/reports/INDEX.json (the trend manifest).
//
// Report reads one eval/reports/<ts>.json by timestamp. The
// timestamp is supplied by the URL path segment and is
// pre-validated by the regex above; the implementation MUST NOT
// perform additional path joining (the :ts is the full
// filename stem).
//
// FrozenSets scans eval/frozen/*.lock.json sidecars and returns
// each Skill's frozen set path + the lock fields the dashboard
// renders (model_ver, prompt_hash, seed, eval_set_version,
// skill_name).
type EvalReporter interface {
	Index() (eval.EvalIndex, error)
	Report(ts string) (eval.Report, error)
	FrozenSets() ([]FrozenSetInfo, error)
}

// FrozenSetInfo is the JSON shape of one row in the
// /api/eval/frozen response. The dashboard renders Path as a
// clickable link + Lock as the D6-16 baseline overlay anchor.
type FrozenSetInfo struct {
	Skill string        `json:"skill"`
	Path  string        `json:"path"`
	Lock  eval.LockFile `json:"lock"`
}

// evalIndexHandler serves GET /api/eval/reports. The 503 / 200 /
// 500 posture mirrors the graph handler exactly (the W-3
// convention: log full err, return generic envelope).
//
// 06-01's executor note: a missing INDEX.json returns an
// os.IsNotExist-inspectable error from eval.LoadIndex. The
// handler maps that to an empty 200 ({"version": 1, "runs":
// []}), NOT a 500 — the dashboard's first-load case is "no
// history yet" and should render an empty-state, not an error
// page.
func (s *Deps) evalIndexHandler(ctx context.Context, c *app.RequestContext) {
	if s.EvalReporter == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "eval reporter not initialized (set Deps.EvalReporter at server construction)",
		})
		return
	}
	index, err := s.EvalReporter.Index()
	if err != nil {
		if isNotExist(err) {
			// First-load case: no history yet. Render an empty index.
			c.JSON(http.StatusOK, eval.EvalIndex{
				Version: eval.IndexVersion,
				Runs:    []eval.IndexRun{},
			})
			return
		}
		// W-3: log full err server-side; return generic envelope.
		log.Printf("[WARN] server: eval index load failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "eval index load failed — see server logs",
		})
		return
	}
	c.JSON(http.StatusOK, index)
}

// evalReportHandler serves GET /api/eval/reports/:ts.
//
// SECURITY: :ts is regex-validated BEFORE EvalReporter.Report is
// invoked. The regex `^[0-9T:-]+Z$` rejects:
//   - any character outside [0-9T:-]
//   - implicit path separators (/, \, .)
//   - any input that does not end in Z
//
// This is the T-6-PATH mitigation. The reporter implementation
// may further sanitize (e.g. path.Join the timestamp into a
// fixed directory), but the regex here is the load-bearing
// defense.
func (s *Deps) evalReportHandler(ctx context.Context, c *app.RequestContext) {
	if s.EvalReporter == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "eval reporter not initialized (set Deps.EvalReporter at server construction)",
		})
		return
	}
	ts := c.Param("ts")
	if !validTimestamp.MatchString(ts) {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "invalid report timestamp (expected ^[0-9T-]+Z$)",
		})
		return
	}
	report, err := s.EvalReporter.Report(ts)
	if err != nil {
		if isNotExist(err) {
			c.JSON(http.StatusNotFound, graphResponseError{
				Error: "report not found",
			})
			return
		}
		log.Printf("[WARN] server: eval report load failed for %q: %v", ts, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "eval report load failed — see server logs",
		})
		return
	}
	c.JSON(http.StatusOK, report)
}

// evalFrozenHandler serves GET /api/eval/frozen. Returns the
// frozen-set list with each entry carrying the path + lock
// metadata. The dashboard uses Lock.PromptHash to pin the
// baseline overlay (D6-16).
func (s *Deps) evalFrozenHandler(ctx context.Context, c *app.RequestContext) {
	if s.EvalReporter == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "eval reporter not initialized (set Deps.EvalReporter at server construction)",
		})
		return
	}
	sets, err := s.EvalReporter.FrozenSets()
	if err != nil {
		log.Printf("[WARN] server: eval frozen sets scan failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "eval frozen sets scan failed — see server logs",
		})
		return
	}
	if sets == nil {
		sets = []FrozenSetInfo{}
	}
	c.JSON(http.StatusOK, sets)
}

// FilesystemEvalReporter is the production EvalReporter. It
// reads eval/reports/INDEX.json + eval/reports/<ts>.json and
// scans eval/frozen/*.lock.json sidecars. The constructor takes
// the same paths cmd/eval uses; defaults match the cmd/eval
// defaults so a single `go run ./cmd/server` finds the same
// archive the CLI wrote.
//
// REPORT PATH SAFETY: Report builds the path via
// filepath.Join(reportsDir, ts+".json") and then resolves to an
// absolute path. The returned path is REJECTED if it escapes
// reportsDir (defense in depth on top of the regex — the regex
// is the load-bearing gate; this catches a refactor mistake
// that adds a Join with a value containing .. or /).
type FilesystemEvalReporter struct {
	ReportsDir string
	FrozenDir  string
}

// NewFilesystemEvalReporter returns a reporter rooted at the
// given directories. Empty strings default to "eval/reports" /
// "eval/frozen" (the cmd/eval default). MkdirAll is NOT called
// here — a missing reports dir is the "no history yet" empty
// case, and a missing frozen dir simply yields an empty
// FrozenSets().
func NewFilesystemEvalReporter(reportsDir, frozenDir string) *FilesystemEvalReporter {
	if reportsDir == "" {
		reportsDir = "eval/reports"
	}
	if frozenDir == "" {
		frozenDir = "eval/frozen"
	}
	return &FilesystemEvalReporter{ReportsDir: reportsDir, FrozenDir: frozenDir}
}

// Index reads eval/reports/INDEX.json. A missing file returns
// the os.IsNotExist-inspectable error from eval.LoadIndex; the
// handler maps it to an empty 200.
func (r *FilesystemEvalReporter) Index() (eval.EvalIndex, error) {
	return eval.LoadIndex(filepath.Join(r.ReportsDir, eval.IndexFileName))
}

// Report reads eval/reports/<ts>.json. Defense in depth on top
// of the regex: a refactor that re-introduces a path separator
// would be caught here.
func (r *FilesystemEvalReporter) Report(ts string) (eval.Report, error) {
	baseAbs, err := filepath.Abs(r.ReportsDir)
	if err != nil {
		return eval.Report{}, err
	}
	candidate := filepath.Join(baseAbs, ts+".json")
	candAbs, err := filepath.Abs(candidate)
	if err != nil {
		return eval.Report{}, err
	}
	if !strings.HasPrefix(candAbs, baseAbs+string(filepath.Separator)) && candAbs != baseAbs {
		return eval.Report{}, errors.New("eval report path escapes reports dir (defense in depth)")
	}
	return eval.LoadReport(candAbs)
}

// FrozenSets scans eval/frozen/*.lock.json sidecars. Each
// sidecar pins one frozen eval set (D6-13); the JSONL set itself
// lives next to the .lock.json with the same stem.
func (r *FilesystemEvalReporter) FrozenSets() ([]FrozenSetInfo, error) {
	matches, err := filepath.Glob(filepath.Join(r.FrozenDir, "*.lock.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	out := make([]FrozenSetInfo, 0, len(matches))
	for _, p := range matches {
		lock, err := eval.LoadLock(p)
		if err != nil {
			// A malformed .lock.json is a STOP signal, not a skip —
			// the gate machinery exists to detect drift; silently
			// skipping a lock would mean a future run could publish
			// scores against an un-pinned baseline.
			return nil, err
		}
		// The set's path is the .jsonl with the same stem as the
		// .lock.json: eval/frozen/compliance.lock.json pins
		// eval/frozen/compliance.jsonl.
		base := strings.TrimSuffix(p, ".lock.json")
		out = append(out, FrozenSetInfo{
			Skill: lock.SkillName,
			Path:  base + ".jsonl",
			Lock:  lock,
		})
	}
	return out, nil
}

// isNotExist returns true iff err is a "file not found" error
// (any of: *os.PathError with Err == ErrNotExist, fs.ErrNotExist,
// os.ErrNotExist, or a wrapped chain thereof). Used to map the
// loader's missing-file errors to the appropriate handler
// response (empty 200 for the index, 404 for an individual
// report).
func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return errors.Is(pathErr.Err, os.ErrNotExist) || errors.Is(pathErr.Err, fs.ErrNotExist)
	}
	return false
}
