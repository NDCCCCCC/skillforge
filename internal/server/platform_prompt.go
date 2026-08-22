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

// Plan 14-02 / PLAT-10 — the admin preview endpoint for the effective
// system prompt. It exposes exactly what the chat path would send to the
// model for a given Skill: the composed prompt body, the platform-floor
// version, and the reproducible prompt_hash that audit rows carry.
//
// Trust model:
//   - The endpoint is admin-only via the same X-Admin-Token gate as
//     GET /api/audit (T-6-TOKEN: crypto/subtle.ConstantTimeCompare).
//   - No user input reaches DDL or SQL; skill lookup is the same
//     Loader-based path used by chat.go.
//   - The prompt_hash is computed with the SAME inputs as the audit row
//     so the preview is byte-identical to what a query would persist.
package server

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/agent"
)

// platformPromptHandler implements GET /api/admin/platform-prompt?skill={id}.
//
// Response envelope (200):
//
//	{
//	  "floor_version":   "1.0.0",
//	  "skill_id":        "compliance",
//	  "composed_prompt": "...",
//	  "prompt_hash":     "sha256-hex"
//	}
//
// Errors: 403 (admin token missing/wrong), 404 (skill not found).
func (s *Deps) platformPromptHandler(ctx context.Context, c *app.RequestContext) {
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, map[string]string{
			"error": "admin token required",
		})
		return
	}

	skillID := string(c.Query("skill"))
	m := s.findSkill(skillID)
	if m == nil {
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
		return
	}

	const modelVersion = "glm-5.2"
	composed := agent.RenderSystemPrompt(m)
	c.JSON(http.StatusOK, map[string]string{
		"floor_version":   agent.PlatformFloorVersion,
		"skill_id":        skillID,
		"composed_prompt": composed,
		"prompt_hash":     computePromptHash(skillID, modelVersion, composed),
	})
}
