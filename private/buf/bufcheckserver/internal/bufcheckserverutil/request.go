// Copyright 2020-2024 Buf Technologies, Inc.
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

package bufcheckserverutil

import (
	"github.com/bufbuild/buf/private/bufpkg/bufprotosource"
	"github.com/bufbuild/bufplugin-go/check"
)

type request struct {
	check.Request

	protosourceFiles        []bufprotosource.File
	againstProtosourceFiles []bufprotosource.File
}

func newRequest(
	checkRequest check.Request,
	protosourceFiles []bufprotosource.File,
	againstProtosourceFiles []bufprotosource.File,
) *request {
	return &request{
		Request:                 checkRequest,
		protosourceFiles:        protosourceFiles,
		againstProtosourceFiles: againstProtosourceFiles,
	}
}

func (r *request) ProtosourceFiles() []bufprotosource.File {
	return r.protosourceFiles
}

func (r *request) AgainstProtosourceFiles() []bufprotosource.File {
	return r.againstProtosourceFiles
}
