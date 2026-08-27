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

// Package duckdb — kpi_engine adapter (16-06).
//
// EngineAdapter is the production QueryEngine implementation that runs a
// pre-compiled KPI SQL template over an attached dataset:
//
//  1. Composite-alias construction: ds_{dataset}__{period} is the engine-
//     side identifier; ValidateDatasetID enforces the system-wide lexical
//     allowlist (^[A-Za-z0-9_-]{1,64}$). The period component contains a
//     hyphen (e.g. "2026-07") so the alias must be DOUBLE-QUOTED in SQL.
//  2. Lazy ATTACH through the AttachLRU; the adapter acquires the LRU
//     mutex so "ensure attached" and "query" land as one atomic step — a
//     concurrent eviction cannot DETACH the alias mid-query (session-scoped
//     catalog discipline; ATTACH/DETACH mutate the connection's catalog).
//  3. Controlled regex substitution: ds_{dataset} → "ds_{dataset}__{period}"
//     with word-boundary anchors so the column name "data" and any
//     substring inside the composite itself (ds_loans followed by _, a
//     word char, has no word boundary) are never touched. The replacement
//     count MUST equal CompiledKPI.TableRefCount — a mismatch means the
//     AST walk (16-05 Layer 2) and the regex walk disagree, which is a
//     definition/contract break that surfaces as kpi.ErrAliasCountMismatch.
//  4. Named binding via driver.NamedValue: values are forwarded unchanged
//     from kpi.EngineRequest.Args (already validated by the executor's
//     fail-closed whitelist — schema check and binding are the two
//     INDEPENDENT layers of injection defense, T-16-06-01).
//  5. Row ceiling: iterating past req.MaxRows returns kpi.ErrTooManyRows
//     after the partial result is discarded. The executor transcodes it
//     into a ReasonResultTooLarge refusal (Pitfall 7 rule ②).
//
// Agent surface has zero SQL text — the SQL flowing through this adapter
// is exclusively the system-compiled $param template. The agent never
// reaches SQL through kpi_call (DATA-03 execution-side lock).
package duckdb

import (
	"context"
	"database/sql/driver"
	"fmt"
	"regexp"

	"github.com/skillforge/skillforge/internal/kpi"
)

// compositeSep separates the dataset id from the period in the composite
// ATTACH alias (e.g. "loans__2026-07"). The double underscore avoids any
// collision with single-underscore identifiers inside the dataset name
// itself (ValidateDatasetID allowlist permits [A-Za-z0-9_-]{1,64} so a
// dataset id can legitimately contain single underscores).
const compositeSep = "__"

// EngineAdapter implements kpi.QueryEngine on top of a single AttachLRU.
// The LRU is the single source of truth for "which dataset is ATTACHed
// in the dedicated session" — sharing it ensures every KPI query runs
// against the same catalog the LRU maintains.
type EngineAdapter struct {
	lru *AttachLRU
}

// NewEngineAdapter wraps an existing AttachLRU. nil panics — assembly
// mistakes must crash at boot (caller bug convention, mirrors
// retrieve.go:113-122).
func NewEngineAdapter(lru *AttachLRU) *EngineAdapter {
	if lru == nil {
		panic("duckdb: NewEngineAdapter called with nil AttachLRU (caller bug)")
	}
	return &EngineAdapter{lru: lru}
}

