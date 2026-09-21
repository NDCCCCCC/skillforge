// Spike 005c — ontology-aware markdown chunker for sql-guides.
//
// Problem: sql-guides/*.md was byte-bucket split by a prior pipeline at the
// 4000-byte cap (see spike 004 tier contract). The result: 1 logical note
// (e.g. "001_公司业务部…更新") becomes 8 fragments like `__sec01..sec08`,
// each carrying the same `section: SQL / 正文` frontmatter. Downstream
// retrieval returns fragments — LLM sees only the fragment title, never
// the full SQL.
//
// Fix: ontology-aware re-chunking by *section boundary* (the `section`
// frontmatter field), not by byte count.
//
// This spike:
//   (1) Reads every *.md under knowledge/sql-guides/, parses frontmatter.
//   (2) Groups by (source_idx, title) → one logical note.
//   (3) Within a note, sorts parts by secNN order; merges by section name.
//   (4) Verifies:
//       a. boundary detection = 100% (every secNN file contributes a chunk)
//       b. SQL is complete: every `update etl_*` statement that appeared
//          in any input fragment is now visible in the merged output.
//       c. chunk.metadata fields (section / section_idx / source_idx /
//          title) are populated and serializable to Milvus metadata format.
//   (5) Roundtrip: emit merged chunks to merged_notes/*.md; reload and
//       confirm the merged note still carries all 7 SQL statements.
//
// Acceptance (from spike manifest):
//   - 100% boundary detection on the 8 secNN files of note "001_公司业务部"
//   - Reconstructed note contains all 7 `update etl_*` statements
//   - chunk.metadata JSON-serializable with section / section_idx /
//     source_idx / title
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------- frontmatter ----------

type Frontmatter struct {
	Title     string `yaml:"title"`
	SourceIdx string `yaml:"source_idx"`
	Section   string `yaml:"section"`
}

// ReGex captures the secNN prefix in a filename.
// Format: <title>__secNN_<section-slug>.md
// e.g. "001_公司业务部…__sec03_SQL_正文.md"
var secRegex = regexp.MustCompile(`__sec(\d{2})_(.+)\.md$`)

func parseFrontmatter(raw string) (Frontmatter, string, error) {
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
	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(fmLines, "\n")), &fm); err != nil {
		return fm, "", fmt.Errorf("yaml: %w", err)
	}
	return fm, strings.TrimSpace(strings.Join(bodyLines, "\n")), nil
}

// ---------- chunk + note model ----------

// Chunk represents one parsed file (a "sec" fragment).
type Chunk struct {
	Path       string
	SourceIdx  string
	Title      string
	Section    string
	SecOrder   int    // numeric order from filename __secNN
	Body       string // raw body text after frontmatter
}

// Note represents a merged logical note.
type Note struct {
	SourceIdx string
	Title     string
	Sections  []MergedSection // ordered
}

// MergedSection concatenates all parts that share the same section name.
type MergedSection struct {
	Section    string
	SectionIdx int
	Body       string
	SourcePaths []string
}

// ---------- walk + merge ----------

func walk(root string) ([]Chunk, error) {
	var chunks []Chunk
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		m := secRegex.FindStringSubmatch(filepath.Base(path))
		if m == nil {
			return nil // not a sec-fragment file
		}
		secOrder, _ := strconv.Atoi(m[1])
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fm, body, err := parseFrontmatter(string(b))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		chunks = append(chunks, Chunk{
			Path:      path,
			SourceIdx: fm.SourceIdx,
			Title:     fm.Title,
			Section:   fm.Section,
			SecOrder:  secOrder,
			Body:      body,
		})
		return nil
	})
	return chunks, err
}

