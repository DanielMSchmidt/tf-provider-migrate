package migrate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/mod/modfile"
)

func TestMigrateFixtures(t *testing.T) {
	t.Parallel()

	fixtures := []string{"mock", "real", "varschema"}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			target := prepareFixture(t, fixture)
			opts := Options{Path: target}

			if _, err := Check(opts); err != nil {
				t.Fatalf("check failed: %v", err)
			}

			if _, err := Migrate(opts); err != nil {
				t.Fatalf("migrate failed: %v", err)
			}

			runGoTest(t, target)
		})
	}
}

func prepareFixture(t *testing.T, name string) string {
	t.Helper()

	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}

	src := filepath.Join(root, "testdata", "providers", name)
	dst := filepath.Join(t.TempDir(), name)

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	stubs := map[string]string{
		frameworkModule: filepath.Join(root, "internal", "stubs", "terraform-plugin-framework"),
		muxModule:       filepath.Join(root, "internal", "stubs", "terraform-plugin-mux"),
		pluginGoModule:  filepath.Join(root, "internal", "stubs", "terraform-plugin-go"),
		"github.com/hashicorp/terraform-plugin-sdk/v2": filepath.Join(root, "internal", "stubs", "terraform-plugin-sdk-v2"),
	}
	if err := addReplaceDirectives(filepath.Join(dst, "go.mod"), stubs); err != nil {
		t.Fatalf("add replaces: %v", err)
	}
	return dst
}

func runGoTest(t *testing.T, dir string) {
	t.Helper()

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go test failed: %v", err)
	}
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func addReplaceDirectives(modPath string, replaces map[string]string) error {
	data, err := os.ReadFile(modPath)
	if err != nil {
		return err
	}
	file, err := modfile.Parse(modPath, data, nil)
	if err != nil {
		return err
	}
	changed := false
	for mod, local := range replaces {
		if err := file.AddReplace(mod, "", local, ""); err == nil {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	formatted, err := file.Format()
	if err != nil {
		return err
	}
	return os.WriteFile(modPath, formatted, 0o644)
}
