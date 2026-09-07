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

// feedback_wire.go — Phase 29 / Plan 29-04 (FBK-01) live-stream turn
// identity emission. This file owns the single helper that publishes a
// turn's server-side id onto the SSE stream; the twelve writeAuditAndTurn
// call sites (pipeline.go / chat.go / runner_setup.go) are the callers
// (chat.go:449's agent-error branch deliberately does NOT emit — see the
// comment there).

package server

import (
	"encoding/json"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/wire"
)

// emitTurnID writes one data-turn SSE frame carrying the assistant
// row's session_turn.id (D-29-01). It is the FBK-01 live-stream half of
// the turn-id exposure: the vote endpoint (29-05) needs this id to key
// UNIQUE(session_id, turn_id) (D-29-02).
//
// Ordering contract (T-29-06 mitigation): the caller MUST have already
// persisted the turn via writeAuditAndTurn and MUST NOT have written a
// terminal frame (text-end / finish-step / finish) yet. Emitting after
// finish would leave useChat's registered assistant message without a
// vote key (Pitfall 3's frame-order window); emitting before the turn
// write would broadcast an id the store never assigned.
//
// Degradation contract: turnID <= 0 means the turn was NOT assigned a
// store id (Store == nil, swallowed append error, or an idempotent
// replay conflict). The frame is NOT emitted — zero bytes — so the FE
// renders its disabled feedback state instead of wiring a vote to a
// fabricated id. Never invent an id.
//
// Error contract: a marshal or write failure is returned to the caller,
// which logs a [WARN] and continues. Emission failure MUST NOT turn
// into a 500 — the audit row and the turn pair are already on disk, and
// the pipeline's terminal frames share the same best-effort posture
// (GEVL-03 keeps exactly one audit row per query).
func emitTurnID(c *app.RequestContext, turnID int64) error {
	if turnID <= 0 {
		// Conflict / degradation path: emit nothing. The FE's disabled
		// state is the honest UX for "this turn has no server identity".
		return nil
	}
	payload, err := json.Marshal(wire.TurnPayload{TurnID: turnID})
	if err != nil {
		return fmt.Errorf("marshal turn payload: %w", err)
	}
	return wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-turn", Data: payload})
}
