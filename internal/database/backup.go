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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shjtmy/go_sh0jitmy_template/ent"
	"github.com/shjtmy/go_sh0jitmy_template/ent/user"
)

// BackupManifest contains metadata about the database backup archive.
type BackupManifest struct {
	Version     string         `json:"version"`
	Timestamp   time.Time      `json:"timestamp"`
	Driver      string         `json:"driver,omitempty"`
	TableCounts map[string]int `json:"table_counts"`
	Checksum    string         `json:"checksum"`
}

// BackupUser represents the serializable User entity including credentials.
type BackupUser struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
}

// BackupResult describes a generated backup artifact.
type BackupResult struct {
	Filename    string         `json:"filename"`
	DownloadURL string         `json:"download_url"`
	Manifest    BackupManifest `json:"manifest"`
	SizeBytes   int64          `json:"size_bytes"`
}

// RestoreResult provides the summary of a completed restore operation.
type RestoreResult struct {
	Success      bool           `json:"success"`
	RestoredAt   time.Time      `json:"restored_at"`
	Manifest     BackupManifest `json:"manifest"`
	RestoredRows map[string]int `json:"restored_rows"`
}

// CreateBackupArchive exports database entities into a compressed tar.gz archive with SHA256 verification.
func CreateBackupArchive(ctx context.Context, client *ent.Client, backupDir string) (*BackupResult, error) {
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 1. Fetch data
	users, err := client.User.Query().Order(ent.Asc(user.FieldID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query users for backup: %w", err)
	}

	var backupUsers []BackupUser
	for _, u := range users {
		backupUsers = append(backupUsers, BackupUser{
			ID:           u.ID,
			Username:     u.Username,
			PasswordHash: u.PasswordHash,
		})
	}

	usersJSON, err := json.MarshalIndent(backupUsers, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal users: %w", err)
	}

	// 2. Compute payload SHA256 checksum
	hasher := sha256.New()
	hasher.Write(usersJSON)
	checksum := hex.EncodeToString(hasher.Sum(nil))

	timestamp := time.Now().UTC()
	manifest := BackupManifest{
		Version:   "1.0.0",
		Timestamp: timestamp,
		Driver:    "sqlite3",
		TableCounts: map[string]int{
			"users": len(backupUsers),
		},
		Checksum: checksum,
	}

	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}

	// 3. Build tar.gz in-memory buffer
	var tarBuf bytes.Buffer
	gw := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gw)

	files := []struct {
		Name string
		Data []byte
	}{
		{"manifest.json", manifestJSON},
		{"users.json", usersJSON},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Mode:     0600,
			Size:     int64(len(file.Data)),
			ModTime:  timestamp,
			Typeflag: tar.TypeReg,
		}
		if writeHdrErr := tw.WriteHeader(hdr); writeHdrErr != nil {
			return nil, fmt.Errorf("write tar header for %s: %w", file.Name, writeHdrErr)
		}
		if _, writeDataErr := tw.Write(file.Data); writeDataErr != nil {
			return nil, fmt.Errorf("write tar content for %s: %w", file.Name, writeDataErr)
		}
	}

	if closeErr := tw.Close(); closeErr != nil {
		return nil, fmt.Errorf("close tar writer: %w", closeErr)
	}
	if closeGzErr := gw.Close(); closeGzErr != nil {
		return nil, fmt.Errorf("close gzip writer: %w", closeGzErr)
	}

	// 4. Save to destination file
	filename := fmt.Sprintf("backup_%s.tar.gz", timestamp.Format("20060102_150405"))
	destPath := filepath.Join(backupDir, filename)

	if writeErr := os.WriteFile(destPath, tarBuf.Bytes(), 0600); writeErr != nil {
		return nil, fmt.Errorf("write backup file: %w", writeErr)
	}

	fi, statErr := os.Stat(destPath)
	if statErr != nil {
		return nil, fmt.Errorf("stat backup file: %w", statErr)
	}

	slog.Info("Successfully created database backup archive",
		"filename", filename,
		"size_bytes", fi.Size(),
		"users_count", len(backupUsers),
		"checksum", checksum,
	)

	return &BackupResult{
		Filename:    filename,
		DownloadURL: "/v1/system/backups/" + filename,
		Manifest:    manifest,
		SizeBytes:   fi.Size(),
	}, nil
}

