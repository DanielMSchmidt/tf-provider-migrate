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
	Ref             string
	RegistryAddress string
	ProviderName    string
}

func TestRealProviders(t *testing.T) {
	t.Parallel()

	for _, providerCase := range realProviderCases() {
		providerCase := providerCase
		t.Run(providerCase.Name, func(t *testing.T) {
			t.Parallel()

			repoDir := cloneRepo(t, providerCase.Repo)
			checkoutRef(t, repoDir, providerCase.Ref)
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
	return []realProviderCase{
		{
			Name: "ansible",
			Repo: "https://github.com/ansible/terraform-provider-ansible.git",
			Ref:  "1723917a21a7607d9175f0eee2b3611299987492",
		},
		{
			Name: "digitalocean",
			Repo: "https://github.com/digitalocean/terraform-provider-digitalocean.git",
			Ref:  "ea617eca38d7b3f7e8c40f944dc0d4f38ad6f3cf",
		},
		{
			Name: "postgresql",
			Repo: "https://github.com/cyrilgdn/terraform-provider-postgresql.git",
			Ref:  "450ce85b50c1f47ebdb239c9d05e2f53847389ba",
		},
	}
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

func checkoutRef(t *testing.T, repoDir, ref string) {
	t.Helper()
	if strings.TrimSpace(ref) == "" {
		return
	}

	fetch := exec.Command("git", "fetch", "--depth=1", "origin", ref)
	fetch.Dir = repoDir
	fetch.Stdout = os.Stdout
	fetch.Stderr = os.Stderr
	if err := fetch.Run(); err != nil {
		t.Fatalf("git fetch %s failed: %v", ref, err)
	}

	cmd := exec.Command("git", "checkout", ref)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git checkout %s failed: %v", ref, err)
	}
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
