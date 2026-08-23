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

// Package data is the Phase 15 import-pipeline domain layer.
package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"iter"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/skillforge/skillforge/internal/duckdb"
	"github.com/skillforge/skillforge/internal/store"
)

// ImportMaxRowsPerBatch is the hard per-batch row ceiling. It is a
// source-grep locked literal (scripts/verify-phase-15.sh Stage 9) — not a
// config knob.
const ImportMaxRowsPerBatch = 50_000_000

// periodRegexp enforces the explicit YYYY-MM period format (D15-C2).
var periodRegexp = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// ImporterConfig parameterizes the import orchestrator.
type ImporterConfig struct {
	// SiteSalt is the global per-deployment salt. Empty → every import
	// action fails closed.
	SiteSalt string
	// RulesDir is the directory holding datarules/{dataset}.yaml files.
	// Empty falls back to "datarules".
	RulesDir string
	// StagingDir is the directory for in-flight staging .duckdb files.
	// Empty falls back to duckdb.DefaultStagingDir.
	StagingDir string
	// DatasetsDir is the directory for committed dataset .duckdb files.
	// Empty falls back to duckdb.DefaultDatasetsDir.
	DatasetsDir string
}

func (c ImporterConfig) rulesDir() string {
	if c.RulesDir == "" {
		return "datarules"
	}
	return c.RulesDir
}

func (c ImporterConfig) stagingDir() string {
	if c.StagingDir == "" {
		return duckdb.DefaultStagingDir
	}
	return c.StagingDir
}

func (c ImporterConfig) datasetsDir() string {
	if c.DatasetsDir == "" {
		return duckdb.DefaultDatasetsDir
	}
	return c.DatasetsDir
}

// ImportStoreSeam is the narrow store surface the importer needs.
// It mirrors the interface-isolation precedent in duckdb.DatasetRegistry.
type ImportStoreSeam interface {
	PutImportBatch(ctx context.Context, row store.ImportBatchRow) error
	LatestImportBatch(ctx context.Context, dataset, period string) (*store.ImportBatchRow, error)
	PutDataset(ctx context.Context, row store.DatasetRow) error
	ListDatasets(ctx context.Context) ([]store.DatasetRow, error)
	PutDatasetDelete(ctx context.Context, row store.DatasetDeleteRow) error
}

// Importer orchestrates the three import channels: Preview, Commit, and Push.
type Importer struct {
	cfg      ImporterConfig
	seam     ImportStoreSeam
	sessions *SessionStore
	lru      *duckdb.AttachLRU
}

// NewImporter constructs an Importer with an internal SessionStore.
func NewImporter(cfg ImporterConfig, seam ImportStoreSeam, lru *duckdb.AttachLRU) *Importer {
	return &Importer{
		cfg:      cfg,
		seam:     seam,
		sessions: NewSessionStore(),
		lru:      lru,
	}
}

// LRU returns the lazy-ATTACH registry for tests that need to prove the
// Windows lock-order boundary (Pitfall 9). It may be nil.
func (imp *Importer) LRU() *duckdb.AttachLRU { return imp.lru }

// PreviewRequest is the human-channel preview call.
type PreviewRequest struct {
	Dataset         string
	Period          string
	Channel         string // "csv" or "dump"
	Reader          io.Reader
	BucketOverrides map[string]float64
}

// PreviewResponse is the preview result shown to the data staff.
type PreviewResponse struct {
	PreviewID     string
	Dataset       string
	Period        string
	ProposedTier  string
	RulesSHA256   string
	ContentSHA256 string
	RowCount      int64
	Columns       []PreviewColumn
	SamplePairs   []RowPair
}

// PreviewColumn describes one column's rule and inferred storage type.
type PreviewColumn struct {
	Name       string
	Rule       string
	Profile    string
	DuckDBType string
	Width      float64
}

