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

// Package version defines application semantic version and build metadata.
package version

// Version is the current semantic version of the application.
// Managed and automatically bumped by tagpr in CI/CD.
var Version = "0.0.1"

// Commit is the git commit hash injected during build time by GoReleaser (-ldflags).
var Commit = "none"

// Date is the build timestamp injected during build time by GoReleaser (-ldflags).
var Date = "unknown"
