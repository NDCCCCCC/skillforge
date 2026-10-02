// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// StepStartPart — visual separator between tool-call steps. Per
// 04-UI-SPEC.md §FEUI-01 the assistant message bubble uses <hr> dividers
// to indicate step boundaries when the agent runs multiple tool calls.
import type { UIMessage } from 'ai';

type StepStartPart = Extract<UIMessage['parts'][number], { type: 'step-start' }>;

export function StepStartPart({ part: _part }: { part: StepStartPart }) {
  return <hr className="step-divider" data-testid="step-start" />;
}
