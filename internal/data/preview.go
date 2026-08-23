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

// Preview session store: in-memory, TTL/capacity bounded, snapshot returns.
//
// The preview phase shows the data staff the raw↔desensitized对照 of the
// first 100 rows (D15-B4) before any persistence. The raw samples live only
// in this process-bound session and are never written to disk or logs
// (persistence/logging discipline is enforced in the HTTP handler layer,
// 15-06/15-07).
package data

import (
	"container/list"
	"sync"
	"time"
)

// PreviewSessionTTL is the hygiene lifetime of a preview session. It is NOT
// a security boundary: the content hash bridge (ValidateCommitAgainstPreview)
// is what prevents previewed file A from being committed as file B. An
// expired session simply cannot be matched because Get returns miss.
//
// Source-grep locked (scripts/verify-phase-15.sh Stage 9).
const PreviewSessionTTL = 30 * time.Minute

// PreviewSessionMaxEntries caps the number of live preview sessions kept in
// process memory. Eviction is oldest-first when capacity is exceeded.
//
// Source-grep locked (scripts/verify-phase-15.sh Stage 9).
const PreviewSessionMaxEntries = 64

// RowPair is one raw↔拟脱敏对照 row shown in the preview UI. The Raw side
// is the only place the original uploaded value survives, and it is confined
// to the in-memory PreviewSession (D15-B4).
type RowPair struct {
	Raw          []string
	Desensitized []string
}

// PreviewSession is the in-memory bridge between the preview request and the
// subsequent commit request (DATA-15 two-stage import). ContentSHA256 is the
// only credential: the commit request must resupply the same file bytes so
// the recomputed digest matches, proving that the previewed file is the file
// being committed (D15-B1 / D11-C4同构).
type PreviewSession struct {
	PreviewID     string
	ContentSHA256 string
	Dataset       string
	Period        string
	RulesSHA256   string
	ProposedTier  string

	BucketOverrides map[string]float64
	SchemaTypes     map[string]string
	SampleRows      []RowPair
	RowCount        int64
	CreatedAt       time.Time
}

// SessionStore is a mutex-guarded in-memory map of preview sessions with
// TTL expiration and capacity eviction. It mirrors the
// SharedChunkAccumulator shape (internal/runtime/fanout_runtime.go:170-260).
type SessionStore struct {
	mu   sync.Mutex
	ll   *list.List
	byID map[string]*list.Element
	max  int
}

type previewEntry struct {
	id   string
	sess *PreviewSession
}

// NewSessionStore returns a fresh store with the default capacity.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		ll:   list.New(),
		byID: make(map[string]*list.Element),
		max:  PreviewSessionMaxEntries,
	}
}

// Save stores a snapshot of sess. If the store is at capacity, the oldest
// session is evicted. Expired sessions are also cleaned opportunistically
// on every write.
func (s *SessionStore) Save(sess *PreviewSession) {
	if sess == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	sessCopy := deepCopyPreviewSession(sess)
	if elem, ok := s.byID[sessCopy.PreviewID]; ok {
		elem.Value = &previewEntry{id: sessCopy.PreviewID, sess: sessCopy}
		s.ll.MoveToFront(elem)
		return
	}

	s.purgeExpiredLocked()
	for s.ll.Len() >= s.max {
		oldest := s.ll.Back()
		if oldest == nil {
			break
		}
		evicted := oldest.Value.(*previewEntry)
		delete(s.byID, evicted.id)
		s.ll.Remove(oldest)
	}

	elem := s.ll.PushFront(&previewEntry{id: sessCopy.PreviewID, sess: sessCopy})
	s.byID[sessCopy.PreviewID] = elem
}

// Get returns a snapshot copy of the session if it exists and has not
// expired. Expired sessions are removed and reported as a miss. The caller
// may freely mutate the returned PreviewSession without affecting the store.
func (s *SessionStore) Get(previewID string) (*PreviewSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	elem, ok := s.byID[previewID]
	if !ok {
		return nil, false
	}
	entry := elem.Value.(*previewEntry)
	if time.Since(entry.sess.CreatedAt) >= PreviewSessionTTL {
		delete(s.byID, previewID)
		s.ll.Remove(elem)
		return nil, false
	}
	return deepCopyPreviewSession(entry.sess), true
}

// ValidateCommitAgainstPreview enforces the content-hash bridge: the file
// bytes recomputed by the commit handler must equal the bytes that produced
// the preview. This closes the TOCTOU / swap-file window (D15-B1).
func ValidateCommitAgainstPreview(sess *PreviewSession, recomputedContentSHA256 string) error {
	if sess == nil {
		return fmtError("ValidateCommitAgainstPreview", errExpiredSession)
	}
	if sess.ContentSHA256 != recomputedContentSHA256 {
		return fmtError("ValidateCommitAgainstPreview", errHashMismatch)
	}
	return nil
}

var errExpiredSession = newSentinel("preview session not found or expired")
var errHashMismatch = newSentinel("preview content hash mismatch — previewed file A cannot be committed as file B (D15-B1)")

func newSentinel(text string) error { return &sentinelError{text} }

type sentinelError struct{ text string }

func (e *sentinelError) Error() string { return e.text }

func deepCopyPreviewSession(src *PreviewSession) *PreviewSession {
	if src == nil {
		return nil
	}
	dst := &PreviewSession{
		PreviewID:      src.PreviewID,
		ContentSHA256:  src.ContentSHA256,
		Dataset:        src.Dataset,
		Period:         src.Period,
		RulesSHA256:    src.RulesSHA256,
		ProposedTier:   src.ProposedTier,
		RowCount:       src.RowCount,
		CreatedAt:      src.CreatedAt,
		SampleRows:     make([]RowPair, len(src.SampleRows)),
		BucketOverrides: make(map[string]float64, len(src.BucketOverrides)),
		SchemaTypes:    make(map[string]string, len(src.SchemaTypes)),
	}
	for k, v := range src.BucketOverrides {
		dst.BucketOverrides[k] = v
	}
	for k, v := range src.SchemaTypes {
		dst.SchemaTypes[k] = v
	}
	for i, rp := range src.SampleRows {
		dst.SampleRows[i] = RowPair{
			Raw:          append([]string(nil), rp.Raw...),
			Desensitized: append([]string(nil), rp.Desensitized...),
		}
	}
	return dst
}

func (s *SessionStore) purgeExpiredLocked() {
	now := time.Now()
	for e := s.ll.Back(); e != nil; {
		prev := e.Prev()
		entry := e.Value.(*previewEntry)
		if now.Sub(entry.sess.CreatedAt) >= PreviewSessionTTL {
			delete(s.byID, entry.id)
			s.ll.Remove(e)
		}
		e = prev
	}
}
