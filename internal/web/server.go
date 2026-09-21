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
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shjtmy/go_sh0jitmy_template/ent"
	"github.com/shjtmy/go_sh0jitmy_template/internal/database"
	"github.com/shjtmy/go_sh0jitmy_template/internal/service"
	"github.com/shjtmy/go_sh0jitmy_template/ogen"
)

// Server は OpenAPI の ogen.ServerInterface を実装する構造体です。
var _ ogen.ServerInterface = (*Server)(nil)

type Server struct {
	db          *ent.Client
	authService *service.AuthService
	backupDir   string
}

// NewServer は API サーバーハンドラーの新しいインスタンスを返します。
func NewServer(db *ent.Client) *Server {
	backupDir := os.Getenv("BACKUP_DIR")
	if backupDir == "" {
		backupDir = "data/backups"
	}
	return &Server{
		db:          db,
		authService: service.NewAuthService(db),
		backupDir:   backupDir,
	}
}

// SetBackupDir sets the directory path for database backup archives.
func (s *Server) SetBackupDir(dir string) {
	s.backupDir = dir
}

// SetupEngine は Gin エンジンを構成し、ミドルウェアおよびハンドラーを登録します。
func SetupEngine(db *ent.Client) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// HSTSミドルウェアを全体に適用
	r.Use(HSTSSetMiddleware())

	// OTelメトリクス収集ミドルウェアを全体に適用
	r.Use(OTelMetricsMiddleware())

	s := NewServer(db)

	// メトリクススクレイプ用エンドポイント（Prometheus形式でエクスポート）
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// System & Observability エンドポイント
	r.GET("/v1/system/healthz", s.GetHealthz)
	r.GET("/v1/system/readyz", s.GetReadyz)
	r.GET("/v1/system/backups", s.ListBackups)
	r.POST("/v1/system/backups", s.CreateBackup)
	r.GET("/v1/system/backups/:filename", func(c *gin.Context) {
		s.DownloadBackup(c, c.Param("filename"))
	})
	r.POST("/v1/system/restores", s.RestoreBackup)
	r.POST("/v1/system/purge", s.PurgeRecords)

	// 認証関連
	r.POST("/v1/login", s.Login)

	authorized := r.Group("/v1")
	authorized.Use(BearerAuthMiddleware("secret-bearer-token"))
	authorized.GET("/users/me", s.GetMe)

	return r
}

// GetHealthz handles GET /v1/system/healthz.
func (s *Server) GetHealthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "OK",
		"timestamp": time.Now().UTC(),
	})
}

// GetReadyz handles GET /v1/system/readyz.
func (s *Server) GetReadyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	// Check DB connection readiness
	if _, err := s.db.User.Query().Count(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "UNAVAILABLE",
			"message": "database not ready",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "READY",
		"timestamp": time.Now().UTC(),
	})
}

// CreateBackup handles POST /v1/system/backups.
func (s *Server) CreateBackup(c *gin.Context) {
	res, err := database.CreateBackupArchive(c.Request.Context(), s.db, s.backupDir)
	if err != nil {
		slog.Error("Failed to create backup", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// ListBackups handles GET /v1/system/backups.
func (s *Server) ListBackups(c *gin.Context) {
	list, err := database.ListBackupArchives(s.backupDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// DownloadBackup handles GET /v1/system/backups/{filename}.
func (s *Server) DownloadBackup(c *gin.Context, filename string) {
	safeName := filepath.Base(filename)
	filePath := filepath.Join(s.backupDir, safeName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup file not found"})
		return
	}

	c.FileAttachment(filePath, safeName)
}

// RestoreBackup handles POST /v1/system/restores.
func (s *Server) RestoreBackup(c *gin.Context) {
	var req ogen.RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	archivePath := req.ArchivePath
	if !filepath.IsAbs(archivePath) {
		archivePath = filepath.Join(s.backupDir, filepath.Base(archivePath))
	}

	res, err := database.RestoreBackupArchive(c.Request.Context(), s.db, archivePath)
	if err != nil {
		slog.Error("Restore failed", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

// PurgeRecords handles POST /v1/system/purge.
func (s *Server) PurgeRecords(c *gin.Context) {
	var req ogen.PurgeRequest
	_ = c.ShouldBindJSON(&req)

	days := 30
	if req.RetentionDays != nil && *req.RetentionDays > 0 {
		days = *req.RetentionDays
	}

	res, err := database.PurgeExpiredRecords(c.Request.Context(), s.db, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

// Login は POST /v1/login エンドポイントの実装です。
func (s *Server) Login(c *gin.Context) {
	var req struct {
		Username string       `json:"username" binding:"required"`
		Password SecretString `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// 安全なロギングおよび監査ログの検証 (log_type: audit)
	slog.InfoContext(ctx, "Login attempt received",
		slog.String("log_type", "audit"),
		slog.String("username", req.Username),
		slog.Any("password", req.Password),
	)

	token, err := s.authService.Authenticate(ctx, req.Username, string(req.Password))
	if err != nil {
		slog.WarnContext(ctx, "Authentication failed", "username", req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
		return
	}

	slog.InfoContext(ctx, "Successfully authenticated user",
		slog.String("log_type", "audit"),
		slog.String("username", req.Username),
	)
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// GetMe は GET /v1/users/me エンドポイントの実装です。
func (s *Server) GetMe(c *gin.Context) {
	username, exists := c.Get("authenticated_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	ctx := c.Request.Context()
	u, err := s.authService.GetUserByUsername(ctx, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       u.ID,
		"username": u.Username,
	})
}
