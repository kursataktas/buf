package bufmodule

import (
	"fmt"
	"strings"
)

func parseModuleFullNameComponents(path string) (registry string, owner string, name string, err error) {
	slashSplit := strings.Split(path, "/")
	if len(slashSplit) != 3 {
		return "", "", "", newInvalidModuleFullNameStringError(path)
	}
	registry = strings.TrimSpace(slashSplit[0])
	if registry == "" {
		return "", "", "", newInvalidModuleFullNameStringError(path)
	}
	owner = strings.TrimSpace(slashSplit[1])
	if owner == "" {
		return "", "", "", newInvalidModuleFullNameStringError(path)
	}
	name = strings.TrimSpace(slashSplit[2])
	if name == "" {
		return "", "", "", newInvalidModuleFullNameStringError(path)
	}
	return registry, owner, name, nil
}

func parseModuleRefComponents(path string) (registry string, owner string, name string, ref string, err error) {
	// split by the first "/" to separate the registry and remaining part
	slashSplit := strings.SplitN(path, "/", 2)
	if len(slashSplit) != 2 {
		return "", "", "", "", newInvalidModuleRefStringError(path)
	}
	registry, rest := slashSplit[0], slashSplit[1]
	// split the remaining part by ":" to separate the reference
	colonSplit := strings.Split(rest, ":")
	switch len(colonSplit) {
	case 1:
		// path excluding registry has no colon, no need to handle its ref
	case 2:
		ref = strings.TrimSpace(colonSplit[1])
		if ref == "" {
			return "", "", "", "", newInvalidModuleRefStringError(path)
		}
	default:
		return "", "", "", "", newInvalidModuleRefStringError(path)
	}
	registry, owner, name, err = parseModuleFullNameComponents(registry + "/" + colonSplit[0])
	if err != nil {
		return "", "", "", "", newInvalidModuleRefStringError(path)
	}
	return registry, owner, name, ref, nil
}

func newInvalidModuleFullNameStringError(s string) error {
	return fmt.Errorf("invalid module name %q: must be in the form registry/owner/name", s)
}

func newInvalidModuleRefStringError(s string) error {
	return fmt.Errorf("invalid module reference %q: must be in the form registry/owner/name[:ref]", s)
}
