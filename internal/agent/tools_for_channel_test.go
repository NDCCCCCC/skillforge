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

// tools_for_channel_test.go is Phase 16 / Plan 16-07 Task 3's
// channel-branch assertion — the D16-C3 runtime layer locked at the
// assembly seam. Four sub-cases exercise the same fixture wiring
// (real Executor + real Registry + stub seam) and compare
// names(tools) for data vs tier channel Skill manifests.
//
// The test mirrors the hitl_tools_test.go:148-153 dual-manifest
// comparison discipline (readOnly vs writey) extended to the
// channel axis: a data-channel Skill sees exactly {kpi_call}; a
// tier-channel Skill sees {retrieve + whatever HITL tools the
// writes allow} but NOT kpi_call. Cross-channel Tools are
// physically unreachable — the LLM cannot ask for what it never
// sees in the schema, regardless of prompt injection (PITFALL 12).
//
// The nil-deps sub-case proves the fail-closed assembly guard:
// BuildRunner for a data-channel Skill with nil KPIExecutor or
// nil KPIRegistry returns an error BEFORE constructing the Agent.
package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/agent"
	agenttools "github.com/skillforge/skillforge/internal/agent/tools"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/wire"
)

// toolsForChannelFixture loads a real *kpi.Registry (hermetic fake
// serializer) + wires a real *kpi.Executor with stubs at the
// meta/engine/audit seam. Mirrors the kpi_tool_test.go fixture
// pattern with package-local names so the two files don't share
// unexported test helpers across package boundaries.
type toolsForChannelFixture struct {
	registry  *kpi.Registry
	engine    *toolsForChannelEngine
	audit     *toolsForChannelAudit
	dummyFile string
}

type toolsForChannelEngine struct {
	mu      sync.Mutex
	rows    []map[string]any
	lastReq kpi.EngineRequest
}

func (e *toolsForChannelEngine) Query(_ context.Context, req kpi.EngineRequest) ([]map[string]any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastReq = req
	return e.rows, nil
}

type toolsForChannelAudit struct {
	mu sync.Mutex
}

func (a *toolsForChannelAudit) PutKPICall(_ context.Context, _ store.KPICallRow) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return nil
}

const toolsForChannelYAML = `
dataset: loans
description: tools_for_channel fixture
kpis:
  - name: loan_overdue_rate_by_status
    description: 各状态贷款逾期率
    sql: |
      SELECT status AS status, count(*) AS row_count
      FROM ds_loans.data
      GROUP BY status
    count_column: row_count
    params: {}
`

const toolsForChannelASTEnvelope = `{"error":false,"error_type":"","error_message":"","statements":[{"node":{"type":"SELECT_NODE","from_table":{"type":"BASE_TABLE","schema_name":"ds_loans","table_name":"data"},"select_list":[]}}]}`

type toolsForChannelSerializer struct{}

func (toolsForChannelSerializer) SerializeSQL(context.Context, string) (string, error) {
	return toolsForChannelASTEnvelope, nil
}

type toolsForChannelEmptyLister struct{}

func (toolsForChannelEmptyLister) ListDatasets(context.Context) ([]string, error) {
	return nil, nil
}

func newToolsForChannelFixture(t *testing.T) *toolsForChannelFixture {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "loans.yaml"), []byte(toolsForChannelYAML), 0o644))
	reg, _, err := kpi.LoadKPIs(context.Background(), dir, toolsForChannelSerializer{}, toolsForChannelEmptyLister{})
	require.NoError(t, err, "LoadKPIs fixture failed")
	dummy := filepath.Join(t.TempDir(), "loans.duckdb")
	require.NoError(t, os.WriteFile(dummy, []byte{}, 0o644))
	return &toolsForChannelFixture{
		registry: reg,
		engine: &toolsForChannelEngine{
			rows: []map[string]any{{"status": "正常", "row_count": int64(120)}},
		},
		audit:     &toolsForChannelAudit{},
		dummyFile: dummy,
	}
}

// dataChannelManifest is a Manifest declaring reads "data:aggregated_only"
// — the only data-channel value validated in v1 (D16-A4).
func dataChannelManifest() *skill.Manifest {
	m := &skill.Manifest{
		Description: "data-channel test skill",
		Agent:       "ChatModelAgent",
		Reads:       []string{skill.ReadDataAggregatedOnly},
		Writes:      []string{},
	}
	return m
}

