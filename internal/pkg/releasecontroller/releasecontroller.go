package releasecontroller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"time"
)

// ReleaseController represents a release controller API client
type ReleaseController struct {
	host   string
	client *http.Client
}

// ReleaseControllerConfig holds configuration options for the ReleaseController
type ReleaseControllerConfig struct {
	DefaultTimeout time.Duration
	Client         *http.Client // optional user-provided client
}

// New creates a new ReleaseController with the given host and configuration
func New(host string, cfg *ReleaseControllerConfig) *ReleaseController {
	if cfg == nil {
		cfg = &ReleaseControllerConfig{DefaultTimeout: 30 * time.Second}
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: cfg.DefaultTimeout}
	}
	return &ReleaseController{host: host, client: client}
}

// Host returns the hostname of the release controller
func (r *ReleaseController) Host() string {
	return r.host
}

// String returns the hostname of the release controller
func (r *ReleaseController) String() string {
	return r.host
}

func (r *ReleaseController) GraphForChannel(ctx context.Context, channel string) (*ReleaseGraph, error) {
	out := &ReleaseGraph{}
	err := r.doHTTPRequestIntoStruct(ctx, "/graph", url.Values{"channel": []string{channel}}, out)
	return out, err
}

func (r *ReleaseController) Graph(ctx context.Context) (*ReleaseGraph, error) {
	out := &ReleaseGraph{}
	err := r.doHTTPRequestIntoStruct(ctx, "/graph", nil, out)
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
func (r *ReleaseController) GetReleaseInfoBytes(ctx context.Context, tag string) ([]byte, error) {
	return r.doHTTPRequestIntoBytes(ctx, filepath.Join("releasetag", tag, "json"), url.Values{})
}

func (r *ReleaseController) GetReleaseInfo(ctx context.Context, tag string) (*ReleaseInfo, error) {
	out := &ReleaseInfo{}
	err := r.doHTTPRequestIntoStruct(ctx, filepath.Join("releasetag", tag, "json"), url.Values{}, out)
	return out, err
}

func (r *ReleaseController) getURLForPath(path string, vals url.Values) url.URL {
	u := url.URL{
		Scheme: "https",
		Host:   r.host,
		Path:   path,
	}

	if vals != nil {
		u.RawQuery = vals.Encode()
	}

	return u
}

func (r *ReleaseController) doHTTPRequest(ctx context.Context, u url.URL) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, fmt.Errorf("got HTTP 404 from %s", u.String())
	}

	return resp, nil
}

func (r *ReleaseController) doHTTPRequestIntoStruct(ctx context.Context, path string, vals url.Values, out interface{}) error {
	resp, err := r.doHTTPRequest(ctx, r.getURLForPath(path, vals))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(out)
}

func (r *ReleaseController) doHTTPRequestIntoBytes(ctx context.Context, path string, vals url.Values) ([]byte, error) {
	resp, err := r.doHTTPRequest(ctx, r.getURLForPath(path, vals))
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
	Amd64OcpReleaseController   = "amd64.ocp.releases.ci.openshift.org"
	Arm64OcpReleaseController   = "arm64.ocp.releases.ci.openshift.org"
	Ppc64leOcpReleaseController = "ppc64le.ocp.releases.ci.openshift.org"
	S390xOcpReleaseController   = "s390x.ocp.releases.ci.openshift.org"
	MultiOcpReleaseController   = "multi.ocp.releases.ci.openshift.org"
	Amd64OkdReleaseController   = "amd64.origin.releases.ci.openshift.org"
)

func getReleaseControllerHost(kind, arch string) (string, error) {
	rcs := map[string]map[string]string{
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

func GetReleaseController(kind, arch string) (*ReleaseController, error) {
	host, err := getReleaseControllerHost(kind, arch)
	if err != nil {
		return nil, err
	}
	return New(host, nil), nil
}

func All() []*ReleaseController {
	hosts := []string{
		Amd64OcpReleaseController,
		Arm64OcpReleaseController,
		Ppc64leOcpReleaseController,
		S390xOcpReleaseController,
		MultiOcpReleaseController,
		Amd64OkdReleaseController,
	}

	rcs := make([]*ReleaseController, 0, len(hosts))
	for _, host := range hosts {
		rcs = append(rcs, New(host, nil))
	}
	return rcs
}
