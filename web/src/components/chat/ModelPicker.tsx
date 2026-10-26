// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ModelPicker — chat-page LLM model selector (260926 quick plan).
//
// Reads the server-side LLM config (GET /api/admin/llm/config) and the
// model list (GET /api/admin/llm/models, which reuses the server-stored
// API key — the key NEVER reaches the frontend), and switches the active
// model via POST /api/admin/llm/model {model}. The switch hot-reloads
// the backend resolver — no server restart.
//
// Posture: fully optional chrome. 403 (no admin token) / network error /
// stub mode without config → renders a muted non-interactive badge, so
// the composer stays usable for anonymous visitors and tests.

import { useEffect, useState } from 'react';
import { Cpu } from 'lucide-react';

const ADMIN_TOKEN_KEY = 'sf_admin_token';

interface LlmConfigResponse {
  source?: string;
  config?: { model?: string; vendor?: string; plan?: string; base_url?: string };
  mode?: string;
  available?: boolean;
}

function adminHeaders(): HeadersInit {
  const token = localStorage.getItem(ADMIN_TOKEN_KEY) ?? '';
  return { 'Content-Type': 'application/json', 'X-Admin-Token': token };
}

export default function ModelPicker() {
  const [model, setModel] = useState<string>('');
  const [mode, setMode] = useState<string>('');
  const [models, setModels] = useState<string[]>([]);
  const [switching, setSwitching] = useState(false);
  const [error, setError] = useState<string>('');
  const [usable, setUsable] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch('/api/admin/llm/config', { headers: adminHeaders() });
        if (!res.ok) return; // 403 without token — stay muted
        const data: LlmConfigResponse = await res.json();
        if (cancelled) return;
        setModel(data.config?.model ?? '');
        setMode(data.mode ?? '');
        setUsable(true);
        // Model list is best-effort: needs a reachable upstream.
        try {
          const q = new URLSearchParams();
          if (data.config?.vendor) q.set('vendor', data.config.vendor);
          if (data.config?.plan) q.set('plan', data.config.plan);
          if (data.config?.base_url) q.set('base_url', data.config.base_url);
          const mres = await fetch(`/api/admin/llm/models?${q.toString()}`, {
            headers: adminHeaders(),
          });
          if (mres.ok) {
            const mdata = (await mres.json()) as { models?: string[] };
            if (!cancelled && Array.isArray(mdata.models)) setModels(mdata.models);
          }
        } catch {
          /* list unavailable — select falls back to free-text current model */
        }
      } catch {
        /* backend down — stay muted */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function switchModel(next: string) {
    if (!next || switching) return;
    setSwitching(true);
    setError('');
    try {
      const res = await fetch('/api/admin/llm/model', {
        method: 'POST',
        headers: adminHeaders(),
        body: JSON.stringify({ model: next }),
      });
      const data = (await res.json()) as { model?: string; mode?: string; error?: string };
      if (!res.ok) {
        setError(data.error ?? `切换失败 (${res.status})`);
        return;
      }
      setModel(data.model ?? next);
      setMode(data.mode ?? '');
    } catch (e) {
      setError(e instanceof Error ? e.message : '切换失败');
    } finally {
      setSwitching(false);
    }
  }

  if (!usable) {
    return (
      <div
        data-testid="model-picker"
        data-mode="unavailable"
        className="composer-skill-badge"
        style={{ opacity: 0.55, cursor: 'default' }}
        title="LLM 未配置（stub 模式）— 在 admin 后台「模型配置」保存提供商后可在此切换"
      >
        <Cpu className="h-3 w-3" aria-hidden />
        <span>模型：未配置</span>
      </div>
    );
  }

  return (
    <div
      data-testid="model-picker"
      data-mode={mode}
      className="composer-skill-badge"
      style={{ gap: 6 }}
    >
      <Cpu className="h-3 w-3" aria-hidden />
      {models.length > 0 ? (
        <select
          data-testid="model-picker-select"
          value={model}
          disabled={switching}
          onChange={(e) => void switchModel(e.target.value)}
          className="bg-transparent text-inherit outline-none cursor-pointer"
          style={{ fontSize: 'inherit', maxWidth: 220 }}
          aria-label="切换模型"
        >
          {!models.includes(model) && model && <option value={model}>{model}</option>}
          {models.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      ) : (
        <span>{model || (mode === 'stub' ? 'stub' : '…')}</span>
      )}
      <span
        data-testid="model-picker-mode"
        style={{ fontSize: 10, opacity: 0.75 }}
        title={error || undefined}
      >
        {switching ? '切换中…' : error ? '⚠' : mode}
      </span>
    </div>
  );
}
