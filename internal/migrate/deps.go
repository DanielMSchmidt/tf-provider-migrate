package migrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

const (
	frameworkModule = "github.com/hashicorp/terraform-plugin-framework"
	muxModule       = "github.com/hashicorp/terraform-plugin-mux"
	pluginGoModule  = "github.com/hashicorp/terraform-plugin-go"

	legacyFrameworkVersion = "v1.0.0"
	legacyMuxVersion       = "v0.8.0"
	legacyPluginGoVersion  = "v0.14.2"

	modernFrameworkVersion = "v1.17.0"
	modernMuxVersion       = "v0.21.0"
	modernPluginGoVersion  = "v0.29.0"

	pluginSDKModule = "github.com/hashicorp/terraform-plugin-sdk/v2"
	sdkModernCutoff = "v2.34.0"
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

	deps := selectDeps(file)

	changed := false
	changed = addRequire(file, frameworkModule, deps.frameworkVersion) || changed
	changed = addRequire(file, muxModule, deps.muxVersion) || changed
	changed = addRequire(file, pluginGoModule, deps.pluginGoVersion) || changed

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

type depVersions struct {
	frameworkVersion string
	muxVersion       string
	pluginGoVersion  string
}

func selectDeps(file *modfile.File) depVersions {
	sdkVersion := requireVersion(file, pluginSDKModule)
	if semver.IsValid(sdkVersion) && semver.Compare(sdkVersion, sdkModernCutoff) >= 0 {
		return depVersions{
			frameworkVersion: modernFrameworkVersion,
			muxVersion:       modernMuxVersion,
			pluginGoVersion:  modernPluginGoVersion,
		}
	}

	return depVersions{
		frameworkVersion: legacyFrameworkVersion,
		muxVersion:       legacyMuxVersion,
		pluginGoVersion:  legacyPluginGoVersion,
	}
}

func requireVersion(file *modfile.File, path string) string {
	for _, req := range file.Require {
		if req.Mod.Path == path {
			return req.Mod.Version
		}
	}
	return ""
}

func ensureGoSum(moduleRoot string) error {
	modPath := filepath.Join(moduleRoot, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return err
	}

	file, err := modfile.Parse(modPath, data, nil)
	if err != nil {
		return err
	}

	deps := selectDeps(file)
	versions := map[string]string{
		frameworkModule: versionOrFallback(requireVersion(file, frameworkModule), deps.frameworkVersion),
		muxModule:       versionOrFallback(requireVersion(file, muxModule), deps.muxVersion),
		pluginGoModule:  versionOrFallback(requireVersion(file, pluginGoModule), deps.pluginGoVersion),
	}

	for mod, version := range versions {
		if version == "" {
			continue
		}
		cmd := exec.Command("go", "mod", "download", fmt.Sprintf("%s@%s", mod, version))
		cmd.Dir = moduleRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

func versionOrFallback(version, fallback string) string {
	if version != "" {
		return version
	}
	return fallback
}

// vendoring is intentionally not performed by this tool
