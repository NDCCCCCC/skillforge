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

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// storeErrorFailureStore is a FailureStore fake that returns a configurable
// store-I/O error from GetFailure when the requested row EXISTS. Missing
// rows fall back to the existing "no failure record" sentinel so IsNotFound
// still routes them to ErrUnknownNote. Used by IN-04 to exercise the
// ErrStoreUnavailable branch in TryClaimRetry — the pre-fix code collapsed
// any non-nil error to ErrUnknownNote; the fix splits on IsNotFound.
type storeErrorFailureStore struct {
	mu       sync.Mutex
	storeErr error
	rows     map[string]Failure
}

func newStoreErrorFailureStore(storeErr error) *storeErrorFailureStore {
	return &storeErrorFailureStore{storeErr: storeErr, rows: map[string]Failure{}}
}

func (s *storeErrorFailureStore) UpsertFailure(f Failure) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[f.NoteID] = f
	return nil
}

func (s *storeErrorFailureStore) ListFailures() ([]Failure, error) {
	return nil, nil
}

func (s *storeErrorFailureStore) GetFailure(noteID string) (Failure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rows[noteID]; !ok {
		return Failure{}, errors.New("ecl: no failure record for note " + noteID)
	}
	return Failure{}, s.storeErr
}

func (s *storeErrorFailureStore) IsNotFound(err error) bool {
	return err != nil && err.Error() != "" &&
		(len(err.Error()) >= 25 && err.Error()[:25] == "ecl: no failure record f")
}

// TestIN04TryClaimRetryDistinguishesStoreError pins the IN-04 contract:
// TryClaimRetry must return ErrStoreUnavailable (not ErrUnknownNote) when
// GetFailure returns a non-IsNotFound error. The adminPipelineRetryHandler
// uses errors.Is(err, ErrStoreUnavailable) → 500 + generic body; if the
// branch is dropped or swapped, store outages surface as 404 "unknown note"
// which both leaks semantic confusion and breaks operator scripts that watch
// the 5xx rate.
//
// TryClaimRetry claims the running flag itself on entry (CAS) and releases
// it on every error path. The test must NOT pre-claim m.running via
// m.CASRunning() — that would make TryClaimRetry's internal CAS fail and
// return ErrPipelineBusy (a different contract entirely).
func TestIN04TryClaimRetryDistinguishesStoreError(t *testing.T) {
	storeErr := errors.New("ecl: sqlite: database is locked")
	fs := newStoreErrorFailureStore(storeErr)

	m, _ := newTestManager(t, threeNoteKBDir(t), newFlakyExtractor(nil), fs, nil)
	require.NotNil(t, m)

	// Pre-seed a DEAD row so the GetFailure path is exercised (otherwise the
	// store-error fake's first lookup would return the generic not-found
	// branch and skip the store-error branch).
	require.NoError(t, fs.UpsertFailure(Failure{
		NoteID: "001",
		Status: FailureDead,
	}))

	err := m.TryClaimRetry("001")
	require.Error(t, err, "TryClaimRetry must surface a store error")

	// IN-04 contract: store I/O errors are ErrStoreUnavailable, NOT
	// ErrUnknownNote. The retry handler maps these differently (500 vs 404).
	assert.True(t, errors.Is(err, ErrStoreUnavailable),
		"store I/O error must return ErrStoreUnavailable so handler maps to 500; got %v", err)
	assert.False(t, errors.Is(err, ErrUnknownNote),
		"store I/O error must NOT collapse to ErrUnknownNote (handler would 404)")
}

// TestIN04TryClaimRetryMapsNotFoundToUnknownNote pins the opposite branch:
// a genuinely missing ledger row must continue to return ErrUnknownNote (404),
// so that operator scripts that probe non-existent note IDs do not get a
// 500 spike on typos. The IN-04 fix preserves this branch via IsNotFound.
func TestIN04TryClaimRetryMapsNotFoundToUnknownNote(t *testing.T) {
	fs := newFakeFailureStore() // no rows seeded; GetFailure returns "no failure record"
	m, _ := newTestManager(t, threeNoteKBDir(t), newFlakyExtractor(nil), fs, nil)
	require.NotNil(t, m)

	err := m.TryClaimRetry("999")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownNote),
		"missing row must return ErrUnknownNote; got %v", err)
	assert.False(t, errors.Is(err, ErrStoreUnavailable),
		"missing row must NOT return ErrStoreUnavailable")
}
