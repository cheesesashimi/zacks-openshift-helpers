package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Returns a list of the job names we're interested in.
func getJobNames() []string {
	return []string{
		"periodic-ci-openshift-openshift-tests-private-release-4.18-amd64-nightly-baremetalds-ipi-ovn-f7-longrun-tp-mco-p1",
		"periodic-ci-openshift-openshift-tests-private-release-4.18-amd64-nightly-baremetalds-ipi-ovn-f7-longrun-tp-mco-p2",
		"periodic-ci-openshift-openshift-tests-private-release-4.18-amd64-nightly-gcp-ipi-longduration-tp-mco-p1-f1",
		"periodic-ci-openshift-openshift-tests-private-release-4.18-amd64-nightly-gcp-ipi-longduration-tp-mco-p2-f1",
		"periodic-ci-openshift-openshift-tests-private-release-4.18-amd64-nightly-gcp-ipi-longduration-tp-mco-p3-f1",
		"periodic-ci-openshift-openshift-tests-private-release-4.18-amd64-nightly-vsphere-upi-zones-f7-longrun-mco-tp-p1",
		"periodic-ci-openshift-openshift-tests-private-release-4.18-amd64-nightly-vsphere-upi-zones-f7-longrun-mco-tp-p2",
		"periodic-ci-openshift-openshift-tests-private-release-4.18-amd64-nightly-vsphere-upi-zones-f7-longrun-mco-tp-p3",
		"periodic-ci-openshift-openshift-tests-private-release-4.18-arm64-nightly-aws-ipi-longrun-mco-tp-proxy-fips-p1-f1",
		"periodic-ci-openshift-openshift-tests-private-release-4.18-arm64-nightly-aws-ipi-longrun-mco-tp-proxy-fips-p2-f1",
		"periodic-ci-openshift-openshift-tests-private-release-4.18-arm64-nightly-aws-ipi-longrun-mco-tp-proxy-fips-p3-f1",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-amd64-nightly-baremetalds-ipi-ovn-f7-longrun-tp-mco-p1",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-amd64-nightly-baremetalds-ipi-ovn-f7-longrun-tp-mco-p2",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-amd64-nightly-baremetalds-ipi-ovn-f7-longrun-tp-mco-p3",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-amd64-nightly-gcp-ipi-longduration-tp-mco-p1-f9",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-amd64-nightly-gcp-ipi-longduration-tp-mco-p2-f9",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-amd64-nightly-gcp-ipi-longduration-tp-mco-p2-f9",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-amd64-nightly-gcp-ipi-longduration-tp-mco-p3-f9",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-amd64-nightly-vsphere-upi-zones-f7-longrun-mco-tp-p1",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-amd64-nightly-vsphere-upi-zones-f7-longrun-mco-tp-p2",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-amd64-nightly-vsphere-upi-zones-f7-longrun-mco-tp-p3",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-arm64-nightly-aws-ipi-longrun-mco-tp-proxy-fips-p1-f28",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-arm64-nightly-aws-ipi-longrun-mco-tp-proxy-fips-p2-f28",
		"periodic-ci-openshift-openshift-tests-private-release-4.19-arm64-nightly-aws-ipi-longrun-mco-tp-proxy-fips-p3-f28",
	}
}

// Attaches the Prow hostname and path to the job names.
func getJobHistoryPageURLs() []string {
	urlPrefix := "https://qe-private-deck-ci.apps.ci.l2s4.p1.openshiftapps.com/job-history/gs/qe-private-deck/logs/"

	names := getJobNames()
	out := make([]string, len(names))

	for i, name := range names {
		out[i] = urlPrefix + name
	}

	return out
}

// Iterates through the job history pages and extracts the JSON disguised as a
// Javascript payload for each one.
func getJobHistoryPages() (map[string][]jobHistory, error) {
	out := map[string][]jobHistory{}

	jobHistoryCookie, err := loadJobHistoryCookie()
	if err != nil {
		return nil, err
	}

	for _, jobHistoryPageURL := range getJobHistoryPageURLs() {
		jh, err := getJobHistory(jobHistoryPageURL, jobHistoryCookie)
		if err != nil {
			return nil, err
		}

		fmt.Println("Extracted job history from", jobHistoryPageURL)

		out[jobHistoryPageURL] = jh
	}

	return out, nil
}

func getHTTPRequestWithCookie(url, cookie string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	cookies, err := http.ParseCookie(cookie)
	if err != nil {
		return nil, err
	}

	for _, c := range cookies {
		req.AddCookie(c)
	}

	return req, nil
}

// Makes the HTTP request for each job history page.
func getJobHistory(url, cookie string) ([]jobHistory, error) {
	req, err := getHTTPRequestWithCookie(url, cookie)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return getJobHistoryFromHTMLPage(resp.Body)
}

type jobHistory struct {
	SpyglassLink string    `json:""`
	ID           string    `json:""`
	Started      time.Time `json:"-"`
	Duration     int       `json:""`
	Result       string    `json:""`
}

// Because we want to convert the timestamp into a time.Time object, we must
// implement our own unmarshaler that first deserializes all of the easy parts,
// then attempts to parse the timestamp into a time.Time object. .
func (j *jobHistory) UnmarshalJSON(data []byte) error {
	type local struct {
		SpyglassLink string `json:""`
		ID           string `json:""`
		Started      string `json:""`
		Duration     int    `json:""`
		Result       string `json:""`
	}

	l := &local{}

	if err := json.Unmarshal(data, l); err != nil {
		return err
	}

	// TODO: Golang probably has this built in.
	layout := "2006-01-02T15:04:05Z"

	parsedTime, err := time.Parse(layout, l.Started)
	if err != nil {
		return fmt.Errorf("could not parse time: %w", err)
	}

	*j = jobHistory{
		SpyglassLink: l.SpyglassLink,
		ID:           l.ID,
		Started:      parsedTime,
		Duration:     l.Duration,
		Result:       l.Result,
	}

	return nil
}

// There doesn't seem to be an API endpoint for getting job history from Prow.
// Thankfully, how they inject the data into the page makes it easy to traverse
// the DOM and get it.
func getJobHistoryFromHTMLPage(r io.Reader) ([]jobHistory, error) {
	script, err := getScriptJSONFromHTML(r)
	if err != nil {
		return nil, err
	}

	// Strip the JavaScript away from the JSON payload so we can serialize it.
	script = strings.ReplaceAll(script, "var allBuilds = ", "")
	script = strings.TrimRight(script, ";\n")
	script = strings.TrimSpace(script)

	out := []jobHistory{}
	if err := json.Unmarshal([]byte(script), &out); err != nil {
		return nil, err
	}

	return out, nil
}

// Parses the DOM from the HTML file and extracts the contents of the first
// script tag it finds that contains the data I need.
func getScriptJSONFromHTML(r io.Reader) (string, error) {
	getChildNodeContent := func(n *html.Node) string {
		for child := range n.ChildNodes() {
			return child.Data
		}

		return ""
	}

	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}

	for n := range doc.Descendants() {
		if n.Type == html.ElementNode && n.Data == "script" && strings.Contains(getChildNodeContent(n), "var allBuilds") {
			for child := range n.ChildNodes() {
				return child.Data, nil
			}
		}
	}

	return "", fmt.Errorf("not found")
}
