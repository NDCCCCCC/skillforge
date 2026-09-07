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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockAggCursor is a minimal cursor for testing GetMaxID.
// The results type must have the same bson tag as what the production
// cursor.All receives — the bson tag makes it a distinct Go type.
type mockAggCursor struct {
	results []struct{ ID int64 `bson:"id"` }
	err     error
	calls   int
}

func (m *mockAggCursor) All(_ context.Context, out interface{}) error {
	m.calls++
	if m.err != nil {
		return m.err
	}
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Slice {
		return assert.AnError
	}
	// Empty result: set an empty slice of the correct type.
	emptySlice := reflect.MakeSlice(rv.Elem().Type(), 0, 0)
	rv.Elem().Set(emptySlice)
	if len(m.results) == 0 {
		return nil
	}
	// Non-empty: build a typed slice with the mock results.
	newSlice := reflect.MakeSlice(rv.Elem().Type(), len(m.results), len(m.results))
	for i := 0; i < len(m.results); i++ {
		newSlice.Index(i).Set(reflect.ValueOf(m.results[i]))
	}
	rv.Elem().Set(newSlice)
	return nil
}

func (m *mockAggCursor) Close(context.Context) error { return nil }

// fakeAggregator is a test double for the aggregator interface.
type fakeAggregator struct {
	cursor *mockAggCursor
}

func (f *fakeAggregator) Aggregate(_ context.Context, _ any) (aggregatorCursor, error) {
	return f.cursor, nil
}

// mockFinderForGetMaxID implements rowFinder with a nil coll.
// GetMaxID does not use finder, but the field is required by Client struct.
type mockFinderForGetMaxID struct{}

func (m *mockFinderForGetMaxID) FindRows(ctx context.Context, filter any) ([]docRow, error) {
	return nil, nil
}

// TestMongoClientGetMaxIDEmpty verifies that an empty collection returns 0.
func TestMongoClientGetMaxIDEmpty(t *testing.T) {
	cur := &mockAggCursor{results: nil}
	mc := &Client{
		cfg:    Config{Database: "test", Collection: "knowledge"},
		finder: &mockFinderForGetMaxID{},
		agg:    &fakeAggregator{cursor: cur},
	}

	id, err := mc.GetMaxID(context.Background(), "authority")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), id, "empty result should return 0")
}

// TestMongoClientGetMaxIDPopulated verifies that the maximum ID is returned
// when chunks exist for the tier. The mock returns rows pre-sorted (reflecting
// the behavior of the $sort + $limit pipeline in production MongoDB).
func TestMongoClientGetMaxIDPopulated(t *testing.T) {
	cur := &mockAggCursor{
		results: []struct{ ID int64 `bson:"id"` }{{ID: 5}, {ID: 4}, {ID: 3}, {ID: 2}, {ID: 1}},
	}
	mc := &Client{
		cfg:    Config{Database: "test", Collection: "knowledge"},
		finder: &mockFinderForGetMaxID{},
		agg:    &fakeAggregator{cursor: cur},
	}

	id, err := mc.GetMaxID(context.Background(), "authority")
	assert.NoError(t, err)
	assert.Equal(t, int64(5), id, "max ID should be 5 for authority tier")
}

// TestMongoClientGetMaxIDUnknownTier verifies that querying a non-existent
// tier returns 0 without an error.
func TestMongoClientGetMaxIDUnknownTier(t *testing.T) {
	cur := &mockAggCursor{results: nil}
	mc := &Client{
		cfg:    Config{Database: "test", Collection: "knowledge"},
		finder: &mockFinderForGetMaxID{},
		agg:    &fakeAggregator{cursor: cur},
	}

	id, err := mc.GetMaxID(context.Background(), "nonexistent-tier")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), id, "unknown tier should return 0")
}
