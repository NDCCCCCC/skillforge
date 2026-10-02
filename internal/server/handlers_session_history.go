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
	"regexp"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/history"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
)

// sessionIDRegex enforces the SC-9 / T-10-11 session-id shape. The
// server-side newSessionID emits 32 lowercase hex chars (T-3-SESSIONID,
// ASVS V6); the fallback counter path emits 32 hex chars too. A
// 8–128 char range covers both shapes plus future migration targets
// while rejecting obvious injection attempts (whitespace, '/', quotes).
//
// CRITICAL (T-10-11): every URL segment other than this regex is a
// 400 — the handler MUST reject malformed input BEFORE issuing any
// SQL query so a malformed session_id cannot reach the store layer.
var sessionIDRegex = regexp.MustCompile(`^[a-zA-Z0-9-]{8,128}$`)

// historyEntry is one rendered session turn in the response payload.
// The wire shape (Field JSON tags) is the FE chat-restore contract —
// see web/src/lib/chat-restore.ts SessionHistoryMessage.
//
//	Role    "user" | "assistant"
//	Content human-readable text (rendered per role)
//	Verdict parsed verdict.Verdict when the assistant row carries a
//	        verdict JSON envelope; nil for user rows AND for refusal /
//	        non-verdict assistant rows
//	Ts      epoch ms (FE renders this for stable message IDs)
//
// Phase 29 / Plan 29-04 (D-29-03) — the turn-identity echo. TurnID is
// the row's session_turn.id (the FBK-01 message identity the FE echoes
// back as the feedback vote key); VotedVerdict is the voter's recorded
// verdict for THIS session's turn, read in ONE VotedVerdictsBySession
// batch query per request (zero extra round-trips for session restore).
// Both are assistant-only + omitempty: user rows and unvoted rows
// serialize without the keys, and the FE treats a missing turn_id as
// "feedback disabled for this row" (T-29-06: never fabricate identity).
type historyEntry struct {
	Role         string           `json:"role"`
	Content      string           `json:"content"`
	Verdict      *verdict.Verdict `json:"verdict,omitempty"`
	Ts           int64            `json:"ts"`
	TurnID       int64            `json:"turn_id,omitempty"`
	VotedVerdict string           `json:"voted_verdict,omitempty"`
}

// historyResponse is the GET /api/session/:id/history wire shape.
// Matches the SessionHistoryResponse FE type (D10-D1 + D10-D2).
type historyResponse struct {
	Messages []historyEntry `json:"messages"`
	SkillID  string         `json:"skill_id"`
}

