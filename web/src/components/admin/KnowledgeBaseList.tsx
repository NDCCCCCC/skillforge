// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 26-08 (KBM-02) — Knowledge Base management view.
//
// THE FIRST ADMIN VIEW WITH MUTATION VERBS. KPIDefinitionList is
// deliberately read-only ("files are the sole source of truth — NO edit
// button"); the KB registry is different BY DESIGN: admins create and
// deactivate bases at runtime, and every mutation lands an INSERT-only
// audit row server-side (26-05: action_kb_created / action_kb_deactivated).
// The skeleton (wire interface, token read, loading/error/empty trio, table
// layout) is the KPIDefinitionList shape — the verb surface is the
// divergence, and the divergence is the point (26-PATTERNS).
//
// Wire shapes (26-05, frozen):
//   GET  /api/admin/kb                  → { signature, bases: KBEntry[] }
//   POST /api/admin/kb                  body {slug,display_name,role,description}
//                                       → 201 | 400 {error} | 403 | 409 | 503
//   POST /api/admin/kb/:slug/deactivate → 200 {slug, active:false} | 403 | 404 | 503
//
// TRUST BOUNDARY (T-26-08-1 / T-26-08-3): every request carries
// X-Admin-Token and the SERVER decides. The slug regex and the role
// whitelist below are UX convenience only — this component never gates.
// A 400 renders the backend rule text verbatim, a 409 the duplicate hint,
// a 403 the token hint. There is deliberately NO delete / re-enable
// control: the backend exposes no such endpoint, and a button that 404s
// would be a fake capability.

import { useCallback, useEffect, useState } from 'react';

/** Mirrors internal/kb/slug.go slugPattern (`MaxSlugLen` = 20). */
const KB_SLUG_PATTERN = /^[a-z][a-z0-9-]{0,19}$/;
const KB_SLUG_RULE =
  'slug 需以小写字母开头，仅含小写字母/数字/连字符，长度 1-20（不接受大写、下划线或空格）';

/** The closed four-value semantic role vocabulary (internal/kb Roles()). */
const KB_ROLES = ['authority', 'historical', 'personal', 'draft'] as const;

interface KBEntry {
  slug: string;
  display_name: string;
  role: string;
  description: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

interface KBAdminResponse {
  signature: string;
  bases: KBEntry[];
}

function EmptyState() {
  return (
    <div data-testid="kb-empty" className="flex flex-col items-center justify-center py-12 text-center">
      <h3 className="text-lg font-semibold text-text">暂无知识库</h3>
      <p className="mt-1 text-sm text-text-muted">用下方表单注册第一个知识库（会写入审计日志）</p>
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="flex flex-col gap-3">
      {[1, 2, 3].map((i) => (
        <div key={i} className="flex h-10 items-center gap-4 rounded-md bg-surface" />
      ))}
    </div>
  );
}

export default function KnowledgeBaseList() {
  const [data, setData] = useState<KBAdminResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  // Create-form state.
  const [slug, setSlug] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [role, setRole] = useState<string>('authority');
  const [description, setDescription] = useState('');
  const [slugError, setSlugError] = useState('');
  const [actionError, setActionError] = useState('');
  const [busy, setBusy] = useState(false);

  const loadBases = useCallback(() => {
    const token = localStorage.getItem('sf_admin_token') ?? '';
    return fetch('/api/admin/kb', {
      headers: {
        'X-Admin-Token': token,
      },
    })
      .then((resp) => {
        if (!resp.ok) {
          setError(true);
          setLoading(false);
          return null;
        }
        return resp.json() as Promise<KBAdminResponse>;
      })
      .then((d) => {
        if (d) {
          setData(d);
          setError(false);
        }
        setLoading(false);
      })
      .catch(() => {
        setError(true);
        setLoading(false);
      });
  }, []);

  useEffect(() => {
    void loadBases();
  }, [loadBases]);

  function handleCreate() {
    setSlugError('');
    setActionError('');

    // UX-only pre-flight: the server re-validates (kb.ValidateNewSlug) and
    // its 400 verdict is what we render on a rule rejection.
    if (!KB_SLUG_PATTERN.test(slug)) {
      setSlugError(KB_SLUG_RULE);
      return;
    }
    if (displayName.trim().length === 0) {
      setSlugError('display_name 不能为空');
      return;
    }

    setBusy(true);
    const token = localStorage.getItem('sf_admin_token') ?? '';
    fetch('/api/admin/kb', {
      method: 'POST',
      headers: {
        'X-Admin-Token': token,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ slug, display_name: displayName, role, description }),
    })
      .then(async (resp) => {
        if (resp.ok) {
          setSlug('');
          setDisplayName('');
          setRole('authority');
          setDescription('');
          await loadBases(); // 201 → refresh the list from the server truth
          return;
        }
        const body = (await resp.json().catch(() => ({}))) as { error?: string };
        if (resp.status === 409) {
          setActionError('slug 已存在');
        } else if (resp.status === 403) {
          setActionError('创建失败：缺少或错误的 X-Admin-Token');
        } else if (resp.status === 503) {
          setActionError('创建失败：服务端知识库注册表未启用');
        } else {
          setActionError(body.error ?? `创建失败（HTTP ${resp.status}）`);
        }
      })
      .catch(() => {
        setActionError('创建失败：网络错误');
      })
      .finally(() => setBusy(false));
  }

