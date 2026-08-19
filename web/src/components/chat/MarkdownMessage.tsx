// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// MarkdownMessage — Open Design redesign B4. The assistant's main
// answer body used to render as a pre-wrap plain-text <p>; it now
// renders through react-markdown + remark-gfm so headings, lists,
// tables, emphasis and fenced code blocks survive.
//
// Fenced code blocks become cmd-blocks (the Open Design terminal
// card): a head row with the language label + a copy CTA, and a mono
// body on the --color-terminal-* token set. The lang is recovered from
// the child <code className="language-x"> that remark emits inside
// <pre> — we intercept `pre`, not `code`, so inline code keeps its
// default styling.
//
// Streaming safety: React.memo on the text so sibling part updates
// (tool output landing, verdict arriving) don't re-parse an unchanged
// body; remark tolerates truncated markdown (an unclosed ``` fence
// renders as a code block without throwing).
//
// Threat model: react-markdown renders React elements only — raw HTML
// in the model output is NOT rendered (no rehype-raw), and hrefs pass
// through react-markdown's default url transform (javascript: URLs
// are dropped). No dangerouslySetInnerHTML anywhere in this file.
import { isValidElement, memo, useState, type ReactNode } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Check, Copy } from 'lucide-react';
import { copyToClipboard } from '../../lib/clipboard';

/** Depth-first text extraction from a rendered node tree. */
function extractText(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === 'boolean') return '';
  if (typeof node === 'string' || typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(extractText).join('');
  if (isValidElement<{ children?: ReactNode }>(node)) {
    return extractText(node.props.children);
  }
  return '';
}

/** Recover { lang, code } from the <code> child remark puts inside <pre>. */
function parsePre(children: ReactNode): { lang: string; code: string } {
  const child = Array.isArray(children) ? children[0] : children;
  if (!isValidElement<{ className?: string; children?: ReactNode }>(child)) {
    return { lang: '', code: extractText(children) };
  }
  const match = /language-([\w+-]+)/.exec(child.props.className ?? '');
  return { lang: match ? match[1] : '', code: extractText(child.props.children) };
}

function CmdBlock({ lang, code }: { lang: string; code: string }) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    const ok = await copyToClipboard(code);
    if (ok) {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    }
    // Copy failure is silent — the copyToClipboard helper's contract is
    // boolean; there is no toast surface at this nesting level (B6 adds
    // the app-level toast).
  }

  return (
    <div className="cmd-block" data-testid="cmd-block" data-lang={lang || 'text'}>
      <div className="cmd-block-head">
        <span className="cmd-block-lang">{lang || 'text'}</span>
        <button
          type="button"
          data-testid="cmd-block-copy"
          onClick={handleCopy}
          className="cmd-block-copy"
          aria-label="复制代码"
        >
          {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
          <span>{copied ? '已复制' : '复制'}</span>
        </button>
      </div>
      <pre className="cmd-block-body">
        <code>{code}</code>
      </pre>
    </div>
  );
}

export interface MarkdownMessageProps {
  text: string;
}

export const MarkdownMessage = memo(function MarkdownMessage({ text }: MarkdownMessageProps) {
  return (
    <div className="markdown-body" data-testid="markdown-body">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          pre: ({ children }) => {
            const { lang, code } = parsePre(children);
            return <CmdBlock lang={lang} code={code} />;
          },
          a: ({ href, children }) => (
            <a href={href} target="_blank" rel="noopener noreferrer">
              {children}
            </a>
          ),
        }}
      >
        {text}
      </ReactMarkdown>
    </div>
  );
});

export default MarkdownMessage;
