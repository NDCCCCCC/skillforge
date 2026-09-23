// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package ecl

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// secRegex captures the secNN prefix in a sql-guides fragment filename.
// Format: <title>__secNN_<section-slug>.md
// e.g. "001_公司业务部…__sec03_SQL_正文.md"
//
// Verbatim port of spike 005c main.go:57 (D-ECL-02 byte-for-byte fidelity).
var secRegex = regexp.MustCompile(`__sec(\d{2})_(.+)\.md$`)

// fmFields mirrors the frontmatter fields the chunker consumes
// (spike parseFrontmatter semantics, yaml.v3). yaml.v3 lowercases field
// names for matching, so Title/SourceIdx/Section bind to title/source_idx/
// section — proven on CJK section names by the spike.
type fmFields struct {
	Title     string `yaml:"title"`
	SourceIdx string `yaml:"source_idx"`
	Section   string `yaml:"section"`
}

// IsSecFragment reports whether a file is an ECL sql-guides fragment.
// Both conditions are REQUIRED:
//  1. the base file name matches the __secNN fragment pattern, AND
//  2. the frontmatter block decodes to a non-empty `section` key.
//
// Non-fragment corpus files (e.g. 取数口径/030_承保清单.md — has
// title+source_idx but no section key; or 024_提数范围….md — no __secNN in
// the name) must NOT route to the ECL path: they stay on the legacy indexer
// logic unchanged (D-ECL-01). IsSecFragment is the exact predicate Plan 06
// wires into internal/knowledge/indexer.go's per-file loop.
//
// It is deliberately cheap: no file reads, inputs only.
func IsSecFragment(baseFileName string, frontmatter []byte) bool {
	if !secRegex.MatchString(baseFileName) {
		return false
	}
	if len(frontmatter) == 0 {
		return false
	}
	var fm fmFields
	if err := yaml.Unmarshal(frontmatter, &fm); err != nil {
		return false
	}
	return strings.TrimSpace(fm.Section) != ""
}

// walkMarkdown discovers all .md files under kbDir and returns their paths
// sorted in lexical order. Hidden directories (name starts with '.') are
// skipped — they are never KB content. Mirrors internal/knowledge
// indexer.go walkTier (WalkDir + hidden-dir skip + sort discipline) so the
// ECL walk descends KB subdirectories identically to the legacy loader.
func walkMarkdown(kbDir string) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(kbDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != kbDir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		matches = append(matches, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

// splitFrontmatterRaw splits CRLF-normalized file content into the YAML
// frontmatter lines and the body, using spike 005c parseFrontmatter's
// line-scan semantics verbatim: the first trimmed "---" line opens the
// block, the next closes it, everything after is body. The body is
// TrimSpace'd exactly as the spike does so merged section bodies and
// BodyBytes reproduce the spike's validated output.
//
// Callers MUST CRLF-normalize before calling (bytes.ReplaceAll \r\n → \n)
// — Windows-edited corpus files must parse (Pitfall 7), mirroring
// indexer.go splitFrontmatter's normalization discipline.
func splitFrontmatterRaw(raw string) (fm string, body string) {
	lines := strings.Split(raw, "\n")
	var fmLines []string
	var bodyLines []string
	var inFM bool
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "---" {
			if !inFM {
				inFM = true
				continue
			}
			// end of frontmatter, rest is body
			inFM = false
			continue
		}
		if inFM {
			fmLines = append(fmLines, ln)
		} else {
			bodyLines = append(bodyLines, ln)
		}
	}
	return strings.Join(fmLines, "\n"), strings.TrimSpace(strings.Join(bodyLines, "\n"))
}

// WalkFragments walks kbDir and returns every ECL fragment (files passing
// IsSecFragment) with frontmatter fields and body parsed, ordered by the
// walker's lexical path order. An unreadable file or unparseable
// frontmatter is an ERROR, never a silently skipped file (T-39-02:
// parse errors must surface, mirroring indexer.go's read-error posture).
func WalkFragments(kbDir string) ([]Fragment, error) {
	paths, err := walkMarkdown(kbDir)
	if err != nil {
		return nil, fmt.Errorf("ecl: walk %s: %w", kbDir, err)
	}
	var frags []Fragment
	for _, path := range paths {
		base := filepath.Base(path)
		// Cheap filename pre-filter — the first half of IsSecFragment.
		if !secRegex.MatchString(base) {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("ecl: read %s: %w", path, err)
		}
		// CRLF → LF before the frontmatter split (Pitfall 7).
		raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
		fmRaw, body := splitFrontmatterRaw(string(raw))
		if !IsSecFragment(base, []byte(fmRaw)) {
			continue
		}
		var fm fmFields
		if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
			return nil, fmt.Errorf("ecl: %s: parse frontmatter: %w", path, err)
		}
		m := secRegex.FindStringSubmatch(base)
		secOrder, _ := strconv.Atoi(m[1])
		frags = append(frags, Fragment{
			Path:      path,
			SecOrder:  secOrder,
			Title:     fm.Title,
			SourceIdx: fm.SourceIdx,
			Section:   fm.Section,
			Body:      body,
		})
	}
	return frags, nil
}

