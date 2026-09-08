#!/usr/bin/env bash
#
# export.sh — export a per-service metric snapshot to CSV with cw-otel-cli, in a
# tidy long format that DuckDB can analyze (see analyze.sql).
#
# cw-otel-cli's native CSV output (from `query -o csv`) is:
#     series,value,timestamp
# We run two instant queries — request rate and error rate per service — and tag
# each row with a `metric` column, producing a long-format CSV:
#     series,value,timestamp,metric
#
# Configuration via environment variables:
#   AWS_REGION   (required)  region to query
#   REQ_METRIC   http_requests_total       request counter with an outcome label
#   SVC_LABEL    @resource.service.name    label to group services by
#   WINDOW       5m                        rate() look-back window
#   OUT          metrics.csv               output file
#   CWPROMQL     cwpromql                  path to the binary
#
# Note: this is a point-in-time snapshot (one value per series). For a full time
# series, `range` has no CSV output, but you can bridge its JSON:
#   cwpromql range '<expr>' --since 24h -o json \
#     | jq -r '.[] | .metric["@resource.service.name"] as $s
#              | .values[] | [$s, .Time, .Value] | @csv'

set -euo pipefail

CWPROMQL="${CWPROMQL:-cwpromql}"
: "${AWS_REGION:?set AWS_REGION to the region to query}"

REQ_METRIC="${REQ_METRIC:-http_requests_total}"
SVC_LABEL="${SVC_LABEL:-@resource.service.name}"
WINDOW="${WINDOW:-5m}"
OUT="${OUT:-metrics.csv}"

echo "Exporting per-service snapshot to ${OUT} (region ${AWS_REGION}, window ${WINDOW})..."

{
  echo "series,value,timestamp,metric"
  "$CWPROMQL" query "sum by (\"$SVC_LABEL\")(rate({__name__=\"$REQ_METRIC\"}[$WINDOW]))" -o csv \
    | tail -n +2 | sed 's/$/,request_rate/'
  "$CWPROMQL" query "sum by (\"$SVC_LABEL\")(rate({__name__=\"$REQ_METRIC\", outcome=\"error\"}[$WINDOW]))" -o csv \
    | tail -n +2 | sed 's/$/,error_rate/'
} > "$OUT"

rows=$(($(wc -l < "$OUT") - 1))
echo "Wrote ${OUT} (${rows} rows). Analyze it with:  duckdb -c \".read analyze.sql\""
