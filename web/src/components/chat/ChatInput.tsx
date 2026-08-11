// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChatInput — composer at the bottom of the right chat panel.
// Implements the sketch-findings `chat-input-attach-skill.md` minimum:
// textarea + send button + Enter-to-send + Shift+Enter newline + drag-
// drop file affordance + 10MB file size validation. The current Skill
// is shown as a hint above the textarea (display only).
//
// denova-aligned composer (visual only): the textarea is a multi-line
// (≥3 rows) box whose height the user can drag-resize (resize-y), and
// the send button sits INSIDE the box at the bottom-right as a circular
// paper-plane affordance — matching the reference app's in-box send
// pattern. The textarea carries right padding so text never runs under
// the button.
import { useState, type KeyboardEvent } from 'react';
import { Send } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import skillsData from '../../__fixtures__/skills.json';

interface SkillFixtureEntry {
  id: string;
  name: string;
}

const SKILLS: SkillFixtureEntry[] = skillsData as SkillFixtureEntry[];
const MAX_FILE_BYTES = 10 * 1024 * 1024; // 10 MB

export interface ChatInputProps {
  onSend: (text: string) => void;
}

interface AttachedFile {
  id: number;
  name: string;
  size: number;
}

let fileIdCounter = 0;

export default function ChatInput({ onSend }: ChatInputProps) {
  const [text, setText] = useState('');
  const [isDragging, setIsDragging] = useState(false);
  const [files, setFiles] = useState<AttachedFile[]>([]);
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  const currentSkill = SKILLS.find((s) => s.id === currentSkillId);

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
      <div data-testid="chat-input-current-skill" className="text-xs text-text-muted mb-2 block">
        {currentSkill ? `当前 Skill：${currentSkill.name}` : '未选择 Skill'}
      </div>

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
          placeholder="输入合规问题，或 @ 触发 file/skill，或拖放文件..."
          className="w-full min-h-[96px] max-h-64 resize-y rounded-md border border-border bg-bg px-3 py-2 pr-12 text-base text-text leading-relaxed focus:outline-none focus:border-primary"
          rows={3}
        />
        <button
          type="button"
          data-testid="chat-input-send"
          onClick={handleSend}
          disabled={!text.trim()}
          aria-label="发送"
          className="absolute right-2 bottom-2 inline-flex items-center justify-center h-8 w-8 rounded-full bg-primary text-white disabled:opacity-40 disabled:cursor-not-allowed hover:opacity-90 transition-opacity"
        >
          <Send className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}
