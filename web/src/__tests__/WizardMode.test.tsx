// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-04 Task 2 — WizardMode + WizardHeader + WizardPreview tests.
//
// Pins the SC-3 acceptance assertions for the right-panel wizard:
//   1. WizardMode is mounted by RightChat when wizardMode.active=true.
//   2. WizardHeader shows 步骤 {N}/{total} + Progress bar + 取消 CTA.
//   3. The 5 step prompts are pushed into the transcript in order.
//   4. Clicking 取消向导 opens the alert-dialog.
//   5. Confirming 放弃 dispatches store.setWizardMode({active:false}).
//   6. WizardPreview renders the 7 fields + 3 confirmation CTAs.
//   7. Clicking 确认并保存 POSTs to /api/skills/wizard/commit.

import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import WizardMode from '../components/layout/WizardMode';
import WizardHeader from '../components/layout/WizardHeader';
import WizardPreview from '../components/layout/WizardPreview';
import { useAppStore } from '../store/app-store';
import type { WizardPreviewResponse } from '../types/wizard';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  useAppStore.setState({
    wizardMode: { active: false, step: 0, totalSteps: 0 },
  });
});

beforeEach(() => {
  useAppStore.setState({
    wizardMode: { active: false, step: 0, totalSteps: 0 },
  });
});

const SAMPLE_PREVIEW: WizardPreviewResponse = {
  preview: {
    description: '审批流合规审计 Skill，依据权威 tier 制度给出可核验判定',
    agent: 'ChatModelAgent',
    reads: ['authority/**'],
    writes: [],
    steps: '1. 检索权威制度条款\n2. 逐条比对',
    guardrails: {
      prohibitions: [
        'Never cite clauses not retrieved from authority tier',
        'Never paraphrase critical clauses',
        'Never modify authority tier content',
      ],
      required_refusal_phrases: ['no_authorized_source'],
      tier_limits: { authority: ['read-only'] },
    },
    temperature: 0.0,
  },
  name: 'compliance-audit',
};

describe('WizardHeader', () => {
  it('shows 步骤 {N}/{total} + Progress bar + 取消 CTA', () => {
    render(
      <WizardHeader
        currentStep={2}
        totalSteps={5}
        answeredSteps={1}
      />,
    );
    const step = screen.getByTestId('wizard-header-step');
    expect(step.textContent).toContain('步骤 2/5');
    expect(step.dataset.step).toBe('2');
    expect(step.dataset.total).toBe('5');
    expect(screen.getByTestId('wizard-header-progress')).toBeInTheDocument();
    expect(screen.getByTestId('wizard-header-progress-fill')).toBeInTheDocument();
    expect(screen.getByTestId('wizard-cancel-trigger')).toBeInTheDocument();
  });

  it('opens the cancel dialog when the 取消 CTA is clicked', () => {
    render(
      <WizardHeader
        currentStep={3}
        totalSteps={5}
        answeredSteps={2}
      />,
    );
    fireEvent.click(screen.getByTestId('wizard-cancel-trigger'));
    expect(screen.getByTestId('wizard-cancel-dialog')).toBeInTheDocument();
  });

  it('confirming 放弃 flips wizardMode.active to false', () => {
    useAppStore.setState({
      wizardMode: { active: true, step: 3, totalSteps: 5 },
    });
    render(
      <WizardHeader
        currentStep={3}
        totalSteps={5}
        answeredSteps={2}
      />,
    );
    fireEvent.click(screen.getByTestId('wizard-cancel-trigger'));
    fireEvent.click(screen.getByTestId('wizard-cancel-dialog-confirm'));
    expect(useAppStore.getState().wizardMode.active).toBe(false);
  });
});

