// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// open.go is the PRODUCTION assembly of the knowledge-base registry: it
// joins the two halves of KBM-01 — the controlled seed file (structure)
// and the persisted runtime registry (operations) — into the one Registry
// the process boots with. cmd/server wires it in 26-09; this plan ships
// the semantics and locks them with tests.
//
// Trust posture: the seed file is a controlled file, so a malformed or
// invalid seed is an ERROR the caller turns fatal (a bad seed must not
// boot a half-known knowledge surface); an ABSENT seed is the single
// legal degraded state, because a deployment may legitimately run on its
// persisted registry alone.
//
// Seed-vs-runtime verdict (research Pitfall 6, locked here and in
// seed.go's header): "structure belongs to config, the switch belongs to
// ops". The seed is the reviewable source of a base's declared identity,
// and it is also the delivery mechanism that lets a PR add a knowledge
// base to every existing deployment — but it is NOT a garbage collector
// for runtime rows and NOT an authority over the deactivation switch an
// admin has flipped. The three failure directions this function exists to
// exclude:
//
//   - an admin deactivation resurrected by a restart (Pitfall 6a — a
//     silent security-posture regression, locked by the structure-only
//     upsert that never lets the seed write the retrieval_enabled column
//     over a persisted row);
//   - a PR-added knowledge base invisible to existing deployments
//     (Pitfall 6b — locked by always upserting declared structure);
//   - an admin-created base destroyed because the seed does not name it
//     (the seed never deletes; no caller of this file may either).
//
// Concretely: a persisted row's retrieval_enabled value is the only thing
// that decides whether the base is searchable, and no seed edit — turning
// a switch on in the file, renaming a base, changing a role — may move it.
//
// The kb package never writes audit rows; the boot/admin caller records
// every structure or switch change it drives as an insert-only audit row
// (26-05).
package kb

import (
	"context"
	"fmt"
)

// Open assembles the production registry from a controlled seed file and
// a persisted Store. Semantics, step by step:
//
//  1. Load the persisted rows: st.LoadBases(ctx). A failure returns the
//     error — a store that cannot serve the registry must not boot.
//  2. Load the seed: LoadSeedFile(seedPath). A malformed or invalid seed
//     is an error (fail-closed — the caller decides fatality, and boot
//     log.Fatalf's it); an absent file is legal and yields no entries.
//  3. For each seed entry (slug already whitelist-validated by the
//     loader): a row that already exists is upserted STRUCTURE-ONLY —
//     display_name, role and description come from the seed, while
//     retrieval_enabled and both timestamps keep their persisted values
//     ("structure belongs to config, the switch belongs to ops"). A row
//     that does not exist is inserted as declared, including the seed's
//     initial switch value. The upsert fires only when a declared field
//     actually differs, so an unchanged seed never writes at boot.
//  4. Persisted rows the seed does not declare are kept untouched — the
//     seed is not a garbage collector, and an admin-created base outlives
//     its absence from the file.
//  5. Re-read the store and build the registry over what it now holds, so
//     the returned snapshot (and its Signature, which audit rows carry)
//     reflects persisted reality, not a parallel in-memory guess.
//
// The returned Registry is store-backed: Create and Deactivate persist
// through the same Store seam and therefore survive a restart. A nil
// context error surfaces before any work; a cancelled context never yields
// a partially assembled registry (AGNT-04).
func Open(ctx context.Context, seedPath string, st Store) (Registry, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("kb.Open: %w", err)
	}

	rows, err := st.LoadBases(ctx)
	if err != nil {
		return nil, fmt.Errorf("kb.Open: load persisted registry: %w", err)
	}

	seeds, err := LoadSeedFile(seedPath)
	if err != nil {
		return nil, err // already names the file, the entry index and the slug
	}

	persisted := make(map[string]Entry, len(rows))
	for _, e := range rows {
		persisted[e.Slug] = e
	}

	for _, seed := range seeds {
		cur, exists := persisted[seed.Slug]
		if !exists {
			// New declared base: insert as declared, seed switch included.
			// This is the path that carries a PR's new knowledge base to
			// every existing deployment (Pitfall 6b).
			if err := st.UpsertBase(ctx, seed); err != nil {
				return nil, fmt.Errorf("kb.Open: seed %q: %w", seed.Slug, err)
			}
			continue
		}
		// Structure-only write guard: an unchanged declaration must not
		// rewrite the row on every boot.
		if cur.DisplayName == seed.DisplayName && cur.Role == seed.Role && cur.Description == seed.Description {
			continue
		}
		// The switch and the timestamps belong to the persisted row; only
		// the declared identity fields move. Passing the persisted switch
		// back through the upsert is what makes an admin deactivation
		// survive a restart (Pitfall 6a).
		updated := Entry{
			Slug:             cur.Slug,
			DisplayName:      seed.DisplayName,
			Role:             seed.Role,
			Description:      seed.Description,
			RetrievalEnabled: cur.RetrievalEnabled,
			CreatedAt:        cur.CreatedAt,
			UpdatedAt:        cur.UpdatedAt,
		}
		if err := st.UpsertBase(ctx, updated); err != nil {
			return nil, fmt.Errorf("kb.Open: seed %q: %w", seed.Slug, err)
		}
	}

	// Step 5: the snapshot is built from what the store now holds — a
	// single source of truth for both the entry set and the signature.
	merged, err := st.LoadBases(ctx)
	if err != nil {
		return nil, fmt.Errorf("kb.Open: reload merged registry: %w", err)
	}
	return newSnapshot(merged, st), nil
}