// RestoreBackupArchive restores entities from a backup tar.gz archive in a transactional manner.
func RestoreBackupArchive(ctx context.Context, client *ent.Client, archivePath string) (*RestoreResult, error) {
	cleanPath := filepath.Clean(archivePath)
	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("open backup file: %w", err)
	}
	defer func() { _ = f.Close() }()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("create gzip reader: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)

	archiveData := make(map[string][]byte)
	for {
		hdr, nextErr := tr.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, fmt.Errorf("read tar entry: %w", nextErr)
		}

		var buf bytes.Buffer
		// Limit to 50MB per file to mitigate decompression bomb (G110)
		if _, copyErr := io.Copy(&buf, io.LimitReader(tr, 50*1024*1024)); copyErr != nil {
			return nil, fmt.Errorf("read content of %s: %w", hdr.Name, copyErr)
		}
		archiveData[filepath.Base(hdr.Name)] = buf.Bytes()
	}

	// 1. Verify manifest
	manifestBytes, ok := archiveData["manifest.json"]
	if !ok {
		return nil, fmt.Errorf("missing manifest.json in archive")
	}

	var manifest BackupManifest
	if unmarshalErr := json.Unmarshal(manifestBytes, &manifest); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", unmarshalErr)
	}

	// 2. Verify payload checksum
	usersBytes, ok := archiveData["users.json"]
	if !ok {
		return nil, fmt.Errorf("missing users.json in archive")
	}

	hasher := sha256.New()
	hasher.Write(usersBytes)
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	if actualChecksum != manifest.Checksum {
		return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", manifest.Checksum, actualChecksum)
	}

	var backupUsers []BackupUser
	if unmarshalUsersErr := json.Unmarshal(usersBytes, &backupUsers); unmarshalUsersErr != nil {
		return nil, fmt.Errorf("unmarshal users: %w", unmarshalUsersErr)
	}

	// 3. Execute transactional atomic restore
	tx, err := client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	// Rollback on any failure
	defer func() {
		_ = tx.Rollback()
	}()

	// Clear current users table
	if _, err := tx.User.Delete().Exec(ctx); err != nil {
		return nil, fmt.Errorf("clear users table: %w", err)
	}

	// Restore users with preserved IDs
	restoredCount := 0
	for _, u := range backupUsers {
		err := tx.User.Create().
			SetUsername(u.Username).
			SetPasswordHash(u.PasswordHash).
			Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("restore user %s: %w", u.Username, err)
		}
		restoredCount++
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit restore transaction: %w", err)
	}

	slog.Info("Successfully restored database from archive",
		"archive", archivePath,
		"restored_users", restoredCount,
		"timestamp", manifest.Timestamp,
	)

	return &RestoreResult{
		Success:    true,
		RestoredAt: time.Now().UTC(),
		Manifest:   manifest,
		RestoredRows: map[string]int{
			"users": restoredCount,
		},
	}, nil
}

// ListBackupArchives returns all backup files in the backup directory, sorted newest first.
func ListBackupArchives(backupDir string) ([]BackupResult, error) {
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return []BackupResult{}, nil
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, fmt.Errorf("read backup dir: %w", err)
	}

	var results []BackupResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		results = append(results, BackupResult{
			Filename:    entry.Name(),
			DownloadURL: "/v1/system/backups/" + entry.Name(),
			SizeBytes:   info.Size(),
			Manifest: BackupManifest{
				Timestamp: info.ModTime(),
			},
		})
	}

	// Sort newest first
	sort.Slice(results, func(i, j int) bool {
		return results[i].Manifest.Timestamp.After(results[j].Manifest.Timestamp)
	})

	return results, nil
}
