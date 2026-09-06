// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// external_channel_test.go — v1.1.1 (DATA-07 second half + T-W62-05):
// needs_external_query reaches the LLM-visible tool set ONLY through the
// explicit Skill contract (reads declaring
// skill.ReadDataExternalAuthorized) AND a non-nil ExternalAuthManager.
// Mirrors TestToolsForChannel's assembly-seam discipline (the tool set
// is re-derived exactly as BuildRunner's data-channel branch composes
// it, after driving BuildRunner itself for the real validation path).
package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/agent"
	agenttools "github.com/skillforge/skillforge/internal/agent/tools"
	"github.com/skillforge/skillforge/internal/externalauth"
	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/skillforge/skillforge/internal/skill"
)

// externalChannelFixtureView mirrors the shipped reference view schema.
const externalChannelFixtureView = `
views:
  - view_name: v_user_consumption
    source: test fixture — read-only consumption replica
    columns:
      - name: user_id
        type: string
      - name: amount
        type: float
      - name: account_no
        type: string
    field_masks:
      - field_name: account_no
        kind: account
        keep_len: 4
`

// newExternalAuthManagerForChannel builds a wired manager over a
// temp-dir controlled view file (loaded through the real LoadViewsDir)
// + MemoryStore + MemorySource. Row data is seeded but never asserted
// here — this file locks REGISTRATION, not the flow (see
// internal/agent/tools/external_query_tool_test.go).
func newExternalAuthManagerForChannel(t *testing.T) *externalauth.Manager {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v_user_consumption.yaml"), []byte(externalChannelFixtureView), 0o644))
	reg, err := externalauth.LoadViewsDir(dir)
	require.NoError(t, err)
	src := externalauth.NewMemorySource(reg)
	require.NoError(t, src.UpsertView("v_user_consumption", []map[string]string{
		{"user_id": "u-001", "amount": "1500.00", "account_no": "6225880137799001"},
	}))
	return externalauth.NewManager(externalauth.NewMemoryStore(), src, reg)
}

// externalAuthorizedManifest declares the v1.1.1 external channel read.
func externalAuthorizedManifest() *skill.Manifest {
	return &skill.Manifest{
		Description: "external authorized channel test skill",
		Agent:       "ChatModelAgent",
		Reads:       []string{skill.ReadDataExternalAuthorized},
		Writes:      []string{},
	}
}

// externalChannelTools mirrors BuildRunner's data-channel branch AS
// UPDATED for the external tool (agentChannelTools predates it and pins
// the aggregated_only shape). Same re-derivation rationale as
// agentChannelTools: the Runner's tool list is unexported, so the test
// re-runs the exact composition the production branch performs.
func externalChannelTools(cfg agent.RunnerBuildConfig) ([]tool.BaseTool, error) {
	if _, err := agent.BuildRunner(context.Background(), cfg); err != nil {
		return nil, err
	}
	tools := []tool.BaseTool{
		agenttools.BuildKPICallTool(cfg.KPIExecutor, cfg.KPIRegistry, cfg.SessionID, cfg.SkillID, cfg.DataRefEmit),
	}
	if slicesContainsReads(cfg.Manifest.Reads, skill.ReadDataExternalAuthorized) {
		tools = append(tools, agenttools.BuildExternalQueryTool(agenttools.ExternalQueryToolConfig{
			Manager:   cfg.ExternalAuthManager,
			ChatModel: cfg.ChatModel,
			Registry:  cfg.ExternalAuthManager.Registry(),
			SessionID: cfg.SessionID,
			SkillID:   cfg.SkillID,
		}))
	}
	return tools, nil
}

// slicesContainsReads mirrors the stdlib slices.Contains call the
// production branch performs (kept local with a domain name so this
// file's intent stays obvious).
func slicesContainsReads(reads []string, want string) bool {
	for _, v := range reads {
		if v == want {
			return true
		}
	}
	return false
}

// TestExternalChannelToolRegistration — the three registration faces.
func TestExternalChannelToolRegistration(t *testing.T) {
	fix := newToolsForChannelFixture(t)
	meta := kpiStubMeta{version: kpi.DatasetVersion{
		Period:   "2026-07",
		FilePath: fix.dummyFile,
	}}
	executor := kpi.NewExecutor(fix.registry, meta, fix.engine, fix.audit, func() time.Time {
		return time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	})

	t.Run("external_authorized_plus_manager_yields_tool", func(t *testing.T) {
		tools, err := externalChannelTools(agent.RunnerBuildConfig{
			Manifest:            externalAuthorizedManifest(),
			Embedder:            newStubEmbedder(),
			Retriever:           &stubRetriever{},
			ChatModel:           minimalChatModel{},
			SessionID:           "sess-ext",
			SkillID:             "external-lookup",
			KPIExecutor:         executor,
			KPIRegistry:         fix.registry,
			ExternalAuthManager: newExternalAuthManagerForChannel(t),
		})
		require.NoError(t, err, "external:authorized + non-nil manager must build")
		names := toolNamesInfo(t, tools)
		assert.Contains(t, names, agenttools.ExternalQueryToolName,
			"external:authorized Skill must see needs_external_query")
		assert.Contains(t, names, agenttools.KPICallToolName,
			"the data-channel base tool stays present")
	})

	t.Run("external_authorized_nil_manager_fails_fast", func(t *testing.T) {
		_, err := agent.BuildRunner(context.Background(), agent.RunnerBuildConfig{
			Manifest:    externalAuthorizedManifest(),
			Embedder:    newStubEmbedder(),
			Retriever:   &stubRetriever{},
			ChatModel:   minimalChatModel{},
			SessionID:   "sess-ext-nil",
			SkillID:     "external-lookup",
			KPIExecutor: executor,
			KPIRegistry: fix.registry,
			// ExternalAuthManager deliberately nil — the server did not
			// wire the channel (EXTERNAL_AUTH_ENABLED != 1).
		})
		require.Error(t, err, "BuildRunner must fail fast on a nil manager for an external:authorized Skill")
		assert.Contains(t, err.Error(), agenttools.ExternalQueryToolName,
			"the failure must name the tool that was refused")
		assert.Contains(t, err.Error(), skill.ReadDataExternalAuthorized,
			"the failure must name the Skill contract value")
		assert.Contains(t, err.Error(), "fail-closed",
			"the failure must signal the fail-closed assembly contract")
	})

	t.Run("aggregated_only_never_sees_external_tool", func(t *testing.T) {
		tools, err := externalChannelTools(agent.RunnerBuildConfig{
			Manifest:    dataChannelManifest(),
			Embedder:    newStubEmbedder(),
			Retriever:   &stubRetriever{},
			ChatModel:   minimalChatModel{},
			SessionID:   "sess-agg",
			SkillID:     "kpi-dashboard",
			KPIExecutor: executor,
			KPIRegistry: fix.registry,
			// Manager present — the point is that the aggregated_only
			// contract, not the wiring, decides visibility.
			ExternalAuthManager: newExternalAuthManagerForChannel(t),
		})
		require.NoError(t, err)
		names := toolNamesInfo(t, tools)
		assert.NotContains(t, names, agenttools.ExternalQueryToolName,
			"an aggregated_only data Skill must never see needs_external_query (DATA-07: capability is granted only by the explicit Skill contract)")
	})
}
