package repo

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
)

//go:embed test-repo.tar.gz
var testRepo []byte

func TestGetDockerfileContentsFromOCPVersion(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		version        string
		buildMode      BuildMode
		expectedOutput []byte
		errExpected    bool
	}

	buildModes := []BuildMode{
		BuildModeFast,
		BuildModeCluster,
	}

	tc := []testCase{}

	for _, version := range supportedVersions() {
		for _, buildMode := range buildModes {
			tc = append(tc, testCase{
				name:        fmt.Sprintf("%s-%s", version, buildMode),
				version:     version,
				buildMode:   buildMode,
				errExpected: false,
			})
		}
	}

	tc = append(tc, testCase{
		name:        "Unsupported OCP version",
		version:     "4.13",
		errExpected: true,
	})

	for _, testCase := range tc {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			out, err := getDockerfileContentsFromOCPVersion(testCase.version, testCase.buildMode)
			if testCase.errExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, string(out), fmt.Sprintf("registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.20-openshift-%s", testCase.version))
				assert.Contains(t, string(out), fmt.Sprintf("quay.io/zzlotnik/machine-config-operator:nmstate-%s", testCase.version))
			}
		})
	}
}

func TestInferOCPVersionFromDockerfile(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		input           []byte
		expectedVersion string
		errExpected     bool
	}

	versions := []string{
		"4.11",
		"4.12",
		"4.13",
		"4.14",
		"4.15",
	}

	tc := []testCase{}

	for _, version := range versions {
		tc = append(tc, testCase{
			name:            version,
			input:           []byte(fmt.Sprintf("FROM registry.ci.openshift.org/ocp/%s:base AS base", version)),
			expectedVersion: version,
		})
	}

	tc = append(tc, testCase{
		name:        "empty file",
		input:       []byte{},
		errExpected: true,
	})

	tc = append(tc, testCase{
		name:        "incorrect pullspec",
		input:       []byte(`FROM scratch`),
		errExpected: true,
	})

	for _, testCase := range tc {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			version, err := inferOCPVersionFromDockerfile(testCase.input)
			if testCase.errExpected {
				assert.Error(t, err)
				t.Logf("error output: %s", err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVersion, version)
			}
		})
	}
}

