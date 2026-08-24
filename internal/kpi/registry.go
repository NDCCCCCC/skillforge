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

// Registry is the immutable in-memory KPI registry.
// Implementation in 16-01 Task 3.
type Registry struct{}

// Lookup returns the compiled KPI definition by name.
// Implementation in 16-01 Task 3.
func (r *Registry) Lookup(name string) (CompiledKPI, bool) {
	return CompiledKPI{}, false
}

// Catalog returns a sorted summary of all registered KPIs.
// Implementation in 16-01 Task 3.
func (r *Registry) Catalog() []CatalogEntry {
	return nil
}

// Names returns the sorted list of all KPI names in the registry.
// Implementation in 16-01 Task 3.
func (r *Registry) Names() []string {
	return nil
}

// Signature returns the content-hash signature of the loaded KPI files.
// Implementation in 16-01 Task 3.
func (r *Registry) Signature() string {
	return ""
}

// CompiledKPI is the loader-compiled form of a KPIDef, ready for
// executor consumption. Implementation in 16-01 Task 3.
type CompiledKPI struct{}

// CatalogEntry is a summary record for the kpi_call tool catalog (D16-B1).
// Implementation in 16-01 Task 3.
type CatalogEntry struct{}

// Signature is the hex-encoded sha256 of the concatenated KPI file contents.
type Signature = string
