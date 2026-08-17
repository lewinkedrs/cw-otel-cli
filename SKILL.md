# SKILL: cwpromql — query CloudWatch OTLP metrics (PromQL)

> Agent guide for the `cwpromql` CLI shipped by this repo
> (`github.com/lewinkedrs/cw-otel-cli`). Point coding agents / assistants at this
> file so they use the tool correctly.

Use **`cwpromql`** whenever the user wants to **query metrics that live in
CloudWatch's OTLP / PromQL store** — OpenTelemetry metrics ingested via the
CloudWatch metrics endpoint. Trigger on: "query CloudWatch metrics", "PromQL",
"OTLP / OTel metrics", "DCGM"/"GPU metrics", "Container Insights metrics",
"managed scraper / Prometheus metrics", "claude_code.* metrics", "metrics aren't
showing in `list-metrics`", "the monitoring endpoint", or an explicit `cwpromql`
mention.

## HARD RULES

1. **These metrics are NOT in `aws cloudwatch list-metrics` / `get-metric-data`.**
   OTLP-ingested metrics (APS managed scrapers with `cloudWatchConfiguration`,
   ADOT OTLP metric export, DCGM/GPU, etc.) only surface through the
   Prometheus-compatible API. Do not report them "missing" based on `list-metrics`.
2. **Use `cwpromql`; do not hand-roll SigV4.** It already SigV4-signs (service
   `monitoring`) the `https://monitoring.<region>.amazonaws.com/api/v1/*` calls.
   Only fall back to a raw signed request if `cwpromql` is unavailable.
3. **Ensure AWS creds are available** (default credential chain or `--profile`).
   The identity needs `cloudwatch:GetMetricData` + `cloudwatch:ListMetrics`.
   Region defaults to **us-east-2**; pass `--region` for anything else.
4. **Every selector needs an exact metric-name matcher.** Regex-on-name
   (`{__name__=~".+"}`) or label-only selectors are rejected by CloudWatch with
   HTTP 400; `cwpromql` guards against this client-side.

## Install / build

```bash
go env -w GOPROXY=direct GOSUMDB=off   # if your network blocks proxy.golang.org
go install .                           # -> $(go env GOPATH)/bin/cwpromql
# or: go build -o bin/cwpromql .
```

## Commands

| Command | Purpose |
|---|---|
| `cwpromql query '<promql>'` | Instant query (single point in time) |
| `cwpromql range '<promql>' --since 1h [--step 60s] [--watch 10s]` | Range query → ASCII chart |
| `cwpromql metrics [--filter <substr>]` | List metric names (`__name__` values) |
| `cwpromql labels [--filter <substr>]` | List all label names |
| `cwpromql label-values <label>` | Values for one label |
| `cwpromql series '<selector>' [--since 1h]` | Series label sets matching a selector |

Global flags: `-o table|chart|json|csv`, `--limit N`, `--region <r>` (default
`us-east-2`), `--profile <name>`, `--no-color`. Default output is a **table** for
`query` and a **chart** for `range`; use `-o json` for scripting/parsing.
`range` extras: `--since`, `--step` (auto if unset), `--watch <interval>`
(live top-style refresh), `--height`, `--width`.

## CloudWatch PromQL dialect

- **Dotted metric names must be quoted:** `{"claude_code.token.usage"}`,
  `{"http.server.duration"}`. Bare names work too: `up`, `DCGM_FI_DEV_GPU_UTIL`.
- **OTel resource attributes use an `@resource.` prefix** (quote the whole label
  because of dots): `{"up","@resource.k8s.namespace.name"="prom-app-demo"}`,
  `sum by ("@resource.team.id")(...)`. Scope prefixes: `@resource.`,
  `@instrumentation.`, `@aws.` (system + tags); datapoint attributes are bare
  (`type`, `model`, `method`, `status`, `decision`).
- **OTLP counters are usually delta** — use `sum_over_time(...[1h])` for a
  windowed total and `rate(...[5m])` for a per-second rate.

## Examples

```bash
# Discover what's there
cwpromql metrics --filter claude          # claude_code.* metric names
cwpromql metrics --filter DCGM            # GPU/DCGM metric names
cwpromql label-values @resource.k8s.namespace.name

# GPU / Container Insights (DCGM)
cwpromql query '{"DCGM_FI_DEV_GPU_UTIL"}'
cwpromql range '{"DCGM_FI_DEV_GPU_UTIL"}' --since 1h --watch 15s

# Managed scraper (Go app)
cwpromql query 'sum by (path, status)(rate(http_requests_total[5m]))'
cwpromql query 'histogram_quantile(0.95, rate(http_request_duration_seconds[5m]))'

# Claude Code fleet
cwpromql query 'sum by ("@resource.team.id")(rate({"claude_code.token.usage"}[5m]))'
cwpromql query 'sum by ("@resource.user.name")(sum_over_time({"claude_code.cost.usage"}[1h]))'

# Inspect a series' full label set
cwpromql series '{"claude_code.token.usage"}' -o json | jq '.[0] | keys'
```

## Gotchas

- **Empty results:** the PromQL API only scans a **24-hour window**; widen
  `--since` within that limit and confirm the name with `cwpromql metrics --filter`.
- **500-series cap** on query/query_range; `cwpromql` surfaces the API `warnings`
  field on stderr when results are truncated — refine the query or pass `--limit`.
- **`command not found` after re-install:** run `hash -r` / `rehash`, or ensure
  `$(go env GOPATH)/bin` (or your symlink dir) is on `PATH`.
