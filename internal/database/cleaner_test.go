// Copyright 2026 [Copyright Holder]
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
// Author: [YOUR_NAME]

package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPurgeExpiredRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client, err := NewClient(ctx, "sqlite3", "file:purge_test?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	res, err := PurgeExpiredRecords(ctx, client, 30)
	require.NoError(t, err)
	assert.Equal(t, 30, res.RetentionDays)
	assert.NotEmpty(t, res.Message)
	assert.False(t, res.CutoffTime.IsZero())
}