// sessionHistoryHandler implements GET /api/session/:id/history (Phase 10
// Plan 03). The handler is the user-facing restore endpoint for
// cross-refresh conversation survival (MEM-02 / D10-D1 + D10-D2).
//
// Behavioral contract (checker B3 + B4 + W4 alignment):
//
//  1. The handler validates the URL-segment session ID against
//     sessionIDRegex; malformed input → 400.
//  2. The handler iterates the RAW rows returned by Store.GetSession.
//     There is NO history-projection call on this user-facing path
//     (the projection stays agent-path only — Plan 02). Rendering is done
//     HERE via history.RenderConclusion + history.RenderRefusal so
//     the user-facing refusal text is rendered ONLY on the
//     response payload, never entering the LLM history prefix.
//  3. The display cap (D10-B3 budget口径) applies
//     history.TruncateToCharBudget at history.DefaultCharBudget so
//     the wire payload stays bounded symmetrically with the agent
//     path's history prefix.
//  4. Unknown session ID returns 200 + empty (NOT 404) — the
//     existence-leak posture (T-10-11 / W4 alignment). A 404 would
//     tell an attacker which session IDs are valid.
//  5. SC-9 / T-3-SQLI isolation is enforced by Store.GetSession's
//     `WHERE session_id = ?` parameter binding; the regex rejects
//     obvious injection attempts before any SQL is issued.
//
// Status codes:
//
//	200 — valid session (empty or populated)
//	400 — malformed session ID
//	404 — Deps.Store == nil (defensive misconfiguration)
//	500 — database error
func (s *Deps) sessionHistoryHandler(ctx context.Context, c *app.RequestContext) {
	if s.Store == nil {
		// Defensive misconfig: a server with no Store cannot serve
		// history. Distinct from "unknown session" (which returns
		// 200 + empty per T-10-11 existence-leak posture) — a 404
		// here signals the operator that the binary is misconfigured.
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "session history store not initialized",
		})
		return
	}

	// T-10-11: validate the URL segment BEFORE any SQL. sessionIDRegex
	// matches the newSessionID output shape (32 lowercase hex chars,
	// 8–128 char range gives the FE some room for future migration).
	sessionID := c.Param("id")
	if !sessionIDRegex.MatchString(sessionID) {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid session id",
		})
		return
	}

	rows, err := s.Store.GetSession(ctx, sessionID)
	if err != nil {
		// SC-9 / T-3-SQLI: the GetSession call is parameterized; a
		// returned error here is an internal failure (sqlite closed,
		// I/O, etc.) — not a cross-session bleed. Surface as 500 with
		// a generic envelope + a server-side log.
		log.Printf("[WARN] server: history load (sess=%s) failed: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "history load failed",
		})
		return
	}

	// T-10-11: empty rows (unknown session) returns 200 + empty —
	// do NOT 404, that would leak session existence.
	//
	// 29-04 (D-29-03): ONE batch VotedVerdictsBySession query feeds the
	// whole response (never per-row queries). A read failure degrades
	// to rows without voted_verdict + a WARN — history availability
	// first; the vote echo is an enhancement, not a dependency, so it
	// must never 500 the restore path.
	voted, vErr := s.Store.VotedVerdictsBySession(ctx, sessionID)
	if vErr != nil {
		log.Printf("[WARN] server: history voted-verdict load (sess=%s) failed: %v (degrading to no voted_verdict; non-fatal)", sessionID, vErr)
		voted = nil
	}
	rendered := make([]historyEntry, 0, len(rows))
	schemaMsgs := make([]schema.Message, 0, len(rows))
	for _, m := range rows {
		entry, sMsg := renderRow(m, voted[m.ID])
		rendered = append(rendered, entry)
		schemaMsgs = append(schemaMsgs, sMsg)
	}

	// D10-B3 budget口径: bound the wire payload symmetrically with the
	// agent path. TruncateToCharBudget drops the OLDEST messages
	// first until the survivors fit history.DefaultCharBudget OR the
	// SafeguardFloor (2) is reached. The schema.Message list is the
	// shape the helper consumes (Content-based char count); map the
	// truncated survivors back to the response entries by walking
	// from the head, dropping rendered entries until len(slice) matches.
	kept, _, _ := history.TruncateToCharBudget(schemaMsgs, history.DefaultCharBudget)
	if drop := len(rendered) - len(kept); drop > 0 {
		rendered = rendered[drop:]
	}

	// skill_id: bind the session to the most recent Skill it touched.
	// Look up the most-recent audit row for this session via the
	// parameterized QueryAudit (T-3-SQLI). The query is bounded by
	// Limit=1 + SessionID filter; SC-9 isolation is enforced by the
	// parameterized WHERE clause, identical to the chat path.
	skillID := ""
	if rows != nil {
		auditRows, qErr := s.Store.QueryAudit(ctx, store.AuditFilter{
			SessionID: sessionID,
			Limit:     1,
		})
		if qErr != nil {
			// Non-fatal — the FE can still render the messages; the
			// missing skill_id falls through to empty and the
			// SkillPicker defaults to the current selection.
			log.Printf("[WARN] server: history skill_id query (sess=%s) failed: %v", sessionID, qErr)
		} else if len(auditRows) > 0 {
			skillID = auditRows[0].SkillID
		}
	}

	c.JSON(http.StatusOK, historyResponse{
		Messages: rendered,
		SkillID:  skillID,
	})
}

