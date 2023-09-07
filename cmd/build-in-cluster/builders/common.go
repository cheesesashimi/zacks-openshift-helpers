package builders

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"

	"k8s.io/klog"
)

func makeBinaries(repoRoot string) error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	cmd := exec.Command("make", "binaries")
	cmd.Env = append(cmd.Env, toEnvVars(map[string]string{
		"HOME":   u.HomeDir,
		"GOARCH": "amd64",
		"GOOS":   "linux",
	})...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	klog.Infof("Running %s", cmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unable to build binaries: %w", err)
	}

	return nil
}

func toEnvVars(in map[string]string) []string {
	out := []string{}

	for k, v := range in {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}

	return out
}
