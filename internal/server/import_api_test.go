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

// Phase 15 Plan 06 — HTTP surface tests for the data-import endpoints.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/data"
	"github.com/skillforge/skillforge/internal/duckdb"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

const testAdminToken = "test-admin-token-15-06"

// testImportEnv wires SKILLFORGE_ADMIN_TOKEN for the duration of one test.
func testImportEnv(t *testing.T) {
	t.Helper()
	prev := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	t.Cleanup(func() {
		if prev == "" {
			_ = os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
		} else {
			_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", prev)
		}
	})
}

// importTestDeps returns a Deps with a real Importer backed by an in-memory
// SQLite store, a DuckDB in-memory connector, and temp staging/datasets dirs.
func importTestDeps(t *testing.T) (*Deps, func()) {
	t.Helper()
	testImportEnv(t)

	ctx := context.Background()
	root := t.TempDir()
	stagingDir := filepath.Join(root, "staging")
	datasetsDir := filepath.Join(root, "datasets")
	datarulesDir := filepath.Join(root, "datarules")
	require.NoError(t, os.MkdirAll(stagingDir, 0o755))
	require.NoError(t, os.MkdirAll(datasetsDir, 0o755))
	require.NoError(t, os.MkdirAll(datarulesDir, 0o755))
	writeLoansRules(t, datarulesDir)

	db, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)

	conn, err := duckdb.New(ctx, duckdb.Config{DSN: ":memory:", MaxOpenConns: 1})
	require.NoError(t, err)
	lru, err := duckdb.NewAttachLRU(conn, 8)
	require.NoError(t, err)

	imp := data.NewImporter(data.ImporterConfig{
		SiteSalt:    "test-site-salt-16bytes",
		RulesDir:    datarulesDir,
		StagingDir:  stagingDir,
		DatasetsDir: datasetsDir,
	}, db, lru)

	deps := &Deps{
		Importer:              imp,
		Store:                 db,
		ImportMaxBodyBytes:    ImportMaxBodyDefault,
		NonImportMaxBodyBytes: NonImportMaxBodyDefault,
	}
	cleanup := func() {
		_ = lru.Close()
		_ = conn.Close()
		_ = db.Close()
	}
	return deps, cleanup
}

