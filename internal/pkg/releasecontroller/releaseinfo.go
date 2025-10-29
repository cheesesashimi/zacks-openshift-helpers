package releasecontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	imagev1 "github.com/openshift/api/image/v1"
)

// There is a way to do this in pure Go, but I'm lazy :P.
func GetComponentPullspecForRelease(componentName, releasePullspec string) (string, error) {
	template := fmt.Sprintf("{{range .references.spec.tags}}{{if eq .name %q}}{{.from.name}}{{end}}{{end}}", componentName)

	outBuf := bytes.NewBuffer([]byte{})

	cmd := exec.Command("oc", "adm", "release", "info", "-o=template="+template, releasePullspec)
	cmd.Stdout = outBuf
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("could not get pullspec for component %q from release pullspec %q: %w", componentName, releasePullspec, err)
	}

	return strings.TrimSpace(outBuf.String()), nil
}

func getReleaseInfoBytes(releasePullspec, authfilePath string) ([]byte, error) {
	outBuf := bytes.NewBuffer([]byte{})
	stderrBuf := bytes.NewBuffer([]byte{})

	opts := []string{"oc", "adm", "release", "info"}
	if authfilePath != "" {
		opts = append(opts, []string{"--registry-config", authfilePath}...)
	}

	opts = append(opts, []string{"-o=json", releasePullspec}...)

	cmd := exec.Command(opts[0], opts[1:]...)
	cmd.Stdout = outBuf
	cmd.Stderr = stderrBuf

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not run %s, got output: %s %s", cmd, outBuf.String(), stderrBuf.String())
	}

	return outBuf.Bytes(), nil
}

func GetReleaseInfoBytesWithAuthfile(releasePullspec, authfilePath string) ([]byte, error) {
	return getReleaseInfoBytes(releasePullspec, authfilePath)
}

func GetReleaseInfoBytes(releasePullspec string) ([]byte, error) {
	return getReleaseInfoBytes(releasePullspec, "")
}

type Config struct {
	Architecture string `json:"architecture,omitempty"`
	Created      string `json:"created,omitempty"`
}

type ReleaseInfo struct {
	Config          Config                    `json:"config,omitempty"`
	Image           string                    `json:"image,omitempty"`
	Digest          string                    `json:"digest,omitempty"`
	ContentDigest   string                    `json:"contentDigest,omitempty"`
	ListDigest      string                    `json:"listDigest,omitempty"`
	References      *imagev1.ImageStream      `json:"references,omitempty"`
	ReleasePullspec string                    `json:"releasePullspec,omitempty"`
	Metadata        Metadata                  `json:"metadata,omitempty"`
	DisplayVersions map[string]DisplayVersion `json:"displayVersions,omitempty"`
}

func (ri *ReleaseInfo) GetTagRefForComponentName(name string) *imagev1.TagReference {
	for _, tag := range ri.References.Spec.Tags {
		if tag.Name == name {
			return tag.DeepCopy()
		}
	}

	return nil
}

type Metadata struct {
	Kind     string   `json:"kind,omitempty"`
	Version  string   `json:"version,omitempty"`
	Previous []string `json:"previous,omitempty"`
}

type DisplayVersion struct {
	Version     string `json:"version,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

func GetReleaseInfo(releasePullspec string) (*ReleaseInfo, error) {
	return getReleaseInfo(releasePullspec, "")
}

func GetReleaseInfoWithAuthfile(releasePullspec, authfilePath string) (*ReleaseInfo, error) {
	return getReleaseInfo(releasePullspec, authfilePath)
}

func getReleaseInfo(releasePullspec, authfilePath string) (*ReleaseInfo, error) {
	riBytes, err := getReleaseInfoBytes(releasePullspec, authfilePath)
	if err != nil {
		return nil, err
	}

	ri := &ReleaseInfo{}
	if err := json.Unmarshal(riBytes, ri); err != nil {
		return nil, err
	}

	ri.ReleasePullspec = releasePullspec

	return ri, nil

}
