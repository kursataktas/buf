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
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strings"

	"github.com/bufbuild/buf/private/buf/bufpluginexec"
	"github.com/bufbuild/buf/private/bufpkg/bufplugin/bufpluginref"
	"github.com/bufbuild/buf/private/bufpkg/bufremoteplugin"
	"github.com/bufbuild/buf/private/pkg/encoding"
	"github.com/bufbuild/buf/private/pkg/syserror"
)

const (
	remoteAlphaPluginDeprecationMessage = "the remote field no longer works as " +
		"the remote generation alpha has been deprecated, see the migration guide to " +
		"now-stable remote plugins: https://buf.build/docs/migration-guides/migrate-remote-generation-alpha/#migrate-to-remote-plugins"
)

// GenerateStrategy is the generation strategy for a protoc plugin.
type GenerateStrategy int

const (
	// GenerateStrategyDirectory is the strategy to generate per directory.
	//
	// This is the default value for local plugins.
	GenerateStrategyDirectory GenerateStrategy = 1
	// GenerateStrategyAll is the strategy to generate with all files at once.
	//
	// This is the only strategy for remote plugins.
	GenerateStrategyAll GenerateStrategy = 2
)

// PluginConfigType is a plugin configuration type.
type PluginConfigType int

const (
	// PluginConfigTypeRemote is the remote plugin config type.
	PluginConfigTypeRemote PluginConfigType = iota + 1
	// PluginConfigTypeBinary is the binary plugin config type.
	PluginConfigTypeBinary
	// PluginConfigTypeProtocBuiltin is the protoc built-in plugin config type.
	PluginConfigTypeProtocBuiltin
	// PluginConfigTypeLocal is the local plugin config type. This type indicates
	// it is to be determined whether the plugin is binary or protoc built-in.
	// We defer further classification to the plugin executor. In v2 the exact
	// plugin config type is always specified and it will never be just local.
	PluginConfigTypeLocal
)

// GeneratePluginConfig is a configuration for a plugin.
type GeneratePluginConfig interface {
	// Type returns the plugin type. This is never the zero value.
	Type() PluginConfigType
	// Name returns the plugin name. This is never empty.
	Name() string
	// Out returns the output directory for generation. This is never empty.
	Out() string
	// Opt returns the plugin options as a comma separated string.
	Opt() string
	// IncludeImports returns whether to generate code for imported files. This
	// is always false in v1.
	IncludeImports() bool
	// IncludeWKT returns whether to generate code for the well-known types.
	// This returns true only if IncludeImports returns true. This is always
	// false in v1.
	IncludeWKT() bool
	// Strategy returns the generation strategy.
	//
	// This is not empty only when the plugin is local, binary or protoc builtin.
	Strategy() GenerateStrategy
	// Path returns the path, including arguments, to invoke the binary plugin.
	//
	// This is not empty only when the plugin is binary.
	Path() []string
	// ProtocPath returns a path to protoc.
	//
	// This is not empty only when the plugin is protoc-builtin
	ProtocPath() string
	// RemoteHost returns the remote host of the remote plugin.
	//
	// This is not empty only when the plugin is remote.
	RemoteHost() string
	// Revision returns the revision of the remote plugin.
	//
	// This is not empty only when the plugin is remote.
	Revision() int

	isGeneratePluginConfig()
}

// NewRemotePluginConfig returns a new GeneratePluginConfig for a remote plugin.
func NewRemotePluginConfig(
	name string,
	out string,
	opt []string,
	includeImports bool,
	includeWKT bool,
	revision int,
) (GeneratePluginConfig, error) {
	return newRemotePluginConfig(
		name,
		out,
		opt,
		includeImports,
		includeWKT,
		revision,
	)
}

// NewLocalPluginConfig returns a new GeneratePluginConfig for a local plugin.
func NewLocalPluginConfig(
	name string,
	out string,
	opt []string,
	includeImports bool,
	includeWKT bool,
	strategy *GenerateStrategy,
) (GeneratePluginConfig, error) {
	return newLocalPluginConfig(
		name,
		out,
		opt,
		includeImports,
		includeWKT,
		strategy,
	)
}

