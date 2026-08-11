// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// LeftSidebar — the read-only SOURCE LIBRARY (真源资料库). Per the
// denova-aligned IA, session + skill selection both live in the right
// chat panel header; the left sidebar is dedicated to the three-tier
// source library (权威 / 历史 / 个人) + draft — the "true-source
// separation" moat made visible. Phase 4 ships hardcoded tiers; the
// real file tree belongs to Phase 5.
//
// denova-aligned polish (visual only, no token changes): section
// header uses text-muted with a lucide line icon, each source tier is
// a two-line row (tier name + path), rows gain hover affordance.
import { Folder, FolderOpen } from 'lucide-react';

const SOURCES = [
  { tier: '权威', path: 'testdata/docs/authority/' },
  { tier: '历史', path: 'testdata/docs/historical/' },
  { tier: '个人', path: 'testdata/docs/personal/' },
  { tier: '草稿', path: 'testdata/docs/draft/' },
];

export default function LeftSidebar() {
  return (
    <div
      data-testid="left-sidebar"
      className="h-full bg-surface border-r border-border overflow-y-auto"
    >
      <section>
        <div className="flex items-center gap-1.5 px-3 pt-3 pb-2 text-text-muted">
          <FolderOpen className="h-3.5 w-3.5 shrink-0" />
          <h4 className="text-xs font-semibold uppercase tracking-wider">资料库 · SOURCES</h4>
        </div>
        <ul className="px-2 pb-3 space-y-0.5">
          {SOURCES.map((src) => (
            <li
              key={src.path}
              data-testid="file-item"
              className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-surface-2 transition-colors cursor-default"
            >
              <Folder className="h-3.5 w-3.5 shrink-0 text-text-muted" />
              <div className="flex flex-col min-w-0">
                <span className="text-xs font-medium text-text">{src.tier}</span>
                <span className="text-xs font-mono text-text-dim truncate">{src.path}</span>
              </div>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
