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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skillforge/skillforge/internal/duckdb"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTempRoot is a package-private temp directory used for the "raw never on
// disk" assertions.  It isolates the leakage check from other test packages that
// run concurrently and write to the system temp directory.
var testTempRoot string

func TestMain(m *testing.M) {
	// os.TempDir caches its first computed value, so redirect it to a private
	// directory before any test (or imported package init) calls it.  We create
	// the directory under the repo root to avoid touching os.TempDir at all
	// during setup.
	root, err := os.MkdirTemp(".", "skillforge-data-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkdir data test temp root:", err)
		os.Exit(1)
	}
	testTempRoot, err = filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "abs data test temp root:", err)
		os.Exit(1)
	}
	_ = os.Setenv("TMPDIR", testTempRoot)
	_ = os.Setenv("TEMP", testTempRoot)
	_ = os.Setenv("TMP", testTempRoot)

	code := m.Run()
	_ = os.RemoveAll(testTempRoot)
	os.Exit(code)
}

func TestPreviewFirst100Pairs(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt:   "test-site-salt-16bytes",
		RulesDir:   "../../datarules",
		KBRegistry: testKBReg(),
	}, nil, nil)

	var rows []string
	rows = append(rows, "customer_name,mobile,cert_no,balance,overdue_days,update_ts,status")
	for i := 0; i < 150; i++ {
		rows = append(rows, fmt.Sprintf("张三%d,138-0000-%04d,11010119900101%04d,%d,%d,2026-07-01,active", i, i, i, 1000+i*10, i%90))
	}
	req := PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(strings.Join(rows, "\n")),
	}

	resp, err := imp.Preview(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "loans", resp.Dataset)
	assert.Equal(t, "2026-07", resp.Period)
	assert.Equal(t, int64(150), resp.RowCount)
	assert.Len(t, resp.SamplePairs, 100)
	assert.Equal(t, "historical", resp.ProposedTier,
		"the proposed tier is the registry slug from the rule file (KBM-04)")
	assert.NotEmpty(t, resp.PreviewID)
	assert.NotEmpty(t, resp.ContentSHA256)
	assert.NotEmpty(t, resp.RulesSHA256)

	// Hash columns become 64-char hex; bucket columns become numeric bucket floors.
	first := resp.SamplePairs[0]
	require.Len(t, first.Raw, 7)
	require.Len(t, first.Desensitized, 7)
	assert.Equal(t, "张三0", first.Raw[0])
	assert.Len(t, first.Desensitized[0], 64) // HMAC hex
	assert.Equal(t, "1000", first.Raw[3])
	assert.Equal(t, "1000", first.Desensitized[3]) // width=1000 bucket floor
}

func TestPreviewBucketOverride(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt:   "test-site-salt-16bytes",
		RulesDir:   "../../datarules",
		KBRegistry: testKBReg(),
	}, nil, nil)

	rows := []string{
		"customer_name,mobile,cert_no,balance,overdue_days,update_ts,status",
		"张三,138-0000-0000,11010119900101X,1250,5,2026-07-01,active",
	}
	req := PreviewRequest{
		Dataset:         "loans",
		Period:          "2026-07",
		Channel:         "csv",
		Reader:          strings.NewReader(strings.Join(rows, "\n")),
		BucketOverrides: map[string]float64{"balance": 500},
	}

	resp, err := imp.Preview(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "1000", resp.SamplePairs[0].Desensitized[3]) // floor(1250/500)*500 = 1000
}

func TestPreviewPeriodFormat(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt:   "test-site-salt-16bytes",
		RulesDir:   "../../datarules",
		KBRegistry: testKBReg(),
	}, nil, nil)

	rows := []string{
		"customer_name,mobile,cert_no,balance,overdue_days,update_ts,status",
		"张三,138-0000-0000,11010119900101X,1250,5,2026-07-01,active",
	}
	for _, bad := range []string{"2026-13", "202607", ""} {
		req := PreviewRequest{
			Dataset: "loans",
			Period:  bad,
			Channel: "csv",
			Reader:  strings.NewReader(strings.Join(rows, "\n")),
		}
		_, err := imp.Preview(context.Background(), req)
		require.Error(t, err, "period %q should be rejected", bad)
		assert.Contains(t, err.Error(), "YYYY-MM")
	}
}

