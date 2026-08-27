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

// Plan 16-08 / KPI-01 — the admin read-only view of the KPI aggregation
// channel: what the registry compiled at startup, plus when each indicator
// was last actually invoked (from the kpi_calls audit table).
//
// READ-ONLY BY CONSTRUCTION (KPI-01 / Pitfall 14). KPI definitions live in
// controlled files under SKILLFORGE_KPIS_DIR and are loaded exactly once at
// startup into an immutable registry (D16-A1 / D16-A4). Any HTTP surface that
// could create, edit, or remove a KPI would be a registration backdoor —
// the very thing the controlled-file design exists to prevent. This file
// therefore contains a single retrieval handler and no mutation verb; the
// route table registers it for retrieval only, and the companion test
// source-greps this file to keep it that way.
//
// Trust model:
//   - X-Admin-Token gate via isAdminRequest (same crypto/subtle compare as
//     GET /api/audit and GET /api/admin/platform-prompt; unset env → 403
//     for every caller).
//   - No caller input reaches SQL: the registry read is in-memory and the
//     audit read is a fixed aggregate query with no parameters.
//   - Fail-closed when the KPI pipeline is not wired (nil registry → 503),
//     mirroring the SKILLFORGE_DATA_SALT-unset import posture.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

// kpiPipelineNotWiredError is the 503 body message when Deps.KPIRegistry is
// nil — the server booted without a KPI registry, so there is nothing
// truthful to report (fail-closed rather than an empty-looking 200).
const kpiPipelineNotWiredError = "KPI pipeline not wired (server started without a KPI registry)"

// kpiCatalogEntry is one registry entry as rendered for the admin view.
type kpiCatalogEntry struct {
	// Name is the registry key — the exact string kpi_call accepts.
	Name string `json:"name"`
	// Description is the human-readable explanation from the controlled file.
	Description string `json:"description"`
	// ParamsSummary is the one-line parameter summary shown in the tool
	// catalog (e.g. "status:enum(2), period:pattern").
	ParamsSummary string `json:"params_summary"`
	// Dataset is the logical dataset the KPI reads.
	Dataset string `json:"dataset"`
}

// kpiAdminResponse is the 200 envelope for the admin KPI view.
type kpiAdminResponse struct {
	// Signature is the registry content hash (D16-A4) — the same value every
	// kpi_calls audit row carries, so an operator can tell which definition
	// version produced a historical answer.
	Signature string `json:"signature"`
	// KPIs is the sorted registry catalog.
	KPIs []kpiCatalogEntry `json:"kpis"`
	// LatestCalls maps KPI name → RFC3339 timestamp of its most recent
	// invocation (from the kpi_calls audit table). KPIs never called are
	// absent from the map, which is exactly the operator signal wanted
	// (e.g. TestSameKPIAcrossSkills triage: defined but never reached).
	LatestCalls map[string]string `json:"latest_calls"`
}

// kpiAdminHandler serves GET /api/admin/kpis.
//
// Response envelope (200):
//
//	{
//	  "signature":    "9f1c…",
//	  "kpis":         [{"name":…,"description":…,"params_summary":…,"dataset":…}],
//	  "latest_calls": {"loan_overdue_rate_by_status": "2026-08-27T03:11:00Z"}
//	}
//
// Errors: 403 (admin token missing/wrong/unset), 503 (KPI pipeline not
// wired), 500 (audit read failure — the registry half is never guessed at).
func (s *Deps) kpiAdminHandler(ctx context.Context, c *app.RequestContext) {
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, map[string]string{
			"error": "admin token required",
		})
		return
	}
	if s.KPIRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kpiPipelineNotWiredError})
		return
	}

	entries := s.KPIRegistry.Catalog()
	kpis := make([]kpiCatalogEntry, 0, len(entries))
	for _, e := range entries {
		dataset := ""
		if def, ok := s.KPIRegistry.Lookup(e.Name); ok {
			dataset = def.Dataset
		}
		kpis = append(kpis, kpiCatalogEntry{
			Name:          e.Name,
			Description:   e.Description,
			ParamsSummary: e.ParamsSummary,
			Dataset:       dataset,
		})
	}

	// The audit half is best-effort ONLY in the sense that a nil Store means
	// "no audit backend configured" (degraded mode, same as elsewhere in this
	// package). A real read failure is surfaced as 500 rather than silently
	// rendering an empty map, which an operator would misread as "never
	// called".
	latest := make(map[string]string)
	if s.Store != nil {
		byFunc, err := s.Store.LatestKPICallByFunc(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, graphResponseError{
				Error: "read kpi call audit: " + err.Error(),
			})
			return
		}
		for name, ts := range byFunc {
			latest[name] = ts.UTC().Format(time.RFC3339)
		}
	}

	c.JSON(http.StatusOK, kpiAdminResponse{
		Signature:   s.KPIRegistry.Signature(),
		KPIs:        kpis,
		LatestCalls: latest,
	})
}
