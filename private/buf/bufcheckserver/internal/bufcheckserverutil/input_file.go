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
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/bufplugin-go/check"
	"github.com/gofrs/uuid/v5"
)

type inputFile struct {
	check.File
}

func newInputFile(file check.File) *inputFile {
	return &inputFile{
		File: file,
	}
}

func (i *inputFile) Path() string {
	return i.File.FileDescriptorProto().GetName()
}

// TODO: We will need to reconcile this on the client-side as right now we rely on ExternalPath
// being passed end-to-end.
func (i *inputFile) ExternalPath() string {
	return i.Path()
}

func (i *inputFile) ModuleFullName() bufmodule.ModuleFullName {
	return nil
}

func (i *inputFile) CommitID() uuid.UUID {
	return uuid.Nil
}
