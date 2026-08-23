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

package data

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// ENCODING ORDERING IRON LAW (STACK-VERIFICATION §3.1, 15-RESEARCH
// Pitfall 10): UTF-8 validity MUST be probed BEFORE the GB18030
// fallback. GB18030's byte space is nearly complete — a UTF-8 byte
// stream fed to the GB18030 decoder "successfully" decodes into
// mojibake instead of erroring. Probing GB18030 first would silently
// corrupt every UTF-8 Chinese file. DetectEncoding is the single
// decision point and the tests lock the order
// (TestDecodeUTF8Priority).
//
// DecodeStream pipeline (streaming — multipart parts cannot be
// re-read, so whole-file utf8.Valid is not an option):
//
//  1. bufio.Peek(3) == EF BB BF → Discard(3) (an unstripped BOM would
//     corrupt the first header cell and fail-closed-misreject the file
//     at rule-column matching).
//  2. bufio.Peek(64KB) sniff (Peek does not consume; a short file
//     simply returns a shorter slice) → DetectEncoding.
//  3. UTF-8 path: the remaining stream is wrapped in a STRICT UTF-8
//     validating reader (the x/text UTF-8 decoder REPLACES invalid
//     bytes with U+FFFD — silent mojibake, unacceptable; fail-closed
//     requires an error).
//  4. GB18030 path: simplifiedchinese.GB18030.NewDecoder().Reader —
//     the x/text decoder ERRORS on illegal sequences, which is the
//     fail-closed behavior we need (GBK is a subset; one decoder
//     covers both fixtures).
//
// Content hashing (D15-B1/B4/C1 bridge) is deliberately NOT done in
// this layer — callers wrap with io.TeeReader(part, hasher) at the
// import boundary (15-05 wiring).

// Encoding labels returned by DetectEncoding.
const (
	EncodingUTF8    = "utf-8"
	EncodingGB18030 = "gb18030"
)

// utf8BOM is the 3-byte UTF-8 byte order mark.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// decodeSniffBytes is the first-block sniff window (64KB). The bufio
// reader is sized sniff+1 so a full-window Peek never hits
// bufio.ErrBufferFull.
const decodeSniffBytes = 64 * 1024

// DecodeStream returns a reader that yields decoded UTF-8 bytes from a
// possibly BOM-prefixed UTF-8 / GBK / GB18030 byte stream, failing
// closed on illegal sequences (mixed encodings are bad data, never
// best-effort mojibake). The returned reader errors mid-stream if the
// chosen path encounters bytes that cannot decode.
func DecodeStream(r io.Reader) (io.Reader, error) {
	br := bufio.NewReaderSize(r, decodeSniffBytes+1)

	// ① Strip a leading UTF-8 BOM. Peek returns what is available
	// (short files yield shorter slices); only an exact 3-byte BOM match
	// is discarded.
	if head, _ := br.Peek(3); len(head) == 3 && bytes.Equal(head, utf8BOM) {
		if _, err := br.Discard(3); err != nil {
			return nil, fmtError("DecodeStream", err)
		}
	}

	// ② Sniff the first block (Peek does not consume it).
	sniff, err := br.Peek(decodeSniffBytes)
	if err != nil && err != io.EOF && err != bufio.ErrBufferFull {
		return nil, fmtError("DecodeStream", err)
	}

	// ③④ Route per the ordering iron law.
	if DetectEncoding(sniff) == EncodingUTF8 {
		return &strictUTF8Reader{src: br}, nil
	}
	// x/text's GB18030 decoder silently replaces illegal bytes with
	// U+FFFD instead of erroring. Wrap it so any replacement character in
	// the decoded stream is a hard fail-closed error (T-15-12).
	return &strictDecoderReader{
		src: simplifiedchinese.GB18030.NewDecoder().Reader(br),
	}, nil
}

// strictDecoderReader wraps a decoded byte stream and treats any
// U+FFFD replacement rune as a hard error. x/text decoders emit this
// rune for bytes that cannot be transcoded (their default is "best
// effort" mojibake); the import pipeline's fail-closed contract
// requires an error instead. The overlap tail handles a replacement
// sequence split across two Read calls.
type strictDecoderReader struct {
	src  io.Reader
	tail []byte // up to 2 bytes from the previous read
}