// Query runs one compiled KPI over its target dataset. The full
// pipeline is inlined in the doc comment above; see godoc on the package
// and on each step for the threat-model rationale.
func (a *EngineAdapter) Query(ctx context.Context, req kpi.EngineRequest) ([]map[string]any, error) {
	if a.lru == nil {
		return nil, fmtError("EngineAdapter.Query", fmt.Errorf("AttachLRU is nil"))
	}

	// ① Composite alias construction + lexical lock.
	composite := req.Dataset + compositeSep + req.Period
	if err := ValidateDatasetID(composite); err != nil {
		return nil, err
	}

	// ② Atomic attach-then-query under the LRU mutex (session-scoped
	// catalog discipline). The locking-free getLocked lets us hold l.mu
	// for the WHOLE query, not just the ATTACH, so a concurrent eviction
	// cannot DETACH the alias between "attached" and "queried".
	a.lru.mu.Lock()
	defer a.lru.mu.Unlock()

	if _, err := a.lru.getLocked(ctx, composite, req.FilePath); err != nil {
		return nil, err
	}

	// ③ Controlled alias substitution. Word-boundary anchors on both
	// sides ensure the column "data" and any identifier that merely
	// STARTS with ds_{dataset} but extends past it (e.g. ds_loans_data)
	// are never touched. QuoteMeta neutralizes regex metacharacters in
	// the dataset id (none expected under ValidateDatasetID, but the
	// lexical lock is paired with the substitution regardless).
	pattern := regexp.MustCompile(`(?i)\bds_` + regexp.QuoteMeta(req.Dataset) + `\b`)
	matches := pattern.FindAllString(req.SQL, -1)
	if len(matches) != req.TableRefCount {
		return nil, fmt.Errorf(
			"duckdb.EngineAdapter.Query: %w: ds_%s appears %d time(s) in SQL but CompiledKPI.TableRefCount=%d (template drift defense; AST count and regex count must agree)",
			kpi.ErrAliasCountMismatch, req.Dataset, len(matches), req.TableRefCount)
	}
	// The composite alias MUST be double-quoted in SQL because the period
	// ("2026-07" → "loans__2026-07") introduces a hyphen, which DuckDB
	// only accepts inside a quoted identifier. The replacement value
	// itself is system-generated and ValidateDatasetID-locked (no
	// metacharacters beyond `[A-Za-z0-9_-]`), so the extra quotes cannot
	// smuggle anything.
	substituted := pattern.ReplaceAllString(req.SQL, `"ds_`+composite+`"`)

	// ④ Query via the dedicated session (driver.QueryerContext cast).
	// The conn is the SAME one the ATTACH ran on — guarantees the query
	// sees the attached catalog.
	queryer, ok := a.lru.conn.(driver.QueryerContext)
	if !ok {
		return nil, fmtError("EngineAdapter.Query",
			fmt.Errorf("dedicated LRU conn does not implement driver.QueryerContext"))
	}
	args := make([]driver.NamedValue, len(req.Args))
	for i, na := range req.Args {
		// Ordinal=-1 signals "use Name" to drivers that consult both.
		args[i] = driver.NamedValue{Ordinal: -1, Name: na.Name, Value: na.Value}
	}

	rows, err := queryer.QueryContext(ctx, substituted, args)
	if err != nil {
		return nil, fmtError("EngineAdapter.Query",
			fmt.Errorf("query failed (dataset=%s period=%s): %w", req.Dataset, req.Period, err))
	}
	defer func() { _ = rows.Close() }()

	// ⑤ Scan rows into []map[string]any. We hold l.mu for the whole
	// scan so a concurrent caller cannot evict the alias out from under
	// us. Column ordering is taken from rows.Columns() at scan start
	// (the order is stable for the lifetime of one Query — DuckDB does
	// not reorder mid-iteration).
	cols := rows.Columns()
	dest := make([]driver.Value, len(cols))
	max := req.MaxRows
	if max <= 0 {
		// Defensive: the executor normalizes to MaxResultRows already.
		// Any zero arriving here is a programming bug — fail loud.
		return nil, fmtError("EngineAdapter.Query",
			fmt.Errorf("maxRows is %d — executor must set a positive cap", max))
	}
	out := make([]map[string]any, 0, 16)
	for {
		if err := rows.Next(dest); err != nil {
			break // io.EOF — all rows consumed
		}
		// Truncation guard: emit at most max+1 entries so we can detect
		// "exceeds" deterministically (== max+1 ⇒ over the ceiling).
		if len(out) > max {
			// Discard partial results; the executor transcodes this to
			// a ReasonResultTooLarge refusal — the KPI definition is
			// unqualified and data staff must fix the file (Pitfall 7).
			return nil, kpi.ErrTooManyRows
		}
		row := make(map[string]any, len(cols))
		for i, name := range cols {
			row[name] = dest[i]
		}
		out = append(out, row)
	}
	// driver.Rows has no Err() method (sql.Rows does). Any iteration
	// error surfaces through the rows.Next io.EOF break above; non-EOF
	// errors from Next are swallowed by io.EOF short-circuit, which is
	// the standard pattern when scanning driver.Rows directly.
	return out, nil
}
