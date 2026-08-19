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
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/store"
)

// sessionEntry is one roster row in the GET /api/sessions payload. The
// FE SessionDrawer consumes this shape (web/src/lib/sessions.ts).
//
//	Title         display title — the session_meta rename override when
//	              present, else the derived first-user-turn title. The
//	              merge happens HERE so the FE carries one field.
//	LastActiveMs epoch ms of the newest session_turn ts (grouping key
//	              for the drawer's 今天 / 7 天内 / 更早 sections).
type sessionEntry struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	LastActiveMs int64  `json:"last_active_ms"`
}

// sessionsResponse is the GET /api/sessions wire shape. Sessions is
// always non-nil (empty array, not null) so the FE can `.map` safely.
type sessionsResponse struct {
	Sessions []sessionEntry `json:"sessions"`
}

// fallbackTitle surfaces when a roster row carries neither an override
// nor a derived title (defensive — AppendTurn always writes a user row
// first; an assistant-only session would hit this).
const fallbackTitle = "未命名会话"

// sessionsListHandler implements GET /api/sessions (session-management
// phase). Returns the full session roster derived from session_turn,
// newest-activity-first, with the session_meta rename override merged
// into a single Title field.
//
// Status codes:
//
//	200 — roster (empty array on an empty DB)
//	404 — Deps.Store == nil (defensive misconfiguration)
//	500 — database error
func (s *Deps) sessionsListHandler(ctx context.Context, c *app.RequestContext) {
	if s.Store == nil {
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "session store not initialized",
		})
		return
	}

	rows, err := s.Store.ListSessions(ctx)
	if err != nil {
		log.Printf("[WARN] server: sessions list failed: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "sessions list failed",
		})
		return
	}

	out := make([]sessionEntry, 0, len(rows))
	for _, r := range rows {
		title := r.Title
		if title == "" {
			title = r.DerivedTitle
		}
		if title == "" {
			title = fallbackTitle
		}
		out = append(out, sessionEntry{
			ID:           r.ID,
			Title:        title,
			LastActiveMs: r.LastActive.UnixMilli(),
		})
	}

	c.JSON(http.StatusOK, sessionsResponse{Sessions: out})
}

// renameTitleLimit caps a user-supplied rename title in RUNES
// (Chinese-first product — rune count, not bytes). 120 runes ≈ 3x the
// derived-title cap; generous for a label, bounded for a wire payload.
const renameTitleLimit = 120

// renameRequest is the PATCH /api/session/:id body.
type renameRequest struct {
	Title string `json:"title"`
}

// sessionRenameHandler implements PATCH /api/session/:id
// (session-management phase). Upserts the session_meta title override.
//
// Behavioral contract:
//
//  1. sessionIDRegex validates the URL segment BEFORE any SQL
//     (T-10-11) — malformed → 400.
//  2. The title is trimmed; empty (after trim) or > 120 runes → 400.
//     The UTF-8 guard mirrors chat.go's rationale: a legacy-codepage
//     client (GBK curl) would otherwise persist U+FFFD mojibake as the
//     visible title.
//  3. RenameSession is an UPSERT — an unknown-but-valid sessionID is
//     200 (pre-seeds the override; Store.RenameSession documents this
//     as legal). NO existence signal leaks (T-10-11 posture).
//
// Status codes:
//
//	200 — override upserted (or pre-seeded for an unknown id)
//	400 — malformed session id / unreadable body / empty or over-long title
//	404 — Deps.Store == nil (defensive misconfiguration)
//	500 — database error
func (s *Deps) sessionRenameHandler(ctx context.Context, c *app.RequestContext) {
	if s.Store == nil {
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "session store not initialized",
		})
		return
	}

	sessionID := c.Param("id")
	if !sessionIDRegex.MatchString(sessionID) {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid session id",
		})
		return
	}

	if raw := c.Request.Body(); len(raw) > 0 && !utf8.Valid(raw) {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "request body must be valid UTF-8",
		})
		return
	}

	var req renameRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "malformed json body",
		})
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "title must not be empty",
		})
		return
	}
	if utf8.RuneCountInString(title) > renameTitleLimit {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "title exceeds 120 characters",
		})
		return
	}

	if err := s.Store.RenameSession(ctx, sessionID, title); err != nil {
		log.Printf("[WARN] server: session rename (sess=%s) failed: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "session rename failed",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]string{"ok": "true"})
}

// sessionDeleteHandler implements DELETE /api/session/:id
// (session-management phase). Drops the session's turns AND its
// session_meta row atomically (Store.DeleteSessionTurns); audit rows
// are deliberately untouched (T-3-AUDIT immutable trail).
//
// IDEMPOTENT: an unknown-but-valid sessionID is still 204 (deleting
// zero rows is a successful delete) — no existence signal leaks
// (T-10-11 posture).
//
// Status codes:
//
//	204 — turns + meta removed (or nothing to remove)
//	400 — malformed session id
//	404 — Deps.Store == nil (defensive misconfiguration)
//	500 — database error
func (s *Deps) sessionDeleteHandler(ctx context.Context, c *app.RequestContext) {
	if s.Store == nil {
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "session store not initialized",
		})
		return
	}

	sessionID := c.Param("id")
	if !sessionIDRegex.MatchString(sessionID) {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid session id",
		})
		return
	}

	if err := s.Store.DeleteSessionTurns(ctx, sessionID); err != nil {
		log.Printf("[WARN] server: session delete (sess=%s) failed: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "session delete failed",
		})
		return
	}

	c.SetStatusCode(http.StatusNoContent)
}

// Compile-time interface guard: the three handlers above close over the
// Store methods this file depends on — keeps a future Store refactor
// from silently dropping the roster surface.
var _ interface {
	ListSessions(ctx context.Context) ([]store.SessionSummary, error)
	RenameSession(ctx context.Context, sessionID, title string) error
	DeleteSessionTurns(ctx context.Context, sessionID string) error
} = (store.Store)(nil)
