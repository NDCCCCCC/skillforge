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

// Package wire — Plan 05-01 Task 1: RedlinePayload + data-redline type.
//
// RedlinePayload is the JSON shape carried by the 18th wire type
// `data-redline` (D5-14). It carries an agent-generated clause-level
// redline proposal: the original clause text, the proposed rewrite, a
// human-readable rationale, and an optional clause id.
//
// V8 wire-smuggling mitigation (per 05-UI-SPEC §Registry Safety):
// the three core fields (OriginalClause / ProposedRewrite / Rationale)
// are REQUIRED on the JSON parse path — they have no `omitempty` tag
// so a truncated payload (one of them missing) returns a json.Unmarshal
// error rather than silently degrading to zero-value data that the UI
// would render as a blank diff.
//
// Use the UnmarshalRedline helper below to additionally reject unknown
// JSON fields (defense-in-depth for any consumer that wants to fail
// closed against a producer that adds new fields without coordinating
// the wire contract — D15 two-PR drift invariant).
package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// RedlinePayload is the JSON shape carried by data-redline SSE chunks.
// The compliance Skill (and future redline-capable Skills) emit one of
// these per clause-level rewrite it proposes.
//
// Field semantics:
//   - ClauseID:         optional stable id of the clause being rewritten
//                       (e.g. "C-7.3.2" — must match the source-document
//                       id in the citation list so the UI can link the
//                       redline back to its citation). Empty when the
//                       redline targets an ad-hoc clause without a
//                       stable id.
//   - OriginalClause:   REQUIRED — the exact text of the clause as it
//                       currently exists in the cited source document.
//   - ProposedRewrite:  REQUIRED — the agent's proposed replacement
//                       text. Must be non-empty.
//   - Rationale:        REQUIRED — human-readable explanation of WHY
//                       the rewrite is proposed (the SkillForge
//                       inspector renders this verbatim so the user
//                       can accept / reject the redline).
type RedlinePayload struct {
	ClauseID        string `json:"clauseId,omitempty"`
	OriginalClause  string `json:"originalClause"`
	ProposedRewrite string `json:"proposedRewrite"`
	Rationale       string `json:"rationale"`
}

// UnmarshalJSON implements V8 wire-smuggling mitigation: a redline
// payload missing any of the three core fields (OriginalClause,
// ProposedRewrite, Rationale) fails the parse rather than silently
// degrading to zero-value data that the UI would render as a blank
// diff. ClauseID is allowed to be absent (omitempty) since some
// redlines target ad-hoc clauses without a stable id.
//
// The standard library's json.Unmarshal does NOT reject missing fields
// — it leaves the destination at its zero value. To enforce
// required-field presence we UnmarshalJSON into an intermediate
// struct, then check that the three required fields were present (a
// sentinel of struct fields-as-pointers, where a missing JSON key
// leaves the pointer at nil — the only way to distinguish "absent"
// from "present-and-empty").
func (r *RedlinePayload) UnmarshalJSON(data []byte) error {
	type rawRedline struct {
		ClauseID        *string `json:"clauseId,omitempty"`
		OriginalClause  *string `json:"originalClause"`
		ProposedRewrite *string `json:"proposedRewrite"`
		Rationale       *string `json:"rationale"`
	}
	var raw rawRedline
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.OriginalClause == nil {
		return errors.New("redline payload: originalClause is required (V8)")
	}
	if raw.ProposedRewrite == nil {
		return errors.New("redline payload: proposedRewrite is required (V8)")
	}
	if raw.Rationale == nil {
		return errors.New("redline payload: rationale is required (V8)")
	}
	r.ClauseID = ""
	if raw.ClauseID != nil {
		r.ClauseID = *raw.ClauseID
	}
	r.OriginalClause = *raw.OriginalClause
	r.ProposedRewrite = *raw.ProposedRewrite
	r.Rationale = *raw.Rationale
	return nil
}

// UnmarshalRedline parses a data-redline chunk's Data payload with
// DisallowUnknownFields (V8 mitigation). Use this on the consumer side
// (e.g. internal/wire handler that deserializes the chunk before
// forwarding to the React UI) so an unexpected JSON field from a
// misbehaving producer fails the parse rather than silently passing
// through.
//
// The required-field check (V8 core) happens automatically inside
// json.Unmarshal because OriginalClause / ProposedRewrite / Rationale
// have no omitempty tag — missing fields return an error.
func UnmarshalRedline(data json.RawMessage) (RedlinePayload, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return RedlinePayload{}, errors.New("redline payload is empty")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var r RedlinePayload
	if err := dec.Decode(&r); err != nil {
		return RedlinePayload{}, fmt.Errorf("decode redline payload: %w", err)
	}
	return r, nil
}