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

package milvus

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

// DeleteByExpr deletes every row matching the caller-built expression —
// the generic, marker-scoped purge primitive behind Phase 39's ECL
// pipeline (Plan 39-02 / RESEARCH Pattern 3).
//
// Why it exists: DeleteTierRows purges a WHOLE tier, which is wrong once
// two producers share a tier. With ECL on, the sql-guides tier holds BOTH
// legacy-path rows AND ECL rows (Pitfall 1, RESEARCH 39): a tier-level
// purge from either producer would destroy the other's rows. The escape
// hatch is the ownership marker (D-Milvus-02): section_count > 0 marks
// ECL-owned rows, so each producer purges exactly its own generation:
//
//	ECL milvus-load: DeleteByExpr(ctx, mc, cfg,
//	    `tier in ["sql-guides"] and section_count > 0`)
//	Boot Index (ECL on): DeleteByExpr(ctx, mc, cfg,
//	    `tier in ["sql-guides"] and section_count == 0`)
//
// CONTRACT — the caller owns the expression. This helper NEVER
// interpolates values into expr; interpolated values (tier slugs,
// source_idx, ...) MUST be escaped by the caller with strconv.Quote /
// fmt.Sprintf("%q", ...) — the injection-safe precedent is
// filter.go:55-59 (and DeleteTierRows' own strconv.Quote loop). Values
// originate from registry slugs / corpus paths, never raw user input,
// but the escaping duty stands regardless (T-39-01 mitigation).
//
// Mechanics (copied verbatim from DeleteTierRows, seed.go): a missing
// collection is a no-op (fresh deploy has nothing to purge); DML requires
// a loaded collection, so LoadCollection runs before the Delete; the
// flush afterwards makes the tombstones durable and visible immediately
// instead of at the next flush boundary.
//
// Callers:
//   - knowledge seedDocs seam (Plan 39-06) — boot-time marker-scoped purge
//   - the ECL pipeline milvus-load stage (Plan 39-05) — ALWAYS paired with
//     a following SeedDocuments of the replacement rows.
func DeleteByExpr(ctx context.Context, mc client.Client, cfg Config, expr string) error {
	cfg = cfg.withDefaults()
	if mc == nil {
		// Mirrors DeleteTierRows' explicit guard so the error names the
		// purge instead of panicking inside the SDK.
		return errors.New("milvus.DeleteByExpr: milvus client is nil")
	}
	if strings.TrimSpace(expr) == "" {
		// Milvus would reject an empty expression server-side, but the
		// failure would read as an SDK error; naming the caller duty here
		// keeps the boot log actionable. An expression is never optional —
		// a "delete everything" convenience is deliberately NOT offered.
		return errors.New("milvus.DeleteByExpr: expr is empty (the caller must build the expression)")
	}
	has, err := mc.HasCollection(ctx, cfg.CollectionName)
	if err != nil {
		return fmt.Errorf("milvus.DeleteByExpr: HasCollection(%q): %w", cfg.CollectionName, err)
	}
	if !has {
		// Fresh deploy: the collection does not exist yet, so there is
		// nothing stale to purge (the paired SeedDocuments will create it).
		return nil
	}
	if err := mc.LoadCollection(ctx, cfg.CollectionName, false); err != nil {
		return fmt.Errorf("milvus.DeleteByExpr: LoadCollection(%q): %w", cfg.CollectionName, err)
	}
	if err := mc.Delete(ctx, cfg.CollectionName, "", expr); err != nil {
		return fmt.Errorf("milvus.DeleteByExpr: Delete(%q, %s): %w", cfg.CollectionName, expr, err)
	}
	if err := mc.Flush(ctx, cfg.CollectionName, false); err != nil {
		return fmt.Errorf("milvus.DeleteByExpr: Flush(%q): %w", cfg.CollectionName, err)
	}
	return nil
}
