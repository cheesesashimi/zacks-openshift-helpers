package main

import "encoding/json"

// TODO: Use containres/image code for this instead...
type skopeoClient struct {
	authfile string
}

type ImageInfo struct {
	Labels map[string]string
}

func newAuthedSkopeoClient(authfile string) *skopeoClient {
	return &skopeoClient{authfile: authfile}
}

func newSkopeoClient() *skopeoClient {
	return &skopeoClient{}
}

func (s *skopeoClient) Inspect(pullspec string) (*ImageInfo, error) {
	args := s.getSkopeoInspectPreamble()
	args = append(args, []string{"--no-tags", "docker://" + pullspec}...)
	outBuf, _, err := runCommandAndCollectOutput(args)
	if err != nil {
		return nil, err
	}

	ii := &ImageInfo{}
	if err := json.Unmarshal(outBuf, ii); err != nil {
		return nil, err
	}

	return ii, nil
}

func (s *skopeoClient) getSkopeoInspectPreamble() []string {
	if s.authfile == "" {
		return []string{"skopeo", "inspect"}
	}

	return []string{"skopeo", "inspect", "--authfile", s.authfile}
}
