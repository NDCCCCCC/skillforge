# Sketch Conventions

Patterns established across sketch sessions for SkillForge frontend (ai-agent).

## Stack

- **Frontend**: React 19 + Vite 8 + TypeScript + Vercel AI SDK v5 + shadcn/ui + Tailwind v4
- **Mockup format**: Pure HTML + CSS variables + inline JavaScript (no build step)
- **Theme**: CSS custom properties in `.planning/sketches/themes/default.css` (denova-inspired dark)
- **Variant tabs**: 2-3 variants per sketch, switchable via topbar tabs (no page reload)

## Structure

- **Sketch directory**: `.planning/sketches/NNN-kebab-name/`
- **Per-sketch files**:
  - `README.md` — frontmatter (`sk`, `name`, `question`, `winner`, `tags`) + sections (Design Question / How to View / Variants / What to Look For)
  - `index.html` — single-file HTML with all variants as tabs
  - (optional) `*-outputs.*` for any captured data
- **Manifest**: `.planning/sketches/MANIFEST.md` lists all sketches + winners
- **Themes**: `.planning/sketches/themes/default.css` shared CSS variables

## Patterns

- **Variants as tabs**: topbar 按钮 group 切换 CSS class on body/main, 不 reload 页面
- **Real content from spike findings**: 使用真实的 SKILL.md schema 字段 (`name`, `agent`, `reads`, `temperature`, `reasoning`) 而非 placeholder
- **Interactive elements functional**: collapsible panels, drag-resize, file chips, dropdown — 全部 vanilla JS 在 inline `<script>` 里
- **Theme via CSS variables**: all sketches link to `../themes/default.css`
- **Markup highlighting in code previews**: 使用 `<span class="code-key|code-string|code-value">` 给 SKILL.md / JSON 加 syntax highlight
- **Status colors**: compliant (green), needs-info (yellow), non-compliant (red), unanswered (gray)

## Avoid

- **Orphan `>` characters at line starts** in HTML files (`sed -i '/^>$/d' file.html` to fix) — caused by accidental formatting markers in Write content
- **Hardcoded widths without resize handles** — users want to adjust
- **Lorem ipsum placeholders** — use real spike-validated field names
- **Multiple HTML files per variant** — keep all variants in one HTML with tabs
- **Missing theme link** — every sketch must link `../themes/default.css`
- **Variants without clear naming** — use A/B/C with descriptive suffixes ("Default", "Compressed", "Wide")
- **Skipping the verify step** — always present checkpoint with "Which variant feels right?"

## Tools

- `sed -i '/^>$/d' file.html` — fix orphan `>` lines
- `gsd-sdk query commit "docs(sketch-NNN): [winner] — [insight]" --files ...` — commit pattern
- Browser: open `.html` files directly (`file://`) — no build step
- Markdown frontmatter: validated on read (sketch workflow uses it for winner tracking)