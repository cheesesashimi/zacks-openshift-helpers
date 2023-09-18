package repo

import (
	"bytes"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed test-repo.tar.gz
var testRepo []byte

func TestRepo(t *testing.T) {
	t.Parallel()

	testRepoDockerfileName := "Dockerfile"
	testRepoMakefileName := "Makefile"

	testRepoDockerfileContents := []byte("FROM scratch\n")
	testRepoMakefileContents := []byte("hello\n")

	testCases := []struct {
		name                       string
		expectedDockerfileName     string
		expectedMakefileName       string
		expectedDockerfileContents []byte
		expectedMakefileContents   []byte
		buildMode                  BuildMode
	}{
		{
			name:                       "Normal build mode",
			expectedDockerfileName:     testRepoDockerfileName,
			expectedMakefileName:       testRepoMakefileName,
			expectedDockerfileContents: testRepoDockerfileContents,
			expectedMakefileContents:   testRepoMakefileContents,
			buildMode:                  BuildModeNormal,
		},
		{
			name:                       "Fast build mode",
			expectedDockerfileName:     fastBuildDockerfileName,
			expectedMakefileName:       fastBuildMakefileName,
			expectedDockerfileContents: fastBuildDockerfile,
			expectedMakefileContents:   fastBuildMakefile,
			buildMode:                  BuildModeFast,
		},
		{
			name:                       "Cluster build mode",
			expectedDockerfileName:     clusterBuildDockerfileName,
			expectedMakefileName:       testRepoMakefileName,
			expectedDockerfileContents: clusterBuildDockerfile,
			expectedMakefileContents:   testRepoMakefileContents,
			buildMode:                  BuildModeCluster,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()

			cmd := exec.Command("tar", "-zxf", "-")
			cmd.Dir = tmpDir
			cmd.Stdin = bytes.NewBuffer(testRepo)
			require.NoError(t, cmd.Run())

			testRepoRoot := filepath.Join(tmpDir, "test-repo")

			assertFileContentsEqual(t, filepath.Join(testRepoRoot, testRepoDockerfileName), testRepoDockerfileContents)
			assertFileContentsEqual(t, filepath.Join(testRepoRoot, testRepoMakefileName), testRepoMakefileContents)

			r, err := NewMCORepo(testRepoRoot, testCase.buildMode)
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

			assertFileContentsEqual(t, filepath.Join(testRepoRoot, testRepoDockerfileName), testRepoDockerfileContents)
			assertFileContentsEqual(t, filepath.Join(testRepoRoot, testRepoMakefileName), testRepoMakefileContents)

			assert.NoError(t, r.TeardownFromBuild())
			if testCase.buildMode == BuildModeFast {
				assert.NoFileExists(t, filepath.Join(testRepoRoot, testCase.expectedDockerfileName))
				assert.NoFileExists(t, filepath.Join(testRepoRoot, testCase.expectedMakefileName))
			}

			if testCase.buildMode == BuildModeCluster {
				assert.NoFileExists(t, filepath.Join(testRepoRoot, testCase.expectedDockerfileName))
			}

			assertFileContentsEqual(t, filepath.Join(testRepoRoot, testRepoDockerfileName), testRepoDockerfileContents)
			assertFileContentsEqual(t, filepath.Join(testRepoRoot, testRepoMakefileName), testRepoMakefileContents)
		})
	}
}

func assertFileContentsEqual(t *testing.T, path string, expectedContents []byte) {
	t.Helper()

	assert.FileExists(t, path)

	contents, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, expectedContents, contents)
}
