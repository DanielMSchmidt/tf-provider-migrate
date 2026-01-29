package migrate

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/mod/modfile"
)

func deriveNames(opts Options, moduleRoot string, strict bool) (string, string, []string, error) {
	var notes []string
	modulePath, err := modulePathFromGoMod(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return "", "", nil, err
	}

	providerName := opts.ProviderName
	if providerName == "" {
		providerName = deriveProviderName(modulePath)
		if providerName == "" {
			if strict {
				return "", "", nil, fmt.Errorf("unable to derive provider name, supply --provider-name")
			}
			notes = append(notes, "provider name not derived; supply --provider-name for migration")
		} else {
			notes = append(notes, "provider name derived from module path")
		}
	}

	registry := opts.RegistryAddress
	if registry == "" {
		registry = deriveRegistryAddress(modulePath)
		if registry == "" {
			if strict {
				return "", "", nil, fmt.Errorf("unable to derive registry address, supply --registry-address")
			}
			notes = append(notes, "registry address not derived; supply --registry-address for migration")
		} else {
			notes = append(notes, "registry address derived from module path")
		}
	}

	return providerName, registry, notes, nil
}

func modulePathFromGoMod(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mod, err := modfile.Parse(path, data, nil)
	if err != nil {
		return "", err
	}
	if mod.Module == nil {
		return "", fmt.Errorf("module path not found in go.mod")
	}
	return mod.Module.Mod.Path, nil
}

func deriveProviderName(modulePath string) string {
	base := moduleBase(modulePath)
	if strings.HasPrefix(base, "terraform-provider-") {
		return strings.TrimPrefix(base, "terraform-provider-")
	}

	if base != "" {
		return base
	}

	return ""
}

func deriveRegistryAddress(modulePath string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) < 3 {
		return ""
	}

	host := parts[0]
	if host == "github.com" && len(parts) >= 3 {
		org := parts[1]
		repo := moduleBase(modulePath)
		if strings.HasPrefix(repo, "terraform-provider-") {
			repo = strings.TrimPrefix(repo, "terraform-provider-")
		}
		return fmt.Sprintf("registry.terraform.io/%s/%s", org, repo)
	}

	return ""
}

var semverMajorRe = regexp.MustCompile(`^v[0-9]+$`)

func moduleBase(modulePath string) string {
	base := path.Base(modulePath)
	if semverMajorRe.MatchString(base) {
		parent := path.Dir(modulePath)
		base = path.Base(parent)
	}
	return base
}
