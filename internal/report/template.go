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

// Package report implements the monthly report rendering and publication pipeline.
// D18-A4: template.go defines the ReportTemplate/SectionTemplate/Slot types
// (mirroring the YAML schema in monthly-report SKILL.md), plus validation.
package report

import (
	"fmt"
	"slices"
	"strings"
)

// ReportTemplate is the template for a monthly report. It mirrors the
// template: YAML block in the monthly-report SKILL.md.
type ReportTemplate struct {
	Title    string            `yaml:"title"    json:"title"`
	Sections []SectionTemplate `yaml:"sections" json:"sections"`
}

// SectionTemplate describes one section of a report: title, KPI names, and slots.
type SectionTemplate struct {
	Title string   `yaml:"title" json:"title"`
	KPIs  []string `yaml:"kpis"  json:"kpis"`
	Slots []Slot   `yaml:"slots" json:"slots"`
}

// Slot describes one narrative slot in a section: type is one of
// narrative | insight | risk_note.
type Slot struct {
	Type string `yaml:"type" json:"type"`
}

// Validate checks static template properties. KPI name cross-checking against
// the KPI registry is performed by skill.ValidateAgainstKPI in loader.go.
func (t *ReportTemplate) Validate() error {
	if len(t.Sections) == 0 {
		return fmt.Errorf("template.sections must be non-empty (D18-A4)")
	}
	for i, sec := range t.Sections {
		if strings.TrimSpace(sec.Title) == "" {
			return fmt.Errorf("template.sections[%d].title is empty (D18-A4)", i)
		}
		for j, slot := range sec.Slots {
			if !slices.Contains([]string{"narrative", "insight", "risk_note"}, slot.Type) {
				return fmt.Errorf(
					"template.sections[%d].slots[%d].type %q not in {narrative, insight, risk_note} (D18-A2)",
					i, j, slot.Type,
				)
			}
		}
	}
	return nil
}
