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

// Package sqlite_test black-box tests the CheckPointStore surface that
// Plan 08-02 adds to SQLiteStore (Get/Set). The package is external
// (sqlite_test) so the tests exercise ONLY the exported surface — the
// same call shape eino's compose.Runner / adk.Runner consume via the
// compose.CheckPointStore interface (compile-time asserted in
// checkpoint.go; compose and adk CheckPointStore are aliases of the same
// core interface).
package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCheckpointStore opens a fresh in-memory SQLiteStore for checkpoint
// tests. Mirrors internal/store/store_test.go newTestStore: a unique
// shared-cache DSN per test keeps concurrent tests isolated.
func newCheckpointStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	return openCheckpointStore(t, fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
}

// openCheckpointStore opens a SQLiteStore at dsn and registers a cleanup
// Close. Close is idempotent, so the cleanup is a safe no-op for tests
// that close the store explicitly (TestCheckpointSurvivesReopen,
// TestCheckpointClosedStore). File-backed DSNs under t.TempDir() are the
// only form that can prove reopen semantics — a reopened :memory: DSN
// is an empty database.
func openCheckpointStore(t *testing.T, dsn string) *sqlite.SQLiteStore {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	require.NoError(t, err, "sqlite.New(%q) should open a store", dsn)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestCheckpointRoundTrip — SC-4 store-layer slice: bytes written via
// Set read back byte-identically via Get (existed=true, err=nil). The
// payload deliberately includes non-UTF8 bytes to prove BLOB
// transparency — the store passes bytes through without parsing (the
// T-8-05 accept disposition: no resume-data interpretation here).
func TestCheckpointRoundTrip(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "checkpoint-roundtrip.db")
	s := openCheckpointStore(t, dsn)
	ctx := context.Background()

	payload := []byte("\x00\x01\xfe\xffcheckpoint-payload-not-utf8")
	require.NoError(t, s.Set(ctx, "cp-1", payload), "Set should insert the checkpoint blob")

	got, existed, err := s.Get(ctx, "cp-1")
	require.NoError(t, err, "Get should read the checkpoint back without error")
	require.True(t, existed, "existed must be true for a stored checkpoint id")
	assert.Equal(t, payload, got, "SC-4: checkpoint bytes must round-trip byte-identically")
}

// TestCheckpointSurvivesReopen — SC-4 "process restart" store-layer
// proof: close the store (process exit), reopen the SAME database file,
// and the checkpoint payload reads back byte-identically. This is the
// property an in-memory checkpoint store cannot provide (Pitfall 4:
// 内存 checkpoint 丢中断态) and the foundation for Phase 11 HITL
// approval state surviving restarts.
func TestCheckpointSurvivesReopen(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "checkpoint-reopen.db")
	ctx := context.Background()

	s1 := openCheckpointStore(t, dsn)
	payload := []byte(`{"interrupt_id":"tool-call-1","state":{"approved":false}}`)
	require.NoError(t, s1.Set(ctx, "cp-restart", payload), "first process writes the checkpoint")
	require.NoError(t, s1.Close(), "Close simulates the process exiting")

	s2 := openCheckpointStore(t, dsn) // "restarted" process, same DB file
	got, existed, err := s2.Get(ctx, "cp-restart")
	require.NoError(t, err, "reopened store must Get without error")
	require.True(t, existed, "SC-4: checkpoint must survive Close+reopen (existed=true)")
	assert.Equal(t, payload, got, "SC-4: payload must read back byte-identically after reopen")
}

// TestCheckpointUpsertOverwrites: Set on an existing id replaces the
// payload (ON CONFLICT(id) DO UPDATE) — a re-interrupted conversation
// must not resurrect the stale checkpoint.
func TestCheckpointUpsertOverwrites(t *testing.T) {
	s := newCheckpointStore(t)
	ctx := context.Background()

	first := []byte("first-checkpoint-payload")
	second := []byte("second-checkpoint-payload")
	require.NoError(t, s.Set(ctx, "cp-up", first))
	require.NoError(t, s.Set(ctx, "cp-up", second), "second Set must upsert, not error")

	got, existed, err := s.Get(ctx, "cp-up")
	require.NoError(t, err)
	require.True(t, existed)
	assert.Equal(t, second, got, "upsert must overwrite — Get returns the SECOND payload")
	assert.NotEqual(t, first, got)
}