func TestPreviewUnknownChannel(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt:   "test-site-salt-16bytes",
		RulesDir:   "../../datarules",
		KBRegistry: testKBReg(),
	}, nil, nil)

	req := PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "xml",
		Reader:  strings.NewReader(""),
	}
	_, err := imp.Preview(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown import channel")
}

func TestPreviewEmptySaltFailClosed(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt:   "",
		RulesDir:   "../../datarules",
		KBRegistry: testKBReg(),
	}, nil, nil)

	rows := []string{
		"customer_name,mobile,cert_no,balance,overdue_days,update_ts,status",
		"张三,138-0000-0000,11010119900101X,1250,5,2026-07-01,active",
	}
	req := PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(strings.Join(rows, "\n")),
	}
	_, err := imp.Preview(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SKILLFORGE_DATA_SALT")
}

func TestPreviewUnknownColumnFailClosed(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt:   "test-site-salt-16bytes",
		RulesDir:   "../../datarules",
		KBRegistry: testKBReg(),
	}, nil, nil)

	rows := []string{
		"customer_name,mobile,cert_no,balance,overdue_days,update_ts,status,extra_col",
		"张三,138-0000-0000,11010119900101X,1250,5,2026-07-01,active,x",
	}
	req := PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(strings.Join(rows, "\n")),
	}
	_, err := imp.Preview(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extra_col")
}

func TestPushCommitDirect(t *testing.T) {
	ctx := context.Background()
	imp, db, _, datasetsDir := testImporter(t)

	res, err := imp.Push(ctx, PushRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  &oneShotReader{r: strings.NewReader(csvFileA()), t: t},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.BatchID)
	assert.False(t, res.Superseded)
	assert.Equal(t, int64(2), res.Rows)

	batches, err := db.ListImportBatches(ctx, "loans")
	require.NoError(t, err)
	assert.Len(t, batches, 1)
	assert.Equal(t, res.BatchID, batches[0].BatchID)

	registry, err := db.ListDatasets(ctx)
	require.NoError(t, err)
	require.Len(t, registry, 1)
	assert.Equal(t, "loans__2026-07", registry[0].Name)
	assert.Equal(t, res.BatchID, registry[0].BatchID)

	assert.Equal(t, int64(2), rowCountInDataset(t, ctx, datasetsDir, "loans__2026-07"))
}

