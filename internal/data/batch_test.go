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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skillforge/skillforge/internal/duckdb"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testImporter(t *testing.T) (*Importer, store.Store, string, string) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	root := t.TempDir()
	stagingDir := filepath.Join(root, "staging")
	datasetsDir := filepath.Join(root, "datasets")
	require.NoError(t, os.MkdirAll(stagingDir, 0o755))
	require.NoError(t, os.MkdirAll(datasetsDir, 0o755))

	imp := NewImporter(ImporterConfig{
		SiteSalt:    "test-site-salt-16bytes",
		RulesDir:    "../../datarules",
		KBRegistry:  testKBReg(),
		StagingDir:  stagingDir,
		DatasetsDir: datasetsDir,
	}, db, nil)
	return imp, db, stagingDir, datasetsDir
}

func csvFileA() string {
	rows := []string{
		"customer_name,mobile,cert_no,balance,overdue_days,update_ts,status",
		"张三,138-0000-0000,11010119900101X,1250,5,2026-07-01,active",
		"李四,138-0000-0001,11010119900101Y,3500,12,2026-07-01,active",
	}
	return strings.Join(rows, "\n")
}

func csvFileB() string {
	rows := []string{
		"customer_name,mobile,cert_no,balance,overdue_days,update_ts,status",
		"张三,138-0000-0000,11010119900101X,9999,5,2026-07-01,active",
		"李四,138-0000-0001,11010119900101Y,3500,12,2026-07-01,active",
		"王五,138-0000-0002,11010119900101Z,700,0,2026-07-01,active",
	}
	return strings.Join(rows, "\n")
}

// oneShotReader wraps a reader and fails the test if it is read again after
// returning io.EOF. This locks the single-pass consumption contract.
type oneShotReader struct {
	r      io.Reader
	t      *testing.T
	sawEOF bool
}

func (o *oneShotReader) Read(p []byte) (int, error) {
	if o.sawEOF {
		// A caller that tries to start a second pass would return data here.
		n, err := o.r.Read(p)
		if n > 0 || err != io.EOF {
			o.t.Fatalf("req.Reader was read again after EOF — single-pass contract violated (n=%d err=%v)", n, err)
		}
		return 0, io.EOF
	}
	n, err := o.r.Read(p)
	if err == io.EOF {
		o.sawEOF = true
	}
	return n, err
}

func TestPreviewCommitHashBridge(t *testing.T) {
	ctx := context.Background()
	imp, db, stagingDir, _ := testImporter(t)

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
		Reader:    &oneShotReader{r: strings.NewReader(csvFileB()), t: t},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "preview content hash mismatch")

	batches, err := db.ListImportBatches(ctx, "loans")
	require.NoError(t, err)
	assert.Empty(t, batches, "zero audit rows on hash mismatch")

	registry, err := db.ListDatasets(ctx)
	require.NoError(t, err)
	assert.Empty(t, registry, "zero registry rows on hash mismatch")

	assertNoDuckDBInDir(t, stagingDir, "staging should contain no .duckdb after mismatch cleanup")
}