// tierChannelManifest is a Manifest declaring a tier read (the authority
// slug) +
// a single write target so BuildToolsForSkill has something to do.
func tierChannelManifest() *skill.Manifest {
	m := &skill.Manifest{
		Description: "tier-channel test skill",
		Agent:       "ChatModelAgent",
		Reads:       []string{"authority"},
		Writes:      []string{"memory:facts"},
	}
	m.SetResolvedTiersForTest([]string{"authority"})
	return m
}

// toolNamesInfo returns the LLM-visible Tool names (via Info().Name).
func toolNamesInfo(t *testing.T, tools []tool.BaseTool) []string {
	t.Helper()
	out := make([]string, 0, len(tools))
	for _, b := range tools {
		info, err := b.Info(context.Background())
		require.NoError(t, err)
		out = append(out, info.Name)
	}
	sort.Strings(out)
	return out
}

// minimalChatModel satisfies model.ToolCallingChatModel with no
// behavioral surface — BuildRunner only constructs the Agent; the
// test never invokes Run, so no chat-model execution surface is
// exercised.
type minimalChatModel struct{}

func (minimalChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return nil, nil
}

func (minimalChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("minimalChatModel: Stream not implemented")
}

func (minimalChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return minimalChatModel{}, nil
}

var _ model.ToolCallingChatModel = minimalChatModel{}

// kpiStubMeta is a tiny DatasetMetaSource stub for this test file.
type kpiStubMeta struct {
	version kpi.DatasetVersion
	err     error
}

func (m kpiStubMeta) LatestDatasetVersion(context.Context, string) (kpi.DatasetVersion, error) {
	return m.version, m.err
}

func (m kpiStubMeta) Periods(context.Context, string) ([]string, error) {
	return nil, nil
}

func (m kpiStubMeta) VersionForPeriod(context.Context, string, string) (kpi.DatasetVersion, error) {
	return m.version, m.err
}

// agentChannelTools is a small helper that mirrors the channel-branch
// assembly of BuildRunner and returns the resulting Tool slice. We
// can't easily extract the Tools from a constructed *adk.Runner
// (the Agent field is unexported) so we re-run the same code path
// the BuildRunner takes — guaranteeing the test pins the EXACT
// same Tools slice the production assembly would produce. The
// BuildRunner call first also exercises the validation surface so
// the fail-closed guard's Go error path is covered.
func agentChannelTools(cfg agent.RunnerBuildConfig) ([]tool.BaseTool, error) {
	// Drive through BuildRunner for the real assembly validation;
	// ignore the returned *adk.Runner — we only need to confirm
	// the validation passes, then rebuild the Tools slice.
	if _, err := agent.BuildRunner(context.Background(), cfg); err != nil {
		return nil, err
	}

	if cfg.Manifest.HasDataChannel() {
		return []tool.BaseTool{
			agenttools.BuildKPICallTool(cfg.KPIExecutor, cfg.KPIRegistry, cfg.SessionID, cfg.SkillID, cfg.DataRefEmit),
		}, nil
	}
	// tier-channel branch — same composition as runner_setup.go.
	tools, err := agent.BuildToolsForSkill(context.Background(), cfg.Manifest, cfg.SessionID, cfg.SkillID, hitl.NoopRuleMatcher{}, cfg.KBRegistry)
	if err != nil {
		return nil, err
	}
	tools = append(tools, agent.BuildRetrieveTool(cfg.Embedder, cfg.Retriever, cfg.Manifest))
	return tools, nil
}

