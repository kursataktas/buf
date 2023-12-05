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
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/bufbuild/buf/private/bufpkg/bufcas"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/pkg/encoding"
	"github.com/bufbuild/buf/private/pkg/slicesext"
	"github.com/bufbuild/buf/private/pkg/storage"
	"github.com/bufbuild/buf/private/pkg/syserror"
)

var (
	// bufLockFileHeader is the header prepended to any lock files.
	bufLockFileHeader = []byte("# Generated by buf. DO NOT EDIT.\n")

	bufLock          = newFileName("buf.lock", FileVersionV1Beta1, FileVersionV1, FileVersionV2)
	bufLockFileNames = []*fileName{bufLock}

	deprecatedDigestTypeToPrefix = map[string]string{
		"b1": "b1-",
		"b3": "b3-",
	}
)

// BufLockFile represents a buf.lock file.
type BufLockFile interface {
	File

	// DepModuleKeys returns the ModuleKeys representing the dependencies as specified in the buf.lock file.
	//
	// Note that ModuleKeys may not have CommitIDs with FileVersionV2.
	// CommitIDs are required for v1beta1 and v1 buf.lock files. Their existence will be verified
	// when calling NewFile or WriteFile for FileVersionV1Beta1 or FileVersionV1, and therefor
	// if FileVersion() is FileVersionV1Beta1 or FileVersionV1, all ModuleKeys will have CommitIDs.
	//
	// All ModuleKeys will have unique ModuleFullNames.
	// ModuleKeys are sorted by ModuleFullName.
	//
	// TODO: We need to add DigestTypes for all the deprecated digests. We then can handle
	// the fact that they're deprecated outside of this package. Another option is to add a
	// buflock.DeprecatedDigestTypeError to return from Digest(), and then handle that downstream.
	DepModuleKeys() []bufmodule.ModuleKey

	isBufLockFile()
}

// NewBufLockFile returns a new validated BufLockFile.
//
// Note that digests are lazily-loaded; if you need to ensure that all digests are valid, run
// ValidateBufLockFileDigests().
func NewBufLockFile(fileVersion FileVersion, depModuleKeys []bufmodule.ModuleKey) (BufLockFile, error) {
	return newBufLockFile(fileVersion, depModuleKeys)
}

// GetBufLockFileForPrefix gets the buf.lock file at the given bucket prefix.
//
// The buf.lock file will be attempted to be read at prefix/buf.lock.
//
// Note that digests are lazily-loaded; if you need to ensure that all digests are valid, run
// ValidateFileDigests().
func GetBufLockFileForPrefix(
	ctx context.Context,
	bucket storage.ReadBucket,
	prefix string,
) (BufLockFile, error) {
	return getFileForPrefix(ctx, bucket, prefix, bufLockFileNames, readBufLockFile)
}

// GetBufLockFileForPrefix gets the buf.lock file version at the given bucket prefix.
//
// The buf.lock file will be attempted to be read at prefix/buf.lock.
func GetBufLockFileVersionForPrefix(
	ctx context.Context,
	bucket storage.ReadBucket,
	prefix string,
) (FileVersion, error) {
	return getFileVersionForPrefix(ctx, bucket, prefix, bufLockFileNames)
}

// PutBufLockFileForPrefix puts the buf.lock file at the given bucket prefix.
//
// The buf.lock file will be attempted to be written to prefix/buf.lock.
// The buf.lock file will be written atomically.
func PutBufLockFileForPrefix(
	ctx context.Context,
	bucket storage.WriteBucket,
	prefix string,
	bufLockFile BufLockFile,
) error {
	return putFileForPrefix(ctx, bucket, prefix, bufLockFile, bufLock, writeBufLockFile)
}

// ReadBufLockFile reads the BufLockFile from the io.Reader.
//
// Note that digests are lazily-loaded; if you need to ensure that all digests are valid, run
// ValidateFileDigests().
func ReadBufLockFile(reader io.Reader) (BufLockFile, error) {
	return readFile(reader, "lock file", readBufLockFile)
}

// WriteBufLockFile writes the BufLockFile to the io.Writer.
func WriteBufLockFile(writer io.Writer, bufLockFile BufLockFile) error {
	return writeFile(writer, "lock file", bufLockFile, writeBufLockFile)
}

// *** PRIVATE ***

type bufLockFile struct {
	fileVersion   FileVersion
	depModuleKeys []bufmodule.ModuleKey
}

