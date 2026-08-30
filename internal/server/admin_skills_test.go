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
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

// TestAdminSkillsRetrievalOnly asserts that admin_skills.go contains no
// mutation HTTP verbs (POST / PUT / DELETE / PATCH). This is the
// structural source-grep lock for T-20-04-MUTATE: the admin skill
// registry view is read-only by construction.
func TestAdminSkillsRetrievalOnly(t *testing.T) {
	src, err := os.ReadFile("admin_skills.go")
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	lines := strings.Split(string(src), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		switch {
		case strings.Contains(line, "POST"),
			strings.Contains(line, "PUT"),
			strings.Contains(line, "DELETE"),
			strings.Contains(line, "PATCH"):
			t.Errorf("line %d: contains mutation verb: %s", i+1, line)
		}
	}
}

// TestAdminSkillsHandler403 asserts that GET /api/admin/skills returns
// 403 when no X-Admin-Token is provided.
func TestAdminSkillsHandler403(t *testing.T) {
	deps := &Deps{}
	h := app.NewApp(":0", func(ctx context.Context, c *app.RequestContext) {
		deps.adminSkillsHandler(ctx, c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/skills", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// TestAdminSkillsHandler200 asserts that GET /api/admin/skills returns
// 200 with valid X-Admin-Token and a well-formed JSON body.
func TestAdminSkillsHandler200(t *testing.T) {
	// Set a valid admin token for this test.
	oldToken := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	os.Setenv("SKILLFORGE_ADMIN_TOKEN", "test-admin-token")
	defer func() {
		if oldToken != "" {
			os.Setenv("SKILLFORGE_ADMIN_TOKEN", oldToken)
		} else {
			os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
		}
	}()

	deps := &Deps{}
	h := app.NewApp(":0", func(ctx context.Context, c *app.RequestContext) {
		deps.adminSkillsHandler(ctx, c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/skills", nil)
	req.Header.Set("X-Admin-Token", "test-admin-token")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the response is valid JSON with a "skills" key.
	body := rec.Body.String()
	if !strings.Contains(body, `"skills"`) {
		t.Errorf("expected JSON with 'skills' key, got: %s", body)
	}
}
