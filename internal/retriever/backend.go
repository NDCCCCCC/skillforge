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

package retriever

import (
	"context"
)

// IndexerBackend is the narrow insert-only interface for the document upload pipeline.
// It is deliberately minimal: only Insert is needed to append new chunks.
// The query/search path goes through retriever.Retriever (a separate interface).
//
// Both Milvus and MongoDB retriever implementations satisfy this interface.
type IndexerBackend interface {
	// Insert appends docs to the backend store. The docs are already
	// validated (tier check, effective window check, embedding computed).
	// Errors are returned verbatim — the caller decides retry/log/abort.
	Insert(ctx context.Context, docs []Document) error
}

// IndexerBackendNotWiredError is the 503 body when Deps.IndexerBackend is nil.
const IndexerBackendNotWiredError = "indexer backend not wired (server started without a retriever backend)"
