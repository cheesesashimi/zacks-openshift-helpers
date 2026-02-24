package main

import (
	"fmt"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/cmd/mcocheckout/internal/repo"
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/containers"
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"github.com/davecgh/go-spew/spew"
)

func doMCORepoCheckout(ref string) error {
	mcoRepo, err := repo.NewMCORepo()
	if err != nil {
		return fmt.Errorf("could not initialize MCO repo: %w", err)
	}

	isClean, err := mcoRepo.IsClean()
	if err != nil {
		return fmt.Errorf("could not check if MCO repo is clean: %w", err)
	}

	if !isClean {
		return fmt.Errorf("repo not clean")
	}

	if err := mcoRepo.Checkout(ref); err != nil {
		return fmt.Errorf("ref %q checkout failed: %w", ref, err)
	}

	return nil
}

var releaseNameNotFound error = fmt.Errorf("release not found")

func isReleaseImage(pullspec string) (bool, error) {
	labels, err := containers.GetImageLabelsWithSkopeo(pullspec)
	if err != nil {
		return false, err
	}

	_, ok := labels["io.openshift.release"]
	return ok, nil
}

func getMCOCommitFromComponentPullspec(pullspec string) (string, error) {
	labels, err := containers.GetImageLabelsWithSkopeo(pullspec)
	if err != nil {
		return "", err
	}

	repoKey := "io.openshift.build.source-location"
	repoVal, ok := labels[repoKey]
	if !ok || repoVal == "" {
		return "", fmt.Errorf("missing annotation key %q", repoKey)
	}

	if repoVal != "https://github.com/openshift/machine-config-operator" {
		return "", fmt.Errorf("invalid repo annotation %s=%s", repoKey, repoVal)
	}

	commitKey := "io.openshift.build.commit.id"
	commitVal, ok := labels[commitKey]
	if !ok || commitVal == "" {
		return "", fmt.Errorf("missing annotation key %q", repoKey)
	}

	return commitVal, nil
}

func getMCOCommitFromPullspec(pullspec string) (string, error) {
	isRelImg, err := isReleaseImage(pullspec)
	if err != nil {
		return "", err
	}

	if isRelImg {
		return getMCOCommitFromReleasePullspec(pullspec)
	}

	return getMCOCommitFromComponentPullspec(pullspec)
}

func getMCOCommitFromReleasePullspec(pullspec string) (string, error) {
	relInfo, err := releasecontroller.GetReleaseInfo(pullspec)
	if err != nil {
		return "", err
	}

	component := "machine-config-operator"

	tagRef := relInfo.GetTagRefForComponentName(component)
	if tagRef == nil {
		return "", fmt.Errorf("no tag ref found for %s", component)
	}

	annoKey := "io.openshift.build.commit.id"
	val, ok := tagRef.Annotations[annoKey]
	if ok && val != "" {
		optTagRefs := []string{"rhel-coreos", "rhel-coreos-extensions", "rhel-coreos-10", "rhel-coreos-10-extensions", "stream-coreos", "stream-coreos-extensions"}

		for _, optTagRef := range optTagRefs {
			if tagRef := relInfo.GetTagRefForComponentName(optTagRef); tagRef != nil && tagRef.From.Name != "" {
				fmt.Println(optTagRef+" image:", tagRef.From.Name)
			}
		}
		fmt.Println(component, "image:", tagRef.From.Name)
		return val, nil
	}

	if val == "" {
		return "", fmt.Errorf("annotation %q is empty", annoKey)
	}

	return "", fmt.Errorf("missing required annotation %q or value empty", annoKey)
}

func findReleaseForName(rc releasecontroller.ReleaseController, name string) (*releasecontroller.Release, error) {
	fmt.Println("Querying release controller", rc, "for", name)
	streams, err := rc.ReleaseStreams().Accepted()
	if err != nil {
		return nil, err
	}

	for stream := range streams {
		releaseTags, err := rc.ReleaseStream(stream).Tags()
		if err != nil {
			return nil, err
		}

		for _, tag := range releaseTags.Tags {
			if tag.Name == name {
				tag := tag
				fmt.Println("Release pullspec:", tag.Pullspec)
				return &tag, nil
			}
		}
	}

	return nil, releaseNameNotFound
}

func run(input string) error {
	vk, err := releasecontroller.GetVersionKind(input)
	if err != nil {
		return err
	}

	commit := ""
	if vk == releasecontroller.SemverVersionKind {
		stripped := strings.Replace(input, "v", "", 1)

		rc, err := releasecontroller.GetReleaseController("ocp", "amd64")
		if err != nil {
			return err
		}

		release, err := findReleaseForName(rc, stripped)
		if err != nil {
			return err
		}

		c, err := getMCOCommitFromReleasePullspec(release.Pullspec)
		if err != nil {
			return err
		}

		commit = c
	} else {
		c, err := getMCOCommitFromPullspec(input)
		if err != nil {
			return err
		}

		commit = c
	}

	fmt.Println("Commit SHA:", commit)

	if err := doMCORepoCheckout(commit); err != nil {
		return err
	}

	return nil
}

func main() {
	mcoRepo, err := repo.NewMCORepo()
	if err != nil {
		panic(err)
	}

	mc, err := mcoRepo.GetMergeCommits()
	if err != nil {
		panic(err)
	}

	spew.Dump(mc[0])

	//	if len(os.Args) == 0 {
	//		panic("must provide an arg")
	//	}
	//
	// input := os.Args[1]
	//
	//	if err := run(input); err != nil {
	//		panic(err)
	//	}
}
