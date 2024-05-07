package repo

import (
	"embed"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	repoMakefileName   string = "Makefile"
	repoDockerfileName string = "Dockerfile"
)

const (
	fastBuildDockerfileName string = "Dockerfile.fast-build"
	fastBuildMakefileName   string = "Makefile.fast-build"
)

const (
	clusterBuildDockerfileName string = "Dockerfile.cluster"
)

//go:embed *
var f embed.FS

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

func newMCORepoWithNormalBuildMode(repoRoot string) (*MCORepo, error) {
	gi, err := getGitInfo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("could not load Git info for repo root %s: %w", repoRoot, err)
	}

	makefileContents, err := os.ReadFile(filepath.Join(repoRoot, repoMakefileName))
	if err != nil {
		return nil, err
	}

	dockerfileContents, err := os.ReadFile(filepath.Join(repoRoot, repoDockerfileName))
	if err != nil {
		return nil, err
	}

	r := &MCORepo{
		repoRoot:  repoRoot,
		buildMode: BuildModeNormal,
		gitInfo:   *gi,
		makefiles: map[BuildMode]overrideableRepoFile{
			BuildModeNormal: {
				path:     filepath.Join(repoRoot, repoMakefileName),
				contents: makefileContents,
			},
		},
		dockerfiles: map[BuildMode]overrideableRepoFile{
			BuildModeNormal: {
				path:     filepath.Join(repoRoot, repoDockerfileName),
				contents: dockerfileContents,
			},
		},
	}

	return r, nil
}

func NewMCORepo(repoRoot string, mode BuildMode) (*MCORepo, error) {
	if _, err := os.Stat(repoRoot); err != nil {
		return nil, fmt.Errorf("could not load MCO repo root: %w", err)
	}

	if _, err := IsValidBuildMode(mode); err != nil {
		return nil, err
	}

	repoDockerfile, err := os.ReadFile(filepath.Join(repoRoot, "Dockerfile"))
	if err != nil {
		return nil, fmt.Errorf("could not read repo dockerfile: %w", err)
	}

	if mode == BuildModeNormal {
		return newMCORepoWithNormalBuildMode(repoRoot)
	}

	version, err := inferOCPVersionFromDockerfile(repoDockerfile)
	if err != nil {
		return nil, fmt.Errorf("could not infer version from repo dockerfile: %w", err)
	}

	versions := sets.New[string](supportedVersions()...)
	if !versions.Has(version) {
		return nil, fmt.Errorf("version %s is not supported for build mode %q, supported versions %v, use build mode %q", version, mode, supportedVersions(), BuildModeNormal)
	}

	fastBuildDockerfileContents, err := getDockerfileContentsFromOCPVersion(version, BuildModeFast)
	if err != nil {
		return nil, err
	}

	clusterBuildDockerfileContents, err := getDockerfileContentsFromOCPVersion(version, BuildModeCluster)
	if err != nil {
		return nil, err
	}

	dockerfiles, err := populateOverrideableRepoFileMap(map[BuildMode]overrideableRepoFile{
		BuildModeFast: {
			path:     filepath.Join(repoRoot, fastBuildDockerfileName),
			contents: fastBuildDockerfileContents,
			embedded: true,
		},
		BuildModeCluster: {
			path:     filepath.Join(repoRoot, clusterBuildDockerfileName),
			contents: clusterBuildDockerfileContents,
			embedded: true,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("could not load Dockerfile: %w", err)
	}

	fastBuildMakefile, err := f.ReadFile(fastBuildMakefileName)
	if err != nil {
		return nil, fmt.Errorf("could not load %s: %w", fastBuildMakefileName, err)
	}

	makefiles, err := populateOverrideableRepoFileMap(map[BuildMode]overrideableRepoFile{
		BuildModeFast: {
			path:     filepath.Join(repoRoot, fastBuildMakefileName),
			contents: fastBuildMakefile,
			embedded: true,
		},
		BuildModeCluster: {
			path: filepath.Join(repoRoot, repoMakefileName),
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

func supportedVersions() []string {
	return []string{
		"4.14",
		"4.15",
	}
}

func getDockerfileContentsFromOCPVersion(version string, buildMode BuildMode) ([]byte, error) {
	suffixes := map[BuildMode]string{
		BuildModeFast:    "fast-build",
		BuildModeCluster: "cluster",
	}

	suffix, ok := suffixes[buildMode]
	if !ok {
		return nil, fmt.Errorf("no dockerfile suffix for build mode %s", buildMode)
	}

	versions := sets.New[string](supportedVersions()...)
	if !versions.Has(version) {
		return nil, fmt.Errorf("no dockerfile for suffix %s and OCP version %s", suffix, version)
	}

	filename := fmt.Sprintf("Dockerfile.%s-%s", suffix, version)
	return f.ReadFile(filename)
}

var ocpBaseRegex = regexp.MustCompile(`registry\.ci\.openshift.org\/ocp\/(4\.[0-9][0-9])\:base`)

func inferOCPVersionFromDockerfile(in []byte) (string, error) {
	matches := ocpBaseRegex.FindAllStringSubmatch(string(in), -1)
	if matches == nil {
		return "", fmt.Errorf("did not find an OCP base image in repo Dockerfile")
	}

	return matches[0][1], nil
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

func mustLoadEmbeddedFile(path string) []byte {
	out, err := f.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return out
}
