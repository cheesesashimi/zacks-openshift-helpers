package installconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderInstallConfig(t *testing.T) {
	testCases := []struct {
		opts        Opts
		errExpected bool
	}{
		{
			opts: Opts{
				Arch: amd64,
				Kind: ocp,
			},
		},
		{
			opts: Opts{
				Arch: arm64,
				Kind: ocp,
			},
		},
		{
			opts: Opts{
				Arch: aarch64,
				Kind: ocp,
			},
		},
		{
			opts: Opts{
				Arch: multi,
				Kind: ocp,
			},
		},
		{
			opts: Opts{
				Arch: amd64,
				Kind: okd,
			},
		},
		{
			opts: Opts{
				Arch: amd64,
				Kind: okdSCOS,
			},
		},
		{
			opts: Opts{
				Arch: arm64,
				Kind: okd,
			},
			errExpected: true,
		},
		{
			opts: Opts{
				Arch:    amd64,
				Kind:    ocp,
				Variant: "unknown-variant",
			},
			errExpected: true,
		},
		{
			opts: Opts{
				Arch:    amd64,
				Kind:    ocp,
				Variant: singleNode,
			},
		},
		{
			opts: Opts{
				Arch:    arm64,
				Kind:    ocp,
				Variant: singleNode,
			},
		},
		{
			opts: Opts{
				Arch:    multi,
				Kind:    ocp,
				Variant: singleNode,
			},
			errExpected: true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		var testName string
		if testCase.opts.Variant != "" {
			testName = fmt.Sprintf("%s-%s-%s", testCase.opts.Arch, testCase.opts.Kind, testCase.opts.Variant)
		} else {
			testName = fmt.Sprintf("%s-%s", testCase.opts.Arch, testCase.opts.Kind)
		}

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()

			testCase.opts.Prefix = "cluster-name-prefix"
			testCase.opts.PullSecretPath = filepath.Join(tempDir, "pull-secret-path")
			testCase.opts.SSHKeyPath = filepath.Join(tempDir, "ssh-key-path")

			sshKey := "im-an-sshkey"
			pullSecret := "im-a-pullsecret"

			require.NoError(t, os.WriteFile(testCase.opts.SSHKeyPath, []byte(sshKey), 0755))
			require.NoError(t, os.WriteFile(testCase.opts.PullSecretPath, []byte(pullSecret), 0755))

			out, err := GetInstallConfig(testCase.opts)
			if testCase.errExpected {
				assert.Error(t, err)
				t.Logf("error output: %s", err)
				return
			}

			assert.NoError(t, err)

			assert.Contains(t, string(out), testCase.opts.Prefix)
			assert.Contains(t, string(out), pullSecret)
			assert.Contains(t, string(out), sshKey)

			if testCase.opts.Variant == singleNode {
				assert.Contains(t, string(out), fmt.Sprintf("cluster-name-prefix-%s-%s-sno", testCase.opts.Kind, testCase.opts.Arch))
				assert.Contains(t, string(out), "replicas: 1")
				assert.Contains(t, string(out), "replicas: 0")
			} else {
				assert.Contains(t, string(out), fmt.Sprintf("cluster-name-prefix-%s-%s", testCase.opts.Kind, testCase.opts.Arch))
				assert.Contains(t, string(out), "replicas: 3")
			}

			if testCase.opts.Arch == aarch64 {
				assert.Contains(t, string(out), "aarch64")
				assert.Contains(t, string(out), "architecture: arm64")
			} else if testCase.opts.Arch != multi {
				assert.Contains(t, string(out), "architecture: "+testCase.opts.Arch)
			}
		})
	}
}