// MergeFragments groups fragments by source_idx and reassembles each
// logical note: parts stable-sorted by SecOrder, same-section bodies
// concatenated in first-seen order (appending "\n" after each non-blank
// part — spike-verbatim, so line-anchored downstream regexes always see
// statement starts), SectionIdx assigned by first-seen order, notes sorted
// by numeric source_idx. Verbatim port of spike 005c mergeByNote
// (main.go:155-199) per D-ECL-02; only sort.Slice → sort.SliceStable
// (the plan pins a stable sort; with pre-sorted paths the output is
// identical on the validated corpus).
func MergeFragments(chunks []Fragment) []Note {
	bySrc := map[string][]Fragment{}
	for _, c := range chunks {
		bySrc[c.SourceIdx] = append(bySrc[c.SourceIdx], c)
	}
	notes := []Note{}
	for src, parts := range bySrc {
		// Stable sort: SecOrder ascending.
		sort.SliceStable(parts, func(i, j int) bool { return parts[i].SecOrder < parts[j].SecOrder })
		// Merge by Section name in first-seen order.
		bySection := map[string]*MergedSection{}
		var order []string
		for _, p := range parts {
			ms, ok := bySection[p.Section]
			if !ok {
				ms = &MergedSection{Section: p.Section}
				bySection[p.Section] = ms
				order = append(order, p.Section)
			}
			// Append body with a trailing newline if non-empty.
			if strings.TrimSpace(p.Body) != "" {
				ms.Body += p.Body + "\n"
			}
			ms.SourcePaths = append(ms.SourcePaths, p.Path)
		}
		secs := []MergedSection{}
		for idx, name := range order {
			ms := bySection[name]
			ms.SectionIdx = idx
			secs = append(secs, *ms)
		}
		title := ""
		if len(parts) > 0 {
			title = parts[0].Title
		}
		notes = append(notes, Note{SourceIdx: src, Title: title, Sections: secs})
	}
	// Stable sort notes by source_idx numeric.
	sort.SliceStable(notes, func(i, j int) bool {
		a, _ := strconv.Atoi(notes[i].SourceIdx)
		b, _ := strconv.Atoi(notes[j].SourceIdx)
		return a < b
	})
	return notes
}

// ChunkMetas builds one ChunkMeta record per merged section — the D-ECL-02
// contract shape (spike chunkMeta, main.go:274-284). BodyBytes is the byte
// length of the merged section body.
func ChunkMetas(notes []Note) []ChunkMeta {
	out := []ChunkMeta{}
	for _, n := range notes {
		for _, s := range n.Sections {
			out = append(out, ChunkMeta{
				SourceIdx:   n.SourceIdx,
				Title:       n.Title,
				Section:     s.Section,
				SectionIdx:  s.SectionIdx,
				Sections:    len(n.Sections),
				SourcePaths: s.SourcePaths,
				BodyBytes:   len(s.Body),
			})
		}
	}
	return out
}

// WalkAndMerge walks the sql-guides KB directory, reassembles the
// __secNN fragments into logical notes, and returns the notes together
// with their per-section ChunkMeta records (the composition the ECL
// pipeline consumes; Plans 04/05/06 build on this output).
func WalkAndMerge(kbDir string) ([]Note, []ChunkMeta, error) {
	frags, err := WalkFragments(kbDir)
	if err != nil {
		return nil, nil, err
	}
	notes := MergeFragments(frags)

	// Tier is the KB subdirectory name (e.g. "sql-guides") — derive from
	// kbDir by taking the last path component. This is stable and matches
	// the retriever filter value (D-TIER-02).
	tier := filepath.Base(kbDir)
	for i := range notes {
		notes[i].Tier = tier
	}

	return notes, ChunkMetas(notes), nil
}
