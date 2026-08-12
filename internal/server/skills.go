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

// wizardRequest is the POST /api/skills/wizard body shape. The body
// carries the 5 interview answers; the BE drives the structured-output
// LLM call + the 7-field Validate gate (D5-11).
type wizardRequest struct {
	Answers     skill.Answers `json:"answers"`
	CurrentStep int           `json:"currentStep,omitempty"` // echoed for FE-driven step indicator; BE does not branch on it
}

// wizardResponse carries the generated preview. Name is the Skill
// directory name extracted from the purpose (first sentence,
// slugified to ^[a-z0-9-]+$). Error is non-empty on any failure.
type wizardResponse struct {
	Preview *skill.Manifest `json:"preview"`
	Name    string          `json:"name"`
	Error   string          `json:"error,omitempty"`
}

// wizardCommitRequest is the POST /api/skills/wizard/commit body shape.
// Name + Preview are both required — the BE uses Name to construct the
// on-disk path (T-5-16 path-traversal gate) and Preview as the Manifest
// payload to validate + write.
type wizardCommitRequest struct {
	Name    string          `json:"name"`
	Preview *skill.Manifest `json:"preview"`
}

// wizardCommitResponse carries the absolute path of the written file
// (the fsnotify watcher in Plan 05-01 picks it up automatically; no
// explicit rescan call). Path is empty on any failure.
type wizardCommitResponse struct {
	OK    bool   `json:"ok"`
	Path  string `json:"path"`
	Error string `json:"error,omitempty"`
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

// wizardHandler implements POST /api/skills/wizard (SC-3 / SYSK-06
// HTTP layer). It drives the 5-step interview → 7-field Manifest
// pipeline:
//   - Decodes the request body (Answers + currentStep).
//   - Extracts a Skill name from the first sentence of the purpose,
//     slugified to ^[a-z0-9-]+$ (T-5-16 first gate).
//   - Calls Wizard.Generate (LLM call + structured output + retry on
//     validation failure per T-5-18).
//   - Returns the previewed Manifest in the response. The FE renders
//     it in the Skill 定义 tab + offers 重新生成 / 确认并保存 / 取消向导.
//   - Does NOT write to disk — the wizard commit endpoint does that.
//
// Status codes:
//   - 400 + {error:"..."} when the body is not parseable JSON or the
//     name extraction fails.
//   - 503 + {error:"..."} when Wizard is not initialized (Deps.Wizard
//     is nil — e.g., a test that bypasses the wizard). Mirrors the
//     Phase-2 fallback posture of returning a structured error rather
//     than a 500 with no context.
//   - 200 + {preview, name} on success.
func (s *Deps) wizardHandler(ctx context.Context, c *app.RequestContext) {
	if s.Wizard == nil {
		c.JSON(http.StatusServiceUnavailable, wizardResponse{
			Error: "wizard not initialized (set Deps.Wizard at server construction)",
		})
		return
	}
	var req wizardRequest
	raw, err := c.Body()
	if err != nil {
		c.JSON(http.StatusBadRequest, wizardResponse{Error: "unreadable request body: " + err.Error()})
		return
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		c.JSON(http.StatusBadRequest, wizardResponse{Error: "malformed json: " + err.Error()})
		return
	}
	name, err := extractSkillName(req.Answers.Purpose)
	if err != nil {
		c.JSON(http.StatusBadRequest, wizardResponse{Error: err.Error()})
		return
	}

	preview, err := s.Wizard.Generate(ctx, req.Answers)
	if err != nil {
		// Validation-failed-twice OR LLM error — surface the message
		// to the FE for inline display per UI-SPEC §SC-3.
		c.JSON(http.StatusOK, wizardResponse{Error: err.Error()})
		return
	}
	preview.Name = name
	c.JSON(http.StatusOK, wizardResponse{Preview: preview, Name: name})
}

// wizardCommitHandler implements POST /api/skills/wizard/commit. It
// validates the previewed Manifest (third gate), writes the file
// atomically (write-temp + rename), and returns the absolute path.
// The fsnotify watcher (Plan 05-01) picks up the new file automatically
// — no explicit rescan call (D5-11 closed loop).
//
// Status codes:
//   - 400 + {error:"..."} when the body is malformed or the name
//     fails the regex / resolved-path checks (T-5-16 + V12 ASVS).
//   - 503 + {error:"..."} when Wizard is not initialized.
//   - 200 + {ok:true, path} on success. Path is the absolute path the
//     fsnotify watcher observed.
func (s *Deps) wizardCommitHandler(ctx context.Context, c *app.RequestContext) {
	if s.Wizard == nil {
		c.JSON(http.StatusServiceUnavailable, wizardCommitResponse{
			Error: "wizard not initialized (set Deps.Wizard at server construction)",
		})
		return
	}
	var req wizardCommitRequest
	raw, err := c.Body()
	if err != nil {
		c.JSON(http.StatusBadRequest, wizardCommitResponse{Error: "unreadable request body: " + err.Error()})
		return
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		c.JSON(http.StatusBadRequest, wizardCommitResponse{Error: "malformed json: " + err.Error()})
		return
	}
	if req.Preview == nil {
		c.JSON(http.StatusBadRequest, wizardCommitResponse{Error: "preview is required"})
		return
	}
	req.Preview.Name = req.Name

	path, err := s.Wizard.Commit(ctx, req.Preview)
	if err != nil {
		c.JSON(http.StatusOK, wizardCommitResponse{OK: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, wizardCommitResponse{OK: true, Path: path})
}
