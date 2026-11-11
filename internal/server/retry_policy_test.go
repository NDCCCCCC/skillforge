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

package server

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The retry decision is load-bearing: live evidence (2026-11-11) showed a
// 401 being retried 4× with 1s/2s/4s backoff, costing ~7s and three extra
// doomed requests before the user saw the real cause.
func TestIsRetryableStreamErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"401 bad key", errors.New(`open stream: error, status code: 401, status: 401 Unauthorized, message: invalid api key`), false},
		{"403 forbidden", errors.New(`status code: 403, message: key revoked`), false},
		{"404 unknown model", errors.New(`status code: 404, message: model not found`), false},
		{"400 malformed", errors.New(`status code: 400, message: bad request`), false},
		{"429 throttled", errors.New(`status: 429 Too Many Requests`), true},
		{"500 upstream", errors.New(`status code: 500, message: internal error`), true},
		{"502 gateway", errors.New(`status code: 502 message: bad gateway`), true},
		{"503 unavailable", errors.New(`status: 503 Service Unavailable`), true},
		{"connection reset (no status)", errors.New(`read tcp 10.0.0.1:1234: connection reset by peer`), true},
		{"context canceled", fmt.Errorf("open LLM stream: %w", context.Canceled), false},
		{"context deadline", fmt.Errorf("open LLM stream: %w", context.DeadlineExceeded), false},
		{"wrapped 401", fmt.Errorf("verdict pipeline: operational LLM: %w", errors.New(`status code: 401`)), false},
		{"wrapped 429", fmt.Errorf("verdict pipeline: %w", errors.New(`status code: 429`)), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isRetryableStreamErr(tc.err))
		})
	}
}

func TestHTTPStatusOf(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"status code form", errors.New(`status code: 401, status: 401 Unauthorized`), 401},
		{"status form", errors.New(`status: 429 Too Many Requests`), 429},
		{"no status", errors.New(`connection reset by peer`), 0},
		{"out of range is ignored", errors.New(`status code: 999`), 0},
		{"first match wins", errors.New(`status code: 502 then status code: 401`), 502},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, httpStatusOf(tc.err))
		})
	}
}