// TestCheckpointUnknownID: Get on an id that was never Set returns
// (nil, false, nil) — existed=false is NOT an error. Treating a missing
// checkpoint as an error would force every resume-path caller to
// special-case the very first run (Pitfall 4 converse).
func TestCheckpointUnknownID(t *testing.T) {
	s := newCheckpointStore(t)
	ctx := context.Background()

	got, existed, err := s.Get(ctx, "missing")
	require.NoError(t, err, "unknown id must NOT be an error")
	assert.False(t, existed, "existed must be false for an unknown id")
	assert.Nil(t, got, "payload must be nil for an unknown id")
}

// TestCheckpointSQLInjection — T-8-04 (T-3-SQLI posture 延续): a DROP
// TABLE payload smuggled through checkPointID is treated as a literal
// string because every statement is `?`-parameterized. Mirrors
// store_test.go TestAuditRowSQLInjection.
func TestCheckpointSQLInjection(t *testing.T) {
	s := newCheckpointStore(t)
	ctx := context.Background()

	require.NoError(t, s.Set(ctx, "cp-real", []byte("real-payload")))

	payload := "x'; DROP TABLE checkpoint; --"

	// Get with the injection id — no SQL syntax error, no match.
	got, existed, err := s.Get(ctx, payload)
	require.NoError(t, err, "parameterized Get must not surface an SQL error on the payload")
	assert.False(t, existed)
	assert.Nil(t, got)

	// Set with the injection id — the payload becomes a literal row key.
	require.NoError(t, s.Set(ctx, payload, []byte("injected-row")))

	// Follow-up: the checkpoint table is still fully functional — the
	// seeded row reads back intact AND the literal payload row does too.
	got, existed, err = s.Get(ctx, "cp-real")
	require.NoError(t, err)
	require.True(t, existed, "the checkpoint table must survive — the seeded row is intact")
	assert.Equal(t, []byte("real-payload"), got)

	got, existed, err = s.Get(ctx, payload)
	require.NoError(t, err)
	require.True(t, existed, "the injection string round-trips as a literal row key")
	assert.Equal(t, []byte("injected-row"), got)
}

// TestCheckpointClosedStore: after Close, Get and Set both return
// sql.ErrConnDone — the isClosed() guard path shared with every other
// SQLiteStore method (session.go precedent).
func TestCheckpointClosedStore(t *testing.T) {
	s := newCheckpointStore(t)
	ctx := context.Background()

	require.NoError(t, s.Close())

	_, _, err := s.Get(ctx, "cp-after-close")
	require.ErrorIs(t, err, sql.ErrConnDone, "Get after Close must return sql.ErrConnDone")

	err = s.Set(ctx, "cp-after-close", []byte("payload"))
	require.ErrorIs(t, err, sql.ErrConnDone, "Set after Close must return sql.ErrConnDone")
}

// TestSQLiteStoreDelete (Plan 11-01 / D11-C2): Set then Delete then
// Get returns (nil, false, nil) — Delete removes the row; subsequent
// Get treats the id as unknown (existed=false is NOT an error, same
// disposition as TestCheckpointUnknownID). Idempotent: deleting a
// non-existent id is a no-op (returns nil, not error).
func TestSQLiteStoreDelete(t *testing.T) {
	s := newCheckpointStore(t)
	ctx := context.Background()

	require.NoError(t, s.Set(ctx, "cp-delete-1", []byte("payload-to-delete")))

	// Verify it landed
	got, existed, err := s.Get(ctx, "cp-delete-1")
	require.NoError(t, err)
	require.True(t, existed)

	// Delete
	require.NoError(t, s.Delete(ctx, "cp-delete-1"), "Delete must succeed for an existing id")

	// Get returns (nil, false, nil) — the row is gone
	got, existed, err = s.Get(ctx, "cp-delete-1")
	require.NoError(t, err, "Get after Delete must not error")
	assert.False(t, existed, "existed must be false after Delete")
	assert.Nil(t, got, "payload must be nil after Delete")

	// Idempotent: Delete on a non-existent id returns nil
	require.NoError(t, s.Delete(ctx, "cp-never-existed"),
		"Delete must be idempotent (non-existent id is a no-op)")
}

// TestSQLiteStoreCheckPointDeleterCompileTime — belt-and-suspenders
// compile-time guard for the adk.CheckPointDeleter interface. The
// primary lock is the `var _ adk.CheckPointDeleter = (*SQLiteStore)(nil)`
// assertion in checkpoint.go (compile-time fails the build on
// interface drift). This reflection-based test verifies the method
// set at the reflection layer — defense-in-depth against a future
// Eino upgrade that introduces an additional method on
// CheckPointDeleter (which would still satisfy the interface but
// would not be implemented by SQLiteStore yet).
func TestSQLiteStoreCheckPointDeleterCompileTime(t *testing.T) {
	s := &sqlite.SQLiteStore{}
	tp := reflect.TypeOf(s)
	method, ok := tp.MethodByName("Delete")
	require.True(t, ok, "SQLiteStore must have a Delete method (adk.CheckPointDeleter contract)")
	// Method signature: Delete(context.Context, string) error.
	mt := method.Type
	require.Equal(t, 3, mt.NumIn(), "Delete must take (receiver, context.Context, string)")
	require.Equal(t, reflect.TypeOf((*error)(nil)).Elem(), mt.Out(0), "Delete must return error")
	// Second parameter must be a string (not a pointer or struct).
	param := mt.In(2)
	require.Equal(t, reflect.String, param.Kind(), "second arg must be string")
}

