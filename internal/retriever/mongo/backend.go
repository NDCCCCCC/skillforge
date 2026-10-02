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

package mongo

import (
	"context"

	"github.com/skillforge/skillforge/internal/retriever"
)

// MongoIndexerBackend satisfies retriever.IndexerBackend.
type MongoIndexerBackend struct {
	mc *Client
}

// Compile-time assertion — any future Retriever interface drift fails here.
var _ retriever.IndexerBackend = (*MongoIndexerBackend)(nil)

// NewIndexerBackend wraps an existing Client as an IndexerBackend.
func NewIndexerBackend(mc *Client) *MongoIndexerBackend {
	return &MongoIndexerBackend{mc: mc}
}

// Insert delegates to Client.Insert.
func (b *MongoIndexerBackend) Insert(ctx context.Context, docs []retriever.Document) error {
	return b.mc.Insert(ctx, docs)
}
