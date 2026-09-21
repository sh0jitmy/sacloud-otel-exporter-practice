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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/shjtmy/go_sh0jitmy_template/ent/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupAndRestore_FullLifecycle(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "template_backup_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	client, err := NewClient(ctx, "sqlite3", "file:backup_lifecycle_test?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// 1. Seed initial data
	err = SeedAdminUser(ctx, client)
	require.NoError(t, err)

	_, err = client.User.Create().
		SetUsername("alice").
		SetPasswordHash("alice-secret-hash").
		Save(ctx)
	require.NoError(t, err)

	// Verify users count is 2 (admin + alice)
	count, err := client.User.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	// 2. Create backup archive
	res, err := CreateBackupArchive(ctx, client, tmpDir)
	require.NoError(t, err)
	require.NotEmpty(t, res.Filename)
	require.FileExists(t, filepath.Join(tmpDir, res.Filename))
	assert.Equal(t, 2, res.Manifest.TableCounts["users"])
	assert.NotEmpty(t, res.Manifest.Checksum)

	// Verify ListBackupArchives
	list, err := ListBackupArchives(tmpDir)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, res.Filename, list[0].Filename)

	// 3. Mutate DB (delete alice, add bob)
	_, err = client.User.Delete().Where(user.Username("alice")).Exec(ctx)
	require.NoError(t, err)

	_, err = client.User.Create().
		SetUsername("bob").
		SetPasswordHash("bob-secret-hash").
		Save(ctx)
	require.NoError(t, err)

	// Confirm mutation
	bobExists, err := client.User.Query().Where(user.Username("bob")).Exist(ctx)
	require.NoError(t, err)
	assert.True(t, bobExists)

	aliceExists, err := client.User.Query().Where(user.Username("alice")).Exist(ctx)
	require.NoError(t, err)
	assert.False(t, aliceExists)

	// 4. Restore DB from backup archive
	archivePath := filepath.Join(tmpDir, res.Filename)
	restoreRes, err := RestoreBackupArchive(ctx, client, archivePath)
	require.NoError(t, err)
	assert.True(t, restoreRes.Success)
	assert.Equal(t, 2, restoreRes.RestoredRows["users"])

	// 5. Verify restored state (alice exists, bob does not exist)
	aliceRestored, err := client.User.Query().Where(user.Username("alice")).Exist(ctx)
	require.NoError(t, err)
	assert.True(t, aliceRestored, "alice should be restored")

	bobRestored, err := client.User.Query().Where(user.Username("bob")).Exist(ctx)
	require.NoError(t, err)
	assert.False(t, bobRestored, "bob should have been removed by clean restore")

	adminRestored, err := client.User.Query().Where(user.Username("admin")).Exist(ctx)
	require.NoError(t, err)
	assert.True(t, adminRestored, "admin should remain intact")
}

func TestRestoreBackupArchive_CorruptedChecksum(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "corrupt_backup_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	client, err := NewClient(ctx, "sqlite3", "file:corrupt_test?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Create corrupted archive with bad checksum in manifest
	manifest := BackupManifest{
		Version:     "1.0.0",
		Checksum:    "invalid-checksum-value",
		Driver:      "sqlite3",
		TableCounts: map[string]int{"users": 1},
	}
	manifestJSON, _ := json.Marshal(manifest)
	usersJSON, _ := json.Marshal([]BackupUser{{ID: 1, Username: "hacker"}})

	var tarBuf bytes.Buffer
	gw := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gw)

	for _, f := range []struct {
		name string
		data []byte
	}{
		{"manifest.json", manifestJSON},
		{"users.json", usersJSON},
	} {
		_ = tw.WriteHeader(&tar.Header{Name: f.name, Size: int64(len(f.data))})
		_, _ = tw.Write(f.data)
	}
	_ = tw.Close()
	_ = gw.Close()

	corruptPath := filepath.Join(tmpDir, "corrupt.tar.gz")
	require.NoError(t, os.WriteFile(corruptPath, tarBuf.Bytes(), 0600))

	// Attempt restore
	_, err = RestoreBackupArchive(ctx, client, corruptPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}