func (r *strictDecoderReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	if n <= 0 {
		return n, err
	}
	chunk := append(r.tail, p[:n]...)
	if bytes.Contains(chunk, []byte{0xEF, 0xBF, 0xBD}) {
		return 0, fmtError("DecodeStream", errors.New(
			"invalid byte sequence in GB18030 stream — fail-closed (decoded replacement character detected)"))
	}
	if len(chunk) >= 2 {
		r.tail = chunk[len(chunk)-2:]
	} else {
		r.tail = chunk
	}
	return n, err
}

// DetectEncoding labels a head chunk per the §3.1 ordering iron law:
// valid UTF-8 wins ("utf-8"), everything else falls back to GB18030
// ("gb18030"). An empty/short head defaults to UTF-8 (utf8.Valid on
// empty is true — an empty stream has no GBK payload to rescue).
func DetectEncoding(head []byte) string {
	if utf8.Valid(head) {
		return EncodingUTF8
	}
	return EncodingGB18030
}

// NewCSVReader wraps a decoded stream in a STRICT encoding/csv reader:
// LazyQuotes stays false (sloppy quoting is rejected, not guessed) and
// FieldsPerRecord is pinned by the first record — every later row with
// a different field count returns csv.ErrFieldCount (the csv package's
// own fail-closed ragged-row guard; no lenient override is exposed).
func NewCSVReader(r io.Reader) *csv.Reader {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 0 // 0 = set from the first (header) record
	return cr
}

// strictUTF8Reader validates UTF-8 while passing bytes through. It
// exists because x/text's UTF-8 decoder substitutes U+FFFD instead of
// erroring; the import pipeline's fail-closed contract (T-15-12)
// requires a hard error on any illegal or truncated sequence.
//
// Chunk-boundary correctness: a rune split across two upstream reads is
// buffered as an incomplete tail (scanUTF8's incomplete flag) until
// enough bytes arrive; a tail still incomplete at EOF is an error.
type strictUTF8Reader struct {
	src    io.Reader
	pend   []byte // validated prefix + at most one possibly-incomplete trailing rune
	srcEOF bool
}

// Read delivers validated bytes, reading (and validating) more from
// src as needed. Errors are wrapped in the data-package fmtError form.
func (r *strictUTF8Reader) Read(p []byte) (int, error) {
	for {
		n, incomplete, invalid := scanUTF8(r.pend)
		if invalid {
			return 0, fmtError("DecodeStream", errors.New(
				"invalid UTF-8 sequence in stream — fail-closed (mixed-encoding input is bad data)"))
		}
		if n > 0 {
			c := copy(p, r.pend[:n])
			r.pend = r.pend[c:]
			return c, nil
		}
		if r.srcEOF {
			if incomplete {
				return 0, fmtError("DecodeStream", errors.New(
					"truncated UTF-8 sequence at EOF — fail-closed"))
			}
			return 0, io.EOF
		}
		chunk := make([]byte, 32*1024)
		m, err := r.src.Read(chunk)
		if m > 0 {
			r.pend = append(r.pend, chunk[:m]...)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				r.srcEOF = true
				continue // re-scan: the tail is now either invalid, truncated, or empty
			}
			return 0, err
		}
	}
}

// scanUTF8 walks b and classifies its leading bytes:
//
//	n          — length of the maximal prefix of COMPLETE valid runes
//	incomplete — b[n:] is a rune truncated at the buffer edge (the
//	             caller must read more before deciding)
//	invalid    — b[n:] starts a sequence that can never decode, even
//	             with more bytes (hard fail)
//
// The discriminator is utf8.FullRune: it reports false ONLY for a
// truncated-but-otherwise-valid rune prefix; invalid start bytes count
// as "full" 1-byte error runes and are caught by the DecodeRune check.
func scanUTF8(b []byte) (n int, incomplete bool, invalid bool) {
	i := 0
	for i < len(b) {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			if !utf8.FullRune(b[i:]) {
				return i, true, false
			}
			return i, false, true
		}
		i += size
	}
	return i, false, false
}
