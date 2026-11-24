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
// Phase 56 / Plan 56-01 (D-56-01) upgrades the wire contract ADDITIVELY
// (every pre-56 field survives; the FE union already carries
// 'rule-matched', wired in 56-02/56-03):
//
//	GET /api/session/:id/approval-rules?scope=session|skill|global[&skill_id=<skill>]
//	    → 200 {"rules": [...ApprovalRuleJSON]}   success
//	    → 400 {error: "..."}                      invalid scope / missing skill_id
//	    → 500 {error: "..."}                      QueryRules failed (generic envelope)
//	    → 503 {error: "..."}                      Deps.Store nil or lacks rule surface
//
//	DELETE /api/session/:id/approval-rules/:ruleId[?reason=<text>]
//	    → 200 {"status":"revoked","rule":{...}}   rule DELETEd + audit row INSERTed (same tx)
//	    → 400 {error: "..."}                      malformed ruleId
//	    → 404 {error: "..."}                      no rule row for the id
//	    → 500 {error: "..."}                      RevokeByID failed (generic envelope)
//	    → 503 {error: "..."}                      Deps.Store nil or lacks rule surface
//
// The ruleId path param is base64url-encoded JSON {"id": <int64>} —
// the surrogate PK since the 56-01 table rebuild (the legacy full-PK
// payload {target, args_hash, skill_id, scope_session_id} is retired:
// the pre-56 FE never parses rule_id, it round-trips the opaque string
// the GET response embedded, so re-encoding the handle is
// FE-transparent). The GET response embeds it per rule so the FE never
// reconstructs the key.
//
// Scope semantics (D-56-01 three-layer scope, replacing the D11-D3
// binary scope): scope=session lists the rules bound to :id;
// scope=skill lists the rules bound to skill_id across sessions
// (skill_id REQUIRED); scope=global lists the cross-Skill rules
// (skill_id NOT required). Listing is an ACTIVE-rules view — the store
// filters expired rules by default.
//
// Threat model (11-05 threat model, carried into 56-01):
//
//   - T-3-SQLI — every value reaches the store via `?` placeholders;
//     the handler never concatenates SQL. Scope is additionally
//     enum-validated at this boundary (exactly three values), so there
//     is no fourth value to smuggle into the store's switch.
//   - T-11-05-03/04 — superseded by the D-56-01 three-value boundary
//     check above: {"session","skill","global"} is the closed set.
//   - D11-D4 — revoke delegates to store.RevokeByID, which runs the
//     SELECT-snapshot + DELETE + action_rule_revoked audit INSERT in
//     ONE transaction (T-3-AUDIT: the audit row is the revoke
//     evidence, now carrying rule_id/match_type/scope alongside the
//     legacy PK fields).
//
// Router registration is NOT in this file's blast radius; the owning
// plan wires s.ListApprovalRulesHandler / s.RevokeApprovalRuleHandler
// into the Hertz router alongside the other session-scoped endpoints.
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
//
// 56-01 additive fields (D-56-01): match_type, args_pattern, enabled,
// expires_at, origin, hit_count, last_hit_at, id. The pre-56 fields
// (target, args_hash, skill_id, scope_session_id, scope, created_at,
// last_used_at, rule_id) all survive unchanged — the old consumers
// keep rendering.
//
//   - scope is now READ FROM THE STORED COLUMN (session | skill |
//     global) instead of derived from scope_session_id.
//   - args_hash is ” for prefix/template rules (exact-only).
//   - last_used_at is a compat shim: the store mirrors it from
//     last_hit_at (HitRule writes both), so old FE keeps rendering;
//     retires after 56-03 switches the panel to last_hit_at.
//   - expires_at / last_hit_at are RFC3339 or null (never-expiring /
//     never-hit rules).
type ApprovalRuleJSON struct {
	ID             int64      `json:"id"`
	Target         string     `json:"target"`
	MatchType      string     `json:"match_type"`
	ArgsHash       string     `json:"args_hash"`
	ArgsPattern    string     `json:"args_pattern"`
	SkillID        string     `json:"skill_id"`
	Scope          string     `json:"scope"`
	ScopeSessionID string     `json:"scope_session_id"`
	Enabled        bool       `json:"enabled"`
	ExpiresAt      *time.Time `json:"expires_at"`
	Origin         string     `json:"origin"`
	HitCount       int64      `json:"hit_count"`
	LastHitAt      *time.Time `json:"last_hit_at"`
	CreatedAt      time.Time  `json:"created_at"`
	// LastUsedAt is the compat shim (== last_hit_at at the store
	// layer). Retires after the 56-03 panel reads last_hit_at.
	LastUsedAt time.Time `json:"last_used_at"`
	// RuleID is the opaque handle the DELETE endpoint consumes
	// (base64url JSON of {"id": <int64>}). Embedded so the FE never
	// reconstructs the key client-side.
	RuleID string `json:"rule_id"`
}

