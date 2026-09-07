// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 29-08 Task 1 — wiring placeholder for the 「复核」 tab (FBK-02).
// Task 1 only mounts the tab so the shell compiles and the tab-order
// test is green; the full pending-queue surface (chunk aggregation +
// three dispositions + disposition history) lands in this plan's Task 2.
export default function ReviewQueuePanel() {
  return <div data-testid="review-view" />;
}
