package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

// Hard-coded workspace root.
var workspaceRoot string = "/home/zzlotnik/Scratchspace/ocl-v1-test-analysis"

func main() {
	fmt.Println("I'm purposely left empty.")
}

// Some of the test output found in the junits refers to an additional
// must-gather. This reads the junit file from disk and searches for the URL,
// if it exists. Some junits have more than one must-gather, so we are careful
// to dedupe them.
func getAdditionalMustGathersFromJunitFile(path, jobName, jobID string) ([]string, error) {
	jtr, err := readJunitFromDisk(path, jobName, jobID)
	if err != nil {
		return nil, err
	}

	addlMustGathers := map[string]struct{}{}

	for _, suite := range jtr.results {
		for _, test := range suite.Tests {
			if strings.Contains(test.SystemOut, "Creating must-gather file") {
				for _, line := range strings.Split(test.SystemOut, "\n") {
					if strings.Contains(line, "Creating must-gather file") {
						split := strings.Split(line, " ")
						mustGather := split[len(split)-1]
						abbrevJobName := getAbbrevJobName(jobName)
						url := fmt.Sprintf("https://gcsweb-qe-private-deck-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/qe-private-deck/logs/%s/%s/artifacts/%s/openshift-extended-test-longduration/artifacts/must-gather/%s", jobName, jobID, abbrevJobName, mustGather)
						addlMustGathers[url] = struct{}{}
					}
				}
			}
		}
	}

	out := []string{}
	for url := range addlMustGathers {
		out = append(out, url)
	}

	return out, nil
}

// Downloads must-gathers, junits, extended logs, etc. for each job found in
// the job history pages.
func downloadStuffFromJobs() error {
	jobHistories, err := getJobHistoryPages()
	if err != nil {
		return err
	}

	artifactCookie, err := loadArtifactCookie()
	if err != nil {
		return err
	}

	for _, jobhistories := range jobHistories {
		for _, item := range jobhistories {
			if item.Result == "PENDING" {
				fmt.Println("Skipping pending job")
				continue
			}

			pjl, err := newProwjobLink(item.SpyglassLink)
			if err != nil {
				return err
			}

			testPath := filepath.Join(workspaceRoot, pjl.JobName(), pjl.JobID())
			if err := os.MkdirAll(testPath, 0o755); err != nil {
				return err
			}

			urlToLocalFilename := func(url string) string {
				return filepath.Join(testPath, filepath.Base(url))
			}

			started := pjl.ArtifactRoot() + filepath.Join("/started.json")
			if err := downloadFile(started, urlToLocalFilename(started), artifactCookie); err != nil {
				return err
			}

			mustgather := pjl.MustGather()
			if err := downloadFile(mustgather, urlToLocalFilename(mustgather), artifactCookie); err != nil {
				return err
			}

			qeTestReport := pjl.ArtifactRoot() + filepath.Join("/artifacts", pjl.JobShortName(), "openshift-extended-test-longduration", "artifacts", "extended.log")
			if err := downloadFile(qeTestReport, urlToLocalFilename(qeTestReport), artifactCookie); err != nil {
				return err
			}

			junitPath := pjl.ArtifactRoot() + filepath.Join("/artifacts", pjl.JobShortName(), "openshift-extended-test-longduration", "artifacts", "junit", "import-MCO.xml")

			localJunitPath := urlToLocalFilename(junitPath)
			if err := downloadFile(junitPath, localJunitPath, artifactCookie); err != nil {
				return err
			}

			addlMustGathers, err := getAdditionalMustGathersFromJunitFile(localJunitPath, pjl.JobName(), pjl.JobID())
			if err != nil {
				fmt.Printf("could not get additional must gathers because %q, skipping\n", err)
				continue
			}

			if len(addlMustGathers) == 0 {
				fmt.Println("No additional must-gathers found, moving on")
				continue
			}

			for _, addlMustGather := range addlMustGathers {
				if err := downloadFile(addlMustGather, urlToLocalFilename(addlMustGather), artifactCookie); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Performs the download of a URL to a given location on disk.
func downloadFile(url, filepath, cookie string) error {
	fmt.Println("Downloading", url)

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	req, err := getHTTPRequestWithCookie(url, cookie)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("got HTTP %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("Downloaded %s to %s\n", url, filepath)

	return nil
}

// Because the QE Prow instance is behind an authentication portal, we need to
// provide a cookie for both the job history pages as well as the artifact
// pages. These load the cookies from a file on disk.
func loadJobHistoryCookie() (string, error) {
	return loadCookie("jobhistorycookie.txt")
}

func loadArtifactCookie() (string, error) {
	return loadCookie("artifactcookie.txt")
}

func loadCookie(name string) (string, error) {
	c, err := os.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("could not load cookie from file: %w", err)
	}

	return string(c), nil
}

// Reads the test results from disk and prints out a colorized aggregated pass
// rate report.
func getTestResults() error {
	results, err := getJunitDataFromDisk(workspaceRoot)
	if err != nil {
		return err
	}

	total := 0
	for testName, aggResult := range results.aggregatedByName() {
		if !isMatch(testName) {
			continue
		}

		fmt.Println(testName)

		status := fmt.Sprintf("\tPass: %d Fail: %d Skipped: %d Error: %d Pass Rate: %.2f%%", aggResult.passed, aggResult.failed, aggResult.skipped, aggResult.errored, aggResult.passRate*100)

		if aggResult.passRate == 1.0 {
			color.Green(status)
		}

		if aggResult.passRate >= 0.95 && aggResult.passRate < 1.0 {
			color.Yellow(status)
		}

		if aggResult.passRate < 0.95 {
			color.Red(status)
		}

		total++
	}

	fmt.Println("Total:", total)

	return nil
}

// Filters the testnames to only the ones we're interested in.
func isMatch(testname string) bool {
	lowered := strings.ToLower(testname)

	if strings.Contains(lowered, "ocb") || strings.Contains(lowered, "layering") && !strings.Contains(lowered, "onclayer") {
		return true
	}

	return false
}

// Reads the junits from disk and produces a CSV version of the pass-rate report.
func toCSV() error {
	results, err := getJunitDataFromDisk(workspaceRoot)
	if err != nil {
		return err
	}

	file, err := os.Create("junits.csv")
	if err != nil {
		return err
	}

	defer file.Close()

	csvwriter := csv.NewWriter(file)

	reportLines := results.aggregatedReportLines()

	outLines := [][]string{reportLines[0]}

	for _, line := range reportLines {
		if isMatch(line[0]) {
			outLines = append(outLines, line)
		}
	}

	return csvwriter.WriteAll(outLines)
}
