package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
)

func doReleaseControllerOp(opFunc func(releasecontroller.ReleaseController) (interface{}, error)) error {
	rc, err := getReleaseController()
	if err != nil {
		return err
	}

	out, err := opFunc(rc)
	if err != nil {
		return err
	}

	return printJSON(out)
}

func getReleaseController() (releasecontroller.ReleaseController, error) {
	allRCs := releasecontroller.All()
	for _, rc := range allRCs {
		if controller == string(rc) {
			return rc, nil
		}
	}

	return "", fmt.Errorf("invalid release controller %q: %v", controller, allRCs)
}

func printJSON(obj interface{}) error {
	if b, ok := obj.([]byte); ok {
		outBuf := bytes.NewBuffer([]byte{})
		if err := json.Indent(outBuf, b, "", "    "); err != nil {
			return err
		}

		_, err := os.Stdout.Write(outBuf.Bytes())
		return err
	}

	jsonb, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(jsonb)
	return err
}
