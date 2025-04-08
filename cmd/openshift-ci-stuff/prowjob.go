package main

import (
	"net/url"
	"path/filepath"
	"strings"
)

const (
	artifactHostname string = "gcsweb-qe-private-deck-ci.apps.ci.l2s4.p1.openshiftapps.com"
	spyglassHostname string = "qe-private-deck-ci.apps.ci.l2s4.p1.openshiftapps.com"
)

// Holds the job path for a given ProwJob and can extract the job ID and job
// name from it. Note: This implementation leaves a lot to be desired since it
// relies upon position within the path to determine the job name and job ID.
// In the future, this should use a regex to find that information instead.
type prowjobLink struct {
	spyglassHost string
	gcsHost      string
	jobPath      string
}

func newProwjobLink(jobPath string) (prowjobLink, error) {
	jobPathURL, err := url.Parse(jobPath)
	if err != nil {
		return prowjobLink{}, err
	}

	pjl := prowjobLink{
		spyglassHost: spyglassHostname,
		gcsHost:      artifactHostname,
		jobPath:      jobPathURL.Path,
	}

	return pjl, nil
}

// Gets the job ID
func (p prowjobLink) JobID() string {
	return filepath.Base(p.jobPath)
}

// Gets the job name.
func (p prowjobLink) JobName() string {
	return filepath.Base(filepath.Dir(p.jobPath))
}

// Gets the spyglass (aka test status / progress) URL.
func (p prowjobLink) Spyglass() string {
	u := url.URL{
		Scheme: "https",
		Host:   p.spyglassHost,
		Path:   p.jobPath,
	}

	return u.String()
}

// Gets the short name of the job. This strips off the periodic=... data from
// the job name and returns it.
func (p prowjobLink) JobShortName() string {
	return getAbbrevJobName(p.JobName())
}

// Gets the root to the artifact directory for the given prowjob url.
func (p prowjobLink) ArtifactRoot() string {
	u := url.URL{
		Scheme: "https",
		Host:   p.gcsHost,
		Path:   strings.ReplaceAll(p.jobPath, "/view/gs", "/gcs"),
	}

	return u.String()
}

// Gets the full absolute URL to the must-gather.
func (p prowjobLink) MustGather() string {
	return p.ArtifactRoot() + filepath.Join("/artifacts", p.JobShortName(), "gather-must-gather", "artifacts", "must-gather.tar")
}

// Abbreviates the job name.
func getAbbrevJobName(jobName string) string {
	split := strings.Split(jobName, "-")
	index := 0
	for i, item := range split {
		if item == "nightly" {
			index = i + 1
			break
		}
	}

	return strings.Join(split[index:len(split)], "-")

}
