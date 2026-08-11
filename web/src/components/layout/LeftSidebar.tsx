// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// LeftSidebar — mock skeleton with SESSIONS / FILES / SKILLS sections
// per 04-UI-SPEC §Layout Contract. Phase 4 ships hardcoded items;
// real multi-session management + file tree belongs to Phase 5.
import SkillPicker from '../chat/parts/SkillPicker';

export default function LeftSidebar() {
  return (
    <div
      data-testid="left-sidebar"
      className="h-full bg-surface border-r border-border p-3 overflow-y-auto"
    >
      <section className="mb-4">
        <h4 className="text-xs font-semibold uppercase tracking-wider text-text-dim mb-2">
          SESSIONS
        </h4>
        <ul>
          <li
            data-testid="session-item"
            className="rounded-md px-2 py-1 text-xs font-semibold bg-primary-dim text-text"
          >
            会话 1 (active)
          </li>
        </ul>
      </section>

      <section className="mb-4">
        <h4 className="text-xs font-semibold uppercase tracking-wider text-text-dim mb-2">
          FILES
        </h4>
        <ul className="space-y-1">
          <li data-testid="file-item" className="text-xs text-text-muted font-mono">
            testdata/docs/authority/
          </li>
          <li data-testid="file-item" className="text-xs text-text-muted font-mono">
            testdata/docs/historical/
          </li>
          <li data-testid="file-item" className="text-xs text-text-muted font-mono">
            testdata/docs/personal/
          </li>
          <li data-testid="file-item" className="text-xs text-text-muted font-mono">
            testdata/docs/draft/
          </li>
        </ul>
      </section>

      <section className="mb-4">
        <h4 className="text-xs font-semibold uppercase tracking-wider text-text-dim mb-2">
          SKILLS
        </h4>
        <SkillPicker />
      </section>
    </div>
  );
}