// ListApprovalRulesResponse is the GET envelope.
type ListApprovalRulesResponse struct {
	Rules []ApprovalRuleJSON `json:"rules"`
}

// RevokeApprovalRuleResponse is the DELETE envelope. Rule carries only
// the revoked rule's ID reliably — the row is already deleted when the
// response is built (its full shape lives on in the action_rule_revoked
// audit row, not in this payload).
type RevokeApprovalRuleResponse struct {
	Status string           `json:"status"`
	Rule   ApprovalRuleJSON `json:"rule"`
}

// ruleIDPayload is the decoded form of the :ruleId path param — the
// surrogate PK introduced by the 56-01 table rebuild.
type ruleIDPayload struct {
	ID int64 `json:"id"`
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
// /api/session/:id/approval-rules?scope=session|skill|global.
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

	// D-56-01 three-layer scope — anything outside
	// {"session","skill","global"} is a 400. The closed-set boundary
	// check leaves the store's switch no fourth value to receive
	// (T-11-05-03/04 posture, carried into the three-scope model).
	var scope sqlite.RuleScope
	switch string(c.Query("scope")) {
	case "session":
		scope = sqlite.RuleScopeSession
	case "skill":
		scope = sqlite.RuleScopeSkill
	case "global":
		scope = sqlite.RuleScopeGlobal
	default:
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "scope must be one of {session, skill, global} (D-56-01 three-layer scope)",
		})
		return
	}

	skillID := string(c.Query("skill_id"))
	if scope == sqlite.RuleScopeSkill && skillID == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "skill_id is required when scope=skill",
		})
		return
	}
	// scope=global deliberately does NOT require skill_id — a global
	// rule applies across every Skill (skill_id may be '').

	rules, err := ars.QueryRules(ctx, sessionID, skillID, scope)
	if err != nil {
		log.Printf("[WARN] server: approval-rules query failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "approval-rules query failed — see server logs",
		})
		return
	}

	// D11-D2 single-table reuse — the FE splits approval_rules by
	// target (memory:facts / review_report / plan_approve). The BE
	// filters in-memory after QueryRules; future growth can push this
	// into the SQL WHERE clause without breaking the FE contract.
	targetFilter := string(c.Query("target"))

	out := ListApprovalRulesResponse{Rules: make([]ApprovalRuleJSON, 0, len(rules))}
	for _, r := range rules {
		if targetFilter != "" && r.Target != targetFilter {
			continue
		}
		out.Rules = append(out.Rules, toApprovalRuleJSON(r))
	}
	c.JSON(http.StatusOK, out)
}

// RevokeApprovalRuleHandler implements DELETE
// /api/session/:id/approval-rules/:ruleId. The revoke delegates to
// store.RevokeByID — SELECT snapshot + DELETE approval_rules row +
// INSERT audit action_rule_revoked in a single transaction (D11-D4).
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

	ruleID, err := decodeRuleID(c.Param("ruleId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "malformed ruleId: " + err.Error(),
		})
		return
	}

	userID := userIDFromCtx(ctx)

	// 11-05 stub (unchanged): the active manifest for the session is
	// resolved by the Phase 12+ session store. The two audit-row
	// snapshot values (skill_version, prompt_hash) degrade to "" — the
	// revoke audit row's identity is the frozen rule snapshot in
	// frozen_args, which is complete without them.
	manifest, err := manifestFromSession(ctx, sessionID, "")
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

	if err := ars.RevokeByID(ctx, sessionID, userID, ruleID, skillVersion, promptHash, reason); err != nil {
		// The store's zero-row error carries the literal "no rule
		// found" — the 404 classifier (same contract as 11-05).
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

	// The rule row is already deleted — only its ID is reliably known
	// here. The full revoked shape lives on in the action_rule_revoked
	// audit row's frozen_args snapshot (T-3-AUDIT).
	c.JSON(http.StatusOK, RevokeApprovalRuleResponse{
		Status: "revoked",
		Rule:   toApprovalRuleJSON(sqlite.ApprovalRule{ID: ruleID}),
	})
}

