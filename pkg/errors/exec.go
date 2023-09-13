package errors

import (
	"fmt"
	"os/exec"
)

type ExecError struct {
	command string
	output  []byte
	err     error
}

func (e *ExecError) Unwrap() error {
	return e.err
}

func (e *ExecError) Error() string {
	return fmt.Sprintf("unable to run %s, output %s, error: %s", e.command, string(e.output), e.err)
}

func NewExecError(cmd *exec.Cmd, output []byte, err error) error {
	return &ExecError{
		command: cmd.String(),
		output:  output,
		err:     err,
	}
}