// CommitRequest is the second leg of the two-stage import.
type CommitRequest struct {
	PreviewID string
	Dataset   string
	Period    string
	Channel   string
	Reader    io.Reader
}

// CommitResult is returned by Commit and Push.
type CommitResult struct {
	BatchID           string
	Rows              int64
	Superseded        bool
	SupersededBatchID string
}

// PushRequest is the machine-channel import call.
type PushRequest struct {
	Dataset string
	Period  string
	Channel string
	Reader  io.Reader
	DryRun  bool
}

// DryRunResult is the zero-side-effect validation payload for Push(DryRun=true).
type DryRunResult struct {
	Columns    []PreviewColumn
	Violations []string
}

// Preview runs the preview path: decode → rule gate → desensitize the first
// 100 rows → infer schema → save an in-memory session. No raw bytes are ever
// written to disk.
func (imp *Importer) Preview(ctx context.Context, req PreviewRequest) (resp *PreviewResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = recoverRowIterError(r)
		}
	}()

	if err := imp.guardSiteSalt(); err != nil {
		return nil, err
	}
	if err := imp.guardDatasetPeriod(req.Dataset, req.Period); err != nil {
		return nil, err
	}
	rules, rulesSHA, err := imp.loadRules(req.Dataset)
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	cw := &countingHashWriter{h: hash}
	tee := io.TeeReader(req.Reader, cw)

	header, rowIter, err := openChannel(req.Channel, tee)
	if err != nil {
		return nil, err
	}
	if err := ValidateColumnSet(rules, header); err != nil {
		return nil, err
	}

	datasetSalt, err := DeriveDatasetSalt(imp.cfg.SiteSalt, req.Dataset)
	if err != nil {
		return nil, err
	}

	var (
		rowCount    int64
		samplePairs []RowPair
	)
	for row := range rowIter {
		if rowCount >= ImportMaxRowsPerBatch {
			return nil, fmtError("Preview", fmt.Errorf(
				"row count exceeds ImportMaxRowsPerBatch=%d", ImportMaxRowsPerBatch))
		}
		desensitized, err := desensitizeRow(row, header, rules, datasetSalt, req.BucketOverrides)
		if err != nil {
			return nil, err
		}
		if len(samplePairs) < 100 {
			samplePairs = append(samplePairs, RowPair{
				Raw:          append([]string(nil), row...),
				Desensitized: desensitizedToStrings(desensitized),
			})
		}
		rowCount++
	}

	schemaTypes := inferSchemaTypes(header, rules, samplePairs)
	contentSHA := hex.EncodeToString(hash.Sum(nil))

	previewID, err := randomHex(16)
	if err != nil {
		return nil, fmtError("Preview", err)
	}
	sess := &PreviewSession{
		PreviewID:       previewID,
		ContentSHA256:   contentSHA,
		Dataset:         req.Dataset,
		Period:          req.Period,
		RulesSHA256:     rulesSHA,
		ProposedTier:    rules.Tier,
		BucketOverrides: cloneOverrides(req.BucketOverrides),
		SchemaTypes:     schemaTypes,
		SampleRows:      samplePairs,
		RowCount:        rowCount,
		CreatedAt:       time.Now().UTC(),
	}
	imp.sessions.Save(sess)

	resp = &PreviewResponse{
		PreviewID:     previewID,
		Dataset:       req.Dataset,
		Period:        req.Period,
		ProposedTier:  rules.Tier,
		RulesSHA256:   rulesSHA,
		ContentSHA256: contentSHA,
		RowCount:      rowCount,
		Columns:       buildPreviewColumns(header, rules, schemaTypes),
		SamplePairs:   samplePairs,
	}
	return resp, nil
}

// Commit validates the preview session and promotes the re-uploaded file.
// The caller's Reader is consumed exactly once; hashing and idempotency checks
// happen only after the staging stream is exhausted.
func (imp *Importer) Commit(ctx context.Context, req CommitRequest) (*CommitResult, error) {
	return imp.finalizeBatch(ctx, req.PreviewID, req.Dataset, req.Period, req.Channel, req.Reader)
}

