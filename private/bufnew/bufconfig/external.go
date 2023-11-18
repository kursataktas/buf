// Copyright 2020-2023 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bufconfig

import (
	"time"
)

var (
	// bufLockFileHeader is the header prepended to any lock files.
	bufLockFileHeader = []byte("# Generated by buf. DO NOT EDIT.\n")
)

// externalBufLockFileV1OrV1Beta1 represents the v1 or v1beta1 buf.lock file,
// which have the same shape.
type externalBufLockFileV1OrV1Beta1 struct {
	Version string                              `json:"version,omitempty" yaml:"version,omitempty"`
	Deps    []externalBufLockFileDepV1OrV1Beta1 `json:"deps,omitempty" yaml:"deps,omitempty"`
}

// externalBufLockFileDepV1OrV1Beta1 represents a single dep within a v1 or v1beta1 buf.lock file,
// which have the same shape.
type externalBufLockFileDepV1OrV1Beta1 struct {
	Remote     string    `json:"remote,omitempty" yaml:"remote,omitempty"`
	Owner      string    `json:"owner,omitempty" yaml:"owner,omitempty"`
	Repository string    `json:"repository,omitempty" yaml:"repository,omitempty"`
	Branch     string    `json:"branch,omitempty" yaml:"branch,omitempty"`
	Commit     string    `json:"commit,omitempty" yaml:"commit,omitempty"`
	Digest     string    `json:"digest,omitempty" yaml:"digest,omitempty"`
	CreateTime time.Time `json:"create_time,omitempty" yaml:"create_time,omitempty"`
}

// externalBufLockFileV2 represents the v2 buf.lock file.
type externalBufLockFileV2 struct {
	Version string                     `json:"version,omitempty" yaml:"version,omitempty"`
	Deps    []externalBufLockFileDepV2 `json:"deps,omitempty" yaml:"deps,omitempty"`
}

// externalBufLockFileDepV2 represents a single dep within a v2 buf.lock file.
type externalBufLockFileDepV2 struct {
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`
	Digest string `json:"digest,omitempty" yaml:"digest,omitempty"`
}

// externalFileVersion represents just the version component of any file.
type externalFileVersion struct {
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}
