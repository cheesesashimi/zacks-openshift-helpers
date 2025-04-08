SELECT DISTINCT job_name, job_run_id, DATE(started) AS daterun, CONCAT('https://qe-private-deck-ci.apps.ci.l2s4.p1.openshiftapps.com/view/gs/qe-private-deck/logs/', job_name, '/', job_run_id) AS url
FROM junits
WHERE daterun
BETWEEN '2025-04-11' AND '2025-04-14'
GROUP BY job_name, job_run_id
ORDER BY daterun DESC;
