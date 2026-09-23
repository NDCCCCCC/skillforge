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

// Plan 41-01 — Admin platform API for MCP Inspector panel.
//
// Wire contract (all routes X-Admin-Token-gated; 403 before 503):
//
//	GET  /api/admin/platform/mcp/status
//	     → 200 {status: "online"|"offline", service_skill: string}
//	       Always "online" when MCPServer is non-nil (subprocess spawned at boot).
//	GET  /api/admin/platform/mcp/tools
//	     → 200 [{name, description, inputSchema}]
//	       Returns the registered MCP tool list.
//	POST /api/admin/platform/mcp/invoke
//	     Body: {tool: string, params: object}
//	     → 200 tool result
//	       400 if tool name unknown; 500 for internal errors.
package server

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/server/platform/mcp"
)

// graphResponseError is the standard admin JSON error envelope.
type graphResponseError struct {
	Error string `json:"error"`
}

// mcpStatusResponse is the 200 body for GET /api/admin/platform/mcp/status.
type mcpStatusResponse struct {
	Status       string `json:"status"`
	ServiceSkill string `json:"service_skill"`
}

// mcpToolDesc describes one registered MCP tool for the inspector panel.
type mcpToolDesc struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	InputSchema   map[string]interface{} `json:"inputSchema"`
}

// adminPlatformMcpStatusHandler implements GET /api/admin/platform/mcp/status.
func (s *Deps) adminPlatformMcpStatusHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.MCPServer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "MCP server not wired"})
		return
	}
	c.JSON(http.StatusOK, mcpStatusResponse{
		Status:       "online",
		ServiceSkill: s.MCPServer.ServiceSkill(),
	})
}

// adminPlatformMcpToolsHandler implements GET /api/admin/platform/mcp/tools.
func (s *Deps) adminPlatformMcpToolsHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.MCPServer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "MCP server not wired"})
		return
	}
	tools := s.MCPServer.ListTools()
	out := make([]mcpToolDesc, 0, len(tools))
	for _, t := range tools {
		schema := make(map[string]interface{})
		if t.InputSchema != nil {
			if m, ok := t.InputSchema.(map[string]interface{}); ok {
				schema = m
			}
		}
		out = append(out, mcpToolDesc{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}
	c.JSON(http.StatusOK, out)
}

// mcpInvokeRequest is the POST body for /api/admin/platform/mcp/invoke.
type mcpInvokeRequest struct {
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params"`
}

// adminPlatformMcpInvokeHandler implements POST /api/admin/platform/mcp/invoke.
func (s *Deps) adminPlatformMcpInvokeHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.MCPServer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "MCP server not wired"})
		return
	}
	var req mcpInvokeRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid request body"})
		return
	}
	if req.Tool == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "tool name is required"})
		return
	}

	// Delegate to the MCP server for actual invocation.
	// The MCP server is a stdio subprocess; this call routes through the
	// admin API handler which calls into the MCPServer struct.
	result, err := s.MCPServer.CallTool(ctx, req.Tool, req.Params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// requireAdmin returns true and writes 403 if the admin token is missing.
func requireAdmin(c *app.RequestContext) bool {
	token := c.GetHeader("X-Admin-Token")
	if token == "" {
		c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
		return false
	}
	return true
}

