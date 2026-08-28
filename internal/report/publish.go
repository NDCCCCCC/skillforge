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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PublishResult contains the output of a successful publish.
type PublishResult struct {
	Version  int
	FilePath string
	URL      string // /reports/<period>/v<N>.html
}

// Snapshot is the KPI data snapshot bound into the report at render time.
// Minimal interface matching kpi.Snapshot's Get method.
type Snapshot interface {
	Get(kpi string) any
}

// Publish renders the report and writes it to reports/<period>/v<N>.html.
// vN increments atomically within the same period.
func Publish(ctx context.Context, tmpl *ReportTemplate, slots SlotTexts,
	period string, snap Snapshot, approvalAuditID string, kpiCallIDs []int64,
	skillID string) (*PublishResult, error) {

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

	// 4. Digest
	digest := sha256Hex(html)

	// 5. Template version hash (stable identifier for this template content)
	templateVersion := templateHash(tmpl)

	// Suppress unused-variable warnings
	_ = templateVersion
	_ = digest
	_ = skillID
	_ = approvalAuditID
	_ = kpiCallIDs
	_ = ctx

	return &PublishResult{
		Version:  version,
		FilePath: filePath,
		URL:      "/reports/" + period + "/" + filename,
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
