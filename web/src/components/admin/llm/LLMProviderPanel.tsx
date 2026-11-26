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
//
// 2610 redesign: the three-section form wrapped in shared SectionCards
// with a PageHeader, buttons migrated to the shared Button, the ok/err
// message colors moved from hardcoded text-green-600/text-red-600 to
// status tokens. Deliberately NOT migrated to useAdminData/adminFetch:
// this panel's token is the user-entered localStorage
// 'skillforge.adminToken' passed explicitly to llmApi (not the shared
// 'sf_admin_token'), and the regression suite pins that header
// semantics — the hand-rolled load path stays.

import { useCallback, useEffect, useState } from 'react';
import { KeyRound, Loader2, PlugZap, Save } from 'lucide-react';
import { PageHeader, SectionCard } from '../shared/Primitives';
import { Button } from '../../ui/button';
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

// CODE-AUDIT 2026-11-05 (D-02): unify admin token storage key.
// Primary key is 'sf_admin_token' (shared by all other admin components),
// while retaining backward compatibility with 'skillforge.adminToken'.
const PRIMARY_TOKEN_KEY = 'sf_admin_token';
const LEGACY_TOKEN_KEY = 'skillforge.adminToken';

function readToken(): string {
  try {
    return (
      window.localStorage.getItem(PRIMARY_TOKEN_KEY) ??
      window.localStorage.getItem(LEGACY_TOKEN_KEY) ??
      ''
    );
  } catch {
    return '';
  }
}

