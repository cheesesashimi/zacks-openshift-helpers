package releasecontroller

import (
	"net/url"
	"path/filepath"
)

type Phase string

const (
	PhaseAccepted Phase = "Accepted"
	PhaseRejected Phase = "Rejected"
	PhaseReady    Phase = "Ready"
)

type ReleaseStream struct {
	name string
	rc   *ReleaseController
}

func (r *ReleaseStream) Name() string {
	return r.name
}

func (r *ReleaseStream) TagsByPhase(phase Phase) (*ReleaseTags, error) {
	out := &ReleaseTags{}
	err := r.rc.doHTTPRequestIntoStruct(filepath.Join("/api/v1/releasestream", r.name, "tags"), url.Values{"phase": []string{string(phase)}}, out)
	return out, err
}

func (r *ReleaseStream) Tags() (*ReleaseTags, error) {
	out := &ReleaseTags{}
	err := r.rc.doHTTPRequestIntoStruct(filepath.Join("/api/v1/releasestream", r.name, "tags"), nil, out)
	return out, err
}

func (r *ReleaseStream) Latest() (*Release, error) {
	out := &Release{}
	err := r.rc.doHTTPRequestIntoStruct(filepath.Join("/api/v1/releasestream", r.name, "latest"), nil, out)
	return out, err
}

func (r *ReleaseStream) Candidate() (*Release, error) {
	out := &Release{}
	err := r.rc.doHTTPRequestIntoStruct(filepath.Join("/api/v1/releasestream", r.name, "candidate"), nil, out)
	return out, err
}

func (r *ReleaseStream) Tag(tag string) (*APIReleaseInfo, error) {
	out := &APIReleaseInfo{}
	err := r.rc.doHTTPRequestIntoStruct(filepath.Join("/api/v1/releasestream", r.name, "release", tag), nil, out)
	return out, err
}

func (r *ReleaseStream) Config() ([]byte, error) {
	return r.rc.doHTTPRequestIntoBytes(filepath.Join("/api/v1/releasestream", r.name, "config"), nil)
}