func TestIdempotentReimport(t *testing.T) {
	ctx := context.Background()
	imp, db, _, datasetsDir := testImporter(t)

	// First import.
	preview1, err := imp.Preview(ctx, PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(csvFileA()),
	})
	require.NoError(t, err)
	res1, err := imp.Commit(ctx, CommitRequest{
		PreviewID: preview1.PreviewID,
		Dataset:   "loans",
		Period:    "2026-07",
		Channel:   "csv",
		Reader:    &oneShotReader{r: strings.NewReader(csvFileA()), t: t},
	})
	require.NoError(t, err)
	assert.False(t, res1.Superseded)
	assert.NotEmpty(t, res1.BatchID)

	stat1, err := os.Stat(filepath.Join(datasetsDir, "loans__2026-07.duckdb"))
	require.NoError(t, err)

	// Re-import identical file.
	preview2, err := imp.Preview(ctx, PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(csvFileA()),
	})
	require.NoError(t, err)
	res2, err := imp.Commit(ctx, CommitRequest{
		PreviewID: preview2.PreviewID,
		Dataset:   "loans",
		Period:    "2026-07",
		Channel:   "csv",
		Reader:    &oneShotReader{r: strings.NewReader(csvFileA()), t: t},
	})
	require.NoError(t, err)
	assert.Equal(t, res1.BatchID, res2.BatchID)
	assert.False(t, res2.Superseded)

	batches, err := db.ListImportBatches(ctx, "loans")
	require.NoError(t, err)
	assert.Len(t, batches, 1, "idempotent reimport adds no audit row")

	stat2, err := os.Stat(filepath.Join(datasetsDir, "loans__2026-07.duckdb"))
	require.NoError(t, err)
	assert.Equal(t, stat1.ModTime(), stat2.ModTime(), "dataset file unchanged")
}

func TestSupersedeChainAudited(t *testing.T) {
	ctx := context.Background()
	imp, db, _, datasetsDir := testImporter(t)

	preview1, err := imp.Preview(ctx, PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(csvFileA()),
	})
	require.NoError(t, err)
	res1, err := imp.Commit(ctx, CommitRequest{
		PreviewID: preview1.PreviewID,
		Dataset:   "loans",
		Period:    "2026-07",
		Channel:   "csv",
		Reader:    strings.NewReader(csvFileA()),
	})
	require.NoError(t, err)
	assert.False(t, res1.Superseded)

	preview2, err := imp.Preview(ctx, PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(csvFileB()),
	})
	require.NoError(t, err)
	res2, err := imp.Commit(ctx, CommitRequest{
		PreviewID: preview2.PreviewID,
		Dataset:   "loans",
		Period:    "2026-07",
		Channel:   "csv",
		Reader:    strings.NewReader(csvFileB()),
	})
	require.NoError(t, err)
	assert.True(t, res2.Superseded)
	assert.Equal(t, res1.BatchID, res2.SupersededBatchID)

	batches, err := db.ListImportBatches(ctx, "loans")
	require.NoError(t, err)
	require.Len(t, batches, 2)
	assert.Equal(t, res2.BatchID, batches[0].BatchID)
	assert.Equal(t, res1.BatchID, batches[0].SupersedesBatchID)

	registry, err := db.ListDatasets(ctx)
	require.NoError(t, err)
	require.Len(t, registry, 1)
	assert.Equal(t, "loans__2026-07", registry[0].Name)
	assert.Equal(t, res2.BatchID, registry[0].BatchID)

	// The on-disk dataset must contain fileB's three desensitized rows.
	count := rowCountInDataset(t, ctx, datasetsDir, "loans__2026-07")
	assert.Equal(t, int64(3), count)
}

func TestCommitSessionExpired(t *testing.T) {
	ctx := context.Background()
	imp, _, _, _ := testImporter(t)

	_, err := imp.Commit(ctx, CommitRequest{
		PreviewID: "no-such-session",
		Dataset:   "loans",
		Period:    "2026-07",
		Channel:   "csv",
		Reader:    strings.NewReader(csvFileA()),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "preview session not found or expired")
}

func assertNoDuckDBInDir(t *testing.T, dir string, msg string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".duckdb") {
			t.Fatalf("%s: found %s", msg, path)
		}
		return nil
	})
	require.NoError(t, err)
}

func rowCountInDataset(t *testing.T, ctx context.Context, datasetsDir, name string) int64 {
	t.Helper()
	c, err := duckdb.New(ctx, duckdb.Config{
		DSN:          filepath.Join(datasetsDir, name+".duckdb"),
		MaxOpenConns: 1,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	var count int64
	require.NoError(t, c.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM data").Scan(&count))
	return count
}

// Compile-time check that store.Store satisfies ImportStoreSeam.
var _ ImportStoreSeam = (*sqlite.SQLiteStore)(nil)
