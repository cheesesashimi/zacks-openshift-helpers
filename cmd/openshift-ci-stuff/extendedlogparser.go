package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Reads test names from a given file without sanitizing them in any way.
func readTestNamesUnfiltered() ([]string, error) {
	in, err := os.ReadFile("ocltestnames.txt")
	if err != nil {
		return nil, err
	}

	out := []string{}
	for _, line := range strings.Split(string(in), "\n") {
		if line == "" {
			continue
		}

		out = append(out, line)
	}

	return out, nil
}

// Reads test names from a given file, stripping off the leading characters
// from it for easier matching.
func readAndCleanTestNames() ([]string, error) {
	r, err := regexp.Compile(`^.*(MCO (ocb|(L|l)ayering).*$)`)
	if err != nil {
		return nil, err
	}

	lines, err := readTestNamesUnfiltered()
	if err != nil {
		return nil, err
	}

	out := []string{}

	for _, line := range lines {
		if line == "" {
			continue
		}

		matches := r.FindAllStringSubmatch(line, -1)
		if matches == nil || len(matches) == 0 {
			continue
		}

		out = append(out, matches[0][1])
	}

	return out, nil
}

// Reads an extended log file line-by-line searching for a started: / failed:
// prefix which has the same test name. Other test terminators such as passed
// or skipped will be ignored.
func parseExtendedLog(filename string) error {
	start := time.Now()

	nameMatcher := `MCO (ocb|(L|l)ayering)`

	startedLineRegex, err := regexp.Compile(`^started\:.*` + nameMatcher)
	if err != nil {
		return err
	}

	passedLineRegex, err := regexp.Compile(`^passed\:.*` + nameMatcher)
	if err != nil {
		return err
	}

	skippedLineRegex, err := regexp.Compile(`^skipped\:.*` + nameMatcher)
	if err != nil {
		return err
	}

	failedLineRegex, err := regexp.Compile(`^failed\:.*` + nameMatcher)
	if err != nil {
		return err
	}

	// Strips the author out of the line so that it can be more easily looked up.
	authorRegex, err := regexp.Compile(`Author\:.*\-\[`)
	if err != nil {
		return err
	}

	names, err := readAndCleanTestNames()
	if err != nil {
		return err
	}

	stripAuthor := func(line string) string {
		return authorRegex.ReplaceAllString(line, `[`)
	}

	isNameInLine := func(line string) (bool, string) {
		stripped := stripAuthor(line)

		for _, name := range names {
			if strings.Contains(stripped, name) {
				return true, name
			}
		}

		return false, ""
	}

	logBytes, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	// /Note: This isn't the most memory-efficient implementation since it reads
	// the entire file into memory. Very large log files (> 1 GB) will likely be
	// problematic unless this is modified to use a buffered reader approach
	// instead. However, this allows us to only read the file once and be able to
	// accurately find and handle the lines we're interested in.
	lines := strings.Split(string(logBytes), "\n")

	// Holds the start and end lines for a given stasrted: -> passed: / failed: /
	// skipped: pair along with its terminal state.
	type startAndEnd struct {
		start int
		end   int
		state string
	}

	tests := map[string]startAndEnd{}

	for i, line := range lines {
		if startedLineRegex.MatchString(line) {
			isInLine, name := isNameInLine(line)
			if isInLine {
				tests[name] = startAndEnd{
					start: i,
				}
			}
		}

		if passedLineRegex.MatchString(line) {
			isInLine, name := isNameInLine(line)
			if isInLine {
				pos := tests[name]
				tests[name] = startAndEnd{
					start: pos.start,
					end:   i + 1,
					state: "passed",
				}
			}
		}

		if skippedLineRegex.MatchString(line) {
			isInLine, name := isNameInLine(line)
			if isInLine {
				pos := tests[name]
				tests[name] = startAndEnd{
					start: pos.start,
					end:   i + 1,
					state: "skipped",
				}
			}
		}

		if failedLineRegex.MatchString(line) {
			isInLine, name := isNameInLine(line)
			if isInLine {
				pos := tests[name]
				tests[name] = startAndEnd{
					start: pos.start,
					end:   i + 1,
					state: "failed",
				}
			}
		}
	}

	// Write the failed tests we found into our workspace directory. This uses a
	// lightly sanitized form of the test name for the filename. This makes
	// keeping things separated much easier.
	for name, test := range tests {
		if test.state == "failed" {
			outfilename := makeFilenameSafe(name) + ".log"
			outfilename = filepath.Join(filepath.Dir(filename), outfilename)
			fmt.Println("Wrote", name, "to", outfilename)
			data := strings.Join(lines[test.start:test.end], "\n")
			err := os.WriteFile(outfilename, []byte(data), 0o755)
			if err != nil {
				return err
			}
		}
	}

	fmt.Println("Processed", filename, "in", time.Since(start))

	return nil
}

// From ChatGPT
// makeFilenameSafe ensures the string is safe to use as a filename
func makeFilenameSafe(input string) string {
	// Replace any invalid characters with an underscore
	// Invalid characters can include those commonly restricted in filenames.
	invalidChars := `[<>:"/\|?*]`
	re := regexp.MustCompile(invalidChars)

	// Replace each invalid character with an underscore
	safe := re.ReplaceAllString(input, "_")

	// Trim leading/trailing spaces
	safe = strings.TrimSpace(safe)

	// Ensure that the filename is not empty
	if len(safe) == 0 {
		safe = "untitled_file"
	}

	return safe
}