func TestPushDumpChannel(t *testing.T) {
	ctx := context.Background()
	imp, db, _, datasetsDir := testImporter(t)

	dump := strings.Join([]string{
		"PRAGMA foreign_keys=OFF;",
		"BEGIN TRANSACTION;",
		"CREATE TABLE loans (customer_name TEXT, mobile TEXT, cert_no TEXT, balance REAL, overdue_days INTEGER, update_ts TEXT, status TEXT);",
		"INSERT INTO loans VALUES ('Alice','138-0000-0000','11010119900101A',1250,5,'2026-07-01','active');",
		"INSERT INTO loans VALUES ('Bob',NULL,'11010119900101B',3500,12,'2026-07-01','inactive');",
		"COMMIT;",
	}, "\n")

	res, err := imp.Push(ctx, PushRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "dump",
		Reader:  strings.NewReader(dump),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.BatchID)
	assert.Equal(t, int64(2), res.Rows)

	batches, err := db.ListImportBatches(ctx, "loans")
	require.NoError(t, err)
	assert.Len(t, batches, 1)

	c, err := duckdb.New(ctx, duckdb.Config{
		DSN:          filepath.Join(datasetsDir, "loans__2026-07.duckdb"),
		MaxOpenConns: 1,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	var count int64
	require.NoError(t, c.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM data").Scan(&count))
	assert.Equal(t, int64(2), count)

	var nullMobile int64
	require.NoError(t, c.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM data WHERE mobile IS NULL").Scan(&nullMobile))
	assert.Equal(t, int64(1), nullMobile, "dump NULL must be stored as SQL NULL, not hashed or a literal")
}

func TestPushDryRun(t *testing.T) {
	ctx := context.Background()
	imp, db, stagingDir, datasetsDir := testImporter(t)

	res, err := imp.DryRun(ctx, PushRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(csvFileA()),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Columns)
	assert.Empty(t, res.Violations)

	batches, err := db.ListImportBatches(ctx, "loans")
	require.NoError(t, err)
	assert.Empty(t, batches, "DryRun writes no audit rows")

	registry, err := db.ListDatasets(ctx)
	require.NoError(t, err)
	assert.Empty(t, registry, "DryRun writes no registry rows")

	assertNoDuckDBInDir(t, stagingDir, "DryRun writes no staging files")
	assertNoDuckDBInDir(t, datasetsDir, "DryRun writes no dataset files")
}

func TestPushFailClosedUnknownColumn(t *testing.T) {
	ctx := context.Background()
	imp, db, stagingDir, datasetsDir := testImporter(t)

	rows := []string{
		"customer_name,mobile,cert_no,balance,overdue_days,update_ts,status,extra_col",
		"张三,138-0000-0000,11010119900101X,1250,5,2026-07-01,active,x",
	}
	_, err := imp.Push(ctx, PushRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(strings.Join(rows, "\n")),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extra_col")

	batches, err := db.ListImportBatches(ctx, "loans")
	require.NoError(t, err)
	assert.Empty(t, batches)
	assertNoDuckDBInDir(t, stagingDir, "unknown-column push leaves no staging residue")
	assertNoDuckDBInDir(t, datasetsDir, "unknown-column push leaves no dataset residue")
}

func TestRawNeverOnDisk(t *testing.T) {
	ctx := context.Background()

	// Snapshot the package-private temp root (not the system temp directory)
	// so the leak check is hermetic when other test packages run in parallel.
	tmpRoot := testTempRoot
	before := snapshotDirNames(t, tmpRoot)

	root, err := os.MkdirTemp(".", "raw-never-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	stagingDir := filepath.Join(root, "staging")
	datasetsDir := filepath.Join(root, "datasets")
	require.NoError(t, os.MkdirAll(stagingDir, 0o755))
	require.NoError(t, os.MkdirAll(datasetsDir, 0o755))

	db, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	imp := NewImporter(ImporterConfig{
		SiteSalt:    "test-site-salt-16bytes",
		RulesDir:    "../../datarules",
		KBRegistry:  testKBReg(),
		StagingDir:  stagingDir,
		DatasetsDir: datasetsDir,
	}, db, nil)

	// Human channel: Preview + Commit.
	previewResp, err := imp.Preview(ctx, PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(csvFileA()),
	})
	require.NoError(t, err)
	_, err = imp.Commit(ctx, CommitRequest{
		PreviewID: previewResp.PreviewID,
		Dataset:   "loans",
		Period:    "2026-07",
		Channel:   "csv",
		Reader:    strings.NewReader(csvFileA()),
	})
	require.NoError(t, err)

	// Machine channel: Push a different file for the same key.
	_, err = imp.Push(ctx, PushRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(csvFileB()),
	})
	require.NoError(t, err)

	// Library-level invariant: only .duckdb files exist, and staging is empty.
	assertNoRawFilesInDir(t, stagingDir, "staging")
	assertNoRawFilesInDir(t, datasetsDir, "datasets")

	// System temp invariant: no new top-level entries appeared.
	after := snapshotDirNames(t, tmpRoot)
	for name := range after {
		if _, ok := before[name]; !ok {
			t.Fatalf("package temp root gained a new top-level entry during import: %q", name)
		}
	}
}

func snapshotDirNames(t *testing.T, dir string) map[string]struct{} {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	out := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		out[e.Name()] = struct{}{}
	}
	return out
}

func assertNoRawFilesInDir(t *testing.T, dir, label string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".csv" || ext == ".sql" || ext == ".dump" {
			t.Fatalf("%s contains raw file: %s", label, path)
		}
		return nil
	})
	require.NoError(t, err)
}
