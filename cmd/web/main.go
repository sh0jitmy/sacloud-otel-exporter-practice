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

// Package main is the entry point for the standalone HTMX web frontend server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shjtmy/go_sh0jitmy_template/internal/database"
	"github.com/shjtmy/go_sh0jitmy_template/internal/version"
	"github.com/shjtmy/go_sh0jitmy_template/internal/web"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:    "go-template-web",
		Usage:   "Standalone HTMX web dashboard for Go template",
		Version: version.Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   "3001",
				Usage:   "Port to listen on",
				EnvVars: []string{"WEB_PORT", "PORT"},
			},
			&cli.StringFlag{
				Name:    "db-driver",
				Value:   "sqlite3",
				Usage:   "Database driver (sqlite3 or postgres)",
				EnvVars: []string{"DATABASE_DRIVER"},
			},
			&cli.StringFlag{
				Name:    "db-dsn",
				Value:   "file:data/app.db?cache=shared&mode=rwc&_pragma=foreign_keys(1)",
				Usage:   "Database DSN",
				EnvVars: []string{"DATABASE_DSN", "DATABASE_URL"},
			},
			&cli.StringFlag{
				Name:    "backup-dir",
				Value:   "data/backups",
				Usage:   "Directory where database backup archives are stored",
				EnvVars: []string{"BACKUP_DIR"},
			},
			&cli.StringFlag{
				Name:  "ssg-export",
				Usage: "Export pre-rendered static site HTML and assets to specified directory and exit",
			},
		},
		Action: func(c *cli.Context) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			dbDriver := c.String("db-driver")
			dbDSN := c.String("db-dsn")
			backupDir := c.String("backup-dir")
			ssgExportDir := c.String("ssg-export")
			port := c.String("port")

			// Connect to DB
			dbClient, err := database.NewClient(ctx, dbDriver, dbDSN)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer func() { _ = dbClient.Close() }()

			// Seed admin user if needed
			_ = database.SeedAdminUser(ctx, dbClient)

			// SSG mode
			if ssgExportDir != "" {
				slog.Info("Exporting static site...", "outDir", ssgExportDir)
				if exportErr := web.ExportStaticSite(ctx, dbClient, backupDir, ssgExportDir); exportErr != nil {
					return fmt.Errorf("static site export failed: %w", exportErr)
				}
				slog.Info("Static site exported successfully!", "dir", ssgExportDir)
				return nil
			}

			// Run UI Server
			uiServer, err := web.NewUIServer(dbClient, backupDir, port)
			if err != nil {
				return fmt.Errorf("failed to create UI server: %w", err)
			}

			srv := &http.Server{
				Addr:              ":" + port,
				Handler:           uiServer.Engine,
				ReadHeaderTimeout: 5 * time.Second,
			}

			go func() {
				slog.Info("Starting Standalone HTMX Dashboard Server...", "port", port, "url", fmt.Sprintf("http://localhost:%s", port))
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					slog.Error("HTTP UI server error", "error", err)
				}
			}()

			<-ctx.Done()
			slog.Info("Shutting down UI server gracefully...")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return srv.Shutdown(shutdownCtx)
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