// renderRow turns one RAW session_turn row into the response payload
// entry + a schema.Message carrying (for the budget-cap helper).
// NO history-projection call here — the projection lives on the
// agent path (Plan 02), and MUST NOT enter the user-facing handler
// (B4 fix).
//
// Role-branch semantics (B3 contract — RenderRefusal's ONLY caller is
// this handler):
//
//   - role=="user" → verbatim content (no scrubbing, no rendering;
//     the user's exact words surface to the FE so the restored chat
//     is byte-identical to what they sent).
//   - role=="assistant" AND IsRefusalContent(content) →
//     history.RenderRefusal (refusal-semantic text on the
//     user-facing path by design; the agent path uses the neutral
//     placeholder via neutral_placeholder.go).
//   - role=="assistant" AND content parses as verdict JSON →
//     verdictRenderer.RenderConclusion + parsed verdict verdict field
//     (FE feeds the parsed verdict into DataPolicyPart → VerdictCard;
//     matches the live chat stream's data-policy shape).
//   - role=="assistant" non-verdict non-refusal → raw content
//     (debuggable; future-proof for Skills that emit plain prose).
//
// 29-04 (D-29-03): ALL FOUR assistant branches fill TurnID=m.ID (the
// session_turn identity — refusal rounds are votable too, Open Q3) and
// VotedVerdict when the batch query recorded a vote for this turn. The
// user branch fills neither (omitempty keeps the keys off the wire).
func renderRow(m store.SessionMessage, votedVerdict string) (historyEntry, schema.Message) {
	ts := m.Ts.UnixMilli()
	switch m.Role {
	case "user":
		entry := historyEntry{
			Role:    "user",
			Content: m.Content,
			Ts:      ts,
		}
		sMsg := schema.Message{
			Role:    schema.User,
			Content: m.Content,
		}
		return entry, sMsg

	case "assistant":
		// Refusal branch — refusal rounds carry the bare refusal-reason
		// code as their session_turn content (chat.go writeAuditAndTurn).
		// RenderRefusal is the handler-only consumer (B3 contract):
		// refusal-semantic words surface ONLY in this response payload.
		if history.IsRefusalContent(m.Content) {
			rendered := history.RenderRefusal(m.Content)
			entry := historyEntry{
				Role:         "assistant",
				Content:      rendered,
				Ts:           ts,
				TurnID:       m.ID,
				VotedVerdict: votedVerdict,
			}
			sMsg := schema.Message{
				Role:    schema.Assistant,
				Content: rendered,
			}
			return entry, sMsg
		}

		// Verdict-JSON branch — parse the verdict envelope so the FE
		// can feed VerdictCard verbatim. On parse failure fall back to
		// the raw content (debuggable; matches Project()'s pass-through
		// posture on agent-path rendering failures).
		var v verdict.Verdict
		if jErr := json.Unmarshal([]byte(m.Content), &v); jErr == nil {
			conclusion, rErr := history.RenderConclusion(m.Content)
			if rErr != nil {
				// RenderConclusion parses the same JSON we just
				// decoded; a parse-failure here means the verdict
				// shape was acceptable JSON but lacked a recognized
				// field. Fall back to the raw content (debuggable).
				entry := historyEntry{
					Role:         "assistant",
					Content:      m.Content,
					Verdict:      &v,
					Ts:           ts,
					TurnID:       m.ID,
					VotedVerdict: votedVerdict,
				}
				sMsg := schema.Message{
					Role:    schema.Assistant,
					Content: m.Content,
				}
				return entry, sMsg
			}
			entry := historyEntry{
				Role:         "assistant",
				Content:      conclusion,
				Verdict:      &v,
				Ts:           ts,
				TurnID:       m.ID,
				VotedVerdict: votedVerdict,
			}
			sMsg := schema.Message{
				Role:    schema.Assistant,
				Content: conclusion,
			}
			return entry, sMsg
		}

		// Non-verdict, non-refusal assistant content — plain prose
		// (legacy text; future Skills that emit non-structured text).
		entry := historyEntry{
			Role:         "assistant",
			Content:      m.Content,
			Ts:           ts,
			TurnID:       m.ID,
			VotedVerdict: votedVerdict,
		}
		sMsg := schema.Message{
			Role:    schema.Assistant,
			Content: m.Content,
		}
		return entry, sMsg
	}

	// Unknown role — defensive. Should not occur because session_turn
	// has a CHECK constraint on Role, but if it ever does, surface
	// the raw content rather than 500.
	entry := historyEntry{
		Role:    m.Role,
		Content: m.Content,
		Ts:      ts,
	}
	sMsg := schema.Message{
		Role:    schema.Assistant,
		Content: m.Content,
	}
	return entry, sMsg
}

// Compile-time guard: the history package's RenderRefusal is exported
// solely for this handler (B3 contract). This file is the single caller.