package releasecontroller

type ReleaseStreams struct {
	rc *ReleaseController
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
