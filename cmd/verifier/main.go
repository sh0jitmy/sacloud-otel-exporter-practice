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

// Package main provides the CLI tool for verifying OpenTelemetry log export to Sakura Cloud Object Storage.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/shjtmy/go_sh0jitmy_template/internal/otels3"
	"github.com/shjtmy/go_sh0jitmy_template/internal/version"
	"github.com/urfave/cli/v2"
)

func main() {
	envFile := ".env"
	for i, arg := range os.Args {
		if (arg == "--env-file" || arg == "-env-file") && i+1 < len(os.Args) {
			envFile = os.Args[i+1]
		} else if strings.HasPrefix(arg, "--env-file=") {
			envFile = strings.TrimPrefix(arg, "--env-file=")
		}
	}
	if envEnv := os.Getenv("ENV_FILE"); envEnv != "" && envFile == ".env" {
		envFile = envEnv
	}
	_ = loadEnvFile(envFile)

	app := &cli.App{
		Name:    "sacloud-otel-verifier",
		Usage:   "Verify OpenTelemetry log delivery to Sakura Cloud Object Storage via sacloud-otel-collector",
		Version: version.Version,
		Flags:   globalFlags(),
		Commands: []*cli.Command{
			{
				Name:    "roundtrip",
				Aliases: []string{"run"},
				Usage:   "Emit OTel logs, wait for collector batch upload, and verify delivery in Sakura Object Storage (default)",
				Flags:   globalFlags(),
				Action:  runRoundtrip,
			},
			{
				Name:   "emit",
				Usage:  "Emit a batch of test logs via OpenTelemetry SDK to the collector",
				Action: runEmit,
				Flags: append(globalFlags(),
					&cli.StringFlag{
						Name:  "run-id",
						Usage: "Custom test run identifier (auto-generated if empty)",
					},
				),
			},
			{
				Name:   "verify",
				Usage:  "Verify whether a specific test run ID exists in Sakura Cloud Object Storage",
				Action: runVerify,
				Flags: append(globalFlags(),
					&cli.StringFlag{
						Name:     "run-id",
						Usage:    "Target test run identifier to search for in storage",
						Required: true,
					},
					&cli.IntFlag{
						Name:  "expected-count",
						Value: 5,
						Usage: "Expected minimum number of matched log records",
					},
				),
			},
			{
				Name:   "inspect",
				Usage:  "List objects under the prefix and inspect latest log records in Sakura Object Storage",
				Action: runInspect,
				Flags: append(globalFlags(),
					&cli.IntFlag{
						Name:  "limit",
						Value: 5,
						Usage: "Maximum number of recent objects to inspect",
					},
				),
			},
		},
		Action: runRoundtrip, // Default action if no subcommand is given
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := app.RunContext(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}
}

func globalFlags() []cli.Flag {
	def := otels3.DefaultConfig()
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "env-file",
			Value:   ".env",
			Usage:   "Path to .env file to load environment variables from",
			EnvVars: []string{"ENV_FILE"},
		},
		&cli.StringFlag{
			Name:    "collector-endpoint",
			Value:   def.CollectorEndpoint,
			Usage:   "OpenTelemetry Collector OTLP gRPC endpoint",
			EnvVars: []string{"OTEL_COLLECTOR_ENDPOINT", "OTEL_EXPORTER_OTLP_ENDPOINT"},
		},
		&cli.BoolFlag{
			Name:    "collector-insecure",
			Value:   def.Insecure,
			Usage:   "Use insecure gRPC connection to collector",
			EnvVars: []string{"OTEL_COLLECTOR_INSECURE"},
		},
		&cli.StringFlag{
			Name:    "s3-endpoint",
			Value:   def.S3Endpoint,
			Usage:   "Sakura Cloud Object Storage endpoint URL (e.g. https://s3.isk01.sakurastorage.jp)",
			EnvVars: []string{"SACLOUD_O2_ENDPOINT", "AWS_ENDPOINT_URL", "S3_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:    "s3-region",
			Value:   def.S3Region,
			Usage:   "S3 region string (e.g. jp-north-1 for Ishikari, jp-east-1 for Tokyo)",
			EnvVars: []string{"SACLOUD_O2_REGION", "AWS_REGION", "S3_REGION"},
		},
		&cli.StringFlag{
			Name:    "s3-bucket",
			Value:   def.S3Bucket,
			Usage:   "Sakura Cloud Object Storage bucket name",
			EnvVars: []string{"SACLOUD_O2_BUCKET", "S3_BUCKET"},
		},
		&cli.StringFlag{
			Name:    "s3-prefix",
			Value:   def.S3Prefix,
			Usage:   "S3 key prefix where logs are partitioned",
			EnvVars: []string{"SACLOUD_O2_PREFIX", "S3_PREFIX"},
		},
		&cli.StringFlag{
			Name:    "s3-access-key",
			Usage:   "Sakura Cloud Object Storage Access Key",
			EnvVars: []string{"SACLOUD_O2_ACCESS_KEY", "AWS_ACCESS_KEY_ID"},
		},
		&cli.StringFlag{
			Name:    "s3-secret-key",
			Usage:   "Sakura Cloud Object Storage Secret Key",
			EnvVars: []string{"SACLOUD_O2_SECRET_KEY", "AWS_SECRET_ACCESS_KEY"},
		},
		&cli.BoolFlag{
			Name:    "s3-force-path-style",
			Value:   def.S3ForcePathStyle,
			Usage:   "Force path-style S3 URLs (bucket in path)",
			EnvVars: []string{"SACLOUD_O2_PATH_STYLE", "S3_FORCE_PATH_STYLE"},
		},
		&cli.BoolFlag{
			Name:    "s3-disable-ssl",
			Value:   def.S3DisableSSL,
			Usage:   "Disable SSL for local testing (e.g. MinIO)",
			EnvVars: []string{"SACLOUD_O2_DISABLE_SSL", "S3_DISABLE_SSL"},
		},
		&cli.IntFlag{
			Name:    "count",
			Value:   def.LogCount,
			Usage:   "Number of test log records to emit",
			EnvVars: []string{"LOG_COUNT"},
		},
		&cli.DurationFlag{
			Name:    "flush-wait",
			Value:   def.FlushWaitTime,
			Usage:   "Duration to wait for collector batch processor before polling S3",
			EnvVars: []string{"FLUSH_WAIT_TIME"},
		},
		&cli.IntFlag{
			Name:    "max-attempts",
			Value:   def.MaxVerifyAttempts,
			Usage:   "Maximum S3 polling attempts",
			EnvVars: []string{"MAX_VERIFY_ATTEMPTS"},
		},
		&cli.DurationFlag{
			Name:    "poll-interval",
			Value:   def.PollInterval,
			Usage:   "Interval between S3 polling attempts",
			EnvVars: []string{"POLL_INTERVAL"},
		},
		&cli.StringFlag{
			Name:    "service-name",
			Value:   def.ServiceName,
			Usage:   "OpenTelemetry service.name resource attribute",
			EnvVars: []string{"OTEL_SERVICE_NAME"},
		},
	}
}

