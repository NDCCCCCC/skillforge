// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick 260929-d5w3 Task 1 — graph focus derivation.
//
// Goal: after an answer lands, the knowledge graph defaults to "仅当前问题"
// mode — only the docs (and their tiers) the answer actually cited stay
// highlighted; everything else fades. The user can flip back to 全库.
//
// Data flow:
//   DetailPanel.messages (useChat UIMessage[])
//     → extractGraphFocus()        — latest assistant verdict's citations
//     → KnowledgeGraphBrowser (pass-through)
//     → GraphCanvas.computeFocusMatch() — citations ∩ graph docs
//
// Tolerance contract (mirrors MessageBubble's citation parse at
// MessageBubble.tsx:312-317): the wire Citation is Go snake_case and today
// carries ONLY { chunk_id, span, tier } — there is no `source` field on the
// Go struct (internal/verdict/citation.go). We still tolerate an optional
// `source` path so the doc-filename basename match lights up the moment the
// BE enriches citations, without a FE change.
import type { UIMessage } from 'ai';
import { isVerdict } from '../types/verdict';
import type { KnowledgeGraph } from '../types/knowledge';

/** The citation-derived focus passed DetailPanel → Browser → Canvas. */
export interface GraphFocus {
  /** Citation chunk ids (Go snake_case wire field). */
  chunkIds: Set<string>;
  /** Raw source paths when the citation carries one (optional today). */
  sources: string[];
}

/** Docs/tiers in the graph matched by a focus. Empty match → null. */
export interface GraphFocusMatch {
  /** GraphCanvas PositionedDoc.key === DocNode.id ("tier::source"). */
  docIds: Set<string>;
  /** Registry slugs of the tiers owning ≥1 focus doc. */
  tiers: Set<string>;
}

/** Basename of a path — last '/' or '\' segment wins (mirror of
 * stripFsPath.ts's defensive posture; citations may carry either sep). */
export function basenameOf(path: string): string {
  const i = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
  return i >= 0 ? path.slice(i + 1) : path;
}

/**
 * Extract the LATEST assistant verdict's citations from the chat stream.
 *
 * Scans messages back-to-front for the first assistant message carrying a
 * data-policy part whose payload isVerdict() (same guard as MessageBubble).
 * That verdict decides the outcome — we do NOT fall back to older verdicts:
 *   - ≥1 citation → GraphFocus
 *   - 0 citations (e.g. refusal rendered as status='refused') → null
 *   - no verdict yet (mid-stream / fresh session) → null
 */
export function extractGraphFocus(messages: UIMessage[]): GraphFocus | null {
  for (let i = messages.length - 1; i >= 0; i--) {
    const message = messages[i];
    if (message.role !== 'assistant') continue;
    const part = message.parts.find((p) => p.type === 'data-policy') as
      | { data?: unknown }
      | undefined;
    const data = part?.data;
    if (data === undefined || !isVerdict(data) || !Array.isArray(data.citations)) continue;
    const chunkIds = new Set<string>();
    const sources: string[] = [];
    for (const citation of data.citations) {
      if (
        typeof citation?.chunk_id === 'string' &&
        citation.chunk_id !== '' &&
        !chunkIds.has(citation.chunk_id)
      ) {
        chunkIds.add(citation.chunk_id);
      }
      // NOT on the current wire (Go Citation = chunk_id/span/tier only) —
      // tolerated so the basename match activates when the BE adds it.
      const source = (citation as { source?: unknown }).source;
      if (typeof source === 'string' && source !== '' && !sources.includes(source)) {
        sources.push(source);
      }
    }
    // Latest verdict found — a citation-less verdict (refusal / needs_input)
    // means "no focus", never "dig up the previous answer's citations".
    return chunkIds.size > 0 || sources.length > 0 ? { chunkIds, sources } : null;
  }
  return null;
}

/**
 * True when at least one assistant turn has produced a data-policy payload
 * (verdict OR refusal). Drives the "当前回答无引用原文 — 显示全库" note:
 * an answered turn without citations is a real empty-focus state, whereas a
 * fresh session with zero assistant messages is just… an empty session.
 */
export function hasAnsweredTurn(messages: UIMessage[]): boolean {
  return messages.some(
    (m) => m.role === 'assistant' && m.parts.some((p) => p.type === 'data-policy'),
  );
}

/**
 * Match a focus against the graph: a doc is a FOCUS DOC when ≥1 of its
 * chunks' ids appears in focus.chunkIds, or (when the citation carries a
 * source path) the source's basename equals the doc's filename.
 *
 * The matched docs' tiers become the FOCUS TIERS. Returns null when nothing
 * in the current graph matches (stale session vs re-indexed corpus) — the
 * canvas then stays in full-graph mode instead of fading everything out.
 */
export function computeFocusMatch(
  graph: KnowledgeGraph,
  focus: GraphFocus | null | undefined,
): GraphFocusMatch | null {
  if (!focus || (focus.chunkIds.size === 0 && focus.sources.length === 0)) return null;
  // Basename lookup for the optional source-path match (spec: "重命名后
  // base 已短" — filenames in the graph are already bare basenames).
  const sourceBasenames = new Set(focus.sources.map((s) => basenameOf(s).toLowerCase()));
  const docIds = new Set<string>();
  const tiers = new Set<string>();
  for (const tier of graph.tiers) {
    for (const doc of tier.docs) {
      const chunkHit = doc.chunks.some((c) => focus.chunkIds.has(c.id));
      const nameHit =
        sourceBasenames.size > 0 && sourceBasenames.has(doc.filename.toLowerCase());
      if (chunkHit || nameHit) {
        docIds.add(doc.id);
        tiers.add(tier.tier);
      }
    }
  }
  return docIds.size > 0 ? { docIds, tiers } : null;
}
