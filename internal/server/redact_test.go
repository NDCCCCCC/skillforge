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
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// redactSecrets 是错误透传链路的安全边界：pipelineErrorTextFor 会把上游
// message 原样给客户端看（用户要求「有什么问题就报什么问题」），所以这里
// 必须保证凭证与地址不会漏出去。
func TestRedactSecrets(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string // 必须不出现的片段
		must []string // 必须出现的内容
	}{
		{
			name: "url is replaced wholesale",
			in:   "dial tcp https://relay.example.com/v1/chat failed",
			want: []string{"relay.example.com"},
			must: []string{"<upstream>"},
		},
		{
			name: "url carrying a key is replaced before key rules split it",
			in:   "POST https://api.example.com/v1?api_key=abcd1234efgh failed",
			want: []string{"abcd1234efgh", "example.com"},
			must: []string{"<upstream>"},
		},
		{
			name: "prefixed key",
			in:   "invalid key sk-abcdef123456 for model",
			want: []string{"sk-abcdef123456"},
			must: []string{"<redacted>"},
		},
		{
			name: "ark prefixed key",
			in:   "invalid key as_sk_9f8e7d6c5b4a for model",
			want: []string{"as_sk_9f8e7d6c5b4a"},
			must: []string{"<redacted>"},
		},
		{
			name: "bearer token",
			in:   "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig rejected",
			want: []string{"eyJhbGciOiJIUzI1NiJ9"},
			must: []string{"Bearer <redacted>"},
		},
		{
			name: "api_key assignment keeps the key name",
			in:   "upstream said api_key=deadbeefcafe1234 is expired",
			want: []string{"deadbeefcafe1234"},
			must: []string{"api_key= <redacted>"},
		},
		{
			name: "bare key= assignment (the case the old rule missed)",
			in:   "request rejected: key=0011223344556677 not allowed",
			want: []string{"0011223344556677"},
			must: []string{"<redacted>"},
		},
		{
			name: "token and secret and password",
			in:   "auth failed token=aaaaaaaabbbb secret: ccccccccdddd password=eeeeeeeeffff",
			want: []string{"aaaaaaaabbbb", "ccccccccdddd", "eeeeeeeeffff"},
			must: []string{"<redacted>"},
		},
		{
			name: "ordinary text is untouched",
			in:   "upstream returned 500 after 3 retries",
			want: []string{"<redacted>", "<upstream>"},
			must: []string{"upstream returned 500 after 3 retries"},
		},
		{
			name: "short values are not treated as credentials",
			in:   "key=abc too short to matter",
			want: []string{"<redacted>"},
			must: []string{"key=abc"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactSecrets(tc.in)
			for _, w := range tc.want {
				assert.NotContains(t, got, w, "secret fragment leaked: %q -> %q", tc.in, got)
			}
			for _, m := range tc.must {
				assert.Contains(t, got, m, "expected %q in %q", m, got)
			}
		})
	}
}

// pipelineErrorTextFor 的契约：能提取出状态码就如实报，提不出才回落
// 无细节的兜底文案；两条路径都不能带出凭证。
func TestPipelineErrorTextFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
		not  []string
	}{
		{
			name: "nil error falls back",
			err:  nil,
			want: pipelineErrorText,
		},
		{
			name: "429 with message is surfaced verbatim",
			err:  errors.New(`upstream error: status code: 429 status: 429 message: rate limit exceeded`),
			want: "结论生成失败：上游返回 HTTP 429 —— rate limit exceeded",
			not:  []string{"status code", "upstream error"},
		},
		{
			name: "status without message keeps the code only",
			err:  errors.New(`post "https://relay.example.com/v1": status code: 401`),
			want: "结论生成失败：上游返回 HTTP 401（详见服务器日志）。",
			not:  []string{"relay.example.com"},
		},
		{
			name: "no status code falls back to the generic text",
			err:  errors.New("connection reset by peer"),
			want: pipelineErrorText,
		},
		{
			name: "message containing a key is redacted before being shown",
			err:  errors.New(`status code: 403 message: key sk-live_abcdef123456 is revoked`),
			want: "结论生成失败：上游返回 HTTP 403 —— key <redacted> is revoked",
			not:  []string{"sk-live_abcdef123456"},
		},
		{
			name: "message containing a url is redacted before being shown",
			err:  errors.New(`status code: 502 message: bad gateway from https://api.internal/v1`),
			want: "结论生成失败：上游返回 HTTP 502 —— bad gateway from <upstream>",
			not:  []string{"api.internal"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipelineErrorTextFor(tc.err)
			assert.Equal(t, tc.want, got)
			for _, n := range tc.not {
				assert.NotContains(t, got, n, "leaked: %q", got)
			}
			assert.False(t, strings.Contains(got, "<nil>"), "no Go nil formatting should leak")
		})
	}
}
