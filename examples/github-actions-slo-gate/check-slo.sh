#!/usr/bin/env bash
#
# check-slo.sh — query a CloudWatch OTel metric with cw-otel-cli and fail
# (non-zero exit) if it breaches a threshold. Designed to be run either in CI
# (see slo-gate.yml) or locally with the same binary.
#
# It "fails closed": if the query returns no data, that is treated as
# "cannot verify the SLO" and is a failure, not a pass.
#
# Configuration is via environment variables so the same script works in CI and
# locally:
#
#   SLO_QUERY   (required)  PromQL expression returning a single scalar series.
#   THRESHOLD   (required)  Numeric breach threshold.
#   COMPARISON  (optional)  "gt" (default) fails when value >  THRESHOLD;
#                           "lt"           fails when value <  THRESHOLD.
#   AWS_REGION  (required)  Region to query (cw-otel-cli reads it from the env).
#   CWPROMQL    (optional)  Path to the cwpromql binary (default: "cwpromql").
#
# Example (local):
#   export AWS_REGION=us-east-1
#   export SLO_QUERY='sum(rate({__name__="http_requests_total", outcome="error"}[5m])) / sum(rate({__name__="http_requests_total"}[5m]))'
#   export THRESHOLD=0.02
#   ./check-slo.sh

set -euo pipefail

: "${SLO_QUERY:?set SLO_QUERY to the PromQL expression to evaluate}"
: "${THRESHOLD:?set THRESHOLD to the numeric breach threshold}"
: "${AWS_REGION:?set AWS_REGION to the region to query}"
CWPROMQL="${CWPROMQL:-cwpromql}"
COMPARISON="${COMPARISON:-gt}"

echo "Querying SLO metric in ${AWS_REGION}..."
echo "  query:      ${SLO_QUERY}"
echo "  threshold:  ${COMPARISON} ${THRESHOLD}"

# -o json emits an array of series: [{ "metric": {...}, "value": { "Time": <float>, "Value": "<string>" } }]
# A single-scalar aggregation yields one element; the numeric value is the
# STRING field .value.Value, so we convert it with `tonumber`.
json="$("${CWPROMQL}" query "${SLO_QUERY}" -o json)"

value="$(printf '%s' "${json}" | jq -r 'if length == 0 then "NODATA" else (.[0].value.Value) end')"

if [ "${value}" = "NODATA" ] || [ "${value}" = "null" ]; then
  echo "::error::SLO query returned no data — cannot verify the SLO. Failing closed."
  exit 1
fi

echo "  observed:   ${value}"

# Numeric comparison via awk (portable, handles floats/scientific notation).
breached="$(awk -v v="${value}" -v t="${THRESHOLD}" -v c="${COMPARISON}" 'BEGIN {
  if (c == "lt") { print (v < t) ? "1" : "0" }
  else           { print (v > t) ? "1" : "0" }
}')"

if [ "${breached}" = "1" ]; then
  echo "::error::SLO BREACH — observed ${value} ${COMPARISON} threshold ${THRESHOLD}"
  exit 1
fi

echo "SLO OK — observed ${value} is within threshold ${THRESHOLD}."
