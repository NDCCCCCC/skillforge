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

// LoadKPIs loads and validates all KPI definition files in dir.
// Implementation in 16-01 Task 3.
func LoadKPIs(dir string) (*Registry, Signature, error) {
	// TODO: 16-01 Task 3 — WalkDir *.yaml, per-file 5-stage pipeline
	// (Lstat symlink guard → ReadFile → sha256 → yaml.Unmarshal → ValidateKPIFile),
	// fail-closed whole-directory, compile Registry.
	return nil, "", nil
}
