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

package duckdb

import (
	"container/list"
	"context"
	"database/sql/driver"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// MaxAttachedDatasets is the LRU cap on attached datasets (D14-D3). Attaching
// is lazy — a dataset is only ATTACHed the first time a query needs it — and
// the cap bounds how many .duckdb files hold an open handle at once. That
// bound is the Windows-lock surface (Pitfall 4): every attached file is
// locked by the process, so an unbounded attach set would make dataset
// cleanup (DETACH + delete, Phase 15/19) fail on held handles.
//
// Magnitude reference: Phase 13's fanout SharedChunkAccumulator caps at 20
// in-memory chunk entries; dataset files are heavier OS resources, so the
// default sits lower.
//
// Source-grep locked via TestAttachLRUCap.
const MaxAttachedDatasets = 8

// AttachLRU is a mutex-guarded lazy-ATTACH registry over a SINGLE dedicated
// driver connection (D14-D3). DuckDB's ATTACH is session-scoped: it mutates
// the catalog of the connection that executed it. Holding one fixed
// connection keeps the attached-catalog deterministic — every ATTACH/DETACH
// and every dataset query goes through the same session. Mirrors the
// SharedChunkAccumulator shape (internal/runtime/fanout_runtime.go): front of
// the list is the most recently used, back is the eviction candidate.
type AttachLRU struct {
	mu   sync.Mutex
	ll   *list.List               // doubly-linked; front = MRU, back = LRU
	byID map[string]*list.Element // dataset id → list element
	max  int

	conn driver.Conn // dedicated session; owns the attached catalog
	exec driver.ExecerContext
}

// attachEntry is the LRU payload; the id snapshot lets the eviction path
// issue DETACH without re-deriving the key from the element.
type attachEntry struct {
	id string
}

// NewAttachLRU opens the dedicated connection for a lazy-ATTACH registry.
// max <= 0 falls back to MaxAttachedDatasets. The caller owns the returned
// AttachLRU and must Close it (before closing the Connector).
func NewAttachLRU(c *Connector, max int) (*AttachLRU, error) {
	if max <= 0 {
		max = MaxAttachedDatasets
	}
	conn, err := c.connector.Connect(context.Background())
	if err != nil {
		return nil, fmtError("NewAttachLRU", err)
	}
	exec, ok := conn.(driver.ExecerContext)
	if !ok {
		_ = conn.Close()
		return nil, fmtError("NewAttachLRU", fmt.Errorf("driver conn does not implement ExecerContext"))
	}
	return &AttachLRU{
		ll:   list.New(),
		byID: make(map[string]*list.Element),
		max:  max,
		conn: conn,
		exec: exec,
	}, nil
}

// Get returns whether dataset id is already attached. A hit re-touches the
// entry (MRU position). A miss validates the identifier, evicts the LRU
// entry when at cap, and ATTACHes the dataset READ_ONLY under the ds_{id}
// alias (via AttachDataset — same allowlist guard).
func (l *AttachLRU) Get(ctx context.Context, id, path string) (hit bool, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, ok := l.byID[id]; ok {
		l.ll.MoveToFront(elem) // idempotent re-touch
		return true, nil
	}
	if err := ValidateDatasetID(id); err != nil {
		return false, err
	}
	// Evict least-recently-used entries while at cap. DETACH runs BEFORE the
	// bookkeeping removal and before any future os.Remove of the file — on
	// Windows the file stays locked until the engine releases its handle
	// (Pitfall 4 / T-14-17). A failed DETACH keeps the entry (conservative:
	// the alias may still be live).
	for l.ll.Len() >= l.max {
		oldest := l.ll.Back()
		if oldest == nil {
			break
		}
		evicted := oldest.Value.(*attachEntry)
		if err := l.execDetachLocked(ctx, evicted.id); err != nil {
			return false, err
		}
		delete(l.byID, evicted.id)
		l.ll.Remove(oldest)
	}
	if err := AttachDataset(l.exec, id, path); err != nil {
		return false, err
	}
	elem := l.ll.PushFront(&attachEntry{id: id})
	l.byID[id] = elem
	return false, nil
}

// Detach releases dataset id from the attached catalog. Idempotent: an id
// that is not currently attached is a no-op (the TTL cleanup path in
// Phase 15/19 may race a concurrent eviction). The engine-side DETACH always
// runs before the caller may delete the file — callers MUST NOT os.Remove a
// dataset file before this returns (Windows lock boundary).
func (l *AttachLRU) Detach(ctx context.Context, id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	elem, ok := l.byID[id]
	if !ok {
		return nil // never attached or already evicted — no-op
	}
	if err := l.execDetachLocked(ctx, id); err != nil {
		return err
	}
	delete(l.byID, id)
	l.ll.Remove(elem)
	return nil
}

// execDetachLocked issues the DETACH for an already-validated id. Callers
// hold l.mu.
func (l *AttachLRU) execDetachLocked(ctx context.Context, id string) error {
	q := fmt.Sprintf("DETACH ds_%s", id)
	if _, err := l.exec.ExecContext(ctx, q, nil); err != nil {
		return fmtError("Detach", fmt.Errorf("ds_%s: %w", id, err))
	}
	return nil
}

// AttachedIDs returns the ds_-prefixed database names the engine currently
// reports for the dedicated connection (via duckdb_databases()), sorted
// alphabetically. This is the engine-side truth — test and operator surfaces
// use it to prove that bookkeeping and engine catalog agree.
func (l *AttachLRU) AttachedIDs(ctx context.Context) ([]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	queryer, ok := l.conn.(driver.QueryerContext)
	if !ok {
		return nil, fmtError("AttachedIDs", fmt.Errorf("driver conn does not implement QueryerContext"))
	}
	rows, err := queryer.QueryContext(ctx, "SELECT database_name FROM duckdb_databases() ORDER BY 1", nil)
	if err != nil {
		return nil, fmtError("AttachedIDs", err)
	}
	defer func() { _ = rows.Close() }()

	var out []string
	dest := make([]driver.Value, 1)
	for {
		if err := rows.Next(dest); err != nil {
			break // io.EOF — all rows consumed
		}
		name, _ := dest[0].(string)
		if strings.HasPrefix(name, "ds_") {
			out = append(out, name[len("ds_"):])
		}
	}
	sort.Strings(out)
	return out, nil
}

// Len returns the number of currently attached datasets (bookkeeping side).
func (l *AttachLRU) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ll.Len()
}

// Close releases the dedicated connection. Safe to call once; the Connector
// that spawned it must be closed separately (its Close discipline is the
// 14-05 double-close contract).
func (l *AttachLRU) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		return nil
	}
	err := l.conn.Close()
	l.conn = nil
	l.exec = nil
	if err != nil {
		return fmtError("Close", err)
	}
	return nil
}
