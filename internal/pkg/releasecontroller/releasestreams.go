package releasecontroller

import (
	"context"
	"fmt"
)

type ReleaseStreams struct {
	rc *ReleaseController
}

func (r *ReleaseStreams) FindReleaseNameAndStream(ctx context.Context, name string) (string, string, error) {
	streams, err := r.All(ctx)
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

func (r *ReleaseStreams) Accepted(ctx context.Context) (map[string][]string, error) {
	return r.doHTTPRequestIntoMapString(ctx, "/api/v1/releasestreams/accepted")
}

func (r *ReleaseStreams) Rejected(ctx context.Context) (map[string][]string, error) {
	return r.doHTTPRequestIntoMapString(ctx, "/api/v1/releasestreams/rejected")
}

func (r *ReleaseStreams) All(ctx context.Context) (map[string][]string, error) {
	return r.doHTTPRequestIntoMapString(ctx, "/api/v1/releasestreams/all")
}

func (r *ReleaseStreams) Approvals(ctx context.Context) ([]Release, error) {
	out := []Release{}
	err := r.rc.doHTTPRequestIntoStruct(ctx, "/api/v1/releasestreams/approvals", nil, &out)
	return out, err
}

func (r *ReleaseStreams) doHTTPRequestIntoMapString(ctx context.Context, path string) (map[string][]string, error) {
	out := map[string][]string{}
	err := r.rc.doHTTPRequestIntoStruct(ctx, path, nil, &out)
	return out, err
}
