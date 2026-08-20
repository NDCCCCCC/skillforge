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

// Plan 11-05 / HITL-08 / D11-D4 — the Approval Rules panel backend.
//
// Wire contract:
//
//	GET /api/session/:id/approval-rules?scope=session|skill[&skill_id=<skill>]
//	    → 200 {"rules": [...ApprovalRuleJSON]}   success
//	    → 400 {error: "..."}                      invalid scope / missing skill_id
//	    → 500 {error: "..."}                      QueryRules failed (generic envelope)
//	    → 503 {error: "..."}                      Deps.Store nil or lacks rule surface
//
//	DELETE /api/session/:id/approval-rules/:ruleId[?reason=<text>]
//	    → 200 {"status":"revoked","rule":{...}}   rule DELETEd + audit row INSERTed (same tx)
//	    → 400 {error: "..."}                      malformed ruleId
//	    → 404 {error: "..."}                      no rule row for the PK
//	    → 500 {error: "..."}                      Revoke failed (generic envelope)
//	    → 503 {error: "..."}                      Deps.Store nil or lacks rule surface
//
// The ruleId path param is base64url-encoded JSON carrying the full
// rule PK {target, args_hash, skill_id, scope_session_id} — the GET
// response embeds it per rule so the FE never reconstructs the key.
//
// Threat model (11-05 threat_model):
//
//   - T-3-SQLI — every value reaches the store via `?` placeholders;
//     the handler never concatenates SQL.
//   - T-11-05-03/04 — scope validation at the boundary rejects anything
//     outside the D11-D3 binary {"session","skill"}; there is no
//     "global" scope value to smuggle in.
//   - D11-D4 — revoke delegates to store.Revoke, which runs the
//     DELETE + action_rule_revoked audit INSERT in ONE transaction
//     (T-3-AUDIT: the audit row is the revoke evidence).
//
// Router registration is NOT in this file's blast radius (the touched
// list for 11-05 contains no router file); the owning plan wires
// s.ListApprovalRulesHandler / s.RevokeApprovalRuleHandler into the
// Hertz router alongside the other session-scoped endpoints.
package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// ApprovalRuleJSON is the wire shape of one approve-and-remember rule.
// Scope is derived: scope_session_id != "" → "session", else "skill"
// (D11-D3 binary scope — no third value exists).
type ApprovalRuleJSON struct {
	Target         string    `json:"target"`
	ArgsHash       string    `json:"args_hash"`
	SkillID        string    `json:"skill_id"`
	ScopeSessionID string    `json:"scope_session_id"`
	Scope          string    `json:"scope"`
	CreatedAt      time.Time `json:"created_at"`
	LastUsedAt     time.Time `json:"last_used_at"`
	// RuleID is the opaque handle the DELETE endpoint consumes
	// (base64url JSON of the rule PK). Embedded so the FE never
	// reconstructs the key client-side.
	RuleID string `json:"rule_id"`
}

// ListApprovalRulesResponse is the GET envelope.
type ListApprovalRulesResponse struct {
	Rules []ApprovalRuleJSON `json:"rules"`
}

// RevokeApprovalRuleResponse is the DELETE envelope.
type RevokeApprovalRuleResponse struct {
	Status string           `json:"status"`
	Rule   ApprovalRuleJSON `json:"rule"`
}

// ruleIDPayload is the decoded form of the :ruleId path param — the
// full D11-D2 primary key.
type ruleIDPayload struct {
	Target         string `json:"target"`
	ArgsHash       string `json:"args_hash"`
	SkillID        string `json:"skill_id"`
	ScopeSessionID string `json:"scope_session_id"`
}

// approvalRulesStoreOf unwraps the narrow ApprovalRulesStore surface
// from Deps.Store. Production wires *sqlite.SQLiteStore which satisfies
// it; a degraded server (nil Store) or a backend without the rule
// surface yields ok=false → the handlers answer 503.
func (s *Deps) approvalRulesStoreOf() (sqlite.ApprovalRulesStore, bool) {
	if s.Store == nil {
		return nil, false
	}
	ars, ok := s.Store.(sqlite.ApprovalRulesStore)
	return ars, ok
}

// ListApprovalRulesHandler implements GET
// /api/session/:id/approval-rules?scope=session|skill.
func (s *Deps) ListApprovalRulesHandler(ctx context.Context, c *app.RequestContext) {
	ars, ok := s.approvalRulesStoreOf()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "approval-rules store not initialized (set Deps.Store at server construction)",
		})
		return
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "session id is required"})
		return
	}

	// D11-D3 binary scope — anything outside {"session","skill"} is a
	// 400. There is no "global" scope to list.
	var scope sqlite.Scope
	switch string(c.Query("scope")) {
	case "session":
		scope = sqlite.ScopeSession
	case "skill":
		scope = sqlite.ScopeSkill
	default:
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "scope must be one of {session, skill} (D11-D3 binary scope)",
		})
		return
	}

	skillID := string(c.Query("skill_id"))
	if scope == sqlite.ScopeSkill && skillID == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "skill_id is required when scope=skill",
		})
		return
	}

	rules, err := ars.QueryRules(ctx, sessionID, skillID, scope)
	if err != nil {
		log.Printf("[WARN] server: approval-rules query failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "approval-rules query failed — see server logs",
		})
		return
	}

	out := ListApprovalRulesResponse{Rules: make([]ApprovalRuleJSON, 0, len(rules))}
	for _, r := range rules {
		out.Rules = append(out.Rules, toApprovalRuleJSON(r))
	}
	c.JSON(http.StatusOK, out)
}

