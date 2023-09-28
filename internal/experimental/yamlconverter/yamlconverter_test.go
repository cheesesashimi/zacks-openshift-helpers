package yamlconverter

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed configmap-test-input.yaml
var configmapTestInput []byte

//go:embed configmap-test-output.txt
var configmapTestOutput string

//go:embed machineconfig-test-input.yaml
var machineconfigTestInput []byte

//go:embed machineconfig-test-output.txt
var machineconfigTestOutput string

func TestYAMLConverter(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		input          []byte
		expectedOutput string
		errExpected    bool
	}{
		{
			name:           "ConfigMaps",
			input:          configmapTestInput,
			expectedOutput: configmapTestOutput,
		},
		{
			name:           "MachineConfig",
			input:          machineconfigTestInput,
			expectedOutput: machineconfigTestOutput,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			out, err := KubeYAMLToStruct(testCase.input)

			if testCase.errExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedOutput, out)
			}
		})
	}
}
