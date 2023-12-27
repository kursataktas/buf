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

//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris

package bufimagefuzz

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bufbuild/buf/private/buf/buftesting"
	"github.com/bufbuild/buf/private/bufpkg/bufimage"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule/bufmoduletesting"
	"github.com/bufbuild/buf/private/pkg/command"
	"github.com/bufbuild/buf/private/pkg/prototesting"
	"github.com/bufbuild/buf/private/pkg/tmp"
	"github.com/bufbuild/buf/private/pkg/tracing"
	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
	"golang.org/x/tools/txtar"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestCorpus(t *testing.T) {
	t.Parallel()
	// To focus on just one test in the corpus, put its file name here. Don't forget to revert before committing.
	focus := ""
	ctx := context.Background()
	runner := command.NewRunner()
	require.NoError(t, filepath.Walk("corpus", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if focus != "" && info.Name() != focus {
			return nil
		}
		t.Run(info.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("corpus", info.Name()))
			require.NoError(t, err)
			result, err := fuzz(ctx, runner, data)
			require.NoError(t, err)
			require.NoError(t, result.error(ctx))
		})
		return nil
	}))
}

// Fuzz is the entrypoint for the fuzzer.
// We use https://github.com/dvyukov/go-fuzz for fuzzing.
// Please follow the instructions
// in their README for help with running the fuzz targets.
//
// Commented out for now, wasn't working for a long time.
//
// Will also cause a bandeps error - we would have to move this into private/buf
// if we want to use it again.
//
// From Makefile:
//
// .PHONY: gofuzz
// gofuzz: $(GO_FUZZ)
//
//	@rm -rf $(TMP)/gofuzz
//	@mkdir -p $(TMP)/gofuzz $(TMP)/gofuzz/corpus
//	# go-fuzz-build requires github.com/dvyukov/go-fuzz be in go.mod, but we don't need that dependency otherwise.
//	# This adds go-fuzz-dep to go.mod, runs go-fuzz-build, then restores go.mod.
//	cp go.mod $(TMP)/go.mod.bak; cp go.sum $(TMP)/go.sum.bak
//	go get github.com/dvyukov/go-fuzz/go-fuzz-dep@$(GO_FUZZ_VERSION)
//	cd ./private/bufpkg/bufimage/bufimagefuzz; go-fuzz-build -o $(abspath $(TMP))/gofuzz/gofuzz.zip
//	rm go.mod go.sum; mv $(TMP)/go.mod.bak go.mod; mv $(TMP)/go.sum.bak go.sum
//	cp private/bufpkg/bufimage/bufimagefuzz/corpus[> $(TMP)/gofuzz/corpus
//	go-fuzz -bin $(TMP)/gofuzz/gofuzz.zip -workdir $(TMP)/gofuzz
//func Fuzz(data []byte) int {
//ctx := context.Background()
//runner := command.NewRunner()
//result, err := fuzz(ctx, runner, data)
//if err != nil {
//// data was invalid in some way
//return -1
//}
//return result.panicOrN(ctx)
//}

func fuzz(ctx context.Context, runner command.Runner, data []byte) (_ *fuzzResult, retErr error) {
	dir, err := tmp.NewDir()
	if err != nil {
		return nil, err
	}
	defer func() {
		retErr = multierr.Append(retErr, dir.Close())
	}()
	if err := untxtar(data, dir.AbsPath()); err != nil {
		return nil, err
	}

	filePaths, err := buftesting.GetProtocFilePathsErr(ctx, dir.AbsPath(), 0)
	if err != nil {
		return nil, err
	}

	actualProtocFileDescriptorSet, protocErr := prototesting.GetProtocFileDescriptorSet(
		ctx,
		runner,
		[]string{dir.AbsPath()},
		filePaths,
		false,
		false,
	)

	image, bufErr := fuzzBuild(ctx, dir.AbsPath())
	return newFuzzResult(
		runner,
		bufErr,
		protocErr,
		actualProtocFileDescriptorSet,
		image,
	), nil
}