// RevokeApprovalRuleHandler implements DELETE
// /api/session/:id/approval-rules/:ruleId. The revoke delegates to
// store.Revoke — DELETE approval_rules row + INSERT audit
// action_rule_revoked in a single transaction (D11-D4).
func (s *Deps) RevokeApprovalRuleHandler(ctx context.Context, c *app.RequestContext) {
	ars, ok := s.approvalRulesStoreOf()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "approval-rules store not initialized (set Deps.Store at server construction)",
		})
		return
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "session id is required"})
		return
	}

	target, argsHash, skillID, scopeSessionID, err := decodeRuleID(c.Param("ruleId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "malformed ruleId: " + err.Error(),
		})
		return
	}

	userID := userIDFromCtx(ctx)

	// 11-05 stub: the active manifest for the session is resolved by
	// the Phase 12+ session store. The two audit-row snapshot values
	// (skill_version, prompt_hash) degrade to "" — the revoke audit
	// row's identity is the rule PK in frozen_args, which is complete
	// without them.
	manifest, err := manifestFromSession(ctx, sessionID, skillID)
	if err != nil {
		log.Printf("[WARN] server: approval-rules manifest lookup failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "approval-rules manifest lookup failed — see server logs",
		})
		return
	}
	skillVersion, promptHash := "", ""
	if manifest != nil {
		skillVersion, promptHash = snapshotFromManifest(manifest)
	}

	reason := string(c.Query("reason"))
	if reason == "" {
		reason = "approval-rules panel revoke"
	}

	if err := ars.Revoke(ctx, sessionID, userID, target, argsHash, skillID, scopeSessionID, skillVersion, promptHash, reason); err != nil {
		if strings.Contains(err.Error(), "no rule found") {
			c.JSON(http.StatusNotFound, graphResponseError{
				Error: "no approval rule found for the given ruleId",
			})
			return
		}
		log.Printf("[WARN] server: approval-rules revoke failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "approval-rules revoke failed — see server logs",
		})
		return
	}

	c.JSON(http.StatusOK, RevokeApprovalRuleResponse{
		Status: "revoked",
		Rule: toApprovalRuleJSON(sqlite.ApprovalRule{
			Target:         target,
			ArgsHash:       argsHash,
			SkillID:        skillID,
			ScopeSessionID: scopeSessionID,
		}),
	})
}

// toApprovalRuleJSON maps the store row to the wire shape, deriving
// the D11-D3 scope string and embedding the opaque rule_id handle.
func toApprovalRuleJSON(r sqlite.ApprovalRule) ApprovalRuleJSON {
	scope := "skill"
	if r.ScopeSessionID != "" {
		scope = "session"
	}
	return ApprovalRuleJSON{
		Target:         r.Target,
		ArgsHash:       r.ArgsHash,
		SkillID:        r.SkillID,
		ScopeSessionID: r.ScopeSessionID,
		Scope:          scope,
		CreatedAt:      r.CreatedAt,
		LastUsedAt:     r.LastUsedAt,
		RuleID:         encodeRuleID(r),
	}
}

// encodeRuleID packs the rule PK as base64url(JSON) — the opaque
// handle the FE passes back to the DELETE endpoint.
func encodeRuleID(rule sqlite.ApprovalRule) string {
	payload, err := json.Marshal(ruleIDPayload{
		Target:         rule.Target,
		ArgsHash:       rule.ArgsHash,
		SkillID:        rule.SkillID,
		ScopeSessionID: rule.ScopeSessionID,
	})
	if err != nil {
		// json.Marshal on a 4-string struct cannot fail; defensive.
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

// decodeRuleID is the inverse of encodeRuleID. target, args_hash, and
// skill_id must be non-empty; scope_session_id may be empty (the
// D11-D3 encoding of a skill-scope rule).
func decodeRuleID(ruleID string) (target, argsHash, skillID, scopeSessionID string, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(ruleID)
	if err != nil {
		return "", "", "", "", fmt.Errorf("base64url decode: %w", err)
	}
	var payload ruleIDPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", "", "", "", fmt.Errorf("json decode: %w", err)
	}
	if payload.Target == "" || payload.ArgsHash == "" || payload.SkillID == "" {
		return "", "", "", "", fmt.Errorf("ruleId payload requires non-empty target, args_hash, skill_id")
	}
	return payload.Target, payload.ArgsHash, payload.SkillID, payload.ScopeSessionID, nil
}

// userIDFromCtx extracts the operator identity from the request ctx.
// 11-05 reads the "user_id" ctx value; Phase 12 wires real auth.
// Absent → "" (the audit row's user_id column tolerates empty).
func userIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value("user_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// manifestFromSession resolves the active Skill manifest for a
// session. 11-05 STUB: returns (nil, nil) — the Phase 12+ session
// store owns the real lookup. Callers treat a nil manifest as
// "snapshot values unknown" and degrade skill_version / prompt_hash
// to "".
func manifestFromSession(_ context.Context, _, _ string) (*skill.Manifest, error) {
	return nil, nil
}

// snapshotFromManifest derives the (skill_version, prompt_hash) audit
// snapshot from a manifest. With the 11-05 stub always returning nil
// manifests this is unreachable today; it is the documented seam the
// Phase 12+ manifest lookup feeds. skill.Manifest carries no Version
// field (see hitl.SkillSnapshot's documented deviation), so both
// values degrade to "" until the session store provides them.
func snapshotFromManifest(_ *skill.Manifest) (skillVersion, promptHash string) {
	return "", ""
}