func TestRepo(t *testing.T) {
	t.Parallel()

	testRepoMakefileContents := []byte("hello\n")

	type testCase struct {
		name                       string
		expectedDockerfileName     string
		expectedMakefileName       string
		expectedDockerfileContents []byte
		expectedMakefileContents   []byte
		buildMode                  BuildMode
		version                    string
		expectedErr                bool
	}

	getTestCasesForVersion := func(version string) []testCase {
		versions := sets.New[string](supportedVersions()...)
		expectErr := !versions.Has(version)

		clusterDockerfileContents := []byte{}
		fastBuildDockerfileContents := []byte{}
		if !expectErr {
			clusterDockerfileContents = mustLoadEmbeddedFile("Dockerfile.cluster-" + version)
			fastBuildDockerfileContents = mustLoadEmbeddedFile("Dockerfile.fast-build-" + version)
		}

		return []testCase{
			{
				name:                       "Normal build mode",
				expectedDockerfileName:     repoDockerfileName,
				expectedMakefileName:       repoMakefileName,
				expectedDockerfileContents: []byte(fmt.Sprintf("FROM registry.ci.openshift.org/ocp/%s:base AS base", version)),
				expectedMakefileContents:   testRepoMakefileContents,
				buildMode:                  BuildModeNormal,
				version:                    version,
				expectedErr:                false,
			},
			{
				name:                       "Fast build mode",
				expectedDockerfileName:     fastBuildDockerfileName,
				expectedMakefileName:       fastBuildMakefileName,
				expectedDockerfileContents: fastBuildDockerfileContents,
				expectedMakefileContents:   mustLoadEmbeddedFile(fastBuildMakefileName),
				buildMode:                  BuildModeFast,
				version:                    version,
				expectedErr:                expectErr,
			},
			{
				name:                       "Cluster build mode",
				expectedDockerfileName:     clusterBuildDockerfileName,
				expectedMakefileName:       repoMakefileName,
				expectedDockerfileContents: clusterDockerfileContents,
				expectedMakefileContents:   testRepoMakefileContents,
				buildMode:                  BuildModeCluster,
				version:                    version,
				expectedErr:                expectErr,
			},
		}
	}

	testCases := append(getTestCasesForVersion("4.14"), getTestCasesForVersion("4.15")...)
	testCases = append(testCases, getTestCasesForVersion("4.13")...)

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name+"-"+testCase.version, func(t *testing.T) {
			t.Parallel()

			testRepoDockerfileContents := []byte(fmt.Sprintf("FROM registry.ci.openshift.org/ocp/%s:base AS base", testCase.version))

			testRepoRoot := setupTestRepoForTest(t, string(testRepoMakefileContents), string(testRepoDockerfileContents))

			assertFileContentsEqual(t, filepath.Join(testRepoRoot, repoDockerfileName), testRepoDockerfileContents)
			assertFileContentsEqual(t, filepath.Join(testRepoRoot, repoMakefileName), testRepoMakefileContents)

			r, err := NewMCORepo(testRepoRoot, testCase.buildMode)
			if testCase.expectedErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			assert.Equal(t, filepath.Join(testRepoRoot, testCase.expectedDockerfileName), r.DockerfilePath())
			assert.Equal(t, filepath.Join(testRepoRoot, testCase.expectedMakefileName), r.MakefilePath())

			assert.Equal(t, testCase.expectedDockerfileContents, r.DockerfileContents())
			assert.Equal(t, testCase.expectedMakefileContents, r.MakefileContents())

			assert.Equal(t, "test-branch", r.Branch())
			assert.Equal(t, "https://github.com/mypersonalorg/mypersonalrepo.git", r.RemoteFork())

			assert.NoError(t, r.SetupForBuild())
			assertFileContentsEqual(t, filepath.Join(testRepoRoot, testCase.expectedDockerfileName), testCase.expectedDockerfileContents)
			assertFileContentsEqual(t, filepath.Join(testRepoRoot, testCase.expectedMakefileName), testCase.expectedMakefileContents)

			assertFileContentsEqual(t, filepath.Join(testRepoRoot, repoDockerfileName), testRepoDockerfileContents)
			assertFileContentsEqual(t, filepath.Join(testRepoRoot, repoMakefileName), testRepoMakefileContents)

			assert.NoError(t, r.TeardownFromBuild())
			if testCase.buildMode == BuildModeFast {
				assert.NoFileExists(t, filepath.Join(testRepoRoot, testCase.expectedDockerfileName))
				assert.NoFileExists(t, filepath.Join(testRepoRoot, testCase.expectedMakefileName))
			}

			if testCase.buildMode == BuildModeCluster {
				assert.NoFileExists(t, filepath.Join(testRepoRoot, testCase.expectedDockerfileName))
			}

			assertFileContentsEqual(t, filepath.Join(testRepoRoot, repoDockerfileName), testRepoDockerfileContents)
			assertFileContentsEqual(t, filepath.Join(testRepoRoot, repoMakefileName), testRepoMakefileContents)
		})
	}
}

func setupTestRepoForTest(t *testing.T, makefileContents, dockerfileContents string) string {
	tmpDir := t.TempDir()

	cmd := exec.Command("tar", "-zxf", "-")
	cmd.Dir = tmpDir
	cmd.Stdin = bytes.NewBuffer(testRepo)
	require.NoError(t, cmd.Run())

	testRepoRoot := filepath.Join(tmpDir, "test-repo")

	require.NoError(t, os.WriteFile(filepath.Join(testRepoRoot, "Dockerfile"), []byte(dockerfileContents), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(testRepoRoot, "Makefile"), []byte(makefileContents), 0o755))

	return testRepoRoot
}

func assertFileContentsEqual(t *testing.T, path string, expectedContents []byte) {
	t.Helper()

	assert.FileExists(t, path)

	contents, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, string(expectedContents), string(contents))
}

func assertDiskFileContentsEqualEmbedded(t *testing.T, diskFilePath, embeddedFilePath string) {
	t.Helper()

	assert.FileExists(t, diskFilePath)

	diskFileContents, err := os.ReadFile(diskFilePath)
	require.NoError(t, err)

	embeddedFileContents, err := f.ReadFile(embeddedFilePath)
	require.NoError(t, err)

	assert.Equal(t, diskFileContents, embeddedFileContents, "disk file contents: \n%s\n\nembedded file contents:\n%s", string(diskFileContents), string(embeddedFileContents))
}