// NewBinaryPluginConfig returns a new GeneratePluginConfig for a binary plugin.
func NewBinaryPluginConfig(
	name string,
	out string,
	opt []string,
	includeImports bool,
	includeWKT bool,
	strategy *GenerateStrategy,
	path []string,
) (GeneratePluginConfig, error) {
	return newBinaryPluginConfig(
		name,
		out,
		opt,
		includeImports,
		includeWKT,
		strategy,
		path,
	)
}

// NewProtocBuiltinPluginConfig returns a new GeneratePluginConfig for a protoc
// builtin plugin.
func NewProtocBuiltinPluginConfig(
	name string,
	out string,
	opt []string,
	includeImports bool,
	includeWKT bool,
	strategy *GenerateStrategy,
	protocPath string,
) (GeneratePluginConfig, error) {
	return newProtocBuiltinPluginConfig(
		name,
		out,
		opt,
		includeImports,
		includeWKT,
		strategy,
		protocPath,
	)
}

// NewGeneratePluginWithIncludeImportsAndWKT returns a GeneratePluginConfig the
// same as the input, with include imports and include wkt overridden.
func NewGeneratePluginWithIncludeImportsAndWKT(
	config GeneratePluginConfig,
	includeImports bool,
	includeWKT bool,
) (GeneratePluginConfig, error) {
	originalConfig, ok := config.(*pluginConfig)
	if !ok {
		return nil, syserror.Newf("unknown implementation of GeneratePluginConfig: %T", config)
	}
	pluginConfig := *originalConfig
	if includeImports {
		pluginConfig.includeImports = true
	}
	if includeWKT {
		pluginConfig.includeWKT = true
	}
	return &pluginConfig, nil
}

// *** PRIVATE ***

type pluginConfig struct {
	pluginConfigType PluginConfigType
	name             string
	out              string
	opts             []string
	includeImports   bool
	includeWKT       bool
	strategy         *GenerateStrategy
	path             []string
	protocPath       string
	remoteHost       string
	revision         int
}

func newPluginConfigFromExternalV1Beta1(
	externalConfig externalGeneratePluginConfigV1Beta1,
) (GeneratePluginConfig, error) {
	if externalConfig.Name == "" {
		return nil, errors.New("plugin name is required")
	}
	if externalConfig.Out == "" {
		return nil, fmt.Errorf("out is required for plugin %s", externalConfig.Name)
	}
	strategy, err := parseStrategy(externalConfig.Strategy)
	if err != nil {
		return nil, err
	}
	opt, err := encoding.InterfaceSliceOrStringToStringSlice(externalConfig.Opt)
	if err != nil {
		return nil, err
	}
	if externalConfig.Path != "" {
		return newBinaryPluginConfig(
			externalConfig.Name,
			externalConfig.Out,
			opt,
			false,
			false,
			strategy,
			[]string{externalConfig.Path},
		)
	}
	return newLocalPluginConfig(
		externalConfig.Name,
		externalConfig.Out,
		opt,
		false,
		false,
		strategy,
	)
}

