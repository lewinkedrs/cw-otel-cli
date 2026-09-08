-- analyze.sql — analyze the metrics.csv exported by export.sh using DuckDB.
--
-- Run it with:
--   duckdb -c ".read analyze.sql"
--
-- The CSV is long-format: series,value,timestamp,metric
-- where `series` looks like "service.name=checkout" and `metric` is one of
-- 'request_rate' or 'error_rate'.

-- Load and normalize.
CREATE OR REPLACE TABLE m AS
  SELECT
    split_part(series, '=', 2)               AS service,
    CAST(value AS DOUBLE)                     AS value,
    to_timestamp(CAST(timestamp AS BIGINT))   AS ts,
    metric
  FROM read_csv_auto('metrics.csv', header = true);

-- Pivot to one row per service with request + error rate, and derive error %.
CREATE OR REPLACE TABLE svc AS
  SELECT
    service,
    MAX(value) FILTER (WHERE metric = 'request_rate')            AS req_per_s,
    COALESCE(MAX(value) FILTER (WHERE metric = 'error_rate'), 0) AS err_per_s
  FROM m
  GROUP BY service;

.print ''
.print '── Services ranked by error ratio ──────────────────────────'
SELECT
  service,
  round(req_per_s, 3)                                  AS req_per_s,
  round(err_per_s, 3)                                  AS err_per_s,
  round(100.0 * err_per_s / nullif(req_per_s, 0), 2)   AS error_pct
FROM svc
ORDER BY error_pct DESC NULLS LAST;

.print ''
.print '── Services above a 2% error budget ─────────────────────────'
SELECT
  service,
  round(100.0 * err_per_s / nullif(req_per_s, 0), 2)   AS error_pct
FROM svc
WHERE err_per_s / nullif(req_per_s, 0) > 0.02
ORDER BY error_pct DESC;

.print ''
.print '── Fleet summary ────────────────────────────────────────────'
SELECT
  count(*)                                                     AS services,
  round(sum(req_per_s), 3)                                     AS total_req_per_s,
  round(100.0 * sum(err_per_s) / nullif(sum(req_per_s), 0), 2) AS fleet_error_pct
FROM svc;
