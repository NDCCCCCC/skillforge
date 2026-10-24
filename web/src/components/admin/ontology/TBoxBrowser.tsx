// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 38-05 — TBox class-hierarchy browser (SC-4). Class cards seed
// from the static Phase 38 metadata (Tier / KBCategory / SubCategory /
// Skill / SkillAgent / RefusalPhrase) and extend at mount time with
// every owl:Class discovered live from the KG (?x rdf:type owl:Class) —
// TBox v7 grew past the original six (InsuranceProduct / Organization /
// NoteChunk / TableSchema), and static-only cards would hide new
// classes on every TBox revision. Static cards keep their curated
// labels; discovered extras render the local name as the label and sort
// to the tail. Clicking a card lists its instances live from the KG
// pattern query (?x rdf:type skf:<Class>). Instances render the
// local-name tail so CJK sub-category IRIs stay readable.
//
// 2610 redesign: the TBox metadata one-shot load migrated to the shared
// useAdminData hook (admin token handled centrally); header + KPIs
// migrated to the shared PageHeader / StatCard primitives.

import { useEffect, useState } from 'react';
import { cn } from '../../../lib/cn';
import { useAdminData } from '../shared/useAdminData';
import { PageHeader, StatCard } from '../shared/Primitives';
import { fetchKGQuery } from './ontologyApi';
import type { TBoxResponse } from './types';

const RDF_TYPE = 'http://www.w3.org/1999/02/22-rdf-syntax-ns#type';

/** Class tier of the TBox hierarchy (must mirror the emitter's owl:Class). */
const OWL_CLASS = 'http://www.w3.org/2002/07/owl#Class';

/** Default namespace (must mirror emitter.DefaultNS). */
const DEFAULT_NS = 'http://skillforge.local/ontology#';

/** The Phase 38 static TBox classes (ONT-02) — curated Chinese labels. */
const TBOX_CLASSES: Array<{ local: string; label: string }> = [
  { local: 'Tier', label: '信任层级' },
  { local: 'KBCategory', label: '知识类目' },
  { local: 'SubCategory', label: '子类目' },
  { local: 'Skill', label: 'Skill' },
  { local: 'SkillAgent', label: 'Agent 类型' },
  { local: 'RefusalPhrase', label: '拒答短语' },
];

/** Local-name tail of any IRI (wildcard: split at the last # or /). */
function iriLocalName(iri: string): string {
  const cut = Math.max(iri.lastIndexOf('#'), iri.lastIndexOf('/'));
  return cut >= 0 ? iri.slice(cut + 1) : iri;
}

/** Strip the namespace prefix for readable instance labels. */
function localTail(iri: string, ns: string): string {
  if (iri.startsWith(ns)) return iri.slice(ns.length);
  const hash = iri.lastIndexOf('#');
  const slash = iri.lastIndexOf('/');
  const cut = Math.max(hash, slash);
  return cut >= 0 ? iri.slice(cut + 1) : iri;
}

export default function TBoxBrowser() {
  const { data: tbox } = useAdminData<TBoxResponse>('/api/admin/ontology/tbox');
  const [ns, setNS] = useState(DEFAULT_NS);
  const [classes, setClasses] = useState<Array<{ local: string; label: string }>>(TBOX_CLASSES);
  const [selected, setSelected] = useState<string>('Skill');
  const [instances, setInstances] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Discover the live namespace from the TBox @prefix line (the env may
  // override SKILLFORGE_ONTOLOGY_NS; hardcoding would show 0 instances).
  useEffect(() => {
    if (!tbox) return;
    const m = tbox.ttl.match(/@prefix\s+skf:\s+<([^>]+)>/);
    if (m) setNS(m[1]);
  }, [tbox]);

  // Discover classes beyond the static Phase 38 metadata: every
  // owl:Class individual in the KG that TBOX_CLASSES does not already
  // cover becomes a fallback card (local name as label), sorted to the
  // tail. Best-effort — on failure the static card set stays.
  useEffect(() => {
    let cancelled = false;
    fetchKGQuery('', RDF_TYPE, OWL_CLASS)
      .then((r) => {
        if (cancelled) return;
        const seen = new Set(TBOX_CLASSES.map((c) => c.local));
        const extra: Array<{ local: string; label: string }> = [];
        for (const t of r.matches) {
          const local = iriLocalName(t.s);
          if (!local || seen.has(local)) continue;
          seen.add(local);
          extra.push({ local, label: local });
        }
        if (extra.length === 0) return;
        extra.sort((a, b) => a.local.localeCompare(b.local));
        setClasses([...TBOX_CLASSES, ...extra]);
      })
      .catch(() => {
        // class discovery is an enhancement, never a render blocker
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Fetch instances for the selected class whenever ns/selected change.
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetchKGQuery('', RDF_TYPE, `${ns}${selected}`)
      .then((r) => {
        if (cancelled) return;
        const names = r.matches
          .map((t) => localTail(t.s, ns))
          .filter((n) => n.length > 0)
          .sort();
        setInstances(names);
        setError(null);
      })
      .catch((e) => !cancelled && setError(String(e)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [ns, selected]);

  return (
    <section data-testid="tbox-browser" className="space-y-3">
      <PageHeader
        title="TBox 类层次"
        description="点击类卡片查看 KG 中的实例（?x a skf:<Class>）"
      />

      <div className="grid grid-cols-2 gap-2 md:grid-cols-3 lg:grid-cols-6">
        <StatCard label="TBox 版本" value={tbox ? `v${tbox.version}` : '—'} />
        <StatCard label="三元组" value={tbox ? tbox.triple_count : '—'} />
      </div>

      <div className="grid grid-cols-2 gap-2 md:grid-cols-3 lg:grid-cols-6">
        {classes.map((c) => (
          <button
            key={c.local}
            type="button"
            data-testid={`tbox-class-${c.local}`}
            onClick={() => setSelected(c.local)}
            className={cn(
              'rounded-md border px-3 py-2 text-left transition-colors',
              selected === c.local
                ? 'border-primary bg-primary-dim text-primary'
                : 'border-border bg-surface-2 text-text hover:bg-surface-3',
            )}
          >
            <div className="font-mono text-xs">{c.local}</div>
            <div className="text-[11px] text-text-muted">{c.label}</div>
          </button>
        ))}
      </div>

      <div className="rounded-md border border-border bg-surface-2 p-3">
        <div className="mb-2 font-mono text-xs text-text-muted">
          ?x a skf:{selected}
        </div>
        {error && <div className="text-xs text-danger">加载失败: {error}</div>}
        {loading && <div className="text-xs text-text-muted">加载中…</div>}
        {!loading && !error && (
          <ul data-testid="tbox-instances" className="flex flex-wrap gap-1.5">
            {instances.map((name) => (
              <li
                key={name}
                className="rounded bg-surface-3 px-2 py-0.5 font-mono text-[11px]"
                title={`${ns}${name}`}
              >
                {name}
              </li>
            ))}
            {instances.length === 0 && (
              <li className="text-xs text-text-muted">无实例</li>
            )}
          </ul>
        )}
      </div>
    </section>
  );
}
