// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable software governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package auditprov

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
)

func baseRow(id int64, ts time.Time) store.AuditRow {
	return store.AuditRow{
		ID:                   id,
		Timestamp:            ts,
		SessionID:            "sess-1",
		UserID:               "u1",
		SkillID:              "compliance",
		Query:                "q1",
		PromptHash:           "h1",
		ModelVersion:         "glm-5.2",
		VerdictJSON:          `{"status":"ok"}`,
		ChunkIDs:             "c1,c2",
		PlatformFloorVersion: "1.1.0",
	}
}

// TestFromSessionRows_Deterministic — same rows in, byte-identical
// PROV-JSON out (the zero-LLM reproducibility contract).
func TestFromSessionRows_Deterministic(t *testing.T) {
	rows := []store.AuditRow{baseRow(1, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))}

	a, err := json.Marshal(FromSessionRows(rows))
	require.NoError(t, err)
	b, err := json.Marshal(FromSessionRows(rows))
	require.NoError(t, err)
	assert.JSONEq(t, string(a), string(b))
}

// TestFromSessionRows_AnswerRowMapping — one answer row produces the full
// graph: turn activity, query + verdict + 2 chunk entities, 4 agents, and
// the generatedBy/used/derivedFrom/attribution relations.
func TestFromSessionRows_AnswerRowMapping(t *testing.T) {
	doc := FromSessionRows([]store.AuditRow{baseRow(7, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))})

	// Activities.
	require.Contains(t, doc.Activity, "sf:turn-7")
	assert.Equal(t, "ex:ChatTurn", doc.Activity["sf:turn-7"]["ex:type"])

	// Entities.
	require.Contains(t, doc.Entity, "sf:query-7")
	assert.Equal(t, "q1", doc.Entity["sf:query-7"]["ex:text"])
	require.Contains(t, doc.Entity, "sf:verdict-7")
	assert.Equal(t, `{"status":"ok"}`, doc.Entity["sf:verdict-7"]["ex:text"])
	require.Contains(t, doc.Entity, "sf:chunk-c1")
	require.Contains(t, doc.Entity, "sf:chunk-c2")

	// Agents.
	for _, a := range []string{"sf:user-u1", "sf:skill-compliance", "sf:model-glm-5.2", "sf:floor-1.1.0"} {
		require.Contains(t, doc.Agent, a, "agent %s", a)
	}

	// Relations.
	require.Len(t, doc.WasGeneratedBy, 1)
	assert.Equal(t, "sf:verdict-7", doc.WasGeneratedBy[0].Entity)
	assert.Equal(t, "sf:turn-7", doc.WasGeneratedBy[0].Activity)
	assert.Equal(t, "2026-09-01T12:00:00Z", doc.WasGeneratedBy[0].Time)

	require.Len(t, doc.Used, 1)
	assert.Equal(t, "sf:query-7", doc.Used[0].Entity)

	require.Len(t, doc.WasDerivedFrom, 2, "verdict derived from 2 chunks")
	assert.Equal(t, "sf:verdict-7", doc.WasDerivedFrom[0].GeneratedEntity)
	assert.Equal(t, "sf:chunk-c1", doc.WasDerivedFrom[0].UsedEntity)
	assert.Equal(t, "sf:chunk-c2", doc.WasDerivedFrom[1].UsedEntity)

	require.Len(t, doc.WasAttributedTo, 4, "turn→{user,skill} + verdict→{model,floor}")
}

// TestFromSessionRows_RefusalRow — a refusal carries empty VerdictJSON: no
// verdict entity and no chunk derivation, but the turn/query/user graph and
// the refusal reason note remain traceable.
func TestFromSessionRows_RefusalRow(t *testing.T) {
	r := baseRow(9, time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC))
	r.VerdictJSON = ""
	r.RefusalReason = "no_authorized_source"
	r.ChunkIDs = ""
	doc := FromSessionRows([]store.AuditRow{r})

	assert.NotContains(t, doc.Entity, "sf:verdict-9")
	assert.NotContains(t, doc.Entity, "sf:chunk-c1")
	assert.Empty(t, doc.WasDerivedFrom)
	assert.Len(t, doc.WasGeneratedBy, 0, "nothing was asserted")
	assert.Equal(t, "no_authorized_source", doc.Activity["sf:turn-9"]["ex:note"])
	require.Contains(t, doc.Entity, "sf:query-9", "the query itself stays traceable")
}

// TestFromSessionRows_HITLActionRow — lifecycle rows type as HITLAction
// with the Action enum in the note. Real lifecycle rows carry frozen_args,
// not verdict JSON, so the model/floor agents (which follow VERDICT
// presence) are not minted.
func TestFromSessionRows_HITLActionRow(t *testing.T) {
	r := baseRow(11, time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC))
	r.Action = "action_rule_revoked"
	r.VerdictJSON = ""
	r.ChunkIDs = ""
	doc := FromSessionRows([]store.AuditRow{r})

	assert.Equal(t, "ex:HITLAction", doc.Activity["sf:turn-11"]["ex:type"])
	assert.Equal(t, "action_rule_revoked", doc.Activity["sf:turn-11"]["ex:note"])
	for id := range doc.Agent {
		assert.NotContains(t, id, "sf:model-", "no verdict → no model attribution")
		assert.NotContains(t, id, "sf:floor-", "no verdict → no floor attribution")
	}
}

// TestFromSessionRows_RedactionPassThrough — the retention sentinel must be
// exported verbatim; the projection never decodes redacted content.
func TestFromSessionRows_RedactionPassThrough(t *testing.T) {
	r := baseRow(13, time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC))
	r.Query = "[REDACTED:90d+ (ts=…, boundary=…)]"
	doc := FromSessionRows([]store.AuditRow{r})

	assert.Equal(t, "[REDACTED:90d+ (ts=…, boundary=…)]", doc.Entity["sf:query-13"]["ex:text"])
}

// TestFromSessionRows_EmptyChunkCSV — empty / malformed ChunkIDs produce no
// chunk entities and no derivations (no nil-map surprises).
func TestFromSessionRows_EmptyChunkCSV(t *testing.T) {
	r := baseRow(15, time.Now())
	r.ChunkIDs = ""
	assert.Empty(t, FromSessionRows([]store.AuditRow{r}).WasDerivedFrom)

	r.ChunkIDs = ", ,"
	assert.Empty(t, FromSessionRows([]store.AuditRow{r}).WasDerivedFrom)
}
