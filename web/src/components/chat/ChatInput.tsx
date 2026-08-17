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
// close). The current Skill renders as a READ-ONLY pill badge above the
// textarea (chat-input-current-skill); selection happens exclusively
// through the slash menu.
//
// Visual upgrade (Phase 4): the composer is now a floating rounded
// card with subtle shadow + focus glow, sitting on a transparent
// background. The send button is a square-rounded button at the
// bottom-right of the card (no longer a circular paper-plane).
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { ArrowUp, Paperclip, X } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';

const MAX_FILE_BYTES = 10 * 1024 * 1024; // 10 MB

export interface ChatInputProps {
  onSend: (text: string) => void;
  /** Optional mode label — when set, the Skill/currentSkill hint is
   * replaced by the mode label (e.g. "模式：Skill 向导"). Plan 05-04. */
  modeLabel?: string;
  /** Disable the textarea + send button (e.g. while a preview fetch is
   * in flight or the wizard is in the preview/commit phase). */
  disabled?: boolean;
  /** Show a small "busy" hint (replaces the send icon with a spinner
   * when true). */
  busy?: boolean;
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
}: ChatInputProps) {
  const [text, setText] = useState('');
  const [isDragging, setIsDragging] = useState(false);
  const [files, setFiles] = useState<AttachedFile[]>([]);
  // Plan 05-07 (D5-IA-01) — subscribe to the live skill surface
  // populated by useSkillReload (item.manifest.description).
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  const skills = useAppStore((s) => s.skills);
  const currentSkill = skills.find((s) => s.id === currentSkillId);
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

  function handleSend() {
    const trimmed = text.trim();
    if (!trimmed) return;
    onSend(trimmed);
    setText('');
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
        <div className="relative mb-2">
          {/* Read-only badge (quick plan 260815-0xb): selection happens
              via the slash menu below, not via a click popover. */}
          {currentSkill && (
            <div
              data-testid="chat-input-current-skill"
              className="composer-skill-badge"
            >
              <span className="composer-skill-badge-dot" aria-hidden />
              <span>{currentSkill.name}</span>
            </div>
          )}
          {!currentSkill && (
            <div
              data-testid="chat-input-current-skill"
              className="chat-header-skill chat-header-skill-missing"
              style={{ padding: '0 4px', fontSize: 11 }}
            >
              未选择 Skill · 输入 / 选择
            </div>
          )}
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
              : '输入问题，或键入 / 选择 Skill，或拖放文件...'
          }
          className="composer-textarea"
          rows={1}
        />
        <button
          type="button"
          data-testid="chat-input-send"
          onClick={handleSend}
          disabled={!text.trim() || disabled}
          aria-label="发送"
          className="composer-send"
        >
          <ArrowUp className="h-4 w-4" />
        </button>
      </div>

      <div className="composer-hint">
        Enter 发送 · Shift+Enter 换行 · / 选择 Skill · 拖放文件附件
      </div>
    </div>
  );
}
