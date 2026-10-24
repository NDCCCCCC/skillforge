// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 39-08 — ECL pipeline tab tests. Pins the 39-UI-SPEC contract:
//   - AdminDashboard renders 9 tabs; ECL 管线 (admin-tab-pipeline) mounts
//     PipelineTab and the URL stays /admin
//   - Poll cadence: 2s while a run is active, re-armed to 10s on done
//     (D-Prog-01) — proven under vi.useFakeTimers
//   - Run flow: confirm-gated POST; 409 renders the locked conflict toast
//   - Retry: enabled ONLY on dead rows, per-row in-flight disable
//   - 403 / 503 render their locked inline copy; polling survives errors

import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import AdminDashboard from '../AdminDashboard';
import PipelineTab from './PipelineTab';
import SfToast from '../../layout/SfToast';
import { useAppStore } from '../../../store/app-store';
import type { PipelineStatus } from './types';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

// Mutable server fixture — each test arranges what GET /status returns.
let stageNow = 'idle';
let statusError: number | null = null;
let runStatus = 200;
let networkFail = false;
let statusOverrides: Partial<PipelineStatus> = {};

const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
  const url = String(input);
  const method = init?.method ?? 'GET';
  if (networkFail) return Promise.reject(new TypeError('network down'));
  if (url === '/api/admin/pipeline/status' && method === 'GET') {
    if (statusError !== null) {
      return Promise.resolve(jsonResponse({ error: 'gate says no' }, statusError));
    }
    return Promise.resolve(jsonResponse({ ...makeStatus(), ...statusOverrides }));
  }
  if (url === '/api/admin/pipeline/run' && method === 'POST') {
    if (runStatus !== 200) {
      return Promise.resolve(jsonResponse({ error: 'pipeline already running' }, runStatus));
    }
    return Promise.resolve(jsonResponse({ status: 'started' }));
  }
  if (url.startsWith('/api/admin/pipeline/retry/') && method === 'POST') {
    return Promise.resolve(jsonResponse({ status: 'queued', note_id: url.split('/').pop() }));
  }
  // CapacityMonitor / other tabs degrade on !ok.
  return Promise.resolve(jsonResponse({ error: 'not stubbed' }, 404));
});

function makeStatus(): PipelineStatus {
  const running = stageNow !== 'idle' && stageNow !== 'done';
  return {
    stage: stageNow,
    current_file: stageNow === 'idle' ? '' : 'sql-guides/013__sec03_payment_due.md',
    done: running ? 12 : stageNow === 'done' ? 26 : 0,
    total: stageNow === 'idle' ? 0 : 26,
    queue_len: running ? 3 : 0,
    failures: [
      {
        note_id: '007',
        stage: 'extract',
        attempt_count: 3,
        last_error: 'connection refused',
        status: 'dead',
        updated_at: '2026-09-21T00:00:00Z',
      },
      {
        note_id: '008',
        stage: 'kg-load',
        attempt_count: 1,
        last_error: 'boom',
        status: 'pending',
      },
      {
        note_id: '009',
        stage: 'milvus-load',
        attempt_count: 2,
        last_error: 'index timeout',
        status: 'resolved',
      },
    ],
  };
}

function statusCalls(): number {
  return fetchMock.mock.calls.filter((c) => String(c[0]) === '/api/admin/pipeline/status').length;
}

