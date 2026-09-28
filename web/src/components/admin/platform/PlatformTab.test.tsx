// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// quick-260928-jxb Task 2 — regression pins for the shared Skill selector.
// The 2026-09-28 operator walkthrough found PlatformTab held skillName:''
// with NO selector — SPARQL Run Query and Hybrid 搜索 always sent
// X-Skill-Name: '' → 401. These tests pin: selector renders, auto-defaults
// to the first ok skill (never ''), failed skills are absent, and one
// selector drives the shared state for BOTH views.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import PlatformTab from './PlatformTab';
import { useAppStore } from '../../../store/app-store';

beforeEach(() => {
  localStorage.setItem('sf_admin_token', 'tok-abc');
  useAppStore.setState({
    skills: [
      { id: '1', name: 'compliance', description: '', status: 'ok' },
      { id: '2', name: 'calculator', description: '', status: 'ok' },
      { id: '3', name: 'broken', description: '', status: 'failed' },
    ],
  });
  // Child views degrade on !ok — they only mount data on user action.
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(new Response(JSON.stringify({ error: 'not stubbed' }), { status: 404 }))),
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('PlatformTab — shared Skill selector (quick-260928-jxb BUG-2)', () => {
  it('renders the selector and auto-defaults to the first ok skill (never empty string)', async () => {
    render(<PlatformTab />);

    const select = await waitFor(() => {
      const el = screen.getByTestId('platform-skill-select') as HTMLSelectElement;
      expect(el.value).toBe('compliance');
      return el;
    });
    expect(select.value).not.toBe('');
  });

  it('lists ok skills only — failed skills are absent from the options', () => {
    render(<PlatformTab />);

    const select = screen.getByTestId('platform-skill-select');
    const names = Array.from(select.querySelectorAll('option')).map((o) => o.value);
    expect(names).toContain('compliance');
    expect(names).toContain('calculator');
    expect(names).not.toContain('broken');
  });

  it('selecting a different skill updates the shared state for both SPARQL and Hybrid views', () => {
    render(<PlatformTab />);

    const select = screen.getByTestId('platform-skill-select') as HTMLSelectElement;
    fireEvent.change(select, { target: { value: 'calculator' } });
    expect(select.value).toBe('calculator');
  });
});
