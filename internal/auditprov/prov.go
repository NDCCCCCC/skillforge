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

// Package auditprov projects the INSERT-only audit chain onto W3C PROV-JSON
// (Phase 24 Task 24-01 — the Semantica对标 "standards export" half).
//
// Design contract (24-CONTEXT.md):
//
//   - ZERO LLM. The mapping is a pure function of the input rows: same rows
//     in, byte-identical PROV-JSON out (encoding/json sorts map keys, so the
//     document is deterministic without extra effort).
//   - READ-ONLY. The package never writes; T-3-AUDIT's INSERT-only invariant
//     is untouched.
//   - Pass-through redaction. If a row already carries the retention
//     sentinel ("[REDACTED:...]"), it is exported verbatim — the projection
//     never decodes or "heals" redacted content.
//
// Mapping (locked in 24-CONTEXT.md):
//
//	audit row        → prov:Activity  sf:turn-<ID>   (ex:ChatTurn, or
//	                                   ex:HITLAction when Action != "")
//	Query            → prov:Entity    sf:query-<ID>  (ex:Query)
//	VerdictJSON      → prov:Entity    sf:verdict-<ID> (ex:Verdict; empty on
//	                                   refusals → no verdict entity)
//	ChunkIDs (CSV)   → prov:Entity    sf:chunk-<cid> (ex:Chunk)
//	UserID/SkillID/
//	ModelVersion/
//	PlatformFloorVersion → prov:Agent sf:user-… / sf:skill-… / sf:model-… /
//	                                   sf:floor-…
//
//	verdict  wasGeneratedBy turn
//	turn     used          query
//	verdict  wasDerivedFrom chunk…   (W3C derivation: the verdict was derived
//	                                  from the retrieved chunks — the
//	                                  citation-whitelist invariant expressed
//	                                  as provenance)
//	verdict  wasAttributedTo user, skill, model, floor
//
// Refusals (no verdict entity) still generate wasGeneratedBy/used on the
// TURN side via the refusal note, so a refusal's provenance is traceable
// even though nothing was asserted.
package auditprov

