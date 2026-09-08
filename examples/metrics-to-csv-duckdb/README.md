# Metrics to CSV → analyze with DuckDB

Export a per-service metric snapshot from CloudWatch to CSV with `cw-otel-cli`,
then slice it with SQL in [DuckDB](https://duckdb.org) — no database to stand
up, just a single binary reading a file.

## Why this needs `cw-otel-cli`

The metrics live behind CloudWatch's SigV4-signed PromQL API, which has **no
`aws-cli` equivalent**. `cw-otel-cli` emits CSV directly (`-o csv`), so getting
metrics into a tool like DuckDB, a spreadsheet, or a notebook is a one-liner
instead of hand-rolled SigV4 + JSON wrangling.

## Files

| File | Purpose |
|------|---------|
| [`export.sh`](./export.sh) | Runs the queries and writes a tidy long-format `metrics.csv`. |
| [`analyze.sql`](./analyze.sql) | DuckDB SQL that ranks services, flags error-budget breaches, and summarizes the fleet. |

## Run it

```bash
export AWS_REGION=us-east-1
./export.sh                       # writes metrics.csv
duckdb -c ".read analyze.sql"     # prints the analysis
```

Install DuckDB first if you don't have it: [duckdb.org/docs/installation](https://duckdb.org/docs/installation).

## How it works

`export.sh` runs two instant queries — per-service request rate and error rate —
and tags each with a `metric` column, producing a long-format CSV:

```
series,value,timestamp,metric
service.name=checkout,120.5,1757338800,request_rate
service.name=checkout,10.1,1757338800,error_rate
...
```

`analyze.sql` then loads it with `read_csv_auto`, splits the service name out of
the `series` column, pivots request/error rate into columns, and computes an
error percentage — ranking the worst services and flagging any above a 2% error
budget.

### Sample analysis output

```
── Services ranked by error ratio ──────────────────────────
┌──────────┬───────────┬───────────┬───────────┐
│ service  │ req_per_s │ err_per_s │ error_pct │
├──────────┼───────────┼───────────┼───────────┤
│ checkout │     120.5 │      10.1 │      8.38 │
│ payments │      88.0 │       1.8 │      2.05 │
│ search   │     210.3 │       0.6 │      0.29 │
└──────────┴───────────┴───────────┴───────────┘
```

## Adapt it to your metrics

Override the defaults via environment variables:

| Variable | Default | Meaning |
|----------|---------|---------|
| `REQ_METRIC` | `http_requests_total` | request counter with an outcome label |
| `SVC_LABEL` | `@resource.service.name` | label to group services by |
| `WINDOW` | `5m` | `rate()` look-back window |
| `OUT` | `metrics.csv` | output file |

CloudWatch requires an exact metric name in the selector
(`{__name__="my_metric", ...}`), not a bare label selector.

## Time series instead of a snapshot

This example exports a point-in-time snapshot (one value per service). The
`range` command has no CSV output, but you can bridge its JSON with `jq`:

```bash
cwpromql range 'sum by ("@resource.service.name")(rate({__name__="http_requests_total"}[5m]))' \
  --since 24h --step 5m -o json \
  | jq -r '.[] | .metric["@resource.service.name"] as $s
           | .values[] | [$s, .Time, .Value] | @csv' \
  > timeseries.csv
```

Then point DuckDB at `timeseries.csv` for windowed/trend analysis.