func newPluginConfigFromExternalV1(
	externalConfig externalGeneratePluginConfigV1,
) (GeneratePluginConfig, error) {
	if externalConfig.Remote != "" {
		return nil, errors.New(remoteAlphaPluginDeprecationMessage)
	}
	// In v1 config, only plugin and name are allowed, since remote alpha plugin
	// has been deprecated.
	if externalConfig.Plugin == "" && externalConfig.Name == "" {
		return nil, fmt.Errorf("one of plugin or name is required")
	}
	if externalConfig.Plugin != "" && externalConfig.Name != "" {
		return nil, fmt.Errorf("only one of plugin or name can be set")
	}
	var pluginIdentifier string
	switch {
	case externalConfig.Plugin != "":
		pluginIdentifier = externalConfig.Plugin
		if _, _, _, _, err := bufremoteplugin.ParsePluginVersionPath(pluginIdentifier); err == nil {
			// A remote alpha plugin name is not a valid remote plugin reference.
			return nil, fmt.Errorf("invalid remote plugin reference: %s", pluginIdentifier)
		}
	case externalConfig.Name != "":
		pluginIdentifier = externalConfig.Name
		if _, _, _, _, err := bufremoteplugin.ParsePluginVersionPath(pluginIdentifier); err == nil {
			return nil, fmt.Errorf("invalid plugin name %s, did you mean to use a remote plugin?", pluginIdentifier)
		}
		if bufpluginref.IsPluginReferenceOrIdentity(pluginIdentifier) {
			// A remote alpha plugin name is not a valid local plugin name.
			return nil, fmt.Errorf("invalid local plugin name: %s", pluginIdentifier)
		}
	}
	if externalConfig.Out == "" {
		return nil, fmt.Errorf("out is required for plugin %s", pluginIdentifier)
	}
	strategy, err := parseStrategy(externalConfig.Strategy)
	if err != nil {
		return nil, err
	}
	opt, err := encoding.InterfaceSliceOrStringToStringSlice(externalConfig.Opt)
	if err != nil {
		return nil, err
	}
	path, err := encoding.InterfaceSliceOrStringToStringSlice(externalConfig.Path)
	if err != nil {
		return nil, err
	}
	if externalConfig.Plugin != "" && bufpluginref.IsPluginReferenceOrIdentity(pluginIdentifier) {
		if externalConfig.Path != nil {
			return nil, fmt.Errorf("cannot specify path for remote plugin %s", externalConfig.Plugin)
		}
		if externalConfig.Strategy != "" {
			return nil, fmt.Errorf("cannot specify strategy for remote plugin %s", externalConfig.Plugin)
		}
		if externalConfig.ProtocPath != "" {
			return nil, fmt.Errorf("cannot specify protoc_path for remote plugin %s", externalConfig.Plugin)
		}
		return newRemotePluginConfig(
			externalConfig.Plugin,
			externalConfig.Out,
			opt,
			false,
			false,
			externalConfig.Revision,
		)
	}
	// At this point the plugin must be local, regardless whehter it's specified
	// by key 'plugin' or 'name'.
	if len(path) > 0 {
		return newBinaryPluginConfig(
			pluginIdentifier,
			externalConfig.Out,
			opt,
			false,
			false,
			strategy,
			path,
		)
	}
	if externalConfig.ProtocPath != "" {
		return newProtocBuiltinPluginConfig(
			pluginIdentifier,
			externalConfig.Out,
			opt,
			false,
			false,
			strategy,
			externalConfig.ProtocPath,
		)
	}
	// It could be either binary or protoc built-in. We defer to the plugin executor
	// to decide whether the plugin is protoc-builtin or binary.
	return newLocalPluginConfig(
		pluginIdentifier,
		externalConfig.Out,
		opt,
		false,
		false,
		strategy,
	)
}

func newPluginConfigFromExternalV2(
	externalConfig externalGeneratePluginConfigV2,
) (GeneratePluginConfig, error) {
	var pluginTypeCount int
	if externalConfig.Remote != nil {
		pluginTypeCount++
	}
	if externalConfig.Binary != nil {
		pluginTypeCount++
	}
	if externalConfig.ProtocBuiltin != nil {
		pluginTypeCount++
	}
	if pluginTypeCount == 0 {
		return nil, errors.New("must specify one of remote, binary and protoc_builtin")
	}
	if pluginTypeCount > 1 {
		return nil, errors.New("only one of remote, binary and protoc_builtin")
	}
	var strategy string
	if externalConfig.Strategy != nil {
		strategy = *externalConfig.Strategy
	}
	parsedStrategy, err := parseStrategy(strategy)
	if err != nil {
		return nil, err
	}
	opt, err := encoding.InterfaceSliceOrStringToStringSlice(externalConfig.Opt)
	if err != nil {
		return nil, err
	}
	switch {
	case externalConfig.Remote != nil:
		var revision int
		if externalConfig.Revision != nil {
			revision = *externalConfig.Revision
		}
		if externalConfig.Strategy != nil {
			return nil, fmt.Errorf("cannot specify strategy for remote plugin %s", *externalConfig.Remote)
		}
		if externalConfig.ProtocPath != nil {
			return nil, fmt.Errorf("cannot specify protoc_path for remote plugin %s", *externalConfig.Remote)
		}
		return newRemotePluginConfig(
			*externalConfig.Remote,
			externalConfig.Out,
			opt,
			externalConfig.IncludeImports,
			externalConfig.IncludeWKT,
			revision,
		)
	case externalConfig.Binary != nil:
		path, err := encoding.InterfaceSliceOrStringToStringSlice(externalConfig.Binary)
		if err != nil {
			return nil, err
		}
		binaryPluginName := strings.Join(path, " ")
		if externalConfig.Revision != nil {
			return nil, fmt.Errorf("cannot specify revision for binary plugin %s", binaryPluginName)
		}
		if externalConfig.ProtocPath != nil {
			return nil, fmt.Errorf("cannot specify protoc_path for binary plugin %s", binaryPluginName)
		}
		return newBinaryPluginConfig(
			strings.Join(path, " "),
			externalConfig.Out,
			opt,
			externalConfig.IncludeImports,
			externalConfig.IncludeWKT,
			parsedStrategy,
			path,
		)
	case externalConfig.ProtocBuiltin != nil:
		var protocPath string
		if externalConfig.ProtocPath != nil {
			protocPath = *externalConfig.ProtocPath
		}
		if externalConfig.Revision != nil {
			return nil, fmt.Errorf("cannot specify revision for protoc built-in plugin %s", *externalConfig.ProtocBuiltin)
		}
		return newProtocBuiltinPluginConfig(
			*externalConfig.ProtocBuiltin,
			externalConfig.Out,
			opt,
			externalConfig.IncludeImports,
			externalConfig.IncludeWKT,
			parsedStrategy,
			protocPath,
		)
	default:
		return nil, syserror.Newf("must specify one of remote, binary and protoc_builtin")
	}
}

