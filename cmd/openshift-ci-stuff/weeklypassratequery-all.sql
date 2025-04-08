WITH weekly_results AS (
    SELECT
        test_name,
        strftime('%Y-%W', started) AS week,
        COUNT(CASE WHEN result = 'failed' THEN 1 END) AS failed_count,
        COUNT(CASE WHEN result = 'passed' THEN 1 END) AS passed_count,
        COUNT(*) AS total_count
    FROM junits
    WHERE result IN('failed', 'passed')
    GROUP BY test_name, week
),
test_failures AS (
    SELECT
        test_name,
        COUNT(failed_count) total_failures
    FROM weekly_results
    WHERE (test_name LIKE '%ocb%' OR test_name LIKE '%layering%' AND test_name NOT LIKE '%OnCLayer%')
    GROUP BY test_name
    ORDER BY total_failures DESC
    LIMIT 10
)
SELECT
    w.test_name AS 'Test Name',
    w.week AS 'Week Number',
    w.passed_count AS 'Passed Count',
    w.failed_count AS 'Failed Count',
    w.total_count AS 'Total Count',
    (w.passed_count * 1.0) / (w.passed_count + w.failed_count) AS 'Pass Rate'
FROM weekly_results w
JOIN test_failures f ON w.test_name = f.test_name
ORDER BY w.test_name, w.week DESC;
