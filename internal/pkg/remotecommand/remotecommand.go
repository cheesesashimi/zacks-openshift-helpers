package remotecommand

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	corev1 "k8s.io/api/core/v1"
)

type execError struct {
	opts      *ExecOpts
	err       error
	errString string
}

func newExecError(opts *ExecOpts, err error) error {
	return &execError{
		opts:      opts,
		err:       err,
		errString: "",
	}
}

func (e *execError) Error() string {
	if e.errString != "" {
		return e.errString
	}

	e.errString = e.getErrString()

	return e.errString
}

func (e *execError) getErrString() string {
	sb := &strings.Builder{}

	fmt.Fprintf(sb, "failed to execute command %v on %s", e.opts.Command, e.opts.getPodContainerNamespace())

	if e.opts.stdoutBuf != nil {
		fmt.Fprintf(sb, " stdout: %q", e.opts.stdoutBuf.String())
	}

	if e.opts.stderrBuf != nil {
		fmt.Fprintf(sb, " stderr: %q", e.opts.stderrBuf.String())
	}

	fmt.Fprintf(sb, ": %s", e.err)

	return sb.String()
}

func (e *execError) Unwrap() error {
	return e.err
}

type ExecOpts struct {
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Container string
	Command   []string
	Pod       *corev1.Pod

	stdoutBuf *bytes.Buffer
	stderrBuf *bytes.Buffer
}

func (e *ExecOpts) getPodContainerNamespace() string {
	if e.Container != "" {
		return fmt.Sprintf("%s/%s/%s", e.Container, e.Pod.Name, e.Pod.Namespace)
	}

	return fmt.Sprintf("%s/%s", e.Pod.Name, e.Pod.Namespace)
}

func (e *ExecOpts) toPodExecOptions() *corev1.PodExecOptions {
	return &corev1.PodExecOptions{
		Container: e.Container,
		Command:   e.Command,
		Stdin:     e.Stdin != nil,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}
}

func (e *ExecOpts) toStreamOptions() remotecommand.StreamOptions {
	so := remotecommand.StreamOptions{}

	so.Stdout = e.Stdout
	so.Stderr = e.Stderr

	if e.Stdin != nil {
		so.Stdin = e.Stdin
	}

	if so.Stdout == nil {
		e.stdoutBuf = bytes.NewBuffer([]byte{})
		so.Stdout = e.stdoutBuf
	}

	if so.Stderr == nil {
		e.stderrBuf = bytes.NewBuffer([]byte{})
		so.Stderr = e.stderrBuf
	}

	return so
}

// This was adapted from: https://discuss.kubernetes.io/t/go-client-exec-ing-a-shel-command-in-pod/5354/5
func ExecuteRemoteCommand(opts *ExecOpts) error {
	// TODO: Figure out if this can use framework.ClientSet.
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		return err
	}

	coreClient, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return err
	}

	podOpts := opts.toPodExecOptions()
	streamOpts := opts.toStreamOptions()

	request := coreClient.CoreV1().RESTClient().
		Post().
		Namespace(opts.Pod.Namespace).
		Resource("pods").
		Name(opts.Pod.Name).
		SubResource("exec").
		VersionedParams(podOpts, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restCfg, "POST", request.URL())
	if err != nil {
		return err
	}

	if err := exec.Stream(streamOpts); err != nil {
		return newExecError(opts, err)
	}

	return nil
}

func PodHasCommand(pod *corev1.Pod, command string) (bool, error) {
	// See: https://stackoverflow.com/questions/2869100/how-to-find-directory-of-some-command
	script := fmt.Sprintf("#!/bin/bash\ntype -a %s", command)

	opts := ExecOpts{
		Command: []string{"/bin/bash", "-c", script},
		Pod:     pod,
	}

	err := ExecuteRemoteCommand(&opts)

	if err == nil {
		return true, nil
	}

	// This feels like the wrong way to do this. Does Bash return a different
	// error code if it can't find the command?
	if strings.Contains(opts.stdoutBuf.String(), "not found") {
		return false, nil
	}

	return false, fmt.Errorf("could not determine if command %q was found: %w", command, err)
}
