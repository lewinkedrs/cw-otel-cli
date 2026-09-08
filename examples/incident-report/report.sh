#!/usr/bin/env bash
#
# report.sh — run a handful of cw-otel-cli queries and print one consolidated
# health report for a service fleet. Read-only and safe to run anytime; this is
# the kind of thing you'd keep in a repo and run when you get paged, rather than
# retyping individual queries.
#
# It degrades gracefully: a query that returns no data shows "n/a" instead of
# aborting the whole report.
#
# Configuration is via environment variables (sensible defaults shown):
#
#   AWS_REGION        (required)  region to query (cw-otel-cli reads it from env)
#   REQ_METRIC        http_requests_total   request counter with an outcome label
#   ERROR_LABEL       outcome="error"       selector marking error requests
#   SVC_LABEL         @resource.service.name  label to group services by
#   WINDOW            5m                    rate() look-back window
#   TOPN              5                     how many worst services to list
#   WARN_ERROR_RATIO  0.02                  error ratio that flips the verdict
#   CWPROMQL          cwpromql              path to the binary
#
# Example:
#   export AWS_REGION=us-east-1
#   ./report.sh

set -uo pipefail   # deliberately NOT -e: a report degrades, it doesn't abort

CWPROMQL="${CWPROMQL:-cwpromql}"
: "${AWS_REGION:?set AWS_REGION to the region to query}"

REQ_METRIC="${REQ_METRIC:-http_requests_total}"
ERROR_LABEL="${ERROR_LABEL:-outcome=\"error\"}"
SVC_LABEL="${SVC_LABEL:-@resource.service.name}"
WINDOW="${WINDOW:-5m}"
TOPN="${TOPN:-5}"
WARN_ERROR_RATIO="${WARN_ERROR_RATIO:-0.02}"

# scalar runs an instant query and echoes the single value, or "NODATA".
# The JSON shape is [{ "metric": {...}, "value": { "Time": .., "Value": "<str>" } }].
scalar() {
  "$CWPROMQL" query "$1" -o json 2>/dev/null \
    | jq -r 'if length == 0 then "NODATA" else .[0].value.Value end'
}

fmt_pct() { awk -v v="$1" 'BEGIN { if (v=="NODATA"||v=="null"||v=="") print "n/a"; else printf "%.2f%%", v*100 }'; }
fmt_num() { awk -v v="$1" 'BEGIN { if (v=="NODATA"||v=="null"||v=="") print "n/a"; else printf "%.3f", v }'; }

line="────────────────────────────────────────────────────────────"
echo "$line"
echo " Service health report"
echo " region:  $AWS_REGION"
echo " window:  last $WINDOW     generated: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
echo "$line"

# 1) Fleet-wide error ratio.
err_ratio="$(scalar "sum(rate({__name__=\"$REQ_METRIC\", $ERROR_LABEL}[$WINDOW])) / sum(rate({__name__=\"$REQ_METRIC\"}[$WINDOW]))")"
echo
echo "Error ratio (fleet):   $(fmt_pct "$err_ratio")"

# 2) Request throughput.
req_rate="$(scalar "sum(rate({__name__=\"$REQ_METRIC\"}[$WINDOW]))")"
echo "Request rate:          $(fmt_num "$req_rate") req/s"

# 3) Top-N services by error ratio (native CSV output: series,value,timestamp).
echo
echo "Top $TOPN services by error ratio:"
topn_query="sum by (\"$SVC_LABEL\")(rate({__name__=\"$REQ_METRIC\", $ERROR_LABEL}[$WINDOW])) / sum by (\"$SVC_LABEL\")(rate({__name__=\"$REQ_METRIC\"}[$WINDOW]))"
rows="$("$CWPROMQL" query "$topn_query" -o csv 2>/dev/null | tail -n +2 | sort -t, -k2 -gr | head -n "$TOPN")"
if [ -z "$rows" ]; then
  echo "  (no data)"
else
  printf '%s\n' "$rows" | while IFS=, read -r series value _; do
    name="${series#*=}"   # strip the "service.name=" prefix the CSV series carries
    printf "  %-40s %s\n" "$name" "$(fmt_pct "$value")"
  done
fi

# 4) Verdict.
echo
if [ "$err_ratio" = "NODATA" ] || [ "$err_ratio" = "null" ] || [ -z "$err_ratio" ]; then
  echo "Verdict: UNKNOWN — no request data in the last $WINDOW."
elif [ "$(awk -v v="$err_ratio" -v t="$WARN_ERROR_RATIO" 'BEGIN { print (v>t)?"1":"0" }')" = "1" ]; then
  echo "Verdict: DEGRADED — fleet error ratio $(fmt_pct "$err_ratio") exceeds $(fmt_pct "$WARN_ERROR_RATIO")."
else
  echo "Verdict: HEALTHY — fleet error ratio within $(fmt_pct "$WARN_ERROR_RATIO")."
fi
echo "$line"
