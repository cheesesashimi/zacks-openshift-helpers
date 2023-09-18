package repo

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	giturl "github.com/kubescape/go-git-url"
	aggerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

type BuildMode string

const (
	BuildModeNormal  BuildMode = "normal"
	BuildModeFast    BuildMode = "fast"
	BuildModeCluster BuildMode = "cluster"
)

func IsValidBuildMode(mode BuildMode) (bool, error) {
	modes := GetBuildModes()
	if modes.Has(mode) {
		return true, nil
	}

	return false, fmt.Errorf("invalid build mode %s, valid build modes: %s", mode, sets.List(modes))
}

func GetBuildModes() sets.Set[BuildMode] {
	return sets.New[BuildMode](BuildModeNormal, BuildModeFast, BuildModeCluster)
}

const (
	fastBuildDockerfileName string = "Dockerfile.fast-build"
	fastBuildMakefileName   string = "Makefile.fast-build"
)

const (
	clusterBuildDockerfileName string = "Dockerfile.cluster"
)

//go:embed Dockerfile.fast-build
var fastBuildDockerfile []byte

//go:embed Makefile.fast-build
var fastBuildMakefile []byte

//go:embed Dockerfile.cluster
var clusterBuildDockerfile []byte

type overrideableRepoFile struct {
	path     string
	contents []byte
	embedded bool
}

func writeIfEmbedded(f overrideableRepoFile) error {
	if f.embedded {
		klog.Infof("Writing %s", f.path)
		return os.WriteFile(f.path, f.contents, 0o755)
	}

	klog.V(4).Infof("File %s already exists, no-op", f.path)
	return nil
}

func deleteIfEmbedded(f overrideableRepoFile) error {
	if f.embedded {
		klog.Infof("Removing %s", f.path)
		return os.Remove(f.path)
	}

	klog.V(4).Infof("File %s is not overridden, no-op", f.path)
	return nil
}

type gitInfo struct {
	branchName string
	remoteURL  string
	repoRoot   string
}

type MCORepo struct {
	repoRoot    string
	buildMode   BuildMode
	dockerfiles map[BuildMode]overrideableRepoFile
	makefiles   map[BuildMode]overrideableRepoFile
	gitInfo
}

func NewMCORepo(repoRoot string, mode BuildMode) (*MCORepo, error) {
	if _, err := os.Stat(repoRoot); err != nil {
		return nil, fmt.Errorf("could not load MCO repo root: %w", err)
	}

	if _, err := IsValidBuildMode(mode); err != nil {
		return nil, err
	}

	dockerfiles, err := populateOverrideableRepoFileMap(map[BuildMode]overrideableRepoFile{
		BuildModeNormal: {
			path: filepath.Join(repoRoot, "Dockerfile"),
		},
		BuildModeFast: {
			path:     filepath.Join(repoRoot, fastBuildDockerfileName),
			contents: fastBuildDockerfile,
			embedded: true,
		},
		BuildModeCluster: {
			path:     filepath.Join(repoRoot, clusterBuildDockerfileName),
			contents: clusterBuildDockerfile,
			embedded: true,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("could not load Dockerfile: %w", err)
	}

	makefiles, err := populateOverrideableRepoFileMap(map[BuildMode]overrideableRepoFile{
		BuildModeNormal: {
			path: filepath.Join(repoRoot, "Makefile"),
		},
		BuildModeFast: {
			path:     filepath.Join(repoRoot, fastBuildMakefileName),
			contents: fastBuildMakefile,
			embedded: true,
		},
		BuildModeCluster: {
			path: filepath.Join(repoRoot, "Makefile"),
		},
	})

	if err != nil {
		return nil, fmt.Errorf("could not load Makefile: %w", err)
	}

	gi, err := getGitInfo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("could not load Git info for repo root %s: %w", repoRoot, err)
	}

	r := &MCORepo{
		repoRoot:    repoRoot,
		buildMode:   mode,
		gitInfo:     *gi,
		dockerfiles: dockerfiles,
		makefiles:   makefiles,
	}

	return r, nil
}

func (m *MCORepo) RemoteFork() string {
	return m.gitInfo.remoteURL
}

func (m *MCORepo) Branch() string {
	return m.gitInfo.branchName
}

func (m *MCORepo) Root() string {
	return m.repoRoot
}

func (m *MCORepo) DockerfileContents() []byte {
	return m.dockerfiles[m.buildMode].contents
}

func (m *MCORepo) MakefileContents() []byte {
	return m.makefiles[m.buildMode].contents
}

func (m *MCORepo) DockerfilePath() string {
	return m.dockerfiles[m.buildMode].path
}

func (m *MCORepo) MakefilePath() string {
	return m.makefiles[m.buildMode].path
}

func (m *MCORepo) SetupForBuild() error {
	return aggerrs.NewAggregate([]error{
		writeIfEmbedded(m.dockerfiles[m.buildMode]),
		writeIfEmbedded(m.makefiles[m.buildMode]),
	})
}

func (m *MCORepo) TeardownFromBuild() error {
	return aggerrs.NewAggregate([]error{
		deleteIfEmbedded(m.dockerfiles[m.buildMode]),
		deleteIfEmbedded(m.makefiles[m.buildMode]),
	})
}

func getGitInfo(repoRoot string) (*gitInfo, error) {
	if _, err := os.Stat(repoRoot); err != nil {
		return nil, err
	}

	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return nil, err
	}

	head, err := repo.Head()
	if err != nil {
		return nil, err
	}

	branches, err := repo.Branches()
	if err != nil {
		return nil, err
	}

	branchName := ""
	branches.ForEach(func(b *plumbing.Reference) error {
		if b.Hash() == head.Hash() {
			branchName = b.Name().Short()
			return storer.ErrStop
		}

		return nil
	})

	if branchName == "" {
		return nil, fmt.Errorf("no branch found")
	}

	remotes, err := repo.Remotes()
	if err != nil {
		return nil, err
	}

	remoteURL, err := getURLFromRemotes(remotes)
	if err != nil {
		return nil, err
	}

	gi := &gitInfo{
		branchName: branchName,
		remoteURL:  remoteURL,
		repoRoot:   repoRoot,
	}

	return gi, nil
}

func getURLFromRemotes(remotes []*git.Remote) (string, error) {
	for _, remote := range remotes {
		for _, u := range remote.Config().URLs {
			if !strings.Contains(u, "openshift/machine-config-operator") {
				gitURL, err := giturl.NewGitURL(u)
				if err != nil {
					return "", err
				}

				return fmt.Sprintf("%s.git", gitURL.GetURL().String()), nil
			}
		}
	}

	return "", fmt.Errorf("no remote found")
}

func populateOverrideableRepoFileMap(in map[BuildMode]overrideableRepoFile) (map[BuildMode]overrideableRepoFile, error) {
	out := map[BuildMode]overrideableRepoFile{}

	for key, repoFile := range in {
		var contents []byte
		if repoFile.embedded {
			contents = repoFile.contents
		} else {
			fileContents, err := os.ReadFile(repoFile.path)
			if err != nil {
				return nil, err
			}
			contents = fileContents
		}

		out[key] = overrideableRepoFile{
			contents: contents,
			path:     repoFile.path,
			embedded: repoFile.embedded,
		}
	}

	return out, nil
}
