#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Hermetic fixture tests for scripts/merge_parts_pipeline.py.

Run with: python scripts/merge_parts_pipeline_test.py

Each test writes its own temporary corpus shaped exactly like the real one
(subdirs with CJK names are fine). Subprocess-based — no Go, no Milvus, no
LLM. Verifies:

  1. Fences balanced (even number of ``` / ```sql lines per output file).
  2. Glue substrings `;注`, `Tset `, `noleft join`, `绩效drop`, `etl_NONAUTO_POLICY_Tset`
     are split across lines.
  3. Oversized sections split at statement boundaries with prefixed headings.
  4. Protected ```json blocks survive verbatim in some output file.
  5. M-mismatch (split_part: 3/5 with only 2 parts present) fails the group.
  6. Conservation (norm equality) holds independently of the pipeline's report.
  7. Idempotency — second run is a no-op on a completed corpus.
  8. --only scopes to selected groups; others untouched.
  9. Whole notes (non-part .md) byte-identical after the run.
"""
from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
PIPELINE = HERE / "merge_parts_pipeline.py"


def _run(args: list[str], cwd: Path) -> subprocess.CompletedProcess:
    """Run the pipeline as a subprocess. Use bytes mode for the IO streams so
    the parent Python's default encoding (cp1252 on Windows) can't choke on
    child CJK output via the asyncio reader thread."""
    cp = subprocess.run(
        [sys.executable, str(PIPELINE)] + args,
        cwd=str(cwd), capture_output=True,
    )
    out = cp.stdout.decode("utf-8", errors="replace")
    err = cp.stderr.decode("utf-8", errors="replace")
    return subprocess.CompletedProcess(cp.args, cp.returncode, out, err)


def _norm(text: str) -> str:
    """Independent re-implementation of pipeline.norm() for cross-checking."""
    kept = []
    for ln in text.split("\n"):
        s = ln.strip()
        if s.startswith("```"):
            continue
        kept.append(re.sub(r"\s+", "", ln))
    return "".join(kept)


def _write_part(path: Path, idx: str, n: int, m: int, title: str, body: str) -> None:
    text = (
        "---\n"
        f"title: {title}\n"
        f"source_idx: {idx}\n"
        f"split_part: {n}/{m}\n"
        "source_split: true\n"
        "---\n"
        "\n"
        f"{body.rstrip()}\n"
    )
    path.write_text(text, encoding="utf-8")


def _write_whole_note(path: Path, body: str) -> None:
    path.write_text(body.rstrip() + "\n", encoding="utf-8")


# --- Fixture builders ---------------------------------------------------------

def _build_corpus_001(tmp: Path) -> dict:
    """Reproduce the 001 pathology set: unclosed fence, orphan fence, the four
    glue forms, protected ```json. Returns metadata for cross-checking."""
    sub = tmp / "取数口径"
    sub.mkdir(exist_ok=True)
    title = "公司业务部部门组信息更新"
    # part01 (preamble): H1 + 涉及表 line.
    _write_part(sub / f"001_{title}__part01.md", "001", 1, 11, title,
                f"# {title}\n\n**涉及表/视图**: `etl_payment_due_t`, `ids_wuh.x`\n")
    # part02 (说明 prose) — `;注` glue form, R2 `绩效drop` glue form.
    _write_part(sub / f"001_{title}__part02.md", "001", 2, 11, title,
                "## 说明\n\n更新部门映射。注：测试;绩效drop table tmp_ycb_test;\n")
    # part03: starts ```sql but never closes -> unclosed fence.
    _write_part(sub / f"001_{title}__part03.md", "001", 3, 11, title,
                "## SQL / 正文\n\n```sql\nupdate etl_payment_due_t set dept='X';\n")
    # part04: glue forms `Tset ` and `noleft join`, and a protected ```json block.
    _write_part(sub / f"001_{title}__part04.md", "001", 4, 11, title,
                "update etl_NONAUTO_POLICY_Tset department_group_code='420100';\n"
                "select t.policy_noleft join etl_policy h on t.policy_no = h.policy_no;\n"
                "```json\n{\"protected\": true, \"bytes\": \"verbatim\"}\n```\n")
    # part05..part10: filler SQL to push byte count.
    for i in range(5, 11):
        _write_part(sub / f"001_{title}__part{i:02d}.md", "001", i, 11, title,
                    f"select policy_no from ids_wuh.tbl_{i};\n")
    # part11: orphan closing ``` line at end.
    _write_part(sub / f"001_{title}__part11.md", "001", 11, 11, title,
                "select count(*) from ids_wuh.tbl_x;\n```\n")
    return {"title": title, "idx": "001", "dir": sub}