// TestCheckpointIDFormat — Plan 11-01 / D11-A1 + D11-C1 pins the
// canonical Runner checkpoint ID shape. Asserts the format is stable
// across the agent package boundary: the store treats the ID as an
// opaque string, but the producer (agent.RunnerRunnerrCheckPointID)
// and the consumer (this test) agree on the exact byte sequence.
//
// The "sess:" prefix isolates SkillForge IDs from eino-internal ID
// namespaces (eino uses random UUIDs by default); the ":cp" suffix
// marks the row as a checkpoint payload (vs. any future run-state
// table that might share the namespace).
func TestCheckpointIDFormat(t *testing.T) {
	id := agent.RunnerCheckPointID("sess-abc-123")
	assert.Equal(t, "sess:sess-abc-123:cp", id,
		"RunnerCheckPointID must produce the canonical format")

	// Round-trip: Set with the format, Get returns the payload
	s := newCheckpointStore(t)
	ctx := context.Background()
	require.NoError(t, s.Set(ctx, id, []byte("payload")))
	got, existed, err := s.Get(ctx, id)
	require.NoError(t, err)
	require.True(t, existed)
	assert.Equal(t, []byte("payload"), got)
}

// TestCheckpointDeleteExpired — D11-C2 expire path.
// SetCheckpointWithSnapshot persists (id, payload, skill_version,
// prompt_hash); GetCheckpointByVersionFilter returns the payload ONLY
// when both snapshot columns match the live Skill. On mismatch the
// resume path Deletes the row (the 11-01 Delete method) and reports
// state="expired" instead of resuming against a drifted Skill. The
// audit row is NEVER deleted (T-3-AUDIT).
func TestCheckpointDeleteExpired(t *testing.T) {
	s := newCheckpointStore(t)
	ctx := context.Background()

	payload := []byte("checkpoint-payload-bytes")
	require.NoError(t, s.SetCheckpointWithSnapshot(ctx, "cp-expire", payload, "v1", "ph-v1"))

	// Matching snapshot → the row is returned.
	got, existed, err := s.GetCheckpointByVersionFilter(ctx, "cp-expire", "v1", "ph-v1")
	require.NoError(t, err)
	require.True(t, existed)
	assert.Equal(t, payload, got)

	// Skill version bumped → expired.
	_, existed, err = s.GetCheckpointByVersionFilter(ctx, "cp-expire", "v2", "ph-v1")
	require.NoError(t, err, "a version mismatch is NOT an error — it is the expire state")
	assert.False(t, existed, "version mismatch must return existed=false")

	// prompt_hash changed → expired.
	_, existed, err = s.GetCheckpointByVersionFilter(ctx, "cp-expire", "v1", "ph-v2")
	require.NoError(t, err)
	assert.False(t, existed, "prompt_hash mismatch must return existed=false")

	// The expire path itself: the caller Deletes the stale row.
	require.NoError(t, s.Delete(ctx, "cp-expire"), "Delete on an expired checkpoint must succeed")

	got, existed, err = s.GetCheckpointByVersionFilter(ctx, "cp-expire", "v1", "ph-v1")
	require.NoError(t, err)
	assert.False(t, existed)
	assert.Nil(t, got)

	// The snapshot upsert tracks the newest interrupt: a re-Set with a
	// new snapshot replaces both payload and snapshot columns.
	require.NoError(t, s.SetCheckpointWithSnapshot(ctx, "cp-expire", []byte("second"), "v2", "ph-v2"))
	_, existed, err = s.GetCheckpointByVersionFilter(ctx, "cp-expire", "v1", "ph-v1")
	require.NoError(t, err)
	assert.False(t, existed, "the stale snapshot must no longer match after the upsert")
	got, existed, err = s.GetCheckpointByVersionFilter(ctx, "cp-expire", "v2", "ph-v2")
	require.NoError(t, err)
	require.True(t, existed)
	assert.Equal(t, []byte("second"), got)
}
