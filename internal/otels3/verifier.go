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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3ClientAPI abstracts the required Amazon S3 API operations for testability.
type S3ClientAPI interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// Verifier inspects Sakura Cloud Object Storage to validate OTel logs uploaded by the collector.
type Verifier struct {
	cfg      Config
	s3Client S3ClientAPI
}

// NewVerifier initializes a Verifier with an authentic AWS S3 client configured for Sakura Cloud Object Storage.
func NewVerifier(ctx context.Context, cfg Config) (*Verifier, error) {
	if cfg.S3Region == "" {
		cfg.S3Region = "jp-north-1"
	}

	awsCfg := aws.Config{
		Region: cfg.S3Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.S3AccessKey,
			cfg.S3SecretKey,
			"",
		),
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
		o.UsePathStyle = cfg.S3ForcePathStyle
	})

	return &Verifier{
		cfg:      cfg,
		s3Client: client,
	}, nil
}

// NewVerifierWithClient allows injecting a mock or pre-configured S3ClientAPI for unit tests.
func NewVerifierWithClient(cfg Config, client S3ClientAPI) *Verifier {
	return &Verifier{
		cfg:      cfg,
		s3Client: client,
	}
}

// ListLogObjects returns the keys of all objects stored under the configured prefix in the bucket.
func (v *Verifier) ListLogObjects(ctx context.Context) ([]string, error) {
	var keys []string
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(v.cfg.S3Bucket),
	}

	var continuationToken *string
	for {
		input.ContinuationToken = continuationToken
		page, err := v.s3Client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects in bucket %s: %w", v.cfg.S3Bucket, err)
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
		if page.IsTruncated != nil && *page.IsTruncated && page.NextContinuationToken != nil {
			continuationToken = page.NextContinuationToken
		} else {
			break
		}
	}

	sort.Strings(keys)
	return keys, nil
}

// FetchAndParseObject downloads a single object from S3, decompresses gzip if necessary, and extracts OTLP logs.
func (v *Verifier) FetchAndParseObject(ctx context.Context, key string) ([]LogRecordInfo, error) {
	out, err := v.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(v.cfg.S3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s: %w", key, err)
	}
	defer func() {
		_ = out.Body.Close()
	}()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object body %s: %w", key, err)
	}

	decompressed, err := decompressIfNeeded(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress object %s: %w", key, err)
	}

	return parseOTLPLogJSON(decompressed)
}

// VerifyRun polls Sakura Cloud Object Storage and verifies that the emitted batch arrived intact.
func (v *Verifier) VerifyRun(ctx context.Context, run *TestRun) (*VerificationResult, error) {
	startTime := time.Now()
	result := &VerificationResult{
		RunID:          run.RunID,
		EmittedCount:   run.ExpectedCount,
		Bucket:         v.cfg.S3Bucket,
		Prefix:         v.cfg.S3Prefix,
		ScannedObjects: make([]string, 0),
		MatchedObjects: make([]string, 0),
		Records:        make([]LogRecordInfo, 0),
	}

	attempts := v.cfg.MaxVerifyAttempts
	if attempts <= 0 {
		attempts = 10
	}
	interval := v.cfg.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, ctx.Err().Error())
			result.Duration = time.Since(startTime)
			return result, ctx.Err()
		default:
		}

		keys, err := v.ListLogObjects(ctx)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("attempt %d: list error: %v", attempt, err))
			time.Sleep(interval)
			continue
		}

		result.ScannedObjects = keys
		var currentRunRecords []LogRecordInfo
		matchedObjMap := make(map[string]bool)

		// Scan objects from newest to oldest
		for i := len(keys) - 1; i >= 0; i-- {
			key := keys[i]
			records, err := v.FetchAndParseObject(ctx, key)
			if err != nil {
				// Record parsing error and continue scanning other objects
				result.Errors = append(result.Errors, fmt.Sprintf("object %s parse error: %v", key, err))
				continue
			}

			for _, rec := range records {
				if rec.Attributes["test_run_id"] == run.RunID ||
					rec.Attributes["run_id"] == run.RunID ||
					strings.Contains(rec.Body, run.RunID) {
					currentRunRecords = append(currentRunRecords, rec)
					matchedObjMap[key] = true
				}
			}
		}

		if len(currentRunRecords) >= run.ExpectedCount {
			result.Success = true
			result.FoundCount = len(currentRunRecords)
			result.MatchedCount = len(currentRunRecords)
			result.Records = currentRunRecords
			for objKey := range matchedObjMap {
				result.MatchedObjects = append(result.MatchedObjects, objKey)
			}
			result.Duration = time.Since(startTime)
			return result, nil
		}

		if attempt < attempts {
			time.Sleep(interval)
		}
	}

	result.Duration = time.Since(startTime)
	if !result.Success {
		result.Errors = append(result.Errors, fmt.Sprintf("timed out waiting for %d logs; only found %d records for run_id %s",
			run.ExpectedCount, len(result.Records), run.RunID))
	}

	return result, nil
}

// decompressIfNeeded checks for gzip magic numbers (0x1f, 0x8b) and decompresses if present.
func decompressIfNeeded(data []byte) ([]byte, error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("gzip reader error: %w", err)
		}
		defer func() {
			_ = r.Close()
		}()
		return io.ReadAll(r)
	}
	return data, nil
}

// parseOTLPLogJSON decodes raw OTLP JSON data and extracts standardized LogRecordInfo items.
func parseOTLPLogJSON(data []byte) ([]LogRecordInfo, error) {
	var payload OTLPLogExport
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}

	var results []LogRecordInfo
	for _, resLog := range payload.ResourceLogs {
		resAttrs := make(map[string]string)
		for _, kv := range resLog.Resource.Attributes {
			resAttrs[kv.Key] = kv.Value.String()
		}

		for _, scopeLog := range resLog.ScopeLogs {
			for _, rec := range scopeLog.LogRecords {
				attrs := make(map[string]string)
				// Copy resource attributes
				for k, v := range resAttrs {
					attrs[k] = v
				}
				// Copy record attributes
				for _, kv := range rec.Attributes {
					attrs[kv.Key] = kv.Value.String()
				}

				var recTime time.Time
				if rec.TimeUnixNano != "" {
					if nano, err := strconv.ParseInt(rec.TimeUnixNano, 10, 64); err == nil {
						recTime = time.Unix(0, nano).UTC()
					}
				}

				info := LogRecordInfo{
					Time:       recTime,
					Severity:   rec.SeverityText,
					Body:       rec.Body.String(),
					TraceID:    rec.TraceID,
					SpanID:     rec.SpanID,
					Attributes: attrs,
				}
				results = append(results, info)
			}
		}
	}

	return results, nil
}
