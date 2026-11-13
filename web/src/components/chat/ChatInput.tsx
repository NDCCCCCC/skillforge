// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChatInput — modernized floating-card composer (Phase 4 visual upgrade).
//
// Implements the sketch-findings `chat-input-attach-skill.md` minimum:
// textarea + send button + Enter-to-send + Shift+Enter newline + drag-
// drop file affordance + 10MB file size validation.
//
// Quick plan 260815-0xb — the Plan 05-07 (D5-IA-01) clickable Skill
// popover was REPLACED by a Claude Code-style slash-command menu:
// typing "/" at the start of a non-wizard, non-disabled input opens a
// filterable Skill menu (arrow-key navigation + Enter select + Esc
// close). 261113-lhu — the READ-ONLY pill badge is replaced by the
// SkillPicker dropdown (moved down from the TopBar): the composer's
// top-left now hosts the same interactive surface as a click-first
// entry, and the slash menu stays as the keyboard-first entry (the two
// are independent floating layers — typing "/" still opens the menu).
// The chat-input-current-skill testid now lives on the picker's w-44
// wrapper so downstream selectors stay stable.
//
// Visual upgrade (Phase 4): the composer is now a floating rounded
// card with subtle shadow + focus glow, sitting on a transparent
// background. The send button is a square-rounded button at the
// bottom-right of the card (no longer a circular paper-plane).
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { ArrowUp, Paperclip, Square, X } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import ModelPicker from './ModelPicker';
import SkillPicker from './parts/SkillPicker';

const MAX_FILE_BYTES = 10 * 1024 * 1024; // 10 MB

export interface ChatInputProps {
  onSend: (text: string) => void;
  /** Optional mode label — when set, the SkillPicker/model-picker row is
   * replaced by the mode label (e.g. "模式：Skill 向导"). Plan 05-04. */
  modeLabel?: string;
  /** Disable the textarea + send button (e.g. while a preview fetch is
   * in flight or the wizard is in the preview/commit phase). */
  disabled?: boolean;
  /** Show a small "busy" hint (replaces the send icon with a spinner
   * when true). */
  busy?: boolean;
  /** Streaming state — when true the send button becomes a stop button.
   * Without this the user has no way to interrupt a running answer:
   * before 2026-11-09 the chat had no stop control at all, so a slow or
   * runaway generation could only be escaped by reloading the page
   * (which loses the in-flight turn). */
  streaming?: boolean;
  /** Abort the in-flight generation. Required for the stop button; when
   * absent the button falls back to the send affordance. */
  onStop?: () => void;
}

interface AttachedFile {
  id: number;
  name: string;
  size: number;
}

let fileIdCounter = 0;

// isSlashToken — the slash menu is open iff the input text is exactly
// one pending "/filter" token: starts with "/" and contains NO
// whitespace. "/cal" filters, "/ calc" (space) closes — a message that
// legitimately begins with a slash command plus arguments stays a
// normal send.
function isSlashToken(text: string): boolean {
  return text.startsWith('/') && !/\s/.test(text);
}

