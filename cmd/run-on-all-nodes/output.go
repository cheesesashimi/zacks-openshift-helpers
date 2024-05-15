package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type output struct {
	RemoteCommand string `json:"remoteCommand"`
	LocalCommand  string `json:"localCommand"`
	node          *corev1.Node
	stdout        *bytes.Buffer
	stderr        *bytes.Buffer
	err           error
}

func (o output) String() string {
	out := &strings.Builder{}
	fmt.Fprintf(out, "[%s - %v]:\n", o.node.Name, getNodeRoles(o.node))
	fmt.Fprintf(out, "$ %s\n", o.RemoteCommand)
	fmt.Fprintln(out, o.stdout.String())
	fmt.Fprintln(out, o.stderr.String())

	if o.err != nil {
		fmt.Fprintln(out, "Full invocation:", o.LocalCommand)
		fmt.Fprintln(out, "Error:", o.err)
	}

	return out.String()
}

func (o output) MarshalJSON() ([]byte, error) {
	type out struct {
		LocalCommand  string   `json:"localCommand"`
		RemoteCommand string   `json:"remoteCommand"`
		NodeRoles     []string `json:"nodeRoles"`
		Node          string   `json:"node"`
		Stdout        string   `json:"stdout"`
		Stderr        string   `json:"stderr"`
	}

	return json.Marshal(out{
		LocalCommand:  o.LocalCommand,
		RemoteCommand: o.RemoteCommand,
		Node:          o.node.Name,
		Stdout:        o.stdout.String(),
		Stderr:        o.stderr.String(),
		NodeRoles:     getNodeRoles(o.node),
	})
}