func newBufLockFile(
	fileVersion FileVersion,
	depModuleKeys []bufmodule.ModuleKey,
) (*bufLockFile, error) {
	if err := validateNoDuplicateModuleKeysByModuleFullName(depModuleKeys); err != nil {
		return nil, err
	}
	// To make sure we aren't editing input.
	depModuleKeys = slicesext.Copy(depModuleKeys)
	sort.Slice(
		depModuleKeys,
		func(i int, j int) bool {
			return depModuleKeys[i].ModuleFullName().String() < depModuleKeys[j].ModuleFullName().String()
		},
	)
	bufLockFile := &bufLockFile{
		fileVersion:   fileVersion,
		depModuleKeys: depModuleKeys,
	}
	if err := validateV1AndV1Beta1DepsHaveCommits(bufLockFile); err != nil {
		return nil, err
	}
	return bufLockFile, nil
}

func (l *bufLockFile) FileVersion() FileVersion {
	return l.fileVersion
}

func (l *bufLockFile) DepModuleKeys() []bufmodule.ModuleKey {
	return l.depModuleKeys
}

func (*bufLockFile) isBufLockFile() {}
func (*bufLockFile) isFile()        {}

func readBufLockFile(
	reader io.Reader,
	allowJSON bool,
) (BufLockFile, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	fileVersion, err := getFileVersionForData(data, allowJSON)
	if err != nil {
		return nil, err
	}
	switch fileVersion {
	case FileVersionV1Beta1, FileVersionV1:
		var externalBufLockFile externalBufLockFileV1Beta1V1
		if err := getUnmarshalStrict(allowJSON)(data, &externalBufLockFile); err != nil {
			return nil, fmt.Errorf("invalid as version %v: %w", fileVersion, err)
		}
		depModuleKeys := make([]bufmodule.ModuleKey, len(externalBufLockFile.Deps))
		for i, dep := range externalBufLockFile.Deps {
			dep := dep
			if dep.Remote == "" {
				return nil, errors.New("remote missing")
			}
			if dep.Owner == "" {
				return nil, errors.New("owner missing")
			}
			if dep.Repository == "" {
				return nil, errors.New("repository missing")
			}
			moduleFullName, err := bufmodule.NewModuleFullName(
				dep.Remote,
				dep.Owner,
				dep.Repository,
			)
			if err != nil {
				return nil, fmt.Errorf("invalid module name: %w", err)
			}
			if dep.Commit == "" {
				return nil, fmt.Errorf("no commit specified for module %s", moduleFullName.String())
			}
			if dep.Digest == "" {
				return nil, fmt.Errorf("no digest specified for module %s", moduleFullName.String())
			}
			for digestType, prefix := range deprecatedDigestTypeToPrefix {
				if strings.HasPrefix(dep.Digest, prefix) {
					return nil, fmt.Errorf(`%s digests are no longer supported, run "buf mod update" to update your buf.lock`, digestType)
				}
			}
			depModuleKey, err := bufmodule.NewModuleKey(
				moduleFullName,
				dep.Commit,
				func() (bufcas.Digest, error) {
					return bufcas.ParseDigest(dep.Digest)
				},
			)
			if err != nil {
				return nil, err
			}
			depModuleKeys[i] = depModuleKey
		}
		return newBufLockFile(fileVersion, depModuleKeys)
	case FileVersionV2:
		var externalBufLockFile externalBufLockFileV2
		if err := getUnmarshalStrict(allowJSON)(data, &externalBufLockFile); err != nil {
			return nil, fmt.Errorf("invalid as version %v: %w", fileVersion, err)
		}
		depModuleKeys := make([]bufmodule.ModuleKey, len(externalBufLockFile.Deps))
		for i, dep := range externalBufLockFile.Deps {
			dep := dep
			if dep.Name == "" {
				return nil, errors.New("no module name specified")
			}
			moduleFullName, err := bufmodule.ParseModuleFullName(dep.Name)
			if err != nil {
				return nil, fmt.Errorf("invalid module name: %w", err)
			}
			if dep.Digest == "" {
				return nil, fmt.Errorf("no digest specified for module %s", moduleFullName.String())
			}
			for digestType, prefix := range deprecatedDigestTypeToPrefix {
				if strings.HasPrefix(dep.Digest, prefix) {
					return nil, fmt.Errorf(`%s digests are no longer supported, run "buf mod update" to update your buf.lock`, digestType)
				}
			}
			depModuleKey, err := bufmodule.NewModuleKey(
				moduleFullName,
				"",
				func() (bufcas.Digest, error) {
					return bufcas.ParseDigest(dep.Digest)
				},
			)
			if err != nil {
				return nil, err
			}
			depModuleKeys[i] = depModuleKey
		}
		return newBufLockFile(fileVersion, depModuleKeys)
	default:
		// This is a system error since we've already parsed.
		return nil, syserror.Newf("unknown FileVersion: %v", fileVersion)
	}
}

