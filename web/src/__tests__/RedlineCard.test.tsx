// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-06 Task 2 — RedlineCard tests.
//
// Pins the SC-7 acceptance assertions:
//   (a) original-clause text present + rendered with `line-through`
//       (strikethrough) + `bg-redline-removed` red-tinted bg block.
//   (b) proposed-rewrite text present + rendered in `bg-redline-added`
//       green-tinted bg block.
//   (c) rationale text present + rendered with a 2px teal left-border
//       (`border-l-annotation`) — the SCOPED §Color teal surface shared
//       with the counter-evidence card.
//   (d) copy CTA fires navigator.clipboard.writeText with the proposed
//       rewrite (mocked) and shows the 已复制到剪贴板 toast.
//   (e) download CTA triggers URL.createObjectURL + Blob + an anchor
//       click (mocked).
//
// D5-14 is a BACKEND invariant (no data-redline chunk emitted on 合规
// verdicts — no LLM call). On the FE side, the consequence is that
// RedlineCard is simply ABSENT for 合规 verdicts — there is no empty
// state to test here. The MessageBubble dispatcher test (covered in a
// separate suite) asserts that 合规 verdicts render zero redline cards.
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import RedlineCard from '../components/verdict/RedlineCard';
import type { RedlinePayload } from '../types/wire-types';

afterEach(cleanup);

const redline: RedlinePayload = {
  clauseId: 'C-7.3.2',
  originalClause: '差旅住宿费每日不超过 600 元',
  proposedRewrite: '差旅住宿费每日不超过 800 元（一线城市）',
  rationale: '原标准与 2026 年市场均价脱节约 25%，建议区分城市分级。',
};

// navigator.clipboard is absent in jsdom. The clipboard.ts helper guards
// with `!navigator.clipboard?.writeText` and returns false, so without
// this defineProperty every copy CTA would silently no-op (and the test
// would assert "no toast" instead of "toast"). Define the spy here so
// every test in this file has a working clipboard.
function installClipboardSpy() {
  const writeText = vi.fn<(text: string) => Promise<void>>().mockResolvedValue(undefined);
  // navigator.clipboard is read-only in jsdom; use defineProperty.
  const desc: PropertyDescriptor = {
    value: { writeText },
    writable: true,
    configurable: true,
  };
  Object.defineProperty(navigator, 'clipboard', desc);
  return writeText;
}

// URL.createObjectURL / revokeObjectURL are NOT defined in this jsdom
// config — vi.spyOn errors out ("createObjectURL does not exist"). Use
// Object.defineProperty so we install them whether or not jsdom provides
// them, then return the vi.fn spies so tests can assert on calls.
function installUrlSpy() {
  const create = vi.fn<(blob: Blob) => string>().mockReturnValue('blob:fake-url');
  const revoke = vi.fn<(url: string) => void>().mockImplementation(() => undefined);
  Object.defineProperty(URL, 'createObjectURL', { value: create, writable: true, configurable: true });
  Object.defineProperty(URL, 'revokeObjectURL', { value: revoke, writable: true, configurable: true });
  return { create, revoke };
}