describe('WizardPreview', () => {
  it('renders the 7 fields + 3 confirmation CTAs', () => {
    render(
      <WizardPreview
        preview={SAMPLE_PREVIEW}
        answeredSteps={5}
        totalSteps={5}
        onRegenerate={vi.fn()}
        onCommit={vi.fn()}
        onCancelWizard={vi.fn()}
        busy={false}
      />,
    );
    expect(screen.getByTestId('wizard-preview-name').textContent).toBe('compliance-audit');
    expect(screen.getByTestId('wizard-preview-agent').textContent).toBe('ChatModelAgent');
    expect(screen.getByTestId('wizard-preview-reads').textContent).toBe('authority/**');
    expect(screen.getByTestId('wizard-preview-writes').textContent).toBe('（空）');
    expect(screen.getByTestId('wizard-preview-temperature').textContent).toBe('0');
    expect(screen.getByTestId('wizard-preview-regenerate')).toBeInTheDocument();
    expect(screen.getByTestId('wizard-preview-confirm')).toBeInTheDocument();
    expect(screen.getByTestId('wizard-preview-cancel')).toBeInTheDocument();
  });

  it('calls onRegenerate when 重新生成 is clicked', () => {
    const onRegenerate = vi.fn();
    render(
      <WizardPreview
        preview={SAMPLE_PREVIEW}
        answeredSteps={5}
        totalSteps={5}
        onRegenerate={onRegenerate}
        onCommit={vi.fn()}
        onCancelWizard={vi.fn()}
        busy={false}
      />,
    );
    fireEvent.click(screen.getByTestId('wizard-preview-regenerate'));
    expect(onRegenerate).toHaveBeenCalledTimes(1);
  });

  it('calls onCommit when 确认并保存 is clicked', () => {
    const onCommit = vi.fn();
    render(
      <WizardPreview
        preview={SAMPLE_PREVIEW}
        answeredSteps={5}
        totalSteps={5}
        onRegenerate={vi.fn()}
        onCommit={onCommit}
        onCancelWizard={vi.fn()}
        busy={false}
      />,
    );
    fireEvent.click(screen.getByTestId('wizard-preview-confirm'));
    expect(onCommit).toHaveBeenCalledTimes(1);
  });
});

describe('WizardMode', () => {
  it('renders the wizard header + first prompt when activated', () => {
    useAppStore.setState({
      wizardMode: { active: true, step: 1, totalSteps: 5 },
    });
    render(<WizardMode />);
    expect(screen.getByTestId('wizard-mode')).toBeInTheDocument();
    expect(screen.getByTestId('wizard-header')).toBeInTheDocument();
    // The first step's assistant prompt is rendered immediately as the
    // initial transcript entry.
    expect(screen.getByTestId('wizard-turn-assistant').textContent).toContain(
      '业务问题',
    );
  });

  it('submits answers into the transcript as user turns', () => {
    useAppStore.setState({
      wizardMode: { active: true, step: 1, totalSteps: 5 },
    });
    render(<WizardMode />);
    const textarea = screen.getByTestId('chat-input-textarea');
    fireEvent.change(textarea, { target: { value: 'Compliance audit for policy reviews.' } });
    fireEvent.click(screen.getByTestId('chat-input-send'));

    expect(screen.getByTestId('wizard-turn-user').textContent).toContain(
      'Compliance audit for policy reviews.',
    );
  });

  it('clicking 取消向导 + 放弃 clears wizardMode.active', () => {
    useAppStore.setState({
      wizardMode: { active: true, step: 2, totalSteps: 5 },
    });
    render(<WizardMode />);
    fireEvent.click(screen.getByTestId('wizard-cancel-trigger'));
    fireEvent.click(screen.getByTestId('wizard-cancel-dialog-confirm'));
    expect(useAppStore.getState().wizardMode.active).toBe(false);
  });

  it('POSTs to /api/skills/wizard when 生成预览 is clicked', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      json: async () => ({ preview: SAMPLE_PREVIEW.preview, name: SAMPLE_PREVIEW.name }),
    } as unknown as Response);
    vi.stubGlobal('fetch', fetchMock);

    useAppStore.setState({
      wizardMode: { active: true, step: 1, totalSteps: 5 },
    });
    render(<WizardMode />);

    // Drive the interview to completion through the ChatInput. Each
    // step is wrapped in act() so React state updates flush before the
    // next render snapshot (the submitAnswer path advances `step`
    // through a setState call).
    //
    // Note: ChatInput's send button is disabled when the textarea is
    // empty (UX guard against accidental submits), so we send a non-
    // empty token for the optional temperature step too — the wizard
    // ignores non-numeric values and falls back to the type default
    // (compliance → 0.0 per D5-12).
    const answers = [
      'Compliance audit for policy reviews.',
      'authority, history',
      'compliance',
      'Never modify authority tier',
      '0', // explicit temperature → confirms type-default
    ];
    for (const a of answers) {
      await act(async () => {
        const textarea = screen.getByTestId('chat-input-textarea');
        fireEvent.change(textarea, { target: { value: a } });
        fireEvent.click(screen.getByTestId('chat-input-send'));
      });
    }

    // After all 5 answers, step === totalSteps and the 生成预览 CTA
    // appears in the transcript pane.
    await waitFor(() => {
      expect(screen.getByTestId('wizard-generate-preview')).toBeInTheDocument();
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId('wizard-generate-preview'));
    });

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/skills/wizard',
        expect.objectContaining({ method: 'POST' }),
      );
    });
  });
});