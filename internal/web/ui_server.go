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
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/shjtmy/go_sh0jitmy_template/ent"
	"github.com/shjtmy/go_sh0jitmy_template/ent/user"
	"github.com/shjtmy/go_sh0jitmy_template/internal/database"
)

//go:embed templates/* static/*
var EmbeddedAssets embed.FS

// SystemMetricsData holds telemetry for dashboard visualization.
type SystemMetricsData struct {
	CPUUsage     float64
	MemAllocMB   uint64
	MemSysMB     uint64
	Goroutines   int
	RequestCount int
}

// DashboardViewModel contains all data needed for full dashboard rendering.
type DashboardViewModel struct {
	SystemMetricsData
	Users   []*ent.User
	Backups []database.BackupResult
}

// UIServer represents the standalone HTMX web frontend server.
type UIServer struct {
	Engine     *gin.Engine
	DB         *ent.Client
	Templates  *template.Template
	StaticFS   http.FileSystem
	BackupDir  string
	ListenPort string
}

// NewUIServer initializes and configures the standalone HTMX frontend server.
func NewUIServer(db *ent.Client, backupDir string, port string) (*UIServer, error) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	tmpl, err := template.ParseFS(EmbeddedAssets, "templates/*.html", "templates/components/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	staticSub, err := fs.Sub(EmbeddedAssets, "static")
	if err != nil {
		return nil, fmt.Errorf("sub static fs: %w", err)
	}

	if backupDir == "" {
		backupDir = "data/backups"
	}

	s := &UIServer{
		Engine:     engine,
		DB:         db,
		Templates:  tmpl,
		StaticFS:   http.FS(staticSub),
		BackupDir:  backupDir,
		ListenPort: port,
	}

	s.setupRoutes()
	return s, nil
}

func (s *UIServer) setupRoutes() {
	// Static assets
	s.Engine.StaticFS("/static", s.StaticFS)

	// Health check
	s.Engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "OK"})
	})

	// Full Dashboard Page
	s.Engine.GET("/", func(c *gin.Context) {
		vm, err := s.fetchDashboardData(c.Request.Context())
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load dashboard data: %v", err)
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := s.Templates.ExecuteTemplate(c.Writer, "dashboard.html", vm); err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
		}
	})

	// HTMX Partial: System Metrics Component
	s.Engine.GET("/ui/components/system-metrics", func(c *gin.Context) {
		metrics := s.collectMetrics()
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = s.Templates.ExecuteTemplate(c.Writer, "system_metrics", metrics)
	})

	// HTMX Partial: Users Table Component
	s.Engine.GET("/ui/components/users-table", func(c *gin.Context) {
		users, _ := s.DB.User.Query().Order(ent.Asc(user.FieldID)).All(c.Request.Context())
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = s.Templates.ExecuteTemplate(c.Writer, "users_table", gin.H{"Users": users})
	})

	// HTMX Partial: Backups Panel Component
	s.Engine.GET("/ui/components/backup-panel", func(c *gin.Context) {
		backups, _ := database.ListBackupArchives(s.BackupDir)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = s.Templates.ExecuteTemplate(c.Writer, "backup_panel", gin.H{"Backups": backups})
	})

	// HTMX Action: Create Backup
	s.Engine.POST("/ui/actions/create-backup", func(c *gin.Context) {
		_, _ = database.CreateBackupArchive(c.Request.Context(), s.DB, s.BackupDir)
		backups, _ := database.ListBackupArchives(s.BackupDir)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = s.Templates.ExecuteTemplate(c.Writer, "backup_panel", gin.H{"Backups": backups})
	})
}

func (s *UIServer) collectMetrics() SystemMetricsData {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemMetricsData{
		CPUUsage:     0.8,
		MemAllocMB:   m.Alloc / 1024 / 1024,
		MemSysMB:     m.Sys / 1024 / 1024,
		Goroutines:   runtime.NumGoroutine(),
		RequestCount: 42,
	}
}

func (s *UIServer) fetchDashboardData(ctx context.Context) (*DashboardViewModel, error) {
	users, err := s.DB.User.Query().Order(ent.Asc(user.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}

	backups, _ := database.ListBackupArchives(s.BackupDir)

	return &DashboardViewModel{
		SystemMetricsData: s.collectMetrics(),
		Users:             users,
		Backups:           backups,
	}, nil
}
