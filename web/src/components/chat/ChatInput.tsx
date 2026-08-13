// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChatInput — composer at the bottom of the right chat panel.
// Implements the sketch-findings `chat-input-attach-skill.md` minimum:
// textarea + send button + Enter-to-send + Shift+Enter newline + drag-
// drop file affordance + 10MB file size validation. The current Skill
// is shown as a clickable selector above the textarea (D5-IA-01).
//
// Plan 05-07 (D5-IA-01) — chat-input-current-skill is a clickable
// Skill selector popover reading store.skills (populated by
// useSkillReload from item.manifest.description). Selecting a row
// calls useAppStore.setCurrentSkill(id). When store.skills is empty
// the popover shows a muted 尚未安装任何 Skill row.
//
// denova-aligned composer (visual only): the textarea is a multi-line
// (≥3 rows) box whose height the user can drag-resize (resize-y), and
// the send button sits INSIDE the box at the bottom-right as a circular
// paper-plane affordance — matching the reference app's in-box send
// pattern. The textarea carries right padding so text never runs under
// the button.
import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { Send } from 'lucide-react';
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
  // Popover state (Plan 05-07) — toggled by the trigger button.
  const [skillOpen, setSkillOpen] = useState(false);
  const skillRef = useRef<HTMLDivElement>(null);

  // Outside-click closes the skill popover (mirrors SkillManagerMenu
  // pattern: ref-contains check + mousedown listener).
  useEffect(() => {
    if (!skillOpen) return;
    function onMouseDown(e: MouseEvent) {
      const target = e.target as Node | null;
      if (!target || !skillRef.current) return;
      if (!skillRef.current.contains(target)) {
        setSkillOpen(false);
      }
    }
    document.addEventListener('mousedown', onMouseDown);
    return () => document.removeEventListener('mousedown', onMouseDown);
  }, [skillOpen]);

  function handleSend() {
    const trimmed = text.trim();
    if (!trimmed) return;
    onSend(trimmed);
    setText('');
  }

  function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
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
        <div className="relative mb-2" ref={skillRef}>
          <button
            type="button"
            data-testid="chat-input-current-skill"
            onClick={() => setSkillOpen((v) => !v)}
            className="text-xs text-text-muted hover:text-text inline-flex items-center gap-1"
            aria-label="切换 Skill"
          >
            {currentSkill ? `当前 Skill：${currentSkill.name}` : '未选择 Skill'}
            <span aria-hidden className="text-text-dim">▾</span>
          </button>
          {skillOpen && (
            <div
              data-testid="chat-input-skill-popover"
              className="absolute bottom-full mb-1 left-0 z-20 w-72 rounded-md border border-border bg-surface-2 shadow-lg"
            >
              <div className="px-3 pt-2 pb-1 text-xs font-semibold text-text-muted">
                选择 Skill
              </div>
              <ul className="px-1 pb-2 max-h-64 overflow-y-auto">
                {skills.length === 0 && (
                  <li
                    data-testid="chat-input-skill-empty"
                    className="px-2 py-1 text-xs text-text-muted"
                  >
                    尚未安装任何 Skill
                  </li>
                )}
                {skills.map((skill) => (
                  <li key={skill.id}>
                    <button
                      type="button"
                      data-testid="chat-input-skill-option"
                      data-skill-id={skill.id}
                      onClick={() => {
                        useAppStore.getState().setCurrentSkill(skill.id);
                        setSkillOpen(false);
                      }}
                      className={cn(
                        'block w-full text-left px-2 py-1 text-xs hover:bg-surface-3 rounded-md',
                        skill.id === currentSkillId && 'bg-primary-dim',
                      )}
                    >
                      <div className="text-text">{skill.name}</div>
                      <div className="text-text-muted">{skill.description}</div>
                    </button>
                  </li>
                ))}
              </ul>
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
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={disabled}
          placeholder={
            modeLabel
              ? '回答向导的问题…'
              : '输入合规问题，或 @ 触发 file/skill，或拖放文件...'
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