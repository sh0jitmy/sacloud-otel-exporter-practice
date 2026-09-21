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

package web

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/shjtmy/go_sh0jitmy_template/ent"
)

// ExportStaticSite renders dashboard pages to static HTML and copies static assets to outDir.
func ExportStaticSite(ctx context.Context, db *ent.Client, backupDir, outDir string) error {
	s, err := NewUIServer(db, backupDir, "0")
	if err != nil {
		return fmt.Errorf("init ui server: %w", err)
	}

	if mkOutErr := os.MkdirAll(outDir, 0750); mkOutErr != nil {
		return fmt.Errorf("create out dir: %w", mkOutErr)
	}

	// 1. Copy static assets from embedded FS
	staticDir := filepath.Join(outDir, "static")
	if mkStaticErr := os.MkdirAll(staticDir, 0750); mkStaticErr != nil {
		return fmt.Errorf("create static dir: %w", mkStaticErr)
	}

	staticSub, err := fs.Sub(EmbeddedAssets, "static")
	if err != nil {
		return fmt.Errorf("sub static fs: %w", err)
	}

	err = fs.WalkDir(staticSub, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		targetPath := filepath.Join(staticDir, path)
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0750)
		}
		data, readErr := fs.ReadFile(staticSub, path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(targetPath, data, 0600)
	})
	if err != nil {
		return fmt.Errorf("copy static assets: %w", err)
	}

	// 2. Render index.html
	vm, err := s.fetchDashboardData(ctx)
	if err != nil {
		return fmt.Errorf("fetch dashboard data: %w", err)
	}

	var buf bytes.Buffer
	if err := s.Templates.ExecuteTemplate(&buf, "dashboard.html", vm); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	if writeErr := os.WriteFile(filepath.Join(outDir, "index.html"), buf.Bytes(), 0600); writeErr != nil {
		return fmt.Errorf("write index.html: %w", writeErr)
	}

	return nil
}
