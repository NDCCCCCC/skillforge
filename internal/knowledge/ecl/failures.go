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

package ecl

// Failure is one dead-letter ledger row (D-DL-01): the per-note record of
// the most recent failed stage, how many attempts have failed, the last
// error text, and the lifecycle status pending | dead | resolved.
//
// The json tags are the LOCKED wire shape (Plan 07 serializes verbatim in
// the status endpoint's failures[] array — 39-UI-SPEC §Data Contract);
// they live here so there is exactly one place that owns the field names.
// UpdatedAt is an RFC3339 UTC string set by the manager (the caller), not
// by a SQL default, keeping retry timing testable.
type Failure struct {
	NoteID       string `json:"note_id"`
	Stage        string `json:"stage"`
	LastError    string `json:"last_error"`
	Status       string `json:"status"`
	AttemptCount int    `json:"attempt_count"`
	UpdatedAt    string `json:"updated_at"`
}

// Failure statuses (D-DL-01 lifecycle).
const (
	FailurePending  = "pending"  // retrying — attempts remain
	FailureDead     = "dead"     // 3 attempts failed; awaits manual re-enqueue
	FailureResolved = "resolved" // a later run / RetryNote succeeded
)

// FailureStore is the narrow persistence seam for the dead-letter ledger
// (induction.go Generator narrow-interface precedent; RESEARCH Alternatives
// recommendation). The sqlite store (internal/store/sqlite/ecl_failures.go)
// implements it against the ecl_failures DDL; manager tests fake it with a
// map-backed implementation. Deliberately NOT part of the big store.Store
// interface — the manager depends on exactly these three operations.
type FailureStore interface {
	// UpsertFailure inserts or updates the row for f.NoteID. Status
	// transitions (pending → dead → resolved) ARE the point of this table.
	UpsertFailure(f Failure) error
	// ListFailures returns every row, newest-updated first.
	ListFailures() ([]Failure, error)
	// GetFailure returns the row for noteID, or a wrapped
	// ErrNoFailureRecord-class error for unknown notes.
	GetFailure(noteID string) (Failure, error)
}
