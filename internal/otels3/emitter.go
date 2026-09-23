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

package otels3

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Emitter is responsible for initializing the OpenTelemetry Log SDK pipeline and emitting structured test logs.
type Emitter struct {
	cfg            Config
	loggerProvider *sdklog.LoggerProvider
	tracerProvider *sdktrace.TracerProvider
	logger         *slog.Logger
	tracer         trace.Tracer
}

// NewEmitter creates a new Emitter instance configured to send logs to the specified collector endpoint.
func NewEmitter(ctx context.Context, cfg Config) (*Emitter, error) {
	// 1. Build OTLP Log Exporter options
	opts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(cfg.CollectorEndpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlploggrpc.WithInsecure())
	}

	logExporter, err := otlploggrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize otlp log exporter: %w", err)
	}

	// 2. Define resource attributes (using NewSchemaless to avoid Schema URL collision)
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			attribute.String("environment", "verification"),
			attribute.String("sacloud.storage_target", cfg.S3Bucket),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 3. Configure LoggerProvider with SimpleProcessor to ensure synchronous dispatch on emission
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	// 4. Configure TracerProvider for context/trace propagation
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	)

	// 5. Construct logger with otelslog bridge
	logger := otelslog.NewLogger(
		cfg.ServiceName,
		otelslog.WithLoggerProvider(lp),
	)
	tracer := tp.Tracer(cfg.ServiceName)

	return &Emitter{
		cfg:            cfg,
		loggerProvider: lp,
		tracerProvider: tp,
		logger:         logger,
		tracer:         tracer,
	}, nil
}

// Close shuts down the LoggerProvider and TracerProvider, flushing remaining records.
func (e *Emitter) Close(ctx context.Context) error {
	var errs []error
	if e.loggerProvider != nil {
		if err := e.loggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown logger provider: %w", err))
		}
	}
	if e.tracerProvider != nil {
		if err := e.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown tracer provider: %w", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// EmitTestLogs emits a batch of structured test logs correlated with a unique test_run_id and OpenTelemetry Trace.
func (e *Emitter) EmitTestLogs(ctx context.Context, runID string) (*TestRun, error) {
	if runID == "" {
		runID = generateRunID()
	}

	// Create root verification span to attach TraceID / SpanID to all logs
	spanCtx, span := e.tracer.Start(ctx, "SacloudOTelS3VerificationBatch",
		trace.WithAttributes(
			attribute.String("test_run_id", runID),
			attribute.Int("log_count", e.cfg.LogCount),
		),
	)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()
	now := time.Now().UTC()

	count := e.cfg.LogCount
	if count <= 0 {
		count = 5
	}

	for i := 1; i <= count; i++ {
		msg := fmt.Sprintf("Verification log record %d of %d for run %s", i, count, runID)
		attrs := []any{
			slog.String("test_run_id", runID),
			slog.Int("seq", i),
			slog.Int("total", count),
			slog.String("target_bucket", e.cfg.S3Bucket),
			slog.String("target_prefix", e.cfg.S3Prefix),
			slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano)),
		}

		// Emit varied severities for thorough testing
		switch {
		case i == 1:
			e.logger.InfoContext(spanCtx, msg, attrs...)
		case i == 2 && count >= 3:
			e.logger.WarnContext(spanCtx, msg+" [warn-check]", attrs...)
		case i == count:
			e.logger.InfoContext(spanCtx, msg+" [batch-complete]", attrs...)
		default:
			e.logger.InfoContext(spanCtx, msg, attrs...)
		}
	}

	// Force flush to ensure collector receives logs immediately
	flushCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := e.loggerProvider.ForceFlush(flushCtx); err != nil {
		return nil, fmt.Errorf("force flush to collector failed: %w", err)
	}

	return &TestRun{
		RunID:         runID,
		Timestamp:     now,
		ExpectedCount: count,
		TraceID:       traceID,
		SpanID:        spanID,
	}, nil
}

func generateRunID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("run-%s-%s", time.Now().Format("20060102-150405"), hex.EncodeToString(b))
}
