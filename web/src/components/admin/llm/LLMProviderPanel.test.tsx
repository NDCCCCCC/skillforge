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

  it('261113-ixo: config missing api_key_env (env/omitempty) must not crash trim on 拉取模型列表', async () => {
    // Regression: for source=env the backend omits api_key_env (Go
    // omitempty / env-derived config). The panel used to copy that
    // undefined straight into state, so buildRequest()'s apiKeyEnv.trim()
    // threw a TypeError — surfaced as an error message and the models
    // click never reached the wire.
    const ENV_CONFIG = {
      source: 'env',
      config: {
        vendor: 'opencode',
        plan: 'go',
        base_url: 'https://opencode.ai/inference/openai/v1',
        model: 'glm-5.3-flash',
        api_key_masked: '',
        api_key_set: true,
        // api_key_env deliberately ABSENT — runtime contract, not the
        // optimistic LLMConfigView type
      },
      mode: 'live',
      available: true,
    };
    const fetchMock = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/providers')) return Promise.resolve(jsonResponse(PROVIDERS));
      if (url === '/api/admin/llm/config' && (!init?.method || init.method === 'GET')) {
        return Promise.resolve(jsonResponse(ENV_CONFIG));
      }
      if (url.startsWith('/api/admin/llm/models')) {
        return Promise.resolve(jsonResponse({ models: ['glm-5.3-flash', 'glm-4.7-air'] }));
      }
      return Promise.resolve(jsonResponse({}));
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<LLMProviderPanel />);
    await screen.findByTestId('llm-provider-panel');

    fireEvent.click(screen.getByRole('button', { name: /拉取模型列表/ }));

    await waitFor(() => {
      expect(screen.getByTestId('llm-msg').textContent).toContain('拉到 2 个模型');
    });
    const modelsCall = (fetchMock.mock.calls as [string, RequestInit][]).find(([url]) =>
      url.startsWith('/api/admin/llm/models'),
    );
    expect(modelsCall).toBeTruthy();
    // the pre-fix TypeError text ("…reading 'trim'") must never surface
    expect(screen.getByTestId('llm-msg').textContent).not.toContain('trim');
  });
  it('261126: fetched models render a real <select> and reconcile the stale model value', async () => {
    // Live report: the panel fetched 2 models (a deepseek-only endpoint) but
    // the 模型 field kept showing glm-5.3-flash from the saved/env config, and
    // the picker could not be opened at all — it was a <datalist>, which only
    // hints while typing. Both halves are pinned here.
    const DS_CONFIG = {
      source: 'env',
      config: {
        vendor: 'custom',
        plan: 'custom',
        base_url: 'https://43.166.20.53:8080/v1',
        model: 'glm-5.3-flash',
      },
      mode: 'live',
      available: true,
    };
    const fetchMock = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/providers')) return Promise.resolve(jsonResponse(PROVIDERS));
      if (url === '/api/admin/llm/config' && (!init?.method || init.method === 'GET')) {
        return Promise.resolve(jsonResponse(DS_CONFIG));
      }
      if (url === '/api/admin/llm/models') {
        return Promise.resolve(jsonResponse({ models: ['deepseek-v4.1-flash', 'deepseek-v4.1-flash-eco'] }));
      }
      return Promise.resolve(jsonResponse({}));
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<LLMProviderPanel />);
    await screen.findByTestId('llm-provider-panel');

    // Before the fetch: free-text input (manual entry stays possible).
    expect(screen.getByTestId('llm-model-input')).toHaveValue('glm-5.3-flash');

    fireEvent.click(screen.getByRole('button', { name: /拉取模型列表/ }));

    await waitFor(() => {
      expect(screen.getByTestId('llm-msg').textContent).toContain('拉到 2 个模型');
    });

    // After the fetch: a real <select> with BOTH fetched models is clickable.
    const select = await screen.findByTestId('llm-model-select');
    const options = Array.from((select as HTMLSelectElement).options).map((o) => o.value);
    expect(options).toEqual(['deepseek-v4.1-flash', 'deepseek-v4.1-flash-eco']);
    // The stale value was reconciled to the first advertised model...
    expect((select as HTMLSelectElement).value).toBe('deepseek-v4.1-flash');
    // ...and the message said so instead of silently swapping.
    expect(screen.getByTestId('llm-msg').textContent).toContain('不在列表内');
  });

  it('261126: a model already present in the fetched list is NOT swapped', async () => {
    const fetchMock = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/providers')) return Promise.resolve(jsonResponse(PROVIDERS));
      if (url === '/api/admin/llm/config' && (!init?.method || init.method === 'GET')) {
        return Promise.resolve(jsonResponse(CONFIG));
      }
      if (url === '/api/admin/llm/models') {
        return Promise.resolve(jsonResponse({ models: ['glm-5.3-flash', 'glm-4.7-air'] }));
      }
      return Promise.resolve(jsonResponse({}));
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<LLMProviderPanel />);
    await screen.findByTestId('llm-provider-panel');
    fireEvent.change(screen.getByLabelText('模型'), { target: { value: 'glm-4.7-air' } });
    fireEvent.click(screen.getByRole('button', { name: /拉取模型列表/ }));

    await waitFor(() => {
      expect(screen.getByTestId('llm-msg').textContent).toContain('拉到 2 个模型');
    });
    const select = await screen.findByTestId('llm-model-select');
    expect((select as HTMLSelectElement).value).toBe('glm-4.7-air');
    // No swap => no "不在列表内" note.
    expect(screen.getByTestId('llm-msg').textContent).not.toContain('不在列表内');
  });
});
