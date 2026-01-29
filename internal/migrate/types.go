package migrate

import "fmt"

type Options struct {
	Path            string
	RegistryAddress string
	ProviderName    string
	DryRun          bool
}

type Report struct {
	ModuleRoot      string
	MainFile        string
	FrameworkFile   string
	ProviderName    string
	RegistryAddress string
	Attributes      int
	Notes           []string
}

func (r Report) Summary() string {
	msg := fmt.Sprintf("module=%s provider=%s registry=%s attrs=%d", r.ModuleRoot, r.ProviderName, r.RegistryAddress, r.Attributes)
	if r.MainFile != "" {
		msg += fmt.Sprintf(" main=%s", r.MainFile)
	}
	if r.FrameworkFile != "" {
		msg += fmt.Sprintf(" framework=%s", r.FrameworkFile)
	}
	if len(r.Notes) > 0 {
		msg += fmt.Sprintf(" notes=%d", len(r.Notes))
	}
	return msg
}
