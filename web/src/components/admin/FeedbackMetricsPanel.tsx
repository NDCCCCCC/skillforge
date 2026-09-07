// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 29-08 Task 1 — wiring placeholder for the 「反馈」 tab (FBK-03).
// Task 1 only mounts the tab so the shell compiles and the tab-order
// test is green; the metrics surface (口径注 + summary cards + 日/周
// trend + 待复核 cross-link) lands in this plan's Task 3.
export default function FeedbackMetricsPanel() {
  return <div data-testid="metrics-view" />;
}