// toApprovalRuleJSON maps the store row to the wire shape, reading the
// D-56-01 fields straight off the struct (no scope derivation — the
// scope column is authoritative) and embedding the opaque rule_id
// handle.
func toApprovalRuleJSON(r sqlite.ApprovalRule) ApprovalRuleJSON {
	out := ApprovalRuleJSON{
		ID:             r.ID,
		Target:         r.Target,
		MatchType:      r.MatchType,
		ArgsHash:       r.ArgsHash,
		ArgsPattern:    r.ArgsPattern,
		SkillID:        r.SkillID,
		Scope:          r.Scope,
		ScopeSessionID: r.ScopeSessionID,
		Enabled:        r.Enabled,
		Origin:         r.Origin,
		HitCount:       r.HitCount,
		CreatedAt:      r.CreatedAt,
		// Compat shim: the store mirrors last_used_at from last_hit_at
		// (HitRule writes both), so the pre-56 FE keeps rendering until
		// the 56-03 panel switches to last_hit_at.
		LastUsedAt: r.LastUsedAt,
		RuleID:     encodeRuleID(r),
	}
	if r.ExpiresAt > 0 {
		t := time.Unix(r.ExpiresAt, 0).UTC()
		out.ExpiresAt = &t
	}
	if !r.LastHitAt.IsZero() {
		t := r.LastHitAt
		out.LastHitAt = &t
	}
	return out
}

// encodeRuleID packs the rule surrogate PK as base64url(JSON) — the
// opaque handle the FE passes back to the DELETE endpoint.
func encodeRuleID(rule sqlite.ApprovalRule) string {
	payload, err := json.Marshal(ruleIDPayload{ID: rule.ID})
	if err != nil {
		// json.Marshal on an int64 field cannot fail; defensive.
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

// decodeRuleID is the inverse of encodeRuleID. The id must be a
// positive int64 — a surrogate PK of 0 or below cannot exist
// (AUTOINCREMENT starts at 1), so anything else is a malformed or
// forged handle.
func decodeRuleID(ruleID string) (int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(ruleID)
	if err != nil {
		return 0, fmt.Errorf("base64url decode: %w", err)
	}
	var payload ruleIDPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0, fmt.Errorf("json decode: %w", err)
	}
	if payload.ID <= 0 {
		return 0, fmt.Errorf("ruleId payload requires a positive id")
	}
	return payload.ID, nil
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

// createApprovalRulePayload is the POST body (54-03 PANEL-01 CRUD).
// match_type picks the tier; args_hash is required ONLY for exact;
// args_pattern (a JSON object of locked fields) is required ONLY for
// prefix. scope is the three-layer model (session/skill/global).
type createApprovalRulePayload struct {
Target       string `json:"target"`
MatchType    string `json:"match_type"`
ArgsHash     string `json:"args_hash"`
ArgsPattern  string `json:"args_pattern"`
SkillID      string `json:"skill_id"`
Scope        string `json:"scope"`
ScopeSessionID string `json:"scope_session_id"`
ExpiresAt    string `json:"expires_at"` // RFC3339; "" = permanent
}

// CreateApprovalRuleHandler implements POST
// /api/session/:id/approval-rules (54-03 PANEL-01: rules become
// first-class CRUD objects instead of an implicit approve-and-remember
// side effect -- the PANEL-03 empty-state guidance links here).
// origin is always 'manual' (approve_remember rows come only from the
// approval decision path; seed rows only from migrations).
func (s *Deps) CreateApprovalRuleHandler(ctx context.Context, c *app.RequestContext) {
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

var body createApprovalRulePayload
rawBody, bodyErr := c.Body()
	if bodyErr != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "body read failed"})
		return
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid JSON body: " + err.Error()})
return
}
if body.Target == "" {
c.JSON(http.StatusBadRequest, graphResponseError{Error: "target is required"})
return
}

