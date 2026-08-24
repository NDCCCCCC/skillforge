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

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Registry is the immutable in-memory KPI registry compiled at service startup
// (D16-A4). Once LoadKPIs returns a Registry, no code path can add, update,
// or remove entries — the interface surface exposes zero mutator methods
// (KPI-01 no-registration-backdoor structural lock).
//
// The map is unexported; reads are lock-free after construction (mirrors
// internal/agent/hitl/registry.go:76-101 "READ-ONLY after package init"
// pattern). If a future hot-reload needs to swap the registry at runtime,
// the swap is a wholesale pointer replacement (not an in-place mutation —
// out of scope for 16-01, see internal/skill/loader.go:52-78 Loader.cached
// atomic-replace pattern).
type Registry struct {
	defs  map[string]CompiledKPI // unexported: no external mutation
	order []string               // sorted KPI names for stable iteration
	sig   string                 // content-hash signature
}

// Lookup returns the compiled KPI definition by name, or false if absent.
// The returned CompiledKPI is a value copy — callers cannot mutate the
// registry's internal state (defensive copy, mirrors hitl/registry.go
// KnownTargets pattern).
func (r *Registry) Lookup(name string) (CompiledKPI, bool) {
	k, ok := r.defs[name]
	return k, ok
}

// Catalog returns a sorted summary of all registered KPIs for the kpi_call
// tool description (D16-B1). The returned slice is a defensive copy —
// callers can mutate it without affecting the registry.
func (r *Registry) Catalog() []CatalogEntry {
	out := make([]CatalogEntry, 0, len(r.order))
	for _, name := range r.order {
		k := r.defs[name]
		out = append(out, CatalogEntry{
			Name:          k.Name,
			Description:   k.Description,
			ParamsSummary: paramsSummary(k.Params),
		})
	}
	return out
}

// Names returns the sorted list of all KPI names in the registry.
// The returned slice is a defensive copy (mirrors hitl/registry.go:116-121).
func (r *Registry) Names() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Signature returns the content-hash signature of the loaded KPI files.
// The signature is the sha256 hex of the concatenated file contents
// (sorted by filename), matching the registry's atomic-replace semantics.
func (r *Registry) Signature() string {
	return r.sig
}

// CompiledKPI is the loader-compiled form of a KPIDef, ready for executor
// consumption (16-06). SQLRaw contains the :param→$param rewritten SQL
// template. Params are pre-compiled (enum sets sorted, patterns compiled
// to *regexp.Regexp) so the executor's validation closure can consume
// them directly without re-parsing.
type CompiledKPI struct {
	// Name is the KPI identifier used in kpi_call(name, params).
	Name string

	// Description is the human-readable explanation shown in the tool catalog.
	Description string

	// SQLRaw is the rewritten SQL template ($param form) ready for
	// go-duckdb named binding via sql.Named.
	SQLRaw string

	// CountColumn names the SELECT-list alias for k_min row-level suppression.
	CountColumn string

	// Dataset is the declared dataset identifier (e.g., "loans").
	Dataset string

	// FileKMin is the per-file k_min default (0 = defer to dataset/global).
	FileKMin int

	// MaxRows is the per-KPI result set size limit (0 = use package default).
	MaxRows int

	// Params is the compiled parameter map (name → CompiledParam).
	Params map[string]CompiledParam
}

// CompiledParam is the compiled form of ParamSpec for executor consumption.
type CompiledParam struct {
	// Type is the parameter type: ParamTypeString or ParamTypeEnum.
	Type string

	// Required controls whether the parameter must be provided at call time.
	Required bool

	// EnumValues is the sorted value allowlist (non-nil when Type==ParamTypeEnum).
	EnumValues []string

	// Pattern is the compiled regex constraint (non-nil when Type==ParamTypeString
	// and Pattern was declared).
	Pattern *regexp.Regexp
}

// CatalogEntry is a summary record for the kpi_call tool catalog (D16-B1).
type CatalogEntry struct {
	// Name is the KPI identifier.
	Name string

	// Description is the human-readable explanation.
	Description string

	// ParamsSummary is a one-line parameter summary (e.g., "status:enum(2), period:pattern").
	ParamsSummary string
}

// Signature is the hex-encoded sha256 of the concatenated KPI file contents.
type Signature = string

// paramsSummary builds a one-line parameter summary for the catalog.
func paramsSummary(params map[string]CompiledParam) string {
	if len(params) == 0 {
		return "none"
	}
	names := make([]string, 0, len(params))
	for n := range params {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		p := params[n]
		var desc string
		switch p.Type {
		case ParamTypeEnum:
			desc = fmt.Sprintf("%s:enum(%d)", n, len(p.EnumValues))
		case ParamTypeString:
			if p.Pattern != nil {
				desc = fmt.Sprintf("%s:pattern", n)
			} else {
				desc = fmt.Sprintf("%s:string", n)
			}
		default:
			desc = fmt.Sprintf("%s:%s", n, p.Type)
		}
		if !p.Required {
			desc += "?"
		}
		parts = append(parts, desc)
	}
	return strings.Join(parts, ", ")
}
