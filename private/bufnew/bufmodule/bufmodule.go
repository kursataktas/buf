package bufmodule

import (
	"context"
	"io"

	"github.com/bufbuild/buf/private/pkg/storage"
)

// What is a Module?
// It definitely includes its .proto files.
// It includes its list of resolved dependencies ie buf.lock
// It includes its LICENSE - this is part of the module, a change in the license is a change to the module
// It includes its README - documentation is either in comments or in readme, both are part of the module
// It does not include breaking or lint config - this doesn't comprise module data, this is just used to operate on the module itself in certain situations (CLI)
// It does not include excludes - these are already excluded when building the module
// It does not include generation config obviously
// Basically, if you find yourself saying "X operates on the module", it's probably not part of the module itself (ie lint config, breaking config, excludes)
//
// What is a Workspace?
// It includes all of its modules
// It includes its list of resolved dependencies?
// What about LICENSE and README? A workspace isn't licensed, a Module is. A workspace isn't documented, a Module is.
//
// What about config?
// Only the CLI cares. This isn't part of Module or Workspace at all.
//
// What about --path/--exclude-path?
// Only the CLI cares.
//
// What about declared dependencies?
// Only the CLI cares.
//
// What about excludes?
// Only the CLI cares.
//
// What does this lead to?
// buf.yaml should not be part of the digest.
// Our Workspace and Module types should fall as above.
// We should figure out how to move Config to something like CommandMeta below, similar to ModuleConfig/ImageConfig/ModuleConfigSet. This should be a type outside of bufwire.
// We should figure out a nice way to deal with TargetFileInfos at a level outside of Module and Workspace.

type ModuleFullName interface {
	Remote() string
	Owner() string
	Name() string

	isModuleFullName()
}

type ModuleReference interface {
	ModuleFullName() ModuleFullName
	Reference() string

	isModuleReference()
}

type ModulePin interface {
	ModuleFullName() ModuleFullName
	Commit() string
	Digest() string

	isModulePin()
}

type ModuleSet interface {
	GetModule(moduleID string) (Module, error)
	Modules() []Module
	Deps() []ModulePin
	// Needed for v1 vs v2
	IsDepsOnModule() bool

	isModuleSet()
}

type ProtoFileInfo interface {
	storage.ObjectInfo

	Module() Module

	isProtoFileInfo()
}

type ProtoFile interface {
	ProtoFileInfo
	io.ReadCloser

	isProtoFile()
}

type Module interface {
	ProtoFileInfos(context.Context) ([]ProtoFileInfo, error)
	GetProtoFile(ctx context.Context, path string) (ProtoFile, error)
	DocContent() string
	DocPath() string
	LicenseContent() string
	ModuleSet() ModuleSet
	// Returns all deps in ModuleSet on v2
	Deps() []ModulePin

	ID() string
	FullName() ModuleFullName
	Commit() string

	isModule()
}
