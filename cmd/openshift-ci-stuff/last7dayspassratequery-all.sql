WITH passfail AS (
    SELECT
        test_name,
        COUNT(CASE WHEN result = 'passed' THEN 1 END) AS passed_count,
        COUNT(CASE WHEN result = 'failed' THEN 1 END) AS failed_count,
        COUNT(*) AS total_count
    FROM junits
    WHERE
        (test_name LIKE '%ocb%' OR test_name LIKE '%layering%' OR test_name LIKE '%ocl%' AND test_name NOT LIKE '%OnCLayer%') AND result IN('passed', 'failed') AND DATE(started) BETWEEN date('now', '-7 day') AND date()
    GROUP BY test_name
)

SELECT
    pf.test_name AS 'Test Name',
    pf.passed_count AS 'Passed Count',
    pf.failed_count AS 'Failed Count',
    pf.total_count AS 'Total Count',
    (pf.passed_count * 1.0) / (pf.passed_count + pf.failed_count) AS pass_rate
FROM passfail pf
ORDER BY pass_rate DESC;
