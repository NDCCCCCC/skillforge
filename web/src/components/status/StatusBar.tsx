// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// StatusBar — the 26px statusbar of the ThreePanel shell. Phase 4
// inlined this inside ThreePanel.tsx and shipped a single static label
// `就绪`. Phase 5 extracts the component AND adds the SC-2 in-flight
// pinned-version indicator:
//
//   When a chat run is using a pre-reload Skill snapshot, the right
//   label switches to:
//     `当前运行使用旧版本（完成后下次生效）`
//
// Detection (per UI-SPEC §SC-2):
//   - The `start` message metadata may carry a `skillVersion` (emitted
//     by the agent callback once the SC-2 routing pinned a version).
//   - We compare that against the current Skill version returned by
//     `/api/skills`. A mismatch + `useChat.status === 'streaming'`
//     triggers the pinned-version label.
//
// For Phase 5/05-03 we don't yet read `messages` (the streaming metadata
// wiring is a 05-04 concern). The component accepts a `pinnedVersion`
// prop and the App shell passes it in once that plumbing lands. Until
// then the statusbar always shows the inherited `就绪` idle label.
import { useAppStore } from '../../store/app-store';

export interface StatusBarProps {
  /** Optional override for the pinned-version label. When `true`,
   * the statusbar shows the in-flight pinned indicator instead of
   * the inherited idle label. */
  pinnedVersion?: boolean;
  /** Optional override for the displayed skill name on the right
   * when `pinnedVersion` is true. */
  pinnedSkillName?: string;
  /** Height in px (defaults to the CSS var --statusbar-height). */
  height?: number;
}

export default function StatusBar({ pinnedVersion = false, pinnedSkillName, height }: StatusBarProps) {
  const detailPanelOpen = useAppStore((s) => s.detailPanelOpen);

  return (
    <div
      className="bg-surface border-t border-border px-3 text-xs text-text-muted flex items-center justify-between"
      style={{ height: height ?? 'var(--statusbar-height)' }}
      data-testid="app-statusbar"
      data-pinned={String(pinnedVersion)}
    >
      <div className="flex items-center gap-3">
        <span>{detailPanelOpen ? '详情面板已展开' : '详情面板已收起'}</span>
      </div>
      <div
        data-testid="statusbar-right"
        className="flex items-center gap-1"
      >
        {pinnedVersion ? (
          <span data-testid="statusbar-pinned" className="text-text">
            {pinnedSkillName
              ? `当前运行使用旧版本（${pinnedSkillName}）`
              : '当前运行使用旧版本（完成后下次生效）'}
          </span>
        ) : (
          <span data-testid="statusbar-idle">就绪</span>
        )}
      </div>
    </div>
  );
}
