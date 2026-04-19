package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This test is a lightweight guardrail for v0.2 "one-click startup".
// We don't run Docker in CI here; we only assert the config files exist and
// contain the expected essentials so contributors don't accidentally break them.
func TestDockerComposeFiles_ExistAndContainBasics(t *testing.T) {
	t.Parallel()

	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	yml := string(b)
	for _, want := range []string{
		"services:",
		"lobster-world-core:",
		"ports:",
		"- \"8080:8080\"",
		"PORT=8080",
	} {
		if !strings.Contains(yml, want) {
			t.Fatalf("docker-compose.yml missing %q", want)
		}
	}

	df, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	dockerfile := string(df)
	for _, want := range []string{
		"FROM",
		"CMD",
		"EXPOSE 8080",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("Dockerfile missing %q", want)
		}
	}
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
