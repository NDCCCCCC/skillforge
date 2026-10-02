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
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readDecodedCSV runs one fixture through the full pipeline:
// file bytes → DecodeStream (BOM strip + encoding detection + decode)
// → NewCSVReader → all rows.
func readDecodedCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	r, err := DecodeStream(f)
	require.NoError(t, err)
	rows, err := NewCSVReader(r).ReadAll()
	require.NoError(t, err)
	return rows
}

// TestDecode is the DATA-01 four-encoding contract: UTF-8, UTF-8-BOM,
// GBK, and GB18030 fixtures carrying the SAME logical content must
// parse to byte-identical cell values (Chinese names, +86 mobile,
// negative amounts, quoted comma fields, NULL empties).
func TestDecode(t *testing.T) {
	fixtures := []string{"utf8.csv", "utf8bom.csv", "gbk.csv", "gb18030.csv"}

	// Sanity: the GBK fixture must NOT be valid UTF-8 — otherwise the
	// cross-encoding comparison would vacuously pass through the UTF-8
	// path and prove nothing about GB18030 decoding.
	gbkRaw, err := os.ReadFile(filepath.Join("testdata", "gbk.csv"))
	require.NoError(t, err)
	assert.False(t, utf8.Valid(gbkRaw), "gbk.csv must be genuine GBK bytes")

	var reference [][]string
	for i, fx := range fixtures {
		rows := readDecodedCSV(t, filepath.Join("testdata", fx))
		if i == 0 {
			reference = rows
			require.Len(t, rows, 6, "header + 5 data rows")
			assert.Equal(t, []string{
				"customer_name", "mobile", "cert_no",
				"balance", "overdue_days", "update_ts", "status",
			}, rows[0])
			assert.Equal(t, "张三", rows[1][0])
			assert.Equal(t, "-567.89", rows[2][3], "negative amount must survive")
			assert.Equal(t, "状态,可疑", rows[3][6], "quoted comma field must survive")
			assert.Equal(t, "", rows[4][2], "NULL empty cell must survive")
			assert.Equal(t, "钱七,二世", rows[5][0], "quoted comma name must survive")
			continue
		}
		require.Equal(t, len(reference), len(rows), "%s: row count", fx)
		for ri := range reference {
			require.Len(t, rows[ri], len(reference[ri]), "%s row %d: field count", fx, ri)
			for ci := range reference[ri] {
				assert.Equal(t, reference[ri][ci], rows[ri][ci],
					"%s cell [%d][%d] must be byte-identical to the UTF-8 reference", fx, ri, ci)
			}
		}
	}
}

// TestDecodeBOMStripped: the BOM must be stripped, else the first
// header cell becomes "customer_name" and rule-file column
// matching fails — a fail-closed MISREJECTION of a perfectly good file.
func TestDecodeBOMStripped(t *testing.T) {
	rows := readDecodedCSV(t, filepath.Join("testdata", "utf8bom.csv"))
	// Exact-equality is the lock: an unstripped BOM (U+FEFF) would make
	// the first header cell differ from "customer_name".
	assert.Equal(t, "customer_name", rows[0][0])
}

// TestDecodeInvalidMixedRejected: a UTF-8 header followed by an illegal
// byte (0x81) must be REJECTED with an error — never silently decoded
// into mojibake (fail-closed, D15-B-adjacent / T-15-12).
func TestDecodeInvalidMixedRejected(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "invalid_mixed.csv"))
	require.NoError(t, err)
	defer f.Close()
	r, err := DecodeStream(f)
	require.NoError(t, err, "detection itself succeeds; the failure surfaces at read time")
	_, err = NewCSVReader(r).ReadAll()
	require.Error(t, err, "mixed-encoding byte stream must fail closed, not pass as garbage")
}

// TestDecodeUTF8Priority locks the §3.1 ordering iron law: UTF-8
// validity is probed BEFORE the GB18030 fallback. GB18030's byte space
// is nearly complete, so probing it first would silently decode every
// UTF-8 Chinese file into mojibake (Pitfall 10).
func TestDecodeUTF8Priority(t *testing.T) {
	utf8Bytes, err := os.ReadFile(filepath.Join("testdata", "utf8.csv"))
	require.NoError(t, err)
	assert.Equal(t, EncodingUTF8, DetectEncoding(utf8Bytes),
		"UTF-8 Chinese content must NEVER take the GB18030 path")

	gbkBytes, err := os.ReadFile(filepath.Join("testdata", "gbk.csv"))
	require.NoError(t, err)
	assert.Equal(t, EncodingGB18030, DetectEncoding(gbkBytes),
		"GBK bytes take the GB18030 superset fallback")
}
