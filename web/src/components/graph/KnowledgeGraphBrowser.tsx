// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// KnowledgeGraphBrowser — Plan 05-05 / SC-5 (KNOW-06) full Accordion
// tree implementation. Replaces the 05-03 empty-state placeholder.
//
// The browser fetches GET /api/knowledge/graph once on mount (via
// useKnowledgeGraph) and renders the 3-level tree (tier -> doc ->
// chunk). Empty + loading + error states branch out per UI-SPEC §SC-5:
//
//   - 知识库为空    — when the corpus has 0 docs across all tiers
//                     (one tier entry per registered knowledge base, so
//                     emptiness is detected by summing docCount).
//   - 正在加载知识层级…  — fetch in flight.
//   - 无法加载知识图谱：{error}  — fetch failed; refetch button.
//
// Click a chunk → setCenterView({kind:'chunk-source', ...}). The
// ChunkSourceView tab in CenterPanel lights up (bidirectional nav via
// the controlled Tabs indicator). Click 引用原文 from a verdict
// citation follows the same dispatcher (reuses the CitationCluster
// path).
//
// Quick 260929-g7x — the browser is a two-mode host: 本体
// (OntologyGraphView — the note↔table/product/org ABox entity graph)
// and 文档 (this historical retrieval-corpus tree, kept verbatim).
// The mode chips sit beside the fullscreen trigger and both modes
// render inside the fullscreen overlay.
//
// 默认模式改为「文档」：本体视图走 /api/admin/ontology/graph，是
// X-Admin-Token 门禁的端点 —— 把它放在默认落地页，等于让每个普通
// 用户第一次打开"知识图谱"面板就撞上"需要管理员令牌 + 令牌输入框"，
// 把一个可用功能感知成不可用。文档/语料树视图对所有用户可见，
// 本体视图降级为次级 tab。
import { Maximize2, Network, X } from 'lucide-react';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import GraphCanvas from './GraphCanvas';
import GraphFilterInput from './GraphFilterInput';
import OntologyGraphView from './OntologyGraphView';
import { hasAdminToken } from './ontologyApi';
import { useKnowledgeGraph } from '../../hooks/useKnowledgeGraph';
import type { GraphFocus } from '../../lib/graph-focus';

/**
 * Quick 260929-g7x — view mode. 文档 (default) renders the
 * retrieval-corpus radial tree (GraphCanvas); 本体 renders the
 * note-centric ABox entity graph (OntologyGraphView) and needs an
 * admin token.
 */
type GraphViewMode = 'ontology' | 'document';

const MODE_CHIP_BASE =
  'inline-flex shrink-0 items-center rounded-md px-2 py-1 text-xs font-semibold transition-colors';
const MODE_CHIP_ON = 'bg-primary-dim text-primary';
const MODE_CHIP_OFF = 'text-text-muted hover:bg-surface-2 hover:text-text';

export interface KnowledgeGraphBrowserProps {
  /**
   * Optional override for the loading state. When the parent owns
   * the lifecycle (rare; the hook is self-driving), this prop keeps
   * the public API compatible with the 05-03 placeholder.
   */
  loading?: boolean;
  /**
   * Quick 260929-d5w3 — citations of the latest assistant verdict
   * (DetailPanel extracts them from the chat messages). Passes straight
   * through to BOTH views (GraphCanvas highlight + OntologyGraphView
   * focused neighborhood); the SAME focus drives the fullscreen overlay
   * because both rails render one shared body.
   */
  focus?: GraphFocus | null;
  /** True when ≥1 assistant turn answered — drives the no-citations note. */
  hasAnswer?: boolean;
}

