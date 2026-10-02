// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 / SC-5 (KNOW-06) — GraphFilterInput.
//
// shadcn Command-style search input rendered at the top of the graph
// panel. Filters doc + chunk nodes by filename / chunk text substring
// (case-insensitive). Empty filter → full tree.
//
// Implementation note: the actual filter logic lives in GraphTree (so
// the same input can drive the file-tree LeftSidebar component in a
// future phase). This component is purely the controlled-input surface.

import { Search } from 'lucide-react';

export interface GraphFilterInputProps {
  value: string;
  onChange: (next: string) => void;
}

export default function GraphFilterInput({
  value,
  onChange,
}: GraphFilterInputProps) {
  return (
    <div
      data-testid="graph-filter-input"
      className="relative flex items-center"
    >
      <Search
        className="pointer-events-none absolute left-2 h-3.5 w-3.5 text-text-dim"
        aria-hidden
      />
      <input
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="按文档名 / chunk ID 过滤"
        aria-label="知识图谱过滤"
        className="w-full rounded-md border border-border bg-surface-1 py-1.5 pl-7 pr-2 text-xs text-text placeholder:text-text-dim"
      />
    </div>
  );
}