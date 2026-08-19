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

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// Session-management phase — handler-level tests for the roster
// surface (GET /api/sessions, PATCH/DELETE /api/session/:id).
//
// Contracts under test:
//
//   - List aggregates session_turn into a newest-first roster with the
//     rename override merged into a single title field
//   - Rename upserts; an unknown-but-valid id is still 200 (no
//     existence leak, T-10-11)
//   - Delete drops turns+meta from the roster while the audit trail
//     survives (T-3-AUDIT), and is idempotent (unknown id → 204)
//   - 400 shapes: malformed id, empty/over-long title, malformed JSON
//   - Store nil → 404 (defensive misconfig posture, same as history)

// sessionsTestHertz spins up a Hertz server with the three roster
// routes bound. Mirrors historyTestHertz (same-package helper) — the
// chat route stays unregistered.
func sessionsTestHertz(t *testing.T, deps *Deps) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.GET("/api/sessions", deps.sessionsListHandler)
	h.PATCH("/api/session/:id", deps.sessionRenameHandler)
	h.DELETE("/api/session/:id", deps.sessionDeleteHandler)

	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()

	cleanup := func() {
		_ = h.Close()
		<-errCh
	}

	addr := "127.0.0.1:" + portStr(port)
	if err := waitForListener(addr, 3*time.Second); err != nil {
		cleanup()
		t.Fatalf("server warmup: %v", err)
	}
	return addr, cleanup
}

// listSessions GETs /api/sessions. Returns the status + parsed body.
func listSessions(t *testing.T, addr string) (int, sessionsResponse) {
	t.Helper()
	resp, err := http.Get("http://" + addr + "/api/sessions")
	if err != nil {
		t.Fatalf("GET /api/sessions: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := sessionsResponse{}
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &out), "body=%s", string(raw))
	}
	return resp.StatusCode, out
}

