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

package document

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParserRegistryRoundTrip(t *testing.T) {
	reg := NewParserRegistry()
	assert.NotNil(t, reg)
	mimes := reg.AllowedMIMETypes()
	assert.Len(t, mimes, 5, "exactly 5 MIME types must be registered")
}

func TestParserContentTypeAllowlist(t *testing.T) {
	reg := NewParserRegistry()
	ctx := context.Background()

	// text/markdown
	rdr := bytes.NewReader([]byte("# Hello\n\nWorld"))
	result, err := reg.Parse(ctx, "text/markdown", rdr, "test.md")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "goldmark@v1.8.5", result.ParserName)

	// application/pdf — magic byte mismatch will reject
	// Use a buffer that starts with %PDF but is too short to be valid
	// The pdfParser will fail at parsing, not at magic check, so we just
	// verify ErrUnknownFormat is NOT returned for valid MIME types.
	for _, mime := range reg.AllowedMIMETypes() {
		rdr := bytes.NewReader([]byte("PK\x03\x04")) // ZIP header for docx/xlsx/pptx; PDF will fail parsing
		result, err := reg.Parse(ctx, mime, rdr, "test.file")
		// We expect an error (parse failure) but NOT ErrUnknownFormat
		if err == ErrUnknownFormat {
			t.Errorf("Parse(%q) returned ErrUnknownFormat — MIME type should be known", mime)
		}
		_ = result // result may be nil on parse failure; that's expected for synthetic buffers
	}
}

func TestParserRejectUnknown(t *testing.T) {
	reg := NewParserRegistry()
	ctx := context.Background()

	rdr := bytes.NewReader([]byte("some content"))
	result, err := reg.Parse(ctx, "application/octet-stream", rdr, "test.bin")
	assert.ErrorIs(t, err, ErrUnknownFormat)
	assert.Nil(t, result)
}

func TestParserMDWithFrontmatter(t *testing.T) {
	reg := NewParserRegistry()
	ctx := context.Background()

	md := `---
title: Test Doc
author: Alice
---

# Hello

This is the body.
`
	rdr := bytes.NewReader([]byte(md))
	result, err := reg.Parse(ctx, "text/markdown", rdr, "test.md")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Body should not contain --- markers.
	assert.NotContains(t, string(result.Content), "---")
	// Content should contain the body text.
	assert.Contains(t, string(result.Content), "Hello")
}

func TestParserMDWithoutFrontmatter(t *testing.T) {
	reg := NewParserRegistry()
	ctx := context.Background()

	md := `# Hello

This is plain text without frontmatter.
`
	rdr := bytes.NewReader([]byte(md))
	result, err := reg.Parse(ctx, "text/markdown", rdr, "test.md")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, string(result.Content), "Hello")
}

func TestParserMagicByteMismatch(t *testing.T) {
	reg := NewParserRegistry()
	ctx := context.Background()

	// Buffer starts with PK (ZIP) but declared as application/pdf.
	// The mdParser does NOT check magic bytes (it parses text), so we test
	// the ZIP-based parsers which do: docx, xlsx, pptx.
	zipBuf := []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00fake content here")
	rdr := bytes.NewReader(zipBuf)
	_, err := reg.Parse(ctx, "application/pdf", rdr, "fake.pdf")
	assert.ErrorIs(t, err, ErrContentTypeMismatch, "ZIP header cannot be parsed as PDF")
}

func TestParserMagicBytesOK(t *testing.T) {
	// Verify that valid magic bytes pass the check.
	buf := []byte(`%PDF-1.4 fake pdf content`)
	err := checkMagicBytes(buf, []byte("%PDF"), "application/pdf")
	assert.NoError(t, err)
}

func TestParserMagicBytesShortBuffer(t *testing.T) {
	// Buffer shorter than magic bytes.
	buf := []byte("%P")
	err := checkMagicBytes(buf, []byte("%PDF"), "application/pdf")
	assert.ErrorIs(t, err, ErrContentTypeMismatch)
}

func TestParserMagicBytesZIP(t *testing.T) {
	// Valid ZIP header (PK..
	buf := []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00")
	err := checkMagicBytes(buf, []byte("PK"), "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	assert.NoError(t, err)
}

func TestParserMagicBytesZIPMismatch(t *testing.T) {
	// Not a ZIP file.
	buf := []byte("NOTAZIPFILE")
	err := checkMagicBytes(buf, []byte("PK"), "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	assert.ErrorIs(t, err, ErrContentTypeMismatch)
}

func TestStripHTML(t *testing.T) {
	html := []byte(`<p>Hello <strong>World</strong></p>`)
	plain := stripHTML(html)
	// All whitespace runs (including multiple consecutive spaces) collapse to single space.
	assert.Equal(t, " Hello World ", string(plain))
}

func TestSplitFrontmatter(t *testing.T) {
	// No frontmatter.
	raw := []byte("# Hello\n\nWorld")
	body, fm := splitFrontmatter(raw)
	assert.Equal(t, raw, body)
	assert.Nil(t, fm)

	// With frontmatter.
	raw = []byte("---\ntitle: Test\n---\n# Hello\n")
	body, fm = splitFrontmatter(raw)
	assert.Contains(t, string(body), "Hello")
	assert.Equal(t, "title: Test", string(fm))

	// Unterminated frontmatter (treated as no frontmatter).
	raw = []byte("---\ntitle: Test\n# Hello\n")
	body, fm = splitFrontmatter(raw)
	assert.Contains(t, string(body), "Hello")
	assert.Nil(t, fm)
}