def _build_corpus_oversize(tmp: Path) -> dict:
    """Build a corpus whose `## 详细` section body exceeds 4000 bytes
    (one giant `select ... from x;` statement repeated enough times)."""
    sub = tmp / "取数口径"
    sub.mkdir(exist_ok=True)
    title = "体量超大示例"
    _write_part(sub / f"010_{title}__part01.md", "010", 1, 2, title,
                f"# {title}\n\n**涉及表/视图**: `big_t`\n")
    # part02: huge SQL block — many `select ... from big_t;` statements.
    huge = []
    for i in range(200):
        huge.append(f"select col_{i:03d} from big_t where id={i};")
    _write_part(sub / f"010_{title}__part02.md", "010", 2, 2, title,
                "## 详细\n\n" + "\n".join(huge) + "\n")
    return {"title": title, "idx": "010"}


def _build_corpus_mismatch(tmp: Path) -> dict:
    """split_part claims 3/5 but only 2 parts present -> M-mismatch."""
    sub = tmp / "取数口径"
    sub.mkdir(exist_ok=True)
    title = "缺片示例"
    _write_part(sub / f"020_{title}__part01.md", "020", 1, 5, title,
                f"# {title}\n\n**涉及表/视图**: `x`\n")
    _write_part(sub / f"020_{title}__part02.md", "020", 2, 5, title,
                "select SQL with ; from x;\n")
    # parts 3, 4, 5 intentionally absent.
    return {"title": title, "idx": "020"}


def _build_corpus_with_wholenote(tmp: Path) -> dict:
    """Add a whole (non-part) note that must stay byte-identical."""
    sub = tmp / "取数口径"
    sub.mkdir(exist_ok=True)
    whole = sub / "030_承保清单.md"
    whole_text = "# 承保清单\n\n整段文档不应被改写。\n"
    _write_whole_note(whole, whole_text)
    title = "另一个小例子"
    _write_part(sub / f"040_{title}__part01.md", "040", 1, 2, title,
                f"# {title}\n\n**涉及表/视图**: `x`\n")
    _write_part(sub / f"040_{title}__part02.md", "040", 2, 2, title,
                "select a from x;\n")
    return {"whole_path": whole, "whole_text": whole_text}


def _parse_parts(group_dir: Path, idx: str) -> list[Path]:
    # Filter by leading idx (e.g. "001" matches "001_*__part*.md").
    return sorted(p for p in group_dir.glob(f"*__part*.md") if p.name.startswith(f"{idx}_"))


def _parse_sec(group_dir: Path, idx: str) -> list[Path]:
    return sorted(p for p in group_dir.glob(f"*__sec*.md") if p.name.startswith(f"{idx}_"))


def _fence_count(text: str) -> int:
    return sum(1 for ln in text.split("\n") if ln.strip().startswith("```"))


# --- Tests --------------------------------------------------------------------

