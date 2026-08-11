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
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/skill"
)

// validationResponse is the POST /api/skills wire shape. Valid=false
// always carries a non-empty Error naming the failing field + rule.
type validationResponse struct {
	Valid bool   `json:"valid"`
	Error string `json:"error"`
}

// listSkillsHandler implements GET /api/skills (SYSK-01 HTTP layer).
//
// Returns the manifests the Loader cached at startup, marshaled via the
// Manifest json tags (the 7 schema fields). An empty / uninitialized
// registry returns `[]` — NOT `null`, NOT 404 — so clients can iterate
// unconditionally.
func (s *Deps) listSkillsHandler(_ context.Context, c *app.RequestContext) {
	var skills []*skill.Manifest
	if s.Loader != nil {
		skills = s.Loader.Skills()
	}
	if skills == nil {
		skills = []*skill.Manifest{}
	}
	c.JSON(http.StatusOK, skills)
}

// validateSkillHandler implements POST /api/skills (SYSK-02 HTTP layer).
//
// D2-07: this is a DRY-RUN validator. It parses the request body into a
// skill.Manifest, runs skill.Validate, and returns the verdict. It NEVER
// persists to disk and NEVER mutates the Loader cache — GET /api/skills
// returns exactly the same list before and after any POST.
//
// Status codes:
//   - 400 + {valid:false,...} when the body is not parseable JSON
//     (T-2-30 / V5 ASVS — the response never reflects the raw body back).
//   - 200 + {valid:false, error:"..."} when the body parses but fails the
//     7-field schema (the request itself was well-formed; the validation
//     result is the payload).
//   - 200 + {valid:true, error:""} on pass.
func (s *Deps) validateSkillHandler(_ context.Context, c *app.RequestContext) {
	var m skill.Manifest
	raw, err := c.Body()
	if err != nil {
		c.JSON(http.StatusBadRequest, validationResponse{
			Valid: false,
			Error: "unreadable request body: " + err.Error(),
		})
		return
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		// Only the unmarshal error text is echoed — never the raw body.
		c.JSON(http.StatusBadRequest, validationResponse{
			Valid: false,
			Error: "malformed json: " + err.Error(),
		})
		return
	}
	if err := skill.Validate(&m); err != nil {
		c.JSON(http.StatusOK, validationResponse{Valid: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, validationResponse{Valid: true, Error: ""})
}
