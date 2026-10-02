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

// Package kpi implements the KPI aggregation pipeline (Phase 16, P10.2).
//
// KPI definitions are authored by data staff in YAML files (kpis/*.yaml),
// validated and compiled at service startup into an immutable in-memory
// Registry, and exposed to the agent via a single kpi_call tool. The agent
// never sees SQL — it calls by name with declared parameters, and the
// executor binds validated values into pre-compiled templates.
//
// # CGO-free constraint
//
// This package MUST NOT import internal/duckdb or github.com/marcboeker/go-duckdb.
// Engine dependencies are injected via narrow interfaces (16-05/16-06 adapters).
// Windows hermetic tests are a package-level constraint: go test ./internal/kpi/
// must work on Windows without CGO.
//
// # Controlled file discipline
//
// KPI YAML files are controlled files (D16-A1): versioned in git, changed via
// PR review, and fail-closed validated at load time. Any single file failing
// validation prevents the entire directory from loading — partial state is
// unspecified and must not occur (KPI-01 no-registration-backdoor invariant).
//
// # Directory layout
//
//   - define.go         — KPIFile / KPIDef / ParamSpec YAML structs + type vocabulary
//   - validate_text.go  — Layer 1 pure-Go fail-closed validation (hermetic)
//   - validate_sql.go   — Layer 2 json_serialize_sql AST walk (WSL-gated, 16-05)
//   - loader.go         — LoadKPIs directory loader (Layer 1 pipeline + signature hash)
//   - registry.go       — Immutable Registry + read-only interface (Lookup/Catalog/Signature)
//   - executor.go       — Call(ctx, name, params) — validate→bind→execute→k_min→snapshot (16-06)
//   - kmin.go           — Row-level k_min suppression post-filter (16-06)
package kpi
