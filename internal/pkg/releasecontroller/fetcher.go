package releasecontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	imagev1 "github.com/openshift/api/image/v1"
	"golang.org/x/sync/errgroup"
)

type releaseInfoFetcher struct {
	rc ReleaseController
}

type ReleaseInfoResults struct {
	ReleaseInfo       json.RawMessage            `json:"releaseInfo,omitempty"`
	ComponentMetadata map[string]json.RawMessage `json:"componentMetadata,omitempty"`
}

type componentImageMetadata struct {
	name string
	data json.RawMessage
}

func NewReleaseInfoFetcher(rc ReleaseController) *releaseInfoFetcher {
	return &releaseInfoFetcher{
		rc: rc,
	}
}

func (r *releaseInfoFetcher) FetchReleaseInfo(tagOrPullspec string) (*ReleaseInfoResults, error) {
	ri, _, err := r.getReleaseInfoForPullspec(tagOrPullspec)
	return ri, err
}

func (r *releaseInfoFetcher) FetchWithComponents(tagOrPullspec string, components []string) (*ReleaseInfoResults, error) {
	ri, _, err := r.getReleaseInfoForPullspec(tagOrPullspec)
	if err != nil {
		return nil, err
	}

	results := &ReleaseInfo{}
	if err := json.Unmarshal(ri.ReleaseInfo, results); err != nil {
		return nil, err
	}

	cim, err := r.fetchAllComponentMetadata(results, components)
	if err != nil {
		return nil, err
	}

	for _, im := range cim {
		ri.ComponentMetadata[im.name] = im.data
	}

	return ri, nil
}

func (r *releaseInfoFetcher) FetchWithAllComponents(tagOrPullspec string) (*ReleaseInfoResults, error) {
	return r.FetchWithComponents(tagOrPullspec, []string{})
}

func (r *releaseInfoFetcher) getReleaseInfoForPullspec(tagOrPullspec string) (*ReleaseInfoResults, string, error) {
	vk, err := GetVersionKind(tagOrPullspec)
	if err != nil {
		return nil, "", err
	}

	pullspec := ""
	if vk == PullspecVersionKind {
		pullspec = tagOrPullspec
	}

	if vk == SemverVersionKind {
		ps, err := r.findPullspecForReleaseTag(tagOrPullspec)
		if err != nil {
			return nil, "", err
		}

		pullspec = ps
	}

	if pullspec == "" {
		return nil, "", fmt.Errorf("invalid versionkind %q", vk)
	}

	riBytes, err := GetReleaseInfoBytes(pullspec)
	if err != nil {
		return nil, "", err
	}

	return &ReleaseInfoResults{ReleaseInfo: riBytes, ComponentMetadata: map[string]json.RawMessage{}}, pullspec, err
}

func (r *releaseInfoFetcher) findPullspecForReleaseTag(releaseTag string) (string, error) {
	stream, release, err := r.rc.ReleaseStreams().FindReleaseNameAndStream(releaseTag)
	if err != nil {
		return "", err
	}

	// TODO: Check if rejected or ready tags can retrieve a release.
	tags, err := r.rc.ReleaseStream(stream).TagsByPhase(PhaseAccepted)
	if err != nil {
		return "", err
	}

	for _, tag := range tags.Tags {
		if tag.Name == release {
			return tag.Pullspec, nil
		}
	}

	return "", fmt.Errorf("unknown tag %q for release stream %q", release, stream)
}

func (r *releaseInfoFetcher) fetchAllComponentMetadata(rl *ReleaseInfo, components []string) ([]componentImageMetadata, error) {
	resultChan := make(chan componentImageMetadata)
	g := new(errgroup.Group)

	// TODO: Figure out how to make this more configurable.
	g.SetLimit(10)

	out := []componentImageMetadata{}
	done := make(chan struct{})

	componentsToFetch, err := filterPayloadComponents(rl, components)
	if err != nil {
		return nil, err
	}

	go func() {
		for r := range resultChan {
			out = append(out, r)
		}
		close(done)
	}()

	for _, tag := range componentsToFetch {
		tag := tag
		g.Go(func() error {
			cim, err := r.fetchComponentImageMetadata(tag)
			if err != nil {
				return err
			}
			resultChan <- *cim
			return nil
		})
	}

	err = g.Wait()
	close(resultChan)

	<-done

	if err != nil {
		return nil, err
	}

	return out, nil
}

func (r *releaseInfoFetcher) fetchComponentImageMetadata(tag imagev1.TagReference) (*componentImageMetadata, error) {
	cmd := exec.Command("skopeo", "inspect", "--no-tags", fmt.Sprintf("docker://%s", tag.From.Name))

	outBuf := bytes.NewBuffer([]byte{})
	cmd.Stdout = outBuf
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not fetch metadata for component %s: %w", tag.Name, err)
	}

	return &componentImageMetadata{
		name: tag.Name,
		data: outBuf.Bytes(),
	}, nil
}

func filterPayloadComponents(rl *ReleaseInfo, componentsToFilter []string) ([]imagev1.TagReference, error) {
	// If there are no desired components, just return all the ones we were given as-is.
	if len(componentsToFilter) == 0 {
		return rl.References.Spec.Tags, nil
	}

	out := []imagev1.TagReference{}
	for _, component := range componentsToFilter {
		found := rl.GetTagRefForComponentName(component)
		if found == nil {
			return nil, fmt.Errorf("unknown component %q", component)
		}

		out = append(out, *found)
	}

	return out, nil
}
