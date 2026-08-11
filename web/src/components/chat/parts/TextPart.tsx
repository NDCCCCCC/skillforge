// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// TextPart — renders the assistant's main response text. Wraps in <p>
// with body font size per 04-UI-SPEC.md §Typography. Trivial wrapper;
// the work is done by the AI SDK v7 parser upstream (text-start /
// text-delta / text-end collapse into a single text part).
import type { TextUIPart } from 'ai';

export function TextPart({ part }: { part: TextUIPart }) {
  return <p className="text-base text-text leading-relaxed">{part.text}</p>;
}
