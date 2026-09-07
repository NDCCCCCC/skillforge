// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DocViewer — center-panel corpus-doc viewer. Renders the full markdown
// source of one knowledge/<tier>/<file> fetched from GET
// /api/knowledge/doc?tier=&file=. Entry points: the left-sidebar FILES
// tree click and the knowledge-graph doc-node click.
//
// Rendering strategy mirrors SkillMdPreview (structured, no markdown
// library): the YAML frontmatter (if present) renders as a metadata
// block (version / effective dates — the same fields the graph browser
// badges), and the body renders as pre-wrap text inside a mono-font
// surface. Clause fidelity matters more than typography here — the
// operator reads policy text exactly as indexed.
import { useEffect, useState } from 'react';
import { FileText } from 'lucide-react';
import { useTierLabel } from '../../lib/kb-bases';
import type { Tier } from '../../types/verdict';

interface DocPayload {
  tier: string;
  file: string;
  content: string;
}

/** Minimal frontmatter split: returns [metaBlock, body] (meta '' when absent). */
function splitFrontmatter(content: string): [string, string] {
  if (!content.startsWith('---\n')) return ['', content];
  const end = content.indexOf('\n---\n', 4);
  if (end < 0) return ['', content];
  return [content.slice(4, end), content.slice(end + 5)];
}

export interface DocViewerProps {
  tier: Tier;
  file: string;
}

export default function DocViewer({ tier, file }: DocViewerProps) {
  const [doc, setDoc] = useState<DocPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  // The header badge shows the registry display label (KNOW-05 bracketed
  // badge form) — the raw slug only stays visible in the loading line,
  // where it names the request path being fetched.
  const tierLabel = useTierLabel(doc?.tier ?? tier);

  useEffect(() => {
    const ctl = new AbortController();
    setDoc(null);
    setError(null);
    fetch(`/api/knowledge/doc?tier=${encodeURIComponent(tier)}&file=${encodeURIComponent(file)}`, {
      signal: ctl.signal,
    })
      .then(async (r) => {
        if (!r.ok) {
          const body = (await r.json().catch(() => ({}))) as { error?: string };
          throw new Error(body.error ?? `HTTP ${r.status}`);
        }
        return r.json() as Promise<DocPayload>;
      })
      .then(setDoc)
      .catch((e: unknown) => {
        if ((e as Error).name !== 'AbortError') setError(String((e as Error).message ?? e));
      });
    return () => ctl.abort();
  }, [tier, file]);

  if (error) {
    return (
      <div data-testid="doc-viewer-error" className="rounded-md border border-danger/40 bg-surface-2 p-3 text-xs text-danger">
        无法加载文档：{error}
      </div>
    );
  }
  if (!doc) {
    return (
      <div data-testid="doc-viewer-loading" className="text-xs text-text-muted">
        正在加载 {tier}/{file} …
      </div>
    );
  }

  const [meta, body] = splitFrontmatter(doc.content);

  return (
    <div data-testid="doc-viewer" className="flex flex-col gap-3">
      <div className="flex items-center gap-2">
        <FileText className="h-4 w-4 shrink-0 text-primary" />
        <h3 className="text-lg font-semibold text-text">{doc.file}</h3>
        <span
          data-testid="doc-viewer-tier"
          className="rounded bg-primary-dim px-1.5 py-0.5 text-xs text-primary"
        >
          {tierLabel}
        </span>
      </div>

      {meta && (
        <pre
          data-testid="doc-viewer-frontmatter"
          className="whitespace-pre-wrap rounded-md border border-border bg-surface-2 p-3 font-mono text-xs text-text-muted"
        >
          {meta}
        </pre>
      )}

      <pre
        data-testid="doc-viewer-body"
        className="whitespace-pre-wrap rounded-md border border-border bg-surface p-4 font-mono text-sm leading-relaxed text-text"
      >
        {body.trim()}
      </pre>
    </div>
  );
}
