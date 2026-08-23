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

package data

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func sampleSession(id string) *PreviewSession {
	return &PreviewSession{
		PreviewID:       id,
		ContentSHA256:   sha256Of("content-" + id),
		Dataset:         "loans",
		Period:          "2026-07",
		RulesSHA256:     sha256Of("rules"),
		ProposedTier:    "历史",
		BucketOverrides: map[string]float64{"age": 10},
		SchemaTypes:     map[string]string{"age": "BIGINT"},
		SampleRows: []RowPair{
			{Raw: []string{"raw"}, Desensitized: []string{"des"}},
		},
		RowCount:  1000,
		CreatedAt: time.Now(),
	}
}

func sha256Of(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestPreviewSessionSaveGet(t *testing.T) {
	store := NewSessionStore()
	sess := sampleSession("A")
	store.Save(sess)

	got, ok := store.Get("A")
	if !ok {
		t.Fatalf("expected session A to be found")
	}
	if got.PreviewID != sess.PreviewID {
		t.Errorf("PreviewID = %q, want %q", got.PreviewID, sess.PreviewID)
	}
	if got.ContentSHA256 != sess.ContentSHA256 {
		t.Errorf("ContentSHA256 mismatch")
	}
	if got.BucketOverrides["age"] != 10 {
		t.Errorf("BucketOverrides mismatch")
	}
	if len(got.SampleRows) != 1 || got.SampleRows[0].Raw[0] != "raw" {
		t.Errorf("SampleRows mismatch")
	}

	// Snapshot discipline: mutating the returned copy must not affect the store.
	got.BucketOverrides["age"] = 99
	got.SampleRows[0].Raw[0] = "mutated"
	got.SchemaTypes["age"] = "VARCHAR"

	got2, _ := store.Get("A")
	if got2.BucketOverrides["age"] != 10 {
		t.Errorf("store BucketOverrides mutated through snapshot")
	}
	if got2.SampleRows[0].Raw[0] != "raw" {
		t.Errorf("store SampleRows mutated through snapshot")
	}
	if got2.SchemaTypes["age"] != "BIGINT" {
		t.Errorf("store SchemaTypes mutated through snapshot")
	}
}

func TestPreviewSessionTTLMiss(t *testing.T) {
	store := NewSessionStore()
	sess := sampleSession("old")
	sess.CreatedAt = time.Now().Add(-PreviewSessionTTL - time.Minute)
	store.Save(sess)

	got, ok := store.Get("old")
	if ok {
		t.Fatalf("expected expired session to miss, got %+v", got)
	}
	if got != nil {
		t.Errorf("expected nil session for expired miss")
	}
}

func TestPreviewSessionCapacityEvict(t *testing.T) {
	store := NewSessionStore()
	for i := 0; i < PreviewSessionMaxEntries+1; i++ {
		sess := sampleSession(fmt.Sprintf("s%d", i))
		// Stagger creation so the oldest is deterministic.
		sess.CreatedAt = time.Now().Add(-time.Duration(PreviewSessionMaxEntries-i) * time.Second)
		store.Save(sess)
	}
	_, ok := store.Get("s0")
	if ok {
		t.Fatalf("oldest session should have been evicted at capacity")
	}
	for i := 1; i <= PreviewSessionMaxEntries; i++ {
		if _, ok := store.Get(fmt.Sprintf("s%d", i)); !ok {
			t.Fatalf("session s%d should still be present", i)
		}
	}
}

func TestPreviewCommitHashBridge(t *testing.T) {
	sess := &PreviewSession{
		PreviewID:     "p1",
		ContentSHA256: sha256Of("file-A-contents"),
	}
	if err := ValidateCommitAgainstPreview(sess, sha256Of("file-A-contents")); err != nil {
		t.Errorf("same hash should validate: %v", err)
	}
	if err := ValidateCommitAgainstPreview(sess, sha256Of("file-B-contents")); err == nil {
		t.Fatalf("different hash should be rejected")
	} else if err.Error() != "preview content hash mismatch — previewed file A cannot be committed as file B (D15-B1)" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
	if err := ValidateCommitAgainstPreview(nil, sha256Of("anything")); err == nil {
		t.Fatalf("nil session should be rejected")
	} else if err.Error() != "preview session not found or expired" {
		t.Errorf("unexpected nil-session error: %q", err.Error())
	}
}
