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

// Package document provides a parser registry for extracting plain-text content
// from uploaded documents (md/pdf/docx/xlsx/pptx). It is the foundation of
// Phase 27's document upload pipeline.
//
// CGO status as of eino-ext pseudo-version v0.0.0-20260904094655-8d8ff4e79992
// and xuri/excelize v2.11.0:
//
//   md     → goldmark v1.8.5:                       PURE-GO (no "C" import)
//   pdf    → eino-ext/pdf → ledongthuc/pdf v0.0.0:   PURE-GO (no "C" import)
//   docx   → eino-ext/docx → eino-contrib/docx2md:   PURE-GO (no "C" import)
//   xlsx   → xuri/excelize/v2 v2.11.0:              PURE-GO (zip-based; no Cgo)
//   pptx   → pure-Go ZIP walker:                     PURE-GO (no external library)
//
// All five parsers are CGO-free. The upload pipeline compiles and runs on
// any host that has a Go toolchain (Linux/Windows/macOS, with or without CGO).
//
// Extension notes:
//   - eino-ext has no pptx parser module; pptx uses a direct ZIP/XML walker.
//   - xuri/excelize/v2 v2.11.0 is pure-Go (ZIP-based Office format reader).
package document

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync"

	"github.com/cloudwego/eino-ext/components/document/parser/pdf"
	"github.com/xuri/excelize/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// MIME-to-parser registry. Format name == magic-byte verified MIME string.
type ParserRegistry struct {
	parsers map[string]Parser
	mu      sync.RWMutex
}

// Parser extracts plain-text content from a document stream.
// Implementations MUST accept io.Reader (not *os.File).
type Parser interface {
	Parse(ctx context.Context, r io.Reader, filename string) ([]byte, error)
	// Name returns the parser identifier, e.g. "eino-ext/pdf@ledongthuc/pdf".
	Name() string
}

// ParseResult is the output of a successful parse.
type ParseResult struct {
	Content     []byte  // plain-text body
	Frontmatter []byte  // YAML block if present, nil otherwise
	ParserName  string  // which parser produced this
}

// ErrPasswordProtected is returned for protected binary docs.
var ErrPasswordProtected = errors.New("document: file is password-protected")

// ErrUnknownFormat is returned when no parser is registered for the MIME type.
var ErrUnknownFormat = errors.New("document: unknown format")

// ErrContentTypeMismatch is returned when magic bytes do not match the declared MIME type.
var ErrContentTypeMismatch = errors.New("document: content type does not match file signature")

// NewParserRegistry returns a registry with all supported formats registered.
// The allowlist is the canonical list — no format is accepted unless registered.
func NewParserRegistry() *ParserRegistry {
	r := &ParserRegistry{parsers: make(map[string]Parser)}
	r.registerParsers()
	return r
}

func (r *ParserRegistry) registerParsers() {
	r.parsers["text/markdown"] = &mdParser{}
	r.parsers["application/pdf"] = &pdfParser{}
	r.parsers["application/vnd.openxmlformats-officedocument.wordprocessingml.document"] = &docxParser{}
	r.parsers["application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"] = &xlsxParser{}
	r.parsers["application/vnd.openxmlformats-officedocument.presentationml.presentation"] = &pptxParser{}
}

// Register records parser for mimeType. Called inside NewParserRegistry.
func (r *ParserRegistry) Register(mimeType string, p Parser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.parsers[mimeType] = p
}

// Parse uses the registered parser for mimeType to produce a ParseResult.
// Returns ErrUnknownFormat if mimeType is not registered.
func (r *ParserRegistry) Parse(ctx context.Context, mimeType string, rdr io.Reader, filename string) (*ParseResult, error) {
	r.mu.RLock()
	p, ok := r.parsers[mimeType]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrUnknownFormat
	}
	content, err := p.Parse(ctx, rdr, filename)
	if err != nil {
		return nil, err
	}
	return &ParseResult{
		Content:    content,
		ParserName: p.Name(),
	}, nil
}

