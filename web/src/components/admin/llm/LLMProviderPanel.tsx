// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 52 (52-03) — the admin LLM provider panel.
//
// One screen, five verbs:
//   1. pick a vendor (厂商) — the catalog carries the plan→base URL mapping
//   2. pick a plan (套餐) — base URL + models URL auto-fill, custom edits stay
//   3. enter the key (paste, or reference an env var by name)
//   4. TEST — minimal chat-completion probe with classified errors
//   5. SAVE — persists to ~/.skillforge/llm.json (0600) and hot-applies
//
// Everything here is X-Admin-Token gated; the token lives in localStorage
// under the same key the rest of the admin console uses.

import { useCallback, useEffect, useState } from 'react';
import { Bot, KeyRound, Loader2, PlugZap, Save } from 'lucide-react';
import {
  fetchLLMConfig,
  fetchLLMModels,
  fetchLLMProviders,
  saveLLMConfig,
  testLLMKey,
  type LLMConfigState,
  type LLMSaveRequest,
  type LLMVendor,
} from './llmApi';

const TOKEN_KEY = 'skillforge.adminToken';

function readToken(): string {
  try {
    return window.localStorage.getItem(TOKEN_KEY) ?? '';
  } catch {
    return '';
  }
}

export default function LLMProviderPanel() {
  const [token, setToken] = useState('');
  const [vendors, setVendors] = useState<LLMVendor[]>([]);
  const [state, setState] = useState<LLMConfigState | null>(null);
  const [vendor, setVendor] = useState('');
  const [plan, setPlan] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [model, setModel] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [apiKeyEnv, setApiKeyEnv] = useState('');
  const [models, setModels] = useState<string[]>([]);
  const [busy, setBusy] = useState<'' | 'test' | 'save' | 'models' | 'load'>('');
  const [msg, setMsg] = useState<{ kind: 'ok' | 'err'; text: string } | null>(null);

  const currentPlan = vendors.find((v) => v.id === vendor)?.plans.find((p) => p.id === plan);

  const loadAll = useCallback(async (tok: string) => {
    setBusy('load');
    try {
      const [prov, cfg] = await Promise.all([fetchLLMProviders(tok), fetchLLMConfig(tok)]);
      setVendors(prov.vendors);
      setState(cfg);
      if (cfg.config) {
        setVendor(cfg.config.vendor);
        setPlan(cfg.config.plan);
        setBaseUrl(cfg.config.base_url);
        setModel(cfg.config.model);
        setApiKeyEnv(cfg.config.api_key_env);
      }
      setMsg(null);
    } catch (e) {
      setMsg({ kind: 'err', text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy('');
    }
  }, []);

  useEffect(() => {
    const tok = readToken();
    setToken(tok);
    if (tok) void loadAll(tok);
  }, [loadAll]);

  function pickVendor(id: string) {
    setVendor(id);
    const v = vendors.find((x) => x.id === id);
    const first = v?.plans[0];
    setPlan(first?.id ?? '');
    setBaseUrl(first?.base_url ?? '');
  }

  function pickPlan(id: string) {
    setPlan(id);
    const p = currentVendor()?.plans.find((x) => x.id === id);
    setBaseUrl(p?.base_url ?? '');
  }

  function currentVendor(): LLMVendor | undefined {
    return vendors.find((v) => v.id === vendor);
  }

  function buildRequest(): LLMSaveRequest {
    const req: LLMSaveRequest = { vendor, plan, base_url: baseUrl, model };
    if (apiKeyEnv.trim()) req.api_key_env = apiKeyEnv.trim();
    else if (apiKey.trim()) req.api_key = apiKey.trim();
    return req;
  }

  async function onTest() {
    setBusy('test');
    setMsg(null);
    try {
      const r = await testLLMKey(token, buildRequest());
      setMsg(
        r.ok
          ? { kind: 'ok', text: `连通成功 (${r.latency_ms}ms, ${r.status})` }
          : { kind: 'err', text: `连通失败: ${r.error ?? r.status}` },
      );
    } catch (e) {
      setMsg({ kind: 'err', text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy('');
    }
  }

  async function onSave() {
    setBusy('save');
    setMsg(null);
    try {
      const r = await saveLLMConfig(token, buildRequest());
      const savedMsg: { kind: 'ok' | 'err'; text: string } = {
        kind: 'ok',
        text: r.available ? '已保存并立即生效' : '已保存（当前无可用上游，仍处于 stub 模式）',
      };
      await loadAll(token);
      // loadAll clears msg on its success path; re-assert AFTER the refresh
      // so the save verdict stays visible next to the refreshed state line.
      setMsg(savedMsg);
    } catch (e) {
      setMsg({ kind: 'err', text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy('');
    }
  }

  async function onModels() {
    setBusy('models');
    setMsg(null);
    try {
      const r = await fetchLLMModels(token, buildRequest());
      setModels(r.models);
      setMsg({ kind: 'ok', text: `拉到 ${r.models.length} 个模型` });
    } catch (e) {
      setMsg({ kind: 'err', text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy('');
    }
  }

  const input =
    'w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';

  return (
    <div className="mx-auto max-w-3xl space-y-4 p-4" data-testid="llm-provider-panel">
      <div className="flex items-center gap-2">
        <Bot className="h-5 w-5" />
        <h2 className="text-lg font-semibold">LLM Provider 配置</h2>
        {state && (
          <span className="ml-auto text-xs text-muted-foreground" data-testid="llm-state">
            source={state.source} mode={state.mode} available={String(state.available)}
          </span>
        )}
      </div>

      {!token && (
        <div className="space-y-2 rounded-md border p-3">
          <label className="text-sm font-medium">Admin Token (X-Admin-Token)</label>
          <input
            className={input}
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder="SKILLFORGE_ADMIN_TOKEN 的值"
          />
          <button
            type="button"
            className="rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground"
            onClick={() => {
              try {
                window.localStorage.setItem(TOKEN_KEY, token);
              } catch {
                /* private mode — keep in-memory only */
              }
              void loadAll(token);
            }}
          >
            连接
          </button>
        </div>
      )}

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <label htmlFor="llm-vendor" className="text-sm font-medium">厂商</label>
          <select id="llm-vendor" className={input} value={vendor} onChange={(e) => pickVendor(e.target.value)}>
            <option value="">选择厂商…</option>
            {vendors.map((v) => (
              <option key={v.id} value={v.id}>
                {v.label}
              </option>
            ))}
          </select>
        </div>
        <div className="space-y-1">
          <label htmlFor="llm-plan" className="text-sm font-medium">套餐</label>
          <select id="llm-plan" className={input} value={plan} onChange={(e) => pickPlan(e.target.value)} disabled={!vendor}>
            <option value="">选择套餐…</option>
            {currentVendor()?.plans.map((p) => (
              <option key={p.id} value={p.id}>
                {p.label}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="space-y-1">
        <label htmlFor="llm-base-url" className="text-sm font-medium">Base URL（可改第三方）</label>
        <input id="llm-base-url" className={input} value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://…/v1" />
      </div>

      <div className="space-y-1">
        <label htmlFor="llm-api-key" className="text-sm font-medium">
          <KeyRound className="mr-1 inline h-4 w-4" />API Key（留空则不改动已存 key）
        </label>
        <input
          id="llm-api-key"
          className={input}
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder="sk-…（或改用下方 env 间接引用）"
          autoComplete="off"
        />
      </div>
      <div className="space-y-1">
        <label htmlFor="llm-api-key-env" className="text-sm font-medium">或 Key 的环境变量名</label>
        <input
          className={input}
          value={apiKeyEnv}
          onChange={(e) => setApiKeyEnv(e.target.value)}
          placeholder="如 OPENCODE_API_KEY（优先于上面的明文）"
        />
      </div>

      <div className="flex items-end gap-3">
        <div className="flex-1 space-y-1">
          <label htmlFor="llm-model" className="text-sm font-medium">模型</label>
          <input
            id="llm-model"
            className={input}
            list="llm-model-options"
            value={model}
            onChange={(e) => setModel(e.target.value)}
            placeholder="glm-5.3-flash"
          />
          <datalist id="llm-model-options">
            {models.map((m) => (
              <option key={m} value={m} />
            ))}
          </datalist>
        </div>
        <button type="button" className="rounded-md border px-3 py-2 text-sm" onClick={() => void onModels()} disabled={busy !== ''}>
          {busy === 'models' ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
          拉取模型列表
        </button>
      </div>

      <div className="flex gap-3">
        <button
          type="button"
          className="flex items-center gap-1 rounded-md border px-3 py-2 text-sm"
          onClick={() => void onTest()}
          disabled={busy !== '' || !vendor}
        >
          {busy === 'test' ? <Loader2 className="h-4 w-4 animate-spin" /> : <PlugZap className="h-4 w-4" />}
          测试连通
        </button>
        <button
          type="button"
          className="flex items-center gap-1 rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground"
          onClick={() => void onSave()}
          disabled={busy !== '' || !vendor || !model}
        >
          {busy === 'save' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
          保存并生效
        </button>
      </div>

      {msg && (
        <p className={msg.kind === 'ok' ? 'text-sm text-green-600' : 'text-sm text-red-600'} data-testid="llm-msg">
          {msg.text}
        </p>
      )}
    </div>
  );
}
