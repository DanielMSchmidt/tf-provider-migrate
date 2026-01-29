package migrate

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var ErrDryRun = errors.New("dry run")

func Check(opts Options) (Report, error) {
	moduleRoot, err := findModuleRoot(opts.Path)
	if err != nil {
		return Report{}, err
	}

	providerInfo, err := findProviderInfo(moduleRoot)
	if err != nil {
		return Report{}, err
	}

	mainFile, mainInfo, err := findMainInfo(moduleRoot)
	if err != nil {
		return Report{}, err
	}

	providerName, registryAddress, notes, err := deriveNames(opts, moduleRoot, false)
	if err != nil {
		return Report{}, err
	}

	if mainInfo.ProviderImport == "" {
		return Report{}, fmt.Errorf("main package does not reference provider.Provider()")
	}

	report := Report{
		ModuleRoot:      moduleRoot,
		MainFile:        mainFile,
		FrameworkFile:   filepath.Join(moduleRoot, "framework", "provider.go"),
		ProviderName:    providerName,
		RegistryAddress: registryAddress,
		Attributes:      len(providerInfo.Attributes),
		Notes:           notes,
	}

	return report, nil
}

func Migrate(opts Options) (Report, error) {
	moduleRoot, err := findModuleRoot(opts.Path)
	if err != nil {
		return Report{}, err
	}

	providerInfo, err := findProviderInfo(moduleRoot)
	if err != nil {
		return Report{}, err
	}

	mainFile, mainInfo, err := findMainInfo(moduleRoot)
	if err != nil {
		return Report{}, err
	}

	providerName, registryAddress, notes, err := deriveNames(opts, moduleRoot, true)
	if err != nil {
		return Report{}, err
	}

	if mainInfo.ProviderImport == "" {
		return Report{}, fmt.Errorf("main package does not reference provider.Provider()")
	}

	report := Report{
		ModuleRoot:      moduleRoot,
		MainFile:        mainFile,
		FrameworkFile:   filepath.Join(moduleRoot, "framework", "provider.go"),
		ProviderName:    providerName,
		RegistryAddress: registryAddress,
		Attributes:      len(providerInfo.Attributes),
		Notes:           notes,
	}

	frameworkPath := filepath.Join(moduleRoot, "framework", "provider.go")
	frameworkSource, err := renderFrameworkProvider(providerInfo, report.ProviderName)
	if err != nil {
		return Report{}, err
	}

	mainSource, err := renderMuxedMain(mainInfo, report.RegistryAddress)
	if err != nil {
		return Report{}, err
	}

	if opts.DryRun {
		report.Notes = append(report.Notes, "dry-run (no files written)")
		return report, ErrDryRun
	}

	if err := writeFile(frameworkPath, frameworkSource); err != nil {
		return Report{}, err
	}

	if err := writeFile(mainFile, mainSource); err != nil {
		return Report{}, err
	}

	if err := ensureModuleDeps(moduleRoot); err != nil {
		return Report{}, err
	}

	if err := ensureGoSum(moduleRoot); err != nil {
		return Report{}, err
	}

	vendorMode := normalizeVendorMode(opts.VendorMode)
	switch vendorMode {
	case "on":
		if err := syncVendor(moduleRoot, true); err != nil {
			return Report{}, err
		}
	case "auto":
		if err := syncVendor(moduleRoot, false); err != nil {
			return Report{}, err
		}
	case "off":
	default:
		return Report{}, fmt.Errorf("invalid vendor mode: %s", opts.VendorMode)
	}

	return report, nil
}

func normalizeVendorMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return "auto"
	}
	return mode
}