// AllowedMIMETypes returns the whitelist of registered MIME types.
func (r *ParserRegistry) AllowedMIMETypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	mimes := make([]string, 0, len(r.parsers))
	for m := range r.parsers {
		mimes = append(mimes, m)
	}
	return mimes
}

// ---------------------------------------------------------------------------
// Markdown parser (goldmark)
// ---------------------------------------------------------------------------

// mdParser converts markdown to plain text via: markdown → goldmark HTML → strip HTML.
type mdParser struct{}

func (p *mdParser) Name() string { return "goldmark@v1.8.5" }

func (p *mdParser) Parse(ctx context.Context, r io.Reader, filename string) ([]byte, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("md ReadAll: %w", err)
	}

	// Normalize CRLF → LF for marker matching.
	raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))

	// Split frontmatter (mirrors internal/knowledge/indexer.go:splitFrontmatter).
	body, _ := splitFrontmatter(raw)

	// Convert markdown to HTML using goldmark. goldmark.New() uses the default
	// HTML renderer automatically when WithRenderer is not specified.
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
	)
	var htmlBuf bytes.Buffer
	if err := md.Convert(body, &htmlBuf); err != nil {
		return nil, fmt.Errorf("goldmark Convert: %w", err)
	}

	// Strip HTML tags to get plain text.
	plain := stripHTML(htmlBuf.Bytes())
	return bytes.TrimSpace(plain), nil
}

// splitFrontmatter extracts the markdown body and the raw YAML
// frontmatter block from raw. Returns (raw, nil) when no frontmatter
// is present, (raw, nil) on unterminated frontmatter. When frontmatter
// is present and well-formed, returns (body, frontmatterBytes) where
// frontmatterBytes is the YAML content between the --- markers (without
// the --- lines themselves).
//
// Mirrors internal/knowledge/indexer.go:splitFrontmatter.
func splitFrontmatter(raw []byte) (body []byte, frontmatter []byte) {
	const open = "---\n"
	if !bytes.HasPrefix(raw, []byte(open)) {
		return raw, nil
	}
	body = raw[len(open):]
	const close = "\n---"
	end := bytes.Index(body, []byte(close))
	if end < 0 {
		return raw, nil
	}
	fm := body[:end]
	after := body[end+len(close):]
	return bytes.TrimLeft(after, "\n"), fm
}

// htmlTagAndContent matches HTML tags and their content.
var htmlTagAndContent = regexp.MustCompile(`<[^>]*>`)

// stripHTML removes all HTML tags from data, returning plain text.
// Consecutive runs of whitespace are collapsed to single spaces.
func stripHTML(data []byte) []byte {
	// Remove all tags.
	text := htmlTagAndContent.ReplaceAll(data, []byte(" "))
	// Normalize whitespace.
	space := regexp.MustCompile(`[[:space:]]+`)
	return space.ReplaceAll(text, []byte(" "))
}

// ---------------------------------------------------------------------------
// PDF parser (ledongthuc/pdf via eino-ext/pdf)
// ---------------------------------------------------------------------------

type pdfParser struct{}

func (p *pdfParser) Name() string { return "eino-ext/pdf@ledongthuc/pdf@v0.0.0" }

func (p *pdfParser) Parse(ctx context.Context, r io.Reader, filename string) ([]byte, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("pdf ReadAll: %w", err)
	}
	if err := checkMagicBytes(buf, []byte("%PDF"), "application/pdf"); err != nil {
		return nil, err
	}

	parser, err := pdf.NewPDFParser(ctx, &pdf.Config{ToPages: false})
	if err != nil {
		return nil, fmt.Errorf("pdf NewPDFParser: %w", err)
	}
	docs, err := parser.Parse(ctx, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("pdf parser.Parse: %w", err)
	}
	var sb bytes.Buffer
	for _, doc := range docs {
		sb.WriteString(doc.Content)
		sb.WriteByte('\n')
	}
	return bytes.TrimSpace(sb.Bytes()), nil
}

