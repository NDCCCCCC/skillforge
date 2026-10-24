// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 26-08 (KBM-02) — Knowledge Base management view.
//
// THE FIRST ADMIN VIEW WITH MUTATION VERBS. KPIDefinitionList is
// deliberately read-only ("files are the sole source of truth — NO edit
// button"); the KB registry is different BY DESIGN: admins create and
// deactivate bases at runtime, and every mutation lands an INSERT-only
// audit row server-side (26-05: action_kb_created / action_kb_deactivated).
//
// 2610 redesign: migrated to the shared admin component library —
// useAdminData / adminFetchResponse (token + three-state), DataTable,
// StatusBadge, PageHeader, SectionCard, ui/button Button. The wire
// interface, response types and every data-testid are unchanged.
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

import { useState } from 'react';
import { adminFetchResponse, useAdminData } from './shared/useAdminData';
import DataTable from './shared/DataTable';
import StatusBadge from './shared/StatusBadge';
import { EmptyState, ErrorState, LoadingSkeleton } from './shared/States';
import { PageHeader, SectionCard } from './shared/Primitives';
import { Button } from '../ui/button';

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

export default function KnowledgeBaseList() {
  const { data, loading, error, reload } = useAdminData<KBAdminResponse>('/api/admin/kb');

  // Deactivate ≠ delete: the entry stays, flagged inactive. Applied as a
  // local overlay over the server truth so the row flips immediately.
  const [deactivatedSlugs, setDeactivatedSlugs] = useState<ReadonlySet<string>>(new Set());
  const bases = (data?.bases ?? []).map((b) =>
    deactivatedSlugs.has(b.slug) ? { ...b, active: false } : b,
  );

  // Create-form state.
  const [slug, setSlug] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [role, setRole] = useState<string>('authority');
  const [description, setDescription] = useState('');
  const [slugError, setSlugError] = useState('');
  const [actionError, setActionError] = useState('');
  const [busy, setBusy] = useState(false);

  async function handleCreate() {
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
    try {
      const resp = await adminFetchResponse('/api/admin/kb', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ slug, display_name: displayName, role, description }),
      });
      if (resp.ok) {
        setSlug('');
        setDisplayName('');
        setRole('authority');
        setDescription('');
        reload(); // 201 → refresh the list from the server truth
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
    } catch {
      setActionError('创建失败：网络错误');
    } finally {
      setBusy(false);
    }
  }

  async function handleDeactivate(slugValue: string) {
    setActionError('');
    // INSERT-only audit trail on the server side (26-05) — ask once.
    if (!window.confirm(`停用知识库 ${slugValue}？该操作会写入审计日志。`)) {
      return;
    }
    setBusy(true);
    try {
      const resp = await adminFetchResponse(
        `/api/admin/kb/${encodeURIComponent(slugValue)}/deactivate`,
        { method: 'POST' },
      );
      if (resp.ok) {
        setDeactivatedSlugs((prev) => new Set(prev).add(slugValue));
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
    } catch {
      setActionError('停用失败：网络错误');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div data-testid="kb-view" className="flex flex-col gap-6">
      <PageHeader
        title="知识库管理"
        description="注册与停用知识库；每次变更都会写入服务端审计日志"
      />

      {loading ? (
        <LoadingSkeleton />
      ) : error ? (
        <ErrorState message="知识库列表加载失败" onRetry={reload} testId="kb-error" />
      ) : bases.length === 0 ? (
        <EmptyState
          title="暂无知识库"
          hint="用下方表单注册第一个知识库（会写入审计日志）"
          testId="kb-empty"
        />
      ) : (
        <DataTable<KBEntry>
          columns={[
            {
              key: 'slug',
              label: 'Slug',
              className: 'font-medium text-text',
              render: (base) => <code className="font-mono text-xs">{base.slug}</code>,
            },
            {
              key: 'display-name',
              label: '显示名',
              className: 'text-sm text-text',
              render: (base) => base.display_name,
            },
            {
              key: 'role',
              label: '角色',
              render: (base) => (
                <code className="font-mono text-xs text-text-muted">{base.role}</code>
              ),
            },
            {
              key: 'description',
              label: '描述',
              className: 'text-sm text-text-muted',
              render: (base) => base.description || '—',
            },
            {
              key: 'status',
              label: '状态',
              className: 'text-sm',
              render: (base) =>
                base.active ? (
                  <StatusBadge status="active" />
                ) : (
                  <StatusBadge status="inactive" testId="kb-deactivated" />
                ),
            },
            {
              key: 'actions',
              label: '操作',
              headClassName: 'text-right',
              className: 'text-right',
              render: (base) =>
                base.active ? (
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    data-testid={`kb-deactivate-${base.slug}`}
                    disabled={busy}
                    onClick={() => handleDeactivate(base.slug)}
                  >
                    停用
                  </Button>
                ) : (
                  <span className="text-xs text-text-muted">—</span>
                ),
            },
          ]}
          rows={bases}
          rowKey={(base) => base.slug}
          rowTestId={(base) => `kb-row-${base.slug}`}
          empty={{ title: '暂无知识库', hint: '用下方表单注册第一个知识库（会写入审计日志）' }}
          emptyTestid="kb-empty"
        />
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
        onSubmit={(e) => {
          e.preventDefault();
          handleCreate();
        }}
      >
        <SectionCard title="注册新知识库" description={KB_SLUG_RULE}>
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
            <Button type="submit" data-testid="kb-submit" disabled={busy}>
              创建
            </Button>
          </div>
          {slugError && (
            <div data-testid="kb-slug-error" role="alert" className="mt-3 text-xs text-danger">
              {slugError}
            </div>
          )}
        </SectionCard>
      </form>
    </div>
  );
}
