package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "embed"

	_ "github.com/mattn/go-sqlite3"
)

func createSqliteDatabaseFromCISystem() error {
	jtr, err := getJunitDataFromProw()
	if err != nil {
		return fmt.Errorf("could not get job test results from prow: %w", err)
	}

	if err := importIntoSqlite(jtr); err != nil {
		return fmt.Errorf("cannot import into sqlite database: %w", err)
	}

	return nil
}

// Takes the given jobTestResults and imports them into a simple sqlite3
// database.
func importIntoSqlite(results jobTestResults) error {
	start := time.Now()

	db, err := sql.Open("sqlite3", "./junits.db")
	if err != nil {
		return err
	}
	defer db.Close()

	sqlStmt := `CREATE TABLE IF NOT EXISTS junits (job_name, job_run_id, result, started, test_name, system_out)`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	queryString := "INSERT INTO junits (job_name, job_run_id, result, started, test_name, system_out) values(?, ?, ?, ?, ?, ?)"
	stmt, err := tx.Prepare(queryString)
	if err != nil {
		return err
	}
	defer stmt.Close()

	rowCount := 0

	for _, result := range results {
		for _, suite := range result.results {
			for _, test := range suite.Tests {
				_, err := stmt.Exec(result.jobName, result.jobID, test.Status, result.started, test.Name, test.SystemOut)
				if err != nil {
					return err
				}
				rowCount++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	fmt.Println("Inserted", fmt.Sprintf("%d", rowCount), "rows, took:", time.Since(start))

	return nil
}

// Holds the start and end date for a given week.
type week struct {
	start time.Time
	end   time.Time
}

//go:embed weeklypassratequery.sql
var weeklyPassRateQuery string

// Represents a passrate row given the sql query found in the above embedded file.
type passRateRow struct {
	testName string
	aggregatedResult
	week
}

// Executes the embedded query in the sqlite3 database and parses the results
// by name.
func getWeeklyPassRatesFromSqlite() (map[string][]passRateRow, map[string]aggregatedResult, error) {
	db, err := sql.Open("sqlite3", "./junits.db")
	if err != nil {
		return nil, nil, err
	}

	defer db.Close()

	rows, err := db.Query(weeklyPassRateQuery)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	prrs := []passRateRow{}

	start := time.Now()

	for rows.Next() {
		prr := passRateRow{}
		var weekStr string
		err = rows.Scan(&prr.testName, &weekStr, &prr.passed, &prr.failed, &prr.skipped, &prr.errored, &prr.total, &prr.passRate)
		if err != nil {
			return nil, nil, err
		}

		week, err := getWeekStartEndFromString(weekStr)
		if err != nil {
			return nil, nil, err
		}

		prr.week = week

		prrs = append(prrs, prr)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	fmt.Println("Retrieved data from database, took:", time.Since(start))

	start = time.Now()
	byName, totals := processPassRateRows(prrs)
	fmt.Println("Aggregated data from database, took:", time.Since(start))
	return byName, totals, nil
}

// Aggregates the returned pass rate rows by name and gets totals for each row.
func processPassRateRows(prrs []passRateRow) (map[string][]passRateRow, map[string]aggregatedResult) {
	byName := map[string][]passRateRow{}
	totals := map[string]aggregatedResult{}

	for _, prr := range prrs {
		if _, ok := byName[prr.testName]; !ok {
			byName[prr.testName] = []passRateRow{}
			totals[prr.testName] = aggregatedResult{}
		}

		byName[prr.testName] = append(byName[prr.testName], prr)
		totals[prr.testName] = totals[prr.testName].add(prr.aggregatedResult)
	}

	for testname, prr := range byName {
		sort.Slice(prr, func(i, j int) bool {
			return prr[i].start.Before(prr[j].start) && prr[i].end.Before(prr[j].end)
		})

		byName[testname] = prr
	}

	return byName, totals
}

// Orders test names in descending order by the number of failures.
func getTestnamesOrderedByFailureRate(totals map[string]aggregatedResult) []string {
	type aggwithname struct {
		name string
		aggregatedResult
	}

	aggs := []aggwithname{}

	for testname, ar := range totals {
		aggs = append(aggs, aggwithname{name: testname, aggregatedResult: ar})
	}

	sort.Slice(aggs, func(i, j int) bool {
		return aggs[i].failed > aggs[j].failed
	})

	names := []string{}
	for _, agg := range aggs {
		names = append(names, agg.name)
	}

	return names
}

func getWeekStartEndFromString(in string) (week, error) {
	split := strings.Split(in, "-")
	year, err := strconv.Atoi(split[0])
	if err != nil {
		return week{}, err
	}

	weekNum, err := strconv.Atoi(split[1])
	if err != nil {
		return week{}, err
	}

	start, end, err := getWeekStartEnd(year, weekNum)
	if err != nil {
		return week{}, err
	}

	return week{
		start: start,
		end:   end,
	}, nil
}

// Retrieves all of the data from the Prow CI server, parses it, and creates
// the database. Then runs the embedded query on it to get the weekly pass
// rate.
func refreshDataAndProduceWeeklyPassRateCSVReport(filename string) error {
	if err := createSqliteDatabaseFromCISystem(); err != nil {
		return err
	}

	if err := produceWeeklyPassRateCSVReport(filename); err != nil {
		return err
	}

	return nil
}

// Produces a weekly pass rate CSV report suitable for uploading to Google Drive.
func produceWeeklyPassRateCSVReport(filename string) error {
	byName, totals, err := getWeeklyPassRatesFromSqlite()
	if err != nil {
		return err
	}

	testnames := getTestnamesOrderedByFailureRate(totals)

	start := time.Now()

	header := []string{"Test Name"}

	for _, prr := range byName[testnames[0]] {
		weekrange := fmt.Sprintf("%s - %s", prr.start.Format("2006-01-02"), prr.end.Format("2006-01-02"))
		header = append(header, weekrange)
	}

	header = append(header, []string{"Total Passed", "Total Failed", "Total Skipped", "Total Errored", "Total Runs", "Overall Pass Rate"}...)

	lines := [][]string{header}

	for _, testname := range testnames {
		line := []string{testname}

		for _, prr := range byName[testname] {
			line = append(line, fmt.Sprintf("%f", prr.passRate))
		}

		line = append(line, fmt.Sprintf("%d", totals[testname].passed))
		line = append(line, fmt.Sprintf("%d", totals[testname].failed))
		line = append(line, fmt.Sprintf("%d", totals[testname].skipped))
		line = append(line, fmt.Sprintf("%d", totals[testname].errored))
		line = append(line, fmt.Sprintf("%d", totals[testname].total))
		line = append(line, fmt.Sprintf("%f", totals[testname].passRate))

		lines = append(lines, line)
	}

	f, err := os.Create(filename)
	defer f.Close()
	if err != nil {
		return err
	}

	csvwriter := csv.NewWriter(f)

	if err := csvwriter.WriteAll(lines); err != nil {
		return err
	}

	fmt.Println("Wrote CSV report to file", filename, "Took:", time.Since(start))

	return nil
}

// Gets the dates bounding a given a week number within a given year.
// From ChatGPT
func getWeekStartEnd(year int, week int) (time.Time, time.Time, error) {
	// Start with the first day of the year (January 1st)
	startOfYear := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)

	// Adjust to the first day of the first week (Monday of the first week of the year)
	// ISO weeks start on Monday, so we calculate the difference between the start of the year and the first Monday
	weekday := startOfYear.Weekday()
	daysToAdd := (7 - int(weekday)) % 7
	firstMonday := startOfYear.Add(time.Duration(daysToAdd) * 24 * time.Hour)

	// Now that we have the first Monday, we can calculate the start of the given week number
	startOfWeek := firstMonday.Add(time.Duration(week-1) * 7 * 24 * time.Hour)

	// The end of the week is 6 days after the start of the week
	endOfWeek := startOfWeek.Add(6 * 24 * time.Hour)

	return startOfWeek, endOfWeek, nil
}