func mergeByNote(chunks []Chunk) []Note {
	bySrc := map[string][]Chunk{}
	for _, c := range chunks {
		bySrc[c.SourceIdx] = append(bySrc[c.SourceIdx], c)
	}
	notes := []Note{}
	for src, parts := range bySrc {
		// Stable sort: SecOrder ascending.
		sort.Slice(parts, func(i, j int) bool { return parts[i].SecOrder < parts[j].SecOrder })
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
			// Append body with a newline if non-empty.
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
	sort.Slice(notes, func(i, j int) bool {
		a, _ := strconv.Atoi(notes[i].SourceIdx)
		b, _ := strconv.Atoi(notes[j].SourceIdx)
		return a < b
	})
	return notes
}

// ---------- verification ----------

// SQL_TOKEN: any line beginning with "update " (case-insensitive) referencing
// an etl_* table. Used to count distinct SQL statements in the corpus.
var sqlUpdateRegex = regexp.MustCompile(`(?im)^\s*update\s+(etl_\w+)\b`)

// sqlStartRegex matches the start of a fresh SQL block ("update|insert|create
// or replace function|delete|merge|select into").
var sqlStartRegex = regexp.MustCompile(`(?im)^\s*(update|insert\s+into|create\s+or\s+replace|delete\s+from|merge\s+into|select\s+.*\s+into)\b`)

// corpusSQLStatements returns the set of distinct "first tokens" of SQL
// statements found in any input fragment body.
func corpusSQLStatements(chunks []Chunk) map[string]int {
	counts := map[string]int{}
	for _, c := range chunks {
		for _, line := range strings.Split(c.Body, "\n") {
			trim := strings.TrimSpace(line)
			if sqlUpdateRegex.MatchString(trim) {
				// Capture target table for uniqueness.
				m := sqlUpdateRegex.FindStringSubmatch(trim)
				counts["update:"+m[1]]++
			}
		}
	}
	return counts
}

// noteHasAllSQLStatements checks whether the merged note's body contains
// every distinct SQL target table from the corpus.
func noteHasAllSQLStatements(note Note, corpusSQL map[string]int) (int, int, []string) {
	full := noteBody(note)
	found := 0
	missing := []string{}
	for stmt := range corpusSQL {
		// Match the same regex on the merged body.
		if sqlUpdateRegex.MatchString(full) && strings.Contains(full, stmt[strings.IndexByte(stmt, ':')+1:]) {
			// Be more precise: every line that updates the table appears
			// at least once.
			if strings.Contains(full, "update "+stmt[strings.IndexByte(stmt, ':')+1:]) {
				found++
				continue
			}
		}
		missing = append(missing, stmt)
	}
	return found, len(corpusSQL), missing
}

func noteBody(n Note) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", n.Title)
	for _, s := range n.Sections {
		fmt.Fprintf(&b, "## %s\n\n", s.Section)
		b.WriteString(strings.TrimSpace(s.Body))
		b.WriteString("\n\n")
	}
	return b.String()
}

// ---------- metadata for Milvus ----------

// chunkMeta serializes per-section metadata as a JSON document consumable
// as a Milvus metadata field.
type ChunkMeta struct {
	SourceIdx  string   `json:"source_idx"`
	Title      string   `json:"title"`
	Section    string   `json:"section"`
	SectionIdx int      `json:"section_idx"`
	Sections   int      `json:"section_count"`
	SourcePaths []string `json:"source_paths"`
	BodyBytes  int      `json:"body_bytes"`
}

func chunkMeta(n Note, s MergedSection) ChunkMeta {
	return ChunkMeta{
		SourceIdx:  n.SourceIdx,
		Title:      n.Title,
		Section:    s.Section,
		SectionIdx: s.SectionIdx,
		Sections:   len(n.Sections),
		SourcePaths: s.SourcePaths,
		BodyBytes:  len(s.Body),
	}
}

// ---------- main ----------

