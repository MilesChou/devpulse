package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "repos.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInit_RegistersRepos(t *testing.T) {
	setEnv(t)
	p := writeYAML(t, "repos:\n  - octocat/hello-world\n  - octocat/Spoon-Knife\n")

	out, err := runCmd(t, "init", p)
	if err != nil {
		t.Fatalf("init: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "octocat/hello-world") {
		t.Errorf("missing hello-world in output: %s", out)
	}
	if !strings.Contains(out, "octocat/Spoon-Knife") {
		t.Errorf("missing Spoon-Knife in output: %s", out)
	}
	if !strings.Contains(out, "2 repo(s) registered") {
		t.Errorf("missing summary in output: %s", out)
	}
}

func TestInit_EmptyRepoList(t *testing.T) {
	setEnv(t)
	p := writeYAML(t, "repos: []\n")

	_, err := runCmd(t, "init", p)
	if err == nil {
		t.Fatal("expected error for empty repo list")
	}
}

func TestInit_InvalidRepoName(t *testing.T) {
	setEnv(t)
	p := writeYAML(t, "repos:\n  - not-a-slug\n")

	_, err := runCmd(t, "init", p)
	if err == nil {
		t.Fatal("expected error for invalid repo name")
	}
}

func TestInit_FileNotFound(t *testing.T) {
	setEnv(t)
	_, err := runCmd(t, "init", "/tmp/does-not-exist.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestInit_InvalidYAML(t *testing.T) {
	setEnv(t)
	p := writeYAML(t, ":\n  - :\n")

	_, err := runCmd(t, "init", p)
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}
