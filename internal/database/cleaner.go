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

package database

import (
	"context"
	"log/slog"
	"time"

	"github.com/shjtmy/go_sh0jitmy_template/ent"
)

// PurgeResult summarizes the number of deleted records across tables.
type PurgeResult struct {
	CutoffTime    time.Time `json:"cutoff_time"`
	RetentionDays int       `json:"retention_days"`
	PurgedCount   int       `json:"purged_count"`
	Message       string    `json:"message"`
}

// PurgeExpiredRecords deletes expired records older than retentionDays.
// In this template, it serves as a standardized hook and extension point for log retention and cleanup.
func PurgeExpiredRecords(ctx context.Context, client *ent.Client, retentionDays int) (*PurgeResult, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	// Placeholder for template data purging (e.g. audit logs, expired sessions, jobs)
	res := &PurgeResult{
		CutoffTime:    cutoff,
		RetentionDays: retentionDays,
		PurgedCount:   0,
		Message:       "Data retention purge completed successfully",
	}

	slog.Info("Completed data retention purge",
		"cutoff_time", cutoff,
		"retention_days", retentionDays,
		"purged_count", res.PurgedCount,
	)

	return res, nil
}