func configFromContext(c *cli.Context) otels3.Config {
	return otels3.Config{
		CollectorEndpoint: c.String("collector-endpoint"),
		Insecure:          c.Bool("collector-insecure"),
		ServiceName:       c.String("service-name"),
		S3Endpoint:        c.String("s3-endpoint"),
		S3Region:          c.String("s3-region"),
		S3Bucket:          c.String("s3-bucket"),
		S3Prefix:          c.String("s3-prefix"),
		S3AccessKey:       c.String("s3-access-key"),
		S3SecretKey:       c.String("s3-secret-key"),
		S3ForcePathStyle:  c.Bool("s3-force-path-style"),
		S3DisableSSL:      c.Bool("s3-disable-ssl"),
		LogCount:          c.Int("count"),
		FlushWaitTime:     c.Duration("flush-wait"),
		MaxVerifyAttempts: c.Int("max-attempts"),
		PollInterval:      c.Duration("poll-interval"),
	}
}

func runRoundtrip(c *cli.Context) error {
	cfg := configFromContext(c)
	ctx := c.Context

	printHeader("OpenTelemetry ➔ Sakura Cloud Object Storage E2E Verification", cfg)

	// Step 1: Initialize Emitter
	fmt.Printf("==> [1/3] Initializing OTel Log Emitter (Collector: %s)...\n", cfg.CollectorEndpoint)
	emitter, err := otels3.NewEmitter(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create emitter: %w", err)
	}
	defer func() {
		_ = emitter.Close(ctx)
	}()

	// Step 2: Emit Logs
	fmt.Printf("==> [2/3] Emitting %d structured log records with OTel Trace Context...\n", cfg.LogCount)
	run, err := emitter.EmitTestLogs(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to emit test logs: %w", err)
	}

	fmt.Printf("    ✓ Logs dispatched! RunID: %s\n", run.RunID)
	fmt.Printf("    ✓ Attached TraceID: %s\n", run.TraceID)
	fmt.Printf("    ✓ Attached SpanID:  %s\n", run.SpanID)
	fmt.Printf("    ⏳ Waiting %v for sacloud-otel-collector batch processor to flush to S3...\n", cfg.FlushWaitTime)
	time.Sleep(cfg.FlushWaitTime)

	// Step 3: Verify in Sakura Cloud Object Storage
	fmt.Printf("==> [3/3] Querying Sakura Cloud Object Storage (%s/%s)...\n", cfg.S3Endpoint, cfg.S3Bucket)
	verifier, err := otels3.NewVerifier(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create verifier: %w", err)
	}

	result, err := verifier.VerifyRun(ctx, run)
	if err != nil {
		return fmt.Errorf("verification query failed: %w", err)
	}

	printReport(result)

	if !result.Success {
		return fmt.Errorf("verification FAILED: expected %d records, found %d", result.EmittedCount, result.MatchedCount)
	}

	return nil
}