function writeToken(val: string): void {
  try {
    window.localStorage.setItem(PRIMARY_TOKEN_KEY, val);
    window.localStorage.setItem(LEGACY_TOKEN_KEY, val);
  } catch {
    /* private mode — keep in-memory only */
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


  const loadAll = useCallback(async (tok: string): Promise<boolean> => {
    setBusy('load');
    try {
      const [prov, cfg] = await Promise.all([fetchLLMProviders(tok), fetchLLMConfig(tok)]);
      setVendors(prov.vendors);
      setState(cfg);
      if (cfg.config) {
        // 261113-ixo: backend omits fields (Go omitempty / env-derived
        // config) — reflected as optional in LLMConfigView since
        // 261113-mkc. Backfill with '' to keep the state contract
        // (string) honest — buildRequest trims these.
        setVendor(cfg.config.vendor ?? '');
        setPlan(cfg.config.plan ?? '');
        setBaseUrl(cfg.config.base_url ?? '');
        setModel(cfg.config.model ?? '');
        setApiKeyEnv(cfg.config.api_key_env ?? '');
      }
      setMsg(null);
      return true;
    } catch (e) {
      setMsg({ kind: 'err', text: e instanceof Error ? e.message : String(e) });
      return false;
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
    // 261113-ixo: belt-and-braces — even with the loadAll backfill the
    // state contract must never feed undefined into trim().
    const envName = apiKeyEnv ?? '';
    const plainKey = apiKey ?? '';
    const req: LLMSaveRequest = { vendor, plan, base_url: baseUrl, model };
    if (envName.trim()) req.api_key_env = envName.trim();
    else if (plainKey.trim()) req.api_key = plainKey.trim();
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
      // CODE-AUDIT 2026-11-05 (E-07): loadAll errors should not be silently
      // overwritten with savedMsg. Only re-assert savedMsg on success.
      const ok = await loadAll(token);
      if (ok) {
        setMsg(savedMsg);
      }
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
      // 261126-list-reconcile: fetching the list used to leave `model`
      // untouched, so the field kept showing whatever the saved config had
      // (e.g. glm-5.3-flash from .env) even when the endpoint only serves
      // deepseek-*. That made the panel look broken: "拉到 2 个模型" but the
      // visible model was not one of them. Reconcile it: an absent current
      // value is replaced by the first fetched model, and the message says
      // so explicitly (never silently swap a model the user chose).
      const current = model.trim();
      if (r.models.length > 0 && !r.models.includes(current)) {
        const next = r.models[0];
        setModel(next);
        setMsg({
          kind: 'ok',
          text: current
            ? `拉到 ${r.models.length} 个模型；当前模型「${current}」不在列表内，已切换为「${next}」`
            : `拉到 ${r.models.length} 个模型，已选择「${next}」`,
        });
      } else {
        setMsg({ kind: 'ok', text: `拉到 ${r.models.length} 个模型` });
      }
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
      <PageHeader
        title="LLM Provider 配置"
        description="选择厂商与套餐，配置 API Key 并验证连通后保存生效"
        actions={
          state ? (
            <span className="text-xs text-muted-foreground" data-testid="llm-state">
              source={state.source} mode={state.mode} available={String(state.available)}
            </span>
          ) : undefined
        }
      />

      {!token && (
        <SectionCard title="连接" description="输入 Admin Token 以加载厂商目录与当前配置">
          <div className="space-y-2">
            <label className="text-sm font-medium">Admin Token (X-Admin-Token)</label>
            <input
              className={input}
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="SKILLFORGE_ADMIN_TOKEN 的值"
            />
            <Button
              type="button"
              onClick={() => {
                writeToken(token);
                void loadAll(token);
              }}
            >
              连接
            </Button>
          </div>
        </SectionCard>
      )}

      <SectionCard title="厂商与套餐" description="套餐决定默认 Base URL，可手动改为第三方兼容端点">
        <div className="space-y-3">
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
        </div>
      </SectionCard>

      <SectionCard title="密钥与模型" description="API Key 留空则不改动已存 key；环境变量名优先于明文">
        <div className="space-y-3">
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
              {/* 261126-model-select: a <datalist> is NOT a dropdown -- it
                  only hints while typing and cannot be opened by clicking.
                  "拉到 N 个模型" therefore had no clickable picker at all.
                  Once a list exists, render a real <select>; keep a free-text
                  input for the manual case (the only way to enter a model
                  that the endpoint does not advertise). */}
              {models.length > 0 ? (
                <>
                  <select
                    id="llm-model"
                    className={input}
                    value={model}
                    onChange={(e) => setModel(e.target.value)}
                    data-testid="llm-model-select"
                  >
                    {!models.includes(model) && model.trim() && (
                      <option value={model}>{model}（不在列表内）</option>
                    )}
                    {models.map((m) => (
                      <option key={m} value={m}>
                        {m}
                      </option>
                    ))}
                  </select>
                  <button
                    type="button"
                    className="text-xs text-muted-foreground underline"
                    onClick={() => setModels([])}
                    data-testid="llm-model-manual-toggle"
                  >
                    手动输入模型名
                  </button>
                </>
              ) : (
                <input
                  id="llm-model"
                  className={input}
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  placeholder="glm-5.3-flash"
                  data-testid="llm-model-input"
                />
              )}
            </div>
            <Button type="button" variant="secondary" onClick={() => void onModels()} disabled={busy !== ''}>
              {busy === 'models' ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              拉取模型列表
            </Button>
          </div>

          <div className="flex gap-3">
            <Button
              type="button"
              variant="secondary"
              onClick={() => void onTest()}
              disabled={busy !== '' || !vendor}
            >
              {busy === 'test' ? <Loader2 className="h-4 w-4 animate-spin" /> : <PlugZap className="h-4 w-4" />}
              测试连通
            </Button>
            <Button
              type="button"
              onClick={() => void onSave()}
              disabled={busy !== '' || !vendor || !model}
            >
              {busy === 'save' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              保存并生效
            </Button>
          </div>
        </div>
      </SectionCard>

      {msg && (
        <p
          className={
            msg.kind === 'ok'
              ? 'text-sm text-status-compliant'
              : 'text-sm text-status-non-compliant'
          }
          data-testid="llm-msg"
        >
          {msg.text}
        </p>
      )}
    </div>
  );
}
