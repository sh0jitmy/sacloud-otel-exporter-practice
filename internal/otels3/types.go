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
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Config holds configuration parameters for the OTel emitter and Sakura Object Storage verifier.
type Config struct {
	// CollectorEndpoint specifies the OpenTelemetry Collector endpoint (e.g., "localhost:4317").
	CollectorEndpoint string `json:"collector_endpoint"`
	// Insecure specifies whether gRPC connection to collector should be unencrypted.
	Insecure bool `json:"insecure"`
	// ServiceName is the service.name resource attribute for emitted logs.
	ServiceName string `json:"service_name"`

	// S3Endpoint is the Sakura Cloud Object Storage or S3 endpoint (e.g. "https://s3.isk01.sakurastorage.jp").
	S3Endpoint string `json:"s3_endpoint"`
	// S3Region is the S3 region string (e.g. "jp-north-1" for Ishikari, "jp-east-1" for Tokyo).
	S3Region string `json:"s3_region"`
	// S3Bucket is the target bucket name.
	S3Bucket string `json:"s3_bucket"`
	// S3Prefix is the prefix where logs are stored (e.g., "logs").
	S3Prefix string `json:"s3_prefix"`
	// S3AccessKey is the Access Key for Sakura Cloud Object Storage.
	S3AccessKey string `json:"s3_access_key"`
	// S3SecretKey is the Secret Key for Sakura Cloud Object Storage.
	S3SecretKey string `json:"s3_secret_key"`
	// S3ForcePathStyle forces path-style addressing (required for Sakura Cloud Object Storage).
	S3ForcePathStyle bool `json:"s3_force_path_style"`
	// S3DisableSSL disables SSL for local S3 emulators like MinIO.
	S3DisableSSL bool `json:"s3_disable_ssl"`

	// LogCount is the number of test logs to emit in a verification run.
	LogCount int `json:"log_count"`
	// FlushWaitTime is the duration to wait after emission for the collector batcher to flush to S3.
	FlushWaitTime time.Duration `json:"flush_wait_time"`
	// MaxVerifyAttempts is the maximum number of poll attempts when checking S3 for uploaded logs.
	MaxVerifyAttempts int `json:"max_verify_attempts"`
	// PollInterval is the sleep duration between S3 polling attempts.
	PollInterval time.Duration `json:"poll_interval"`
}

// DefaultConfig returns recommended default settings for local and Sakura Cloud testing.
func DefaultConfig() Config {
	return Config{
		CollectorEndpoint: "localhost:4317",
		Insecure:          true,
		ServiceName:       "sacloud-otel-verifier",
		S3Endpoint:        "https://s3.isk01.sakurastorage.jp",
		S3Region:          "jp-north-1",
		S3Bucket:          "otel-logs",
		S3Prefix:          "logs",
		S3ForcePathStyle:  true,
		S3DisableSSL:      false,
		LogCount:          5,
		FlushWaitTime:     4 * time.Second,
		MaxVerifyAttempts: 10,
		PollInterval:      2 * time.Second,
	}
}

// TestRun represents metadata about an emitted test batch.
type TestRun struct {
	RunID         string    `json:"run_id"`
	Timestamp     time.Time `json:"timestamp"`
	ExpectedCount int       `json:"expected_count"`
	TraceID       string    `json:"trace_id"`
	SpanID        string    `json:"span_id"`
}

// LogRecordInfo is a simplified representation of a parsed OTel log record from storage.
type LogRecordInfo struct {
	Time       time.Time         `json:"time"`
	Severity   string            `json:"severity"`
	Body       string            `json:"body"`
	TraceID    string            `json:"trace_id,omitempty"`
	SpanID     string            `json:"span_id,omitempty"`
	Attributes map[string]string `json:"attributes"`
}

// VerificationResult summarizes the verification outcome.
type VerificationResult struct {
	Success        bool            `json:"success"`
	RunID          string          `json:"run_id"`
	EmittedCount   int             `json:"emitted_count"`
	FoundCount     int             `json:"found_count"`
	MatchedCount   int             `json:"matched_count"`
	Bucket         string          `json:"bucket"`
	Prefix         string          `json:"prefix"`
	ScannedObjects []string        `json:"scanned_objects"`
	MatchedObjects []string        `json:"matched_objects"`
	Records        []LogRecordInfo `json:"records"`
	Duration       time.Duration   `json:"duration"`
	Errors         []string        `json:"errors,omitempty"`
}

// OTLP JSON format structs as exported by OpenTelemetry Collector awss3exporter.

// OTLPLogExport represents the root structure of an OTLP JSON log payload.
type OTLPLogExport struct {
	ResourceLogs []ResourceLog `json:"resourceLogs"`
}

// ResourceLog contains resources and associated scope logs.
type ResourceLog struct {
	Resource  Resource   `json:"resource"`
	ScopeLogs []ScopeLog `json:"scopeLogs"`
}

// Resource contains resource attributes.
type Resource struct {
	Attributes []KeyValue `json:"attributes"`
}

// ScopeLog groups log records under an instrumentation scope.
type ScopeLog struct {
	Scope      Scope           `json:"scope"`
	LogRecords []OTLPLogRecord `json:"logRecords"`
}

// Scope contains scope details.
type Scope struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// OTLPLogRecord represents an individual OTLP log record.
type OTLPLogRecord struct {
	TimeUnixNano         string     `json:"timeUnixNano"`
	ObservedTimeUnixNano string     `json:"observedTimeUnixNano"`
	SeverityNumber       int        `json:"severityNumber"`
	SeverityText         string     `json:"severityText"`
	Body                 AnyValue   `json:"body"`
	Attributes           []KeyValue `json:"attributes"`
	TraceID              string     `json:"traceId"`
	SpanID               string     `json:"spanId"`
}

// KeyValue represents an OTLP attribute pair.
type KeyValue struct {
	Key   string   `json:"key"`
	Value AnyValue `json:"value"`
}

// AnyValue wraps various OTLP primitive value types.
type AnyValue struct {
	StringValue *string     `json:"stringValue,omitempty"`
	BoolValue   *bool       `json:"boolValue,omitempty"`
	IntValue    any         `json:"intValue,omitempty"`
	DoubleValue *float64    `json:"doubleValue,omitempty"`
	ArrayValue  *ArrayValue `json:"arrayValue,omitempty"`
	KvlistValue *KeyValList `json:"kvlistValue,omitempty"`
	BytesValue  *string     `json:"bytesValue,omitempty"`
}

// ArrayValue wraps a list of values.
type ArrayValue struct {
	Values []AnyValue `json:"values"`
}

// KeyValList wraps a list of KeyValue pairs.
type KeyValList struct {
	Values []KeyValue `json:"values"`
}

// String returns the string representation of AnyValue regardless of concrete underlying type.
func (v AnyValue) String() string {
	if v.StringValue != nil {
		return *v.StringValue
	}
	if v.BoolValue != nil {
		return fmt.Sprintf("%t", *v.BoolValue)
	}
	if v.IntValue != nil {
		switch val := v.IntValue.(type) {
		case string:
			return val
		case float64:
			return strconv.FormatInt(int64(val), 10)
		default:
			return fmt.Sprintf("%v", val)
		}
	}
	if v.DoubleValue != nil {
		return fmt.Sprintf("%f", *v.DoubleValue)
	}
	if v.BytesValue != nil {
		return *v.BytesValue
	}
	if v.ArrayValue != nil {
		b, _ := json.Marshal(v.ArrayValue)
		return string(b)
	}
	if v.KvlistValue != nil {
		b, _ := json.Marshal(v.KvlistValue)
		return string(b)
	}
	return ""
}