export default function ChatInput({
  onSend,
  modeLabel,
  disabled = false,
  busy = false,
  streaming = false,
  onStop,
}: ChatInputProps) {
  const [text, setText] = useState('');
  const [isDragging, setIsDragging] = useState(false);
  const [files, setFiles] = useState<AttachedFile[]>([]);
  // Plan 05-07 (D5-IA-01) — subscribe to the live skill surface
  // populated by useSkillReload (item.manifest.description).
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  const skills = useAppStore((s) => s.skills);
  // Slash menu state (quick plan 260815-0xb). menuOpen is derived from
  // the input text (never toggled independently) so the menu cannot
  // desync from the textarea; activeIndex tracks the keyboard
  // highlight within the filtered list. `dismissed` is the Esc /
  // outside-click latch: it suppresses the menu while the slash token
  // stays in the input, and any subsequent edit re-arms it (matching
  // Claude Code: Esc dismisses, typing again reopens).
  const [dismissed, setDismissed] = useState(false);
  const menuOpen = !modeLabel && !disabled && !dismissed && isSlashToken(text);
  const filter = menuOpen ? text.slice(1) : '';
  const [activeIndex, setActiveIndex] = useState(0);
  const menuRef = useRef<HTMLDivElement>(null);
  // 261113-j42：textarea auto-grow 用。rows={1} 仅作 SSR/首帧基线，
  // 实际高度由下面的 effect 按 scrollHeight 撑高。
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Filtered skills: case-insensitive substring match against BOTH the
  // name and the description (a Skill is findable by what it does,
  // not only by its name).
  const filteredSkills = useMemo(() => {
    if (!menuOpen) return [];
    const q = filter.toLowerCase();
    return skills.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        s.description.toLowerCase().includes(q),
    );
  }, [menuOpen, filter, skills]);

  // Clamp activeIndex whenever the list shrinks (skills reloaded,
  // filter narrowed) so the highlight always sits on a live row.
  useEffect(() => {
    if (activeIndex > 0 && activeIndex >= filteredSkills.length) {
      setActiveIndex(Math.max(0, filteredSkills.length - 1));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filteredSkills.length]);

  // Outside-mousedown closes the menu (mirrors the SkillManagerMenu
  // pattern: ref-contains check + mousedown listener). The typed text
  // is PRESERVED — dismissal latches via `dismissed`, it does not
  // discard the user's input.
  useEffect(() => {
    if (!menuOpen) return;
    function onMouseDown(e: MouseEvent) {
      const target = e.target as Node | null;
      if (!target || !menuRef.current) return;
      if (!menuRef.current.contains(target)) {
        setDismissed(true);
      }
    }
    document.addEventListener('mousedown', onMouseDown);
    return () => document.removeEventListener('mousedown', onMouseDown);
  }, [menuOpen]);

  // Plan 05-08 Task 1 (WR-02 fix) — stale currentSkillId defense.
  useEffect(() => {
    const stale = currentSkillId !== null && !skills.some((s) => s.id === currentSkillId);
    if (stale) useAppStore.getState().setCurrentSkill(null);
  }, [currentSkillId, skills]);

  // Phase 17 D17-C1: drill-chip prefill — read from app-store.pendingDraft
  const pendingDraft = useAppStore((s) => s.pendingDraft);
  const setPendingDraft = useAppStore((s) => s.setPendingDraft);

  useEffect(() => {
    if (pendingDraft && !text) {
      setText(pendingDraft);
      setPendingDraft(''); // consume one-shot
    }
  }, [pendingDraft, text, setPendingDraft]);

  // D18-D4: chat-input-prefill event from MessageBubble's regenerate chip
  // writes to pendingDraft so the next render cycle pre-fills ChatInput.
  useEffect(() => {
    const handler = (e: Event) => {
      const customEvent = e as CustomEvent;
      if (typeof customEvent.detail === 'string' && customEvent.detail) {
        setPendingDraft(customEvent.detail);
      }
    };
    document.addEventListener('chat-input-prefill', handler);
    return () => document.removeEventListener('chat-input-prefill', handler);
  }, [setPendingDraft]);

  // 261113-j42：textarea auto-grow——多行内容撑高输入框而不是出现内部
  // 滚动条。先重置为 auto 让浏览器按内容算出 scrollHeight，再钳到 200
  // （与 theme.css 的 .composer-textarea max-height 一致，超限后 CSS
  // overflow 接管）。依赖 text：发送清空后自动回落到单行（CSS 的
  // min-height:56px 兜底，scrollHeight 小于它时不缩水）。
  useEffect(() => {
    const ta = textareaRef.current;
    if (!ta) return;
    ta.style.height = 'auto';
    ta.style.height = `${Math.min(ta.scrollHeight, 200)}px`;
  }, [text]);

  // CODE-AUDIT 2026-11-05 (D-01): attached files were collected into
  // `files` state, rendered as removable chips, and then SILENTLY DROPPED —
  // onSend carries only the text, so the user believed a document reached
  // the model when nothing did (silent data loss). Until a real upload
  // channel exists, sending with attachments is blocked and the chip bar
  // says so explicitly; after a blocked send the attachments survive.
  const hasAttachments = files.length > 0;

  function handleSend() {
    const trimmed = text.trim();
    if (!trimmed || hasAttachments) return;
    onSend(trimmed);
    setText('');
    // Phase 17: ensure pendingDraft is cleared on send
    if (pendingDraft) {
      setPendingDraft('');
    }
  }

  function selectSkill(id: string) {
    useAppStore.getState().setCurrentSkill(id);
    setText('');
  }

  function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (menuOpen) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (filteredSkills.length > 0) {
          setActiveIndex((i) => (i + 1) % filteredSkills.length);
        }
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (filteredSkills.length > 0) {
          setActiveIndex((i) => (i - 1 + filteredSkills.length) % filteredSkills.length);
        }
        return;
      }
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        const skill = filteredSkills[activeIndex];
        if (skill) selectSkill(skill.id);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setDismissed(true);
        return;
      }
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  function handleDrop(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault();
    setIsDragging(false);
    const dropped = Array.from(e.dataTransfer.files);
    const accepted: AttachedFile[] = [];
    for (const file of dropped) {
      if (file.size > MAX_FILE_BYTES) {
        // eslint-disable-next-line no-console
        console.warn('[chat-input] file exceeds 10MB limit:', file.name);
        continue;
      }
      accepted.push({ id: ++fileIdCounter, name: file.name, size: file.size });
    }
    if (accepted.length > 0) setFiles((prev) => [...prev, ...accepted]);
  }

  function removeFile(id: number) {
    setFiles((prev) => prev.filter((f) => f.id !== id));
  }

  return (
    <div
      data-testid="chat-input"
      data-mode={modeLabel ?? 'chat'}
      data-busy={busy}
      ref={menuRef}
      onDragOver={(e) => {
        e.preventDefault();
        setIsDragging(true);
      }}
      onDragLeave={() => setIsDragging(false)}
      onDrop={handleDrop}
      className="composer-modern"
    >
      {modeLabel ? (
        <div
          data-testid="chat-input-current-skill"
          className="chat-empty-prompt mb-2"
          style={{ cursor: 'default' }}
        >
          <span className="chat-empty-prompt-icon" aria-hidden>
            <Paperclip className="h-3 w-3" />
          </span>
          <span style={{ fontSize: 'var(--text-xs)' }}>模式：{modeLabel}</span>
        </div>
      ) : (
        <div className="relative mb-2 flex items-center justify-between gap-2">
          {/* Left: SkillPicker (261113-lhu — moved down from the TopBar;
              chat-input-current-skill stays on this w-44 wrapper so the
              testid is stable); Right: LLM model picker (260926 —
              chat-page model switching, hot-reload). */}
          <div className="w-44 flex-none" data-testid="chat-input-current-skill">
            <SkillPicker />
          </div>
          <ModelPicker />
          {menuOpen && (
            <div
              data-testid="chat-input-skill-menu"
              className="composer-skill-menu"
            >
              <div className="px-3 pt-2 pb-1 text-xs font-semibold text-text-muted">
                {filter ? `匹配 “${filter}” 的 Skill` : '选择 Skill'}
              </div>
              <ul className="px-1 pb-2 max-h-64 overflow-y-auto">
                {filteredSkills.length === 0 && (
                  <li
                    data-testid="chat-input-skill-empty"
                    className="px-2 py-1 text-xs text-text-muted"
                  >
                    尚未安装任何 Skill
                  </li>
                )}
                {filteredSkills.map((skill, i) => (
                  <li key={skill.id}>
                    <button
                      type="button"
                      data-testid="chat-input-skill-option"
                      data-skill-id={skill.id}
                      data-active={i === activeIndex ? 'true' : undefined}
                      onClick={() => selectSkill(skill.id)}
                      onMouseEnter={() => setActiveIndex(i)}
                      className={cn(
                        'block w-full text-left px-2 py-1 text-xs hover:bg-surface-3 rounded-md',
                        i === activeIndex && 'bg-primary-dim',
                        skill.id === currentSkillId && 'ring-1 ring-primary',
                      )}
                    >
                      <div className="text-text">{skill.name}</div>
                      <div className="text-text-muted">{skill.description}</div>
                    </button>
                  </li>
                ))}
              </ul>
              <div className="px-3 pb-2 pt-1 text-[10px] text-text-dim">
                ↑↓ 导航 · Enter 选择 · Esc 关闭
              </div>
            </div>
          )}
        </div>
      )}

      <div
        className="composer-card"
        data-dragging={isDragging ? 'true' : 'false'}
      >
        {files.length > 0 && (
          <div data-testid="chat-input-files" className="composer-files">
            {files.map((file) => (
              <span
                key={file.id}
                data-testid="chat-input-file-chip"
                className="composer-file-chip"
              >
                <Paperclip className="h-3 w-3" style={{ color: 'var(--color-text-muted)' }} />
                <span>{file.name}</span>
                <span className="composer-file-chip-size">
                  {Math.round(file.size / 1024)} KB
                </span>
                <button
                  type="button"
                  data-testid="chat-input-file-remove"
                  onClick={() => removeFile(file.id)}
                  className="composer-file-remove"
                  aria-label={`Remove ${file.name}`}
                >
                  <X className="h-2.5 w-2.5" />
                </button>
              </span>
            ))}
          </div>
        )}
        <textarea
          data-testid="chat-input-textarea"
          ref={textareaRef}
          value={text}
          onChange={(e) => {
            setText(e.target.value);
            setDismissed(false);
          }}
          onKeyDown={handleKeyDown}
          disabled={disabled}
          placeholder={
            modeLabel
              ? '回答向导的问题…'
              : '输入问题，或键入 / 选择 Skill...'
          }
          className="composer-textarea"
          rows={1}
        />
        <button
          type="button"
          data-testid={streaming && onStop ? 'chat-input-stop' : 'chat-input-send'}
          onClick={streaming && onStop ? onStop : handleSend}
          disabled={streaming && onStop ? false : !text.trim() || disabled || hasAttachments}
          aria-label={streaming && onStop ? '停止生成' : hasAttachments ? '请先移除附件' : '发送'}
          className="composer-send"
        >
          {streaming && onStop ? <Square className="h-4 w-4" /> : <ArrowUp className="h-4 w-4" />}
        </button>
      </div>

      {/* Open Design B6 — dual hint: Enter affordances left, retrieval
          scope right. */}
      <div className="composer-hint" data-testid="composer-hint">
        <span>Enter 发送 · Shift+Enter 换行 · / 选择 Skill</span>
        <span>检索范围：当前 Skill 定义的知识层</span>
      </div>
    </div>
  );
}
