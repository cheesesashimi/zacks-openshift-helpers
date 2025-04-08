package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"

	junit "github.com/joshdk/go-junit"
)

type junitFile struct {
	jobName string
	jobID   string
	path    string
}

// Searches a given local path for junit files matching a certain name. Assumes
// that the path for each junit XML file is formatted like <job name>/<job
// id>/import-MCO.xml.
func findJunits(searchPath string) ([]junitFile, error) {
	junits := []junitFile{}

	err := filepath.Walk(searchPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}

		if info.Name() == "import-MCO.xml" {
			jobID := filepath.Base(filepath.Dir(path))
			jobName := filepath.Base(filepath.Dir(filepath.Dir(path)))
			junits = append(junits, junitFile{
				jobID:   jobID,
				jobName: jobName,
				path:    path,
			})
		}

		return nil
	})

	return junits, err
}

// Holds an aggregated result from each junit test.
type aggregatedResult struct {
	passed   int
	failed   int
	skipped  int
	errored  int
	total    int
	passRate float64
}

// Takes the junit statuses and returns an aggregated result.
func newAggregatedResult(statuses []junit.Status) aggregatedResult {
	ar := aggregatedResult{}

	for _, status := range statuses {
		if status == junit.StatusPassed {
			ar.passed++
		}

		if status == junit.StatusFailed {
			ar.failed++
		}

		if status == junit.StatusSkipped {
			ar.skipped++
		}

		if status == junit.StatusError {
			ar.errored++
		}

		ar.total++
	}

	ar.passRate = ar.getPassRate()

	return ar
}

// Computes the pass rate.
func (a aggregatedResult) getPassRate() float64 {
	return float64(float64(a.passed) / float64((a.passed + a.skipped + a.failed + a.errored)))
}

// Adds one aggregatedResult to another and returns the product without
// mutating either the source or given results.
func (a aggregatedResult) add(ar aggregatedResult) aggregatedResult {
	out := aggregatedResult{
		passed:  a.passed + ar.passed,
		failed:  a.failed + ar.failed,
		skipped: a.skipped + ar.skipped,
		errored: a.errored + ar.errored,
		total:   a.total + ar.total,
	}

	out.passRate = out.getPassRate()

	return out
}

// Holds all of the job test results.
type jobTestResults []jobTestResult

// Prepares a slice of string slices suitable for CSV or other tabular output.
func (j jobTestResults) aggregatedReportLines() [][]string {
	out := [][]string{
		[]string{"Test", "Pass", "Fail", "Skipped", "Error", "Total", "Pass Rate"},
	}

	for testName, aggResult := range j.aggregatedByName() {
		out = append(out, []string{
			testName,
			fmt.Sprintf("%d", aggResult.passed),
			fmt.Sprintf("%d", aggResult.failed),
			fmt.Sprintf("%d", aggResult.skipped),
			fmt.Sprintf("%d", aggResult.errored),
			fmt.Sprintf("%d", aggResult.total),
			fmt.Sprintf("%f", aggResult.passRate),
		})
	}

	return out
}

// Aggregates all of the job test results by test name.
func (j jobTestResults) aggregatedByName() map[string]aggregatedResult {
	testNames := map[string][]junit.Status{}

	for _, result := range j {
		for _, suite := range result.results {
			for _, test := range suite.Tests {
				_, ok := testNames[test.Name]
				if !ok {
					testNames[test.Name] = []junit.Status{test.Status}
				} else {
					testNames[test.Name] = append(testNames[test.Name], test.Status)
				}
			}
		}
	}

	out := map[string]aggregatedResult{}

	for testName, statuses := range testNames {
		out[testName] = newAggregatedResult(statuses)
	}

	return out
}

// Holds the job test result for each given junit.
type jobTestResult struct {
	jobName string
	jobID   string
	results []junit.Suite
	started time.Time
}

// Traverses the given directory and reads all junits it finds that match the
// pattern given in the findJunits function. TODO: Make the pattern more
// customizable.
func getJunitDataFromDisk(path string) (jobTestResults, error) {
	junitfiles, err := findJunits(path)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	results := jobTestResults{}

	for _, junitfile := range junitfiles {
		jtr, err := readJunitFromDisk(junitfile.path, junitfile.jobName, junitfile.jobID)
		if err != nil {
			fmt.Printf("could not parse %q because %q, ignoring...\n", junitfile.path, err)
			continue
		}

		results = append(results, jtr)
	}

	fmt.Println("Processed", len(junitfiles), "JUnit files. Took:", time.Since(start))

	return results, nil
}

