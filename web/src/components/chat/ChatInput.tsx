// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChatInput — composer at the bottom of the right chat panel.
// Implements the sketch-findings `chat-input-attach-skill.md` minimum:
// textarea + send button + Enter-to-send + Shift+Enter newline + drag-
// drop file affordance + 10MB file size validation. The current Skill
// is shown as a hint above the textarea (display only).
import { useState, type KeyboardEvent } from 'react';
import { Button } from '../ui/button';
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
      <div data-testid="chat-input-current-skill" className="text-xs text-text-muted mb-1 block">
        {currentSkill ? `当前 Skill：${currentSkill.name}` : '未选择 Skill'}
      </div>

      {files.length > 0 && (
        <div data-testid="chat-input-files" className="mt-2 flex flex-wrap gap-2">
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

      <div className="flex items-end gap-2">
        <textarea
          data-testid="chat-input-textarea"
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入合规问题，或 @ 触发 file/skill，或拖放文件..."
          className="flex-1 min-h-[40px] max-h-32 resize-none rounded-md border border-border bg-bg px-3 py-2 text-base text-text"
          rows={1}
        />
        <Button
          data-testid="chat-input-send"
          onClick={handleSend}
          disabled={!text.trim()}
        >
          发送
        </Button>
      </div>
    </div>
  );
}
