// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChatInput — composer at the bottom of the right chat panel.
// Implements the sketch-findings `chat-input-attach-skill.md` minimum:
// textarea + send button + Enter-to-send + Shift+Enter newline + drag-
// drop file affordance + 10MB file size validation.
//
// Quick plan 260815-0xb — the Plan 05-07 (D5-IA-01) clickable Skill
// popover was REPLACED by a Claude Code-style slash-command menu:
// typing "/" at the start of a non-wizard, non-disabled input opens a
// filterable Skill menu (arrow-key navigation + Enter select + Esc
// close). The current Skill renders as a READ-ONLY badge above the
// textarea (chat-input-current-skill); selection happens exclusively
// through the slash menu, which calls useAppStore.setCurrentSkill(id)
// and clears the "/filter" token from the input on selection. The
// interaction borrows Claude Code's harness mental model (slash →
// filter → navigate → select) instead of a click-dropdown.
//
// denova-aligned composer (visual only): the textarea is a multi-line
// (≥3 rows) box whose height the user can drag-resize (resize-y), and
// the send button sits INSIDE the box at the bottom-right as a circular
// paper-plane affordance — matching the reference app's in-box send
// pattern. The textarea carries right padding so text never runs under
// the button.
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { Send } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';

const MAX_FILE_BYTES = 10 * 1024 * 1024; // 10 MB

export interface ChatInputProps {
  onSend: (text: string) => void;
  /** Optional mode label — when set, the Skill/currentSkill hint is
   * replaced by the mode label (e.g. "模式：Skill 向导"). Plan 05-04.
   * Wizard mode also disables the slash menu entirely. */
  modeLabel?: string;
  /** Disable the textarea + send button (e.g. while a preview fetch is
   * in flight or the wizard is in the preview/commit phase). Disabled
   * input never opens the slash menu. */
  disabled?: boolean;
  /** Show a small "busy" hint (replaces the send icon with a spinner
   * when true; preserved so a future streaming wizard reply can show
   * progress without churning the API). */
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
  // If the selected skill is removed from store.skills (e.g. backend
  // dropped the YAML or reload failed and the Skill was unregistered),
  // the badge renders '未选择 Skill' but currentSkillId is still set
  // in the store. Without this effect, the next /api/chat could carry a
  // dangling id. Clear it so the UI + outbound messages stay consistent.
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
    // Slash-menu keys take priority over Enter-to-send while the menu
    // is open (an open menu means the input holds a pending /filter
    // token, never a message the user intends to send).
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
        // Close the menu but KEEP the typed text — Esc dismisses the
        // affordance, it does not discard the user's input.
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
      className={cn(
        'border-t border-border bg-surface p-3',
        isDragging && 'chat-input-dragging',
      )}
    >
      {modeLabel ? (
        <div
          data-testid="chat-input-current-skill"
          className="text-xs text-text-muted mb-2 block"
        >
          {`模式：${modeLabel}`}
        </div>
      ) : (
        <div className="relative mb-2">
          {/* Read-only badge (quick plan 260815-0xb): selection happens
              via the slash menu below, not via a click popover. */}
          <div
            data-testid="chat-input-current-skill"
            className="text-xs text-text-muted inline-flex items-center gap-1"
          >
            {currentSkill ? `当前 Skill：${currentSkill.name}` : '未选择 Skill'}
          </div>
          {menuOpen && (
            <div
              data-testid="chat-input-skill-menu"
              className="absolute bottom-full mb-1 left-0 z-20 w-72 rounded-md border border-border bg-surface-2 shadow-lg"
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

      {files.length > 0 && (
        <div data-testid="chat-input-files" className="mb-2 flex flex-wrap gap-2">
          {files.map((file) => (
            <span
              key={file.id}
              data-testid="chat-input-file-chip"
              className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-0.5 text-xs"
            >
              <span>{file.name}</span>
              <span className="text-text-dim">{Math.round(file.size / 1024)} KB</span>
              <button
                type="button"
                data-testid="chat-input-file-remove"
                onClick={() => removeFile(file.id)}
                className="text-text-muted hover:text-text"
                aria-label={`Remove ${file.name}`}
              >
                ×
              </button>
            </span>
          ))}
        </div>
      )}

      <div className="relative">
        <textarea
          data-testid="chat-input-textarea"
          value={text}
          onChange={(e) => {
            setText(e.target.value);
            // Any edit after an Esc/outside-click dismissal re-arms the
            // slash menu (typing again reopens it, Claude Code-style).
            setDismissed(false);
          }}
          onKeyDown={handleKeyDown}
          disabled={disabled}
          placeholder={
            modeLabel
              ? '回答向导的问题…'
              : '输入问题，或键入 / 选择 Skill，或拖放文件...'
          }
          className="w-full min-h-[96px] max-h-64 resize-y rounded-md border border-border bg-bg px-3 py-2 pr-12 text-base text-text leading-relaxed focus:outline-none focus:border-primary disabled:opacity-60"
          rows={3}
        />
        <button
          type="button"
          data-testid="chat-input-send"
          onClick={handleSend}
          disabled={!text.trim() || disabled}
          aria-label="发送"
          className="absolute right-2 bottom-2 inline-flex items-center justify-center h-8 w-8 rounded-full bg-primary text-white disabled:opacity-40 disabled:cursor-not-allowed hover:opacity-90 transition-opacity"
        >
          <Send className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}
