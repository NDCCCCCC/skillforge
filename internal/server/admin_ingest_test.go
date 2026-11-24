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

// Phase 55-03 (ING-05) -- the ingest read surface the admin Jobs view
// consumes. Two endpoints, one projection:
//
//   GET /api/admin/ingest/jobs      -> {jobs: [ingestJobDTO...]} newest first
//   GET /api/admin/ingest/jobs/:id  -> ingestJobDTO
//
// The list endpoint was a declared-but-unbuilt half of 54-01 Task 3. These
// tests pin the gap closed AND pin the shared-projection property: both
// endpoints must return the SAME field mapping for the same job (a second
// hand-written projection would let the two surfaces drift).
package server

import (
"context"
"encoding/json"
"net/http"
"testing"

"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"

ut "github.com/cloudwego/hertz/pkg/common/ut"

"github.com/skillforge/skillforge/internal/ingest"
)

// ingestReadRequest issues one admin GET against a fresh Router so the
// route table under test is the REAL one (New() registers the handlers).
func ingestReadRequest(t *testing.T, deps *Deps, path, token string) *ut.ResponseRecorder {
t.Helper()
h := New(":0", deps)
headers := []ut.Header{}
if token != "" {
headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: token})
}
return ut.PerformRequest(h.Engine, http.MethodGet, path, nil, headers...)
}

// TestIngestJobsListEndpoint pins the roster read: 200 with a jobs[] array
// carrying the full envelope for every job the store holds, newest first.
func TestIngestJobsListEndpoint(t *testing.T) {
deps := uploadTestDeps(t, &mockIndexerBackend{})

// One real job through the upload path so the roster has a row whose
// steps the runner seeded.
body, ct, n := buildUploadMultipart(t, "Roster List.md", "text/markdown", []byte("# Roster List\n\nBody."))
up := invokeUpload(t, deps, "authority", testAdminToken, body, ct, n)
require.Equal(t, http.StatusOK, up.Code)
deps.Ingest.WaitIdle()

jobs, err := deps.Ingest.ListJobs(context.Background())
require.NoError(t, err)
require.NotEmpty(t, jobs, "the upload must leave a job row")

resp := ingestReadRequest(t, deps, "/api/admin/ingest/jobs", testAdminToken)
require.Equal(t, http.StatusOK, resp.Code)

var envelope struct {
Jobs []map[string]any `json:"jobs"`
}
require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &envelope))
// The shape must be a real array (never null -- a null roster renders
// as "no jobs" in the FE and would lie about the store).
assert.NotNil(t, envelope.Jobs)
require.NotEmpty(t, envelope.Jobs, "the roster must carry the uploaded job")
first := envelope.Jobs[0]
for _, k := range []string{"job_id", "slug", "filename", "sha256", "doc_id",
"status", "error", "created_at", "updated_at", "steps", "failed_items"} {
_, ok := first[k]
assert.True(t, ok, "list envelope missing %q", k)
}
// 8 step rows are seeded at enqueue (the SC 5 tuple) and every one
// must serialize with the full field set.
steps, ok := first["steps"].([]any)
require.True(t, ok, "steps must be a JSON array")
assert.Len(t, steps, len(ingest.StepSequence))
for _, raw := range steps {
row, ok := raw.(map[string]any)
require.True(t, ok)
for _, k := range []string{"step", "status", "attempt", "started_at",
"completed_at", "duration_ms", "error", "artifact_ref"} {
_, ok := row[k]
assert.True(t, ok, "step row missing %q", k)
}
}
}

// TestIngestJobsEndpointsShareProjection pins the single-projection
// invariant: GET /jobs and GET /jobs/:id must serialize the same job
// identically. A second hand-written DTO mapping is exactly the drift this
// test forbids (55-03 renders both surfaces).
func TestIngestJobsEndpointsShareProjection(t *testing.T) {
deps := uploadTestDeps(t, &mockIndexerBackend{})
body, ct, n := buildUploadMultipart(t, "Roster Doc.md", "text/markdown", []byte("# Roster\n\nBody."))
resp := invokeUpload(t, deps, "authority", testAdminToken, body, ct, n)
require.Equal(t, http.StatusOK, resp.Code)

var created struct {
JobID string `json:"job_id"`
}
require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &created))
require.NotEmpty(t, created.JobID)
deps.Ingest.WaitIdle()

one := ingestReadRequest(t, deps, "/api/admin/ingest/jobs/"+created.JobID, testAdminToken)
require.Equal(t, http.StatusOK, one.Code)
list := ingestReadRequest(t, deps, "/api/admin/ingest/jobs", testAdminToken)
require.Equal(t, http.StatusOK, list.Code)

var detail ingestJobDTO
require.NoError(t, json.Unmarshal(one.Body.Bytes(), &detail))

var envelope struct {
Jobs []ingestJobDTO `json:"jobs"`
}
require.NoError(t, json.Unmarshal(list.Body.Bytes(), &envelope))
require.NotEmpty(t, envelope.Jobs)

var fromList *ingestJobDTO
for i := range envelope.Jobs {
if envelope.Jobs[i].JobID == created.JobID {
fromList = &envelope.Jobs[i]
break
}
}
require.NotNil(t, fromList, "the created job must appear in the roster")
// Field-by-field equality: the two endpoints share buildIngestJobDTO.
assert.Equal(t, detail.JobID, fromList.JobID)
assert.Equal(t, detail.Slug, fromList.Slug)
assert.Equal(t, detail.Filename, fromList.Filename)
assert.Equal(t, detail.SHA256, fromList.SHA256)
assert.Equal(t, detail.DocID, fromList.DocID)
assert.Equal(t, detail.Status, fromList.Status)
assert.Equal(t, len(detail.Steps), len(fromList.Steps))
for i := range detail.Steps {
assert.Equal(t, detail.Steps[i], fromList.Steps[i],
"step %d must serialize identically on both endpoints", i)
}
// 8 steps are seeded by the runner (the SC 5 tuple is complete).
assert.Equal(t, len(ingest.StepSequence), len(detail.Steps))
}

// TestIngestRoutesTokenAndNilRunnerOrder pins the locked 403-before-503
// order and the unknown-job 404 on both reads.
func TestIngestRoutesTokenAndNilRunnerOrder(t *testing.T) {
deps := uploadTestDeps(t, &mockIndexerBackend{})

assert.Equal(t, http.StatusForbidden,
ingestReadRequest(t, deps, "/api/admin/ingest/jobs", "").Code)
assert.Equal(t, http.StatusForbidden,
ingestReadRequest(t, deps, "/api/admin/ingest/jobs/ing-nope", "").Code)

assert.Equal(t, http.StatusNotFound,
ingestReadRequest(t, deps, "/api/admin/ingest/jobs/ing-nope", testAdminToken).Code)

// nil runner -> 503 (the pipeline-not-wired posture), still AFTER the
// token check.
bare := &Deps{}
assert.Equal(t, http.StatusForbidden,
ingestReadRequest(t, bare, "/api/admin/ingest/jobs", "").Code)
assert.Equal(t, http.StatusServiceUnavailable,
ingestReadRequest(t, bare, "/api/admin/ingest/jobs", testAdminToken).Code)
}
