# Incident report

A single script that runs several `cw-otel-cli` queries and prints one
consolidated health report for a service fleet — the kind of artifact you keep
in a repo and run when you get paged, instead of retyping individual queries.

## Why this needs `cw-otel-cli`

Every line of the report is a PromQL query against CloudWatch's SigV4-signed
PromQL API, which has **no `aws-cli` equivalent**. Composing them into one
report is just shell + `jq` on top of the CLI's JSON/CSV output — no SDK, no
signing code.

## Files

| File | Purpose |
|------|---------|
| [`report.sh`](./report.sh) | Runs the queries and prints the report. |

## What it reports

1. **Fleet-wide error ratio** — errors ÷ total requests over the window.
2. **Request throughput** — total req/s.
3. **Top-N services by error ratio** — the worst offenders, ranked.
4. **A verdict** — HEALTHY / DEGRADED / UNKNOWN based on a threshold.

It **degrades gracefully**: a query that returns no data shows `n/a` rather than
aborting the report.

## Run it

```bash
export AWS_REGION=us-east-1
./report.sh
```

Everything else has a default you can override via environment variables:

| Variable | Default | Meaning |
|----------|---------|---------|
| `REQ_METRIC` | `http_requests_total` | request counter with an outcome label |
| `ERROR_LABEL` | `outcome="error"` | selector marking error requests |
| `SVC_LABEL` | `@resource.service.name` | label to group services by |
| `WINDOW` | `5m` | `rate()` look-back window |
| `TOPN` | `5` | how many worst services to list |
| `WARN_ERROR_RATIO` | `0.02` | error ratio that flips the verdict to DEGRADED |

Adapt `REQ_METRIC` / `ERROR_LABEL` / `SVC_LABEL` to your service's metric names.
CloudWatch requires an exact metric name in the selector
(`{__name__="my_metric", ...}`), not a bare label selector.

## Sample output

```
────────────────────────────────────────────────────────────
 Service health report
 region:  us-east-1
 window:  last 5m     generated: 2026-09-08T13:40:00Z
────────────────────────────────────────────────────────────

Error ratio (fleet):   3.10%
Request rate:          142.500 req/s

Top 5 services by error ratio:
  checkout                                 8.40%
  payments                                 2.10%
  search                                   0.30%
  catalog                                  0.05%
  cart                                     0.00%

Verdict: DEGRADED — fleet error ratio 3.10% exceeds 2.00%.
────────────────────────────────────────────────────────────
```

## Requirements

`jq` and `awk` (both standard on macOS and Linux).
