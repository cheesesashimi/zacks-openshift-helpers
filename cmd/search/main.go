package main

import (
	"fmt"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/cisearch"
)

func main() {
	q := cisearch.Query{
		Search:      "could not detect AppliedFilesAndOS=True",
		MaxAge:      "168h",
		Name:        `^periodic.*4\.22.*`,
		ExcludeName: "^pull",
		MaxBytes:    41943040,
	}

	r, err := cisearch.Execute(q)
	if err != nil {
		panic(err)
	}

	for u := range r {
		if strings.Contains(u, "prow.ci") {
			fmt.Println(u)
		}
	}
}
