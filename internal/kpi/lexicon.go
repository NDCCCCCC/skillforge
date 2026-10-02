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

package kpi

import "regexp"

// KPINamePattern returns the lexical grammar for KPI names and dataset
// identifiers — the canonical regex ^([A-Za-z][A-Za-z0-9_]{0,63})$.
// The returned *regexp.Regexp is the SAME instance the loader uses
// internally (validate_text.go kpiNamePattern); exporting it lets the
// WSL test layer (internal/kpi/wsl) lock the cross-package lexical
// contract against duckdb.ValidateDatasetID without copying the regex
// (TestNameLexiconParity, 16-06 Task 3). Drift between this accessor
// and the loader's internal pattern is what the test catches.
func KPINamePattern() *regexp.Regexp {
	return kpiNamePattern
}
