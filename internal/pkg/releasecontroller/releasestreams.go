package releasecontroller

import "fmt"

type ReleaseStreams struct {
	rc *ReleaseController
}

func (r *ReleaseStreams) FindReleaseNameAndStream(name string) (string, string, error) {
	streams, err := r.All()
	if err != nil {
		return "", "", err
	}

	for stream, releases := range streams {
		for _, release := range releases {
			if release == name {
				return stream, release, nil
			}
		}
	}

	return "", "", fmt.Errorf("could not find release %q", name)
}

func (r *ReleaseStreams) Accepted() (map[string][]string, error) {
	return r.doHTTPRequestIntoMapString("/api/v1/releasestreams/accepted")
}

func (r *ReleaseStreams) Rejected() (map[string][]string, error) {
	return r.doHTTPRequestIntoMapString("/api/v1/releasestreams/rejected")
}

func (r *ReleaseStreams) All() (map[string][]string, error) {
	return r.doHTTPRequestIntoMapString("/api/v1/releasestreams/all")
}

func (r *ReleaseStreams) Approvals() ([]Release, error) {
	out := []Release{}
	err := r.rc.doHTTPRequestIntoStruct("/api/v1/releasestreams/approvals", nil, &out)
	return out, err
}

func (r *ReleaseStreams) doHTTPRequestIntoMapString(path string) (map[string][]string, error) {
	out := map[string][]string{}
	err := r.rc.doHTTPRequestIntoStruct(path, nil, &out)
	return out, err
}