// ---------------------------------------------------------------------------
// DOCX parser (pure-Go ZIP walker — reads document.xml from the docx zip)
// ---------------------------------------------------------------------------

type docxParser struct{}

func (p *docxParser) Name() string { return "pure-go/docx-zip-walker@v0.0.0" }

func (p *docxParser) Parse(ctx context.Context, r io.Reader, filename string) ([]byte, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("docx ReadAll: %w", err)
	}
	if err := checkMagicBytes(buf, []byte("PK"), "application/vnd.openxmlformats-officedocument.wordprocessingml.document"); err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return nil, fmt.Errorf("docx zip.NewReader: %w", err)
	}

	// Find word/document.xml inside the zip.
	var docXML *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docXML = f
			break
		}
	}
	if docXML == nil {
		return nil, fmt.Errorf("docx: word/document.xml not found in archive")
	}

	rc, err := docXML.Open()
	if err != nil {
		return nil, fmt.Errorf("docx: open word/document.xml: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("docx: read word/document.xml: %w", err)
	}

	text, err := extractDOCXText(data)
	if err != nil {
		return nil, fmt.Errorf("docx: extract text: %w", err)
	}
	return bytes.TrimSpace(text), nil
}

// extractDOCXText parses the Office Open XML document.xml byte slice and
// returns concatenated plain text of all paragraphs and table cells.
func extractDOCXText(data []byte) ([]byte, error) {
	type Text struct {
		Val string `xml:"t,attr"`
	}
	type Run struct {
		Text []Text `xml:"t"`
	}
	type Paragraph struct {
		Run []Run `xml:"r"`
	}
	type TableCell struct {
		Paragraphs []Paragraph `xml:"p"`
	}
	type TableRow struct {
		Cells []TableCell `xml:"tc"`
	}
	type Body struct {
		Paragraphs []Paragraph `xml:"p"`
		Tables     []struct {
			Rows []TableRow `xml:"tr"`
		} `xml:"tbl"`
	}
	type Document struct {
		Body Body `xml:"body"`
	}

	var doc Document
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("docx xml unmarshal: %w", err)
	}

	var sb bytes.Buffer
	for _, para := range doc.Body.Paragraphs {
		for _, run := range para.Run {
			for _, t := range run.Text {
				sb.WriteString(t.Val)
			}
		}
		sb.WriteByte('\n')
	}
	for _, tbl := range doc.Body.Tables {
		for _, row := range tbl.Rows {
			for ci, cell := range row.Cells {
				if ci > 0 {
					sb.WriteString(" | ")
				}
				for _, para := range cell.Paragraphs {
					for _, run := range para.Run {
						for _, t := range run.Text {
							sb.WriteString(t.Val)
						}
					}
				}
			}
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}
	return sb.Bytes(), nil
}

// ---------------------------------------------------------------------------
// XLSX parser (xuri/excelize/v2)
// ---------------------------------------------------------------------------

type xlsxParser struct{}

func (p *xlsxParser) Name() string { return "xuri/excelize/v2@v2.11.0" }

func (p *xlsxParser) Parse(ctx context.Context, r io.Reader, filename string) ([]byte, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("xlsx ReadAll: %w", err)
	}
	if err := checkMagicBytes(buf, []byte("PK"), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"); err != nil {
		return nil, err
	}

	f, err := excelize.OpenReader(bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("excelize.OpenReader: %w", err)
	}
	defer f.Close()

	var sb bytes.Buffer
	sheets := f.GetSheetList()
	for _, sheetName := range sheets {
		sb.WriteString("=== Sheet: ")
		sb.WriteString(sheetName)
		sb.WriteString(" ===\n")
		rows, err := f.GetRows(sheetName)
		if err != nil {
			continue
		}
		for _, row := range rows {
			for colIdx, cell := range row {
				if colIdx > 0 {
					sb.WriteString(" | ")
				}
				sb.WriteString(cell)
			}
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}
	return bytes.TrimSpace(sb.Bytes()), nil
}

