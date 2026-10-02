// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// kb-bases — FE client for the KB registry projection (Phase 26 / KBM-02).
//
// Wire source (26-05, frozen):
//   GET /api/knowledge/bases → 200 { bases: [{ slug, display_name, role }] }
//     …active-only, token-free public projection. A nil/unwired registry
//     answers 503 (D26-05-B).
//
// This module is the SINGLE FE source of knowledge-base display names. The
// server is the trust boundary — the projection is display-only and its
// absence degrades (never gates): every consumer falls back to
// `TIER_FALLBACK_LABELS` / `[slug]` and the UI keeps rendering. No CN tier
// literal may appear anywhere else in the FE (26-08 zero-hardcode standard;
// the offline map below is the one whitelisted home).
import { useEffect, useState } from 'react';

/** One entry of the public projection (26-05: slug/display_name/role only). */
export interface KnowledgeBase {
  slug: string;
  display_name: string;
  role: string;
}

interface BasesResponse {
  bases?: KnowledgeBase[];
}

/**
 * Offline fallback labels for the 4 baseline seed slugs
 * (knowledge_bases.yaml). This is the display-name map the FE can answer
 * WITHOUT a network round-trip — the registry projection overrides it once
 * loaded, and any other slug degrades to `[slug]`. Keys are SLUGS; the
 * pre-migration CN tier values are deliberately NOT keys here (they are no
 * longer registry identifiers), so a replayed audit value renders as the
 * bracketed legacy string instead of masquerading as a live tier
 * (T-26-08-2).
 */
export const TIER_FALLBACK_LABELS: Record<string, string> = {
  authority: '[权威]',
  historical: '[历史]',
  personal: '[个人]',
  draft: '[草稿]',
};

/** Synchronous offline label: seed fallback when known, `[slug]` otherwise. */
export function fallbackTierLabel(tier: string): string {
  return TIER_FALLBACK_LABELS[tier] ?? `[${tier}]`;
}

// Module-level promise cache: one projection fetch per browser session. A
// failure is CACHED as [] (no retry storm, no throw) — the display layer
// falls back and the console carries the WARN.
let cachedBases: Promise<KnowledgeBase[]> | null = null;

/**
 * Fetch (once per session) and return the active knowledge-base projection.
 * Resolves to [] — never rejects — when the projection is unreachable,
 * non-2xx, or malformed, so callers can treat it as "no registry data".
 */
export function fetchKnowledgeBases(): Promise<KnowledgeBase[]> {
  if (!cachedBases) {
    cachedBases = (async () => {
      try {
        const resp = await fetch('/api/knowledge/bases', {
          headers: { Accept: 'application/json' },
        });
        if (!resp.ok) {
          throw new Error(`HTTP ${resp.status} ${resp.statusText}`);
        }
        const body = (await resp.json()) as BasesResponse;
        return Array.isArray(body?.bases) ? body.bases : [];
      } catch (err) {
        // eslint-disable-next-line no-console
        console.warn('[kb-bases] registry projection unavailable, using offline fallback:', err);
        return [] as KnowledgeBase[];
      }
    })();
  }
  return cachedBases;
}

/**
 * Bracketed display label for a tier slug: the registry display_name when
 * the slug is registered, `[slug]` otherwise (unknown slug, new KB, or a
 * pre-migration CN value replayed from an audit row). The brackets are part
 * of the contract — this is the KNOW-05 mono-text badge form.
 */
export async function displayNameFor(tier: string): Promise<string> {
  const bases = await fetchKnowledgeBases();
  const hit = bases.find((b) => b.slug === tier);
  return `[${hit ? hit.display_name : tier}]`;
}

/**
 * First active slug carrying the requested semantic role (e.g. 'authority'
 * for the citable role). Returns '' when the projection is unavailable or
 * no base carries the role — the empty string is the honest "no default"
 * answer; callers must not substitute a hardcoded tier for it.
 */
export async function firstSlugByRole(role: string): Promise<string> {
  const bases = await fetchKnowledgeBases();
  return bases.find((b) => b.role === role)?.slug ?? '';
}

/**
 * React hook: the display label for a tier slug.
 *
 * First paint is the offline fallback (or `[slug]`); the registry projection
 * overrides the label once loaded. An explicit `override` short-circuits
 * every lookup (callers that already hold a resolved label). An empty tier
 * resolves to '' — callers render nothing rather than a bare `[]`.
 */
export function useTierLabel(tier: string, override?: string): string {
  const [resolved, setResolved] = useState<string | null>(null);

  useEffect(() => {
    if (override !== undefined || !tier) return;
    let cancelled = false;
    void displayNameFor(tier).then((label) => {
      if (cancelled) return;
      // Skip the state write when the label is unchanged (identical to the
      // current render) — keeps unrelated renders act-clean and quiet.
      setResolved((prev) => ((prev ?? fallbackTierLabel(tier)) === label ? prev : label));
    });
    return () => {
      cancelled = true;
    };
  }, [tier, override]);

  if (override !== undefined) return override;
  if (!tier) return '';
  return resolved ?? fallbackTierLabel(tier);
}