func writeBufLockFile(
	writer io.Writer,
	bufLockFile BufLockFile,
) error {
	if err := validateV1AndV1Beta1DepsHaveCommits(bufLockFile); err != nil {
		return err
	}
	switch fileVersion := bufLockFile.FileVersion(); fileVersion {
	case FileVersionV1Beta1, FileVersionV1:
		depModuleKeys := bufLockFile.DepModuleKeys()
		externalBufLockFile := externalBufLockFileV1Beta1V1{
			Version: fileVersion.String(),
			Deps:    make([]externalBufLockFileDepV1Beta1V1, len(depModuleKeys)),
		}
		for i, depModuleKey := range depModuleKeys {
			digest, err := depModuleKey.Digest()
			if err != nil {
				return err
			}
			externalBufLockFile.Deps[i] = externalBufLockFileDepV1Beta1V1{
				Remote:     depModuleKey.ModuleFullName().Registry(),
				Owner:      depModuleKey.ModuleFullName().Owner(),
				Repository: depModuleKey.ModuleFullName().Name(),
				Commit:     depModuleKey.CommitID(),
				Digest:     digest.String(),
			}
		}
		// No need to sort - depModuleKeys is already sorted by ModuleFullName
		data, err := encoding.MarshalYAML(&externalBufLockFile)
		if err != nil {
			return err
		}
		_, err = writer.Write(append(bufLockFileHeader, data...))
		return err
	case FileVersionV2:
		depModuleKeys := bufLockFile.DepModuleKeys()
		externalBufLockFile := externalBufLockFileV2{
			Version: fileVersion.String(),
			Deps:    make([]externalBufLockFileDepV2, len(depModuleKeys)),
		}
		for i, depModuleKey := range depModuleKeys {
			digest, err := depModuleKey.Digest()
			if err != nil {
				return err
			}
			externalBufLockFile.Deps[i] = externalBufLockFileDepV2{
				Name:   depModuleKey.ModuleFullName().String(),
				Digest: digest.String(),
			}
		}
		// No need to sort - depModuleKeys is already sorted by ModuleFullName
		data, err := encoding.MarshalYAML(&externalBufLockFile)
		if err != nil {
			return err
		}
		_, err = writer.Write(append(bufLockFileHeader, data...))
		return err
	default:
		// This is a system error since we've already parsed.
		return syserror.Newf("unknown FileVersion: %v", fileVersion)
	}
}

func validateNoDuplicateModuleKeysByModuleFullName(moduleKeys []bufmodule.ModuleKey) error {
	moduleFullNameStringMap := make(map[string]struct{})
	for _, moduleKey := range moduleKeys {
		moduleFullNameString := moduleKey.ModuleFullName().String()
		if _, ok := moduleFullNameStringMap[moduleFullNameString]; ok {
			return fmt.Errorf("duplicate module %q attempted to be added to lock file", moduleFullNameString)
		}
		moduleFullNameStringMap[moduleFullNameString] = struct{}{}
	}
	return nil
}

func validateV1AndV1Beta1DepsHaveCommits(bufLockFile BufLockFile) error {
	switch fileVersion := bufLockFile.FileVersion(); fileVersion {
	case FileVersionV1Beta1, FileVersionV1:
		for _, depModuleKey := range bufLockFile.DepModuleKeys() {
			if depModuleKey.CommitID() == "" {
				// This is a system error.
				return syserror.Newf(
					"%s lock files require commits, however we did not have a commit for module %q",
					fileVersion.String(),
					depModuleKey.ModuleFullName().String(),
				)
			}
		}
		return nil
	case FileVersionV2:
		// We do not need commits in v2.
		return nil
	default:
		// This is a system error since we've already parsed.
		return syserror.Newf("unknown FileVersion: %v", fileVersion)
	}
}

// externalBufLockFileV1Beta1V1 represents the v1 or v1beta1 buf.lock file,
// which have the same shape.
type externalBufLockFileV1Beta1V1 struct {
	Version string                            `json:"version,omitempty" yaml:"version,omitempty"`
	Deps    []externalBufLockFileDepV1Beta1V1 `json:"deps,omitempty" yaml:"deps,omitempty"`
}

// externalBufLockFileDepV1Beta1V1 represents a single dep within a v1 or v1beta1 buf.lock file,
// which have the same shape.
type externalBufLockFileDepV1Beta1V1 struct {
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
