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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// periodRE is the CR-05 (18-REVIEW) write-side traversal guard. The
// period rides the LLM-controlled tool args and is joined into
// reports/<period>/v<N>.html; without this gate a prompt-injected
// "../x" (or an absolute / Windows-drive form) would MkdirAll +
// WriteFile OUTSIDE the reports directory. Same character class the
// GET route's reportPathAllowlist enforces per segment — defense in
// depth at BOTH the HITL tool boundary and the publish boundary.
var periodRE = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// IsValidPeriod reports whether period is safe to join into the
// reports/ directory (CR-05). Exported so the HITL tool boundary
// enforces the SAME character class the publish boundary does.
func IsValidPeriod(period string) bool { return periodRE.MatchString(period) }

// ParseTemplate converts the generic JSON-round-trip template payload
// (map[string]any — what json/yaml Unmarshal materializes from the LLM
// args and the manifest block) into the typed struct and validates it
// fail-closed (CR-01 conversion + WR-06 validation in ONE seam, so
// every caller gets the same conversion + the same rejection rules).
func ParseTemplate(template any) (*ReportTemplate, error) {
	raw, err := json.Marshal(template)
	if err != nil {
		return nil, fmt.Errorf("re-encode template: %w", err)
	}
	var tmpl ReportTemplate
	if err := json.Unmarshal(raw, &tmpl); err != nil {
		return nil, fmt.Errorf("decode template: %w", err)
	}
	if err := tmpl.Validate(); err != nil {
		return nil, fmt.Errorf("template invalid: %w", err)
	}
	return &tmpl, nil
}

// TemplateHash is the stable content hash of a template (titles + KPI
// name sets) — exported for WR-03 so the HITL tool can freeze the SAME
// identifier the reports row will carry into the operator-visible
// frozen args.
func TemplateHash(tmpl *ReportTemplate) string { return templateHash(tmpl) }

// PublishResult contains the output of a successful publish.
type PublishResult struct {
	Version  int
	FilePath string
	URL      string // /reports/<period>/v<N>.html
	// TemplateVersion is the stable content hash of the template that
	// produced this report (mirrors reports.template_version).
	TemplateVersion string
	// Digest is the SHA-256 (16-byte hex) of the rendered HTML
	// (mirrors reports.result_digest).
	Digest string
}

// Snapshot is the KPI data snapshot bound into the report at render time.
// Minimal interface matching kpi.Snapshot's Get method.
type Snapshot interface {
	Get(kpi string) any
}

// MapSnapshot adapts a plain KPI-name → value map to the Snapshot
// interface (CR-06 18-REVIEW): the HITL frozen args carry exactly this
// shape, produced by the run's kpi_call results in Pass 1.
type MapSnapshot map[string]any

// Get returns the value recorded for kpi, or nil when absent.
func (m MapSnapshot) Get(kpi string) any { return m[kpi] }

// ReportAuditSink is the narrow audit-write seam for the reports table
// (CR-04 18-REVIEW): one INSERT-only writer. store.Store implementers
// (internal/store/sqlite) satisfy it. Kept as an interface so the
// render/publish package never imports a driver.
type ReportAuditSink interface {
	PutReport(ctx context.Context, row store.ReportRow) error
}

// Publish renders the report and writes it to reports/<period>/v<N>.html.
// vN increments within the same period. After the file write succeeds
// the metadata row is appended to the reports audit table (RPT-04) via
// the ReportAuditSink — the append-only record tying the published
// artifact to its approval row + KPI call IDs (CR-04 18-REVIEW; the
// previous build discarded every audit input with `_ =`).
//
// Ordering contract (T-18-B3 posture): FILE FIRST, then the DB row. A
// failed audit write does NOT delete the file (INSERT-only trail — the
// operator can re-link it); the error carries the file path so the
// failure is recoverable, never silent.
func Publish(ctx context.Context, tmpl *ReportTemplate, slots SlotTexts,
	period string, snap Snapshot, approvalAuditID string, kpiCallIDs []int64,
	skillID string, audit ReportAuditSink) (*PublishResult, error) {

	// 0. CR-05 traversal gate — fail closed BEFORE any path use
	// (MkdirAll/WriteFile/ReadDir). The HITL tool validates too; this
	// is the publish-boundary backstop.
	if !periodRE.MatchString(period) {
		return nil, fmt.Errorf("report: invalid period %q (must match ^[A-Za-z0-9_-]+$)", period)
	}

	// 1. Render
	html, err := Render(tmpl, slots, snap, DefaultValueFormatter)
	if err != nil {
		return nil, fmt.Errorf("report: render failed: %w", err)
	}

	// 2. vN increment
	version, err := getNextVersion(period)
	if err != nil {
		return nil, fmt.Errorf("report: version increment failed: %w", err)
	}

	// 3. Write file
	dir := filepath.Join("reports", period)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("report: mkdir failed: %w", err)
	}
	filename := fmt.Sprintf("v%d.html", version)
	filePath := filepath.Join(dir, filename)

	if err := os.WriteFile(filePath, html, 0644); err != nil {
		return nil, fmt.Errorf("report: write failed: %w", err)
	}

	// 4. Digest + template version (both persisted to the audit row)
	digest := sha256Hex(html)
	templateVersion := templateHash(tmpl)

	// 5. Append-only audit row (CR-04). Fail-closed: a missing sink or
	// a failed INSERT is a publish ERROR carrying the file path — the
	// reports table must never stay silently empty while files accrue.
	if audit == nil {
		return nil, fmt.Errorf(
			"report: audit sink not wired — file %s was written but no reports row can be recorded (fail-closed, CR-04)",
			filePath)
	}
	kpiJSON, err := json.Marshal(kpiCallIDs)
	if err != nil {
		return nil, fmt.Errorf("report: marshal kpi_call_ids (file=%s): %w", filePath, err)
	}
	row := store.ReportRow{
		TS:              time.Now().UTC(),
		Period:          period,
		SkillID:         skillID,
		Version:         version,
		Title:           tmpl.Title,
		FilePath:        filePath,
		ApprovalAuditID: approvalAuditID,
		KPICallIDs:      string(kpiJSON),
		TemplateVersion: templateVersion,
		ResultDigest:    digest,
	}
	if err := audit.PutReport(ctx, row); err != nil {
		return nil, fmt.Errorf(
			"report: audit row write failed (file=%s already published; DB row missing — recoverable, do not re-render): %w",
			filePath, err)
	}

	return &PublishResult{
		Version:         version,
		FilePath:        filePath,
		URL:             "/reports/" + period + "/" + filename,
		TemplateVersion: templateVersion,
		Digest:          digest,
	}, nil
}

func getNextVersion(period string) (int, error) {
	// Scan reports/<period>/ for v<N>.html files and return MAX(N)+1.
	// Returns 1 if no existing versions.
	dir := filepath.Join("reports", period)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	maxV := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "v") || !strings.HasSuffix(name, ".html") {
			continue
		}
		vStr := strings.TrimSuffix(strings.TrimPrefix(name, "v"), ".html")
		v, err := strconv.Atoi(vStr)
		if err != nil {
			continue
		}
		if v > maxV {
			maxV = v
		}
	}
	return maxV + 1, nil
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:16])
}

func templateHash(tmpl *ReportTemplate) string {
	// Simple hash: concatenate section titles + KPI names
	var key strings.Builder
	for _, sec := range tmpl.Sections {
		key.WriteString(sec.Title)
		for _, k := range sec.KPIs {
			key.WriteString(k)
		}
	}
	h := sha256.Sum256([]byte(key.String()))
	return fmt.Sprintf("%x", h[:8])
}
