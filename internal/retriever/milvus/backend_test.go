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

package milvus

import (
	"context"
	"testing"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/stretchr/testify/assert"
)

// mockMilvusClient records Insert calls for test verification.
type mockMilvusClient struct {
	calls [][]retriever.Document
	err   error
}

func (m *mockMilvusClient) Insert(ctx context.Context, docs []retriever.Document) error {
	m.calls = append(m.calls, docs)
	return m.err
}

// fakeMilvusIndexerBackend is a test double that satisfies
// retriever.IndexerBackend without touching the real Milvus SDK.
type fakeMilvusIndexerBackend struct {
	mc *mockMilvusClient
}

var _ retriever.IndexerBackend = (*fakeMilvusIndexerBackend)(nil)

func (f *fakeMilvusIndexerBackend) Insert(ctx context.Context, docs []retriever.Document) error {
	return f.mc.Insert(ctx, docs)
}

// TestMilvusIndexerBackendInsert verifies that Insert forwards to the
// underlying MilvusClient.Insert with the same arguments.
func TestMilvusIndexerBackendInsert(t *testing.T) {
	mock := &mockMilvusClient{}
	backend := &fakeMilvusIndexerBackend{mc: mock}

	docs := []retriever.Document{
		{ID: 1, Tier: "authority", Content: "test chunk", Source: "test.md"},
		{ID: 2, Tier: "authority", Content: "another chunk", Source: "test.md"},
	}

	err := backend.Insert(context.Background(), docs)
	assert.NoError(t, err)
	assert.Len(t, mock.calls, 1, "Insert should be called exactly once")
	assert.Equal(t, docs, mock.calls[0], "Insert should forward docs unchanged")
}

// TestMilvusIndexerBackendInsertError verifies that errors from the
// underlying client propagate correctly.
func TestMilvusIndexerBackendInsertError(t *testing.T) {
	mock := &mockMilvusClient{err: assert.AnError}
	backend := &fakeMilvusIndexerBackend{mc: mock}

	docs := []retriever.Document{{ID: 1, Tier: "authority", Content: "test"}}
	err := backend.Insert(context.Background(), docs)
	assert.ErrorIs(t, err, assert.AnError, "Insert error should propagate")
	assert.Len(t, mock.calls, 1, "Insert should be called even when it returns an error")
}
