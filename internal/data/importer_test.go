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

package data

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreviewFirst100Pairs(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt: "test-site-salt-16bytes",
		RulesDir: "../../datarules",
	}, nil, nil)

	var rows []string
	rows = append(rows, "customer_name,mobile,cert_no,balance,overdue_days,update_ts,status")
	for i := 0; i < 150; i++ {
		rows = append(rows, fmt.Sprintf("张三%d,138-0000-%04d,11010119900101%04d,%d,%d,2026-07-01,active", i, i, i, 1000+i*10, i%90))
	}
	req := PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(strings.Join(rows, "\n")),
	}

	resp, err := imp.Preview(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "loans", resp.Dataset)
	assert.Equal(t, "2026-07", resp.Period)
	assert.Equal(t, int64(150), resp.RowCount)
	assert.Len(t, resp.SamplePairs, 100)
	assert.Equal(t, "历史", resp.ProposedTier)
	assert.NotEmpty(t, resp.PreviewID)
	assert.NotEmpty(t, resp.ContentSHA256)
	assert.NotEmpty(t, resp.RulesSHA256)

	// Hash columns become 64-char hex; bucket columns become numeric bucket floors.
	first := resp.SamplePairs[0]
	require.Len(t, first.Raw, 7)
	require.Len(t, first.Desensitized, 7)
	assert.Equal(t, "张三0", first.Raw[0])
	assert.Len(t, first.Desensitized[0], 64) // HMAC hex
	assert.Equal(t, "1000", first.Raw[3])
	assert.Equal(t, "1000", first.Desensitized[3]) // width=1000 bucket floor
}

func TestPreviewBucketOverride(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt: "test-site-salt-16bytes",
		RulesDir: "../../datarules",
	}, nil, nil)

	rows := []string{
		"customer_name,mobile,cert_no,balance,overdue_days,update_ts,status",
		"张三,138-0000-0000,11010119900101X,1250,5,2026-07-01,active",
	}
	req := PreviewRequest{
		Dataset:         "loans",
		Period:          "2026-07",
		Channel:         "csv",
		Reader:          strings.NewReader(strings.Join(rows, "\n")),
		BucketOverrides: map[string]float64{"balance": 500},
	}

	resp, err := imp.Preview(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "1000", resp.SamplePairs[0].Desensitized[3]) // floor(1250/500)*500 = 1000
}

func TestPreviewPeriodFormat(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt: "test-site-salt-16bytes",
		RulesDir: "../../datarules",
	}, nil, nil)

	rows := []string{
		"customer_name,mobile,cert_no,balance,overdue_days,update_ts,status",
		"张三,138-0000-0000,11010119900101X,1250,5,2026-07-01,active",
	}
	for _, bad := range []string{"2026-13", "202607", ""} {
		req := PreviewRequest{
			Dataset: "loans",
			Period:  bad,
			Channel: "csv",
			Reader:  strings.NewReader(strings.Join(rows, "\n")),
		}
		_, err := imp.Preview(context.Background(), req)
		require.Error(t, err, "period %q should be rejected", bad)
		assert.Contains(t, err.Error(), "YYYY-MM")
	}
}

func TestPreviewUnknownChannel(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt: "test-site-salt-16bytes",
		RulesDir: "../../datarules",
	}, nil, nil)

	req := PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "xml",
		Reader:  strings.NewReader(""),
	}
	_, err := imp.Preview(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown import channel")
}

func TestPreviewEmptySaltFailClosed(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt: "",
		RulesDir: "../../datarules",
	}, nil, nil)

	rows := []string{
		"customer_name,mobile,cert_no,balance,overdue_days,update_ts,status",
		"张三,138-0000-0000,11010119900101X,1250,5,2026-07-01,active",
	}
	req := PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(strings.Join(rows, "\n")),
	}
	_, err := imp.Preview(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SKILLFORGE_DATA_SALT")
}

func TestPreviewUnknownColumnFailClosed(t *testing.T) {
	imp := NewImporter(ImporterConfig{
		SiteSalt: "test-site-salt-16bytes",
		RulesDir: "../../datarules",
	}, nil, nil)

	rows := []string{
		"customer_name,mobile,cert_no,balance,overdue_days,update_ts,status,extra_col",
		"张三,138-0000-0000,11010119900101X,1250,5,2026-07-01,active,x",
	}
	req := PreviewRequest{
		Dataset: "loans",
		Period:  "2026-07",
		Channel: "csv",
		Reader:  strings.NewReader(strings.Join(rows, "\n")),
	}
	_, err := imp.Preview(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extra_col")
}
