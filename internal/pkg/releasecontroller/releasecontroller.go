package releasecontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
)

type ReleaseController string

func (r *ReleaseController) GraphForChannel(channel string) (*ReleaseGraph, error) {
	out := &ReleaseGraph{}
	err := r.doHTTPRequestIntoStruct("/graph", url.Values{"channel": []string{channel}}, out)
	return out, err
}

func (r *ReleaseController) Graph() (*ReleaseGraph, error) {
	out := &ReleaseGraph{}
	err := r.doHTTPRequestIntoStruct("/graph", nil, out)
	return out, err
}

func (r *ReleaseController) ReleaseStreams() *ReleaseStreams {
	return &ReleaseStreams{rc: r}
}

func (r *ReleaseController) ReleaseStream(name string) *ReleaseStream {
	return &ReleaseStream{
		name: name,
		rc:   r,
	}
}

// https://amd64.ocp.releases.ci.openshift.org/releasetag/4.15.0-0.nightly-2023-11-28-101923/json
//
// This returns raw bytes for now so we can use a dynamic JSON pathing library
// to parse it to avoid fighting with go mod.
//
// The raw bytes returned are very similar to the ones returned by $ oc adm
// release info. The sole difference seems to be that $ oc adm release info
// returns the fully qualified pullspec for the release instead of the tagged
// pullspec.
func (r *ReleaseController) GetReleaseInfoBytes(tag string) ([]byte, error) {
	return r.doHTTPRequestIntoBytes(filepath.Join("releasetag", tag, "json"), url.Values{})
}

func (r *ReleaseController) GetReleaseInfo(tag string) (*ReleaseInfo, error) {
	out := &ReleaseInfo{}
	err := r.doHTTPRequestIntoStruct(filepath.Join("releasetag", tag, "json"), url.Values{}, out)
	return out, err
}

func (r *ReleaseController) getURLForPath(path string, vals url.Values) url.URL {
	u := url.URL{
		Scheme: "https",
		Host:   string(*r),
		Path:   path,
	}

	if vals != nil {
		u.RawQuery = vals.Encode()
	}

	return u
}

func (r *ReleaseController) doHTTPRequest(u url.URL) (*http.Response, error) {
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("got HTTP 404 from %s", u.String())
	}

	return resp, nil
}

func (r *ReleaseController) doHTTPRequestIntoStruct(path string, vals url.Values, out interface{}) error {
	resp, err := r.doHTTPRequest(r.getURLForPath(path, vals))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(out)
}

func (r *ReleaseController) doHTTPRequestIntoBytes(path string, vals url.Values) ([]byte, error) {
	resp, err := r.doHTTPRequest(r.getURLForPath(path, vals))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	out := bytes.NewBuffer([]byte{})

	if _, err := io.Copy(out, resp.Body); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

const (
	Amd64OcpReleaseController   ReleaseController = "amd64.ocp.releases.ci.openshift.org"
	Arm64OcpReleaseController   ReleaseController = "arm64.ocp.releases.ci.openshift.org"
	Ppc64leOcpReleaseController ReleaseController = "ppc64le.ocp.releases.ci.openshift.org"
	S390xOcpReleaseController   ReleaseController = "s390x.ocp.releases.ci.openshift.org"
	MultiOcpReleaseController   ReleaseController = "multi.ocp.releases.ci.openshift.org"
	Amd64OkdReleaseController   ReleaseController = "amd64.origin.releases.ci.openshift.org"
)

func GetReleaseController(kind, arch string) (ReleaseController, error) {
	rcs := map[string]map[string]ReleaseController{
		"ocp": {
			"amd64":   Amd64OcpReleaseController,
			"arm64":   Arm64OcpReleaseController,
			"ppc64le": Ppc64leOcpReleaseController,
			"s390x":   S390xOcpReleaseController,
			"multi":   MultiOcpReleaseController,
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
