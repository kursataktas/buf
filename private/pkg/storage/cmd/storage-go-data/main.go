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

package main

import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"io"
	"math"
	"path/filepath"
	"sort"

	"github.com/bufbuild/buf/private/pkg/app"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/storage"
	"github.com/bufbuild/buf/private/pkg/storage/storageos"
	"github.com/spf13/pflag"
)

const (
	programName = "storage-go-data"

	pkgFlagName = "package"

	sliceLength = math.MaxInt64
)

func main() {
	appcmd.Main(context.Background(), newCommand())
}

func newCommand() *appcmd.Command {
	flags := newFlags()
	return &appcmd.Command{
		Use:  fmt.Sprintf("%s path/to/dir", programName),
		Args: appcmd.ExactArgs(1),
		Run: func(ctx context.Context, container app.Container) error {
			return run(ctx, container, flags)
		},
		BindFlags: flags.Bind,
	}
}

type flags struct {
	Pkg string
}

func newFlags() *flags {
	return &flags{}
}

func (f *flags) Bind(flagSet *pflag.FlagSet) {
	flagSet.StringVar(
		&f.Pkg,
		pkgFlagName,
		"",
		"The name of the generated package.",
	)
}

func run(ctx context.Context, container app.Container, flags *flags) error {
	dirPath := container.Arg(0)
	packageName := flags.Pkg
	if packageName == "" {
		packageName = filepath.Base(dirPath)
	}
	pathToData, err := getPathToData(ctx, dirPath)
	if err != nil {
		return err
	}
	golangFileData, err := getGolangFileData(pathToData, packageName)
	if err != nil {
		return err
	}
	_, err = container.Stdout().Write(golangFileData)
	return err
}

func getPathToData(ctx context.Context, dirPath string) (map[string][]byte, error) {
	readWriteBucket, err := storageos.NewProvider(storageos.ProviderWithSymlinks()).NewReadWriteBucket(dirPath)
	if err != nil {
		return nil, err
	}
	pathToData := make(map[string][]byte)
	if err := storage.WalkReadObjects(
		ctx,
		readWriteBucket,
		"",
		func(readObject storage.ReadObject) error {
			data, err := io.ReadAll(readObject)
			if err != nil {
				return err
			}
			pathToData[readObject.Path()] = data
			return nil
		},
	); err != nil {
		return nil, err
	}
	return pathToData, nil
}

func getGolangFileData(pathToData map[string][]byte, packageName string) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	_, _ = buffer.WriteString(`// Code generated by `)
	_, _ = buffer.WriteString(programName)
	_, _ = buffer.WriteString(`. DO NOT EDIT.

package `)
	_, _ = buffer.WriteString(packageName)
	_, _ = buffer.WriteString(`

import (
	"github.com/bufbuild/buf/private/pkg/storage"
	"github.com/bufbuild/buf/private/pkg/storage/storagemem"
	"github.com/bufbuild/buf/private/pkg/normalpath"
)

var (
	// ReadBucket is the storage.ReadBucket with the static data generated for this package.
	ReadBucket storage.ReadBucket

	pathToData = map[string][]byte{
`)

	paths := make([]string, 0, len(pathToData))
	for path := range pathToData {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		_, _ = buffer.WriteString(`"`)
		_, _ = buffer.WriteString(path)
		_, _ = buffer.WriteString(`": {
`)
		data := pathToData[path]
		for len(data) > 0 {
			n := sliceLength
			if n > len(data) {
				n = len(data)
			}
			accum := ""
			for _, elem := range data[:n] {
				accum += fmt.Sprintf("0x%02x,", elem)
			}
			_, _ = buffer.WriteString(accum)
			_, _ = buffer.WriteString("\n")
			data = data[n:]
		}
		_, _ = buffer.WriteString(`},
`)
	}
	_, _ = buffer.WriteString(`}
)

func init() {
	readBucket, err := storagemem.NewReadBucket(pathToData)
	if err != nil {
		panic(err.Error())
	}
	ReadBucket = readBucket
}

// Exists returns true if the given path exists in the static data.
//
// The path is normalized within this function.
func Exists(path string) bool {
	_, ok := pathToData[normalpath.Normalize(path)]
	return ok
}
`)

	return format.Source(buffer.Bytes())
}
