// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// KnowledgeGraphBrowser — Plan 05-03 / SC-5 (KNOW-06) empty-state placeholder.
//
// The full Accordion tree (tier → doc → chunk, with search filter + click-
// to-open in 引用原文) ships in Plan 05-05. This plan mounts the empty
// + loading states so the 5-tab CenterPanel is fully wired and the
// graph browser surface is visually discoverable from day one.
//
// Copywriting Contract (05-UI-SPEC §SC-5):
//   - Heading: `知识库为空` (Display size)
//   - Body: `请先通过知识索引导入文档（权威 / 历史 / 个人 / 草稿），或运行 cmd/spiked index 示例语料。`
//   - Loading: `正在加载知识层级...`
//
// The graph browser's data source is `GET /api/knowledge/graph`
// returning `{ tiers: [{ tier, docs: [...] }] }`. The endpoint shape
// is defined in 05-05; the empty + loading UI surfaces ship here.
import { Network } from 'lucide-react';

export interface KnowledgeGraphBrowserProps {
  /**
   * When true, renders the loading state `正在加载知识层级...`
   * instead of the empty hero. Phase 5/05-05 wires this to the
   * fetch lifecycle.
   */
  loading?: boolean;
}

export default function KnowledgeGraphBrowser({ loading = false }: KnowledgeGraphBrowserProps) {
  if (loading) {
    return (
      <div
        data-testid="knowledge-graph-loading"
        className="flex flex-col items-center justify-center text-center min-h-[60vh]"
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
          <Network className="h-6 w-6 text-primary animate-pulse" />
        </div>
        <div className="text-base text-text-muted">正在加载知识层级...</div>
      </div>
    );
  }

  return (
    <div
      data-testid="knowledge-graph-empty"
      className="flex flex-col items-center justify-center text-center min-h-[60vh]"
    >
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
        <Network className="h-6 w-6 text-primary" />
      </div>
      <div className="text-2xl font-semibold text-text">知识库为空</div>
      <p className="mt-2 text-base text-text-muted max-w-md">
        请先通过知识索引导入文档（权威 / 历史 / 个人 / 草稿），或运行 <code className="font-mono text-xs">cmd/spiked index</code> 示例语料。
      </p>
    </div>
  );
}