func runEmit(c *cli.Context) error {
	cfg := configFromContext(c)
	ctx := c.Context
	runID := c.String("run-id")

	printHeader("OpenTelemetry Log Emission", cfg)

	emitter, err := otels3.NewEmitter(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize emitter: %w", err)
	}
	defer func() {
		_ = emitter.Close(ctx)
	}()

	run, err := emitter.EmitTestLogs(ctx, runID)
	if err != nil {
		return fmt.Errorf("emission failed: %w", err)
	}

	fmt.Println("✅ Successfully emitted test logs to sacloud-otel-collector:")
	fmt.Printf("  • Test Run ID:  %s\n", run.RunID)
	fmt.Printf("  • Record Count: %d\n", run.ExpectedCount)
	fmt.Printf("  • Trace ID:     %s\n", run.TraceID)
	fmt.Printf("  • Span ID:      %s\n", run.SpanID)
	fmt.Printf("  • Timestamp:    %s\n", run.Timestamp.Format(time.RFC3339))
	fmt.Println("\nTo verify delivery in storage, run:")
	fmt.Printf("  verifier verify --run-id %s --expected-count %d\n", run.RunID, run.ExpectedCount)

	return nil
}

func runVerify(c *cli.Context) error {
	cfg := configFromContext(c)
	ctx := c.Context
	runID := c.String("run-id")
	expectedCount := c.Int("expected-count")

	printHeader(fmt.Sprintf("Verifying Test Run: %s", runID), cfg)

	verifier, err := otels3.NewVerifier(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize verifier: %w", err)
	}

	run := &otels3.TestRun{
		RunID:         runID,
		ExpectedCount: expectedCount,
	}

	result, err := verifier.VerifyRun(ctx, run)
	if err != nil {
		return fmt.Errorf("verification error: %w", err)
	}

	printReport(result)
	if !result.Success {
		return fmt.Errorf("verification FAILED")
	}
	return nil
}

