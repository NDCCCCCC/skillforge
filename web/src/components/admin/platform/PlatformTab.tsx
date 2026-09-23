// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-04 Task 0 — Platform tab placeholder shell.
//
// This is a minimal placeholder. Plan 05 replaces it with the real
// three-view implementation (SparqlPlaygroundView / McpInspectorView /
// HybridDebugView). The placeholder exists so AdminDashboard can import
// it without errors during the Plan 04 wiring phase.

export default function PlatformTab() {
  return <div data-testid="platform-tab-placeholder">Platform tab placeholder</div>;
}