  function handleDeactivate(slugValue: string) {
    setActionError('');
    // INSERT-only audit trail on the server side (26-05) — ask once.
    if (!window.confirm(`停用知识库 ${slugValue}？该操作会写入审计日志。`)) {
      return;
    }
    setBusy(true);
    const token = localStorage.getItem('sf_admin_token') ?? '';
    fetch(`/api/admin/kb/${encodeURIComponent(slugValue)}/deactivate`, {
      method: 'POST',
      headers: {
        'X-Admin-Token': token,
      },
    })
      .then(async (resp) => {
        if (resp.ok) {
          // Deactivate ≠ delete: the entry stays, flagged inactive.
          setData((prev) =>
            prev
              ? { ...prev, bases: prev.bases.map((b) => (b.slug === slugValue ? { ...b, active: false } : b)) }
              : prev,
          );
          return;
        }
        const body = (await resp.json().catch(() => ({}))) as { error?: string };
        if (resp.status === 403) {
          setActionError('停用失败：缺少或错误的 X-Admin-Token');
        } else if (resp.status === 503) {
          setActionError('停用失败：服务端知识库注册表未启用');
        } else {
          setActionError(body.error ?? `停用失败（HTTP ${resp.status}）`);
        }
      })
      .catch(() => {
        setActionError('停用失败：网络错误');
      })
      .finally(() => setBusy(false));
  }

  if (loading) {
    return <LoadingSkeleton />;
  }

  return (
    <div data-testid="kb-view" className="flex flex-col gap-6">
      {error && (
        <div data-testid="kb-error" className="flex items-center gap-2 text-sm text-danger">
          <span>知识库列表加载失败</span>
        </div>
      )}

      {!error && data && data.bases.length === 0 && <EmptyState />}

      {data && data.bases.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-border text-xs text-text-muted">
                <th scope="col" className="pb-2 text-left font-semibold">Slug</th>
                <th scope="col" className="pb-2 text-left font-semibold">显示名</th>
                <th scope="col" className="pb-2 text-left font-semibold">角色</th>
                <th scope="col" className="pb-2 text-left font-semibold">描述</th>
                <th scope="col" className="pb-2 text-left font-semibold">状态</th>
                <th scope="col" className="pb-2 text-right font-semibold">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {data.bases.map((base) => (
                <tr key={base.slug} data-testid={`kb-row-${base.slug}`} className="hover:bg-surface transition-colors">
                  <td className="py-3 font-medium text-text">
                    <code className="font-mono text-xs">{base.slug}</code>
                  </td>
                  <td className="py-3 text-sm text-text">{base.display_name}</td>
                  <td className="py-3">
                    <code className="font-mono text-xs text-text-muted">{base.role}</code>
                  </td>
                  <td className="py-3 text-sm text-text-muted">{base.description || '—'}</td>
                  <td className="py-3 text-sm">
                    {base.active ? (
                      <span className="text-text-muted">active</span>
                    ) : (
                      <span
                        data-testid="kb-deactivated"
                        className="rounded-md bg-surface-2 px-2 py-0.5 text-xs font-semibold text-text-muted"
                      >
                        停用
                      </span>
                    )}
                  </td>
                  <td className="py-3 text-right">
                    {base.active ? (
                      <button
                        type="button"
                        data-testid={`kb-deactivate-${base.slug}`}
                        disabled={busy}
                        onClick={() => handleDeactivate(base.slug)}
                        className="rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold text-text hover:bg-surface-3 disabled:opacity-50"
                      >
                        停用
                      </button>
                    ) : (
                      <span className="text-xs text-text-muted">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {actionError && (
        <div
          data-testid="kb-action-error"
          role="alert"
          className="rounded-md border border-border bg-surface-2 px-3 py-2 text-sm text-danger"
        >
          {actionError}
        </div>
      )}

      <form
        data-testid="kb-create-form"
        className="flex flex-col gap-3 rounded-md border border-border bg-surface p-4"
        onSubmit={(e) => {
          e.preventDefault();
          handleCreate();
        }}
      >
        <h3 className="text-sm font-semibold text-text">注册新知识库</h3>
        <p className="text-xs text-text-muted">{KB_SLUG_RULE}</p>
        <div className="flex flex-wrap items-end gap-3">
          <label className="flex flex-col gap-1 text-xs text-text-muted">
            Slug
            <input
              data-testid="kb-slug-input"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              placeholder="kb-2026"
              className="w-48 rounded-md border border-border bg-surface-2 px-2 py-1 font-mono text-xs text-text"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-text-muted">
            显示名
            <input
              data-testid="kb-display-name-input"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="新库"
              className="w-48 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs text-text"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-text-muted">
            角色
            <select
              data-testid="kb-role-select"
              value={role}
              onChange={(e) => setRole(e.target.value)}
              className="rounded-md border border-border bg-surface-2 px-2 py-1 text-xs text-text"
            >
              {KB_ROLES.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs text-text-muted">
            描述（可选）
            <input
              data-testid="kb-description-input"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="w-64 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs text-text"
            />
          </label>
          <button
            type="submit"
            data-testid="kb-submit"
            disabled={busy}
            className="rounded-md bg-primary-dim px-3 py-1.5 text-xs font-semibold text-primary hover:bg-surface-3 disabled:opacity-50"
          >
            创建
          </button>
        </div>
        {slugError && (
          <div data-testid="kb-slug-error" role="alert" className="text-xs text-danger">
            {slugError}
          </div>
        )}
      </form>
    </div>
  );
}