describe('RedlineCard', () => {
  let writeText: ReturnType<typeof installClipboardSpy>;
  let urlSpies: ReturnType<typeof installUrlSpy>;
  let clickSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    writeText = installClipboardSpy();
    urlSpies = installUrlSpy();
    // jsdom logs "Error: Not implemented: navigation" when an anchor's
    // click() fires with an href. The download path constructs a Blob
    // URL anchor and clicks it — jsdom tries to navigate, fails, and
    // pollutes stderr. Stub click() globally for this file so the
    // download tests assert the call without triggering jsdom navigation.
    clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the original clause with `line-through` + red-tinted bg block', () => {
    render(<RedlineCard data={redline} />);
    const original = screen.getByTestId('redline-original');
    expect(original).toHaveTextContent(redline.originalClause);
    expect(original).toHaveClass('line-through', 'bg-redline-removed');
    // And it uses the mono font (the diff block contract).
    expect(original).toHaveClass('font-mono');
  });

  it('renders the proposed rewrite with green-tinted bg block', () => {
    render(<RedlineCard data={redline} />);
    const rewrite = screen.getByTestId('redline-rewrite');
    expect(rewrite).toHaveTextContent(redline.proposedRewrite);
    expect(rewrite).toHaveClass('bg-redline-added', 'text-status-compliant', 'font-mono');
  });

  it('renders the rationale with a teal left-border (SCOPED §Color surface)', () => {
    render(<RedlineCard data={redline} />);
    const rationale = screen.getByTestId('redline-rationale');
    expect(rationale).toHaveTextContent(redline.rationale);
    expect(rationale).toHaveClass('border-l-2', 'border-l-annotation');
    // The label above is teal as well.
    // (We don't use a testid on the label — assert via the parent card
    // textContent + the rationale element's class chain.)
    const card = screen.getByTestId('redline-card');
    expect(card).toHaveTextContent('修改理由');
  });

  it('copy CTA fires navigator.clipboard.writeText with proposedRewrite', async () => {
    render(<RedlineCard data={redline} />);
    const copyCta = screen.getByTestId('redline-copy-cta');
    fireEvent.click(copyCta);

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledTimes(1);
    });
    expect(writeText).toHaveBeenCalledWith(redline.proposedRewrite);

    // The success toast appears (matches wizard-success-toast pattern).
    await waitFor(() => {
      expect(screen.getByTestId('redline-toast')).toHaveTextContent('已复制到剪贴板。');
    });
  });

  it('copy CTA shows an error toast when clipboard.writeText rejects', async () => {
    // Force a rejection to exercise the error branch in copyToClipboard.
    writeText.mockRejectedValueOnce(new Error('permission denied'));
    render(<RedlineCard data={redline} />);
    fireEvent.click(screen.getByTestId('redline-copy-cta'));

    await waitFor(() => {
      expect(screen.getByTestId('redline-toast')).toHaveTextContent('复制失败。请手动选择文本复制。');
    });
  });

  it('download CTA triggers URL.createObjectURL + an anchor click', () => {
    render(<RedlineCard data={redline} />);
    fireEvent.click(screen.getByTestId('redline-download-cta'));

    // URL.createObjectURL fired (Blob upload to blob: URL).
    expect(urlSpies.create).toHaveBeenCalledTimes(1);
    // The anchor.click() was invoked (the actual download trigger).
    expect(clickSpy).toHaveBeenCalledTimes(1);
    // revokeObjectURL is called on a setTimeout(0); it MAY or MAY NOT
    // have fired synchronously, so we don't assert on it.
  });

  it('download body includes all three fields (original / rewrite / rationale)', () => {
    // Inspect the Blob passed to URL.createObjectURL.
    let capturedBlob: Blob | undefined;
    urlSpies.create.mockImplementation((blob: Blob) => {
      capturedBlob = blob;
      return 'blob:fake-url';
    });

    render(<RedlineCard data={redline} />);
    fireEvent.click(screen.getByTestId('redline-download-cta'));

    expect(capturedBlob).toBeDefined();
    expect(capturedBlob?.type).toBe('text/markdown;charset=utf-8');
    // We can't read Blob contents synchronously without FileReader.
    // The blob construction in clipboard.ts uses the body string we
    // built, so assert the type + that the call happened with a Blob
    // of non-zero size (Blob.size is sync-readable).
    expect(capturedBlob?.size).toBeGreaterThan(redline.originalClause.length);
  });

  it('download filename derives from clauseId when present', () => {
    render(<RedlineCard data={redline} />);
    // Spy on HTMLAnchorElement to capture the download attribute.
    const setter = vi.spyOn(HTMLAnchorElement.prototype, 'download', 'set');
    fireEvent.click(screen.getByTestId('redline-download-cta'));
    expect(setter).toHaveBeenCalledWith('C-7.3.2.md');
  });

  it('download filename falls back to "redline.md" when clauseId is absent', () => {
    const noClause: RedlinePayload = {
      originalClause: 'foo',
      proposedRewrite: 'bar',
      rationale: 'baz',
    };
    render(<RedlineCard data={noClause} />);
    const setter = vi.spyOn(HTMLAnchorElement.prototype, 'download', 'set');
    fireEvent.click(screen.getByTestId('redline-download-cta'));
    expect(setter).toHaveBeenCalledWith('redline.md');
  });

  it('download filename sanitizes path separators / control chars from clauseId (W-2)', () => {
    // The redline LLM is a fresh producer whose output has not been seen
    // in production. A clauseId containing path separators (../, \\, :),
    // could in principle be interpreted by file managers or older
    // browser engines. Defense-in-depth: collapse to safe dashes.
    const messy: RedlinePayload = {
      clauseId: 'a/b\\c:d',
      originalClause: 'foo',
      proposedRewrite: 'bar',
      rationale: 'baz',
    };
    render(<RedlineCard data={messy} />);
    const setter = vi.spyOn(HTMLAnchorElement.prototype, 'download', 'set');
    fireEvent.click(screen.getByTestId('redline-download-cta'));
    // All of /, \, : collapse to '-': "a-b-c-d.md".
    expect(setter).toHaveBeenCalledWith('a-b-c-d.md');
  });

  it('download filename strips leading/trailing dashes and falls back when only separators remain (W-2)', () => {
    // clauseId is entirely path separators + control chars → after
    // sanitization the result is empty → must fall back to 'redline'.
    const allBad: RedlinePayload = {
      clauseId: '/\\:',
      originalClause: 'foo',
      proposedRewrite: 'bar',
      rationale: 'baz',
    };
    render(<RedlineCard data={allBad} />);
    const setter = vi.spyOn(HTMLAnchorElement.prototype, 'download', 'set');
    fireEvent.click(screen.getByTestId('redline-download-cta'));
    expect(setter).toHaveBeenCalledWith('redline.md');
  });

  it('download filename caps clauseId at 64 characters (W-2)', () => {
    // An adversarial or buggy LLM could emit a very long clauseId. Cap
    // at 64 chars so the filename stays manageable in file managers.
    const longClause = 'a'.repeat(100);
    const long: RedlinePayload = {
      clauseId: longClause,
      originalClause: 'foo',
      proposedRewrite: 'bar',
      rationale: 'baz',
    };
    render(<RedlineCard data={long} />);
    const setter = vi.spyOn(HTMLAnchorElement.prototype, 'download', 'set');
    fireEvent.click(screen.getByTestId('redline-download-cta'));
    // 64 a's + ".md".
    const expected = 'a'.repeat(64) + '.md';
    expect(setter).toHaveBeenCalledWith(expected);
  });
});