beforeEach(() => {
  localStorage.setItem('sf_admin_token', 'tok');
  stageNow = 'idle';
  statusError = null;
  runStatus = 200;
  networkFail = false;
  statusOverrides = {};
  fetchMock.mockClear();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('AdminDashboard — pipeline tab (the 9th tab)', () => {
  it('renders 11 subroute tabs and activates the ECL 管线 tab; URL stays /admin', async () => {
    useAppStore.setState({ adminSubroute: 'batches' });
    render(<AdminDashboard />);
    const tabs = screen.getByTestId('admin-subroute-tabs');
    expect(tabs.querySelectorAll('button')).toHaveLength(12);
    fireEvent.click(screen.getByTestId('admin-tab-pipeline'));
    expect(await screen.findByTestId('pipeline-tab')).toBeInTheDocument();
    expect(screen.getByTestId('admin-tab-pipeline')).toHaveTextContent('ECL 管线');
    expect(window.location.pathname).toBe('/admin');
  });
});

describe('PipelineTab — copy contract (39-UI-SPEC §Copywriting)', () => {
  it('renders the locked idle copy verbatim', async () => {
    statusOverrides = { failures: [] }; // never run → no failure rows yet
    render(<PipelineTab />);
    expect(await screen.findByTestId('pipeline-run-btn')).toHaveTextContent('运行 ECL');
    expect(screen.getByTestId('pipeline-stage-badge')).toHaveTextContent('空闲');
    expect(screen.getByTestId('pipeline-idle-hero')).toHaveTextContent(
      '尚未运行 — 点击「运行 ECL」启动管线',
    );
    expect(screen.getByTestId('pipeline-progress-counter')).toHaveTextContent('—');
    expect(screen.getByTestId('pipeline-current-file')).toHaveTextContent('—');
    expect(screen.getByTestId('pipeline-failures-table')).toHaveTextContent(
      '暂无失败记录 — 管线运行中的失败自动重试 3 次后进入死信，可在此手动重试',
    );
    expect(
      screen.getByText('手动触发；boot 不自动运行。重复运行按 note 级替换，结果幂等。'),
    ).toBeInTheDocument();
    for (const h of ['Note', '阶段', '尝试', '最近错误', '状态', '操作']) {
      expect(screen.getByRole('columnheader', { name: h })).toBeInTheDocument();
    }
  });

  it('renders an unknown stage as the raw string (never blank)', async () => {
    stageNow = 'weird-stage';
    render(<PipelineTab />);
    await screen.findByTestId('pipeline-run-btn');
    expect(screen.getByTestId('pipeline-stage-badge')).toHaveTextContent('weird-stage');
  });

  it('renders mono progress counter + chips + 当前文件 while running', async () => {
    stageNow = 'extract';
    render(<PipelineTab />);
    await screen.findByTestId('pipeline-run-btn');
    await waitFor(() => {
      expect(screen.getByTestId('pipeline-progress-counter')).toHaveTextContent('12/26');
    });
    expect(screen.getByTestId('pipeline-current-file')).toHaveTextContent(
      'sql-guides/013__sec03_payment_due.md',
    );
    expect(screen.getByTestId('pipeline-current-file')).toHaveAttribute(
      'title',
      'sql-guides/013__sec03_payment_due.md',
    );
    expect(screen.getByText('待加载队列').parentElement?.textContent).toContain('3');
    expect(screen.getByText('失败').parentElement?.textContent).toContain('3');
  });
});

describe('PipelineTab — poll cadence (D-Prog-01)', () => {
  it('polls every 2s while running and re-arms to 10s after stage flips to done', async () => {
    vi.useFakeTimers();
    stageNow = 'extract';
    render(<PipelineTab />);

    // Mount: first fetch resolves extract → cadence re-arms to 2s
    // (immediate refetch + 2s interval, useEvalReports re-arm pattern).
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(statusCalls()).toBe(2);
    expect(screen.getByTestId('pipeline-stage-badge')).toHaveTextContent('实体抽取');

    // While running: no poll before 2s, exactly one at 2s.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1999);
    });
    expect(statusCalls()).toBe(2);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(statusCalls()).toBe(3);

    // Flip server truth to done → next poll re-arms the loop to 10s.
    stageNow = 'done';
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    expect(statusCalls()).toBe(5); // poll + immediate refetch after re-arm
    expect(screen.getByTestId('pipeline-stage-badge')).toHaveTextContent('已完成');

    // 10s cadence: no poll within 9999ms of the re-arm, one at 10000ms.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(9999);
    });
    expect(statusCalls()).toBe(5);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(statusCalls()).toBe(6);
  });

  it('polling survives transient errors and recovers on the next tick', async () => {
    vi.useFakeTimers();
    networkFail = true;
    render(<PipelineTab />);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(screen.getByTestId('pipeline-load-error')).toHaveTextContent(
      '管线状态加载失败（点击重试）',
    );
    networkFail = false;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000);
    });
    expect(screen.queryByTestId('pipeline-load-error')).toBeNull();
    expect(screen.getByTestId('pipeline-stage-badge')).toBeInTheDocument();
  });
});