func newRemotePluginConfig(
	name string,
	out string,
	opt []string,
	includeImports bool,
	includeWKT bool,
	revision int,
) (*pluginConfig, error) {
	if includeWKT && !includeImports {
		return nil, errors.New("cannot include well-known types without including imports")
	}
	remoteHost, err := parseRemoteHostName(name)
	if err != nil {
		return nil, err
	}
	if revision < 0 || revision > math.MaxInt32 {
		return nil, fmt.Errorf("revision %d is out of accepted range %d-%d", revision, 0, math.MaxInt32)
	}
	return &pluginConfig{
		pluginConfigType: PluginConfigTypeRemote,
		name:             name,
		remoteHost:       remoteHost,
		revision:         revision,
		out:              out,
		opts:             opt,
		includeImports:   includeImports,
		includeWKT:       includeWKT,
	}, nil
}

func newLocalPluginConfig(
	name string,
	out string,
	opt []string,
	includeImports bool,
	includeWKT bool,
	strategy *GenerateStrategy,
) (*pluginConfig, error) {
	if includeWKT && !includeImports {
		return nil, errors.New("cannot include well-known types without including imports")
	}
	return &pluginConfig{
		pluginConfigType: PluginConfigTypeLocal,
		name:             name,
		strategy:         strategy,
		out:              out,
		opts:             opt,
		includeImports:   includeImports,
		includeWKT:       includeWKT,
	}, nil
}

func newBinaryPluginConfig(
	name string,
	out string,
	opt []string,
	includeImports bool,
	includeWKT bool,
	strategy *GenerateStrategy,
	path []string,
) (*pluginConfig, error) {
	if len(path) == 0 {
		return nil, errors.New("must specify a path to the plugin")
	}
	if includeWKT && !includeImports {
		return nil, errors.New("cannot include well-known types without including imports")
	}
	return &pluginConfig{
		pluginConfigType: PluginConfigTypeBinary,
		name:             name,
		path:             path,
		strategy:         strategy,
		out:              out,
		opts:             opt,
		includeImports:   includeImports,
		includeWKT:       includeWKT,
	}, nil
}

func newProtocBuiltinPluginConfig(
	name string,
	out string,
	opt []string,
	includeImports bool,
	includeWKT bool,
	strategy *GenerateStrategy,
	protocPath string,
) (*pluginConfig, error) {
	if includeWKT && !includeImports {
		return nil, errors.New("cannot include well-known types without including imports")
	}
	return &pluginConfig{
		pluginConfigType: PluginConfigTypeProtocBuiltin,
		name:             name,
		protocPath:       protocPath,
		out:              out,
		opts:             opt,
		strategy:         strategy,
		includeImports:   includeImports,
		includeWKT:       includeWKT,
	}, nil
}

func (p *pluginConfig) Type() PluginConfigType {
	return p.pluginConfigType
}

func (p *pluginConfig) Name() string {
	return p.name
}

func (p *pluginConfig) Out() string {
	return p.out
}

func (p *pluginConfig) Opt() string {
	return strings.Join(p.opts, ",")
}

func (p *pluginConfig) IncludeImports() bool {
	return p.includeImports
}