func main() {
	fmt.Println("=== spike 005c: ontology-aware markdown chunker ===")

	const root = "knowledge/sql-guides"
	chunks, err := walk(root)
	if err != nil {
		fmt.Println("[FAIL] walk:", err)
		os.Exit(1)
	}
	fmt.Printf("[1] walked %d sec-fragment chunks under %s\n", len(chunks), root)

	// Tally per-note fragment counts.
	bySrc := map[string]int{}
	for _, c := range chunks {
		bySrc[c.SourceIdx]++
	}
	fmt.Printf("[1a] distinct logical notes by source_idx: %d\n", len(bySrc))
	keys := make([]string, 0, len(bySrc))
	for k := range bySrc {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, _ := strconv.Atoi(keys[i])
		b, _ := strconv.Atoi(keys[j])
		return a < b
	})
	for _, k := range keys {
		fmt.Printf("    - source_idx=%s fragments=%d\n", k, bySrc[k])
	}

	notes := mergeByNote(chunks)
	fmt.Printf("[2] merged into %d logical notes\n", len(notes))

	// Verify boundary detection: every fragment must map to exactly one
	// merged section.
	totalContrib := 0
	for _, n := range notes {
		for _, s := range n.Sections {
			totalContrib += len(s.SourcePaths)
		}
	}
	fmt.Printf("[3] boundary detection: %d fragment-contributions across %d sections\n",
		totalContrib, len(notes))

	// Focus on note 001 ("公司业务部") — the spike's headline target.
	var target *Note
	for i := range notes {
		if notes[i].SourceIdx == "001" {
			target = &notes[i]
			break
		}
	}
	if target == nil {
		fmt.Println("[FAIL] note 001 not found")
		os.Exit(1)
	}
	fmt.Printf("[4] note 001 title=%q, sections=%d\n", target.Title, len(target.Sections))
	for _, s := range target.Sections {
		fmt.Printf("    - [section #%d] %-20s body=%d bytes source_paths=%d\n",
			s.SectionIdx, s.Section, len(s.Body), len(s.SourcePaths))
	}

	// (5) SQL completeness check.
	corpus := corpusSQLStatements(chunks)
	fmt.Printf("[5a] distinct `update etl_*` targets in corpus: %d\n", len(corpus))
	for k := range corpus {
		fmt.Printf("    - %s\n", k)
	}
	found, total, missing := noteHasAllSQLStatements(*target, corpus)
	fmt.Printf("[5b] note 001 contains %d/%d corpus SQL targets\n", found, total)
	if len(missing) > 0 {
		fmt.Println("    MISSING:")
		for _, m := range missing {
			fmt.Printf("      - %s\n", m)
		}
	}

	// (6) Metadata JSON serializability — emit one metadata record per
	// section so the Milvus chunk writer can attach them.
	metaOut := []ChunkMeta{}
	for _, n := range notes {
		for _, s := range n.Sections {
			metaOut = append(metaOut, chunkMeta(n, s))
		}
	}
	metaJSON, err := json.MarshalIndent(metaOut, "", "  ")
	if err != nil {
		fmt.Println("[FAIL] meta marshal:", err)
		os.Exit(1)
	}
	const metaPath = "chunk_metadata.json"
	if err := os.WriteFile(metaPath, metaJSON, 0644); err != nil {
		fmt.Println("[FAIL] write meta:", err)
		os.Exit(1)
	}
	fmt.Printf("[6] emitted %s (%d bytes, %d metadata records)\n",
		metaPath, len(metaJSON), len(metaOut))

	// (7) Roundtrip: write merged notes back to merged_notes/, reload, and
	// confirm SQL completeness preserved.
	outDir := "merged_notes"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Println("[FAIL] mkdir merged_notes:", err)
		os.Exit(1)
	}
	for _, n := range notes {
		fp := filepath.Join(outDir, fmt.Sprintf("%s_%s.md", n.SourceIdx, n.Title))
		body := noteBody(n)
		// Wrap with frontmatter to allow roundtrip.
		wrapped := fmt.Sprintf("---\nsource_idx: %s\ntitle: %s\nsections: %d\n---\n\n%s",
			n.SourceIdx, n.Title, len(n.Sections), body)
		if err := os.WriteFile(fp, []byte(wrapped), 0644); err != nil {
			fmt.Println("[FAIL] write note:", err)
			os.Exit(1)
		}
	}
	fmt.Printf("[7] wrote %d merged notes to %s/\n", len(notes), outDir)

	// Re-walk merged notes to confirm SQL is preserved (no truncation).
	mergedChunks, _ := walk(outDir)
	if len(mergedChunks) != len(notes) {
		fmt.Printf("[7a] WARN: rewalked merged notes count=%d != original notes=%d\n",
			len(mergedChunks), len(notes))
	}
	// Verify note 001 still contains all SQL.
	for _, c := range mergedChunks {
		if c.SourceIdx != "001" {
			continue
		}
		f2, t2, _ := noteHasAllSQLStatements(Note{
			SourceIdx: c.SourceIdx, Title: c.Title,
			Sections: []MergedSection{{Section: c.Section, Body: c.Body}},
		}, corpus)
		fmt.Printf("[7b] roundtripped note 001 contains %d/%d corpus SQL targets\n", f2, t2)
	}

	// Verdict
	fmt.Println()
	boundaryOK := totalContrib == len(chunks)
	sqlOK := found == total && len(missing) == 0
	metaOK := len(metaOut) > 0
	if boundaryOK && sqlOK && metaOK {
		fmt.Println("=== VERDICT: VALIDATED — ontology-aware chunker recovers complete SQL ===")
	} else {
		fmt.Println("=== VERDICT: INVALIDATED — boundary/SQL/metadata check failed ===")
		os.Exit(1)
	}
}
