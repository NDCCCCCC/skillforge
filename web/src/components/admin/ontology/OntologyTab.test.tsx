// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 38-05 — ontology tab tests. Pins:
//   - the 8th admin tab (本体) activates and mounts OntologyTab
//   - TBoxBrowser renders the 6 class cards from the TBox/KG endpoints
//   - KGQueryPanel triggers the pattern query on run
//   - ProposalDrawer opens from a row click and wires approve/reject

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import AdminDashboard from '../AdminDashboard';
import OntologyTab from './OntologyTab';
import ProposalDrawer from './ProposalDrawer';
import { useAppStore } from '../../../store/app-store';
import type { Proposal } from './types';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

const NS = 'http://skillforge.local/ontology#';
const RDF_TYPE = 'http://www.w3.org/1999/02/22-rdf-syntax-ns#type';

const TTL = `@prefix skf:  <${NS}> .
skf:Skill a owl:Class .
skf:skill_a a skf:Skill ;
    skf:skillName "a" .
`;

function stubFetch() {
  vi.stubGlobal(
    'fetch',
    vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? 'GET';
      if (url.startsWith('/api/admin/ontology/tbox')) {
        return Promise.resolve(jsonResponse({ version: 1, triple_count: 3, ttl: TTL }));
      }
      if (url.startsWith('/api/admin/ontology/kg/stats')) {
        return Promise.resolve(
          jsonResponse({ triple_count: 3, byS_size: 2, byP_size: 2, byO_size: 3, memory_kb_estimate: 1 }),
        );
      }
      if (url.startsWith('/api/admin/ontology/kg/query')) {
        const u = new URL(url, 'http://x');
        if (u.searchParams.get('o') === `${NS}Skill` && !u.searchParams.get('s')) {
          return Promise.resolve(
            jsonResponse({
              matches: [
                { s: `${NS}skill_a`, p: RDF_TYPE, o: `${NS}Skill` },
                { s: `${NS}skill_b`, p: RDF_TYPE, o: `${NS}Skill` },
              ],
            }),
          );
        }
        return Promise.resolve(jsonResponse({ matches: [] }));
      }
      if (url === '/api/admin/ontology/proposals' && method === 'GET') {
        return Promise.resolve(jsonResponse({ proposals: [makeProposal()] }));
      }
      if (url.startsWith('/api/admin/ontology/proposals/') && url.endsWith('/approve')) {
        return Promise.resolve(
          jsonResponse({ proposal: { ...makeProposal(), status: 'approved' }, tbox_version: 2 }),
        );
      }
      if (url.startsWith('/api/admin/ontology/proposals/') && url.endsWith('/reject')) {
        return Promise.resolve(
          jsonResponse({ proposal: { ...makeProposal(), status: 'rejected' } }),
        );
      }
      // CapacityMonitor etc. degrade on !ok.
      return Promise.resolve(jsonResponse({ error: 'not stubbed' }, 404));
    }),
  );
}

function makeProposal(): Proposal {
  return {
    id: 'prop-0001',
    kind: 'Class',
    description_zh: '业务规则',
    description_en: 'business rule',
    local_name: 'BusinessRule',
    proposed_iri: `${NS}BusinessRule`,
    proposed_triples: [`<${NS}BusinessRule> a owl:Class ;`, `    rdfs:label "业务规则"@zh , "business rule"@en .`],
    status: 'pending',
    source_text: 'compliance notes reference business rules',
    created_at: '2026-09-21T00:00:00Z',
  };
}