// fuzzBuild does a builder.Build for a fuzz test.
func fuzzBuild(ctx context.Context, dirPath string) (bufimage.Image, error) {
	moduleSet, err := bufmoduletesting.NewModuleSetForDirPath(dirPath)
	if err != nil {
		return nil, err
	}
	return bufimage.BuildImage(
		ctx,
		tracing.NopTracer,
		bufmodule.ModuleSetToModuleReadBucketWithOnlyProtoFiles(moduleSet),
		bufimage.WithExcludeSourceCodeInfo(),
	)
}

// txtarParse is a wrapper around txtar.Parse that will turn panics into errors.
// This is necessary because of an issue where txtar.Parse can panic on invalid data. Because data is generated by the
// fuzzer, it will occasionally generate data that causes this panic.
// See https://github.com/golang/go/issues/47193
func txtarParse(data []byte) (_ *txtar.Archive, retErr error) {
	defer func() {
		if p := recover(); p != nil {
			retErr = fmt.Errorf("panic from txtar.Parse: %v", p)
		}
	}()
	return txtar.Parse(data), nil
}

// untxtar extracts txtar data to destDirPath.
func untxtar(data []byte, destDirPath string) error {
	archive, err := txtarParse(data)
	if err != nil {
		return err
	}
	if len(archive.Files) == 0 {
		return fmt.Errorf("txtar contains no files")
	}
	for _, file := range archive.Files {
		dirPath := filepath.Dir(file.Name)
		if dirPath != "" {
			if err := os.MkdirAll(filepath.Join(destDirPath, dirPath), 0700); err != nil {
				return err
			}
		}
		if err := os.WriteFile(
			filepath.Join(destDirPath, file.Name),
			file.Data,
			0600,
		); err != nil {
			return err
		}
	}
	return nil
}

type fuzzResult struct {
	runner                        command.Runner
	bufErr                        error
	protocErr                     error
	actualProtocFileDescriptorSet *descriptorpb.FileDescriptorSet
	image                         bufimage.Image
}

func newFuzzResult(
	runner command.Runner,
	bufErr error,
	protocErr error,
	actualProtocFileDescriptorSet *descriptorpb.FileDescriptorSet,
	image bufimage.Image,
) *fuzzResult {
	return &fuzzResult{
		runner:                        runner,
		bufErr:                        bufErr,
		protocErr:                     protocErr,
		actualProtocFileDescriptorSet: actualProtocFileDescriptorSet,
		image:                         image,
	}
}

// panicOrN panics if there is an error or returns the appropriate value for Fuzz to return.
func (f *fuzzResult) panicOrN(ctx context.Context) int {
	if err := f.error(ctx); err != nil {
		panic(err.Error())
	}
	// This will return 1 for valid protobufs and 0 for invalid in order to encourage the fuzzer to generate more
	// realistic looking data.
	if f.protocErr == nil {
		return 1
	}
	return 0
}

// error returns an error that should cause Fuzz to panic.
func (f *fuzzResult) error(ctx context.Context) error {
	if f.protocErr != nil {
		if f.bufErr == nil {
			return fmt.Errorf("protoc has error but buf does not: %v", f.protocErr)
		}
		return nil
	}
	if f.bufErr != nil {
		return fmt.Errorf("buf has error but protoc does not: %v", f.bufErr)
	}
	image := bufimage.ImageWithoutImports(f.image)
	fileDescriptorSet := bufimage.ImageToFileDescriptorSet(image)

	diff, err := prototesting.DiffFileDescriptorSetsJSON(
		ctx,
		f.runner,
		fileDescriptorSet,
		f.actualProtocFileDescriptorSet,
		"buf",
		"protoc",
	)
	if err != nil {
		return fmt.Errorf("error diffing results: %v", err)
	}
	if strings.TrimSpace(diff) != "" {
		return fmt.Errorf("protoc and buf have different results: %v", diff)
	}
	return nil
}