// Push is the machine-channel import. DryRun requests are rejected here;
// use DryRun() for zero-side-effect validation.
func (imp *Importer) Push(ctx context.Context, req PushRequest) (*CommitResult, error) {
	if req.DryRun {
		return nil, fmtError("Push", errors.New("dry-run requests must use Importer.DryRun"))
	}
	return imp.pushBatch(ctx, req.Dataset, req.Period, req.Channel, req.Reader)
}

// DryRun runs a zero-side-effect validation of the upload: it decodes the
// header, checks the column gate, buffers up to 100 rows, infers types, and
// reports any rule violations. No staging file, registry row, or audit row is
// written.
func (imp *Importer) DryRun(ctx context.Context, req PushRequest) (*DryRunResult, error) {
	if err := imp.guardSiteSalt(); err != nil {
		return nil, err
	}
	if err := imp.guardDatasetPeriod(req.Dataset, req.Period); err != nil {
		return nil, err
	}
	rules, _, err := imp.loadRules(req.Dataset)
	if err != nil {
		return nil, err
	}

	header, src, err := openChannelSource(req.Channel, req.Reader)
	if err != nil {
		return nil, err
	}
	if err := ValidateColumnSet(rules, header); err != nil {
		return nil, err
	}

	var sampleRows [][]string
	for len(sampleRows) < 100 {
		row, err := src.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		sampleRows = append(sampleRows, row)
	}

	schemaTypes := inferSchemaTypesFromRows(header, rules, sampleRows)
	columns := buildPreviewColumns(header, rules, schemaTypes)
	violations := dryRunViolations(header, rules, schemaTypes, sampleRows)
	return &DryRunResult{Columns: columns, Violations: violations}, nil
}

// pushBatch is the machine-channel write path. It buffers the first 100 rows
// for type inference, then streams the remainder into WriteStagingBatch without
// re-reading the input.
func (imp *Importer) pushBatch(ctx context.Context, dataset, period, channel string, r io.Reader) (*CommitResult, error) {
	if err := imp.guardSiteSalt(); err != nil {
		return nil, err
	}
	if err := imp.guardDatasetPeriod(dataset, period); err != nil {
		return nil, err
	}
	rules, rulesSHA, err := imp.loadRules(dataset)
	if err != nil {
		return nil, err
	}

	datasetSalt, err := DeriveDatasetSalt(imp.cfg.SiteSalt, dataset)
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	cw := &countingHashWriter{h: hash}
	tee := io.TeeReader(r, cw)

	header, src, err := openChannelSource(channel, tee)
	if err != nil {
		return nil, err
	}
	if err := ValidateColumnSet(rules, header); err != nil {
		return nil, err
	}

	var sampleRows [][]string
	for len(sampleRows) < 100 {
		row, err := src.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		sampleRows = append(sampleRows, row)
	}

	schemaTypes := inferSchemaTypesFromRows(header, rules, sampleRows)
	buf := &bufferedSource{buffer: sampleRows, src: src}
	discardTS := time.Now().UTC().Format(time.RFC3339Nano)

	batchID, err := newBatchID()
	if err != nil {
		return nil, fmtError("pushBatch", err)
	}
	stagingPath, rowCount, err := imp.writeStaging(
		ctx, batchID, dataset, header, buf, rules, datasetSalt, nil, schemaTypes, cw)
	if err != nil {
		return nil, err
	}

	contentSHA := hex.EncodeToString(cw.Sum(nil))
	rawSize := cw.n

	return imp.commitOrSupersede(ctx, batchID, dataset, period, rules, rulesSHA,
		header, schemaTypes, stagingPath, contentSHA, rawSize, rowCount, discardTS)
}

