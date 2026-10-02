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
import { fetchChunkContent } from './chunk-lookup';

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
 * Quick 260929-g7x — one source path → ontology note local name, or null.
 *
 * ECL note identity is the corpus file's leading index: the sql-guides
 * corpus is merged into logical notes keyed by frontmatter source_idx, and
 * the file naming convention prefixes it — "032_保单号查支付方式与开户行.md"
 * → source_idx "032" → KG subject skf:note_032. So the derivation is:
 * basename → leading digits → note_XXX.
 */
export function noteIdFromSource(source: string): string | null {
  const match = /^(\d+)/.exec(basenameOf(source));
  return match ? `note_${match[1]}` : null;
}

/**
 * Quick 260929-g7x — citation sources → ontology note local names for the
 * 本体视图's focused neighborhood fetch (GET /api/admin/ontology/graph?notes=).
 *
 * The `source` citation field is tolerated-but-not-yet-on-the-wire (same
 * tolerance contract as computeFocusMatch above), so an empty result is
 * common and means "no focused neighborhood from source paths". The chunk_id
 * path is covered by resolveFocusNoteIds below.
 */
export function noteIdsFromFocus(focus: GraphFocus | null | undefined): string[] {
  if (!focus) return [];
  const out: string[] = [];
  for (const source of focus.sources) {
    const noteId = noteIdFromSource(source);
    if (noteId && !out.includes(noteId)) out.push(noteId);
  }
  return out;
}

/**
 * Quick 260929-g7x follow-up — chunk_ids → ontology note local names.
 *
 * Verdict citations carry ONLY {chunk_id, span, tier} on the wire — chunk
 * ids are opaque Milvus PKs that cannot be mapped to ECL source_idx without
 * a lookup. This resolves each chunk_id through the EXISTING
 * fetchChunkContent endpoint (GET /api/knowledge/chunk/:id, which returns
 * the chunk's raw `source` path), then applies the same basename
 * leading-digits → note_XXX derivation as noteIdFromFocus.
 *
 * Failure policy (per-chunk isolation, never fail the whole resolution):
 *   - fetchChunkContent resolves null (404 soft-miss)        → skip
 *   - fetchChunkContent rejects (network / 5xx)             → skip
 *   - resolved source is null or has no leading digits      → skip
 * When EVERY chunk fails the result is [] — the caller degrades to 全库.
 * Dedup + first-seen chunk order preserved.
 */
export async function resolveFocusNoteIds(chunkIds: string[]): Promise<string[]> {
  if (chunkIds.length === 0) return [];
  const results = await Promise.allSettled(
    chunkIds.map((id) => fetchChunkContent(id)),
  );
  const out: string[] = [];
  for (const result of results) {
    if (result.status !== 'fulfilled') continue; // fetch threw → skip
    const source = result.value?.source;
    if (!source) continue; // 404 soft-miss / sourceless chunk → skip
    const noteId = noteIdFromSource(source);
    if (noteId && !out.includes(noteId)) out.push(noteId);
  }
  return out;
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