import (
	"fmt"
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// Namespace prefixes for the PROV-JSON document. "sf" is SkillForge's
// extension namespace; "prov" carries the W3C built-in types.
const (
	prefixSF   = "sf"
	prefixPROV = "prov"
)

// Property keys used inside entity/activity/agent bodies.
const (
	provType = "prov:type"
	exType   = "ex:type"
	exText   = "ex:text"
	exNote   = "ex:note"
)

// Document is the W3C PROV-JSON document shape. Maps are used (not slices)
// because PROV-JSON keys each record by its qualified name; encoding/json
// sorts map keys, making Marshal deterministic for identical inputs.
type Document struct {
	Entity          map[string]map[string]any `json:"entity,omitempty"`
	Activity        map[string]map[string]any `json:"activity,omitempty"`
	Agent           map[string]map[string]any `json:"agent,omitempty"`
	WasGeneratedBy  []Relation                `json:"wasGeneratedBy,omitempty"`
	Used            []Relation                `json:"used,omitempty"`
	WasAttributedTo []Attribution             `json:"wasAttributedTo,omitempty"`
	WasDerivedFrom  []Derivation              `json:"wasDerivedFrom,omitempty"`
}

// Relation is one prov:wasGeneratedBy / prov:used assertion.
type Relation struct {
	Entity   string `json:"entity"`
	Activity string `json:"activity"`
	Time     string `json:"time,omitempty"`
}

// Attribution is one prov:wasAttributedTo assertion.
type Attribution struct {
	Entity string `json:"entity"`
	Agent  string `json:"agent"`
}

// Derivation is one prov:wasDerivedFrom assertion.
type Derivation struct {
	GeneratedEntity string `json:"generatedEntity"`
	UsedEntity      string `json:"usedEntity"`
}

// FromSessionRows projects one session's audit rows onto a PROV-JSON
// document. Rows MUST already be session-filtered (the caller applies the
// SC-9 session_id WHERE clause via store.QueryAudit); the package does not
// re-check isolation.
func FromSessionRows(rows []store.AuditRow) Document {
	doc := Document{
		Entity:   map[string]map[string]any{},
		Activity: map[string]map[string]any{},
		Agent:    map[string]map[string]any{},
	}

	agent := func(key string) string {
		id := fmt.Sprintf("%s:%s", prefixSF, key)
		if _, ok := doc.Agent[id]; !ok {
			doc.Agent[id] = map[string]any{provType: fmt.Sprintf("%s:Agent", prefixPROV)}
		}
		return id
	}

	for _, r := range rows {
		turnID := fmt.Sprintf("%s:turn-%d", prefixSF, r.ID)

		// Activity — every audit row is one turn-shaped activity. HITL
		// lifecycle rows carry their Action enum as the ex:type.
		body := map[string]any{provType: fmt.Sprintf("%s:Activity", prefixPROV)}
		if r.Action != "" {
			body[exType] = "ex:HITLAction"
			body[exNote] = r.Action
		} else {
			body[exType] = "ex:ChatTurn"
			// Refusal rows carry empty VerdictJSON — the refusal reason is
			// the load-bearing note on the activity, so a refusal's
			// provenance stays traceable without a verdict entity.
			if r.RefusalReason != "" {
				body[exNote] = r.RefusalReason
			}
		}
		doc.Activity[turnID] = body

		// Query entity — always present (even refusals have a query).
		queryID := fmt.Sprintf("%s:query-%d", prefixSF, r.ID)
		doc.Entity[queryID] = map[string]any{provType: fmt.Sprintf("%s:Entity", prefixPROV), exType: "ex:Query", exText: r.Query}
		doc.Used = append(doc.Used, Relation{Entity: queryID, Activity: turnID, Time: formatTime(r.Timestamp)})

		// User attribution — every row is attributed to its (possibly
		// anonymous) user.
		userKey := r.UserID
		if userKey == "" {
			userKey = "anonymous"
		}
		doc.WasAttributedTo = append(doc.WasAttributedTo, Attribution{Entity: turnID, Agent: agent("user-" + userKey)})

		// Skill attribution.
		if r.SkillID != "" {
			doc.WasAttributedTo = append(doc.WasAttributedTo, Attribution{Entity: turnID, Agent: agent("skill-" + r.SkillID)})
		}

		// Verdict entity + its derivation chain. Refusal/error rows carry
		// empty VerdictJSON — nothing was asserted, so no verdict entity and
		// no chunk derivation (the refusal reason lives on the activity).
		if r.VerdictJSON != "" {
			verdictID := fmt.Sprintf("%s:verdict-%d", prefixSF, r.ID)
			vbody := map[string]any{provType: fmt.Sprintf("%s:Entity", prefixPROV), exType: "ex:Verdict", exText: r.VerdictJSON}
			if r.RefusalReason != "" {
				vbody[exNote] = r.RefusalReason
			}
			doc.Entity[verdictID] = vbody
			doc.WasGeneratedBy = append(doc.WasGeneratedBy, Relation{Entity: verdictID, Activity: turnID, Time: formatTime(r.Timestamp)})

			for _, cid := range splitChunkIDs(r.ChunkIDs) {
				chunkID := fmt.Sprintf("%s:chunk-%s", prefixSF, cid)
				if _, ok := doc.Entity[chunkID]; !ok {
					doc.Entity[chunkID] = map[string]any{provType: fmt.Sprintf("%s:Entity", prefixPROV), exType: "ex:Chunk"}
				}
				doc.WasDerivedFrom = append(doc.WasDerivedFrom, Derivation{GeneratedEntity: verdictID, UsedEntity: chunkID})
			}

			// Model + platform floor agents attach where a model actually
			// produced output (verdict rows); pure HITL lifecycle rows skip
			// them (they are human/tool decisions, not model output).
			if r.ModelVersion != "" {
				doc.WasAttributedTo = append(doc.WasAttributedTo, Attribution{Entity: verdictID, Agent: agent("model-" + r.ModelVersion)})
			}
			if r.PlatformFloorVersion != "" {
				doc.WasAttributedTo = append(doc.WasAttributedTo, Attribution{Entity: verdictID, Agent: agent("floor-" + r.PlatformFloorVersion)})
			}
		}
	}
	return doc
}

// splitChunkIDs expands the CSV chunk-id column ("c1,c2"). Empty entries
// (empty column, trailing commas) are dropped.
func splitChunkIDs(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// formatTime renders a row timestamp as RFC3339 UTC — the W3C PROV time
// syntax and the same convention as the /api/audit wire shape.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
