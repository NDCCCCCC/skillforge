// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 38-05 — TBox class-hierarchy browser (SC-4). Six class cards
// (Tier / KBCategory / SubCategory / Skill / SkillAgent / RefusalPhrase);
// clicking a card lists its instances live from the KG pattern query
// (?x rdf:type skf:<Class>). Instances render the local-name tail so
// CJK sub-category IRIs stay readable.

import { useEffect, useState } from 'react';
import { Boxes } from 'lucide-react';
import { cn } from '../../../lib/cn';
import { fetchKGQuery, fetchTBox } from './ontologyApi';

const RDF_TYPE = 'http://www.w3.org/1999/02/22-rdf-syntax-ns#type';

/** Default namespace (must mirror emitter.DefaultNS). */
const DEFAULT_NS = 'http://skillforge.local/ontology#';

/** The six TBox classes (ONT-02). */
const TBOX_CLASSES: Array<{ local: string; label: string }> = [
  { local: 'Tier', label: '信任层级' },
  { local: 'KBCategory', label: '知识类目' },
  { local: 'SubCategory', label: '子类目' },
  { local: 'Skill', label: 'Skill' },
  { local: 'SkillAgent', label: 'Agent 类型' },
  { local: 'RefusalPhrase', label: '拒答短语' },
];

/** Strip the namespace prefix for readable instance labels. */
function localTail(iri: string, ns: string): string {
  if (iri.startsWith(ns)) return iri.slice(ns.length);
  const hash = iri.lastIndexOf('#');
  const slash = iri.lastIndexOf('/');
  const cut = Math.max(hash, slash);
  return cut >= 0 ? iri.slice(cut + 1) : iri;
}

export default function TBoxBrowser() {
  const [ns, setNS] = useState(DEFAULT_NS);
  const [version, setVersion] = useState<number | null>(null);
  const [tripleCount, setTripleCount] = useState<number | null>(null);
  const [selected, setSelected] = useState<string>('Skill');
  const [instances, setInstances] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Discover the live namespace from the TBox @prefix line (the env may
  // override SKILLFORGE_ONTOLOGY_NS; hardcoding would show 0 instances).
  useEffect(() => {
    let cancelled = false;
    fetchTBox()
      .then((t) => {
        if (cancelled) return;
        setVersion(t.version);
        setTripleCount(t.triple_count);
        const m = t.ttl.match(/@prefix\s+skf:\s+<([^>]+)>/);
        if (m) setNS(m[1]);
      })
      .catch((e) => !cancelled && setError(String(e)));
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
      <div className="flex items-center justify-between">
        <h2 className="flex items-center gap-1.5 text-sm font-semibold">
          <Boxes className="h-4 w-4" />
          TBox 类层次
        </h2>
        <div className="font-mono text-[11px] text-text-muted">
          v{version ?? '—'} · {tripleCount ?? '—'} triples
        </div>
      </div>

      <div className="grid grid-cols-2 gap-2 md:grid-cols-3 lg:grid-cols-6">
        {TBOX_CLASSES.map((c) => (
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