func runInspect(c *cli.Context) error {
	cfg := configFromContext(c)
	ctx := c.Context
	limit := c.Int("limit")

	printHeader("Sakura Cloud Object Storage Log Inspector", cfg)

	verifier, err := otels3.NewVerifier(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize verifier: %w", err)
	}

	keys, err := verifier.ListLogObjects(ctx)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	fmt.Printf("Found %d total objects under prefix '%s':\n", len(keys), cfg.S3Prefix)
	for idx, k := range keys {
		fmt.Printf("  [%d] %s\n", idx+1, k)
	}
	start := 0
	if len(keys) > limit {
		start = len(keys) - limit
	}

	for i := len(keys) - 1; i >= start; i-- {
		key := keys[i]
		fmt.Printf("\n📦 Object: %s\n", key)
		records, err := verifier.FetchAndParseObject(ctx, key)
		if err != nil {
			fmt.Printf("   ⚠️  Failed to parse object: %v\n", err)
			continue
		}
		fmt.Printf("   Contains %d log record(s):\n", len(records))
		for idx, rec := range records {
			fmt.Printf("   [%d] %s | %-5s | %s (TraceID: %s)\n",
				idx+1, rec.Time.Format("15:04:05.000"), rec.Severity, rec.Body, rec.TraceID)
			if len(rec.Attributes) > 0 {
				attrBytes, _ := json.Marshal(rec.Attributes)
				fmt.Printf("       Attributes: %s\n", string(attrBytes))
			}
		}
	}

	return nil
}

func printHeader(title string, cfg otels3.Config) {
	fmt.Println("================================================================================")
	fmt.Printf("  %s\n", title)
	fmt.Println("================================================================================")
	fmt.Printf("  • Collector Endpoint : %s (insecure: %t)\n", cfg.CollectorEndpoint, cfg.Insecure)
	fmt.Printf("  • Storage Endpoint   : %s (region: %s)\n", cfg.S3Endpoint, cfg.S3Region)
	fmt.Printf("  • Target Bucket      : %s (prefix: %s)\n", cfg.S3Bucket, cfg.S3Prefix)
	fmt.Printf("  • Access Key (len)   : %d chars (empty=%t)\n", len(cfg.S3AccessKey), cfg.S3AccessKey == "")
	fmt.Printf("  • Path-Style         : %t | SSL: %t\n", cfg.S3ForcePathStyle, !cfg.S3DisableSSL)
	fmt.Println("--------------------------------------------------------------------------------")
}

func printReport(r *otels3.VerificationResult) {
	fmt.Println("\n================================================================================")
	fmt.Println("                             VERIFICATION REPORT                                ")
	fmt.Println("================================================================================")
	statusText := "✅ PASSED"
	if !r.Success {
		statusText = "❌ FAILED"
	}
	fmt.Printf("  Overall Status   : %s\n", statusText)
	fmt.Printf("  Test Run ID      : %s\n", r.RunID)
	fmt.Printf("  Emitted Records  : %d\n", r.EmittedCount)
	fmt.Printf("  Matched Records  : %d\n", r.MatchedCount)
	fmt.Printf("  Target Storage   : %s/%s\n", r.Bucket, r.Prefix)
	fmt.Printf("  Scan Duration    : %v\n", r.Duration.Round(time.Millisecond))
	fmt.Printf("  Objects Scanned  : %d\n", len(r.ScannedObjects))
	fmt.Printf("  Matched Objects  : %d\n", len(r.MatchedObjects))

	if len(r.MatchedObjects) > 0 {
		fmt.Println("\n  [Matched Storage Keys]")
		for _, key := range r.MatchedObjects {
			fmt.Printf("    • %s\n", key)
		}
	}

	if len(r.Records) > 0 {
		fmt.Println("\n  [Verified Log Records Sample]")
		for i, rec := range r.Records {
			if i >= 5 {
				fmt.Printf("    ... and %d more verified records\n", len(r.Records)-5)
				break
			}
			fmt.Printf("    [%d] %-5s %s\n", i+1, rec.Severity, rec.Body)
			fmt.Printf("        TraceID: %s | SpanID: %s\n", rec.TraceID, rec.SpanID)
			fmt.Printf("        Seq: %s | Time: %s\n", rec.Attributes["seq"], rec.Time.Format(time.RFC3339Nano))
		}
	}

	if len(r.Errors) > 0 {
		fmt.Println("\n  [Errors / Warnings]")
		for _, errStr := range r.Errors {
			fmt.Printf("    ⚠️  %s\n", strings.TrimSpace(errStr))
		}
	}
	fmt.Println("================================================================================")
}

func loadEnvFile(path string) error {
	cleanPath := filepath.Clean(path)
	f, err := os.Open(cleanPath) //nolint:gosec // Path is specified by user via CLI flag or default .env
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			_ = os.Setenv(k, v)
		}
	}
	return scanner.Err()
}