func (p *pluginConfig) IncludeWKT() bool {
	return p.includeWKT
}

func (p *pluginConfig) Strategy() GenerateStrategy {
	if p.strategy == nil {
		return GenerateStrategyDirectory
	}
	return *p.strategy
}

func (p *pluginConfig) Path() []string {
	return p.path
}

func (p *pluginConfig) ProtocPath() string {
	return p.protocPath
}

func (p *pluginConfig) RemoteHost() string {
	return p.remoteHost
}

func (p *pluginConfig) Revision() int {
	return p.revision
}

func (p *pluginConfig) isGeneratePluginConfig() {}

func newExternalGeneratePluginConfigV2FromPluginConfig(
	generatePluginConfig GeneratePluginConfig,
) (externalGeneratePluginConfigV2, error) {
	pluginConfig, ok := generatePluginConfig.(*pluginConfig)
	if !ok {
		return externalGeneratePluginConfigV2{}, syserror.Newf("unknown implementation of GeneratePluginConfig: %T", generatePluginConfig)
	}
	externalPluginConfigV2 := externalGeneratePluginConfigV2{
		Out:            generatePluginConfig.Out(),
		IncludeImports: generatePluginConfig.IncludeImports(),
		IncludeWKT:     generatePluginConfig.IncludeWKT(),
	}
	opts := pluginConfig.opts
	switch {
	case len(opts) == 1:
		externalPluginConfigV2.Opt = opts[0]
	case len(opts) > 1:
		externalPluginConfigV2.Opt = opts
	}
	strategy := pluginConfig.strategy
	switch {
	case strategy != nil && *strategy == GenerateStrategyDirectory:
		externalPluginConfigV2.Strategy = toPointer("directory")
	case strategy != nil && *strategy == GenerateStrategyAll:
		externalPluginConfigV2.Strategy = toPointer("all")
	}
	switch generatePluginConfig.Type() {
	case PluginConfigTypeRemote:
		externalPluginConfigV2.Remote = toPointer(generatePluginConfig.Name())
		if revision := generatePluginConfig.Revision(); revision != 0 {
			externalPluginConfigV2.Revision = &revision
		}
	case PluginConfigTypeBinary:
		path := generatePluginConfig.Path()
		switch {
		case len(path) == 1:
			externalPluginConfigV2.Binary = path[0]
		case len(path) > 1:
			externalPluginConfigV2.Binary = path
		}
	case PluginConfigTypeProtocBuiltin:
		externalPluginConfigV2.ProtocBuiltin = toPointer(generatePluginConfig.Name())
		if protocPath := generatePluginConfig.ProtocPath(); protocPath != "" {
			externalPluginConfigV2.ProtocPath = &protocPath
		}
	case PluginConfigTypeLocal:
		binaryName := "protoc-gen-" + generatePluginConfig.Name()
		_, err := exec.LookPath(binaryName)
		if err == nil || errors.Is(err, exec.ErrDot) {
			externalPluginConfigV2.Binary = binaryName
			break
		}
		if _, isProtocBuiltin := bufpluginexec.ProtocProxyPluginNames[generatePluginConfig.Name()]; isProtocBuiltin {
			externalPluginConfigV2.ProtocBuiltin = toPointer(generatePluginConfig.Name())
			break
		}
		return externalGeneratePluginConfigV2{}, fmt.Errorf("plugin %s is not found locally and %s is not built-in to protoc", binaryName, generatePluginConfig.Name())
	}
	return externalPluginConfigV2, nil
}

func parseStrategy(s string) (*GenerateStrategy, error) {
	var strategy GenerateStrategy
	switch s {
	case "":
		return nil, nil
	case "directory":
		strategy = GenerateStrategyDirectory
	case "all":
		strategy = GenerateStrategyAll
	default:
		return nil, fmt.Errorf("unknown strategy: %s", s)
	}
	return &strategy, nil
}

func parseRemoteHostName(fullName string) (string, error) {
	if identity, err := bufpluginref.PluginIdentityForString(fullName); err == nil {
		return identity.Remote(), nil
	}
	reference, err := bufpluginref.PluginReferenceForString(fullName, 0)
	if err == nil {
		return reference.Remote(), nil
	}
	return "", err
}

// TODO: where to put this?
func toPointer[T any](value T) *T {
	return &value
}
