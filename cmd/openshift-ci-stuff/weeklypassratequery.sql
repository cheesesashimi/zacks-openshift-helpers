WITH weekly_results AS (
    SELECT
        test_name,
        strftime('%Y-%W', started) AS week,
        COUNT(CASE WHEN result = 'failed' THEN 1 END) AS failed_count,
        COUNT(CASE WHEN result = 'passed' THEN 1 END) AS passed_count,
        COUNT(CASE WHEN result = 'skipped' THEN 1 END) AS skipped_count,
        COUNT(CASE WHEN result = 'errored' THEN 1 END) AS errored_count,
        COUNT(*) AS total_count
    FROM junits
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
    w.test_name,
    w.week,
    w.passed_count,
    w.failed_count,
    w.skipped_count,
    w.errored_count,
    w.total_count,
    (w.passed_count * 1.0) / (w.passed_count + w.failed_count + w.skipped_count + w.errored_count) AS pass_rate
FROM weekly_results w
JOIN test_failures f ON w.test_name = f.test_name
ORDER BY w.test_name, w.week DESC;