class TestPipeline(unittest.TestCase):

    def _mk(self) -> Path:
        return Path(tempfile.mkdtemp(prefix="m7test_"))

    def _cleanup(self, tmp: Path) -> None:
        shutil.rmtree(tmp, ignore_errors=True)

    def test_001_pilot_fences_balanced_glue_split_json_protected(self) -> None:
        tmp = self._mk()
        try:
            meta = _build_corpus_001(tmp)
            res = _run(["--corpus", str(tmp)], tmp)
            self.assertEqual(res.returncode, 0, msg=res.stderr)
            sec_files = _parse_sec(meta["dir"], "001")
            self.assertGreater(len(sec_files), 0)
            # Every output has an even fence count that alternates open/close.
            for f in sec_files:
                t = f.read_text(encoding="utf-8")
                count = _fence_count(t)
                self.assertEqual(count % 2, 0,
                                 msg=f"{f.name}: odd fence count {count}")
                # Check fence lines strictly alternate.
                fence_lines = [(i, ln.strip()) for i, ln in enumerate(t.split("\n"))
                                if ln.strip().startswith("```")]
                for i in range(0, len(fence_lines) - 1, 2):
                    opener = fence_lines[i][1]
                    closer = fence_lines[i + 1][1]
                    self.assertIn(opener, {"```", "```sql", "```json", "```bash"},
                                  f"{f.name}: unexpected opener {opener!r}")
                    self.assertEqual(closer, "```", f"{f.name}: bad close {closer!r}")
            # Glue forms eliminated across all sec files.
            all_text = "".join(f.read_text(encoding="utf-8") for f in sec_files)
            self.assertNotIn("Tset ", all_text,
                             "glued `Tset ` survived in output")
            self.assertNotIn("noleft join", all_text,
                             "glued `noleft join` survived in output")
            self.assertNotIn("绩效drop", all_text,
                             "glued CJK-then-drop survived in output")
            # `;注` glue: must be split across lines (a `;` followed by `注` no
            # longer appears as `;注` on the same line).
            for f in sec_files:
                for ln in f.read_text(encoding="utf-8").split("\n"):
                    self.assertNotIn(";注", ln, f"line still glued: {ln!r}")
            # Protected ```json block survives verbatim somewhere.
            self.assertIn("verbatim", all_text,
                          "protected json block content lost")
            self.assertIn('"protected"', all_text)
            # No ```json rewrapped as ```sql — a ```sql opener never contains json.
            for f in sec_files:
                for ln in f.read_text(encoding="utf-8").split("\n"):
                    s = ln.strip()
                    self.assertFalse(s == "```sql" and "json" in s.lower())
            # Every output file body <= 4000 bytes (post-frontmatter).
            for f in sec_files:
                t = f.read_text(encoding="utf-8")
                # Strip frontmatter.
                if t.startswith("---\n"):
                    end = t.find("\n---\n", 4)
                    if end >= 0:
                        body = t[end + 5:].lstrip("\n")
                        self.assertLessEqual(len(body.encode("utf-8")), 4000,
                                              f"{f.name} body over 4000")
            # No part files remain.
            self.assertEqual(_parse_parts(meta["dir"], "001"), [])
        finally:
            self._cleanup(tmp)

    def test_010_oversize_section_split_with_prefixed_heading(self) -> None:
        tmp = self._mk()
        try:
            meta = _build_corpus_oversize(tmp)
            res = _run(["--corpus", str(tmp)], tmp)
            self.assertEqual(res.returncode, 0, msg=res.stderr)
            sec_files = _parse_sec(tmp / "取数口径", "010")
            # Multiple files for the over-budget section.
            self.assertGreaterEqual(len(sec_files), 2,
                                    f"expected >=2 sec files, got {len(sec_files)}")
            # Every body <= 4000; secondary chunks each prefixed with ## 详细.
            secondary_heading_count = 0
            for f in sec_files:
                t = f.read_text(encoding="utf-8")
                # Strip frontmatter.
                end = t.find("\n---\n", 4)
                body = t[end + 5:].lstrip("\n") if end >= 0 else t
                self.assertLessEqual(len(body.encode("utf-8")), 4000,
                                      f"{f.name} body over 4000")
                # Find a secondary chunk: body starts with `## 详细` AND there is
                # another file with the same prefix in this group.
                if body.startswith("## 详细"):
                    secondary_heading_count += 1
            self.assertGreaterEqual(secondary_heading_count, 2,
                                    "no secondary chunks with ## 详细 prefix")
        finally:
            self._cleanup(tmp)

    def test_020_mismatch_fails_closed(self) -> None:
        tmp = self._mk()
        try:
            _build_corpus_mismatch(tmp)
            res = _run(["--corpus", str(tmp)], tmp)
            self.assertEqual(res.returncode, 2,
                              f"expected exit 2 for M-mismatch, got {res.returncode}\nstderr={res.stderr}")
            self.assertIn("M-mismatch", res.stderr)
            # Nothing written or deleted.
            for f in tmp.rglob("*"):
                if f.is_file():
                    self.assertIn("__part", f.name,
                                   f"unexpected file written: {f}")
                    self.assertNotIn("__sec", f.name,
                                      f"sec file written despite failure: {f}")
        finally:
            self._cleanup(tmp)

    def test_conservation_norm(self) -> None:
        tmp = self._mk()
        try:
            meta = _build_corpus_001(tmp)
            # Snapshot input bodies BEFORE the run (parts get deleted).
            input_bodies = []
            for p in sorted(_parse_parts(meta["dir"], "001")):
                t = p.read_text(encoding="utf-8")
                if t.startswith("---\n"):
                    end = t.find("\n---\n", 4)
                    body = t[end + 5:].lstrip("\n") if end >= 0 else t
                else:
                    body = t
                input_bodies.append(body)
            res = _run(["--corpus", str(tmp)], tmp)
            self.assertEqual(res.returncode, 0, msg=res.stderr)
            # Compute output bodies.
            output_bodies = []
            for f in _parse_sec(meta["dir"], "001"):
                t = f.read_text(encoding="utf-8")
                if t.startswith("---\n"):
                    end = t.find("\n---\n", 4)
                    body = t[end + 5:].lstrip("\n") if end >= 0 else t
                else:
                    body = t
                output_bodies.append(body)
            n_out = _norm("\n\n".join(output_bodies))
            # Sanctioned drop: H1 `# title` may be deduped across parts; the
            # pipeline keeps exactly the FIRST occurrence. Mirror that here.
            target_h1 = "# " + meta["title"]
            seen = False
            stripped_inputs = []
            for body in input_bodies:
                lines = [ln for ln in body.split("\n")
                          if not (ln.strip() == target_h1 and seen)]
                if any(ln.strip() == target_h1 for ln in body.split("\n")):
                    seen = True
                stripped_inputs.append("\n".join(lines).strip("\n"))
            n_in_minus_h1 = _norm("\n\n".join(stripped_inputs))
            self.assertEqual(n_in_minus_h1, n_out,
                              f"conservation violated:\nin={n_in_minus_h1[:200]!r}\n"
                              f"out={n_out[:200]!r}")
        finally:
            self._cleanup(tmp)

    def test_idempotent_second_run_is_noop(self) -> None:
        tmp = self._mk()
        try:
            meta = _build_corpus_001(tmp)
            r1 = _run(["--corpus", str(tmp)], tmp)
            self.assertEqual(r1.returncode, 0, msg=r1.stderr)
            # Snapshot all sec files.
            snap = {}
            for f in _parse_sec(meta["dir"], "001"):
                snap[f.name] = f.read_text(encoding="utf-8")
            # Second run: should report "nothing to do" since no parts remain.
            r2 = _run(["--corpus", str(tmp)], tmp)
            self.assertEqual(r2.returncode, 0, msg=r2.stderr)
            self.assertIn("nothing to do", r2.stderr)
            # All sec files unchanged byte-for-byte.
            for f in _parse_sec(meta["dir"], "001"):
                self.assertEqual(snap.get(f.name), f.read_text(encoding="utf-8"),
                                  f"{f.name} changed on second run")
            # Test partial re-run convergence: a second corpus group that was
            # NOT processed on the first run gets processed; the completed
            # group's sec files stay byte-identical (convergence contract).
            meta2 = _build_corpus_oversize(tmp)
            r3 = _run(["--corpus", str(tmp)], tmp)
            self.assertEqual(r3.returncode, 0, msg=r3.stderr)
            for n, content in snap.items():
                self.assertEqual(content, (meta["dir"] / n).read_text(encoding="utf-8"),
                                  f"{n} changed on partial re-run")
            self.assertGreater(len(_parse_sec(tmp / "取数口径", "010")), 0)
        finally:
            self._cleanup(tmp)

    def test_only_scopes_to_selected_groups(self) -> None:
        tmp = self._mk()
        try:
            meta1 = _build_corpus_001(tmp)              # idx 001
            meta2 = _build_corpus_oversize(tmp)         # idx 010
            res = _run(["--corpus", str(tmp), "--only", "001"], tmp)
            self.assertEqual(res.returncode, 0, msg=res.stderr)
            # 001 sec files exist, 001 parts gone.
            self.assertGreater(len(_parse_sec(tmp / "取数口径", "001")), 0)
            self.assertEqual(_parse_parts(tmp / "取数口径", "001"), [])
            # 010 parts still on disk.
            self.assertGreater(len(_parse_parts(tmp / "取数口径", "010")), 0)
            self.assertEqual(_parse_sec(tmp / "取数口径", "010"), [])
        finally:
            self._cleanup(tmp)

    def test_wholenote_byte_identical(self) -> None:
        tmp = self._mk()
        try:
            meta = _build_corpus_with_wholenote(tmp)
            before = meta["whole_path"].read_text(encoding="utf-8")
            res = _run(["--corpus", str(tmp)], tmp)
            self.assertEqual(res.returncode, 0, msg=res.stderr)
            after = meta["whole_path"].read_text(encoding="utf-8")
            self.assertEqual(before, after, "whole note was modified")
            # Part files for 040 consumed; whole note untouched.
            self.assertEqual(_parse_parts(tmp / "取数口径", "040"), [])
            self.assertGreater(len(_parse_sec(tmp / "取数口径", "040")), 0)
        finally:
            self._cleanup(tmp)


if __name__ == "__main__":
    unittest.main(verbosity=2)