// match_type closed-set + per-tier field requirements (56-02 engine
// contract mirrored at the boundary).
switch body.MatchType {
case "exact":
if body.ArgsHash == "" {
c.JSON(http.StatusBadRequest, graphResponseError{Error: "args_hash is required when match_type=exact"})
return
}
case "prefix":
if body.ArgsPattern == "" {
c.JSON(http.StatusBadRequest, graphResponseError{Error: "args_pattern is required when match_type=prefix"})
return
}
case "template":
// no per-tier field
default:
c.JSON(http.StatusBadRequest, graphResponseError{Error: "match_type must be one of {exact, prefix, template}"})
return
}

var scope sqlite.RuleScope
switch body.Scope {
case "session":
scope = sqlite.RuleScopeSession
case "skill":
scope = sqlite.RuleScopeSkill
if body.SkillID == "" {
c.JSON(http.StatusBadRequest, graphResponseError{Error: "skill_id is required when scope=skill"})
return
}
case "global":
scope = sqlite.RuleScopeGlobal
default:
c.JSON(http.StatusBadRequest, graphResponseError{Error: "scope must be one of {session, skill, global}"})
return
}

var expiresAt time.Time
if body.ExpiresAt != "" {
t, err := time.Parse(time.RFC3339, body.ExpiresAt)
if err != nil {
c.JSON(http.StatusBadRequest, graphResponseError{Error: "expires_at must be RFC3339"})
return
}
expiresAt = t
} // .Unix() at the struct literal; zero time -> 0 = perpetual

rule := sqlite.ApprovalRule{
Target:       body.Target,
MatchType:    body.MatchType,
ArgsHash:     body.ArgsHash,
ArgsPattern:  body.ArgsPattern,
SkillID:      body.SkillID,
Scope:        string(scope),
ScopeSessionID: scopeSessionIDFor(sessionID, body.Scope, body.ScopeSessionID),
Enabled:      true, // explicit bool: the store treats the zero value as DISABLED (56-01 godoc)
ExpiresAt:    expiresAt.Unix(), // 0 = never expires (zero-time guard)
Origin:       "manual",
}
if err := ars.CreateRule(ctx, rule); err != nil {
log.Printf("[WARN] server: approval-rule create failed: %v", err)
c.JSON(http.StatusInternalServerError, graphResponseError{Error: "approval-rule create failed -- see server logs"})
return
}
c.JSON(http.StatusCreated, map[string]any{"ok": true, "target": rule.Target, "match_type": rule.MatchType, "scope": rule.Scope})
}

// scopeSessionIDFor resolves the stored scope_session_id: session scope
// pins the route's :id; skill/global store ''.
func scopeSessionIDFor(routeSessionID, scope, explicit string) string {
if scope == "session" {
if explicit != "" {
return explicit
}
return routeSessionID
}
return ""
}

// patchApprovalRulePayload is the PATCH body (54-03 PANEL-01): only the
// operationally-mutable fields -- enabled and expires_at. The semantic
// identity (scope/match_type/args_*) is NOT patchable: editing those =
// delete + create two steps (the plan-specified shape), because a rule's
// identity IS its match semantics.
type patchApprovalRulePayload struct {
Enabled   *bool  `json:"enabled"`
ExpiresAt *string `json:"expires_at"` // RFC3339; "" clears to permanent
}

// PatchApprovalRuleHandler implements PATCH
// /api/session/:id/approval-rules/:ruleId (54-03 PANEL-01: the enable
// toggle and TTL edit are the two operations the rules panel needs
// without deleting and recreating the rule).
func (s *Deps) PatchApprovalRuleHandler(ctx context.Context, c *app.RequestContext) {
ars, ok := s.approvalRulesStoreOf()
if !ok {
c.JSON(http.StatusServiceUnavailable, graphResponseError{
Error: "approval-rules store not initialized (set Deps.Store at server construction)",
})
return
}
sessionID := c.Param("id")
ruleIDStr := c.Param("ruleId")
ruleID, err := decodeRuleID(ruleIDStr)
if err != nil {
c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid rule_id"})
return
}

var body patchApprovalRulePayload
rawBody, bodyErr := c.Body()
	if bodyErr != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "body read failed"})
		return
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid JSON body: " + err.Error()})
return
}
if body.Enabled == nil && body.ExpiresAt == nil {
c.JSON(http.StatusBadRequest, graphResponseError{Error: "at least one of {enabled, expires_at} is required"})
return
}

