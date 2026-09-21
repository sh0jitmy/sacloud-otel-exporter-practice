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
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/shjtmy/go_sh0jitmy_template/ent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestUIDB(t *testing.T) *ent.Client {
	t.Helper()
	dsn := fmt.Sprintf("file:mem_ui_%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", t.Name())
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	drv := entsql.OpenDB("sqlite3", db)
	client := ent.NewClient(ent.Driver(drv))
	require.NoError(t, client.Schema.Create(context.Background()))
	return client
}

func TestUIServer_RoutesAndHTMX(t *testing.T) {
	t.Parallel()
	db := setupTestUIDB(t)
	defer func() { _ = db.Close() }()

	server, err := NewUIServer(db, "", "0")
	require.NoError(t, err)

	// 1. Healthz
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
		server.Engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "OK")
	}

	// 2. Dashboard HTML
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		server.Engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Go Template Dashboard")
		assert.Contains(t, w.Body.String(), "hx-get=\"/ui/components/system-metrics\"")
	}

	// 3. HTMX System Metrics Partial
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/ui/components/system-metrics", nil)
		server.Engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Process CPU Usage")
		assert.Contains(t, w.Body.String(), "Active Goroutines")
	}

	// 4. HTMX Users Table Partial
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/ui/components/users-table", nil)
		server.Engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "登録ユーザー一覧")
	}

	// 5. HTMX Backups Panel Partial
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/ui/components/backup-panel", nil)
		server.Engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "データベースバックアップ")
	}

	// 6. Static CSS Asset
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/static/css/dashboard.css", nil)
		server.Engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "--bg-base")
	}
}

func TestExportStaticSite(t *testing.T) {
	t.Parallel()
	db := setupTestUIDB(t)
	defer func() { _ = db.Close() }()

	tmpDir, err := os.MkdirTemp("", "ssg_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	err = ExportStaticSite(context.Background(), db, tmpDir, tmpDir)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tmpDir, "index.html"))
	assert.FileExists(t, filepath.Join(tmpDir, "static", "css", "dashboard.css"))
	assert.FileExists(t, filepath.Join(tmpDir, "static", "js", "htmx.min.js"))
}
