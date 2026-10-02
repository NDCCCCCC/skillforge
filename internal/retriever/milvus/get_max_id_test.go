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

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/stretchr/testify/assert"
)

// getMaxIDMockClient records Query calls and returns configurable results.
// It embeds client.Client so only the methods we override need implementation.
type getMaxIDMockClient struct {
	client.Client
	calls      []queryCall
	resultRows []client.ResultSet // each call returns results[callIndex], then advances
	callIndex  int
}

type queryCall struct {
	ctx    context.Context
	coll   string
	expr   string
	limit  int64
}

func (m *getMaxIDMockClient) Query(
	ctx context.Context,
	coll string,
	partitions []string,
	expr string,
	outputFields []string,
	opts ...client.SearchQueryOptionFunc,
) (client.ResultSet, error) {
	m.calls = append(m.calls, queryCall{
		ctx:   ctx,
		coll:  coll,
		expr:  expr,
		limit: 10000, // default; tests can inspect calls to verify limit used
	})
	if m.callIndex < len(m.resultRows) {
		out := m.resultRows[m.callIndex]
		m.callIndex++
		return out, nil
	}
	return nil, nil
}

// idCol is a test helper that constructs a *ColumnInt64 for the "id" field.
func idCol(values []int64) *entity.ColumnInt64 {
	col := entity.NewColumnInt64("id", values)
	return col
}

// TestMilvusClientGetMaxIDEmpty verifies that an empty collection returns 0.
func TestMilvusClientGetMaxIDEmpty(t *testing.T) {
	tc := &getMaxIDMockClient{
		resultRows: []client.ResultSet{
			client.ResultSet{}, // empty result for "authority" tier
		},
	}
	mc := Wrap(tc, Config{CollectionName: "test_knowledge"})

	id, err := mc.GetMaxID(context.Background(), "authority")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), id, "empty result should return 0")
	assert.Len(t, tc.calls, 1)
	assert.Equal(t, `tier == "authority"`, tc.calls[0].expr)
	assert.Equal(t, "test_knowledge", tc.calls[0].coll)
}

// TestMilvusClientGetMaxIDPopulated verifies that the maximum ID is returned
// when chunks exist for the tier.
func TestMilvusClientGetMaxIDPopulated(t *testing.T) {
	tc := &getMaxIDMockClient{
		resultRows: []client.ResultSet{
			client.ResultSet{idCol([]int64{1, 2, 3, 4, 5})}, // authority: max is 5
		},
	}
	mc := Wrap(tc, Config{CollectionName: "test_knowledge"})

	id, err := mc.GetMaxID(context.Background(), "authority")
	assert.NoError(t, err)
	assert.Equal(t, int64(5), id, "max ID should be 5 for authority tier")
}

// TestMilvusClientGetMaxIDUnknownTier verifies that querying a non-existent
// tier returns 0 without an error.
func TestMilvusClientGetMaxIDUnknownTier(t *testing.T) {
	tc := &getMaxIDMockClient{
		resultRows: []client.ResultSet{
			client.ResultSet{}, // no rows for the unknown tier
		},
	}
	mc := Wrap(tc, Config{CollectionName: "test_knowledge"})

	id, err := mc.GetMaxID(context.Background(), "nonexistent-tier")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), id, "unknown tier should return 0")
	assert.Equal(t, `tier == "nonexistent-tier"`, tc.calls[0].expr)
}
