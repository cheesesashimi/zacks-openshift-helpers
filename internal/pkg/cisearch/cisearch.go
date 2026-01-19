package cisearch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	jiraBaseClient "github.com/andygrunwald/go-jira"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SearchType string

const (
	BugIssueJunitSearchType SearchType = "bug+issue+junit"
	BugJunitSearchType      SearchType = "bug+junit"
	BugIssueSearchType      SearchType = "bug+issue"
	IssueSearchType         SearchType = "issue"
	BugSearchType           SearchType = "bug"
	JunitSearchType         SearchType = "junit"
	BuildLogSearchType      SearchType = "build-log"
	AllSearchType           SearchType = "all"
)

// Copied from ci-search repo and modified slightly.
type Match struct {
	Name         string                `json:"name,omitempty"`
	LastModified metav1.Time           `json:"lastModified"`
	FileType     string                `json:"filename"`
	Context      []string              `json:"context,omitempty"`
	MoreLines    int                   `json:"moreLines,omitempty"`
	URL          string                `json:"url,omitempty"`
	Issue        *jiraBaseClient.Issue `json:"issues,omitempty"`
}

type Query struct {
	Search      string
	MaxAge      string
	Context     int
	Type        SearchType
	Name        string
	ExcludeName string
	MaxMatches  int
	MaxBytes    int
	GroupBy     string
}

func (q Query) setDefaults() Query {
	if q.MaxAge == "" {
		q.MaxAge = "48h"
	}

	if q.Context == 0 {
		q.Context = -1
	}

	if q.Type == "" {
		q.Type = BugIssueJunitSearchType
	}

	if q.MaxMatches == 0 {
		q.MaxMatches = 5
	}

	if q.MaxBytes == 0 {
		q.MaxBytes = 20971520
	}

	if q.GroupBy == "" {
		q.GroupBy = "job"
	}

	return q
}

func Execute(q Query) (map[string]map[string][]*Match, error) {
	qu := q.setDefaults()

	u := url.URL{
		Scheme: "https",
		Host:   "search.dptools.openshift.org",
		Path:   "/search",
	}

	uq := u.Query()
	uq.Set("search", qu.Search)
	uq.Set("maxAge", qu.MaxAge)
	uq.Set("context", fmt.Sprintf("%d", qu.Context))
	uq.Set("type", string(qu.Type))
	uq.Set("name", qu.Name)
	uq.Set("excludeName", qu.ExcludeName)
	uq.Set("maxMatches", fmt.Sprintf("%d", qu.MaxMatches))
	uq.Set("maxBytes", fmt.Sprintf("%d", qu.MaxBytes))
	uq.Set("groupBy", qu.GroupBy)

	u.RawQuery = uq.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("got HTTP 404 from %s", u.String())
	}

	defer resp.Body.Close()

	out := map[string]map[string][]*Match{}

	err = json.NewDecoder(resp.Body).Decode(&out)

	return out, err
}