// ---------------------------------------------------------------------------
// PPTX parser (pure-Go ZIP walker — no external library)
// ---------------------------------------------------------------------------

type pptxParser struct{}

func (p *pptxParser) Name() string { return "pure-go/pptx-zip-walker@v0.0.0" }

func (p *pptxParser) Parse(ctx context.Context, r io.Reader, filename string) ([]byte, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("pptx ReadAll: %w", err)
	}
	if err := checkMagicBytes(buf, []byte("PK"), "application/vnd.openxmlformats-officedocument.presentationml.presentation"); err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return nil, fmt.Errorf("pptx zip.NewReader: %w", err)
	}

	var sb bytes.Buffer
	// PPTX is a ZIP. Slides are in ppt/slides/slideN.xml.
	// Notes are in ppt/notesSlides/notesSlideN.xml.
	for _, f := range zr.File {
		name := f.Name
		switch {
		case isSlideXML(name):
			text, err := extractPPTXText(f)
			if err == nil && len(bytes.TrimSpace(text)) > 0 {
				sb.WriteString("=== Slide ===\n")
				sb.Write(text)
				sb.WriteByte('\n')
			}
		case isNotesXML(name):
			text, err := extractPPTXText(f)
			if err == nil && len(bytes.TrimSpace(text)) > 0 {
				sb.WriteString("[notes] ")
				sb.Write(text)
				sb.WriteByte('\n')
			}
		}
	}
	return bytes.TrimSpace(sb.Bytes()), nil
}

func isSlideXML(name string) bool {
	return len(name) > 15 && name[:15] == "ppt/slides/slide" && len(name) >= 19 && name[len(name)-4:] == ".xml"
}

func isNotesXML(name string) bool {
	return len(name) > 22 && name[:22] == "ppt/notesSlides/notesSlide" && len(name) >= 26 && name[len(name)-4:] == ".xml"
}

func extractPPTXText(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	// Extract text from PPTX XML using the pure-Go approach.
	// <a:t> tags contain text runs; </a:t> closes them.
	// </p:sp> and </p:txBody> close shape/text body elements.
	return extractPPTXTextFromXML(data), nil
}

// extractPPTXTextFromXML extracts readable text from PPTX slide/notes XML.
func extractPPTXTextFromXML(data []byte) []byte {
	var sb bytes.Buffer
	// Simple state machine: track whether we're inside <a:t>...</a:t>.
	inText := false
	i := 0
	for i < len(data) {
		if data[i] == '<' {
			// Find the end of the tag.
			j := i + 1
			for j < len(data) && data[j] != '>' {
				j++
			}
			if j >= len(data) {
				break
			}
			tag := string(data[i : j+1])
			// Detect opening <a:t ...> and bare <a:t> (some implementations omit space).
			if tag == "<a:t>" || tag == "<a:t " {
				inText = true
			} else if tag == "</a:t>" {
				sb.WriteByte(' ')
				inText = false
			} else if tag == "</p:sp>" || tag == "</p:txBody>" {
				sb.WriteByte('\n')
			}
			i = j + 1
			continue
		}
		if inText {
			sb.WriteByte(data[i])
		}
		i++
	}
	return bytes.TrimSpace(sb.Bytes())
}

// ---------------------------------------------------------------------------
// Magic byte verification
// ---------------------------------------------------------------------------

// checkMagicBytes verifies the file's magic bytes match the expected signature.
// pptx/docx/xlsx are all ZIP-based (PK header); pdf uses %PDF.
func checkMagicBytes(buf []byte, expected []byte, mimeType string) error {
	if len(buf) < len(expected) {
		return ErrContentTypeMismatch
	}
	if !bytes.HasPrefix(buf, expected) {
		return ErrContentTypeMismatch
	}
	return nil
}

// stripHTMLFromString is a convenience wrapper for strings.
func stripHTMLFromString(s string) string {
	return string(stripHTML([]byte(s)))
}