if body.Enabled != nil {
if err := ars.SetRuleEnabled(ctx, ruleID, *body.Enabled); err != nil {
log.Printf("[WARN] server: approval-rule enable toggle failed: %v", err)
c.JSON(http.StatusInternalServerError, graphResponseError{Error: "approval-rule update failed -- see server logs"})
return
}
}
if body.ExpiresAt != nil {
var exp time.Time
if *body.ExpiresAt != "" {
t, perr := time.Parse(time.RFC3339, *body.ExpiresAt)
if perr != nil {
c.JSON(http.StatusBadRequest, graphResponseError{Error: "expires_at must be RFC3339"})
return
}
exp = t
}
if err := ars.SetRuleExpiry(ctx, ruleID, exp.Unix()); err != nil {
log.Printf("[WARN] server: approval-rule expiry set failed: %v", err)
c.JSON(http.StatusInternalServerError, graphResponseError{Error: "approval-rule update failed -- see server logs"})
return
}
}
_ = sessionID // reserved: a per-session authorization check lands here
c.JSON(http.StatusOK, map[string]any{"ok": true, "rule_id": ruleID})
}

// ruleSuggestion is one cluster of repeated HUMAN approvals that has no
// rule yet (54-03 PANEL-03 empty-state guidance's data half). v1 clusters
// by target only (the prefix-tier field-lock pattern needs frozenArgs
// shape analysis — deferred; the one-click create lands a TEMPLATE rule,
// which needs no field analysis and is the safest broadens).
type ruleSuggestion struct {
Target      string `json:"target"`
Occurrences int    `json:"occurrences"`
// MatchType is fixed "template" in v1 (see above); the FE suggestion
// card copies it into createApprovalRule payload verbatim.
MatchType string `json:"match_type"`
}

// listRuleSuggestionsHandler implements GET
// /api/session/:id/approval-rule-suggestions (54-03 PANEL-03). Source:
// the audit table's action_approved rows for the session, clustered by
// the frozen_args target field; clusters with <2 occurrences and targets
// that ALREADY have a rule are filtered out (a suggestion for an
// existing rule is noise).
func (s *Deps) listRuleSuggestionsHandler(ctx context.Context, c *app.RequestContext) {
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

rows, err := s.Store.ListApprovedFrozenArgs(ctx, sessionID)
if err != nil {
log.Printf("[WARN] server: rule suggestions scan failed: %v", err)
c.JSON(http.StatusInternalServerError, graphResponseError{Error: "suggestions scan failed -- see server logs"})
return
}

counts := map[string]int{}
for _, raw := range rows {
var probe struct {
Target string `json:"target"`
}
if json.Unmarshal([]byte(raw), &probe) != nil || probe.Target == "" {
continue // malformed/missing target: skip, never die on one row
}
counts[probe.Target]++
}

// Existing-rule filter: targets with ANY rule (any scope/match) for
// this session are already covered — suggesting them is noise.
existing := map[string]bool{}
if rules, err := ars.QueryRules(ctx, sessionID, "", sqlite.RuleScopeSession); err == nil {
for _, r := range rules {
existing[r.Target] = true
}
}
if rules, err := ars.QueryRules(ctx, sessionID, "", sqlite.RuleScopeSkill); err == nil {
for _, r := range rules {
existing[r.Target] = true
}
}
if rules, err := ars.QueryRules(ctx, sessionID, "", sqlite.RuleScopeGlobal); err == nil {
for _, r := range rules {
existing[r.Target] = true
}
}

out := make([]ruleSuggestion, 0, len(counts))
for target, n := range counts {
if n < 2 || existing[target] {
continue
}
out = append(out, ruleSuggestion{Target: target, Occurrences: n, MatchType: "template"})
}
c.JSON(http.StatusOK, map[string]any{"suggestions": out})
}
