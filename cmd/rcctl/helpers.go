package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
)

func doReleaseControllerOp(opFunc func(context.Context, *releasecontroller.ReleaseController) (interface{}, error)) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rc, err := getReleaseController()
	if err != nil {
		return err
	}

	out, err := opFunc(ctx, rc)
	if err != nil {
		return err
	}

	return printJSON(out)
}

func getReleaseController() (*releasecontroller.ReleaseController, error) {
	allRCs := releasecontroller.All()
	for _, rc := range allRCs {
		if controller == rc.Host() {
			return rc, nil
		}
	}

	return nil, fmt.Errorf("invalid release controller %q: %v", controller, allRCs)
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