// guardSiteSalt mirrors the isAdminRequest unset-env fail-closed posture.
func (imp *Importer) guardSiteSalt() error {
	if imp.cfg.SiteSalt == "" {
		return fmtError("guardSiteSalt", errors.New(
			"site salt is empty — set SKILLFORGE_DATA_SALT (fail-closed)"))
	}
	return nil
}

// guardDatasetPeriod validates the dataset id and the explicit period format.
func (imp *Importer) guardDatasetPeriod(dataset, period string) error {
	if err := duckdb.ValidateDatasetID(dataset); err != nil {
		return fmtError("guardDatasetPeriod", err)
	}
	if !periodRegexp.MatchString(period) {
		return fmtError("guardDatasetPeriod", fmt.Errorf(
			"period %q must be YYYY-MM (e.g. 2026-07)", period))
	}
	return nil
}

// loadRules loads the controlled rule file for dataset.
func (imp *Importer) loadRules(dataset string) (*Rules, string, error) {
	path := filepath.Join(imp.cfg.rulesDir(), dataset+".yaml")
	rules, sha, err := LoadRules(path)
	if err != nil {
		return nil, "", fmtError("loadRules", err)
	}
	return rules, sha, nil
}

// openChannel decodes the upload stream into a header and a row iterator.
// Supported channels: "csv", "dump". The returned iterator panics with an
// error value on decode failures; callers recover via recoverRowIterError.
func openChannel(channel string, r io.Reader) (header []string, rows iter.Seq[[]string], err error) {
	header, src, err := openChannelSource(channel, r)
	if err != nil {
		return nil, nil, err
	}
	return header, toIter(src), nil
}

// rowSource is a pull-style row stream. It is used by Push so the first 100
// rows can be buffered for type inference while the remainder stays streamed.
type rowSource interface {
	Next() ([]string, error)
}

// openChannelSource returns the header and a resumable rowSource for the
// channel. Callers that only need iter.Seq should use openChannel.
func openChannelSource(channel string, r io.Reader) (header []string, src rowSource, err error) {
	switch channel {
	case "csv":
		return openCSVSource(r)
	case "dump":
		return openDumpSource(r)
	default:
		return nil, nil, fmtError("openChannel", fmt.Errorf(
			"unknown import channel %q (valid: csv, dump)", channel))
	}
}

func toIter(src rowSource) iter.Seq[[]string] {
	return func(yield func([]string) bool) {
		for {
			rec, err := src.Next()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				panic(rowIterError{err})
			}
			if !yield(rec) {
				return
			}
		}
	}
}

// bufferedSource replays an in-memory prefix before delegating to the upstream
// source. It is what makes Push a true single-pass stream: the first 100 rows
// are inspected for type inference and then yielded again to the stager.
type bufferedSource struct {
	buffer [][]string
	idx    int
	src    rowSource
}

func (b *bufferedSource) Next() ([]string, error) {
	if b.idx < len(b.buffer) {
		row := b.buffer[b.idx]
		b.idx++
		return row, nil
	}
	return b.src.Next()
}

type csvSource struct {
	r *csv.Reader
}

func (s *csvSource) Next() ([]string, error) {
	rec, err := s.r.Read()
	if errors.Is(err, io.EOF) {
		return nil, io.EOF
	}
	if err != nil {
		return nil, fmtError("openCSVChannel", err)
	}
	return rec, nil
}

func openCSVSource(r io.Reader) ([]string, rowSource, error) {
	decoded, err := DecodeStream(r)
	if err != nil {
		return nil, nil, err
	}
	cr := NewCSVReader(decoded)
	header, err := cr.Read()
	if err != nil {
		return nil, nil, fmtError("openCSVChannel", fmt.Errorf("read CSV header: %w", err))
	}
	return header, &csvSource{r: cr}, nil
}

type dumpSource struct {
	rows [][]string
	idx  int
}

func (s *dumpSource) Next() ([]string, error) {
	if s.idx >= len(s.rows) {
		return nil, io.EOF
	}
	row := s.rows[s.idx]
	s.idx++
	return row, nil
}

