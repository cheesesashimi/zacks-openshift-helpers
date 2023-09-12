package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	giturl "github.com/kubescape/go-git-url"
	aggerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

const (
	dockerfileName string = "Dockerfile.fast-build"
	makefileName   string = "Makefile.fast-build"
)

//go:embed Dockerfile.fast-build
var dockerfile []byte

//go:embed Makefile.fast-build
var makefile []byte

func writeFile(name string, contents []byte) error {
	klog.Infof("Writing %s", name)
	return os.WriteFile(name, contents, 0o755)
}

func deleteFile(name string) error {
	klog.Infof("Removing %s", name)
	return os.Remove(name)
}

func setupRepo(gi *gitInfo) error {
	return aggerrs.NewAggregate([]error{
		writeFile(gi.dockerfilePath(), dockerfile),
		writeFile(gi.makefilePath(), makefile),
	})
}

func teardownRepo(gi *gitInfo) error {
	return aggerrs.NewAggregate([]error{
		deleteFile(gi.dockerfilePath()),
		deleteFile(gi.makefilePath()),
	})
}

type gitInfo struct {
	branchName string
	remoteURL  string
	repoRoot   string
}

func (g *gitInfo) makefilePath() string {
	return filepath.Join(g.repoRoot, makefileName)
}

func (g *gitInfo) dockerfilePath() string {
	return filepath.Join(g.repoRoot, dockerfileName)
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

func sanitizeBranchName(branchName string) string {
	return strings.ReplaceAll(branchName, "/", "_")
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