// Reads the started.json file that's found in the artifact root to get the
// timestamp for when the job began.
func readStartedFileFromDisk(path string) (time.Time, error) {
	type in struct {
		Timestamp int64 `json:""`
	}

	inbytes, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}

	i := &in{}
	if err := json.Unmarshal(inbytes, i); err != nil {
		return time.Time{}, err
	}

	return time.Unix(i.Timestamp, 0).UTC(), nil
}

// Reads a given junit file into memory while also optionally getting the
// timestamp from the started.json file, if present.
func readJunitFromDisk(path, jobName, jobID string) (jobTestResult, error) {
	startedPath := filepath.Join(filepath.Dir(path), "started.json")
	timestamp, err := readStartedFileFromDisk(startedPath)
	if err != nil && !os.IsNotExist(err) {
		return jobTestResult{}, err
	}

	f, err := os.Open(path)
	defer f.Close()

	if err != nil {
		return jobTestResult{}, err
	}

	jtr, err := readerToJobTestResult(f, jobName, jobID)
	if err != nil {
		return jtr, err
	}

	jtr.started = timestamp

	return jtr, nil
}

// Accepts an io.Reader which allows the junit parser to get input from any
// source such as an HTTP body, a string reader, or even a file.
func readerToJobTestResult(r io.Reader, jobName, jobID string) (jobTestResult, error) {
	jtr := jobTestResult{}

	start := time.Now()

	suites, err := junit.IngestReader(r)
	if err != nil {
		return jtr, err
	}

	jtr.jobName = jobName
	jtr.jobID = jobID
	jtr.results = suites

	fmt.Println("Parsed junit after:", time.Since(start))

	return jtr, nil
}

// Retrieves all of the junits from prow and parses them without saving them to
// disk.
func getJunitDataFromProw() (jobTestResults, error) {
	jobHistories, err := getJobHistoryPages()
	if err != nil {
		return nil, err
	}

	results := jobTestResults{}

	junitCount := 0

	start := time.Now()

	artifactCookie, err := loadArtifactCookie()
	if err != nil {
		return nil, err
	}

	for _, jobhistories := range jobHistories {
		for _, item := range jobhistories {
			if item.Result == "PENDING" {
				continue
			}

			queryStart := time.Now()

			pjl, err := newProwjobLink(item.SpyglassLink)
			if err != nil {
				return nil, err
			}

			junitPath := pjl.ArtifactRoot() + filepath.Join("/artifacts", pjl.JobShortName(), "openshift-extended-test-longduration", "artifacts", "junit", "import-MCO.xml")

			req, err := getHTTPRequestWithCookie(junitPath, artifactCookie)
			if err != nil {
				return nil, err
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil, err
			}

			defer resp.Body.Close()

			jtr, err := readerToJobTestResult(resp.Body, pjl.JobName(), pjl.JobID())
			if err != nil {
				fmt.Printf("could not parse %q because %q, ignoring...\n", junitPath, err)
			}

			resp.Body.Close()

			fmt.Println("Extracted junit from", pjl.JobName(), pjl.JobID(), "Took:", time.Since(queryStart))
			jtr.started = item.Started
			results = append(results, jtr)

			junitCount++
		}
	}

	fmt.Println("Extracted", fmt.Sprintf("%d", junitCount), "junits, took:", time.Since(start))

	return results, nil
}

// Prints the junit results from a given file in an easy to read manner.
func printJunitResults(file string) error {
	suites, err := junit.IngestFile(file)
	if err != nil {
		return err
	}

	for _, suite := range suites {
		for _, test := range suite.Tests {
			if test.Status == junit.StatusPassed {
				color.Green(test.Name)
			}

			if test.Status == junit.StatusFailed {
				color.Red(test.Name)
			}

			if test.Status == junit.StatusSkipped || test.Status == junit.StatusError {
				color.Yellow(test.Name)
			}
		}
	}

	return nil
}
