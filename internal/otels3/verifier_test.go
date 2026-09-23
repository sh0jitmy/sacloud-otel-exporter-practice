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
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockS3Client struct {
	objects map[string][]byte
}

func (m *mockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	var contents []types.Object
	for key := range m.objects {
		k := key
		contents = append(contents, types.Object{
			Key:          aws.String(k),
			LastModified: aws.Time(time.Now()),
		})
	}
	return &s3.ListObjectsV2Output{
		Contents:    contents,
		IsTruncated: aws.Bool(false),
	}, nil
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	data, ok := m.objects[*params.Key]
	if !ok {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(data)),
	}, nil
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(data)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

const sampleOTLPJSON = `{
  "resourceLogs": [
    {
      "resource": {
        "attributes": [
          {"key": "service.name", "value": {"stringValue": "sacloud-otel-verifier"}},
          {"key": "environment", "value": {"stringValue": "verification"}}
        ]
      },
      "scopeLogs": [
        {
          "scope": {
            "name": "sacloud-otel-verifier",
            "version": "1.0.0"
          },
          "logRecords": [
            {
              "timeUnixNano": "1726912345000000000",
              "severityNumber": 9,
              "severityText": "INFO",
              "body": {"stringValue": "Verification test message 1"},
              "attributes": [
                {"key": "test_run_id", "value": {"stringValue": "run-test-12345"}},
                {"key": "seq", "value": {"intValue": 1}}
              ],
              "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
              "spanId": "00f067aa0ba902b7"
            },
            {
              "timeUnixNano": "1726912346000000000",
              "severityNumber": 13,
              "severityText": "WARN",
              "body": {"stringValue": "Verification test message 2"},
              "attributes": [
                {"key": "test_run_id", "value": {"stringValue": "run-test-12345"}},
                {"key": "seq", "value": {"intValue": 2}}
              ],
              "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
              "spanId": "00f067aa0ba902b7"
            }
          ]
        }
      ]
    }
  ]
}`

func TestParseOTLPLogJSON(t *testing.T) {
	t.Parallel()

	records, err := parseOTLPLogJSON([]byte(sampleOTLPJSON))
	require.NoError(t, err)
	require.Len(t, records, 2)

	// Record 1 assertions
	assert.Equal(t, "INFO", records[0].Severity)
	assert.Equal(t, "Verification test message 1", records[0].Body)
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", records[0].TraceID)
	assert.Equal(t, "00f067aa0ba902b7", records[0].SpanID)
	assert.Equal(t, "run-test-12345", records[0].Attributes["test_run_id"])
	assert.Equal(t, "sacloud-otel-verifier", records[0].Attributes["service.name"])
	assert.Equal(t, "1", records[0].Attributes["seq"])

	// Record 2 assertions
	assert.Equal(t, "WARN", records[1].Severity)
	assert.Equal(t, "Verification test message 2", records[1].Body)
}

func TestDecompressIfNeeded(t *testing.T) {
	t.Parallel()

	raw := []byte("plain text payload")
	decompressed, err := decompressIfNeeded(raw)
	require.NoError(t, err)
	assert.Equal(t, raw, decompressed)

	gz := gzipBytes(t, raw)
	decompressedGz, err := decompressIfNeeded(gz)
	require.NoError(t, err)
	assert.Equal(t, raw, decompressedGz)
}

func TestVerifier_VerifyRun_Success(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.MaxVerifyAttempts = 2
	cfg.PollInterval = 10 * time.Millisecond

	mock := &mockS3Client{
		objects: map[string][]byte{
			"logs/2026/09/21/18/batch-001.json.gz": gzipBytes(t, []byte(sampleOTLPJSON)),
		},
	}

	verifier := NewVerifierWithClient(cfg, mock)
	run := &TestRun{
		RunID:         "run-test-12345",
		ExpectedCount: 2,
		TraceID:       "4bf92f3577b34da6a3ce929d0e0e4736",
	}

	res, err := verifier.VerifyRun(context.Background(), run)
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, 2, res.MatchedCount)
	assert.Contains(t, res.MatchedObjects, "logs/2026/09/21/18/batch-001.json.gz")
}

func TestVerifier_VerifyRun_NotFound(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.MaxVerifyAttempts = 1
	cfg.PollInterval = 5 * time.Millisecond

	mock := &mockS3Client{
		objects: map[string][]byte{
			"logs/2026/09/21/18/batch-001.json.gz": gzipBytes(t, []byte(sampleOTLPJSON)),
		},
	}

	verifier := NewVerifierWithClient(cfg, mock)
	run := &TestRun{
		RunID:         "run-non-existent-id",
		ExpectedCount: 2,
	}

	res, err := verifier.VerifyRun(context.Background(), run)
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Equal(t, 0, res.MatchedCount)
	assert.NotEmpty(t, res.Errors)
}

func TestVerifier_VerifyRun_DockerFileLog(t *testing.T) {
	t.Parallel()

	dockerOTLPJSON := `{
  "resourceLogs": [
    {
      "resource": {
        "attributes": [
          {"key": "source", "value": {"stringValue": "docker_container"}}
        ]
      },
      "scopeLogs": [
        {
          "scope": {"name": "filelog"},
          "logRecords": [
            {
              "timeUnixNano": "1726912345000000000",
              "severityText": "INFO",
              "body": {"stringValue": "Docker container log record 1 of 2 for run docker-test-999"},
              "attributes": [
                {"key": "stream", "value": {"stringValue": "stdout"}},
                {"key": "run_id", "value": {"stringValue": "docker-test-999"}},
                {"key": "container_name", "value": {"stringValue": "sacloud-docker-log-producer"}}
              ]
            },
            {
              "timeUnixNano": "1726912346000000000",
              "severityText": "INFO",
              "body": {"stringValue": "Docker container log record 2 of 2 for run docker-test-999"},
              "attributes": [
                {"key": "stream", "value": {"stringValue": "stdout"}},
                {"key": "run_id", "value": {"stringValue": "docker-test-999"}}
              ]
            }
          ]
        }
      ]
    }
  ]
}`

	cfg := DefaultConfig()
	cfg.MaxVerifyAttempts = 1
	cfg.PollInterval = 5 * time.Millisecond

	mock := &mockS3Client{
		objects: map[string][]byte{
			"logs/2026/09/21/19/docker-001.json.gz": gzipBytes(t, []byte(dockerOTLPJSON)),
		},
	}

	verifier := NewVerifierWithClient(cfg, mock)
	run := &TestRun{
		RunID:         "docker-test-999",
		ExpectedCount: 2,
	}

	res, err := verifier.VerifyRun(context.Background(), run)
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, 2, res.MatchedCount)
	assert.Contains(t, res.MatchedObjects, "logs/2026/09/21/19/docker-001.json.gz")
	assert.Equal(t, "docker_container", res.Records[0].Attributes["source"])
	assert.Equal(t, "stdout", res.Records[0].Attributes["stream"])
}
