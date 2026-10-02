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

//go:build contract

package contract

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// importScanRoots are the production code trees that handle untrusted data
// imports.  Tests are included for the multipart-form ban because a regression
// test that falls back to c.FormFile is still a leak.
var importScanRoots = []string{
	"internal/server",
	"internal/data",
}

// importSkipDirs mirrors the Phase 8/11/13 WalkDir gate skip set.
var importSkipDirs = map[string]bool{
	".claude":      true,
	".git":         true,
	"node_modules": true,
	".planning":    true,
	"screenshots":  true,
	"build":        true,
}

// importGateFile is the gate file itself; it carries the banned literals in
// pattern strings and must not flag itself.
const importGateFile = "import_gates_test.go"

// TestNoPreParseMultipartPath verifies that no file in the import surface
// calls the Hertz multipart pre-parse APIs.  Those APIs materialize uploaded
// parts on disk for large payloads, violating the "raw never on disk" rule
// (T-15-02 / D15-B1 / Pitfall 1).
//
// Banned literals:
//   - .MultipartForm(
//   - .FormFile(
//
// Comments and string literals are ignored so that documentation mentioning
// the banned APIs does not create a false positive.
func TestNoPreParseMultipartPath(t *testing.T) {
	root := findModuleRoot(t)
	var hits []string
	for _, rel := range importScanRoots {
		abs := filepath.Join(root, rel)
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			continue
		}
		walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if importSkipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Name() == importGateFile || !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}
			checkCodeLines(t, root, path, []string{".MultipartForm(", ".FormFile("}, &hits)
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s: %v", abs, walkErr)
		}
	}
	if len(hits) > 0 {
		t.Fatalf("pre-parse multipart API use detected (raw would hit disk):\n  %s", strings.Join(hits, "\n  "))
	}
}

// TestNoRowInsertInImportPath verifies that the production import write path
// never uses row-by-row INSERT INTO.  Bulk ingestion must go through the
// DuckDB Appender path (WriteStagingBatch) — SC-1 / STACK-VERIFICATION §2.8.
//
// internal/data/dump.go is EXCLUDED: it parses untrusted .dump files and
// therefore legitimately contains "INSERT INTO" in validator strings and
// error messages.  internal/store/sqlite is out of scope because the SQLite
// side of the store layer is allowed to use SQL INSERT for audit/registry rows.
func TestNoRowInsertInImportPath(t *testing.T) {
	root := findModuleRoot(t)
	var hits []string

	// internal/data production code (exclude dump.go parser and tests).
	dataRoot := filepath.Join(root, "internal", "data")
	walkErr := filepath.WalkDir(dataRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if importSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == importGateFile || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		if d.Name() == "dump.go" {
			return nil // parser/validator, not an execution path
		}
		checkCodeLines(t, root, path, []string{"INSERT INTO"}, &hits)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", dataRoot, walkErr)
	}

	// internal/duckdb/importer_write.go production bulk-write helper.
	writePath := filepath.Join(root, "internal", "duckdb", "importer_write.go")
	checkCodeLines(t, root, writePath, []string{"INSERT INTO"}, &hits)

	if len(hits) > 0 {
		t.Fatalf("row-by-row INSERT INTO found in import write path (must use Appender):\n  %s", strings.Join(hits, "\n  "))
	}
}

// TestNoRowValueLoggingInImportAPI verifies that internal/server/import_api.go
// never logs raw row values.  Only batch-level metadata (batch_id, content
// SHA256, row count, column names, elapsed time) may appear in logs
// (D15-B4 / Pitfall 8).
//
// Banned identifiers inside a log.Printf / log.Errorf / log.Warnf call:
//   - SamplePairs
//   - sample
//   - RowPair
//   - rows[
//   - .Raw
func TestNoRowValueLoggingInImportAPI(t *testing.T) {
	root := findModuleRoot(t)
	path := filepath.Join(root, "internal", "server", "import_api.go")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	logCall := regexp.MustCompile(`log\.(Printf|Errorf|Warnf)\(`)
	banned := []string{"SamplePairs", "sample", "RowPair", "rows[", ".Raw"}
	var hits []string

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if !logCall.MatchString(line) {
			continue
		}
		for _, b := range banned {
			if strings.Contains(line, b) {
				rel, _ := filepath.Rel(root, path)
				hits = append(hits, rel+":"+itoa(lineNo)+" contains '"+b+"' in log call")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	if len(hits) > 0 {
		t.Fatalf("row-value logging detected in import API:\n  %s", strings.Join(hits, "\n  "))
	}
}

// checkCodeLines reads a Go file line by line and reports any non-comment line
// that contains one of the banned substrings.  Pure comment lines are ignored
// so that documentation and gate files can mention the banned APIs without
// causing false positives.  String literals on a code line are still scanned —
// that is intentional: an executable statement like `db.Exec("INSERT INTO ...")`
// will be caught.
func checkCodeLines(t *testing.T, root, path string, banned []string, hits *[]string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	rel, _ := filepath.Rel(root, path)
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		text := scanner.Text()
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		for _, b := range banned {
			if strings.Contains(text, b) {
				*hits = append(*hits, rel+":"+itoa(lineNo))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