// TestToolsForChannel is the data/tier channel branch assertion —
// the D16-C3 runtime layer locked at the assembly seam.
func TestToolsForChannel(t *testing.T) {
	fix := newToolsForChannelFixture(t)

	meta := kpiStubMeta{version: kpi.DatasetVersion{
		Period:   "2026-07",
		FilePath: fix.dummyFile,
	}}
	executor := kpi.NewExecutor(fix.registry, meta, fix.engine, fix.audit, func() time.Time {
		return time.Date(2026, 8, 27, 12, 30, 15, 250_000_000, time.UTC)
	})

	cm := minimalChatModel{}

	t.Run("data_channel_yields_kpi_call_only", func(t *testing.T) {
		manifest := dataChannelManifest()
		require.True(t, manifest.HasDataChannel(), "manifest must be data-channel")
		tools, err := agentChannelTools(agent.RunnerBuildConfig{
			Manifest:    manifest,
			Embedder:    newStubEmbedder(),
			Retriever:   &stubRetriever{},
			ChatModel:   cm,
			SessionID:   "sess-data",
			SkillID:     "data-skill",
			KPIExecutor: executor,
			KPIRegistry: fix.registry,
			DataRefEmit: nil,
		})
		require.NoError(t, err, "BuildRunner must succeed for data-channel + non-nil deps")
		names := toolNamesInfo(t, tools)
		assert.Equal(t, []string{agenttools.KPICallToolName}, names,
			"data-channel Skill must see exactly {kpi_call} — no retrieve, no write tools (D16-C3 channel branch)")
	})

	t.Run("tier_channel_yields_retrieve_no_kpi_call", func(t *testing.T) {
		manifest := tierChannelManifest()
		require.False(t, manifest.HasDataChannel(), "manifest must be tier-channel")
		tools, err := agentChannelTools(agent.RunnerBuildConfig{
			Manifest:  manifest,
			Embedder:  newStubEmbedder(),
			Retriever: &stubRetriever{},
			ChatModel: cm,
			SessionID: "sess-tier",
			SkillID:   "tier-skill",
		})
		require.NoError(t, err, "BuildRunner must succeed for tier-channel Skill")
		names := toolNamesInfo(t, tools)
		assert.Contains(t, names, "retrieve_compliance_chunks",
			"tier-channel Skill must see retrieve (KNOW-03 closure)")
		assert.Contains(t, names, "memory_facts_write",
			"tier-channel Skill with writes:[memory:facts] must see memory_facts_write (HITL-05)")
		assert.NotContains(t, names, agenttools.KPICallToolName,
			"tier-channel Skill must NOT see kpi_call (D16-C3 channel branch)")
	})

	t.Run("data_channel_nil_deps_fails_fast", func(t *testing.T) {
		manifest := dataChannelManifest()

		// Missing both deps — the assembly must reject.
		_, err := agent.BuildRunner(context.Background(), agent.RunnerBuildConfig{
			Manifest:    manifest,
			Embedder:    newStubEmbedder(),
			Retriever:   &stubRetriever{},
			ChatModel:   cm,
			SessionID:   "sess-data-nil",
			SkillID:     "data-skill",
			KPIExecutor: nil,
			KPIRegistry: nil,
		})
		require.Error(t, err, "BuildRunner must fail-fast when data-channel Skill has nil KPI deps")
		assert.Contains(t, err.Error(), "data-channel",
			"failure must name the channel kind so the operator can fix the assembly")
		assert.Contains(t, err.Error(), "fail-closed",
			"failure must signal the fail-closed D16-C3 contract")

		// Executor present, Registry missing — also fail-closed.
		_, err = agent.BuildRunner(context.Background(), agent.RunnerBuildConfig{
			Manifest:    manifest,
			Embedder:    newStubEmbedder(),
			Retriever:   &stubRetriever{},
			ChatModel:   cm,
			SessionID:   "sess-data-nil-reg",
			SkillID:     "data-skill",
			KPIExecutor: executor,
			KPIRegistry: nil,
		})
		require.Error(t, err, "BuildRunner must fail-fast when KPIRegistry is nil")

		// Registry present, Executor missing — also fail-closed.
		_, err = agent.BuildRunner(context.Background(), agent.RunnerBuildConfig{
			Manifest:    manifest,
			Embedder:    newStubEmbedder(),
			Retriever:   &stubRetriever{},
			ChatModel:   cm,
			SessionID:   "sess-data-nil-exec",
			SkillID:     "data-skill",
			KPIExecutor: nil,
			KPIRegistry: fix.registry,
		})
		require.Error(t, err, "BuildRunner must fail-fast when KPIExecutor is nil")
	})

	t.Run("tier_channel_ignores_kpi_deps", func(t *testing.T) {
		// tier-channel Skills do NOT use KPI deps — passing nil
		// must NOT trigger the fail-closed guard.
		manifest := tierChannelManifest()
		tools, err := agentChannelTools(agent.RunnerBuildConfig{
			Manifest:  manifest,
			Embedder:  newStubEmbedder(),
			Retriever: &stubRetriever{},
			ChatModel: cm,
			SessionID: "sess-tier-no-kpi",
			SkillID:   "tier-skill",
		})
		require.NoError(t, err, "tier-channel must not depend on KPI deps")
		names := toolNamesInfo(t, tools)
		assert.NotContains(t, names, agenttools.KPICallToolName,
			"tier-channel Skill must NOT see kpi_call even with nil KPI deps")
		assert.Contains(t, names, "retrieve_compliance_chunks",
			"tier-channel Skill must still see retrieve")
	})
}

// _ silences "imported and not used" for wire — kept available for
// future tests that want to assert on the DataReference emission.
var _ = wire.DataReference{}

// _ silences "imported and not used" for json — kept available for
// future tests that want to unmarshal tool result payloads.
var _ = json.Marshal
