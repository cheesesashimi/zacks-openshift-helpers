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
				Arch: "amd64",
				Kind: "ocp",
			},
		},
		{
			opts: Opts{
				Arch: "arm64",
				Kind: "ocp",
			},
		},
		{
			opts: Opts{
				Arch: "multi",
				Kind: "ocp",
			},
		},
		{
			opts: Opts{
				Arch: "amd64",
				Kind: "okd",
			},
		},
		{
			opts: Opts{
				Arch: "amd64",
				Kind: "okd-scos",
			},
		},
		{
			opts: Opts{
				Arch: "arm64",
				Kind: "okd",
			},
			errExpected: true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(fmt.Sprintf("%s-%s", testCase.opts.Arch, testCase.opts.Kind), func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()

			testCase.opts.Username = "user"
			testCase.opts.PullSecretPath = filepath.Join(tempDir, "pull-secret-path")
			testCase.opts.SSHKeyPath = filepath.Join(tempDir, "ssh-key-path")

			sshKey := "im-an-sshkey"
			pullSecret := "im-a-pullsecret"

			require.NoError(t, os.WriteFile(testCase.opts.SSHKeyPath, []byte(sshKey), 0755))
			require.NoError(t, os.WriteFile(testCase.opts.PullSecretPath, []byte(pullSecret), 0755))

			out, err := GetInstallConfig(testCase.opts)
			if testCase.errExpected {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			assert.Contains(t, string(out), testCase.opts.Username)
			assert.Contains(t, string(out), pullSecret)
			assert.Contains(t, string(out), sshKey)
		})
	}
}
