// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Admin document upload panel — POST /api/admin/kb/:slug/upload
// (multipart/form-data, X-Admin-Token).
//
// 2610 redesign: migrated to the shared admin component library — the
// upload request now goes through adminFetchResponse (token injected by
// the shared helper, no hand-rolled X-Admin-Token header) and the panel
// buttons use the ui/button Button component. State machine, wire
// contract and every data-testid are unchanged:
//   idle → uploading → parsing → indexing → done | error

import { useEffect, useState } from 'react';
import { ChevronDown, ChevronUp, FileText, AlertCircle, CheckCircle2, Loader2 } from 'lucide-react';
import { fetchKnowledgeBases } from '../../lib/kb-bases';
import { adminFetchResponse } from './shared/useAdminData';
import { Button } from '../ui/button';

type UploadState = 'idle' | 'uploading' | 'parsing' | 'indexing' | 'done' | 'error';

export default function DocUpload() {
  const [state, setState] = useState<UploadState>('idle');
  const [bases, setBases] = useState<Array<{ slug: string; display_name: string }>>([]);
  const [selectedSlug, setSelectedSlug] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState('');
  const [chunks, setChunks] = useState<number | null>(null);
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    fetchKnowledgeBases()
      .then((bs) => setBases(bs.map((b) => ({ slug: b.slug, display_name: b.display_name }))))
      .catch(() => {}); // non-critical — KB dropdown just stays empty
  }, []);

  async function handleSubmit() {
    if (!selectedSlug || !file) return;
    setError('');
    setState('uploading');

    const formData = new FormData();
    formData.append('file', file);

    try {
      // Brief pause so user sees the parsing/indexing states too.
      await new Promise((r) => setTimeout(r, 200));
      setState('parsing');
      await new Promise((r) => setTimeout(r, 200));
      setState('indexing');

      const resp = await adminFetchResponse(
        `/api/admin/kb/${encodeURIComponent(selectedSlug)}/upload`,
        { method: 'POST', body: formData },
      );

      if (resp.ok) {
        const data = (await resp.json()) as { chunks: number };
        setChunks(data.chunks);
        setState('done');
      } else {
        const body = (await resp.json().catch(() => ({}))) as { error?: string };
        setError(body.error ?? `上传失败（HTTP ${resp.status}）`);
        setState('error');
      }
    } catch {
      setError('网络错误');
      setState('error');
    }
  }

  function reset() {
    setState('idle');
    setFile(null);
    setError('');
    setChunks(null);
  }

  if (!expanded) {
    return (
      <div className="mt-6 border-t border-border pt-4">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          data-testid="doc-upload-expand"
          onClick={() => setExpanded(true)}
        >
          <ChevronDown className="h-4 w-4" />
          上传文档到知识库
        </Button>
      </div>
    );
  }

  return (
    <div className="mt-6 border-t border-border pt-4" data-testid="doc-upload-panel">
      <div className="flex items-center justify-between">
        <h3 className="flex items-center gap-2 text-sm font-semibold text-text">
          <FileText className="h-4 w-4 text-text-muted" />
          上传文档到知识库
        </h3>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          data-testid="doc-upload-collapse"
          onClick={() => setExpanded(false)}
          aria-label="收起上传面板"
        >
          <ChevronUp className="h-4 w-4" />
        </Button>
      </div>
      <p className="mt-1 text-xs text-text-dim">
        支持 .md / .pdf / .docx / .xlsx / .pptx（最大 100 MB，默认未配置 SKILLFORGE_UPLOAD_MAX_BYTES）。
      </p>

      <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-[1fr_2fr_auto]">
        <select
          data-testid="doc-upload-slug"
          value={selectedSlug}
          onChange={(e) => setSelectedSlug(e.target.value)}
          disabled={state === 'uploading' || state === 'parsing' || state === 'indexing'}
          className="rounded-md border border-border bg-surface px-2 py-1 text-sm text-text"
        >
          <option value="">选择知识库…</option>
          {bases.map((b) => (
            <option key={b.slug} value={b.slug}>{b.display_name} ({b.slug})</option>
          ))}
        </select>

        <input
          data-testid="doc-upload-file"
          type="file"
          accept=".md,.pdf,.docx,.xlsx,.pptx"
          onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          disabled={state === 'uploading' || state === 'parsing' || state === 'indexing'}
          className="rounded-md border border-border bg-surface px-2 py-1 text-sm text-text file:mr-2 file:rounded file:border-0 file:bg-primary-dim file:px-2 file:py-1 file:text-primary"
        />

        <Button
          type="button"
          data-testid="doc-upload-submit"
          onClick={handleSubmit}
          disabled={!selectedSlug || !file || state === 'uploading' || state === 'parsing' || state === 'indexing'}
        >
          上传
        </Button>
      </div>

      <div className="mt-3 min-h-[1.5rem] text-sm">
        {state === 'uploading' && (
          <div className="flex items-center gap-2 text-text-muted" data-testid="doc-upload-uploading">
            <Loader2 className="h-4 w-4 animate-spin" /> 正在上传…
          </div>
        )}
        {state === 'parsing' && (
          <div className="flex items-center gap-2 text-text-muted" data-testid="doc-upload-parsing">
            <Loader2 className="h-4 w-4 animate-spin" /> 正在解析…
          </div>
        )}
        {state === 'indexing' && (
          <div className="flex items-center gap-2 text-text-muted" data-testid="doc-upload-indexing">
            <Loader2 className="h-4 w-4 animate-spin" /> 正在索引入库…
          </div>
        )}
        {state === 'done' && (
          <div className="flex items-center gap-2 text-success" data-testid="doc-upload-done">
            <CheckCircle2 className="h-4 w-4" /> {chunks} 个片段已入库
            <button type="button" onClick={reset} className="ml-3 text-xs text-text-muted">
              再传一份
            </button>
          </div>
        )}
        {state === 'error' && (
          <div className="flex items-start gap-2 text-danger" data-testid="doc-upload-error">
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
            <span>{error}</span>
            <button type="button" onClick={reset} className="ml-auto text-xs text-text-muted">
              重试
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