describe('PipelineTab — Run lifecycle', () => {
  it('confirm-gates the POST; cancel is a no-op; 409 renders the conflict toast', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    render(
      <>
        <PipelineTab />
        <SfToast />
      </>,
    );
    await screen.findByTestId('pipeline-run-btn');
    const runCalls = () =>
      fetchMock.mock.calls.filter(
        (c) => String(c[0]) === '/api/admin/pipeline/run' && (c[1]?.method ?? 'GET') === 'POST',
      ).length;

    fireEvent.click(screen.getByTestId('pipeline-run-btn'));
    expect(confirmSpy).toHaveBeenCalledWith(
      '运行 ECL 管线？将重建 sql-guides 的切块、实体抽取与索引（幂等操作，可重复执行）。',
    );
    expect(runCalls()).toBe(0); // cancel = no POST

    confirmSpy.mockReturnValue(true);
    runStatus = 409;
    fireEvent.click(screen.getByTestId('pipeline-run-btn'));
    await waitFor(() =>
      expect(screen.getByTestId('sf-toast')).toHaveTextContent('管线正在运行中，请等待完成'),
    );
    expect(runCalls()).toBe(1);
  });

  it('disables Run as 运行中… only when the server confirms a running stage', async () => {
    stageNow = 'extract';
    render(<PipelineTab />);
    const btn = await screen.findByTestId('pipeline-run-btn');
    await waitFor(() => expect(btn).toBeDisabled());
    expect(btn).toHaveTextContent('运行中…');
    const badge = screen.getByTestId('pipeline-stage-badge');
    expect(badge).toHaveTextContent('实体抽取');
    expect(badge.className).toContain('animate-pulse');
  });

  it('renders the locked 403 line', async () => {
    statusError = 403;
    render(
      <>
        <PipelineTab />
        <SfToast />
      </>,
    );
    expect(await screen.findByTestId('pipeline-403')).toHaveTextContent(
      '缺少或错误的 X-Admin-Token',
    );
  });

  it('renders the locked 503 notice and disables Run as ECL 未启用', async () => {
    statusError = 503;
    render(
      <>
        <PipelineTab />
        <SfToast />
      </>,
    );
    expect(await screen.findByTestId('pipeline-503')).toHaveTextContent(
      'ECL 管线未启用（SKILLFORGE_ECL_ENABLED=off）',
    );
    const btn = screen.getByTestId('pipeline-run-btn');
    expect(btn).toBeDisabled();
    expect(btn).toHaveTextContent('ECL 未启用');
  });
});

describe('PipelineTab — Retry lifecycle (SC-5)', () => {
  it('enables 重试 ONLY on dead rows; click POSTs retry and toasts queued', async () => {
    render(
      <>
        <PipelineTab />
        <SfToast />
      </>,
    );
    await screen.findByTestId('pipeline-retry-007');
    expect(screen.getByTestId('pipeline-retry-007')).toBeEnabled();
    expect(screen.getByTestId('pipeline-retry-007')).toHaveAttribute(
      'aria-label',
      '重试 note 007',
    );
    expect(screen.getByTestId('pipeline-retry-008')).toBeDisabled(); // pending
    expect(screen.getByTestId('pipeline-retry-009')).toBeDisabled(); // resolved

    fireEvent.click(screen.getByTestId('pipeline-retry-007'));
    await waitFor(() =>
      expect(screen.getByTestId('sf-toast')).toHaveTextContent('已重新入队 note 007'),
    );
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/pipeline/retry/007',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('disables the row button while its retry POST is in flight', async () => {
    // Object holder: a `let` closure-assignment gets CFA-narrowed to
    // `never` at the release site — property writes don't.
    const gate: { resolve: ((v: Response) => void) | null } = { resolve: null };
    fetchMock.mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.startsWith('/api/admin/pipeline/retry/') && (init?.method ?? 'GET') === 'POST') {
        return new Promise<Response>((resolve) => {
          gate.resolve = resolve;
        });
      }
      if (url === '/api/admin/pipeline/status') {
        return Promise.resolve(jsonResponse(makeStatus()));
      }
      return Promise.resolve(jsonResponse({ error: 'not stubbed' }, 404));
    });
    render(<PipelineTab />);
    await screen.findByTestId('pipeline-retry-007');
    fireEvent.click(screen.getByTestId('pipeline-retry-007'));
    await waitFor(() => expect(screen.getByTestId('pipeline-retry-007')).toBeDisabled());
    gate.resolve?.(jsonResponse({ status: 'queued', note_id: '007' }));
    await waitFor(() => expect(screen.getByTestId('pipeline-retry-007')).toBeEnabled());
  });

  it('renders the retry failure toast with the raw server reason', async () => {
    fetchMock.mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.startsWith('/api/admin/pipeline/retry/') && (init?.method ?? 'GET') === 'POST') {
        return Promise.resolve(jsonResponse({ error: 'unknown or non-dead note' }, 404));
      }
      if (url === '/api/admin/pipeline/status') {
        return Promise.resolve(jsonResponse(makeStatus()));
      }
      return Promise.resolve(jsonResponse({ error: 'not stubbed' }, 404));
    });
    render(
      <>
        <PipelineTab />
        <SfToast />
      </>,
    );
    await screen.findByTestId('pipeline-retry-007');
    fireEvent.click(screen.getByTestId('pipeline-retry-007'));
    await waitFor(() =>
      expect(screen.getByTestId('sf-toast')).toHaveTextContent(
        '重试失败：unknown or non-dead note',
      ),
    );
  });
});
