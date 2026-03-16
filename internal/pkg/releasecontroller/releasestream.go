package releasecontroller

import (
	"context"
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

func (r *ReleaseStream) TagsByPhase(ctx context.Context, phase Phase) (*ReleaseTags, error) {
	out := &ReleaseTags{}
	err := r.rc.doHTTPRequestIntoStruct(ctx, filepath.Join("/api/v1/releasestream", r.name, "tags"), url.Values{"phase": []string{string(phase)}}, out)
	return out, err
}

func (r *ReleaseStream) Tags(ctx context.Context) (*ReleaseTags, error) {
	out := &ReleaseTags{}
	err := r.rc.doHTTPRequestIntoStruct(ctx, filepath.Join("/api/v1/releasestream", r.name, "tags"), nil, out)
	return out, err
}

func (r *ReleaseStream) Latest(ctx context.Context) (*Release, error) {
	out := &Release{}
	err := r.rc.doHTTPRequestIntoStruct(ctx, filepath.Join("/api/v1/releasestream", r.name, "latest"), nil, out)
	return out, err
}

func (r *ReleaseStream) Candidate(ctx context.Context) (*Release, error) {
	out := &Release{}
	err := r.rc.doHTTPRequestIntoStruct(ctx, filepath.Join("/api/v1/releasestream", r.name, "candidate"), nil, out)
	return out, err
}

func (r *ReleaseStream) Tag(ctx context.Context, tag string) (*APIReleaseInfo, error) {
	out := &APIReleaseInfo{}
	err := r.rc.doHTTPRequestIntoStruct(ctx, filepath.Join("/api/v1/releasestream", r.name, "release", tag), nil, out)
	return out, err
}

func (r *ReleaseStream) Config(ctx context.Context) ([]byte, error) {
	return r.rc.doHTTPRequestIntoBytes(ctx, filepath.Join("/api/v1/releasestream", r.name, "config"), nil)
}
