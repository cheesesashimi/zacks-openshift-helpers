package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
)

type releaseStreamsHelper struct {
	rc *releasecontroller.ReleaseController
}

func newReleaseStreamsHelper(rc *releasecontroller.ReleaseController) *releaseStreamsHelper {
	return &releaseStreamsHelper{rc: rc}
}

func (r *releaseStreamsHelper) AllReleaseStreamNames(ctx context.Context) ([]string, error) {
	streams, err := r.rc.ReleaseStreams().All(ctx)
	if err != nil {
		return nil, err
	}

	out := []string{}
	for stream := range streams {
		out = append(out, stream)
	}

	sort.Strings(out)

	return out, nil
}

func (r *releaseStreamsHelper) AllReleasesForReleaseStreams(ctx context.Context, releaseStreamNames []string) (map[string][]string, error) {
	return r.filterReleases(ctx, releaseStreamNames, r.rc.ReleaseStreams().All)
}

func (r *releaseStreamsHelper) AcceptedReleasesForReleaseStreams(ctx context.Context, releaseStreamNames []string) (map[string][]string, error) {
	return r.filterReleases(ctx, releaseStreamNames, r.rc.ReleaseStreams().Accepted)
}

func (r *releaseStreamsHelper) RejectedReleasesForReleaseStreams(ctx context.Context, releaseStreamNames []string) (map[string][]string, error) {
	return r.filterReleases(ctx, releaseStreamNames, r.rc.ReleaseStreams().Rejected)
}

func (r *releaseStreamsHelper) filterReleases(ctx context.Context, keys []string, queryFunc func(context.Context) (map[string][]string, error)) (map[string][]string, error) {
	releases, err := queryFunc(ctx)
	if err != nil {
		return nil, err
	}

	// If there are no keys to filter, return the data as-is.
	if len(keys) == 0 {
		return releases, nil
	}

	out := map[string][]string{}
	for _, key := range keys {
		val, ok := releases[key]
		if !ok {
			return nil, fmt.Errorf("invalid releasestream %q", key)
		}

		out[key] = val
	}

	return out, nil
}
