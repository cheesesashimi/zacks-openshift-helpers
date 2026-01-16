package repo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

const targetModule = "github.com/openshift/machine-config-operator"

type mcoRepo struct {
	path string
}

func NewMCORepo() (*mcoRepo, error) {
	root, err := validateMCORepoRoot()
	if err != nil {
		return nil, err
	}

	return &mcoRepo{path: root}, nil
}

func (m *mcoRepo) IsClean() (bool, error) {
	// --porcelain v1 provides a stable, easy-to-parse output.
	// If the output is empty, the working directory is clean.
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=no")
	cmd.Dir = m.path

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return false, fmt.Errorf("failed to run git status: %w", err)
	}

	// Trim whitespace and check if the string length is zero
	return len(strings.TrimSpace(out.String())) == 0, nil
}

func (m *mcoRepo) Checkout(ref string) error {
	cmd := exec.Command("git", "checkout", ref)
	cmd.Dir = m.path

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not check out ref %s: %w", ref, err)
	}

	return nil
}

func validateMCORepoRoot() (string, error) {
	// 1. Get the Git root directory
	// 'git rev-parse --show-toplevel' is the standard way to find the repo root
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("current directory is not part of a git repository")
	}

	repoRoot := strings.TrimSpace(string(out))

	// 2. Read the go.mod file
	goModPath := filepath.Join(repoRoot, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no go.mod found at repository root: %s", repoRoot)
		}
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	// 3. Parse go.mod using the official modfile library
	// The third argument (nil) is for an optional version fix-up function
	f, err := modfile.Parse(goModPath, content, nil)
	if err != nil {
		return "", fmt.Errorf("failed to parse go.mod: %w", err)
	}

	// 4. Validate the module path
	if f.Module == nil {
		return "", fmt.Errorf("go.mod does not contain a module declaration")
	}

	if f.Module.Mod.Path != targetModule {
		return "", fmt.Errorf("module mismatch: expected %q, found %q", targetModule, f.Module.Mod.Path)
	}

	return repoRoot, nil
}