func openDumpSource(r io.Reader) ([]string, rowSource, error) {
	res, err := ParseDump(r)
	if err != nil {
		return nil, nil, err
	}
	header := make([]string, 0, len(res.Columns))
	for _, c := range res.Columns {
		header = append(header, c.Name)
	}
	return header, &dumpSource{rows: res.Rows}, nil
}

// rowIterError is the panic payload used by openChannel iterators to surface
// decode errors through iter.Seq.
type rowIterError struct{ err error }

func recoverRowIterError(r any) error {
	if r == nil {
		return nil
	}
	if e, ok := r.(rowIterError); ok {
		return e.err
	}
	if e, ok := r.(error); ok {
		return e
	}
	return fmt.Errorf("panic: %v", r)
}

// desensitizeRow transforms one raw row into a row of database values.
// DumpNullValue cells become SQL NULL (nil) regardless of rule kind.
func desensitizeRow(row, header []string, rules *Rules, datasetSalt []byte, overrides map[string]float64) ([]any, error) {
	if len(row) != len(header) {
		return nil, fmtError("desensitizeRow", fmt.Errorf(
			"row length %d does not match header length %d", len(row), len(header)))
	}
	out := make([]any, len(header))
	for i, col := range header {
		v := row[i]
		if v == DumpNullValue {
			out[i] = nil
			continue
		}
		rule, ok := rules.Columns[col]
		if !ok {
			return nil, fmtError("desensitizeRow", fmt.Errorf(
				"column %q has no rule (should have been rejected by ValidateColumnSet)", col))
		}
		switch rule.Rule {
		case RuleHash:
			normalized, err := Normalize(rule.Profile, v)
			if err != nil {
				return nil, fmtError("desensitizeRow", fmt.Errorf("column %q: %w", col, err))
			}
			out[i] = L1Hash(datasetSalt, col, normalized)
		case RuleBucketFixed:
			if v == "" {
				return nil, fmtError("desensitizeRow", fmt.Errorf(
					"column %q: bucket column requires a numeric value but got empty string", col))
			}
			val, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, fmtError("desensitizeRow", fmt.Errorf(
					"column %q: cannot parse %q as float: %w", col, v, err))
			}
			width := rule.Width
			if w, ok := overrides[col]; ok {
				width = w
			}
			out[i] = FixedBucket(val, width)
		case RulePassthrough:
			out[i] = v
		default:
			return nil, fmtError("desensitizeRow", fmt.Errorf(
				"column %q: unsupported rule %q", col, rule.Rule))
		}
	}
	return out, nil
}

// inferSchemaTypes returns the DuckDB type for each column. Rule-declared
// duckdb_type takes precedence; otherwise the rule kind decides the storage
// shape (hash → VARCHAR, bucket_fixed → DOUBLE), and only passthrough columns
// fall back to sampling the raw strings.
func inferSchemaTypes(header []string, rules *Rules, samples []RowPair) map[string]string {
	byCol := make(map[string][]string, len(header))
	for _, pair := range samples {
		for i, col := range header {
			byCol[col] = append(byCol[col], pair.Raw[i])
		}
	}
	return inferSchemaTypesFromByCol(header, rules, byCol)
}

// inferSchemaTypesFromRows is the Push/DryRun variant: the buffered rows are
// raw string slices instead of RowPairs.
func inferSchemaTypesFromRows(header []string, rules *Rules, rows [][]string) map[string]string {
	byCol := make(map[string][]string, len(header))
	for _, row := range rows {
		for i, col := range header {
			byCol[col] = append(byCol[col], row[i])
		}
	}
	return inferSchemaTypesFromByCol(header, rules, byCol)
}

