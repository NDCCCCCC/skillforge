// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 52 (52-03) — LLMProviderPanel vitest coverage.
// Mocks the fetch layer (not the api module) so the wire shape is part of
// the contract: headers must carry X-Admin-Token, save must POST the
// catalog-resolved payload, and the state line mirrors source/mode/available.

import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import LLMProviderPanel from './LLMProviderPanel';

const PROVIDERS = {
  vendors: [
    {
      id: 'zhipu',
      label: '智谱 GLM',
      plans: [
        {
          id: 'coding-plan',
          label: 'Coding Plan',
          base_url: 'https://open.bigmodel.cn/api/coding/paas/v4',
          models_url: '',
          note: '',
        },
      ],
    },
    {
      id: 'opencode',
      label: 'OpenCode',
      plans: [
        {
          id: 'go',
          label: 'Go ($10/月)',
          base_url: 'https://opencode.ai/inference/openai/v1',
          models_url: 'https://opencode.ai/inference/v1/models',
          note: '',
        },
      ],
    },
  ],
};

const CONFIG = {
  source: 'none',
  config: null,
  mode: 'stub',
  available: false,
};

function jsonResponse(body: unknown, ok = true, status = 200) {
  return {
    ok,
    status,
    text: () => Promise.resolve(JSON.stringify(body)),
  };
}

describe('LLMProviderPanel', () => {
  beforeEach(() => {
    window.localStorage.setItem('skillforge.adminToken', 'tok-123');
  });

  it('renders the state line from GET config and the vendor list', async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.endsWith('/providers')) return Promise.resolve(jsonResponse(PROVIDERS));
      if (url.endsWith('/config')) return Promise.resolve(jsonResponse(CONFIG));
      return Promise.resolve(jsonResponse({}));
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<LLMProviderPanel />);
    await waitFor(() => {
      expect(screen.getByTestId('llm-state').textContent).toContain('source=none');
      expect(screen.getByTestId('llm-state').textContent).toContain('mode=stub');
      expect(screen.getByTestId('llm-state').textContent).toContain('available=false');
    });
    const calls = fetchMock.mock.calls as [string, RequestInit][];
    expect(calls.every(([, init]) => (init?.headers as Record<string, string>)['X-Admin-Token'] === 'tok-123')).toBe(
      true,
    );
  });

  it('picking a vendor + plan auto-fills the catalog base URL and save POSTs it', async () => {
    const fetchMock = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/providers')) return Promise.resolve(jsonResponse(PROVIDERS));
      if (url === '/api/admin/llm/config' && (!init?.method || init.method === 'GET')) {
        return Promise.resolve(jsonResponse(CONFIG));
      }
      if (url === '/api/admin/llm/config' && init?.method === 'POST') {
        return Promise.resolve(jsonResponse({ saved: true, mode: 'live', available: true }));
      }
      return Promise.resolve(jsonResponse({}));
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<LLMProviderPanel />);
    await screen.findByTestId('llm-provider-panel');

    fireEvent.change(screen.getByLabelText('厂商'), { target: { value: 'opencode' } });
    // plan auto-picks the first (go) and the base URL auto-fills
    expect(screen.getByLabelText('套餐')).toHaveValue('go');
    expect(screen.getByLabelText('Base URL（可改第三方）')).toHaveValue(
      'https://opencode.ai/inference/openai/v1',
    );

    fireEvent.change(screen.getByLabelText(/API Key（留空则不改动已存 key）/), { target: { value: 'sk-opencode-1' } });
    fireEvent.change(screen.getByLabelText('模型'), { target: { value: 'glm-5.3-flash' } });
    fireEvent.click(screen.getByRole('button', { name: /保存并生效/ }));

    await waitFor(() => {
      expect(screen.getByTestId('llm-msg').textContent).toContain('已保存并立即生效');
    });
    const save = (fetchMock.mock.calls as [string, RequestInit][]).find(
      ([url, init]) => url === '/api/admin/llm/config' && init?.method === 'POST',
    );
    expect(save).toBeTruthy();
    const body = JSON.parse(String(save?.[1]?.body)) as Record<string, string>;
    expect(body).toMatchObject({
      vendor: 'opencode',
      plan: 'go',
      base_url: 'https://opencode.ai/inference/openai/v1',
      model: 'glm-5.3-flash',
      api_key: 'sk-opencode-1',
    });
    expect(body.api_key_env).toBeUndefined();
  });

  it('surfaces a failed test call as an error message', async () => {
    const fetchMock = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/providers')) return Promise.resolve(jsonResponse(PROVIDERS));
      if (url === '/api/admin/llm/config' && (!init?.method || init.method === 'GET')) {
        return Promise.resolve(jsonResponse(CONFIG));
      }
      if (url === '/api/admin/llm/test') {
        return Promise.resolve(
          jsonResponse({ ok: false, latency_ms: 12, status: '401', error: 'key 被拒绝' }),
        );
      }
      return Promise.resolve(jsonResponse({}));
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<LLMProviderPanel />);
    await screen.findByTestId('llm-provider-panel');
    fireEvent.change(screen.getByLabelText('厂商'), { target: { value: 'zhipu' } });
    fireEvent.change(screen.getByLabelText(/API Key（留空则不改动已存 key）/), { target: { value: 'sk-bad' } });
    fireEvent.change(screen.getByLabelText('模型'), { target: { value: 'glm-5.3-flash' } });
    fireEvent.click(screen.getByRole('button', { name: /测试连通/ }));

    await waitFor(() => {
      expect(screen.getByTestId('llm-msg').textContent).toContain('连通失败');
    });
  });
});
