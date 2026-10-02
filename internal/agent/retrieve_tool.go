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

// Package agent — retrieve_tool.go is the Phase 11 / Plan 11-01
// minimal-displacement wrapper around the new
// internal/agent/tools.BuildRetrieveTool.
//
// The full implementation has been lifted into
// internal/agent/tools/retrieve.go (D11-A3 / D11-B1 file split).
// This file stays as a thin alias so existing call sites in
// internal/agent/agent.go (NewAgent) and the legacy unit tests
// (TestBuildRetrieveTool, TestRetrieveToolFramesChunksAsData,
// TestRetrieveToolEnforcesResolvedTiers) keep compiling without
// churn. The 11-08 final acceptance gate deletes this file once the
// legacy react.Agent path is also retired.
//
// Why the package shape:
//
//   - Legacy path (react.Agent, internal/agent/agent.go): imports
//     agent.BuildRetrieveTool from this file.
//   - New path (adk.Runner, internal/agent/runner_setup.go): imports
//     agenttools.BuildRetrieveTool from internal/agent/tools/retrieve.go.
//
// Both paths compile against the same canonical Tool implementation
// (the agenttools package is the new home; this wrapper is the
// minimum-displacement shim).
package agent

import (
	"github.com/cloudwego/eino/components/tool"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"

	agenttools "github.com/skillforge/skillforge/internal/agent/tools"
)

// RetrieveToolName is the canonical Eino tool name. Re-exported from
// the agenttools package so the legacy code path keeps a stable
// import surface.
//
// Deprecated: 11-08 deletes this re-export. New code should import
// agenttools.RetrieveToolName directly.
const RetrieveToolName = agenttools.RetrieveToolName

// retrieveTopK is the production default for Retriever.Search's topK
// argument. Kept here so the legacy agent.go::preRetrieveOriginalQuery
// keeps compiling after the Phase 11 / Plan 11-01 lift into
// internal/agent/tools/retrieve.go. The literal `5` matches
// agenttools.retrieveTopK — the duplication is intentional and
// regression-locked by TestRetrieveToolFramesChunksAsData (asserts 5
// chunks).
//
// Deprecated: 11-08 deletes this constant; the canonical home is
// agenttools.retrieveTopK (the Runner path reads it from there).
const retrieveTopK = 5

// BuildRetrieveTool constructs the retrieve_compliance_chunks
// InvokableTool. Thin alias for agenttools.BuildRetrieveTool — the
// real implementation lives in internal/agent/tools/retrieve.go (the
// new Phase 11 home per D11-A3).
//
// Deprecated: 11-08 deletes this alias. New code should call
// agenttools.BuildRetrieveTool directly (the Runner path already does
// in internal/agent/runner_setup.go::BuildRunner).
func BuildRetrieveTool(emb embedder.Embedder, ret retriever.Retriever, m *skill.Manifest) tool.InvokableTool {
	return agenttools.BuildRetrieveTool(emb, ret, m)
}

// formatChunksAsData renders retriever.Hit slices as the canonical
// [CHUNK N] ... \n---\n envelope. Thin alias for
// agenttools.FormatChunksAsData — the real implementation lives in
// internal/agent/tools/retrieve.go.
//
// Deprecated: 11-08 deletes this alias.
func formatChunksAsData(hits []retriever.Hit) string {
	return agenttools.FormatChunksAsData(hits)
}
