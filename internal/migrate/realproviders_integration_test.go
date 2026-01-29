//go:build integration

package migrate

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type realProviderCase struct {
	Name            string
	Repo            string
	RegistryAddress string
	ProviderName    string
}

func TestRealProviders(t *testing.T) {
	t.Parallel()

	cases := realProviderCases()
	if len(cases) == 0 {
		t.Skip("no real providers configured")
	}

	for _, providerCase := range cases {
		providerCase := providerCase
		t.Run(providerCase.Name, func(t *testing.T) {
			t.Parallel()

			repoDir := cloneRepo(t, providerCase.Repo)
			opts := Options{
				Path:            repoDir,
				RegistryAddress: providerCase.RegistryAddress,
				ProviderName:    providerCase.ProviderName,
			}

			if _, err := Check(opts); err != nil {
				t.Fatalf("check failed: %v", err)
			}

			if _, err := Migrate(opts); err != nil {
				t.Fatalf("migrate failed: %v", err)
			}

			runGoTestCompile(t, repoDir)
		})
	}
}

func realProviderCases() []realProviderCase {
	if raw := strings.TrimSpace(os.Getenv("TPM_REAL_PROVIDERS")); raw != "" {
		return parseProviderCases(raw)
	}

	return []realProviderCase{
		{
			Name: "ansible",
			Repo: "https://github.com/ansible/terraform-provider-ansible.git",
		},
		{
			Name: "digitalocean",
			Repo: "https://github.com/digitalocean/terraform-provider-digitalocean.git",
		},
		{
			Name: "postgresql",
			Repo: "https://github.com/cyrilgdn/terraform-provider-postgresql.git",
		},
	}
}

func parseProviderCases(raw string) []realProviderCase {
	parts := strings.Split(raw, ",")
	cases := make([]realProviderCase, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		fields := strings.Split(part, "|")
		entry := realProviderCase{
			Name: strings.TrimSpace(fields[0]),
		}
		if entry.Name == "" {
			continue
		}
		if len(fields) > 1 {
			entry.Repo = strings.TrimSpace(fields[1])
		}
		if len(fields) > 2 {
			entry.RegistryAddress = strings.TrimSpace(fields[2])
		}
		if len(fields) > 3 {
			entry.ProviderName = strings.TrimSpace(fields[3])
		}
		if entry.Repo == "" {
			entry.Repo = entry.Name
		}
		cases = append(cases, entry)
	}
	return cases
}

func cloneRepo(t *testing.T, repo string) string {
	t.Helper()

	tmp := t.TempDir()
	target := filepath.Join(tmp, "repo")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", repo, target)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out.String())
	}

	return target
}

func runGoTestCompile(t *testing.T, dir string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "./...", "-run", "TestDoesNotExist", "-mod=mod")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go test compile failed: %v", err)
	}
}
