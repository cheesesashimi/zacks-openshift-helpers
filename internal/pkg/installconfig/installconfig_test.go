package installconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateInstallConfig(t *testing.T) {
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
		{
			opts: Opts{
				Arch:              amd64,
				Kind:              ocp,
				EnableTechPreview: true,
			},
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

			parsedInstallConfig, err := ParseInstallConfig(out)
			assert.NoError(t, err)

			assert.Contains(t, string(out), testCase.opts.Prefix)
			assert.Contains(t, string(out), pullSecret)
			assert.Contains(t, string(out), sshKey)

			assert.Equal(t, parsedInstallConfig.SSHKey, sshKey)
			assert.Equal(t, parsedInstallConfig.PullSecret, pullSecret)

			clusterName := ""
			if testCase.opts.Variant == singleNode {
				clusterName = fmt.Sprintf("cluster-name-prefix-%s-%s-sno", testCase.opts.Kind, testCase.opts.Arch)
				assert.Contains(t, string(out), "replicas: 1")
				assert.Contains(t, string(out), "replicas: 0")

				assert.Equal(t, parsedInstallConfig.Name, clusterName)
			} else {
				clusterName = fmt.Sprintf("cluster-name-prefix-%s-%s", testCase.opts.Kind, testCase.opts.Arch)
				assert.Contains(t, string(out), "replicas: 3")
			}

			assert.Contains(t, string(out), clusterName)
			assert.Equal(t, parsedInstallConfig.Name, clusterName)

			if testCase.opts.Arch == aarch64 {
				assert.Contains(t, string(out), "aarch64")
				assert.Contains(t, string(out), "architecture: arm64")
				assert.Equal(t, parsedInstallConfig.Architecture, "arm64")
			} else if testCase.opts.Arch != multi {
				assert.Contains(t, string(out), "architecture: "+testCase.opts.Arch)
				assert.Equal(t, parsedInstallConfig.Architecture, testCase.opts.Arch)
			}

			techPreviewFeatureSet := "featureSet: TechPreviewNoUpgrade"
			if testCase.opts.EnableTechPreview {
				assert.Equal(t, parsedInstallConfig.FeatureSet, "TechPreviewNoUpgrade")
				assert.Contains(t, string(out), techPreviewFeatureSet)
			} else {
				assert.NotContains(t, string(out), techPreviewFeatureSet)
				assert.Equal(t, parsedInstallConfig.FeatureSet, "")
			}
		})
	}
}

func TestGetInstallConfigFromPath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		clusterName         string
		cfgSSHKey           string
		cfgPullSecret       string
		fileSSHKey          string
		filePullSecret      string
		expectedSSHKey      string
		expectedPullSecret  string
		expectedClusterName string
		errExpected         bool
	}{
		{
			name:               "Injects pull secret and SSH key from provided paths",
			cfgSSHKey:          "",
			cfgPullSecret:      "",
			fileSSHKey:         "ssh-key-abc",
			filePullSecret:     "pull-secret-123",
			expectedSSHKey:     "ssh-key-abc",
			expectedPullSecret: "pull-secret-123",
		},
		{
			name:               "Overrides SSH key from path value",
			cfgSSHKey:          "ssh-key-123",
			cfgPullSecret:      "pull-secret-123",
			fileSSHKey:         "ssh-key-abc",
			expectedSSHKey:     "ssh-key-abc",
			expectedPullSecret: "pull-secret-123",
		},
		{
			name:               "Overrides pull secret from path value",
			cfgSSHKey:          "ssh-key-123",
			cfgPullSecret:      "pull-secret-123",
			filePullSecret:     "pull-secret-abc",
			expectedSSHKey:     "ssh-key-123",
			expectedPullSecret: "pull-secret-abc",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()

			paths := Paths{
				InstallConfigPath: filepath.Join(tempDir, "install-config.yaml"),
			}

			if testCase.fileSSHKey != "" {
				paths.SSHKeyPath = filepath.Join(tempDir, "ssh-key")
				require.NoError(t, os.WriteFile(paths.SSHKeyPath, []byte(testCase.fileSSHKey), 0o755))
			}

			if testCase.filePullSecret != "" {
				paths.PullSecretPath = filepath.Join(tempDir, "pull-secret")
				require.NoError(t, os.WriteFile(paths.PullSecretPath, []byte(testCase.filePullSecret), 0o755))
			}

			installCfg := string(baseInstallConfigAMD64)
			if testCase.cfgSSHKey != "" {
				installCfg = strings.ReplaceAll(installCfg, "sshKey: ''", fmt.Sprintf("sshKey: %q", testCase.cfgSSHKey))
			}

			if testCase.cfgPullSecret != "" {
				installCfg = strings.ReplaceAll(installCfg, "pullSecret: ''", fmt.Sprintf("pullSecret: %q", testCase.cfgPullSecret))
			}

			opts := Opts{
				Prefix: "cluster",
				Kind:   "ocp",
				Paths:  paths,
			}

			writeConfigAndParse := func() *ParsedInstallConfig {
				require.NoError(t, os.WriteFile(paths.InstallConfigPath, []byte(installCfg), 0o755))

				finalCfg, err := GetInstallConfig(opts)
				assert.NoError(t, err)

				parsed, err := ParseInstallConfig(finalCfg)
				assert.NoError(t, err)

				return parsed
			}

			parsed := writeConfigAndParse()
			assert.Equal(t, testCase.expectedSSHKey, parsed.SSHKey)
			assert.Equal(t, testCase.expectedPullSecret, parsed.PullSecret)
			assert.Equal(t, "cluster-ocp-amd64", parsed.Name)
			assert.NotEqual(t, parsed.FeatureSet, "TechPreviewNoUpgrade")

			embeddedClusterName := "test-cluster"
			installCfg = strings.ReplaceAll(installCfg, "name: \"\"", fmt.Sprintf("name: %q", embeddedClusterName))
			parsed = writeConfigAndParse()
			assert.Equal(t, embeddedClusterName, parsed.Name)

			opts.EnableTechPreview = true
			parsed = writeConfigAndParse()
			assert.Equal(t, "TechPreviewNoUpgrade", parsed.FeatureSet)
		})
	}
}
