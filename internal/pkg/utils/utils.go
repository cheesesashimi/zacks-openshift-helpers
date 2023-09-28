package utils

import (
	"fmt"
	"os"
	"os/exec"

	aggerrs "k8s.io/apimachinery/pkg/util/errors"
)

func CheckForBinaries(bins []string) error {
	errs := []error{}

	for _, bin := range bins {
		if _, err := exec.LookPath(bin); err != nil {
			errs = append(errs, fmt.Errorf("binary %q not found: %w", bin, err))
		}
	}

	return aggerrs.NewAggregate(errs)
}

func ToEnvVars(in map[string]string) []string {
	out := os.Environ()

	for key, val := range in {
		envVar := fmt.Sprintf("%s=%s", key, val)
		out = append(out, envVar)
	}

	return out
}
