// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 58-05 (ROI-01) -- the audit_meta retrieval_stats read path.
//
// The ROI queries must be able to answer "what fraction of turns used the
// graph?" from PERSISTED rows, so this pins:
//   A. a stats blob round-trips through audit_meta keyed by the audit id;
//   B. a row WITHOUT stats is simply absent from the map (not zero);
//   C. a pre-58 database (no column) degrades to an empty map, not an error.
package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
)

func TestRetrievalStatsBySessionRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	id, err := s.PutAudit(ctx, store.AuditRow{
		Timestamp: time.Now().UTC(),
		SessionID: "sess-roi",
		SkillID:   "compliance",
		Query:     "q",
	})
	require.NoError(t, err)

	stats := `{"channels":{"vector":3,"graph":1,"hybrid":2}}`
	require.NoError(t, s.PutAuditMeta(ctx, store.AuditMeta{
		QueryID:            id,
		ClassifierDecision: "n/a",
		RetrievalStats:     stats,
	}))

	got, err := s.RetrievalStatsBySession(ctx, "sess-roi")
	require.NoError(t, err)
	require.Contains(t, got, id)
	assert.Equal(t, stats, got[id], "the stats blob round-trips verbatim")
}

func TestRetrievalStatsBySessionOmitsRowsWithoutStats(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	id, err := s.PutAudit(ctx, store.AuditRow{
		Timestamp: time.Now().UTC(),
		SessionID: "sess-roi2",
		SkillID:   "compliance",
		Query:     "q",
	})
	require.NoError(t, err)
	// No stats on this turn's meta row.
	require.NoError(t, s.PutAuditMeta(ctx, store.AuditMeta{
		QueryID:            id,
		ClassifierDecision: "n/a",
	}))

	got, err := s.RetrievalStatsBySession(ctx, "sess-roi2")
	require.NoError(t, err)
	assert.NotContains(t, got, id,
		"a turn without stats must be ABSENT from the map, not present with a zero blob")
}

func TestRetrievalStatsBySessionUnknownSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	got, err := s.RetrievalStatsBySession(ctx, "sess-none")
	require.NoError(t, err)
	assert.Empty(t, got)
}