func inferSchemaTypesFromByCol(header []string, rules *Rules, byCol map[string][]string) map[string]string {
	out := make(map[string]string, len(header))
	for _, col := range header {
		rule := rules.Columns[col]
		if rule.DuckDBType != "" {
			out[col] = rule.DuckDBType
			continue
		}
		switch rule.Rule {
		case RuleHash:
			out[col] = "VARCHAR"
		case RuleBucketFixed:
			out[col] = "DOUBLE"
		default:
			out[col] = inferDuckDBType(byCol[col])
		}
	}
	return out
}

func inferDuckDBType(samples []string) string {
	allInt := true
	allFloat := true
	for _, s := range samples {
		if s == DumpNullValue {
			continue
		}
		if s == "" {
			allInt = false
			allFloat = false
			continue
		}
		if _, err := strconv.ParseInt(s, 10, 64); err != nil {
			allInt = false
		}
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			allFloat = false
		}
	}
	if allInt && len(samples) > 0 {
		return "BIGINT"
	}
	if allFloat && len(samples) > 0 {
		return "DOUBLE"
	}
	return "VARCHAR"
}

func buildPreviewColumns(header []string, rules *Rules, schemaTypes map[string]string) []PreviewColumn {
	out := make([]PreviewColumn, 0, len(header))
	for _, col := range header {
		rule := rules.Columns[col]
		out = append(out, PreviewColumn{
			Name:       col,
			Rule:       rule.Rule,
			Profile:    rule.Profile,
			DuckDBType: schemaTypes[col],
			Width:      rule.Width,
		})
	}
	return out
}

// dryRunViolations inspects the buffered sample and reports rule/value mismatches
// that would fail during desensitization. Unknown columns are rejected earlier by
// ValidateColumnSet; this function covers per-cell shape violations.
func dryRunViolations(header []string, rules *Rules, schemaTypes map[string]string, rows [][]string) []string {
	colIdx := make(map[string]int, len(header))
	for i, col := range header {
		colIdx[col] = i
	}
	var violations []string
	for _, col := range header {
		rule := rules.Columns[col]
		for i, row := range rows {
			v := row[colIdx[col]]
			if v == DumpNullValue {
				continue
			}
			switch rule.Rule {
			case RuleBucketFixed:
				if v == "" {
					violations = append(violations, fmt.Sprintf(
						"row %d column %q: bucket column is empty", i+1, col))
					continue
				}
				if _, err := strconv.ParseFloat(v, 64); err != nil {
					violations = append(violations, fmt.Sprintf(
						"row %d column %q: cannot parse %q as number", i+1, col, v))
				}
			case RuleHash:
				if v == "" {
					violations = append(violations, fmt.Sprintf(
						"row %d column %q: hash column is empty", i+1, col))
				}
			}
		}
	}
	return violations
}

func desensitizedToStrings(row []any) []string {
	out := make([]string, len(row))
	for i, v := range row {
		if v == nil {
			out[i] = "NULL"
			continue
		}
		out[i] = fmt.Sprintf("%v", v)
	}
	return out
}

