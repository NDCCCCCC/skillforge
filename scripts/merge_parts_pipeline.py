#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Merge knowledge/sql-guides __partNN.md files into semantic __secNN_slug.md sections.

Reusable corpus-reorg pipeline (quick task 260915-m7, locked decision 方案 A — zero Go changes).

Why: a prior one-off session split whole user-supplied notes into __partNN.md files by
line count to fit internal/knowledge/indexer.go's maxDocBytes = 4000 body cap, with zero
semantic awareness. SQL statements straddle part boundaries, bare SQL is glued to Chinese
prose, fences are unclosed or orphaned. This pipeline reshapes the corpus instead: one h2
section = one chunk = one self-contained answer unit.

Phases:
  1 discover+group  2 merge  3 normalize (glue rules + fence re-derivation)
  4 h2 section split  5 byte budget  6 conservation gates  7 write (collect-then-delete)

stdlib only. Exit codes: 0 ok/nothing-to-do, 1 usage, 2 validation/conservation failure.
"""
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path
from typing import Optional

DEFAULT_CORPUS = "knowledge/sql-guides"
DEFAULT_MAX_BYTES = 4000  # mirrors internal/knowledge/indexer.go:67 maxDocBytes

PART_RE = re.compile(r"^(?P<stem>.+)__part(?P<num>\d{1,})\.md$")
STEM_RE = re.compile(r"^(?P<idx>\d+)_(?P<title>.+)$")
H2_RE = re.compile(r"^##\s+(.*?)\s*$")
H1_DEDUPE_RE_CACHE: dict[str, re.Pattern] = {}

FENCE_RE = re.compile(r"^\s*```(.*)$")
FENCE_CLOSE_RE = re.compile(r"^\s*```\s*$")

HEADING_RE = re.compile(r"^(#{1,6})\s+(.*)$")
CJK_RE = re.compile(r"[一-鿿]")

SQL_START_RE = re.compile(
    r"^\s*(select|insert|update|delete|drop|create|alter|truncate|merge|with|grant|revoke|exec|execute|call|begin|declare)\b",
    re.I,
)

# Continuation tokens — longest-first to avoid alternation ambiguity.
_CONT_START_TOKENS = (
    "order", "group", "union", "having", "values", "where",
    "left", "right", "inner", "outer", "join", "from",
    "and", "or", "on", "set", "by", "as",
)
CONT_START_RE = re.compile(
    r"^\s*(?:" + "|".join(re.escape(t) + r"\b" for t in _CONT_START_TOKENS) + r"|,|\)|\(|;)",
    re.I,
)
CONT_END_RE = re.compile(r"(?:,|;|\(|=|--)\s*$")
IDENT_EQ_RE = re.compile(r"^\s*[A-Za-z_][A-Za-z0-9_]*\s*=")
COMMENT_RE = re.compile(r"^\s*--")

PROTECTED_MARKER_RE = re.compile(r"^@@GSDEP_PROTECTED_(\d+)@@\s*$")

# --- Glue rules (closed list — every rule inserts only a newline, so re-applying
# cannot re-match, making normalization idempotent by construction). ---
GLUE_RULES = (
    ("semi_cjk", re.compile(r";(?=[一-鿿])"), ";\n"),
    ("semi_kw", re.compile(r";(?=(?:select|insert|update|delete|drop|create|alter|truncate|merge|with|from|where|and)\b)", re.I), ";\n"),
    ("cjk_kw", re.compile(r"(?<=[一-鿿])(select|insert|update|delete|drop|create|alter|truncate|merge|with|from|where|and)\b", re.I), r"\n\g<1>"),
    ("glued_set", re.compile(r"(?<=[A-Za-z0-9_.)])set (?=[A-Za-z0-9_.\"'\[\]]+=)", re.I), "\nset "),
    ("glued_join", re.compile(r"(?<=[A-Za-z0-9_.])((?:left join|right join|inner join))\b", re.I), r"\n\g<1>"),
)

WINDOWS_RESERVED = {
    "CON", "PRN", "AUX", "NUL",
    *(f"COM{i}" for i in range(1, 10)),
    *(f"LPT{i}" for i in range(1, 10)),
}


# ----- Frontmatter -----

def split_frontmatter(text: str) -> tuple[str, dict]:
    """Mirror internal/knowledge/indexer.go splitFrontmatter: opening `---` on
    line 1, closing `\n---`. Body = everything after, left-trimmed of newlines.
    Returns (body, fm_dict). Permissive: missing frontmatter -> whole text as body.
    """
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    open_marker = "---\n"
    if not text.startswith(open_marker):
        return text, {}
    body_start = len(open_marker)
    close_marker = "\n---"
    end = text.find(close_marker, body_start)
    if end < 0:
        # Unterminated — mirror Go: treat as no frontmatter.
        return text, {}
    fm_block = text[body_start:end]
    after = text[end + len(close_marker):]
    body = after.lstrip("\n")
    fm: dict[str, str] = {}
    for line in fm_block.split("\n"):
        if not line.strip():
            continue
        m = re.match(r"^([A-Za-z_][A-Za-z0-9_-]*)\s*:\s*(.*?)\s*$", line)
        if m:
            fm[m.group(1)] = m.group(2)
    return body, fm


def parse_split_part(s: str) -> tuple[int, int]:
    m = re.match(r"^(\d+)/(\d+)$", s.strip())
    if not m:
        return 0, 0
    return int(m.group(1)), int(m.group(2))


# ----- Discovery -----

class Part:
    __slots__ = ("path", "num", "stem", "body", "title", "idx", "split_n", "split_m")

    def __init__(self, path: Path, num: int, stem: str, body: str, fm: dict):
        self.path = path
        self.num = num
        self.stem = stem
        self.body = body
        self.title = fm.get("title", "").strip()
        self.idx = fm.get("source_idx", "").strip()
        sp = parse_split_part(fm.get("split_part", ""))
        self.split_n, self.split_m = sp
        # Fallback idx/title from stem.
        if not self.idx or not self.title:
            m = STEM_RE.match(stem)
            if m:
                if not self.idx:
                    self.idx = m.group("idx")
                if not self.title:
                    self.title = m.group("title")


class Group:
    __slots__ = ("dirpath", "idx", "title", "parts", "stem")

    def __init__(self, dirpath: Path, idx: str, title: str, parts: list[Part]):
        self.dirpath = dirpath
        self.idx = idx
        self.title = title
        self.parts = parts
        # Group stem = stem of lowest-numbered part.
        self.stem = min(parts, key=lambda p: p.num).stem


def discover(corpus: Path):
    """Return (all_md_paths, parts_list, groups_list).

    groups are sorted by (dirpath, idx) for stable output.
    parts within a group are sorted by num.
    """
    all_md = sorted(p for p in corpus.rglob("*.md") if p.is_file())
    parts: list[Part] = []
    for p in all_md:
        m = PART_RE.match(p.name)
        if not m:
            continue
        raw = p.read_text(encoding="utf-8")
        body, fm = split_frontmatter(raw)
        parts.append(Part(p, int(m.group("num")), m.group("stem"), body, fm))

    groups_map: dict[tuple[str, str, str], list[Part]] = {}
    for part in parts:
        key = (str(part.path.parent), part.idx, part.title)
        groups_map.setdefault(key, []).append(part)
    groups = [
        Group(Path(d), idx, title, sorted(g, key=lambda p: p.num))
        for (d, idx, title), g in groups_map.items()
    ]
    groups.sort(key=lambda g: (g.dirpath.as_posix(), g.idx))
    return all_md, parts, groups


# ----- Phase 2: Merge -----

def merge_group(group: Group, stats: dict) -> tuple[str, int]:
    """Return (merged_body, h1_drop_count). H1 dedupe per locked spec."""
    target_h1 = "# " + group.title
    h1_drops = 0
    pieces: list[str] = []
    for p in group.parts:
        b = p.body.rstrip("\n")
        if pieces and pieces[-1].rstrip() == target_h1 and b.lstrip() == target_h1:
            # Edge case: keep nothing if the very next piece starts with H1.
            h1_drops += 1
            pieces.append(b)
            continue
        # Strip a leading H1 line that is exactly the title (sanctioned dedupe).
        lines = b.split("\n")
        new_lines = []
        for ln in lines:
            if ln.strip() == target_h1:
                h1_drops += 1
                continue
            new_lines.append(ln)
        pieces.append("\n".join(new_lines).strip("\n"))
    merged = "\n\n".join(pieces).strip("\n")
    return merged, h1_drops


# ----- Phase 3a: Glue (per-line, attributed) -----

def glue_normalize_lines(lines: list[str], label: str, verbose: bool, stats: dict) -> list[str]:
    out = []
    for i, line in enumerate(lines):
        cur = line
        for name, rgx, repl in GLUE_RULES:
            new, n = rgx.subn(repl, cur)
            if n:
                stats.setdefault("glue_hits", {}).setdefault(name, 0)
                stats["glue_hits"][name] += n
                if verbose:
                    print(f"  [glue {name}] {label}:{i + 1} hits={n}", file=sys.stderr)
                cur = new
        out.append(cur)
    return out


# ----- Phase 3b: Fence re-derivation -----

def _fence_info(line: str) -> Optional[str]:
    m = FENCE_RE.match(line)
    if not m:
        return None
    return m.group(1).strip().lower()


def _lift_protected(lines: list[str]) -> tuple[list[str], dict[str, list[str]]]:
    """Replace non-SQL fenced blocks with marker lines; return (new_lines, marker -> original_lines)."""
    protected: dict[str, list[str]] = {}
    out: list[str] = []
    i = 0
    while i < len(lines):
        info = _fence_info(lines[i])
        if info is not None and info not in ("", "sql"):
            block = [lines[i]]
            i += 1
            closed = False
            while i < len(lines):
                block.append(lines[i])
                if FENCE_CLOSE_RE.match(lines[i]):
                    closed = True
                    i += 1
                    break
                i += 1
            marker = f"@@GSDEP_PROTECTED_{len(protected)}@@"
            # Safety: assert marker does not collide with real content.
            assert marker not in "\n".join(lines), f"marker collision: {marker}"
            protected[marker] = block
            out.append(marker)
            continue
        out.append(lines[i])
        i += 1
    return out, protected


def _strip_fences(lines: list[str]) -> list[str]:
    return [
        ln for ln in lines
        if not (_fence_info(ln) is not None and _fence_info(ln) in ("", "sql"))
    ]


def _classify_regions(lines: list[str]) -> list[tuple[str, list[str]]]:
    """Return list of (kind, [lines]). kind in {'prose','sql','protected'}.

    SQL regions, once started, end only at: heading, CJK-prose-start, protected
    marker, or EOF. Blank lines do NOT end a region. `--` comment lines are
    treated as ordinary content for whichever region they fall in (no buffering;
    buffering scrambled the input order across the multi-part corpus).
    """
    regions: list[tuple[str, list[str]]] = []
    state = "prose"   # 'prose' or 'sql'
    buf: list[str] = []

    def flush_prose():
        nonlocal buf
        if buf:
            regions.append(("prose", buf))
            buf = []

    def flush_sql():
        nonlocal buf
        if buf:
            regions.append(("sql", buf))
            buf = []

    for ln in lines:
        if PROTECTED_MARKER_RE.match(ln.strip()):
            if state == "sql":
                flush_sql()
                state = "prose"
            else:
                flush_prose()
            regions.append(("protected", [ln]))
            continue
        if HEADING_RE.match(ln):
            if state == "sql":
                flush_sql()
                state = "prose"
            else:
                flush_prose()
            regions.append(("prose", [ln]))
            continue
        if CJK_RE.match(ln):
            if state == "sql":
                flush_sql()
                state = "prose"
            else:
                flush_prose()
            regions.append(("prose", [ln]))
            continue
        if state == "prose":
            if SQL_START_RE.match(ln):
                flush_prose()
                state = "sql"
                buf = [ln]
                continue
            buf.append(ln)
            continue
        # state == 'sql'
        buf.append(ln)
    # EOF
    if state == "sql":
        flush_sql()
    else:
        flush_prose()
    return regions


def _emit_regions(regions: list[tuple[str, list[str]]], protected: dict[str, list[str]]) -> str:
    parts: list[str] = []
    for kind, lns in regions:
        if kind == "protected":
            for ln in lns:
                parts.extend(protected[ln.strip()])
            parts.append("")
            continue
        if kind == "sql":
            parts.append("```sql")
            parts.extend(lns)
            parts.append("```")
        else:
            parts.extend(lns)
        parts.append("")
    return "\n".join(parts).strip("\n") + "\n"


def rederive_fences(text: str) -> str:
    lines = text.split("\n")
    lifted, protected = _lift_protected(lines)
    stripped = _strip_fences(lifted)
    regions = _classify_regions(stripped)
    return _emit_regions(regions, protected)


# ----- Phase 4: Sections -----

def split_sections(text: str, title: str) -> list[tuple[str, list[str]]]:
    """Split on `^## ` headings. Content before first h2 = preamble section."""
    lines = text.split("\n")
    sections: list[tuple[str, list[str]]] = []
    cur_heading = title
    cur: list[str] = []
    started = False
    for ln in lines:
        m = H2_RE.match(ln)
        if m:
            sections.append((cur_heading, cur))
            cur_heading = m.group(1).strip()
            cur = [ln]
            started = True
            continue
        cur.append(ln)
    sections.append((cur_heading, cur))
    # Trim blank-line edges per section (whitespace; conservation-safe).
    out = []
    for h, lns in sections:
        while lns and not lns[0].strip():
            lns.pop(0)
        while lns and not lns[-1].strip():
            lns.pop()
        if lns:
            out.append((h, lns))
    return out


# ----- Phase 5: Byte budget -----

def _hard_cut_lines(lines: list[str], budget: int) -> list[list[str]]:
    chunks: list[list[str]] = []
    cur: list[str] = []
    cur_bytes = 0
    for ln in lines:
        b = len((ln + "\n").encode("utf-8"))
        if b > budget:
            if cur:
                chunks.append(cur)
                cur, cur_bytes = [], 0
            remaining = ln
            while True:
                enc = remaining.encode("utf-8")
                if len(enc) <= budget:
                    break
                # Rune-boundary slice.
                acc: list[str] = []
                nb = 0
                for ch in remaining:
                    cb = len(ch.encode("utf-8"))
                    if nb + cb > budget:
                        break
                    acc.append(ch)
                    nb += cb
                if not acc:
                    # Defensive: a single rune exceeds budget — emit empty chunk to terminate loop.
                    chunks.append([""])
                    break
                chunks.append(["".join(acc)])
                remaining = remaining[len(acc):]
            if remaining:
                cur = [remaining]
                cur_bytes = len((remaining + "\n").encode("utf-8"))
            continue
        if cur_bytes + b > budget:
            chunks.append(cur)
            cur, cur_bytes = [], 0
        cur.append(ln)
        cur_bytes += b
    if cur:
        chunks.append(cur)
    return chunks


def _split_at_statements(lines: list[str], max_bytes: int, heading: str) -> list[str]:
    """Greedy: accumulate `;`-terminated statement blocks until budget exceeded;
    then start a fresh chunk with `## {heading}` prefix. If the input wraps a
    SQL region (```sql ... ```), each chunk is re-wrapped so secondary pieces
    stay self-contained fenced blocks.
    """
    # Detect + strip a wrapping ```sql ... ``` around the section body so the
    # inner lines can be processed uniformly, then re-wrap each chunk. The
    # section body starts with its own `## {heading}` line (emitted by
    # split_sections) — scan past that to find the opener.
    fence_open: list[str] = []
    fence_close: list[str] = []
    body_lines = list(lines)
    skip = 0
    # Skip the leading h2 heading if present.
    if body_lines and re.match(r"^##\s+", body_lines[0]):
        skip = 1
    # Skip blank lines after the heading.
    while skip < len(body_lines) and not body_lines[skip].strip():
        skip += 1
    if skip < len(body_lines) and body_lines[skip].strip() == "```sql":
        fence_open = ["```sql"]
        # Drop the opener line from the inner body.
        body_lines = body_lines[:skip] + body_lines[skip + 1:]
    # Trim trailing blank lines and any trailing ``` closer.
    while body_lines and not body_lines[-1].strip():
        body_lines.pop()
    if body_lines and body_lines[-1].strip() == "```":
        fence_close = ["```"]
        body_lines = body_lines[:-1]
    while body_lines and not body_lines[-1].strip():
        body_lines.pop()

    prefix = f"## {heading}"
    prefix_bytes = len((prefix + "\n\n").encode("utf-8"))
    # Byte cost of the fence opener and closer lines we'll re-attach per chunk.
    fence_open_extra = len(b"```sql\n") if fence_open else 0
    fence_close_extra = len(b"```\n") if fence_close else 0
    overhead = prefix_bytes + fence_open_extra + fence_close_extra

    blocks: list[list[str]] = []
    curblk: list[str] = []
    for ln in body_lines:
        curblk.append(ln)
        if ln.rstrip().endswith(";"):
            blocks.append(curblk)
            curblk = []
    if curblk:
        blocks.append(curblk)

    chunks: list[list[str]] = []
    cur: list[str] = []
    cur_bytes = overhead
    for blk in blocks:
        blk_text = "\n".join(blk).rstrip("\n") + "\n"
        blk_bytes = len(blk_text.encode("utf-8"))
        # If blk alone exceeds (max_bytes - overhead), hard-cut into multiple pieces.
        if overhead + blk_bytes > max_bytes:
            if cur:
                chunks.append(cur)
                cur, cur_bytes = [], overhead
            for piece in _hard_cut_lines(blk, max_bytes - overhead):
                chunks.append(piece)
            continue
        if cur and cur_bytes + blk_bytes > max_bytes:
            chunks.append(cur)
            cur, cur_bytes = [], overhead
        cur.extend(blk)
        cur_bytes += blk_bytes
    if cur:
        chunks.append(cur)

    bodies: list[str] = []
    for ch in chunks:
        body = prefix + "\n\n"
        if fence_open:
            body += "```sql\n"
        body += "\n".join(ch).strip("\n") + "\n"
        if fence_close:
            body += "```\n"
        bodies.append(body)
    return bodies


def enforce_budget(
    sections: list[tuple[str, list[str]]],
    max_bytes: int,
) -> tuple[list[tuple[str, str, bool]], int]:
    """Return (list of (heading, body_text, secondary), max_body_bytes)."""
    out: list[tuple[str, str, bool]] = []
    max_b = 0
    secondary_count = 0
    for heading, lines in sections:
        body = "\n".join(lines).strip("\n") + "\n"
        b = len(body.encode("utf-8"))
        if b <= max_bytes:
            out.append((heading, body, False))
            max_b = max(max_b, b)
            continue
        pieces = _split_at_statements(lines, max_bytes, heading)
        for piece in pieces:
            pb = len(piece.encode("utf-8"))
            out.append((heading, piece, True))
            max_b = max(max_b, pb)
        secondary_count += 1
    return out, max_b


# ----- Phase 6: Conservation gates -----

def norm(text: str) -> str:
    kept = []
    for ln in text.split("\n"):
        s = ln.strip()
        if s.startswith("```"):
            continue
        kept.append(re.sub(r"\s+", "", ln))
    return "".join(kept)


def _normalized_headings(text: str) -> set[str]:
    s: set[str] = set()
    for ln in text.split("\n"):
        m = HEADING_RE.match(ln)
        if m:
            s.add(m.group(2).strip())
    return s


def _first_diff(a: str, b: str) -> str:
    for i, (x, y) in enumerate(zip(a, b)):
        if x != y:
            ctx_a = a[max(0, i - 20):i + 20]
            ctx_b = b[max(0, i - 20):i + 20]
            return f"len(a)={len(a)} len(b)={len(b)} first_diff@{i} a={ctx_a!r} b={ctx_b!r}"
    if len(a) != len(b):
        return f"length differs: a={len(a)} b={len(b)}"
    return "equal"


def run_gates(group: Group, merged: str, normalized: str,
              primary_sections: list[tuple[str, list[str]]],
              final_sections: list[tuple[str, str, bool]],
              max_bytes: int) -> list[str]:
    failures: list[str] = []
    # Sanctioned H1 dedupe: the only permitted deletion. Mirror merge_group:
    # the FIRST `# {title}` line is kept; subsequent duplicates are dropped.
    target_h1 = "# " + group.title
    seen_h1 = False
    sum_sanctioned_parts: list[str] = []
    for p in group.parts:
        lines = p.body.split("\n")
        kept: list[str] = []
        for ln in lines:
            if ln.strip() == target_h1:
                if seen_h1:
                    continue
                seen_h1 = True
            kept.append(ln)
        sum_sanctioned_parts.append("\n".join(kept).strip("\n"))
    sum_minus_sanctioned = "\n\n".join(sum_sanctioned_parts).strip("\n")

    primary_concat = "\n\n".join("\n".join(ls).strip("\n") for _, ls in primary_sections)

    n1a, n1b = norm(sum_minus_sanctioned), norm(merged)
    if n1a != n1b:
        failures.append(f"G1 { _first_diff(n1a, n1b)}")

    n2a, n2b = norm(merged), norm(normalized)
    if n2a != n2b:
        failures.append(f"G2 { _first_diff(n2a, n2b)}")

    n3a, n3b = norm(normalized), norm(primary_concat)
    if n3a != n3b:
        failures.append(f"G3 { _first_diff(n3a, n3b)}")

    norm_headings = _normalized_headings(normalized)
    for h, _, _ in final_sections:
        if h not in norm_headings:
            failures.append(f"G4 prefixed heading not in normalized body: {h!r}")

    for h, body, _ in final_sections:
        b = len(body.encode("utf-8"))
        if b > max_bytes:
            failures.append(f"G5 section over budget: {h!r} body={b}B > {max_bytes}B")

    return failures


# ----- Phase 7: Write -----

def _slugify(heading: str) -> str:
    bad = set('\\/:*?"<>|#')
    cleaned = "".join("" if ch in bad else ch for ch in heading)
    cleaned = re.sub(r"\s+", "_", cleaned.strip())
    cleaned = cleaned.strip("_.")
    if not cleaned:
        cleaned = "sec"
    # Reserved Windows names -> prefix underscore.
    if cleaned.upper() in WINDOWS_RESERVED:
        cleaned = "_" + cleaned
    # Truncate to 24 runes (preserves a CJK string of <=24 chars).
    return cleaned[:24]


def _fmt_fm(title: str, idx: str, section: str) -> str:
    def esc(s: str) -> str:
        # Quote values containing ':' or starting with special chars.
        if any(c in s for c in ":#-") or s.startswith('"') or s.startswith("'"):
            return '"' + s.replace('"', '\\"') + '"'
        return s
    return (
        "---\n"
        f"title: {esc(title)}\n"
        f"source_idx: {esc(idx)}\n"
        f"section: {esc(section)}\n"
        "---\n\n"
    )


# ----- Glue: run a single group end-to-end (no write) -----

def process_group(group: Group, max_bytes: int, verbose: bool, stats: dict) -> dict:
    """Return dict with keys: ok, errors, merged, normalized, primary_sections,
    final_sections, h1_drops, max_body_bytes.
    """
    errors: list[str] = []
    # M-mismatch check.
    ms = {p.split_m for p in group.parts if p.split_m}
    if ms:
        M = max(ms)
        present = sorted(p.num for p in group.parts)
        missing = [i for i in range(1, M + 1) if i not in present]
        if len(present) != M or missing:
            errors.append(
                f"M-mismatch: found {len(present)}/{M} parts (missing {missing})"
            )
    # Duplicate part numbers.
    nums = [p.num for p in group.parts]
    if len(set(nums)) != len(nums):
        dups = sorted({n for n in nums if nums.count(n) > 1})
        errors.append(f"duplicate part numbers: {dups}")
    # Title consistency across parts.
    titles = {p.title for p in group.parts if p.title}
    if len(titles) > 1:
        errors.append(f"inconsistent titles in group: {titles}")

    if errors:
        return {"ok": False, "errors": errors}

    # Phase 2: merge.
    merged, h1_drops = merge_group(group, stats)
    # Phase 3a: glue on per-part bodies (attribution), then concat.
    labeled_lines: list[tuple[str, str]] = []
    for p in group.parts:
        rel = p.path.relative_to(p.path.parents[len(p.path.parents) - len(group.dirpath.parts) - 1] if False else p.path).as_posix()
        # Use the path as a label, relative-form best-effort.
        labeled_lines.append((p.path.name, p.body))
    # Apply glue per part, attributed by filename + line offset.
    glued_parts: list[str] = []
    for fname, body in labeled_lines:
        body_lines = body.split("\n")
        glued = glue_normalize_lines(body_lines, fname, verbose, stats)
        glued_parts.append("\n".join(glued).strip("\n"))
    glued_merged = "\n\n".join(glued_parts).strip("\n")
    # Phase 3b: fence re-derivation.
    normalized = rederive_fences(glued_merged)

    # Phase 4: sections (use normalized text).
    primary_sections = split_sections(normalized, group.title)

    # Phase 5: byte budget.
    final_sections, max_b = enforce_budget(primary_sections, max_bytes)

    # Phase 6: gates.
    failures = run_gates(group, glued_merged, normalized,
                         primary_sections, final_sections, max_bytes)
    if failures:
        errors.extend(failures)

    return {
        "ok": not errors,
        "errors": errors,
        "merged": merged,
        "normalized": normalized,
        "primary_sections": primary_sections,
        "final_sections": final_sections,
        "h1_drops": h1_drops,
        "max_body_bytes": max_b,
    }


# ----- Top-level -----

def _print_plan(group: Group, proc: dict, max_bytes: int, dry: bool, file=sys.stdout):
    final = proc.get("final_sections", [])
    if not final:
        print(f"[group] dir={group.dirpath.as_posix()} idx={group.idx} "
              f"title={group.title!r} parts={len(group.parts)} sections=0 (FAILED: {proc.get('errors')})",
              file=file)
        return
    print(f"[group] dir={group.dirpath.as_posix()} idx={group.idx} "
          f"title={group.title!r} parts={len(group.parts)} sections={len(final)} "
          f"max_body_bytes={proc['max_body_bytes']} h1_drops={proc['h1_drops']}",
          file=file)
    stem = group.stem
    for i, (h, body, sec) in enumerate(final, start=1):
        slug = _slugify(h)
        out_name = f"{stem}__sec{i:02d}_{slug}.md"
        body_bytes = len(body.encode("utf-8"))
        marker = " [secondary-split]" if sec else ""
        verb = "would-write" if dry else "write"
        print(f"  {verb}: {out_name} bytes={body_bytes} heading={h!r}{marker}", file=file)
    if dry:
        for p in group.parts:
            print(f"  would-delete: {p.path.as_posix()}", file=file)


def _write_group(group: Group, proc: dict, max_bytes: int) -> tuple[list[Path], list[Path]]:
    """Write the group's sec files; delete stale sec files matching stem prefix.
    Returns (written, stale_deleted).
    """
    final = proc["final_sections"]
    stem = group.stem
    written: list[Path] = []
    stale = sorted(group.dirpath.glob(f"{stem}__sec*.md"))
    stale_paths = list(stale)
    for i, (h, body, _) in enumerate(final, start=1):
        slug = _slugify(h)
        out_name = f"{stem}__sec{i:02d}_{slug}.md"
        out_path = group.dirpath / out_name
        out_path.write_text(_fmt_fm(group.title, group.idx, h) + body, encoding="utf-8")
        written.append(out_path)
        if out_path in stale_paths:
            stale_paths.remove(out_path)
    for s in stale_paths:
        s.unlink()
    return written, stale_paths


def _delete_parts(parts: list[Part]) -> list[Path]:
    deleted: list[Path] = []
    for p in parts:
        if p.path.exists():
            p.path.unlink()
            deleted.append(p.path)
    return deleted


def run(argv: Optional[list[str]] = None) -> int:
    parser = argparse.ArgumentParser(
        description="Reorganize __partNN.md corpus files into semantic __secNN_slug.md sections.",
    )
    parser.add_argument("--corpus", default=DEFAULT_CORPUS,
                        help=f"corpus root directory (default {DEFAULT_CORPUS})")
    parser.add_argument("--dry-run", action="store_true",
                        help="print the per-group plan; write/delete nothing")
    parser.add_argument("--only", default="",
                        help="comma-separated source_idx values to limit processing (pilot mode)")
    parser.add_argument("--max-bytes", type=int, default=DEFAULT_MAX_BYTES,
                        help=f"per-section body byte budget (default {DEFAULT_MAX_BYTES}; "
                             f"mirrors internal/knowledge/indexer.go maxDocBytes)")
    parser.add_argument("--verbose", action="store_true",
                        help="verbose per-rule logs to stderr")
    args = parser.parse_args(argv)

    corpus = Path(args.corpus)
    if not corpus.is_dir():
        print(f"ERROR: --corpus path not a directory: {corpus}", file=sys.stderr)
        return 1

    selected_idxs: set[int] = set()
    if args.only:
        for tok in args.only.split(","):
            tok = tok.strip()
            if not tok:
                continue
            if not tok.isdigit():
                print(f"ERROR: --only value must be numeric, got {tok!r}", file=sys.stderr)
                return 1
            selected_idxs.add(int(tok))

    all_md, parts, groups = discover(corpus)

    if not parts:
        print("nothing to do (zero part files in corpus)", file=sys.stderr)
        return 0

    if selected_idxs:
        # Under --only, non-selected groups are left fully untouched (their
        # parts must remain on disk). Without --only, process everything.
        if all(g.idx.isdigit() for g in groups):
            groups = [g for g in groups if int(g.idx) in selected_idxs]
            parts = [p for p in parts if p.idx.isdigit() and int(p.idx) in selected_idxs]
        else:
            groups = [g for g in groups if g.idx in selected_idxs or g.idx in {str(i) for i in selected_idxs}]
        if not parts:
            print(f"nothing to do (--only filter selected no parts in {corpus})",
                  file=sys.stderr)
            return 0

    stats = {
        "glue_hits": {},
        "groups_processed": 0,
        "sections_written": 0,
        "sections_secondary_split": 0,
        "max_section_body_bytes": 0,
        "h1_drops": 0,
        "part_files_deleted": 0,
        "whole_notes_untouched": len(all_md) - len(parts),
    }

    processed_groups: list[tuple[Group, dict]] = []
    all_errors: list[tuple[str, str]] = []

    for g in groups:
        if selected_idxs and g.idx.isdigit() and int(g.idx) not in selected_idxs:
            continue
        proc = process_group(g, args.max_bytes, args.verbose, stats)
        _print_plan(g, proc, args.max_bytes, args.dry_run)
        if not proc["ok"]:
            all_errors.append((f"{g.dirpath.as_posix()}/idx={g.idx}", "; ".join(proc["errors"])))
            continue
        processed_groups.append((g, proc))
        stats["groups_processed"] += 1
        stats["sections_written"] += len(proc["final_sections"])
        stats["sections_secondary_split"] += sum(1 for _, _, s in proc["final_sections"] if s)
        stats["max_section_body_bytes"] = max(stats["max_section_body_bytes"], proc["max_body_bytes"])
        stats["h1_drops"] += proc["h1_drops"]

    if all_errors:
        print("\nFATAL: validation/conservation failures (nothing written):", file=sys.stderr)
        for where, err in all_errors:
            print(f"  [{where}] {err}", file=sys.stderr)
        return 2

    if args.dry_run:
        print(f"\n[dry-run] would process {stats['groups_processed']} groups -> "
              f"{stats['sections_written']} sections; would delete {len(parts)} part files",
              file=sys.stdout)
        return 0

    # Phase 7: write.
    all_written: list[Path] = []
    for g, proc in processed_groups:
        written, stale = _write_group(g, proc, args.max_bytes)
        all_written.extend(written)
        for s in stale:
            print(f"  stale-deleted: {s.as_posix()}", file=sys.stderr)
    all_deleted = _delete_parts(parts)
    stats["part_files_deleted"] = len(all_deleted)

    print()
    print(f"STATS parts_consumed={len(parts)} groups={stats['groups_processed']} "
          f"sections_written={stats['sections_written']} "
          f"sections_secondary_split={stats['sections_secondary_split']} "
          f"max_section_body_bytes={stats['max_section_body_bytes']} "
          f"h1_drops={stats['h1_drops']} part_files_deleted={len(all_deleted)} "
          f"whole_notes_untouched={stats['whole_notes_untouched']}",
          file=sys.stdout)
    return 0


if __name__ == "__main__":
    sys.exit(run())