export default function KnowledgeGraphBrowser({
  loading: loadingOverride,
  focus = null,
  hasAnswer = false,
}: KnowledgeGraphBrowserProps = {}) {
  const { graph, status, error, refetch } = useKnowledgeGraph();
  const [filter, setFilter] = useState('');
  // 默认落在「文档」视图 —— 本体视图是管理员门禁的端点，不能当默认
  // 落地页（否则普通用户一进来就看到"需要管理员令牌"）。
  const [mode, setMode] = useState<GraphViewMode>('document');
  // 每帧重读：管理台登录后切回本体 tab 即可生效（localStorage 变化
  // 不会触发 React 重渲染，但切 tab 会）。
  const ontologyAvailable = hasAdminToken();
  // Quick 260929-d5w2 — the detail-panel rail is too narrow for the
  // radial graph; 全屏 swaps the body for a fixed inset-0 overlay that
  // renders the SAME graph body (mode chips + canvas) at viewport size.
  // Portal to document.body (KnowledgeModal B7 posture): ancestors
  // carrying backdrop-filter/transform would otherwise become the
  // containing block and clip the fixed overlay to the panel.
  const [fullscreen, setFullscreen] = useState(false);

  const isLoading = loadingOverride ?? status === 'loading';

  // Esc exits fullscreen — mirror of KnowledgeModal's Esc-close posture.
  useEffect(() => {
    if (!fullscreen) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setFullscreen(false);
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [fullscreen]);

  // Empty-state hero: render when the corpus has 0 docs across all
  // tiers. The BE emits one tier entry per REGISTERED knowledge base
  // (Phase 26 registry — the fixed 4-tier shape is gone), so emptiness is
  // detected by summing docCount rather than by counting tiers.
  const totalDocs = useMemo(
    () => (graph ? graph.tiers.reduce((acc, t) => acc + t.docCount, 0) : 0),
    [graph],
  );

  // Quick 260929-g7x — mode chips (文档 default / 本体). The document-
  // corpus states below (loading/error/empty heroes) are DOCUMENT-mode
  // concerns; the ontology view owns its own data lifecycle and renders
  // regardless of the /api/knowledge/graph corpus state.
  const modeChips = (
    <div className="flex flex-none items-center gap-1" role="group" aria-label="图谱视图切换">
      <button
        type="button"
        data-testid="graph-mode-document"
        aria-pressed={mode === 'document'}
        onClick={() => setMode('document')}
        className={`${MODE_CHIP_BASE} ${mode === 'document' ? MODE_CHIP_ON : MODE_CHIP_OFF}`}
      >
        文档
      </button>
      <button
        type="button"
        data-testid="graph-mode-ontology"
        aria-pressed={mode === 'ontology'}
        onClick={() => setMode('ontology')}
        className={`${MODE_CHIP_BASE} ${mode === 'ontology' ? MODE_CHIP_ON : MODE_CHIP_OFF}`}
      >
        本体
      </button>
    </div>
  );

  /**
   * 模式切换条 + 主体。
   *
   * 文档模式的空/错/加载 hero 必须把 chips 一起带上 —— 否则用户在
   * "知识库为空"这一屏上根本看不到切换入口（chips 被早返回吃掉了），
   * 也就无法切到本体视图查看实体关系。
   */
  const withModeChips = (body: ReactNode) => (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="flex flex-none items-center gap-2">{modeChips}</div>
      {body}
    </div>
  );

  if (mode === 'document' && status === 'error') {
    return withModeChips(
      <div
        data-testid="knowledge-graph-error"
        className="flex flex-col items-center justify-center text-center min-h-[60vh]"
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-status-non-compliant/20 mb-4">
          <Network className="h-6 w-6 text-status-non-compliant" />
        </div>
        <div className="text-2xl font-semibold text-text">
          无法加载知识图谱
        </div>
        <p className="mt-2 text-base text-text-muted max-w-md">
          {error ?? '请稍后重试，或确认服务端已启用 /api/knowledge/graph。'}
        </p>
        <button
          type="button"
          data-testid="graph-refresh"
          onClick={refetch}
          className="mt-4 rounded-md border border-border bg-surface-1 px-3 py-1.5 text-xs font-semibold text-text hover:bg-surface-2"
        >
          重试
        </button>
      </div>,
    );
  }

  if (mode === 'document' && isLoading) {
    return withModeChips(
      <div
        data-testid="knowledge-graph-loading"
        className="flex flex-col items-center justify-center text-center min-h-[60vh]"
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
          <Network className="h-6 w-6 text-primary animate-pulse" />
        </div>
        <div className="text-base text-text-muted">正在加载知识层级...</div>
      </div>,
    );
  }

  if (mode === 'document' && (!graph || totalDocs === 0)) {
    return withModeChips(
      <div
        data-testid="knowledge-graph-empty"
        className="flex flex-col items-center justify-center text-center min-h-[60vh]"
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
          <Network className="h-6 w-6 text-primary" />
        </div>
        <div className="text-2xl font-semibold text-text">知识库为空</div>
        <p className="mt-2 text-base text-text-muted max-w-md">
          请先通过知识索引导入文档到已注册的知识库，或运行 <code className="font-mono text-xs">scripts/spiked index</code> 示例语料。
        </p>
      </div>,
    );
  }

  // Shared render block — the narrow rail and the fullscreen overlay both
  // render exactly this (mode chips + [filter] + fullscreen trigger +
  // active view). flex-1 min-h-0 gives the canvas the full container
  // height in fullscreen mode while the narrow rail keeps its scroll
  // behavior.
  const graphBody = (
    <div data-testid="knowledge-graph" className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="flex flex-none items-center gap-2">
        {modeChips}
        {mode === 'document' ? (
          <div className="min-w-0 flex-1">
            <GraphFilterInput value={filter} onChange={setFilter} />
          </div>
        ) : (
          <div className="min-w-0 flex-1" />
        )}
        <button
          type="button"
          data-testid="graph-fullscreen-btn"
          onClick={() => setFullscreen(true)}
          title="全屏查看知识图谱"
          className="inline-flex shrink-0 items-center gap-1 rounded-md border border-border bg-surface-1 px-2 py-1.5 text-xs font-semibold text-text hover:bg-surface-2"
        >
          <Maximize2 className="h-3.5 w-3.5" />
          全屏
        </button>
      </div>
      <div className="min-h-0 flex-1">
        {mode === 'ontology' ? (
          ontologyAvailable ? (
            <OntologyGraphView focus={focus} />
          ) : (
            // 没有管理员令牌时不要把"需要令牌 + 令牌输入框"摆成
            // 落地页 —— 那会让普通用户以为整个面板坏了。一句说明
            // 足够：本体关系图是管理员功能，文档视图继续可用。
            <div
              data-testid="ontology-needs-admin"
              className="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 text-center"
            >
              <div className="text-sm font-semibold text-text">本体视图需要管理员权限</div>
              <p className="max-w-md text-xs text-text-muted">
                本体视图读取知识库里的实体关系投影，属于管理员功能。
                如需查看，请在管理控制台登录后回到这里；文档视图对所有用户可用。
              </p>
            </div>
          )
        ) : graph ? (
          // Unreachable-null guard: the document-mode early returns above
          // guarantee a populated corpus graph before this branch renders.
          <GraphCanvas graph={graph} filter={filter} focus={focus} hasAnswer={hasAnswer} />
        ) : null}
      </div>
    </div>
  );

  if (fullscreen) {
    return createPortal(
      <div
        data-testid="graph-fullscreen-overlay"
        className="fixed inset-0 z-50 flex flex-col bg-bg"
      >
        <div className="flex h-12 flex-none items-center justify-between border-b border-border px-4">
          <span className="text-sm font-semibold tracking-wide text-text">知识图谱</span>
          <button
            type="button"
            data-testid="graph-fullscreen-close"
            onClick={() => setFullscreen(false)}
            aria-label="退出全屏"
            className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="flex min-h-0 flex-1 flex-col p-4">{graphBody}</div>
      </div>,
      document.body,
    );
  }

  return graphBody;
}