beforeEach(() => {
  localStorage.setItem('sf_admin_token', 'tok');
  stubFetch();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('AdminDashboard — ontology tab', () => {
  it('renders 11 subroute tabs and activates the 本体 tab', async () => {
    useAppStore.setState({ adminSubroute: 'batches' });
    render(<AdminDashboard />);
    const tabs = screen.getByTestId('admin-subroute-tabs');
    expect(tabs.querySelectorAll('button')).toHaveLength(11);
    fireEvent.click(screen.getByTestId('admin-tab-ontology'));
    expect(await screen.findByTestId('ontology-tab')).toBeInTheDocument();
  });
});

describe('OntologyTab — TBoxBrowser', () => {
  it('renders the 6 class cards and lists Skill instances', async () => {
    render(<OntologyTab />);
    // 6 cards
    for (const c of ['Tier', 'KBCategory', 'SubCategory', 'Skill', 'SkillAgent', 'RefusalPhrase']) {
      expect(screen.getByTestId(`tbox-class-${c}`)).toBeInTheDocument();
    }
    // Default selected class = Skill → instances from KG query.
    await waitFor(() => {
      expect(screen.getByTestId('tbox-instances').textContent).toContain('skill_a');
      expect(screen.getByTestId('tbox-instances').textContent).toContain('skill_b');
    });
  });
});

describe('OntologyTab — TBoxBrowser dynamic classes', () => {
  const OWL_CLASS = 'http://www.w3.org/2002/07/owl#Class';

  it('appends fallback cards for owl:Class individuals beyond the static metadata', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = init?.method ?? 'GET';
        if (url.startsWith('/api/admin/ontology/tbox')) {
          return Promise.resolve(jsonResponse({ version: 1, triple_count: 3, ttl: TTL }));
        }
        if (url.startsWith('/api/admin/ontology/kg/stats')) {
          return Promise.resolve(
            jsonResponse({ triple_count: 3, byS_size: 2, byP_size: 2, byO_size: 3, memory_kb_estimate: 1 }),
          );
        }
        if (url.startsWith('/api/admin/ontology/kg/query')) {
          const u = new URL(url, 'http://x');
          // TBox v7 class discovery: 10 owl:Class individuals, 4 of them
          // absent from the static Phase 38 card set.
          if (u.searchParams.get('o') === OWL_CLASS) {
            const classes = [
              'Tier', 'KBCategory', 'SubCategory', 'Skill', 'SkillAgent', 'RefusalPhrase',
              'InsuranceProduct', 'Organization', 'NoteChunk', 'TableSchema',
            ];
            return Promise.resolve(
              jsonResponse({
                matches: classes.map((c) => ({ s: `${NS}${c}`, p: RDF_TYPE, o: OWL_CLASS })),
              }),
            );
          }
          if (u.searchParams.get('o') === `${NS}Skill` && !u.searchParams.get('s')) {
            return Promise.resolve(jsonResponse({ matches: [] }));
          }
          return Promise.resolve(jsonResponse({ matches: [] }));
        }
        if (url === '/api/admin/ontology/proposals' && method === 'GET') {
          return Promise.resolve(jsonResponse({ proposals: [] }));
        }
        return Promise.resolve(jsonResponse({ error: 'not stubbed' }, 404));
      }),
    );
    render(<OntologyTab />);
    // Dynamic fallback cards appear with the local name as the label.
    expect(await screen.findByTestId('tbox-class-InsuranceProduct')).toBeInTheDocument();
    expect(screen.getByTestId('tbox-class-Organization')).toBeInTheDocument();
    expect(screen.getByTestId('tbox-class-NoteChunk')).toBeInTheDocument();
    expect(screen.getByTestId('tbox-class-TableSchema')).toBeInTheDocument();
    // Static cards keep their testids and curated positions.
    expect(screen.getByTestId('tbox-class-Skill')).toBeInTheDocument();
    // Discovered extras sort after every static card.
    const grid = screen.getByTestId('tbox-class-Tier').parentElement as HTMLElement;
    const locals = Array.from(grid.querySelectorAll('button')).map((b) =>
      b.getAttribute('data-testid')?.replace('tbox-class-', ''),
    );
    for (const extra of ['InsuranceProduct', 'NoteChunk', 'Organization', 'TableSchema']) {
      expect(locals.indexOf(extra)).toBeGreaterThan(locals.indexOf('RefusalPhrase'));
    }
    expect(locals).toHaveLength(10);
  });
});

describe('OntologyTab — KGQueryPanel', () => {
  it('triggers the pattern query on run and renders matches', async () => {
    render(<OntologyTab />);
    fireEvent.click(screen.getByTestId('ontology-view-kg'));
    fireEvent.click(screen.getByTestId('kg-query-run'));
    await waitFor(() => {
      expect(screen.getByTestId('kg-query-results').textContent).toContain('skill_a');
    });
  });
});

describe('ProposalDrawer', () => {
  it('renders diff + reasoning and wires approve', async () => {
    const onDecided = vi.fn();
    const onClose = vi.fn();
    render(
      <ProposalDrawer proposal={makeProposal()} onClose={onClose} onDecided={onDecided} />,
    );
    expect(screen.getByTestId('proposal-drawer')).toBeInTheDocument();
    expect(screen.getByTestId('proposal-diff').textContent).toContain('BusinessRule');
    expect(screen.getByTestId('proposal-reasoning').textContent).toContain('business rule');
    fireEvent.click(screen.getByTestId('proposal-approve'));
    await waitFor(() => expect(onDecided).toHaveBeenCalled());
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('reject path calls the reject endpoint', async () => {
    const onDecided = vi.fn();
    const onClose = vi.fn();
    render(
      <ProposalDrawer proposal={makeProposal()} onClose={onClose} onDecided={onDecided} />,
    );
    fireEvent.click(screen.getByTestId('proposal-reject'));
    await waitFor(() => expect(onDecided).toHaveBeenCalled());
  });

  it('renders nothing for a null proposal', () => {
    const { container } = render(
      <ProposalDrawer proposal={null} onClose={() => {}} onDecided={() => {}} />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});

describe('OntologyTab — SchemaProposalsTab', () => {
  it('lists proposals and opens the drawer on row click', async () => {
    render(<OntologyTab />);
    fireEvent.click(screen.getByTestId('ontology-view-proposals'));
    const row = await screen.findByTestId('proposal-row-prop-0001');
    fireEvent.click(row);
    expect(await screen.findByTestId('proposal-drawer')).toBeInTheDocument();
  });
});
