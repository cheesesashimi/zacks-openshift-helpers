package releasecontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVersionKind(t *testing.T) {
	testCases := []struct {
		name                string
		input               string
		expectedVersionKind VersionKind
		expectErr           bool
	}{
		{
			name:                "OCP release version",
			input:               "4.18.1",
			expectedVersionKind: SemverVersionKind,
		},
		{
			name:                "OCP release version with leading v",
			input:               "v4.18.1",
			expectedVersionKind: SemverVersionKind,
		},
		{
			name:                "OKD release version",
			input:               "4.20.0-okd-scos.17",
			expectedVersionKind: SemverVersionKind,
		},
		{
			name:                "OKD release version with leading v",
			input:               "4.20.0-okd-scos.17",
			expectedVersionKind: SemverVersionKind,
		},
		{
			name:                "Tagged pullspec",
			input:               "quay.io/org/repo:tag",
			expectedVersionKind: PullspecVersionKind,
		},
		{
			name:                "Digested pullspec",
			input:               "quay.io/example/image@sha256:544d9fd59f8c711929d53e50ac22b19b329d95c2fcf1093cb590ac255267b2d8",
			expectedVersionKind: PullspecVersionKind,
		},
		{
			name:      "Invalid",
			input:     "invalid",
			expectErr: true,
		},
		{
			name:      "Special character",
			input:     "@",
			expectErr: true,
		},
		{
			name:      "Special character",
			input:     "/",
			expectErr: true,
		},
		{
			name:      "Special character",
			input:     ":",
			expectErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			vk, err := GetVersionKind(testCase.input)
			if testCase.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedVersionKind, vk)
		})
	}
}