func writeLoansRules(t *testing.T, dir string) {
	t.Helper()
	yaml := `dataset: loans
tier: 历史
k_min: 10
columns:
  customer_name: { rule: hash, profile: person_name }
  mobile:        { rule: hash, profile: phone_cn }
  cert_no:       { rule: hash, profile: id_card_cn }
  balance:       { rule: bucket_fixed, width: 1000 }
  overdue_days:  { rule: bucket_fixed, width: 30 }
  update_ts:     { rule: passthrough }
  status:        { rule: passthrough }
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "loans.yaml"), []byte(yaml), 0o644))
}

func csvRow(i int) string {
	return fmt.Sprintf("张三%d,138-0000-%04d,11010119900101%04d,%d,%d,2026-07-01,active", i, i, i, 1000+i*10, i%90)
}

func makeCSV(rows int) string {
	var b strings.Builder
	b.WriteString("customer_name,mobile,cert_no,balance,overdue_days,update_ts,status\n")
	for i := 0; i < rows; i++ {
		b.WriteString(csvRow(i))
		b.WriteByte('\n')
	}
	return b.String()
}

// buildMultipartForm constructs a multipart/form-data body. fields are text
// parts; fileContent becomes the "file" part. Returns the body bytes, the
// Content-Type value (including boundary), and the total body length.
func buildMultipartForm(t *testing.T, fields map[string]string, fileContent string) ([]byte, string, int) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		fw, err := w.CreateFormField(k)
		require.NoError(t, err)
		_, err = fw.Write([]byte(v))
		require.NoError(t, err)
	}
	fw, err := w.CreateFormFile("file", "upload.csv")
	require.NoError(t, err)
	_, err = fw.Write([]byte(fileContent))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes(), w.FormDataContentType(), buf.Len()
}

func invokeImport(t *testing.T, deps *Deps, method, path, token string, body []byte, contentType string, contentLength int) *ut.ResponseRecorder {
	t.Helper()
	h := New(":0", deps)
	headers := []ut.Header{{Key: "Content-Type", Value: contentType}}
	if token != "" {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: token})
	}
	return ut.PerformRequest(h.Engine, method, path, &ut.Body{Body: bytes.NewReader(body), Len: contentLength}, headers...)
}

// TestImportStreaming sends a >16 MB multipart push and asserts:
//   1. HTTP 200 + batch_id in response.
//   2. The batch lands in the SQLite import_batches audit table.
//   3. No new entries appear in os.TempDir() (Hertz did not pre-parse the
//      multipart form into a temp file).
func TestImportStreaming(t *testing.T) {
	deps, cleanup := importTestDeps(t)
	defer cleanup()

	tmpRoot := os.TempDir()
	before := snapshotTempNames(t, tmpRoot)

	// Build a CSV large enough to exceed 16 MiB.
	csv := makeCSV(500_000)
	require.Greater(t, len(csv), 16<<20, "test payload must exceed 16 MiB")

	fields := map[string]string{
		"dataset": "loans",
		"period":  "2026-07",
		"channel": "csv",
	}
	body, ct, n := buildMultipartForm(t, fields, csv)

	resp := invokeImport(t, deps, http.MethodPost, "/api/import/push", testAdminToken, body, ct, n)
	require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

	var cr commitResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &cr))
	assert.NotEmpty(t, cr.BatchID)
	assert.Greater(t, cr.Rows, int64(0))

	// Verify audit row exists.
	latest, err := deps.Store.LatestImportBatch(context.Background(), "loans", "2026-07")
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, cr.BatchID, latest.BatchID)
	assert.Equal(t, int64(len(csv)), latest.RawSizeBytes)

	// Verify no new top-level entries in os.TempDir().
	after := snapshotTempNames(t, tmpRoot)
	for name := range after {
		if _, ok := before[name]; !ok {
			t.Fatalf("os.TempDir() gained new top-level entry during streaming import: %q", name)
		}
	}
}

func snapshotTempNames(t *testing.T, dir string) map[string]struct{} {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	out := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		out[e.Name()] = struct{}{}
	}
	return out
}

// TestImportEndpointsAdminGated verifies the X-Admin-Token gate on all four
// import endpoints. Missing/wrong tokens → 403; correct token → not 403.
func TestImportEndpointsAdminGated(t *testing.T) {
	deps, cleanup := importTestDeps(t)
	defer cleanup()

	csv := makeCSV(1)
	body, ct, n := buildMultipartForm(t, map[string]string{
		"dataset": "loans",
		"period":  "2026-07",
		"channel": "csv",
	}, csv)

	for _, tc := range []struct {
		method string
		path   string
		body   []byte
		n      int
	}{
		{http.MethodPost, "/api/import/preview", body, n},
		{http.MethodPost, "/api/import/commit", body, n},
		{http.MethodPost, "/api/import/push", body, n},
		{http.MethodDelete, "/api/import/dataset/loans__2026-07", nil, 0},
	} {
		t.Run(tc.method+" "+tc.path+" no token", func(t *testing.T) {
			resp := invokeImport(t, deps, tc.method, tc.path, "", tc.body, ct, tc.n)
			assert.Equal(t, http.StatusForbidden, resp.Code)
		})
		t.Run(tc.method+" "+tc.path+" wrong token", func(t *testing.T) {
			resp := invokeImport(t, deps, tc.method, tc.path, "wrong-token", tc.body, ct, tc.n)
			assert.Equal(t, http.StatusForbidden, resp.Code)
		})
		t.Run(tc.method+" "+tc.path+" correct token", func(t *testing.T) {
			resp := invokeImport(t, deps, tc.method, tc.path, testAdminToken, tc.body, ct, tc.n)
			assert.NotEqual(t, http.StatusForbidden, resp.Code)
		})
	}
}

// TestImportSaltUnset503 verifies that when Deps.Importer is nil (the
// SKILLFORGE_DATA_SALT unset posture), all import endpoints return 503 with
// the expected error message.
func TestImportSaltUnset503(t *testing.T) {
	testImportEnv(t)
	deps := &Deps{Importer: nil}
	csv := makeCSV(1)
	body, ct, n := buildMultipartForm(t, map[string]string{
		"dataset": "loans",
		"period":  "2026-07",
		"channel": "csv",
	}, csv)

	for _, path := range []string{"/api/import/preview", "/api/import/commit", "/api/import/push"} {
		resp := invokeImport(t, deps, http.MethodPost, path, testAdminToken, body, ct, n)
		require.Equal(t, http.StatusServiceUnavailable, resp.Code, path)
		var ge graphResponseError
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &ge))
		assert.Contains(t, ge.Error, "SKILLFORGE_DATA_SALT")
	}

	resp := invokeImport(t, deps, http.MethodDelete, "/api/import/dataset/loans__2026-07", testAdminToken, nil, ct, 0)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

// TestPreviewEndpointShape verifies the preview response carries the
// expected shape: preview_id, sample raw↔desensitized pairs, proposed tier,
// and column metadata (name + rule + duckdb_type + width).
func TestPreviewEndpointShape(t *testing.T) {
	deps, cleanup := importTestDeps(t)
	defer cleanup()

	csv := makeCSV(5)
	body, ct, n := buildMultipartForm(t, map[string]string{
		"dataset": "loans",
		"period":  "2026-07",
		"channel": "csv",
	}, csv)

	resp := invokeImport(t, deps, http.MethodPost, "/api/import/preview", testAdminToken, body, ct, n)
	require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

	var pr previewResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &pr))
	assert.NotEmpty(t, pr.PreviewID)
	assert.Equal(t, "loans", pr.Dataset)
	assert.Equal(t, "2026-07", pr.Period)
	assert.Equal(t, "历史", pr.ProposedTier)
	assert.NotEmpty(t, pr.RulesSHA256)
	assert.NotEmpty(t, pr.ContentSHA256)
	assert.Equal(t, int64(5), pr.RowCount)
	require.Len(t, pr.SamplePairs, 5)
	require.NotEmpty(t, pr.Columns)

	first := pr.SamplePairs[0]
	require.Len(t, first.Raw, 7)
	require.Len(t, first.Desensitized, 7)
	assert.Equal(t, "张三0", first.Raw[0])
	assert.Len(t, first.Desensitized[0], 64) // HMAC hex

	colNames := make([]string, len(pr.Columns))
	for i, c := range pr.Columns {
		colNames[i] = c.Name
		assert.NotEmpty(t, c.Rule)
		assert.NotEmpty(t, c.DuckDBType)
	}
	assert.Equal(t, []string{"customer_name", "mobile", "cert_no", "balance", "overdue_days", "update_ts", "status"}, colNames)
}

// TestCommitViaPreview exercises the two-stage human channel:
// preview(fileA) → commit(preview_id + fileA) → 200 batch_id;
// commit(preview_id + fileB) → 400 hash mismatch.
func TestCommitViaPreview(t *testing.T) {
	deps, cleanup := importTestDeps(t)
	defer cleanup()

	csvA := makeCSV(3)
	bodyA, ctA, nA := buildMultipartForm(t, map[string]string{
		"dataset": "loans",
		"period":  "2026-07",
		"channel": "csv",
	}, csvA)

	resp := invokeImport(t, deps, http.MethodPost, "/api/import/preview", testAdminToken, bodyA, ctA, nA)
	require.Equal(t, http.StatusOK, resp.Code)
	var pr previewResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &pr))

	// Commit with the same file succeeds.
	bodyA2, ctA2, nA2 := buildMultipartForm(t, map[string]string{
		"dataset":    "loans",
		"period":     "2026-07",
		"channel":    "csv",
		"preview_id": pr.PreviewID,
	}, csvA)
	resp = invokeImport(t, deps, http.MethodPost, "/api/import/commit", testAdminToken, bodyA2, ctA2, nA2)
	require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())
	var cr commitResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &cr))
	assert.NotEmpty(t, cr.BatchID)
	assert.Equal(t, int64(3), cr.Rows)

	// Commit with a different file fails the hash bridge.
	csvB := csvA + csvRow(999) + "\n"
	bodyB, ctB, nB := buildMultipartForm(t, map[string]string{
		"dataset":    "loans",
		"period":     "2026-07",
		"channel":    "csv",
		"preview_id": pr.PreviewID,
	}, csvB)
	resp = invokeImport(t, deps, http.MethodPost, "/api/import/commit", testAdminToken, bodyB, ctB, nB)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	var ge graphResponseError
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &ge))
	assert.Contains(t, ge.Error, "hash mismatch")
}

// TestPushDryRunFlag verifies push with dry_run=true returns column/violation
// metadata and leaves no dataset or import_batches row behind.
func TestPushDryRunFlag(t *testing.T) {
	deps, cleanup := importTestDeps(t)
	defer cleanup()

	csv := makeCSV(1)
	body, ct, n := buildMultipartForm(t, map[string]string{
		"dataset": "loans",
		"period":  "2026-07",
		"channel": "csv",
		"dry_run": "true",
	}, csv)

	resp := invokeImport(t, deps, http.MethodPost, "/api/import/push", testAdminToken, body, ct, n)
	require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

	var dr dryRunResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &dr))
	require.NotEmpty(t, dr.Columns)
	// With valid sample data we expect no violations.
	assert.Empty(t, dr.Violations)

	// No batch was persisted.
	latest, err := deps.Store.LatestImportBatch(context.Background(), "loans", "2026-07")
	require.NoError(t, err)
	assert.Nil(t, latest)
}

// pushDatasetViaHTTP imports a small CSV via POST /api/import/push and returns
// the committed batch id. It is a test helper for the delete-endpoint suite.
func pushDatasetViaHTTP(t *testing.T, deps *Deps, dataset, period string, rows int) string {
	t.Helper()
	csv := makeCSV(rows)
	body, ct, n := buildMultipartForm(t, map[string]string{
		"dataset": dataset,
		"period":  period,
		"channel": "csv",
	}, csv)
	resp := invokeImport(t, deps, http.MethodPost, "/api/import/push", testAdminToken, body, ct, n)
	require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())
	var cr commitResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &cr))
	require.NotEmpty(t, cr.BatchID)
	return cr.BatchID
}

// TestDeleteDatasetAudited proves DELETE after a lazy ATTACH succeeds on
// Windows (Pitfall 9) and leaves file/registry/dataset_deletes audit in sync.
func TestDeleteDatasetAudited(t *testing.T) {
	ctx := context.Background()
	deps, cleanup := importTestDeps(t)
	defer cleanup()

	pushDatasetViaHTTP(t, deps, "loans", "2026-07", 2)
	pushDatasetViaHTTP(t, deps, "loans", "2026-06", 1)

	// Find the target file path in the registry.
	rows, err := deps.Store.ListDatasets(ctx)
	require.NoError(t, err)
	var target string
	for _, r := range rows {
		if r.Name == "loans__2026-07" {
			target = r.FilePath
			break
		}
	}
	require.NotEmpty(t, target)

	// Trigger a lazy ATTACH via the importer's own LRU — this is the Windows
	// lock-order regression surface: the file is now held open by the engine.
	lru := deps.Importer.LRU()
	require.NotNil(t, lru)
	_, err = lru.Get(ctx, "loans__2026-07", target)
	require.NoError(t, err)

	resp := invokeImport(t, deps, http.MethodDelete, "/api/import/dataset/loans__2026-07", testAdminToken, nil, "", 0)
	require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())
	var dr deleteResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &dr))
	assert.Equal(t, []string{"loans__2026-07"}, dr.Deleted)

	// File removed.
	_, err = os.Stat(target)
	require.True(t, os.IsNotExist(err), "dataset file must be removed")

	// Registry row removed, sibling period intact.
	remaining, err := deps.Store.ListDatasets(ctx)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, "loans__2026-06", remaining[0].Name)

	// dataset_deletes audit row exists.
	sqlDB := deps.Store.(*sqlite.SQLiteStore).DB()
	var name, dataset string
	err = sqlDB.QueryRowContext(ctx,
		"SELECT name, dataset FROM dataset_deletes WHERE name = ?", "loans__2026-07").
		Scan(&name, &dataset)
	require.NoError(t, err)
	assert.Equal(t, "loans__2026-07", name)
	assert.Equal(t, "loans", dataset)
}

// TestDeleteDatasetLogicalAllPeriods deletes by logical dataset name and
// removes every period while writing one audit row per deleted registry name.
func TestDeleteDatasetLogicalAllPeriods(t *testing.T) {
	ctx := context.Background()
	deps, cleanup := importTestDeps(t)
	defer cleanup()

	pushDatasetViaHTTP(t, deps, "loans", "2026-07", 1)
	pushDatasetViaHTTP(t, deps, "loans", "2026-06", 1)

	resp := invokeImport(t, deps, http.MethodDelete, "/api/import/dataset/loans", testAdminToken, nil, "", 0)
	require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())
	var dr deleteResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &dr))
	assert.ElementsMatch(t, []string{"loans__2026-06", "loans__2026-07"}, dr.Deleted)

	remaining, err := deps.Store.ListDatasets(ctx)
	require.NoError(t, err)
	assert.Empty(t, remaining)

	sqlDB := deps.Store.(*sqlite.SQLiteStore).DB()
	var count int
	require.NoError(t, sqlDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM dataset_deletes WHERE dataset = ?", "loans").Scan(&count))
	assert.Equal(t, 2, count)
}

// TestDeleteDatasetNotFound verifies an unmatched name returns 404.
func TestDeleteDatasetNotFound(t *testing.T) {
	deps, cleanup := importTestDeps(t)
	defer cleanup()

	resp := invokeImport(t, deps, http.MethodDelete, "/api/import/dataset/loans__2026-07", testAdminToken, nil, "", 0)
	require.Equal(t, http.StatusNotFound, resp.Code)
}

// TestDeleteDatasetUnauthorized verifies missing/wrong tokens → 403 and no
// dataset_deletes audit row is written.
func TestDeleteDatasetUnauthorized(t *testing.T) {
	ctx := context.Background()
	deps, cleanup := importTestDeps(t)
	defer cleanup()

	pushDatasetViaHTTP(t, deps, "loans", "2026-07", 1)

	resp := invokeImport(t, deps, http.MethodDelete, "/api/import/dataset/loans__2026-07", "", nil, "", 0)
	require.Equal(t, http.StatusForbidden, resp.Code)

	resp = invokeImport(t, deps, http.MethodDelete, "/api/import/dataset/loans__2026-07", "bad-token", nil, "", 0)
	require.Equal(t, http.StatusForbidden, resp.Code)

	sqlDB := deps.Store.(*sqlite.SQLiteStore).DB()
	var count int
	require.NoError(t, sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM dataset_deletes").Scan(&count))
	assert.Equal(t, 0, count)
}
