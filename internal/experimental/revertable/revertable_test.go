package daemon

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRevertable struct {
	mock.Mock
}

func (m *mockRevertable) Apply() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockRevertable) Revert() error {
	args := m.Called()
	return args.Error(0)
}

func TestRevertable(t *testing.T) {
	testCases := []struct {
		name        string
		setupFunc   func([]*mockRevertable)
		expectedErr bool
	}{
		{
			name: "all applies are run",
			setupFunc: func(mrs []*mockRevertable) {
				for i := range mrs {
					mrs[i].On("Apply").Return(nil)
				}
			},
		},
		{
			name: "revert is ran for each revertable following error",
			setupFunc: func(mrs []*mockRevertable) {
				for i := 0; i <= 8; i++ {
					mrs[i].On("Apply").Return(nil)
					mrs[i].On("Revert").Return(nil)
				}

				mrs[9].On("Apply").Return(fmt.Errorf("apply error"))
				mrs[9].On("Revert").Return(nil)
			},
			expectedErr: true,
		},
		{
			name: "revert is ran even when a revert error occurs",
			setupFunc: func(mrs []*mockRevertable) {
				for i := 0; i <= 7; i++ {
					mrs[i].On("Apply").Return(nil)
					mrs[i].On("Revert").Return(nil)
				}

				mrs[8].On("Apply").Return(nil)
				mrs[8].On("Revert").Return(fmt.Errorf("revert error"))

				mrs[9].On("Apply").Return(fmt.Errorf("apply error"))
				mrs[9].On("Revert").Return(nil)
			},
			expectedErr: true,
		},
		{
			name: "revert halts after canceled revert error",
			setupFunc: func(mrs []*mockRevertable) {
				for i := 0; i <= 6; i++ {
					mrs[i].On("Apply").Return(nil)
				}

				mrs[7].On("Apply").Return(nil)
				mrs[7].On("Revert").Return(fmt.Errorf("cancelable revert error: %w", ErrCanceledRevert))

				mrs[8].On("Apply").Return(nil)
				mrs[8].On("Revert").Return(fmt.Errorf("revert error"))

				mrs[9].On("Apply").Return(fmt.Errorf("apply error"))
				mrs[9].On("Revert").Return(nil)
			},
			expectedErr: true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mrs := []*mockRevertable{}
			for i := 0; i <= 10; i++ {
				mrs = append(mrs, new(mockRevertable))
			}

			testCase.setupFunc(mrs)

			rs := []revertable{}

			for _, item := range mrs {
				rs = append(rs, item)
			}

			err := RunRevertables(rs)
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			for i := range mrs {
				mrs[i].AssertExpectations(t)
			}
		})
	}
}