// patchSessionTitle issues PATCH /api/session/:id with a JSON body.
// body may be raw bytes (malformed-JSON tests) or a title string
// (encoded as {"title":...}).
func patchSessionTitle(t *testing.T, addr, sessionID string, body any) int {
	t.Helper()
	var payload []byte
	switch b := body.(type) {
	case string:
		payload, _ = json.Marshal(map[string]string{"title": b})
	case []byte:
		payload = b
	default:
		t.Fatalf("unsupported body type %T", body)
	}
	req, err := http.NewRequest(http.MethodPatch,
		"http://"+addr+"/api/session/"+sessionID, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// deleteSession issues DELETE /api/session/:id.
func deleteSession(t *testing.T, addr, sessionID string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete,
		"http://"+addr+"/api/session/"+sessionID, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func TestSessionsAPIListAggregatesNewestFirst(t *testing.T) {
	s := historyTestStore(t)
	addr, cleanup := sessionsTestHertz(t, &Deps{Store: s})
	defer cleanup()

	base := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	seedSessionTurn(t, s, "sess-alpha-0001", "第一句问话", `{"status":"compliant"}`, base)
	seedSessionTurn(t, s, "sess-beta-0002", "更新的问题", `{"status":"compliant"}`, base.Add(2*time.Hour))

	status, body := listSessions(t, addr)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, body.Sessions, 2)

	// Newest activity first; derived title = first user turn.
	assert.Equal(t, "sess-beta-0002", body.Sessions[0].ID)
	assert.Equal(t, "更新的问题", body.Sessions[0].Title)
	assert.Greater(t, body.Sessions[0].LastActiveMs, body.Sessions[1].LastActiveMs)
	assert.Equal(t, "sess-alpha-0001", body.Sessions[1].ID)
	assert.Equal(t, "第一句问话", body.Sessions[1].Title)
}

func TestSessionsAPIListEmptyDBIsEmptyArrayNotNull(t *testing.T) {
	s := historyTestStore(t)
	addr, cleanup := sessionsTestHertz(t, &Deps{Store: s})
	defer cleanup()

	status, body := listSessions(t, addr)
	require.Equal(t, http.StatusOK, status)
	// sessionsResponse.Sessions starts as a non-nil empty slice via
	// make(); the FE relies on array-not-null to .map safely.
	assert.Empty(t, body.Sessions)
	assert.NotNil(t, body.Sessions)
}

func TestSessionsAPIRenameOverrideWinsInList(t *testing.T) {
	s := historyTestStore(t)
	addr, cleanup := sessionsTestHertz(t, &Deps{Store: s})
	defer cleanup()

	seedSessionTurn(t, s, "sess-alpha-0001", "原始问题", `{}`, time.Now().UTC())

	require.Equal(t, http.StatusOK,
		patchSessionTitle(t, addr, "sess-alpha-0001", "自定义名字"))

	status, body := listSessions(t, addr)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, body.Sessions, 1)
	assert.Equal(t, "自定义名字", body.Sessions[0].Title, "override replaces the derived title on the wire")
}

func TestSessionsAPIRenameUnknownIDStill200(t *testing.T) {
	// T-10-11: the upsert is legal before any turns exist — 200 either
	// way leaks no existence signal.
	s := historyTestStore(t)
	addr, cleanup := sessionsTestHertz(t, &Deps{Store: s})
	defer cleanup()

	assert.Equal(t, http.StatusOK,
		patchSessionTitle(t, addr, "sess-never-was-99", "预命名"))
}

func TestSessionsAPIDeleteRemovesRosterKeepsAudit(t *testing.T) {
	s := historyTestStore(t)
	addr, cleanup := sessionsTestHertz(t, &Deps{Store: s})
	defer cleanup()
	ctx := context.Background()

	now := time.Now().UTC()
	seedSessionTurn(t, s, "sess-doomed-001", "将被删除", `{}`, now)
	_, err := s.PutAudit(ctx, store.AuditRow{
		Timestamp:   now,
		SessionID:   "sess-doomed-001",
		SkillID:     "compliance",
		Query:       "将被删除",
		PromptHash:  "deadbeef",
		VerdictJSON: "{}",
	})
	require.NoError(t, err)

	// Present before delete.
	_, body := listSessions(t, addr)
	require.Len(t, body.Sessions, 1)

	assert.Equal(t, http.StatusNoContent, deleteSession(t, addr, "sess-doomed-001"))

	// Gone from the roster + history…
	_, body = listSessions(t, addr)
	assert.Empty(t, body.Sessions)
	hist, err := s.GetSession(ctx, "sess-doomed-001")
	require.NoError(t, err)
	assert.Empty(t, hist)

	// …while the audit trail survives (T-3-AUDIT).
	audits, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-doomed-001", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, audits, 1, "immutable audit rows must survive session deletion")
}

func TestSessionsAPIDeleteIdempotent(t *testing.T) {
	s := historyTestStore(t)
	addr, cleanup := sessionsTestHertz(t, &Deps{Store: s})
	defer cleanup()

	seedSessionTurn(t, s, "sess-once-00001", "删一次就够", `{}`, time.Now().UTC())
	require.Equal(t, http.StatusNoContent, deleteSession(t, addr, "sess-once-00001"))
	// Second delete + unknown id: still 204, zero rows affected.
	assert.Equal(t, http.StatusNoContent, deleteSession(t, addr, "sess-once-00001"))
	assert.Equal(t, http.StatusNoContent, deleteSession(t, addr, "sess-never-was-99"))
}

func TestSessionsAPIBadRequestShapes(t *testing.T) {
	s := historyTestStore(t)
	addr, cleanup := sessionsTestHertz(t, &Deps{Store: s})
	defer cleanup()

	longTitle := strings.Repeat("字", 121)

	cases := []struct {
		name   string
		method string
		id     string
		body   any
	}{
		{"patch malformed id", http.MethodPatch, "short", "x"},
		{"patch empty title", http.MethodPatch, "sess-alpha-0001", "   "},
		{"patch title over 120 runes", http.MethodPatch, "sess-alpha-0001", longTitle},
		{"patch malformed json", http.MethodPatch, "sess-alpha-0001", []byte(`{"title":`)},
		{"delete malformed id", http.MethodDelete, "bad/id", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var status int
			switch tc.method {
			case http.MethodPatch:
				status = patchSessionTitle(t, addr, tc.id, tc.body)
			case http.MethodDelete:
				status = deleteSession(t, addr, tc.id)
			}
			// "bad/id" breaks the route path → Hertz 404 before the
			// handler runs (same posture as the history test's
			// path-breaking cases: rejected either way).
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, status)
		})
	}
}

func TestSessionsAPINilStoreIs404(t *testing.T) {
	addr, cleanup := sessionsTestHertz(t, &Deps{})
	defer cleanup()

	status, _ := listSessions(t, addr)
	assert.Equal(t, http.StatusNotFound, status)

	assert.Equal(t, http.StatusNotFound,
		patchSessionTitle(t, addr, "sess-alpha-0001", "x"))
	assert.Equal(t, http.StatusNotFound,
		deleteSession(t, addr, "sess-alpha-0001"))
}

// Compile-time reuse guard: the roster tests lean on the same in-memory
// store factory as the history tests (same package) — this keeps an
// accidental rename of historyTestStore from silently orphaning this
// file at link time instead of at review time.
var _ = sqlite.Config{DSN: ":memory:"}
