package migrate

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

const (
	frameworkModule = "github.com/hashicorp/terraform-plugin-framework"
	muxModule       = "github.com/hashicorp/terraform-plugin-mux"
	pluginGoModule  = "github.com/hashicorp/terraform-plugin-go"

	defaultFrameworkVersion = "v1.6.0"
	defaultMuxVersion       = "v0.5.0"
	defaultPluginGoVersion  = "v0.26.0"
)

func ensureModuleDeps(moduleRoot string) error {
	modPath := filepath.Join(moduleRoot, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return err
	}

	file, err := modfile.Parse(modPath, data, nil)
	if err != nil {
		return err
	}

	changed := false
	changed = addRequire(file, frameworkModule, defaultFrameworkVersion) || changed
	changed = addRequire(file, muxModule, defaultMuxVersion) || changed
	changed = addRequire(file, pluginGoModule, defaultPluginGoVersion) || changed

	if !changed {
		return nil
	}

	formatted, err := file.Format()
	if err != nil {
		return fmt.Errorf("format go.mod: %w", err)
	}
	return os.WriteFile(modPath, formatted, 0o644)
}

func addRequire(file *modfile.File, path, version string) bool {
	for _, req := range file.Require {
		if req.Mod.Path == path {
			return false
		}
	}
	if err := file.AddRequire(path, version); err != nil {
		return false
	}
	return true
}
