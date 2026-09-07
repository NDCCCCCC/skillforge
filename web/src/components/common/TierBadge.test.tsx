// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 26-08 Task 1 — TierBadge is REGISTRY-DRIVEN (Phase 26 / KBM-02).
//
// Pins the 26-08 display contract:
//   1. a slug the registry projection knows renders the registry display_name
//   2. the baseline seed slugs render their display name on FIRST PAINT
//      (offline fallback — no network round-trip needed for the 4 seed KBs)
//   3. an unknown slug degrades to `[slug]` — never a crash, never a blank
//   4. a pre-migration CN tier value (historical audit replay) degrades to a
//      bracketed label too — it is NOT a registry slug any more
//   5. a failed projection fetch keeps the `[slug]` fallback
//   6. the KNOW-05 mono-text class + data-tier attribute are unchanged
//      (tier is differentiated by TEXT, never by color — sketch findings)
//   7. an explicit displayName prop wins over every lookup
//   8. an empty tier renders nothing (the badge-hidden degradation)
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';

// kb-bases caches its projection promise at module scope, so every test
// re-imports the module graph after vi.resetModules() — the cache can
// never bleed across cases.
async function freshTierBadge() {
  vi.resetModules();
  const mod = await import('./TierBadge');
  return mod.default;
}

const KNOWN_BASES = [
  { slug: 'authority', display_name: '权威', role: 'authority' },
  { slug: 'kb-2026', display_name: '新库', role: 'historical' },
];

function basesOk(bases: unknown[] = KNOWN_BASES): Response {
  return new Response(JSON.stringify({ bases }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

function stubFetch(impl: () => Promise<Response>): void {
  vi.stubGlobal('fetch', vi.fn(impl));
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('TierBadge', () => {
  it('renders the registry display_name for a known slug', async () => {
    stubFetch(() => Promise.resolve(basesOk()));
    const TierBadge = await freshTierBadge();
    render(<TierBadge tier="kb-2026" />);
    expect(await screen.findByText('[新库]')).toBeInTheDocument();
  });

  it('renders the baseline seed display name on FIRST PAINT (offline fallback)', async () => {
    stubFetch(() => Promise.resolve(basesOk()));
    const TierBadge = await freshTierBadge();
    render(<TierBadge tier="authority" />);
    expect(screen.getByTestId('tier-badge')).toHaveTextContent('[权威]');
  });

  it('degrades an unregistered slug to `[slug]` (no crash, no blank)', async () => {
    stubFetch(() => Promise.resolve(basesOk([])));
    const TierBadge = await freshTierBadge();
    render(<TierBadge tier="kb-2026" />);
    expect(screen.getByTestId('tier-badge')).toHaveTextContent('[kb-2026]');
  });

  it('degrades a pre-migration CN tier value to a bracketed label', async () => {
    // An audit replay carries tier:"权威" — NOT a registry slug any more,
    // so it must render as the bracketed legacy value, never as a live tier.
    stubFetch(() => Promise.resolve(basesOk([])));
    const TierBadge = await freshTierBadge();
    render(<TierBadge tier="权威" />);
    expect(screen.getByTestId('tier-badge')).toHaveTextContent('[权威]');
  });

  it('keeps the `[slug]` fallback when the projection fetch rejects', async () => {
    stubFetch(() => Promise.reject(new Error('network down')));
    const TierBadge = await freshTierBadge();
    render(<TierBadge tier="kb-2026" />);
    expect(screen.getByTestId('tier-badge')).toHaveTextContent('[kb-2026]');
  });

  it('keeps the KNOW-05 mono-text class + data-tier attribute unchanged', async () => {
    stubFetch(() => Promise.resolve(basesOk()));
    const TierBadge = await freshTierBadge();
    render(<TierBadge tier="authority" />);
    const badge = screen.getByTestId('tier-badge');
    expect(badge).toHaveClass(
      'inline-flex',
      'items-center',
      'rounded-md',
      'border',
      'border-border',
      'bg-surface-2',
      'px-2',
      'py-0.5',
      'font-mono',
      'text-xs',
      'font-semibold',
      'text-primary',
    );
    expect(badge).toHaveAttribute('data-tier', 'authority');
  });

  it('an explicit displayName prop wins over the projection lookup', async () => {
    stubFetch(() => Promise.resolve(basesOk()));
    const TierBadge = await freshTierBadge();
    render(<TierBadge tier="kb-2026" displayName="[定制名]" />);
    const badge = screen.getByTestId('tier-badge');
    expect(badge).toHaveTextContent('[定制名]');
  });

  it('renders nothing for an empty tier (badge-hidden degradation)', async () => {
    stubFetch(() => Promise.resolve(basesOk()));
    const TierBadge = await freshTierBadge();
    const { container } = render(<TierBadge tier="" />);
    expect(container).toBeEmptyDOMElement();
  });
});
