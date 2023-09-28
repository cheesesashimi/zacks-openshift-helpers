package releasecontroller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
)

type ReleaseController string

func (r *ReleaseController) withReleaseStream(stream string) string {
	u := url.URL{
		Scheme: "https",
		Host:   string(*r),
		Path:   filepath.Join("/api/v1/releasestream", stream, "latest"),
	}

	return u.String()
}

const (
	Amd64OcpReleaseController   ReleaseController = "amd64.ocp.releases.ci.openshift.org"
	Arm64OcpReleaseController   ReleaseController = "arm64.ocp.releases.ci.openshift.org"
	Ppc64leOcpReleaseController ReleaseController = "ppc64le.ocp.releases.ci.openshift.org"
	S390xOcpReleaseController   ReleaseController = "s390x.ocp.releases.ci.openshift.org"
	MultiOcpReleaseController   ReleaseController = "multi.ocp.releases.ci.openshift.org"
	Amd64OkdReleaseController   ReleaseController = "amd64.origin.releases.ci.openshift.org"
)

type Release struct {
	Name        string `json:"name"`
	Phase       string `json:"phase"`
	Pullspec    string `json:"pullSpec"`
	DownloadURL string `json:"downloadURL"`
}

func GetReleaseController(kind, arch string) (ReleaseController, error) {
	rcs := map[string]map[string]ReleaseController{
		"ocp": {
			"amd64": Amd64OcpReleaseController,
			"arm64": Arm64OcpReleaseController,
			"multi": MultiOcpReleaseController,
		},
		"okd": {
			"amd64": Amd64OkdReleaseController,
		},
		"okd-scos": {
			"amd64": Amd64OkdReleaseController,
		},
	}

	if _, ok := rcs[kind]; !ok {
		return "", fmt.Errorf("invalid kind %q", kind)
	}

	if _, ok := rcs[kind][arch]; !ok {
		return "", fmt.Errorf("invalid arch %q for kind %q", arch, kind)
	}

	return rcs[kind][arch], nil
}

func GetRelease(rc ReleaseController, releaseStream string) (*Release, error) {
	resp, err := http.Get(rc.withReleaseStream(releaseStream))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("unknown release stream %q on release controller %s", releaseStream, rc)
	}

	defer resp.Body.Close()

	out := &Release{}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, err
	}

	return out, nil
}
