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
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// SlotTexts maps section index + slot index to LLM-generated text.
// SectionIdx -> SlotIdx -> text
type SlotTexts map[int]map[int]string

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.CJK),
	// Deliberately omitting html.WithUnsafe() — raw HTML is escaped by goldmark
	// as the first defense layer.
)

var policy = sync.OnceValue(func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowElements("section", "table", "thead", "tbody", "tr", "th", "td")
	p.AllowTables()
	return p
})

// Render executes the sandcastle pipeline: bind → goldmark → sanitise.
// Order is STRUCTURALLY LOCKED — the bind step produces markdown, goldmark
// converts to raw HTML, and bluemonday sanitises the final output.
// D18-A3: CheckNarrativeConsistency is called at exit; mismatches added
// as visible markdown blockquote markers (non-blocking, "标记" semantics).
func Render(tmpl *ReportTemplate, slots SlotTexts, snap Snapshot, fmtFn ValueFormatter) ([]byte, error) {
	body := buildMarkdown(tmpl, slots, snap, fmtFn)

	// D18-A3: Check narrative number consistency (non-blocking markers).
	// WR-04: the previous HTML-comment marker was stripped by goldmark
	// (raw HTML omitted) and bluemonday (no AllowComments). Blockquote
	// markers survive both (GFM renders them to <blockquote> which
	// UGCPolicy allows) AND they are visible to the operator — the
	// stated "标记" semantics.
	consistency := CheckNarrativeConsistency(tmpl, slots, snap, fmtFn)
	if consistency != nil && !consistency.Clean {
		body = appendConsistencyMarkers(body, consistency)
	}

	// goldmark: markdown → raw HTML (with GFM + CJK extensions)
	var buf bytes.Buffer
	if err := md.Convert([]byte(body), &buf); err != nil {
		return nil, fmt.Errorf("report: goldmark convert: %w", err)
	}

	// bluemonday: sanitize raw HTML (always last in the pipeline)
	return policy().SanitizeBytes(buf.Bytes()), nil
}

// appendConsistencyMarkers appends markdown blockquote lines marking
// narrative inconsistencies. WR-04: only markdown that survives the
// pipeline can reach the published HTML — HTML comments were stripped
// by goldmark + bluemonday (the previous markers were never emitted).
func appendConsistencyMarkers(body string, report *ConsistencyReport) string {
	for _, m := range report.Mismatches {
		marker := fmt.Sprintf(
			"\n\n> ⚠️ 一致性提示：第 %d 节第 %d 槽 narrative 数值 %s 与 KPI 快照 %s 不一致（容差 ±0.5%%）\n\n",
			m.SectionIdx, m.SlotIdx, m.ActualValue, m.ExpectedValue)
		body += marker
	}
	return body
}

// buildMarkdown assembles the markdown body from template + slot texts + KPI values.
// KPI values are formatted by fmtFn before insertion to prevent injection.
func buildMarkdown(tmpl *ReportTemplate, slots SlotTexts, snap Snapshot, fmtFn ValueFormatter) string {
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(tmpl.Title)
	sb.WriteString("\n\n")

	for si, sec := range tmpl.Sections {
		sb.WriteString("## ")
		sb.WriteString(sec.Title)
		sb.WriteString("\n")

		// KPI table. CR-06 fail-safe: a nil snapshot skips the table
		// instead of panicking on the nil-interface Get call (the HITL
		// tool refuses KPI-declaring templates without data; this guard
		// keeps Render total for any future caller).
		if len(sec.KPIs) > 0 && snap != nil {
			sb.WriteString("| 指标 | 值 |\n|---|---|\n")
			for _, kpi := range sec.KPIs {
				val := snap.Get(kpi)
				display := fmtFn(val)
				sb.WriteString("| ")
				sb.WriteString(mdCellEscaper.Replace(kpi))
				sb.WriteString(" | ")
				sb.WriteString(display)
				sb.WriteString(" |\n")
			}
			sb.WriteString("\n")
		}

		// Slot texts
		if slotMap, ok := slots[si]; ok {
			for slotIdx, text := range slotMap {
				if slotIdx < len(sec.Slots) {
					slotType := sec.Slots[slotIdx].Type
					sb.WriteString("### ")
					sb.WriteString(slotType)
					sb.WriteString("\n\n")
					sb.WriteString(text)
					sb.WriteString("\n\n")
				}
			}
		}
	}
	return sb.String()
}