func cloneOverrides(m map[string]float64) map[string]float64 {
	if m == nil {
		return nil
	}
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// countingHashWriter counts bytes and computes a SHA-256 digest.
type countingHashWriter struct {
	h hash.Hash
	n int64
}

func (w *countingHashWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return w.h.Write(p)
}

func (w *countingHashWriter) Sum(b []byte) []byte {
	return w.h.Sum(b)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// finalizeBatch is the single-pass write path shared by Commit and Push.
//
// Invariants:
//   - The input Reader is consumed exactly once.
//   - TeeReader→hasher is wrapped around the Reader BEFORE openChannel, so
//     the digest covers the same raw bytes as Preview.
//   - ValidateCommitAgainstPreview and checkIdempotent run ONLY after the
//     WriteStagingBatch stream is exhausted.
func (imp *Importer) finalizeBatch(ctx context.Context, previewID, dataset, period, channel string, r io.Reader) (*CommitResult, error) {
	// Session lookup happens before any stream consumption.
	sess, ok := imp.sessions.Get(previewID)
	if !ok {
		return nil, fmtError("finalizeBatch", errExpiredSession)
	}
	if err := imp.guardSiteSalt(); err != nil {
		return nil, err
	}
	if err := imp.guardDatasetPeriod(dataset, period); err != nil {
		return nil, err
	}
	rules, rulesSHA, err := imp.loadRules(dataset)
	if err != nil {
		return nil, err
	}
	if rulesSHA != sess.RulesSHA256 {
		return nil, fmtError("finalizeBatch", errors.New(
			"rule file changed since preview"))
	}

	datasetSalt, err := DeriveDatasetSalt(imp.cfg.SiteSalt, dataset)
	if err != nil {
		return nil, err
	}

	batchID, err := newBatchID()
	if err != nil {
		return nil, fmtError("finalizeBatch", err)
	}

	hash := sha256.New()
	cw := &countingHashWriter{h: hash}
	tee := io.TeeReader(r, cw)

	header, src, err := openChannelSource(channel, tee)
	if err != nil {
		return nil, err
	}
	if err := ValidateColumnSet(rules, header); err != nil {
		return nil, err
	}

	discardTS := time.Now().UTC().Format(time.RFC3339Nano)
	stagingPath, rowCount, err := imp.writeStaging(
		ctx, batchID, dataset, header, src, rules, datasetSalt,
		sess.BucketOverrides, sess.SchemaTypes, cw)
	if err != nil {
		return nil, err
	}

	contentSHA := hex.EncodeToString(cw.Sum(nil))
	rawSize := cw.n

	if err := ValidateCommitAgainstPreview(sess, contentSHA); err != nil {
		_ = os.Remove(stagingPath)
		return nil, err
	}

	return imp.commitOrSupersede(ctx, batchID, dataset, period, rules, rulesSHA,
		header, sess.SchemaTypes, stagingPath, contentSHA, rawSize, rowCount, discardTS)
}

// writeStaging materializes one staging .duckdb from a decoded rowSource.
// The caller's countingHashWriter is updated by the underlying TeeReader as the
// source is consumed, so the digest and byte count are authoritative once this
// function returns.
func (imp *Importer) writeStaging(
	ctx context.Context,
	batchID, dataset string,
	header []string,
	src rowSource,
	rules *Rules,
	datasetSalt []byte,
	overrides map[string]float64,
	schemaTypes map[string]string,
	cw *countingHashWriter,
) (stagingPath string, rowCount int64, err error) {
	table, err := buildStagingTable(header, schemaTypes)
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(imp.cfg.stagingDir(), 0o755); err != nil {
		return "", 0, fmtError("writeStaging", err)
	}
	stagingPath = filepath.Join(imp.cfg.stagingDir(), batchID+".duckdb")

	var pipeErr error
	desensitizedSeq := func(yield func([]any) bool) {
		defer func() {
			if r := recover(); r != nil {
				pipeErr = recoverRowIterError(r)
			}
		}()
		for {
			row, err := src.Next()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				pipeErr = err
				return
			}
			out, err := desensitizeRow(row, header, rules, datasetSalt, overrides)
			if err != nil {
				pipeErr = err
				return
			}
			if !yield(out) {
				return
			}
		}
	}

	rowCount, err = duckdb.WriteStagingBatch(ctx, stagingPath, table, desensitizedSeq)
	if err != nil {
		_ = os.Remove(stagingPath)
		return "", 0, fmtError("writeStaging", err)
	}
	if pipeErr != nil {
		_ = os.Remove(stagingPath)
		return "", 0, pipeErr
	}
	return stagingPath, rowCount, nil
}

// commitOrSupersede is the common tail of finalizeBatch and pushBatch:
// idempotency check, atomic commit/supersede, and audit row.
func (imp *Importer) commitOrSupersede(
	ctx context.Context,
	batchID, dataset, period string,
	rules *Rules,
	rulesSHA string,
	header []string,
	schemaTypes map[string]string,
	stagingPath, contentSHA string,
	rawSize, rowCount int64,
	discardTS string,
) (*CommitResult, error) {
	existingBatchID, supersededBatchID, err := checkIdempotent(
		ctx, imp.seam, dataset, period, contentSHA)
	if err != nil {
		_ = os.Remove(stagingPath)
		return nil, err
	}
	if existingBatchID != "" {
		_ = os.Remove(stagingPath)
		return &CommitResult{
			BatchID:    existingBatchID,
			Rows:       rowCount,
			Superseded: false,
		}, nil
	}

	if err := os.MkdirAll(imp.cfg.datasetsDir(), 0o755); err != nil {
		_ = os.Remove(stagingPath)
		return nil, fmtError("commitOrSupersede", err)
	}
	dsName := registryName(dataset, period)
	schemaJSON, err := schemaJSON(header, schemaTypes)
	if err != nil {
		_ = os.Remove(stagingPath)
		return nil, err
	}
	now := time.Now().UTC()
	row := store.DatasetRow{
		Name:          dsName,
		Dataset:       dataset,
		Period:        period,
		Tier:          rules.Tier,
		SchemaJSON:    schemaJSON,
		FilePath:      filepath.Join(imp.cfg.datasetsDir(), dsName+".duckdb"),
		ContentSHA256: contentSHA,
		BatchID:       batchID,
		KMinOverride:  rules.KMin,
		CreatedAt:     now,
	}

	targetExists := supersededBatchID != ""
	if targetExists {
		err = duckdb.SupersedeStagedDataset(ctx, duckdb.SupersedeOptions{
			StagingPath: stagingPath,
			DatasetID:   dsName,
			Row:         row,
			Registry:    imp.seam,
			DatasetsDir: imp.cfg.datasetsDir(),
			BackupDir:   imp.cfg.stagingDir(),
			LRU:         imp.lru,
		})
	} else {
		err = duckdb.CommitStagedDataset(ctx, duckdb.CommitOptions{
			StagingPath: stagingPath,
			DatasetID:   dsName,
			Row:         row,
			Registry:    imp.seam,
			DatasetsDir: imp.cfg.datasetsDir(),
			LRU:         imp.lru,
		})
	}
	if err != nil {
		return nil, fmtError("commitOrSupersede", err)
	}

	auditRow := newAuditRow(
		batchID, dataset, period, rules.Tier, contentSHA, rulesSHA,
		rawSize, rowCount, discardTS, supersededBatchID, now,
	)
	if err := writeAuditRow(ctx, imp.seam, auditRow); err != nil {
		return nil, fmtError("commitOrSupersede", err)
	}

	return &CommitResult{
		BatchID:           batchID,
		Rows:              rowCount,
		Superseded:        targetExists,
		SupersededBatchID: supersededBatchID,
	}, nil
}

// buildStagingTable assembles the DuckDB table spec from the header and the
// schema types frozen at preview time.
func buildStagingTable(header []string, schemaTypes map[string]string) (duckdb.StagingTable, error) {
	cols := make([]duckdb.ColumnSpec, len(header))
	for i, col := range header {
		typ, ok := schemaTypes[col]
		if !ok || typ == "" {
			return duckdb.StagingTable{}, fmtError("buildStagingTable", fmt.Errorf(
				"column %q has no frozen schema type", col))
		}
		cols[i] = duckdb.ColumnSpec{Name: col, DuckDBType: typ}
	}
	return duckdb.StagingTable{Name: "data", Columns: cols}, nil
}

func schemaJSON(header []string, schemaTypes map[string]string) (string, error) {
	type col struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	cols := make([]col, len(header))
	for i, c := range header {
		cols[i] = col{Name: c, Type: schemaTypes[c]}
	}
	b, err := json.Marshal(cols)
	if err != nil {
		return "", fmtError("schemaJSON", err)
	}
	return string(b), nil
}
