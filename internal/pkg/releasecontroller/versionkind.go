package releasecontroller

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/containers/image/v5/docker/reference"
	"github.com/coreos/go-semver/semver"
)

type VersionKind string

const (
	SemverVersionKind   VersionKind = "Semver"
	PullspecVersionKind VersionKind = "Pullspec"
)

func GetVersionKind(in string) (VersionKind, error) {
	vk, err := getVersionKind(in)
	if err != nil {
		return "", fmt.Errorf("unable to determine version kind: %w", err)
	}

	return vk, nil
}

func getVersionKind(in string) (VersionKind, error) {
	if hasSpecialCharsForImagePullspec(in) {
		return getVersionKindFromPullspec(in)
	}

	return getVersionKindFromInputString(in)

}

func getVersionKindFromInputString(in string) (VersionKind, error) {
	if strings.HasPrefix(in, "v") {
		in = strings.Replace(in, "v", "", 1)
	}

	if !unicode.IsDigit([]rune(in)[0]) {
		return "", fmt.Errorf("does not start with a digit")
	}

	ver, err := semver.NewVersion(in)
	if err != nil {
		return "", err
	}

	if ver.Major != 0 && ver.Minor != 0 {
		return SemverVersionKind, nil
	}

	return "", fmt.Errorf("unknown OCP / OKD version kind")
}

func getVersionKindFromPullspec(in string) (VersionKind, error) {
	named, err := reference.ParseNormalizedNamed(in)
	if err != nil {
		return "", err
	}

	if _, ok := named.(reference.NamedTagged); ok {
		return PullspecVersionKind, nil
	}

	if _, ok := named.(reference.Digested); ok {
		return PullspecVersionKind, nil
	}

	return "", nil
}

func hasSpecialCharsForImagePullspec(in string) bool {
	chars := []string{"/", "@", ":"}
	for _, char := range chars {
		if strings.Contains(in, char) {
			return true
		}
	}

	return false
}
